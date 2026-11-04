# P13-gk 规格 · E2 的下沿：影拿走 79 级，那根线在白天是暗的、夜里是亮的

> 轮次：P13-gk（`docs/liquid-glass.md` §6.156）
> 上一轮：P13-gj（§6.155，E2 **顶边**的三段：影 / 描边 / 冠）
> 选题档位：纪律第 **3** 档（上一轮报告「仍待」④ 亲自点名的下一格）

## 1. 意图（一句话）

用与 §6.155 **同一把尺子**量 E2（浮层）的**下沿**，把「浮起来」这件事的另一半从形容词
变成读数，并顺手判掉「给下沿补一条 `EdgeShadow`」这条看起来最自然的错路。

为什么现在做：§6.155 只量了顶边，浮层的层级因此只有一半证据；而 `EdgeShadow`
（§3 表里那条「面板的暗底边」）**从未在帧内被读过**——它究竟是「还缺的一笔」还是
「已经有影在干这件事」，只有量了才知道。

## 2. 量尺（与 §6.155 一致，只有列不同）

- 场地：真 toast（`internal/shell/notices.go`：`Float` + `Border(1, InputStroke)` +
  `InsetShadow(0,1,0,0, EdgeHighlight)` + `ElevThrow`），settings 面，720×520，默认 47% 框
  alpha，入场弹簧 settle 后。
- 两帧对比：`before` = 还没有 toast 的那一帧（地面），`after` = settle 后。
- 列：**浮层自己的水平中线** `(left+right)/2`（顶边那一轮用的是 ✕ 中心；中线避开
  左侧的 mark、右侧的 ✕，也避开圆角）。
- 值：`straightLuma`（除掉框自身 alpha），1x 屏。

五行：

| 行 | 怎么定位 |
|---|---|
| `face` | 底边那一行的**上面一行** |
| `border` | 面往下第一处亮度跳 ≥ 12 的行（盒子自己那一行） |
| `contact` | `border` 往下 6 行里最暗的一行 |
| `ground` | `before` 帧同一行（＝设置面板自己的面） |
| `throw` | `border` 往下，到「亮过 ground−1」为止的行数 |

## 3. 逐条落地项

| # | 位置 | 现状 | 应改成 |
|---|---|---|---|
| 156-1 | `internal/shell/floatbottom_test.go`（新） | 下沿从未帧内读数 | 四个字面量：浅 `{face 249, border 227, contact 167, ground 246}` / 深 `{24, 45, 16, 23}` |
| 156-2 | 同上 | 没人说过下沿靠什么分离 | 浅：`ground − contact ≥ 24`（实测 79）；深：`ground − contact ≤ 8`（实测 7）且 `border − contact ≥ 16`（实测 29） |
| 156-3 | 同上 | 底边那条线的**符号**从未被钉 | 浅 `face − border ≥ 12`（−22，暗线）；深 `border − face ≥ 12`（+21，亮线）——同一条 `InputStroke` 随地面极性换符号 |
| 156-4 | 同上 | 「下沿影比上沿重」没人守 | 浅：下沿 79 ≥ 2 × 上沿 30；深：下沿 7 ≤ 8。两层影都朝下偏移，所以下沿才是深度的所在，不许对称化 |
| 156-5 | 同上 | 「夜里影短」会被误读成缺陷 | `throw` 深 18 < 浅 33：夜里地面只有 23，影 7 级就落地，不是几何变短 |
| 156-6 | 同上 opt-in dump | 无图 | `FLOTER_P13GK_DUMP=1` → `/tmp/floter-P13-gk-floatbottom-{light,dark}.png`（3×） |

**产品代码：字节零改动**（本轮是量尺轮，和 P13-fh / P13-gj 同一种交付）。

### 反判（候选，已实测挡掉）

`p.Root.InsetShadow(0, 0, 0, 1, tokens.EdgeShadow)`（给下沿补一条暗线）：

- 浅：面那一行 `249 → 231`，与 `border` 的 227 只差 **4** 级 → 下沿变成两行等亮的带
  （§1 禁令 3 的同构错误：一行的事，画成两行）；
- 深：面里多出一行 17（面 24 → 17 → 线 45），三行读出四段。

→ `EdgeShadow` 在 E2 的下沿**故意不做**，理由进 §6.156 的表。

## 4. 红线

行为契约零改动：高度链 / 键盘模型 / 会话管理 / 设置写回 / i18n 文案 / 命令接线。
本轮不碰任何产品文件。

## 5. 门槛（PASS 基线取实测）

```
gofmt -l cmd internal                       # 空
go vet ./...                                # 净
GOOS=windows go vet ./... ; GOOS=darwin go vet ./...
go test -count=1 ./...                      # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"   # 基线 3483，只许涨
GOOS=linux/windows/darwin go build ./...
```

## 6. mutation 计划

| # | 注入 | 该红的 pin |
|---|---|---|
| M1 | `internal/theme/elevation.go` 的 `ElevThrow` 接触影 `0.20 → 0.05` | 156-2 的 `border − contact ≥ 16`（浅）、156-1 字面量 |
| M2 | `ElevThrow` 宽投影 `0.30 → 0.00` | 156-2 的 `ground − contact ≥ 24`、156-5 的 `throw` |
| M3 | `internal/theme/theme.go` 深色 `edgeShadow` **不动**、改 `InputStroke` 深色档 → 156-3 的 `border − face ≥ 12` |
