# glabtop

GitLab activity dashboard in your terminal — **btop**-inspired layout, **Bubble Tea** TUI, and **GitLab REST** polling with **SQLite** caching for semi-realtime visibility into commits, merges, and closed issues across groups and projects.

## Requirements

- Go **1.22+** (to build)
- A GitLab **personal access token** or **project access token** with API scope
- Optional: your own **btop** theme files (this repo ships themes under `themes/`)

## Install / run

```bash
go build -o glabtop ./cmd/glabtop
./glabtop
```

Flags:

- `-purge-cache` — wipe the SQLite DB (snapshots + timeline buckets), then start fresh; the first sync repulls history from GitLab for charts and lists.

- `-config path` — use a specific `glabtop.toml` (otherwise `./glabtop.toml` then `~/.config/glabtop/glabtop.toml` is used)
- `-offline` — read the local cache only (no `GITLAB_API_KEY` required)

## Configuration

1. Copy [`glabtop.toml.example`](glabtop.toml.example) to `./glabtop.toml` or `~/.config/glabtop/glabtop.toml`.

2. Set **`gitlab.host`** to the instance root only — **no** `/api/v4` suffix.

   Example: if your project URL is  
   `https://gitlab.int.example.net/protrion/protrion`  
   then use:

   ```toml
   [gitlab]
   host = "https://gitlab.int.example.net"
   ```

   and list the path under **`targets`**:

   ```toml
   [targets]
   groups = ["protrion"]
   projects = ["protrion/protrion"]
   ```

3. Export your token (default env name is **`GITLAB_API_KEY`**):

   ```bash
   export GITLAB_API_KEY=glpat-xxxxxxxx
   ```

4. **Themes**: set `ui.theme` to a name matching `themes/<name>.theme` (default: **catppuccin_mocha**), or set `ui.theme_path` to a file.

5. **Refresh**: `ui.refresh_interval_sec` defaults to **600** (10 minutes). Press **`r`** for an immediate refresh, **`p`** to pause auto-refresh.

## UI keys

| Key | Action |
|-----|--------|
| `t` | Cycle time range: 1h → 1d → 1w → 1m → 1y |
| `r` | Refresh now |
| `p` | Pause / resume auto-refresh |
| `1` `2` `3` | Toggle stats / commits / issues panes |
| `d` | Detail layout (focus one list) |
| `Tab` | Switch list focus (commits vs issues) |
| `/` | Edit filter (substring with `/` → project path, else author) |
| `q` / `Ctrl+C` | Quit (saves pane layout to `state.toml` beside the config) |

## Cache & offline

- SQLite DB path: **`cache.db_path`** in config, defaulting to **`~/.cache/glabtop/cache.db`**.
- **Timeline** rows store per-bucket commit / merge / closed-issue counts so the **stacked time-axis chart** can reload quickly and work in **`-offline`** mode even when the in-memory snapshot has no embedded `series`.
- Cached snapshots are keyed by window id; the TUI shows **CACHED** when displaying data from disk without a live refresh.
- Use **`-offline`** for read-only historical views (e.g. no VPN).
- Use **`-purge-cache`** to wipe snapshots and timeline and force a full re-pull from GitLab on the next run.

## Development

```bash
make help     # targets
make all      # fmt-check, vet, test, build (matches CI)
make tidy     # go mod tidy (run before committing dep changes)
```

## License

See repository metadata (add a `LICENSE` file if you publish this project publicly).
