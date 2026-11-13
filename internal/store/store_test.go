package store

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"
)

// Quinta-feira, 12/11/2026, 10:00.
var now = time.Date(2026, 11, 12, 10, 0, 0, 0, time.Local)

func TestParseDue(t *testing.T) {
	cases := map[string]string{
		"":                 "",
		"hoje":             "2026-11-12",
		"amanhã":           "2026-11-13",
		"amanha 14h":       "2026-11-13 14:00",
		"sexta":            "2026-11-13",
		"quinta":           "2026-11-12",
		"segunda-feira":    "2026-11-16",
		"sexta às 9h30":    "2026-11-13 09:30",
		"friday at 18:45":  "2026-11-13 18:45",
		"+3d":              "2026-11-15",
		"+2s":              "2026-11-26",
		"em 1 mês":         "2026-12-12",
		"20/11 14:00":      "2026-11-20 14:00",
		"20/11/2027":       "2027-11-20",
		"05/01":            "2027-01-05", // já passou neste ano
		"14:00":            "2026-11-12 14:00",
		"12/11/2026 08:00": "2026-11-12 08:00",
	}
	for in, want := range cases {
		got, err := ParseDue(in, now)
		if err != nil || got != want {
			t.Errorf("ParseDue(%q) = %q, %v; esperava %q", in, got, err, want)
		}
	}
	for _, bad := range []string{"ontem?", "31/02", "25:00", "sextaa", "+3x"} {
		if got, err := ParseDue(bad, now); err == nil {
			t.Errorf("ParseDue(%q) = %q, esperava erro", bad, got)
		}
	}
	// O que FormatDue mostra pode ser digitado de volta.
	for _, due := range []string{"2026-11-20", "2026-11-20 14:05"} {
		if got, _ := ParseDue(FormatDue(due), now); got != due {
			t.Errorf("ida e volta de %q deu %q", due, got)
		}
	}
}

// Um banco de uma versão anterior ganha as colunas novas, a ordem manual e o
// histórico das quests que já estavam concluídas.
func TestMigrateOldDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	for _, stmt := range []string{
		`CREATE TABLE quests (id INTEGER PRIMARY KEY AUTOINCREMENT, title TEXT NOT NULL, notes TEXT NOT NULL DEFAULT '',
			kind TEXT NOT NULL DEFAULT 'side', done INTEGER NOT NULL DEFAULT 0, tracked INTEGER NOT NULL DEFAULT 0,
			due TEXT NOT NULL DEFAULT '', created_at TEXT NOT NULL, done_at TEXT NOT NULL DEFAULT '')`,
		`INSERT INTO quests (title, created_at) VALUES ('pendente', '2026-11-01T10:00:00Z')`,
		`INSERT INTO quests (title, done, created_at, done_at) VALUES ('feita', 1, '2026-11-01T10:00:00Z', '2026-11-02T10:00:00Z')`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	db.Close()

	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	quests, err := s.Quests()
	if err != nil || len(quests) != 2 {
		t.Fatalf("Quests() = %d, %v", len(quests), err)
	}
	if quests[0].Position != 1 || quests[1].Position != 2 {
		t.Errorf("posições %d e %d, esperava 1 e 2", quests[0].Position, quests[1].Position)
	}
	if hist, _ := s.Completions(); len(hist) != 1 || hist[0].Title != "feita" {
		t.Errorf("histórico migrado: %+v", hist)
	}
	// Abrir de novo não duplica o histórico.
	s.Close()
	if s, err = Open(path); err != nil {
		t.Fatal(err)
	}
	if hist, _ := s.Completions(); len(hist) != 1 {
		t.Errorf("histórico duplicado ao reabrir: %d", len(hist))
	}
}

func TestCompleteAndHistory(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "q.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	plain := Quest{Title: "uma vez"}
	weekly := Quest{Title: "regar plantas", Due: DateOnly(time.Now()).Format(dateLayout)}
	for _, q := range []*Quest{&plain, &weekly} {
		if err := s.AddQuest(q); err != nil {
			t.Fatal(err)
		}
	}
	for _, id := range []int64{plain.ID, weekly.ID} {
		if err := s.Complete(id); err != nil {
			t.Fatal(err)
		}
	}
	// Concluir de novo não duplica o histórico.
	if err := s.Complete(plain.ID); err != nil {
		t.Fatal(err)
	}
	quests, _ := s.Quests()
	if !quests[0].Done || !quests[1].Done {
		t.Error("as duas quests deveriam estar concluídas")
	}

	hist, _ := s.Completions()
	if len(hist) != 2 {
		t.Fatalf("histórico com %d conclusões, esperava 2", len(hist))
	}
	if err := s.Reopen(plain.ID); err != nil {
		t.Fatal(err)
	}
	if hist, _ = s.Completions(); len(hist) != 1 {
		t.Fatalf("depois de reabrir, histórico com %d", len(hist))
	}
	if err := s.Reopen(weekly.ID); err != nil {
		t.Fatal(err)
	}

	// Ordem manual e prioridade.
	if err := s.Swap(plain.ID, weekly.ID); err != nil {
		t.Fatal(err)
	}
	quests, _ = s.Quests()
	if quests[0].ID != weekly.ID {
		t.Error("Swap não trocou a ordem")
	}
	if err := s.SetPriority(plain.ID, true); err != nil {
		t.Fatal(err)
	}

	// Editar: prioridade pelo UpdateQuest, etapas renomeadas e reordenadas.
	q := quests[1]
	q.Priority, q.Title = false, "uma vez (editada)"
	if err := s.UpdateQuest(q); err != nil {
		t.Fatal(err)
	}
	for _, title := range []string{"a", "b"} {
		if err := s.AddStep(plain.ID, title); err != nil {
			t.Fatal(err)
		}
	}
	quests, _ = s.Quests()
	steps := quests[1].Steps
	if quests[1].Title != "uma vez (editada)" || len(steps) != 2 {
		t.Fatalf("depois de editar: %q com %d etapas", quests[1].Title, len(steps))
	}
	if err := s.RenameStep(steps[0].ID, "primeira"); err != nil {
		t.Fatal(err)
	}
	if err := s.SwapSteps(steps[0].ID, steps[1].ID); err != nil {
		t.Fatal(err)
	}
	quests, _ = s.Quests()
	if got := quests[1].Steps; got[0].Title != "b" || got[1].Title != "primeira" {
		t.Errorf("etapas depois de renomear e trocar: %q, %q", got[0].Title, got[1].Title)
	}

	// Lembretes: só o primeiro registro conta.
	first, _ := s.MarkNotified(plain.ID, "today", "2026-11-12")
	again, _ := s.MarkNotified(plain.ID, "today", "2026-11-12")
	if !first || again {
		t.Errorf("MarkNotified: primeiro=%v, de novo=%v", first, again)
	}
}
