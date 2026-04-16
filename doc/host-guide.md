# 房主胎教手册 — 你 + 朋友联机的最快路径

> 慢慢看版。第一次开房 + 拉朋友进来的全流程。

## 推荐架构：独立服务器 + 4 个客户端

**从 v0.2 开始，推荐你开一个独立 `rontama serve` 终端做服务器，然后自己也另开一个 terminal 当客户端加入。** 这样你关自己 TUI 只是"断线"，服务器还在跑，bot 接管你的座位，你重开 TUI 输同一个名字就能续上。

旧的"Host LAN Game + You play seat 0"（在一个进程里既是服务器又是玩家）还留着，但你退就整个房间死。**不建议主局用。**

---

## Step 0 — 现状

- 仓库：<https://github.com/Minalinnski/RonTama>
- 你的局域网 IP：
  ```bash
  ipconfig getifaddr en0
  # 例：192.168.2.160
  ```
  换 WiFi 会变。

---

## Step 1 — 把二进制发给朋友

Cross-compile 好的文件在 `dist/` 下（如果没了，运行 `make-binaries.sh` 或手动 `GOOS=darwin GOARCH=arm64 go build -o dist/rontama-mac-arm64 ./cmd/rontama`）：

| 文件 | 给谁 |
|---|---|
| `dist/rontama-mac-arm64` | Apple Silicon Mac (M1/M2/M3/M4) |
| `dist/rontama-mac-intel` | Intel Mac |
| `dist/rontama-linux` | Linux x86_64 |

**发**：AirDrop / iMessage / 微信 / Dropbox / scp 都行。

**朋友收到后（macOS 一次性）**：
```bash
chmod +x ~/Downloads/rontama-mac-arm64
xattr -d com.apple.quarantine ~/Downloads/rontama-mac-arm64
mv ~/Downloads/rontama-mac-arm64 /usr/local/bin/rontama
rontama version
```

第二条是为了绕 macOS Gatekeeper。一次搞定。

---

## Step 2 — 你开房（推荐：纯服务器模式）

### Terminal A（服务器）

```bash
rontama
```
进 lobby：
1. ↓ 选 **Host LAN Game**
2. 字段配置：
   - **Rule**: `sichuan` 或 `riichi`
   - **Server only**: **选 Yes**（左/右键切）→ seat 0 也标成 Remote
   - **Seat 1/2/3**: 同理，都选 Remote（等 3 个朋友）或混 bot
   - **Wait**: 60s 左右（给朋友进入的时间）
3. `[ Start ]` → 进入"等待界面"，会显示：
   ```
   🀄  HOSTING — waiting for friends to join
   
   Rule:    riichi
   Seats:   seat0=Remote, seat1=Remote, seat2=Remote, seat3=Remote
   
   Tell friends:
     192.168.2.160:7777
   ```

**这个 terminal 不要关**。关了服务器死、所有人被踢。

### Terminal B（你作为客户端加入）

再开一个 terminal：
```bash
rontama
```
进 lobby：
1. 先去 `Edit your name` 改名字（默认 $USER，想改就改）
2. ↓ 选 **Join by IP address**
3. 输入 `127.0.0.1:7777`（本机）或 `192.168.2.160:7777` 都行
4. enter → 连服务器 → 进牌桌

**你现在是 seat 0（第一个加进来的），朋友 2-4 个人会补上 seat 1-3。**

---

## Step 3 — 朋友加入

每个朋友在他自己电脑上：
```bash
rontama
```
进 lobby：
1. `Edit your name` → 输自己名字
2. 选 `Join LAN Game`（mDNS 自动）或 `Join by IP address` 手输 `192.168.2.160:7777`
3. enter → 连上，占剩下的 seat

4 个人全上了，服务器 terminal 开始分牌，所有客户端进入牌桌。

---

## Step 4 — 断线怎么办（新）

### 朋友 A 打到一半 Ctrl+C 退了

- 服务器检测到 A 断开
- A 的 seat 切成 **摸切 bot**（只把摸的牌打掉，不鸣不胡）
- 其他人的 TUI 里 A 的名字后面加 `(掉线)` 标记
- 游戏继续

### 朋友 A 想回来

- A 重新跑 `rontama` → 输**同一个名字** → `Join by IP address` → 同一个服务器
- 服务器看到名字匹配到 A 的掉线座位 → 把 A 重新接回来
- A 从现在这步开始恢复正常打

