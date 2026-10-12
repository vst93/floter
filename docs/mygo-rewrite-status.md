# floter × mygo 重构：当前进度与后续规划

> 交接文档。状态时间：2026-10-11，分支 `mygo-rewrite`（已推送）。
> **仓库现在只有 Go 实现**：Tauri/Rust/React 时代的代码已删除（P6-b）。
> 详细的分轮记录见 `docs/mygo-rewrite-plan.md`（含每轮的决策与「未做」清单）。

## 一句话状态

用 mygo（原生 GPU 自绘 UI + Go）重写的 floter **已经是一个可打包、可运行的完整应用**：单窗口三表面
（启动器 / 设置 / 终端）、搜索内核、扩展平台（含后台运行、列表协议、导入导出、权限审计）、剪贴板、
浏览器、计算器、系统集成、打包与 CI 全部就位。**556 项测试全绿**，三平台都能构建，Linux 打包
（可执行文件 + .deb + tar.gz）与 macOS 打包（.app + .dmg）都已在本机跑通，并在真实数据上验证过
（旧扩展仓库、剪贴板历史、扩展配置）。仓库里只有 `cmd/` + `internal/` 的 Go 实现（约 30k 行，
含测试）。

## 快速上手（换到新电脑后）

```sh
git clone git@github.com:vst93/floter.git && cd floter
git checkout mygo-rewrite

# 门槛（全部必须过）
gofmt -l cmd internal          # 必须为空
go vet ./...
go test -count=1 ./...         # 556 项测试
GOOS=linux go build ./... && GOOS=windows go build ./... && GOOS=darwin go build ./...

# 开发运行（打开真实窗口）
go run ./cmd/floter                 # 默认启动器
FLOTER_OPEN=settings go run ./cmd/floter
FLOTER_OPEN=terminal go run ./cmd/floter

# 打包（CLI 是 go.mod 里的 tool，无需 npm/bun/node）
go tool mygo build                                  # 本机平台
go tool mygo build -platform linux/amd64,windows/amd64,darwin/universal

# 可选的真环境测试（会下载/加载原生库）
FLOTER_TERMINAL_TEST=1 go test ./internal/terminalui      # libghostty-vt 真会话
FLOTER_SQLITE_TEST=1  go test ./internal/browser ./internal/extensions  # sqlite 真实读写
FLOTER_REGISTRY_TEST=1 go test ./internal/extensions -run TestRealRegistry  # 真 npm registry
```

约定：`gofmt` / `go vet` 必须净；三平台都能 `build`；每轮改动自带测试；用户数据文件（settings / extension-repository / clipboard-history）**磁盘格式不变**。

## 目录结构（全部实现，约 15k 行）

| 包 | 职责 |
|---|---|
| `cmd/floter` | 入口：单实例、settings 加载、`WhenReady` 起壳、深链回调 |
| `internal/shell` | 应用本体：单窗口三表面切换、窗口尺寸/可调性、玻璃面板、托盘、应用菜单、全局快捷键、深链、登录项、剪贴板监听、浏览器搜索、钉住窗口 |
| `internal/launcher` | 启动器：搜索/排序、结果虚拟化列表、计算器行、应用行、系统命令行、最近使用、扩展命令与参数模式、剪贴板模式、浏览器模式 |
| `internal/settingsui` | 设置面：侧栏路由（General/Sessions/Shortcuts/Integrations/About）、真控件、写回、集成列表与权限/检测/卸载、快捷键录制 |
| `internal/terminalui` | 终端面：`plugins/terminal` 会话、外观映射、钉住输出、命令运行 |
| `internal/settings` | settings.json 读写与规范化（含终端外观、窗口行为、UI 缩放、透明度、玻璃档），未知键原样保留 |
| `internal/theme` | theme + ui_scale → `*ui.Theme`（含半径 token） |
| `internal/i18n` | en/zh 全量文案（结构化 Copy，缺字段编译期报错）+ 权限名 |
| `internal/glassmap` | 玻璃档 → 插件材质 + 雾层（以及为什么 blur/饱和没法复刻的记录） |
| `internal/calc` | 表达式求值（计算器行） |
| `internal/apps` | 应用扫描（.app/.lnk/.desktop，含 Info.plist 名字与别名、图标）+ PATH 命令扫描 |
| `internal/plist` | Apple property list 读取（XML 与 bplist00） |
| `internal/pinyin` | 常用字的拼音首字母表（GB2312 一级集） |
| `internal/calculator` | 计算器历史（fold 去重、收藏豁免、写时淘汰、原子写） |
| `internal/tools` | 工具安装目录（12 工具 × 平台配方、8 个包管理器探测） |
| `internal/drops` | 拖放路径规范化 + shell 引用 |
| `internal/spawn` | 启动即忘的程序启动（浏览器/静默命令/终端模拟器） |
| `internal/clipboard` | 剪贴板历史（text/image/files、去重、容量/30 天保留、原子写） |
| `internal/browser` | 浏览器历史/书签（Chrome/Safari/Firefox，只读 SQLite） |
| `internal/extensions` | 扩展平台：manifest、仓库（未知键保留的原子读写）、inventory、运行时解析、describe/complete/diagnose 协议、命令模式数据、配置注入、本地/npm 安装、权限审批 |
| `internal/shortcuts` | 加速键两种拼写的互转与录制 |

## 已完成（按区域）

**三表面与搜索**（P0–P2）
- 单窗口三表面（启动器/设置/终端），按壳改尺寸与可调性、失焦隐藏 + 页面驻留、`last_settings_page`。
- 搜索：应用扫描（macOS 读 `Info.plist` 与本地化名字 + 别名 + 图标、Linux `.desktop`、Windows `.lnk`）、
  算式求值、PATH 命令（可开关）、最近使用、扩展命令与参数模式、剪贴板/浏览器/计算器三种内置模式
  （触发词可直接进入，含旧版中文词），结果虚拟化、编号快捷键 ⌘1–⌘0、行图标与 120ms 淡入。
- 玻璃保真：材质阶梯 + 雾层 + `ScrollEdge`（读源码证伪了「用 glass.Blur 复刻桌面模糊」）。

**扩展平台**（P3）
- 与旧版同一份磁盘布局（`extension-repository.json` 未知键原样保留、包目录、`extension-data`）。
- manifest / inventory / 运行时解析 / describe·complete·diagnose 协议 / 命令参数模式与动态补全。
- 配置注入（envVar/argument，口令只进环境）、本地与 npm 安装（semver、SRI、安全解包）、权限审批
  （批准绑定清单 sha256）、orphan 接管与删除、命令级开关、后台运行与输出回看（含列表协议）、
  完成通知、权限审计、导入导出（密钥绝不进文件）。

**内置插件与系统集成**（P4）
- 剪贴板历史（text/image/files、去重、收藏与删除、容量与保留）、计算器历史（fold、收藏豁免、写时淘汰）、
  浏览器（三平台 profile 表、目标/排序/搜索范围、标签页、自定义目录）。
- 单实例、`floter://` 深链、托盘（跟随设置）、登录项、应用菜单、全局快捷键与自定义快捷键、
  终端会话快照与尺寸写回、钉住输出到独立窗口。

**打包与质量**（P5/P6）
- `go tool mygo build` 三平台产物（macOS .app/.dmg、Linux .deb/tar.gz+install.sh、Windows .exe）；
  CI 矩阵跑 gofmt/vet/build/test 并打包上传；About 页更新检查（无更新源时如实说明）。
- 一帧 67–516 µs（headless 基准），二进制 20 MB / 14 MB（stripped），常驻内存 166 MB
  （同环境 mygo 最小窗口 137 MB）。

## 最近几轮（详细记录见 plan）

**P7-aa 图标控件的 tooltip（本轮）**
- **偏离点**：旧版给每个只有图标、没有可见文字的工具栏控件都配了 `title=` 的一句话
  （`settings.quitHint` / `settings.closeHint` / `terminal.newCommandHint` / `terminal.openInTerminalHint`）。
  本构建把这四句文案都写好了（en/zh 齐全），却**没有任何视图画过它们**——设置页头部、
  终端标题栏的退出 / 关闭 / 新命令 / 在终端打开都是光秃秃的字形。全仓此前只有一处 `Tooltip` 调用
  （P7-t 给 `␣` 键帽补的那处），所以这类“文案写好但没人展示”的漏洞只剩这一批。
- **修法**：`settingsui.headerButton` 与 `terminalui.barButton` 各多收一个 `hint`，落到
  `Element.Tooltip(hint)`；接上 `QuitHint` / `CloseHint` / `NewCommandHint` / `OpenExternalHint`。
  `Tooltip` 在元素没有 `Description` 时把它当无障碍说明，`Label` 仍是无障碍名字，两者不冲突。
- **测试**：`TestSettingsHeaderButtonsCarryTheirHints`（退出 / 关闭：悬停、停 700ms、tip 出现）、
  `TestTerminalBarButtonsCarryTheirHints`（新命令 / 在终端打开；标题栏先因悬停揭示，再让按钮接手悬停）。

**P7-z 行的 source 槽**
- **偏离点**：旧版 `row-content.ts` 的 `resultRowContent` 给每行算出两个可选字符串——`subtitle`
  与**右侧固定槽** `source`（`.launcher-result__source`）。`source` 对 `command` 行是**贡献该命令的
  集成名**，对 install 行是**包管理器**。本构建此前把这两个事实塞进了 `Detail`
  （`detail = description || IntegrationName`），install 行则把管理器放进了 `Shortcut` 槽——
  同一格既当键位又当出处。于是有描述的命令行**看不到**它来自哪个集成。
