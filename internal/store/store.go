// Package store guarda as quests em um banco SQLite local.
package store

import (
	"database/sql"
	"time"

	_ "modernc.org/sqlite"
)

type Kind string

const (
	Main Kind = "main"
	Side Kind = "side"
)

type Step struct {
	ID      int64
	QuestID int64
	Title   string
	Done    bool
}

type Quest struct {
	ID        int64
	Title     string
	Notes     string
	Kind      Kind
	Done      bool
	Tracked   bool
	Priority  bool   // prioridade alta: sobe para o topo do grupo
	Due       string // "AAAA-MM-DD" ou "AAAA-MM-DD HH:MM"; vazio é sem prazo
	Position  int64  // ordem manual dentro da lista
	CreatedAt time.Time
	DoneAt    time.Time
	Steps     []Step
}

// StepsDone devolve quantas etapas já foram concluídas.
func (q Quest) StepsDone() int {
	n := 0
	for _, s := range q.Steps {
		if s.Done {
			n++
		}
	}
	return n
}

// CurrentStep devolve o índice da primeira etapa pendente (o objetivo atual),
// ou -1 quando não há etapas pendentes.
func (q Quest) CurrentStep() int {
	for i, s := range q.Steps {
		if !s.Done {
			return i
		}
	}
	return -1
}

// Completion é uma conclusão registrada no histórico. Quests recorrentes
// deixam uma por vez que são concluídas.
type Completion struct {
	QuestID int64
	Title   string
	Kind    Kind
	At      time.Time
}

type Store struct {
	db *sql.DB
}

const schema = `
CREATE TABLE IF NOT EXISTS quests (
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	title      TEXT NOT NULL,
	notes      TEXT NOT NULL DEFAULT '',
	kind       TEXT NOT NULL DEFAULT 'side',
	done       INTEGER NOT NULL DEFAULT 0,
	tracked    INTEGER NOT NULL DEFAULT 0,
	due        TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL,
	done_at    TEXT NOT NULL DEFAULT ''
);
CREATE TABLE IF NOT EXISTS steps (
	id       INTEGER PRIMARY KEY AUTOINCREMENT,
	quest_id INTEGER NOT NULL,
	title    TEXT NOT NULL,
	done     INTEGER NOT NULL DEFAULT 0
);
-- colunas acrescentadas depois (priority, position) vêm em migrate()
CREATE INDEX IF NOT EXISTS steps_quest ON steps(quest_id);
CREATE TABLE IF NOT EXISTS notified (
	quest_id INTEGER NOT NULL,
	kind     TEXT NOT NULL,
	mark     TEXT NOT NULL,
	PRIMARY KEY (quest_id, kind, mark)
);
`

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	for _, stmt := range []string{"PRAGMA busy_timeout = 3000", schema} {
		if _, err := db.Exec(stmt); err != nil {
			db.Close()
			return nil, err
		}
	}
	if err := migrate(db); err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

// columns devolve os nomes das colunas de uma tabela.
func columns(db *sql.DB, table string) (map[string]bool, error) {
	rows, err := db.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cols := map[string]bool{}
	for rows.Next() {
		var cid, notnull, pk int
		var name, typ string
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
			return nil, err
		}
		cols[name] = true
	}
	return cols, rows.Err()
}

