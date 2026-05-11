package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"glabtop/internal/model"
)

func renderView(m *Model) string {
	if m.w == 0 {
		return ""
	}
	st := m.styles
	base := st.Base.Width(m.w)

	var b strings.Builder
	title := st.Title.Render(" glabtop ")
	stale := ""
	if m.snapshot != nil && m.snapshot.Stale {
		stale = st.Hi.Render(" CACHED ")
	}
	errS := ""
	if m.lastErr != nil {
		errS = st.Hi.Render(" ERR ") + m.styles.Inactive.Render(truncateStr(m.lastErr.Error(), m.w-40))
	}
	line1 := lipgloss.JoinHorizontal(lipgloss.Left, title, stale, st.Inactive.Render(fmt.Sprintf(" range=%s ", m.tr.String())), errS)
	b.WriteString(line1)
	b.WriteString("\n")

	status := "● idle"
	if m.loading {
		status = "◌ loading"
	}
	if m.paused {
		status += " · paused"
	}
	if m.offline {
		status += " · offline"
	}
	nr := ""
	if !m.nextRefresh.IsZero() && !m.paused && !m.offline {
		d := time.Until(m.nextRefresh)
		if d < 0 {
			d = 0
		}
		nr = fmt.Sprintf("next refresh %ds | ", int(d.Seconds()))
	}
	proj := fmt.Sprintf("projects=%d", len(m.projects))
	if m.resolveErr != nil {
		proj = "resolve error"
	}
	line2 := st.Inactive.Render(fmt.Sprintf("%s%s%s", status, nr, proj))
	b.WriteString(line2)
	b.WriteString("\n")

	bodyW := m.w - 2
	var parts []string
	if m.showStats {
		parts = append(parts, statsBlock(m, bodyW))
	}

	lists := listsBlock(m, bodyW)
	if lists != "" {
		parts = append(parts, lists)
	}
	if len(parts) > 0 {
		b.WriteString(lipgloss.JoinVertical(lipgloss.Left, parts...))
		b.WriteString("\n")
	}

	help := st.Inactive.Render("[t] range [r] refresh [p] pause [1-3] panes [d] detail [Tab] focus [/] filter [q] quit")
	b.WriteString(help)
	if m.filterFocus {
		b.WriteString("\n")
		b.WriteString(st.Hi.Render("filter: "))
		b.WriteString(m.fi.View())
	}
	return base.Render(b.String())
}

func statsBlock(m *Model, w int) string {
	st := m.styles
	box := st.BorderStyle.Border(st.Border).Width(w).Padding(0, 1)
	if m.snapshot == nil {
		return box.Render(st.Title.Render("Activity") + "\n" + st.Inactive.Render("no data"))
	}
	if len(m.snapshot.Series) == 0 {
		line := fmt.Sprintf("totals: %d commits · %d merges · %d issues — %s",
			m.snapshot.Counts.Commits, m.snapshot.Counts.Merges, m.snapshot.Counts.ClosedIssues,
			"use [r] to refresh or wait for sync")
		return box.Render(st.Title.Render("Activity") + "\n" + st.Inactive.Render(line))
	}
	title := st.Title.Render("Activity — time →  (stacked █ commits / merges / issues)")
	innerW := w - 4
	if innerW < 8 {
		innerW = 8
	}
	chart := stackedTimeSeriesChart(m, innerW, 8)
	legend := chartLegend(m)
	return box.Render(title + "\n" + chart + "\n" + legend)
}

func chartLegend(m *Model) string {
	st := m.styles
	c := lipgloss.NewStyle().Foreground(st.MeterLow).Render("██")
	cm := lipgloss.NewStyle().Foreground(st.MeterMid).Render("██")
	ci := lipgloss.NewStyle().Foreground(st.MeterHigh).Render("██")
	return st.Inactive.Render("y: count  ") + c + st.Inactive.Render(" commits  ") +
		cm + st.Inactive.Render(" merges  ") + ci + st.Inactive.Render(" issues")
}