- **修法**：`launcher.Item.Source`；`commandItems` 的 `Detail` 只放 description、`Source` 放
  `IntegrationName`（有描述时两者并存，而不是一个顶替另一个）；`toolInstallItems` 的
  `Source = tools.ManagerName(...)`（`brew` → `Homebrew`，管理器的 proper noun），`Shortcut` 留空；
  命令模式的补全行同样带 `Source`。
- **右端成组**：行的右端按旧版顺序：**警告点、`␣` 键帽、source、收藏星、行控件、编号、键位**。
  之前警告点与 `␣` 键帽画在标题行**内**（标题文字之后、行中间），现在它们是文本列的**兄弟**，
  因此窄窗口的省略号先吃掉标题而不是它们——这正是「出处」最需要可见的时候。
- **文本寄存器单行**：标题与副标题现在 `SingleLine()`（旧版 `.launcher-result__title` /
  `__subtitle` 的 `white-space: nowrap; text-overflow: ellipsis`）。此前它们是普通文本，超长时会
  **换行**——画出的行比窗口记账的高，把列表推出玻璃。source 槽又把文本列变窄，所以这一步与它同做。
- **测试**：`TestCommandRowCarriesItsIntegrationAsItsSource`、`TestCommandModeRowsCarryTheSource`、
  `TestInstallRowNamesTheManagerAsItsSource`、`TestManagerName`、
  `TestTheSourceWordSitsAtTheRowsRightEdge`（source 在标题右侧、编号徽标左侧）、
  `TestTheRowsMarksSitAtTheRightEdge`（`␣` 在标题与副标题的右外侧）、
  `TestLongRowTextIsCutNotWrapped`（超长标题/副标题仍是一行）。

**P7-y 设置/配置/插件表单的每一行都带自己的第二行说明（本轮）**
- **根因**：`settingsui.checkbox` 是唯一一个把行的说明丢掉的控件助手——它调用 `a.row(c, label, "", ...)`，
  于是所有**开关行**的第二行（旧版 `SettingsRow` 的 `sublabel`、插件表单的 `field.help`）都被丢掉：
  General 页的四个开关（登录自启 / 失焦隐藏 / 显示常用应用 / 菜单栏图标）只剩一个光秃秃的标签，
  而它们的 `i18n` 文案（`LaunchAtStartupHint` 等）明明已经写好、只是没人画；
  Integrations 页的「在搜索里显示 PATH 命令」同理。同一处 bug 还波及两处**声明式表单**：
  集成配置表单的布尔字段（`config.go` 里 `description` 算出来了却没传给开关）与插件配置表的
  toggle（`plugincfg.Field.Help` 对 select/slider/text 都传了，只有 toggle 没传）。
- **修法**：`checkbox` 增加 `description` 参数并落到 `a.row` 的第二行；8 个调用点各自补上自己的说明
  （General 四个开关、Integrations 的 PATH 开关、终端的光标闪烁传空、配置表单的布尔字段传
  `description`、插件表单的 toggle 传 `field.Help`）。开关行与其它行从此同一种形状。
- **顺带**：外观行的 `Theme` 此前也是裸下拉框（旧版有 `settings.themeHint`「自动模式跟随系统外观。」），
  补上 `i18n.ThemeHint`（en/zh）并传进 `pick`。
- **测试**：`TestSettingsRowsKeepTheirHints`（General 页的四个开关说明 + 主题说明都在，切到
  Integrations 后 PATH 开关的说明也在）；`TestPluginSheetRendersAndReportsChanges` 追加
  `BrowserPluginHint`（toggle 的 help 真的画出来）；`TestConfigurationForm` 给布尔字段加说明并断言它出现。

**P7-x 配置表打开时 chips 行随列表一起退场（本轮）**
- **偏离点**：旧版 R41 的原文——用户看到「进入设置界面，中间的状态切换栏还是会显示出来」，
  要求筛选 chips 属于**下面的列表**而不是上面的搜索框——在旧版是 `pluginFilterRowVisible` 的第三条
  条件：`configOpen` 为真时 chips 行不画，窗口高度也少收这一带。本构建的 `filtersVisible()` 只问
  `output != nil`，配置表打开时 chips 行照旧画在表的上方，表体还被 `listEdge = edge + chips` 往下
  推了 5u，而 `Geometry.Height()` 在 `Config != nil` 时**根本不收 chips 带**——画出来的比记账的多。
- **接线**：`filtersVisible()` 加 `a.configOpen()` 一条；配置表打开时 chips 行整行消失，表体从
  `edge` 起画，和高度预算对齐（顺带修掉那 5u 的溢出）。
- **齿轮回到字段行**：旧版把插件齿轮放在**字段行的行尾**（`.collapsed-card__settings--plugin`，
  打开时图标从滑杆翻成 ✕），且插件模式下字段行只有它——会话/设置两个按钮不是插件动作、
  在这里让位。本构建此前把齿轮放在 chips 行里（chips 一隐藏就没有唯一的关闭控件了），
  现在 `pluginGear` 画在字段行行尾，`filterPluginID()` 为空时才回到 `fieldButtons`。
- **测试**：`TestConfigSheetHidesTheChipsRow`（齿轮在 chips 之上、插件模式没有会话/设置、
  打开后 chips 消失而表体与 ✕ 在、Esc 后 chips 回来）；既有 `TestModeOptionsGear`/`TestFieldRowButtons`
  继续钉住齿轮的开关与普通页的两个按钮。

**P7-w 声明式输入的行内校验真的接线了（本轮）**
- **根因**：文档里 P7-o 说「选择型缺选项、布尔型缺旗标，都会在该行下面印一条灰色的警告行」，
  但**没有任何调用点**：`extensions.ParamRowIssues`、`settingsui.issueUnder`、
  `i18n.Settings.ParamNeedsOptions`/`ParamNeedsFlag` 三样都在仓库里躺着，没有一行视图代码调用
  它们（`ParamRowIssues` 自己的注释写「没有 id 的行不产生 issue」，代码却给它写了一个 `"id"`
  ——正因为没人用，这处自相矛盾才一直没被发现）。表单照旧把笔误弹回后端解释。
- **接线**：`toolParamsEditor` 每帧用 `editorParams()`（与编辑器逐行对应、空 id 保留）算出
  `ParamRowIssues`，把原因映射成 `i18n.Settings` 的文案、用 `issueUnder` 印在**导致它的那一行
  下面**；`localToolSection` 的保存按钮在有任何 issue 时**直接不提交**——行下已经说明原因，
  不再让后端回传错误。
- **补齐规则与文案**：`ParamRowIssues` 现在返回具名的 `ParamRowIssue`，规则对齐后端
  `ValidateParams`：非法 id、重复 id、选项型无选项、布尔型无旗标、不是单个 token 的旗标；
  重复 id 只怪**后出现的那一行**（旧版行为），空 id 的行不算声明、不产生 issue。新增
  `ParamInvalidID`/`ParamDuplicateID`/`ParamInvalidFlag` 三处中英文案（旧版 `customParamInvalidId`
  等的原文），与已有的 `ParamNeedsOptions`/`ParamNeedsFlag` 一起用。
- **测试**：`TestParamRowIssues`（好声明无 issue；非法 id / 无选项 / 无旗标 / 坏旗标各自的原因；
  重复只怪后者；空 id 不产生 issue）、`TestConnectFormNamesAnUnfinishedInputAndRefusesToSave`
  （行下真的印出「A choice needs its options.」；点连接不提交；补上选项后行清空且提交成功）。

**P7-v 终端配色的预览（本轮）**
- **偏离点**：旧版 R43 的 `TerminalPalettePreview`——用户在选配色时要求「这个配色最好可以在下方
  或者边上有一个大概的预览」——在设置页的终端配色行下画一个**迷你终端**（三行静态文本，用该
  配色自己的背景/前景/光标/选区上色）。本构建的终端配色只是一个裸的下拉框：选了「Ink」还是
  「Paper」要回到终端才知道，配色行是设置页里唯一一个「选了看不到结果」的控件。
- **补上预览**：`terminalui.PaletteColorsFor(name, dark)` 交出某个配色 id 画的四色（背景/前景/
  光标/选区，背景不透明——预览讲的是颜色，不是透明度）；`settingsui.paletteRow` 把下拉框放在
  行里、把迷你终端放在**行下**（`row` 的 `below` 槽，卡片内、永不是浮层），三行是 `$ floter
  --version` / 选中色的 `0.3.5-preview` / `$` + 块光标，等宽字体。
- **`inherit` 跟外观**：`inherit` 没有固定配色，预览取终端自己的默认主题（`terminal.DarkTheme()`
  / `LightTheme()`，光标与选区为零时回落到前景，与插件的规则一致），随明暗切换重画——对应旧版
  预览读 `--terminal-*` token 的行为。
- **同帧跟随**：下拉框改变的**那一帧**预览就用新配色重画（`shown` 取当帧的 `chosen`），不是等
  下一帧。
