# P21 · 分离 / 盯住（detach surface）

用户原话：

> 除了最基础的搜索主页之外，其他的插件页和功能（包括终端页）应该要有"分离"功能，
> 或者叫"盯住"的功能：
> 1. 触发与交互：点击或者用快捷键触发之后，页面将不再自动消失，而是固定在位置上
>    进行展示，相当于和主窗口进行分割。
> 2. 视觉样式：这个分离出来的窗口样式可以做一些特殊的标记或特效，让用户感受到
>    这是被分离或者固定住的。

方案由用户拍板：**C —— 同一份状态，两个窗口各画一个 surface**。快捷键默认
`super+D`（可改）。不做「再次触发」的收放。终端底层逻辑不改。

第二版（本轮的形状）来自用户的纠正：

> 操作太过于割裂了。我希望的是分离之后和原来的浮窗几乎保持一致，只有少量的标识来
> 区分是主窗口还是独立窗口，整体的样式、布局以及交互应该保持不变。可以加一个特殊的
> 窗口边框，或者加一个特殊的标签也可以。

即第一版把分离窗口做成了另一套外观，是错的；它必须是**同一张卡**。见「视觉标记」。

## 为什么是 C

| 方案 | 问题 |
| --- | --- |
| A. 每页各写一个第二窗口 | 5 个页面 × 各自回调 = 大量重复，`detach.go` 会从 890 行涨到 2000+ |
| B. 第二个 `shell.App` 实例 | `terminalui.App` 各自持有 session 列表，**终端会话没法共享** |
| **C. 同一份状态，两个窗口各画一个 surface** | 状态只有一份：终端会话、设置草稿、launcher 查询都不需要同步 |

C 的代价是「一个 launcher App 不能同时画两页」（mode 是单值），所以**插件页**分离
时需要第二个 `launcher.App` 实例；设置与终端各自的 App 天然只被一个窗口绘制，直接
让位即可。

## 改动

| # | 位置 | 改法 |
| --- | --- | --- |
| 21-1 | `internal/settings/shortcutmap.go` | 新增 `ShortcutDetachSurface = "detach_surface"`，进 `ShortcutActions`；`DefaultDetachShortcut()` 返回平台修饰键 + `D` |
| 21-2 | `internal/i18n` | `Settings.SurfaceDetach` / `SurfaceDetachHint`（设置页头的图钉键）、`Launcher.DetachedMark`（分离窗口 bar 上的「已固定」） |
| 21-3 | `internal/shell/detachedsurface.go`（新） | `detachedSurface` 结构、`detachSurface()`、`detachedSurfaceView()`、`detachedSurfaceBar()`、`closeDetachedSurfaces()`、`forgetDetachedSurface()` |
| 21-4 | 同文件 | `canDetachSurface()`：设置 / 终端 / launcher 的插件页与配置浮层为真，普通搜索列表为假 |
| 21-5 | 同文件 | `detachLauncherPage()`：插件页分离时建第二个 `launcher.App`，用 `detachedLauncherActions()`（把 `ResizeTo`/`DetachOutput`/`PinText` 置 nil）+ `seedDetachedLauncher()` 播种目录与页面 |
| 21-6 | `internal/launcher/fieldkeys.go` | 新增 `LeavePluginMode()`、`PluginTitle()`、`AdoptPage(from)` |
| 21-7 | `internal/shell/shell.go` | `appShortcuts()` 里先问 detach chord（`canDetachSurface()` 为真时）；`quit` 里 `closeDetachedSurfaces()`；`Options.OpenDetachedSurface` 测试钩子；`settingsui.Actions.PinSurface` 接线 |
| 21-8 | `internal/settingsui/settingsui.go` | 页头加图钉键（`headerButton("pin", …)`），在电源与关闭之前 |

## 两条已成立的规则（不是本轮新加）

- **分离窗口不自动消失**：`HideOnBlur` 只挂在 `a.Win` 上（`shell.go:1443`），第二
  窗口天然不受它管。参考实现的 `hideOnBlurApplies(windowLabel, hideOnBlur)`
  （`main:src/plugin-window/detach.ts`）是同一条规则：只有 `main` 窗口会因失焦隐藏。
- **分离窗口在面板隐藏时活着**：`closeDetachedSurfaces()` 只在 `quit` 里调用。

