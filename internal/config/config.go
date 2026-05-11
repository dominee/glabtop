package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"

	"glabtop/internal/model"
)

const DefaultTokenEnv = "GITLAB_API_KEY"

type File struct {
	GitLab  GitLabSection  `toml:"gitlab"`
	Targets TargetsSection `toml:"targets"`
	UI      UISection      `toml:"ui"`
	Cache   CacheSection   `toml:"cache"`
	Filters FiltersSection `toml:"filters"`
}

type GitLabSection struct {
	Host     string `toml:"host"`
	TokenEnv string `toml:"token_env"`
}

type TargetsSection struct {
	Groups   []string `toml:"groups"`
	Projects []string `toml:"projects"`
}

type UISection struct {
	Theme              string `toml:"theme"`
	ThemePath          string `toml:"theme_path"`
	RefreshIntervalSec int    `toml:"refresh_interval_sec"`
}

type CacheSection struct {
	DBPath string `toml:"db_path"`
}

type FiltersSection struct {
	ProjectSubstring string `toml:"project_substring"`
	Username         string `toml:"username"`
}

// Load searches ./glabtop.toml then ~/.config/glabtop/glabtop.toml unless path is non-empty.
func Load(explicitPath string) (File, string, error) {
	var path string
	var err error
	if explicitPath != "" {
		path = explicitPath
	} else {
		path, err = discoverPath()
		if err != nil {
			return File{}, "", err
		}
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return File{}, path, fmt.Errorf("read config: %w", err)
	}
	var f File
	if _, err := toml.Decode(string(raw), &f); err != nil {
		return File{}, path, fmt.Errorf("parse toml: %w", err)
	}
	setDefaults(&f)
	if err := validate(f); err != nil {
		return File{}, path, err
	}
	return f, path, nil
}

func discoverPath() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	local := filepath.Join(cwd, "glabtop.toml")
	if st, err := os.Stat(local); err == nil && !st.IsDir() {
		return local, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	xdg := filepath.Join(home, ".config", "glabtop", "glabtop.toml")
	if st, err := os.Stat(xdg); err == nil && !st.IsDir() {
		return xdg, nil
	}
	return "", fmt.Errorf("no config found (tried %s and %s)", local, xdg)
}

func setDefaults(f *File) {
	f.GitLab.Host = strings.TrimRight(f.GitLab.Host, "/")
	if f.GitLab.TokenEnv == "" {
		f.GitLab.TokenEnv = DefaultTokenEnv
	}
	if f.UI.RefreshIntervalSec <= 0 {
		f.UI.RefreshIntervalSec = 600
	}
	if f.UI.Theme == "" && f.UI.ThemePath == "" {
		f.UI.Theme = "catppuccin_mocha"
	}
	if f.Cache.DBPath == "" {
		if home, err := os.UserHomeDir(); err == nil {
			f.Cache.DBPath = filepath.Join(home, ".cache", "glabtop", "cache.db")
		} else {
			f.Cache.DBPath = filepath.Join(".", "glabtop-cache.db")
		}
	}
}

func validate(f File) error {
	if f.GitLab.Host == "" {
		return errors.New("gitlab.host is required")
	}
	if strings.Contains(f.GitLab.Host, "/api/") {
		return errors.New("gitlab.host must not include /api path; use e.g. https://gitlab.example.com")
	}
	if len(f.Targets.Groups) == 0 && len(f.Targets.Projects) == 0 {
		return errors.New("targets: set at least one group or project path")
	}
	return nil
}

// ResolvedThemePath returns an absolute path to the active .theme file.
func ResolvedThemePath(f File, configFile string) (string, error) {
	if f.UI.ThemePath != "" {
		if filepath.IsAbs(f.UI.ThemePath) {
			return f.UI.ThemePath, nil
		}
		base := filepath.Dir(configFile)
		return filepath.Abs(filepath.Join(base, f.UI.ThemePath))
	}
	name := f.UI.Theme
	if !strings.HasSuffix(name, ".theme") {
		name += ".theme"
	}
	// Try cwd/themes, repo themes, ~/.config/glabtop/themes
	candidates := []string{
		filepath.Join(".", "themes", name),
		filepath.Join(filepath.Dir(configFile), "themes", name),
	}
	if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates, filepath.Join(home, ".config", "glabtop", "themes", name))
	}
	for _, p := range candidates {
		if ap, err := filepath.Abs(p); err == nil {
			if st, err := os.Stat(ap); err == nil && !st.IsDir() {
				return ap, nil
			}
		}
	}
	return "", fmt.Errorf("theme %q not found in themes/ or ~/.config/glabtop/themes/", f.UI.Theme)
}

func TokenFromEnv(f File) string {
	return os.Getenv(f.GitLab.TokenEnv)
}

// PersistedUIState is optional machine-local toggles.
type PersistedUIState struct {
	TimeRange   model.TimeRange `toml:"time_range"`
	ShowStats   bool            `toml:"show_stats"`
	ShowCommits bool            `toml:"show_commits"`
	ShowIssues  bool            `toml:"show_issues"`
	DetailPane  bool            `toml:"detail_pane"`
	ChartByUser bool            `toml:"chart_by_user"`
}

func StatePath(configPath string) string {
	return filepath.Join(filepath.Dir(configPath), "state.toml")
}

func LoadState(configPath string) (PersistedUIState, error) {
	p := StatePath(configPath)
	raw, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return PersistedUIState{
				TimeRange:   model.Range1W,
				ShowStats:   true,
				ShowCommits: true,
				ShowIssues:  true,
				DetailPane:  false,
				ChartByUser: false,
			}, nil
		}
		return PersistedUIState{}, err
	}
	var s PersistedUIState
	if _, err := toml.Decode(string(raw), &s); err != nil {
		return PersistedUIState{}, err
	}
	if !s.ShowStats && !s.ShowCommits && !s.ShowIssues {
		s.ShowStats, s.ShowCommits, s.ShowIssues = true, true, true
	}
	return s, nil
}

func SaveState(configPath string, s PersistedUIState) error {
	p := StatePath(configPath)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	var buf strings.Builder
	if err := toml.NewEncoder(&buf).Encode(s); err != nil {
		return err
	}
	return os.WriteFile(p, []byte(buf.String()), 0o644)
}
