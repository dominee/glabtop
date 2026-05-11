package tui

// focusPane identifies where keyboard input applies (for status + borders).
type focusPane int

const (
	focusFilter focusPane = iota
	focusActivity
	focusCommits
	focusIssues
)

func currentFocus(m *Model) focusPane {
	if m.filterFocus {
		return focusFilter
	}
	if m.detailPane {
		if m.showCommits && m.showIssues {
			if m.listCommitActive {
				return focusCommits
			}
			return focusIssues
		}
		if m.showCommits {
			return focusCommits
		}
		if m.showIssues {
			return focusIssues
		}
	}
	if m.showCommits && m.showIssues {
		if m.listCommitActive {
			return focusCommits
		}
		return focusIssues
	}
	if m.showCommits {
		return focusCommits
	}
	if m.showIssues {
		return focusIssues
	}
	if m.showStats {
		return focusActivity
	}
	return focusActivity
}

func focusPaneLabel(p focusPane) string {
	switch p {
	case focusFilter:
		return "filter"
	case focusActivity:
		return "activity"
	case focusCommits:
		return "commits"
	case focusIssues:
		return "issues"
	default:
		return "—"
	}
}

func activityPaneFocused(m *Model) bool {
	return currentFocus(m) == focusActivity
}

func commitsPaneFocused(m *Model) bool {
	return currentFocus(m) == focusCommits
}

func issuesPaneFocused(m *Model) bool {
	return currentFocus(m) == focusIssues
}