## 视觉标记

用户否掉了第一版：「操作太过于割裂了。我希望的是分离之后和原来的浮窗几乎保持一致，
只有少量的标识来区分是主窗口还是独立窗口，整体的样式、布局以及交互应该保持不变。」

第一版把分离窗口做成了**另一套外观**：`tokens.Opaque` 实心底，加一条自己的 bar
（`● 已固定 │ Settings ×`）。那条 bar 是面板从来没有的第二层 chrome——它把页面往下
推、和页面自己的页头竞争，让分离窗口成了「另一个东西」而不是「同一个东西换了位置」。

现在分离窗口**就是面板本身**：

| | 做法 |
| --- | --- |
| 卡片 | `viewWindow(c, surf, detached, alt)`——和面板**同一个函数**画的同一张卡：同样的玻璃、圆角、insets、sheen、`FaceFillAt`、`DrawPanelEdge`/`DrawCardEdge` |
| 页面 | 卡片下面直接是 `a.Settings.View(c)` / `a.Terminal.View(c)` / 第二个 `launcher.App`，没有中间层 |
| 描边 | 面板是 `InputStroke`（聚焦时 `InputStrokeActive`），分离窗口是 `AccentEdgeStrong`（dark `rgba(143,183,255,0.68)`、light `rgba(6,74,166,0.68)`）——一行 1px 的差别 |
| 标题 | 窗口标题是 `已固定 · Settings · floter`（`detachedWindowTitle`），走的是窗口本来就有的 chrome，对页面布局零成本 |

**为什么是 `AccentEdgeStrong` 而不是 `Theme().Accent`**：后者是**墨**的 accent，
`internal/theme/readability.go` 里 `p.accent = readableInk(p.accent, toward, 5.5, backs)`
会把它一路推向外观的极值以凑够 5.5 对比度——深色档最终落在**纯白**。1px 的纯白边框
读起来是一条生硬的异物边，不是这个应用的强调色（实测 `{255 255 255 255}`，而面板的
`InputStroke` 是 `{23 23 23 23}`）。`AccentEdgeStrong` 是同一个色相、应用本来就在用的
强调**边**透明度，实测深 `{103 132 184 184}` / 浅 `{4 50 113 173}`。

**除描边外逐像素相同**（720×560，`General` 页）：第 2 行起 panel 与 pinned 都是
`{22 22 22 207}`（深）/ `{199 199 200 207}`（浅）。

## 判据

- `internal/shell/detachsurface_test.go`
  - `TestOnlyPagesCanBePinnedNotTheSearchList`：搜索列表与已输入查询为假；剪贴板页 /
    设置 / 终端为真
  - `TestPinningTheSettingsSendsThePanelBackToTheSearchList`：分离后 `a.Surf` 回到
    `SurfaceLauncher`
  - `TestPinningTheTerminalKeepsTheSameSession`：`a.Terminal` 指针不变（终端底层未改）
  - `TestPinningAPluginPageAdoptsItIntoASecondLauncher`：面板退出模式、第二个实例进入
    同一模式且字段文本一致
  - `TestTheDetachChordIsRebindable`：默认值是 `DefaultDetachShortcut()`，且
    `SetShortcut` 能改
  - `TestAPinnedPageKeepsThePanelsShapeAndOnlyItsStrokeChanges`：分离窗口里是**完整
    的设置页**（`General`、`Appearance` 在），页面内**没有**任何 band 承载
    `DetachedMark`（标记属于窗口标题与卡片描边，不属于页面布局），且
    `detachedWindowTitle` 以标记开头并含页面名

## 未做

- 终端页没有页头图钉键：终端 bar 已有 4 枚键，再加会挤；它的入口是快捷键
- 分离窗口不设 `Panel`/`Frameless`/`SkipTaskbar`：它是普通窗口，这正是它不被
  面板的 `HideOnBlur` 管辖的原因；玻璃选项（`Transparent`、`Vibrancy`、
  `DisableShadow`）与面板同一套
- 不做「再次触发把窗口收回来」（用户明确说是笔误）
- 插件页分离时不做滚动位置/选中行的镜像：分离页从自己第一行开始
  （与 `showOutput` 重置滚动、`AdoptPage` 只搬页面不搬视图状态是同一条规则）
