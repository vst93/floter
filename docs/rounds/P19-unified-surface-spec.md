# P19 · 除终端外，所有页面同一层玻璃

用户反馈：「设置界面和搜索界面透明度还有颜色不一致；理论上除了终端页，其他页面和内置插件页面都要统一。」

## 现状（改前）

- 搜索页与全部内置插件页（剪贴板、浏览器、计算器…）都画在启动器的读取面板上：
  `ui.Box(c).Fill().Background(tk.Sunken)`（`internal/launcher/launcher.go`）。
- 设置页在 `934b2c94` 里把正文的 `FaceOver(地, Sunken)` 换成了 `ui.Transparent`，
  于是它的可见 alpha 就是滑块本身（74/86/98 → 189/215/241），比搜索页薄一档；
  同时侧栏与正文落在同一个 z 平面上（`Sunken` 面板被拿掉后，两栏只靠 1px 竖发丝线分家）。
- 结果：同一个透明度设置，切换页面时透出的桌面多少与色调都变了。

## 落成

| # | 位置 | 改法 |
| --- | --- | --- |
| 19-1 | `internal/settingsui/settingsui.go` `View` 的根列 | `Background(tokens.Sunken)`：整张设置卡（含页头带）与启动器面板是**同一个 token**，于是两处玻璃逐像素同一层 |
| 19-2 | 同处 内容列 | 保持 `Background(ui.Transparent)`，继承根列那层 `Sunken`；分组卡仍是它上面的 `Control` 面 |

不改：终端页（自带 `TerminalSpecForPreferencesOS` 与画布 alpha，本就不在这条链上）、
窗口框（`glassmap` 的滑块链）、浮层（`Float`）、卡片与控件（`Control`）。

## 判据

`internal/shell/redesign_test.go` 的 `TestSettingsAndLauncherShareOneGlass`（改名自
`TestSettingsGlassFollowsOpacity`）：浅/深 × 五个设置页 × 74/86/98 三个滑块位，
① 滑块单调到达设置正文；② 设置正文与启动器面板的可见 alpha **逐值相等**。

重定基线（V）：旧断言是「正文 median ≤ 195，即比窗口更透」，钉的正是本次要改掉的行为。
`internal/shell/floatedge_test.go` 的五行字面量随「toast 现在压在 `Sunken` 面板上」平移
（浅 `{248,237,227,255,250}` → `{244,233,228,255,251}`、深 `{24,23,46,71,25}` → `{25,24,46,71,25}`），
结构判据（谁在分离、冠线同号）一个数没动；浅色「面 vs 地面」上限 +6 → +8，因为面板比窗口地厚一档。

## 证据

`internal/settingsui` 全量失败集与改前逐条相同（82 条，既有失败），`internal/shell` 除本轮的
浮层顶边重定基线外无新增失败；`internal/ci` 全绿；`gofmt`、`go vet`、三平台 `go build` 通过。
