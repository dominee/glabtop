package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"

	"glabtop/internal/model"
	th "glabtop/internal/theme"
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

// detailSectionHeader renders a bold section label (theme title style).
func detailSectionHeader(st th.Styles, label string) string {
	return st.Title.Render(label)
}

func appendDetailSpacer(lines *[]string) {
	*lines = append(*lines, "")
}

func appendDetailSection(lines *[]string, st th.Styles, header, body string, innerW int) {
	appendDetailSpacer(lines)
	*lines = append(*lines, detailSectionHeader(st, header))
	s := strings.TrimSpace(body)
	if s == "" {
		*lines = append(*lines, st.Inactive.Render("—"))
		return
	}
	for _, ln := range wrapToWidthPlain(s, innerW) {
		if ln == "" {
			*lines = append(*lines, "")
		} else {
			*lines = append(*lines, st.Base.Render(ln))
		}
	}
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
		lines = append(lines, detailSectionHeader(st, "Commit"))
		appendDetailSpacer(&lines)
		lines = append(lines, st.Inactive.Render("no selection"))
		return joinDetailLines(lines, innerH)
	}

	lines = append(lines, detailSectionHeader(st, "Commit"))
	appendDetailSection(&lines, st, "SHA", c.ShortID+" · "+c.ID, innerW)
	appendDetailSection(&lines, st, "Project", c.ProjectPath, innerW)
	auth := strings.TrimSpace(c.AuthorName)
	if em := strings.TrimSpace(c.AuthorEmail); em != "" {
		if auth != "" {
			auth = auth + "\n" + em
		} else {
			auth = em
		}
	}
	appendDetailSection(&lines, st, "Author", auth, innerW)
	appendDetailSection(&lines, st, "Date (UTC)", c.CreatedAt.UTC().Format(time.RFC3339), innerW)
	appendDetailSection(&lines, st, "Message", c.Title, innerW)

	return joinDetailLines(lines, innerH)
}

func renderIssueDetail(m *Model, innerW, innerH int) string {
	st := m.styles
	var lines []string
	i, ok := selectedIssueRow(m)
	if !ok {
		lines = append(lines, detailSectionHeader(st, "Issue"))
		appendDetailSpacer(&lines)
		lines = append(lines, st.Inactive.Render("no selection"))
		return joinDetailLines(lines, innerH)
	}

	lines = append(lines, detailSectionHeader(st, "Issue"))
	appendDetailSection(&lines, st, "Reference", fmt.Sprintf("#%d", i.IID), innerW)
	appendDetailSection(&lines, st, "Project", i.ProjectPath, innerW)
	people := strings.TrimSpace(i.AuthorName)
	if a := strings.TrimSpace(i.AssigneeName); a != "" {
		if people != "" {
			people = fmt.Sprintf("%s\n→ %s", people, a)
		} else {
			people = "→ " + a
		}
	}
	appendDetailSection(&lines, st, "People", people, innerW)
	appendDetailSection(&lines, st, "State", i.State, innerW)
	appendDetailSection(&lines, st, "Closed (UTC)", i.ClosedAt.UTC().Format(time.RFC3339), innerW)
	appendDetailSection(&lines, st, "Updated (UTC)", i.UpdatedAt.UTC().Format(time.RFC3339), innerW)
	appendDetailSection(&lines, st, "Link", i.WebURL, innerW)
	appendDetailSection(&lines, st, "Title", i.Title, innerW)

	return joinDetailLines(lines, innerH)
}

func detailRightBorder(m *Model) lipgloss.Style {
	st := m.styles
	s := st.BorderStyle.Border(st.Border)
	fg := lipgloss.Color(m.theme.Hex("div_line", "#6C7086"))
	return s.BorderForeground(fg)
}
