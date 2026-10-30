# P13-r 规格 —— 分段控件的选中格：浅色档里它是「槽里更深的洞」

## 1 意图

设置页与终端抽屉共用的分段控件（`control.SegmentedIndex`），选中那一格在**浅色档**比它
自己的凹槽**暗 24 级**——同一枚控件在深色档是「抬起来的面」（86 对面 58，+28），在浅色档
是「凹槽里再挖深一层」（207 对 232，−25）。代码注释自己写着 "the chosen segment is a
raised pane"，§2 的 E1 也要求面比它坐着的底更「起」。这一轮把浅色档的选中格改成
**纸自己的颜色**（现代 macOS/iOS 的分段滑块），深色档一个字节不动。

## 2 为什么现在做（证据）

`docs/liquid-glass.md` §6 没有 `未开始` 行；`Lens` 只剩 1 个调用点且 §6.2b 9-11 已判
`故意不做`（accent 环盖住 rim/edge，换 `LitFace` 像素上不可见）。按纪律转通用队列的
**层级 / 空间感**格，先量再改。探针（`SegmentedIndex` 画在 247 地面上，320u 宽，三格）：

| 外观 | 地面 | 槽面（未选中格） | 选中格面 | 选中格顶内行 | 选中格底内行 |
|---|---|---|---|---|---|
| 浅色 | 247 | 232 | **207** | `129,154,188`（accent 环） | `120,144,178`（accent 环） |
| 深色 | 39 | 58 | **86** | `123,136,159`（accent 环） | 环 |

根因不是环，是**填充**：选中格的填充是 `tokens.ControlHover`。浅色档的「抬起」是黑洗
（`0,0,0,0.105`），压在凹槽面 232 上得到 207——比凹槽本身还暗。深色档的「抬起」是白洗
（`255,255,255,0.14`），压在 58 上得到 86，方向正确。**同一个 token 在两种底上指向相反
的 z 方向**，于是同一个控件在深色档说「这是抬起来的」，在浅色档说「这是挖下去的」。

用户 2026-10-30 的原话是「整体 UI 偏老气化，有点偏 macOS 旧版本」——「凹槽里更深的一格」
正是 macOS 10.x 分段控件的读法；现代 macOS/iOS 的滑块是**纸色抬起**。

## 3 落地项

| # | 位置 | 现状 | 应改成 |
|---|---|---|---|
| 1 | `internal/theme/theme.go` `Tokens` + `palette` | 没有「凹槽里的选中格」这一档；选中格借 `ControlHover` | 新 token `Thumb`：浅 `rgba(255,255,255,0.85)`、深 `rgba(255,255,255,0.14)`（= 深色档现值，**外观不变**）。浅色档取 0.85 是为了让它在**卡面**（`Control` 230）上的凹槽（216）里复合到 **249**，即页面纸色 247 ±2 —— 「抬起来的面就是纸本身」 |
| 2 | `internal/control/control.go` 选中臂 | `segment.Background(tokens.ControlHover)` | `segment.Background(tokens.Thumb)`；`Lens` 与 `AccentEdge` 环**不动**（§6.2b 9-11：环是它的身份线） |
| 3 | `internal/control/thumb_test.go`（新） | 选中格的 z 方向没有任何 pin | 三条像素 pin：①两套外观下选中格比槽面亮 ≥10 级（今天浅色档 −25）；②浅色档选中格落在纸色 ±3；③accent 环相对选中格面仍然可见（R 通道差 ≥60，防「白面把身份线洗掉」）。外加 `FLOTER_PNG=1` 的 before/after dump |
| 4 | `docs/liquid-glass.md` §6.2b 9-11 | `故意不做`（只谈 Lens↔LitFace） | 补一句：**填充**另案，P13-r 落地，见 §6.2b-5 |

## 4 行为契约零改动红线

只改一个 token 的墨与一个 `Background` 调用。高度链（30u 段高）、水平度量（`inset`
9u/6u）、`Lens` 的四层、`AccentEdge` 环、`Padding`/`MinHeight`/`Radius`、标签墨
（`TextStrong` / 580）、点击回调、键盘模型、i18n 文案：**一个字节不动**。

## 5 门槛命令 + PASS 基线（实测）

```
gofmt -l cmd internal                空
go vet ./...                         净（+ windows/darwin）
go test -count=1 ./...               全绿；PASS 2963 → 只许涨
GOOS=linux|windows|darwin go build   过
```

基线：**2963 PASS / 0 FAIL / 74 SKIP / 2180 函数**（P13-q 收尾实测）。

## 6 mutation 计划（≥2 条）

- **M1** 选中臂退回 `tokens.ControlHover` → `TestTheChosenSegmentRisesOutOfItsGroove` 浅色档红（−25）。
- **M2** 浅色 `thumb` 改成 `rgba(0,0,0,0.10)`（黑洗）→ 同一条 + `TestTheLightThumbIsTheSheetColour` 红。
- **M3** 删掉选中臂的 `InsetShadow(... AccentEdge)` → `TestTheChosenThumbKeepsItsAccentRing` 红。
- **M4** 浅色 `thumb` 抬到 `rgba(255,255,255,0.99)`（洗白到 254）→ 纸色那条 ±3 红。
