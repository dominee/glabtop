package model

import (
	"testing"
	"time"
)

func TestBuildUserChartFromAgg_topNAndOthers(t *testing.T) {
	since := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	until := since.Add(48 * time.Hour)
	tr := Range1W
	per := map[int64]map[string]int{
		since.Unix():                     {"alice": 2, "bob": 1, "cara": 1},
		since.Add(24 * time.Hour).Unix(): {"alice": 1, "bob": 3},
	}
	uc := BuildUserChartFromAgg(since, until, tr, per, 2)
	if len(uc.Users) != 3 {
		t.Fatalf("users: got %v want len 3 (2 + others)", uc.Users)
	}
	if uc.Users[2] != "others" {
		t.Fatalf("expected others last, got %v", uc.Users)
	}
	if uc.MaxTotal != 4 {
		t.Fatalf("max total %d, want 4", uc.MaxTotal)
	}
}

func TestUserLabel(t *testing.T) {
	if got := UserLabel("  Ann ", ""); got != "Ann" {
		t.Fatal(got)
	}
	if got := UserLabel("", "x@y.z"); got != "x" {
		t.Fatal(got)
	}
	if got := UserLabel("", ""); got != "unknown" {
		t.Fatal(got)
	}
}

func TestDistinctUserCount(t *testing.T) {
	m := map[int64]map[string]int{
		1: {"alice": 1, "bob": 1},
		2: {"bob": 2, "cara": 1},
	}
	if n := DistinctUserCount(m); n != 3 {
		t.Fatalf("got %d want 3", n)
	}
	if DistinctUserCount(nil) != 0 || DistinctUserCount(map[int64]map[string]int{}) != 0 {
		t.Fatal()
	}
}

func TestIssueChartActor(t *testing.T) {
	if got := IssueChartActor(IssueRow{AssigneeName: "Sam", AuthorName: "Ann"}); got != "Sam" {
		t.Fatal(got)
	}
	if got := IssueChartActor(IssueRow{AuthorName: "Pat"}); got != "Pat" {
		t.Fatal(got)
	}
	if got := IssueChartActor(IssueRow{}); got != "unknown" {
		t.Fatal(got)
	}
}
