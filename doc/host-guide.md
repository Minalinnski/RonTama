# 房主胎教手册 — 你 + 朋友联机的最快路径

> 慢慢看版。第一次开房 + 拉朋友进来的全流程。

## Step 0 — 你已经准备好了

- 仓库已 push 到 <https://github.com/Minalinnski/RonTama>（CI 跑在 <https://github.com/Minalinnski/RonTama/actions>）
- 你机器上已经有 `./rontama` 二进制（直接跑就行）
- 你的局域网 IP 用这条命令拿：
  ```bash
  ipconfig getifaddr en0
  # 例：192.168.2.160
  ```
  换 WiFi 后这个值会变，得重新拿。

---

## Step 1 — 把二进制发给朋友

我已经给三个平台 cross-compile 好了，全在 `dist/` 下：

| 文件 | 给谁 |
|---|---|
| `dist/rontama-mac-arm64` (~8 MB) | Apple Silicon Mac (M1/M2/M3/M4) |
| `dist/rontama-mac-intel` (~8.5 MB) | Intel Mac |
| `dist/rontama-linux`   (~8 MB) | Linux x86_64 |

**怎么发**：AirDrop 最快；也可以微信/iMessage 发文件、scp、Dropbox 等。

**朋友怎么判断自己的 Mac 是什么**：
左上角 → 关于本机 → 芯片栏。
- 写 "Apple M..." → arm64
- 写 "Intel" → intel

### 朋友收到后（macOS 一次性设置）

```bash
# 假设文件在 ~/Downloads/rontama-mac-arm64
chmod +x ~/Downloads/rontama-mac-arm64
xattr -d com.apple.quarantine ~/Downloads/rontama-mac-arm64

# 可选：放进 PATH，以后哪都能跑
mv ~/Downloads/rontama-mac-arm64 /usr/local/bin/rontama

# 验证
rontama version
# → rontama dev (none, built unknown)
```

第二条 `xattr` 是为了绕开 macOS Gatekeeper（未签名二进制第一次会被拦"无法验证开发者"）。这是一次性的。

如果朋友不想搬到 `/usr/local/bin`，直接在 Downloads 里 `./rontama-mac-arm64` 跑也行。

### 替代方案：朋友自己装 Go

