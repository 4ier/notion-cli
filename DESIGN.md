# Notion CLI — Design Document

> Build something worthy of standing next to `gh`.

## Philosophy

1. **Commands map to concepts, not API endpoints.** Users think in pages, databases, blocks — not HTTP verbs.
2. **Sensible defaults, full control.** Simple operations should be one-liners. Complex operations should be possible.
3. **Two audiences:** Humans (pretty output, interactive prompts) and Agents/Scripts (JSON output, piping, zero interactivity).
4. **Offline-first thinking.** Cache what makes sense. Don't hit the API when you don't have to.

## Name

`notion` — short, obvious, no ambiguity. Installable as a single binary.

## Authentication

```
notion auth login              # Interactive: open browser → OAuth flow
notion auth login --token      # Paste an integration token directly
notion auth logout
notion auth status             # Show current auth state
notion auth switch             # Switch between multiple workspaces
```

Store tokens in OS keychain (like `gh`) with fallback to `~/.config/notion/credentials.json`. Also support `NOTION_TOKEN` env var for CI/agent use.

## Command Structure

### Core Resources

```
notion page       # Work with pages
notion db         # Work with databases
notion block      # Work with content blocks
notion search     # Search across workspace
notion user       # User information
notion comment    # Comments on pages/blocks
notion file       # File uploads
```

### `notion page` — Work with Pages

```
notion page view <page-id|url>              # Display page content (rendered markdown)
notion page create <parent-id> --title "X"  # Create page under parent
notion page edit <page-id>                  # Open in $EDITOR (markdown round-trip)
notion page delete <page-id>                # Archive (soft delete)
notion page move <page-id> --to <parent>    # Move page to new parent
notion page list <parent-id>                # List child pages
notion page props <page-id>                 # Show page properties
notion page set <page-id> <prop>=<value>    # Set a property value
notion page open <page-id|url>              # Open in browser
```

**Design notes:**
- `view` renders page blocks as readable markdown in terminal
- `edit` downloads as markdown, opens editor, diffs, patches back
- Accepts both UUID and full Notion URLs (auto-extract ID)
- `--format json` on any command for machine-readable output

### `notion db` — Work with Databases

```
notion db view <db-id|url>                  # Show database schema
notion db query <db-id> [--filter ...] [--sort ...]  # Query rows
notion db create <parent-id> --title "X"    # Create database
notion db update <db-id>                    # Update database schema
notion db add <db-id> <prop=value ...>      # Add a row (page with properties)
notion db list                              # List accessible databases
notion db export <db-id> [--format csv|json|md]  # Export database
notion db open <db-id|url>                  # Open in browser
```

**Filter syntax (inspired by `jq`/`gh`):**
```
notion db query <id> --filter 'Status=Done' --filter 'Priority=High'
notion db query <id> --filter 'Date>=2026-01-01' --sort 'Date:desc'
notion db query <id> --filter 'Tags~=backend' --limit 10
```

Operators: `=` `!=` `>=` `<=` `>` `<` `~=` (contains) `!~=` (not contains) `?=` (is_empty) `!?=` (is_not_empty)

### `notion block` — Work with Content Blocks

```
notion block list <parent-id>               # List child blocks
notion block get <block-id>                 # Get a specific block
notion block append <parent-id> [--type paragraph] "text"  # Add block
notion block delete <block-id>              # Delete a block
notion block move <block-id> --after <id>   # Reposition block
```

**Block types for append:** `paragraph`, `heading1`, `heading2`, `heading3`, `todo`, `bullet`, `numbered`, `toggle`, `code`, `quote`, `divider`, `callout`, `image`, `bookmark`

**Pipe-friendly:**
```
echo "Hello world" | notion block append <page-id>
cat notes.md | notion block append <page-id> --format markdown
```

### `notion search` — Search

```
notion search "query"                       # Search pages and databases
notion search "query" --type page           # Only pages
notion search "query" --type database       # Only databases
notion search "query" --sort last_edited    # Sort by edit time
```

### `notion comment` — Comments

```
notion comment list <page-id>               # List comments on page
notion comment add <page-id> "text"         # Add comment
notion comment reply <comment-id> "text"    # Reply to comment
```

### `notion user` — Users

```
notion user me                              # Current bot user info
notion user list                            # List workspace users
notion user get <user-id>                   # Get user details
```

### `notion file` — File Uploads

```
notion file upload <file-path> [--to <page-id>]  # Upload file
notion file list                            # List uploads
```

### `notion api` — Raw API Access (escape hatch)

```
notion api GET /v1/users/me                 # Raw API call
notion api POST /v1/search --body '{"query":"x"}'
```

Like `gh api` — for anything the CLI doesn't cover yet.

## Global Flags

```
--format json|table|text|md    # Output format (default: auto-detect tty)
--workspace <name>             # Use specific workspace
--token <token>                # Override auth token
--no-cache                     # Skip local cache
--debug                        # Show HTTP requests/responses
--quiet                        # Minimal output
--yes                          # Skip confirmations
```

