# P11-an 报告 · 浮动层的投影跟着框的透明度缩放

选题顺序照 `.pending-bugs.md` 顶部：用户实测反馈（F1/F2/F3 已由 P11-y/z/aa 收完）→
合约未落地条目（§6 只剩 `⌘K` 动作面＝拍板）→ 上一轮「仍待」第 1 条（`AccentEdge` 的
3:1＝拍板）→ **界面质感与交互**。这一档里找到的是一条**代码自证**的缺陷：投影的 alpha
是字面值，而两处注释都写着它「跟着框的 alpha 缩放」。规格 `docs/rounds/P11-an-spec.md`。

## 1 根因

参考把每一档 elevation 的投影都乘上 `--elev-shadow-scale`
（`main:src/styles/base.css:178`）：

```css
--elev-shadow-scale: calc(0.45 + 0.9 * (1 - var(--main-opacity)));
```

于是同一条 `--elev-3`（`0 10px 26px 0.3` + `0 1px 2px 0.2`，`base.css:209-210`）
在 100% 的框上是 0.135，在滑块最薄那一档（10%）才是 0.378。移植时抄的是
**scale = 1 那一档的字面值**，四个浮动层都成了常数：

- `internal/launcher/launcher.go:2965-2972` 警告状态行的接触边 `0.2`，注释写着
  「a function of the frame's own alpha, so its 0.2 is scaled here exactly as the
  terminal note's is」；
- `internal/settingsui/uninstall.go:166-167` 卸载对话框的 `0.3` / `0.2`；
- `internal/terminalui/terminalui.go:1204-1205` 退出便条、`:1254-1255` 失败提示。

注释描述的缩放**从未实现**。后果正是用户说的「不轻」：滑块往下拉时填充变薄、投影不变，
一块几乎透明的面板顶着一团没变轻的黑边；反过来在默认档（`MainOpacity` 47 → scale
0.927）本仓的投影比参考重 7%。

## 2 落地

`internal/theme/elevation.go`（新）：`ElevScale` / `ElevThrow` / `ElevContact`，四个调用点
全部转发，几何（10/26、1/2 的 `--u` 步、圆角、边框、`LensRim`）一个字节没动。

## 3 台账

- **V = 0**：没有旧 pin 钉过这四处投影的像素或 alpha（`TestTheTerminalViewsPaintFromTokensNotLiterals`
  只扫 hex 与 `rgba(` 字面，`ui.RGBA(...)` 不在它的判据里——这也正是这些字面值能活到今天的原因）。
- **B = 0**：既有 pin 一条没改，全绿。
- **S = 1 类**：四处字面投影 → 一处算术（`ElevScale`），注释写清旧 → 新。
- 新增两条 pin：`internal/theme/elevation_test.go`（`TestTheElevationScaleFollowsTheFrameAlpha`：
  两端、默认档、单调性、夹取）与 `internal/ci/elevation_contract_test.go`
  （`TestFloatingThrowsFollowTheFrameScale`：三个画字面的包里 `Shadow(` 的 alpha 不许是
  裸小数字面量）。
- 函数 1937 → **1939**，PASS 2372 → **2374**，SKIP 27，FAIL 0。
- 门槛：`gofmt -l cmd internal` 空 · `go vet ./...` 净 · `GOOS=windows/darwin go vet ./...` 净 ·
  三平台 `go build` 过。

## 4 mutation（实跑，原始报错串）

M1 `ElevScale` 恒返回 `1`（＝旧的字面档）：

```
--- FAIL: TestTheElevationScaleFollowsTheFrameAlpha (0.00s)
    elevation_test.go:46: the default frame's scale is 1; the port painted 1 (unscaled)
```

M2 把 `theme.ElevThrow(notice, …)` 改回 `notice.Shadow(0, t.Space(2.5), t.Space(6.5), 0, ui.RGBA(0, 0, 0, 0.3))`：

```
--- FAIL: TestFloatingThrowsFollowTheFrameScale (0.00s)
    elevation_contract_test.go:48: ../terminalui/terminalui.go:1256 casts a shadow at a literal
    alpha (Shadow(0, t.Space(2.5), t.Space(6.5), 0, ui.RGBA(0, 0, 0, 0.3); paint it with
    theme.ElevThrow / theme.ElevContact so it follows the frame's own transparency
```

两条都由 `md5sum -c` 逐字节还原并复跑绿（`internal/theme/elevation.go: 成功`）。

## 5 图与像素（我实测看到了什么）

`/tmp/floter-P11-an-dialog-light-new.png` / `-light-old.png` / `-dark-new.png` / `-dark-old.png`
（`ui.Tester` 720×640，卸载对话框 + scrim；`-old` 是 M1 那版，即本轮之前的字面投影）。

- **浅色**：**33032** 个像素在对话框四周变亮，最大单像素 RGB 差 **15**：
  `(137,550)` 从 `(113,113,113)` → `(118,118,118)`——那一圈正是对话框下沿外的投影带。
- **深色**：**7618** 个像素变亮，最大差 **3**：`(170,84)` 从 `(15,15,17)` → `(16,16,18)`。
  深色面本来就吃光，7% 的 alpha 在近黑底上只值一个通道。
- 图上看到的一处变化：对话框右下角的影子从「一圈能看出边界的黑」变成「一层更散的灰」，
  面板本身（圆角、边框、`LensRim`）一个像素没动。

## 6 仍待

1. **投影只跟了 `main_opacity`，没跟 `terminal_opacity`**：终端页的画布自己有一条透明度
   轴，但便条/提示浮着的是**面板**，不是画布，所以两者取面板那条（本轮按这条判据）。
   若将来终端便条要跟着画布的透明度走，那是另一条决定，别顺手改。
2. 老拍板项原样有效：caret 脱离 accent、默认浅色安静档 3.04:1、终端滚动缓冲、`⌘K` 动作面、
   `AccentEdge` 3:1。
3. `internal/theme/theme.go:1085` 的 `InsetShadow(0, 1, 2, 0, ui.RGBA(0, 0, 0, 0.06))` 是键帽
   的内阴影（`--keycap` 一族），不在 `Shadow(` 的判据里，也不属于浮动层——别当遗留字面值重领。
