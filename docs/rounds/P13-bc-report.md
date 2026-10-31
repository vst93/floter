# P13-bc 报告 · accent 主 CTA 的指针阶梯：两枚同一个角色，三个不同的说法

**方向一句话**：合约 §5 第 2 条点名的「主 CTA 填充」这一类在本仓有两枚（About 页的更新键、
扩展面板的 Save / Run），它们对指针的回答此前是**三种**——一枚不动、一枚淡掉整棵子树、
一枚（`theme` 里的 `AccentHover` / `AccentPressed`）设计好了但**从没接上**。这一轮把这一族
收成一句：面抬一档、按下沉一档，冠线与墨不动。

- 规格 `docs/rounds/P13-bc-spec.md`
- 合约新增 `docs/liquid-glass.md` **§6.22**（5 行全部 `已落地`）

## 这轮为什么让它更轻、更有空间感

「轻」在这一支的判据里不是「更淡」，是**层级只用面差说话，不用透明度说话**。上一枚
`updateButton` 用 `.Opacity(0.9)` 表达 hover——透明度乘的是**整棵子树**，于是：

1. §3 的「受光边 = 这块面可以被碰的记号」恰好在指针说「可以碰」的那一刻**自己熄了一半**；
2. 同一个淡在两种外观上**方向相反**：浅色 `6/74/166` 淡向 247 的地面变成 `29/91/173`
   （变亮 = 沉进纸里），深色 `143/183/255` 淡向 14 变成 `130/166/232`（变暗 = 也沉）。
   这正是 P13-r 在分段控件上判掉的那条算术：**透明度不是一句 z 声明**，它的方向由脚下的
   地面决定，不由设计决定。

另一枚更彻底：`extensionPrimaryButton` 的静息帧与 hover 帧**逐像素相同**。一枚能按的面在
指针下不动，读起来像「它没收到这个指针」。

改完之后，这一族与中性胶囊族第一次说同一句话（`Control` / `ControlHover` / `ControlPress`
↔ `Accent` / `AccentHover` / `AccentPressed`）：**指针的每一档只改面，不改墨、不熄灯**。

## 读数（`internal/settingsui/accentcta_test.go`，真设置页地面）

| 控件 | 外观 | 静息面 | hover 面（改前 → 改后） | hover 墨（改前 → 改后） |
|---|---|---|---|---|
| `extensionPrimaryButton` | 浅 | `6/74/166` | **`6/74/166`（不动）→ `8/87/196`** | `247/249/255`（不动）→ **不动** |
| 同上 | 深 | `143/183/255` | **`143/183/255`（不动）→ `163/196/255`** | `21/23/27`（不动）→ **不动** |
| `updateButton` | 浅 | `6/74/166` | `29/91/173` → **`8/87/196`** | `247/249/255` → `226/235/250`（Δ21）→ **不动** |
| 同上 | 深 | `143/183/255` | `130/166/232` → **`163/196/255`** | `21/23/27` → `33/38/47`（Δ20）→ **不动** |

按下档（改前两枚都**没有**这一档，改后 = `AccentPressed`）：

| 外观 | `AccentPressed` | 按下实测面 |
|---|---|---|
| 浅 | `5/60/135` | `5/60/135`（两枚同值） |
| 深 | `122/167/255` | `122/167/255`（两枚同值） |

**几何零改动**：`updateButton` 的盒仍 `85.2 × 28.8`、`extensionPrimaryButton` 仍 `44 × 27`
（`updateButton` **不加** `Border`——参考的 `.update-banner__button` 本来就没有边，
`settings.css:1542-1568`；加了会把盒高长 2px，那是几何改动不是墨）。

## 落地（5 行）

| # | 位置 | 旧 | 新 |
|---|---|---|---|
| 1 | `internal/settingsui/rows.go` `accentPrimaryFace`（新增） | — | 三档面 + 冠线 + 按下几何 + disabled 臂，一个地方定义 |
| 2 | `rows.go` `extensionPrimaryButton` | 只有 disabled 臂；静息与 hover 同帧 | 走 `accentPrimaryFace`；边仍留在这里（两枚的边本来不同） |
| 3 | `internal/settingsui/settingsui.go` `updateButton` | `if Hovered() { .Opacity(0.9) }` | 走 `accentPrimaryFace`，`.Opacity(0.9)` **删掉** |
| 4 | `internal/theme/theme.go` | `AccentHover` / `AccentPressed` **零调用点**（P8-a 起的死值） | 值一个数不动，由上面两处接上 |
| 5 | `internal/settingsui/accentcta_test.go` | — | 普查：一个函数里同时出现「accent 面」与 `.Opacity(` 即红（`go/ast` 读函数体，不是 grep 行） |

## 门槛（全部实测）

- `gofmt -l cmd internal` → **空**
- `go vet ./...` → 净；`GOOS={windows,darwin} go vet ./...` → 净
- `go test -count=1 ./...` → 全绿（**0 FAIL**）
- PASS **3122 → 3132**（+10）| SKIP 103 → **104**（+1，opt-in 出图）| 测试函数 2301 → **2306**（+5）
- `GOOS={linux,windows,darwin} go build ./...` → 三平台过
- **V 0 / B 0 / S 0**

## mutation（5 条实跑变红并还原，`md5sum -c` 逐字节核对）

