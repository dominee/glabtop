package cache

import (
	"path/filepath"
	"testing"
	"time"

	"glabtop/internal/model"
)

func TestPutGetSnapshot_roundTrip(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	snap := &model.Snapshot{
		WindowID:     "1d:test-window",
		TimeRange:    "1d",
		SinceRFC3339: "2026-05-01T00:00:00Z",
		UntilRFC3339: "2026-05-02T00:00:00Z",
		Counts: model.Counts{
			Commits:       3,
			Merges:        1,
			ClosedIssues:  2,
			OpenIssues:    4,
			DistinctUsers: 5,
		},
		Series: []model.TimeBucket{
			{StartRFC3339: "2026-05-01T00:00:00Z", Commits: 3, Merges: 1, ClosedIssues: 2},
		},
		FetchedUnix: 12345,
		Stale:       false,
	}
	if err := s.PutSnapshot(snap); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetSnapshot("1d:test-window")
	if err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Fatal("nil snapshot")
	}
	if !got.Stale {
		t.Fatal("GetSnapshot must mark stale")
	}
	if got.Counts.Commits != 3 || got.Counts.OpenIssues != 4 || got.Counts.DistinctUsers != 5 {
		t.Fatalf("%+v", got.Counts)
	}
	if len(got.Series) != 1 {
		t.Fatal(got.Series)
	}
}

func TestPutSnapshot_nil(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.PutSnapshot(nil); err == nil {
		t.Fatal("expected error")
	}
}

func TestGetSnapshot_missing(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	got, err := s.GetSnapshot("nope")
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Fatal(got)
	}
}

func TestLastGoodSnapshot_newestRow(t *testing.T) {
	if testing.Short() {
		t.Skip("needs >1s delay between PutSnapshot for distinct updated_unix")
	}
	s, err := Open(filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	_ = s.PutSnapshot(&model.Snapshot{WindowID: "a", Series: []model.TimeBucket{{Commits: 1}}})
	time.Sleep(1100 * time.Millisecond)
	_ = s.PutSnapshot(&model.Snapshot{WindowID: "b", Series: []model.TimeBucket{{Commits: 2}}})

	last, err := s.LastGoodSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	if last == nil || len(last.Series) != 1 || last.Series[0].Commits != 2 {
		t.Fatalf("%+v", last)
	}
}

func TestReplaceTimeline_LoadSeries(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	since := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	until := since.Add(3 * time.Hour)
	gran := model.GranularityKey(model.Range1D)
	buckets := []model.TimeBucket{
		{StartRFC3339: since.Format(time.RFC3339), Commits: 1},
		{StartRFC3339: since.Add(time.Hour).Format(time.RFC3339), Merges: 2},
		{StartRFC3339: since.Add(2 * time.Hour).Format(time.RFC3339), ClosedIssues: 1},
	}
	if err := s.ReplaceTimeline(gran, since, until, buckets); err != nil {
		t.Fatal(err)
	}
	series, err := s.LoadSeries(model.Range1D, since, until)
	if err != nil {
		t.Fatal(err)
	}
	got := model.NormalizeSeries(series)
	if got.Commits != 1 || got.Merges != 2 || got.ClosedIssues != 1 {
		t.Fatalf("%+v", got)
	}
}

func TestMeta_lastFetch(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	u, err := s.GetLastFetch("g/p")
	if err != nil || u != 0 {
		t.Fatal(u, err)
	}
	if err := s.SetLastFetch("g/p", 999); err != nil {
		t.Fatal(err)
	}
	u, err = s.GetLastFetch("g/p")
	if err != nil || u != 999 {
		t.Fatal(u, err)
	}
}
