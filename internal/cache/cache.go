package cache

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite" // driver

	"glabtop/internal/model"
)

type Store struct {
	db *sql.DB
}

func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil && filepath.Dir(path) != "." {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(`
CREATE TABLE IF NOT EXISTS snapshots (
	window_id TEXT PRIMARY KEY,
	payload TEXT NOT NULL,
	updated_unix INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS meta (
	k TEXT PRIMARY KEY,
	v TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS timeline (
	bucket_unix INTEGER NOT NULL,
	granularity TEXT NOT NULL,
	commits INTEGER NOT NULL DEFAULT 0,
	merges INTEGER NOT NULL DEFAULT 0,
	closed_issues INTEGER NOT NULL DEFAULT 0,
	PRIMARY KEY(bucket_unix, granularity)
);
`); err != nil {
		return nil, err
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) PutSnapshot(snap *model.Snapshot) error {
	if snap == nil {
		return errors.New("nil snapshot")
	}
	b, err := json.Marshal(snap)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(
		`INSERT INTO snapshots(window_id, payload, updated_unix) VALUES(?,?,?)
		 ON CONFLICT(window_id) DO UPDATE SET payload=excluded.payload, updated_unix=excluded.updated_unix`,
		snap.WindowID, string(b), time.Now().Unix(),
	)
	return err
}

func (s *Store) GetSnapshot(windowID string) (*model.Snapshot, error) {
	var payload string
	var u int64
	err := s.db.QueryRow(`SELECT payload, updated_unix FROM snapshots WHERE window_id=?`, windowID).Scan(&payload, &u)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var snap model.Snapshot
	if err := json.Unmarshal([]byte(payload), &snap); err != nil {
		return nil, err
	}
	snap.Stale = true
	return &snap, nil
}

// LastGoodSnapshot returns the most recently updated cache row (any window), for offline boot.
func (s *Store) LastGoodSnapshot() (*model.Snapshot, error) {
	var payload string
	err := s.db.QueryRow(`SELECT payload FROM snapshots ORDER BY updated_unix DESC LIMIT 1`).Scan(&payload)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var snap model.Snapshot
	if err := json.Unmarshal([]byte(payload), &snap); err != nil {
		return nil, err
	}
	snap.Stale = true
	return &snap, nil
}

func (s *Store) SetLastFetch(projectPath string, unix int64) error {
	key := "last_fetch:" + projectPath
	_, err := s.db.Exec(`INSERT INTO meta(k,v) VALUES(?,?) ON CONFLICT(k) DO UPDATE SET v=excluded.v`, key, fmt.Sprintf("%d", unix))
	return err
}

func (s *Store) GetLastFetch(projectPath string) (int64, error) {
	var v string
	err := s.db.QueryRow(`SELECT v FROM meta WHERE k=?`, "last_fetch:"+projectPath).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	var u int64
	_, err = fmt.Sscanf(v, "%d", &u)
	return u, err
}

// Purge removes snapshots, timeline, and meta.
func (s *Store) Purge() error {
	var firstErr error
	for _, q := range []string{
		`DELETE FROM snapshots`,
		`DELETE FROM timeline`,
		`DELETE FROM meta`,
	} {
		if _, err := s.db.Exec(q); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// ReplaceTimeline overwrites activity buckets for [since,until) at the given granularity.
func (s *Store) ReplaceTimeline(gran string, since, until time.Time, buckets []model.TimeBucket) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(
		`DELETE FROM timeline WHERE granularity=? AND bucket_unix>=? AND bucket_unix<?`,
		gran, since.Unix(), until.Unix(),
	); err != nil {
		return err
	}
	st, err := tx.Prepare(`INSERT INTO timeline(bucket_unix,granularity,commits,merges,closed_issues) VALUES(?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer st.Close()
	for _, b := range buckets {
		t, err := time.Parse(time.RFC3339, b.StartRFC3339)
		if err != nil {
			continue
		}
		if _, err := st.Exec(t.Unix(), gran, b.Commits, b.Merges, b.ClosedIssues); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// LoadSeries reads timeline rows into a full zero-filled series for the window.
func (s *Store) LoadSeries(tr model.TimeRange, since, until time.Time) ([]model.TimeBucket, error) {
	gran := model.GranularityKey(tr)
	rows, err := s.db.Query(
		`SELECT bucket_unix, commits, merges, closed_issues FROM timeline
		 WHERE granularity=? AND bucket_unix>=? AND bucket_unix<? ORDER BY bucket_unix`,
		gran, since.Unix(), until.Unix(),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	commits := make(map[int64]int)
	merges := make(map[int64]int)
	issues := make(map[int64]int)
	for rows.Next() {
		var u int64
		var c, m, iss int
		if err := rows.Scan(&u, &c, &m, &iss); err != nil {
			return nil, err
		}
		commits[u] = c
		merges[u] = m
		issues[u] = iss
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return model.SeriesFromMaps(since, until, tr, commits, merges, issues), nil
}
