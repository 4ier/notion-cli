# CHANGELOG

All Git commit history is recorded in reverse chronological order. New commits are added at the top of the table.

| Date | Type | Scope | Change (with purpose) |
|---|---|---|---|
| 2026-06-05 14:19 | fix | util | Support extracting page ID from `app.notion.com/p/...` copied URLs — improved `<page-id\|url>` input compatibility (#57) |
| 2026-04-30 14:20 | feat | page | Add `page markdown` (GET /v1/pages/:id/markdown) + `page set-markdown` (PATCH, 4 modes: replace/append/after/range) — Notion server-side markdown I/O as first-class citizen (#37) |
| 2026-04-30 14:20 | feat | page | Add `page property`: GET /v1/pages/:id/properties/:id with auto-pagination — fix silent truncation for relation/rollup/rich_text exceeding 25 items (#38) |
| 2026-04-30 14:20 | feat | block | Add `block update --file <md>` / `--markdown` support — consistent markdown experience with append/insert, fail-fast on block type mismatch (#36) |
| 2026-04-30 14:20 | feat | comment | Add `comment update` (PATCH) + `comment delete` (DELETE) — wrap 2025 API new endpoints (#33) |
| 2026-04-30 14:20 | feat | page | Promote `page archive` / `page trash` as canonical, keep `page delete` as alias — clarify soft-delete semantics (#35) |
| 2026-04-30 14:20 | feat | file | Add `file get <upload-id>`: wrap GET /v1/file_uploads/:id, check upload status/URL (#34) |
| 2026-04-30 13:00 | feat | auth,client | Expose integration type in `auth status`/`doctor` + add concrete workaround hint for workspace-root creation error (#25) |
| 2026-04-30 13:00 | feat | file | Extend `notion file upload` to accept stdin(`-`) and http(s) URL sources, add `--name` override flag (#26) |
| 2026-04-30 13:00 | feat | block | Add `--image-file`/`--image-upload` and file/video/audio/pdf media flag family — one-command upload-embed workflow (#23) |
| 2026-04-30 13:00 | feat | block | Auto-batch children.length>100 + auto-split rich_text exceeding 2000 chars at newline boundaries in code blocks, add `--on-oversize=split\|truncate\|fail` flag (#21) |
| 2026-04-30 13:00 | feat | block | Auto-normalize markdown code fence language aliases (ts/sh/yml/py/…) to Notion enums, warn and fallback to plain text for unrecognized (#22) |
| 2026-04-30 13:00 | fix | api | Auto-correct `/v1` prefix, support `--body @file` / `--body -`, align help examples with actual behavior (#24) |
| 2026-03-14 21:00 | chore | gitignore | Add notion.exe binary to gitignore — prevent tracking Windows build artifacts |
| 2026-03-14 20:10 | fix | block | Move table_row children into table{} — comply with Notion API spec (fix 'table.children should be defined' error) |
| 2026-03-14 19:52 | feat | block | Add GFM table parsing + inline formatting (bold/italic/code/link/strike) support — fix broken markdown table uploads to Notion CLI |