## Output Design

### Human mode (tty detected):
```
$ notion page view abc123
📄 Project Roadmap
━━━━━━━━━━━━━━━━━
Last edited: 2 hours ago by @alice

## Phase 1
- [x] Design review
- [ ] Implementation
- [ ] Testing
```

### Agent/Script mode (--format json or piped):
```json
{
  "id": "abc123",
  "title": "Project Roadmap",
  "last_edited": "2026-02-18T12:00:00Z",
  "blocks": [...]
}
```

## MVP Scope (v0.1.0)

### Must have:
1. `notion auth login --token` + `NOTION_TOKEN` env var
2. `notion search`
3. `notion page view` + `notion page list`
4. `notion page create` (simple: title + optional text body)
5. `notion db query` (with basic filter/sort)
6. `notion db list`
7. `notion block list` + `notion block append`
8. `notion api` (raw escape hatch)
9. JSON + table + text output formats
10. URL-to-ID auto-parsing

### Post-MVP:
- `notion page edit` (markdown round-trip)
- `notion db add` (property-typed row creation)
- `notion db export`
- `notion comment` commands
- `notion file upload`
- `notion page move`
- OS keychain auth storage
- OAuth flow (for public integrations)
- Shell completions
- Local caching

## Notion API → CLI Mapping

| API Endpoint | CLI Command |
|---|---|
| POST /v1/search | `notion search` |
| GET /v1/pages/{id} | `notion page view <id>` |
| POST /v1/pages | `notion page create` |
| PATCH /v1/pages/{id} | `notion page set` |
| POST /v1/pages/{id}/move | `notion page move` |
| GET /v1/pages/{id}/properties/{pid} | `notion page props` |
| GET /v1/databases/{id} | `notion db view` |
| POST /v1/databases | `notion db create` |
| PATCH /v1/databases/{id} | `notion db update` |
| POST /v1/data_sources/{id}/query | `notion db query` |
| GET /v1/blocks/{id} | `notion block get` |
| GET /v1/blocks/{id}/children | `notion block list` |
| PATCH /v1/blocks/{id}/children | `notion block append` |
| PATCH /v1/blocks/{id} | `notion block update` |
| DELETE /v1/blocks/{id} | `notion block delete` |
| GET /v1/comments | `notion comment list` |
| POST /v1/comments | `notion comment add` |
| GET /v1/users | `notion user list` |
| GET /v1/users/me | `notion user me` |
| GET /v1/users/{id} | `notion user get` |
| POST /v1/file_uploads | `notion file upload` |
| * | `notion api <method> <path>` |

## Project Structure (Go)

```
notion-cli/
├── cmd/
│   ├── root.go            # Root command, global flags
│   ├── auth.go            # auth subcommands
│   ├── page.go            # page subcommands
│   ├── db.go              # db subcommands
│   ├── block.go           # block subcommands
│   ├── search.go          # search command
│   ├── comment.go         # comment subcommands
│   ├── user.go            # user subcommands
│   ├── file.go            # file subcommands
│   └── api.go             # raw api command
├── internal/
│   ├── client/            # Notion API client
│   │   ├── client.go      # HTTP client, auth, retry
│   │   ├── pages.go       # Page operations
│   │   ├── databases.go   # Database operations
│   │   ├── blocks.go      # Block operations
│   │   ├── search.go      # Search
│   │   ├── users.go       # User operations
│   │   └── comments.go    # Comment operations
│   ├── render/            # Output formatting
│   │   ├── json.go        # JSON output
│   │   ├── table.go       # Table output
│   │   ├── markdown.go    # Markdown rendering
│   │   └── text.go        # Plain text
│   ├── config/            # Config & auth storage
│   │   └── config.go
│   └── util/
│       ├── url.go         # URL/ID parsing
│       └── pagination.go  # Auto-pagination helper
├── main.go
├── go.mod
├── go.sum
├── Makefile
├── README.md
└── LICENSE
```

## Tech Choices

- **CLI framework:** [cobra](https://github.com/spf13/cobra) — same as `gh`, industry standard
- **HTTP client:** `net/http` + thin wrapper (no need for heavy SDK)
- **JSON:** `encoding/json` standard lib
- **Table output:** [tablewriter](https://github.com/olekukonez/tablewriter) or custom
- **Color/styling:** [lipgloss](https://github.com/charmbracelet/lipgloss) or [color](https://github.com/fatih/color)
- **Config:** XDG-compliant, `~/.config/notion-cli/`
- **Build:** goreleaser for cross-platform binaries + homebrew tap

## Quality Bar

This must feel like a first-party tool. That means:
- Every error message is helpful (not just "request failed")
- Tab completion works
- Help text is clear and has examples
- Performance feels instant (<200ms for cached, <1s for API)
- Works in CI/CD pipelines without interactivity
- Works as an agent tool without human input