func stackedTimeSeriesChart(m *Model, width, height int) string {
	st := m.styles
	v := m.snapshot.Series
	if len(v) == 0 {
		return ""
	}
	maxTotal := 1
	for _, b := range v {
		t := b.Commits + b.Merges + b.ClosedIssues
		if t > maxTotal {
			maxTotal = t
		}
	}
	colW := width / len(v)
	if colW < 1 {
		colW = 1
	}
	if colW > 2 {
		colW = 2
	}

	commitStyle := lipgloss.NewStyle().Foreground(st.MeterLow)
	mergeStyle := lipgloss.NewStyle().Foreground(st.MeterMid)
	issueStyle := lipgloss.NewStyle().Foreground(st.MeterHigh)
	emptyCh := lipgloss.NewStyle().Foreground(lipgloss.Color("#6C7086")).Render("░")

	var rows []string
	for row := 0; row < height; row++ {
		var line strings.Builder
		fromBottom := height - 1 - row
		for _, b := range v {
			tot := b.Commits + b.Merges + b.ClosedIssues
			colH := 0
			if maxTotal > 0 {
				colH = tot * height / maxTotal
			}
			if tot > 0 && colH == 0 {
				colH = 1
			}
			hC, hM, _ := splitStack(colH, b.Commits, b.Merges, b.ClosedIssues, tot)
			for k := 0; k < colW; k++ {
				if fromBottom >= colH {
					line.WriteString(emptyCh)
					continue
				}
				seg := fromBottom
				switch {
				case seg < hC:
					line.WriteString(commitStyle.Render("█"))
				case seg < hC+hM:
					line.WriteString(mergeStyle.Render("█"))
				default:
					line.WriteString(issueStyle.Render("█"))
				}
			}
		}
		rows = append(rows, line.String())
	}

	var axis strings.Builder
	for _, b := range v {
		lbl := bucketAxisLabel(m.tr, b.StartRFC3339)
		axis.WriteString(padTrim(lbl, colW))
	}
	rows = append(rows, st.Inactive.Render(axis.String()))

	return strings.Join(rows, "\n")
}

func splitStack(colH, c, m, i, tot int) (hC, hM, hI int) {
	if tot <= 0 || colH <= 0 {
		return 0, 0, 0
	}
	hC = c * colH / tot
	hM = m * colH / tot
	hI = colH - hC - hM
	return hC, hM, hI
}

func bucketAxisLabel(tr model.TimeRange, startRFC string) string {
	t, err := time.Parse(time.RFC3339, startRFC)
	if err != nil {
		return "?"
	}
	switch tr {
	case model.Range1H:
		return t.Format("15:04")
	case model.Range1D:
		return t.Format("15")
	case model.Range1W, model.Range1M:
		return t.Format("02") + "/" + t.Format("Jan")
	case model.Range1Y:
		return t.Format("Jan")
	default:
		return t.Format("02")
	}
}

func padTrim(s string, w int) string {
	if w <= 0 {
		return ""
	}
	rr := []rune(s)
	if len(rr) > w {
		return string(rr[:w])
	}
	return s + strings.Repeat(" ", w-len(rr))
}

func listsBlock(m *Model, w int) string {
	st := m.styles
	if m.detailPane {
		if m.showCommits {
			fr := st.BorderStyle.Border(st.Border).Width(w)
			return fr.Render(m.commitList.View())
		}
		if m.showIssues {
			fr := st.BorderStyle.Border(st.Border).Width(w)
			return fr.Render(m.issueList.View())
		}
	}
	if m.showCommits && m.showIssues {
		hw := (w - 2) / 2
		left := st.BorderStyle.Border(st.Border).Width(hw).Render(m.commitList.View())
		right := st.BorderStyle.Border(st.Border).Width(hw).Render(m.issueList.View())
		return lipgloss.JoinHorizontal(lipgloss.Top, left, " ", right)
	}
	if m.showCommits {
		fr := st.BorderStyle.Border(st.Border).Width(w)
		return fr.Render(m.commitList.View())
	}
	if m.showIssues {
		fr := st.BorderStyle.Border(st.Border).Width(w)
		return fr.Render(m.issueList.View())
	}
	return ""
}