1. **M1** `accentPrimaryFace` 的 hover 臂退回静息面（`plane = t.Accent`）：
   ```
   accentcta_test.go:164: [light] updateButton: hovered face = {6 74 166 255}, want the ladder's
   AccentHover {8 87 196 255} — the accent primary has no hover rung (liquid-glass.md §5 第 2 条)
   accentcta_test.go:164: [dark] extensionPrimaryButton: hovered face = {143 183 255 255}, want
   the ladder's AccentHover {163 196 255 255}
   ```
2. **M2** `updateButton` 重新挂上 `.Opacity(0.9)`：
   ```
   accentcta_test.go:190: [light] updateButton: hover moved the label ink {247 249 255} →
   {226 235 250} (Δ 21) — the rung belongs to the face, not to the subtree
   accentcta_test.go:190: [dark] updateButton: hover moved the label ink {21 23 27} →
   {33 38 47} (Δ 20)
   ```
3. **M3+M4** 按下臂整条删掉（面与几何都删）：
   ```
   accentcta_test.go:230: [light] updateButton: pressed face = {6 74 166 255}, want the ladder's
   AccentPressed {5 60 135 255} — the press rung is the ladder stepping back down
   ```
4. **M5** 一处 accent 面重新用 `.Opacity(` 表达深度（普查那一条）：
   ```
   accentcta_test.go:302: an accent face says its depth through `.Opacity(`, which resolves
   against whatever ground is underneath (liquid-glass.md §5 第 2 条 / P13-r)
   ```
   还原后 `md5sum -c` 逐字节相同，`touch` 复跑绿。

## 偷了什么（craft-loop 工艺密度菜单 + preflight）

- **craft-loop #5「多层染色阴影」→ 退一步为「一条阶梯，不是一层淡」**。这一族没有加影，
  而是把「深度」从**透明度**（一层淡，方向随地面翻转）改成**阶梯**（三档色值，方向恒定）。
  具体数值：`Accent` `#064AA6` / `#0857C4` / `#053C87`（浅）、`#8FB7FF` / `#A3C4FF` /
  `#7AA7FF`（深）——即 `theme.AccentHover` / `theme.AccentPressed` 这两个**原本就在仓库里、
  但零调用点**的值。
- **craft-loop #6「编辑细节」→ 借用中性族的既有刻度**。中性胶囊的三档是
  `Control` / `ControlHover` / `ControlPress`（P13-q 立的），accent 族此前没有对齐；
  这一轮把 `pressed` 的几何也取同一个 `0.97`（`extensionActionFace` 的 `:active`），
  所以两个族在指针下是**同一句话**。
- **craft-loop #12「材质细节」→ 受光边的语义（§3）当成断言**。`TestAnAccentCTAsHoverMovesOnlyItsFace`
  量的是「hover 时冠线仍然亮着」——因为 `updateButton` 的旧写法恰好在那一刻把它熄了。
- **preflight A（反 AI 套路）**：无渐变、无发光、无 `transition: all`、无 `scale(0)` 入场。
  唯一几何变化是 `0.97` 的按下收档（§5 已有的那一档）。
- **preflight B（设计读取一致性）**：产出与「轻便液态玻璃 + 明确 elevation 阶梯」一致——
  这一轮把「用透明度表达 z」这条**违反**阶梯的做法删掉了。
- **preflight C（动效抽查）**：`Colors: true` + `theme.ColorStep`（120ms），只动颜色；
  按下只动 transform。`prefers-reduced-motion` 由框架的 `ReduceMotion` 管，探针里就设着它。
  **不做弹跳**、**不做整卡 pose transform**（myr 的渲染红线）。
- **preflight D2（中文页）**：本轮没有新增任何可见文案（i18n 一个字节没改），
  `Body` / `Caption` 的字号张力是既有的「需拍板」项，本轮不碰。

**没做成的一项（如实记录）**：`AccentPressed` 的**按下几何**（`Scale(0.97)`）没有单独的 pin。
探针读的是像素面，而 `Scale` 是渲染变换——量几何要读盒，而盒在布局里不随 `Scale` 变
（`pressface_test.go` 的 P13-q 已经用「同一帧里读画出来的盒」解决过，但那需要一张对照图）。
这一条按纪律不造脆弱断言，写进「仍待」。

## 需拍板（沿用，本轮未碰）

1. `ui/tooltip.go:133` 的 `s.tip && s.flags&flagDisabled == 0`——禁用元素不弹提示。
2. `tokens.Body` 10.8px / `tokens.Caption` 11px 与 preflight D2「正文 ≥ 14px」的张力。
3. `cardRow` 控制槽 `Shrink(0)`；多选箱宽度。
4. 终端弧 S1b（`CloseAll` 改 detach）、S1c（Windows ConPTY）、S2 尾（客户端不告而别）。

## 下一格

§6 表仍无 `未开始` 行 → 通用队列。候选：① 按下几何的像素 pin（本报告「没做成」那条，
需要一张对照图的做法）；② 空态与满态的**文案密度**（`Caption` 档的句子在 720 宽下折几行，
与 §4.1 的「一行就是一行」是同族）；③ `internal/shell` / `internal/launcher` 的
「后置禁用」普查（P13-ba 的普查只扫了 `internal/settingsui`）。
