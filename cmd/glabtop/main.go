package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"glabtop/internal/cache"
	"glabtop/internal/config"
	"glabtop/internal/gitlab"
	"glabtop/internal/model"
	"glabtop/internal/theme"
	"glabtop/internal/tui"
)

func main() {
	cfgPath := flag.String("config", "", "path to glabtop.toml (default: search paths)")
	offline := flag.Bool("offline", false, "show cached data only (no network)")
	purgeCache := flag.Bool("purge-cache", false, "delete all cached snapshots and timeline data, then run (forces GitLab re-sync)")
	flag.Parse()

	cfg, path, err := config.Load(*cfgPath)
	if err != nil {
		log.Fatal(err)
	}

	store, err := cache.Open(cfg.Cache.DBPath)
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()

	if *purgeCache {
		if err := store.Purge(); err != nil {
			log.Fatalf("purge cache: %v", err)
		}
	}

	themePath, err := config.ResolvedThemePath(cfg, path)
	if err != nil {
		log.Fatal(err)
	}
	thm, err := theme.Load(themePath)
	if err != nil {
		log.Fatal(err)
	}

	st, err := config.LoadState(path)
	if err != nil {
		log.Fatal(err)
	}

	var bootstrap *model.Snapshot
	if !*purgeCache {
		if snap, err := store.GetSnapshot(model.WindowID(time.Now().UTC(), st.TimeRange)); err == nil && snap != nil {
			bootstrap = snap
		} else if snap, err := store.LastGoodSnapshot(); err == nil && snap != nil {
			bootstrap = snap
		}
	}

	var client *gitlab.Client
	if !*offline {
		tok := config.TokenFromEnv(cfg)
		if tok == "" {
			fmt.Fprintf(os.Stderr, "missing %s (export your GitLab token) or use -offline\n", cfg.GitLab.TokenEnv)
			os.Exit(1)
		}
		client, err = gitlab.NewClient(cfg.GitLab.Host, tok)
		if err != nil {
			log.Fatal(err)
		}
	}

	p := tea.NewProgram(
		tui.NewModel(cfg, path, thm, client, store, st, *offline, bootstrap),
		tea.WithAltScreen(),
	)
	if _, err := p.Run(); err != nil {
		log.Fatal(err)
	}
}
