# P13-da 规格 · 会话行的高度：56 的地板在生产的行上已经成立（先量，不改）

## 意图

P13-cz 的「仍待」第 1 条说会话行步距实测 **46~47px**，而参考的
`.session-manager__row` 是 `min-height: 56px`（`main:src/styles/settings.css:1237-1244`），
怀疑本仓缺一块地板。本轮把这句话量清楚：**46~47px 是夹具的读数，不是生产的读数**。
生产里 `Session.Created` 永远非零（`internal/shell/shell.go:1070` 从
`terminalui.Session.Created` 直接抄），所以每一行至少是「标题 + 状态行」两行字，
盒高**恰好 56**——参考那条地板在本仓是**被内容自然满足**的，不需要补一个常量。

证据（本轮实测，浅色档、默认 interface step、720×760、`x=238` 扫盒）：

| 夹具 | 卡片盒 | 行盒 |
|---|---|---|
| 只有状态行（生产形状） | 177..235 | **178..233 = 56** |
| 带 cwd（三行） | 177..251 | 178..249 = 72 |
| 只有标题（P13-cz 的夹具） | 177..224 | 178..222 = 45 |

行内几何：标题盒 `y=187`（= 行顶 178 + 行自己的 `Space(2.25)` = 9）、`h=23`；
状态行 `y=211`（= 标题底 210 + 行的 1px 行距）、`h=15`；带 cwd 时
cwd `y=211 h=15`、meta `y=227 h=15`——**两行同高、同列、隔 1px**。

## 落地项（本轮不改画笔）

1. `internal/settingsui/sessionrow_test.go` 新增两条 pin：
   - `TestTheSessionRowMeetsTheReferencesFloor`：生产形状的会话行盒 = **56**（参考的地板值），
     标题盒顶 = 行顶 + 9（`Space(2.25)`）——地板由行自己的内边距 + 两行字满足；
     带 cwd 的行**高于**地板，且标题仍从行顶 +9 起（padding 管顶，地板管底）。
   - `TestTheSessionRowsStateLineIsOneRegisterWithItsBlock`：cwd 盒与状态盒同高（caption 15）、
     同左列（244）、相隔正好 1px（`rowLineGap`）。
2. `docs/liquid-glass.md` §6.70 记这笔账；`.pending-bugs.md` 顶部记「P13-cz 的 46~47 是夹具读数」。

## 行为契约零改动

高度链、键盘模型、会话管理、设置写回、i18n、命令接线：**零改动**。本轮无生产代码改动。

## 门槛（本轮实测基线）

`gofmt -l cmd internal` 空｜`go vet ./...` 净｜三平台 vet 净｜`go test -count=1 ./...` 全绿｜
PASS **3302** / FAIL 0 / SKIP 143 / 测试函数 2448。

## mutation 计划

- **M1**：`internal/settingsui/rows.go:1468` `Padding(t.Space(2.25), inset-1)` → `Padding(t.Space(1), inset-1)`
  → 行盒缩到 ~40，pin 1 红。
- **M2**：`internal/settingsui/rows.go:392` `rowLineGap` 的 `return 1` → `return t.Space(2)`
  → 两行相隔 8px，pin 2 红。
