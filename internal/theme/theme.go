package theme

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var themeLine = regexp.MustCompile(`^theme\[([^]]+)]\s*=\s*"(#[0-9A-Fa-f]{6})"`)

// Theme maps btop keys to hex colors for lipgloss.
type Theme struct {
	Colors map[string]string
}

func Load(path string) (*Theme, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	t := &Theme{Colors: make(map[string]string)}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		m := themeLine.FindStringSubmatch(line)
		if len(m) == 3 {
			t.Colors[m[1]] = m[2]
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if len(t.Colors) == 0 {
		return nil, fmt.Errorf("no theme[key] lines parsed from %s", path)
	}
	return t, nil
}

func (t *Theme) Hex(key, fallback string) string {
	if v := t.Colors[key]; v != "" {
		return v
	}
	return fallback
}

func (t *Theme) Color(key, fallback string) lipgloss.Color {
	return lipgloss.Color(t.Hex(key, fallback))
}

// Styles returns commonly used lipgloss styles for the TUI.
func (t *Theme) Styles() Styles {
	return Styles{
		Base:     lipgloss.NewStyle().Foreground(t.Color("main_fg", "#CDD6F4")).Background(t.Color("main_bg", "#1E1E2E")),
		Title:    lipgloss.NewStyle().Bold(true).Foreground(t.Color("title", "#CDD6F4")),
		Hi:       lipgloss.NewStyle().Foreground(t.Color("hi_fg", "#89B4FA")),
		Inactive: lipgloss.NewStyle().Foreground(t.Color("inactive_fg", "#7F849C")),
		Border: lipgloss.Border{Top: "─", Bottom: "─", Left: "│", Right: "│",
			TopLeft: "╭", TopRight: "╮", BottomLeft: "╰", BottomRight: "╯"},
		BorderStyle: lipgloss.NewStyle().Foreground(t.Color("div_line", "#6C7086")),
		Selected:    lipgloss.NewStyle().Background(t.Color("selected_bg", "#45475A")).Foreground(t.Color("selected_fg", "#89B4FA")),
		MeterLow:    t.Color("cpu_start", "#74C7EC"),
		MeterMid:    t.Color("cpu_mid", "#89DCEB"),
		MeterHigh:   t.Color("cpu_end", "#94E2D5"),
	}
}

type Styles struct {
	Base        lipgloss.Style
	Title       lipgloss.Style
	Hi          lipgloss.Style
	Inactive    lipgloss.Style
	Border      lipgloss.Border
	BorderStyle lipgloss.Style
	Selected    lipgloss.Style
	MeterLow    lipgloss.Color
	MeterMid    lipgloss.Color
	MeterHigh   lipgloss.Color
}
