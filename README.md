# Arsenal

Local-first manager for your curated technical resources — videos, articles, repos, tools, courses, podcasts, newsletters, communities and more — stored in a single SQLite database under `~/.arsenal/`.

Three modes:

- **`arsenal`** — interactive TUI (Bubble Tea).
- **`arsenal <verb>`** — fast command-line operations (`add`, `list`, `search`, `rm`, `restore`, `purge`, `trash`, `star`, `cat`, `tag`, `export`, `import`, `migrate`, `backup`).
- **`arsenal web`** — local HTTP server with HTMX UI, auto-opened in your browser.

> Status: M0 bootstrap. Subcommands beyond `version` land in M1.

## Quick start

```bash
make install-tools   # one-time: goose, sqlc, golangci-lint, goimports
make build           # builds ./arsenal
./arsenal version
./arsenal --help
```

## Layout

```
cmd/arsenal/           # binary entrypoint, embeds migrations
internal/
  cli/                 # cobra commands
  config/              # XDG paths (~/.arsenal)
  domain/              # pure domain types
  store/               # sqlc-generated queries + connection + FTS5 search
    queries/           # *.sql sources for sqlc
  migrate/legacy/      # one-shot import from arsenal-app v1 (resources.db)
  scrape/              # OG metadata fetcher for `arsenal add <url>`
  tui/                 # Bubble Tea models
  web/                 # chi router + templ + HTMX
  log/                 # charm log wrapper
migrations/            # goose SQL migrations
web/templates/         # *.templ
web/static/            # htmx, css
```

## Make targets

```
make build         build local binary
make test          go test ./... -race -count=1
make test-cover    coverage report
make lint          golangci-lint
make migrate-up    apply pending migrations against ./arsenal.db
make sqlc          regenerate store/ from queries/
make install-tools install dev tools to $GOPATH/bin
```

## Data model

| Table          | Purpose |
|----------------|---------|
| `resources`    | One row per saved item: title, url, description, type, language, optional category, notes, favorite flag, soft-delete timestamp. |
| `categories`   | 10 curated categories with icons (Architecture, AI-Native, Gamedev, …). |
| `tags`         | Free-form labels, normalized. |
| `resource_tags`| Many-to-many resource ↔ tag. |
| `resources_fts`| FTS5 virtual table indexing title, description, notes, tags. Triggers keep it in sync. |

## License

MIT — see [LICENSE](LICENSE).