- **测试**：`TestThePaletteRowPreviewsItsColors`（Ink 的预览背景就是 `0x0b0d10`；换 Paper 后
  重画）、`TestThePalettePreviewFollowsTheAppearanceForInherit`（inherit 随明暗换背景）。

**P7-u 内置插件的门（本轮）**
- **根因**：浏览器模式在插件关闭时的空态文案是「浏览器书签与历史已停用。可在“设置 > 集成”中启用。」
  （`copy.BrowserDisabled`，中英文都这么写）——但设置页的 Integrations **根本没有内置插件区**：
  没有开关，也没有打开内置插件配置的按钮（`settingsui` 里甚至留着一个从未被调用的 `calculatorCard`，
  是已删除的 Plugins 页的残骸）。文案在承诺一扇不存在的门。
- **补上「基础插件」区**（旧版 Integrations 页自己的 `settings.plugins.baseSection`）：三个内置插件各一张
  卡片，名字 + 一句说明 + 剪贴板自己的隐私行（警告色），右侧是它有的控件——开关（剪贴板/浏览器，
  计算器没有持久化开关就不画假开关）与「配置」按钮。行名是点击目标，与设置页其它开关行一致。
- **统一入口**：新增 `shell.openPluginConfig(id)`——离开当前表面（含设置）回启动器、进入该插件自己的
  模式、再叠上通用配置覆盖层。进模式不是装饰：启动器的 `syncConfig` 一旦发现「当前模式不是这张表
  的插件」就会关掉表，所以模式是表的**前置条件**。设置页的「配置」按钮走的就是这条路。
- **`plugin:calculator` 从未接线**：`settings.CustomPluginCalculator` 常量一直存在（启动器的齿轮、
  配置表都在用），但 `shell.openPluginMode` 的 switch 只认剪贴板与浏览器——用户绑了
  `plugin:calculator` 只会得到一行日志；快捷键页的动作下拉里也**没有**这一项（旧版
  `CUSTOM_SHORTCUT_PLUGINS` 三项齐全）。补上 switch 分支与下拉项，并加一条走**下拉表本身**的测试：
  表里每个 `plugin:` 动作都必须开出一个模式，第四个条目不可能再悄悄溜过。
- **清理**：删掉被 P7-c 遗留的死代码（`calculatorCard`/`setCalculator`/`setBrowser`/settingsui 的
  `browserTargets` 与 `Actions.BrowserTargets`）。
- **测试**：`TestBasePluginsSectionOpensConfigurationAndToggles`（文案、隐私行、配置回调、开关回调、
  计算器无开关）、`TestBasePluginsSectionNeedsAnAction`（没有动作就不画，不承诺空控件）、
  `TestOpenPluginConfigEntersTheModeAndOpensTheSheet`（三个插件：表面、模式、表、下一帧仍在）、
  `TestOpenPluginConfigIgnoresAnUnknownPlugin`、`TestEveryPickerPluginActionOpensAMode`。
- **一处有意的差异**：旧版 `openPluginConfig` 在浏览器插件关闭时**拒绝**并弹一条提示（它的覆盖层
  在开关背后）。本构建的配置表**第一项就是开关本身**，拒绝反而是死路（用户无法从设置页开启它），
  所以照常打开；开启/关闭仍由行上的开关与表里的开关共同负责。

**P7-t 触发词进模式**
- **根因**：字段自己的提示说「一个空格进入该命令的模式」（`TriggerHint` 的「· 进入」），
  直出命令的行尾还有一颗 `␣` 键帽（R65）——但 `syncTypedMode` **只认内置模式**（剪贴板/浏览器/
  计算器/文件），外部命令的触发词路径从未接线。于是提示与键帽都在承诺一个不存在的转换：
  敲 `jv ` 只是给查询加了个空格。
- **修法**：`syncTypedMode` 的 `default` 分支接上外部命令——`jv ` 进入该命令的模式（触发词被消耗，
  其后的内容成为字段里的 argv，`jv -f ` 进去就是 `-f`），与旧版 `externalPluginModeEntry` 同一读法。
  词表复用提示自己的 `commandTriggers`（id / 名字首词 / 别名首词），两处**不可能再分歧**；
  内置模式仍先占命名空间，与它们各自的解析器一致。
- **键帽的 tooltip**：`␣` 的注释声称「说明在行内 tooltip 里」，但代码里**没有任何 `Tooltip` 调用**
  （全仓都没有）。补上 `Element.Tooltip(copy.DirectOutputHint)`（旧版 `launcher.directOutputHintTitle`
  的中英文原文），并给 `i18n.Launcher` 加 `DirectOutputHint`（完整性测试自动覆盖）。
- **测试**：`TestTriggerWordEntersTheCommandMode`（id/名字/别名、argv 携带、裸词不进）、
  `TestBuiltInTriggerWordsWinTheNamespace`、`TestTypingATriggerWordAndSpaceEntersTheCommandMode`
  （编辑器端到端）、`TestDirectOutputRowCarriesTheSpaceKeycapTooltip`（悬停 700ms 后 tooltip 真出现）、
  `TestSpaceKeycapGestureRunsTheDirectOutputCommand`（空格 → 模式 → 回车 → 输出落在搜索区）。

**P7-s 列表协议的分页**
- **协议**：`extensions.ParseEmission` 除了裸数组（一份完整列表），现在也读对象形态
  `{"output": [...], "page": {cursor, hasMore}}`；`page` 块格式不合法就被丢掉而不是猜
  （cursor 不是字符串就是起点、hasMore 严格布尔）——猜一个续页正是列表静默循环的来源。
- **内存窗口化**：不自带分页的插件，其整份输出在内存里按视口分页（每页 `PageSize = 10` 行，
  首屏 `InitialPages = 2` 页）。窗口是**前缀**不是切片，页边界只会让列表变长，用户已经看到的行
  永不移动；装得下首屏的列表拿不到 `page` 块，因此不画页脚、全部行都画。
- **自带 cursor 的插件**：它自己的 `page` 块被尊重（行不窗口化），滚到底时**带着 cursor 重新执行**
  命令——cursor 走 `FLOTER_PAGE_CURSOR` 环境变量（旧版只回传、从不重跑，是一个死页脚；这里是
  它的真实现）。回页按 id 去重合并（先出现的胜出），插件不再说协议时保留已有行并停止追问。
- **页脚四态**：`loading`（自页插件在飞）/`more`（滚动触发就是提示）/`end`（插件说完了）/无
  （列表完整）。滚到底的触发用 `ui.ListState.AtEnd()`，即旧版 `LauncherResults` 的滚动触发器。
- **行内输出视图**的窗口高度仍由启动器固定的十行带决定，所以分页只加页脚，不改窗口几何。

**P7-r 透明底（本轮）**
- **根因**：面板用 `glass.Glass` 填充，而插件的 shader 永远输出 `vec4(c, 1.0)`——它映射的是场景内
  元素下面的像素，不是桌面。于是整个面板 100% 不透明，透明度滑块怎么拖都看不到桌面
  （headless 渲染实测卡片中心 alpha = 1）。
- **修法**：`glassmap` 新增 `Frame(spec, theme)`，返回 base.css 的 `--glass-tint`——帧自己的 alpha
  （滑块）叠上档位的 `--glass-step-dim`（`1 - (1 - dim*(1-opacity)) * (1 - opacity)`），一个**半透明**
  色；外壳卡片用它作背景（`off` 档仍用不透明的 `Opaque`）。窗口的 `VibrancyUnderWindow` 在有背板的
  平台模糊桌面，Linux 没有背板就是 tint 本身——与旧版 `backdrop-filter` 的降级一致。
- **去掉单独的雾层**：之前把 dim 画成一整屏 veil，在没有 backdrop 的平台上就是“加不透明”，半屏面纱
  把玻璃变成实心板；现在 dim 折进 tint 的 alpha，不再单画。
- **顺带**：卡片加上旧版的 sheen 渐变（`--sheen-top` / `--sheen-bottom`），面板不再是一块平 tint。
  实测：卡片顶部 alpha 0.63、内容面 0.89、字段块 1.0（旧版 R21 的刻意实心）。
- **行高密度**：启动器结果行之前是 `MinHeight(42u)` + 上下各 6px 内边距，实际画出来约 49px（旧版是固定
  40u、内容居中、无纵向内边距）；设置行是上下各 9px + 两行自然行盒 ≈ 55px（旧版约 48）。
  现在结果行去掉纵向内边距、只留 `MinHeight(40u)` 居中（模型同步把 `rowPadding` 改为 0、
  `rowTwoLineUnits` 改为 10），设置行/堆叠行的纵向内边距 9 → 6px。列表与设置页明显变紧凑。
  注：框架的自然行盒（11pt→17px、13pt→20px）比旧版 CSS 的 `line-height`（1.15/1.2）高，
  `LineHeight`/`FixedLineHeight` 只能调高不能调低，所以无法完全对齐——只能靠去内边距补。
- **窗口阴影**：之前全局 `DisableShadow: true`，macOS 上连窗口服务器的阴影也没了，面板没有深度。
  现在 macOS 保留原生阴影（跟随圆角透明内容），Linux/Windows 由卡片自己画（旧版
  `.platform-linux` / `.platform-windows` 的 `0 4px 18px ambient + 0 1px 5px contact`），
  因为无边框窗口的原生阴影会跟着矩形而不是圆角。

