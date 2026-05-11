package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"glabtop/internal/model"
)

func TestLoad_minimalValid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "glabtop.toml")
	content := `
[gitlab]
host = "https://gitlab.example.com"

[targets]
projects = ["g/p"]
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	f, p, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if p != path {
		t.Fatalf("path %q", p)
	}
	if f.GitLab.Host != "https://gitlab.example.com" {
		t.Fatal(f.GitLab.Host)
	}
	if f.GitLab.TokenEnv != DefaultTokenEnv {
		t.Fatalf("token env default: got %q", f.GitLab.TokenEnv)
	}
	if f.UI.RefreshIntervalSec != 600 {
		t.Fatalf("refresh default: %d", f.UI.RefreshIntervalSec)
	}
	if f.UI.Theme != "catppuccin_mocha" {
		t.Fatal(f.UI.Theme)
	}
	if len(f.Targets.Projects) != 1 || f.Targets.Projects[0] != "g/p" {
		t.Fatal(f.Targets.Projects)
	}
}

func TestLoad_invalidToml(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "glabtop.toml")
	if err := os.WriteFile(path, []byte(`[gitlab`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, _, err := Load(path)
	if err == nil {
		t.Fatal("expected parse error")
	}
}

func TestValidate_table(t *testing.T) {
	tests := []struct {
		name    string
		f       File
		wantErr string
	}{
		{
			name:    "missing host",
			f:       File{Targets: TargetsSection{Projects: []string{"a/b"}}},
			wantErr: "gitlab.host is required",
		},
		{
			name: "host includes api path",
			f: File{
				GitLab:  GitLabSection{Host: "https://x.com/api/v4"},
				Targets: TargetsSection{Projects: []string{"a/b"}},
			},
			wantErr: "/api",
		},
		{
			name: "no targets",
			f: File{
				GitLab: GitLabSection{Host: "https://gitlab.example.com"},
			},
			wantErr: "group or project",
		},
		{
			name: "ok groups only",
			f: File{
				GitLab:  GitLabSection{Host: "https://gitlab.example.com"},
				Targets: TargetsSection{Groups: []string{"g"}},
			},
		},
		{
			name: "trailing slash on host still validates",
			f: File{
				GitLab:  GitLabSection{Host: "https://gitlab.example.com/"},
				Targets: TargetsSection{Projects: []string{"a"}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := tt.f
			setDefaults(&f)
			err := validate(f)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("err %q want substring %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestResolvedThemePath_explicitRelative(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "glabtop.toml")
	themeDir := filepath.Join(dir, "themes")
	if err := os.MkdirAll(themeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	themeFile := filepath.Join(themeDir, "probe.theme")
	if err := os.WriteFile(themeFile, []byte("theme[main_bg]=\"#000000\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	f := File{
		GitLab:  GitLabSection{Host: "https://gitlab.example.com"},
		Targets: TargetsSection{Projects: []string{"a/b"}},
		UI:      UISection{ThemePath: "themes/probe.theme"},
	}
	got, err := ResolvedThemePath(f, cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	want, _ := filepath.Abs(themeFile)
	gotAbs, _ := filepath.Abs(got)
	if gotAbs != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestLoadState_missingFile_defaults(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "glabtop.toml")
	st, err := LoadState(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if st.TimeRange != model.Range1W {
		t.Fatal(st.TimeRange)
	}
	if !st.ShowStats || !st.ShowCommits || !st.ShowIssues {
		t.Fatalf("%+v", st)
	}
}

func TestSaveState_roundTrip(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "glabtop.toml")
	want := PersistedUIState{
		TimeRange:   model.Range1M,
		ShowStats:   false,
		ShowCommits: true,
		ShowIssues:  true,
		DetailPane:  true,
		ChartByUser: true,
	}
	if err := SaveState(cfg, want); err != nil {
		t.Fatal(err)
	}
	got, err := LoadState(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if got.TimeRange != want.TimeRange || got.DetailPane != want.DetailPane || got.ChartByUser != want.ChartByUser {
		t.Fatalf("%+v vs %+v", got, want)
	}
}

func TestLoadState_allPanesOff_forcesMinimum(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.toml")
	raw := `
show_stats = false
show_commits = false
show_issues = false
`
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := filepath.Join(dir, "glabtop.toml")
	st, err := LoadState(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !st.ShowStats || !st.ShowCommits || !st.ShowIssues {
		t.Fatalf("expected fallback to all true: %+v", st)
	}
}

func TestTokenFromEnv(t *testing.T) {
	t.Setenv("MY_GITLAB", "secret")
	f := File{GitLab: GitLabSection{TokenEnv: "MY_GITLAB"}}
	if TokenFromEnv(f) != "secret" {
		t.Fatal()
	}
}
