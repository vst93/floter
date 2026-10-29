# P11-bg 规格：终端的失败提示也挂成画布顶边的一条带

## 1. 意图（一句话）

终端页上最后一处真正的玻璃浮卡——动作失败时那条 4.5 秒的提示——改成挂在标题条下沿的
**整宽带**（`Control` 面 + 底边一条 `HairlineSoft`），与 P11-bc 给「进程已退出」便条
做的同一件事，兑现用户 F3「不要 floating/overlay 观感，元素要 inline、要贴进布局」。

## 2. 为什么现在做（证据）

- 选题顺序：用户实测反馈（F1/F2/F3 已由 P11-y/z/aa 收完，`Super+N` 编号已由 P11-bb
  收完）→ 合约未落地条目（§6 全表 `已落地`，只剩 `⌘K`＝拍板）→ 上一轮「仍待」
  （P11-bf 第 1 条全是老拍板项，第 2 条 `configDiscardBar` 的 12u margin 是设计决定，
  第 3/4 条是侦察结论）→ **界面质感与交互逻辑**（用户 2026-10-28 定的优先档）。
- P11-ba 的报告就把这一条列成「界面质感这一档剩下的新候选」：*终端退出便条 / 失败提示
  要不要改 inline*。P11-bc 做掉了前者（`residentNote`），并在报告里写明后者
  「故意没动（它是消息不是家具）」。**同族的另一半至今还浮着**：
  - `internal/terminalui/terminalui.go:1276` 的
    `ui.Row(c).Absolute().Top(t.Space(3)).Left(t.Space(3)).PassThrough()` ——
    离面板左上各 12u 的浮卡；
  - 同处 `:1279-1285` 的 `Radius(tokens.RadiusMD)`、`Border(1, tokens.ControlEdge)`、
    `Background(tokens.Float)`、`Transition{Enter: &ui.Motion{Y: -2}, 160ms}`；
  - `:1295-1296` 的 `theme.ElevThrow(notice, ...)` 与 `InsetShadow(0,1,0,0, tokens.LensRim)`。
  于是**同一页两种说法**：底边那条（P11-bc）是贴边的带，顶边这条还是带投影、受光边、
  圆角、入场位移的玻璃片。
- 用户 F3 的判据（`docs/raycast-design.md` §1/§5/§6.6）只看「它是不是一块压在内容上的
  浮面」，不看它是常驻还是转瞬：一条 4.5 秒的提示同样是**状态**，不是家具也不是浮窗。
- `residentstrip_test.go:106` 已经把「浮卡的四件套」写成了源码契约
  （`ElevThrow(`/`InsetShadow(`/`Radius(`/`Transition(`）。它当时只守 `residentNote`。

## 3. 逐条落地项（动手前逐条核过真实行号）

| # | 位置 | 现状 | 应改成 |
|---|------|------|--------|
| 1 | `terminalui.go:1276` | `Absolute().Top(Space(3)).Left(Space(3))`，自然宽（内容宽） | `Absolute().Top(barHeight(t)).Left(0).Right(0)`：**挂在标题条下沿、整宽贴边**。标题条本就浮在画布顶上（`:1368` 的 `Top(0)/Height(Space(7))`，画布不预留高度），所以这条带读作 chrome 的第二行，而不是压在输出上的一块面 |
| 2 | `:1279-1285` | 圆角 `RadiusMD`、`ControlEdge` 描边、`Float` 面、160ms 从上方 2u 落下的入场 | 无圆角、无描边环、无入场动画；面 `tokens.Control`（与 `residentNote`/`drawer` 同一条带面），**底边**一条 `HairlineSoft`（底边那条带是上边一条；两条带各自朝画布内侧收口） |
| 3 | `:1295-1296` | `ElevThrow` 投影 + `LensRim` 内高光 | 两条都删 |
| 4 | `:1276` / `:1291` / `:1294` | 外层与两个字层各 `PassThrough()` | **原样保留**（参考 `terminal.css:414` 的 `pointer-events: none`；P11-bd 的 `TestTheCanvasOverlaysGoQuietWhenHidden` 按 `1+Text+Icon` 数层数） |
| 5 | `:1250-1260` | `c.AnimationFrame()` + `terminalFeedbackDuration`（4500ms）时钟、`feedbackShown` 去重 | **零改动**：它仍是会自己消失的提示，帧要继续来 |
| 6 | `:1264-1275` 与 `:1292-1298` | 15u 警示字形、body 步 560、`t.Warning` 墨 | **零改动**（语义色不动，用户 F2 的验收要求错误提示仍然看得见） |
| 7 | `:890` / `:1368` / `:1276` | 同一个「标题条高度」在三处各写一次 `t.Space(7)` | 合成一个 `barHeight(t)`（同族「一个值域一个答案」，本轮只是顺手把这条带挂上去要靠它） |

## 4. 行为契约零改动红线

- **高度链**：这条带是 `Absolute()`，**不占布局高度**，所以画布高度、PTY 行数与
  `MinHeight(barHeight)` 一个字节不动（与 P11-bc 同一条理由）。
- 键盘模型、会话管理、设置写回、i18n 文案、命令接线：一律不碰。本轮只改墨与面。
- 文案仍用现成的 `a.feedback`（由调用方 `ShowFeedback` 塞进来），不加新字。

## 5. 门槛命令 + PASS 基线（本轮实测的当前值）

```
gofmt -l cmd internal                       # 空
go vet ./...                                # 净
GOOS=windows go vet ./...                   # 净
GOOS=darwin  go vet ./...                   # 净
go test -count=1 ./...                      # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"   # 基线 2440（只许涨）
GOOS=linux/windows/darwin go build ./...    # 过
```

测试函数基线 `grep -rhn '^func Test' internal/ cmd/ --include=*_test.go | wc -l` = **1984**。

## 6. 变异计划（≥2 条，说明每条让哪个 pin 变红）

- **M1**：把 `Background(tokens.Control)` 改回 `Background(tokens.Float)` ——
  像素 pin 里「带面 == 画布」那一臂红（`Float` 与终端画布填充在两外观下都不同，
  但带面必须与带外的像素分得开，并且与 `residentNote` 同一档）。
  预测：`TestTheFailureNoteIsADockedStrip` 的「band face is not the canvas」红。
- **M2**：把 `theme.ElevThrow(...)` 加回去 —— 源码契约 pin
  `TestTheFailureNoteLetsThePointerThrough` 红；像素 pin 的「带上沿之外是画布」一臂
  也会红（投影会在带的上方留一圈更暗的像素）。
- **M3**：把 `.PassThrough()` 从任一字层删掉 ——
  `TestTheCanvasOverlaysGoQuietWhenHidden` 的层数一臂红。
