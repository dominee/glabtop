package tui

import (
	"testing"
	"time"

	"glabtop/internal/model"
)

func TestTruncateStr(t *testing.T) {
	if got := truncateStr("abc", 10); got != "abc" {
		t.Fatal(got)
	}
	if got := truncateStr("abcdef", 4); got != "abc…" {
		t.Fatal(got)
	}
}

func TestCommitItem_strings(t *testing.T) {
	it := commitItem{c: model.CommitRow{
		ShortID:     "ab12cd",
		Title:       "fix things",
		AuthorName:  "Ada",
		ProjectPath: "g/p",
		CreatedAt:   time.Date(2026, 2, 1, 15, 4, 5, 0, time.UTC),
	}}
	if it.FilterValue() != "fix thingsAdag/p" {
		t.Fatal(it.FilterValue())
	}
	if it.Title() == "" {
		t.Fatal()
	}
	if it.Description() == "" {
		t.Fatal()
	}
}

func TestIssueItem_strings(t *testing.T) {
	it := issueItem{iss: model.IssueRow{
		IID:         7,
		Title:       "bug",
		AuthorName:  "a",
		AssigneeName: "b",
		ProjectPath: "g/p",
		ClosedAt:    time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC),
	}}
	if it.Title() == "" || it.Description() == "" {
		t.Fatal(it.Title(), it.Description())
	}
}

func TestCommitItems_nilSnapshot(t *testing.T) {
	if commitItems(nil, nil) != nil || issueItems(nil, nil) != nil {
		t.Fatal()
	}
}
