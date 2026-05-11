package model

import (
	"strconv"
	"time"
)

type TimeRange int

const (
	Range1H TimeRange = iota
	Range1D
	Range1W
	Range1M
	Range1Y
)

func (r TimeRange) String() string {
	switch r {
	case Range1H:
		return "1h"
	case Range1D:
		return "1d"
	case Range1W:
		return "1w"
	case Range1M:
		return "1m"
	case Range1Y:
		return "1y"
	default:
		return "1h"
	}
}

func (r TimeRange) Next() TimeRange {
	switch r {
	case Range1H:
		return Range1D
	case Range1D:
		return Range1W
	case Range1W:
		return Range1M
	case Range1M:
		return Range1Y
	case Range1Y:
		return Range1H
	default:
		return Range1H
	}
}

func ParseTimeRange(s string) TimeRange {
	switch s {
	case "1h":
		return Range1H
	case "1d":
		return Range1D
	case "1w":
		return Range1W
	case "1m":
		return Range1M
	case "1y":
		return Range1Y
	default:
		return Range1H
	}
}

// WindowBounds returns [since, until) style bounds in UTC; until is now.
func WindowBounds(r TimeRange, now time.Time) (since, until time.Time) {
	until = now.UTC()
	switch r {
	case Range1H:
		since = until.Add(-1 * time.Hour)
	case Range1D:
		since = until.Add(-24 * time.Hour)
	case Range1W:
		since = until.Add(-7 * 24 * time.Hour)
	case Range1M:
		since = until.AddDate(0, -1, 0)
	case Range1Y:
		since = until.AddDate(-1, 0, 0)
	default:
		since = until.Add(-1 * time.Hour)
	}
	return since, until
}

// WindowID keys cached snapshots for the active wall-clock window.
func WindowID(now time.Time, r TimeRange) string {
	u := now.UTC()
	switch r {
	case Range1H:
		return "1h:" + time.Unix(u.Unix()-u.Unix()%3600, 0).Format(time.RFC3339)
	case Range1D:
		return "1d:" + u.Format("2006-01-02")
	case Range1W:
		year, week := u.ISOWeek()
		return "1w:" + strconv.Itoa(year) + "-W" + strconv.Itoa(week)
	case Range1M:
		return "1m:" + u.Format("2006-01")
	case Range1Y:
		return "1y:" + strconv.Itoa(u.Year())
	default:
		return "1h:" + u.Format(time.RFC3339)
	}
}

type ProjectRef struct {
	ID                int    `json:"id"`
	PathWithNamespace string `json:"path_with_namespace"`
}

type CommitRow struct {
	ID          string    `json:"id"`
	ShortID     string    `json:"short_id"`
	Title       string    `json:"title"`
	AuthorName  string    `json:"author_name"`
	AuthorEmail string    `json:"author_email"`
	CreatedAt   time.Time `json:"created_at"`
	ProjectPath string    `json:"-"`
}

type IssueRow struct {
	IID          int       `json:"iid"`
	Title        string    `json:"title"`
	State        string    `json:"state"`
	AuthorName   string    `json:"-"`
	AssigneeName string    `json:"-"`
	ClosedAt     time.Time `json:"closed_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	WebURL       string    `json:"web_url"`
	ProjectPath  string    `json:"-"`
}

type Counts struct {
	Commits      int
	Merges       int
	ClosedIssues int
}

type Snapshot struct {
	WindowID     string       `json:"window_id"`
	TimeRange    string       `json:"time_range"`
	SinceRFC3339 string       `json:"since"`
	UntilRFC3339 string       `json:"until"`
	Counts       Counts       `json:"counts"`
	Series       []TimeBucket `json:"series"`
	Commits      []CommitRow  `json:"commits"`
	Issues       []IssueRow   `json:"issues"`
	FetchedUnix  int64        `json:"fetched_unix"`
	Stale        bool         `json:"stale"`
}
