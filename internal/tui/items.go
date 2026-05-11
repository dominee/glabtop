package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/lipgloss"

	"glabtop/internal/model"
	th "glabtop/internal/theme"
)

type commitItem struct {
	c   model.CommitRow
	pal []lipgloss.Color
}

func (it commitItem) FilterValue() string {
	return it.c.Title + it.c.AuthorName + it.c.ProjectPath
}

func (it commitItem) Title() string {
	title := truncateStr(it.c.Title, 60)
	if len(it.pal) == 0 {
		return it.c.ShortID + " · " + title
	}
	k := accentIndex(it.c.ShortID, len(it.pal))
	id := lipgloss.NewStyle().Foreground(it.pal[k]).Render(it.c.ShortID)
	return id + " · " + title
}

func (it commitItem) Description() string {
	return it.c.ProjectPath + " · " + it.c.AuthorName + " · " + it.c.CreatedAt.UTC().Format("2006-01-02 15:04")
}

type issueItem struct {
	iss model.IssueRow
	pal []lipgloss.Color
}

func (it issueItem) FilterValue() string {
	return it.iss.Title + it.iss.AuthorName + it.iss.ProjectPath
}

func (it issueItem) Title() string {
	ti := truncateStr(it.iss.Title, 50)
	if len(it.pal) == 0 {
		return fmt.Sprintf("#%d · %s", it.iss.IID, ti)
	}
	k := accentIndex(fmt.Sprintf("%d", it.iss.IID), len(it.pal))
	head := lipgloss.NewStyle().Foreground(it.pal[k]).Render(fmt.Sprintf("#%d", it.iss.IID))
	return head + " · " + ti
}

func (it issueItem) Description() string {
	a := it.iss.AuthorName
	if it.iss.AssigneeName != "" {
		a += " → " + it.iss.AssigneeName
	}
	return it.iss.ProjectPath + " · " + a + " · " + it.iss.ClosedAt.UTC().Format("2006-01-02")
}

func chartPaletteColors(t *th.Theme) []lipgloss.Color {
	if t == nil {
		return nil
	}
	return t.ChartPalette(8)
}

func commitItems(snap *model.Snapshot, t *th.Theme) []list.Item {
	if snap == nil {
		return nil
	}
	pal := chartPaletteColors(t)
	it := make([]list.Item, 0, len(snap.Commits))
	for _, c := range snap.Commits {
		it = append(it, commitItem{c: c, pal: pal})
	}
	return it
}

func issueItems(snap *model.Snapshot, t *th.Theme) []list.Item {
	if snap == nil {
		return nil
	}
	pal := chartPaletteColors(t)
	it := make([]list.Item, 0, len(snap.Issues))
	for _, iss := range snap.Issues {
		it = append(it, issueItem{iss: iss, pal: pal})
	}
	return it
}

func truncateStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
