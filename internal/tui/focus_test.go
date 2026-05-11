package tui

import (
	"testing"

	th "glabtop/internal/theme"
)

func TestCurrentFocus(t *testing.T) {
	emptyTheme := &th.Theme{Colors: map[string]string{"main_bg": "#1e1e2e", "main_fg": "#cdd6f4"}}
	m := Model{
		theme:       emptyTheme,
		styles:      emptyTheme.Styles(),
		showStats:   true,
		showCommits: true,
		showIssues:  true,
	}
	if currentFocus(&m) != focusIssues {
		t.Fatalf("both lists visible, no detail: want issues (second), got %v", currentFocus(&m))
	}

	m.listCommitActive = true
	if currentFocus(&m) != focusCommits {
		t.Fatal(currentFocus(&m))
	}

	m.filterFocus = true
	if currentFocus(&m) != focusFilter {
		t.Fatal()
	}
}

func TestFocusPaneLabel(t *testing.T) {
	if focusPaneLabel(focusActivity) != "activity" {
		t.Fatal()
	}
	if focusPaneLabel(focusPane(-1)) != "—" {
		t.Fatal()
	}
}