如果朋友是技术宅且装了 [Go 1.26+](https://go.dev/dl/)，一行：

```bash
go install github.com/Minalinnski/RonTama/cmd/rontama@latest
```

不用你传文件。但要求他装 Go runtime。

---

## Step 2 — 你开房

```bash
cd /Users/williamgu/Documents/Github/personal/RonTama
./rontama
```

进 Lobby 后：

1. ↓ 选 **Host LAN Game (open a room for friends)** → enter
2. 进入配置表单，用方向键 / hjkl 移动焦点：
   - **Rule**: ←/→ 切 `sichuan` 或 `riichi`
   - **Seat 1 / 2 / 3**: ←/→ 切：
     - `Remote` — 等朋友远程加入
     - `Easy bot` / `Medium bot` / `Hard bot` — 本地 bot 占位（朋友不够人时凑数）
   - **Wait**: ←/→ 调 `5s` ~ `600s`，超时后空 `Remote` 座位会被 Easy bot 自动顶上（保证游戏一定能开始）
3. ↓ 移到 `[ Start ]` → enter

进入"等待界面"，看到类似：

```
🀄  HOSTING — waiting for friends to join

Rule:    sichuan
Seats:   seat0=You, seat1=Remote, seat2=Remote, seat3=Remote
Wait:    30s before any unfilled remote seats become bots

Tell friends:
  1. They run `rontama` → Join LAN Game (mDNS auto-discover), OR
  2. They run `rontama` → Join by IP address → type one of:
       192.168.2.160:7777
```

**把 `192.168.2.160:7777` 念给朋友** 或者发微信。

---

## Step 3 — 朋友加入

朋友在他自己电脑上：
```bash
rontama
```

进 Lobby：

### 同一个 WiFi 场景（mDNS 通常能用）

选 **Join LAN Game (auto-discover via mDNS)** → 等几秒 → 列表里出现你的服务器 → ↓ 选 → enter。

### mDNS 不通的场景

公司 WiFi 拦广播、不同子网、有些路由器禁了 mDNS — 这时候 "Join LAN Game" 会显示 "no servers found"。

让朋友选 **Join by IP address (manual)**：
- 输入你给的地址，比如 `192.168.2.160:7777`
- 也可以只输 `192.168.2.160`，默认端口 7777
- enter 连接

朋友连上后，你的"等待界面"自动消失 → 4 个 panel 摆好 → 开打。

---

## Step 4 — 真在打牌时怎么操作

桌面是 3×3 panel 网格，**你永远在底部** YOU 那个 panel。当前轮到打牌的玩家，他的 panel 边框会变青色 + 名字前出现 `●`。立直过的玩家名字后面会跟个 `立`。

### 你的回合（panel 边框变青时）

| 操作 | 按键 |
|---|---|
| 选牌（手牌横向，**摸的牌固定在最右**，不会被自动理进去） | `←/→` 或 `1-9 / a-e` |
| 打出选中的牌 | `space` 或 `enter` |
| 自摸 | `t` |
| 立直（Riichi 听牌时） | `r` |

`r` 在自己回合里 = 立直宣告（把选中的那张作为立直牌打出去）。条件：手牌门清 + 分数≥1000 + 牌山剩≥4 张 + 打这张后是听牌。不满足会直接拒绝（loop 校验）。

### 别人打牌后跳出的 Call 提示

| 操作 | 按键 |
|---|---|
| Ron / 胡 | `r` |
| Kan / 杠 | `k` |
| Pon / 碰 | `p` |
| Chi / 吃（仅 Riichi、仅下家） | `c` |
| Pass / 过 | `n` |

优先级：**Ron > Kan > Pon > Chi**。同一玩家如果同时能 Ron 和 Pon，按 `r` 取 Ron，按 `p` 取 Pon。

`r` 在 Call 上下文 = Ron；在自己回合 = 立直。同一个键，根据上下文不同含义。

### 川麻特殊

| 操作 | 按键 |
|---|---|
| 换三张选 3 张同色 | `1-9` toggle 选 / 取消，`space` 提交 |
| 选缺一门 | `m` 萬 / `p` 筒 / `s` 索 |

（川麻没有吃。开局有"换三张 → 定缺"两步。）

### 退出

`q` / `esc` / `Ctrl+C` — 任何时候都行。

---

## Step 5 — 翻车手册

| 问题 | 怎么办 |
|---|---|
| 朋友 `Join LAN Game` 看不到你的服务器 | mDNS 被网络拦了。让朋友走 `Join by IP address` 手输你的 IP |
| 朋友提示 `no servers found via mDNS` | 同上 |
| 朋友连了但桌面错位 / `YOU` 标在错位置 | 已知小限制：当前 TUI 假定客户端是 seat 0；多人客户端只有第一个加入的人 TUI 完全正确。第二个起加入的可以临时用 `rontama join -addr ... -bot` 走 headless bot 模式回避 |
| 你（房主）的 IP 变了 | 重连 WiFi、路由器分配新 IP 都会导致。重跑 `rontama` 重新看 banner 上的 IP |
| 朋友 Mac 跑 `./rontama-mac-arm64` 报 "killed" 或 "无法打开" | macOS Gatekeeper。回 Step 1 看 `xattr -d com.apple.quarantine ...` 命令 |
| 朋友跨了 WiFi（他用手机热点你用家里路由器） | 局域网根本不通，无解。要么所有人连同一个 WiFi，要么所有人装 [Tailscale](https://tailscale.com/)（免费、5 分钟搭好），把所有人放到同一个 overlay 网络，然后 join 时用 Tailscale 给你的 100.x.x.x 地址 |
| 大家都看到 "no wins (wall exhaustion)" | 牌墙抓完没人胡。Riichi 的 bot 因为不会鸣牌 + 不积极攻击，确实容易流局；川麻爆发率高很多 |
| 房主关了 / 网络断了 | 客户端会收到 EOF 或 RoundEnd 错误，TUI 显示 ERROR + press q to quit。重开一局 |

---

## TL;DR — 现在就能做的 3 行

```bash
# 1. AirDrop dist/rontama-mac-arm64 给朋友（或对应平台）

# 2. 朋友本地:
chmod +x rontama-mac-arm64
xattr -d com.apple.quarantine rontama-mac-arm64
./rontama-mac-arm64

# 3. 你本地（仓库目录里）:
./rontama
# Lobby → Host LAN Game → 留 3 个 Remote → Start
# → 念出 banner 里的 192.168.x.x:7777 给朋友
```

朋友在 lobby 里 `Join LAN Game` 自动找，找不到就 `Join by IP address` 手输。开打。

---

## 后续可改进的地方（如果朋友提需求）

- **多人 TUI 客户端**：现在第二个加入的人 TUI 显示假定他在 seat 0，会错位。`tui.HumanSeat` 改成 PlayModel 字段就修了。
- **跨网 / 公网**：内置中继可以加（WebRTC / TURN）。短期 workaround 用 Tailscale。
- **房间持续到打多个东风局 / 半庄**：当前一次开房只打 1 局，结束后程序退。改成 N 局 / 直到所有人 -25000 分需要 lobby + game loop 联动。
- **Web 客户端**：Bubble Tea + 一个浏览器渲染壳就能搞，不大，但需要新协议层。

这些都不是 MVP 必要，看朋友怎么用再说。
