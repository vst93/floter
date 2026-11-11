# P19 报告 · 除终端外，所有页面同一层玻璃

## 用户反馈

「感觉现在设置界面和搜索界面透明度还有颜色不一致？理论上除了终端页，其他页面和内置插件页面都要统一，看着是设置页面的颜色和透明度有差异。」

## 根因

`934b2c94`（P17「界面比例」轮）把设置正文的填充从 `theme.FaceOver(地, Sunken)` 改成了
`ui.Transparent`，并在同一处注释里写明「No fixed paper colour behind the content」。而
搜索页与所有内置插件页都画在 `internal/launcher` 的读取面板上
（`ui.Box(c).Fill().Background(tk.Sunken)`）。两块表面因此不再是同一层玻璃：

- 设置正文的可见 alpha = 滑块本身；
- 搜索/插件页 = `Sunken`（滑块 + 0.09 的一步凹陷）。

同一 47% 设置下实测（720×580，`ui.Tester` 真实渲染，取正文带的中位 alpha）：
设置 189 / 搜索 211。浅色档还额外差一个色调（`Sunken` 的近白薄罩）。

## 落地

`internal/settingsui/settingsui.go` `View` 的根列戴上 `Background(tokens.Sunken)`，
内容列继续透明并继承它。页头带因此也落在同一层玻璃上——两表面的第一个带子不只是同高，
还是同一张纸。侧栏与正文重新有了层级（面差来自正文继承的 `Sunken`，1px 竖发丝线退回成接缝）。

## 验证

- 新增/改名 pin：`internal/shell/redesign_test.go` 的 `TestSettingsAndLauncherShareOneGlass`，
  浅/深 × 常规/会话/快捷键/集成/关于 × 74/86/98：滑块单调到达正文，且设置正文与启动器面板
  alpha **逐值相等**（10 个子用例全绿）。
- `internal/settingsui` 全量（104s）失败集与改前**逐条相同**：82 条，均为既有失败（含
  `TestTheBasePluginCardsAreTheirOwnBoxes`、`TestTheRowFacePaintsItsPaneAndKeyline` 的
  越界 panic 等，回放改前源码同样红）。
- `internal/shell` 全量失败集改前 17 条、改后 17 条：唯一差异是
  `TestTheFloatingTopEdgeIsThreePartsAndTheySwapRoles`——toast 现在压在 `Sunken` 面板上，
  五行字面量按旧 → 新平移并已重定基线；结构判据（影/描边谁在分离、冠线同号、冠线不与描边并线）
  一个数没动。
- `internal/ci` 全绿；`gofmt -l`、`go vet`、darwin/windows/linux 三平台 `go build ./...` 通过。

## V / B / S

- **V**（视觉基线重定）：`TestSettingsGlassFollowsOpacity` → `TestSettingsAndLauncherShareOneGlass`
  （旧断言钉的正是要改掉的行为）；`floatedge_test.go` 五行字面量与浅色面差上限 +6 → +8。
- **B**：无旧行为断言被放宽。
- **S**：无新增 Skip，无删除测试场景。

## 未做（说明）

- 未动终端页：它有自己的 `TerminalSpecForPreferencesOS` 与画布 alpha 链，本就不在这条玻璃上。
- 未把侧栏单独抬成 `Control` 面：试过一版（chrome = `Control`），浅色档侧栏与正文的面差
  只有 3 级、深色档 11 级，反而把「1px 线分家」的问题换成「两个几乎同色的面」；本轮只做用户
  要求的「同一层玻璃」，侧栏层级留待单独一轮。
