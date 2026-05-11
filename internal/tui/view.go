package tui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"glabtop/internal/model"
)

func renderView(m *Model) string {
	if m.w == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString(renderStatusBar(m))
	b.WriteString("\n")

	bodyW := m.w
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

	b.WriteString(renderHelpLine(m))
	if m.filterFocus {
		b.WriteString("\n")
		b.WriteString(m.paletteBold(3).Render("filter: "))
		b.WriteString(m.fi.View())
	}
	// Do not apply Width()+Render() to the whole view — it reflows ANSI-rich
	// blocks and breaks charts/lists (spurious blank rows, split borders).
	return b.String()
}

func renderHelpLine(m *Model) string {
	st := m.styles
	dim := st.Inactive
	join := dim.Render(" ")
	chunks := []string{
		m.paletteBold(0).Render("[t]") + dim.Render(" range"),
		m.paletteBold(1).Render("[r]") + dim.Render(" refresh"),
		m.paletteBold(2).Render("[p]") + dim.Render(" pause"),
		m.paletteBold(3).Render("[u]") + dim.Render(" user chart"),
		m.paletteBold(4).Render("[1-3]") + dim.Render(" panes"),
		m.paletteBold(5).Render("[d]") + dim.Render(" detail"),
		m.paletteBold(6).Render("[Tab]") + dim.Render(" focus"),
		m.paletteBold(7).Render("[/]") + dim.Render(" filter"),
		m.paletteBold(5).Render("[q]") + dim.Render(" quit"),
	}
	return strings.Join(chunks, join)
}

