# RonTama — 项目约定

给未来的 Claude（以及任何协作者）看，工程上的硬性约定。

## 项目背景

终端麻将游戏。规则：**川麻（血战到底）+ 日麻（リーチ）**。Bot 分 Easy/Medium/Hard 三档。局域网 4 人房，缺人 bot 补位。
完整路线见 `~/.claude/plans/` 下的最新 plan 文件，或 `README.md` 的 Roadmap 段。

## 技术栈

- **Go 1.26+**，module path `github.com/Minalinnski/RonTama`
- **TUI**：Bubble Tea + Lipgloss + Bubbles
- **网络**：标准库 `net` + `gorilla/websocket`，JSON 协议
- **mDNS**：`hashicorp/mdns`
- **CJK 宽度**：`mattn/go-runewidth`
- **随机**：`crypto/rand`（绝对不要 `math/rand` — 朋友局可重现的随机数会被喷）
- **日志**：`log/slog`
- **测试**：标准库 `testing`，table-driven

## 目录结构

```
cmd/rontama/             # CLI 入口
internal/
  tile/                  # 牌、手牌、wall、dora
  shanten/               # 向听数 + 有效进张（共用）
  rules/                 # RuleSet interface
    sichuan/             # 川麻
    riichi/              # 日麻
  game/                  # 状态机（规则无关）
  ai/                    # bot 接口 + easy/medium/hard
  net/                   # proto/server/client
  discovery/             # mDNS
  tui/                   # Bubble Tea
```

新代码默认放 `internal/`。`pkg/` 暂不需要。

## 架构原则

1. **Server 权威**：所有牌墙、未公开手牌只在 server 侧。client 只收"视野内"的信息。即便单进程模式也走 server/client 抽象，单进程 = in-memory loopback。
2. **规则可插拔**：`RuleSet` interface 让川麻/日麻共用 `internal/game` 状态机。
3. **Bot 规则无关**：bot 只依赖 `RuleSet` + `PlayerView`。
4. **防作弊**：日志里不要打印未公开信息；测试桩除外。

## 编码风格

- 标准 `gofmt` + `go vet` 必须过
- Error wrapping 用 `fmt.Errorf("...: %w", err)`
- 包注释用 `// Package xxx ...` 起头
- 公开 API 加 godoc；内部短小函数可省
- 测试文件 table-driven 优先；测试名 `TestThing_Scenario`

## 测试要求

- `internal/tile`、`internal/shanten`、`internal/rules/*`：**强制** 单元测试覆盖
- `internal/game`、`internal/ai`：行为测试（多局对战，统计断言）
- `internal/net`：集成测试（启 server + 多 client）
- 提交前 `go test ./...` 必须绿
- Shanten 计算要带 benchmark，回归看耗时

## Commit 风格

- 一行祈使句，主题清楚（"add shanten calculator"、"fix sichuan dingque flow"）
- 多条变更 → 多个 commit，别堆 squash
- 关联 phase：`[phase1] add tile types`

## 不要做的事

- 不要引入 `math/rand`（用 `crypto/rand`）
- 不要在 client 侧持有完整牌墙 / 他家手牌
- 不要为了"以后可能需要"加抽象（YAGNI）
- 不要在没读现有代码前重复造轮子（先 grep）

## 当前状态

见 git log + README roadmap。

## 已知 TODO（不影响阶段推进）

- **Bot 强度未对齐难度名**：Phase 3 实现了 Easy/Medium/Hard 三档不同策略，但在 Sichuan 血战的"先手为王"特性下，Medium 的"追求清一色"和 Hard 的"防守"实际拖慢了胡牌速度，1000 局对战中三档差异不明显。Phase 7 (bot 强化) 时再做 ML 风格的真正调优；现在的实现保证了三档各有不同代码路径，方便后续替换。
- **call 后流程简化**：`internal/game/loop.go` 中 pon/kan 后没有正确处理"主叫者直接弃牌"流程，目前 call 之后会让主叫者再摸一张。Easy bot 永不 call，所以 Phase 2-3 测试不会触发。Phase 5 联机时统一修。
