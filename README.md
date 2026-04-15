# RonTama 🀄

终端里就能玩的麻将。**川麻（血战到底）+ 日麻（リーチ）**，支持局域网联机，bot 分难度。

> Status: Phase 0–6 done + Phase 8 release pipeline. 单机/联机/规则/bot 都跑通了，可玩。

## 路线图

- [x] **Phase 0** — Foundation（Go module + Bubble Tea hello + CI）
- [x] **Phase 1** — 牌 / 手牌 / 向听数
- [x] **Phase 2** — 川麻规则引擎 + 单进程对战
- [x] **Phase 3** — Bot 三档难度（Easy 贪心 / Medium +EV / Hard +防守）
- [x] **Phase 4** — Bubble Tea TUI（人 + 3 bot 单局可玩）
- [x] **Phase 5** — 局域网 client/server + mDNS 发现
- [x] **Phase 6** — 日麻规则 + 役种算番（MVP，仍缺平和等少数役）
- [ ] **Phase 7** — Bot 强化（可选接 Mortal）
- [x] **Phase 8** — Release & GoReleaser 多平台二进制

详细规划见 `~/.claude/plans/` 下 plan 文件，限制 / 已知坑见 `CLAUDE.md`。

## 安装（朋友视角）

### 方式 A — 下载 Release 二进制（推荐）

发版 tag 后在 [Releases](https://github.com/Minalinnski/RonTama/releases) 下载对应平台压缩包：

```bash
# Apple Silicon Mac
curl -L https://github.com/Minalinnski/RonTama/releases/latest/download/rontama_*_darwin_arm64.tar.gz | tar xz
./rontama --help

# Intel Mac
curl -L https://github.com/Minalinnski/RonTama/releases/latest/download/rontama_*_darwin_amd64.tar.gz | tar xz

# Linux x86_64
curl -L https://github.com/Minalinnski/RonTama/releases/latest/download/rontama_*_linux_amd64.tar.gz | tar xz
```

**macOS Gatekeeper 提示**：第一次跑会被拦，执行：

```bash
xattr -d com.apple.quarantine ./rontama
```

或在 Finder 里右键 → 打开 → 仍要打开（一次性）。

### 方式 B — 源码编译

需要 Go 1.26+。

```bash
git clone https://github.com/Minalinnski/RonTama.git
cd RonTama
go install ./cmd/rontama
rontama --help
```

### 方式 C — Homebrew tap（待启用）

> 等 `Minalinnski/homebrew-tap` 仓建好以后启用 `.goreleaser.yaml` 里的 `brews:` 配置。届时一行装：
>
> ```bash
> brew install Minalinnski/tap/rontama
> ```

## 玩法

```bash
# 单机：你 + 3 bot 川麻 TUI
rontama play -tui

# 单机：4 bot 川麻自动对战，纯文字输出
rontama play -rounds 10

# 单机：日麻 4 bot
rontama play -rule riichi -rounds 5

# bot 难度对战统计
rontama botbattle -rounds 1000 -seats easy,easy,medium,hard

# 局域网开服（mDNS 自动广播）
rontama serve

# 局域网加入（mDNS 自动发现，找到第一个 server 就连）
rontama join

# 局域网加入（手动 IP）
rontama join -addr 192.168.1.5:7777
```

### 键盘操作（TUI 内）

| 按键 | 行为 |
|---|---|
| ←/→ 或 h/l | 选择手牌 |
| space / enter | 打出选中的牌 |
| t | 自摸 |
| m / p / s | 选择 缺 (Sichuan 开局) |
| r / p / k / n | 鸣牌：Ron / Pon / Kan / Pass |
| q / esc / Ctrl+C | 退出 |

## 开发

```bash
go mod tidy
go vet ./...
go test ./... -race
go build -o rontama ./cmd/rontama
./rontama play -rounds 100  # 跑 100 局 4-bot 川麻看一下
```

CI 在每次 push / PR 上跑 vet + race tests + build（matrix: ubuntu + macOS, Go 1.26）。

## 发版

```bash
git tag v0.1.0
git push origin v0.1.0
```

GitHub Actions 触发 GoReleaser，自动构建 darwin/linux/windows × arm64/amd64 二进制 + checksum，并发布到 Releases 页面。

## License

MIT。详见 [LICENSE](./LICENSE)。
