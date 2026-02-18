---
name: notion-cli
description: |
  Work with Notion from the terminal using the `notion` CLI. Use when the user needs to read, create, update, query, or manage Notion pages, databases, blocks, comments, users, or files programmatically. Covers the entire Notion API. Triggers: Notion workspace automation, database queries, page creation, block manipulation, comment threads, file uploads, or any Notion API interaction from the command line.
---

# Notion CLI

`notion` is a CLI for the Notion API. Single binary, full API coverage, dual output (pretty tables for humans, JSON for agents).

## Install

```bash
go install github.com/4ier/notion-cli@latest
```

Or download a binary from [GitHub Releases](https://github.com/4ier/notion-cli/releases).

## Auth

```bash
# Option 1: Save token to config
notion auth login --token ntn_xxxxxxxxxxxxx

# Option 2: Environment variable
export NOTION_TOKEN=ntn_xxxxxxxxxxxxx
```

Create an integration at https://www.notion.so/my-integrations. Share target pages/databases with the integration.

## Command Reference

### Search
```bash
notion search "query"                    # search everything
notion search "query" --type page        # pages only
notion search "query" --type database    # databases only
```

### Pages
```bash
notion page view <id|url>                # render page content
notion page list                         # list workspace pages
notion page create <parent> --title "X" --body "content"
notion page delete <id>                  # archive page
notion page move <id> --to <parent>      # reparent page
notion page open <id>                    # open in browser
notion page set <id> Key=Value ...       # set properties (type-aware)
notion page props <id>                   # show all properties
notion page props <id> <prop-id>         # get specific property
```

### Databases
```bash
notion db list                           # list accessible databases
notion db view <id>                      # show schema (columns, types, options)
notion db query <id>                     # query all rows
notion db query <id> -F 'Status=Done' -s 'Date:desc'  # filter + sort
notion db create <parent> --title "X" --props "Status:select,Date:date"
notion db update <id> --title "New Name" --add-prop "Priority:select"
notion db add <id> "Name=Task" "Status=Todo" "Date=2026-01-01"
notion db open <id>                      # open in browser
```

#### Filter operators
| Syntax | Meaning |
|--------|---------|
| `=` | equals |
| `!=` | not equals |
| `>` | greater than |
| `>=` | greater than or equal |
| `<` | less than |
| `<=` | less than or equal |
| `~=` | contains |

Multiple `-F` flags combine with AND. Property types are auto-detected from schema.

#### Sort syntax
```bash
-s 'Date:desc'    # descending
-s 'Name:asc'     # ascending (default)
```

### Blocks
```bash
notion block list <parent-id>            # list child blocks
notion block get <id>                    # get single block
notion block append <parent> "text"      # append paragraph
notion block append <parent> "text" -t bullet          # bullet point
notion block append <parent> "text" -t h1              # heading 1
notion block append <parent> "code" -t code --lang go  # code block
notion block update <id> --text "new"    # update block content
notion block delete <id>                 # delete block
```

Block types: `paragraph`, `h1`/`heading1`, `h2`, `h3`, `bullet`, `numbered`, `todo`, `quote`, `code`, `callout`, `divider`

### Comments
```bash
notion comment list <page-id>
notion comment add <page-id> "comment text"
```

### Users
```bash
notion user me                           # current bot info
notion user list                         # all workspace users
notion user get <user-id>               # specific user
```

### Files
```bash
notion file list                         # list uploads
notion file upload ./path/to/file        # upload file (auto MIME detection)
```

### Raw API (escape hatch)
```bash
notion api GET /v1/users/me
notion api POST /v1/search '{"query":"test"}'
notion api PATCH /v1/pages/<id> '{"archived":true}'
```

## Output Modes

- **Terminal (TTY)**: colored tables, readable formatting
- **Piped / scripted**: JSON automatically
- **Explicit**: `--format json` or `--format table`
- `--debug`: show HTTP request/response details

All commands output full Notion UUIDs (never truncated).

## Tips

- All commands accept both Notion UUIDs and full `notion.so` URLs
- `notion db add` and `notion page set` auto-detect property types from the schema — just use `Key=Value`
- For multi-select: `Tags=tag1,tag2,tag3`
- For checkbox: `Done=true` or `Done=false`
- Pipe JSON output to `jq` for extraction: `notion db query <id> -F 'Status=Done' --format json | jq '.results[].id'`