**P7-q 调色板、材质与排版对齐（本轮）**
- **调色板**：`theme.For` 之前只覆盖了描边、控制档、发线、Sunken 与文字阶，框架的**整套配色仍是 mygo
  的默认**——饱和蓝 `#3b82f6`、不透明灰的 `SurfaceHover`、`#a1a1aa` 的 muted。于是每一个视图里
  `t.Accent`/`t.TextMuted`/`t.SurfaceHover` 画出来的都不是旧版的样子。现在把 base.css 的整份
  配色写进框架 theme：accent `#8fb7ff`（浅色 `#064aa6`）、text/muted/secondary/tertiary、
  控制阶梯（control/hover/press，**半透明**白或黑，不是实心灰）、track/field、selection、focus、
  warning、scrollbar、背景 tint（`#121316` / `#fafafc`），控制圆角取阶梯的 `--radius-sm`。
  一个决定，所有视图（按钮、输入、开关、焦点环、滚动条）随之对齐。
- **控制透镜**：新增 `Tokens.Lens`，把 `--glass-lens-stack` 的四层 inset（上缘高光、宽面 sheen、
  下缘暗边、pane edge）落到 `InsetShadow`；图标底板与设置卡片戴上它，玻璃控件不再只是「平 tint + 边框」。
- **启动器字段块**：输入行恢复旧版的「近实心、比卡片亮一档」的块（`--glass-field` over
  `--surface-opaque`）+ 底部 1px 发线；删掉旧版已废弃的 scroll-edge 渐变带（R31）；面板改成
  `--surface-sunken` 的阅读面。外壳卡片在启动器上去掉左右内边距并 `Clip()`，字段块因此**通栏**、
  吃卡片圆角，文字列回到 16u。
- **侧栏选中**：框架给「被选中的 list 行」在行**背后**画 accent，半透明抬升会被染成蓝药丸。
  新增不透明的 `RaisedQuiet`（control-hover 合成到内容面上），侧栏当前页是中性药丸 + accent 图标，
  与旧版 SETTINGS-APPLE 的模型一致。
- **筛选 chips**：去掉选中项的 accent 填充与下划线，改回旧版 R72 的「颜色 + 字重（640）」；
  行内边距 16u、间距 14u、字号回到 body 阶；提示行用 tertiary。
- **行内细节**：直出行恢复 18u `␣` 键帽（描边 + 控制面 + 键帽高光）；选中行的副标题取 secondary；
  设置页头部两个控件（退出 / 关闭）从实心药丸改回 28u 透明方块 + 16u 标记（`.toolbar-button`）；
  输出页的钉住 / 分离控件同样；字段的清除按钮也是同一个透明方块。
- **设置内容列**：18u 的左右内边距落在内容列（不是滚动容器），滚动条仍贴边。
- **新特性**：mygo 已升到 `v0.4.5`（在 v0.4.4 的基础上新增 `Element.Draggable` / `DragOffset`
  与 `ui.Nearest`：指针拖动元素并抛出，带 Bounds / Snap / 弹簧收尾，见 #244；其余差异都是为它
  铺的内部改动，没有别的公开 API）。**本应用不采用 Draggable**——面板里没有可重排的列表，设置里
  没有靠拖动定位的元素，唯一的值拖动控件（设置页与插件表单的滑块）是框架自带的 `ui.Slider`，
  窗口移动也仍是原生 `DragWindow()` 的拖动区；给历史行加「横向拖走删除」会与既有的「选中 +
  二次确认」删除语义重叠，属于为凑 API 而改产品行为。仍用上 v0.4.3 的
  `ui.ContentTransition{Numeric: true}`，让滑块的百分比读数与计算器答案在变化时滚动数字
  （#110 的内容过渡）。
- **终端**：空态的两个按键行改成旧版的 20u 键帽（control edge/fill + 键帽高光 + xs 圆角），
  悬停只把文字提一档、不画面板；resident 提示改用 `--glass-float`（新增 `Tokens.Float`）
  与 `--elev-3` 的抛影 + 上缘高光，标题用 secondary。
- **集成状态**：`Running` 是 accent 文字、`Broken` 是暖色 chip + 警告描边、其余是 secondary
  （`.extension-status` 的语言），状态不再是清一色的 muted 灰。
- **独立窗口**：detached / pinned 两个自有 Context 的窗口此前没设 floter 主题，画的是框架默认配色；
  现在设主题、用 `Opaque` 作不透明底，detached 的头部控件也改成透明的 bar button。
- **插件配置覆盖层**：字段区回到旧版 R59 的 4u/16u/12u 内边距；标题从 13/620/strong 改为
  body 600 muted（名字是面包屑，不是第二级标题）；分组标题加上卡片自己的 14u 缩进，与它下面的
  行标签同列。
- **发线与透镜**：设置卡头部/侧栏的分隔线改成 `--hairline-fade` 的渐变发线（mygo 渐变只有两
  个 stop，用「透明→发线、实心中段、发线→透明」三段拼出）；删掉页头下多出来的一条实心分隔线；
  侧栏当前页的药丸戴上 `--elev-0` 透镜。
- **卸载对话框**：三个数据类别之前只画了勾选框，**说明行被丢掉**；现在每个类别是一张卡片
  （勾选框 + 名称 + 说明），与旧版 `.extensions-uninstall-component` 一致。
- **i18n 空白字段**：声明了但**两种语言都没赋值**的文案——计算器命令行（启动器里一行完全空白的行）、
  电源确认的取消按钮、以及整个卸载对话框的标题/说明/三个类别名与提示。补上英文/中文，
  并把完整性测试从「zh 相对 en 不空」加强到「en 自己也不能有空字段」（这才是这类的漏洞）。

**P7-a mygo v0.4.0 + 菜单栏与透明**（本轮）
- mygo `v0.3.7 → v0.4.0`：新增 `ui.WebView`（页面内嵌原生 WebView，配 `Window.NewWebView`）、
  `OnPaste`、`Dismissed`、`InsetShadow`。本应用不需要（插件是 CLI 工具，不是网页），但升级后三平台全绿。
- **菜单栏**：mygo 在从未调用 `SetMenu` 时会安装一份*默认*应用菜单，Linux/Windows 上它被画在窗口**内部**
  —— 浮动面板顶上一条 File/Edit/View。旧版根本没有应用菜单（托盘即菜单），所以现在非 macOS 上显式
  `SetMenu(nil)`，macOS 保留（那里的菜单进系统菜单栏，不占窗口）。
- **玻璃面纱**：`haze` 是为了盖住旧版 `backdrop-filter` 模糊出来的桌面；没有 backdrop 的平台
  （Linux，以及 Windows 11 之前，`c.Vibrancy()` 为 false）下面纱不是"盖模糊"而只是"加不透明"，
  于是半屏面纱把玻璃变成了实心板。这些平台不再画面纱，只有 tint 的 alpha（用户设置的不透明度）透出桌面。

**P7-b 单位与启动器几何（本轮）**
- **字号**：框架默认正文 13/14pt，而本设计是旧版的 `--text-body: 11px * ui-scale`（本仓库的每个
  `Space(n)` 都是它的 4 倍）。字号大了四分之一，导致每行都比原型高、文字与框的比例整体走样。
  现在正文就是 11u，整条阶梯（caption/emphasis/title/display）随之落位。
- **启动器几何**：输入行 56u（R37 那个和设置页头部等高的带）、面板内边距 4u 上 / 2u 下（`.launcher-bottom`）、
  行高不低于 42u（两行）/ 34u（一行）。于是 `66 + rows × 42` 就是窗口高度，和旧版 `result-budget.ts` 一字不差。

**P7-c 插件配置覆盖层（本轮）**
- 「配置页面通用化：点击设置之后，弹出一个基于通用规则的配置页面，而不是一个新的完全独立的页面。」
  旧版 R29 用声明式 schema + 一个通用覆盖层回答了它；本仓库此前用"每个插件一个设置页"回答，
  而那正是 R29 取代的东西。现在齿轮在启动器自己的带里打开表单，用设置页自己的卡片语言渲染。
  - 新包 `internal/plugincfg`：字段（kind = toggle/select/radio/checkboxes/slider/number/text/action）、
    标签、帮助、边界/选项、分组，加上三个内置插件的 schema 与旧版的高度预算。
  - `settingsui.PluginSheet` 渲染任意 schema，不认识任何插件名。
  - 启动器持有带、armed action 与失败行；齿轮在字段行的行尾（旧版 `.collapsed-card__settings--plugin`
    的位置，P7-x 从 chips 行搬回），打开时变成 ✕（R41：打开它的控件是唯一关闭它的控件），
    chips 行则随列表一起退场。
  - shell 持有"一个改动意味着什么"：每个字段写插件自己的块，回给表单的是**存储里的值**而不是请求的值。
  - `action` 字段是命令不是值：第一次按只 arm，第二次才执行（不弹系统对话框）。
  - 随之删掉 settings 的 Plugins 页（旧版 R33 也删了自己的插件页）；磁盘上遗留的
    `last_settings_page: "plugins"` 落到第一页而不是报错。

**P7-d 输入行按钮 + 分离输出窗口 + 五个没接线的动作（本轮）**
- 输入行尾部恢复旧版的两个字形按钮（会话页 / 设置），点一下就走，不用打命令。
- 命令的输出可以「在窗口中打开」：一个不随面板召唤/隐藏的窗口，记住命令并可从自己的头部重跑。
- **审计发现五个 launcher action 在 shell 里从未接线**（电源行、打开路径、在目录里开终端、
  外部终端、安装会话）——即五个功能静默失效。全部接上，并加了反射测试：shell 必须回答
  launcher 声明的每一个 action，下一个不会再静默。
