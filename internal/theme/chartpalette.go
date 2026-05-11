package theme

import (
	"math"
	"strings"
	"sync"

	"github.com/charmbracelet/lipgloss"
	"github.com/lucasb-eyer/go-colorful"
)

const chartPaletteSize = 8

// goldenAngleHueDegrees spaces successive generated hues for perceptual spread (Phyllotaxis-style).
const goldenAngleHueDegrees = 137.50776405003785

// minChartDistFromBG is a minimum DistanceLab from go-colorful (same scale as greedyMaxMinLab: ~0 identical,
// around 1+ for very different colors per library docs) so bars stay off the window background.
const minChartDistFromBG = 0.18

var themeChartPaletteCache sync.Map // *Theme -> []lipgloss.Color

// ChartPalette returns up to n colors from the loaded btop theme, chosen to be
// well separated in Lab space so stacked segments remain distinguishable while
// staying on-theme. Cached per Theme pointer.
func (t *Theme) ChartPalette(n int) []lipgloss.Color {
	if t == nil || n < 1 {
		return nil
	}
	if n > chartPaletteSize {
		n = chartPaletteSize
	}
	v, ok := themeChartPaletteCache.Load(t)
	if !ok {
		p := buildChartPaletteForTheme(t, chartPaletteSize)
		themeChartPaletteCache.Store(t, p)
		v = p
	}
	full := v.([]lipgloss.Color)
	if len(full) == 0 {
		return fallbackChartPalette(t, n)
	}
	if n > len(full) {
		n = len(full)
	}
	out := make([]lipgloss.Color, n)
	copy(out, full[:n])
	return out
}

func buildChartPaletteForTheme(t *Theme, n int) []lipgloss.Color {
	bg, bgHex, hasBg := chartExcludeBackground(t)
	hexes := distinctChartCandidateHexes(t)
	cols := parseAndFilterChartColors(hexes, bg, bgHex, hasBg)
	picked := greedyMaxMinLab(cols, n)
	if len(picked) == 0 {
		return fallbackChartPalette(t, n)
	}
	return extendPaletteFromSeeds(picked, n, bg, bgHex, hasBg)
}

// extendPaletteFromSeeds fills targetN distinct chart colors starting from Lab-spread seeds.
// If few theme entries survive filtering, we hue-spin from those seeds instead of repeating one hex.
func extendPaletteFromSeeds(seeds []labColor, targetN int, bg colorful.Color, bgHexNorm string, hasBg bool) []lipgloss.Color {
	if targetN < 1 || len(seeds) == 0 {
		return nil
	}
	seen := make(map[string]struct{})
	out := make([]lipgloss.Color, 0, targetN)
	tryAdd := func(c colorful.Color) bool {
		hex := strings.ToLower(c.Hex())
		if _, ok := seen[hex]; ok {
			return false
		}
		if tooCloseToBackground(c, hex, bg, bgHexNorm, hasBg) {
			return false
		}
		_, s, l := c.Hsl()
		if l < 0.07 || l > 0.94 || s < 0.11 {
			return false
		}
		seen[hex] = struct{}{}
		out = append(out, lipgloss.Color(hex))
		return true
	}
	for _, s := range seeds {
		if len(out) >= targetN {
			break
		}
		tryAdd(s.c)
	}
	for spin := 1; len(out) < targetN && spin <= 256; spin++ {
		base := seeds[(spin-1)%len(seeds)]
		h, sat, v := base.c.Hsv()
		if sat < 0.18 {
			sat = 0.52
		}
		if v < 0.22 {
			v = 0.45
		} else if v > 0.93 {
			v = 0.82
		}
		nh := math.Mod(h+float64(spin)*goldenAngleHueDegrees, 360)
		tryAdd(colorful.Hsv(nh, sat, v))
	}
	if len(out) < targetN {
		for _, eh := range []string{
			"#89b4fa", "#f38ba8", "#a6e3a1", "#f9e2af", "#cba6f7", "#fab387", "#94e2d5", "#f5c2e7",
		} {
			if len(out) >= targetN {
				break
			}
			c, err := colorful.Hex(eh)
			if err != nil {
				continue
			}
			tryAdd(c)
		}
	}
	if len(out) == 0 {
		out = append(out, lipgloss.Color("#89b4fa"))
	}
	for len(out) < targetN {
		out = append(out, out[len(out)-1])
	}
	return out[:targetN]
}

