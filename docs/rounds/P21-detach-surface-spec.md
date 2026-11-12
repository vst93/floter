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

分离窗口顶部一条 bar，与另外两个第二窗口（`detach.go` / `pinned.go`）同一族：

- 前导一枚 6u 的 accent 圆点 + 「已固定」（`DetachedMark`），然后一条 1px `Hairline`
  竖线，再是页面自己的名字
- 尾部一枚 28u 透明方键（`×`）关窗
- 整条 bar 底边一条 `InsetShadow(0, -1, 0, 0, tokens.Hairline)`

accent 只用在圆点上：§5 的判据是 accent 只能回答「哪个被选中」，这里它回答的是
「这一页是固定的」，与列表选中行同一个词。

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
  - `TestThePinnedPageWearsTheMarkThatSaysItIsPinned`：分离窗口的 bar 上同时有
    `DetachedMark` 与页面名

## 未做

- 终端页没有页头图钉键：终端 bar 已有 4 枚键，再加会挤；它的入口是快捷键
- 不做「再次触发把窗口收回来」（用户明确说是笔误）
- 插件页分离时不做滚动位置/选中行的镜像：分离页从自己第一行开始
  （与 `showOutput` 重置滚动、`AdoptPage` 只搬页面不搬视图状态是同一条规则）
