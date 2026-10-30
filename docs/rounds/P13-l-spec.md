# P13-l 规格：设置页最后两块「平涂 + 画框」的控件面戴上 E1 的一对面

## 意图（一句话）

`internal/settingsui` 里还剩**两块**「`Control` 平涂 + 一圈 `ControlEdge` 画框」的**面**——
快捷键录制钮（`shortcutRecorder`）与**非危险臂**的告警带（`noticeBand(c, false, …)`）——
它们停在了 P12-c 点名的那个状态（「一块被画框圈住的平涂」），补上 E1 的受光边 + 对侧暗线。

## 为什么现在做（证据）

1. **§6 表在 P13-k 之后没有 `未开始` 行**，按纪律转「细节打磨通用队列」；队列里的
   「层级 / 状态反馈」这一格在 `settingsui` 上还有具体的行可指。
2. **同一族的前车之鉴**：P12-c 第 2-4 行（`fieldFace`）把「`Control` + `ControlEdge`」
   判成「只停在了一块被画框圈住的平涂」，并补了 `LitFace`；P13-i 的 9-1~9-10 又把十处
   同形状的控件面逐个换掉。**同一族没有扫干净**——下面两处漏在框外：
   - `internal/settingsui/settingsui.go:3711` `shortcutRecorder`：
     `Background(tokens.Control).Border(1, tokens.ControlEdge).FocusRing(true)`，
     全程没有 `LitFace`。它是快捷键页与自定义工具表单共用的**控件**（82u × 27u，
     两个调用点：`settingsui.go:3690`、`custom.go:120`）。
   - `internal/settingsui/settingsui.go:5878` `noticeBand`：`Control` + `ControlEdge`，
     只在 `danger` 臂叠一条 `InsetShadow(0,0,0,1, t.Warning)` 的警告环。
     **非危险臂**（`settingsui.go:5935` `disableConfirmation`，关掉一个扩展时出现）
     因此只剩一圈发丝——`grep` 全仓 `noticeBand(` 的四个调用点里，只有它一个是这个下场。
3. **两块面在同一个 z 平面上**：它们都坐在分组卡（E1，P12-c 第 2-3 行）里，
   自己却比卡还平，读起来像卡上贴了一张纸片。这正是用户说的「空间感不够」。

## 逐条落地项（行号实测于 HEAD = 72a4af9c）

| # | 位置 | 现状 | 应改成 |
|---|---|---|---|
| L1 | `settingsui.go:3715-3716`（`shortcutRecorder` 的 `button`） | `Background(Control)` + `Border(1, ControlEdge)` + `FocusRing(true)`，无内线 | 建好后 `tokens.LitFace(button)`（**三个 rung 共用**：hover / press 只换面，不换线——与 `uninstall.go:425-447` 的 `uninstallPartRow` 同一写法）。外圈 `ControlEdge` 不动，`FocusRing` 不动。 |
| L2 | `settingsui.go:5895-5898`（`noticeBand` 的 `if danger` 块） | `danger` 臂叠警告环；`else` 臂什么都没有 | 加 `else { tokens.LitFace(band) }`。**`danger` 臂故意不加**：`InsetShadow(0,0,0,1,…)` 的环占 padding 第一行，与 `LitFace` 的 `y=1` 受光边是同一行、加了会把环顶洗淡 9% 白、底压暗 6%（实测），一圈同色的环就不成立（与 §6.2b 9-11、§6.4 4-4 同一条判据）。 |
| L3 | `docs/liquid-glass.md` §6.2b | 表里 9-1~9-12 已收口 | 追加 9-13 / 9-14 两行（均 `已落地`），并在 §6.2b 的「只剩 1 个调用点」家族注释里说明这两处是**面**不是**槽**。 |
| L4 | `internal/shell/glasscensus_test.go` | census 里没有这两枚控件的条目 | 不新增条目：这两处从来不是四层透镜（`Lens`）的调用点，census 钉的是「透镜被**故意**退掉」的账。在报告里写明为什么不动它。 |

## 行为契约零改动红线

- 高度链：`MinHeight(Space(6.75))` / `Padding` / `Radius` **一个字节不动**；
  `LitFace` 只加两条 `InsetShadow`（不占布局）。
- 键盘模型：`Focusable()` / `FocusRing(true)` / `Label(aria)` 不动。
- 会话管理、设置写回、i18n 文案、命令接线：不碰。
- **视觉轮只改墨与面。**

## 门槛命令 + PASS 基线（本轮实测）

```
gofmt -l cmd internal                                  # 空
go vet ./...                                           # 净
GOOS=windows go vet ./...                              # 净
GOOS=darwin  go vet ./...                              # 净
go test -count=1 ./...                                 # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"      # 2923（基线）
go test -count=1 -v ./... | grep -c -- "--- FAIL"      # 0
go test -count=1 -v ./... | grep -c -- "--- SKIP"      # 69
grep -rn "^func Test" internal/ cmd/ --include=*_test.go | wc -l   # 2157
GOOS=linux go build ./... && GOOS=windows go build ./... && GOOS=darwin go build ./...
```

## V / B / S 预判

- **V**：`docs/liquid-glass.md` §6.2b 两行新增（视觉数值台账，不是断言）。
- **B**：现有 `settingsui` 测试全部零改动必须绿。`shortcutRecorder` 的既有断言若有
  「静息面 = `Control`」之类的像素 pin，先按 B 跑；红了再按 V 处置。
- **S**：无。

## mutation 计划（≥2 条，逐条说明让哪个 pin 变红）

| # | 注入的缺陷 | 应红的 pin |
|---|---|---|
| M1 | `shortcutRecorder` 换回 `tokens.Lens(button)`（四层透镜） | `TestTheShortcutRecorderIsAFaceNotARing`：左右两侧列变成 `ControlEdge`（①），深色档底内线比面亮（②） |
| M2 | 删掉 `shortcutRecorder` 的 `LitFace`（退回平涂） | 同上：冠线（③）不亮 |
| M3 | `noticeBand` 的 `else` 臂删掉 `LitFace` | `TestTheNoticeBandIsAFaceNotARing`：非危险带冠线不亮 |
| M4 | 把 `LitFace` 也加到 `danger` 臂 | `TestTheDangerNoticeBandKeepsItsWarningRing`：环的对称性被打破（顶内行 ≠ 底内行） |
| M5 | 给 `shortcutRecorder` 加一笔 `Shadow(` | 同上：块外投影判据（④） |