- 电源确认改回旧版的面板内确认行（`.launcher-system-confirm`）：动作自己的按钮（"重启"而不是"继续"）
  + 返回，回车不再直接执行；Esc 即取消。

**P7-h 本地工具可以是脚本（本轮）**
- 后端一直支持脚本 provider（manifest `runtime.type: "script"`、解释器查找、`provider.<ext>`、
  导入导出），但表单只能指向"机器上的程序"，这一半功能不可达。现在表单先问**类型**：
  - `internal/scriptlang`：八种语言（js/shell/powershell/python/ruby/php/go/rust），各自的扩展名、
    是否编译型、必须在 PATH 上的工具链、双语模板（旧版 R9-1 的那张表，放在一处，语言不会被"半加"）。
  - `extensions.ScriptRuntimeFor`：这台机器有什么（解释器或编译器 + 版本），**从不失败**——
    "没找到"是答案，状态行说出来，而不是阻止保存。
  - 切换语言会替换"未被改动过的模板"，用户写过的内容保留；空脚本不连接（和没填程序的路径一样）。
  - 草稿字段只走一个映射（`customRequest`），连接与编辑两条路不会分歧；编辑表单把脚本从包里读回来。

**P7-i 召唤时重新归位（本轮）**
- 旧版每次 reveal 都把窗口移回默认位置（launcher 是"召唤"，它属于用户注意力所在的那块屏幕）。
  本构建只在建窗时放一次，于是拖动会被下一次召唤继承。现在每次 reveal 都重新归位（显示前一次、
  显示后一次——macOS 会丢弃未映射窗口上的几何设置，两次在同一帧内，中间不会闪现）。
- 屏幕取"窗口当前所在的那块"（中心点判定，退回主屏）：框架不报告哪块屏幕有键盘焦点，
  窗口自身位置是最诚实的替代——被拖到第二块屏的面板会在第二块屏归位，而不是被拽回主屏。

**P7-j 终端：链接、通知、多会话（本轮）**
- **链接**：Cmd+点击终端输出里的超链接 / URL / 路径 → 用应用自己的两个 opener 打开（URL 走用户选的
  浏览器，路径走文件管理器），与启动器行一致；编译器打印的 `:line:col` 后缀在整串不是文件时被剥掉，
  相对路径不猜（会话的 cwd 不在这里跟踪）。
- **通知**：程序自己的通知请求（OSC 9 / OSC 777）走应用已有的通知通道。
- **多会话**：终端表面现在持有自己的会话列表（旧版由 broker 托管会话，本构建在进程内）：
  `Session`（程序、屏幕、标题、是否已结束、创建时间）、`NewSession`/`SelectSession`/`CloseSession`/
  `CloseAll`；运行命令或"在此打开终端"开新会话而不是顶掉旧的；resident（已结束）状态按会话区分；
  外观刷新作用于每个会话；退出时结束所有会话。
  - 设置里的会话页列出全部（正在显示 / 运行中 / 已结束），非当前会话有"显示"，每个都有"关闭"
    （先 arm 再执行——关闭会杀掉程序里的进程），另有"新建会话"；恢复一个会话会把终端表面带到前台。

**P7-p mygo v0.4.2 与按压态（本轮）**
- **mygo v0.4.2**：toast viewport 的 keying 修复（关掉的对话框不再把自己的身份——和悬停——
  借给 toast 栈）、spinner 跟随 `TextColor`。无新的应用级特性。
- **行的按压态**：行之前只有悬停与选中。控制材质自己的按压面（base.css 的
  `--glass-control-press`）比静息面**更不透明**——按下的行沉进玻璃里，而不是从玻璃里凸出来。
  启动器行、设置行同一规则。
- 顺带：启动器行的状态判定改为单一的 switch（press > selection > hover）。

**P7-o 声明式输入的行内校验（本轮）**
- 声明式输入的编辑器此前让保存把笔误**弹回后端**才解释。旧版 `paramIssues` 的规则：错误渲染在
  导致它的那一行下面——"a save never has to bounce back from the server to explain a typo"。
- 现在：选择型缺选项、布尔型缺旗标，都会在该行下面印一条灰色的警告行（caption 步、警告色），
  写明这一行还缺什么。
- **订正（P7-w）**：这一轮只加了机制（`ParamRowIssues`/`issueUnder`/两处文案），**没有调用点**
  ——表单照旧把笔误弹回后端；P7-w 把它真正接进了编辑器与保存路径。

**P7-m 行内寄存器补齐（本轮）**
- 计算器行回到旧版 R50 的寄存器：**答案完整在右**（nowrap、strong、620、tabular），表达式是让出
  空间的标题——历史行与"算式即查询"的答案行用的是同一行，此前答案行的两行是反的。
- 历史行的删除是**两步**：第一下装填（行印"再按一次确认删除"，行唯一可花的暖色 token），第二下
  删除；装填随选择移动解除，Escape 先回答它。
- 动作栏标题是**交给终端的命令行**：用终端自己的等宽字体、520（旧版 `.launcher-action-bar__title`），
  其余行仍是界面字体。
- 直接输出命令的行尾 `␣` 标记（R65）；浏览器插件关闭时模式里说真话（`launcher.browserDisabled`）。

**P7-l 浮起的按钮回到行内（本轮）**
- 终端栏的四个控件从框架按钮（实心药丸）改回旧版的 `.toolbar-button`：28u 透明方块、16u 的标记、
  自己的标签（辅助技术），悬停才落到静息面上——浮动条里最响的东西不该是一个实心药丸。
- 启动器行自己的 pin/删除控件同理：20u 命中区 + 标记，随选中出现（`.launcher-result__pin`），
  与历史行星标是同一语言。
- 动作栏带上自己的 3u 呼吸（旧版把它和 42u 带一起记作 `LAUNCHER_ACTION_BAR_UNITS`），
  常量存在，所以底边不会随结果数越过 0/1 而跳动。
- 行内的警告点是一颗真 8u 圆（`.launcher-result__warning`），不再字体字形。

**P7-k 行内控件与注册字号（本轮）**
- 输入行/行内控件补齐旧版的细节：历史行自己的**星标**（20u，悬停/选中时显现、收藏时常显，
  点击收藏而不运行行，控件自己的名字跟随状态）；编号徽标是**存储的 select_result 绑定**与数字
  （⌘1 / Ctrl+1，不再只是裸数字）。
- 状态行回到旧版的 30u 带与警告面（警告色描边、更重的字重）；设置面板自己的 56u 头带（应用名 +
  `v版本` + 退出/关闭两个控件——之前设置面板没有可见的离开方式）；页标题/副标题、分组标题/提示、
  22u 的分组节奏、148px 侧栏列、行内 44u 下限与强调/正文字号。
- 终端空状态的两行是**自己的键**（↩ 新会话、esc 返回——之前空状态没有键盘出路），resident 提示
  用浮动卡自己的描边与阴影；会话页的行高 56u 的节奏沿用。

**P7-e 输出按终端语义读 ANSI（本轮）**
- 新包 `internal/ansi`：纯解析器，把 CLI 输出里的 SGR（颜色/粗细/下划线/反显 + 16 名色 /
  256 色 / 24 位真彩）读成带样式的 run，其余转义（光标移动、擦除、OSC）**丢弃而不是打印**；
  复制走的是去掉转义后的文本。输出视图用应用自己的 16 个程序色（`theme.ANSI`，对应旧版
  base.css 的 `--ansi-*`）绘制。

**P7-f 尺寸账本对齐（本轮）**
- 发现并修掉一类 bug：**画出来的比窗口记账的多**。
  - 行高：框架的行盒是字体自己的 ascent+descent+gap（14pt → 22px），模型却按 1.4 估算（17.6），
    每行少记约 4px，长列表最后几行落到窗口外。现在由 `Context.MeasureText` 实测，模型按实测记账。
  - shell 的 `targetSize` 少了实测行高与打开的配置表单，窗口会先短几像素再收敛。
  - 筛选 chip 自带 padding，比它被记账的 20u 带更高（R72 的 chip 就是行盒本身）。
  - 新增测试：把每个状态渲染在**恰好记账高度**的窗口里，任何一行落到窗口外就失败。

**P7-g 五个没接线的 launcher action（本轮）**
- 审计发现 `launcher.Actions` 里有五个字段在 shell 里从未赋值：电源行、打开路径、在目录里开终端、
  外部终端、安装会话——五个功能静默失效。全部接上，并加反射测试钉住「shell 必须回答每个 action」。
- 电源确认回到面板内确认行（旧版 `.launcher-system-confirm`）：动作自己的按钮 + 返回，Esc 取消。

**P7-d 分离输出窗口 + 输入行按钮（本轮）**
- 命令输出可「在窗口中打开」：不随面板召唤/隐藏的窗口，记住命令并可从头重跑。
- 输入行尾部恢复旧版两个字形按钮（会话页 / 设置）。