func renderStatusBar(m *Model) string {
	st := m.styles
	fp := currentFocus(m)
	focusStr := lipgloss.NewStyle().Bold(true).Foreground(m.paletteAt(4)).Render("▶"+focusPaneLabel(fp)+" ")

	stale := ""
	if m.snapshot != nil && m.snapshot.Stale {
		stale = lipgloss.NewStyle().Foreground(m.paletteAt(5)).Render("cached") + st.Inactive.Render(" · ")
	}

	status := "idle"
	if m.loading {
		status = "loading"
	}
	if m.paused {
		status += "·pause"
	}
	if m.offline {
		status += "·off"
	}
	statusStr := lipgloss.NewStyle().Foreground(m.paletteAt(6)).Render(status)

	nr := ""
	if !m.nextRefresh.IsZero() && !m.paused && !m.offline {
		d := time.Until(m.nextRefresh)
		if d < 0 {
			d = 0
		}
		nr = st.Inactive.Render(" · ") + lipgloss.NewStyle().Foreground(m.paletteAt(7)).Render(fmt.Sprintf("r:%ds", int(d.Seconds())))
	}

	dot := st.Inactive.Render(" · ")
	repoLbl := st.Inactive.Render("repos:")
	repoVal := lipgloss.NewStyle().Foreground(m.paletteAt(1))
	repoPart := dot + repoLbl + repoVal.Render(fmt.Sprintf("%d", len(m.projects)))
	if m.resolveErr != nil {
		repoPart = dot + lipgloss.NewStyle().Foreground(m.paletteAt(3)).Render("repos:err")
	}

	var periodPart string
	if m.snapshot != nil {
		co := m.snapshot.Counts
		usersVal := lipgloss.NewStyle().Foreground(m.paletteAt(3)).Render(fmt.Sprintf("%d", co.DistinctUsers))
		cV := lipgloss.NewStyle().Foreground(typeChartSegmentColor(m, 0)).Render(strconv.Itoa(co.Commits))
		mV := lipgloss.NewStyle().Foreground(typeChartSegmentColor(m, 1)).Render(strconv.Itoa(co.Merges))
		iV := lipgloss.NewStyle().Foreground(typeChartSegmentColor(m, 2)).Render(strconv.Itoa(co.ClosedIssues))
		clV := lipgloss.NewStyle().Foreground(typeChartSegmentColor(m, 2)).Render(strconv.Itoa(co.ClosedIssues))
		opV := lipgloss.NewStyle().Foreground(m.paletteAt(2)).Render(strconv.Itoa(co.OpenIssues))
		periodPart = dot + st.Inactive.Render("users:") + usersVal +
			dot + st.Inactive.Render("c/m/i:") + cV + st.Inactive.Render("/") + mV + st.Inactive.Render("/") + iV +
			dot + st.Inactive.Render("closed:") + clV +
			dot + st.Inactive.Render("open:") + opV
	}

	title := m.paletteBold(0).Render("glabtop")
	rng := lipgloss.NewStyle().Bold(true).Foreground(m.paletteAt(2)).Render(m.tr.String())

	leftCore := lipgloss.JoinHorizontal(lipgloss.Left,
		title,
		st.Inactive.Render(" · "),
		stale,
		rng,
		dot,
		focusStr,
		st.Inactive.Render(" · "),
		statusStr,
		nr,
		repoPart,
		periodPart,
	)

	errMsg := ""
	if m.lastErr != nil {
		errMsg = m.lastErr.Error()
	}

	avail := m.w - lipgloss.Width(leftCore) - 1
	if errMsg != "" && avail > 8 {
		e := truncateStr(errMsg, avail-4)
		errRendered := st.Hi.Render(" err:") + st.Inactive.Render(e)
		line := lipgloss.JoinHorizontal(lipgloss.Left, leftCore, errRendered)
		if lipgloss.Width(line) > m.w {
			e2 := truncateStr(errMsg, max(4, m.w-lipgloss.Width(leftCore)-8))
			errRendered = st.Hi.Render(" ") + st.Inactive.Render(e2)
			line = lipgloss.JoinHorizontal(lipgloss.Left, leftCore, errRendered)
		}
		return line
	}

	line := leftCore
	if lipgloss.Width(line) > m.w {
		return leftCore
	}
	return line
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func paneBorder(m *Model, focused bool) lipgloss.Style {
	st := m.styles
	fgHi := m.paletteAt(3)
	fgDim := lipgloss.Color(m.theme.Hex("div_line", "#6C7086"))
	s := st.BorderStyle.Border(st.Border)
	if focused {
		return s.BorderForeground(fgHi)
	}
	return s.BorderForeground(fgDim)
}

// borderContentWidth is the lipgloss Width for a bordered block so that the
// final string width (including left/right border runes) equals outer.
func borderContentWidth(outer int) int {
	if outer < 2 {
		return 1
	}
	return outer - 2
}

func statsBlock(m *Model, w int) string {
	st := m.styles
	box := paneBorder(m, activityPaneFocused(m)).Width(borderContentWidth(w)).Padding(0, 0)
	if m.snapshot == nil {
		h := m.paletteBold(1).Render(" Activity ")
		return box.Render(h + "\n" + st.Inactive.Render("no data"))
	}
	if len(m.snapshot.Series) == 0 {
		co := m.snapshot.Counts
		cN := lipgloss.NewStyle().Foreground(typeChartSegmentColor(m, 0)).Render(strconv.Itoa(co.Commits))
		mN := lipgloss.NewStyle().Foreground(typeChartSegmentColor(m, 1)).Render(strconv.Itoa(co.Merges))
		iN := lipgloss.NewStyle().Foreground(typeChartSegmentColor(m, 2)).Render(strconv.Itoa(co.ClosedIssues))
		line := st.Inactive.Render("totals: ") + cN + st.Inactive.Render(" commits · ") +
			mN + st.Inactive.Render(" merges · ") + iN + st.Inactive.Render(" issues — ") +
			st.Inactive.Render("use ") + m.paletteBold(4).Render("[r]") + st.Inactive.Render(" to refresh or wait for sync")
		h := m.paletteBold(0).Render(" Activity ")
		return box.Render(h + "\n" + line)
	}
	title := m.paletteBold(0).Render(" Activity ") + lipgloss.NewStyle().Foreground(m.paletteAt(5)).Render("— time →  ")
	canUC := m.snapshot.UserChart != nil && len(m.snapshot.UserChart.Users) > 0
	useUser := m.chartByUser && canUC
	note := ""
	if m.chartByUser && !canUC {
		note = st.Inactive.Render("by-user breakdown unavailable — showing by type") + "\n"
	}
	if useUser {
		title = m.paletteBold(0).Render(" Activity (by user) ") + lipgloss.NewStyle().Foreground(m.paletteAt(5)).Render("— time →  ")
	}
	innerChartW := borderContentWidth(w)
	if innerChartW < 4 {
		innerChartW = 4
	}
	chart := stackedTimeSeriesChart(m, innerChartW, 8)
	if useUser {
		chart = stackedUserChart(m, innerChartW, 8)
	}
	legend := typeChartLegend(m)
	if useUser {
		legend = userChartLegend(m)
	}
	return box.Render(title + "\n" + note + chart + "\n" + legend)
}

func typeChartLegend(m *Model) string {
	st := m.styles
	c := lipgloss.NewStyle().Foreground(typeChartSegmentColor(m, 0)).Render("██")
	cm := lipgloss.NewStyle().Foreground(typeChartSegmentColor(m, 1)).Render("██")
	ci := lipgloss.NewStyle().Foreground(typeChartSegmentColor(m, 2)).Render("██")
	return st.Inactive.Render("stack ") + c + lipgloss.NewStyle().Foreground(typeChartSegmentColor(m, 0)).Render(" c ") +
		cm + lipgloss.NewStyle().Foreground(typeChartSegmentColor(m, 1)).Render(" m ") + ci + lipgloss.NewStyle().Foreground(typeChartSegmentColor(m, 2)).Render(" i")
}

func columnWidths(n, total int) []int {
	if n <= 0 || total < 0 {
		return nil
	}
	if total == 0 {
		w := make([]int, n)
		return w
	}
	w := make([]int, n)
	base := total / n
	rem := total % n
	for i := 0; i < n; i++ {
		w[i] = base
		if i < rem {
			w[i]++
		}
	}
	return w
}

// chartUserColor uses theme-derived, Lab-spaced palette entries.
func chartUserColor(m *Model, i int) lipgloss.Color {
	p := m.theme.ChartPalette(8)
	if len(p) == 0 {
		return lipgloss.Color(m.theme.Hex("hi_fg", "#89B4FA"))
	}
	return p[i%len(p)]
}

// typeChartSegmentColor maps commits / merges / issues to spread indices in the theme palette.
func typeChartSegmentColor(m *Model, seg int) lipgloss.Color {
	p := m.theme.ChartPalette(8)
	if len(p) == 0 {
		return lipgloss.Color(m.theme.Hex("hi_fg", "#89B4FA"))
	}
	ix := []int{0, 3, 6}
	if seg >= 0 && seg < len(ix) {
		return p[ix[seg]%len(p)]
	}
	return p[seg%len(p)]
}

func stackedTimeSeriesChart(m *Model, width, height int) string {
	st := m.styles
	v := m.snapshot.Series
	if len(v) == 0 || width == 0 {
		return ""
	}
	maxTotal := 1
	for _, b := range v {
		t := b.Commits + b.Merges + b.ClosedIssues
		if t > maxTotal {
			maxTotal = t
		}
	}
	axisBodyW := len(strconv.Itoa(maxTotal))
	if axisBodyW < 1 {
		axisBodyW = 1
	}
	axisW := axisBodyW + 1
	plotW := width - axisW
	if plotW < 1 {
		plotW = 1
	}
	colWidths := columnWidths(len(v), plotW)
	commitStyle := lipgloss.NewStyle().Foreground(typeChartSegmentColor(m, 0))
	mergeStyle := lipgloss.NewStyle().Foreground(typeChartSegmentColor(m, 1))
	issueStyle := lipgloss.NewStyle().Foreground(typeChartSegmentColor(m, 2))
	rulerCh := lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.Hex("div_line", "#6C7086"))).Render("─")

	var rows []string
	for row := 0; row < height; row++ {
		var line strings.Builder
		line.WriteString(yAxisPrefix(m, row, height, maxTotal, axisBodyW, axisW, st.Inactive))
		fromBottom := height - 1 - row
		for j, b := range v {
			colW := colWidths[j]
			if colW == 0 {
				continue
			}
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
					line.WriteString(rulerCh)
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
	axis.WriteString(strings.Repeat(" ", axisW))
	for j, b := range v {
		cw := colWidths[j]
		if cw == 0 {
			continue
		}
		lbl := bucketAxisLabel(m.tr, b.StartRFC3339, cw)
		axis.WriteString(lipgloss.NewStyle().Foreground(m.paletteAt(6+j%8)).Render(lbl))
	}
	rows = append(rows, axis.String())

	return strings.Join(rows, "\n")
}

func stackedUserChart(m *Model, width, height int) string {
	st := m.styles
	uc := m.snapshot.UserChart
	if uc == nil || len(uc.Buckets) == 0 || width == 0 {
		return ""
	}
	v := uc.Buckets
	maxTotal := uc.MaxTotal
	if maxTotal < 1 {
		maxTotal = 1
	}
	axisBodyW := len(strconv.Itoa(maxTotal))
	if axisBodyW < 1 {
		axisBodyW = 1
	}
	axisW := axisBodyW + 1
	plotW := width - axisW
	if plotW < 1 {
		plotW = 1
	}
	colWidths := columnWidths(len(v), plotW)
	rulerCh := lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.Hex("div_line", "#6C7086"))).Render("─")

	var rows []string
	for row := 0; row < height; row++ {
		var line strings.Builder
		line.WriteString(yAxisPrefix(m, row, height, maxTotal, axisBodyW, axisW, st.Inactive))
		fromBottom := height - 1 - row
		for j, b := range v {
			colW := colWidths[j]
			if colW == 0 {
				continue
			}
			tot := 0
			for _, c := range b.Counts {
				tot += c
			}
			colH := 0
			if maxTotal > 0 {
				colH = tot * height / maxTotal
			}
			if tot > 0 && colH == 0 {
				colH = 1
			}
			heights := splitUserStack(colH, tot, b.Counts)
			for k := 0; k < colW; k++ {
				if fromBottom >= colH {
					line.WriteString(rulerCh)
					continue
				}
				si := userSegmentAt(fromBottom, heights)
				if si < 0 {
					line.WriteString(rulerCh)
					continue
				}
				col := lipgloss.NewStyle().Foreground(chartUserColor(m, si)).Render("█")
				line.WriteString(col)
			}
		}
		rows = append(rows, line.String())
	}

	var axis strings.Builder
	axis.WriteString(strings.Repeat(" ", axisW))
	for j, b := range v {
		cw := colWidths[j]
		if cw == 0 {
			continue
		}
		lbl := bucketAxisLabel(m.tr, b.StartRFC3339, cw)
		axis.WriteString(lipgloss.NewStyle().Foreground(m.paletteAt(6+j%8)).Render(lbl))
	}
	rows = append(rows, axis.String())

	return strings.Join(rows, "\n")
}