### 你（房主）自己断了

- 你在 Terminal B 的 TUI 关了/Ctrl+C
- Terminal A 的服务器**不受影响**
- 你重开 `rontama` → 同名 → Join → 回到同一个座位

### 服务器 terminal 挂了

那就整个房间死。重开得从头来。**所以不要关 Terminal A。**

---

## Step 5 — 多局对战（新）

- **川麻**：默认 1 局就结算（血战到底本来就是单局制）
- **日麻**：默认 **4 局东风战 (東風戦)**，带连庄（庄家赢继续庄，非庄家赢下家做庄）

所有 4 局打完才算一个 match 结束，最终积分定胜负。Riichi 起始分每人 25000，最后看谁高。

---

## 操作（TUI 内）

| 按键 | 行为 |
|---|---|
| `←/→` 或 `h/l` | 选择手牌 |
| `1-9 / a-e` | 数字/字母直接跳到对应位置的牌 |
| `space / enter` | 打出选中的牌 |
| `t` | 自摸（只在真能胡的时候出现 `t=自摸` 提示） |
| `r` | (日麻) 立直（听牌时把选中的牌作为立直宣告牌打出） |
| `m / p / s` | (川麻) 选缺：萬 / 筒 / 索 |
| `1-9/a-e` toggle | (川麻) 换三张时多选 3 张同色 |
| `r / k / p / c / n` | 鸣牌：Ron / Kan / Pon / Chi / Pass |
| `q / esc / Ctrl+C` | 退出（=掉线，可重连） |

当前轮到打牌的玩家：panel 边框变青色 + 名字前 `●`。立直过：名字后 `立`。掉线：名字后 `(掉线)`。

---

## Step 6 — 翻车手册

| 问题 | 怎么办 |
|---|---|
| 朋友 `Join LAN Game` 看不到服务器 | mDNS 被拦。用 `Join by IP address` 手输 `192.168.2.160:7777` |
| 朋友的 Mac 跑报 "killed" / "无法打开" | Gatekeeper。回 Step 1 看 `xattr` 命令 |
| 你家 IP 变了（换了 WiFi） | 重启服务器 terminal，banner 上会显示新 IP，重新发给朋友 |
| 朋友跨了 WiFi（你家 WiFi vs 他手机热点） | 局域网不通。要么同 WiFi，要么 [Tailscale](https://tailscale.com/) 给你和朋友分配 100.x.x.x，然后 Join 用那个 |
| 有人"消失"了一整局，bot 一直在摸切 | 名字没对上，重连匹配不到。让他确认 lobby 里 `Edit your name` 填的跟之前 Register 的完全一样 |
| 发现同名两个人 | 第一个先连上的先认；第二个被拒 `no seat waiting for this name`。让其中一个改名 |
| 服务器 terminal 被 Ctrl+C | 整桌结束。没有 session 持久化，重开要重新来 |

---

## TL;DR — 3 个 terminal

```bash
# Terminal 1 — 服务器（整局期间不要关）
rontama
# → Host LAN Game → Server only: Yes → Start → 念 banner 里的 IP

# Terminal 2 — 你作为玩家
rontama
# → Edit your name → Join by IP address → 127.0.0.1:7777

# Terminal 3+ — 朋友（他们自己开）
rontama
# → Edit your name → Join LAN Game 或 Join by IP
```

全员进来后，服务器自动开打。中途任何人退 = 掉线，bot 顶；同名 Join 回来 = 恢复。

---

## 局限（目前还没做的）

- **跨 WiFi / 公网**: 只支持同 LAN。用 Tailscale 绕。
- **房间持久化**: 服务器 terminal 一关就全没。短期内会加 `save-session` / `resume` 让服务器重启也能续局。
- **昵称唯一性**: 同名冲突直接拒第二个。将来可能加 token-based 会话。
- **断线重连时的 in-flight 状态同步**: 重连后会收到下一次 StateUpdate，但如果正好在某个 Ask 中间断的，那一瞬间的动作已经被 tsumogiri 处理了。影响不大但会察觉。
- **日麻**: dead wall 暂时没切，dora indicator 由 server 内部管理。honba 计数了但奖金加成还没接到结算上。

全都在 `CLAUDE.md` 里有记录。
