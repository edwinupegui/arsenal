# Arsenal

Local-first manager for your curated technical resources — videos, articles, repos, tools, courses, podcasts, newsletters, communities and more — stored in a single SQLite database under `~/.arsenal/`.

Three faces, one database:

- **`arsenal`** — interactive TUI (Bubble Tea).
- **`arsenal <verb>`** — fast command-line operations (`add`, `list`, `search`, `show`, `rm`, `restore`, `purge`, `trash`, `star`, `cat`, `tag`, `export`, `import`, `migrate`, `backup`).
- **`arsenal web`** — local HTTP server with HTMX UI on `http://127.0.0.1:7777`, auto-opened in your browser.

Anything you do in one surface is visible in the others — they share the same SQLite file.

## Install

### Homebrew (macOS / Linux)

```bash
brew install edwinupegui/tap/arsenal
```

### `go install`

```bash
go install github.com/edwinupegui/arsenal/cmd/arsenal@latest
```

The binary lands at `$(go env GOPATH)/bin/arsenal`.

### From source

```bash
git clone https://github.com/edwinupegui/arsenal
cd arsenal
make install-tools   # one-time: goose, sqlc, goreleaser, golangci-lint, goimports
make build           # builds ./arsenal
```

### Pre-built binaries

Each tagged release on GitHub publishes archives for darwin / linux / windows × amd64 / arm64. Download from the [releases page](https://github.com/edwinupegui/arsenal/releases) and extract `arsenal` somewhere on your `PATH`.

## First run

```bash
arsenal init                # creates ~/.arsenal/{arsenal.db,backups/,logs/}
arsenal add https://example.com/post --tag patterns
arsenal                     # opens the TUI
arsenal web                 # opens the browser UI
```

`arsenal init` is idempotent — running it on an existing database just applies any pending migrations.

If you're coming from the legacy Astro app:

```bash
arsenal migrate --from /path/to/old/resources.db
```

## Shell completion

Cobra emits completion scripts for the major shells. Slugs, tag names, type / language enums and recent resource ids tab-complete dynamically against your live database.

| Shell | One-liner |
|---|---|
| bash | `arsenal completion bash > ~/.bash_completion.d/arsenal` (or pipe into `/etc/bash_completion.d/`) |
| zsh  | `arsenal completion zsh > "${fpath[1]}/_arsenal"` (then `compinit`) |
| fish | `arsenal completion fish > ~/.config/fish/completions/arsenal.fish` |
| pwsh | `arsenal completion powershell \| Out-String \| Invoke-Expression` |

The Homebrew formula installs all four automatically.

## Where data lives

| Path | Purpose |
|---|---|
| `~/.arsenal/arsenal.db` | Source of truth (override with `ARSENAL_HOME=/some/dir`) |
| `~/.arsenal/backups/` | `arsenal backup` snapshots |
| `~/.arsenal/logs/` | Reserved for future structured logs |

## Project layout

```
cmd/arsenal/           binary entrypoint
internal/
  backup/              VACUUM INTO snapshot
  cli/                 cobra commands (one file per surface)
  config/              XDG paths (~/.arsenal)
  domain/              pure domain types + validators
  exportmd/            markdown export and import
  migrate/legacy/      one-shot import from arsenal-app v1
  migrations/          embedded goose SQL migrations
  resources/           service layer (Create/Update/Import/SoftDelete/Restore/Purge/SetFavorite)
  scrape/              OG metadata fetcher for `arsenal add <url>`
  store/               sqlc-generated queries + connection + FTS5 search
  tui/                 Bubble Tea models
  web/                 chi router + html/template + HTMX
    templates/         html/template files
    static/            app.css + htmx.min.js (vendored)
```

## Make targets

```
make build              build local binary
make run                build and run TUI
make test               go test ./... -race -count=1
make test-cover         coverage report
make lint               golangci-lint
make fmt                go fmt + goimports
make tidy               go mod tidy
make sqlc               regenerate internal/store/ from queries/
make migrate-up         apply pending migrations to ./arsenal.db
make migrate-new NAME=… create a new migration file
make completions        emit dist/completions/{bash,zsh,fish}
make release-check      validate .goreleaser.yaml
make release-snapshot   build a full release locally without publishing
make install-tools      install dev tools to $GOPATH/bin
```

## Data model

| Table          | Purpose |
|----------------|---------|
| `resources`    | One row per saved item: title, url, description, type, language, optional category, notes, favorite flag, soft-delete timestamp. |
| `categories`   | Curated buckets with icons (uncategorized resources are allowed). |
| `tags`         | Free-form labels, normalized (lowercase, trimmed, deduped). |
| `resource_tags`| Many-to-many resource ↔ tag. |
| `resources_fts`| FTS5 virtual table indexing title, description, notes, joined tag list. Triggers keep it in sync. Diacritics are folded. |

## Releasing

The repo ships with GoReleaser config and a GitHub Actions workflow that fires on `v*.*.*` tags.

To enable the Homebrew tap:

1. Create an empty repo at `github.com/edwinupegui/homebrew-tap`.
2. Mint a personal access token (classic) with `repo` scope.
3. Add it as `HOMEBREW_TAP_GITHUB_TOKEN` under the arsenal repo's Actions secrets.
4. Tag and push:
   ```bash
   git tag v0.1.0 && git push origin v0.1.0
   ```

GoReleaser builds darwin/linux/windows × amd64/arm64, uploads archives to the GitHub release, and commits the formula to the tap.

For a dry-run locally:

```bash
make release-snapshot
ls dist/
```

## License

MIT — see [LICENSE](LICENSE).
