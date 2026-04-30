---
name: notion-cli
description: |
  Work with Notion from the terminal using the `notion` CLI. Use when the user needs to read, create, update, query, or manage Notion pages, databases, blocks, comments, users, or files programmatically. Covers the entire Notion API with 50+ commands. Triggers: Notion workspace automation, database queries, page creation, block manipulation, comment threads, file uploads, relation management, database export, multi-workspace management, or any Notion API interaction from the command line.
---

# Notion CLI

`notion` is a CLI for the Notion API. Single Go binary, full API coverage, dual output (pretty tables for humans, JSON for agents). Current: **v0.7.0**.

## Install

```bash
# Homebrew
brew install 4ier/tap/notion-cli

# npm
npm install -g @4ier/notion-cli

# Go
go install github.com/4ier/notion-cli@latest

# Or download a binary from https://github.com/4ier/notion-cli/releases
```

## Auth

```bash
notion auth login --with-token <<< "ntn_xxxxxxxxxxxxx"
notion auth login --with-token --profile work <<< "ntn_xxx"  # named profile
export NOTION_TOKEN=ntn_xxxxxxxxxxxxx                        # env var alternative
notion auth status        # shows workspace + integration type (internal/public)
notion auth switch        # interactive profile picker
notion auth switch work   # direct switch
notion auth doctor        # health check — warns if internal integration
```

`auth status` / `doctor` surface the integration type, so it's easy to spot when you need to share a parent page before creating workspace-root content.

## Search

```bash
notion search "query"                    # everything
notion search "query" --type page        # pages only
notion search "query" --type database    # databases only
```

## Pages

```bash
notion page view <id|url>                # render page content
notion page list                         # list workspace pages
notion page create <parent> --title "X" --body "content"
notion page create <db-id> --db "Name=Review" "Status=Todo"  # database row

# Archive / restore (soft-delete)
notion page archive <id>                 # canonical
notion page trash <id>                   # alias
notion page delete <id>                  # alias (legacy)
notion page restore <id>                 # reverse

# Move / open / edit
notion page move <id> --to <parent>
notion page open <id>                    # open in browser
notion page edit <id|url>                # edit in $EDITOR (markdown round-trip)

# Properties (type-aware)
notion page set <id> Key=Value ...
notion page props <id>                   # show all (summary; paginated values may be truncated)
notion page props <id> <prop-id>         # single raw JSON

# NEW in v0.7: paginated single-property fetch (fixes >25-item truncation)
notion page property <id> <prop-id>
notion page property <id> --name "References"         # resolve id by display name
notion page property <id> <prop-id> --format json

# Relations
notion page link <id> --prop "Rel" --to <target-id>
notion page unlink <id> --prop "Rel" --from <target-id>

# NEW in v0.7: server-side markdown I/O (preferred for full-page dumps)
notion page markdown <id>                # print to stdout
notion page markdown <id> --out page.md  # write to file
notion page markdown <id> --format json  # full response (truncated flag, unknown_block_ids)

notion page set-markdown <id> --file new.md           # replace whole page (default)
cat new.md | notion page set-markdown <id> --file -   # stdin
notion page set-markdown <id> --append --text "\n\n> Appended"
notion page set-markdown <id> --after "Status...pending" --text "Now: done"
notion page set-markdown <id> --range "old...stale" --text "fresh" --allow-deleting-content
```

**`page markdown` vs `block list --md`**: prefer `page markdown` for whole pages — it uses the server renderer and handles toggles, columns, synced blocks, and databases-as-pages correctly. Use `block list --md` only when you need a single sub-block.

## Databases

```bash
notion db list                           # list databases
notion db view <id>                      # show schema
notion db query <id>                     # all rows
notion db query <id> -F 'Status=Done' -s 'Date:desc'
notion db query <id> --filter-json '{"or":[...]}'
notion db query <id> --all
notion db create <parent> --title "X" --props "Status:select,Date:date"
notion db update <id> --title "New Name" --add-prop "Priority:select"
notion db add <id> "Name=Task" "Status=Todo" "Priority=High"
notion db add-bulk <id> --file items.json
notion db export <id>                    # CSV (default)
notion db export <id> --format json
notion db export <id> --format md -o report.md
notion db open <id>
```

### Filter operators

| Syntax | Meaning |
|--------|---------|
| `=` | equals |
| `!=` | not equals |
| `>` / `>=` | greater than (or equal) |
| `<` / `<=` | less than (or equal) |
| `~=` | contains |

Multiple `-F` flags combine with AND. Property types are auto-detected from schema.

