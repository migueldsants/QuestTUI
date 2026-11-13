package config

import (
	"os"
	"path/filepath"
	"testing"
)

// Uma configuração que não veio de Load não grava nada; uma que veio grava
// no mesmo arquivo de onde foi lida.
func TestSaveOnlyLoadedConfig(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("APPDATA", dir)
	file := filepath.Join(dir, "questtui", "config.toml")

	if err := (&Config{Icons: "ascii"}).Save(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Fatal("uma configuração montada à mão gravou o arquivo")
	}

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Icons != "nerd" {
		t.Fatalf("ícones padrão: %q", cfg.Icons)
	}
	cfg.Icons = "unicode"
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	again, err := Load()
	if err != nil || again.Icons != "unicode" {
		t.Fatalf("depois de gravar: %q, %v", again.Icons, err)
	}
}
