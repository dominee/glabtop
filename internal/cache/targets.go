package cache

import (
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"glabtop/internal/model"
)

const metaKeyResolvedTargets = "resolved_targets_v1"

type cachedTargetsPayload struct {
	Host     string             `json:"host"`
	Projects []model.ProjectRef `json:"projects"`
	Updated  int64              `json:"updated_unix"`
}

// PutResolvedTargets stores the last successfully resolved project list for offline mode.
func (s *Store) PutResolvedTargets(host string, projects []model.ProjectRef) error {
	host = strings.TrimSpace(strings.TrimRight(host, "/"))
	p := cachedTargetsPayload{
		Host:     host,
		Projects: append([]model.ProjectRef(nil), projects...),
		Updated:  time.Now().Unix(),
	}
	b, err := json.Marshal(p)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(
		`INSERT INTO meta(k,v) VALUES(?,?) ON CONFLICT(k) DO UPDATE SET v=excluded.v`,
		metaKeyResolvedTargets, string(b),
	)
	return err
}

// GetResolvedTargets returns cached project refs and host from the last online resolve.
func (s *Store) GetResolvedTargets() (host string, projects []model.ProjectRef, updated int64, err error) {
	var v string
	err = s.db.QueryRow(`SELECT v FROM meta WHERE k=?`, metaKeyResolvedTargets).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil, 0, nil
	}
	if err != nil {
		return "", nil, 0, err
	}
	var p cachedTargetsPayload
	if err := json.Unmarshal([]byte(v), &p); err != nil {
		return "", nil, 0, err
	}
	return p.Host, p.Projects, p.Updated, nil
}