func distinctChartCandidateHexes(t *Theme) []string {
	seen := make(map[string]struct{})
	var out []string
	add := func(h string) {
		h = strings.TrimSpace(h)
		if len(h) < 7 || !strings.HasPrefix(h, "#") {
			return
		}
		k := strings.ToLower(h)
		if _, ok := seen[k]; ok {
			return
		}
		seen[k] = struct{}{}
		out = append(out, h)
	}
	for k, v := range t.Colors {
		if chartSkipThemeKey(k) {
			continue
		}
		add(v)
	}
	for _, key := range []string{"hi_fg", "title", "cpu_start", "cpu_mid", "cpu_end", "mem_start", "mem_end", "net_download", "net_upload", "proc", "gpu_start", "temp_start"} {
		add(t.Hex(key, ""))
	}
	return out
}

func chartSkipThemeKey(k string) bool {
	switch strings.ToLower(k) {
	case "main_bg", "selected_bg", "graph_bg", "cpu_graph_bg", "mem_graph_bg", "net_graph_bg", "proc_graph_bg":
		return true
	default:
		return false
	}
}

func chartExcludeBackground(t *Theme) (bg colorful.Color, bgHexNorm string, ok bool) {
	h := strings.TrimSpace(t.Hex("main_bg", ""))
	if h == "" {
		return colorful.Color{}, "", false
	}
	c, err := colorful.Hex(h)
	if err != nil {
		return colorful.Color{}, "", false
	}
	return c, strings.ToLower(strings.TrimSpace(h)), true
}

func tooCloseToBackground(c colorful.Color, h string, bg colorful.Color, bgHexNorm string, hasBg bool) bool {
	if !hasBg {
		return false
	}
	if strings.ToLower(strings.TrimSpace(h)) == bgHexNorm {
		return true
	}
	return c.DistanceLab(bg) < minChartDistFromBG
}

type labColor struct {
	hex string
	c   colorful.Color
}

func parseAndFilterChartColors(hexes []string, bg colorful.Color, bgHexNorm string, hasBg bool) []labColor {
	var out []labColor
	for _, h := range hexes {
		c, err := colorful.Hex(h)
		if err != nil {
			continue
		}
		if tooCloseToBackground(c, h, bg, bgHexNorm, hasBg) {
			continue
		}
		_, s, l := c.Hsl()
		if l < 0.07 || l > 0.94 {
			continue
		}
		if s < 0.11 {
			continue
		}
		out = append(out, labColor{hex: strings.ToLower(h), c: c})
	}
	return out
}

func greedyMaxMinLab(cs []labColor, n int) []labColor {
	if len(cs) == 0 || n <= 0 {
		return nil
	}
	if len(cs) <= n {
		return cs
	}
	start := 0
	bestSum := -1.0
	for i := range cs {
		sum := 0.0
		for j := range cs {
			if i != j {
				sum += cs[i].c.DistanceLab(cs[j].c)
			}
		}
		if sum > bestSum {
			bestSum = sum
			start = i
		}
	}
	picked := []labColor{cs[start]}
	inPicked := map[int]struct{}{start: {}}
	for len(picked) < n {
		bestI := -1
		bestMin := -1.0
		for i := range cs {
			if _, ok := inPicked[i]; ok {
				continue
			}
			minD := math.MaxFloat64
			for _, p := range picked {
				d := cs[i].c.DistanceLab(p.c)
				if d < minD {
					minD = d
				}
			}
			if minD > bestMin {
				bestMin = minD
				bestI = i
			}
		}
		if bestI < 0 {
			break
		}
		inPicked[bestI] = struct{}{}
		picked = append(picked, cs[bestI])
	}
	return picked
}

func fallbackChartPalette(t *Theme, n int) []lipgloss.Color {
	bg, bgHex, hasBg := chartExcludeBackground(t)
	keys := []string{"hi_fg", "title", "cpu_start", "cpu_end", "mem_end", "net_download", "proc", "gpu_start"}
	var seeds []labColor
	for _, k := range keys {
		h := t.Hex(k, "")
		if h == "" {
			continue
		}
		c, err := colorful.Hex(h)
		if err != nil {
			continue
		}
		if tooCloseToBackground(c, h, bg, bgHex, hasBg) {
			continue
		}
		_, s, l := c.Hsl()
		if l < 0.07 || l > 0.94 || s < 0.11 {
			continue
		}
		seeds = append(seeds, labColor{hex: strings.ToLower(h), c: c})
	}
	if len(seeds) == 0 {
		c, err := colorful.Hex("#89B4FA")
		if err == nil {
			seeds = append(seeds, labColor{hex: "#89b4fa", c: c})
		}
	}
	if len(seeds) == 0 {
		out := make([]lipgloss.Color, n)
		for i := range out {
			out[i] = lipgloss.Color("#89b4fa")
		}
		return out
	}
	return extendPaletteFromSeeds(seeds, n, bg, bgHex, hasBg)
}
