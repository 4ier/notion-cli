# Notion CLI 分发方案

## 一、安装渠道（按优先级排序）

### Tier 1 — 必须做（覆盖 80% 用户）

| 渠道 | 目标用户 | 实现方式 | 工作量 |
|------|---------|---------|--------|
| **GitHub Releases** | 所有平台 | goreleaser + GitHub Actions | 1h |
| **go install** | Go 开发者 | 已就绪（`go install github.com/4ier/notion-cli@latest`） | 0 |
| **Homebrew Tap** | macOS/Linux 开发者 | goreleaser 自动生成 formula → `4ier/homebrew-tap` | 30min |

### Tier 2 — 应该做（扩大覆盖）

| 渠道 | 目标用户 | 实现方式 | 工作量 |
|------|---------|---------|--------|
| **npm wrapper** | Node.js/agent 生态 | 轻量 npm 包 `@4ier/notion-cli`，postinstall 拉二进制 | 2h |
| **Docker** | CI/CD/自动化 | `ghcr.io/4ier/notion-cli` | 30min |
| **Scoop** | Windows | goreleaser 内置 scoop manifest | 15min |

### Tier 3 — 锦上添花（长尾）

| 渠道 | 目标用户 | 实现方式 | 工作量 |
|------|---------|---------|--------|
| **AUR** | Arch Linux | PKGBUILD | 30min |
| **Nix** | NixOS | flake.nix | 1h |
| **skills.sh** | AI agent | 已就绪 | 0 |

---

## 二、实现计划

### Phase 1: goreleaser + CI（今天）

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

**执行步骤：**
1. 创建 `.goreleaser.yaml`
2. 创建 `.github/workflows/release.yml` + `.github/workflows/test.yml`
3. 创建 `4ier/homebrew-tap` 和 `4ier/scoop-bucket` 仓库
4. 打 tag `v0.2.0`，推送触发自动发布
5. 验证: `brew install 4ier/tap/notion-cli`

### Phase 2: npm wrapper（本周）

```
notion-cli-npm/
├── package.json     # name: @4ier/notion-cli
├── install.js       # postinstall: 检测平台 → 下载对应 GitHub Release 二进制
├── bin/notion       # shell wrapper → 执行下载的二进制
└── README.md
```

用户体验: `npx @4ier/notion-cli search "meeting notes"`

### Phase 3: Docker（本周）

```dockerfile
FROM alpine:3.21
COPY notion /usr/local/bin/
ENTRYPOINT ["notion"]
```

goreleaser 内置 Docker 支持，一并配置。

---

## 三、推广渠道（按 ROI 排序）

### 高 ROI
| 渠道 | 策略 | 时机 |
|------|------|------|
| **r/Notion** (1.2M members) | "I built a CLI for Notion" 帖，demo GIF，链接 GitHub | v0.2.0 发布当天 |
| **Hacker News** | Show HN: Full Notion CLI — 38 commands | 同上，UTC 上午 |
| **X/Twitter** | 线程：问题→方案→demo→链接，@NotionHQ | 同上 |

### 中 ROI
| 渠道 | 策略 | 时机 |
|------|------|------|
| **r/commandline** | 侧重 CLI 设计哲学（gh 对标） | 发布 +1 天 |
| **Product Hunt** | 完整 launch page | 发布 +3 天 |
| **Dev.to / Hashnode** | 技术文章：Notion API → CLI 的设计决策 | 发布 +1 周 |

### 长尾
| 渠道 | 策略 | 时机 |
|------|------|------|
| **Notion 社区** (Discord/论坛) | 作为工具分享 | 持续 |
| **GitHub trending** | 靠 star 自然进入 | 有机增长 |
| **Awesome Notion** | 提 PR 加入列表 | v0.2.0 后 |

---

## 四、推广素材（需要准备）

1. **Demo GIF/视频** — 30 秒终端录屏，展示核心流程：
   - `notion auth login` → `notion search` → `notion db query --filter` → `notion page create`
   - 用 [vhs](https://github.com/charmbracelet/vhs) 或 asciinema 录制

2. **README 升级** — 加 badges、GIF、安装方式表格、对标竞品

3. **一句话 pitch**: "Like `gh` for GitHub, but for Notion. 39 commands. One binary."

4. **Twitter 线程**（已有 Notion page 可以改写）

---

## 五、时间线

| 日期 | 里程碑 |
|------|--------|
| 2/19 | goreleaser + CI + homebrew tap + scoop ✅ |
| 2/19 | 打 v0.2.0 tag，触发首次自动发布 |
| 2/19 | README 升级 + demo GIF 录制 |
| 2/20 | npm wrapper 发布 |
| 2/20 | r/Notion + HN + X 同步发帖 |
| 2/21 | Product Hunt 准备 |
| 2/22 | Docker image + Awesome Notion PR |
| 持续 | 根据反馈迭代，社区回复 |

---

## 六、成功指标

| 指标 | 1 周目标 | 1 月目标 |
|------|---------|---------|
| GitHub Stars | 50 | 300 |
| npm 周下载 | 20 | 100 |
| Homebrew 安装 | 10 | 50 |
| GitHub Issues | 5 | 20 |
| skills.sh 安装 | 10 | 50 |
