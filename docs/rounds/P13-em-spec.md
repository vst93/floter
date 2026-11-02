# P13-em 规格：失败那一次，卡上有两处告警（状态词 + 带子）

## 意图（一句话）

**告警只有一处**：保存失败时，`configToolbar` 的状态词与它下面的错误带子都在用
`Warning` 喊同一件事；带子扛着句子的全部信息（图标 + keyline + 错误原文），词退回到
陈述档（`TextTertiary`），词本身一个字不改。

## 为什么现在做（证据）

- 上一轮（P13-el，`docs/liquid-glass.md` §6.107）已经把「状态词是**状态**，不是告警」
  写进合约，并把「在飞」那一档从 `Warning` 拉回 `TextTertiary`。剩下的失败档是同一格：
  `internal/settingsui/config.go:227-231` 只有两档面，`dirty` 恒为 `Warning`，而失败时
  `dirty` 仍为真（`finishConfigSave` 只在成功时 re-baseline）。
- 失败时同一张卡上**两处** `Warning`：
  1. 工具栏状态词 `ConfigUnsaved` + `t.Warning`（`config.go:230`）；
  2. 错误带子 `noticeLine`：`Control` 面 + `ControlEdge` 边 + 1px `Warning` 内 keyline +
     `alert` 图标 `Warning` + 句子 `Warning` @ `Body`（`config.go:292-303`）。
- 两处同色同义 = 卡片喊两遍，而**能读的那一处**（带子，有原文）在下面，喊得最响的
  那一处（词，在字段上面）没有新信息。§6.107 的判据反过来用：词只在「它是唯一载体」
  时才是告警。
- 词一个字不改：`ConfigUnsaved` 在失败后**仍然为真**（改动确实还没存住），改词要新增
  i18n 文案 → 踩红线（`.pending-bugs.md` 的「需拍板」）。

## 逐条落地项

| # | 位置 | 现状 | 应改成 |
|---|---|---|---|
| 1 | `internal/settingsui/config.go` `configToolbar`（`config.go:238` 之后） | 失败时 `ink` 仍是 `t.Warning`（与脏逐字节相同） | `a.configError[integration.ID] != ""` 时 `ink = tokens.TextTertiary`；**词不动**（仍 `ConfigUnsaved`） |
| 2 | 同上，几何 | 带子插在字段与动作行之间 | **不动**：状态词在字段**上面**，它的盒子 y 逐字节不变（pin 断言） |

## 行为契约零改动红线

只改一格墨。不碰：高度链、键盘模型、会话管理、设置写回（`SaveConfiguration` 的调用与
`finishConfigSave` 的语义）、i18n 文案（词一个字不改、不加键）、命令接线。带子的存在
与否、位置、颜色一律不动。

## 门槛命令 + PASS 基线（本轮实测）

```
gofmt -l cmd internal          空
go vet ./...                   净
GOOS=windows/darwin go vet     净
go test -count=1 ./...         全绿
--- PASS                       3345（HEAD 实测 3343 + 1 FAIL）
--- SKIP                       172
测试函数                        2519
三平台 go build ./...          过
```

## mutation 计划（≥2，说明每条让哪个 pin 变红）

- **M1 退回**（删掉新那一段）→ `the failed save's word is painted … nearest the warning step`
  变红（同时「脏 → 失败」两帧的墨逐字节相同）。
- **M2 连带子一起熄灯**（`noticeLine` 的句子改成 `TextTertiary`）→ 带子那条
  `the error band is painted … want the warning ink` 变红：证明这条 pin 不是「只要不红就绿」。
- **M3 一刀切**（无条件 `ink = tokens.TextTertiary`）→ 对照组（脏、未失败）那条
  `a dirty form's word is painted … nearest the warning step` 变红。
