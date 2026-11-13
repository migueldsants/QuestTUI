// Package config cuida da pasta de dados e do arquivo config.toml.
package config

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Theme       string `toml:"theme"`
	IdleMinutes int    `toml:"idle_minutes"` // 0 desliga a vigília automática
	Clock24h    bool   `toml:"clock_24h"`
	Icons       string `toml:"icons"` // nerd, unicode ou ascii
	Lang        string `toml:"lang"`  // pt ou en

	Sound        bool `toml:"sound"`         // efeitos sonoros ao concluir
	Reminders    bool `toml:"reminders"`     // notificações de prazo enquanto o diário está aberto
	FocusMinutes int  `toml:"focus_minutes"` // duração de cada rodada do modo foco
	BreakMinutes int  `toml:"break_minutes"` // duração da pausa entre rodadas

	EditorMode     string `toml:"editor_mode"`     // editor das notas: "padrao" ou "vim"
	ExternalEditor string `toml:"external_editor"` // editor externo (^E); vazio usa $VISUAL, $EDITOR ou vim
	NotesPreview   bool   `toml:"notes_preview"`   // prévia do markdown ao lado do editor

	file string // arquivo de onde foi lida; vazio não grava
}

func defaults() *Config {
	return &Config{
		Theme: "heroi", IdleMinutes: 5, Clock24h: true, Icons: "nerd", Lang: "pt",
		Reminders: true, FocusMinutes: 25, BreakMinutes: 5,
		EditorMode: "padrao", NotesPreview: true,
	}
}

// Dir devolve (e cria) a pasta de dados do app.
func Dir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, "questtui")
	if err := os.MkdirAll(filepath.Join(dir, "themes"), 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

func path() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.toml"), nil
}

// Load lê o config.toml; se ele não existir, cria um com os padrões.
func Load() (*Config, error) {
	p, err := path()
	if err != nil {
		return nil, err
	}
	cfg := defaults()
	cfg.file = p
	if _, err := toml.DecodeFile(p, cfg); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return cfg, cfg.Save()
		}
		return nil, err
	}
	return cfg, nil
}

// Save grava o config.toml de onde a configuração foi lida. Uma configuração
// montada à mão (nos testes, por exemplo) não veio de arquivo nenhum e não é
// gravada, para nunca sobrescrever a configuração de verdade.
func (c *Config) Save() error {
	if c.file == "" {
		return nil
	}
	f, err := os.Create(c.file)
	if err != nil {
		return err
	}
	defer f.Close()
	return toml.NewEncoder(f).Encode(c)
}
