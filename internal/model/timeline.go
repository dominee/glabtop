package model

import (
	"sort"
	"time"
)

// TimeBucket is one bar on the time-axis chart (stacked segments in the UI).
type TimeBucket struct {
	StartRFC3339 string `json:"start"`
	Commits      int    `json:"commits"`
	Merges       int    `json:"merges"`
	ClosedIssues int    `json:"closed_issues"`
}

// GranularityKey labels how bucket_unix is aligned (stored in SQLite).
func GranularityKey(tr TimeRange) string {
	switch tr {
	case Range1H:
		return "5m"
	case Range1D:
		return "1h"
	case Range1W, Range1M:
		return "1d"
	case Range1Y:
		return "1mo"
	default:
		return "1d"
	}
}

// BucketStart aligns t to the start of its bucket for the given range preset.
func BucketStart(t time.Time, tr TimeRange) time.Time {
	u := t.UTC()
	switch tr {
	case Range1H:
		return u.Truncate(5 * time.Minute)
	case Range1D:
		return u.Truncate(time.Hour)
	case Range1W, Range1M:
		y, m, d := u.Date()
		return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
	case Range1Y:
		return time.Date(u.Year(), u.Month(), 1, 0, 0, 0, 0, time.UTC)
	default:
		return u.Truncate(5 * time.Minute)
	}
}

// NextBucket advances one step on the time axis.
func NextBucket(t time.Time, tr TimeRange) time.Time {
	switch tr {
	case Range1H:
		return t.UTC().Add(5 * time.Minute)
	case Range1D:
		return t.UTC().Add(time.Hour)
	case Range1W, Range1M:
		return t.UTC().Add(24 * time.Hour)
	case Range1Y:
		return t.UTC().AddDate(0, 1, 0)
	default:
		return t.UTC().Add(5 * time.Minute)
	}
}

// EnumerateBucketStarts returns ordered bucket starts covering [since, until).
func EnumerateBucketStarts(since, until time.Time, tr TimeRange) []time.Time {
	var out []time.Time
	t := BucketStart(since, tr)
	for t.Before(since) {
		t = NextBucket(t, tr)
	}
	for t.Before(until) {
		out = append(out, t)
		t = NextBucket(t, tr)
	}
	return out
}

// SeriesFromMaps builds a sorted slice from aggregate maps (when keys may be sparse).
func SeriesFromMaps(since, until time.Time, tr TimeRange, commits, merges, issues map[int64]int) []TimeBucket {
	starts := EnumerateBucketStarts(since, until, tr)
	out := make([]TimeBucket, len(starts))
	for i, st := range starts {
		k := st.Unix()
		out[i] = TimeBucket{
			StartRFC3339: st.Format(time.RFC3339),
			Commits:      commits[k],
			Merges:       merges[k],
			ClosedIssues: issues[k],
		}
	}
	return out
}

// NormalizeSeries ensures snapshot totals match summed series (for header counts).
func NormalizeSeries(series []TimeBucket) Counts {
	var c Counts
	for _, b := range series {
		c.Commits += b.Commits
		c.Merges += b.Merges
		c.ClosedIssues += b.ClosedIssues
	}
	return c
}

// SortCommitsByTimeDesc sorts commits for list display.
func SortCommitsByTimeDesc(cc []CommitRow) {
	sort.Slice(cc, func(i, j int) bool { return cc[i].CreatedAt.After(cc[j].CreatedAt) })
}

// SortIssuesByTimeDesc sorts issues by closed time.
func SortIssuesByTimeDesc(ii []IssueRow) {
	sort.Slice(ii, func(i, j int) bool { return ii[i].ClosedAt.After(ii[j].ClosedAt) })
}