- P2-i macOS 应用名与别名（`internal/plist` 读 XML/bplist00，读 `Info.plist` 与本地化 strings）
- P2-j 应用图标（macOS `.icns` 最大 PNG 条目、Linux `Icon=`，懒读 + 缓存）
- P2-k 结果编号快捷键（⌘1–⌘0，随视口重算）、P2-l Windows 图标提取
- P3-h 后台运行与输出回看、P3-i orphan 接管/删除、P3-j 完成通知与权限审计、P3-k 列表协议、P3-l 导入/导出
- P4-j 命令开关与别名、P4-k 自定义快捷键、P4-l 计算器插件、P4-n 终端尺寸写回与会话快照、
  P4-o 文件拖放、P4-p 应用快捷键表
- P5 动效/无障碍/性能基线、P6-c 发布链（CI 矩阵 + 安装器 + 更新检查）

**P4-o 文件拖放**（本轮）
- 新包 `internal/drops`（规范化 + shell 引用），启动器 files 模式：每个文件三行安全动作
  （打开 / 在此打开终端 / 复制路径），落地不执行；`RunShellIn` 以目录起新会话。

**P4-p 应用快捷键表**（本轮）
- `shortcuts` 块（七动作、平台默认、不可解析回落），`shortcuts.Parse/Match` 反解供视图比对；
  数字键随 `select_result` 绑定走，new_command / open_external_terminal 在启动器内生效；
  Shortcuts 页「应用按键」组可逐条录制。

**P4-s 工具安装目录与推荐工具接管**（本轮）
- `internal/tools`（12 工具 × 平台配方 + 8 包管理器探测与命令语法，探测是 stat）；
  启动器给未安装的工具一行「安装 X」，回车只复制命令；推荐工具（v-tools，go:embed）在
  Integrations 页一键接管，走同一条本地安装管线。

**P4-t 快捷键收尾**（本轮）
- 「恢复默认按键」按钮（清空映射 + 唤回键回默认并重注册）；录制期间释放全部全局键、结束再注册；
  修掉「自定义录制器没让出 Escape，Esc 会关掉设置面板」的 bug。

**P4-u 电源动作与图片缩略图**（本轮）
- 查询正好是 restart/shutdown 词时给出重启/关机行（先确认后执行，平台命令各自正确）；
- 剪贴板图片条目显示缩略图（懒解码 + 按路径缓存）。

**P3-m 集成配置表单**（本轮）
- `extensions.SaveConfiguration`（schema 校验、password → 新密钥代、values 占位、旧代清理）；
  Integrations 页的配置表单（按类型渲染、草稿存活、错误上报）；端到端：保存后注入环境正确。
- 顺带修掉 `Description.Configuration` 为 nil 时的空指针（描述不带 configuration 块的集成）。

**P2-m 操作条**（本轮）
- 输入 URL/路径/命令直接给对应行（打开、文件管理器、在此打开终端、复制路径、在终端运行）；
  Enter 落点规则照旧版（命令词/空白/无匹配时落 shell 行）。

**P2-n 内置命令双语与拼音搜索**（本轮）
- 内置命令行带中英文措辞 + 中文名拼音首字母（重启 cq/cxqd/zq、剪贴板 jtb/jtbls、浏览器 llq、
  计算器 jsq、终端 zd、设置 sz、退出 tc、关机 gj/gbdn），19 组用例钉住。
- **受阻**：应用名的拼音表（`compute_initials` 需 2 万字表）——无网络、无本地数据源，嵌入未验证的
  表不可接受；同列的子序列匹配（`scoreNormalized` 第四档）与应用名拼音一起做。

**P4-v connect / register 深链**（本轮）
- `floter://connect?manifest=…`（审查后接入，拒绝不装）、`floter://register?cmd=…`（带出集成页即止）。

**P3-n 生命周期探针**（本轮）
- manifest `lifecycle.probes` 的执行与 `probeReport` 持久化：required 失败 → broken、optional 失败 →
  degraded、全过 → healthy、空集 → `ErrNoProbes` 不抹旧报告；集成页「检查」先探针后 diagnose。

**P3-o lifecycle.launch 接线**（本轮）
- manifest 声明的「集成整体怎么跑」现在生效：launch 程序/前置参数（self = runtime 解析的可执行）、
  cwd 策略（home/toolData/fixed）、终端环境（TERM/COLORTERM/TERM_PROGRAM）。

**P3-p 健康状态行上显示**（本轮）
- `probeReport.status` 持久化并显示在集成行上（跨运行可见；「检查」刷新它），broken 时不贡献命令。

**P2-o 命令行结构化解析**（本轮）
- `ParseCommandLine`：token（引号/转义）、前导 NAME=value 解码进环境、shell 语法检测；
  操作条的 shell 行把 argv 与环境分开交给终端会话。

**P2-p 安装行的终端交接**（本轮）
- 修正安装行的行为：Enter 打开**裸终端会话并打出命令**（旧版 R68 语义，会话在安装后仍是交互式的），
  不是复制到剪贴板；`terminalui.RunShellWithCommand` 带 5s 提示符等待。

**P2-q 查询历史**（本轮）
- 用户在终端跑过的命令行进历史（去重、上限 20）；**无结果时** ↑/↓ 走历史，草稿保留（↑ 先记住
  正在打的，↓ 回到现在）；有结果时 ↑/↓ 仍是结果导航。

**P2-t 应用扫描缓存**（本轮）
- 签名（FNV-1a：排序去重的路径 + 长度 + mtime，限频 30s）未变时 launcher 立即用上一次扫描；
  变了才后台重扫。进程内缓存（旧版另有磁盘持久化缓存，未做）。

**P2-u 剪贴板「清空历史」**（本轮）
- Plugins 页的清空控件：确认后移除所有未收藏条目（收藏豁免），确认与清除可注入。

**P4-w 终端空态的控制**（本轮）
- 终端页无会话时的空态有标题 +「新建空白会话」+「回到搜索」两个控件；spawn 失败的错误替换说明。

**P4-x 终端标题栏的旧控件**（本轮）
- 标题行回到旧版的两个控制：「新命令」（回启动器+清空输入）与「在终端打开」（开系统终端窗口）。

**P3-p 组件化卸载**（本轮）
- `extensions.UninstallComponentized`（程序必移、三个数据类别按勾选、全选=移除数据根）；集成行
  「卸载…」展开对话框（标题/说明/三个勾选/卸载/取消），shell 确认后执行。

**P3-q 能力探测**（本轮）
- `ProbeCapabilities`：version/help 结构探针 + manifest 自定义探针，退出码与 stdout 对照预期，
  报告聚合出版本号/支持功能/限制；「检查」流程变为探针 → diagnose。

**P2-v .strings 的 UTF-16 与无引号键**（本轮）
- `.strings` 文本的 UTF-16 BOM 检测解码（LE/BE）、无引号键、块注释剥离、转义解码——旧版
  `localized_name_from_strings_text` 的完整移植。企业微信的英文名（无引号键）因此可达。

**P2-w 子序列搜索**（本轮）
- `isSubsequence` 加进 `candidateScore` 的梯子：`vsc` → Visual Studio Code，`wyyyy` →
  网易云音乐。评 3 分，低于包含、高于无匹配。
- mygo v0.3.5 → v0.3.7（go.mod 已升级，三平台构建全绿）。

**P4-y 命令别名编辑器**（本轮）
- Integrations 页每命令一个别名输入框，提交写入 `command_aliases`（清空即删），冲突时行内
  标注"别名已被其他命令占用"；写入后重交启动器命令列表并重扫 PATH 工具。

**P1-z 启动器窗口高度跟随内容**（本轮）
- 窗口高度 = 卡片带（字段行 + 列表上缘 + 逐行高度 + 卡片内边距）+ 平台内缩，主题值逐行测量；
  十行帽 + 一行收缩滞回。View 用 `Animate`（150ms EaseOut）逐帧走窗口边缘，高度没变不碰平台。
  `internal/launcher/height.go` 是唯一来源。

**P3-r `--help` 推断**（本轮）
- `helpargs.go`：`DeriveArguments`（四种 CLI 风格 + 紧凑旗标摘要）、`DeriveSubcommands`（v 风格 +
  cobra 段）、`stripANSI`、别名组、字形剥离、严格 charset；`ProbeDerive` 连接时根 + 二级探测
  （至多 12 个子命令）；`SemverFromVersionOutput` 版本提取。`cmd.Dir` 只在目录存在时设置。

**P1-ac 毛玻璃雾层 PassThrough**（本轮，修「点不动」）
- 雾层盖在行上且无 PassThrough → hit chain 命中雾层 → 所有行看得见点不着。加 PassThrough 修复，
  两个测试钉住启动器与设置两条路径。

**P4-z 重探命令**（本轮）
- `ReprobeCommands` + Integrations 页按钮：重新推导生成的自定义集成的命令列表，原子刷新描述符，
  sidecar 记录新鲜度；工具哑了保留旧描述符。发布者内容永不覆盖。

**P4-aa 多来源工具清点**（本轮）
- `internal/inventory`：桌面条目 + flatpak/snap/nix/Homebrew/LaunchServices/Chocolatey/Scoop/
  WinGet 多来源发现、同工具合并、策展排序、指纹、TTL 缓存。

