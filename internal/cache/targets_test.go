package cache

import (
	"os"
	"path/filepath"
	"testing"

	"glabtop/internal/model"
)

func TestResolvedTargetsRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	refs := []model.ProjectRef{
		{ID: 1, PathWithNamespace: "g/a"},
		{ID: 2, PathWithNamespace: "g/b"},
	}
	if err := s.PutResolvedTargets("https://gitlab.example.com", refs); err != nil {
		t.Fatal(err)
	}
	host, out, u, err := s.GetResolvedTargets()
	if err != nil {
		t.Fatal(err)
	}
	if host != "https://gitlab.example.com" {
		t.Fatalf("host %q", host)
	}
	if u == 0 {
		t.Fatal("updated")
	}
	if len(out) != 2 || out[0].PathWithNamespace != "g/a" {
		t.Fatalf("projects %#v", out)
	}

	_ = s.Purge()
	_, out2, _, err := s.GetResolvedTargets()
	if err != nil {
		t.Fatal(err)
	}
	if len(out2) != 0 {
		t.Fatal("expected empty after purge")
	}
}

func TestOpenCreatesParentDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "cache.db")
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err := os.Stat(filepath.Dir(dir)); err != nil {
		t.Fatal(err)
	}
}
