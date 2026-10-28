# P11-ae 规格：终端标题栏的墨属于它浮着的那块面

## 0 意图与为什么现在做

终端页的浮条（`titleBar`）读的是**窗口**的四档墨（`tokens.TextSecondary` /
`TextTertiary` / `Theme.TextMuted` / `Theme.SurfaceHover`），但它浮着的那块面是**画布**——
`panelFill` 画的是调色板自己的底色。两者相反时（浅色窗口 + `ink`/`dusk`/`forest` 等深色
调色板），窗口的次级墨落在近黑面上：实测 `#6B6B70` 在 `ink` 的合成面上 **1.10:1**、
`forest` **1.24:1**（`.pending-bugs.md` P11-ad「仍待」第 3 条：默认 46% 透明度下都不到 2:1）。

这是用户实测反馈 **F2 的验收标准**（「保持浅色终端背景下的可读性与对比度达标」）在终端页
唯一还没被量过的一条，也是 P11-z/aa 之后**同一块表面上**唯一还读不出的字。
P11-ad 把它列为拍板项时给的三个选项里 (c)「墨随合成底明度选」实测不够——因为合成面是
一层**中灰**（浅色窗口 + 深色调色板：`ink` 的合成面 `(100,101,103)`），任何一侧的墨都不
过线。**本轮取第三条路：让 bar 穿画布自己的颜色**——面 = 调色板底色、墨 = 调色板前景，
这块浮条因此属于它浮着的面（终端自己的标题行就是这么做的），不动高度链、不动浮条位置、
不加渐变/rim/高光。

> 本轮同时**回收 `stash@{0}`**（`d2458db0`「floter orphan 2026-10-28 17:58」）。那条 stash
> 的骨架（`barInks` + `canvasInks` + 调用点改成 `inks.*`）原样接上；它缺的那一半——**bar
> 自己的脸**与「够用就最薄」的 alpha 推导——本轮补齐。接上后 `git status` 干净、门槛全过。

## 1 逐条落地项（动手前实测的行号）

| 位置 | 现状 | 应改成 |
|---|---|---|
| `internal/terminalui/terminalui.go:969` `tokensOf` 之后 | 没有「bar 的墨」这个概念，四个调用点各自读窗口 token | 新增 `barInks{Ink,Muted,Tertiary,Wash,Face}`、`windowBarInks`、`barInksFor`、`barFaceAlpha`、`quietRung`、`inkContrast`、`relativeLuminance` |
| `:1142 barButton` | 字形 `t.TextMuted`、hover 面 `t.SurfaceHover` | `inks.Muted` / `inks.Wash` |
| `:1164 titleBar` | 标题 `tokens.TextSecondary`、整条无面 | `inks.Ink`；**揭示时**（`revealed`）加 `Background(inks.Face)`，顶角取面板圆角（`panelCorner`），静止时不画面 |
| `:1253 statusDot` | 点 `tokens.TextTertiary` | `inks.Tertiary`（签名多一个 `barInks`） |
| `internal/terminalui/barink_test.go` | 无 | 3 条 pin + 1 条 opt-in dump |

**取值规则（一条，可算）**

- `inherit` / 未知 id / 空页（`running=false`）→ 窗口四档、`Face.A == 0`（**逐像素与今天
  一致**）。
- 命名调色板 + 有会话 → 面 = 调色板底色，`face.Alpha(α)`，α 取「在窗口面之上让调色板前景
  过 4.5:1 的**最小** 1/32 档」；墨 = 调色板前景；quiet 两档 = 同一墨在**这块合成面**上
  分别过 3.0:1（16u 字形，WCAG 1.4.11）与 2.5:1（1.75u 状态点，靠呼吸说话）的最小不透明度；
  hover 面 = 前景 0.10（参考的 toolbar-button hover 档）。

**红线（零改动）**：高度链、键盘模型、会话管理、设置写回、i18n 文案、命令接线、抽屉与
resident note 的几何、浮条位置与 28u 高、`Lens` 那一档。视觉轮只改墨与面。

## 2 门槛与基线（本轮实测）

| 门槛 | 基线（HEAD `5c1a01a` = P11-ad） | 本轮要求 |
|---|---|---|
| `gofmt -l cmd internal` | 空 | 空 |
| `go vet ./...` / `GOOS=windows` / `GOOS=darwin` | 净 / 净 / 净 | 同 |
| `go test -count=1 ./...` | 2352 PASS / 0 FAIL / 21 SKIP | PASS 只许涨 |
| 测试函数数 | 1915 | ≥ |
| 三平台 `go build ./...` | 过 | 过 |

## 3 mutation 计划（每条都要真红、真还原）

- **M1**：`barInksFor` 的调色板分支短路（`|| true` 直接 `return window`）→
  `TestTheBarWearsTheColourOfTheCanvasItFloatsOn` 红（bar 的墨回到窗口次级档、quiet 档
  不再是「最静」）。
- **M2**：面强制不透明（`scrim := face.Alpha(1)`）→ 同一条 pin 红（「面比它需要的更实」）。
- **M3**：`titleBar` 里删掉 `Background(inks.Face)` → `TestTheBarPaintsItsOwnFaceOverAPalette`
  红（像素上那条带子回到根黑）。
