# Notion CLI Distribution Plan

## 1. Installation Channels (by priority)

### Tier 1 — Must-do (covers 80% of users)

| Channel | Target Users | Approach | Effort |
|------|---------|---------|--------|
| **GitHub Releases** | All platforms | goreleaser + GitHub Actions | 1h |
| **go install** | Go developers | Already ready (`go install github.com/4ier/notion-cli@latest`) | 0 |
| **Homebrew Tap** | macOS/Linux developers | goreleaser auto-generate formula → `4ier/homebrew-tap` | 30min |

### Tier 2 — Should-do (expand coverage)

| Channel | Target Users | Approach | Effort |
|------|---------|---------|--------|
| **npm wrapper** | Node.js/agent ecosystem | Lightweight npm package `@4ier/notion-cli`, postinstall fetches binary | 2h |
| **Docker** | CI/CD/automation | `ghcr.io/4ier/notion-cli` | 30min |
| **Scoop** | Windows | goreleaser built-in scoop manifest | 15min |

### Tier 3 — Nice-to-have (long tail)

| Channel | Target Users | Approach | Effort |
|------|---------|---------|--------|
| **AUR** | Arch Linux | PKGBUILD | 30min |
| **Nix** | NixOS | flake.nix | 1h |
| **skills.sh** | AI agent | Already ready | 0 |

---

## 2. Implementation Plan

### Phase 1: goreleaser + CI (today)

```
.goreleaser.yaml
├── builds: linux/darwin/windows × amd64/arm64
├── archives: tar.gz (unix) / zip (windows)
├── checksum: SHA256
├── homebrew_formulas: 4ier/homebrew-tap
├── scoop: 4ier/scoop-bucket
└── changelog: auto from git

.github/workflows/release.yml
├── on: push tags v*
├── setup-go
├── goreleaser-action
└── GITHUB_TOKEN (auto)
```

**Steps:**
1. Create `.goreleaser.yaml`
2. Create `.github/workflows/release.yml` + `.github/workflows/test.yml`
3. Create `4ier/homebrew-tap` and `4ier/scoop-bucket` repos
4. Tag `v0.2.0`, push to trigger automated release
5. Verify: `brew install 4ier/tap/notion-cli`

### Phase 2: npm wrapper (this week)

```
notion-cli-npm/
├── package.json     # name: @4ier/notion-cli
├── install.js       # postinstall: detect platform → download corresponding GitHub Release binary
├── bin/notion       # shell wrapper → execute downloaded binary
└── README.md
```

User experience: `npx @4ier/notion-cli search "meeting notes"`

### Phase 3: Docker (this week)

```dockerfile
FROM alpine:3.21
COPY notion /usr/local/bin/
ENTRYPOINT ["notion"]
```

goreleaser has built-in Docker support, configure together.

---

## 3. Distribution Channels (by ROI)

### High ROI
| Channel | Strategy | Timing |
|------|------|------|
| **r/Notion** (1.2M members) | "I built a CLI for Notion" post, demo GIF, link to GitHub | v0.2.0 launch day |
| **Hacker News** | Show HN: Full Notion CLI — 38 commands | Same day, UTC morning |
| **X/Twitter** | Thread: problem → solution → demo → link, @NotionHQ | Same day |

### Medium ROI
| Channel | Strategy | Timing |
|------|------|------|
| **r/commandline** | Focus on CLI design philosophy (gh parity) | Launch +1 day |
| **Product Hunt** | Full launch page | Launch +3 days |
| **Dev.to / Hashnode** | Technical article: Notion API → CLI design decisions | Launch +1 week |

### Long tail
| Channel | Strategy | Timing |
|------|------|------|
| **Notion Community** (Discord/forums) | Share as a tool | Ongoing |
| **GitHub trending** | Organic growth via stars | Organic |
| **Awesome Notion** | Submit PR to be listed | After v0.2.0 |

---

## 4. Launch Materials (to prepare)

1. **Demo GIF/video** — 30-second terminal recording showing core flow:
   - `notion auth login` → `notion search` → `notion db query --filter` → `notion page create`
   - Record with [vhs](https://github.com/charmbracelet/vhs) or asciinema

2. **README upgrade** — Add badges, GIF, installation methods table, comparison with alternatives

3. **One-liner pitch**: "Like `gh` for GitHub, but for Notion. 39 commands. One binary."

4. **Twitter thread** (existing Notion page can be adapted)

---

## 5. Timeline

| Date | Milestone |
|------|--------|
| 2/19 | goreleaser + CI + homebrew tap + scoop ✅ |
| 2/19 | Tag v0.2.0, trigger first automated release |
| 2/19 | README upgrade + demo GIF recording |
| 2/20 | npm wrapper release |
| 2/20 | r/Notion + HN + X simultaneous posts |
| 2/21 | Product Hunt preparation |
| 2/22 | Docker image + Awesome Notion PR |
| Ongoing | Iterate based on feedback, community replies |

## 6. Success Metrics

| Metric | 1 Week Target | 1 Month Target |
|------|---------|---------|
| GitHub Stars | 50 | 300 |
| npm Weekly Downloads | 20 | 100 |
| Homebrew Installs | 10 | 50 |
| GitHub Issues | 5 | 20 |
| skills.sh Installs | 10 | 50 |
