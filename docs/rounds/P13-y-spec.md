# P13-y 规格 · 插件窗口 bar 上的三个键：悬停戴 E1 的一对面，禁用只淡一道

**轮号**：P13-y（P13-x 之后）
**表面**：`.plugin-window__bar`——**输出窗口**（`internal/shell/detach.go` `detachedBar`）与
**便条窗口**（`internal/shell/pinned.go` `pinnedBody`）的同一根 bar。两个窗口画的是同一条 bar
（同一个 `detachedBarPadY/PadX/Gap`、同一个 8u 圆角、同一条 `Hairline` 底线），所以这一轮
是**一个表面**，不是两个。

## 意图

`docs/liquid-glass.md` §6 没有 `未开始` 行，按纪律回到通用队列的「状态反馈」那一格：
P13-s / P13-t / P13-u 把设置页的**禁用面**收口成「一道淡」，P13-h / P13-i / P13-p 把
**悬停小格**收口成 E1 的一对面——但两族的账本都只扫 `internal/settingsui` 与
`internal/launcher`。`internal/shell` 里这三处键停在两条契约之间：

| 位置 | 现状 | 违反的契约 |
|---|---|---|
| `detach.go:246-257` `barButton`（重跑键 / 关闭键）的 `Hovered()` 臂 | `Background(tokens.ControlHover)` 一块**平涂**，没有边也没有受光边 | §2 E1「面 + 1px `ControlEdge` + `LensRim` 顶边」 |
| 同上 `!enabled` 臂 | `Disabled(true)`（框架 0.5）× `Opacity(0.5)` = **0.25** | §1「禁用的材质」：**透明面**不叠材质，只剩框架那一道 0.5 |
| `pinned.go:96-101` 便条窗口的关闭键 | 同一个 ✕、同一个 `Hovered()` 平涂 | 同上（两个窗口是同一根 bar） |

`internal/shell/detach.go:252` 的注释写着「the framework's disabled flag alone (which changes no
ink) would leave the key looking live」——**那句话已经过期**：`ui/paint.go:140` 的
`p.opacity *= 0.5` 与 `ui/inline.go:109-113` 的 `alpha = 0.5` 就是框架那一道，产品的 0.5 是
第二次淡。这正是 P13-s 在设置页抓到的同一个「两个淡乘起来 = 一句话说了两遍」。

## 逐条落地项

1. **`internal/shell/detach.go` `barButton` 的 hover 臂**
   - 现状：`button.Background(tokens.ControlHover)`（平涂）。
   - 改成：`button = tokens.LitFace(button.Background(tokens.ControlHover))`
     —— 与 `launcher.rowControl`（P13-h）、`settingsui.headerButton`（P13-i）、
     `terminalui.barButton`（P13-d 3-7）同一个语汇：**面 + 顶受光边 + 底对侧暗线，无环、无影**。
2. **`internal/shell/detach.go` `barButton` 的 `!enabled` 臂**
   - 现状：`button.Disabled(true)` + `button.Opacity(0.5)`（有效 0.25）。
   - 改成：只留 `button.Disabled(true)`；注释按 §1 的三种读法重写（这是**透明面**：没有面也没有
     边，只剩一半的墨，不许长出面、不许戴受光边）。
3. **`internal/shell/pinned.go` 便条窗口的关闭键**
   - 现状：`close.Background(tokens.ControlHover)`（平涂）。
   - 改成：`close = tokens.LitFace(close.Background(tokens.ControlHover))`。
   - 两个窗口的同一个 ✕ 必须同一个触感：改一个不改另一个 = 同一个应用里同一个键两种读法。

**不做（写进 `.pending-bugs.md` 仍待）**：`internal/shell/notices.go:476` toast 里的 ✕ 是同一
形状的平涂，但它在 **E2 的 toast 面**上（§1 禁令 2 的边界另一侧），属于「弹层」那个表面，
下一轮单独做，避免这一轮跨表面。

## 行为契约零改动红线

高度链（`detachedBarPadY/X`、`detachedBarButtonY/X`）、键盘模型、会话管理、设置写回、
i18n 文案、命令接线**一个字都不动**；只改墨与面：三处 `Background` 后多一对内线，
一处少一个 `Opacity`。`Scale` / 命中区域 / `Label` / `Clicked()` 全部不动。

## 门槛命令 + PASS 基线（本轮实测，不是旧报告的数字）

```
gofmt -l cmd internal                                   # 必须空
go vet ./...                                            # 必须净
GOOS=windows go vet ./... ; GOOS=darwin go vet ./...    # 必须净
go test -count=1 ./...                                  # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"       # 只许涨
GOOS=linux/windows/darwin go build ./...                # 三平台过
```

基线（P13-x 交付时实测）：**PASS 3001 / FAIL 0 / SKIP 81 / 测试函数 2213**。

## pin 计划

新文件 `internal/shell/pluginwindowbar_test.go`：

1. `TestThePluginWindowBarKeysAreE1Faces`——两个窗口、三个键、深/浅两套外观：
   hover 之后在键自己的盒子里量三行（冠线 / 面 / 对侧），要求
   `lum(冠) > lum(面) > lum(对侧)`，并要求冠线**更接近**「同一块面上叠 `LensRim`」的复合值，
   而不是「叠 `EdgeHighlight`」的那个（候选值由测到的面当场算出来，pin 里不写死渲染器的像素）。
   **改前必红**（平涂：冠 = 面 = 对侧）。
2. `TestTheDisabledBarKeyDimsOnce`——重跑键在飞时的墨对比 / 静息时的墨对比，比值取平方根
   （`shellColorDistance` 是平方距离），要求落在 `[0.40, 0.60]`（一道 0.5）。
   **改前必红**（0.25）。

## mutation 计划（≥2 条，每条说明会让哪个 pin 变红）

- **M1**：把 `detach.go` 的 `tokens.LitFace(...)` 换回 `button.Background(...)`（即撤销第 1 条）
  → pin 1 的重跑键 / 关闭键两处变红。
- **M2**：把 `pinned.go` 的 `LitFace` 换回平涂 → pin 1 的便条关闭键变红（证明两个窗口各钉各的）。
- **M3**：把 `Opacity(0.5)` 加回去 → pin 2 变红（0.25 < 0.40）。
- **M4**：把 `LitFace` 换成 E2 的冠线（`InsetShadow(0,1,0,0, tokens.EdgeHighlight)`）→ pin 1 的
  「更接近 LensRim」那一半变红（证明 pin 分得清 E1 与 E2 的线）。
