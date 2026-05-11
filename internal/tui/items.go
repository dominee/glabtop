package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/list"

	"glabtop/internal/model"
)

type commitItem struct{ c model.CommitRow }

func (i commitItem) FilterValue() string {
	return i.c.Title + i.c.AuthorName + i.c.ProjectPath
}
func (i commitItem) Title() string {
	return i.c.ShortID + " · " + truncateStr(i.c.Title, 60)
}
func (i commitItem) Description() string {
	return i.c.ProjectPath + " · " + i.c.AuthorName + " · " + i.c.CreatedAt.UTC().Format("2006-01-02 15:04")
}

type issueItem struct{ i model.IssueRow }

func (i issueItem) FilterValue() string {
	return i.i.Title + i.i.AuthorName + i.i.ProjectPath
}
func (i issueItem) Title() string {
	return fmt.Sprintf("#%d · %s", i.i.IID, truncateStr(i.i.Title, 50))
}
func (i issueItem) Description() string {
	a := i.i.AuthorName
	if i.i.AssigneeName != "" {
		a += " → " + i.i.AssigneeName
	}
	return i.i.ProjectPath + " · " + a + " · " + i.i.ClosedAt.UTC().Format("2006-01-02")
}

func commitItems(snap *model.Snapshot) []list.Item {
	if snap == nil {
		return nil
	}
	it := make([]list.Item, 0, len(snap.Commits))
	for _, c := range snap.Commits {
		it = append(it, commitItem{c})
	}
	return it
}

func issueItems(snap *model.Snapshot) []list.Item {
	if snap == nil {
		return nil
	}
	it := make([]list.Item, 0, len(snap.Issues))
	for _, iss := range snap.Issues {
		it = append(it, issueItem{iss})
	}
	return it
}

func truncateStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
