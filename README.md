# RonTama 🀄

终端里就能玩的麻将。**川麻 (血战到底) + 日麻 (リーチ)**，支持局域网联机，bot 分难度。

```
$ rontama
```

— 一行就开局。Lobby 里自己选模式：本地单人、开房等朋友、或者加入朋友的房。

---

## 安装

> **Release 二进制目前还没发布**（`v0.x` tag 还没打）。下面两条路二选一。

### 路线 A — 源码编译（目前推荐）

需要 [Go 1.26+](https://go.dev/dl/)。

```bash
git clone https://github.com/Minalinnski/RonTama.git
cd RonTama
go install ./cmd/rontama
```

`go install` 会把 `rontama` 装进 `$(go env GOBIN)`（默认 `$HOME/go/bin`）。
确认 PATH 里有它：

```bash
which rontama  # /Users/yourname/go/bin/rontama
rontama version
```

### 路线 B — 给朋友打包二进制

如果朋友没装 Go，自己 build 一份给他：

```bash
# 在你机器上为对方平台 build
GOOS=darwin GOARCH=arm64 go build -o rontama-darwin-arm64 ./cmd/rontama
GOOS=darwin GOARCH=amd64 go build -o rontama-darwin-amd64 ./cmd/rontama
GOOS=linux  GOARCH=amd64 go build -o rontama-linux-amd64  ./cmd/rontama

# scp / AirDrop / U 盘传给朋友
# 朋友执行：
chmod +x ./rontama-darwin-arm64
xattr -d com.apple.quarantine ./rontama-darwin-arm64  # macOS 第一次跑会被拦
./rontama-darwin-arm64
```

### 路线 C — Homebrew tap（待启用）

发版后会启用 `.goreleaser.yaml` 里的 `brews:` stanza，届时一行装：

```bash
brew install Minalinnski/tap/rontama
```

现在还没。等 `v0.1.0` tag。

---

## 一个人玩

```bash
rontama
```

→ Lobby 出来，选 `New Local Game`：
- Rule: `sichuan` 或 `riichi`（左右键切换）
- Seat 1/2/3: bot 难度（Easy/Medium/Hard）
- 你永远在 Seat 0
- 选中 `[ Start ]` → 进入牌桌

---

## 跟朋友联机玩 — 完整流程

### 谁起服务器？
**任意一人** 起服务器（"开房间"）。其他人加入。所有人都需要装好 `rontama`。

### 步骤

#### 主机方（开房的人）

1. 跑 `rontama`
2. Lobby 选 `Host LAN Game`
3. 配置每个 seat：
   - `Remote` = 等朋友加入
   - `Easy/Medium/Hard bot` = bot 占位
   - Seat 0 默认是你 (`You`)
4. 配置 `Wait`（等朋友超时；超时后空 Remote seat 用 Easy bot 补位）
5. `[ Start ]` → 进入"等待界面"，会显示：
   ```
   🀄 HOSTING — waiting for friends to join
   
   Tell friends:
     1. They run `rontama` → Join LAN Game (mDNS auto-discover), OR
     2. They run `rontama` → Join by IP address → type one of:
          192.168.1.5:7777
          10.0.0.42:7777
   ```
6. 把上面那两行 IP 念给朋友（或者让他们靠 mDNS 自动发现）
7. 朋友连上后，等待界面会自动消失，开打

#### 朋友方（加入的人）

**情况 1 — 同一个 WiFi（mDNS 能用）**：

1. 跑 `rontama`
2. Lobby 选 `Join LAN Game`
3. 等几秒，会列出 LAN 上的 RonTama 服务器
4. 用方向键选服务器 → enter → 连上

**情况 2 — mDNS 不工作（不同子网、企业 WiFi 拦截广播等）**：

1. 跑 `rontama`
2. Lobby 选 `Join by IP address`
3. 输入主机给的地址 (e.g. `192.168.1.5:7777` 或者只 `192.168.1.5`，默认端口 7777)
4. enter → 连上

### 跨网/公网怎么办？

目前**只支持局域网**。跨 WiFi / 公网需要：
- 主机做端口转发（路由器把 7777 端口映射到自己机器），或者
- 用 Tailscale / WireGuard 之类把所有人串到同一个 overlay 网络

此项目不内置中继。

---

## 操作（TUI 内）

| 按键 | 行为 |
|---|---|
| `←/→` 或 `h/l` | 选择手牌 |
| `1-9 / a-e` | 直接跳到对应位置的牌 |
| `space / enter` | 打出选中的牌 |
| `t` | 自摸 |
| `r` | (日麻) 立直宣告 (打出选中的牌作为立直宣告牌) |
| `m / p / s` | (川麻) 选择缺一门 |
| `1-9/a-e` toggle | (川麻) 换三张时多选 3 张同色 |
| `r / p / k / n` | 鸣牌：Ron / Pon / Kan / Pass |
| `q / esc / Ctrl+C` | 退出 |

颜色：万红、筒蓝、索绿、字白。当前轮到打牌的玩家，他的 panel 边框会变青色 + 名字前有 `●`。立直过的玩家名字后面会有 `立`。

---

## 当前支持的玩法 / 限制

**川麻 (血战到底)**：完整支持。换三张 + 定缺 + 不能吃 + 多人和 (一炮多响) + 血战到底（先胡的人退出，剩下的继续）。番型：平胡 / 大对子 / 七对 / 龙七对 / 清一色 + 清碰/清七对/清龙七对 复合，自摸/海底/抢杠加番。

**日麻**：MVP 完整可玩。完整 136 张 + 字牌；役种支持立直/一发/门前清自摸和/海底/河底/嶺上/槍槓/断幺九/役牌/一気通貫/三色同順/対々和/七対子/混一色/清一色/平和；役満支持 国士無双/四暗刻/大三元；庄家×6 闲家×4 标准结算 + 立直棒池。

**还没做**（影响不大但要知道）：
- 日麻的 dead wall 切分还是单一 wall（dora indicator 由 server 内部生成）
- 单局对战；没有连庄/南场/本场棒/跨局立直棒结转
- 鸣牌后的 "喰い替え" 约束没限制
- 一些少见役（三色同刻/小三元/三槓子等）

详细 TODO 见 [`CLAUDE.md`](./CLAUDE.md)。

---

## 开发

```bash
go mod tidy
go vet ./...
go test ./... -race
go build -o rontama ./cmd/rontama
```

CI: GitHub Actions 在 push/PR 跑 vet + race tests + build (matrix: ubuntu + macOS, Go 1.26)。

发版：

```bash
git tag v0.1.0
git push origin v0.1.0
```

GitHub Actions 触发 GoReleaser，自动 build darwin/linux/windows × arm64/amd64 二进制 + checksums，挂到 Releases 页面。

---

## License

MIT。详见 [LICENSE](./LICENSE)。
