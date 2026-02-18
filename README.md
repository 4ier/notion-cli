<h1 align="center">
  notion-cli
</h1>

<p align="center">
  <b>Like <code>gh</code> for GitHub, but for Notion. 39 commands. One binary.</b>
</p>

<p align="center">
  <img src="https://github.com/4ier/notion-cli/releases/download/v0.2.0/demo.gif" alt="demo" width="640">
</p>

<p align="center">
  <a href="https://github.com/4ier/notion-cli/releases"><img src="https://img.shields.io/github/v/release/4ier/notion-cli?style=flat-square" alt="Release"></a>
  <a href="https://github.com/4ier/notion-cli/actions"><img src="https://img.shields.io/github/actions/workflow/status/4ier/notion-cli/test.yml?style=flat-square&label=tests" alt="Tests"></a>
  <a href="https://github.com/4ier/notion-cli/blob/main/LICENSE"><img src="https://img.shields.io/github/license/4ier/notion-cli?style=flat-square" alt="License"></a>
  <a href="https://goreportcard.com/report/github.com/4ier/notion-cli"><img src="https://goreportcard.com/badge/github.com/4ier/notion-cli?style=flat-square" alt="Go Report Card"></a>
</p>

---

A full-featured command-line interface for [Notion](https://notion.so). Manage pages, databases, blocks, comments, users, and files — all from your terminal. Built for developers and AI agents who need programmatic access without the browser.

## Install

### Homebrew (macOS/Linux)
```sh
brew install 4ier/tap/notion-cli
```

### Go
```sh
go install github.com/4ier/notion-cli@latest
```

### npm
```sh
npm install -g notion-cli-go
```

### Scoop (Windows)
```powershell
scoop bucket add 4ier https://github.com/4ier/scoop-bucket
scoop install notion-cli
```

### Docker
```sh
docker run --rm -e NOTION_TOKEN ghcr.io/4ier/notion-cli search "meeting"
```

### Binary

Download from [GitHub Releases](https://github.com/4ier/notion-cli/releases) — available for Linux, macOS, and Windows (amd64/arm64).

## Quick Start

```sh
# Authenticate
notion auth login --token ntn_xxxxx

# Search your workspace
notion search "meeting notes"

# Query a database with filters
notion db query <db-id> --filter 'Status=Done' --sort 'Date:desc'

# Create a page in a database
notion page create <db-id> --db "Name=Weekly Review" "Status=Todo"

# Read page content as Markdown
notion block list <page-id> --depth 3 --md

# Append blocks from a Markdown file
notion block append <page-id> --file notes.md

# Raw API escape hatch
notion api GET /v1/users/me
```

## Commands

| Group | Commands | Description |
|-------|----------|-------------|
| **auth** | `login` `logout` `status` `doctor` | Authentication & diagnostics |
| **search** | `search` | Search pages and databases |
| **page** | `view` `list` `create` `delete` `restore` `move` `open` `set` `props` `link` `unlink` | Full page lifecycle |
| **db** | `list` `view` `query` `create` `update` `add` `add-bulk` `open` | Database CRUD + query |
| **block** | `list` `get` `append` `insert` `update` `delete` | Content block operations |
| **comment** | `list` `add` `get` | Discussion threads |
| **user** | `me` `list` `get` | Workspace members |
| **file** | `list` `upload` | File management |
| **api** | `<METHOD> <path>` | Raw API escape hatch |

**39 subcommands** covering 100% of the Notion API.

## Features

### Human-Friendly Filters
No JSON needed for 90% of queries:
```sh
notion db query <id> --filter 'Status=Done' --filter 'Priority=High' --sort 'Date:desc'
```

For complex queries (OR, nesting), use the JSON escape hatch:
```sh
notion db query <id> --filter-json '{"or":[{"property":"Status","status":{"equals":"Done"}},{"property":"Status","status":{"equals":"Cancelled"}}]}'
```

### Schema-Aware Properties
Property types are auto-detected from the database schema:
```sh
notion page create <db-id> --db "Name=Sprint Review" "Date=2026-03-01" "Points=8" "Done=true"
```

### Smart Output
- **Terminal**: Colored tables, formatted text
- **Pipe/Script**: Clean JSON for `jq`, scripts, and AI agents
```sh
# Pretty table in terminal
notion db query <id>

# JSON when piped
notion db query <id> | jq '.results[].properties.Name'
```

### Markdown I/O
```sh
# Read blocks as Markdown
notion block list <page-id> --md --depth 3

# Write Markdown to Notion
notion block append <page-id> --file document.md
```
Supports headings, bullets, numbered lists, todos, quotes, code blocks, and dividers.

### Recursive Block Reading
```sh
notion block list <page-id> --depth 5 --all
```

### URL or ID — Your Choice
```sh
# Both work
notion page view abc123def
notion page view https://notion.so/My-Page-abc123def456
```

### Actionable Error Messages
```
object_not_found: Could not find page with ID abc123
  → Check the ID is correct and the page/database is shared with your integration
```

## For AI Agents

This CLI is designed to be agent-friendly:
- **JSON output** when piped — no parsing needed
- **Schema-aware** — agents don't need to know property types
- **URL resolution** — paste Notion URLs directly
- **Single binary** — no runtime dependencies
- **Exit codes** — 0 for success, non-zero for errors

Install as an agent skill:
```sh
npx skills add 4ier/notion-cli
```

## Configuration

```sh
# Token is stored in ~/.config/notion-cli/config.json (mode 0600)
notion auth login --token ntn_xxxxx

# Or use environment variable
export NOTION_TOKEN=ntn_xxxxx

# Check authentication
notion auth status
notion auth doctor
```

## Contributing

Issues and PRs welcome at [github.com/4ier/notion-cli](https://github.com/4ier/notion-cli).

## License

[MIT](LICENSE)