func splitUserStack(colH, tot int, counts []int) []int {
	n := len(counts)
	out := make([]int, n)
	if n == 0 || tot <= 0 || colH <= 0 {
		return out
	}
	rem := colH
	for i := 0; i < n; i++ {
		h := counts[i] * colH / tot
		out[i] = h
		rem -= h
	}
	for i := 0; i < n && rem > 0; i++ {
		if counts[i] > 0 {
			out[i]++
			rem--
		}
	}
	return out
}

func userSegmentAt(fromBottom int, heights []int) int {
	acc := 0
	for i, h := range heights {
		if h <= 0 {
			continue
		}
		next := acc + h
		if fromBottom >= acc && fromBottom < next {
			return i
		}
		acc = next
	}
	return -1
}

func userChartLegend(m *Model) string {
	st := m.styles
	uc := m.snapshot.UserChart
	if uc == nil || len(uc.Users) == 0 {
		return st.Inactive.Render("users")
	}
	sep := st.Inactive.Render(" · ")
	parts := make([]string, 0, len(uc.Users))
	for i, u := range uc.Users {
		runes := []rune(u)
		name := u
		if len(runes) > 12 {
			name = string(runes[:11]) + "…"
		}
		sq := lipgloss.NewStyle().Foreground(chartUserColor(m, i)).Render("██")
		parts = append(parts, sq+lipgloss.NewStyle().Foreground(m.paletteAt((i+3)%8)).Render(" "+name))
	}
	return strings.Join(parts, sep)
}

