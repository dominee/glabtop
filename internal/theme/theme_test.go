package theme

import (
	"path/filepath"
	"testing"
)

func TestLoadCatppuccinMocha(t *testing.T) {
	p := filepath.Join("..", "..", "themes", "catppuccin_mocha.theme")
	th, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if th.Hex("main_bg", "") != "#1E1E2E" {
		t.Fatalf("main_bg got %q", th.Hex("main_bg", ""))
	}
}
