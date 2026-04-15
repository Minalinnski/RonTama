# RonTama 🀄

终端里就能玩的麻将。**川麻（血战到底）+ 日麻（リーチ）**，支持局域网联机，bot 分难度。

> Status: **Phase 0 — Foundation**（脚手架阶段，还不能玩）

## 路线图

- [x] **Phase 0** — Foundation（Go module + Bubble Tea hello + CI）
- [ ] **Phase 1** — 牌 / 手牌 / 向听数
- [ ] **Phase 2** — 川麻规则引擎 + 单进程对战
- [ ] **Phase 3** — Bot 三档难度（Easy 贪心 / Medium +EV / Hard +防守）
- [ ] **Phase 4** — Bubble Tea TUI
- [ ] **Phase 5** — 局域网 client/server + mDNS 发现
- [ ] **Phase 6** — 日麻规则 + 役种算番
- [ ] **Phase 7** — Bot 强化（可选接 Mortal）
- [ ] **Phase 8** — Release & Homebrew tap

详细规划见 `docs/` 或开发者本地的 plan 文件。

## 开发

需要 Go 1.26+。

```bash
git clone https://github.com/Minalinnski/RonTama.git
cd RonTama
go mod tidy
go build -o rontama ./cmd/rontama
./rontama          # 当前是 Phase 0 占位 TUI，按 q 退出
```

测试 / 静态检查：

```bash
go vet ./...
go test ./...
```

## 安装（朋友视角，未来）

> 还没发版。Phase 8 完成后会支持：
>
> ```bash
> brew install Minalinnski/tap/rontama
> # 或
> curl -L https://github.com/Minalinnski/RonTama/releases/latest/download/rontama_darwin_arm64.tar.gz | tar xz
> ```

## 联机（未来）

```bash
rontama serve              # 开服务端
rontama join               # 同 WiFi mDNS 自动发现
rontama join 192.168.1.5   # 或手动 IP
```

## License

MIT。详见 [LICENSE](./LICENSE)。