### Sort: `-s 'Date:desc'` or `-s 'Name:asc'`

### Bulk add file format

```json
[{"Name": "Task A", "Status": "Todo"}, {"Name": "Task B", "Status": "Done"}]
```

## Blocks

```bash
notion block list <parent-id>            # list child blocks
notion block list <parent-id> --all
notion block list <parent-id> --depth 3  # recursive
notion block list <parent-id> --md       # markdown; prefer 'page markdown' for pages
notion block get <id>

# Append / insert — both handle >100 children and >2000-char code blocks automatically
notion block append <parent> "text"
notion block append <parent> "text" -t bullet
notion block append <parent> "text" -t code --lang ts        # 'ts' / 'sh' / 'yml' etc. normalized
notion block append <parent> --file notes.md                 # any length, auto-batched
notion block append <parent> --file big.md --on-oversize=truncate
notion block insert <parent> "text" --after <block-id>

# Update — now with markdown support
notion block update <id> --text "plain new content"
notion block update <id> --text "See **[docs](https://x.com)**" --markdown
notion block update <id> --file patch.md                     # must parse to exactly one block

notion block delete <id1> [id2] [id3]
notion block move <id> --after <target>
notion block move <id> --before <target>
notion block move <id> --parent <new-parent>
```

Block types: `paragraph`/`p`, `h1`/`h2`/`h3`, `bullet`, `numbered`, `todo`, `quote`, `code`, `callout`, `divider`.

### Media blocks (image / file / video / audio / pdf)

```bash
# External URL (http/https)
notion block append <page> --image-url https://example.com/a.png --caption "fig 1"

# Local file → upload + embed in one command
notion block append <page> --image-file ./chart.png --caption "heap usage"
notion block append <page> --pdf-file ./spec.pdf
notion block append <page> --video-file ./demo.mp4

# Reference an existing file_upload id
notion block append <page> --image-upload 351d45fb-... --caption "reused"
```

Same triple (`--<kind>-url` / `--<kind>-file` / `--<kind>-upload`) exists for `image`, `file`, `video`, `audio`, `pdf`. `--caption` works with any.

## Comments

```bash
notion comment list <page-id>
notion comment add <page-id> --text "comment text"
notion comment add <page-id> --text "with @mention" --mention-user <user-id>
notion comment get <comment-id>
notion comment reply <comment-id> "reply text"          # same thread

# NEW in v0.7
notion comment update <comment-id> --text "edited text"
notion comment update <comment-id> --text "with @mention" --mention-user <user-id>
notion comment delete <comment-id>                      # single or many
notion comment delete <id1> <id2> <id3>                 # variadic
```

## Users

```bash
notion user me                           # current bot info
notion user list                         # all workspace users
notion user get <user-id>
```

## Files

```bash
# List / retrieve / upload
notion file list
notion file get <upload-id>                                # NEW in v0.7 — status / size / URL
notion file upload ./local/file.pdf                        # local path
notion file upload https://example.com/chart.png --name chart.png   # URL source
curl -sSL https://host/file.zip | notion file upload - --name file.zip   # stdin
notion file upload ./image.png --to <page-id>              # upload + attach to page
```

## Raw API (escape hatch)

```bash
notion api GET /v1/users/me              # /v1/ is auto-prepended if missing
notion api GET /users/me                 # → prints 'note: prepending /v1...' and works
notion api POST /v1/search --body '{"query":"test"}'
notion api PATCH /v1/pages/<id> --body @body.json        # read body from file
echo '{"query":"x"}' | notion api POST /v1/search --body -  # explicit stdin
```

## Output Modes

- **Terminal (TTY)**: colored tables, readable formatting
- **Piped / scripted**: JSON automatically
- **Explicit**: `--format json` / `--format table` / `--format md`
- `--debug`: show HTTP request/response details

All output includes full Notion UUIDs. All commands accept Notion URLs or IDs.

## Tips for agents

- `notion db add` and `notion page set` auto-detect property types from schema, so `Tags=a,b,c` (multi_select) and `Done=true` (checkbox) both just work.
- For long markdown, prefer `notion page set-markdown --file` over `notion block append --file` — server-side parsing has no 100-children limit.
- For relation / rollup properties that may have >25 items, always use `notion page property` (not `page view` / `page props`).
- Pipe to `jq`: `notion db query <id> -F 'Status=Done' --format json | jq '.results[].id'`
- When an error looks confusing, check it for a `→` hint line — the CLI decorates common API errors with actionable next steps.
- When working with an internal integration: workspace-root page creation isn't allowed — share a parent page first, then pass its id.
