package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"

	"glabtop/internal/model"
)

func detailSplitOuter(m *Model) (leftOut, rightOut int) {
	const gapW = 1
	leftOut = (m.w - gapW) / 2
	rightOut = m.w - gapW - leftOut
	return leftOut, rightOut
}

func selectedCommitRow(m *Model) (model.CommitRow, bool) {
	it := m.commitList.SelectedItem()
	if it == nil {
		return model.CommitRow{}, false
	}
	ci, ok := it.(commitItem)
	if !ok {
		return model.CommitRow{}, false
	}
	return ci.c, true
}

func selectedIssueRow(m *Model) (model.IssueRow, bool) {
	it := m.issueList.SelectedItem()
	if it == nil {
		return model.IssueRow{}, false
	}
	ii, ok := it.(issueItem)
	if !ok {
		return model.IssueRow{}, false
	}
	return ii.i, true
}

func commitDetailString(c model.CommitRow) string {
	return fmt.Sprintf(
		"SHA\n%s\n\nProject\n%s\n\nAuthor\n%s\n%s\n\nDate (UTC)\n%s\n\nMessage\n%s",
		c.ID,
		c.ProjectPath,
		c.AuthorName,
		c.AuthorEmail,
		c.CreatedAt.UTC().Format(time.RFC3339),
		c.Title,
	)
}

func issueDetailString(i model.IssueRow) string {
	people := i.AuthorName
	if strings.TrimSpace(i.AssigneeName) != "" {
		people += " → " + i.AssigneeName
	}
	return fmt.Sprintf(
		"Issue #%d\n\nProject\n%s\n\nPeople\n%s\n\nState\n%s\n\nClosed (UTC)\n%s\n\nUpdated (UTC)\n%s\n\nURL\n%s\n\nTitle\n%s",
		i.IID,
		i.ProjectPath,
		people,
		i.State,
		i.ClosedAt.UTC().Format(time.RFC3339),
		i.UpdatedAt.UTC().Format(time.RFC3339),
		i.WebURL,
		i.Title,
	)
}

func hardBreakRune(s string, width int) []string {
	if width < 1 {
		width = 1
	}
	rs := []rune(s)
	var lines []string
	i := 0
	for i < len(rs) {
		w := 0
		j := i
		for j < len(rs) {
			rw := runewidth.RuneWidth(rs[j])
			if w+rw > width {
				break
			}
			w += rw
			j++
		}
		if j == i {
			j = i + 1
		}
		lines = append(lines, string(rs[i:j]))
		i = j
	}
	return lines
}

func wrapToWidthPlain(s string, width int) []string {
	if width < 1 {
		width = 1
	}
	var out []string
	for _, para := range strings.Split(s, "\n") {
		if para == "" {
			out = append(out, "")
			continue
		}
		words := strings.Fields(para)
		if len(words) == 0 {
			out = append(out, "")
			continue
		}
		line := ""
		flush := func() {
			if line != "" {
				out = append(out, line)
				line = ""
			}
		}
		for _, word := range words {
			if runewidth.StringWidth(word) > width {
				flush()
				out = append(out, hardBreakRune(word, width)...)
				continue
			}
			cand := word
			if line != "" {
				cand = line + " " + word
			}
			if runewidth.StringWidth(cand) > width {
				flush()
				line = word
			} else {
				line = cand
			}
		}
		flush()
	}
	return out
}

func joinDetailLines(lines []string, h int) string {
	if h <= 0 {
		return ""
	}
	if len(lines) > h {
		lines = lines[:h]
	}
	for len(lines) < h {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

func renderCommitDetail(m *Model, innerW, innerH int) string {
	st := m.styles
	var lines []string
	c, ok := selectedCommitRow(m)
	if !ok {
		lines = append(lines, st.Title.Render(" Commit "))
		lines = append(lines, st.Inactive.Render("no selection"))
		return joinDetailLines(lines, innerH)
	}
	lines = append(lines, st.Title.Render(" Commit "))
	for _, ln := range wrapToWidthPlain(commitDetailString(c), innerW) {
		if ln == "" {
			lines = append(lines, "")
		} else {
			lines = append(lines, st.Base.Render(ln))
		}
	}
	return joinDetailLines(lines, innerH)
}

func renderIssueDetail(m *Model, innerW, innerH int) string {
	st := m.styles
	var lines []string
	i, ok := selectedIssueRow(m)
	if !ok {
		lines = append(lines, st.Title.Render(" Issue "))
		lines = append(lines, st.Inactive.Render("no selection"))
		return joinDetailLines(lines, innerH)
	}
	lines = append(lines, st.Title.Render(" Issue "))
	for _, ln := range wrapToWidthPlain(issueDetailString(i), innerW) {
		if ln == "" {
			lines = append(lines, "")
		} else {
			lines = append(lines, st.Base.Render(ln))
		}
	}
	return joinDetailLines(lines, innerH)
}

func detailRightBorder(m *Model) lipgloss.Style {
	st := m.styles
	s := st.BorderStyle.Border(st.Border)
	fg := lipgloss.Color(m.theme.Hex("div_line", "#6C7086"))
	return s.BorderForeground(fg)
}