// migrate atualiza bancos criados por versões anteriores.
func migrate(db *sql.DB) error {
	added := []struct{ table, name, def, fill string }{
		{"quests", "priority", "INTEGER NOT NULL DEFAULT 0", ""},
		{"quests", "position", "INTEGER NOT NULL DEFAULT 0", "UPDATE quests SET position = id"},
		{"steps", "position", "INTEGER NOT NULL DEFAULT 0", "UPDATE steps SET position = id"},
	}
	for _, c := range added {
		cols, err := columns(db, c.table)
		if err != nil {
			return err
		}
		if cols[c.name] {
			continue
		}
		if _, err := db.Exec(`ALTER TABLE ` + c.table + ` ADD COLUMN ` + c.name + ` ` + c.def); err != nil {
			return err
		}
		if c.fill != "" {
			if _, err := db.Exec(c.fill); err != nil {
				return err
			}
		}
	}

	// O histórico nasce com as quests já concluídas, para as conquistas
	// contarem o que foi feito antes dele existir.
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'completions'`).Scan(&n); err != nil {
		return err
	}
	if n == 0 {
		stmts := []string{
			`CREATE TABLE completions (
				id       INTEGER PRIMARY KEY AUTOINCREMENT,
				quest_id INTEGER NOT NULL,
				title    TEXT NOT NULL,
				kind     TEXT NOT NULL,
				done_at  TEXT NOT NULL
			)`,
			`INSERT INTO completions (quest_id, title, kind, done_at)
				SELECT id, title, kind, done_at FROM quests WHERE done = 1 AND done_at != '' ORDER BY done_at`,
		}
		for _, stmt := range stmts {
			if _, err := db.Exec(stmt); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Store) Close() error { return s.db.Close() }

// Quests devolve todas as quests com suas etapas, na ordem manual.
func (s *Store) Quests() ([]Quest, error) {
	rows, err := s.db.Query(`SELECT id, title, notes, kind, done, tracked, priority, due, position, created_at, done_at
		FROM quests ORDER BY position, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var quests []Quest
	index := map[int64]int{}
	for rows.Next() {
		var q Quest
		var kind, created, doneAt string
		if err := rows.Scan(&q.ID, &q.Title, &q.Notes, &kind, &q.Done, &q.Tracked, &q.Priority,
			&q.Due, &q.Position, &created, &doneAt); err != nil {
			return nil, err
		}
		q.Kind = Kind(kind)
		q.CreatedAt, _ = time.Parse(time.RFC3339, created)
		q.DoneAt, _ = time.Parse(time.RFC3339, doneAt)
		index[q.ID] = len(quests)
		quests = append(quests, q)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	srows, err := s.db.Query(`SELECT id, quest_id, title, done FROM steps ORDER BY position, id`)
	if err != nil {
		return nil, err
	}
	defer srows.Close()
	for srows.Next() {
		var st Step
		if err := srows.Scan(&st.ID, &st.QuestID, &st.Title, &st.Done); err != nil {
			return nil, err
		}
		if i, ok := index[st.QuestID]; ok {
			quests[i].Steps = append(quests[i].Steps, st)
		}
	}
	return quests, srows.Err()
}

// AddQuest cria a quest no fim da lista.
func (s *Store) AddQuest(q *Quest) error {
	if q.Kind == "" {
		q.Kind = Side
	}
	q.CreatedAt = time.Now()
	res, err := s.db.Exec(`INSERT INTO quests (title, notes, kind, due, priority, position, created_at)
		VALUES (?, ?, ?, ?, ?, (SELECT COALESCE(MAX(position), 0) + 1 FROM quests), ?)`,
		q.Title, q.Notes, string(q.Kind), q.Due, q.Priority, q.CreatedAt.Format(time.RFC3339))
	if err != nil {
		return err
	}
	q.ID, err = res.LastInsertId()
	return err
}

// UpdateQuest grava título, notas, tipo, prazo e prioridade.
func (s *Store) UpdateQuest(q Quest) error {
	_, err := s.db.Exec(`UPDATE quests SET title = ?, notes = ?, kind = ?, due = ?, priority = ? WHERE id = ?`,
		q.Title, q.Notes, string(q.Kind), q.Due, q.Priority, q.ID)
	return err
}

// SetNotes grava as notas da quest.
func (s *Store) SetNotes(id int64, notes string) error {
	_, err := s.db.Exec(`UPDATE quests SET notes = ? WHERE id = ?`, notes, id)
	return err
}

func (s *Store) DeleteQuest(id int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, stmt := range []string{
		`DELETE FROM steps WHERE quest_id = ?`,
		`DELETE FROM notified WHERE quest_id = ?`,
		`DELETE FROM quests WHERE id = ?`,
	} {
		if _, err := tx.Exec(stmt, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// Complete conclui a quest e registra a conclusão no histórico.
func (s *Store) Complete(id int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var title, kind string
	var done bool
	if err := tx.QueryRow(`SELECT title, kind, done FROM quests WHERE id = ?`, id).Scan(&title, &kind, &done); err != nil {
		return err
	}
	if done {
		return nil
	}
	stamp := time.Now().Format(time.RFC3339)
	if _, err := tx.Exec(`INSERT INTO completions (quest_id, title, kind, done_at) VALUES (?, ?, ?, ?)`,
		id, title, kind, stamp); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE quests SET done = 1, tracked = 0, done_at = ? WHERE id = ?`, stamp, id); err != nil {
		return err
	}
	return tx.Commit()
}

// Reopen reabre uma quest concluída e tira a última conclusão do histórico.
func (s *Store) Reopen(id int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`UPDATE quests SET done = 0, done_at = '' WHERE id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM completions WHERE id = (SELECT MAX(id) FROM completions WHERE quest_id = ?)`, id); err != nil {
		return err
	}
	return tx.Commit()
}

// ToggleTracked rastreia a quest (só uma por vez) ou tira o rastreio dela.
func (s *Store) ToggleTracked(id int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var tracked bool
	if err := tx.QueryRow(`SELECT tracked FROM quests WHERE id = ?`, id).Scan(&tracked); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE quests SET tracked = 0`); err != nil {
		return err
	}
	if !tracked {
		if _, err := tx.Exec(`UPDATE quests SET tracked = 1 WHERE id = ?`, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) SetPriority(id int64, high bool) error {
	_, err := s.db.Exec(`UPDATE quests SET priority = ? WHERE id = ?`, high, id)
	return err
}

// Swap troca duas quests de lugar na ordem manual.
func (s *Store) Swap(a, b int64) error { return s.swap("quests", a, b) }

// SwapSteps troca duas etapas de lugar.
func (s *Store) SwapSteps(a, b int64) error { return s.swap("steps", a, b) }

// swap troca as posições de duas linhas. As duas posições são lidas antes:
// numa única UPDATE com subconsultas, a segunda linha já veria a primeira
// alterada.
func (s *Store) swap(table string, a, b int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var pa, pb int64
	if err := tx.QueryRow(`SELECT position FROM `+table+` WHERE id = ?`, a).Scan(&pa); err != nil {
		return err
	}
	if err := tx.QueryRow(`SELECT position FROM `+table+` WHERE id = ?`, b).Scan(&pb); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE `+table+` SET position = ? WHERE id = ?`, pb, a); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE `+table+` SET position = ? WHERE id = ?`, pa, b); err != nil {
		return err
	}
	return tx.Commit()
}

// AddStep acrescenta a etapa no fim da quest.
func (s *Store) AddStep(questID int64, title string) error {
	_, err := s.db.Exec(`INSERT INTO steps (quest_id, title, position)
		VALUES (?, ?, (SELECT COALESCE(MAX(position), 0) + 1 FROM steps WHERE quest_id = ?))`, questID, title, questID)
	return err
}

func (s *Store) RenameStep(id int64, title string) error {
	_, err := s.db.Exec(`UPDATE steps SET title = ? WHERE id = ?`, title, id)
	return err
}

func (s *Store) SetStepDone(id int64, done bool) error {
	_, err := s.db.Exec(`UPDATE steps SET done = ? WHERE id = ?`, done, id)
	return err
}

func (s *Store) DeleteStep(id int64) error {
	_, err := s.db.Exec(`DELETE FROM steps WHERE id = ?`, id)
	return err
}

// Completions devolve o histórico de conclusões, do mais antigo ao mais novo.
func (s *Store) Completions() ([]Completion, error) {
	rows, err := s.db.Query(`SELECT quest_id, title, kind, done_at FROM completions ORDER BY done_at, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Completion
	for rows.Next() {
		var c Completion
		var kind, at string
		if err := rows.Scan(&c.QuestID, &c.Title, &kind, &at); err != nil {
			return nil, err
		}
		c.Kind = Kind(kind)
		if c.At, err = time.Parse(time.RFC3339, at); err != nil {
			continue
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// MarkNotified registra que um lembrete foi enviado. first é false quando ele
// já tinha sido registrado antes, para não repetir a notificação.
func (s *Store) MarkNotified(questID int64, kind, mark string) (first bool, err error) {
	res, err := s.db.Exec(`INSERT OR IGNORE INTO notified (quest_id, kind, mark) VALUES (?, ?, ?)`, questID, kind, mark)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}