**P7-n 滚动条与进入动画（本轮）**
- **启动器列表没有滚动条**（旧版用户两轮争取来的："bar 只在悬停时出现，列表由键盘和滚轮驱动"）——
  滚动仍然可用（滚轮/触控板/↑↓，选择自己滚进视野），只是 bar 没了，卡片边缘一条干净的线。
  用 `ScrollbarInsets` 把它推出列表自己的裁剪区；悬停在溢出的列表上不再画药丸，滚动测试全过。
  设置页的滚动条**保留**（旧版给设置页自己的样式，"设置页面的滚动条也还是没有贴边"）。
- 状态行与 resident 提示按旧版的 `feedback-enter` 进入：淡入并从 2u 上方落定（200ms）。

**P7-p mygo v0.4.2 与按压态（本轮）**
- **mygo v0.4.2**：toast viewport 的 keying 修复（关掉的对话框不再把自己的身份——和悬停——
  借给 toast 栈）、spinner 跟随 `TextColor`。无新的应用级特性。
- **行的按压态**：行之前只有悬停与选中。控制材质自己的按压面（base.css 的
  `--glass-control-press`）比静息面**更不透明**——按下的行沉进玻璃里，而不是从玻璃里凸出来。
  启动器行、设置行同一规则。
- 顺带：启动器行的状态判定改为单一的 switch（press > selection > hover）。

**P7-o 声明式输入的行内校验（本轮）**
- 声明式输入的编辑器此前让保存把笔误**弹回后端**才解释。旧版 `paramIssues` 的规则：错误渲染在
  导致它的那一行下面——"a save never has to bounce back from the server to explain a typo"。
- 现在：选择型缺选项、布尔型缺旗标，都会在该行下面印一条灰色的警告行（caption 步、警告色），
  写明这一行还缺什么。

**P7-m 行内寄存器补齐（本轮）**

## 液态玻璃（mygo 内置材质，本轮重构）

- **桌面模糊回来了**：面板的窗口带 `VibrancyUnderWindow`——macOS 的 NSVisualEffectView、
  Windows 11 的系统背板（Linux 无，玻璃的 tint 就是全部材质）。旧版的 CSS
  `backdrop-filter` 模糊的就是桌面。
- **面板的填充是半透明 tint，不是 `glass.Glass`**：插件的 shader 无论怎样都写 `vec4(c, 1.0)`
  （它映射的是**场景内**元素下面的像素），所以用它填面板永远不透明、桌面透不过来。现在面板画的是
  base.css 的 `--glass-tint`：帧自己的 alpha（透明度滑块）再叠上档位的 `--glass-step-dim`，
  一个半透明色，由窗口的 vibrancy 透出模糊桌面。
- **档位的映射**：`frosted` dim 0.6 / `regular` 0.5 / `liquid` 0.2。dim 不是单独一层雾
  （之前画成一整屏 veil，在没有 backdrop 的平台上把玻璃变成了实心板），而是折进 tint 的 alpha——
  与旧版 `--glass-tint-alpha` 的公式一致：`1 - (1 - dim*(1-opacity)) * (1 - opacity)`。
  滑块仍是唯一的 alpha 真相，档位只动 dim。
- **不采用**：`Interactive: true`（按压生长）——旧版 `.collapsed-card:active { transform: none }`
  是刻意关掉的；`glass.Blur`（场景内 backdrop blur）——面板下是透明的根，没有东西可模糊。
  `glass.ScrollEdge` 仍用在窗口**内部**的滚动内容上（启动器结果、设置正文）。

## 视觉核验（本轮）

- **scope 标记与输入框**：标记 x 20–85、输入 x 89 起、首行 y 86（字段 56u + chips 之下）——无重叠。
- **浏览器模式**：mode 行的来源词按旧版 R30 丢弃（scope 标记已说），行只带 URL 与访问时间。
- **文件行**：旧版无独立样式（普通 `.launcher-result` 行），本构建一致。
- **浏览器行**：旧版 R26-A 的副标题即 URL（tab 行的类型词是它所在的组名）；本构建行带 URL +
  访问时间，无右侧来源词——一致。R73 之后 live-tab 组没有组标题（本构建同样没有），行上的
  tab 标记一致。
- **内置模式的行**：type word 由 scope 标记承载（本构建同一）。
- **行的 source 槽**：command 行右侧是**集成名**、install 行右侧是**包管理器**（proper noun）；
  警告点与 `␣` 键帽与它们同组，都在标题的右外侧，窄窗口先省略标题而不是它们。
- **设置面板**：不可用户调整大小、无尺寸持久化（旧版同）；启动器查询历史内存态（旧版同）。
- **i18n 覆盖**：`i18n.Launcher` 111 个字段对旧版 73 个 `launcher.*` 键（超集）；旧版 1030 个
  `settings.*` 键里的大多数属于已不存在的页面/能力（iframe 页、会话 broker、Web 版本号），不为
  "对齐键数"而搬。旧版的 files 模式只有拖拽（`files` 触发词是本构建的加项）；`CD` 命令行在本构建
  是"以目录起会话"（同一个去处）。
- **设置变更的实时应用**：启动项、应用图标（托盘同换）、菜单/托盘语言、托盘可见性、最近启动、
  PATH 命令、自定义快捷键、终端字体/调色板——全部经 store 监听即时生效。
- **会话页的状态行**：旧版 `.session-manager` 的三句（`Running · attached` / `Running · detached` /
  `Process exited · code N`）——本构建的会话自带自己的状态行，页面只印它。
- **托盘菜单**：语言变化重建（菜单滞后于界面比没有菜单更糟）；App icon 明暗切换同时换托盘标记。
- **窗口高度的滞回**：旧版 `LAUNCHER_ROW_HYSTERESIS = 1`——进入立即增长（差一行会截断行），
  收缩只等内容落到持有数之下一行（一行余量吸收 1↔2 边界，永不震荡）。本构建同值（`Hysteresis = 1`）。
- **参数补全**：枚举值只在它的参数是最后一个 token 时列出（`-mode` 之前不出现 `pretty`）。
- **⌘⏎ 的两条路径**（R60）：字段提交记录**按下时持有的修饰键**（`submitMods`，框架内部），所以
  ⌘⏎ 与裸 ⏎ 走同一条 `activate`——动作栏行被选中的 ⌘⏎ 直接运行它（R60 对"非 held-terminal 行"的
  规则），held-terminal 行有它自己的 ⌘⏎ 分支（进终端）。旧版的"按住修饰键把高亮移到动作栏"由
  held-modifier 的预选承担。
- **计算器 Enter 的两种落点**（R50）：字段持有未求值的表达式 → 答案行求值并记录；字段为空或选择
  在历史行上 → 复制该行。两种路径后都离开模式并隐藏窗口，字段清空（旧版自己的 copy 也是
  `setQuery("")` + 隐藏）。
- **⌘ 裸终端行**：R64 的规则一致——只在普通页 + 空字段 + 修饰键按下时出现，位于列表**最后一行**
  且被预选（R61 的"列表额外增加"已废弃），输入查询时该行完全不画；行内的快捷键徽标是动作栏自己的
  `⌘↩`（放进该行的和弦就是打开它的和弦）。
- **普通页 Tab**：本构建直接进入参数模式（旧版展开命令行文本）——有意为之的差异，已命名记录。
- **设置选择器**：220u（旧版 `.settings-select` 的 max-width）。
- **插件表单**：meta/title/label/help 的层级、8u 分组间距、卡片 14u 内缩、失败行 caption + 警告色、
  选择器 220u——全部回到旧版 `.plugin-config` 自己的读法。
- **反馈进入动画**：`feedback-enter` 的 160ms（旧版 `--dur-3`），状态行与 resident 提示同一。
- **计算器行**：R50 的寄存器（表达式是标题、答案完整在右），历史行与"算式即查询"的答案行一致。

## 底色与透明的审计结论（本轮）

三个表面的透明，逐一核对：

- **启动器**：`main_opacity`（默认 47%）是卡片玻璃材质的 tint alpha——桌面经
  `VibrancyUnderWindow` 模糊后，透过 tint、雾层与卡片边缘的透明边距可见。headless 渲染核验：
  卡片面有 tint（186,186,186）、边缘 alpha=0。
- **终端**：`terminal_opacity`（默认 46%）是画布自己的 tint alpha（Ghostty 的
  background-opacity 语义）——画布以该 alpha 画调色板背景，其余透过卡片玻璃与 vibrancy 可见。
  旧版的 `--glass-tint-terminal` 把 dim 合进 tint，本构建是 Ghostty 语义的等价物。
- **设置**：旧版 `.settings-card__body` 有自己的填充（`--surface-sunken`，alpha =
  0.62 + 0.18 × main_opacity）——设置页是阅读面，比启动器更实。本构建此前**没有**这层填充
  （行直接画在玻璃上，读起来和启动器一样透），本轮补上（`theme.Tokens.Sunken`）。
- **卡片边缘**：4u/10u 透明边距——vibrancy 之后边缘下是模糊的桌面（旧版是清晰桌面）；
  视觉材质内的内缩是 macOS 本地面板自己的读法。

## 核验收尾（本轮）

- **命令历史**：旧版 R-xxx 的规则一致——只在"把命令交给终端"时记录（去重、晋升、上限 20），
  ↑ 保留草稿、↓ 回到当下。启动器行与安装行各自的记录点核对过。
- **重扫**：托盘的 Re-scan 覆盖四个来源（应用、PATH 工具、安装目录、集成），缓存扫描在其前。
- **扫描容错**：应用扫描内部逐条容错（无整体失败态），旧版 scanFailed + 重试按钮的场景在本构建
  不存在。

