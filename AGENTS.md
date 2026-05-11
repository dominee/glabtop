# AGENTS.md — glabtop

Short reference for automated agents working in this repo.

## Purpose

`glabtop` is a terminal UI that polls GitLab (commits, merged MRs, closed issues) for configured **groups** and **projects**, aggregates counts, lists recent activity, caches snapshots in **SQLite**, and renders a **btop-style** layout with **btop-compatible `.theme` files**.

## Layout

| Path | Role |
|------|------|
| [`cmd/glabtop/main.go`](cmd/glabtop/main.go) | Flags, load config/theme/state/cache, GitLab client, `tea.NewProgram` |
| [`internal/config`](internal/config) | TOML load (`glabtop.toml`), paths (`./` then `~/.config/glabtop/`), theme path resolution, `state.toml` persist |
| [`internal/theme`](internal/theme) | Parses `theme[key]="#hex"` lines → lipgloss |
| [`internal/gitlab`](internal/gitlab) | REST client: resolve projects from groups + paths; fetch snapshot (commits / MRs / issues) |
| [`internal/cache`](internal/cache) | SQLite snapshots + **timeline** table (bucket counts for charts / offline) |
| [`internal/model`](internal/model) | `TimeRange`, window bounds/ids, `Snapshot`, row DTOs |
| [`internal/tui`](internal/tui) | Bubble Tea model: panes, refresh ticker, filters, cache warm vs fetch race handling |
| [`Makefile`](Makefile) | `make help`, `all` (CI parity), `build`, `test`, `fmt`, `tidy`, `run` |

## Change hotspots

- **New metric / API**: extend [`internal/gitlab/client.go`](internal/gitlab/client.go) and [`internal/model/model.go`](internal/model/model.go), then map into [`internal/tui/view.go`](internal/tui/view.go).
- **Config keys**: [`internal/config/config.go`](internal/config/config.go) + [`glabtop.toml.example`](glabtop.toml.example) + [`README.md`](README.md).
- **Cache shape**: [`internal/cache/cache.go`](internal/cache/cache.go) and JSON fields on `model.Snapshot`.

## Conventions

- GitLab base URL must not include `/api/v4` (appended in code).
- Token header: `PRIVATE-TOKEN` (from `gitlab.token_env`, default `GITLAB_API_KEY`).
- `INSTRUCTIONS.md` is intentionally gitignored (local context for humans); do not rely on it being in remotes.

## Verify

- `make all` (and CI) runs `fmt-check`, `vet`, `test`, and `build`; run `make tidy` locally after dependency changes.
