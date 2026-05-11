package theme

import (
	"strings"
	"testing"
)

func TestChartPaletteLen(t *testing.T) {
	tm := &Theme{Colors: map[string]string{
		"c1": "#ff6b6b",
		"c2": "#4ecdc4",
		"c3": "#ffe66d",
		"c4": "#95e1d3",
		"c5": "#f38181",
		"c6": "#aa96da",
		"main_bg": "#2d3436",
		"main_fg": "#dfe6e9",
	}}
	p := tm.ChartPalette(5)
	if len(p) != 5 {
		t.Fatalf("len %d", len(p))
	}
}

func TestChartPaletteFallback(t *testing.T) {
	tm := &Theme{Colors: map[string]string{
		"hi_fg": "#89b4fa",
	}}
	p := tm.ChartPalette(3)
	if len(p) != 3 {
		t.Fatal(p)
	}
}

func TestChartPaletteExcludesBackground(t *testing.T) {
	tm := &Theme{Colors: map[string]string{
		"main_bg": "#1e1e2e",
		"hi_fg":   "#89b4fa",
		"dup":     "#1e1e2e",
		"pink":    "#f38ba8",
		"green":   "#a6e3a1",
	}}
	p := tm.ChartPalette(4)
	bg := strings.ToLower(tm.Hex("main_bg", ""))
	for _, c := range p {
		if strings.ToLower(string(c)) == bg {
			t.Fatalf("palette includes background: %v", p)
		}
	}
	if len(p) < 4 {
		t.Fatal(p)
	}
}

func TestChartPaletteStarshipRainbowThenGenerative(t *testing.T) {
	tm := &Theme{Colors: map[string]string{
		"main_bg": "#2d3436",
		"hi_fg":   "#89b4fa",
	}}
	p := tm.ChartPalette(8)
	if strings.ToLower(string(p[0])) != "#f38ba8" {
		t.Fatalf("first slot should be Starship catppuccin red, got %q", p[0])
	}
	// Later slots use generative / theme fill; still distinct type-chart indices.
	a := strings.ToLower(string(p[0]))
	b := strings.ToLower(string(p[3]))
	c := strings.ToLower(string(p[6]))
	if a == b || b == c || a == c {
		t.Fatalf("indices 0,3,6: %q %q %q", a, b, c)
	}
}
