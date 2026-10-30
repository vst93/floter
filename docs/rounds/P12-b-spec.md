# P12-b 规格：启动器表面——E1 的受光边落到这张卡的四个面上

**意图一句话**：把 `docs/liquid-glass.md` §6.1 那 7 行里**有面**的 5 行做实——选中块、
页脚动作条的选中块、键帽、捕获输出页的选中行、暖面反馈行——让它们从「一块平涂」变成
「一块站起来的 E1」，并用一条 `LitFace`（顶受光边 + 底对侧暗线）把这条规则收成一个函数。

**为什么现在做**：`docs/liquid-glass.md` §6.1 是差距表的第一个 `未开始` 表面（选题纪律第 0 条）；
P12-a 只把 token 层升了代（`LensRim` 0.04 → 0.09），**还没有任何组件真的画那条线**——
`.pending-bugs.md` P12-a「仍待」第 3 条原话：「E1 受光边还没落到任何组件上」。

## 逐条落地项

行号均为**动手前实测**（`git log` HEAD = `bea73678`）。

### 1-2 结果行选中块（`internal/launcher/launcher.go:2355-2371`）

- 现状：`case i == a.Selected:` 只有 `row.Background(tokens.RowSelected)` —— 一块平涂，
  与底挤在同一 z 平面。实测（720×580，`/tmp/floter-P12-b-list-dark-before.png`）：
  `(600, 116)` 是 `(61,61,63)`，同帧面板 `(600, 160)` 是 `(28,28,30)` —— 面差 33 级但
  **顶边没有任何一条更亮的线**。
- 应改成：`row = tokens.LitFace(row)` —— 顶边 1px `LensRim` + 底边 1px `LensEdge`。
  E1 **不画投影**（§2 判据 2）。

### 1-3 页脚动作条的选中块（`internal/launcher/launcher.go:2352-2360`）

- 现状：`case i == a.Selected && isBar:` 同上，一块平涂。
- 应改成：同一个 `tokens.LitFace`。

### 1-4 键帽（`internal/launcher/keycap.go:86-90`）

- 现状：`Border(1, tokens.KeycapEdge).Background(tokens.KeycapFace)`，没有顶边。
- 应改成：`key = tokens.LitFace(key)`。键帽是最该受光的小面。

### 1-5 捕获输出页的选中行（`internal/launcher/output.go` `outputRow`）

- 现状：`case selected: style.Background(tokens.RowSelected)`。
- 应改成：同一个 `tokens.LitFace`——两张列表是同一张卡里的同一件东西。

### 1-7 暖面反馈行（`internal/launcher/launcher.go` `feedbackRow`）

- 现状：顶边 `LensRim` **只在 `a.toastWarning` 分支里画**，静息档的注释自己写着
  「The non-warning row carries none of them」。同一条带在两个状态里是两个材质。
- 应改成：`feedback = tokens.LitFace(feedback)` 无条件画，警告档多出来的只是 keyline
  与两层影（§3：受光边与色相无关）。

### 1-1 卡片（E0）与 1-6 空态（E0）

- **保持不动**：卡片是 E0（它是窗口/面板本身，macOS 由窗口服务器投影），空态是一句居中、
  无底板。两行标 `已落地（保持）`。

## 为什么是 `LitFace` 而不是 `Lens`

`Lens` 是四层控件透镜：顶 rim、宽 sheen、底影、**四边 `ControlEdge` 环**。第四层是
「画框分层」——正是这一支要退掉的那一代。而 `docs/raycast-design.md` §4.3 明写选中行
「no border, no ring, no glow」。E1 只用三件事站起来：面差、顶受光边、底对侧暗线。

**为什么必须有底对侧暗线**：浅色档的面是近白（`Control` 0.07 黑压在 sunken 面板上），
0.09 的白线压在它上面只抬 **2 级**（实测 `(600,94)` 221 → 227）。真正让块在浅色上
站起来的是对侧那条 `LensEdge`（实测 `(600,129)` 220 → 203，17 级）。这与 §3 的
「顶亮一分、底暗一分，控件才站起来」是同一句话，只是此前没人把它落到组件上。

## 行为契约零改动红线

只改墨与面。高度链（`rowUnits`/`actionBarUnits`/`fieldRowUnits`）、键盘模型、会话管理、
设置写回、i18n 文案、命令接线**一个字不动**。

## 门槛命令 + PASS 基线（实测）

```
gofmt -l cmd internal                                  # 空
go vet ./...                                           # 净
GOOS=windows go vet ./...                              # 净
GOOS=darwin  go vet ./...                              # 净
go test -count=1 ./...                                 # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"      # 只许涨
GOOS={linux,windows,darwin} go build ./...             # 过
```

基线（P12-a 收尾实测）：PASS **2805** / FAIL 0 / SKIP 59 / 测试函数 **2086**。

## mutation 计划（≥2 条）

| # | 注入 | 应红的 pin |
|---|---|---|
| M1 | 结果行选中块去掉 `LitFace` | `TestTheChosenRowStandsOnOneLitEdge` |
| M2 | 键帽去掉 `LitFace` | `TestTheKeycapIsTheSmallestLitFace` |
| M3 | 反馈带去掉 `LitFace` | `TestTheFeedbackBandIsOneMaterialInBothStates` |
| M4 | 输出页选中行去掉 `LitFace` | `TestTheOutputSelectedRowWearsTheSameLitFace` |

## V/B/S 预判

本轮**不预期**任何 B 类红。已预判的一处 V 类：`internal/shell/glass_test.go` 的
`TestTheFrameSheenIsFixedAcrossStops` 采样带 `y=90..120` 落在**行与键帽上**，那条 pin 的
断言是「帧自己的脸不随 stop 变」，而控件透镜本来就该随 stop 分级（`--glass-lens-scale`
0.5/1/1.6）。受光边抬到 0.09 后该带的 stop 间色阶差从 0.14 涨到 2.36，超过 pin 的
`spread > 1`。处置：采样带换成 `y=300..340`（面板上什么都没有的一段），断言强度不变
（实测该带 stop 间差 0.00 深 / 0.02 浅）。这是 V 类重定基线，注释写明理由。