- **"Restore defaults" 此前是空操作**：快捷键页的分组动作的 handler 是空函数——按下什么都不发生，
  而且它被包在第二张卡与第二行里。旧版把分组动作放在**分组标题行**上（`settings-section__heading-
  actions`，与标题基线对齐）。现在动作在标题行上（`sectionWithAction`），并真正运行 shell 的重置。

## 已知差距（框架不支持，诚实记录）

- **普通页 Tab 展开命令行**：旧版普通页里 Tab 把选中的命令行**展开成完整命令行文本**
  （`setQuery(commandLine)`）。本构建 Tab **直接进入参数模式**、列出该命令自己的补全——同一个能力
  的另一扇门（R80 的静态补全仍然都在）。**触发词+空格**这条旧版的入口已在 P7-t 补上，两条路
  现在都通。
- ~~直接输出的空格交互~~ **已完成（P7-t）**：旧版 R65 的行尾 `␣` 提示（tooltip 讲解）与
  「敲命令名 + 空格进入直出模式、输出显示在结果区」的路径都已接线；本构建的后台输出视图
  就是同一个结果区。
- **终端选中即复制 / 滚动条 / bracketed paste**：见下面的终端外观条目。
- ~~插件模式的 scope 标记~~ **已完成（P7-m）**：进入模式消耗触发词并清空字段，输入框左侧画
  `图标 + 模式名`（`.collapsed-card__scope`）；模式内的搜索/补全把字段当 needle；Esc 离开时
  needle 保留为普通查询（R31），**运行**则清空字段（旧版自己的行为）；模式只由用户自己离开。
- **会话排序 / 轮询**：旧版会话页有"最新/最早"排序与 5 秒轮询（会话由 broker 托管，可能跨天存活）；
  本构建的会话在进程内、生命周期与 App 相同，列表按创建顺序，不做轮询。
- **设置里的 Plugins 页已删**：内置插件的配置现在是启动器里的通用表单（P7-c），与旧版 R33 一致；
  遗留的 `last_settings_page: "plugins"` 会落到第一页。旧版 Integrations 页自己的「基础插件」区
  （三个内置插件 + 开关 + 「配置」）已在 P7-u 补上，它也是浏览器模式空态文案指向的那扇门。

- 终端外观的四个设置**磁盘上存在、会被保留，但当前无 UI、也不起作用**：
  `terminal_bold`（加粗模式：粗体字/亮色）、`terminal_scrollbar`、`terminal_select_copy`（选中即复制）、
  `terminal_paste_safe`、`terminal_wheel_lines`。mygo v0.3.7 的 terminal 插件（`plugins/terminal`）
  没有暴露这些旋钮（内部固定：滚动条总画、滚轮步长固定、复制仅 ⌘C、无 bracketed-paste 开关、
  bold 由字体权重决定），所以 Go 侧既不能渲染（渲染了就是骗人）也不能接线。等框架暴露后再补。
  字号/字体族/光标形状/闪烁/行高/内边距/调色板/透明度**已全部接线**，字体族还带真实安装探测。

**P1-ad 版式与交互对齐（多轮，本轮）**
- 设置面板全量改造为分组卡片语言 + 页面头部 + 侧栏字形/选中 + 权限两层 + 每集成一卡 + 本地工具连接表单。
- 启动器：图标底板/字形集、触发提示、分节标题、模式空状态、平台化徽标、动作栏 Enter 徽标、停靠状态行、
  筛选 chip 轴、共享命令词警告点、调用行。
- 终端：浮动栏 + 呼吸点 + 已结束会话保留 + 字体系探测。

**已知差距：历史行的删除键（框架限制）**
- 旧版的 `⌃⌫` 删除选中的剪贴板/计算器行在本框架不可达：聚焦的文本框编辑器会先取走所有 Backspace/Delete
  （`editor.wants` 对 m≠0 的 Backspace 与 Delete 都返回 true，且编辑器的分支在窗口快捷键之前，没有
  `claimed` 守卫）。删除的**能力**保留在选中行自己的 ✕ 按钮上（`row()` 里已经实现），只是键位换成了控件。
  等框架允许快捷键先于编辑器裁决，或暴露编辑器的 claimed 守卫后再补键位。

**P4-ab 本机工具连接 + 窗口放置**（本轮）
- 后端 `CreateCustom`/`PrepareCustom`（铸造 id、写包、help 推导、本地安装、版本探测）+ 前端表单 + shell 接线。
- 窗口放置纯函数 + 多显示器居中夹取；About 页 deep-link 卡片；held-modifier 的裸终端行。

**P4-ac 声明式输入 + 发现闭环**（本轮）
- manifest `params` + 校验 + `ParamArgv`/`ParamIssues`/`ParamDefaults`；连接请求可带 params；卡片运行表单。
- Detected 列表 + 一键连接；`floter://register` 解析并连接；从文件夹安装；freshness 行；禁用前确认。

**P4-ad 完成反馈与声明式输入编辑器**（本轮）
- 连接表单新增「参数」编辑器（每行：id/类型/旗标/必填/选项 + 添加与移除），行成为连接的声明。
- 窗口可见时完成反馈落在面板自己的停靠行（隐藏时才发系统通知）——之前可见时什么都不说。

## 尚未做（按建议优先级）

1. **列表协议的分页**：已完成（本轮）——`{"output": [...], "page": {cursor, hasMore}}` 的发射
   形态、内存按视口窗口化（首屏两页、滚到底加一页）、页脚四态（loading/more/end）与滚到底触发；
   自带 cursor 的插件会被带着 `FLOTER_PAGE_CURSOR` 重新执行，行按 id 去重合并。旧版还有
   `⌘1–⌘0` 的编号角标，Go 侧已完成。
2. **i18n 全量对齐**：UI 包内没有裸文案（已审计），旧 `src/i18n.ts` 的 1700+ 键大部分属于已不存在的
   页面（插件 iframe 页、剪贴板页、会话页），不为「对齐键数」而搬。
3. **浏览器插件**：已完成——Safari 的 `Bookmarks.plist`（二进制 plist，`internal/plist` + Safari
   自己的节点类型）现在也读了；这是旧版**没有**的能力（它只解析 Chromium JSON 与 Firefox
   places.sqlite），属于补上而不是对齐。
4. **Windows 图标**：已完成（P2-l）——`.lnk` 的图标经 PowerShell 提取并缓存为 PNG。
5. **系统通知**：已完成（后台命令、安装/卸载/检测）。电源动作带 SysV 回退（旧版 `run_first` 语义）。
6. **P5 打磨**：已完成；更细的换壳过渡受原生窗口尺寸变化限制，不做应用层过渡。
7. **P6 发布链**：剩下的是一次性维护者动作——`mygo keygen` 生成更新密钥、`mygo.json` 填 `updates`
   与 `macos.signingIdentity`/`macos.notarize`、CI 注入私钥、把产物发到 GitHub Release（含预发布通道）。
8. **小项**：开机自启需要在真实打包应用上验证一次；
   （命令别名编辑已完成：Integrations 页每命令一个输入框，P4-y；脚本语言表单已完成，P7-h；
   能力探测已完成：manifest `lifecycle.probes` 的执行与 `probeReport` 都在，见 P3-n；
   触发词进模式已完成，见 P7-t。）
   旧版 `package.json` 归档字段。（导入导出的权限审批已改为**一次汇总**，
   与旧版一致；旧版 `launch_counts` 已在启动时一次性导入 `usage.json`，本构建的记录优先。）

> 上面 1–8 已全部完成或只剩一次性维护者动作，所以这一轮的入口是第 3 条：对照原版实现做系统
> 排查。P7-t 找到的是「提示与键帽都在承诺一个从未接线的转换」；P7-u 找到的是同一类的第二处
> ——空态文案把用户指向「设置 > 集成」，而那一页从来没有内置插件区。

## 注意事项（踩过的坑）

- **Tester 的 `Type("\b")` 是插入文本，不是按退格**：测试里删字符要用 `tt.Key(0, ui.KeyBackspace)`。
- **视图里的快捷键先于聚焦元素的 `HandleInput`**：录制器必须在自己录的时候让出 Escape（已处理）。
- **程序设置文本后要显式把光标移到末尾**（`SetTextSelection`），否则打字会插进命令词中间导致模式退出。
- **测试不要碰真实系统资源**（剪贴板、登录项、窗口、原生库）：相关能力都在 `shell.Options` 里可注入（`WriteClipboard`、`RegisterShortcut`、`OpenAtLogin`、`OpenPinned`、`ConfirmPermissions`、`Registry`、`NewTerminal`、`Paths`）。
- **`mygo build` 曾因旧 `package.json` 生成空的 `src/mygo.ts`**；旧前端删除后已消失。
- **平台表要写成纯函数**：`internal/browser` 的浏览器路径表以前按宿主平台返回，Linux CI 直接红；
  现在是 `(goos, home, …)` 的纯函数，任何宿主都能断言三个平台的布局。运行期解析同理
  （`internal/extensions` 的 `lookTool`/`findInterpreter` 可注入，测试不看机器 PATH）。
- **原生库依赖**：终端插件要 libghostty-vt、浏览器插件要 libsqlite3；`go test` 首次会按需下载到 `~/Library/Caches/mygo/natives/`，打包时由 CLI 内嵌。