// yAxisPrefix returns a fillWidth left gutter for one chart row (scale labels).
func yAxisPrefix(m *Model, row, height, maxTotal, axisBodyW, axisW int, inactive lipgloss.Style) string {
	numStyle := lipgloss.NewStyle().Foreground(m.paletteAt(5))
	midRow := height / 2
	var val string
	switch {
	case row == 0:
		val = strconv.Itoa(maxTotal)
	case row == height-1:
		val = "0"
	case height > 5 && row == midRow && maxTotal > 1:
		mid := maxTotal / 2
		if mid <= 0 || mid >= maxTotal {
			return inactive.Render(strings.Repeat(" ", axisW))
		}
		val = strconv.Itoa(mid)
	default:
		return inactive.Render(strings.Repeat(" ", axisW))
	}
	padded := fmt.Sprintf("%*s ", axisBodyW, val)
	return numStyle.Render(strings.TrimRight(padded, " ")) + inactive.Render(" ")
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

func bucketAxisLabel(tr model.TimeRange, startRFC string, maxChars int) string {
	t, err := time.Parse(time.RFC3339, startRFC)
	if err != nil {
		return padTrim("?", maxChars)
	}
	var s string
	switch tr {
	case model.Range1H:
		s = t.Format("15:04")
	case model.Range1D:
		s = t.Format("15")
	case model.Range1W, model.Range1M:
		if maxChars <= 2 {
			s = fmt.Sprintf("%d", t.Day())
		} else {
			s = fmt.Sprintf("%d/%s", t.Day(), t.Format("Jan"))
		}
	case model.Range1Y:
		s = t.Format("Jan")
	default:
		s = t.Format("02")
	}
	return padTrim(s, maxChars)
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
	innerH := m.listViewportH
	if innerH < 1 {
		innerH = 6
	}
	if m.detailPane {
		const gapW = 1
		if m.showCommits && m.showIssues {
			leftOut, rightOut := detailSplitOuter(m)
			listOuter := leftOut
			detailOuter := rightOut
			detailInner := detailOuter - 2
			if detailInner < 1 {
				detailInner = 1
			}
			if m.listCommitActive {
				left := paneBorder(m, commitsPaneFocused(m)).Width(borderContentWidth(listOuter)).Render(m.commitList.View())
				right := detailRightBorder(m).Width(borderContentWidth(detailOuter)).Render(renderCommitDetail(m, detailInner, innerH))
				return lipgloss.JoinHorizontal(lipgloss.Top, left, strings.Repeat(" ", gapW), right)
			}
			left := paneBorder(m, issuesPaneFocused(m)).Width(borderContentWidth(listOuter)).Render(m.issueList.View())
			right := detailRightBorder(m).Width(borderContentWidth(detailOuter)).Render(renderIssueDetail(m, detailInner, innerH))
			return lipgloss.JoinHorizontal(lipgloss.Top, left, strings.Repeat(" ", gapW), right)
		}
		if m.showCommits {
			leftOut, rightOut := detailSplitOuter(m)
			detailInner := rightOut - 2
			if detailInner < 1 {
				detailInner = 1
			}
			left := paneBorder(m, commitsPaneFocused(m)).Width(borderContentWidth(leftOut)).Render(m.commitList.View())
			right := detailRightBorder(m).Width(borderContentWidth(rightOut)).Render(renderCommitDetail(m, detailInner, innerH))
			return lipgloss.JoinHorizontal(lipgloss.Top, left, " ", right)
		}
		if m.showIssues {
			leftOut, rightOut := detailSplitOuter(m)
			detailInner := rightOut - 2
			if detailInner < 1 {
				detailInner = 1
			}
			left := paneBorder(m, issuesPaneFocused(m)).Width(borderContentWidth(leftOut)).Render(m.issueList.View())
			right := detailRightBorder(m).Width(borderContentWidth(rightOut)).Render(renderIssueDetail(m, detailInner, innerH))
			return lipgloss.JoinHorizontal(lipgloss.Top, left, " ", right)
		}
	}
	if m.showCommits && m.showIssues {
		const gapW = 1
		leftOuter := (m.w - gapW) / 2
		rightOuter := m.w - gapW - leftOuter
		left := paneBorder(m, commitsPaneFocused(m)).Width(borderContentWidth(leftOuter)).Render(m.commitList.View())
		right := paneBorder(m, issuesPaneFocused(m)).Width(borderContentWidth(rightOuter)).Render(m.issueList.View())
		return lipgloss.JoinHorizontal(lipgloss.Top, left, " ", right)
	}
	if m.showCommits {
		fr := paneBorder(m, commitsPaneFocused(m)).Width(borderContentWidth(w))
		return fr.Render(m.commitList.View())
	}
	if m.showIssues {
		fr := paneBorder(m, issuesPaneFocused(m)).Width(borderContentWidth(w))
		return fr.Render(m.issueList.View())
	}
	return ""
}
