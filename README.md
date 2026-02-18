# notion-cli

Work seamlessly with Notion from the command line.

`notion` is a CLI tool for the [Notion API](https://developers.notion.com/). It lets you manage pages, databases, blocks, comments, users, and files without leaving your terminal. Built for developers and AI agents.

## Install

### From source (requires Go 1.21+)

```bash
go install github.com/4ier/notion-cli@latest
```

### Binary releases

Download from the [Releases](https://github.com/4ier/notion-cli/releases) page.

## Authentication

Create an [internal integration](https://www.notion.so/my-integrations) and grab the token.

```bash
# Save your token
notion auth login --token ntn_xxxxxxxxxxxxx

# Or use an environment variable
export NOTION_TOKEN=ntn_xxxxxxxxxxxxx

# Verify
notion auth status
```

## Quick Start

```bash
# Search your workspace
notion search "meeting notes"

# View a page
notion page view <page-id>

# List databases
notion db list

# Query a database with filters
notion db query <db-id> --filter 'Status=Done' --sort 'Date:desc'

# Add a row to a database
notion db add <db-id> "Name=My Task" "Status=Todo" "Priority=High"

# Append content to a page
notion block append <page-id> "Hello from the CLI"
```

## Commands

### Pages

```
notion page view <id>           View a page's content
notion page list                List pages in the workspace
notion page create <parent-id>  Create a new page (--title, --body)
notion page delete <id>         Archive a page
notion page move <id> --to <id> Move a page to a new parent
notion page open <id>           Open in browser
notion page set <id> Key=Value  Set page properties
notion page props <id>          Show page properties
```

### Databases

```
notion db list                  List accessible databases
notion db view <id>             Show database schema
notion db query <id>            Query with --filter and --sort
notion db create <parent-id>    Create a database (--title, --props)
notion db update <id>           Update title or add properties
notion db add <id> Key=Value    Add a row to a database
notion db open <id>             Open in browser
```

#### Filter Syntax

Filters use `property operator value` syntax:

| Operator | Meaning |
|----------|---------|
| `=`      | equals |
| `!=`     | not equals |
| `>`      | greater than |
| `>=`     | greater than or equal |
| `<`      | less than |
| `<=`     | less than or equal |
| `~=`     | contains |

Multiple filters are combined with AND:

```bash
notion db query <id> --filter 'Status=Done' --filter 'Priority=High'
```

Property types are auto-detected from the database schema.

#### Sort Syntax

```bash
notion db query <id> --sort 'Date:desc' --sort 'Name:asc'
```

### Blocks

```
notion block list <parent-id>   List child blocks
notion block get <id>           Get a specific block
notion block append <id> "text" Append content (--type: paragraph, h1, h2, h3, bullet, numbered, todo, quote, code, callout, divider)
notion block update <id>        Update block content (--text)
notion block delete <id>        Delete a block
```

### Comments

```
notion comment list <page-id>   List comments on a page
notion comment add <page-id> "text"  Add a comment
```

### Users

```
notion user me                  Show current bot user
notion user list                List workspace users
notion user get <id>            Get user details
```

### Files

```
notion file list                List file uploads
notion file upload <path>       Upload a file
```

### Search

```
notion search "query"           Search pages and databases
notion search "query" --type page     Filter by type
notion search "query" --type database
```

### Raw API Access

For any endpoint not covered by a dedicated command:

```bash
notion api GET /v1/users/me
notion api POST /v1/search '{"query":"test"}'
notion api PATCH /v1/pages/<id> '{"archived":true}'
```

## Global Flags

| Flag | Description |
|------|-------------|
| `--format json` | Output as JSON (auto-detected when piped) |
| `--format table` | Force table output |
| `--debug` | Show HTTP request/response details |
| `--version` | Show version |

## Output

- **Terminal**: Pretty-printed with colors and tables
- **Piped/scripted**: JSON by default (auto-detected)
- **Explicit**: `--format json` or `--format table`

## For AI Agents

`notion` is designed to work well with AI agents:

- All commands support `--format json` for structured output
- Auto-detects non-TTY and outputs JSON
- Full IDs in all output (no truncation)
- Accepts both Notion page IDs and full URLs
- `notion api` as an escape hatch for any Notion API endpoint

## License

MIT

## Links

- [Notion API Documentation](https://developers.notion.com/)
- [Create an Integration](https://www.notion.so/my-integrations)
