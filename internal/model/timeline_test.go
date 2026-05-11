package model

import (
	"testing"
	"time"
)

func TestParseTimeRange_String_Next(t *testing.T) {
	tests := []struct {
		in   string
		want TimeRange
	}{
		{"1h", Range1H},
		{"1d", Range1D},
		{"1w", Range1W},
		{"1m", Range1M},
		{"1y", Range1Y},
		{"bogus", Range1H},
	}
	for _, tt := range tests {
		if got := ParseTimeRange(tt.in); got != tt.want {
			t.Fatalf("%q: got %v want %v", tt.in, got, tt.want)
		}
	}
	if Range1H.String() != "1h" {
		t.Fatal(Range1H.String())
	}
	next := Range1H
	for i := 0; i < 5; i++ {
		next = next.Next()
	}
	if next != Range1H {
		t.Fatalf("cycle after 5 Next: got %v want %v", next, Range1H)
	}
}

func TestWindowBounds_orderAndSpan(t *testing.T) {
	now := time.Date(2026, 6, 15, 12, 30, 0, 0, time.UTC)
	since, until := WindowBounds(Range1H, now)
	if !since.Before(until) {
		t.Fatal("since >= until")
	}
	if d := until.Sub(since); d != time.Hour {
		t.Fatalf("1h span: %v", d)
	}
	sinceD, untilD := WindowBounds(Range1D, now)
	if untilD.Sub(sinceD) != 24*time.Hour {
		t.Fatal(untilD.Sub(sinceD))
	}
}

func TestWindowID_shape(t *testing.T) {
	now := time.Date(2026, 3, 2, 14, 0, 0, 0, time.UTC)
	id1h := WindowID(now, Range1H)
	if id1h == "" || len(id1h) < 4 {
		t.Fatal(id1h)
	}
	id1d := WindowID(now, Range1D)
	if want := "1d:2026-03-02"; id1d != want {
		t.Fatalf("got %q want %q", id1d, want)
	}
	id1m := WindowID(now, Range1M)
	if want := "1m:2026-03"; id1m != want {
		t.Fatalf("got %q want %q", id1m, want)
	}
	id1y := WindowID(now, Range1Y)
	if want := "1y:2026"; id1y != want {
		t.Fatalf("got %q want %q", id1y, want)
	}
}

func TestGranularityKey(t *testing.T) {
	if GranularityKey(Range1H) != "5m" {
		t.Fatal()
	}
	if GranularityKey(Range1D) != "1h" {
		t.Fatal()
	}
	if GranularityKey(Range1W) != "1d" {
		t.Fatal()
	}
	if GranularityKey(Range1Y) != "1mo" {
		t.Fatal()
	}
}

func TestBucketStart_NextBucket_enumeration(t *testing.T) {
	tr := Range1H
	t0 := time.Date(2026, 1, 1, 12, 7, 33, 0, time.UTC)
	b0 := BucketStart(t0, tr)
	if b0.Minute()%5 != 0 || b0.Second() != 0 {
		t.Fatalf("not aligned: %v", b0)
	}
	b1 := NextBucket(b0, tr)
	if b1.Sub(b0) != 5*time.Minute {
		t.Fatal(b1.Sub(b0))
	}
	since := b0
	until := b0.Add(12 * time.Minute)
	starts := EnumerateBucketStarts(since, until, tr)
	if len(starts) < 2 {
		t.Fatalf("starts %d", len(starts))
	}
}

func TestSeriesFromMaps_NormalizeSeries(t *testing.T) {
	since := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	until := since.Add(2 * time.Hour)
	tr := Range1D
	u0 := since.Truncate(time.Hour).Unix()
	u1 := since.Add(time.Hour).Truncate(time.Hour).Unix()
	commits := map[int64]int{u0: 2, u1: 1}
	merges := map[int64]int{u0: 0, u1: 3}
	issues := map[int64]int{u0: 1, u1: 1}
	series := SeriesFromMaps(since, until, tr, commits, merges, issues)
	got := NormalizeSeries(series)
	if got.Commits != 3 || got.Merges != 3 || got.ClosedIssues != 2 {
		t.Fatalf("%+v", got)
	}
}

func TestSortCommitsByTimeDesc(t *testing.T) {
	a := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	b := a.Add(time.Hour)
	cc := []CommitRow{{CreatedAt: a}, {CreatedAt: b}}
	SortCommitsByTimeDesc(cc)
	if cc[0].CreatedAt != b || cc[1].CreatedAt != a {
		t.Fatal(cc)
	}
}

func TestSortIssuesByTimeDesc(t *testing.T) {
	a := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	b := a.Add(time.Hour)
	ii := []IssueRow{{ClosedAt: a}, {ClosedAt: b}}
	SortIssuesByTimeDesc(ii)
	if ii[0].ClosedAt != b {
		t.Fatal(ii)
	}
}
