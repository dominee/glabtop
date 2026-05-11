package tui

import (
	"github.com/charmbracelet/lipgloss"
)

func (m *Model) paletteAt(i int) lipgloss.Color {
	if m == nil || m.theme == nil {
		return lipgloss.Color("#89B4FA")
	}
	p := m.theme.ChartPalette(8)
	if len(p) == 0 {
		return lipgloss.Color(m.theme.Hex("hi_fg", "#89B4FA"))
	}
	return p[i%len(p)]
}

func (m *Model) paletteBold(i int) lipgloss.Style {
	return lipgloss.NewStyle().Bold(true).Foreground(m.paletteAt(i))
}

func accentIndex(s string, mod int) int {
	if mod <= 0 {
		return 0
	}
	h := 0
	for _, r := range s {
		h = h*31 + int(r)
	}
	if h < 0 {
		h = -h
	}
	return h % mod
}
