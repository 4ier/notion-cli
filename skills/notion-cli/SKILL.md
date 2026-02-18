---
name: notion-cli
description: |
  Work with Notion from the terminal using the `notion` CLI. Use when the user needs to read, create, update, query, or manage Notion pages, databases, blocks, comments, users, or files programmatically. Covers the entire Notion API with 38 commands. Triggers: Notion workspace automation, database queries, page creation, block manipulation, comment threads, file uploads, relation management, or any Notion API interaction from the command line.
---

# Notion CLI

`notion` is a CLI for the Notion API. Single Go binary, full API coverage, dual output (pretty tables for humans, JSON for agents).

## Install

```bash
go install github.com/4ier/notion-cli@latest
```

## Auth

```bash
notion auth login --with-token <<< "ntn_xxxxxxxxxxxxx"   # or interactive
export NOTION_TOKEN=ntn_xxxxxxxxxxxxx                     # env var alternative
notion auth doctor                                        # health check
```

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
notion page restore <id>                 # unarchive page
notion page move <id> --to <parent>
notion page open <id>                    # open in browser
notion page set <id> Key=Value ...       # set properties (type-aware)
notion page props <id>                   # show all properties
notion page props <id> <prop-id>         # get specific property
notion page link <id> --prop "Rel" --to <target-id>    # add relation
notion page unlink <id> --prop "Rel" --from <target-id> # remove relation
```

### Databases
```bash
notion db list                           # list databases
notion db view <id>                      # show schema
notion db query <id>                     # query all rows
notion db query <id> -F 'Status=Done' -s 'Date:desc'  # filter + sort
notion db create <parent> --title "X" --props "Status:select,Date:date"
notion db update <id> --title "New Name" --add-prop "Priority:select"
notion db add <id> "Name=Task" "Status=Todo" "Priority=High"
notion db add-bulk <id> --file items.json              # bulk create from JSON
notion db open <id>                      # open in browser
```

#### Filter operators
| Syntax | Meaning |
|--------|---------|
| `=` | equals |
| `!=` | not equals |
| `>` / `>=` | greater than (or equal) |
| `<` / `<=` | less than (or equal) |
| `~=` | contains |

Multiple `-F` flags combine with AND. Property types are auto-detected from schema.

#### Sort: `-s 'Date:desc'` or `-s 'Name:asc'`

#### Bulk add file format
```json
[{"Name": "Task A", "Status": "Todo"}, {"Name": "Task B", "Status": "Done"}]
```

### Blocks
```bash
notion block list <parent-id>            # list child blocks
notion block list <parent-id> --all      # paginate through all
notion block get <id>                    # get single block
notion block append <parent> "text"      # append paragraph
notion block append <parent> "text" -t bullet          # bullet point
notion block append <parent> "text" -t code --lang go  # code block
notion block append <parent> --file notes.md           # from file
notion block insert <parent> "text" --after <block-id> # positional insert
notion block update <id> --text "new"    # update content
notion block delete <id1> [id2] [id3]    # delete one or more
```

Block types: `paragraph`/`p`, `h1`, `h2`, `h3`, `bullet`, `numbered`, `todo`, `quote`, `code`, `callout`, `divider`

### Comments
```bash
notion comment list <page-id>
notion comment add <page-id> "comment text"
```

### Users
```bash
notion user me                           # current bot info
notion user list                         # all workspace users
notion user get <user-id>
```

### Files
```bash
notion file list                         # list uploads
notion file upload ./path/to/file        # upload (auto MIME detection)
```

### Raw API (escape hatch)
```bash
notion api GET /v1/users/me
notion api POST /v1/search '{"query":"test"}'
notion api PATCH /v1/pages/<id> '{"archived":true}'
```

## Output Modes

- **Terminal (TTY)**: colored tables, readable formatting
- **Piped/scripted**: JSON automatically
- **Explicit**: `--format json` or `--format table`
- `--debug`: show HTTP request/response details

All output includes full Notion UUIDs. All commands accept Notion URLs or IDs.

## Tips

- `notion db add` and `notion page set` auto-detect property types from schema
- Multi-select: `Tags=tag1,tag2,tag3`
- Checkbox: `Done=true`
- Pipe to jq: `notion db query <id> -F 'Status=Done' --format json | jq '.results[].id'`
