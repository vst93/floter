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
随后又要求：「需要优化下现在独立后的系统菜单栏，可以有创意点，只要能和现在风格融合好。」

第一版把分离窗口做成了**另一套外观**：`tokens.Opaque` 实心底，加一条自己的 bar
（`● 已固定 │ Settings ×`）。那条 bar 是面板从来没有的第二层 chrome——它把页面往下
推、和页面自己的页头竞争，让分离窗口成了「另一个东西」而不是「同一个东西换了位置」。

现在分离窗口**就是面板本身**，另加一条**属于窗口自己的标题栏**：

| | 做法 |
| --- | --- |
| 卡片 | `viewWindow(c, surf, detached, alt)`——和面板**同一个函数**画的同一张卡：同样的玻璃、圆角、insets、sheen、`FaceFillAt`、`DrawPanelEdge`/`DrawCardEdge` |
| 页面 | 卡片下面直接是 `a.Settings.View(c)` / `a.Terminal.View(c)` / 第二个 `launcher.App`，没有中间层 |
| 标题栏 | 窗口用 `TitleBarStyle: TitleBarHiddenInset`（macOS）/ `TitleBarHidden`（其他）隐藏系统栏，改由 `detachedTitleBar` 在**卡片自己的面内**、页面之上画一条：`● 已固定 │ 页面名` + 一条 `HairlineSoft` 底线，整条可拖窗。左侧让开系统红绿灯（`c.TitleBar().Left`），右侧让开窗口按钮 |
| 描边 | 面板是 `InputStroke`（聚焦时 `InputStrokeActive`），分离窗口是 `AccentEdgeStrong`（dark `rgba(143,183,255,0.68)`、light `rgba(6,74,166,0.68)`） |
| 窗口标题 | `已固定 · 页面名 · floter`（`detachedWindowTitle`） |

**为什么标题栏画在卡片内**：画在卡片外时，它的底是窗口的透明背景（露桌面），于是
「一条浮在上面的灰条 + 下面一张卡」——正是用户说的割裂。画在卡片的 `face` 之内，bar
和页面就是同一块玻璃，accent 描边绕两者一圈，读成一个对象。

**为什么是 `AccentEdgeStrong` 而不是 `Theme().Accent`**：后者是**墨**的 accent，
`internal/theme/readability.go` 里 `p.accent = readableInk(p.accent, toward, 5.5, backs)`
会把它一路推向外观的极值以凑够 5.5 对比度——深色档最终落在**纯白**。1px 的纯白边框
读起来是一条生硬的异物边（实测 `{255 255 255 255}`，而面板的 `InputStroke` 是
`{23 23 23 23}`）。`AccentEdgeStrong` 是同一个色相、应用本来就在用的强调**边**透明度，
实测深 `{103 132 184 184}` / 浅 `{4 50 113 173}`。

**除描边与标题栏外逐像素相同**（720×560，`General` 页）。

## 只能整个退出（21-9）

用户：「独立出来的窗口不应该能回退到搜索页，不能通过删除、esc 回退，只能整个退出。」

三条回退路径，全部堵住：

| 路径 | 堵法 |
| --- | --- |
| Escape 的模式阶梯 | `launcher.App.pinned` 标志；`leavePluginMode()` 直接 `return false`，Escape 那一支在 `pinned` 时整体 `return`（不清查询、不 Dismiss） |
| 空字段退格 | 走的是同一个 `leavePluginMode()`（`fieldkeys.go` 的「退格删空即停」），同一个标志 |
| 设置/终端页的 Close（Escape 的最后一档） | `Close: a.surfaceExit`——`a.detachedDrawing` 为真时什么都不做 |

`detachedDrawing` 是绘制期的标志：输入是在建树时读的，页面的 Escape 处理就跑在这次
`detachedSurfaceView` 调用**之内**，两个 App 又是与面板共享的，没有 per-window 的
Actions 可以承载这个答案。页面**内部**的阶梯照常工作（取消确认、放弃录制、关掉集成
详情），只有「离开这一页」那一档被扣住。

## 保留当前操作结果（21-10）

用户：「独立切换时要保留当前的操作结果，最好就是把插件本体独立出来，不影响接着操作。」

`AdoptPage(from)` 搬的不只是模式，还有**内容**：查询、选中行、已算出的行表
（`drawn`）、模式自己的数据（`calculatorFound`、`browserFound`）、输出视图与滚动状态。
所以分离窗口打开时是面板那一刻的同一份列表、同一个选择，下一次按键接着做，而不是
重新开始。

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
    的设置页**（`General`、`Appearance` 在），`DetachedMark` 在整帧里**只出现一次**
    （窗口标题栏，不是页面布局里的一条 band），且 `detachedWindowTitle` 以标记开头
    并含页面名
  - `TestAPinnedPageCannotWalkBackToTheSearchList`：Escape 不离开模式、不改字段；
    空字段退格不离开模式；`surfaceExit` 在分离窗口绘制期间不动主窗口（21-9 未修时
    报「Backspace on an empty field left the pinned page's mode」）
  - `TestAPinnedPageOpensWhereThePanelWas`：分离窗口的字段与选中行都是面板那一刻
    的值（21-10）

## 未做

- 终端页没有页头图钉键：终端 bar 已有 4 枚键，再加会挤；它的入口是快捷键
- 分离窗口不设 `Panel`/`Frameless`/`SkipTaskbar`：它是普通窗口，这正是它不被
  面板的 `HideOnBlur` 管辖的原因；玻璃选项（`Transparent`、`Vibrancy`、
  `DisableShadow`）与面板同一套
- 不做「再次触发把窗口收回来」（用户明确说是笔误）
- 插件页分离时不做滚动位置/选中行的镜像：分离页从自己第一行开始
  （与 `showOutput` 重置滚动、`AdoptPage` 只搬页面不搬视图状态是同一条规则）
