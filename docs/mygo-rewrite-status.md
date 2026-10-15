# floter × mygo 重构：当前进度与后续规划

> 交接文档。状态时间：2026-10-16，分支 `mygo-rewrite`（已推送）。框架：mygo v0.5.1。
> **仓库现在只有 Go 实现**：Tauri/Rust/React 时代的代码已删除（P6-b）。
> 详细的分轮记录见 `docs/mygo-rewrite-plan.md`（含每轮的决策与「未做」清单）。

## 一句话状态

用 mygo（原生 GPU 自绘 UI + Go）重写的 floter **已经是一个可打包、可运行的完整应用**：单窗口三表面
（启动器 / 设置 / 终端）、搜索内核、扩展平台（含后台运行、列表协议、导入导出、权限审计）、剪贴板、
浏览器、计算器、系统集成、打包与 CI 全部就位。三平台都能构建，Linux 打包
（可执行文件 + .deb + tar.gz）与 macOS 打包（.app + .dmg）都已在本机跑通，并在真实数据上验证过
（旧扩展仓库、剪贴板历史、扩展配置）。仓库里只有 `cmd/` + `internal/` 的 Go 实现（约 30k 行，
含测试）。**1400 项测试全绿**（`go test -v ./... | grep -c -- "--- PASS"`，含子测试）。

## 快速上手（换到新电脑后）

```sh
git clone git@github.com:vst93/floter.git && cd floter
git checkout mygo-rewrite

# 门槛（全部必须过）
gofmt -l cmd internal          # 必须为空
go vet ./...
go test -count=1 ./...         # 1400 项测试
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
| `internal/inputtext` | 输入边界：Unicode 空格分隔符（Zs）→ U+0020、粘贴去一个尾换行；所有变成 PTY 字节的字符串都过这里 |
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
- 玻璃保真：材质阶梯 + 雾层 + `ScrollEdge`（读源码证伪了「用 glass.Blur 复刻桌面模糊」）；
  控件透镜按原版 `glass-reaxis` census 逐条落地（P7-em）、帧 sheen 回到原版固定 token（P7-en）。

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

> **本轮：P7-hd 分段控件只有材质选择器居中，其余段回到原版的「行」左对齐（原版
> `main:src/styles/settings.css:1055,1060` 的 `.settings-option { text-align:left; padding:0 9u }`
> 对照 `:792-799` 的 `.glass-intensity__segments .settings-option { text-align:center; justify-items:center;
> padding:0 6u }`）**
> ——P7-hc 收尾后「具体待办（P7-hb 重写）」余下第 3 条，本轮取它落地，P7-hb 重写的 3 条全部结清。
> 依赖检查：`go.mod` 已是 **v0.5.1** = `go list -m -versions github.com/egoist/mygo` 的最新
> （`api.github.com/repos/egoist/mygo/releases/latest` 仍被限流），无版本可升；只对照 `main`
> （`backup/main-pre-mygo` = v0.3.14 原版树），未翻历史提交。
>
> **偏离（P7-hb-3·基础段一律居中）**：原版裸 `.settings-option` 写 `text-align: left`
> （`settings.css:1060`），`Padding: 0 9u`（`:1055`），所以共享轨道 `.settings-options--inline` 里的
> 一个段是**行**——标签贴 9u 内缩、从左往右读；`SegmentedChoice`（theme/language/interface-size/字体、
> 插件表的 radio、终端抽屉的 padding/cursor）都走这一形。唯一居中的是材质选择器
> `.glass-intensity__segments .settings-option`（`:792-799`）：`text-align:center; justify-items:center`
> 且水平内缩收到 `0 6u`——CSS 自带的注释点明理由「A stop's label is the effect's name; centered
> because the segment is a choice, not a row」，即基础段是「行」、材质段才是「选择」。**本构建（改前）**：
> `internal/control/control.go` 的段一律 `Justify(ui.Center)`，所有调用方（设置页 pick、插件表
> sheetRadio、终端抽屉）都被居中，没有两态。
>
> **落地**：`internal/control/control.go` 的 `segmented` 加 `glass` 形参——基础态 `inset = Space(2.25)`
> （9u）、`Justify(ui.Start)`；材质态 `inset = Space(1.5)`（6u）、`Justify(ui.Center)`。新增导出
> `SegmentedGlassFill`（材质专用的全宽形），`Segmented`/`SegmentedFill` 保持基础态。
> `internal/settingsui/settingsui.go` 把 `pick` 拆成 `pickWith` + 两条包装：`pick`（基础）与
> **仅材质段用的** `glassPick`；新增 `a.glassSegmented`，General 页的材质 `a.pick(copy.Glass,…)` 改走
> `a.glassPick`。别的调用方调用未改，因此这次改动只把它们从「居中」纠正为原版的「行」。
>
> **验收**：新增 `internal/control/segmentalign_test.go` 的
> `TestTheBaseSegmentsAreRowsAndOnlyTheGlassTrackIsCentered`——从活控件采未选段标签自己的墨迹，量它
> 离段左右边的留白：基础段左留白 == 9u（`Space(2.25)`）且远小于右留白（是行），材质段左右留白相等
> （居中）且左留白 > 9u，且两形段高仍是 30u（`Space(7.5)`）。变异核查：把基础态改回 `Justify(ui.Center)`
> → 基础段左留白读成 80.2、右 81.4，与 9u 及「右留白 > 半宽」两条同时红。
>
> **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 全绿、
> `GOOS=linux/windows/darwin go build ./...` 三平台通过。
>
> **清单状态**：「具体待办（P7-hb 重写）」第 3 条（P7-hb-3）已落地，**3 条全部结清、清单归零**，
> 下一轮回到「对照原版排查偏离点」。

> **本轮：P7-hc 分段控件的标签整条轨道都回到原版的 `--text-strong`/580（原版
> `main:src/styles/settings.css:1060,1111-1120` 的 `.settings-option { text-align:left }`、
> `.settings-option__label { color: var(--text-strong); font-weight: 580 }`，对每个段一样）**
> ——P7-hb 收尾后「具体待办（P7-hb 重写）」余下第 2·3 条，本轮取第 2 条落地。依赖检查：`go.mod`
> 已是 **v0.5.1** = `go list -m -versions github.com/egoist/mygo` 的最新（`api.github.com` 本轮仍被
> 限流），无版本可升；只对照 `main`（`backup/main-pre-mygo` = v0.3.14 原版树），未翻历史提交。
>
> **偏离（P7-hb-2·未选段标签被压暗）**：原版 `.settings-option__label` 的 `color`/`font-weight`
> **不随选中改变**——选中态只由 `.settings-option--active`（`settings.css:1095-1098`）的 raised pane
> + accent 键线承担，裸 `.settings-option` 的字色是 `--text-primary`（`:1060`），没有任何按选中的
> 压暗规则。**本构建（改前）**：`internal/control/control.go` 的非当前段走 `t.TextMuted` + 500，
> 只有当前段 `TextStrong` + 580——这是本构建发明的差异。
>
> **落地**：`internal/control/control.go` 的 `segmented` 删掉按选中的 `TextMuted`/500 分支，标签统一
> `tokens.TextStrong` / 580（`t.TextMuted` 不再被引用）。选中态仍由段上的 `ControlHover` + `Lens` +
> `AccentEdge` 键线承担，未动。
>
> **验收**：新增 `internal/control/segmentlabel_test.go` 的 `TestEverySegmentLabelIsTextStrong`——
> 用 `tt.Image()` 采每条标签自己的墨迹像素（离该段表面最远的像素 = 字形自身的颜色），断言它与
> `tokens.TextStrong` 合成到该段表面后的值相等、且**不**等于 `TextMuted` 的合成值（选中/未选三段
> 都测）。变异核查：把 `TextMuted`/500 分支加回 → 未选段读成 `{146 147 150}` 而目标是 `{236 237 239}`，
> 红。
>
> **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 全绿、
> `GOOS=linux/windows/darwin go build ./...` 三平台通过。
>
> **清单状态**：「具体待办（P7-hb 重写）」第 2 条（P7-hb-2）已落地，余下第 3 条（基础段的对齐）
> 待下一轮。

> **本轮：P7-hb 设置/抽屉共用分段控件的段盒回到原版的 30u 下限（原版 `main:src/styles/settings.css:1031-1055`
> 的 `.settings-option { min-height: calc(var(--u) * 30); padding: 0 calc(var(--u) * 9) }`，R43 把 34u 收到
> 30u、轨道内缩 3u→2u）**
> ——P7-ha 收尾后「具体待办（P7-gx 重写）」4 条全部结清、清单归零，本轮按规则回到「对照原版排查偏离点」，
> 换一个**全文档从未单独审计**的原版入口：`SegmentedChoice`（`main:src/settings/SettingsRows.tsx:152-197`）
> 与它穿的 `.settings-options`/`.settings-option`/`.settings-option__label`（`settings.css:763-765`、
> `:1012-1035`、`:1067-1120`），本构建对应 `internal/control/control.go` 的 `Segmented`/`SegmentedFill`。
> P7-ab 只对过材质（轨道 `LensInset`、选中段 `Lens` + accent 键线），P7-em 只对过透镜 census，
> **段的度量/字级/对齐/字色从未逐字段比对**。依赖检查：`go.mod` 已是 **v0.5.1** = `go list -m -versions`
> 的最新（`api.github.com` 本轮被限流），无版本可升；只对照 `main`（`backup/main-pre-mygo` = v0.3.14
> 原版树），未翻历史提交。
>
> **偏离（P7-hb·段盒没有原版的 30u 下限）**：原版把段盒写成 `.settings-option` 的
> `min-height: calc(var(--u) * 30)`（`settings.css:1045`）加 `padding: 0 calc(var(--u) * 9)`
> （`:1055`）——高度来自**下限**，不来自竖直内边距，所以标签短也是一格 30u。**本构建（改前）**：
> `internal/control/control.go:47` 用 `Padding(t.Space(0.75), t.Space(2.25))`（竖直 3u + 水平 9u）
> 且没有下限，段盒高度于是随标签行盒走：默认界面档实测 **22.4px**，原版的 30u 是 **27px**——段比它
> 抄的那个矮约六分之一，且高度会随标签变化。
>
> **落地**：`internal/control/control.go` 的 `segment` 改
> `Padding(0, t.Space(2.25)).MinHeight(t.Space(7.5))`（= `padding: 0 9u; min-height: 30u`），
> 原有的 `AlignItems(ui.Center)` 负责把行盒放到中线上；`Segmented`/`SegmentedFill` 两形共用这一处，
> 设置页的 theme/language/cursor/interface-size 选择器与终端抽屉的 cursor/palette 选择器同时生效。
>
> **验收**：新增 `internal/control/segmentmetrics_test.go` 的 `TestTheSegmentBoxIsThirtyUnitsHigh`——
> 三条段的 `Find` 盒高都 == `Space(7.5)` = 30u，且互等（选中段只换材质、不换盒高）。变异核查：
> 摘掉 `MinHeight` → 三条 17.0 ≠ 27.0，红。
>
> **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 29 包全绿、
> `GOOS=linux/windows/darwin go build ./...` 三平台通过。
>
> **清单状态**：「尚未做」被重写为下面 3 条带 file:line / 原版对照 / 验收标准的具体待办
> （见「具体待办（P7-hb 重写）」）；本轮取第 1 条落地。余下第 2 条（未选段标签的字色/字重）、
> 第 3 条（基础段的对齐）待下一轮。

> **本轮：P7-ha 终端空态去掉原版没有的第二句说明（原版 `main:src/App.tsx:3852-3872` 的空态只有
> `terminal-empty__title` + 两条 `terminal-empty__line`；`main:src/styles/terminal.css:346-350`）**
> ——P7-gz 收尾后「具体待办（P7-gx 重写）」只剩第 4 条，本轮取它落地，P7-gx 重写的 4 条全部结清。
> 依赖检查：`go.mod` 已是 **v0.5.1** = GitHub releases latest（本轮 `releases/latest` 复核为
> v0.5.1），无版本可升；只对照 `main`（`backup/main-pre-mygo` = v0.3.14 原版树），未翻历史提交。
>
> **偏离（P7-ha·空态多画了一句原版没有的说明）**：原版的空态是标题 + 两条**都是真动作**的行
> （Enter / 用户自己的返回键，`App.tsx:3852-3872`），标题用 `--text-emphasis`
> （`terminal.css:346-350`），**没有**第二段说明。**本构建（改前）**：`internal/terminalui/terminalui.go`
> 的空态分支在标题之后多画了一句 `ui.Text(c, copy.Hint)`——`copy.Hint` = “The session has not
> started yet.”，与标题 “No terminal session yet” 语义重复（`internal/i18n/copy.go:953,1802`）。
>
> **落地**：`internal/terminalui/terminalui.go` 的空态分支删掉 `hint` 变量与那句 `copy.Hint`，只留
> `EmptyTitle` 与两条动作行（`emptyLine`）；启动失败时**唯一**保留的一行是 `a.Err.Error()`，
> 因为原版这页没有自己的启动错误面，而本构建要如实说出为什么没能开会话（`TestTerminalSessionErrorShows`
> 继续覆盖）。`internal/i18n/i18n.go` 删除 `Terminal.Hint` 字段（en/zh 两份字面量同步删值，结构化
> Copy 因此编译期确认无漏）；`internal/terminalui/view_test.go` 的 `TestTerminalEmptyState` 改断言
> 空态**有** `EmptyTitle`/`EmptyNew`/`EmptyBack`、**无** “The session has not started yet.”；
> `internal/shell/shell_test.go` 的终端快照把该串换成 `EmptyTitle`。
>
> **验收**：`TestTerminalEmptyState`（空态三件套在、被删的句子不在）、`TestTerminalSessionErrorShows`
> （失败原因仍显示）、`TestSurfaceSnapshots` 的 terminal 例（快照改要 `No terminal session yet`）。
> 变异核查：把 `copy.Hint` 那一行加回 → `TestTerminalEmptyState` 的「不再含 Hint 串」条红。
>
> **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 29 包全绿、
> `GOOS=linux/windows/darwin go build ./...` 三平台通过。
>
> **清单状态**：「具体待办（P7-gx 重写）」第 4 条（P7-gx-4）已落地，**4 条全部结清、清单归零**。
> 下一轮回到「对照原版排查偏离点」，换一个尚未单独审计的原版入口重新写实待办。

> **本轮：P7-gz 终端浮栏悬停时的 `--elev-bar` 阴影（`terminal.css:99-104` 的
> `@media (hover:hover) .terminal-bar:hover { box-shadow: var(--elev-bar) }`）**
> ——P7-gy 收尾后「具体待办（P7-gx 重写）」余下第 3·4 条，本轮取第 3 条落地。依赖检查：`go.mod`
> 已是 **v0.5.1**，`go list -m -versions` 复核无 v0.5.2+（`api.github.com/releases/latest` 本轮被
> 限流，故改用 proxy 列表），无版本可升；只对照 `main`（`backup/main-pre-mygo` = v0.3.14 原版树），
> 未翻历史提交。
>
> **偏离（P7-gz·悬停浮栏不给它自己投影）**：原版 `.terminal-bar`（`terminal.css:90-97`）本身
> `background: transparent; box-shadow: none`，但在 `@media (hover:hover)` 下 `:hover` 加上
> `--elev-bar`——`base.css:224-226` 的 `0 6px 16px rgba(0,0,0,calc(0.18 * var(--elev-shadow-scale)))`，
> CSS 注释点名它是「把深度重新投在下面滚动的内容上」的轻阴影，是「这里可以拖」的暗示，不是第四级
> elevation。**本构建（改前）**：`internal/terminalui/terminalui.go` 的 `titleBar`（`:1059-1077`）
> 悬停时只把身份行与按钮 `revealed`，从不给浮栏自己加阴影——浮栏在指针下抬不起来。
>
> **落地**：`internal/terminalui/terminalui.go` 的 `titleBar` 在 `bar.Hovered()` 时调
> `bar.Shadow(0, t.Space(1.5), t.Space(4), 0, ui.RGBA(0, 0, 0, 0.18))`——offset/blur 取原版的
> `0 6px 16px` 折成 `Space`，alpha 用 base `0.18`（与 `sessionNote`/`feedbackNote` 用 `--elev-3` 的
> base `0.3` 同法，本构建的阴影都不随窗口透明度缩放）；键盘焦点仍只揭示身份行、不抬升表面，与原版
> `:hover` 一致。
>
> **验收**：新增 `internal/terminalui/barshadow_test.go` 的
> `TestTheTerminalBarLiftsItsShadowOnHover`——在灰色画布上以「指针在浮栏上」与「指针离开」两帧同列对比：
> 浮栏正下方第一行（`Space(7)` 处）在悬停帧更暗（实测 idle 184 → hovered 158）、未悬停帧那一行就是
> 裸画布（±4）、离浮栏 30px 外两帧无差（阴影有落点、不是平铺）。变异核查：删掉 `bar.Shadow` → 悬停行
> 与 idle 完全同值（184 == 184）红。
>
> **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 全绿、
> `GOOS=linux/windows/darwin go build ./...` 三平台通过。
>
> **清单状态**：「具体待办（P7-gx 重写）」第 3 条（P7-gx-3）已落地，余下第 4 条（空态多画一行说明）
> 是下一轮最高优先级。

> **本轮：P7-gy 终端外观抽屉的 slider 宽度/读数/格宽回到原版 `terminal.css`（`.terminal-settings`
> 的 `minmax(230px, 1fr)`、`.terminal-setting-range` 的 `width: 110px`、
> `.terminal-setting-control__value` 的 body/等宽/accent/tabular）**
> ——P7-gx 收尾后「具体待办（P7-gx 重写）」余下第 2·3·4 条，本轮取第 2 条落地。依赖检查：`go.mod`
> 已是 **v0.5.1**，`go list -m -versions` 复核无 v0.5.2+（`api.github.com` 本轮被限流），无版本可升；
> 只对照 `main`（`backup/main-pre-mygo` = v0.3.14 原版树），未翻历史提交。
>
> **偏离 1（P7-gy·slider 的宽度不是原版的 110px）**：原版 `.terminal-setting-range`
> （`terminal.css:677-692`）是 `width: 110px`。**本构建（改前）**：`internal/terminalui/drawer.go:72,100`
> 两个 range 都写 `Width(t.Space(22))` = 88px，比原版窄 22px。
>
> **偏离 2（P7-gy·读数的字级/颜色/字形都不是原版的数）**：原版是 `settings.css:854-865` 的
> `.terminal-setting-control__value`——`font-family: "SF Mono", Menlo…`、`font-size: var(--text-body)`、
> `font-variant-numeric: tabular-nums`、`font-weight: 600`、`color: var(--accent)`。**本构建（改前）**：
> 读数是 `FontSize(tokens.Emphasis).TextColor(t.TextMuted)`——小一档、muted 而非 accent、无等宽栈、
> 无 tabular。
>
> **偏离 3（P7-gy·格下限是 240px 而不是原版的 230px）**：原版 `.terminal-settings`
> （`terminal.css:654-658`）是 `grid-template-columns: repeat(auto-fit, minmax(230px, 1fr))`。
> **本构建（改前）**：`drawerRow` 的格下限调用从 `Space(60)` = 240px。
>
> **落地**：
> - `internal/terminalui/drawer.go`：新增 `drawerRangeWidth` 常量（27.5 = 110px）与 `drawerRange`/
>   `drawerReadout` 两个 helper；两个 slider 改走 `drawerRange`（固定宽度，不随格伸缩），两处读数
>   改走 `drawerReadout`（等宽栈 + body 步 + accent + 600 + `tnum`）；新增 `drawerCellFloor`（57.5 =
>   230px）/`paletteCellFloor`（95）+ `drawerGrid(c, cellFloor)`，`drawer` 调 `drawerGrid(c,
>   t.Space(drawerCellFloor))`——格下限成为可测参数，`drawerGrid` 一行一个格。
> - 顺带把 `drawer` 里的行构造拆成 `drawerGrid`，让测试能在**同一 window** 下用两个 floor 渲染真
>   网格（`drawerRow`/`drawerGrid` 都是生产路径，不是测试替身）。
>
> **验收**：新增 `internal/terminalui/drawermetrics_test.go` 三条——
> `TestTheDrawerSliderIsTheReferenceWidth`（活抽屉：font-size 设到范围两端，range 自己的**旋钮**
> 在标签与读数之间的**亮墨迹中心**移动的距离，比同法渲染在 22u 宽度下的参考更接近 27.5u 下的参考；
> 先钉 `drawerRangeWidth == 27.5`）、`TestTheDrawerReadoutIsTheAccentBodyRegister`（活抽屉：读数的
> 盒宽更接近 body 参考而非 emphasis；读数盒内最强像素的 **蓝−红** 通道差 ≥40 且 **绿−红** ≥15，
> 即 accent 墨迹（muted 是近中性灰，差只几个单位））、`TestTheDrawerGridCellsKeepTheReferenceFloor`
> （`drawerGrid` 在 57.5u 与 60u 两个 floor 下对 row-lines 从 3 折到 2 的窗口宽度做二分：
> 57.5u 的折点更靠左，且两折点**之间**的窗口在原版 floor 下更少一行）。
> 变异核查：`drawerRangeWidth` 改回 22 → 宽度条红（实测 live travel −61 = probe 22，`reference == old` 直接 Fatal）；
> `drawerReadout` 改回 `Emphasis`/`TextMuted` → 读数条红（box 25 vs body 27；最强像素蓝−红 = 6）；
> `drawerCellFloor` 改回 60 → 格宽条红（`drawerCellFloor != 57.5` 直接 Fatal）。
>
> **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 全绿、
> `GOOS=linux/windows/darwin go build ./...` 三平台通过。
>
> **清单状态**：「具体待办（P7-gx 重写）」第 2 条（P7-gx-2）已落地，余下第 3 条（浮栏 hover 阴影）
> 与第 4 条（空态多出的第三行）是下一轮最高优先级。另：本轮实测中 `internal/launcher` 的
> `TestTriggerHintFits` 在一次全量跑里红过一次（窗口太小），单跑与重跑均绿——如实记录为**环境相关
> 的偶发**，未改它。

> **本轮：P7-gx 终端浮栏的身份行与状态点回到原版 `terminal.css`（`.terminal-bar__title`
> 的 body 步 / `--text-secondary` / `-0.01em`；`.terminal-bar__dot--exited` 的 `--text-tertiary`；
> 活状态点的 `--elev-halo`）**
> ——P7-gw 收尾后「具体待办（P7-gu 重写）」归零，本轮按规则回到「对照原版排查偏离点」，换一个
> **全文档从未审计**的原版入口：终端表面 chrome 的**度量与颜色**表 `main:src/styles/terminal.css`
> （浮栏/状态点/抽屉/驻留条/空态）。`grep -c` 复核：`terminal-bar__title`、`terminal-settings__row`、
> `terminal-setting-range`、`terminal-resident`、`terminal-empty`、`terminal-copy-notice` 在 `docs/`
> 全为 0——`terminal.css` 此前只对照过浮栏的**词与 tooltip**（P7-fl/fm）和空态两行（P7-cm），
> 视觉度量从未逐条比对。依赖检查：`go.mod` 已是 **v0.5.1**，`go list -m -versions` 复核无
> v0.5.2+（`api.github.com` 本轮被限流，故改用 proxy 列表），无版本可升；只对照 `main`
> （`backup/main-pre-mygo` = v0.3.14 原版树），未翻历史提交。
>
> **偏离 1（P7-gx·浮栏身份行的字级与颜色都低了一档）**：原版 `.terminal-bar__title`
> （`terminal.css:165-175`）是 `color: var(--text-secondary)`、`font-size: var(--text-body)`、
> `font-weight: 580`、`line-height: 1`、`letter-spacing: -0.01em`。**本构建（改前）**：
> `internal/terminalui/terminalui.go:1094-1096` 的 `titleBar` 身份文字是
> `FontSize(t.FontSize - 1).FontWeight(580).TextColor(t.TextMuted)`——比原版**小一档**、且落在
> **muted** 而非 **secondary**，没有 `letter-spacing`。也就是说，浮栏里那个说出「你正在看哪个会话」
> 的名字，是它所在浮栏里最安静的一行。
>
> **偏离 2（P7-gx·退出状态点取了 muted 而不是 tertiary）**：原版 `.terminal-bar__dot--exited`
> （`terminal.css:158-163`）是 `background: var(--text-tertiary)`、`opacity: 0.7`、`box-shadow: none`。
> **本构建（改前）**：`statusDot`（`terminalui.go:1136-1152`）落 `t.TextMuted`——比原版**亮一档**。
>
> **偏离 3（P7-gx·活状态点缺 `--elev-halo`）**：原版 `.terminal-bar__dot`（`terminal.css:148-157`）
> 除 7px 的 `--accent` 圆点外还画 `box-shadow: var(--elev-halo)`，而 `--elev-halo`
> （`base.css:206-207`）正是 `0 0 0 2px var(--accent-halo)`（`--accent-halo` = dark
> `rgba(143,183,255,0.18)` / light `rgba(6,74,166,0.24)`，`base.css:693,958`）——这是原版唯一一条
> **非 elevation** 的阴影，被点名声明为「状态辉光」。**本构建（改前）**：`statusDot` 只画圆点，
> 没有任何环，退出态把 `box-shadow` 一起丢掉是对的、但活态本来就没有。
>
> **落地**：
> - `internal/theme/theme.go`：Tokens 新增 `AccentHalo`（base.css 的 `--accent-halo`），palette
>   两个外观各一份（dark `RGBA(143,183,255,0.18)`、light `RGBA(6,74,166,0.24)`），`AccentHalo` 落到
>   `tokens` 上；`theme_test.go` 钉住两个值。
> - `internal/terminalui/terminalui.go`：`titleBar` 的身份文字改 `FontSize(t.FontSize)`、
>   `TextColor(tokensOf(a, c).TextSecondary)`、`LetterSpacing(-0.01 * t.FontSize)`；`statusDot` 的
>   退出分支改 `tokens.TextTertiary`，活分支加 `dot.Shadow(0, 0, 0, t.Space(0.5), tokens.AccentHalo)`
>   （2u 的 `--elev-halo` 环）。
>
> **验收**：`internal/terminalui/barchrome_test.go` 两条——
> `TestTheTerminalBarTitleIsTheReferenceSecondaryBody`（活浮栏：身份行墨迹宽度更接近 body 参考而非
> body−1；墨迹质量更接近 `--text-secondary` 参考而非 `--text-muted`）与
> `TestTheTerminalBarDotCarriesItsHaloAndWearsTertiary`（驻留会话：点的**中心像素**逐字节等于
> `--text-tertiary`@0.7 的实心填充、且不等于 muted@0.7；活会话：中心像素等于 `--accent`，且墨迹宽度
> 比裸点宽 ≥3——2u 的环被画出来）。**中心像素**法：点在圆心处满覆盖，正对上文「同一列、同相位」的
> 亚像素抗锯齿难题；把参考填充渲染在同一相位会把差别抹平，所以改用圆心的整数像素。变异核查：
> - `FontSize(t.FontSize-1)`+`t.TextMuted` → 两条断言红（实测 ink 147 vs body 173/body−1 147、
>   mass 39359 vs secondary 54675/muted 49674）；
> - `TextTertiary` 换回 `t.TextMuted` → 中心 `{33 33 37}` vs 期望 `{30 30 34}` 红；
> - 去掉 `dot.Shadow(...)` → 活点宽 6 = 裸点宽 6，环未画红。
>
> **门槛**：在隔离 worktree（HEAD + 本轮改动）`gofmt -l cmd internal` 空、`go vet ./...` 净、
> `go test -count=1 ./...` 29 包全绿、`GOOS=linux/windows/darwin go build ./...` 三平台通过。
>
> **清单状态**：「尚未做」被重写为下面 4 条带 file:line / 原版对照 / 验收标准的具体待办
> （见文末「具体待办（P7-gx 重写）」）；本轮取第 1 条落地（= 偏离 1·2·3 三条，同在浮栏）。余下
> 第 2 条（抽屉 slider 的宽度/读数/格宽）、第 3 条（浮栏 hover 阴影）、第 4 条（空态多出的第三行）
> 是下一轮最高优先级。
> **本轮插曲（如实记录）**：与本轮并发的另一个 pi 正在同一工作树改
> `internal/settingsui/uninstall.go`（卸载对话框的组件化 WIP）并留下一个编译不过的
> `internal/settingsui/zzprobe_test.go`；本轮不吞入它，改动在隔离 worktree 完成并提交，推送前把远程
> 已落地的并发提交（`264d991`，同样自标 P7-gw）一起 rebase 进来——本轮因此标 **P7-gx**，避开重名。

> **本轮：P7-gw 插件配置表的 Number/Slider 控件面回到原版 R59（`plugin-config.css:97-192` 的
> 28u `--glass-field` 槽 + `--glass-field-shadow`；slider 值回 `--text-caption`/`--text-muted`/tabular）**
> ——P7-gu/P7-gv 之后「具体待办（P7-gu 重写）」只剩第 3 条，本轮取它落地。依赖检查：`go.mod` 已是
> **v0.5.1**，`goproxy.io` 的 `@v/list` 复核仍到 v0.5.1（`api.github.com` 本轮被限流，故改用 proxy 列表），
> 无版本可升；只对照 `main`（`backup/main-pre-mygo` = v0.3.14 原版树），未翻历史提交。
>
> **偏离 1（Number 是框架的 `ui.NumberInput`）**：原版 R59 的 `.plugin-config-number`
> （`plugin-config.css:153-164`，与 `.plugin-config-text` 共享 `:97-118` 的盒：`height:28u`、
> `border:1px solid var(--glass-control-edge)`、`--radius-sm`、`background:var(--glass-field)`、
> `box-shadow:var(--glass-field-shadow)`）是**单位在盒内、输入右对齐的 52u 槽**：`display:inline-flex`、
> `gap:4u`、`padding:0 8u`，`__input` 宽 52u、tabular、右对齐、无自盒，`__unit` 是 `--text-caption` +
> `--text-muted`。**本构建（改前）**：`pluginsheet.go` 的 `plugincfg.Number` 走 `a.sheetNumber` 的
> `ui.NumberInput`——框架自带的带框面板 + `−`/`+` 步进器 + 正文步进，盒高与 `--glass-field-shadow`
> 都不是原版的。
>
> **偏离 2（Slider 值文字是正文步进）**：原版 `.plugin-config-slider__value`
> （`plugin-config.css:174-192`）是 `--text-caption` + `--text-muted` + `tabular-nums` +
> `min-width:48u` 右对齐，轨道与值之间 `gap:10u`。**本构建（改前）**：`sheetSlider` 的值文字是
> `t.FontSize`（正文步进）、gap 8u，没有 48u 值列。
>
> **落地**：
> - `internal/settingsui/pluginsheet.go`：新增共享的 `sheetFieldSlot(c)`（28u、`Padding(0,8u)`、
>   `RadiusSM`、`ControlEdge` 发丝、`Field` 底、`tokens.FieldSlot` 的内阴影栈），`sheetTextField` 与新的
>   `sheetNumberSlot` 都走它；`sheetNumber` 不再用 `ui.NumberInput`，改画原版槽（`TextInputBase`
>   宽 52u / `TextAlign(End)` / `FontFeatures("tnum")`，单位在盒内 caption+muted，gap 4u）；数值边输边
>   应用、越界在写回时按 `clampNumber` 收进 schema 的 `[Min,Max]`，半打的文本存进 `App.pluginNumberDrafts`
>   直到失焦再回读存储值（与框架 `ui.NumberInput` 的本地草稿同义）。`sheetSlider` 的值文字改
>   `tokens.Caption`/`TextMuted`/tabular、放 48u 右对齐列，轨道→值 gap 回到 10u。
> - `internal/theme/theme.go`：新增 `Tokens.FieldSlot`——base.css 的 `--glass-field-shadow`
>   （`:524-527` dark：contact + lens-rim + lens-edge；`:887-888` light：一条固定 0.85 白 rim + 0.06
>   黑 contact，无底影），层序与 `LensInset` 同法（后加者在上）。
> - `internal/settingsui/settingsui.go`：`App` 新增 `pluginNumberDrafts map[string]string`。
>
> **验收**：`internal/settingsui/plugincontrols_test.go` 三条——
> `TestThePluginNumberControlIsTheReferenceGlassSlot`（活 Browser 表：单位左侧一列扫出 28u 高的
> `--glass-field` 面、与卡片面差 ≥40；单位宽度更接近 caption 而非 body）、
> `TestThePluginSliderValueIsTheReferenceCaption`（活 Clipboard 表：值文字宽度更接近 caption 而非 body）
> 与 `TestThePluginNumberControlAppliesAndClampsTyping`（聚焦→清空→打 "45" 得 45，不粘前值；打 "99999"
> 写回被 clamp 到 `MaxBrowserHistoryDays`）。变异核查：把 `sheetNumber` 改回 `ui.NumberInput` → 槽高
> 断言红；把 slider 值改回 `t.FontSize` → register 断言红；去掉草稿状态 → 打字断言红（得 450）。
>
> **门槛**：在隔离 worktree（HEAD + 本轮改动）`gofmt -l cmd internal` 空、`go vet ./...` 净、
> `go test -count=1 ./...` 29 包全绿、`GOOS=linux/windows/darwin go build ./...` 三平台通过。
>
> **清单状态**：`docs/mygo-rewrite-status.md` 的「具体待办（P7-gu 重写）」第 3 条已落地，清单归零 →
> 下一轮回到「对照原版排查偏离点」，换一个未审计的原版入口重新写实待办。
> **本轮插曲（如实记录）**：与本轮并发的另一个 pi 正在同一工作树改 `uninstall.go` / `zzprobe_test.go`
> （卸载对话框的组件化 WIP，且 `zzprobe_test.go` 编译不过），本轮不吞入，改动在隔离 worktree 完成并提交。

> **本轮：P7-gu/P7-gv 插件配置表的控件面回到原版 R59（`plugin-config.css:97-118` 的
> `.plugin-config-text` 28u 玻璃槽；`:223-260` 的 `.plugin-config-action__button` 安静 hairline 药丸，
> 危险变体取 `--text-warning`；`:218-224` 的 6u 按钮簇；`src/i18n.ts:248,1146` 的确认问句）**
> ——P7-gt 收尾后清单归零，本轮按规则回到「对照原版排查偏离点」，换一个**全文档从未审计**的原版入口：
> 通用插件配置表 `main:src/plugins/PluginConfigOverlay.tsx` + `main:src/plugins/controls.tsx` +
> `main:src/styles/plugin-config.css`（`grep -c "plugin-config.css" docs/` = 0）。依赖检查：`go.mod` 已是
> **v0.5.1**，`releases/latest` 复核仍为 v0.5.1（无新版），无版本可升；只对照 `main`
> （`backup/main-pre-mygo` = v0.3.14 原版树），未翻历史提交。
>
> **偏离 1（P7-gu·Action 控件是框架的填充按钮）**：原版 R38 的 `.plugin-config-action__button`
> （`plugin-config.css:223-260`）是**安静 hairline 药丸**：`height: 28u`、`padding: 0 10u`、
> `border: 1px solid var(--glass-control-edge)`、`border-radius: 999px`、`background: var(--glass-control)`、
> `font-size: var(--text-caption)`；`--danger` 变体把描边换成
> `color-mix(in srgb, var(--text-warning) 45%, transparent)`、墨色换成 `--text-warning`、底色
> `transparent`——它**是破坏性动作，所以绝不能读成表的主按钮**。**本构建（改前）**：
> `internal/settingsui/pluginsheet.go:268-288` 的 `sheetAction` 用 `ui.Button`（框架的填充按钮：
> 14u 侧内边距、中性描边、正文步进），`field.Confirm` 配 `t.Danger`、`field.Cancel`/`field.Label`
> 用默认面，三颗按钮都读成药丸主按钮。
>
> **落地**：
> - `internal/settingsui/pluginsheet.go`：新增 `sheetActionButton(c, label, danger, run)`——`ui.ButtonBase`
>   + `MinHeight(Space(7))`（28u）+ `Padding(0, Space(2.5))`（0 10u）+ `Radius(Space(3.5))`（14u，
>   `999px` 在本盒高下等价于半高）+ `Border(1, edge)` + `Background(plane)` + `Caption` 墨字；
>   danger 面取 `t.Warning` 墨、`t.Warning.Alpha(0.45)` 描边、`ui.Transparent` 底，hover 一律
>   `tokens.ControlHover`；`sheetAction` 的三颗按钮改走它，按钮簇的 `Gap` 对齐原版
>   `.plugin-config-action` 的 6u（`Space(1.5)`），失败行改用 `tokens.Caption`。
> - `internal/plugincfg/schema.go`、`internal/i18n`：原版 `config-schema.ts:210-219` 的 action 字段把
>   `confirmKey` 接的是 `plugins.config.clearHistoryConfirm`（"Clear history?" / "确认清空历史？"，
>   `i18n.ts:248,1146`），`cancelKey` 是 `clearHistoryCancel`。本构建此前 `Confirm` 复用了按钮词
>   `ClipboardClearButton`（"Clear"/"清空"），于是 armed 行读成第二颗同字按钮、而非问句。现新增
>   `i18n.Settings.ClipboardClearConfirm` 并接进两处 `clear_history`（与原版逐字一致）；
>   `internal/settingsui/pluginsheet_test.go` 与 `internal/shell/pluginconfig_test.go` 的第二按下
>   改点该问句。
>
> **偏离 2（P7-gv·Text 字段不是原版的控件盒）**：原版 `plugin-config.css:97-118` 的
> `.plugin-config-text` 是 **28u 的玻璃槽**：`height:28u`、`padding:0 8u`、`Box-sizing:border-box`、
> `border:1px solid var(--glass-control-edge)`、`--radius-sm` 圆角、`background:var(--glass-field)`、
> `--glass-field-shadow`、`--text-body`。**本构建（改前）**：`pluginsheet.go` 的 `plugincfg.Text` 走
> `a.text(...)`（`settingsui.go:3047` 的 `textInput(...).Width(220)`），即**框架默认输入框**——不贴
> `--glass-field` 槽底、不是 28u 盒、也没有 8u 侧内边距与 `--radius-sm`。原版还把它归到
> `STACKED_KINDS`（`controls.tsx:291-296`），占整行而不是尾随槽。
>
> **落地（P7-gv）**：
> - `internal/settingsui/pluginsheet.go`：新增 `sheetTextField(c, placeholder, value)`——
>   `ui.Row` + `MinHeight(Space(7))`（28u）+ `Padding(0, Space(2))`（0 8u）+ `Radius(tokens.RadiusSM)`
>   + `Border(1, tokens.ControlEdge)` + `Background(tokens.Field)`，里面放 **`ui.TextInputBase`**（框架
>   的无面输入框，所以它的自带 padding/surface/border 不会与槽打架）；`plugincfg.Text` 改走
>   `a.stackedRow(...)`（原版 STACKED）而不是尾随 `a.text`。
>
> **验收**：`internal/settingsui/pluginsheetface_test.go` 三条——
> `TestThePluginActionButtonIsTheReferenceHairlinePill`（单渲染：盒高 == 28u、标签两侧内边距 == 10u、
> danger 描边与墨色都是暖色、药丸圆角在顶行就把描边拉进 ≥5px；框架 `ui.Button` 的 `--radius-xs`
> 圆角与中性配色两处都不符）、`TestThePluginActionClusterIsWarmAndAsksBeforeItActs`（活表：danger
> 药丸右缘外 40u 带内的暖色墨迹 warmth ≥30——黑色根帧下读到 `--text-warning` 自身的 R−B ≈ 123，
> 框架 `ui.Button` 的中性描边 warmth = 0；armed 第二帧画出 `Clear history?` 问句，且它不是按钮词）
> 与 `TestThePluginTextFieldIsTheReferenceGlassSlot`（活 Browser 表：帮文之下的槽高 == 28u、槽底比
> 卡片面亮 ≥40 通道）。变异核查：把 action 三颗按钮改回 `ui.Button` → 圆角与暖色两条红（warmth 0）；
> 把 Text 改回 `a.text` → 槽高条红（实测 2）。
>
> **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 全绿（29 包）、
> `GOOS=linux/windows/darwin go build ./...` 三平台通过。
>
> **清单状态**：`docs/mygo-rewrite-status.md` 的「尚未做」被重写为 3 条带 file:line / 原版对照 /
> 验收标准的具体待办（见文末「具体待办（P7-gu 重写）」），本轮取第 1·2 条一并落地（P7-gu/P7-gv）；
> 只剩第 3 条（Number / Slider 的控件盒，做不了便如实记录），是下一轮最高优先级。
> **本轮插曲（如实记录）**：本提交前 `settingsui.go` 已被另一个并行 WIP 改动（`motion`/`capture` 的
> 快捷键可视化重构，含一份未提交且编译不过的 `shortcutvisual_test.go`），本轮不吞入它，只保留自己
> 的两处改动。

> **本轮：P7-gt 材质 section 回到原版自己的 10u 间距与 10u 上下内边距（`.settings-section--material`，
> 原版 `src/styles/settings.css:767-772,491-495`）**
> ——P7-gs 收尾后「具体待办（P7-gr 重写）」只剩第 3 条，本轮取它落地。依赖检查：`go.mod` 已是
> **v0.5.1**，`releases/latest` 复核仍为 v0.5.1（无新版），无版本可升；只对照 `main`
> （`backup/main-pre-mygo` = v0.3.14 原版树），未翻历史提交。
>
> **偏离（P7-gt·材质区与其它 section 用同一个 8u 间距）**：原版 General 页的材质区是唯一一个
> `.settings-section--material`，它同时是 `gap: calc(var(--u) * 10)` 与 `padding: calc(var(--u) * 10) 0`
> （`settings.css:767-772`），比普通 `.settings-section` 的 8u（`:411-419`）更松；它的两张卡之间又是
> `.settings-cards { gap: 10u }`（`:491-495`）。**本构建（改前）**：`internal/settingsui/rows.go` 的
> `sectionFoot` 被所有 section 共用，只有 `Gap(t.Space(2))`（8u），材质区没有自己的 10u gap 与
> `padding: 10u 0`。
>
> **落地**：
> - `internal/settingsui/rows.go`：新增 `sectionGroupSpaced(c, title, gap, padding, fn)`——sectionGroup
>   的可选间距版本（`padding > 0` 时 `Padding(padding, 0)`），`sectionGroup` 委托它；新增材质专用的
>   `sectionMaterial(c, title, footnotes, fn)`（`gap: 10u`、`padding: 10u 0`，脚注在两张卡之下），
>   `sectionFoot` 被它取代并删除；`section`/`sectionGroup` 现在返回组元素（调用方照旧忽略），测试
>   因此能取到组自己的盒子。
> - `internal/settingsui/settingsui.go`：`general` 的材质段从 `a.sectionFoot(...)` 改走
>   `a.sectionMaterial(...)`。
> - 10u 内边距落在**组盒子内部**，所以它与相邻 section 的 22u 页沟（`pageColumn`，P7-gs）不变。
>
> **验收**：`internal/settingsui/materialspacing_test.go` 两条——
> `TestTheMaterialSectionKeepsItsOwnSpacing`（用盒子供两张卡，逐条读组内四沟 heading→卡、卡→卡、
> 卡→脚注、脚注→脚注 == 10u，组上/下内边距 == 10u；同法构建的普通 section 仍是 8u 且无内边距）与
> `TestTheLiveMaterialSectionUsesItsOwnSpacing`（General 页上的材质组同法测量）。间距从
> `c.Theme()`（帧自己的主题，Space(2.5)=10u）读，不用旁边另建的主题（后者 Space(2.5)=9）。变异核查：
> 把 `sectionMaterial` 改回 `Space(2)`/无内边距 → 两条共 8 处断言红（实测 8 ≠ 10、0 ≠ 10）。
>
> **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 全绿（29 包）、
> `GOOS=linux/windows/darwin go build ./...` 三平台通过。
>
> **清单状态**：「具体待办（P7-gr 重写）」3 条全部落地、清单归零。下一轮按规则先换一个尚未单独
> 审计的原版入口，把「尚未做」重新写实，再取第 1 条落地。

> **本轮：P7-gs General 页 section 之间的沟回到原版 `.settings-page` 的 22u，并把五个页共用的页列
> 提成 `pageColumn`（原版 `src/styles/settings.css:379-387`）**
> ——P7-gr 收尾后「具体待办（P7-gr 重写）」第 1 条已落地，本轮取最高优先级第 2 条落地。依赖检查：
> `go.mod` 已是 **v0.5.1** = latest tag（`go list -m -versions` 复核，无 v0.5.2+；`releases/latest`
> 也复核为 v0.5.1），无版本可升；只对照 `main`（`backup/main-pre-mygo` = v0.3.14 原版树），未翻历史
> 提交。
>
> **偏离（P7-gs·General 页 section 沟是 16u，其它页是 22u）**：原版 `.settings-page` 是页头与每个
> section 共用的网格容器，`gap: calc(var(--u) * 22)`（`settings.css:379-387`），所以相邻 section 之间
> 也是 22u。**本构建（改前）**：`internal/settingsui/settingsui.go:2715` 的 `general` 顶层列用
> `Gap(c.Theme().Space(4))` = 16u，而 `sessions`/`integrations`/`shortcuts`/`about` 四处都用
> `Space(5.5)` = 22u——General 是唯一例外，这个沟的定义散在五个调用点里，才会单独漂移。
>
> **落地**：
> - `internal/settingsui/rows.go`：新增 `pageColumn(c, fn)`——五个页共用的页列，`FillWidth()` +
>   `Gap(c.Theme().Space(5.5))`（22u），这个沟从五处收敛成一处定义。
> - `internal/settingsui/settingsui.go`：`general`/`sessions`/`integrations`/`shortcuts`/`about`
>   五个页的顶层列改走 `a.pageColumn(c, func(){…})`（`shortcuts` 因此不再需要自己的 `t`）。
> - `internal/settingsui/rows.go`：`sectionFoot`/`sectionInline`/`sectionWithControls` 的 8u section
>   列提成共享的 `sectionGroup`，并给每个 section 的组盒子挂上 `sectionLabel(title)`（= 标题 +
>   `\u00a0section`，既是组无障碍名，也让测试能取到组自己的盒子）。
> - `internal/settingsui/view_test.go`：新增 `TestTheGeneralPageKeepsTheReferenceSectionGap`——
>   General 页相邻 section 的沟（前一组盒底 → 后一组盒顶）逐条 == `Space(5.5)`（22u）；同一沟在
>   Shortcuts 页也逐条 == 22u，两页永远一致。**不用文字盒**：标题的行盒与控件的居中都往沟里掺余量。
>
> **验收**：`TestTheGeneralPageKeepsTheReferenceSectionGap`。变异核查：把 `pageColumn` 的
> `Space(5.5)` 改回 `Space(4)` → General 3 条 + Shortcuts 2 条断言同时红（实测 16.0 ≠ 22.0）。
>
> **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 全绿（29 包）、
> `GOOS=linux/windows/darwin go build ./...` 三平台通过。
>
> **清单状态**：「具体待办（P7-gr 重写）」第 2 条本轮落地（并顺手把页列/section 列收敛为共享助手）；
> 只剩第 3 条（材质 section 自己的 10u 间距与 10u 内边距），是下一轮最高优先级。

> **本轮：P7-gr General 页的 section 顺序回到原版的「材质最后」——Appearance → 窗口行为 → 菜单栏 →
> 终端外观 → 玻璃（原版 `src/settings/GeneralPage.tsx:322/405/482/533/558`）**
> ——P7-gq 收尾后「具体待办（P7-go 重写）」3 条全部结清、清单归零，本轮按规则回到「对照原版排查
> 偏离点」，换一个**全文档从未单独审计**的原版入口：General 设置页的**顶层 section 顺序**（此前只
> 对照过单卡内的行序 P7-ca、分组归类 P7-by/P7-bz，页面级的 section 先后从未对照）。依赖检查：
> `go.mod` 已是 **v0.5.1** = latest tag（`go list -m -versions` 复核，无 v0.5.2+；`releases/latest`
> 仍被 API 限流），无版本可升；只对照 `main`（`backup/main-pre-mygo` = v0.3.14 原版树），未翻历史
> 提交。
>
> **偏离（P7-gr·材质区被画在 Appearance 之后）**：原版 General 页的五个 section 按
> Appearance(`:322`) → Window behaviour(`:405`) → Menu bar(`:482`) → Terminal appearance(`:533`) →
> Glass/材质(`:558`) 依次渲染，`.settings-page` 是它们的网格容器（`settings.css:379-387`）。
> **本构建（改前）**：`internal/settingsui/settingsui.go:2712` 的 `general` 把材质 sectionFoot 画在
> Appearance 之后、Window behaviour 之前——玻璃与透明度控件夹在「界面长什么样」和「窗口怎么行为」
> 之间。
>
> **落地**：
> - `internal/settingsui/settingsui.go`：`general` 的材质 `sectionFoot`（玻璃强度卡 + 透明度卡 + 两句
>   脚注）从 Appearance 之后移到 GroupTerminal 之后，成为页面最后一个 section；注释写明原版对照
>   位置（`GeneralPage.tsx:558`，在终端外观 `:533` 之后）。
> - `internal/settingsui/view_test.go`：新增 `TestTheGeneralSectionsKeepTheReferenceOrder`——五个
>   section 标题自上而下正是原版顺序；材质区在每个其它 section 之下；玻璃控件与两句脚注在材质标题
>   之下且脚注收尾。
> - `internal/settingsui/view_test.go` / `scrolltest_test.go`：两处既有测试把「页面最后一块」当成
>   终端配色预览、或用一个不够高的窗口去点材质控件，那是「材质在前」旧布局的假设；现在最后一块是
>   材质的透明度脚注、材质控件在折线之下，测试改用新末块与足够高的窗口（断言语义不变）。
>
> **验收**：`TestTheGeneralSectionsKeepTheReferenceOrder`。变异核查：把材质 sectionFoot 移回
> Appearance 之后 → 四条断言同时红（`Glass ... is not below Terminal appearance`，三个 section
> 「below the material section」）。
>
> **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 全绿（29 包）、
> `GOOS=linux/windows/darwin go build ./...` 三平台通过。
>
> **清单状态**：新写的「具体待办（P7-gr 重写）」3 条，第 1 条本轮落地；第 2 条（General 页 section
> 之间的 22u 沟，本构建是 16u）与第 3 条（材质 section 自己的 10u 间距/内边距）是下一轮最高优先级。

> **本轮：P7-gq Sessions 排序键补回原版的 `ArrowUpDown` 记号；标题行的动作槽放宽到「多个控件」
> （原版 `src/settings/SessionsPage.tsx:116-135`、`src/styles/settings.css:1406-1436`）**
> ——P7-gp 收尾后「具体待办（P7-go 重写）」只剩第 3 条（Sessions 标题行的排序/刷新控件），本轮取它
> 落地。依赖检查：`go.mod` 已是 **v0.5.1** = latest tag（`go list -m -versions` 复核，无 v0.5.2+；
> `releases/latest` 也是 v0.5.1），无版本可升；只对照 `main`（`backup/main-pre-mygo` = v0.3.14 原版
> 树），未翻历史提交。
>
> **偏离（P7-gq·排序键是纯文字动作，缺原版的 `ArrowUpDown` 记号）**：原版
> `SessionsPage.tsx:116-135` 的 heading 里，排序键 `session-manager__sort-toggle` 是
> `<ArrowUpDown size={13} strokeWidth={1.8}/>` + 排序词（`:123-125`），`.session-manager__sort-toggle`
> 的度量与 `.settings-reset` 同（`settings.css:1406-1436`：5u gap、24u 下限、2u 内边距、`--radius-xs`、
> accent/620）。**本构建（改前）**：`settingsui.go:1178-1180` 的 `sectionWithAction` 只画一个纯文字动作
> （排序词），没有记号——控制说了列表**正**怎么排，却没说自己**能**怎么排。
>
> **落地**：
> - `internal/launcher/glyphs.go`：新增 `glyphArrowUpDown`（lucide 的 ArrowUpDown，24×24 描边）并注册
>   `"arrow-up-down"`——记号必须进 app 的封闭字形表，否则 `mustGlyph` 会静默回退成 alert 圆。
> - `internal/settingsui/rows.go`：新增 `sectionWithControls`（标题行的动作槽接受一段画多个控件的
>   closure，共享底对齐与 12u 沟）；`sectionWithAction` 改为委托它（签名不变，其余调用方不受影响）。
> - `internal/settingsui/settingsui.go:1181`：Sessions 段由 `sectionWithAction` 改走
>   `sectionWithControls` + `actionWithGlyph(c, "arrow-up-down", order, ...)`——13px 记号 + 词，度量正是
>   `actionWithGlyph` 已有的 `.settings-reset` 盒（5u gap / 24u 下限 / 2u 内边距），与原版
>   `session-manager__sort-toggle` 逐条相同。
> - `internal/settingsui/sessionssort_test.go`（新）：活页 Sessions——排序词左侧 5u 沟里是 accent 记号
>   （不是纯平面）、`arrow-up-down` 在字形表里、按下后翻到另一个词。
>
> **取舍（如实记录）**：原版 heading 的第二个控件（刷新图标键 `session-manager__icon-button`，
> `RefreshCw` 14px，`disabled={loading}`）**本构建不画**。原版的会话由 broker 托管、靠 5 秒轮询 +
> 手动刷新取回，所以刷新键有实义；本构建的会话在**本进程内**，列表由 `a.Sessions()`（shell 把
> `a.Terminal.Sessions` 逐帧拉平，`shell.go:883-902`）**当帧**取得，没有可重跑的取回——这已在 P7-aj
> 记成「轮询不做」。画一个永远可点却什么都不做、或永远 `disabled`（暗示列表一直在 loading）的键，
> 比不画更骗人。因此本条只补排序键的记号，刷新键按已记录的偏离保留缺席。
>
> **验收**：`TestTheSessionsSortControlLeadsWithTheReferenceMark`。变异核查：把 `settingsui.go` 的
> `actionWithGlyph(c, "arrow-up-down", order, ...)` 换回 `action(c, order, ...)` → 沟里 min channel
> 254（纯平面），断言红。
>
> **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 全绿（29 包）、
> `GOOS=linux/windows/darwin go build ./...` 三平台通过。
>
> **清单状态**：「具体待办（P7-go 重写）」第 3 条落地（排序记号部分；刷新键部分按已记录的偏离结清），
> **3 条全部结清、清单归零**。下一轮回到「对照原版排查偏离点」，换一个从未进入任何历史待办的原版入口。

> **本轮：P7-gp 深链标题的说明回到原版 `heading-main` 的 `--inline` 第二行——说明在卡片之前、
> 与标题同列、间距 1px（原版 `src/settings/DeepLinkRow.tsx:61-64`、`src/styles/settings.css:430-434,
> 1209-1211`）**
> ——P7-go 收尾后「具体待办（P7-go 重写）」还剩第 2·3 条，本轮取最高优先级第 2 条落地。依赖检查：
> `go.mod` 已是 **v0.5.1** = latest tag（`go list -m -versions` 复核，无 v0.5.2+；`releases/latest`
> 仍被 API 限流），无版本可升；只对照 `main`（`backup/main-pre-mygo` = v0.3.14 原版树），未翻历史提交。
>
> **偏离（P7-gp·深链说明被画成卡片之后的普通分组说明）**：原版 `DeepLinkRow.tsx:61-64` 的
> `heading-main` 是 `display:grid; gap:1px`（`settings.css:430-434`）的 label +
> `settings-section__hint settings-section__hint--inline` 两行，`--inline { padding: 0 }`
> （`:1209-1211`）——这是原版**唯一**一条骑在标题块内的说明，因此它画在**卡片之前**、与标题同列、
> 间距 1px。**本构建（改前）**：P7-go 把标题说明统一改成卡片之后的 `sectionHint` 行，深链这条也
> 被一并挪到了卡片后面，与「标题块内第二行」的原版位置不符。
>
> **落地**：
> - `internal/settingsui/rows.go`：`sectionHeading`（`:73`）改为委托新的 `sectionHeadingInline`
>   （`:83`）——`heading-main` 用 `ui.Column(...).Gap(1)` 复刻 1px 栈，inline hint 走同列的
>   muted/body/500 文本；新增 `sectionInline`（`:56`）——卡片之前带 inline 说明的分组，卡片与 2u
>   分节间距与 `section` 一致。
> - `internal/settingsui/settingsui.go:2606`：深链段由 `a.section` 改走 `a.sectionInline`。
> - `internal/settingsui/sectionheading_test.go`：新增 `TestTheDeepLinkHintRidesInsideItsHeading`
>   （说明在卡片之前、与标题同列、间距 == 1px）。
>
> **验收**：`TestTheDeepLinkHintRidesInsideItsHeading`。变异核查：把 `settingsui.go:2606` 改回
> `a.section` → 说明跑到卡片之后（hint Y=412 > value Y=329），且间距变 107，两条断言同时红。
> 既有 `sectionHint` 调用方不变，由 `TestTheGroupHintRidesAfterItsCards`、
> `TestTheLiveShortcutsHintIsNotInTheHeading` 继续覆盖。
>
> **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 全绿（29 包）、
> `GOOS=linux/windows/darwin go build ./...` 三平台通过。
>
> **旁注（如实记录）**：工作区里发现一份**未提交、且测试失败**的组件化卸载 WIP（`uninstall.go` +
> `uninstall_test.go` 的 gap/16px margin 改动，其测试用文本盒测行间距、断言恒不可能成立）。它不在
> 任何待办清单里，本轮不擅自吞入绿门槛提交；已把补丁留在 `/tmp/uninstall-wip.patch` 并回退这两份
> 文件，保持本轮提交聚焦且全绿。
>
> **清单状态**：「具体待办（P7-go 重写）」第 2 条本轮落地；只剩第 3 条（Sessions 标题行的排序/
> 刷新控件）是下一轮最高优先级。

> **本轮：P7-go 分组标题行回到原版的 `.settings-section__heading`——标题 inset 2u、动作底对齐、
> 分组说明改回卡片之后的 `__hint` 行（原版 `src/styles/settings.css:411-448,1198-1206`、
> `src/settings/ShortcutsPage.tsx:383-424`、`src/settings/SettingsRows.tsx:200-216`）**
> ——P7-gn 收尾后「具体待办」归零，本轮按规则回到「对照原版排查偏离点」，换一个**全文档从未单独
> 审计**的入口：设置页的分组标题行原语（此前只对照过行、文本动作与空态，标题行本身没有 file:line
> 对照）。依赖检查：`go.mod` 已是 **v0.5.1** = latest tag（`releases/latest` 仍被 API 限流），无
> 版本可升；只对照 `main`（`backup/main-pre-mygo` = v0.3.14 原版树），未翻历史提交。
>
> **偏离（P7-go·标题与卡片齐平、说明挤在标题下、动作浮在块中间）**：原版 `.settings-section` 是
> 8u 间距的网格（`settings.css:411-419`），第一格 `.settings-section__heading` 是 flex 行：
> `align-items: flex-end`、`justify-content: space-between`、`gap: 12u`、`padding: 0 2u`
> （`:421-429`），标题在 `heading-main` 块里（`:430-434`），分组说明是**卡片之后**的
> `.settings-section__hint` 行（`:1198-1206`，`padding: 0 2u`；`ShortcutsPage.tsx:424`、
> `GeneralPage.tsx:597-598`）。**本构建（改前）**：`section`/`sectionFoot` 把标题画在内容最左边
> （无 2u 内缩），说明紧贴标题下方（8u），`sectionWithAction` 的标题行用 `AlignItems(ui.Center)`
> 让动作浮在块中间。
>
> **落地**：
> - `internal/settingsui/rows.go`：新增共享 `sectionHeading(c, title, action)`（`:61`）——`ui.Row`
>   + `AlignItems(ui.End)` + `Justify(ui.SpaceBetween)` + `Gap(Space(3))` + `Padding(0, Space(0.5))`，
>   返回行元素供测试读盒子；`sectionFoot`（`:40`）与 `sectionWithAction`（`:363`）都改走它。新增
>   `sectionHint(c, hint)`（`:83`）——卡片之后的 2u 内缩说明行；`section`/`sectionWithAction` 的
>   `hint` 与 `sectionFoot` 的 `footnotes` 都走它。
> - `internal/settingsui/sectionheading_test.go`（新）：三条——孤立标题行（2u 内缩、短控件时动作
>   底对齐、右端）；`section` 的说明在卡片之后且 2u 内缩；活页 Shortcuts 的说明不在标题行内。
> - `internal/settingsui/actionmetrics_test.go`：`TestSectionHeadingActionSitsAtTheRightEndBottomAligned`
>   的块底改读标题底（标题行现在只有标题）。
>
> **取舍（如实记录）**：`heading-main` 的 `min-width: 0` 用 `ui.Column(c).Grow(1)` 复刻，但 mygo
> 的 flex 子项不会自动换行，超长分组标题仍会溢出（原版会换行）——留作后续；原版
> `.settings-section__label` 的 `letter-spacing: -0.004em` / `line-height: 1.25` 未接（mygo 有
> `LetterSpacing`/`LineHeight` 两个旋钮，只是本构建还没用）。
>
> **验收**：`internal/settingsui/sectionheading_test.go` 三条 + `actionmetrics_test.go` 的
> `TestSectionHeadingActionSitsAtTheRightEndBottomAligned`。变异核查：`AlignItems(ui.End)` 换
> `ui.Center` → 动作底对齐条红（17 vs 24）；说明挪回卡片前 → 「在卡片之后」条红；去掉说明的 2u
> 内缩 → 内缩条红（0 vs 1.8）。
>
> **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 全绿、
> `GOOS=linux/windows/darwin go build ./...` 三平台通过。
>
> **清单状态**：新写的「具体待办（P7-go 重写）」3 条，第 1 条本轮落地；第 2 条（深链标题的
> `--inline` 说明）与第 3 条（Sessions 标题的排序/刷新控件）是下一轮候选。

> **本轮：P7-gn 自定义快捷键空态回到原版共享的 `SettingsEmpty`——22px Keyboard 记号 +
> `--text-secondary` 标题 + `＋` 文本动作出路（原版 `src/settings/ShortcutsPage.tsx:240-252`、
> `src/settings/SettingsRows.tsx:201-244`、`src/styles/settings.css:633-676,682-730`、
> `src/i18n.ts:403-405,1290-1292`）**
> ——P7-gm 收尾后「具体待办（P7-gk 重写）」只剩第 3 条，本轮取它落地，清单归零。依赖检查：
> `go.mod` 已是 **v0.5.1** = latest tag（`go list -m -versions` 复核；`releases/latest` 仍被 API
> 限流），无版本可升；只对照 `main`（`backup/main-pre-mygo` = v0.3.14 原版树），未翻历史提交。
>
> **偏离（P7-gn·自定义快捷键空态是填充按钮 + 自造度量，不是原版的共享空态）**：原版
> `ShortcutsPage.tsx:240-252` 的空态走共享 `SettingsEmpty`——`<Keyboard size={22} strokeWidth={1.6}/>`
> 记号、title `customShortcutsEmpty`、hint `customShortcutsEmptyHint`、action 是带 `＋` 记号的
> `.settings-reset` 文本动作（`customShortcutsAddFirst`）；基类给 92u 底、24u padding、8u gap、
> 记号 `--text-tertiary` @0.72、title `--text-secondary` 580、hint `--text-caption`。
> **本构建（改前）**：`custom.go:35-45` 是 `Gap(Space(2)).Padding(Space(4)).Center()`，记号是
> `mustGlyph("text")` @20u、title @`TextMuted`、hint @`FontSize-1`，出路是 `ui.Button`
> （填充按钮、无 ＋ 记号）；title 词尾还多一个句号。
>
> **落地**：
> - `internal/settingsui/custom.go` 的空支改走共享 `emptyState(c, "keyboard", copy.ShortcutsNone,
>   copy.ShortcutsEmptyHint, 0, 0, action)`，出路是
>   `a.actionWithGlyph(c, "plus", copy.ShortcutsAddFirst, …)`（24u 文本动作，非填充按钮）。
> - `internal/launcher/glyphs.go`：新增 `keyboard`（lucide-react 1.31.0 的 Keyboard）与 `plus`
>   （lucide Plus）两个记号并进闭集。
> - `internal/i18n`：`ShortcutsNone` 的 en/zh 去掉旧端口多出来的句号，逐字回到原版
>   `customShortcutsEmpty`（`i18n.ts:403,1290`）。
>
> **取舍（如实记录）**：原版该组标题行还有一个带 `＋` 的 "Add" 动作（`ShortcutsPage.tsx:229-236`），
> 本构建的 `customSection` 用 `a.section`（label+hint）没有标题行动作；本条待办只针对空态，标题行
> 动作不在本轮范围（若要做，是下一处对照点）。原版空态的 `role` 无 alert 支，无需处理。
>
> **验收**：`internal/settingsui/customempty_test.go` 两条——
> `TestTheCustomShortcutsEmptyStateIsTheReferenceEmptyRegion`（活页：原版词在、旧句号句消失、标题
> 上方 22u 带内有 Keyboard 记号墨迹、标题墨色 = `--text-secondary` 合成值且不等于旧 `TextMuted`、
> 出路左侧 20u 带内有 accent `＋` 记号且其平面 == 卡片底（非填充 pill）、点击出路开录制器）、
> `TestTheCustomShortcutsEmptyRegionHasTheListFloor`（直接渲染共享空态：高 ≥92u、Keyboard 记号墨迹
> 高 ≈22u）；`internal/i18n/i18n_test.go` 的 `TestCustomShortcutsEmptyWordsMatchTheOldDictionary`
> （en/zh 三条逐字）。变异核查：出路改回 `ui.Button` → 记号/平面两条断言红；空态记号名改 `""` →
> 记号条红。
>
> **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 全绿（1405 项）、
> `GOOS=linux/windows/darwin go build ./...` 三平台通过。
>
> **清单状态**：第 3 条（P7-gn）落地；「具体待办（P7-gk 重写）」**3 条全部结清、清单归零**。
> 下一轮回到「对照原版排查偏离点」，换一个从未进入任何历史待办的原版入口。

> **上一轮：P7-gm Integrations 页的空态回到原版共享的 `SettingsEmpty`——`Package` 20px 记号 +
> `--text-secondary` 标题 + 92u 列表底（原版 `src/ExtensionsPanel.tsx:2462,3177-3191`、
> `src/settings/SettingsRows.tsx:218-244`、`src/styles/settings.css:633-676`、`src/i18n.ts:443,1330`）**
> ——P7-gl 收尾后「具体待办（P7-gk 重写）」还剩第 2·3 条，本轮取最高优先级第 2 条落地。依赖检查：
> `go.mod` 已是 **v0.5.1** = latest tag（`go list -m -versions` 复核，`releases/latest` 仍被 API
> 限流），无版本可升；只对照 `main`（`backup/main-pre-mygo` = v0.3.14 原版树），未翻历史提交。
>
> **偏离（P7-gm·Integrations 空态是一行裸文本，不是原版的共享空态）**：原版
> `ExtensionsPanel.tsx:3177-3191` 的 `EmptyState` 一律走共享的 `SettingsEmpty`；安装列表空的那一支
> （`:2462`）画 `<Package size={20} strokeWidth={2}/>` + `settings.extensions.emptyInstalled`
> （en「No integrations available」/ zh「暂无可用集成」），走基类 `.settings-empty`
> （`settings.css:633-676`）的 92u 列表底、24u padding、8u gap、标题 `--text-secondary` 580。
> **本构建（改前）**：`settingsui.go` 的空态是 `Padding(Space(4)).Center()` 里一行
> `copy.IntegrationsEmpty`（「No integrations are installed.」）用 `t.TextMuted`，没有记号、没有
> 92u 底，词条也与原版 `emptyInstalled` 不同。
>
> **落地**：
> - `internal/settingsui/empty.go`：共享 `emptyState` 增加 `iconSize` 参数（原版 `SettingsEmpty` 的
>   icon 是调用方给的节点，Sessions/快捷键给 22px、集成列表给 20px；0 → 22u）。
> - `internal/settingsui/settingsui.go` 的 `integrations` 空支改走
>   `a.emptyState(c, "package", copy.IntegrationsEmpty, "", t.Space(5), 0, nil)`（20u 记号、92u 底）。
> - `internal/launcher/glyphs.go`：新增 `package`（lucide-react 1.31.0 的 Package 路径，逐字取自
>   该版本 tarball）并进闭集。
> - `internal/i18n`：`IntegrationsEmpty` 的 en/zh 改成原版 `settings.extensions.emptyInstalled`
>   的逐字文案（`i18n.ts:443,1330`）。
>
> **取舍（如实记录）**：原版 `EmptyState` 还有两支——首帧加载的 `LoaderCircle` + `loading` 词、以及
> 过滤空态在 hint 行回显查询串。本构建的安装列表来自进程内同步读（`installedIntegrations()`），
> 没有加载帧；集成页也没有过滤输入（原版 v0.3.14 的 `EmptyState` 的 `query` 参数本就无人传），
> 所以两支都没有来源，只画 resting 一支、不画 hint。原版 error 支的 `role="alert"` 同样无来源。
>
> **验收**：`internal/settingsui/integrationsempty_test.go` 两条——
> `TestTheIntegrationsEmptyStateIsTheReferenceEmptyRegion`（活页：画出原版词、旧自造句消失、标题上方
> 20u 带内有 Package 记号墨迹、标题墨色 = `--text-secondary` 合成值且不等于旧的 `TextMuted` 合成值）、
> `TestTheIntegrationsEmptyRegionHasTheListFloor`（直接渲染共享空态盒：高 == 92u、记号墨迹高 ≈20u）。
> 变异核查：把空支改回旧的裸 `ui.Text` → 记号条红；把标题色改成 `TextMuted` → 两条墨色断言红
> （ink {114,114,123} vs want {87,87,93}）。
>
> **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 全绿（1402 项）、
> `GOOS=linux/windows/darwin go build ./...` 三平台通过。
>
> **清单状态**：第 2 条（P7-gm）落地；「具体待办（P7-gk 重写）」还剩第 3 条（自定义快捷键空态同改 +
> keyboard/＋ 记号），是下一轮最高优先级。

> **上一轮：P7-gl Sessions 页的空态回到原版共享的 `SettingsEmpty`——三态映射 + 22u 图标记号 +
> 8u 标题/提示间距 + 160u 页体底（原版 `src/settings/SessionsPage.tsx:142-167`、
> `src/settings/SettingsRows.tsx:218-244`、`src/styles/settings.css:633-676,1393`、
> `src/i18n.ts:162-166,1069-1073`）**
> ——P7-gk 收尾后「具体待办（P7-gk 重写）」有 3 条待做，本轮取最高优先级第 1 条落地。依赖检查：
> `go.mod` 已是 **v0.5.1** = GitHub releases latest（本轮以 `go list -m -versions` 复核，v0.5.1 即
> latest tag；`releases/latest` 仍被 API 限流），无版本可升；只对照 `main`
> （`backup/main-pre-mygo` = v0.3.14 原版树），未翻历史提交。
>
> **偏离（P7-gl·Sessions 空态是一行裸文本，不是原版的共享空态）**：原版
> `SessionsPage.tsx:142-167` 在 `sessions.length === 0` 时走共享的 `SettingsEmpty`
> （`SettingsRows.tsx:218-244`）——`error ? AlertCircle : loading ? RefreshCw : SquareTerminal`
> （均 22px、`strokeWidth 1.6`）三态图标，标题取 `terminal.sessionsError` /
> `terminal.sessionsLoading` / `terminal.sessionsEmpty`，提示取 `terminal.sessionsRefreshHint` /
> 无 / `terminal.sessionsEmptyHint`；CSS `.settings-empty`（`settings.css:633-676`）给 92u 底、
> 24u padding、8u gap、图标 `--text-tertiary` @0.72、标题 `--text-secondary` 580、提示
> `--text-caption` 320u；`.session-manager > .settings-empty`（`:1393`）把这条列表的底抬到 160u
> （空态*就是*整页 body）。**本构建（改前）**：`settingsui.go` 的空态是 `Gap(Space(3))` +
> `Padding(Space(4))` 的一行 `copy.SessionsNone`（“No sessions are running.”）+ 一个
> `ui.Button(SessionsNew)`，没有图标、没有提示、没有 160u 底，且词条与 `terminal.sessionsEmpty`
> （“No persistent sessions”/“暂无持久化会话”）不同。
>
> **落地**：
> - `internal/settingsui/empty.go`（新）：共享 `emptyState`——`MinHeight(floor)`（0 → 92u）、
>   `Gap(t.Space(2))`、`Padding(t.Space(6))`、`Center()`；图标记号 22u（`t.Space(5.5)`）
>   `TextTertiary` @0.72；标题 `tokens.Body` 580 `TextSecondary`；提示 `tokens.Caption`
>   `TextTertiary` `MaxWidth(t.Space(80))`；可选 action 槽（`Margin(t.Space(0.5),0,0,0)`）。
>   同文件 `sessionsEmptyFaceFor(copy, failed, loading)` 是原版三态选择的纯函数。
> - `internal/settingsui/settingsui.go` 的 `sessions` 空支改走 `a.emptyState(..., t.Space(40), …)`
>   （160u），页面 own 的 `New session` 控制留在空态里做出路。
> - `internal/launcher/glyphs.go`：新增 `square-terminal`（lucide SquareTerminal）与 `refresh`
>   （lucide RefreshCw）两个记号并进闭集。
> - `internal/i18n`：删掉自造的 `SessionsNone`，补原版五条 `SessionsLoading`/`SessionsEmpty`/
>   `SessionsEmptyHint`/`SessionsError`/`SessionsRefreshHint`（en/zh 逐字取
>   `i18n.ts:162-166,1069-1073`）。
>
> **取舍（如实记录）**：本构建的会话列表来自进程内的 `a.Terminal.Sessions`（同步读），没有原版
> broker 轮询带来的 loading/error 帧；三态映射与词条/记号都按原版落地并经测试钉住，但页面只会走
> 默认支。原版 error 的 `role="alert"` 仍无 `ui.Role` 枚举（既有「已知差距」），本轮不接线。
>
> **验收**：`internal/settingsui/empty_test.go` 三条——
> `TestTheEmptyRegionWearsTheReferenceSettingsEmptyMetrics`（绑定空态盒：高 == 160u、title/hint
> 间距 == 8u、标题上方 22u 带内有图标记号且墨迹高度 ≈22u）、
> `TestTheSessionsEmptyStateIsTheReferenceEmptyRegion`（活页：画出 `SessionsEmpty`/`SessionsEmptyHint`/
> `SessionsNew`、不再出现旧句、点 `New session` 恰好触发一次）、
> `TestSessionsEmptyFaceFollowsTheReferenceThreeWay`（resting/loading/failed 三支 + 失败优先 + zh）；
> `internal/i18n/i18n_test.go` 的 `TestSessionsEmptyWordsMatchTheOldDictionary`（en/zh 五条逐字）。
> 变异核查：去掉 `MinHeight` → 盒高 115 红；把记号名查成 `""` → 「no glyph ink」红。
>
> **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go tool mygo vet .` 净、
> `go test -count=1 ./...` 全绿（1400 项）、`GOOS=linux/windows/darwin go build ./...` 三平台通过。
>
> **清单状态**：第 1 条（P7-gl）落地；「具体待办（P7-gk 重写）」还剩第 2 条（Integrations 页空态
> 改用同一 `emptyState`）与第 3 条（自定义快捷键空态同改 + keyboard/＋ 记号），是下一轮最高优先级。

> **上一轮：P7-gk 共享文本动作回到原版 `.settings-reset` 的度量与交互——24u 底、2u 内边距、
> `--radius-xs`，hover/active 只降墨色透明度（原版 `src/settings/SettingsRows.tsx:201-216`、
> `src/styles/settings.css:682-730`）**
> ——P7-gj 收尾后「具体待办（P7-gi 重写）」只剩第 3 条（共享文本动的 `.settings-reset` 度量），
> 本轮取它落地。依赖检查：`go.mod` 已是 **v0.5.1** = GitHub releases latest（本轮以
> `go list -m -versions` 复核：v0.5.1 即 latest tag；`releases/latest` 因 API 限流不可用），
> 无版本可升；只对照 `main`（`backup/main-pre-mygo` = v0.3.14 原版树），未翻历史提交。
>
> **偏离 1（P7-gk·文本动作比原版高 6u、宽 4u，圆角错档）**：原版 `SettingsAction` 就是
> `.settings-reset` 本身（`SettingsRows.tsx:201-216`）：`settings.css:682-730` 给它
> `min-height: 24u`、`padding: 0 2u`、`gap: 5u`、`border-radius: var(--radius-xs)`。**本构建（改前）**
> `rows.go` 的 `actionWithGlyph` 用 `Padding(t.Space(1), t.Space(1.5))` = 4u/6u、`RadiusSM` = 9px，
> 于是同一个动作的盒子高 30u、左侧内缩 6u（实测），而原版是 24u/2u。
>
> **偏离 2（P7-gk·hover 画了一条底色带，原版只降墨色）**：原版 `.settings-reset:not(:disabled):hover
> { opacity: .78 }`、`:active { opacity: .6 }`（`settings.css:713-721`），也就是交互只改**墨色**、
> 不动底下的平面（`background: transparent`）。**本构建（改前）**：hover 时 `row.Background(t.SurfaceHover)`
> 把动作后面的平面点亮（实测 delta 21），既不是原版的样子，也让「文本动作」看成了填充控件。
>
> **落地**：`internal/settingsui/rows.go` 的 `actionWithGlyph`——`MinHeight(t.Space(6))`（=24u）、
> `Padding(0, t.Space(0.5))`（=0 2u）、`Radius(tokens.RadiusXS)`（6px）、`AlignItems(ui.Center)`；
> hover/active 改成 `row.Opacity(0.78)` / `row.Opacity(0.6)` 并与原版同序（pressed 优先，`switch`）；
> `action`/`actionWithGlyph` 改为返回 `ui.Element`，供测试用 `Handle.Bind`+`Resolve` 直接读盒子。
> 两个调用方（`sectionWithAction` 的 `Restore defaults`、深链复制）自动回到原版度量。
>
> **验收**：`internal/settingsui/actionmetrics_test.go` 两条——
> `TestTheTextActionsWearTheReferenceResetMetrics`（单渲染动作：左侧内缩 2u、右侧 2u、盒高
> 23.5≤h≤25，与旧的 6u/30u 盒子两侧都被夹住）与 `TestTheTextActionHoverDimsTheInkOnly`（在活的
> Shortcuts 组上驱动 `Restore defaults`：hover 后墨色最小通道上升，而文字旁的平面像素差 ≤2，
> 即没有新增底色）。变异核查：把度量改回 `Padding(4u,6u)`+`RadiusSM`+`SurfaceHover` → 两条当场红
> （inset 6/6、高 30；平面 delta 21、墨色不变）。
>
> **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go tool mygo vet .` 净、
> `go test -count=1 ./...` 全绿（1396 项）、`GOOS=linux/windows/darwin go build ./...` 三平台通过。
>
> **清单状态**：第 3 条（P7-gk）落地；「具体待办（P7-gi 重写）」**3 条全部结清、清单归零**。
> 下一轮回到「对照原版排查偏离点」，换一个**全文档从未审计**的入口（原版 `SettingsAction` 的兄弟
> 原语 `SettingsEmpty` 只在 P7-gc 的 Shortcuts 空态用过），把「尚未做」重写为带 file:line 的具体
> 待办并取第 1 条落地。

> **上一轮：P7-gj 深链值回到原版的代码块 chip——从行标签改画 `.settings-deep-link__value`
> （原版 `src/settings/DeepLinkRow.tsx:67,79`、`src/styles/settings.css:1524-1535`）**
> ——P7-gi 收尾后「具体待办（P7-gi 重写）」还剩第 2·3 条，本轮取最高优先级的第 2 条落地。
> 依赖检查：`go.mod` 已是 **v0.5.1** = GitHub releases latest（本轮 `releases/latest` 复核），
> 无版本可升；只对照 `main`（`backup/main-pre-mygo` = v0.3.14 原版树），未翻历史提交。
>
> **偏离（P7-gj·深链值是普通行标签，不是代码块 chip）**：原版 `DeepLinkRow.tsx:67,79` 的行 label 是
> `<code className="settings-deep-link__value">`；`settings.css:1524-1535` 给它 `display:block`、
> `padding: 4u 8u`、`--radius-xs`、`background: var(--surface-sunken)`、
> `border: 1px solid var(--glass-control-edge)`、`color: --text-secondary`、等宽字体
> （SF Mono/Menlo/Monaco/Consolas）、`--text-caption`、`overflow-wrap:anywhere`，于是 `floter://` 一行
> 读成「值」而不是「正文」。**本构建（改前）**：`settingsui.go` 的
> `a.row(c, link, "", …)` 把链接当普通行 label 画成 `--text-emphasis` 的 strong 文本
> （`rows.go` 的 `rowAligned`），没有 sunken 底、等宽、caption 字级或 8u/4u 内边距。
>
> **落地**：`internal/settingsui/rows.go` 把 `rowAligned` 的行框抽成共享的 `cardRow`
> （分隔线、`cardRows` 计数、44u 底、标签列、尾随控件槽、`below` 槽都只此一份），`rowAligned` 与新的
> `codeRow` 都走它——`codeRow` 只替换标签列的绘制：`Padding(Space(1), Space(2))` = 4u/8u、
> `Radius(tokens.RadiusXS)` = `--radius-xs`、`Background(tokens.Sunken)` = `--surface-sunken`、
> `Border(1, tokens.ControlEdge)` = `--glass-control-edge`、`FontSize(tokens.Caption)`、
> `TextColor(tokens.TextSecondary)`、`Font(deepLinkValueFont)` = 原版等宽栈；`settingsui.go` 的
> deep-link 行改走 `a.codeRow`。（此前把整行复制了一份 `codeRow`，本轮改为共享行框，避免 chip 与
> 普通行各自漂移。）
>
> **验收**：`internal/settingsui/deeplinkcopy_test.go` 新增两条——
> `TestTheDeepLinkValueIsTheReferenceCodeChip`（暗色下 chip 的 sunken 底与卡片底像素差 ≥12；chip 的
> 内边距在文字上下的填充带正好是 4u，与 `padding:4u 8u` 一致）与
> `TestTheDeepLinkValueUsesTheReferenceCodeFont`（等宽栈逐字：headless 渲染器只用一种字体 shaping，
> 字体族读不回来，故钉住交给 `ui.Text` 的常量）。变异核查：把 `a.codeRow` 换回 `a.row` → chip 底与
> 卡片底差 0，第一条当场红。
>
> **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go tool mygo vet .` 净、
> `go test -count=1 ./...` 全绿（1394 项）、`GOOS=linux/windows/darwin go build ./...` 三平台通过。
>
> **清单状态**：第 2 条（P7-gj）落地；「具体待办（P7-gi 重写）」还剩第 3 条（共享文本动作的
> `.settings-reset` 度量），是下一轮最高优先级。

> **上一轮：P7-gi About 页深链复制控件回到原版的 accent 文本动作 + Copy/Check 图标，并补上成功 toast
> （原版 `src/settings/DeepLinkRow.tsx:31-58`、`src/settings/SettingsRows.tsx:200-216`、
> `src/styles/settings.css:682-730`）**
> ——P7-gh 收尾后「具体待办（P7-gb 重写）」归零。本轮回到「对照原版排查偏离点」，换一个**全文档
> 从未提过**的入口：About 页的 `DeepLinkRow`（`grep -c` = 0）。依赖检查：`go.mod` 已是 **v0.5.1**
> = GitHub releases latest（本轮 `releases/latest` 复核），无版本可升；只对照 `main`
> （`backup/main-pre-mygo` = v0.3.14 原版树），未翻历史提交。
>
> **偏离 1（P7-gi·复制控件是填充按钮、无图标）**：原版 `SettingsAction` = `.settings-reset` 是**无填充
> 的 accent 文本动作**（`settings.css:682-730`：`gap:5u`、`min-height:24u`、`padding:0 2u`、hover
> 只降透明度），键面在 1.6s 内切 lucide 的 `Copy`/`Check`（13px、`strokeWidth=2`）；
> **本构建（改前）**：`settingsui.go:2608` 用 `ui.Button(c, label)`，既无图标也无文本动作形状。
>
> **偏离 2（P7-gi·复制后没有全局成功提示）**：原版 `copy()` 成功写剪贴板后调
> `onCopied(t("settings.deepLinkCopied"))`，让确认同时落进应用自己的 toast 栈；**本构建（改前）**
> `Actions` 只有 `NotifyWarning`，success 通道不存在（「已知差距」里还留着「success 分支有口子、
> 等 raiser」的旧记录）。
>
> **落地**：`internal/launcher/glyphs.go` 新增 `glyphCopy` 与 `Glyph("copy")`（lucide Copy 的两页
> 形状；`clipboard` 是另一个记号）；`internal/settingsui/rows.go` 把 `action` 拆出
> `actionWithGlyph`（可选前导图标、`gap:Space(1.25)` = 5u、`Space(3.25)` = 13u、accent 墨色）；
> `settingsui.go` 的 deep-link 行改走 `actionWithGlyph(linkCopyGlyph(copied), …)`，新增
> `linkCopyGlyph` 与 `deepLinkConnectExample`/`deepLinkRegisterExample` 两个常量；
> `Actions.NotifySuccess` → `shell.notifySuccess`（success 类型、`successNotice` 4s 窗口）→
> `ToastViewportBase` 的既有 accent/check 档。
>
> **验收**：`internal/settingsui/deeplinkcopy_test.go` 三条——
> `TestDeepLinkCopyRaisesTheSuccessToast`（复制值 + `NotifySuccess` 逐字收到 `DeepLinkCopied`）、
> `TestDeepLinkCopyUsesTheReferenceGlyphs`（两态 + 两名字都在闭集里）、
> `TestTheDeepLinkCopyActionWearsItsIcon`（标签前 20u 带内最大通道差 ≥60；填充按钮是 ≤28）；
> `internal/shell/notices_test.go` 的 `TestTheSuccessNoticeCarriesItsKindAndWindow`。变异核查：换回
> `ui.Button` 且不调 `NotifySuccess` → 前两条当场红。
>
> **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 全绿、
> `GOOS=linux/windows/darwin go build ./...` 三平台通过。
>
> **清单状态**：新写的「具体待办（P7-gi 重写）」3 条，第 1 条本轮落地（P7-gi）；第 2 条
> （`.settings-deep-link__value` 代码块 chip）与第 3 条（共享文本动作的 `.settings-reset` 度量）
> 是下一轮候选。

> **上一轮：P7-gh 独立窗口的 body 回到原版的三态运行状态机
> （原版 `src/plugin-window/DetachedPluginApp.tsx:329-350`、词条 `src/i18n.ts:37-39,965-967`）**
> ——P7-gd 收尾后「具体待办（P7-gb 重写）」只剩第 3 条（body 的 running/failed/empty），
> 本轮取它落地，清单归零。依赖检查：`go.mod` 已是 **v0.5.1** = GitHub releases latest
> （`api.github.com` 被限流，改用 `releases/latest` 页面与 `go list -m -versions` 两路核对，最新
> 仍是 v0.5.1，无版本可升）；只对照 `main`（`backup/main-pre-mygo` = v0.3.14 原版树），未翻历史提交。
>
> **偏离（P7-gh·窗口 body 只有「文本 or idle」两个状态）**：原版 body 是一台
> 三态状态机（`DetachedPluginApp.tsx:329-350`）：`status === "running"` →
> `launcher.externalRunning`（“Running…”/“正在执行…”）**且不画出上一次的输出**；
> `status === "failed"` → `runErrorMessage(state.failure, t)` 本地化（认不出才回落
> `failure ?? launcher.externalFailed`）画成 `--text-warning`；`status === "done" && view === null`
> → `launcher.externalEmpty`（“The command produced no output”/“命令没有输出”）；
> `status === "idle" && !request` → `pluginWindow.idle`（“No command pinned yet”）。**本构建
> （改前）**：`internal/shell/detach.go:195-203` 只画 `detached.view.Text`，空则一律
> `OutputWindowIdle`——重跑在飞的整段时间仍显示旧输出（用户看不出「正在执行」），跑空的窗口
> 读成「尚未固定任何命令」，而 `OutputFor` 在 keyed 失败上已把句子塞进 `view.Text`（与退出码
> 非零的 `done` 混为一谈）。
>
> **落地**：
> - `internal/i18n`：新增 `Launcher.OutputWindowRunning`/`OutputWindowFailed`/`OutputWindowEmpty`
>   三条（en/zh 逐字取 `i18n.ts:37-39,965-967`）。
> - `internal/shell/detach.go`：`detachedOutput` 新增 `failed string`（一次未产出答案的运行的句子）
>   与 `ran bool`（是否跑过，用来区分「跑过但空」与「还没跑过」）；`detachedBody` 是新状态机
>   （running → failed → 有文本 → ran → idle），渲染时 failed 走 `tokens.Warning`、其余三态走
>   `tokens.Caption`（复刻 `.plugin-window__status` 与 `--failed` 的 `--text-warning`）；
>   `rerunDetached` 起跑时清 failed 并 `Invalidate()`，`applyDetachedRun` 多收一个 `runErr`，
>   只在 `runErr != nil && view.Text == ""` 时记句子（超时但说了话的运行保留输出，逐字复刻
>   `externalRunText` 的 stdout→stderr 规则），句子取 `Launcher.RunStatus`（与 launcher 状态行
>   同一套本地化）；空句子回落词表的通用句。
>
> **验收**：`internal/shell/detach_test.go` 新增
> `TestTheDetachedBodyFollowsTheRunStateMachine`（running 画 running 词且**不**画旧输出；failed
> 只画失败句；跑空的画 empty 而非 idle；没跑过的画 idle 而非 empty）、
> `TestARerunMarksTheWindowRunningUntilTheAnswerLands`（起跑到答案之间的帧是 running，答案落地后
> running=false/ran=true）、`TestARunThatNeverStartedCarriesItsFailureSentence`（keyed 失败句子含
> 路径、说话的超时保留输出、空错误回落通用句）；`detachwords_test.go` 的词表补三态逐字。
> 变异核查：删 running 分支 → 第一条红（画出旧输出）；不记 failed → 新测试红；看漏 `ran` →
> empty/idle 条红；删空句子回落 → 空错误条红。
>
> **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go tool mygo vet .` 净、
> `go test -count=1 ./...` 全绿（1388 项）、`GOOS=linux/windows/darwin go build ./...` 三平台通过。
>
> **清单状态**：「具体待办（P7-gb 重写）」3 条**全部结清、清单归零**。下一轮回到「对照原版
> 排查偏离点」，重新写实清单。

> **上一轮：P7-gf mygo v0.4.8 → v0.5.1 升级，并修正上一轮引入的状态行内边距回归
> （`go tool mygo migrate-ui` 的 checked-elements 迁移；原版 `src/styles/launcher.css:1099-1114,1480-1491`）**
> ——上一轮把 `go.mod` 升到 v0.5.1 后中断，代码还没适配。本轮把升级做完，并补上上一轮
> （`status note: align side padding`）留下的两处坏账。**升级范围**：
>
> 1. **`go tool mygo migrate-ui -write .`**：v0.5.x 把「元素键」从链式 `.Key(x)` 移到
>    `Context.Key(x)`（键必须在状态初始化前落位），仓库里只有两处：
>    `internal/launcher/launcher.go` 的 `rowBody`（`ui.Row(c).Key(item.ID)` →
>    `ui.Row(c.Key(item.ID))`）与 `internal/launcher/output.go` 的 `outputRow`；codemod 之后
>    `go tool mygo vet .`（框架的类型化生命周期检查）也干净。其余 v0.5 API 对本仓库是源码兼容的。
> 2. **终端 `OnPaste` 换了参数**：v0.4.8 的钩子收 `text string`，v0.5.1 收
>    `transfer.Data`（剪贴板全部格式，见 `plugins/terminal/terminal.go` 的 `OnPaste`）。
>    `internal/terminalui/terminalui.go` 的边界改成读 `transfer.Text` 表示再走 `pasteText`：
>    没有文本表示的粘贴（图片、文件列表）返回 `false`，交回框架自己的路径——
>    这正是旧钩子在 `text == ""` 时的行为。`paste_test.go` 改用 `transfer.TextData(...)`，
>    并补一条「图片粘贴不接管」的断言。
>
> **修正（P7-gf·上一轮 `Space(12)` 是单位搞混）**：上一轮把状态行的侧内边距从 `Space(3)` 改成
> `Space(12)`，注释说「对齐参考的 12u」——但本仓库 `Space(n) = --u * 4n`（`internal/theme/theme.go`），
> 参考的 `padding: 0 calc(var(--u) * 12)` 正是 `Space(3)`；`Space(12)` = 48u，把状态行的句子推进了
> 36u。实证：`Space(3)` 时状态行文字 X=54（与普通结果行标题同列，即参考的 16u 文本列 + 28u 图标
> + 10u gap），`Space(12)` 时 X=90。已改回 `Space(3)`。
>
> **修正（P7-gf·测试引用了不存在的字段）**：上一轮新增的 `TestStatusNotePaddingMatchesReference`
> 用了 `a.Items = []Item{...}`——`App` 没有这个字段，测试包根本编译不过；断言的 64px 也是按
> 「Spacing=4、16u*4」误算的。改写成**与真实结果行标题同列**的断言（先跑普通目录取第一个结果行
> 标题的 X，再在剪贴板关闭态取状态行文字的 X，两者必须相等）。变异核查：把内边距换回 `Space(12)`
> → 立刻红（`status note text X = 90, want 54`）。
>
> **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go tool mygo vet .` 净、
> `go test -count=1 ./...` 全绿、`GOOS=linux/windows/darwin go build ./...` 三平台通过。

> **P7-ge 历史行的两步删除回到原版的键盘专属——移除行内 ✕ 控件，装填后只有
> “Press again to delete”纯文本提示
> （原版 `src/launcher/LauncherResults.tsx:805,908-911`、`src/styles/launcher.css:1450-1455`；
> R50 的删除状态机在 `src/plugins/history-actions.ts`，UI 只有 `.launcher-result__armed` 文本，
> 没有点击控件）**
> ——对照上一轮 P7-gd 之后的「具体待办（P7-gb 重写）」时顺带发现：P7-v 之后本构建的行尾
> ✕ 控件仍是自造的第二条删除路，原版从没有过点击删除。本轮把这条多余控件清掉，删除只走
> 键盘 ⌘⌫，装填行只显示提示文字（`deleteConfirm`）。依赖检查：`go.mod` 已是 **v0.4.8** =
> GitHub releases latest，无版本可升；只对照 `main`（`backup/main-pre-mygo` = v0.3.14
> 原版树），未翻历史提交。
>
> **偏离（P7-ge·删除行有自造的点击控件）**：原版 `LauncherResults.tsx:805` 只问
> `deleteArmed = item.id === armedDeleteId && favoriteEntry !== undefined`，`:908-911` 在
> 装填时画一个 `<span className="launcher-result__armed">{t("launcher.deleteConfirm")}</span>`；
> CSS `launcher.css:1450-1455` 是 `--text-warning` + `--text-caption` + `font-weight: 560`。
> **本构建（改前）**：`internal/launcher/launcher.go` 的 `rowBody` 在未装填时用
> `a.rowControl(c, "close", copy.HistoryDelete, …)` 画 ✕，装填时除提示文字外还画一个可点击的
> close 图标；i18n 因此多一个 `HistoryDelete`（“Delete”/“删除”）键。
>
> **落地（P7-ge）**：`internal/launcher/launcher.go` 的 `rowBody` 删掉 `rowControl` 的 close 分支
> 与装填态里的 close 图标，装填时只画 `copy.DeleteConfirm`（字号 caption、字重 560、`t.Warning`，
> 与原版一致）；`internal/i18n` 删除不再使用的 `HistoryDelete` 键；`view_test.go` 的
> `TestClipboardModeFavoritesAndDelete` / `TestCalculatorDeleteControl` 相应改为用 `deleteKey`。
>
> **验收**：两者都在字段键状态机测试（`fieldkeys_test.go`）的既有覆盖上——`⌘⌫` 装填、再按确认、
> Escape/其他键/超时解除都不变；视图测试改用 `deleteKey` 仍走完整两步。原版对照已确认没有点击
> 删除控件。
>
> **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 全绿、
> `GOOS=linux/windows/darwin go build ./...` 三平台通过。
>
> **清单状态**：「具体待办（P7-gb 重写）」第 3 条（body 的 running/failed/empty 状态行）仍是
> 下一轮最高优先级；本轮无新待办。

> **P7-gd 独立窗口 bar 的关闭键改成原版的 X 图标、rerun 恢复运行中禁用态
> （原版 `src/plugin-window/DetachedPluginApp.tsx:284-315`、`src/styles/launcher.css:1752-1771`）**
> ——P7-gb 收尾后「具体待办（P7-gb 重写）」还剩第 2·3 条，本轮取最高优先级的第 2 条落地（P7-gd）。
> 依赖检查：`go.mod` 已是 **v0.4.8** = GitHub releases latest，无版本可升；只对照 `main`
> （`backup/main-pre-mygo` = v0.3.14 原版树），未翻历史提交。
>
> **偏离 1（P7-gd·关闭键画的是词不是 X）**：原版 bar 的两个键都叫 `.plugin-window__bar-button`，但
> close 键的键面是图标键——`<X size={16} strokeWidth={1.8} aria-hidden>`，只有 `title`/`aria-label`
> 取 `pluginWindow.close`（`DetachedPluginApp.tsx:307-315`）。**本构建（改前）**：
> `internal/shell/detach.go` 的 `barButton(label, run)` 对 rerun 与 close 都画 `ui.Text(label)`，
> 词表上「Close」和「Rerun」是两个同形的文字键。
>
> **偏离 2（P7-gd·rerun 没有运行中禁用态）**：原版 rerun 键 `disabled={state.status === "running"}`，
> CSS `:disabled { opacity: 0.5; cursor: default }`、hover 规则也带 `:not(:disabled)`
> （`DetachedPluginApp.tsx:293-306`、`launcher.css:1766-1771`）。**本构建（改前）**：
> `rerunDetached` 内部挡了 `detached.running`，但键面永远是可点的亮色，用户看不出「正在执行」。
>
> **落地**：`internal/shell/detach.go` 的 `detachedView`——`barButton(label, enabled, run)` 在
> `enabled == false` 时 `Disabled(true)`（mygo 的 `ui.Element.Disabled` 会挡住点击/焦点，并按框架
> 规则把整棵子树降到 0.5 不透明度，正是原版 `:disabled` 的观感），且只在启用时才画 hover 底；rerun
> 传 `!detached.running` 并仍用文字键；close 传 `true` 改为 `ui.Icon(c, launcher.Glyph("close"))`
> （`.Size(t.Space(4), t.Space(4))`，即原版的 16px），`Label(copy.OutputWindowClose)` 继续保留可访问名。
> 关闭路径补 `detached.win != nil` 守卫（测试里窗口常为 nil，原代码会 panic）。
>
> **验收**：新增 `internal/shell/detachbar_test.go` 三条——`TestTheCloseKeyIsTheReferenceIconKey`
> （close 键墨迹纵横比 ≈1、rerun 文字键更宽更扁，且两个可访问名都在）、
> `TestTheRerunKeyDimsWhileARunIsInFlight`（`running` 时 rerun 键的墨色比空闲时更贴近底色）、
> `TestTheCloseKeyClosesTheWindow`（点 close 的图标键仍能关窗，同时走到 nil 守卫）。变异核查：把
> close 换回文字 → 纵横比 2.88、红；把 `Disabled` 去掉 → 两次墨色完全相同、红。
>
> **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 全绿（1384 项）、
> `GOOS=linux/windows/darwin go build ./...` 三平台通过。
>
> **清单状态**：第 2 条（P7-gd）落地；「具体待办（P7-gb 重写）」还剩第 3 条（body 的
> running/failed/empty 状态行），是下一轮最高优先级；第 2 条文中「文本窗是否也补 bar」的取舍留作
> 第 3 条之后。

> **P7-gb 独立窗口（pin/detach）的标题与词表回到原版
> （原版 `src/plugin-window/DetachedPluginApp.tsx:120-124`、`src/plugin-window/detach.ts:140`、
> `src/i18n.ts:46-49,969-976`）**
> ——P7-ga 收尾后「具体待办（P7-fz 重写）」归零，本轮按规则回到「对照原版排查偏离点」，换一个
> **从未进入任何历史待办**的入口：原版独立插件窗口 `src/plugin-window/DetachedPluginApp.tsx`
> （本构建对应 `internal/shell/detach.go` 与 `internal/shell/pinned.go`），把清单重写为 3 条带
> file:line / 原版对照 / 验收标准的具体待办（见文末「具体待办（P7-gb 重写）」），并取第 1 条落地。
> 依赖检查：`go.mod` 已是 **v0.4.8** = GitHub releases latest，无版本可升；只对照 `main`
> （`backup/main-pre-mygo` = v0.3.14 原版树），未翻历史提交。
>
> **偏离 1（P7-gb·独立窗口的词是自造的、任务栏标题没有 app 尾巴）**：原版独立窗口把 OS 标题设成
> `` `${windowTitle} · floter` ``（`DetachedPluginApp.tsx:120-124`，U+00B7 中点）、fallback 名取
> `pluginWindow.fallbackTitle`（“floter plugin”/“floter 插件”，`i18n.ts:50,976`），窗口内三个词
> 取 `pluginWindow.idle`（“No command pinned yet”/“尚未固定任何命令”，`i18n.ts:49,975`）与
> `pluginWindow.rerun`（“Rerun”/“重新执行”，`i18n.ts:47,973`）。**本构建（改前）**：
> `internal/i18n/copy.go:208,209,211,1060,1063` 把三条词意译成 “floter command” / “Re-run” /
> “Nothing has run yet” 与 “floter 命令” / “还没有执行过”；`internal/shell/detach.go` 的窗口
> 标题直接用内容标签，**没有** ` · floter` 尾巴。
>
> **偏离 2（P7-gb·窗口 bar 的关闭键不是原版的 X、rerun 没有运行中禁用态）**：原版
> `DetachedPluginApp.tsx:284-315` 的 Close 是 `<X size={16} strokeWidth={1.8} aria-hidden>` 图标键
> （只有 `title`/`aria-label` 取词），rerun 文本键在 `state.status === "running"` 时 `disabled`。
> **本构建**：`internal/shell/detach.go:145-169` 的 `barButton` 对两个键都画**文本**，没有 X 图形，
> 也没有 `running` 禁用态（`rerunDetached` 内部挡了一下，但键面看不出来）。
>
> **偏离 3（P7-gb·窗口 body 缺 running/failed/empty 三条状态行）**：原版 body 是一台状态机
> （`DetachedPluginApp.tsx:322-338`）：`running` → `launcher.externalRunning`（“Running…”）、
> `failed` → `runErrorMessage(failure)` 本地化（`src/extensions/run-errors.ts`）、`done && view ===
> null` → `launcher.externalEmpty`（“The command produced no output”）。**本构建**：
> `internal/shell/detach.go:170-179` 只画 `detached.view.Text`（空则 `OutputWindowIdle`），重跑在飞
> 的整段时间仍显示旧输出。
>
> **落地（第 1 条）**：`internal/i18n/copy.go` 的 `OutputWindowFallback`/`OutputWindowRerun`/
> `OutputWindowIdle` 改回原版词（en “floter plugin”/“Rerun”/“No command pinned yet”，zh
> “floter 插件”/“重新执行”/“尚未固定任何命令”）；`internal/shell/detach.go` 新增
> `detachWindowTitleSuffix`（`" \u00b7 floter"`）与 `detachWindowTitle(label)`，窗口创建与复用两处
> 的 `SetTitle`/`WindowOptions.Title` 都改走它（窗口**内**的 header 仍用裸标签）。
>
> **验收**：`internal/shell/detachwords_test.go` 新增
> `TestDetachedWindowWordsMatchTheOldDictionary`（en/zh 三条词逐字）与
> `TestDetachedWindowTitleCarriesTheAppTail`（`detachWindowTitle("jv --floter") ==
> "jv --floter · floter"`）。变异核查：把 `detachWindowTitle` 换回裸标签 → 标题条立刻红；把三个词
> 任一改回旧译 → 词表条红。
>
> **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 全绿、
> `GOOS=linux/windows/darwin go build ./...` 三平台通过。
>
> **清单状态**：第 1 条（P7-gb）落地；「具体待办（P7-gb 重写）」还剩第 2·3 条（bar 的 X 键与
> 禁用态 / body 的三条状态行），是下一轮最高优先级。

> **最新一轮：P7-ga 终端 palette 行回到原版的一行分列、stacked 行恢复同一字级
> （原版 `src/settings/TerminalAppearance.tsx:271-292`、`src/styles/settings.css:520-546,871-878`）**
> ——P7-fz 收尾后「具体待办（P7-fz 重写）」有 2 条待做（palette 行布局 / stacked 行字级），
> 本轮两条一并落地，清单归零。依赖检查：`go.mod` 已是 **v0.4.8** = GitHub releases latest，
> 无版本可升；只对照 `main`（`backup/main-pre-mygo` = v0.3.14 原版树），未翻历史提交。
>
> **偏离 1（P7-ga·palette 行是尾随 select + 下方预览，不是一行分列）**：原版
> `TerminalAppearance.tsx:271-292` 的 `terminal_theme` 行 `stacked: true`，control 是
> `.terminal-palette`（`settings.css:871-878`：`width:100%; display:flex; align-items:center;
> justify-content:space-between`），左边 `select.settings-select`（`max-width:220px`）、右边
> `TerminalPalettePreview`（`min-width:148u`）。**本构建（改前）**：
> `internal/settingsui/settingsui.go` 的 `paletteRow` 用 `a.row(..., below→palettePreview)`——
> select 在标签同一行的尾随槽，预览块在整行下方，读成「两行堆叠」。
>
> **偏离 2（P7-ga·stacked 行的标签/说明比尾随行小一档）**：原版 `.settings-row__label`
> （`--text-emphasis`）与 `.settings-row__sublabel`（`--text-body`）对 stacked 与尾随是同一条规则
> （`settings.css:520-546`），`SettingsRow` 不因 `stacked` 改字级（`SettingsRows.tsx:104-135`）。
> **本构建（改前）**：`rows.go` 的 `stackedRow` 与 `settingsui.go` 的 `slider` 用
> `FontSize` + `FontSize-1`，而尾随行 `rowAligned` 用 `FontSize+1` + `FontSize`——同一个标签在
> stacked 行读 22px、尾随行读 23px。
>
> **落地**：
> - `internal/settingsui/settingsui.go`：`paletteRow` 改 `a.stackedRow`，control 内
>   `ui.Row(c).FillWidth().Justify(ui.SpaceBetween).AlignItems(ui.Center)`——左 select
>   （`Width(220)`）、右预览；`palettePreview` 从 `FillWidth()` 改为固定 `t.Space(37)`（实测 148u，
>   与原版 `min-width:148u` 一致，也与终端抽屉同一宽度）。
> - `internal/settingsui/rows.go`：`stackedRow` 标签 `FontSize+1`、说明 `FontSize`。
> - `internal/settingsui/settingsui.go`：`slider` 标签 `FontSize+1`、说明 `FontSize`（读数保持
>   原版的 `--text-body`）。
>
> **验收**：`internal/settingsui/view_test.go` 新增
> `TestThePaletteRowSharesOneLineWithItsPreview`（select 触发文本顶边 ≥ 标签底边；预览与 select
> 触发文本垂直重叠、预览在其右）与 `TestStackedRowsKeepTheTrailingRowsTypeSteps`
> （Language 标签/提示的 `H` == Launch at startup 标签/提示的 `H`）。变异核查：把 `paletteRow`
> 换回 `a.row` → 预览跑到下一行、测试红；把 `stackedRow` 字级换回 `FontSize`/`FontSize-1` →
> 22≠23、红。
>
> **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 全绿（1378 项）、
> `GOOS=linux/windows/darwin go build ./...` 三平台通过。
>
> **清单状态**：第 1·2 条（P7-ga）全部落地；「具体待办（P7-fz 重写）」**全部结清、清单归零**。
> 下一轮回到「对照原版排查偏离点」，重新写实清单。

> **最新一轮：P7-fz General 页的分段选择行回到原版的 `stacked` 全宽布局
> （原版 `src/settings/GeneralPage.tsx:340,356,382,516,568`、`src/styles/settings.css:557-577`）**
> ——P7-fy 收尾后「具体待办（P7-fy 重写）」只剩第 3 条（Appearance 卡的三个分段行未按原版的
> `stacked` 全宽布局绘制），本轮取它落地，清单归零。依赖检查：`go.mod` 已是 **v0.4.8** = GitHub
> releases latest，无版本可升（v0.4.8 的 release notes 无新用点）；只对照 `main`
> （`backup/main-pre-mygo` = v0.3.14 原版树），未翻历史提交。
>
> **偏离（P7-fz·分段选择行是尾随控件而不是 stacked 全宽行）**：原版 General 页里五处
> `<SettingsRow stacked …><SegmentedChoice/></SettingsRow>`（`GeneralPage.tsx:340,356,382` 的
> Theme / Language / UI scale、`:516` 的 AppIcon、`:568` 的 Glass）把分段控件放在标签与说明
> **下面的一整行**；CSS `.settings-row--stacked` 是 `grid-template-columns: minmax(0,1fr)`、
> `.settings-row--stacked .settings-row__control { width:100%; display:grid; grid-template-columns:
> minmax(0,1fr) }`（`settings.css:557-577`），分段轨道再用 `repeat(auto-fit, minmax(0,1fr))` 均分
> 整行（`settings.css:1016-1024`），`TerminalAppearance.tsx:388` 的 `stacked={row.slider||row.stacked}`
> 是同一形状。**本构建（改前）**：`internal/settingsui/settingsui.go:2811-2826` 的 `pick` 走
> `a.row`（`rows.go:124-126` → `rowAligned` 的尾随 control 槽），于是 Theme/Language/UI scale/
> Glass/AppIcon 的分段控件与标签同一行、被挤在右端，分段轨道还按内容宽度收缩而不是占满一行。
>
> **落地**：
> - `internal/settingsui/rows.go`：把原在 `pluginsheet.go` 的 `stackedRow` 移到这里（标签、灰色
>   说明在上，`control` 在下一行整宽），与 `rowAligned`/`slider` 共用同一套卡片行计数与分隔线。
> - `internal/settingsui/settingsui.go`：`pick` 改用 `a.stackedRow`；`segmented` 改用
>   `control.SegmentedFill`（只有这里与插件配置的 radio 行用，两处本来就是原版的 stacked 行）。
> - `internal/control/control.go`：新增 `SegmentedFill`，与 `Segmented` 共用一个内部
>   `segmented(..., fill bool)`——全宽形态让轨道 `FillWidth()`、每个分段 `Grow(1)`，复刻原版
>   `repeat(auto-fit, minmax(0,1fr))` 的均分（尾随/抽屉继续用原来的内容宽度形态）。
>
> **验收**：`internal/settingsui/view_test.go` 新增 `TestAppearanceChoicesStackUnderTheirLabels`
> （Theme（以 ThemeHint 为锚）/ Language / UI scale 三行：首个分段顶边 ≥ 标签行底边，且首尾分段
> 跨度 ≥ 行宽的一半）；变异核查：把 `pick` 的 `a.stackedRow` 换回 `a.row` → 测试立刻红
> （theme/language 的分段跑到标签同一行、跨度缩到 134/100）。
>
> **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 全绿（1376 项）、
> `GOOS=linux/windows/darwin go build ./...` 三平台通过。
>
> **清单状态**：第 3 条（P7-fz）落地；「具体待办（P7-fy 重写）」3 条**全部结清、清单归零**。
> 本轮按审计入口把清单重写为 2 条带 file:line / 原版对照 / 验收标准的具体待办（见文末「具体待办
> （P7-fz 重写）」），是下一轮的最高优先级。

> **P7-fy 设置页终端外观卡的读数与 About 页更新行的分组回到原版
> （原版 `src/settings/TerminalAppearance.tsx:185-215`、`src/settings/AboutPage.tsx:62-70`）**
> ——P7-fx 收尾后「具体待办（P7-fw 重写）」归零，本轮按规则换入口：逐字段对照原版 General 页的
> 「终端外观」卡（`TerminalAppearanceSettings` 的 `card` 变体）与 About 页的分组结构
> （`AboutPage` 的 `settings.group.update` 段），把清单重写为 3 条带 file:line / 原版对照 /
> 验收标准的具体待办（见文末「具体待办（P7-fy 重写）」），并取第 1·2 条一并落地。依赖检查：
> `go.mod` 已是 **v0.4.8** = GitHub releases latest，无版本可升（v0.4.8 的 release notes 用点
> `ui/editor` 已由 P7-ev 落地；`ui.Access` 口子因 `platform.AccessNode` 仍是内部类型而接不到 app
> 侧，error toast 的 `role="alert"` 继续如实留在「已知差距」）；只对照 `main`
> （`backup/main-pre-mygo` = v0.3.14 原版树），未翻历史提交。
>
> **偏离 1（P7-fy·设置卡的两个滑块读数与抽屉不一致）**：原版 `TerminalAppearance.tsx:185-215`
> 的同一份 schema 喂两个面，字号行的读数是 `` `${fontSize} px` ``（`:194`）、行距行的读数是
> `` `${lineHeight.toFixed(2)}×` ``（`:213`），而字号行的 `sublabel` 只在 `variant === "card"`
> 时是 `t("settings.terminalAppearanceHint")`（`:194`）、抽屉侧 `undefined`（`:195`）。**本构建
> （改前）**：`internal/settingsui/settingsui.go:2764-2786` 的字号读数是裸数字（`fmt.Sprintf("%d", …)`，
> 漏了 ` px`）、行距读数是 `%.1f×`（漏了 `toFixed(2)` 的第二位），且字号行的 hint 传空——于是同一个
> `14 px` / `1.20×` 值在设置卡读作 `14` / `1.2×`，在终端抽屉读作 `14 px` / `1.20×`（`internal/terminalui/drawer.go:69-71,88-99`
> 早已是原版读法），两面各说一套。
>
> **偏离 2（P7-fy·About 页的更新行没有原版的分组）**：原版 `AboutPage.tsx:62-70` 把更新行放在
> 自己的 `<section>` 里——`settings.group.update` 的标题 + `<SettingsCard label=…>`——而不是散落在
> 构建身份行之间。**本构建（改前）**：`internal/settingsui/settingsui.go:2560-2580` 的 `about` 把
> `updateRow` 直接塞在第二张无标题的卡里，与「框架 / 链接协议 / 设置文件 / 项目」同一层，页面上
> 唯一要用户按的控件没有自己的分组。
>
> **落地**：
> - `internal/settingsui/settingsui.go` 的 `general`：`AxisFontSize` 的 format 改为
>   `fmt.Sprintf("%d px", …)`，`AxisLineHeight` 改为 `fmt.Sprintf("%.2f\u00d7", …)`，字号行传
>   `copy.TerminalHint` 作 sublabel（复刻原版 card-only hint）。同文件 `paletteRow` 把原来的
>   `copy.TerminalHint` sublabel 去掉：原版调色板行是 `stacked: true` 且不画 hint
>   （`TerminalAppearance.tsx:272-276`），卡-only 的 hint 只属于字号行。
> - `internal/i18n`：新增 `Settings.GroupUpdate`（en “Updates” / zh “更新”，逐字取 `i18n.ts:213,1115`）。
> - `internal/settingsui/settingsui.go` 的 `about`：`a.card(updateRow)` 换成
>   `a.section(c, copy.GroupUpdate, "", func() { a.card(...) })`。
>
> **验收**：`internal/settingsui/view_test.go` 新增 `TestTerminalCardReadoutsMatchTheDrawer`
> （设置卡渲染出 `14 px` 与 `1.20×` 两个读数 + `TerminalHint` 恰好一次），并改 `TestAboutUpdateRow…`
> 断言更新行位于 `en.GroupUpdate` 标题下方。变异核查：把字号 format 改回 `%d` → 读数测试红；
> 把行距改回 `%.1f×` → 红色（`1.20×` 不存在）；把 paletteRow 的 sublabel 改回 `copy.TerminalHint`
> → hint 计数变 2 而红；把 `about` 的 section 换回裸 `a.card` → 分组测试红。
>
> **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 全绿（1375 项）、
> `GOOS=linux/windows/darwin go build ./...` 三平台通过。
>
> **清单状态**：第 1·2 条（P7-fy）落地；「具体待办（P7-fy 重写）」还剩第 3 条（Appearance 卡的三个
> 分段行未按原版的 `stacked` 全宽布局绘制），是下一轮最高优先级。

> **P7-fx Sessions 行的状态色与整行激活回到原版
> （原版 `src/settings/SessionsPage.tsx:170,188-191,197-198`、`src/styles/settings.css:1280-1296`）**
> ——P7-fw 收尾后「具体待办（P7-fw 重写）」还剩第 2·3 条（exited 行状态色 / 整行激活），本轮取
> 最高优先级、两条一并落地，清单归零。依赖检查：`go.mod` 已是 **v0.4.8** = GitHub releases latest，
> 无版本可升；只对照 `main`（`backup/main-pre-mygo` = v0.3.14 原版树），未翻历史提交。
>
> **偏离 1（P7-fx·exited 行没有 `session-state--exited` 的状态色）**：原版 `SessionsPage.tsx:188-191`
> 把状态词单独放在 `<span className={session.exited ? "session-state session-state--exited" :
> "session-state"}>`，`src/styles/settings.css:1290-1296` 给 `.session-state` 的是 **`var(--accent)`**
> （运行中的状态词是强调色）、`.session-state--exited` 回落 **`var(--text-tertiary)`**；同行的
> `.session-manager__meta`（`:1280-1286`）底色就是 `--text-tertiary`，尺寸与时间始终安静。**本构建
> （改前）**：`internal/settingsui/settingsui.go:1237-1267` 的 `sessionLines` 把状态词、尺寸、时间
> `" · "` 拼成一个字符串数组，`internal/settingsui/rows.go:131-133` 的 `rowLines` 对每一行一律
> `TextColor(t.TextMuted)`——状态词与其它 meta 同色，running 行也没有原版的 accent。
>
> **偏离 2（P7-fx·只有 Show 按钮能恢复会话）**：原版 `SessionsPage.tsx:170,197-198` 的
> `resumable = !session.exited && !session.attached`，`SettingsRow` 的 `onActivate` 只在
> `resumable` 时挂上 `() => onResume(session)`——**整行点哪里都恢复**；Play 按钮（`:203-213`）
> 是同一动作的显式控件。**本构建（改前）**：`internal/settingsui/settingsui.go:1175` 把 `a.rowLines`
> 的返回值 `clicked` 直接丢弃（`rows.go:174-176` 已经把它记下），恢复只能由 Show 按钮触发。
>
> **落地**：
> - `internal/settingsui/rows.go`：新增 `rowLine`（一条灰行 = 纯文本或若干带角色色的段）、`rowSegment`
> （段文本 + `rowSegmentRole`：`rowSegmentTertiary` = `tokens.TextTertiary`、`rowSegmentAccent` =
> `t.Accent`）；`rowLines`/`rowAligned` 的 `[]string` 换成 `[]rowLine`，有段的行改画成
> `ui.Row` 的 `Wrap()` 元信息行（复刻 `.session-manager__meta` 的 `flex-wrap`）。
> - `internal/settingsui/settingsui.go`：`sessionLines` 返回 `[]rowLine`，新增 `sessionMeta`
>   把状态词单独成段（`session.Running` → accent，否则 tertiary），尺寸/时间仍 tertiary；`sessions`
>   里把 `rowLines` 的返回值接住，`resumable := session.Running && !session.Active` 时才调
>   `Actions.SelectSession(session.Index)`。
>
> **验收**：`internal/settingsui/sessionsstate_test.go` 两条——
> `TestSessionLinesMarkTheStateWordWithItsOwnColourRole`（running 段 = accent、exited 段 = tertiary、
> 尺寸/时间恒 tertiary）与 `TestSessionStateWordsCarryTheOldStateColours`（渲染后逐词取墨色：
> running/exited 状态词不同、running 状态词 ≠ muted 尺寸、exited 状态词 = 同色 muted 尺寸、两行的
> 尺寸相同）；`TestClickingAResumableSessionRowResumesIt`（点 vim 行 → `SelectSession(1)`；active 行
> 与 exited 行点了不响应；显式 Show 仍可用）。变异核查：把段的颜色改回 `t.TextMuted` → 像素测试红；
> 把 `resumable` 放宽为 `true` → 激活测试在「active/exited 行不响应」处红。
>
> **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 全绿（1374 项）、
> `GOOS=linux/windows/darwin go build ./...` 三平台通过。
>
> **清单状态**：第 2·3 条（P7-fx）落地；「具体待办（P7-fw 重写）」3 条**全部结清、清单归零**。
> 下一轮回到「对照原版排查偏离点」，重新写实清单。

> **P7-fw Sessions 行的退出状态词回到原版（原版 `src/settings/SessionsPage.tsx:173-177`、
> `src/i18n.ts:179,1085`）**
> ——P7-fv 收尾后「具体待办（P7-fv 重写）」归零，本轮按规则换入口：逐字段对照原版 Sessions 页的
> 状态行（此前只对照过排序轴、行事实与关闭武装，状态词本身未比对），把「尚未做」重写为 3 条带
> file:line / 原版对照 / 验收标准的具体待办（见文末「具体待办（P7-fw 重写）」），并取第 1 条落地。
> 依赖检查：`go.mod` 已是 **v0.4.8** = GitHub releases latest，无版本可升；只对照 `main`
> （`backup/main-pre-mygo` = v0.3.14 原版树），未翻历史提交。
>
> **偏离（P7-fw·Sessions 行把退出码画在干净退出上）**：原版 `SessionsPage.tsx:173-177` 的 `state`
> 是 `session.exited ? t("terminal.sessionExited") + (session.exitCode !== 0 ? ` · exit ${code}` : "") : ...`，
> 即 **0 只写「Exited」**、非零才追加原版字面 ` · exit N`。**本构建（改前）**：
> `internal/terminalui/terminalui.go:338-356` 的 `NoteOf` 对任何结束的会话都返回
> `copy.ProcessExited(code)`（“Process exited · code 0”），与终端表面 resident note 的那条词混用；
> `internal/i18n` 只有 `ProcessExited`，没有原版的 `SessionExited`。
>
> **落地**：`internal/i18n` 新增 `Terminal.SessionExited`（en “Exited”、zh “已退出”，逐字取
> `i18n.ts:179,1085`）；`terminalui.NoteOf` 只在非零时拼 `" · exit " + strconv.Itoa(code)`，0 只返回
> `SessionExited`；`ProcessExited` 仍只属于 resident note。
>
> **验收**：`internal/terminalui/sessions_test.go` 新增
> `TestSessionNoteNamesTheExitCodeOnlyWhenItIsNotZero`（en/zh：0 → “Exited”/“已退出”，3 →
> “Exited · exit 3”/“已退出 · exit 3”，运行中两态、越界空）；`internal/shell/scripttool_test.go` 的
> `TestSessionsListReportsEverySession` 追加 ended 行 Note 逐字断言；`internal/i18n/i18n_test.go` 的
> `TestTerminalBarWordsMatchTheOldDictionary` 补 en/zh `SessionExited`。变异核查：把 `NoteOf` 改回
> `ProcessExited` → 新测试立刻红。
>
> **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 全绿（1371 项）、
> `GOOS=linux/windows/darwin go build ./...` 三平台通过。
>
> **清单状态**：第 1 条（P7-fw）落地；「具体待办（P7-fw 重写）」还剩第 2·3 条（exited 行状态色 /
> 整行激活），是下一轮最高优先级。

> **最新一轮：P7-fv About 页更新行回到原版的一行状态机（原版 `src/settings/AboutPage.tsx:35-113`、
> `src/App.tsx:548-552,2763-2809`、`src/i18n.ts:416-422,1303-1309`）**
> ——P7-fu 收尾后「具体待办（P7-fs 重写）」归零，本轮按规则换入口：逐字段对照原版 About 页
> （此前只做过「无更新源时如实说明」的收尾，更新行本身未对照），把「尚未做」重写为 3 条带
> file:line / 原版对照 / 验收标准的具体待办（见文末「具体待办（P7-fv 重写）」），并三条一并落地。
> 依赖检查：`go.mod` 已是 **v0.4.8** = GitHub releases latest，无版本可升；只对照 `main`
> （`backup/main-pre-mygo` = v0.3.14 原版树），未翻历史提交。
>
> **偏离（P7-fv·更新行不是原版的一行状态机）**：原版 `AboutPage.tsx:71-111` 是一行
> `SettingsRow`——label `Current version: v{appVersion}`，sublabel 按 `updateFailed → updateInfo →
> upToDate` 三选一，control 按 `updateProgress → updateDownloading → updateFailed → updateInfo`
> 四态画字节进度条 / 禁用的 `Installing…` / `Retry` / `Download`；`App.tsx:2763-2769` 挂载即
> 自动 `check()`，页面上没有「检查更新」按钮。**本构建（改前）**：`internal/settingsui/settingsui.go`
> 的 `updateRow` 是「Check for updates」+ 可选「Download and install」+ 自由句 `UpdateStatus`；
> `internal/shell/update.go` 只存本地化字符串，版本/下载中/失败/进度不可区分，启动也不检查。
>
> **落地**：
> - `internal/settingsui/settingsui.go`：新增 `UpdateState`/`UpdateProgress` 与四态 `updateRow`；
>   新增 `updateProgress`（track + accent bar + 读数）、`updateProgressLabel`、`formatBytes`（逐字
>   复刻 `AboutPage.tsx:9-15,45-48,75-88`）；删掉 `Actions.CheckForUpdates` 与旧
>   `UpdateStatus`/`UpdateReady`。
> - `internal/shell/update.go`：`checkForUpdates`/`installUpdate` 改写状态机；`installUpdate`
>   先重查再装（复刻 `downloadAndInstallUpdate`），失败可重试，成功后 `relaunchApp()`；
>   `Start()`（`shell.go:1182`）启动即检查，复刻原版挂载检查。
> - `internal/i18n`：删掉旧的 `UpdateCheck/UpdateChecking/UpdateCurrent/UpdateFailed/
>   UpdateAvailable/UpdateInstall/UpdateInstalling/UpdateInstalled/AboutVersion`，换成原版八条
>   （`UpdateCurrentVersion`/`UpdateLatestVersion`/`UpdateUpToDate`/`UpdateDisabled`/
>   `UpdateDownload`/`UpdateInstalling`/`UpdateRetry`/`UpdateFailedRetry`），en/zh 逐字取
>   `i18n.ts:416-422,1303-1309`。
>
> **验收**：`internal/settingsui/view_test.go` 新增 `TestAboutUpdateRowFollowsTheOldStateMachine`
> （五行/四态逐字与 Download/Retry 落点）与 `TestUpdateProgressFormatting`（`formatBytes` 七例、
> 两条 label、越界 100% 夹取）；`internal/shell/shell_test.go` 的 `TestUpdateCheckWithoutAFeed`
> 改断状态机。变异核查：把 control 分支换回「Check for updates」→ 状态机测试红；把 `formatBytes`
> 换回整数打印 → 进度测试红。
>
> **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 全绿（1370 项）、
> `GOOS=linux/windows/darwin go build ./...` 三平台通过。
>
> **清单状态**：「具体待办（P7-fv 重写）」3 条**全部结清、清单归零**。下一轮回到「对照原版排查
> 偏离点」，重新写实清单。

> **最新一轮：P7-fu 扩展 homepage 的任何打开失败都成为错误通知
> （原版 `src/ExtensionsPanel.tsx:2577-2580`）**
> ——P7-ft 收尾后「具体待办（P7-fs 重写）」只剩第 3 条（homepage 的非白名单类打开失败仍只记日志），
> 本轮取它落地，清单归零。依赖检查：`go.mod` 已是 **v0.4.8** = GitHub releases latest，无版本可升；
> 只对照 `main`（`backup/main-pre-mygo` = v0.3.14 原版树），未翻历史提交。
>
> **偏离（P7-fu·homepage 的非白名单失败只记日志）**：原版 `onRepair` 的 homepage 分支
> `void invoke("open_url", { url: extension.homepage }).catch((error) => {
> onNotify("error", errorMessage(error, t)); })`（`ExtensionsPanel.tsx:2577-2580`）：任何 reject
> 都是一条 error 通知——非 web 的 keyed 拒绝经 `errorMessage` 本地化成
> `settings.extensions.openNonWebUrl`，普通 spawn 错误原样显示。**本构建（改前）**：
> `internal/shell/shell.go:1742-1757` 的 `openHomepage` 只对 `errors.Is(err, errNonWebURL)` 排队，
> 其余错误只 `log.Printf`，用户看不到。
>
> **落地**：
> - `internal/shell/shell.go` 的 `openHomepage`：任何 `openURL` 错误都 `queueNotice`；错误串默认取
>   `err.Error()`（对应原版 `errorMessage` 的原样透传），`errors.Is(err, errNonWebURL)` 时换成
>   i18n 的 `Settings.OpenNonWebURL(url)`。
> - `internal/shell/shell.go` 的注入缝 `Options.OpenURL` / `openURLCommand` 改为
>   `func(browserID, url string) error`，让「普通 spawn 失败」也能在测试里到达 `openHomepage`；
>   `openURL` 把注入命令的错误原样返回，不再吞掉。
>
> **验收**：`internal/shell/weburl_test.go` 新增 `TestHomepageReportsAnOpenerFailure`（注入返回
> `errors.New("opener exploded")` 的 opener，`openHomepage` 后恰有一条 `Type == "error"` 且标题含
> 该错误串的通知）；既有 `TestHomepageRefusesANonWebURL` 继续钉住非 web 的本地化词与「成功保持安静」。
> 变异核查：把 `openHomepage` 的错误分支改回只处理 `errNonWebURL` → 新测试立刻红。
>
> **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 全绿（1369 项）、
> `GOOS=linux/windows/darwin go build ./...` 三平台通过。
>
> **清单状态**：第 3 条（P7-fu）落地；「具体待办（P7-fs 重写）」3 条**全部结清、清单归零**。
> 下一轮回到「对照原版排查偏离点」，重新写实清单。

> **上一轮：P7-ft `open_path` 补回原版的 `~`/相对路径解析与存在性要求
> （原版 `src-tauri/src/commands/actions.rs:73-135`）**
> ——P7-fs 收尾后「具体待办（P7-fs 重写）」还剩第 2·3 条，本轮取最高优先级的第 2 条落地。
> 依赖检查：`go.mod` 已是 **v0.4.8** = GitHub releases latest，无版本可升；只对照 `main`
> （`backup/main-pre-mygo` = v0.3.14 原版树），未翻历史提交。
>
> **偏离（P7-ft·`open_path` 不解析路径、也不要求存在）**：原版 `open_path`（`:73-98`）先
> `resolve_path(&path, &home)`（`:107-135`）再 `resolved.exists()`，不存在返回
> `Path does not exist: …`，前端（`useLauncherActions.ts:430-443,565-571`）把这个错误读成
> 「留在启动器上让用户改」；`resolve_path` 只把整段 `~`/`~/…` 解析成 home，`~alice` 与相对
> 路径都按 `home.join` 处理。**本构建（改前）**：`internal/shell/launch.go:23-40` 的 `openPath`
> 直接 `spawn.Program(systemOpener(), path)`；`~/x` 与 `relative` 原样交给 opener，路径不存在
> 也静默启动——`openTerminalLink`（`:86-107`）对 `~/…` 只做 `HasPrefix` 判断后原样传下去。
>
> **落地**：
> - `internal/shell/launch.go` 新增 `resolvePath(path, home)`（逐字复刻 `resolve_path`：
>   `~`/`~/`/`~//…` 去分隔符后 join home、`~alice` 当字面目录名 join home、相对路径 join home、
>   绝对路径原样）；`openPath` 先取 home（`a.homeDir`，空则 `os.UserHomeDir`，取不到即拒），
>   再 `resolvePath` + `os.Stat`，不存在就发 `OpenPathFailed` 且**不**触达 opener；成功路径统一
>   用一个 `fail` 闭包记日志 + 警告反馈。注入的 `OpenPath` 命令收到的是**解析后**的路径。
> - `openTerminalLink` 无需改动：它已把 `~/x.go:12` 去掉行号后缀后交给 `openPath`，解析与存在性
>   现在集中在 `openPath` 一处。
>
> **验收**：`internal/shell/launch_test.go` 新增 `TestOpenPathResolvesAndRequiresExistence`
> （`~/note.txt` 与 `note.txt` 都触达 `$HOME/note.txt`；`missing.txt` 不触达 opener，帧上画出
> `OpenPathFailed` 逐字）、`TestResolvePathMatchesTheOldResolver`（七条：`~`、`~/`、`~//notes`、
> `~/notes/a.md`、`~alice`、相对路径、绝对路径）、`TestTerminalLinkResolvesTilde`
> （`~/main.go:12` 触达 `$HOME/main.go`）。变异核查：把 `openPath` 的存在性 `os.Stat` 删掉 →
> `TestOpenPathResolvesAndRequiresExistence` 立刻红（missing 会走到注入 opener）。
>
> **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 全绿、
> `GOOS=linux/windows/darwin go build ./...` 三平台通过。
>
> **清单状态**：第 2 条（P7-ft）落地；「具体待办（P7-fs 重写）」只剩第 3 条（homepage 的非
> 白名单类打开失败仍只记日志），是下一轮最高优先级。

> **上一轮：P7-fs 通用 opener 补回原版 `open_url` 的 web 协议白名单
> （原版 `src-tauri/src/commands/actions.rs:44-68`）**
> ——P7-fr 收尾后「具体待办（P7-fq 重写）」归零，本轮按规则换入口：逐文件对照原版
> `src-tauri/src/commands/actions.rs`（此前未进入任何历史待办），把「尚未做」重写为 3 条带
> file:line / 原版对照 / 验收标准的具体待办（见文末「具体待办（P7-fs 重写）」），取第 1 条落地。
> 依赖检查：`go.mod` 已是 **v0.4.8** = GitHub releases latest，无版本可升；只对照 `main`
> （`backup/main-pre-mygo` = v0.3.14 原版树），未翻历史提交。
>
> **偏离（P7-fs·通用 opener 没有协议白名单）**：原版 `open_url`（`:44-52`）先用
> `is_web_url`（`:57-68`）校验，只有 `http://`/`https://`/`ftp://`（大小写不敏感）才
> `spawn_opener`；否则返回 `settings.extensions.openNonWebUrl:{url}`（`i18n.ts:851,1706`）。
> 这条命令的调用点包括 action bar 的 URL 行、插件 output 的 `open` 动作
> （`useLauncherActions.ts:649,851`）和扩展 homepage（`ExtensionsPanel.tsx:2577`）。**本构建
> （改前）**：`internal/shell/shell.go:2522-2535` 的 `openURL` 对空 browserID 直接
> `mygo.Shell.OpenExternal(url)`；`openHomepage`（`:1742-1753`）与 `internal/launcher/output.go:359-364`
> 的 output `open` 动作都走它，`file://`/`javascript:`/`mailto:` 一律交给系统 opener。
> `internal/launcher/actionbar.go:26` 的 `URLQuery` 只在 action bar 这一条路挡住，另外两条没有。
>
> **落地**：
> - `internal/shell/weburl.go`（新）：`webSchemes` 逐字复刻 `is_web_url` 的三个 scheme，
>   `isWebURL` 用长度守卫做大小写不敏感前缀比较（Go 版的 `url.get(..scheme.len())`，短串不会越界）。
> - `internal/shell/shell.go` 的 `openURL`：`browserID == ""`（即原版 `open_url`）时先过
>   `isWebURL`，否则返回 `errNonWebURL`；browser 行始终带自己的 id，走
>   `browser.OpenURL`（原版 `browser_open_url`，没有这条白名单）。`openHomepage` 的失败除日志外，
>   非 web 时用 `queueNotice` 发原版 `settings.extensions.openNonWebUrl` 词。
> - `internal/launcher/output.go` 的 `runOutputRow`：`open` 动作检查错误，失败置
>   `OpenURLFailed` 并留在 output view，复刻原版 `openWithSystem` 的「拒绝不关窗」。
> - `internal/i18n`：`Settings.OpenNonWebURL`（en/zh 逐字取 `i18n.ts:851,1706`）。
>
> **验收**：`internal/shell/weburl_test.go` 新增 `TestIsWebURLMatchesTheOldAllowList`（五个收、
> 八个拒，含 `ftp:/`、` httpx://` 这类近似串）、`TestHomepageRefusesANonWebURL`（`file:///etc/passwd`
> 不触达注入的 opener、队列里是逐字 `Refusing to open a non-web URL: file:///etc/passwd`；
> `HTTPS://…` 仍触达且不排队）、`TestBrowserSpecificOpenKeepsItsOwnSchemes`（`chrome` 行的
> `file://` 仍放行）；`internal/launcher/outputopen_test.go` 新增 `TestOutputOpenRefusesWithoutClosing`
> （失败后 view 仍开、toast 是 `OpenURLFailed`）；`internal/i18n/i18n_test.go` 新增
> `TestOpenNonWebURLWordMatchesTheOldDictionary`（en/zh 逐字）。`internal/shell/launch_test.go` 的
> `TestTerminalLinkOpening` 把 mailto 改成「被拒但由 terminal 回调认领」，并补一个大小写 https 用例。
> 变异核查：把 `openURL` 的白名单判断删掉 → `TestHomepageRefusesANonWebURL` 红；把
> `runOutputRow` 的错误分支删掉 → `TestOutputOpenRefusesWithoutClosing` 红。
>
> **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 全绿、
> `GOOS=linux/windows/darwin go build ./...` 三平台通过。
>
> **清单状态**：第 1 条（P7-fs）落地；「具体待办（P7-fs 重写）」还剩第 2·3 条（`open_path` 的
> `~`/相对路径与存在性；homepage 非白名单类失败仍只记日志），是下一轮最高优先级。

> **上一轮：P7-fr 电源动作等命令的退出结果，而不是只看 spawn 成功
> （原版 `src-tauri/src/commands/system.rs:21-118`）**
> ——P7-fq 收尾后「具体待办（P7-fq 重写）」只剩第 2 条（电源动作要等命令的退出结果），本轮取它落地，
> 清单归零。依赖检查：`go.mod` 已是 **v0.4.8** = GitHub releases latest，无版本可升；只对照 `main`
> （`backup/main-pre-mygo` = v0.3.14 原版树），未翻历史提交。
>
> **偏离（P7-fr·电源只看 spawn 成功）**：原版 `system_power`（`:21-30`）调 `run_first`（`:72-118`）：
> 对每个候选 `Command::output()` 加 30s 超时；**只有** `ErrorKind::NotFound` 才落到下一个候选，候选
> 一旦启动，非零退出（polkit/loginwindow 拒绝）或超时都原样返回错误（带 stderr），不再尝试更特权的
> 后备命令。**本构建（改前）**：`internal/shell/power.go` 的 `runPower` 用 `spawn.Program`
> （fire-and-forget）逐个候选尝试，返回 nil 的条件只是「spawn 成功」——一个启动后立刻被 polkit 拒绝的
> `systemctl` 会被报成成功，而一次权限类 spawn 错误又会继续落到 `reboot`，与「启动后拒绝不降级」的
> 原版规则相反。
>
> **落地**：`internal/shell/power.go` 把 `runPower` 的核心抽成 `runPowerCandidates(candidates, timeout)`，
> 逐字复刻原版 `run_first`：`context.WithTimeout`（30s，`powerActionTimeout`）+ `exec.CommandContext`
> 运行候选，单独缓冲 stderr；`errors.Is(err, exec.ErrNotFound)` 才 `continue`，超时返回
> `timed out while requesting power action`，非零退出返回 `program: exit code N: stderr`，无 stderr 时
> 退回 `program: exit code N`（信号终止则 `terminated by signal`），空表返回原版的
> `Power actions are not supported on this platform`。`spawn.Program` 不再用于电源路径。
>
> **验收**：`internal/shell/power_test.go`（新，`//go:build unix`）六条——候选一缺失时落到候选二
> （`sh -c "exit 0"`）；候选一启动后 `exit 7` 且 stderr 有「permission denied」时返回错误、**不跑候选二**
> （候选二写 sentinel 文件，断言不存在）、错误串同时含「exit code 7」与「permission denied」；挂起的
> 工具被 20ms 超时切断；无 stderr 的 `exit 3` 仍报「exit code 3」；未知动作被拒；空候选表报「not
> supported」。变异核查：把 `runPowerCandidates` 换回 `spawn.Program` 的「spawn 成功即返回 nil」→
> 「refusal」与「timed out」两条立刻红。
>
> **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 全绿、
> `GOOS=linux/windows/darwin go build ./...` 三平台通过。
>
> **清单状态**：第 2 条落地；「具体待办（P7-fq 重写）」2 条**全部结清、清单归零**。下一轮回到
> 「对照原版排查偏离点」，重新写实清单。

> **上一轮：P7-fq settings 写盘补回原版的恢复副本与原子替换
> （原版 `src-tauri/src/commands/config.rs:1473-1506`）**
> ——P7-fp 收尾后「尚未做」再次只剩历史清单。本轮按规则换入口：逐函数对照原版 settings 的写盘路径
> （`write_settings_to` / `write_settings_file`），把「尚未做」重写为 2 条带 file:line / 原版对照 /
> 验收标准的具体待办（见文末「具体待办（P7-fq 重写）」），并取第 1 条落地。依赖检查：`go.mod` 已是
> **v0.4.8** = GitHub releases latest，无版本可升；只对照 `main`（`backup/main-pre-mygo` = v0.3.14
> 原版树），未翻历史提交。
>
> **偏离（P7-fq·写盘不原子、也从不刷新恢复副本）**：原版 `write_settings_to`（`:1473-1491`）先把
> 完整内容写进 `settings.json.backup` 并 `sync_directory`，再用 `write_settings_file` 把同一份内容写进
> `settings.json`；`write_settings_file`（`:1493-1506`）用同目录 `NamedTempFile` + `persist` 做原子
> 替换，并在 `persist` 前 `flush`/`sync_all`。所以原版的备份**永远等于最近一次成功保存的内容**，而
> `load_settings_from`（`:945-970`）在 primary 读不到或解析失败时会读它。**本构建（改前）**：
> `internal/settings/settings.go:474-485` 的 `Save` 直接 `os.WriteFile(path, data, 0o644)`——没有临时
> 文件、没有 rename、没有 fsync，也没有更新 `settings.json.backup`；`Load`（`:256-275`）虽然会回退
> 读备份，但备份只在首次迁移时写一次，之后一直是旧内容。一次崩溃/断电可以把 settings.json 截断成半截
> JSON，而恢复副本仍是几个月前的旧值。
>
> **落地**：
> - `internal/settings/write.go`（新）新增 `writeFileAtomic(path, data, mode)`：同目录
>   `os.CreateTemp(".settings-*.tmp")` → `Write` → `Sync` → `Close` → `Chmod` → `Rename`，最后
>   best-effort 同步目录（Unix 上 `dir.Sync()`，Windows 忽略错误）。
> - `internal/settings/settings.go` 的 `Save` 改为先 `writeFileAtomic(path+backupSuffix, data, 0o600)`
>   再 `writeFileAtomic(path, data, 0o600)`，逐字复刻原版「先备份、后主文件」的顺序与 0600 权限。
>
> **验收**：`internal/settings/write_test.go` 新增 `TestSaveWritesTheRecoveryCopyBesideThePrimary`
> （主文件与备份字节相同、权限 0600、目录里没有 `.settings-*.tmp` 残留）、
> `TestSaveRefreshesTheRecoveryCopyOnEveryWrite`（连写两次，把 primary 截断后 `Load` 恢复的是
> **第二次**的值，不是第一次）；`internal/settings/write_unix_test.go` 新增
> `TestWriteFileAtomicLeavesTheOldFileOnFailure`（目标目录不可写时返回错误且旧文件仍在）。
> 变异核查：把 `Save` 改回 `os.WriteFile` → 前两条立刻红。
>
> **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 全绿、
> `GOOS=linux/windows/darwin go build ./...` 三平台通过。
>
> **清单状态**：第 1 条落地；「具体待办（P7-fq 重写）」还剩第 2 条（电源动作要等命令的退出结果，
> 而不是只看到 spawn 成功）。下一轮取它。

> **上一轮：P7-fp 移除通知按 kind 说话——补上 P7-fe 记录的最后一个有意偏离
> （原版 `src/ExtensionsPanel.tsx:1286-1340`、`src/i18n.ts:545-547,1430-1432`）**
> ——P7-fe 收尾时把 system/package 的通知记成「两条都走统一的 `IntegrationRemoved`，留待后续单独
> 一轮」。P7-fo 之后「尚未做」只剩历史已结清清单、没有新的具体待办，本轮按规则换入口：重读 P7-fe
> 的残留记录，把清单重写为 2 条带 file:line / 原版对照 / 验收标准的具体待办（见文末「具体待办
> （P7-fp 重写）」），并全部落地。依赖检查：`go.mod` 已是 **v0.4.8** = GitHub releases latest，无版本
> 可升；release notes 的 `ui/editor` 用点已由 P7-ev 落地。只对照 `main`
> （`backup/main-pre-mygo` = v0.3.14 原版树），未翻历史提交。
>
> **偏离（P7-fp·移除通知只有一条通用句）**：原版 `confirmRemoval`（`:1286-1304`）对 system/package
> 直接调 `extensions_uninstall {removeData:false}`，成功后按 kind 二选一：system →
> `disconnectedNotice`（“Disconnected {name}; the external files and system tool were kept”）、
> package → `packageRemovedNotice`（“Removed extension package {name} from the list; external files
> were kept”）；`confirmComponentizedUninstall`（`:1320-1340`）对 custom → `customDeleted`
> （“Deleted custom integration {name}”）、npm → `uninstalledNotice`（“Uninstalled {name}”）。**本构建
> （改前）**：`internal/settingsui/settingsui.go:755,774`（shell）两条卸载路径都调
> `IntegrationRemoved`（“{name} was removed”/“{name} 已移除”）——一句自造的通用话，把确认条刚
> 承诺的 kind 信息（Delete/Uninstall/Disconnect/Remove package）在完成通知里丢掉了。
>
> **落地**：
> - `internal/i18n`：`Notifications` 删掉 `IntegrationRemoved`，新增 `IntegrationCustomDeleted` /
>   `IntegrationUninstalled` / `IntegrationDisconnected` / `IntegrationPackageRemoved`，en/zh 逐字取
>   `i18n.ts:545-547,1430-1432`；
> - `internal/settingsui`：`RemovalCustom` / `RemovalNPM` / `RemovalSystem` / `RemovalPackage` 四个
>   kind 常量导出；`Actions.UninstallIntegration` 与 `Actions.UninstallComponents` 各加一个 `kind` 参数，
>   两处调用点把 `integration.removalKind()` 传下去；
> - `internal/shell`：新增纯函数 `integrationRemovalNotice(copy, kind, name)` 与
>   `notifyIntegrationRemoval(kind, name)`，两条卸载路径共用它，按 kind 选原版的四条词。
>
> **验收**：`internal/i18n/i18n_test.go` 新增 `TestRemovalNoticesMatchTheOldDictionary`（en/zh 各四句
> 逐字）；`internal/shell/removalnotice_test.go` 新增 `TestIntegrationRemovalNoticeFollowsTheKind`
> （四 kind 纯映射 + 未知 kind 落 package）；`internal/settingsui/integrationremoval_test.go` 的
> `TestOnlyOwnedKindsOpenTheComponentizedDialog` 与 `TestAnNpmRowGoesStraightToTheComponentizedDialog`
> 分别钉住 system 行把 `RemovalSystem`、npm 行把 `RemovalNPM` 交给 shell。变异核查：把
> `integrationRemovalNotice` 换回 `c.IntegrationUninstalled` 一条 → shell 测试三条红；把
> `removalKind()` 固定返回 package → settingsui 两条红。
>
> **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 全绿、
> `GOOS=linux/windows/darwin go build ./...` 三平台通过。
>
> **清单状态**：「具体待办（P7-fp 重写）」2 条**全部结清、清单归零**。下一轮回到「对照原版排查
> 偏离点」，重新写实清单。

> **上一轮：P7-fo 滑块与命令别名的 180ms 去抖（原版 `src/hooks/useSettings.ts:149,396-530`）**
> ——P7-fn 收尾后「具体待办（P7-fn 重写）」只剩第 2 条（滑块/别名的 180ms 去抖），本轮取它落地，
> 清单归零。依赖检查：`go.mod` 已是 **v0.4.8** = GitHub releases latest（无版本可升，release notes
> 的 `ui/editor` 用点已由 P7-ev 落地）；只对照 `main`（`backup/main-pre-mygo` = v0.3.14 原版树），
> 未翻历史提交。
>
> **偏离（P7-fo·滑块与别名逐帧落盘）**：原版 `SETTINGS_DEBOUNCE_MS = 180`
> （`src/hooks/useSettings.ts:149`）；`changeOpacity`（`:396-412`，两个透明度滑块）、
> `changeFontSize`（`:444-459`）、`changeTerminalLineHeight`（`:486-501`）与 `changeCommandAlias`
> （`:515-530`）都先把新值上屏、清旧 timer，再 `setTimeout(..., 180)` 才 `persistSettings()`。
> **本构建（改前）**：`internal/settingsui/settingsui.go:2542/2548/2612/2621` 四处滑块每帧命中一次
> 就 `a.set(...)` → `Store.Update` → 同步 `Save`，拖动一次写几十遍文件；别名输入框只在
> `Submitted()`（回车）时写，既不是原版的每次编辑，也没有停顿合并。
>
> **落地**：
> - `internal/settings/store.go` 新增 `Preview`（改内存 + 通知监听器、不写盘）与 `Commit`（写一次、
>   失败回滚到上次确认快照并通知）；`Update` 成功时维护 `confirmed`，失败时回滚到 `confirmed`。
> - `internal/settingsui/debounce.go`（新）新增 `settingsDebounce = 180ms`、`previewSetting`/
>   `debounceAlias`/`expireSettingsWrites`/`flushSettingsWrites` 与 `Flush`；别名按命令键合并，
>   滑块共享同一个窗口；`View` 每帧 `expireSettingsWrites`，`set` 与 `selectPage` 先 flush。
> - `internal/shell/shell.go` 在离开设置面（`Open`）、隐藏窗口（`Hide`）与退出（`quit` 包装）
>   三条路调用 `Settings.Flush()`，最后一次拖动不会丢。
>
> **验收**：`internal/settings/store_test.go` 三条——`TestStorePreviewCoalescesUntilCommit`（三次
> preview 写 0 次、commit 写 1 次、二次 commit 不再写）、
> `TestStorePreviewRollsBackWhenCommitFails`（失败回滚 + 监听器先新后旧）、
> `TestStoreUpdateAfterPreviewCommitsTheWholeSnapshot`（preview + 立即 Update 合成一次写）；
> `internal/settingsui/debounce_test.go` 四条——`TestSliderPreviewDebouncesTheWrite`（窗口内文件不存在、
> 过窗后文件是最后一次值、空转帧不再写）、`TestCommandAliasDebouncesAndFlushesOnPageSwitch`（三次
> 编辑合并一次动作、切页 flush）、`TestCommandAliasFieldDebouncesTyping`（真敲键盘：一个字符不写，
> 过窗后才写）、`TestSetFlushesPendingPreview`。变异核查：把 `previewSetting` 换回
> `a.set` → 第一条立刻红；去掉 `selectPage` 的 flush → 别名第二条红。
>
> **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 全绿（1349 项）、
> `GOOS=linux/windows/darwin go build ./...` 三平台通过。
>
> **清单状态**：第 2 条（滑块/别名的 180ms 去抖）落地；「具体待办（P7-fn 重写）」2 条**全部结清、
> 清单归零**。下一轮回到「对照原版排查偏离点」，重新写实清单。

> **上一轮：P7-fn 设置写入被拒时回落到上次确认的快照（原版 `src/hooks/useSettings.ts:220-236`、
> `src/settings-persistence.ts:90-98` 的 `rollbackRejectedSettings`）**
> ——P7-fm 收尾后「尚未做」再次为空。本轮按惯例换入口：逐函数对照原版 `settings-persistence.ts`
> 与 `useSettings.ts` 的落盘/失败路径（本构建对应 `internal/settings/store.go` 的 `Store` 与
> `internal/settingsui/settingsui.go` 的 `set()`），把清单重写为 2 条带 file:line 的具体待办；
> 取第 1 条落地。依赖检查：`go.mod` 已是 **v0.4.8** = GitHub releases latest，无版本可升
> （release notes 的 `ui/editor` 用点已由 P7-ev 落地）；只对照 `main`
> （`backup/main-pre-mygo` = v0.3.14 原版树），未翻历史提交。
>
> **偏离（P7-fn·失败写不回落）**：原版 `commitSettings` 在 `save_settings` reject 后重读
> `get_settings`，再 `rollbackRejectedSettings(settingsRef.current, next, confirmed)`——只把
> **仍等于被拒值**的字段写回确认值，之后的编辑不被覆盖；被拒的改动不再以新值参与渲染或下一次读。
> 本构建（改前）的 `Store.Update` 先 `st.current = next` 再 `Save`，写失败后内存、监听器与后续
> 读都当成已提交，和 P7-av 的 5s 横幅各说各话。
>
> **落地**：`internal/settings/store.go` 的 `Update` 先克隆 `previous` 与 `next`，只在 `Save`
> 成功后提交 `st.current = next` 并通知监听器；`Save` 失败时保留 `previous`、写 `LastError`、
> 不通知、返回错误。`internal/settingsui/settingsui.go` / `savealert.go` 的注释同步成「回落后由
> 横幅报错」。
>
> **验收**：`internal/settings/store_test.go` 的 `TestStoreRollsBackWhenTheWriteFails`（失败后
> `equal(before, Snapshot())`、`LastError` 仍记、监听器 0 次）；`internal/settingsui/savealert_test.go`
> 的 `TestTheSaveFailureBannerAppearsAndExpires` 改断回落 + 横幅。变异核查：把 `Update` 改回
> 「失败也提交」→ 两条立即红。
>
> **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 全绿、
> `GOOS=linux/windows/darwin go build ./...` 三平台通过。
>
> **清单状态**：第 1 条（失败写回落）落地；第 2 条（滑块 180ms 去抖）由下一轮 P7-fo 结清。

> **上一轮：P7-fm 终端浮栏身份行报退出码（原版 `src/App.tsx:3716-3722`、`src/i18n.ts:170,1077`）**
> ——P7-fl 收尾后「具体待办（P7-fl 重写）」还剩第 3 条（浮栏身份行的退出码），本轮取它落地，清单归零。
> 依赖检查：`go list -m -versions github.com/egoist/mygo` 最新仍是 **v0.4.8**（与 `go.mod` 一致），
> 无版本可升、无新 API 用点；GitHub releases API 限流。只对照 `main`
> （`backup/main-pre-mygo` = v0.3.14 原版树），未翻历史提交。
>
> **偏离（P7-fm·浮栏身份行不报退出码）**：原版 `App.tsx:3716-3722` 的 `identityTitle` 在会话
> ended 时拼成 `` `${title} · ${t("terminal.headerExited", { code: exitCode ?? 0 })}` ``（词条
> `terminal.headerExited` = “Exited · code {code}”/“已退出 · 退出码 {code}”，`i18n.ts:170,1077`），
> 这串既是 `.terminal-bar__identity` 的 `title=` 也是它读出来的身份文本；它与 resident note 的
> `terminal.processExited`（“Process exited · code {code}”，`App.tsx:3891-3893`）是**两条词**。
> **本构建（改前）**：`internal/terminalui/terminalui.go` 的 `titleBar` 身份只画 `a.sessionTitle()`
> （空则 `copy.Title`），置灰点区分死活但**不报退出码**；`i18n.Terminal` 只有 `ProcessExited`，
> 退出码只出现在 `residentNote`。
>
> **落地**：
> - `internal/i18n`：`Terminal` 新增 `HeaderExited func(code int) string`（en/zh 逐字取原版两条词）。
> - `internal/terminalui/terminalui.go`：`titleBar` 先把身份串算出来（`sessionTitle()` 空则
>   `copy.Title`，`Session.Resident` 时追加 `" \u00b7 " + copy.HeaderExited(session.ExitCode)`），
>   同一句既是身份列的 `ui.Text`，也挂上身份行的 `.Label(...)`/`.Tooltip(...)`（原版的 `title=`）。
>
> **验收**：`internal/i18n/i18n_test.go` 的 `TestTerminalBarWordsMatchTheOldDictionary` 增四条
> （en/zh 的 headerExited 与 processExited 逐字，钉住两者是两条词）；`internal/terminalui/view_test.go`
> 新增 `TestTerminalBarIdentityNamesTheExitCode`（`ResidentAt(0,3)` → 帧里出现 “Exited · code 3”，
> `withTitle("zsh")` 后是 “zsh · Exited · code 3”，且 resident note 的 “Process exited · code 3”
> 仍在）。变异核查：把身份行的 `copy.HeaderExited` 换成 `copy.ProcessExited` → 新测试立刻红
> （`the identity line does not name the exit code "Exited · code 3"`）。
>
> **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 全绿（1342 项）、
> `GOOS=linux/windows/darwin go build ./...` 三平台通过。
>
> **清单状态**：第 3 条（浮栏身份行的退出码）落地；「具体待办（P7-fl 重写）」3 条**全部结清、
> 清单归零**。下一轮换入口（逐文件对照原版另一面 / 重新写实清单）。

> **上一轮：P7-fl 终端浮动栏两个控件的词与无障碍名（原版 `src/App.tsx:3764-3805`、`src/i18n.ts:146-150,1054-1058`）**
> ——P7-fk 收尾后「具体待办（P7-fj 重写）」3 条全部结清、清单归零；本轮按上节建议换入口：逐字段
> 对照**终端表面浮栏与输入区**（`main:src/App.tsx` 的 `.terminal-panel__actions` 与
> `.terminal-text-input`，本构建对应 `internal/terminalui/terminalui.go` 的 `titleBar`/`View`），
> 把清单重新写实为 **3 条带 file:line 的具体待办**（见文末「具体待办（P7-fl 重写）」），取第 1·2 条
> 落地。依赖检查：`go list -m -versions github.com/egoist/mygo` 最新仍是 **v0.4.8**（与 `go.mod`
> 一致），无版本可升、无新 API 用点；GitHub releases API 限流。只对照 `main`
> （`backup/main-pre-mygo` = v0.3.14 原版树），未翻历史提交。
>
> **偏离（P7-fl·浮栏两个控件的词）**：原版浮栏的两个控件把**用户自己的绑定**写进 tooltip——
> `App.tsx:3771` 的 popout `title={t("terminal.openInTerminalHint", { shortcut:
> formatShortcut(shortcuts.open_external_terminal) })}`、`App.tsx:3781` 的关闭键
> `title={t("terminal.newCommandHint", { shortcut: formatShortcut(shortcuts.new_command) })}`，
> 而 aria-label 是 `terminal.openInTerminal` = “Continue in System Terminal” / `terminal.newCommand`
> = “New command”（`i18n.ts:146-149,1054-1057`）。**本构建（改前）**：
> `internal/i18n/copy.go:942,948` 的 `NewCommandHint` 是自由句 “Back to the search field”、
> `OpenExternalHint` 是 “Opens the system's own terminal window”，`OpenExternal` 的词是
> “Open in terminal”；`internal/terminalui/terminalui.go:1078,1081` 直接把这两个常量当 tooltip 传，
> 绑定连读都没读；zh 侧 `NewCommand` 还是“新命令”而不是原版的“新建命令”。
>
> **偏离（P7-fl·终端视图没有无障碍名）**：原版 `App.tsx:3805` 的终端文本输入带
> `aria-label={t("terminal.input")}`（`i18n.ts:150,1058`）；**本构建（改前）**
> `internal/terminalui/terminalui.go:776` 的 `terminal.View(...).Bind(&a.Focus)` 没有 `Label`，
> `i18n.Terminal` 也没有 `Input` 字段——聚焦到一个无名区域，辅助技术读不出它是什么。
>
> **落地**：
> - `internal/i18n`：`Terminal.NewCommandHint`/`OpenExternalHint` 变成 `func(shortcut string) string`
>   （逐字“New command ({shortcut})”/“Continue in System Terminal ({shortcut})”，zh 用全角括号）；
>   `OpenExternal` 改回原版词、`NewCommand` 的 zh 修正为“新建命令”；新增 `Terminal.Input`。
> - `internal/terminalui/terminalui.go` 新增纯函数 `barHints(copy, settings.Settings)`（从
>   `settings.Shortcut` 读 `new_command`/`open_external_terminal` 绑定，过 `shortcuts.Display`——
>   即原版的 `formatShortcut`），`titleBar` 用它；`View` 给终端视图链上 `.Label(copy.Input)`。
>
> **验收**：`internal/i18n/i18n_test.go` 新增 `TestTerminalBarWordsMatchTheOldDictionary`（en/zh
> 十句逐字）；`internal/terminalui/view_test.go` 新增 `TestTerminalBarHintsFollowTheUserBinding`
> （把两条绑定改掉 → 两条 tooltip 跟着变，且与默认不同）与 `TestTerminalInputCarriesItsAccessibleName`
> （终端视图找得到 “Terminal input” 这个标签），`TestTerminalBarButtonsCarryTheirHints` 的期望值改成
> 从绑定算出。变异核查：把 `barHints` 的 open-external 绑定换成常量 `"Ctrl+N"` → 第一条立刻红
> （`want it to name "Ctrl+Alt+J"`）；摘掉 `.Label(copy.Input)` → 第二条立刻红（`no accessible name
> "Terminal input"`）。
>
> **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 全绿（1336 项）、
> `GOOS=linux/windows/darwin go build ./...` 三平台通过。
>
> **清单状态**：第 1 条（浮栏两个控件的词）与第 2 条（终端视图的无障碍名）落地；
> 「具体待办（P7-fl 重写）」还剩第 3 条（浮栏身份行的退出码），是下一轮最高优先级。

> **上一轮：P7-fk 自定义快捷键的 `conflict` 拒收与两个词（原版 `src-tauri/src/commands/config.rs:1666-1718`、`src/settings/ShortcutsPage.tsx:293-296`、`src/i18n.ts:412-413,1299-1300`）**
> ——P7-fj 收尾后「具体待办（P7-fj 重写）」还剩两条，本轮把第 2 条（撞车判 `conflict`）与第 3 条
> （拒收原因收成原版两个 token）一起结清，清单归零。依赖检查：`go list -m -versions
> github.com/egoist/mygo` 最新仍是 **v0.4.8**（与 `go.mod` 一致），无版本可升、无新 API 用点；
> GitHub releases API 限流。只对照 `main`（`backup/main-pre-mygo` = v0.3.14 原版树），未翻历史提交。
>
> **偏离（P7-fk·撞车判据、拒收原因与本地化）**：原版 `set_custom_shortcuts` 的 `reserved` 取
> `resolved_shortcuts(&settings)` 的全部值，逐条与 `shortcut_binding_eq` 比对，撞上内置组合即
> `CustomShortcutRejection { reason: "conflict" }` 并跳过；`resolved_shortcuts` 内部的
> `insert_shortcut_if_available` 还会用 `shortcut_conflicts` 处理 `select_result` **家族**
> （同一修饰键下的任一数字，含 `0`）——所以 `⌘⇧7` 与家族头 `⌘⇧1` 是一把钥匙。OS 拒收（`register` 返回
> `Err`）则记 `reason: "occupied"`，且**只把注册成功的几条**写回磁盘（`:1715-1717`）。前端
> `ShortcutsPage.tsx:293-296` 只认这两个 token 选词（`occupied` → `customShortcutsOccupied`，
> 其余 → `customShortcutsConflict`）。**本构建（改前）**：`internal/shell/custom.go` 的
> `ApplyCustomShortcuts` 只挡 `accelerator == a.summonKey`，不比对内置动作；`setCustomShortcuts`
> 无条件把整张表落盘（被拒的键会留在磁盘上下次开机重试）；原因字段塞的是英文自由句
> （`err.Error()` 直接上图），且 `internal/i18n` 只有一句「Could not bind X: <自由句>」——**中文侧
> 连映射都没有**，页面上会直接印出 `conflict`/`occupied` 这种 token。
>
> **落地**：
> - `internal/settings/shortcutmap.go` 新增 `CustomShortcutConflict`（逐 action 过 `ShortcutsOf`
>   并以 `shortcuts.Equal` 等值比对；`select_result` 走家族判据；`toggle_window` 留给 shell 自己
>   的 summon 检查）；`internal/shortcuts/shortcuts.go` 新增 `SameResultFamily`（同修饰键 + 单个数字）。
> - `internal/shell/custom.go` 的注册收敛成唯一一条 `applyCustomShortcuts`：先释放旧键，再
>   冲突/占用逐条判，返回（注册成功的 entries, rejections）；`ApplyCustomShortcuts` 与
>   `setCustomShortcuts` 都走它，避免两条路径各写一份规则。`setCustomShortcuts` 只写回注册成功的
>   列表，并在写盘前记 `customSignature`，免得 Store 监听把同一批键再 claim 一次。
> - `internal/settingsui/settingsui.go` 新增 `CustomShortcutReasonConflict`/`…Occupied` 两个 token
>   常量（`:688`）；`internal/i18n/copy.go` 的 `shortcutsRejectedEN`/`…ZH` 按 token 选词，逐字取
>   `i18n.ts:412-413,1299-1300`。
>
> **验收**：`internal/settings/shortcutmap_test.go` 的 `TestCustomShortcutConflict`（等值/异拼写
> `Command+W`/家族任一数字均冲突，别的修饰键、空闲键、toggle 键均不冲突）；`internal/shell/shell_test.go`
> 的 `TestCustomShortcutsRefuseConflictsAndKeepOnlyRegistered`（`Cmd+W` → `conflict` 且不注册不落盘；
> `Cmd+Shift+T` → `occupied` 且不落盘；空闲键注册且落盘，内存与磁盘只留这一条）；
> `internal/settingsui/view_test.go` 的 `TestCustomShortcutRejectionWording`（en 两词）与
> `TestCustomShortcutsPage` 的 `occupied` 那句；`internal/i18n/i18n_test.go` 的
> `TestCustomShortcutRejectionWordsMatchTheOldDictionary`（en/zh 各两句，并钉住未知 token 原样透传）。
> 变异核查：把 zh 分支退回直通 → 该 i18n 测试立刻红（`无法绑定 Cmd+J：conflict`）。
>
> **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 全绿（1333 项）、
> `GOOS=linux/windows/darwin go build ./...` 三平台通过。
>
> **清单状态**：第 2 条（撞车判 `conflict`）与第 3 条（两个 word）落地；「具体待办（P7-fj 重写）」
> 3 条**全部结清、清单归零**。下轮回到「对照原版排查偏离点」，重新写实清单。

> **上一轮：P7-fj 自定义快捷键的键在读写时补上原版的形状校验（原版 `src-tauri/src/commands/config.rs:1385-1404` 的 `normalize_custom_shortcuts` + `:1408-1434` 的 `normalize_shortcut`）**
> ——P7-fi 收尾后「具体待办（P7-fh 重写）」归零。本轮按上节建议换入口：逐文件对照原版的
> **自定义快捷键落盘/读盘路径**（`main:src/custom-shortcuts.ts` 与 `config.rs` 的三个函数），把
> 「尚未做」重新写实为 3 条带 file:line 的具体待办（下方「具体待办（P7-fj 重写）」），并取第 1 条
> 落地。依赖检查：`go list -m -versions github.com/egoist/mygo` 最新仍是 **v0.4.8**（与 `go.mod`
> 一致），无版本可升、无新 API 用点；GitHub releases API 限流。只对照 `main`
> （`backup/main-pre-mygo` = v0.3.14 原版树），未翻历史提交。
>
> **偏离（P7-fj·自定义快捷键的键没有形状校验）**：原版后端 `normalize_custom_shortcuts` 把每个
> `key` 过一遍 `normalize_shortcut`——任一段 `trim` 后为空、修饰键重复、两个键、没有键都是 `None`
> ——再用 `.filter(|key| key.contains('+'))` 丢掉裸键，并按规范拼写落盘。**本构建（改前）**：
> `internal/settings/customshortcuts.go:87-107` 的 `NormalizeCustomShortcuts` 只 `TrimSpace` + 去重，
> `"F5"`/`"Ctrl++A"`/`"Ctrl+Ctrl+A"`/`"Ctrl+A+B"`/`"+"` 全部原样落盘；`internal/shortcuts`
> 的宽松 `Normalize`（`:97-128`）对空段 `continue`、对重复修饰键静默吞并，所以连那条路也修不出
> 原版判据（`"Ctrl++A"` 被修成 `"Ctrl+A"` 而不是拒收）。一条存下来的自定义快捷键会变成**全局**
> 系统热键，形状必须在磁盘上就是完整的，不能只在录制器里完整。
>
> **落地**：`internal/shortcuts/shortcuts.go` 新增 `NormalizeBinding`（与 `Normalize` 共用修饰键/键名
> 表与 `join`，但空段/重复修饰键/两键/无修饰键一律拒绝、且要有至少一个修饰键）；
> `internal/settings/customshortcuts.go` 的 `NormalizeCustomShortcuts` 改走它（`CustomShortcutsOf`
> 读盘与 `SetCustomShortcuts` 落盘共用，两处都受用）。
>
> **验收**：`internal/shortcuts/shortcuts_test.go` 的 `TestNormalizeBinding`（六例收、十一例拒，并钉
> 住宽松 `Normalize("Ctrl++A") == "Ctrl+A"` 而严格路径拒绝）；
> `internal/settings/customshortcuts_test.go` 的 `TestCustomShortcutsDropShapesTheRecorderCouldNotMake`
> （八条入、两条留，键与动作都规范/裁剪）与 `TestCustomShortcutsOfDropsAHandEditedBadKey`
> （手改文件里的裸键读盘即丢）。变异核查：把 `NormalizeBinding` 换回 `strings.TrimSpace` → 新三条
> 用例全红。
>
> **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 全绿、
> `GOOS=linux/windows/darwin go build ./...` 三平台通过。
>
> **清单状态**：第 1 条（P7-fj）落地；「具体待办（P7-fj 重写）」还剩第 2·3 条，均由下一轮 P7-fk
> （撞车判 `conflict`、拒收原因收成原版两个 token）结清。

> **上一轮：P7-fi 删除确认条也走同一条横条，并带上原版的 error 底色；两条确认条共用一个 Cancel 词
> （原版 `src/extensions/RemovalConfirmation.tsx:38-44`、`DisableConfirmation.tsx:60-68`、`i18n.ts:554`）**
> ——P7-fh 收尾后「具体待办（P7-fh 重写）」还剩 3 条，本轮取文档标注的最高优先级第 2 条（删除确认条
> 回到同一条横条），顺带把第 3 条（共享一个 Cancel 词）一起结清，清单归零。依赖检查：
> `go list -m -versions github.com/egoist/mygo` 最新仍是 **v0.4.8**（与 `go.mod` 一致），无版本可升、
> 无新 API 用点；GitHub releases API 限流。只对照 `main`（`backup/main-pre-mygo` = v0.3.14 原版树），
> 未翻历史提交。
>
> **偏离（P7-fi·删除确认条的形状、底色与句子）**：原版 `RemovalConfirmation.tsx:38-44` 与停用条是
> **同一条** `.extensions-notice .extension-discard-bar`，只多两样：`:39` 的
> `extensions-notice--error`（CSS `src/styles/extensions.css:169-173`：底色仍是 `--glass-control`，
> 但 `color: --text-warning`、`box-shadow: inset 0 0 0 1px --text-warning`），以及 `:43-44` 的
> `extensions-action-button--danger`（`extensions.css:1364-1367` 的 `color: --text-warning`）确认键。
> 句子只有 `:41` 一个 `<span>`：`{removalTitle}` **加上**（仅当 `generatedCustom`）
> `deleteCustomWarning` —— `removalTextKey` 虽然也能取 `Description`，但这条横条**从不读它**，原版
> i18n 里那几条 `*Description` 在整棵原版树里没有任何读者。**本构建（改前）**：
> `internal/settingsui/settingsui.go` 的 `removeConfirmation` 是**纵向卡片**（加粗标题一行 + 弱化正文
> 一行 + 按钮行，P7-cf 当年照抄的是「当时的」停用条），正文画的是 `RemoveDetail` 系——即原版从不显示
> 的那一半文案；底色与按钮都是普通控制面色，没有任何「这是一次损失」的信号。
>
> **落地**：
> - `internal/settingsui/settingsui.go` 新增共享的 `noticeBand(c, danger, sentence, answers)`：横条
>   （`ui.Row` + 居中的 `mustGlyph("alert")` 15px + 一个 `Grow(1)` 的 `ui.Text` + 同排 answers），
>   内边距/外边距/圆角/底色照 `.extension-discard-bar`；`danger` 为真时墨色与描边换成
>   `t.Warning`（原版 `extensions-notice--error`）。`disableConfirmation` 与 `removeConfirmation`
>   都改用它（后者 `danger=true`，确认键 `TextColor(t.Danger)`，即原版的 `--danger` 键）。
> - 新增 `Integration.removalSentence(copy)`：照 `RemovalConfirmation.tsx:41` 把标题与（仅生成的）
>   `RemoveCustomWarning` 连成一句；`removalWords` 的第二个返回值保留为「原版从不画的那半文案」并在
>   注释里写明。
> - `internal/i18n`：`DisableCancel` 与 `RemoveCancel` 合并为一个 `IntegrationsCancel`
>   （原版两条确认条都取同一个 `settings.extensions.cancel`）。
>
> **验收**：`internal/settingsui/disablebar_test.go` 新增两条——
> `TestRemovalBarIsTheSameNoticeBand`（整句就是 `Remove V Tools?` 一个节点、原版从不显示的
> `IntegrationsRemovePackageDetail` 不出现、Cancel 在整句右侧且位置点在它右边能确认到动作）；
> `TestTheRemovalBandWearsTheErrorTint`（`bandInkWarmth` 量整句方框的 R−B 均值：停用条 −0.64、删除条
> +11.86，阈值 +8；并断言两条横条都取同一个 `IntegrationsCancel`）。
> `view_test.go` 的 `TestGeneratedIntegrationRemovalBar` 改成原版的一句
> `Delete Ripgrep? This cannot be undone from Floter. …`，并钉住旧正文不再出现。
> 变异核查：让 `noticeBand` 忽略 `danger` → 第二条立刻红（−0.34 vs −0.64）；把整句换回 `title + detail`
> → 第一条红。
>
> **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 全绿（1324 项）、
> `GOOS=linux/windows/darwin go build ./...` 三平台通过。
>
> **清单状态**：第 2 条（P7-fi·删除确认条）与第 3 条（共享 Cancel 词）落地；
> 「具体待办（P7-fh 重写）」3 条**全部结清、清单归零**。下轮回到「对照原版排查偏离点」，重新写实清单。

> **上一轮：P7-fh 停用确认条回到原版的横条与词（原版 `src/extensions/DisableConfirmation.tsx:60-68`）**
> ——P7-fg 收尾后「具体待办（P7-ff 重写）」两条全部结清、清单归零；本轮按审计入口
> `main:src/extensions/DisableConfirmation.tsx`（`src/extensions/` 下最后一个尚未逐字段对照的组件）
> 逐字段比对，把清单重写为 3 条带 file:line 的具体待办（见文末「具体待办（P7-fh 重写）」），取第 1 条
> 落地。依赖检查：`go list -m -versions github.com/egoist/mygo` 与 GitHub releases API 都是
> **v0.4.8**（与 `go.mod` 一致），无版本可升、无新 API 用点；只对照 `main`
> （`backup/main-pre-mygo` = v0.3.14 原版树），未翻历史提交。
>
> **偏离（P7-fh·停用确认条的形状与词）**：原版 `DisableConfirmation.tsx` 是一条**横向通知条**
> （`:60` 的 `className="extensions-notice extension-discard-bar"`）：一个 `AlertCircle` 15px 图标打头
> （`:61`），接着**一个** `<span>` 把标题与后果连成一句（`:63-64` 的
> `{disableTitle} {disableDescription}`，整句都是 `--text-secondary`，标题并不加粗），右边同排 Cancel
> （`:66`，`settings.extensions.cancel`）与 confirm（`:67-68`，`settings.extensions.disable` =
> “Disable integration”/“禁用集成”，带 `PowerOff` 14px 图标）；description 逐字 “Floter will stop any
> running commands immediately. This only marks the integration as disabled — its files and data are
> kept, and you can re-enable it at any time.”（`i18n.ts:504`）。**本构建（改前）**：
> `internal/settingsui/settingsui.go` 的 `disableConfirmation` 是一个**纵向卡片**（`ui.Column`：加粗
> 标题一行 + 弱化描述一行 + 按钮行），没有前导图标，confirm 的词是自造的 “Disable”/“停用”，英文
> description 是意译（“Floter stops any running command at once…”）。
>
> **落地**：
> - `internal/settingsui/settingsui.go` 的 `disableConfirmation` 改成横条：`ui.Row` + 居中的
>   `mustGlyph("alert")`（15px，`TextSecondary`）+ 一个 `ui.Text` 把 `DisableTitle(name)` 与
>   `DisableDescription` 用空格连成一句（`Grow(1)`，`TextSecondary`）+ 同排 Cancel/Disable 两个按钮；
>   内边距/外边距按 `.extension-discard-bar`（`padding: 8px 10px`、`margin: 0 12px`）。
> - `internal/i18n`：`DisableDescription` 改成原版逐字英文；`DisableConfirm` 改成
>   `settings.extensions.disable` = “Disable integration”/“禁用集成”。
>
> **验收**：`internal/settingsui/disablebar_test.go` 新增 `TestDisableBarIsOneNoticeSentence`——
> 打开停用确认后，整句（标题 + 逐字后果）是**一个**文本节点（旧卡片的两个节点拼不出这句）、
> `Find("Disable integration")` 命中（旧词 “Disable” 不命中）、两个按钮与整句在**同一行且在其右侧**、
> 点 confirm 落到 `SetIntegrationEnabled(id, false)`。`view_test.go:1010` 的
> `Click("Disable")` 随之改成 `Click("Disable integration")`；`TestDisableAsksFirst` 与
> `TestIntegrationsPage` 的 `HasText("Disable V Tools?")` 是子串匹配，保持绿。变异核查：把
> `disableConfirmation` 改回 `ui.Column`（标题与描述两个 `ui.Text`）→ 第一条立刻红（整句节点不存在）。
>
> **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 全绿（1322 项）、
> `GOOS=linux/windows/darwin go build ./...` 三平台通过。
>
> **清单状态**：第 1 条（P7-fh）落地；「具体待办（P7-fh 重写）」现有 3 条带 file:line 的具体待办，
> 第 2 条（删除确认条也走同一条横条）是下轮最高优先级。

> **上一轮：P7-fg 集成行的 Run 按钮——无参数直接跑、有参数展开表单、Cancel 收起
> （原版 `src/extensions/ExtensionRow.tsx:381-399,459-471`、`src/ExtensionsPanel.tsx:1712-1744`、
> `src/extensions/RunParamForm.tsx:96-99`）**
> ——P7-ff 把「尚未做」重写为 2 条带 file:line 的具体待办并落地第 1 条（表单的框），本轮取剩下的最高
> 优先级第 2 条（Run 披露与 Cancel）。依赖检查：`go list -m -versions github.com/egoist/mygo` 最新仍是
> **v0.4.8**（与 `go.mod` 一致），无版本可升、无新 API 用点；GitHub releases API 限流。只对照 `main`
> （`backup/main-pre-mygo` = v0.3.14 原版树），未翻历史提交。
>
> **偏离（P7-fg·Run 披露）**：原版 connected 行有一个 `extensions-icon-button--row` 的 Run 按钮
> （`Play` 图标，`aria-expanded={hasParams ? runFormOpen : undefined}`，不可运行时 disabled 且 title 说明，
> `ExtensionRow.tsx:381-399`）；点击时 `openRunForm`（有参数 → 打开表单，`:1712-1727`）/ `runExtension`
> （无参数 → 直接跑，`:1740-1744`）；表单里 `RunParamForm.tsx:96-99` 有 Cancel（`customRunCancel`）。
> **本构建（改前）**：`internal/settingsui/settingsui.go:1723-1725` 的集成行**无条件**调 `runForm`，于是
> 有参数的集成表单**常驻**、没有关闭入口；`runForm` 的动作行只有 Run，没有 Cancel；无参数的已连接集成
> 在设置页根本无法手动运行。
>
> **落地**：
> - `internal/settingsui/settingsui.go`：`App` 新增 `runOpen map[string]bool`（仿 `outputOpen`）；
>   `runForm` 开头加 `if !a.runOpen[integration.ID] { return }`；动作行加 Cancel
>   （`copy.RunCancel` → 关 `runOpen` 并清 `runError`）；
> - 集成行（enable 动作区）加 Run 按钮：`a.Actions.RunWithParams != nil` 时，`integration.Running &&
>   !integration.Broken` 可运行——`len(integration.Params) > 0` → 切换 `runOpen[id]`（并清 `runError`）；
>   否则直接 `a.Actions.RunWithParams(id, nil)`；不可运行的行保留按钮但 `Disabled(true)` +
>   `Tooltip(copy.RunActionUnavailable)`；
> - `internal/i18n` 新增 `RunAction`/`RunActionUnavailable`/`RunCancel`（en/zh 逐字取
>   `i18n.ts:738-739,782`）。
>
> **验收**：`internal/settingsui/runform_test.go` 新增 `TestRunButtonRunsOrOpensTheForm` 两条子测——
> 「no inputs run at once」（点 Run → `RunWithParams("plain", nil)`）、「inputs open the form and Cancel
> folds it」（初始不画表单标题/确认按钮 → 点 Run 后画 → 点 Cancel 收起）；`view_test.go` 三条 run 表单
> 测试与 `runform_test.go` 的 `TestRunFormDrawsTheReferenceFrame` 改为先点 Run 展开。变异核查：摘掉
> `runForm` 的 `runOpen` 挡板 → 「初始不画表单」条立刻红。
>
> **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 全绿（1321 项）、
> `GOOS=linux/windows/darwin go build ./...` 三平台通过。
>
> **清单状态**：第 2 条（P7-fg）落地；「具体待办（P7-ff 重写）」两条带 file:line 的具体待办
> **全部结清**。下轮回到「对照原版排查偏离点」，重新写实清单。

> **上一轮：P7-ff 运行参数表单回到原版的框（头部/提示/种类标签/必填标记/注入说明/输出路由）
> （原版 `src/extensions/RunParamForm.tsx`）**
> ——P7-fe 收尾后「尚未做（P7-fb 重写）」4 条全部结清、清单归零；本轮按审计入口
> `main:src/extensions/RunParamForm.tsx` + `ExtensionRow.tsx` 的 Run 披露逐字段对照，把清单重写为
> **2 条带 file:line 的具体待办**（见文末「具体待办（P7-ff 重写）」），取第 1 条落地。依赖检查：
> `go list -m -versions github.com/egoist/mygo` 最新仍是 **v0.4.8**（与 `go.mod` 一致），无版本可升、
> 无新 API 用点；GitHub releases API 限流。只对照 `main`（`backup/main-pre-mygo` = v0.3.14 原版树），
> 未翻历史提交。
>
> **偏离（P7-ff·运行参数表单的框）**：原版 `RunParamForm.tsx` 的表单是一个有名字的块——顶部
> `customRunValues`（“Run parameters”）标题 + `customRunValuesHint` 提示（`:40-43`）；每个输入行的第二
> 行是参数的**种类**词 `customParamType.{kind}`，必填时旁边一个 `<em>` `customParamRequired`
> （`:53-60`）；底部 `customParamInjectionNote` 注入说明（`:131`）+ 按 manifest 的 `output` 二选一的
> `customRunOutputHintTerminal`/`Background`（`:136-138`）。**本构建（改前）**：
> `internal/settingsui/settingsui.go:4165` 的 `runForm` 直接画卡片行，没有标题/提示；行的第二行用的是
> **placeholder** + `ConfigRequired`（“required”）而不是种类词 + “Required”；底部只有一句自造的
> `RunOutputHint`（“The values are passed as separate arguments.”），既不是注入说明也不是路由提示。
>
> **落地**：
> - `internal/i18n` 新增 `RunValues`/`RunValuesHint`/`RunRequired`/`RunInjectionNote`/
>   `RunOutputHintTerminal`/`RunOutputHintBackground`（en/zh 逐字取 `i18n.ts:725,772-780`），删掉变死的
>   `RunOutputHint`；
> - `internal/settingsui/settingsui.go` 的 `runForm`：卡片前加标题+提示（`RunValues`/`RunValuesHint`）；
>   每行 `sublabel` 改读新 `runParamKindLabel(copy, param.Kind)`（复用已有的 `ParamsKind*` 五词）并按
>   `RunRequired` 追加必填标记，placeholder 仍只作输入框的 `Placeholder`；卡片后加 `RunInjectionNote`
>   与按 `integration.OutputMode` 二选一的输出路由提示；
> - `Integration` 新增 `OutputMode` 字段，`internal/shell/shell.go` 的 `integrationList` 用
>   `integration.Manifest.Route()` 投影。
>
> **验收**：`internal/settingsui/runform_test.go` 两条——
> `TestRunParamKindLabelNamesEveryKind`（五个 kind + 空/未知退化到 Text，en/zh 各一）；
> `TestRunFormDrawsTheReferenceFrame`（background/terminal 两个子测：标题/提示/注入说明/本档路由提示
> 都在、**另一档**路由提示不出现、必填 text 行的第二行逐字 “Text  ·  Required”、toggle 行的种类词
> 在）；`internal/shell/runroute_test.go` 的 `TestTheRowCarriesTheManifestsRunRoute`（声明
> `output: terminal` → `RouteTerminal`，不声明 → `RouteBackground`）。变异核查：把 `sublabel` 换回
> `param.Placeholder`、把标题换成 “MUTATED” → 第一条像素测三处立刻红（种类词/必填标记/标题缺失）。
>
> **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 全绿、
> `GOOS=linux/windows/darwin go build ./...` 三平台通过。
>
> **清单状态**：第 1 条（P7-ff）落地；「具体待办（P7-ff 重写）」现有 2 条带 file:line 的具体待办，
> 第 2 条（Run 披露与 Cancel，见文末）是下轮最高优先级。

> **最新一轮：P7-fe 组件化卸载对话框只给 npm/custom，其余 kind 回到原版的「先问一次再删」
> （原版 `src/ExtensionsPanel.tsx:1286-1323` 的 `confirmRemoval`）**
> ——上一轮 P7-fb 把「尚未做」重写为 4 条带 file:line 的具体待办并落地第 1–3 条，本轮取剩下的
> 最高优先级第 4 条。依赖检查：`go list -m -versions github.com/egoist/mygo` 最新仍是 **v0.4.8**
> （与 `go.mod` 一致），无版本可升、无新 API 用点；GitHub releases API 限流。只对照 `main`
> （`backup/main-pre-mygo` = v0.3.14 原版树），未翻历史提交。
>
> **偏离（P7-fe·卸载路由）**：原版所有 kind 的卸载按钮都先走 `RemovalConfirmation` 条
> （`removalTarget`），`confirmRemoval`（`ExtensionsPanel.tsx:1286-1304`）再分叉：npm/custom →
> `setUninstallDialogTarget`（组件化对话框），system/package → 直接
> `extensions_uninstall {removeData:false}`；**本构建（改前）**：`internal/settingsui/settingsui.go`
> 的集成行只要 `Actions.UninstallComponents != nil`（shell `shell.go:735,761` 两个入口都接线）
> 就对**任何** kind 直接 `<verb>…` 展开组件化对话框（`uninstall.go:25`），把 system/package 也
> 当成了「可挑数据删除」的种类；`removeConfirmation`/`requestRemove` 因此成了死代码。
>
> **落地**：
> - `internal/settingsui/settingsui.go` 新增 `Integration.componentizedRemoval()`——`removalKind()`
>   为 `npm`/`custom` 时为真；集成行的动作分支改成三分支：npm/custom 且接了组件化动作 → 开对话框；
>   否则若接了普通卸载动作 → `requestRemove`（`removeConfirmation` 条 → `UninstallIntegration`）；
>   仅接组件化动作（部分测试夹具）→ 回退到组件化门。
>
> **验收**：`internal/settingsui/integrationremoval_test.go` 两条——
> `TestOnlyOwnedKindsTakeTheComponentizedRemoval`（四 kind 的纯映射：custom/npm 真、system/package 假）、
> `TestOnlyOwnedKindsOpenTheComponentizedDialog`（system 行点 Disconnect 先画自己的
> `IntegrationsDisconnectTitle` 条、四行组件化文案一个都不出现、确认前不调任何动作；位置点 Cancel
> 右侧的 danger 后只调 `UninstallIntegration`、不调 `UninstallComponents`）。变异核查：把
> `componentizedRemoval()` 改成 `return true` → 两条红（纯映射红、system 行变成“Disconnect…”直接开
> 对话框而没有「Disconnect」按钮可点）。既有 `TestEachIntegrationCardShowsItsOwnRemovalVerb` /
> `TestEachRemovalConfirmationAsksItsOwnQuestionOnScreen` / `TestGeneratedIntegrationRemovalBar`
> 保持绿（仅接 `UninstallComponents` 的夹具走回退分支，四 kind 的动词仍照画）。
>
> **有意偏离（如实记录）**：原版 system/package 删完的通知是 `disconnectedNotice`
> (`Disconnected`)/`packageRemovedNotice`，本构建两条都走 shell 的 `IntegrationRemoved(name)`
> （`shell.go:771`）——`UninstallIntegration(id, name)` 的签名不带 kind，改它要动跨包接口，
> 留待后续单独一轮。
>
> **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 全绿
> （`internal/shell` 的 `TestCapturedRunNotifiesOnlyWhenHidden` 首次跑偶发一红、单跑与复跑均绿，
> 与本轮改动无关）、`GOOS=linux/windows/darwin go build ./...` 三平台通过。
>
> **清单状态**：第 4 条（P7-fe）落地，「尚未做（P7-fb 重写）」的 4 条带 file:line 的具体待办
> **全部结清**。下轮回到「对照原版排查偏离点」，重新写实清单。

> **上一轮：P7-fb 组件化卸载对话框补回原版的第一行「程序文件」，标题改回原版的
> `uninstallComponentsTitle`，动作顺序改回 Cancel 在前
> （原版 `src/extensions/ComponentizedUninstallDialog.tsx:70-72,84-98,119-141`）**
> ——上一轮 P7-fa 之后「尚未做（P7-ew 重写）」归零，本轮按审计入口
> `main:src/extensions/ComponentizedUninstallDialog.tsx` 逐字段比对，把清单重写为 4 条带
> file:line 的具体待办（见「尚未做」），取第 1–3 条落地。依赖检查：
> `go list -m -versions github.com/egoist/mygo` 最新仍是 **v0.4.8**（与 `go.mod` 一致），
> 无版本可升、无新 API 用点；GitHub releases API 限流。只对照 `main`
> （`backup/main-pre-mygo` = v0.3.14 原版树），未翻历史提交。
>
> **偏离（P7-fb·组件化卸载对话框）**：原版对话框有四行（`ComponentizedUninstallDialog.tsx:84-115`）：
> program（checked+disabled）、host config、tool data、artifacts；标题是
> `uninstallComponentsTitle` =“Uninstall {name}”（`:70-72`）；动作行 Cancel 在前、danger 在后
> （`:119-141`）。**本构建（改前）**：`internal/settingsui/uninstall.go` 只画三行（P3-p 的压缩），
> 标题用自造句“Uninstall this integration” + `"  ·  "` + 名字，动作先 confirm 后 Cancel。
>
> **落地**：
> - `internal/i18n` 新增 `UninstallProgram`/`UninstallProgramHint`（en/zh 逐字取 `i18n.ts:532-533`）；
>   `IntegrationsUninstallTitle` 由自造 string 改成 `func(name string) string`，en/zh 逐字
>   “Uninstall {name}”/“卸载 {name}”（`internal/i18n/i18n.go:525`、`copy.go` en/zh）。
> - `internal/settingsui/uninstall.go` 新增 `uninstallPart`/`uninstallParts` 值函数（四行、program
>   行 `Fixed`）与 `uninstallPartRow`（fixed 行 `ui.Checkbox(...).Label(label).Disabled(true)`，
>   可选行继续写回 draft 字段）；`uninstallDialog` 改用它、标题改读新函数、动作行 Cancel 先。
>
> **验收**：`internal/settingsui/uninstall_test.go` 三条——
> `TestComponentizedUninstallListsAllFourComponents`（四行、program 固定无 Value、其余三行指
> draft、两语言不同）、`TestComponentizedUninstallDrawsTheReferenceRows`（标题逐字
> “Uninstall NPM Tool”、自造句不出现、四行按 program/host/tool/artifacts 顺序、四条 hint 逐字、
> 取消勾选 host 只影响该字段的 payload）、`TestComponentizedUninstallKeepsCancelFirst`（Cancel 的
> x < confirm 的 x）。变异核查：抽掉 `uninstallParts` 的 program 行 → 前两条红；把 confirm 提到
> 前面 → 第三条红。
>
> **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 全绿、
> `GOOS=linux/windows/darwin go build ./...` 三平台通过。
>
> **清单状态**：第 1–3 条（P7-fb）落地；「尚未做」清单现有 4 条带 file:line 的具体待办，第 4 条
> （组件化对话框只给 npm/custom，P7-fe）待做，是下轮最高优先级。

> **上一轮：P7-fa 脚本模式补回原版的「导出脚本」动作，把这个表单的最后一条内容动作落地
> （原版 `src/extensions/CustomIntegrationDrawer.tsx:145` 的「Export script」按钮 →
> `src/ExtensionsPanel.tsx:1951-1964` 的 `exportCustomScript` → 后端
> `src-tauri/src/commands/extensions.rs:1516-1560` 的 `extensions_custom_export_script`）**
> ——「尚未做」的 5 条具体待办里第 1–4 条（P7-ew/P7-ex/P7-ey/P7-ez）已落地，本轮取文档标注的最高
> 优先级第 5 条（导出脚本写文件），落完这一条后「尚未做（P7-ew 重写）」清单**归零**。依赖检查：
> `go list -m -versions github.com/egoist/mygo` 与 GitHub `releases/latest` 均确认最新仍是
> **v0.4.8**（与 `go.mod` 一致），无版本可升、无新 API 用点；只对照 `main`
> （`backup/main-pre-mygo` = v0.3.14 原版树），未翻历史提交。
>
> **偏离（P7-fa·导出脚本）**：原版 script 模式的动作行有两个按钮——「Copy script」与「Export
> script」；后者开原生保存对话框（文件名 `{id}-script.{extension}`，`id` 取
> `editingCustomId ?? customIntegration.command`，`extension` 由语言表给出），写文件成功后
> `showSuccess(customScriptExported)`（`i18n.ts:651` 的 “Script exported”）。**本构建（改前）**：
> P7-ew 只落了复制这一支，`grep -rn 'ExportScript\|custom_export_script\|ScriptExported' internal` 为空
> ——脚本只能停在编辑器里，用户没法把它存成文件。
>
> **落地**：
> - `internal/i18n` 新增 `LocalToolExportScript`("Export script")/`LocalToolScriptExported`
>   ("Script exported")，en/zh 逐字取 `i18n.ts:649,651`；另补 `LocalToolExportFailed`
>   （写盘/对话框失败的那一句，原版走 `errorMessage(nextError)`，本构建给自己的键，
>   `internal/i18n/i18n.go:868-874`、`copy.go` en/zh）。
> - `internal/settingsui` 的 `Actions` 新增 `ExportScript func(name, content string)`；
>   `toolContentActions` 收 `language` 参数，script 模式下在 Copy script 之后画
>   `exportScriptAction`（无 exporter 或脚本为空则不画）；新增
>   `scriptExportName(id, command, language)` 复刻原版 `{id}-script.{extension}`
>   （无 id 用 command，两者皆空退回 `floter-provider`）。
> - `internal/shell/sync.go` 新增 `exportScript(name, content)`：复用 `saveFile` 在渲染线程外开
>   对话框，`os.WriteFile` 写盘，成功发 `script-export` 成功 toast，关闭对话框静默、失败发
>   `LocalToolExportFailed`；`saveFile` 的过滤器改为跟随文件名后缀（`.py` → Python），
>   集成导出的 `.json` 仍是 JSON filter。
>
> **验收**：`internal/settingsui/toolcontent_test.go` 新增 `TestLocalToolFormExportsTheScript`
> （program 模式不画导出按钮、script 模式画、点击后 `Actions.ExportScript` 收到
> `notes-tool-script.js` 与表单脚本、导出不占剪贴板的 1.6s 确认窗）与 `TestScriptExportName`
> （四例：编辑 id / command 兜底 / 双空兜底 / 语言扩展）；`TestLocalToolFormCopiesTheScript` 加一条
> 「无 exporter 时不画导出按钮」。变异核查：让 `exportScriptAction` 直接返回 → 红
> （"the script form is missing its export button"）。`internal/shell/shell_test.go` 新增
> `TestTheScriptExportWritesTheFile`（对话框收到的名字逐字、文件内容等于表单脚本、成功 toast 文案与
> 类型）与 `TestTheScriptExportStaysQuietWhenItWritesNothing`（关闭对话框无 toast 无 notice、
> 对话框报错 → `LocalToolExportFailed`）。变异核查：去掉 `os.WriteFile` → 红
> （"the script was not written"）。
>
> **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 全绿
> （1301 项）、`GOOS=linux/windows/darwin go build ./...` 三平台通过。
>
> **清单状态**：「尚未做（P7-ew 重写）」的 5 条带 file:line 的具体待办**全部落地**
> （P7-ew/P7-ex/P7-ey/P7-ez/P7-fa）。下轮若继续，需要新一轮逐字段审计把清单重新写实
> （候选入口：`localToolSection`/`runForm` 之外的 `pluginsheet`、`transfer`、`uninstall` 等）。

> **上一轮：P7-ez 声明式输入编辑器回到原版的字段顺序，补回「占位提示」与「默认值」两个字段
> （原版 `src/extensions/CustomIntegrationDrawer.tsx:83-104` 的 `ScriptParamEditor`）**
> ——「尚未做」的 5 条具体待办里第 1–3 条（P7-ew/P7-ex/P7-ey）已落地，本轮取文档标注的最高优先级
> 第 4 条（参数编辑器字段顺序）。依赖检查：`go list -m -versions github.com/egoist/mygo` 与
> `proxy.golang.org` 的 `@latest` 均确认最新仍是 **v0.4.8**（与 `go.mod` 一致），无版本可升、无新
> API 用点；只对照 `main`（`backup/main-pre-mygo` = v0.3.14 原版树），未翻历史提交。
>
> **偏离（P7-ez·输入编辑器）**：原版每个参数一行，字段组按 label/id/kind/（非 boolean 才有）
> placeholder/default/flag 排开，`.extension-param-editor__controls`（`required` 开关 + 删除按钮）
> 收在行尾，`select` 的 options 在行下展开，行末是这一行自己的报错（`:95-104`）。**本构建（改前）**：
> `internal/settingsui/settingsui.go` 的 `toolParamsEditor` 行头只有 id/kind/flag 三个控件，label 掉到
> 行下，**`placeholder` 与 `default` 两个字段根本没有**——`toolParam` 无这两个字段、
> `editorParams`/`beginEdit`（`:4504` 一带）都不读它们，于是编辑一个带占位符/默认值的集成再保存，
> 这两个值被静默丢掉（后端 `ParamDefinition` 是有这两个字段的，`internal/extensions/params.go:20`）。
>
> **落地**：
> - `internal/i18n` 新增 `ParamsPlaceholder`("Placeholder") / `ParamsDefault`("Default")，en/zh 逐字取
>   `i18n.ts:723-724`（`internal/i18n/i18n.go:649-670`、`copy.go` en/zh）。
> - `internal/settingsui/settingsui.go` 的 `toolParam` 增加 `Placeholder`/`Default`；`editorParams`
>   与 `beginEdit` 读写它们（经 `strings.TrimSpace` 归一，同其余字段）。
> - `toolParamsEditor` 重排：行头只放原版的 controls 组（`Required` 开关 + `Remove` 按钮），字段按
>   label/id/kind/(非 boolean 才有)placeholder/default/flag 逐行铺开，`select` 的 Choices 行仍在
>   Flag 之后，行末是这一行自己的报错（照原版把报错从行首挪到行尾）。
>
> **验收**：`internal/settingsui/params_test.go` 新增
> `TestConnectFormKeepsTheReferenceParamFields`——字段以 label/id/kind/placeholder/default/flag 的顺序
> 出现；`boolean` 不画 placeholder 行（默认值/Flag 仍在）；`declaredParams()` 带出 placeholder/default；
> `beginEdit` 从存储的声明里还原两者。变异核查：把非 boolean 的 placeholder 行去掉 → 红
> （“the field \"Placeholder\" is missing or out of the reference's order”）；把 Default 行提到 Kind
> 之前 → 红。既有 `TestConnectFormDeclaresInputs`/`TestParamOptionsNewlineCodec`/`TestParamRowIssues`
> 保持绿。
>
> **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 全绿、
> `GOOS=linux/windows/darwin go build ./...` 三平台通过。
>
> **清单状态**：第 4 条（参数编辑器字段顺序 + 补齐 placeholder/default）落地；「尚未做」5 条具体待办中
> 第 1–4 条已完成，只剩第 5 条（脚本模式「导出脚本」写文件动作）。下轮最高优先级是第 5 条。

> **上一轮：P7-ey 编辑态补回原版的只读「集成 ID」行
> （原版 `src/extensions/CustomIntegrationDrawer.tsx:144` 的 `editingId &&` 分支，词条
> `i18n.ts:683` 的 `customId`）**
> ——「尚未做」的 5 条具体待办里第 1 条（P7-ew 内容动作行）、第 2 条（P7-ex Program 行）已落地，
> 本轮取文档标注的最高优先级第 3 条（编辑态只读 ID 行）。依赖检查：GitHub `releases/latest` 与
> `go list -m -versions github.com/egoist/mygo` 均确认最新仍是 **v0.4.8**（与 `go.mod` 一致），
> 无版本可升、无新 API 用点；只对照 `main`（`backup/main-pre-mygo` = v0.3.14 原版树），未翻历史提交。
>
> **偏离（P7-ey·集成 ID 行）**：原版编辑态在 Name 行之前画一条 `customId` label 后跟不可编辑的
> `<code>{id}</code>`；创建态整行隐藏（R9-3：id 由后端铸造且此后不可变，前端不展示可推导的值）。
> **本构建（改前）**：`internal/settingsui/settingsui.go` 的 `localToolSection` 在
> `a.toolEditing != ""` 时只把提交按钮文案换成 `LocalToolSave`，没有任何一行点名正在改的是哪个 id
> （`grep -rn 'CustomId\|LocalToolId\|Integration ID' internal/settingsui` 为空）。
>
> **落地**：
> - `internal/i18n` 新增 `LocalToolId`，en/zh 逐字取 `i18n.ts:683`（`internal/i18n/i18n.go:806-813`
>   一带、`copy.go` en/zh）。
> - `internal/settingsui/settingsui.go:2987-2992` 的卡片顶部（Name 行之前）在 `a.toolEditing != ""` 时
>   走 `a.keyValue(c, copy.LocalToolId, a.toolEditing)`——与 About 页只读值同一读法：label 在行首、
>   值在行尾且 `Selectable()`，没有可编辑控件；创建态该行不画。
>
> **验收**：`internal/settingsui/toolcontent_test.go` 新增
> `TestLocalToolFormNamesTheIntegrationBeingEdited`——en/zh 标签逐字断言；列表态不画该行；点
> 「Edit…」进编辑态后 `HasText(copy.LocalToolId)` 且 `a.toolEditing == "local.abc"`；`endEdit()` 后
> 该行消失。变异核查：删掉 `localToolSection` 的 `if a.toolEditing != ""` 块 → 新测试红（
> “the edit form does not name the integration”）。
>
> **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 全绿、
> `GOOS=linux/windows/darwin go build ./...` 三平台通过。
>
> **清单状态**：第 3 条（编辑态只读集成 ID 行）落地；「尚未做」5 条具体待办中第 1–3 条已完成，
> 第 4–5 条待做：参数编辑器字段顺序、脚本模式导出脚本写文件。下轮最高优先级是第 4 条。

> **上一轮：P7-ex 自定义工具表单的 Program 行补回原版的占位符与保存期提示
> （原版 `src/extensions/CustomIntegrationDrawer.tsx:145` 的 `customExecutable` label 下方
> `customExecutablePlaceholder` 与 `changeExecutableHint`，文案在 `i18n.ts:665,687`）**
> —— P7-ew 把「尚未做」重写为 5 条具体待办并落地第 1 条（内容动作行），本轮取第 2 条
> （Program 行的 hint 与占位符，文档标注为“改动最小”）。依赖检查：`go list -m -versions
> github.com/egoist/mygo` 最新仍是 **v0.4.8**（与 `go.mod` 一致），无版本可升；GitHub releases
> API 限流。只对照 `main`（`backup/main-pre-mygo` = v0.3.14 原版树），未翻历史提交。
>
> **偏离（P7-ex·Program 行）**：原版可执行模式的 `customExecutable` 字段有两条词——输入框的
> `customExecutablePlaceholder`（“Type a path or search installed tools”，`i18n.ts:687`）与字段下方
> 的 `changeExecutableHint`（“Search the tools installed on this device, or type a full path. Changing
> the executable re-runs verification when you save.”，`:665`），后者说明换可执行文件会在保存时重跑
> 验证。**本构建（改前）**：`internal/settingsui/settingsui.go:3009-3014` 的 Program 行只
> `Placeholder("/usr/bin/rg")`，没有 hint——`grep -rn 'ChangeExecutable\|LocalToolProgramHint' internal`
> 为空，占位符也是自造路径。
>
> **落地**：
> - `internal/i18n` 新增 `LocalToolProgramPlaceholder` / `LocalToolProgramHint`，en/zh 逐字取
>   `i18n.ts:665,687`（`internal/i18n/i18n.go:808-813`、`copy.go` en/zh）。
> - `internal/settingsui/settingsui.go:3009` 的 Program 行改成
>   `a.row(c, copy.LocalToolProgram, copy.LocalToolProgramHint, …)`，`textInput` 的 `Placeholder`
>   改成 `copy.LocalToolProgramPlaceholder`——与相邻字段行共用同一套 hint 节奏（原版
>   `.extension-custom-form__hint` 就是 label 下的一条 caption）。脚本模式不画 Program 行，两条词
>   自然不出现。
>
> **验收**：`internal/settingsui/toolcontent_test.go` 新增 `TestLocalToolProgramRowCarriesItsWord`——
> en/zh 四个值逐字断言；打开表单后 Program 行确有 hint（`HasText`），占位符**不**泄漏进
> `Texts()`（mygo 把 placeholder 画在编辑器里、不注册成 label，同 `view_test.go:2178` 的注释）；
> 切到 script 模式后 hint 消失。既有 `TestLocalToolForm` 因表单又高一行为 1800 渲染高度（注释
> 说明内容动作行 + Program hint 两处增长）。变异核查：把 `settingsui.go` 的 hint 参数改回 `""` →
> 新测试红（“the program row is missing its hint”）。
>
> **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 全绿、
> `GOOS=linux/windows/darwin go build ./...` 三平台通过。
>
> **清单状态**：第 2 条（Program 行的 hint 与占位符）落地；「尚未做」清单现有 5 条带 file:line 的
> 具体待办，第 1 条（P7-ew）、第 2 条（P7-ex）已完成，第 3–5 条待做：集成 ID 只读行、参数编辑器
> 字段顺序、导出脚本写文件动作。下轮最高优先级是第 3 条（编辑态只读 ID 行）。

> **上一轮：P7-ew 自定义工具表单的内容动作行（复制脚本 / 复制命令 / 复制执行计划；
> 原版 `src/extensions/CustomIntegrationDrawer.tsx:145-146` 的 `.extension-custom-form__actions`，
> 行为在 `src/ExtensionsPanel.tsx:1934-1964`）**
> —— P7-ev 收尾后用户反馈 F1–F4 全部收口，「尚未做」上只剩「回到 `localToolSection`/`runForm`
> 等入口逐函数审计」这句宽泛候选。本轮按规则先把它**写实为 5 条带 file:line / 原版对照 / 验收标准
> 的具体待办**（见「尚未做」的「具体待办（P7-ew 重写）」），再取第 1 条落地。依赖检查：
> `go list -m -versions github.com/egoist/mygo` 最新仍是 **v0.4.8**（与 `go.mod` 一致），无版本可升；
> GitHub releases API 限流。只对照 `main`（`backup/main-pre-mygo` = v0.3.14 原版树），未翻历史提交。
>
> **偏离（P7-ew·内容动作行）**：原版自定义工具抽屉在字段之后有一行**内容动作**——script 模式一个
> 「Copy script」、executable 模式「Copy command」+「Copy execution plan」；每次按下走
> `copyCustomContent`（单飞行 `customContentRef` + `navigator.clipboard.writeText` + `showSuccess` 确认
> 1.6s）或 `copyExecutionPlan`（把 `{program, args, mode:"pty"}` 美化 JSON 后同一条路）。**本构建
> （改前）**：`internal/settingsui/settingsui.go:3009-3054` 的表单只有字段行 + 脚本编辑器 + Output 行，
> **没有任何复制按钮**（`grep -rn 'LocalToolCopyScript\|copyScript\|copyPlan' internal/settingsui` 为空，
> i18n 也无对应词条）；用户要复制脚本/命令只能手动选中。
>
> **落地**：
> - `internal/i18n` 六个键 `LocalToolCopyScript`("Copy script")/`LocalToolScriptCopied`("Script copied")/
>   `LocalToolCopyCommand`/`LocalToolCommandCopied`/`LocalToolCopyPlan`/`LocalToolPlanCopied`，en/zh
>   逐字取 `i18n.ts:648-655`（`internal/i18n/i18n.go:840-850`、`copy.go`）。
> - `internal/settingsui/rows.go`（新 `actionRow`）：无标签的卡内行，照常画隔行的 hairline 与缩进，
>   让这串按钮与旁边的字段行共享同一套卡内节奏（原版的 `.extension-custom-form__actions` 也是
>   一格 full-width 行）。
> - `internal/settingsui/settingsui.go`（新 `toolContentActions`/`contentAction`/`contentCopied`/
>   `executionPlanJSON`/`splitPlanArgs` + `App.copiedContent`/`copiedContentUntil` 跨帧确认）：
>   script 模式一个「Copy script」（复制编辑器当前 buffer）、program 模式「Copy command」
>   （`strings.TrimSpace(command)`）+「Copy execution plan」（`{program, args: strings.Fields(args),
>   mode:"pty"}` 的 `json.MarshalIndent`，空参数读作 `[]` 而非 `null`）；`:3054` 接在字段之后、
>   Output 行之前（原版顺序）。按下的按钮翻成自己的确认词 1.6s（`copiedContentDuration`），
>   只点亮被按的那个（按 action 而非按文本去重，两个恰好同字节的按钮不会一起亮）。复用已有的
>   `Actions.CopyText`（shell 已接 `mygo.Clipboard.WriteText`）。
>
> **验收**：`internal/settingsui/toolcontent_test.go`（新，三条）：
> `TestLocalToolFormCopiesItsContent`——program 模式画两个按钮、点 Copy command 后剪贴板拿到 `rg`
> 且按钮翻成 "Command copied"、再点 Copy plan 后剪贴板第二条能 JSON 解出 `{/usr/bin/rg, [--floter -n],
> pty}` 且只 plan 按钮亮（command 已回休）；另断 `contentCopied` 的 1.6s 窗口。
> `TestLocalToolFormCopiesTheScript`——script 模式只画 Copy script（程序两个按钮不画）、复制出语言的
> 模板、按钮翻成 "Script copied"。`TestExecutionPlanJSON`——空白参数读作空列表、绝无 `null`。
> 既有 `TestLocalToolForm` 因表单高一行为 1700 渲染高度（注释说明）。
> 变异核查：把 `a.toolContentActions(...)` 摘掉（改成 `_ =`）→ 两条新测试红（“missing its content
> actions” / “missing its copy button”）。
>
> **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 全绿、
> `GOOS=linux/windows/darwin go build ./...` 三平台通过。
>
> **清单状态**：第 1 条（内容动作行）落地；「尚未做」清单现有 **5 条带 file:line 的具体待办**
> （第 1 条已落地，第 2–5 条待做：可执行字段的 hint/占位符、集成 ID 只读行、参数编辑器字段顺序、
> 导出脚本写文件动作）。下轮最高优先级是第 2 条（Program 行的 hint 与占位符，改动最小）。

> **上一轮：P7-ev F-new——脚本内容编辑器换用 mygo `editor.View`（v0.4.8 新包的自然用点；
> 原版 `src/ExtensionsPanel.tsx` 的自定义工具表单里脚本内容是带高亮的代码编辑区，`settings.extensions.customScript*`）**
> —— P7-eu 收尾后「尚未做」清单上只剩 **F-new**，本轮取它。依赖检查见 P7-eu：`go.mod` 仍是 v0.4.8，
> 无版本可升；只对照 `main`（`backup/main-pre-mygo` = v0.3.14 原版树）。
>
> **偏离（F-new·脚本编辑器）**：原版的脚本内容是一个带语法高亮、行号的代码编辑区
> （`src/extensions/` 下的 script 编辑器组件）。**本构建（改前）**：`internal/settingsui/settingsui.go:3294`
> 是一个纯粹的 `ui.TextArea(c, script)`（多行文本框，无行号、无高亮），绑定表单局部变量 `script`
> （`*string`），表单结束时 `a.toolScript = script` 写回 App 状态（`:3105`）；语言切换回调
> （`:3276-3284`）在 `script` 为空或 `isTemplate(*script)` 时直接 `*script = scriptlang.Template(id)`。
>
> **落地前必须先确认的三件事（状态文档 F-new 条目的要求）**：
> (1) **editor.View 的输入路径**：mygo editor 的粘贴/键入由框架自己处理，不经过 `internal/inputtext`。
> 逐条核实后确认**不适用**：inputtext 的 Zs→U+0020 / 粘贴去尾换行边界存在的原因是「终端字节必须匹配
> 键盘会发的字节」，而脚本内容是 **provider 文件内容**（宿主写出、解释器读取），不是 PTY 字节流——
> 它不经过 shell token 化，也不进终端。编辑器自己的粘贴路径（mygo ui/editor）这里是正确的那一个。
> 如实写进 `scripteditor.go` 的注释，**不是偏离**。
> (2) **表单焦点/提交共存**：`editor.View` 处理自己的键输入（`DefaultBindings`），绘制在 `stackedRow`
> 的闭包里；表单的提交按钮是点击触发，`Submitted()` 链不被编辑器打断。
> `TestScriptEditorTakesAnOutsideDraft` 端到端验证 connect 提交的就是编辑器 buffer 的内容。
> (3) **LSP/补全依赖**：editor 的补全由 app 主动调 `ShowCompletions` 触发，不接 language server；
> 我们只取行号 + 高亮，不引入 LSP 依赖。`editor.State` 不设任何 completion 字段即无补全菜单。
>
> **落地**：
> - `internal/settingsui/scripteditor.go`（新）：`scriptEditor` 结构体挂在 App 上跨帧持久
>   （`state *editor.State` / `language` / `written`）——每帧 NewBuffer 会丢 undo 历史、光标和滚动位置。
>   `scriptEditorView(c, copy, language, script)`：状态样式逐帧解析（`Font ui.Font{Family:"monospace",
>   Size: t.FontSize}`、`LineHeight` 按 `t.Space(2.25)/t.FontSize` 换算复刻原 `FixedLineHeight(2.25)`、
>   `Theme: editor.DefaultTheme(t)`、`Label`）；语言切换时 `SetLanguage` + buffer 仍是空/旧模板则
>   `Set(scriptlang.Template(新语言))`（替代原回调的直接赋值）；外部改 `*script`（表单 seed/重开编辑）
>   经 `*script != se.written` 判定后 `Set` 进 buffer；渲染后 `*script = st.Buffer.String()` 写回表单。
> - `editorLanguage(id)`：`scriptlang.Extension(id)` → `editor.LanguageFor("main."+ext)`；**PowerShell
>   无内置语法**（`LanguageFor("main.ps1")` 返回 nil），内嵌 `editor.Language{Name:"PowerShell",
>   LineComment:"#", Brackets: 花括号/圆括号}`（匹配与缩进所需，无 tokenizer 着色）。
> - `internal/settingsui/settingsui.go`：`scriptRows` 的 `ui.TextArea(c, script)` 换成
>   `a.scriptEditorView(c, copy, language, script)`（`.Height(t.Space(18)).Grow(1)` 同原位）；语言回调
>   删掉直接 `*script = scriptlang.Template(id)`（改走同步逻辑）；`App` 加 `scriptEditor *scriptEditor`
>   字段。
>
> **验收**：
> - `internal/settingsui/scripteditor_test.go`（新，五条）：
>   `TestScriptEditorDrawsLineNumbers`——gutter 行号是**像素断言**（tester 不把 gutter 数字注册为文本
>   元素，类似 placeholder 穿过编辑器不记为 label，见 `view_test.go:2178` 的同款注释；mygo
>   `ui/editor/paint.go`）：hint 下方左缘 48×110 条带内与背景色差 >25 的像素 ≥30（四个数字的量）。
>   `TestEveryScriptLanguageHasAnEditorLanguage`——8 种语言全部有 editor 语法（ps1 走内嵌）。
>   `TestScriptEditorTakesAnOutsideDraft`——外部 seed `a.toolScript = "print('hi')\\n"` 渲染后 connect
>   提交的 `ScriptContent` 逐字等于它。
>   `TestScriptEditorFollowsTheLanguageSwitch`——开表单 grammar = JavaScript；点 "JavaScript" 展开选择器
>   再点 "Python" 后 grammar = Python 且 `a.toolScript` = Python 模板（同步链完整）。
>   `TestScriptEditorHandsTheUsersTypingOn`——把键盘点进编辑器框、键入 `MARKER` 后 connect，提交的
>   `ScriptContent` 必须含 `MARKER`（钉住渲染末的 buffer→表单回写这一条，见下变异核查）。
> - 既有 `pluginsheet_test.go` 的脚本端到端（语言切换模板交换 + connect 提交 `"print('hi')\\n"`）
>   在新实现下保持全绿。
> - 变异核查（逐条实测，两处更正）：把语言同步块删掉 → `TestScriptEditorFollowsTheLanguageSwitch` 红
>   （grammar 不跟着走）。把 `*script = st.Buffer.String()` 回写删掉 → `TestScriptEditorHandsTheUsersTypingOn`
>   红（**键入的内容没到 `a.toolScript`**）；注意外部 seed 那条（`TestScriptEditorTakesAnOutsideDraft`）
>   单删回写**不会**红——表单局部 `script` 仍持有 seed，是 `st.Buffer.Set(*script)` 那个**入向**同步才是
>   它钉的：删 `Set(*script)` → 该条红（提交到旧模板）。两处同步各钉一条，互不掩盖。
>
> **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 全绿、
> `GOOS=linux/windows/darwin go build ./...` 三平台通过。
>
> **清单状态**：**F-new 落地，用户反馈 F1–F4 全部收口**。F1（P7-eg）、F2（P7-d）、F3a（P7-em）、
> F3b（P7-en）、F4a（P7-eh）、F4b（P7-ei）、F4c（P7-ek）、F4d（P7-ej）、F4e（P7-el/P7-ep）、
> F4f（P7-eo）、F4g 五条入口（P7-eq/P7-er/P7-es/P7-et/P7-eu）全部落地。余下候选：`localToolSection` /
> `runForm` 等其他入口的逐函数审计（`ExtensionsPanel.tsx` 尚有未逐函数对照的流程）。

> **上一轮：P7-eu F4g 最后一条入口——导入导出的调用点（原版 `src/ExtensionsPanel.tsx:1206-1240`
> 的 `exportExtensions`/`importExtensions` 与 `:2300-2385` 的报告块、
> `src-tauri/src/commands/extensions.rs:1029-1215` 的 `extensions_export`/`extensions_import`、
> `src-tauri/src/extensions/sync.rs:360-470` 的 `import_document`）** —— P7-et 收尾后「尚未做」清单
> 点名的下轮最高优先级就是 **F4g 的导入导出对照**（唯一未逐函数对照的 F4g 入口），本轮取它。
> 依赖检查：GitHub releases API 限流；`proxy.golang.org` 的 `@v/list` 最新仍是 **v0.4.8**
> （只有 tag、无 release），`go.mod` 无版本可升；只对照 `main`（`backup/main-pre-mygo` = v0.3.14
> 原版树），未翻历史提交。
>
> **偏离（F4g·import/export call point）**：原版这一点做了五件事——(1) 单飞行闸
> `syncOperationRef.current`（同一时刻一个传输）；(2) `syncOperation` 非空时**禁用两个按钮**并在
> 行内显示 `exporting`/`importing` 忙态；(3) 导出成功发 **success toast** `exportedNotice{count}`
> （“Exported {count} integrations (secrets and device paths excluded)”）；(4) 导入成功把
> `importReport` 渲染成**页面报告块**（`excludedFields` 警告 / `importRolledBack`+`importFailedDetail`+
> `importFailedMore` 或 `importSummary{succeeded,skipped}`，`title` = 失败明细或路径）；(5) 任何错误走
> `showError` **toast**。**本构建（改前）**：只有「两个按钮 + 一行 `TransferStatus`」，没有单飞行闸、
> 没有忙态、导出成功只落一行文字、导入不渲染报告、错误也落同一行。
>
> **落地**：
> - `internal/i18n`：新增 `IntegrationsExporting`（“Exporting…”）、`IntegrationsImporting`、
>   `IntegrationsImportSummary`、`IntegrationsImportExcludedFields`、`IntegrationsImportFailedDetail`、
>   `IntegrationsImportFailedMore`；`IntegrationsExported` 的签名由 `(count, path)` 改成原版
>   `exportedNotice` 的逐字文案 `(count)`；删掉自造的 `IntegrationsImported` 与两个 helper
>   （原版没有这条合并句子），逐字取 `i18n.ts:509-520`。
> - `internal/extensions/sync.go`：`SyncReport` 新增 `ExcludedFields []ExcludedFieldWarning`，
>   `ImportSync` 在应用任何条目之前按 `entry.FieldMetadata.Excluded` 收集（原版 `sync.rs:377-386`
>   的位置），失败条目也算。
> - `internal/shell/sync.go`：`beginTransfer`/`finishTransfer` 复刻原点位的单飞行闸
>   （`syncOperationRef`）；忙态写 `Settings.TransferBusy`（`export`/`import`）；导出成功用
>   `queueToast`（新 `transferNoticeID`，success，4s）发原版文案，失败用 `queueNotice` 发 error
>   toast；导入完成用 `setTransferReport` 把 `settingsui.ImportReport` 留在页面上再
>   `RefreshIntegrations`。
> - `internal/settingsui/transfer.go`（新）：`ImportReport`/`TransferFailure`/`ExcludedField` 与
>   `transferReport`/`transferNotice`（excluded 警告 / 前三条失败 `id: reason` + “and N more” /
>   summary）。`internal/settingsui/settingsui.go` 的 `transferRow` 改为：忙时两个按钮 `.Disabled`、
>   行内显示忙态文案、行下渲染报告块；`TransferStatus` 字段换成 `TransferBusy`/`TransferReport`。
> - **有意偏离**：原版导入是**全或无**，失败即整体回滚，其失败通知以 `importRolledBack`
>   （“Import failed and all changes were rolled back”）打头。本构建的 `ImportSync` 按条应用
>   （P3-l 的语义，已写进 plan），失败**不回滚**已落地的条目，故失败通知只给原版的
>   `importFailedDetail`/`importFailedMore`，不加会撒谎的 `rolledBack` 抬头——记为本轮有意偏离。
>
> **验收**：
> - `internal/extensions/sync_test.go` 的 `TestImportSyncReportsExcludedFields`：导出的文档带回
>   `fieldMetadata`，导入报告的 `ExcludedFields` 命中带密钥与设备路径的条目且计数为 2。
> - `internal/settingsui/view_test.go` 的 `TestIntegrationTransferButtons`（改写）：忙态显示
>   “Exporting…”/“Importing…”，干净导入显示 `Imported 4, unchanged 1`，失败导入显示 excluded 警告
>   与 `Failed: a: …; b: …; c: … (and 1 more)`。
> - `internal/shell/shell_test.go` 的 `TestIntegrationExportAndImport`（改写）：导出后的 toast 文案 =
>   原版 `exportedNotice` 且类型 success；导入后 `TransferReport` 的 path/succeeded 正确；坏文件只发
>   error toast、不留报告。`TestTheTransferIsOneAtATime`：忙时 `beginTransfer` 第二次被拒、第二次
>   `exportIntegrations` 不再开文件对话框、传输结束后忙态清空。
> - 变异核查：删掉 `transferRow` 末尾的报告块 → settingsui 三条断言红；放开 `beginTransfer` 的 guard
>   → `TestTheTransferIsOneAtATime` 红（“the transfer guard allowed a second claim”）。
>
> **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 全绿、
> `GOOS=linux/windows/darwin go build ./...` 三平台通过。
>
> **清单状态**：**F4g 五条入口全部对照落地**（安装文件夹 P7-eq、权限审批 P7-er、运行工具选择
> P7-es、输出回看 P7-et、导入导出 P7-eu）；「尚未做」只剩 **F-new**（脚本编辑器换 `ui/editor`）。
> 下轮最高优先级是 **F-new**，或回到 `localToolSection`/`runForm` 等其他入口的逐函数审计。

> **上一轮：P7-et F4g 第 4 条入口——漂移提示（原版 `src/ExtensionsPanel.tsx:2078-2132` 的
> `handleDriftNotice`、`src-tauri/src/extensions/install.rs:843-975` 的
> `reprobe_on_tool_version_change`、`src-tauri/src/commands/extensions.rs:139-188` 的
> `dispatch_drift_reprobe`）** —— P7-es 收尾后「尚未做」清单里文档点名的下轮最高优先级就是
> **F4g 的第 4 条入口**（输出回看），本轮取它。依赖检查：GitHub releases API 限流；
> `proxy.golang.org` 的 `@v/list` 最新仍是 **v0.4.8**（只有 tag、无 release），`go.mod` 无版本可升；
> 只对照 `main`（`backup/main-pre-mygo` = v0.3.14 原版树），未翻历史提交。
>
> **偏离（F4g·drift notice）**：原版 `extensions_list`（`commands/extensions.rs:205-300`）在装配
> 列表时收集**生成的自定义集成**（`runtime_ownership == System` 且 `is_generated_custom_integration`）
> 作为 `drift_candidates`，交给 `dispatch_drift_reprobe`（`:139-188`）：`try_start_drift_reprobe`
> 保证同一时刻只有一个后台循环（一次列表风暴最多每个漂移探一次），循环对每个候选调
> `install::reprobe_on_tool_version_change`（`install.rs:843-975`）——当工具的**记录版本**与现在
> `--version` 报的不一致时，重跑 connect 期的 help 推导（原子替换 + help-probe sidecar），并把
> 记录版本改成新的；完成后若 `report.previous_command_count != Some(report.command_count)` 就在既有
> `extension-op-progress` 事件上发**一帧** `notice`（`ReprobeNotice{tool_version,
> previous_command_count, command_count}`）。前端唯一消费者是 `handleDriftNotice`（`ExtensionsPanel.tsx:2078-2132`）：
> 先 `decideDriftNotice(frame, name, reprobeNoticeGate.current)` 定夺（无变化不发、无名不发、同一变化
> 一次；且**无变化先判**，不烧名额），把 `decision.display` 写回抽屉的新鲜度行与列表行
> （`lastProbeAt`/`commandCount`/`previousCommandCount`，缺字段保留旧值、不填 0），`decision.announce`
> 非空才发一条 `onNotify("success", t("settings.extensions.reprobeNotice", {name, delta}))`，带一个
> `settings.extensions.viewDetails` 动作用来跳详情。
> **本构建（改前）**：`internal/extensions/reprobe.go` 的 `ReprobeCommands` 是**手动、同步**的；
> `internal/extensions/driftnotice.go`（P7-bc）已落地**纯判定**（`DecideDriftNotice`/`DriftNoticeGate`），
> 但**没有任何调度与消费**——全仓无人调它（`grep DecideDriftNotice` 只命中测试），也没有
> 「记录版本 vs 当前版本」的比较，更没有后台循环或 toast。
>
> **落地**：
> - `internal/extensions/driftversion.go`（新）：`CheckToolVersionDrift(ctx, paths, id) (DriftReprobe, error)`
>   —— 复刻 `reprobe_on_tool_version_change` 的判定与动作：`driftReprobeEligible`（enabled +
>   `RuntimeOwnershipSystem` + `isGeneratedIntegration` + 非 script）→ 可执行存在 → `probeVersion`
>   读当前版本（空即无比较，不探）→ 与 `Entry.ToolVersion` 相等即闲置 → `ReprobeCommands`
>   重生成（失败软返回，保留旧描述符）→ `SetToolVersion` 记录新版本；`ReprobeRan` 报告是否真探了。
> - `internal/extensions/reprobe.go`：新增进程级 `reprobeMu` 串行化 `ReprobeCommands`
>   （原版 repo `mutation_lock` 的等价物，防两处并发重探丢失 staged 文件）。
> - `internal/shell/driftreprobe.go`（新）：`App.reprobeDriftedIntegrations`（后台循环，
>   `driftReprobeCandidates` 收集 id；`startDriftReprobe`/`finishDriftReprobe` 复刻
>   `try_start_drift_reprobe` 的单飞行闸）→ 逐个 `reprobeDriftedIntegration`（调
>   `CheckToolVersionDrift`，构造 `ReprobeNoticeFrame`，交 `DecideDriftNotice` 定夺）→
>   `raiseDriftNotice` 用新 `queueToast` 排一条 **success** toast（`ReprobeNotice` 句 + delta 词 +
>   `ViewDetails` 动作，`TOAST_DISMISS_MS.success` = 4s），动作 `openIntegrationCard` 打开设置面
>   Integrations 页；`driftNoticeGate` 建会话级 gate；`refreshAfterDrift` 刷新命令表与帧。
> - `internal/shell/shell.go`：`RefreshIntegrations` 末尾调 `reprobeDriftedIntegrations()`，让每次列表
>   （启动/安装/卸载/重探等）都带上漂移检查——正是原版 `extensions_list` 的位置。
> - `internal/shell/notices.go`：新增 `queueToast(ui.Toast)`（`queueNotice` 改为调它）与
>   `successNotice` 窗口常量。
> - `internal/i18n`：`Settings` 新增 `ReprobeNotice(name,delta)`（“{name} 命令已更新（{delta}）”）、
>   `ReprobeDeltaIncrease`（`+{n}`）、`ReprobeDeltaDecrease`（`−{n}`，U+2212）、`ViewDetails`
>   （“查看详情”），逐字取原版 `i18n.ts:614-617,1496-1499`。
>
> **验收**：
> - `internal/extensions/driftversion_test.go`（新）：版本移动即重探且报告 previous/command；记了新版本
>   后第二次闲置（幂等）；无版本不探；script/bundled、publisher、disabled 三种不合资格；可执行消失不探。
> - `internal/shell/driftreprobe_test.go`（新）：移动的版本恰好弹一条 success toast（句子 = 原版
>   `reprobeNotice` + `+2`、动作 = `viewDetails`、窗口 = `successNotice`）；同一变化第二次不弹（gate）
>   而不同变化仍弹；无版本不弹；重探失败软着陆（不 toast、不标 broken、版本不动）；单飞行闸；
>   toast 真的被 `toastViewport` 画出且动作落到 Integrations 页；`RefreshIntegrations` 经列表本身
>   也能弹（在它 spawn 的 goroutine 上）。
> - 变异核查：删掉 `RefreshIntegrations` 末尾的调度 → `TestRefreshIntegrationsRaisesTheDriftToast` 红
>   （“a listing never raised the drift toast”）；把 `raiseDriftNotice` 短路 → 四条 toast 测试红；
>   放开单飞行闸 → `TestTheDriftReprobeLoopDoesNotStack` 红；放开资格闸 → 三条资格测试红。
>
> **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 全绿、
> `GOOS=linux/windows/darwin go build ./...` 三平台通过。
>
> **清单状态**：**F4g 第 4 条入口落地**（漂移提示）。残留与有意偏离：(1) 原版把列表行也实时改写
> （`setExtensions` 里 `lastProbeAt`/`commandCount` 缺字段保留旧值），本构建的行来自磁盘刷新
> （`refreshAfterDrift` 重读 repository+manifest），数值一致但多一次 I/O；(2) 原版 toast 的
> `viewDetails` 动作用 `setSelectedId` 打开该集成的抽屉，本构建**没有逐集成抽屉**，动作落到
> Integrations 页（带新鲜度点的卡片是「详情」最接近的落点），未按 id 高亮。余下 F4g 最后一条入口
> （**导入导出**的调用点：原版 `extensions_export` / `extensions_import`）与 **F-new**（脚本编辑器
> 换 `ui/editor`）。下轮最高优先级是 **F4g 的导入导出对照**，除非 F4g 的对照先查出新缺口。
>
> **上一轮：P7-es F4g 第 3 条入口——多个候选时的运行工具选择 `chooseSystemTool`（原版
> `src/ExtensionsPanel.tsx:1393-1401,1975-1988,2623-2648`、`i18n.ts:472-473`、
> `src-tauri/src/commands/extensions.rs:1754-1759`）** —— P7-er 收尾后「尚未做」清单里文档点名的
> 下轮最高优先级就是 **F4g 的第 3 条入口**（多个候选才弹 `chooseSystemTool`），本轮取它。依赖检查：
> GitHub releases API 限流、`go list -m -versions github.com/egoist/mygo` 网络超时（均无新信息），
> `go.mod` 仍是 **v0.4.8**，无版本可升；只对照 `main`（`backup/main-pre-mygo` = v0.3.14 原版树），
> 未翻历史提交。
>
> **偏离（F4g·tool selection）**：原版给每个扩展算一份 `toolCandidates`（`commands/extensions.rs:258-300`
> 用 `resolution_candidates(resolve_manifest_candidate(...))`）：把清单 `executableNames` 交给
> `resolver::resolve_executable_names`（`extensions/resolver.rs:52-88`）在**全部发现来源**（PATH、
> Launch Services、desktop entry…）里打分，平手即 `ResolveResult::Ambiguous`，于是
> `connectRecommended` / `reconnectSystem`（`ExtensionsPanel.tsx:1393-1401,1975-1988`）在
> `toolCandidates.length > 1` 时弹 `pendingToolSelection`（`:2623-2648`）：标题
> `chooseSystemTool`（“Choose a runtime for {name}”）、提示 `chooseSystemToolHint`、每个候选一行
> （`name` / `locator.path` / `sources.join(" · ")`），`chooseSystemTool` 把选中路径交给
> `connectRecommendedAt(extension, path)` 或 `reconnectSystemAt(extension, path)`。
> **本构建（改前）**：没有候选概念——`ResolveRuntime`（`internal/extensions/provider.go:43-84`）
> 用 `lookTool(names...)` 取**第一个**匹配就绑定，`reconnectIntegration` 调
> `extensions.Reconnect(paths,id)`（丢弃记录路径后按名重解析），`connectRecommended` 直接
> `Commit`，两边都不问用户。
>
> **落地**：
> - `internal/extensions/candidates.go`（新）：`ExecutableCandidate{Name,Path,Sources}`（原版
>   `ToolCandidate` 的裁剪，`Sources` 恒为 `["path"]`，本构建只有 PATH 这一种发现来源）；
>   `LookTools` / `lookToolsIn`（按搜索路径顺序列出全部匹配，按解析后路径去重——符号链接不算两个）；
>   `lookTools` 变量 + `StubToolCandidates` 测试缝（与既有 `lookTool`/`StubToolLookup` 同构）；
>   `SystemRuntimeCandidates(manifest, recordedPath)`——记录路径即原版 `preferred_locator`：仍在
>   就返回它一条（原版 history 打分使它唯一胜出），否则返回全部匹配（>1 即原版 `Ambiguous`）。
> - `internal/extensions/reconnect.go`：`Reconnect` 拆出 `ReconnectAt(paths,id,executablePath)`，
>   非空路径直接用（先 `isExecutableFile` 校验），空路径保持原「丢弃记录后按名重解析」；对应原版
>   `extensions_reconnect_system` 的 `executable_path` 参数（`commands/extensions.rs:1754-1759`）。
> - `internal/shell/system.go`：`App.chooseSystemTool`（注入点 nil 走原生
>   `mygo.Dialog.Message`）与纯函数 `chooseSystemToolText`；按钮一个候选一个（`名称 — 路径`）+ Cancel，
>   body 依原版顺序拼 hint + 每个候选 `• 名称 — 路径 (来源)`。
> - `internal/shell/shell.go`：`Options.ChooseSystemTool` + `App.chooseTool`；新增
>   `recommendedToolPath(name, manifest, recordedPath)`（0 个返回空、1 个返回它、>1 弹选择）；
>   `connectRecommended` 在权限审阅**之前**先解析程序并写回 `prepared.ExecutablePath`（原版
>   `connectRecommended` 先选再审），顺带修好 P7-er 记下的残留「审阅时不解析候选」；
>   `reconnectIntegration` 走同一解析并把选中路径交给 `ReconnectAt`。
> - `internal/i18n`：`Settings` 新增 `ChooseSystemToolTitle`（“Choose a runtime for {name}”/
>   “为 {name} 选择运行工具”）与 `ChooseSystemToolHint`，文案逐字取原版 `i18n.ts:472-473,1359-1360`。
>
> **验收**：
> - `internal/extensions/candidates_test.go`（新）：`LookTools` 按搜索路径顺序列出全部匹配；
>   符号链接折叠（不重复计）；`SystemRuntimeCandidates` 记录路径优先（返回仅一条）与无记录时返回全部；
>   非 system 运行时返回 nil；`ReconnectAt` 绑定到指定程序、拒绝不可运行的程序。
> - `internal/shell/choosesystemtool_test.go`（新）：`chooseSystemToolText` 标题 + hint + 每个候选
>   `• v — 路径 (path)`；推荐 connect 两个候选时弹一次选择、绑到选中路径；取消选择则不审不装；
>   reconnect 两个候选时弹选择并绑到选中路径。
> - `internal/shell/reconnect_test.go` 两条既有测试改注入 `StubToolCandidates`（本机恰有
>   `/usr/bin/systool`，旧缝 `StubToolLookup` 已不覆盖候选枚举）。
> - 变异核查：把 `connectRecommended` 的候选解析删掉 →
>   `TestRecommendedConnectAsksWhichExecutableToBind` 红（“the connect never landed”，
>   runtime-unavailable）。
>
> **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 全绿（1270 项）、
> `GOOS=linux/windows/darwin go build ./...` 三平台通过。
>
> **清单状态**：**F4g 第 3 条入口落地**（多个候选时的运行工具选择）。残留（如实记为有意偏离）：
> (1) 本构建的候选只有 PATH 一种发现来源，`Sources` 恒为 `["path"]`（原版有 Launch Services /
> desktop entry 等多种）；(2) 原版 `toolCandidates` 的打分含 version/stability/fingerprint，本构建
> 只用「记录路径优先，否则全列」这一条能观察到的规则；(3) 原版弹窗是可聚焦的 listbox（`role="listbox"`），
> 本构建用原生多按钮 message box（与 F4g 前两条入口的对话框实现一致）。余下 F4g 两条入口
> （输出回看 `handleDriftNotice` / 导入导出）与 **F-new**（脚本编辑器换 `ui/editor`）。下轮最高优先级是
> **F4g 的第 4 条入口**（输出回看：原版 `ExtensionsPanel.tsx:2078-2132` 的 `handleDriftNotice`），
> 除非 F4g 的对照先查出新缺口。
>
> **P7-er F4g 第 2 条入口——推荐工具的权限审阅（原版
> `src/ExtensionsPanel.tsx:1349-1400,2594-2620`、`src-tauri/src/extensions/install.rs:2175-2240`、
> `src/extensions/PermissionTierList.tsx`、`i18n.ts:465-469,553`）** —— P7-eq 收尾后「尚未做」
> 清单里文档点名的下轮最高优先级就是 **F4g 的第 2 条入口**（权限审批），本轮取它。依赖检查：
> GitHub releases API 限流，`go list -m -versions github.com/egoist/mygo` 最新仍是 **v0.4.8**
> （与 `go.mod` 一致），无版本可升；只对照 `main`（`backup/main-pre-mygo` = v0.3.14 原版树），
> 未翻历史提交。
>
> **偏离（F4g·recommended review）**：原版点推荐工具的 Connect 走 `connectRecommendedAt`
> （`ExtensionsPanel.tsx:1349-1400`）：先调 `extensions_recommended_permissions`
> （`commands/extensions.rs:1689-1698`）→ `install::permission_review`
> （`extensions/install.rs:2175-2240`），它把清单**全部 declared 权限**转成
> `{permission, enforcement, title, description}` 摘要；**只有非空**才弹 `pendingPermissionReview`
> （`:2594-2620`）——标题 `confirmConnectRecommended`（“Connect the recommended tool {name} to
> Floter?”）/`confirmConnectManifest` 二选一、显示可执行文件路径、`permissionsRequired` 标签、
> `PermissionTierList`（enforced / disclosure 两层，各带 label+hint，每条带 title+description）、
> 边界句 `permissionReviewBoundary`，按钮 Cancel / `connect`；Confirm 才走
> `extensions_connect_recommended` 且提交**全部 declared**。**本构建（改前）**：`connectRecommended`
> （`internal/shell/shell.go`）只调 `recommended.Prepare` → `prepared.Approval.NeedsApproval()` →
> `confirmInstallPermissions` → `permissionDialog`（`internal/shell/system.go`）：标题
> “Allow {name} to run?”、一行一条 `标签 — 强制/声明` 的平铺列表、Allow/Cancel。**没有**标题区分、
> 没有两层分组与每条的说明、没有边界句，且门槛用的是 `Added`（增量）而不是原版的“有声明就审”。
>
> **落地**：
> - `internal/shell/system.go`：`recommendedInstallReview`（原生 `mygo.Dialog.Message`）与纯函数
>   `recommendedInstallReviewText`（title + body，便于测试）；body 依原版顺序拼“可执行文件路径
>   （`Prepared.ExecutablePath` 非空时）+ `permissionsRequired` 标签 + 两层权限 + 边界句”。新增
>   `permissionTierLines`：按 `extensions.PermissionEnforced` 分组（enforced 在前），每层画
>   label + hint，每条画 `• 标签 — 说明`（说明取 `i18n.PermissionDescription`）。
> - `internal/shell/shell.go`：`Options.ReviewRecommended func(extensions.Prepared) bool` +
>   `App.reviewRecommended` 注入点（nil 走原生对话框）；`connectRecommended` 改为
>   **`len(prepared.Approval.Declared) > 0` 时先审**（原版 `review.permissions.length` 门槛），
>   用户确认才 `prepared.Commit(true)`（提交全部 declared，与原版 `confirmPermissionReview` 一致）。
> - `internal/i18n`：`Settings` 新增 `PermissionsRequired`（“Permissions required”/“需要以下权限”）、
>   `ConnectRecommendedTitle`（“Connect the recommended tool {name} to Floter?”/“将推荐工具 {name}
>   接入 Floter？”）、`PermissionsConnect`（“Connect”/“连接”）；边界句复用 `InstallReviewBoundary`
>   （原版 `permissionReviewBoundary` 一个键被两个对话框共用，`i18n.ts:553`）。
>
> **验收**：
> - `internal/shell/recommendedreview_test.go`（新）两条：
>   `TestRecommendedInstallReviewNamesTheToolAndTiersItsPermissions`（纯函数：标题
>   “Connect the recommended tool V Tools to Floter?”、可执行路径行、`Permissions required`
>   标签、enforced 层 label+hint+`• 启动进程 — …`、disclosure 层 label+hint+`• 访问网络 — …`、
>   边界句、**enforced 层在 disclosure 之前**、路径为空时不带路径行）、
>   `TestRecommendedConnectReviewsBeforeCommitting`（流程：注入 `ReviewRecommended`，拒绝 → 仓库为空
>   且磁盘无包；确认 → 落地，review 每次 connect 调一次）。
> - `internal/shell/shell_test.go:TestConnectRecommendedInstallsTheShippedPackage` 改注入
>   `ReviewRecommended` 并断言 review 一次、declared 七条（原来注入的是 `ConfirmPermissions`）。
> - 变异核查：把 `connectRecommended` 换回旧的 `NeedsApproval()`+`confirmInstallPermissions` 支 →
>   `TestRecommendedConnectReviewsBeforeCommitting` 红（“the review was never shown”）。
>
> **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 全绿（1260 项）、
> `GOOS=linux/windows/darwin go build ./...` 三平台通过。
>
> **清单状态**：**F4g 第 2 条入口落地**（推荐工具的权限审阅）。残留（如实记为有意偏离）：
> (1) 原版标题的 `manifestSuggestion` 一支（`confirmConnectManifest`）在本构建**不可达**——本构建没有
> “约定位置清单”这一种 detected 行（`detectedTools` 只列 PATH 程序），故只落了 recommended 一支；
> (2) 可执行文件路径行只在 `Prepared.ExecutablePath` 非空时出现，推荐流程在审阅时不解析候选程序
> （原版先 `inspect_manifest_executable` 再审）。余下 F4g 三条入口（运行参数 / 输出回看 / 导入导出）
> 与 **F-new**（脚本编辑器换 `ui/editor`）。下轮最高优先级是 **F4g 的第 3 条入口**（运行参数：原版
> `ExtensionsPanel.tsx:2560-2580` 的 `pendingToolSelection` 弹窗，多个候选才弹 `chooseSystemTool`），
> 除非 F4g 的对照先查出新缺口。

> **P7-eq F4g 第 1 条入口——安装文件夹的 manifest review（原版
> `src/ExtensionsPanel.tsx:1430-1440,1550-1560`、`src/extensions/LocalInstallDialog.tsx`、
> `src-tauri/src/commands/extensions.rs:1470-1501`）** —— P7-ep 收尾后「尚未做」清单里文档点名的
> 下轮最高优先级就是 **F4g 的第 1 条入口**（安装文件夹的 manifest review），本轮取它。依赖检查：
> GitHub releases API 限流，`proxy.golang.org` 的 `@v/list` 最新仍是 **v0.4.8**（与 `go.mod` 一致），
> 无版本可升；只对照 `main`（`backup/main-pre-mygo` = v0.3.14 原版树），未翻历史提交。
>
> **偏离（F4g·folder review）**：原版选完文件夹后走 `connectLocal` → `reviewLocalManifest`
> （`ExtensionsPanel.tsx:1430-1440`），它调 `extensions_local_manifest_review`
> （`src-tauri/src/commands/extensions.rs:1470-1501`）拿到 `{extensionName, runtime, platforms,
> source:"local", permissions}`，然后**无条件**弹 `LocalInstallDialog`（`:2600`）：标题
> `localConfirmTitle`（“Connect {name}?”）、提示 `localConfirmHint`、三行 manifest 摘要
> （`localSource`→`localDirectory`“Local package”、`localRuntime`、`customPlatforms`）、
> `PermissionTierList`（全部 declared 权限）、边界句 `permissionReviewBoundary`，按钮
> Cancel / `localConfirm`（“Confirm connection”）。**只有该对话框的 Confirm 走
> `extensions_install`**（`:1554-1556`），且提交的是**全部 declared 权限**。
> 本构建（改前）：`internal/shell/shell.go` 的 `installPackageFolder` 只在
> `prepared.Approval.NeedsApproval()` 时弹一个**纯权限**的原生 message box
> （`confirmInstallPermissions` → `permissionDialog`，`internal/shell/system.go`），**没有**
> name/source/runtime/platforms 摘要、也没有边界句；**不声明权限的包直接静默安装**，连对话框都不弹。
>
> **落地**：
> - `internal/shell/system.go`：`localInstallReview`（原生 `mygo.Dialog.Message`）与纯函数
>   `localInstallReviewText`（title + body，便于测试）；body 依原版顺序拼 hint / Source /
>   Runtime / Supported platforms / 每条 declared 权限（带 enforced/declared 句）/ 边界句。
>   权限行与 `permissionDialog` 共用一个新的 `permissionLines`，两条路径不再各写一遍。
> - `internal/shell/shell.go`：`Options.ReviewInstall func(extensions.Prepared) bool` +
>   `App.reviewInstall` 注入点（nil 走原生对话框）；`installPackageFolder` 改为**无条件**先
>   `reviewLocalInstall(prepared)`，用户确认才 `prepared.Commit(true)`（提交全部 declared，与原版
>   `confirmLocal` 一致），拒绝则什么都不落。
> - `internal/i18n`：`Settings` 新增 `InstallReviewTitle/Hint/Source/LocalDirectory/Runtime/
>   Platforms/Unavailable/Boundary/Confirm`，EN/ZH 双份，文案逐字取原版 `i18n.ts:640-645,553,551`
>   （`unavailable` = “Not available”/“暂无”）。
>
> **验收**：
> - `internal/shell/installreview_test.go`（新）三条：
>   `TestLocalInstallReviewCarriesTheManifestSummary`（纯函数：标题 “Connect Ripgrep?”、
>   Source/Runtime/Platforms 三行、无权限时**不带**边界句也不带权限行、空平台列表说 “Not available”）、
>   `TestLocalInstallReviewListsDeclaredPermissions`（enforced 与 declared 两种权限行 + 边界句）、
>   `TestInstallFromFolderReviewsBeforeCommitting`（流程：一个**不声明任何权限**的本地包，注入的
>   `ReviewInstall` 仍被调用一次——这正是旧代码会跳过的情形；拒绝 → 仓库为空；确认 → 落地且
>   `WithID("dev.floter.reviewed")` 可查到）。
> - 变异核查：把 `installPackageFolder` 的 `reviewLocalInstall` 换回
>   `if prepared.Approval.NeedsApproval()` 守卫 → `TestInstallFromFolderReviewsBeforeCommitting` 红
>   （不声明权限的包不再触发 review）。
>
> **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 全绿（1258 项）、
> `GOOS=linux/windows/darwin go build ./...` 三平台通过。
>
> **清单状态**：**F4g 第 1 条入口落地**（安装文件夹的 manifest review）。余下 F4g 的四条入口
> （权限审批 / 运行参数 / 输出回看 / 导入导出）与 **F-new**（脚本编辑器换 `ui/editor`）。下轮最高
> 优先级是 **F4g 的第 2 条入口**（权限审批：原版 `ExtensionsPanel.tsx:1349-1400,2594-2620` 的
> `connectRecommendedAt` 与 `extension-permission-backdrop`，对照本构建的 `confirmInstallPermissions`
> / `permissionDialog` / `permissionAudit`），除非 F4g 的对照先查出新缺口。

> **P7-ep F4e 的 repair 一支：broken 行回到原版的 Wrench「Re-check」（原版
> `src/ExtensionsPanel.tsx:1255-1267`、`src/extensions/ExtensionRow.tsx:313-330`、
> `i18n.ts:505-507`）** —— P7-eo 收尾后「尚未做」清单里文档点名的下轮最高优先级就是 **F4e 的
> `extensions_repair`（Recheck）一支**，本轮取它。依赖检查：GitHub releases API 限流，
> `go list -m -versions github.com/egoist/mygo` 最新仍是 **v0.4.8**（与 `go.mod` 一致），无版本可升；
> 只对照 `main`（`backup/main-pre-mygo` = v0.3.14 原版树），未翻历史提交。
>
> **偏离（F4e·repair）**：原版对 **connected && state === "broken"** 的行画一个 Wrench，
> `aria-label`/`title` 都是 `settings.extensions.recheck`（`:313-330`），走 `extensions_repair`
> （`ExtensionsPanel.tsx:1255-1267`）——它**校验已装包并按需修复**，toast 按 `report.repaired` 二选一：
> `repairedNotice`（“Repaired”）或 `recheckedNotice`（“Re-check complete: healthy”，`i18n.ts:505-507`）。
> Rust 侧 `commands/extensions.rs:2057-2130` 的 `repair()` 顺序是：先 `verify_installed_locked`
> （清单身份 + runtime 解析），通过就 `clear_broken` 并回 `repaired:false`；失败则 `mark_broken`，
> 再对 system runtime 走 `reconnect_system_locked` 兜底（回 `repaired:true`）。
> 本构建（改前）**没有 repair**：broken 行只有一个 `copy.IntegrationsCheck` 按钮 →
> `internal/shell/shell.go` 的 `diagnoseIntegration`（跑 `Reprobe` + `Diagnose`，**只报告**、不校验包
> 完整性也不修）；i18n 也没有 `recheck`/`repairedNotice`/`recheckedNotice`；而且 P7-el 在
> `repairKind()` 里留了一处**有意偏离**（broken 且绑定类失败的行复用 Reconnect），因为当时没有别的入口。
>
> **落地**：
> - `internal/extensions/repair.go`（新）：`Repair(paths, id) (RepairReport, error)`。`RepairReport`
>   只有 `{Repaired, ManifestPath}`。校验顺序照原版：包目录与清单的存在性 + `ManifestDigest` 与
>   `ApprovedManifestDigest`（`ApprovalIsStale`，缺批准记录按“未知”不算 stale），再 `ResolveRuntime`；
>   `ErrPackageMissing` / `ErrManifestChanged` 是**报告**（reinstall / 重新批准才修得了）。两处**真修**：
>   记忆的 `manifestPath` 已失效而清单在 id 目录下（`manifestPathFor` 的 fallback）→ 重写该字段
>   （`Repaired=true`）；system runtime 的程序搬走 → 复用 `Reconnect`（`reconnectIfSystemOwned`，
>   原版 `repair()` 的 `reconnect_system_locked` 兜底）。校验通过但 entry 还是 `broken` → `clearBroken`
>   清掉失败记录（原版 `clear_broken`），但那是**校验结果不是修复**，所以 `Repaired` 仍为 false。
> - `internal/settingsui`：`Actions.RepairIntegration`（nil 隐藏按钮）；行内先判 `integration.Broken`，
>   是则画 `copy.IntegrationsRecheck`（原版 `settings.extensions.recheck`）→ `RepairIntegration`，
>   否则才走原来的 `repairKind()` 分叉；`repairKind()` 去掉那处偏离（`if i.Broken || i.RuntimeAvailable
>   { return repairNone }`），`bindingFailure()` 随之删除。
> - `internal/shell`：`RepairIntegration: a.repairIntegration`；`repairIntegration` 先
>   `a.Integrations.Refresh`（原版 `refreshAfterMutation`）再按 `err`/`report.Repaired` 三选一发通知。
> - `internal/i18n`：`Settings.IntegrationsRecheck`（“Re-check”/“重新检测”）、
>   `Notifications.IntegrationRepaired`（“X was repaired”/“X 已修复”）、
>   `Notifications.IntegrationRechecked`（“X re-checked: healthy”/“X 已重新检测：状态正常”）。
>
> **验收**：
> - `internal/extensions/repair_test.go`（新）六条：`TestTheRecheckVerifiesThePackagesOwnBytes`
>   三行表（包目录缺失 → `ErrPackageMissing`、digest 与批准记录不一致 → `ErrManifestChanged` 且批准
>   记录未被改写、两者正常 → `Repaired=false`）、`TestTheRecheckRewritesAStaleManifestPath`
>   （`Repaired=true` 且仓库字段改到 id 目录）、`TestTheRecheckClearsABrokenStateItContradicts`
>   （`State` 回 enabled、失败记录清空、`Repaired` 仍 false）、
>   `TestTheRecheckRebindsASystemRuntimeThatMoved`（`Repaired=true`、`ExecutablePath` 回新路径）、
>   `TestTheRecheckRefusesToRebindARuntimeItDoesNotOwn`（bundled 只报告）。
> - `internal/settingsui/integrationrepair_test.go`：`TestABrokenRowOffersTheRecheck`（像素：同屏一个
>   Re-check + 一个 Reconnect，各在自己行上；点 Re-check 落到 `RepairIntegration("broken")`；
>   nil action 不画按钮），并把 `TestTheRepairFollowsTheRuntimeOwnerAndThePage` 里那处偏离的用例改为
>   「broken 行 `repairKind()==repairNone`」。
> - `internal/shell/reconnect_test.go`：`TestTheRepairNoticesSplitRepairedFromRechecked` 两个子测试——
>   包完好 → `IntegrationRechecked`，`manifestPath` 漂移 → `IntegrationRepaired` 且字段被重写。
> - 变异核查：把 `repairKind()` 的 `i.Broken` 判断去掉 → settingsui 两条红（纯映射 + 偏离用例）；
>   把 `Repair` 的 digest 不一致支改成 `Repaired=false, nil` → extensions 的「不一致」行红。
>
> **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 全绿（1255 项）、
> `GOOS=linux/windows/darwin go build ./...` 三平台通过。
>
> **清单状态**：**F4e 全部落地**（P7-el 的 Reconnect 一支 + 本轮的 repair 一支）。余下 **F4g**（逐入口
> 对照，五条已写实行号）与 **F-new**（脚本编辑器换 `ui/editor`）。下轮最高优先级是 **F4g 的第 1 条入口**
> （安装文件夹的 manifest review：`ExtensionsPanel.tsx:1430-1440,1554` 对照
> `internal/settingsui` 的 `localToolSection`/`installationReview`），除非 F4g 的对照先查出新缺口。
>
> **上一轮：P7-eo F4f 移除确认条按 kind 说话（原版 `src/ExtensionsPanel.tsx:2140-2151`、
> `src/extensions/RemovalConfirmation.tsx:41`、`i18n.ts:521-542`）** —— P7-en 收尾后「尚未做」清单
> 里文档点名的下轮最高优先级就是 **F4f**（移除确认条按 kind 说话，F4a 留下的尾巴），本轮取它。
> 依赖检查：`proxy.golang.org/github.com/egoist/mygo/@latest` 仍是 **v0.4.8**（与 `go.mod` 一致），
> 无版本可升（GitHub releases API 限流，改用 module proxy 查最新 tag）；只对照 `main`
> （`backup/main-pre-mygo` = v0.3.14 原版树），未翻历史提交。
>
> **偏离（F4f）**：原版确认条的**标题/正文/确认按钮**与菜单项走**同一个 per-kind key**——
> `ExtensionsPanel.tsx:2140-2151` 的 `removalTextKey` 按 `removalKind` 选
> `deleteCustom|uninstall|disconnect|removePackage` 词干，`RemovalConfirmation.tsx:41` 用它读 Title 与
> 正文（`t(textKey(extension, "Title"))` / `t(textKey(extension, ""))`）。本构建 P7-eh 已把**菜单动词**
> 按 kind 落地（`removalLabel`），但确认条仍是**一句通用文案**：`removeConfirmation` 一律走
> `RemoveTitle`/`RemoveDetail`/`RemoveConfirm`（只有 custom 例外），于是「Disconnect」按钮底下问的是
> 「Uninstall X?」——按钮与它打开的问题自相矛盾。
>
> **落地**：`internal/settingsui/settingsui.go` 新增 `Integration.removalWords(copy) (title, detail string)`
> ——按 `removalKind()` 四分支取词（custom 仍带 `RemoveCustomWarning` 的额外警告，原版 `:41` 就是在
> 标题后追加 `deleteCustomWarning`）；`removeConfirmation` 改用它取标题/正文，确认按钮改用已有的
> `integration.removalLabel(copy)`（原版确认按钮读的就是 `textKey(extension, "")`，与菜单同一个词干）。
> `internal/i18n/i18n.go`+`copy.go` 新增 `IntegrationsDisconnectTitle`/`IntegrationsDisconnectDetail`、
> `IntegrationsRemovePackageTitle`/`IntegrationsRemovePackageDetail`（en/zh 用原版原文），并删掉因此
> 变成死键的 `RemoveConfirm`（`TestNoDeadCopy` 会红）。npm 支沿用 `RemoveTitle`/`RemoveDetail`、custom 支
> 沿用 `RemoveCustom*`，与原版 `removalTextKey` 的四支一一对应。
>
> **验收**：`internal/settingsui/integrationremoval_test.go` 两条（新）——
> `TestTheRemovalConfirmationAsksTheKindsOwnQuestion`（纯映射：四 kind 的 `removalWords` 标题等于各自
> 原版 key，且非 npm 支不得等于 npm 通用问句）、
> `TestEachRemovalConfirmationAsksItsOwnQuestionOnScreen`（像素：同屏四行分别点各自动词打开确认条，
> 钉四个标题出现、且非 npm 行**不出现**通用 npm 问句）。既有 `internal/settingsui/view_test.go` 的
> `TestIntegrationsPage` 与 `internal/shell/settingsescape_test.go` 的
> `TestEscapeAnswersTheSettingsQuestionFirst` 原本钉的是**旧的错口径**（package 行问「Uninstall X?」），
> 按 F4f 改写为「Remove X?」并加一条「package 行不得问 npm 问句」的反向断言。
> 变异核查：把 `removalWords` 的 system 支改回 `RemoveTitle`/`RemoveDetail` → 两条新测试立刻红
> （纯映射与像素各一条）。
>
> **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 全绿、
> `GOOS=linux/windows/darwin go build ./...` 三平台通过。
>
> **清单状态**：**F4f 落地**；余下 F4e 的 `extensions_repair`（Recheck）一支、F4g、F-new。下轮最高
> 优先级是 **F4e 的 repair 一支**（原版 `ExtensionRow.tsx:313-330` 的 Wrench「Recheck」→
> `extensions_repair`，file:line 与验收都已写实），除非 F4g 的逐入口对照先查出新缺口。
>
> **上一轮：P7-en F3b 帧 sheen 回固定口径（原版 `src/styles/base.css:628-629,906-907`、
> `src/styles/launcher.css:119`）** —— P7-em 收尾后「尚未做」清单里 F3b 是**文档点名的下轮最高
> 优先级**，且它此前的阻塞条件（「先决于 F3a」，怕固定帧 sheen 让三档在无窗口材质时完全一致）
> 已被 P7-em 解除：控件透镜已成可测像素（四个包的像素测试都钉住 0.5/1/1.6 单调上升）。依赖检查：
> GitHub releases API 限流，`go list -m -versions github.com/egoist/mygo` 最新仍是 **v0.4.8**
> （与 `go.mod` 一致），无版本可升；只对照 `main`（`backup/main-pre-mygo` = v0.3.14 原版树），
> 未翻历史提交。
>
> **偏离（F3b）**：原版卡片是 `linear-gradient(180deg, var(--sheen-top), var(--sheen-bottom)),
> var(--launcher-tint)`（`launcher.css:119`），而 `--sheen-top`/`--sheen-bottom` 是**按外观固定的
> token**（`base.css:628-629` 暗 0.046/0.01、`:906-907` 亮 0.66/0.22），**不随 `[data-glass]` 变**；
> 档位在**控件**上走的 `--glass-lens-scale`（`:784/:791/:798`）与帧的亮面无关。本构建
> `internal/theme/theme.go` 把帧 sheen 写成 `sheen = spec.TintAlpha * spec.LensScale`（P7-c/P7-d 为
> 「3 档看不出区别」加的耦合），于是帧的亮面随档位变——原版不这样。
>
> **落地**：`internal/theme/theme.go` 的 sheen 改回**按外观固定的 token**（`spec.Glass` 时
> `capAlpha(p.sheenTop/p.sheenBottom, spec.TintAlpha)`，否则为 0，即 `off` 无 sheen）；
> `capAlpha` 保留（比帧还重的 sheen 会让薄帧顶部比滑杆更密，`FaceFill` 的补偿不能要负的 base），
> 所以帧的 **alpha 仍是滑杆的**（`FaceFill` 的补偿照旧）；`scaleAlpha` 的注释收窄为「只管
> `--glass-lens-scale` 的控件透镜」，`Tokens.SheenTop/SheenBottom` 的注释写明「每档同一个固定 token」。
>
> **验收**：
> - `internal/theme/theme_test.go` 两条（新）：`TestTheFrameSheenIsFixedAcrossStops`（frosted /
>   regular / liquid 的 `SheenTop`/`SheenBottom` 完全相等，且等于按外观固定 token 的 `capAlpha` 值；
>   `off` 为零）、`TestTheFrameSheenIsTheFixedTokenNotTheTint`（滑杆最底时暗色仍是原版的 0.046、
>   亮色是「固定 token 被 cap」，不是「tint 被缩放」）。
> - `internal/shell/glass_test.go` 的 `TestTheGlassStopsAreVisiblyDifferent` **改写并改名**为
>   `TestTheFrameSheenIsFixedAcrossStops`（F3b 待办点名要改写它）：帧顶带（y=90..120）里 off 无
>   sheen、三档颜色**相同**（RGB sum 极差 ≤1，残差是控件透镜渗进带均值），而三档在**控件**上的差
>   由 P7-em 的四组像素测试与 `glasscensus_test.go` 钉住；帧 alpha 仍是滑杆的（`off` 半透明、
>   开启不加密）。
> - 变异核查：把 `theme.go` 的 sheen 改回 `* spec.LensScale` → theme 两条红（「the stop graded the
>   sheen's top/bottom」），shell 那条也红（light：三档差 13.22 RGB sum）。
>
> **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 27 包全绿
> （1233 项）、`GOOS=linux/windows/darwin go build ./...` 三平台通过。
>
> **清单状态**：**F3a、F3b 均已落地**（用户反馈 F3「毛玻璃效果配置」整条收口）；余下 F4e 的
> repair 一支、F4f、F4g、F-new。下轮最高优先级是 **F4f**（移除确认条按 kind 说话，F4a 留下的
> 尾巴，file:line 与验收都已写实）——除非 F4g 的逐入口对照先查出新缺口。
>
> **上一轮：P7-em F3a 控件透镜落地（原版 `src/tests/glass-reaxis.test.ts:47-90` 的 lens census、
> `src/styles/base.css:188,231,593-597`）** —— P7-el 之后「尚未做」清单五条（F4e 的 repair 一支、
> F4f、F4g、F3a、F3b）全是具体项，文档点名 **F3a 为下轮最高优先级**，本轮取它。依赖检查：
> GitHub releases API 限流，`go list -m -versions github.com/egoist/mygo` 最新仍是 **v0.4.8**
> （与 `go.mod` 一致），无版本可升；只对照 `main`（`backup/main-pre-mygo` = v0.3.14 原版树），未翻历史提交。
>
> **F3a 偏离（P7-ej 实测的根因）**：P7-ej 用生产帧扫全画布，发现控件透镜在 regular↔liquid 之间
> 只贡献 ≤0.4 RGB——因为设置面/启动器的按钮、分段控件、开关**大多由框架的 `ui.Button`/`ui.Switch`/
> `ui.Segmented` 直接绘制，根本不带透镜**；原版 `--elev-0`/`--elev-track` 是**每条 CSS 规则**自己带的，
> 而 `glass-reaxis.test.ts:47-90` 就是那份「哪些控件必须画透镜」的逐条清单。本构建此前只有
> 四处落点（卡片 `rows.go`、结果行图标 `glyphs.go`、侧栏/终端少数），其余全是裸框架控件。
>
> **F3a 修法（三层，按待办给出的建议顺序）**：
> - **框架能力层**：`internal/theme/theme.go` 新增 `Tokens.LensInset(e)` —— `--glass-lens-stack-inset`
>   的等价物（rim 在下、暗边在上、内槽影、track-edge 环），并新增 `TrackEdge` token
>   （`--glass-track-edge`，暗 0.06 / 亮 0.07）。`LensInset` 的层按「CSS 第一条在最上」的语义倒序添加
>   （mygo 的 inset 影后加者在上），已写进函数注释。
> - **设置面控件**：`internal/settingsui/rows.go` 新增 `actionButton`（`Lens(ui.Button)`，原版
>   `.extensions-action-button` / `.settings-save-alert button`）、`switchControl`
>   （`LensInset(ui.Switch)`，`.settings-switch`）、`segmentedControl`
>   （`LensInset(ui.Segmented)`，`.extension-custom-mode`）；扩展页 26 个 `ui.Button`、7 个 `ui.Switch`、
>   4 个 `ui.Segmented` 全部改走它们；侧栏 hover 行、设置头栏图标按钮 hover、保存失败横幅的 ✕ hover
>   也补上透镜（原版这三条都是 hover-only rung）。
> - **控制台与终端**：`internal/control/control.go` 的 `Segmented` 轨道改用 `LensInset`（原版
>   `.settings-options--inline`），选中段改用完整 `Lens` + accent 键线（原版 `.settings-option--active`
>   的 `--elev-0, inset accent-edge`）；`internal/shell/notices.go` 的 toast 动作 chip 补 `Lens`
>   （`.app-toast__action`）；`internal/terminalui/terminalui.go` 的栏按钮 hover 补 `Lens`
>   （`.toolbar-button:hover`）。
>
> **验收**：
> - `internal/shell/glasscensus_test.go`（新）——把原版 `glass-reaxis.test.ts:47-90` 的 **13 个选择器
>   逐条映射**到本构建的绘制点（file + 必须仍在的透镜调用 + 像素测试名，或如实标注 `pending` 的理由），
>   并断言清单与原版逐项同名同序；每个 landed 落点读源文件确认透镜调用仍在（落点丢透镜即红）。
> - 像素测试（同一 `main_opacity`、**常背景**隔离帧 sheen）：`internal/theme/lens_test.go` 两条
>   （凹槽底边被点亮、且亮度差随 0.5/1/1.6 单调上升）；`internal/control/lens_test.go` 一条
>   （分段控件整帧 RGB sum 随档位严格上升，regular→liquid ≥3000）；`internal/settingsui/lens_test.go`
>   三条（动作按钮/卡片 sum 上升 ≥300；开关与自定义模式控件的「底边-顶边」差随档位上升）；
>   `internal/launcher/plate_lens_test.go` 一条（结果行/动作栏图标底板 sum 上升 ≥50）。
> - 变异核查：把 `LensInset` 摘成只有 `LensRim` 一层 → theme 两条红；把 `control.Segmented` 的
>   `Lens(segment)` 摘掉 → control 那条红；把 `a.actionButton` 换回 `ui.Button` → settingsui 那条红。
>
> **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 27 包全绿
> （1231 项）、`GOOS=linux/windows/darwin go build ./...` 三平台通过。
>
> **清单状态**：**F3a 落地**；F3b（帧 sheen 回固定口径）因此**解除「先决于 F3a」**、成为下轮最高
> 优先级（其验收「帧专属带差 ≤0.4、控件差 ≥ 阈值」现在控件一侧已成立）；余下 F4e 的 repair 一支、
> F4f、F4g、F-new 仍在清单上。`.extensions-icon-button:hover` 一条如实记为 pending（本构建扩展页
> 的动作都是 `.extensions-action-button`，没有「静止透明、hover 才亮」的独立图标按钮）。
>
> **上一轮：P7-el F4 残留「卡片的修复动作」逐函数对照 + 缺掉的 Reconnect 落地
> （原版 `src/extensions/ExtensionRow.tsx:310-370`、`src/ExtensionsPanel.tsx:1255-1267,1955-1979`）**
> ——「尚未做」清单上只剩 F3a（一个待决策的分叉）与 F3b（已实测标为「先决于 F3a」），**两条都不是可直接落地的
> 具体项**；按规则先把清单写实（见文末「具体待办（P7-el 重写）」），再取第 1 条实现。第 1 条改为
> **F4 残留里本轮新查出的真实缺口**：原版卡片对「runtime 不可用」的行有一个修复按钮，本构建
> **一个都没有**。依赖检查：`go.mod` 已是 **v0.4.8** = GitHub releases API 的 latest（无版本可升）；
> v0.4.8 的 release notes 里唯一自然用点是新的 `ui/editor`（代码编辑器/差分视图），落点是本构建
> 的脚本内容编辑框（`internal/settingsui/settingsui.go:3258` 的 `ui.TextArea`），已写成本轮的新待办
> 而不是本轮实现（换控件要连带动 mygo 表单 API 与 `internal/inputtext` 的边界约定）。只对照 `main`
> （`backup/main-pre-mygo` = v0.3.14 原版树），未翻历史提交。
>
> **偏离（F4e）**：原版同一张已连接卡片在两个状态下给出修复入口——
> - `connected && state === "broken"` → **Wrench「Recheck」**，走 `extensions_repair`，toast 按
>   `report.repaired ? repairedNotice : recheckedNotice`（`ExtensionRow.tsx:313-330`、
>   `ExtensionsPanel.tsx:1255-1267`）；
> - `connected && state !== "broken" && !runtimeAvailable && (reconnectAvailable || homepage ||
>   runtimeSource === "system")` → **RefreshCw「Reconnect」**（`extensions_reconnect_system`）或
>   **Wrench「Install tool first」**（无 runtime 可找、有主页时去主页），tooltip 三态
>   （`reconnect` / `reconnectUnboundHint`），见 `ExtensionRow.tsx:345-370`。
>
> 本构建 **`grep -rn "econnect" internal/` 只命中一句 i18n 文案**：既没有 `extensions_reconnect_system`，
> 也没有 `extensions_repair`，`settingsui.Integration` 里连 `RuntimeAvailable`/`Homepage` 都没有。
> 后果是一条**死路**：系统工具被重装到别处（或从 PATH 消失）时，行上只有「Check」（`diagnoseIntegration`
> → `Reprobe`），而 `Reprobe` 用的是**记录的绝对路径**，解析失败就只把 `binding-missing` 记进条目、
> 把行标成 broken——用户在界面上没有任何入口能让它重新去 PATH 找一次。
>
> **F4e 修法**：
> - `internal/extensions/reconnect.go`（新）：`Reconnect(paths, id)` —— 原版 `extensions_reconnect_system`
>   的等价物。**先故意忘掉记录的 `ExecutablePath` 再解析**，因为 reconnect 的语义就是「程序搬走了/没了，
>   重新问一遍 search path」（`manifest.runtime.executableNames` 走 `lookTool`）；成功则改写 `executablePath`
>   并清掉**绑定类**失败（`binding-changed`/`binding-missing`/`runtime-unavailable`/`binding-check-failed`），
>   把行的状态交回给开关自己说（enabled/disabled）；失败则按 `recordBindingFailure` 把带 keyed detail 的
>   原因写回条目，让行用用户的语言说清楚。新增 `RuntimeOwnershipSystem` 常量（`install.go` 同步使用），
>   非 system 归属一律 `ErrNotSystemOwned`（原版 `reconnect_system_locked` 同样只认 system）。
> - `internal/settingsui`：`Integration` 新增 `RuntimeAvailable` 与 `Homepage`；新增 `repairKind()`
>   （三态：`repairReconnect` / `repairHomepage` / `repairNone`）、`repairLabel()`、`repairTooltip()`；
>   卡片动作行按它画按钮（原版那个按钮的三个分支）；新增 actions `ReconnectIntegration`/`OpenHomepage`。
> - `internal/shell`：`integrationList` 用 `extensions.ResolveRuntime` 投影 `RuntimeAvailable`、用
>   `manifest.Homepage` 投影 `Homepage`；新增 `reconnectIntegration`（先 `Refresh` 再 toast，照原版
>   `await refreshAfterMutation()` → `showSuccess` 的顺序）与 `openHomepage`（走已有的 `openURL` 缝，
>   默认浏览器）。i18n 新增 `IntegrationsReconnect`（“Reconnect”/“重新连接”）、
>   `IntegrationsReconnectUnboundHint`（原版 470-471/1357-1358 原文）、通知
>   `IntegrationReconnected`。
> - **记录一处有意偏离**：原版在 `state === "broken"` 时**隐藏**这个按钮（因为原版另有 Wrench「Recheck」
>   的 `extensions_repair` 接手）；本构建没有 repair，若照抄这层守卫，被自己的 Check 标成
>   `binding-missing` 的行反而**永久失去**唯一的修复入口。故 `repairKind` 对**绑定类**失败保留修复
>   （探测类失败仍显示诊断、不给修复），理由写进了函数注释。
>
> **验收**：新增 `internal/extensions/reconnect_test.go` 七条（搬到新路径并改写 `executablePath`；清掉绑定失败
> 并把状态交回开关，含 disabled 保持 disabled；找不到时记录 keyed `runtime-unavailable` 且**保留**旧的
> 过期路径；什么都没变时不写盘（`updatedAt` 不动）；非 system 归属 `ErrNotSystemOwned` 且不碰条目；未知 id
> `ErrNoIntegration`；探测类失败 `probe_failed` 不被清掉）。新增 `internal/settingsui/integrationrepair_test.go`
> 两条（七行表钉三态与 en/zh 两句 + tooltip；四条应画的/两条不应画的行同屏**计数**断言 + 点击分别落到
> 两个 action + action 为 nil 时不画死按钮）。新增 `internal/shell/reconnect_test.go` 三条（端到端：投影→动作→
> 条目被改写→页面不再需要修复、通知是「已重新连接」；找不到时通知失败句且路径保留；`openHomepage` 交给
> 默认浏览器且空串不开）。变异核查：把 `repairLabel` 的 homepage 支改回 `IntegrationsReconnect` →
> 两条立刻红（`repairLabel(en) = "Reconnect", want "Install tool first"`；像素条 `3 Reconnect buttons, want 2`）。
>
> **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 27 包全绿
> （1209 项）、`GOOS=linux/windows/darwin go build ./...` 三平台通过。
>
> **清单状态**：F4e（本轮的修复入口）落地；文末「具体待办（P7-el 重写）」把 F3a/F3b 与新查出的 F4 残留
> 拆成五条带 file:line/原版对照/验收标准的具体项，下轮最高优先级是 **F3a（控件透镜，原版有一份
> `tests/glass-reaxis.test.ts` 的 lens census 可逐条对齐）**。

> **上一轮：P7-ek F4c detected 行的动作回到原版三态 + 目录接通安装命令
> （原版 `src/extensions/ExtensionRow.tsx:296-302`、`src/ExtensionsPanel.tsx:2552-2574`、
> `src/extensions/tool-rows.ts:202-212`）** —— P7-ej 后清单只剩 F3a/F3b（F3b 已被实测标为
> 「先决于 F3a」）与 F4c（「待做；动作三态在本构建退化，先决于数据投影」）。F3b 明确不能直接落地，
> 故本轮取 **F4c**。依赖检查：GitHub releases API 限流，`go list -m -versions github.com/egoist/mygo`
> 最新仍是 **v0.4.8**（与 `go.mod` 一致），无版本可升；只对照 `main`
> （`backup/main-pre-mygo` = v0.3.14 原版树），未翻历史提交。
>
> **F4c 偏离**：原版同一张检测卡片的右侧按钮有**三态**——`runtimeAvailable` → Connect；否则
> `installInTerminal`（目录里有本平台配方）→ 在终端里跑安装命令；否则 → 去主页，且
> `disabled = busy || (!runtimeAvailable && !homepage && !installInTerminal)`
> （`ExtensionRow.tsx:296-302`）。本构建 `internal/settingsui/settingsui.go` 的 `detectedSection`
> 每行只有一个恒为 `IntegrationsConnect` 的按钮，既没有安装态也没有禁用态。
>
> **F4c 修法（取「接通目录」这一支）**：
> - `settingsui.DetectedTool` 新增 `RuntimeAvailable` 与 `InstallCommand` 两个字段；
> - 新增 `detectedActionFor(tool)`（原版三态选择，`runtimeAvailable` 优先）与
>   `detectedLabel(copy, kind)`（Connect / Install in terminal / Install tool first，原版
>   `i18n.ts:465,474,477` 的同一批词）；`detectedSection` 的行改走新的 `detectedAction`——
>   可运行 → Connect；不可运行但有配方 → `InstallDetected(command)`；都没有 → 画成禁用。
> - `internal/shell/shell.go` 的 `detectedTools` 为每个候选算出安装命令：候选没有目录 id，改以
>   **可执行文件名**去匹配 `tools.Catalog` 的 `Probes`（目录自己用来探测工具的那批名字），再用
>   `tools.ChosenRecipe` + `tools.InstallCommand` 取本机有的包管理器配方；新 action
>   `InstallDetected` 接到 `a.openShellCommand`（打开终端面跑那条命令，正是原版 `onInstallInTerminal`）。
> - i18n 新增 `IntegrationsInstallInTerminal`（“Install in terminal”/“在终端中安装”）、
>   `IntegrationsInstallTool`（“Install tool first”/“请先安装工具”）。
> - **本构建为何只退化成「可判但少见」**：`detectedTools` 只列出 `candidate.Available` 的项，
>   所以生产里 detected 行基本都走 Connect；`Install in terminal` 态由**数据**（同一探针指向一个不可
>   运行的文件）驱动，可判且已测。「本机没有、目录能装」的历史落点其实是启动器的 install 行
>   （`internal/launcher` 的 toolInstallRows），原版把它和 detected 塞进同一列表是因为原版没有那层。
>
> **验收**：新增 `internal/settingsui/detectedaction_test.go`——`TestTheDetectedActionFollowsTheRow`
> 四行表（含「可运行胜过有配方」）钉 `detectedActionFor`/`detectedLabel` 的 en/zh 两句；
> `TestTheDetectedRowsCarryTheirThreeActions` 同屏三行，像素上钉三个词都在，点「Install in terminal」
> 把命令原样交给 action、点 Connect 连对的那行、点禁用词则两个 action 都不动。新增
> `internal/shell/detected_test.go` 五行表钉 `detectedInstallCommand`（有管理器/无管理器回退第一配方/
> Windows 去 `.exe`/目录不认识/平台无配方）。变异核查：把 `detectedLabel` 的
> `detectedInstallInTerminal` 支改回 `IntegrationsConnect` → 两条测试立刻红
> （`detectedLabel(en) = "Connect", want "Install in terminal"`）。
>
> **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 27 包全绿
> （1189 项）、`GOOS=linux/windows/darwin go build ./...` 三平台通过。
>
> **清单状态**：F4 全部（F4a–F4d）落地；剩 **F3a/F3b**（F3b 先决于 F3a，见下）。

> **上一轮：P7-ej register 深链高亮标记 + Detected/register 连接真的落地
> （原版 `src/extensions/ExtensionRow.tsx:196-205,290`、`src/ExtensionsPanel.tsx:1500-1548,2516-2528`、
> `src-tauri/src/deep_link.rs:389`）** —— 「尚未做」清单里 F3b 是最高优先级，本轮先按 F3b 的验收
> 去**实测**「帧 sheen 改固定、档位差异全靠控件透镜」是否成立，结论否（下详），故不动 F3b，
> 转而落地同组的具体项 **F4d**。依赖检查：GitHub releases API 限流，
> `go list -m -versions github.com/egoist/mygo` 最新仍是 **v0.4.8**（与 `go.mod` 一致），无版本可升；
> 只对照 `main`（`backup/main-pre-mygo` = v0.3.14 原版树），未翻历史提交。
>
> **F3b 实测（为什么不落地）**：F3b 的前提是「档位差异全部落到控件透镜」。把 `theme.go` 的帧 sheen
> 临时改成固定 token（`sheen = 1`）后，用生产帧扫全画布：在帧专属带（y=90..120，不含任何控件）
> frosted/regular/liquid 的 RGB sum 为 dark 100.3/100.3/100.5、light 397.2/397.8；再以 20px 为步长
> 逐带扫描，regular 与 liquid 的 RGB sum 差**处处 ≤0.4**。也就是说控件透镜在本构建渲染里几乎不
> 贡献像素（`launcher` 只在搜索框 1px 内影与图标底板上用 lens），固定帧 sheen 后三档在**没有窗口
> 材质**（Linux / 无头）时完全一样——会重演用户报过的「玻璃 3 档看不出区别」。故本轮**保留**现有
> 「帧 sheen 随档位」（P7-c/P7-d 为补 Linux 无材质而做的偏离），把证据写回 F3a/F3b 待办，改为
> 「先决定档位 rung 归帧还是归控件、并给控件透镜可测的像素落地」。
>
> **F4d 落地**：
> - i18n 新增三键 `RegisterHighlight`（原版 `registerHighlight` 原文）、
>   `RegisterNotFound(name)` / `RegisterAlreadyConnected(name)`（原版 `i18n.ts:437-438`）。
> - `internal/settingsui`：`Integration` 增 `Highlighted`；`integrationRow` 在被点名时用
>   `rowNote` 画出 `RegisterHighlight`（原版是名字前的 `extension-row__register-dot`；原生面板没有
>   tooltip，故用可见文案当标记）。
> - `internal/shell`：`registerTarget` + `registerTimer` + `registerHighlight`；`markRegisterTarget`
>   记下路径并按 `registerHighlightWindow`（5s，原版 `useTimedReset` 默认 3s，这里放宽是因为卡片要等
>   异步连接落地）自清；`hasExecutable`/`sameExecutable` 判「是否已连接」（按 `ExecutablePath`，符号
>   链接两侧对齐）；`registerNotice` 把未命中/已连接各说一句（原版 `registerMiss`）。`registerFromLink`
>   不再只 `log`，`integrationList` 按路径把对应卡片标 `Highlighted`。
> - **顺带修的真 bug**：`connectDetected` 构造的 `CustomTool` 从来没带 `Platforms`，于是
>   `PrepareCustom` 一律报 `platformRequired`——Detected 的「Connect」与 `floter://register` 其实
>   **从未连接成功**（日志里满是 `could not prepare … platformRequired`）。新增
>   `settingsui.CurrentPlatform()` 并在 `connectDetected` 填入本机平台。
>
> **验收**：新增 `internal/settingsui/integrationregister_test.go`（像素：只有被点名的卡片画
> `RegisterHighlight`，且全屏只一次）；新增 `internal/shell/register_test.go`（未命中给
> `RegisterNotFound` 且不设 target；命中后连接落地、卡片被标记，二次 link 给
> `RegisterAlreadyConnected`；标记是 moment，到点自清；`sameExecutable` 的符号链接表）。
> 变异核查：把 `integrationRow` 的 `highlight` 摘掉 → 像素条红；把 `connectDetected` 的 `Platforms`
> 摘掉 → 连接失败、`TestTheRegisterLinkMarksTheProgramItNamed` 红。
>
> **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 27 包全绿
> （1178 项）、`GOOS=linux/windows/darwin go build ./...` 三平台通过。
>
> **清单状态**：F4 全部（F4a–F4d）落地；剩 **F3a/F3b**（已按上表重写为带实测证据的待办）。

> **上一轮：P7-ei F4b 集成卡片的 meta 行补回「集成种类」词
> （原版 `src/extensions/ExtensionRow.tsx:77-101,220-222,255`、`i18n.ts:446-453`）**
> —— P7-eh 之后「尚未做」里 F3b 与 F4b 都是具体项，本轮取 **F4b**（用户四项反馈里 F4 的
> 第 2 条；与 F4a 同属「集成组件逻辑差异」这组，紧接上一轮）。依赖检查：GitHub releases
> API 限流，`go list -m -versions github.com/egoist/mygo` 最新仍是 **v0.4.8**（与 `go.mod`
> 一致），无版本可升；只对照 `main`（`backup/main-pre-mygo` = v0.3.14 原版树），未翻历史提交。
>
> **偏离点**：原版每张卡片的 meta 行画两个槽——包身份（`packageName ?? publisherName`）与
> **类型词** `t(integrationKindKey(extension))`（`ExtensionRow.tsx:220-222`；`integrationKindKey`
> 在 `:77-94` 七分，词条在 `i18n.ts:446-453` 的 `settings.extensions.integrationKind.*`）。
> 本构建 `internal/settingsui/settingsui.go:1505` 的 meta 行只印 `integration.identity()`
> （publisher · version · ID），**类型词整格缺失**——一张系统工具卡与一张 npm 集成卡的第二行除了
> id 没有任何区别。
>
> **修法**：`settingsui.go` 新增 `Integration.integrationKind(copy)` 与 `identityLine(copy)`
> （identity 在前、类型词在后，两格都是原版自己的槽位与顺序）；卡片那句 `a.row(...)` 改读
> `identityLine(copy)`。类型词与移除动词（F4a）是同一行的两个不同事实：系统卡片同时读
> “System tool” 与 “Disconnect”。i18n 新增四键 `IntegrationsKindCustom` / `KindNPM` /
> `KindSystem` / `KindPackage`（en/zh 用原版原文）。
>
> **为何是四键不是七键**：原版 `integrationKindKey` 的 `recommended` / `manifest` / `tool` 三支都要求
> `!connected`，而本构建把这三类行放在 **Detected / Recommended 两个独立段**、用各自的渲染器
> （`DetectedTool` / `RecommendedTool`）而非 `Integration` 卡片——所以在「卡片是 connected」这一
> 前提下七分退化为四分（custom / npm / system / package）。把另三个词也声明出来会留下三枚无人读的
> i18n 键（`TestNoDeadCopy` 会红），故只加被读的四个；段合并是 F4c 的独立决定。
>
> **验收**：新增 `internal/settingsui/integrationkind_test.go` 三条——
> `TestTheKindWordFollowsTheIntegrationType`（六行表：custom / generated-beats-npm / npm / system /
> bundled / local；并钉类型词 ≠ 移除动词）、`TestTheAppIdentityLineCarriesTheTypeWord`
> （identity · kind 的拼接与空身份不留尾分隔符）、`TestEachIntegrationCardShowsItsOwnKind`
> （四行同屏像素断言四个词都在，且系统卡仍画 Disconnect）。变异核查：把卡片的
> `identityLine(copy)` 换回 `identity()` → 第二条（像素）立刻红
> （`the page is missing "Custom integration"…`）。
>
> **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 27 包全绿、
> `GOOS=linux/windows/darwin go build ./...` 三平台通过。
>
> **清单状态**：F4 进展为 F4a/F4b 已落地，剩 F4c（detected 行动作三态）/F4d（register 标记）；
> F3a/F3b 仍在清单上。下轮最高优先级是 **F3b（帧 sheen 回固定口径）** 或 F4c。

> **上一轮：P7-eh F4 逐函数对照落成四条具体待办 + F4a「移除动词随 kind」落地
> （原版 `src/extensions/ExtensionRow.tsx:109-114,409-418`）** —— P7-eg 之后「尚未做」里
> F3a/F3b 已是具体项，唯 F4 还停在「审计入口已钉、逐项待下一轮」，按规则把 F4 写成**带 file:line /
> 原版行号 / 验收标准的四条待办**（F4a–F4d，文档末尾），再取第 1 条（F4a）落地。依赖检查：
> GitHub releases API 限流，`go list -m -versions github.com/egoist/mygo` 最新仍是 **v0.4.8**
> （与 `go.mod` 一致），无版本可升；只对照 `main`（`backup/main-pre-mygo` = v0.3.14 原版树），
> 未翻历史提交。
>
> **F4a 偏离**：原版 `removalKind`（`ExtensionRow.tsx:109-114`）把集成分成 custom / npm / system /
> package 四种，`:409-418` 用它选菜单文案（`deleteCustom` / `uninstall` / `disconnect` /
> `removePackage`，`i18n.ts:521-540`）。本构建三个移除入口一律说 `IntegrationsUninstall`
> （`internal/settingsui/uninstall.go:27,45`、`settingsui.go:1698-1701`）：`system` 行的运行时
> 归系统、包不是 floter 的，`local` 链入包的文件也不在 floter 目录里——一个词说了三件不同的事。
> 数据侧还缺字段（`settingsui.Integration` 没有 `distributionSource`/`runtimeOwnership`）。
>
> **F4a 修法**：`internal/settingsui/settingsui.go` 的 `Integration` 新增 `DistributionSource`、
> `RuntimeOwnership` 并加 `removalKind()`（原版逐行，`generatedCustom` 优先）与 `removalLabel()`
> （custom 复用已有的 `RemoveDelete`，正是原版 `deleteCustom` 的同一句话）；三处入口改读它；
> `internal/shell/shell.go` 的 `integrationList` 投影两个 Entry 字段；i18n 新增 `IntegrationsDisconnect`、
> `IntegrationsRemovePackage`（en/zh）。**验收**：新增 `internal/settingsui/integrationremoval_test.go`
> 两条（六行表 + 四行同屏像素断言）。变异核查：`removalLabel` 的 system/package 改回
> `IntegrationsUninstall` → 两条红。
>
> **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 全绿、
> `GOOS=linux/windows/darwin go build ./...` 三平台通过。
>
> **清单状态**：F4 由“不具体”变成 F4a（已落地）/F4b/F4c/F4d 四条具体待办；下轮最高优先级是
> F3b（帧 sheen 回固定口径）或 F4b（meta 行种类词）。

> **上一轮：P7-eg 用户四项实测反馈写实 + F1 落地（原版 `src/launcher/terminal-row.ts:77-84`
> R60/R64）** —— 「尚未做」清单已空（P7-ee 重写的 1–3 全部落地、第 4 条是记录），本轮按规则
> 把用户新报告的 F1–F4 **逐条写实**为带 file:line / 原版对照 / 验收标准的待办（文档末尾
> 「用户实测反馈」一节），再取第 1 条落地。依赖检查：GitHub releases 被限流，
> `go list -m -versions` 与 `go.mod` 均为 **v0.4.8**，无版本可升；只对照 `main`
> （`backup/main-pre-mygo` = v0.3.14），未翻历史提交。
>
> **F1（搜索框输入后按住 Super → 底部「打开终端」行）**：原版 R64 的 `bareTerminalPlacement`
> 把行窄化成「空字段才画」，本构建此前照做并把它钉成参考行为；用户在最新报告里要求更宽的手势。
> 按「用户最新要求优先」恢复 **R60 的输入态追加**：按住 App 修饰键时，`heldTerminalItem` 不再因
> 字段非空而返回 false，`Results()` 在动作栏与匹配行之后把它追加为**最后一行**（动作栏的存在性
> 闸门只读不含该行的 `other`）。file:line：`internal/launcher/launcher.go` 的 `heldTerminalItem`
> 去挡板、`internal/launcher/search.go` 的 `Results`/`otherResults` 调整追加位置。验收：
> `internal/launcher/filters_test.go` 的 `TestHeldModifierBareTerminalRow` 末段（先键入再按住）
> 与 `internal/shell/heldmodifier_test.go` 的 `TestHeldModifierRowStaysOverAQuery`（旧 `…QueryGate`
> 已改写）。变异核查：把 query 挡板加回 → 两条红。
>
> **F2 复核**：编号随滚动视口已由 P7-d 落地（`assignNumbers` + `resultnumbers_test.go`），本轮
> 逐行核对 `result-budget.ts:883-912` 后判定「键滞后一帧」是即时模式与原版 React 的同一行为，不是
> 偏差；把「去一帧」记为可选后续。**F3/F4 拆成可执行子项**（F3b 的帧 sheen 耦合 file:line 已钉、
> F4 的审计入口 file:line 已钉）留待后续轮次。
>
> **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 全绿、
> `GOOS=linux/windows/darwin go build ./...` 三平台通过。

> **最新一轮：P7-ef 剪贴板模式的「已关闭」档 + 计算器模式层的抓取深度
> （原版 `src/hooks/useLauncherCatalog.ts:814-816`、`src/plugins/clipboard/mode.ts:34-56`、
> `src/plugins/status.ts`、`src/launcher/LauncherResults.tsx:749-760`、`src/App.tsx:2165-2166`；
> 以及 `src/hooks/useLauncherCatalog.ts:871-878`、`src/calculator.ts:250`、
> `src/plugins/calculator/mode.ts:79-80`）**
> —— P7-ee 重写的清单第 1 条已落地，本轮取第 2 条（剪贴板「已关闭」档，最高优先级）；
> 第 3 条（计算器模式层上限）原本要先判定值不值得动手，本轮一并结清并把它钉成测试。
> 依赖检查：`curl .../releases/latest` 被限流，`go list -m -versions github.com/egoist/mygo`
> 最新仍是 **v0.4.8**（与 `go.mod` 一致），无版本可升；只对照 `main`
> （`backup/main-pre-mygo` = v0.3.14 原版树），未翻历史提交。
>
> **偏离点 1（剪贴板「已关闭」是相反的话）**：原版 `useLauncherCatalog.ts:814-816` 在插件关闭时
> **先于任何读取**返回一条 `clipboardStatusRow("clipboard-disabled", "clipboard.pageUnavailable")`
> （词条 `i18n.ts:291` “Clipboard history is off”），模式自己说发生了什么。本构建改前
> `internal/shell/shell.go:792` 无条件建 store，插件开关只影响**监听**（`internal/shell/system.go:274`），
> 于是关掉插件后仍能进模式、仍列出历史，空态走 `internal/launcher/launcher.go:585-591` 的
> `ClipboardEmpty`（“暂无复制记录”）——一句与事实相反的话。浏览器模式早有对应档
> （`BrowserResults.Disabled` → `BrowserDisabled`），剪贴板缺失。
>
> **修法 1**：
> - `internal/launcher/search.go` 新增导出 `ClipboardDisabled = "clipboard-disabled"`（原版 id 逐字）
>   与 `Item.status`；`clipboardItems` **先读设置**（`settings.ClipboardOf`）——store 无论如何都在，
>   所以「模式信任接口是否存在」会列出一份 app 已停止记录的历史——关闭时返回这一条 status 行：
>   `Title: a.copy().ClipboardDisabled`、`status: true`、**没有 `Run`**。
> - `internal/i18n`：新键 `ClipboardDisabled`（en/zh），`TestNoDeadCopy`/完整性测试守卫。
> - **status 行的绘制**（此前只有 captured-output 列表有 status 行）：`Item.status` 经
>   `windowRows`（`internal/launcher/output.go`）设 `RowInfo.Status`，`Geometry.Height` 早已支持
>   该档（`internal/launcher/height.go:109-111,227-230` 的 `launcherStatusUnits = 7.5` = `LAUNCHER_STATUS_UNITS`）；
>   `internal/launcher/launcher.go` 的 `row()` 新增该支：**不画图标底板**、28u 图标列放 lucide 的
>   `Info`（新增 `glyphInfo`，原版 `.launcher-status__icon` 的 `<InfoIcon size={14}/>`）、`--text-tertiary`
>   里的图标 + muted 的 480 字重一行，`MinHeight(launcherStatusUnits)`。
> - **键盘不可达**：新 `rowSelectable(item) = !heading && !status`，替换四处只看 `heading` 的判断
>   （`nextSelectable`、`firstSelectable`、`defaultSelection`、`clampSelection`），这样箭头不会停在
>   note 上、数字族也不会给它编号（原版 `kind: "status"` 即「无 ⌘N、无指针态、无 Enter」）。
>
> **验收 1**（新增 `internal/launcher/clipboardoff_test.go`，4 条）：
> `TestTheClipboardModeSaysWhenTheHistoryIsOff` 在 3 条历史 + 关闭开关下进入模式：只有这一条 status 行、
> 历史三行都不在、文案 = `ClipboardDisabled`、不是 `ClipboardEmpty`、`Run == nil`、`a.numbers[0]` 不存在；
> `TestTheClipboardNoteOnlyAppearsWhenTheHistoryIsOff` 钉开关打开时同屏列历史且不出现 note；
> `TestTheClipboardOffNoteIsChargedTheStatusBand` 钉 `a.Rows[0].Status`、**不**是 `Heading`、
> 且价格比一行结果行低 `(rowOneLineUnits - launcherStatusUnits) × spacing`、`HeldRows == 1`；
> `TestTheClipboardOffNoteSurvivesNoStore` 把 `a.Clipboard` 置 nil，note 仍画得出（原版也没读 store）。
>
> **偏离点 2（计算器模式层没有上限）**：原版 `useLauncherCatalog.ts:871-878` 给
> `calculatorModeRows` 传 `MAX_CALCULATOR_MAX_ITEMS`（`calculator.ts:250` = 500），模式在 chip 过滤**之后**
> `matched.slice(0, limit)`（`plugins/calculator/mode.ts:79-80`）；`CALCULATOR_FETCH_LIMIT = MAX_RESULTS`（10）
> 只是输出视图那盒子的分组上限。本构建 `internal/launcher/calculator.go` 改前遍历 `a.calculatorFound` 全量、
> 不设上限；`internal/calculator` 的 `maxItems` 只约束**非收藏**项，而收藏永不淘汰，极多收藏的历史会比原版多出行。
>
> **修法 2**：新增导出 `CalculatorFetchLimit = calculator.MaxMaxItems`（500，注释指向两处原版行号，
> 并写明两个天花板为何是同一个数）；`calculatorItems` 的 chip 过滤**保持在前**，过滤后再
> `history >= CalculatorFetchLimit` 收手（与 P7-ec/P7-ee 同一口径）。
> **验收 2**（新增 `internal/launcher/calculatorcap_test.go`，2 条）：
> `TestTheCalculatorModeHoldsTheReferenceFetchDepth` 钉常量 == 500 == `calculator.MaxMaxItems`，
> 600 条下模式恰好 500 行、最新行在最前；`TestTheCalculatorChipFilterSeesTheWholeHeldSet` 把唯一
> 收藏（且最老）摆在 600 条之后，钉收藏 chip 仍拿到它。变异核查：把遍历改成
> `a.calculatorFound[:CalculatorFetchLimit]`（上限先花）→ 第二条红
> （`the favourites view holds 0 rows, want 1`）。
>
> **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 全绿、
> `GOOS=linux/windows/darwin go build ./...` 三平台通过。
>
> **清单状态**：P7-ee 重写的第 2·3 条至此落地，第 4 条是「十行翻页不做」的记录。下一轮回到
> 「逐文件对照原版排查偏离点」或按新用户报告定入口。

> **最新一轮：P7-ee 剪贴板模式按原版的抓取深度持有结果，且上限不再切在 chip 之前
> （原版 `src/hooks/useLauncherCatalog.ts:833-834`、`src/clipboard-history.ts:440`、`src/plugins/clipboard/mode.ts:79-95`）**
> —— P7-ed 收尾后「尚未做」又一次为空（只剩 P7-ec 第 4 条「十行翻页不做」的记录），本轮按惯例换入口做
> 「逐文件对照原版」的审计，落在**内置插件模式层持有多少行、上限与 chip 过滤谁先花**（P7-da 审过计算器
> 历史的存储与设置块，没审过模式层的持有量）。先把清单重写成 3 条带 file:line 的待办（见「尚未做」），
> 再取第 1 条落地。依赖检查：`curl .../releases/latest` 被限流，`git ls-remote --tags
> github.com/egoist/mygo` 与 `go.mod` 均为 **v0.4.8**，无版本可升；只对照 `main`
> （`backup/main-pre-mygo` = v0.3.14 原版树），未翻历史提交。
>
> **偏离点（两处，同一文件）**：
> 1. **持有深度**：原版内联启动器把 `MAX_CLIPBOARD_MAX_ITEMS`（`src/clipboard-history.ts:440` = 500，
>    正是历史自己的 `max_items` 天花板）当模式层的抓取深度传给 `clipboardModeRows`/`clipboardSearchRows`
>    （`useLauncherCatalog.ts:833-834`）；模式自己的 `CLIPBOARD_FETCH_LIMIT = MAX_RESULTS`（10，
>    `plugins/clipboard/mode.ts:34`）只是**插件输出视图**那个固定高度盒子的分组上限。本构建改前只有
>    `internal/launcher/search.go` 的 `maxClipboardResults = 50`（改前行号 `:351`/`:437`），既是
>    `Store.Search` 的 limit、也直接当了模式持有量：空查询最多 50 行（原版 500），比历史容量（默认 300）
>    还浅。这个 50 全仓无出处，也没写进任何待办或文档。
> 2. **过滤顺序**：原版 `clipboardModeRows` 先过 needle、再过 chip、最后才 `matched.slice(0, limit)`；
>    本构建把上限花在 `Store.Search` 里（chip 之前），于是命中数超过 50 时 `images`/`favorites` 这类
>    chip 会看不到本该存在的行——正是 P7-ec 给浏览器修掉的同一个毛病。
>
> **修法（决定：不翻页，保留滚动，与 P7-ec 同一口径）**：`internal/launcher/search.go`
> - 新增导出常量 `ClipboardFetchLimit = clipboard.MaxMaxItems`（500），注释逐字指向
>   `useLauncherCatalog.ts:833-834` 与 `clipboard-history.ts:440`，并写明为何只取两个天花板里的这一个
>   （十行是输出视图的补丁，本构建列表自己滚动）；
> - `clipboardItems` 改成 `a.Clipboard.Search(a.clipboardQuery(), 0)`（只上 needle，不花上限）→
>   chip 过滤（`clipboardFilterAllows`）→ `len(out) >= ClipboardFetchLimit` 收手（对应原版的
>   `matched.slice(0, limit)`）；
> - `pruneThumbs` 的调用从「刚 Search 出来的行」挪到循环之后，keep 集合是**这一帧真正建出的行**
>   （对应原版 R97 传渲染自己的 id），依旧只在 memo 超过 `clipboard.MaxMaxItems` 时收紧。
>
> **测试**（新增 `internal/launcher/clipboarddepth_test.go` 两条）：
> `TestTheClipboardModeHoldsTheReferenceFetchDepth` 钉 `ClipboardFetchLimit == 500 ==
> clipboard.MaxMaxItems`，并在 600 条历史（容量 500，最旧的 100 条被老化裁掉）下钉模式恰好持 500 行、
> 最新行在最前；`TestTheClipboardChipFilterSeesTheWholeHeldSet` 把唯一的图片（且最老、靠收藏豁免老化）
> 摆在 600 条文本之后，钉 `images` chip 仍能拿到它。变异核查：把常量换回 50 → 第一条红
> （`ClipboardFetchLimit = 50, want 500`）；把上限挪回 `Store.Search` 里（上限先花）→ 第二条红
> （`images chip shows 0 rows, want 1`）。
>
> **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 全绿（本机
> `go test -v ./... | grep -c -- "--- PASS"` = 1150 项，含本轮新增 2 项）、
> `GOOS=linux/windows/darwin go build ./...` 三平台通过。
>
> **清单状态**：P7-ee 重写的第 1 条至此落地；第 2 条（剪贴板模式的「已关闭」档）是下一轮最高优先级，
> 第 3 条（计算器模式层上限）低优先级且需先判定。依赖检查同上，无版本可升。

> **上一轮：P7-ed 插件配置的值规范化补齐原版 `normalizeConfigValue`（原版 `src/plugins/config-schema.ts:448-500`）**
> —— P7-ec 收尾后「尚未做」只剩第 3 条（`internal/plugincfg` 的值规范化与原版有四处口径差），它正是本轮
> 最高优先级的具体项。依赖检查：`curl .../releases/latest` 与 `go.mod` 均为 **v0.4.8**，无版本可升；
> 只对照 `main`（`backup/main-pre-mygo` = v0.3.14 原版树），未翻历史提交。
>
> **原版对照**：`main:src/plugins/config-schema.ts:448-500` 的 `normalizeConfigValue` 是**全函数**，
> 同时用在写入（`applyConfigChange`）与加载（`configValues`）上：text → `trim` 后空为 `null`；
> slider/number → 非有限回退 `field.min`，再 `clampStep`（按 `step` 从 `min` 对齐、clamp、`Math.round(x*1e6)/1e6`）；
> select/radio → 未知值回退第一个选项；checkboxes → 过滤、**去重并按 schema 选项顺序**。
>
> **本轮决定（保留拒绝分支）**：本构建的 `coerce`（`internal/plugincfg/schema.go`）是**偏函数**——形状不符或
> select/radio 未知值一律拒绝、保留旧值，这是 `TestApplyRejectsAValueOutsideTheOptions` 钉住的有意行为
> （「控件不能写一个它自己读不出来的值」）。改回原版的回退语义会推翻那条测试，属于独立决定；本轮按待办给出的
> 分支**保留拒绝**，只把**不与现有测试冲突的三处**补齐，避免半改。
>
> **修法**（`internal/plugincfg/schema.go` 的 `coerce`）：
> - **text 现在 `strings.TrimSpace`**，全空白返回 `nil`（原版的 `null`：没有值，而不是空串）；
> - **slider/number 走新的 `clampStep(n, lo, hi, step)`**：`step <= 0` 取 1，先 `math.Round((n-lo)/step)*step+lo`
>   对齐到锚在 `min` 的步进网格，再 `clamp`，最后 `math.Round(x*1e6)/1e6`（原版对浮点步进留的
>   `0.30000000000000004` 补丁）；非有限数回退 `min`（与 `asFloat` 接受的 float 形状不冲突，形状拒绝仍保留）；
> - **checkboxes 先去重成集合、再按 `f.Options` 的顺序输出**（原版 `field.options.map(...).filter(picked.includes)`），
>   未知项照旧丢掉。
>
> **测试**（`internal/plugincfg/schema_test.go`）：
> - `TestCoerceShapesValuesLikeTheReferenceNormalizer`（10 行表）钉 trim/空白为 nil/向下对齐/向上对齐/
>   对齐后仍 clamp/NaN→min/浮点六位舍入/去重排序/未知项丢弃/全无效为空。
> - `TestTheShippedValuesAlreadyFitTheSchema` 遍历三个插件的初始 `Values`，钉**加载路径**交给 schema 的值
>   本来就过 `coerce`、且 shipped 的 bounded number 已在步进网格上——即加载路径不需要额外规范化
>   （typed settings 已保证形状），这半条由此有据。
> - 变异核查：把 `clampStep` 换回 `clamp`、把 trim 摘掉、把 checkboxes 改回按输入顺序保留重复——三次
>   对应子用例分别红（`snaps_down/snaps_up/non-finite`、`a_text_value_is_trimmed/a_whitespace-only...`、
>   `checkboxes_de-duplicate...`）。
>
> **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 全绿（1154 项）、
> `GOOS=linux/windows/darwin go build ./...` 三平台通过。
>
> **清单状态**：P7-ec 重写的第 3 条至此落地；「尚未做」本轮为空（仅剩一条记录性条目——原版十行翻页本构建不做）。

> **上一轮：P7-ec 浏览器模式按原版的抓取深度持有结果（原版 `plugins/browser/mode.ts:70,84,199,218`）**
> —— P7-eb 收尾后「尚未做」只剩两条：本轮取最高优先级的第 1 条（浏览器结果的数量上限与抓取深度
> 不对齐原版）。依赖检查：GitHub releases API 限流，`proxy.golang.org/.../@v/list` 最新仍是
> **v0.4.8**（与 `go.mod` 一致），无版本可升。只对照 `main`（`backup/main-pre-mygo` = v0.3.14 原版树），
> 未翻历史提交。
>
> **偏离点**：原版 `main:src/plugins/browser/mode.ts` 有**两个**天花板。`BROWSER_FETCH_LIMIT = 500`
> （`:70`）是内联启动器拉取并持有的深度；`BROWSER_GROUP_LIMIT = MAX_RESULTS = 10`（`:84`，
> `launcher/result-budget.ts:46`）只是**插件输出视图**那个不能滚动的盒子自己的分组上限。内联模式把分组的
> `cap` 提到抓取深度（`useLauncherCatalog` 传 `limit: BROWSER_FETCH_LIMIT`），再用 `paginatePluginRows`
> 每次翻十行显示（`:199` 的合并组按 `cap` 截断、`:218` 的标签页组按 `cap * 2` 的**总数**截断）。本构建改前
> 只有一条 `internal/shell/system.go:251` 的 `browserResultLimit = 50`，同时当
> `browser.Options.Limit`（`internal/shell/shell.go:1590`，搜索合并后的**后置**截断，不是 SQL LIMIT）
> 与标签页上限（`:1605-1606`）——于是空查询的浏览器列表最多 50 行（原版 500），且截断发生在
> 启动器的范围 chip **之前**：书签一多，`history` chip 就什么都看不到。
>
> **修法（决定：不翻页，保留滚动）**：本构建的启动器列表是虚拟化可滚动的，原版的「每页十行」是它自身固定
> 高度盒子的补丁，所以**不复刻分页**（也不复刻输出视图的 `BROWSER_GROUP_LIMIT = 10`——那会藏住滚动本可到达
> 的行）；只把「模式持有多少行」的判据搬回模式层，与 `browserSearchRows` 同位：
> - `internal/launcher/search.go`：新增导出常量 `BrowserFetchLimit = 500`（注释逐字指向 `mode.ts:70`，
>   并写明为什么只保留两个天花板中的这一个）；`browserItems` 的合并组在 `len(out) >= BrowserFetchLimit`
>   时收手（对应 `mode.ts:199-200`）、标签页组在总数 `>= 2*BrowserFetchLimit` 时收手（对应 `mode.ts:218`）。
> - `internal/shell/system.go`：删掉 `browserResultLimit` 与 `browserOptions.limit`（后置截断搬走）。
> - `internal/shell/shell.go`：`browser.Search` 不再传 `Limit`（0 = 不截断；后端自己的每 profile SQL 窗口
>   `internal/browser/browser.go:411` 的 4000 行仍是读入上限，比原版的 500 更深），标签页也不再在 shell 里
>   截断——模式拿到完整匹配集后先过 chip、再按抓取深度持有。
>
> **测试**（新增 `internal/launcher/browsercap_test.go` 两条）：
> `TestTheBrowserModeHoldsTheReferenceFetchDepth` 注入 700 条历史 + 700 个标签页，先钉 `BrowserFetchLimit == 500`，
> 再钉空查询第一组恰好 500、标签页组 500、整表 1000（原版的 `cap` 与 `cap * 2`）；
> `TestTheBrowserTabGroupIsNotMergedWithTheFirstGroup` 把同一 URL 同时作为书签与标签页，钉两行都在、id 不同
> （原版 `mode.ts:90` 明确标签页组不与第一组合并）。变异核查：摘掉合并组的 `BrowserFetchLimit` 收手，第一条立刻红
> （`first group = 700 rows, want 500`）；摘掉标签页组的 `2*BrowserFetchLimit` 收手，立刻红
> （`live-tab group = 700 rows, want 500`）。
>
> **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 全绿（1136 项）、
> `GOOS=linux/windows/darwin go build ./...` 三平台通过。
>
> **残留**：原版的 `paginatePluginRows` 十行分页本构建不实现（列表自己滚动），已如实写进「具体待办
> （P7-ec 重写）」为「已决定不做」。清单只剩第 3 条（plugincfg 的值规范化口径）。

> **最新一轮：P7-eb 浏览器结果行的字形与副标题回到原版（原版 `LauncherResults.tsx:290-294`、`plugins/browser/mode.ts:195,208,216`）**
> —— P7-ea 收尾后「尚未做」清单又一次为空。本轮按惯例换入口（逐文件对照原版），落在**浏览器模式的
> 行内容**（`src/browser-page.ts` / `src/plugins/browser/mode.ts` / `LauncherResults.tsx`）——此前
> 未被任一待办覆盖。发现四处真实偏离并本轮全部落地。依赖检查：GitHub releases API 限流，
> `proxy.golang.org/.../@v/list` 最新仍是 **v0.4.8**（与 `go.mod` 一致），无版本可升。只对照 `main`
> （`backup/main-pre-mygo` = v0.3.14 原版树），未翻历史提交。
>
> **偏离点（四处，同一个列表的四张脸）**：
> 1. **历史记录的图标是地球，不是时钟**。原版 `LauncherResults.tsx:290-294` 的 `BrowserRowIcon`：
>    bookmark → `BookmarkIcon`、history → `ClockIcon`、live tab → `AppWindowIcon`，只有「来源未知」
>    才落到 `GlobeIcon`。本构建 `internal/launcher/glyphs.go:76-80` 的 `rowGlyph` 只判 `Kind == "bookmark"`，
>    其余（含 history）一律 `glyphGlobe`——时钟标（`glyphClock`）被定义为只服务最近使用。
> 2. **历史行的副标题多了访问时间**。原版 `plugins/browser/mode.ts:195` 给每条搜索结果
>    `subtitle: row.url`（纯 URL）；`last_visit`/`date_added` 只被 `browser-page.ts:37` 解析、
>    从不渲染（`git grep last_visit main -- src` 只有解析处）。本构建 `internal/launcher/search.go:246-248`
>    在 URL 后拼 `  ·  <YYYY-MM-DD HH:MM>`。
> 3. **标签页行的副标题多了 `· Open tab`**。原版 `mode.ts:208` 的 tab 行 `subtitle: tab.url`，靠
>    `AppWindowIcon`（R73 注释：「tab rows are told apart by their own glyph and subtitle」）区分；
>    本构建在 URL 后拼 `copy.BrowserTab`（`internal/launcher/search.go:252-256`）。
> 4. **标签页行的 id 用 URL，同一网址开在两个窗口会撞**。原版 `browser-page.ts:216` 的 `tabRowKey`
>    是 `tab:${browser_id}:${window_index}:${tab_index}`，注释写死「Position, not URL: the same page
>    can be open in two windows and the two rows must stay distinct」。本构建用的是
>    `"tab:" + tab.BrowserID + ":" + tab.URL`（`search.go:255`）——两条同 URL 的标签页拿到同一个 id。
>
> **修法**：
> - `internal/launcher/glyphs.go`：`rowGlyph` 的 `item.web` 分支改成按 `browser.KindBookmark` /
>   `browser.KindHistory` 分派（bookmark → 书签、history → 时钟、其余 → 地球）。
> - `internal/launcher/search.go`：搜索结果的 `Detail` 回到纯 `result.URL`；标签页行的 `Detail` 回到
>   纯 `tab.URL`，id 改成 `tab:<browserID>:<window>:<index>`；删掉随之不再使用的 `copy := StringsFor(...)`。
> - `internal/i18n`：`BrowserTab` 成了死键（唯一读者已删），按文档的 `TestNoDeadCopy` 守卫从
>   `Copy` 结构体与 en/zh 字典各删一行。
>
> **测试**（新增 `internal/launcher/browserrows_test.go` 的 `TestBrowserRowsFollowTheReferenceRowContent`）：
> 一条历史 + 一条书签 + 两条**同 URL 不同窗口**的标签页，逐项钉 history→时钟 / bookmark→书签 /
> tab→app-window 三个字形，历史与标签页的副标题都等于纯 URL，两条标签页的 id 分别是
> `tab:chrome:0:0` 与 `tab:chrome:1:2`（且不相等）。改写 `view_test.go` 的 `TestBrowserModeOpensAndCopies`：
> 此前断言历史副标题带 `  ·  `（日期），现在断言它**只**是 URL、且日期串不再出现。
> 变异核查：把 `rowGlyph` 的 history 分支改回地球、把两处 `Detail` 改回带后缀、把 id 改回 URL，
> 对应断言立刻红。
>
> **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 全绿、
> `GOOS=linux/windows/darwin go build ./...` 三平台通过。
>
> **残留**：无。清单再次为空；下轮按惯例换入口（逐文件对照原版 / 核对「已知差距」里框架已支持
> 只是本应用没用的项）。

> **上一轮：P7-ea PATH 命令的别名带参数时还原成真命令（原版 `src/command-aliases.ts` 的 `rebaseAliasCommandLine`）**
> —— P7-dz 收尾后「尚未做」清单又一次为空。本轮按惯例换入口（逐文件对照原版），落在
> **命令别名模块**（`main:src/command-aliases.ts`）：它的搜索半边本构建早已接好，还原半边
> （`rebaseAliasCommandLine`）却没有对应物，`grep -rni rebase internal cmd` 无命中。清单因此重写为
> 一条带 file:line 的具体待办并本轮落地。依赖检查：GitHub releases API 限流，
> `proxy.golang.org/.../@v/list` 最新仍是 **v0.4.8**（与 `go.mod` 一致），无版本可升。只对照 `main`
> （`backup/main-pre-mygo` = v0.3.14 原版树），未翻历史提交。
>
> **偏离点**：原版 `main:src/command-aliases.ts:184-201` 的 `rebaseAliasCommandLine` 把行首那个
> **恰好**等于别名的 token 换回真命令名，由 `main:src/hooks/useLauncherCatalog.ts:1328-1334` 用在行的
> `commandLine` 上，注释写死「`gfm -m x` therefore executes `git -m x`, never a nonexistent shell
> command named `gfm`」；`matchedCommandAlias`（`:167-175`）限定只有**整 token、大小写不敏感**的相等
> 才算别名命中。本构建的搜索半边齐全（`internal/launcher/search.go:809-850` 的 `aliasHint` / `aliasRank`、
> `internal/settings/plugins.go:159` 的 `ResolveCommandAliases`），但带参运行走的是
> `internal/launcher/actionbar.go:164` 的 `ParseCommandLine(value)`——`argv` 的命令 token 原样进终端，
> `gfm status` 于是跑一个不存在的 `gfm`。
>
> **修法**：新增 `internal/launcher/aliasrebase.go`——`aliasToCommand`（按 `ResolveCommandAliases` 反查，
> 冲突时名字升序的第一个命令拥有别名，与搜索半边同一策略）与 `RebaseAliasArgv`（只替换 argv[0]，
> 其余字节逐字保留；前缀/包含命中不动，与 `matchedCommandAlias` 同口径）。`actionbar.go` 在算出 `argv`
> 后、两条 Run 路线之前统一调一次 `RebaseAliasArgv(a.Aliases, argv)`，环境变量路线与应用路线共用同一个
> argv。shell 语法行（`gfm | grep x`）按原版范围**不还原**，仍原样交给用户自己的 shell。
>
> **测试**（新增 `internal/launcher/aliasrebase_test.go` 六条）：纯函数五组（整别名换名、别名单独、
> 大小写不敏感、前缀/包含不动、空 argv/无别名不动、冲突取升序第一、空白别名丢弃、不变异入参）、
> 走运行路径的 `gfm status` → `[git status]`、环境路线 `FOO=gfm gfm status`（把 `git` 换掉而 `FOO`
> 的值不被误改）、shell 语法 `gfm | grep x` 不被还原。变异核查：把 `actionbar.go` 里的
> `RebaseAliasArgv` 调用摘掉，三条经运行路径的测试立刻红；把 `RebaseAliasArgv` 改成前缀也命中，
> 纯函数的前缀/包含用例立刻红。
>
> **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 全绿、
> `GOOS=linux/windows/darwin go build ./...` 三平台通过。
>
> **残留**：无。清单再次为空；下轮按惯例换入口（逐文件对照原版 / 核对「已知差距」里框架已支持
> 只是本应用没用的项）。

> **上一轮：P7-dz 启动器列表不再留着死代码里的「块标题」口子（原版 `list-sections.ts` 的 pluginGroup 归输出视图）**
> —— P7-dy 收尾后「具体待办（P7-dy 重写）」第 1 条（`launcher.Item.section` 是全仓死代码）是清单最高
> 优先级且带 file:line 的代码健康项。本轮把它落地。依赖检查：GitHub releases API 限流，
> `proxy.golang.org/.../@v/list` 最新仍是 **v0.4.8**（与 `go.mod` 一致），无版本可升。只对照 `main`
> （`backup/main-pre-mygo` = v0.3.14 原版树），未翻历史提交。
>
> **偏离点（死代码）**：原版 `main:src/launcher/list-sections.ts` 的块标题有三种——`history` /
> `files` / `pluginGroup`，其中 `pluginGroup`（`:39-49` 的 `listSectionStarts`）读的是
> `item.type === "plugin"` 的 `group`（`LauncherResults.tsx:836-838` 把 `.launcher-section-title`
> 印在行上方）。本构建的 `internal/launcher/search.go:77` 声明了 `Item.section`、
> `internal/launcher/launcher.go:1535-1541` 读它并在行上方画标题，但**全仓没有任何非测试代码写入
> `section`**：原版的 `type: "plugin"` 行只由 `plugin-mode.ts:424` 造出（插件列表协议），在本构建走
> captured-output 视图，那里的 `outputRow`（`internal/launcher/output.go:521-530`）已按 `row.Group`
> 画同一条标题。启动器自己的结果列表从不放这类行，故该分支不可达。files 块走 `section:dropped`
> 的 `heading` 行、history 块同理，都不经过 `section`。
>
> **修法**：
> - `internal/launcher/search.go`：删掉 `Item.section` 字段与其注释；`heading` 的注释去掉「a row
>   *with* a section instead prints the title above itself」这句指向已删分支的话。
> - `internal/launcher/launcher.go`：`row()` 的 `ui.Column` 里删掉 `if item.section != ""` 那段
>   标题行，只留 `a.rowBody(...)`。
> - `internal/launcher/filters_test.go` 的 `TestSectionHeadings` 注释同步改写（去掉「a block's
>   first row prints its title above itself」）。
>
> **测试**（新增 `internal/launcher/rowsection_test.go`）：`TestTheOrdinaryListChargesNoSectionBands`——
> 一个带应用、最近使用与外部插件命令的结果集，`windowRows` 报告 **0** 条 section band；块标题只以
> `heading` 行存在并被记为 `RowInfo.Heading`，普通行不被记成标题。删除是编译期强制的（字段没了），
> 该测试钉住「普通列表的标题账目不走 section」这条不变量，且不动 `outputRow` 的 group 标题——
> 既有的 `TestAnOutputListsOpeningGroupPrintsItsHeading` / `TestOutputGroupHeadingSharesTheLauncherSectionRegister`
> 仍全绿。变异核查：把 `if item.section != ""` 那段加回（配一个临时字段）会让 `windowRows` 的
> section 账目与 `row()` 的绘制再次分叉，本测试与既有 heading 测试都会红。
>
> **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 30 包全绿、
> `GOOS=linux/windows/darwin go build ./...` 三平台通过。
>
> **残留**：P7-dy 重写的两条至此结清（第 1 条本轮落地，第 2 条 `role="alert"` 是框架无枚举的死
> 口子，仅记录）。清单又空；下轮按惯例换入口（逐文件对照原版 / 核对「已知差距」里框架已支持
> 只是本应用没用的项）。

> **上一轮：P7-dy toast 用回旧版自己的视口——每档自己的色调/图标，浮层按表面定位（原版 `ToastStack.tsx` + `styles/extensions.css:210-290`）**
> —— 本轮把「逐文件对照原版」的入口放在「已知差距」里那条**框架已支持、只是本应用没用**的
> toast 色调（其末尾写着框架的 `ToastViewportBase` + `Data` 就是口子）。清单为空，故此条即
> 本轮待办；依赖检查：GitHub releases API 限流，`proxy.golang.org` 的 `@v/list` 最新仍是
> **v0.4.8**（与 `go.mod` 一致），无版本可升。只对照 `main`（`backup/main-pre-mygo` = v0.3.14
> 原版树），未翻历史提交。
>
> **偏离点（两条，都在同一块浮层里）**：
> - 原版 `main:src/components/ToastStack.tsx:62-84` 的每个 toast 按 `kind` 带自己的图标
>   （error=AlertCircle / warning=TriangleAlert / success=Check）与色调，`src/styles/extensions.css:321-334`
>   的 `.app-toast--error` 与 `--warning` 共用 `--text-warning`（CSS 自己的 R9-1 注释写明 warning
>   「shares the error band's amber *label*」），`--success` 用 `--accent`。本构建用框架默认
>   `buildToasts`（`mygo@v0.4.8:ui/toast.go:343`），它只读 `t.inverse()`、**完全不看 `Toast.Type`**——
>   于是拒绝深链的 error toast 与缺工具链的 warning toast 长得一模一样。
> - 原版 `main:src/styles/extensions.css:210-290` 的堆栈**按表面定位**：全高页面（设置/终端/插件页）
>   挂在**右上** `top:64px; right:16px`、宽 `min(360px, 100% - 32px)`、最多三枚；折叠启动器是约
>   58px 的窄条，改钉在**左下** `bottom:6px; left:6px`、宽 `min(300px, 100% - 12px)`、只显示**最新**
>   一枚（20px 单行条）。框架默认把每枚 toast 摆在中下（`viewport.Bottom(...).AlignItems(Center)`），
>   启动器表面放不下。
>
> **修法**：
> - `internal/shell/notices.go` 新增 `toastViewport`（`:169`）——`ui.ToastViewportBase` 替掉框架的
>   主题视口（本帧一次性），逐枚用 `ui.ToastBase` 自己画；`toastTone`（`:130`）按 kind 取
>   `Theme.Warning` / `Theme.Accent` / `Theme.Text`，`toastMark`（`:144`）取字形名；
>   `launcherGlyph`（`:276`）复用应用的字形词表。背景用 `tokens.Float`（旧版 `--glass-float`）、
>   边框 `tokens.InputStroke`（`--stroke-control`），与旧版 `.app-toast` 一致；动作按钮是旧版
>   `.app-toast__action` 的安静 chip，关闭钮复用 savealert 的图标钮惯用法。
> - 定位：`internal/shell/notices.go:180-191` 按 `a.Surf == SurfaceLauncher` 分叉——启动器
>   `Bottom(1.5).Left(1.5).AlignItems(Start)`（且只画最新一枚），其余
>   `Top(16).Right(4).AlignItems(End).Justify(Start)`，宽度按原版 `min(360/300px, …)` 夹取。
> - `internal/launcher/glyphs.go:53-54,219-222` 补两个共享字形 `triangle-alert` / `check`
>   （`alert` 即 AlertCircle，原就有）；`internal/i18n/i18n.go:448` + `copy.go` 补关闭钮的无障碍标签
>   `DismissNotice`（原版 `settings.extensions.dismissNotice` = “Dismiss notice”/“关闭提示”）。
> - `internal/shell/shell.go:3096` 的 `View` 在 `flushNotices` 之后调 `a.toastViewport(c)`（须在其后，
>   flush 才把这帧的通知放进视口读的栈）。
>
> **测试**（`internal/shell/notices_test.go`）：`TestToastTonesAndMarksFollowTheirKind`（三档色彩/字形
> 逐条钉住，并断言字形名真在词表里）、`TestTheToastIsDrawnByTheAppsOwnViewport`（自绘视口画出句子
> 与 `DismissNotice` 关闭钮，点它真关掉）、`TestTheToastActionRunsAndClosesTheToast`（动作 chip 跑闭包
> 并关 toast）、`TestTheToastStackAnchorsPerSurface`（启动器左下、设置右上）、
> `TestTheLauncherToastStackKeepsOnlyTheNewest`（折叠面只留最新，全高面保留整摞）。变异核查：把
> `View` 里的 `a.toastViewport(c)` 摘掉，第二条立刻红（框架默认没有 `DismissNotice` 控件）；把 else 分支
> 换回框架的中下对齐，定位测试立刻红（`the settings notice is not top-right`）。
>
> **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 30 包全绿、
> `GOOS=linux/windows/darwin go build ./...` 三平台通过。
>
> **残留**：原版 error toast 的 `role="alert"`（含 `role="status"` 之外的无障碍角色）在框架里没有
> 对应枚举（`ui.Role` 只有 `RoleStatus`/`RoleAlertDialog`，`ToastBase` 固定 `RoleStatus`），故不接线，
> 留「已知差距」。success 分支按原版接好，但本构建暂无 success raiser（旧版的成功 toast 多由复制反馈
> 触发，本构建的复制反馈在终端状态行），如实记录为「有口子、等 raiser」。清单又空，下轮换入口。

> **上一轮：P7-dx sessions 的关闭武装改用会话身份，不再用位置（原版 `killArmedId` 是 `sessionId`）**
> —— P7-dw 收尾后「具体待办（P7-dw 重写）」第 1 条（武装用 `Index` 当身份，删会话后会落到
> 别的会话上）是清单最高优先级且带 file:line 的已核实缺陷。本轮把它落地。依赖检查：GitHub
> releases API 限流，`proxy.golang.org/.../@v/list` 最新仍是 **v0.4.8**（与 `go.mod` 一致），
> 无版本可升。只对照 `main`（`backup/main-pre-mygo` = v0.3.14 原版树），未翻历史提交。
>
> **偏离点**：原版 `main:src/settings/SessionsPage.tsx:54` 的 `killArmedId` 是
> `string | null`——**稳定的 `session.sessionId`**（`:199` 的 `key={session.sessionId}`、
> `:224/:241` 的 `killArmedId === session.sessionId`），列表重排/增删都不改它的指向。本构建
> P7-dw 的武装存在 `internal/settingsui/settingsui.go` 的 `armedSession int`，存的是
> `session.Index`（位置，`internal/shell/shell.go` 的 `Sessions` 提供者按切片位置编号）。会话
> 被删时切片收缩、后面的 index 前移（`internal/terminalui/terminalui.go:266` 的 `EnsureSession`
> 丢弃驻留会话、`:406` 的 `CloseSession`、`:891` 的 `CommandSessionEnded`），于是「武装 index 2
> → 排在前面的会话先结束 → 原 index 2 变成 index 1，而 `armedSession` 仍是 2」——确认钮画在
> **另一个**会话上，按下去 `CloseSessionAt(2)` 杀错人。
>
> **修法**：
> - `internal/terminalui/terminalui.go`：`Session` 加稳定字段 `ID uint64`（`:78-84` 的注释逐字
>   指向原版 `sessionId`）；`App` 加 `nextSessionID uint64` 计数器，`startSession`
>   （`:222-223`）每建一个会话 `++` 一次并填进 `Session.ID`。身份发一次、不复用。
> - `internal/settingsui/settingsui.go`：`Session` 加 `ID uint64` 字段（`:380-383`）；
>   `armedSession` 由 `int` 改 `uint64`，存 **ID**（0 永不武装）；`armSessionClose(id uint64)`
>   收 ID；渲染里判 `a.armedSession != 0 && a.armedSession == session.ID`，确认钮仍用当前帧的
>   `session.Index`（提供者给的**新鲜**位置），于是杀的就是被问的那个会话。
> - 行消失即自动解除：`sessions`（`:1000-1005`）在画列表前 `if a.armedSession != 0 &&
>   !sessionInList(sessions, a.armedSession) { a.disarmSessionClose() }`——新增 `sessionInList`
>   助手。原版列表每轮对着 broker 重渲染，被删行的武装随行消失。
> - `internal/shell/shell.go` 的 `Sessions` 提供者补 `ID: sessions[i].ID`。
>
> **测试**：`internal/terminalui/sessionid_test.go` 新增
> `TestSessionIDsAreStableAcrossDeletion`（三个会话 ID 非零且互异；删掉第一个后后面的会话
> **保住各自的 ID**；新建会话拿到的 ID 不复用旧的）。`internal/settingsui/sessionsidentity_test.go`
> 新增两条：`TestTheCloseArmFollowsTheSessionWhenTheListShrinks`（武装 B(id 2) → 排在它前面的
> A 被删、B 从位置 1 移到 0 → 问题仍在、确认调用 `CloseSessionAt(0)` 即 B 的**新**位置，不是
> 陈旧的 1）；`TestTheCloseArmIsDroppedWhenItsSessionDisappears`（武装 B → B 自己消失 → 问题随行
> 消失、确认钮不可点、不关任何会话）。变异核查：把渲染判据改回 `a.armedSession == session.Index`，
> 第一条立刻红（`confirm closed [1], want [0]`）；删掉 `sessionInList` 的自动解除，第二条立刻红
> （`the question outlived its session`）。
>
> **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 30 包全绿、
> `GOOS=linux/windows/darwin go build ./...` 三平台通过。
>
> **残留**：P7-dw 重写的三条至此全部结清（第 2 条已落地 P7-dw，第 3 条已结清无落点）。清单又
> 空；下轮按惯例换入口（见「尚未做」）。

> **上一轮：P7-dw sessions 的关闭武装带上 3s 时钟、离页即清（原版 SessionsPage 的 `killArmTimer`）**
> —— P7-dv 收尾后「具体待办（P7-dv 重写）」第 1 条（sessions 武装跨页驻留、无 3s 自动解除）是清单
> 最高优先级且带 file:line 的具体项（第 2 条 key-up reassert 已结清为无落点）。本轮把它落地。
> 依赖检查：GitHub releases API 限流，`go list -m -versions github.com/egoist/mygo` 最新仍是
> **v0.4.8**（与 `go.mod` 一致），无版本可升。只对照 `main`（`backup/main-pre-mygo` = v0.3.14
> 原版树），未翻历史提交。
>
> **偏离点**：原版 `main:src/settings/SessionsPage.tsx` 的关闭武装是**页组件自己的**局部 state——
> `:53-71` 的 `armKill` 用 `killArmTimer`（`useRef`）起 `KILL_CONFIRM_TIMEOUT = 3000`（`:10`，注释
> 写明与扩展面板的行内丢弃确认同一节奏 `75fff0b`）的 `setTimeout` 到时 `setKillArmedId(null)`；
> `disarmKill` 先 `clearTimeout` 再归零；`:86-89` 的 unmount cleanup 清掉定时器。所以原版有两条
> 界：**3 秒自解**与**离页即失**（组件状态随 unmount 消失，`killArmedId` 是**单个** id）。本构建
> 把武装放在 `settingsui.App` 上（P7-dv 后是 `settingsui.go:492` 的 `sessionsArmed map[int]bool`），
> `selectPage`（`:886`）不清、也没有超时：离开 Sessions 页再回来仍在，且一直不会自己收回。
>
> **修法**（`internal/settingsui/settingsui.go`）：
> - `sessionsArmed map[int]bool` → `armedSession int` + `armedSessionAt time.Time`（`:490-497`）：
>   原版一次只武装一个（单个 state），且 deadline 与「武装了谁」**必须成对**。用 `time.Time` 零值
>   而非 `map` 存在性当「问过了」的判据（index 0 是真实会话，不能用 0 值表达「没问」），并加
>   `sessionCloseWindow = 3 * time.Second`（`:3402`）逐字对应 `KILL_CONFIRM_TIMEOUT`。
> - 三个小助手取代原来的一对：`armSessionClose(index)`（重设 deadline，等价原版 `armKill` 每次都
>   `clearTimeout` 重启计时器，所以武装一个自动放下别人）、`disarmSessionClose()`（一次清掉人和钟，
>   两者不会漂移）、`expireSessionClose(c)`（`:3443`，仿 launcher 的 `expireArmedDelete` /
>   `expirePowerConfirm`：超窗即解除，并 `c.AnimationFrame()` 保证静止页上也能到点）。
> - `View`（`:665`）每帧调 `expireSessionClose`；`selectPage`（`:896`）在 `page != PageSessions` 时
>   `disarmSessionClose`——原版的武装活在组件里，离页 unmount 即失；Escape 分支改调
>   `disarmSessionClose`（P7-dv 接的 `anySessionArmed` 语义由 `armedSessionAt` 承载）。
>
> **测试**（`internal/settingsui/sessionsclock_test.go` 新增）：
> `TestTheSessionCloseQuestionExpiresAndDoesNotCrossPages`——注入 `Now`，武装后 2s 问题仍在、
> 6s 后回到普通关闭钮且**没杀会话**；再武装后切到 General 再切回 Sessions，问题**不**跟着回来、
> 也没杀会话。变异核查：把 `View` 里的 `a.expireSessionClose(c)` 去掉，第一条断言立刻红
> （`the question outlived its window`）。
>
> **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 27 包全绿、
> `GOOS=linux/windows/darwin go build ./...` 三平台通过。
>
> **新写实的残留（下轮最高优先级，见「具体待办（P7-dw 重写）」）**：武装用 `Session.Index`
> （位置）当身份，而原版用的是 `sessionId`（`SessionsPage.tsx:54` 的 `killArmedId: string | null`）。
> 本构建把会话从切片里删掉时（`internal/terminalui/terminalui.go:266`、`:406`、`:891`）**后面的
> index 会前移**，于是「武装了后面的会话 → 前面的会话先结束」之后，确认钮会落到**另一个**会话上，
> 按下去杀错人。

> **上一轮：P7-dv 设置页的 Escape 先答问题再离页（原版捕获层监听，本构建被 shell 抢先认领）**
> —— P7-du 收尾后「具体待办（P7-dt 重写）」第 3 条（terminal 的 new-command key-up reassert，
> macOS）是清单最高优先级。本轮先按验收核实框架：**mygo 的焦点层已把这条缝补掉**——每个启动器
> 帧都调 `launcher.App.View` → `a.FocusSearch()`（`internal/launcher/launcher.go:762`），而
> mygo 的 NSView 是**整个 surface 唯一的原生 first responder**（`internal/darwin/surface.go:667`
> 的 `makeFirstResponder:`、`:800-809` 的 `become/resignFirstResponder` 都只在 surface 一级；
> 每 widget 的焦点是引擎内部状态，`SurfaceBlur/Focus` 只调 `focusChanged()`，`ui/input.go:1323`），
> 所以原版 `useAppKeyboard.ts:130-142` 要防的「Cmd 松手时面板再动一次 first responder、把输入框
> 的焦点抢走」在 mygo 里没有对应事件。第 3 条**结清为无落点**，写进「已知差距/结论」。
> 依赖检查：GitHub releases API 限流，`proxy.golang.org/.../@v/list` 最新仍是 **v0.4.8**
> （与 `go.mod` 一致），无版本可升。只对照 `main`（`backup/main-pre-mygo` = v0.3.14 原版树），
> 未翻历史提交。
>
> **偏离点（同一片键盘归属里核出的真实 bug）**：设置页的 Escape **答不了自己的问题**。原版的
> 删除/停用确认条与 sessions 页的关闭武装各自装了一个**捕获层** `document.addEventListener(
> "keydown", …, true)` 并 `event.stopImmediatePropagation()`（`main:src/extensions/
> RemovalConfirmation.tsx:34-37`、`DisableConfirmation.tsx`、`main:src/settings/SessionsPage.tsx:98-107`
> 的 `disarmKill`），捕获先于 window 上的 bubble 处理器（`useAppKeyboard.ts`），所以 Escape 先
> 取消问题、**不离开页面**。本构建把这条意图写在 `internal/settingsui/settingsui.go:664-678` 的
> Escape 分支上，也在 settingsui 自己的 tester 里测过（`TestIntegrationsPage`）——但它用的是
> `c.Shortcut(0, ui.KeyEscape)`（窗口级 id 0），而 shell 的 `appShortcuts` 在
> `internal/shell/shell.go:3092` **先于** surface view（`:3216`）调 `c.Shortcut(0, ui.KeyEscape)`
> → `Open(SurfaceLauncher)`。mygo 的 `Shortcut` 同帧内先到先得（`ui/input.go:776-787`），于是
> Escape 被 shell 吃掉：**删除确认条打开时按 Escape 关掉整个设置面板，而不是取消确认**。
> 顺带同一处：sessions 页的「先问一次」武装 `sessionsArmed` 也没接进这个 Escape 分支
> （原版 SessionsPage 的捕获监听专管它），Escape 会离页而武装还在。
>
> **修法**：
> - `internal/shell/shell.go` 的 `appShortcuts` settings 分支**删掉 Escape 认领**，只留字面 ⌘W
>   与 new-command 两条 close-settings 规则；与 launcher 分支「把 Escape 留给自己的视图」同一
>   形状。设置视图的 Escape 因此第一次真正跑起来（先 `anyConfirmOpen` → 取消，其次
>   `anySessionArmed` → 解除，最后 `Actions.Close` 关页），函数头注释同步改写。
> - `internal/settingsui/settingsui.go` 的 Escape switch 加一档 `case a.anySessionArmed():
>   a.disarmSessions()`，并新增两个小助手（`anySessionArmed` / `disarmSessions`）——原版
>   SessionsPage 捕获监听 `disarmKill` 的等价物。
>
> **测试**：`internal/shell/settingsescape_test.go` 新增
> `TestEscapeAnswersTheSettingsQuestionFirst`（设置页打开删除确认条 → 第 1 次 Escape 留在
> settings 且确认条消失 → 第 2 次 Escape 回 launcher）与
> `TestEscapeClosesTheSettingsWhenNothingAsks`（无问题时 Escape 仍是面板出口）；
> `internal/settingsui/view_test.go` 新增 `TestEscapeDisarmsASessionClose`（武装 → Escape 不调
> `Close`、不杀会话、解除武装 → 再 Escape 才 `Close` 一次）。变异核查：把 shell 的
> `if c.Shortcut(0, ui.KeyEscape)` 加回 settings 分支，第一条立刻红（`Escape left the settings
> panel: surf = 0`）；删掉 settingsui 的 `anySessionArmed` 档，第三条立刻红（`Close ran 1 times`）。
>
> **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 27 包全绿、
> `GOOS=linux/windows/darwin go build ./...` 三平台通过。
>
> **新写实的残留（下轮最高优先级，见「具体待办（P7-dv 重写）」）**：sessions 武装**跨页驻留**
> 且**没有 3s 自动解除**——原版 `SessionsPage.tsx:64-71` 的 `armKill` 起 3000ms 定时器归零、
> 状态又随页面 unmount 消失；本构建 `sessionsArmed` 是 `settingsui.App` 的字段
> （`settingsui.go:492`），`selectPage`（`:886`）不清、也没有超时。

> **上一轮：P7-du launcher 上的字面 ⌘W 不再是自己的规则（改绑 new-command 后它什么也不做）**
> —— P7-dt 收尾后「具体待办（P7-dt 重写）」第 2 条（launcher 上字面 ⌘W 仍会隐藏窗口）是清单最高
> 优先级的已核实偏离。本轮把它落地。依赖检查：GitHub releases API 限流，
> `proxy.golang.org/.../@v/list` 最新仍是 **v0.4.8**（与 `go.mod` 一致），无版本可升。只对照 `main`
> （`backup/main-pre-mygo` = v0.3.14 原版树），未翻历史提交。
>
> **偏离点**：原版 `main:src/surface-policy.ts:207-211` 的 `DISMISS_TABLE.collapsed` 里
> `"mod-w"` 是 **null**——注释写着「only the configurable new-command binding dismisses here」。
> `"new-command"` 才 `{ action: "hide-window" }`，而它默认就是 ⌘W，所以**两种键在默认下看不出
> 差别**；用户一旦把 `new_command` 改绑到别处，原版的字面 ⌘W 就**什么也不做**。本构建
> `internal/shell/shell.go:3046-3053` 的 default（launcher）分支却先无条件 `c.Shortcut(ui.Cmd,
> ui.KeyW)` → `Hide()`（注释自认「the same key by default」），于是改绑后 ⌘W 仍隐藏窗口。
>
> **修法**（`internal/shell/shell.go`）：删掉 launcher 分支里的字面 ⌘W 认领，只留
> `shortcuts.Parse(settings.Shortcut(..., ShortcutNewCommand))` 命中时 `Hide()`——与 settings
> 分支里那条**保留**的字面 ⌘W（原版 `settings["mod-w"]` 确有 close-settings 规则）形成对照。
> 函数头与内联注释同步改写：原版把「⌘W 在每个面都同义」写死过，本轮按表逐面写实。
>
> **可观测性**：`Hide()` 在测试里没有窗口可观察（`a.Win == nil`），所以按 P7-do 给
> `windowVisible` 的做法加一条缝 `App.hideWindow`（`shell.go:222-226`）：非 nil 时 `Hide()`
> 走它，否则走真窗口。它不是 `Options` 字段（正式构建永远用真窗口），只有测试需要看见。
>
> **测试**（`internal/shell/appkeys_test.go`）：`TestDismissTable` 的第一段加计数器，钉住
> 「默认 ⌘W 经 new-command 规则隐藏窗口 1 次」；terminal 段补「⌘W 不隐藏窗口」（它只把面切回
> 启动器）。新增 `TestALiteralModWDoesNothingAfterARebind`：把 `ShortcutNewCommand` 改绑成
> `Alt+N`，发 ⌘W → surface 与窗口都不动（`hides == 0`），再发 `Alt+N` → 隐藏 1 次（用户自己的
> 键才是那个入口）。变异核查：把字面 ⌘W 分支加回 launcher，新测试立刻红
> （`a rebound ⌘W hid the window 1 times, want 0`）。
>
> **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 全绿、
> `GOOS=linux/windows/darwin go build ./...` 三平台通过。

> **上一轮：P7-dt 终端空态的返回行印出并绑上用户自己的 new-command 键（不再是字面 Escape）**
> —— P7-ds 收尾后「尚未做（P7-ds 重写）」只剩「块光标的字形颜色」（已核实框架无口子），清单
> 事实上已空。本轮把清单写实：先逐条核对 P7-dq 剩下的第 2·3 条（结论：`EnsureSession` →
> `startSession` 同步拿回会话对象，没有异步 spawn 窗口，240ms 空态延迟**无落点**；本构建没有
> 自定义集成抽屉/discard bar，溢出菜单里的 Export/Import 已由 `transferRow` 两个直按钮直接暴露，
> 脏比较与菜单都**无落点**）；再把入口换到「原版 `src/App.tsx` 的终端空态 + `src/surface-policy.ts`
> 的 dismiss 表」。依赖检查：GitHub releases API 限流，`proxy.golang.org/.../@v/list` 最新仍是
> **v0.4.8**（与 `go.mod` 一致），无版本可升。只对照 `main`（`backup/main-pre-mygo` = v0.3.14 原版树），
> 未翻历史提交。
>
> **偏离点**：终端空态的「返回」行**印错了键也绑错了动作**。原版 `main:src/App.tsx:3863-3872`
> 的第二行 keycap 是 `formatShortcut(shortcuts.new_command)`（配置里的键），点击是
> `returnToInputMode`；`main:src/surface-policy.ts:207-215` 的 `DISMISS_TABLE.terminal.escape` 是
> **null**——Escape 在终端面归运行中的 shell，只有可配置的 `new-command` 才 `return-to-input`。
> 本构建（改前）：`internal/terminalui/terminalui.go:711-724` 印字面量 `"esc"`、点击
> `Actions.Close`（只 `Open(SurfaceLauncher)`，**不清字段**），并且视图自己
> `c.Shortcut(0, ui.KeyEscape)` 抢走了 Escape；而 new-command 键其实由
> `internal/shell/shell.go:3031-3033` 的 `appShortcuts` → `returnToInput` 单独回答。于是同一件事
> 有两套说法：键印 Escape、点击走 Close，与原版都不一致。
>
> **修法**：
> - `internal/terminalui/terminalui.go`：`emptyLine`（`:779`）改收**已排好的 keycap 文本**
>   （`badgeText`），不再在里面套 `keyBadge`；新增纯函数 `newCommandBadge(s settings.Settings)`
>   （`:813`）返回 `shortcuts.Display(settings.Shortcut(s, settings.ShortcutNewCommand))`——与
>   原版 `formatShortcut` 同一个 token，Mac 读 ⌘W、别处读绑定自身的写法。返回行改用
>   `Actions.NewCommand`（`:727`），并删掉视图对 `ui.KeyEscape` 的认领（Escape 归 shell，符合
>   `DISMISS_TABLE.terminal.escape = null`）。`keyBadge` 因此只剩 `enter`→`↩` 一档。
> - `internal/shell/shell.go`：`termActions.NewCommand`（`:829`）从
>   `ResetQuery(); Open(SurfaceLauncher)` 改为 `a.returnToInput()`，与 appShortcuts 的键路径、
>   header 的「New command」按钮、原版空态第二行四处同一入口（都经 `returnToInputMode` 的等价物
>   `returnToInput`，带 `scanToolCatalog()`）。
>
> **测试**（`internal/terminalui/sessions_test.go`）：`TestEmptyStateLines` 重写——把
> `ShortcutNewCommand` 改绑成 `Alt+B`，断言渲染里出现 `shortcuts.Display("Alt+B")`、**不再有**
> `"esc"`；按 Escape 断言 new-command/close 都**没**跑；点返回行断言
> `Actions.NewCommand` 跑了一次。变异核查：把 badge 改回 `"esc"` 或把动作改回 `Actions.Close`，
> 断言立即红。
>
> **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 全绿、
> `GOOS=linux/windows/darwin go build ./...` 三平台通过。

> **上一轮：P7-ds `inherit` 终端的光标色取回 `--terminal-cursor` token（面板光标不再是前景色的替身）**
> —— P7-dr 收尾后「尚未做（P7-dr 重写）」第 1 条（块光标的字形颜色应取单元格自己的前景色）是清单
> 最高优先级。本轮按验收先核实框架：**确实没有逐单元格的口子**——mygo v0.4.8 的
> `plugins/terminal/paint.go:500-505` 在块光标铺满后固定用 `color(v.colors.Background)` 重绘字形，
> 只有 `Theme.CursorText.A > 0` 时才换成那一个全局色（`theme.go:13-14` 的字段注释就是「一个颜色」。
> 所以该条按验收落进「已知差距」（记录已在 P7-dr 写实，本轮只补上一句「框架确认」），**不半步接线**
> （给 `CursorText` 填一个全局前景色会在 `amber` 这种光标与前景都偏亮的调色板上把字压没，而原版自己
> 在同样的格子上画同样的低对比——但那只是碰巧，不是等价）。依赖检查：GitHub releases API 仍限流，
> `go list -m -versions github.com/egoist/mygo` 最新仍是 **v0.4.8**（与 `go.mod` 一致），无版本可升。
> 只对照 `main`（`backup/main-pre-mygo` = v0.3.14 原版树），未翻历史提交。
>
> **偏离点**：核对同一段 `cursorRect`（`paint.go:487-508`）时发现——光标**自身的颜色**也不对。
> 框架 `cursorRect` 的取色是 `c = color(v.colors.Foreground)`，只有插件主题里 `Cursor.A > 0` 才换成
> `v.theme.Cursor`（`theme.go:91-95` 的 `apply` 只在 `Cursor.A > 0` 时把 cursor 交给 `SetColors`）。
> 而本构建的 `palette("inherit")` **返回 nil**，插件于是回落到它的零光标规则（前景色）。原版不是这样：
> `main:src/terminal/render.ts:242-244` 的 `updateTheme()` 对 `inherit` 读的是 CSS 的
> `--terminal-cursor`——`main:src/styles/base.css:718` 暗色 **#8bd5ca**、`:975/:1107` 亮色
> **#007c86**，两个都是与 `--terminal-fg`（#d7dae0 / #202124）**不同的青绿**。于是本构建的 inherit
> 光标一直是白的/黑的（跟前最色），原版是青的。设置页的调色板预览用同一份 `PaletteColorsFor`，
> 预览里的块光标也因此画成前景色。
>
> **修法**（`internal/terminalui/terminalui.go`）：
> - `palette(name) *terminal.Theme` 改为 `themeFor(name) (theme, dark *terminal.Theme)`
>   （`terminalui.go:1175`）：命名调色板仍返回**一个固定主题**（第二返回为 nil，插件在两种外观下
>   都用它，同原先），`inherit` 返回插件自带的**亮/暗一对**（`terminal.LightTheme()` /
>   `terminal.DarkTheme()`），只把 `Cursor` 换掉——这正是插件 `view.go:128-133` 给
>   Ghostty `theme = light:…,dark:…` 的语义，所以 `inherit` 依然跟随窗口外观，只多了那一个颜色。
> - 新增纯函数 `inheritCursor(dark bool)`（`terminalui.go:1197`）：暗 #8bd5ca / 亮 #007c86，
>   与 base.css 逐字对应。`PaletteColorsFor("inherit", dark)` 的 cursor 也改用它，预览与真终端
>   从同一个 token 取色。
> - `options()` 把两个返回都交给 `terminal.Options{Theme, DarkTheme}`（`terminalui.go:634`），
>   `Refresh()` 原样把这一对交给 `SetTheme`，所以运行中的会话下一帧也换得过来。`DarkTheme` 这只
>   旋钮以前一直空着（只给了固定的 `Theme`），从本轮起对 `inherit` 有了作用。
>
> **测试**（`internal/terminalui/palettebackground_test.go`）：新增
> `TestTheInheritCursorUsesTheShippedToken`——亮/暗 cursor 逐值钉在 token 上、前景/背景仍等于插件
> 默认（证明只有 cursor 被覆盖）、预览走同一 token、命名调色板仍是单主题+自带 cursor。
> `view_test.go:99` 原先把继承写成「没有主题」，按新契约改为「有亮/暗一对、暗色 cursor = token」；
> `TestASessionNeverPaintsItsOwnBackground` 的 inherit 断言同步。变异核查：删掉
> `lightBase.Cursor = inheritCursor(false)`，`TestTheInheritCursorUsesTheShippedToken` 与
> `TestOptionsFollowTheSettings` 两条立刻红。
>
> **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 全绿、
> `GOOS=linux/windows/darwin go build ./...` 三平台通过。

> **上一轮：P7-dr 终端配色的背景改由视图按帧绘制（`Transparent` 不再是创建期的陈旧标志）**
> —— P7-dq 收尾后「尚未做（P7-dq 重写）」第 1 条（终端外观除「进入」之外还有没有别的写入时机）
> 是清单最高优先级。本轮逐条走完全部建会话入口，结论是**该条无落空**（都经 `App.startSession`
> → `options()` 读当帧快照，见下方「结论」），但核对时**发现并落地**了一处真实偏离。依赖检查：
> GitHub releases API 限流，`proxy.golang.org/.../@v/list` 最新仍是 **v0.4.8**（与 `go.mod`
> 一致），无版本可升。只对照 `main`（`backup/main-pre-mygo` = v0.3.14 原版树），未翻历史提交。
>
> **偏离点**：`terminal.Options.Transparent` 是**创建期**选项——mygo v0.4.8 的
> `plugins/terminal` 只导出 `SetFont`/`SetTheme` 两个 setter（`Cursor`/`NoBlink` 同样只在建会话时
> 落到 `t.term.SetDefaultCursor(...)`，`plugins/terminal/terminal.go:245`）。本构建此前把它设成
> `s.TerminalTheme == "inherit"`，于是**跨 inherit/命名配色这条边界**时它必然过期：在 `inherit`
> 上开的会话，之后选中的命名配色永远不画（`paint.go:399` 的 `!Transparent` 分支才是画背景的那
> 一处）；反过来在命名配色上开的会话，切回 `inherit` 后仍旧画自己那块背景，玻璃就丢了。原版没有
> 这个问题，但**不是靠 setter**：`main:src/terminal/render.ts:219` 的 `updateTheme()` 把
> `this.bg`/`this.bgOpacity`/`this.bgFill` 重算一遍，`draw()` 每帧 `ctx.clearRect` +
> `ctx.fillStyle = this.bgFill; ctx.fillRect(...)`；`main:src/hooks/useTerminalView.ts:502-521`
> 又每次进入终端重建 renderer，构造参数就是当帧设置——**背景是每帧从当帧设置算出来的应用层**，
> 不是终端插件的一个开关。
>
> **修法**（`internal/terminalui/terminalui.go`）：
> - `options()` 改为常量 `Transparent: true`（`terminalui.go:657`），把我方画布定为**应用自己的
>   一层**。
> - 新增纯函数 `canvasFill(name, opacity, highContrast)`（`terminalui.go:1127`）：命中 `palettes`
>   时返回 `packed(spec.bg)` 配 `canvasAlpha` 的 alpha，`inherit`/未知 id 返回 `ui.Color{}`（不画，
>   正是原版 `inherit` 的 `--terminal-bg` 语义）；`canvasAlpha`（`terminalui.go:1143`）是原版
>   `canvasFill(floor, solidTop, transparency)` 的公式 `min(0.98, max(0.86, opacity))`——与
>   `glassmap` 给这层下面的 veil 用的同一对常量（`glassmap.SolidTop` / `ContrastFrameFloor`），
>   于是画布与它底下的 veil 对「终端有多实」的判断一致；0.98 这个上限正是「全不透明会把材质的
>   反光与描边压没、读起来像涂料」（`main:src/styles/base.css:378-388` 的 `--glass-solid-top` /
>   `--glass-frame-floor`）。
> - `View` 的挂载盒（`terminalui.go:747-760`）在闭包内重新读 `a.Store.Snapshot()` 并
>   `.Background(canvasFill(...))`，所以配色与滑杆在**会话已经跑着**时于下一帧生效。
> - 顺带把 `palette(name, opacity)` 的 alpha 去掉、改为 `palette(name)`（`terminalui.go:1161`）：
>   插件的画布填充用的是 **vt 的颜色**（`paint.go:399` 的 `color(v.colors.Background)`，而
>   `theme.go:98` 的 `rgb()` 把 alpha 丢掉），所以 `Theme.Background` 的 alpha 从来不参与像素
>   ——色彩是色彩、实度是实度，后者只在上面那条路径上。
>
> **测试**（`internal/terminalui/palettebackground_test.go` 新增）：`TestCanvasFillFollowsThePalette`
> （纯表：`inherit`/未知 id → 不画；`forest` 46 → `Alpha(0.46)`；10 → `0.1`；100 → `Alpha(0.98)`；
> 高对比 46 → `0.86`、100 → `0.98`；`paper` 0 + 高对比 → `0.86`）与
> `TestASessionNeverPaintsItsOwnBackground`（用 `captureTerminal` 建会话，`inherit` 与
> `store.Update` 到 `forest`/80 之后各建一次：两次都 `Transparent == true`、`inherit` 无主题、
> `forest` 的 `Theme.Background` 是**不透明**的调色板色）。`view_test.go:145` 原先把
> `Theme.Background` 钉在 `Alpha(0.8)`，按新契约改为不透明并写明理由。
>
> **诚实记录（框架不支持，同轮核实）**：① 块光标的字形颜色——原版
> `main:src/terminal/render.ts:586-591` 用**该单元格自己的前景色**重绘（`cellFg`，亮色模式还会
> `brighten`），mygo 插件用 `color(v.colors.Background)`（`paint.go:504`），除非
> `Theme.CursorText.A > 0`；没有逐单元格的口子，`inherit` 更是根本没有主题，只能如实记。
> ② 在运行中的会话上改光标形状/闪烁仍不生效：原版走
> `renderer.setOptions({cursorBlink,…})`（`main:src/hooks/useTerminalView.ts:623-627`），mygo 无
> setter。两条都已写进「已知差距」。
>
> **结论（第 1 条）**：全部建会话入口都经 `App.startSession`（`terminalui.go:166`）→ `options()`
> （`terminalui.go:632`）读**当帧** `Store.Snapshot()`，`a.NewTerminal` 只在 `terminalui.go:206`
> 出现一次；命令会话（`RunCommand` `terminalui.go:558`、`RunCommandLine` `terminalui.go:514`）、
> `RunShellIn`（`terminalui.go:539`）、shell 侧 `OpenTerminal`（`shell.go:519`）、
> `RunInTerminal`/`RunInTerminalWithEnv`（`shell.go:563-608`）、
> `RunShellWithCommand`/`RunCommandLine`（`shell.go:1827-1836`）、启动器 `RunCommand`
> （`shell.go:2358-2361`）与 `Open(SurfaceTerminal)`（`shell.go:2553-2561`）都不新建第二个入口，
> 所以「新会话必得当帧外观」成立；旧会话由入场刷新（P7-dq）+ 本轮的按帧背景覆盖。该条**关闭**。
>
> **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 全绿、
> `GOOS=linux/windows/darwin go build ./...` 三平台通过。

> **上一轮：P7-dq 回到终端面时补上外观（原版终端视图每次 mount 都用当帧设置重建 renderer）**
> —— P7-dp 收尾后「尚未做（P7-do 重写）」第 2 条（`OverflowMenu` 是否有落点）与第 3 条
> （`src/deferred-repaint.ts` 的重绘尾沿合并）是清单最高优先级的两条待比对项。本轮逐条核对，**两条
> 都不产生代码**（详见下方「具体待办」与各自结论），但核对第 3 条时逐行读终端外观的落点，**发现并
> 落地**了一处真实偏离。依赖检查：GitHub releases API 限流，`go list -m -versions
> github.com/egoist/mygo` 最新仍是 **v0.4.8**（与 `go.mod` 一致），无版本可升。只对照 `main`
> （`backup/main-pre-mygo` = v0.3.14 原版树），未翻历史提交。
>
> **偏离点**：本构建只在 `Surf == SurfaceTerminal` 时把设置写进会话
> （`internal/shell/shell.go:985-987` 的 store 监听），而 `Open(SurfaceTerminal)` 对**已经在跑的
> 会话**不再应用外观——`internal/shell/shell.go:2557-2561` 只 `EnsureSession()`（会话在跑就早返回）
> 再 `FocusTerminal()`。于是「在设置页改终端配色/字号/字体族 → 回终端」看到的是旧外观，除非之后
> 又在终端面上改一次设置。原版的终端视图（`main:src/hooks/useTerminalView.ts:502-521`）在
> **每次进入**都新建 `TerminalCanvas`，构造参数就是当帧的 `fontFamily`/`fontSize`/`lineHeight`/
> `theme`/`cursorBlink`/…——离开终端会卸载 renderer 与其 canvas，回来重建，所以原版「回来即当前
> 外观」。本构建的会话跨面常驻，这条 mount 规则落空。
>
> **修法**（`internal/shell/shell.go`）：新增 `Options.RefreshTerminal func()` 缝（默认
> `a.Terminal.Refresh`，测试替换它就能在没有原生会话时观察刷新）；`Open` 的 `case
> SurfaceTerminal:` 在 `EnsureSession()` 之后调 `a.refreshTerminal()`（`shell.go:2561`），把
> 「这一次进入用的就是当前外观」写实；store 监听也改走同一函数（`shell.go:986`）。刷新本身是幂等
> 的（每个会话 `SetFont` + `SetTheme`），对新会话重复一次无害。
>
> **测试**（`internal/shell/terminalappearance_test.go`）：
> `TestReopeningTheTerminalAppliesAppearanceChangedElsewhere`（在设置面改 `FontSize` → 离屏不刷，
> 进终端 → 刷 1 次）与 `TestAnAppearanceChangeOnTheTerminalAppliesAtOnce`（在终端面改设置 → 当次
> 就刷，钉住既有路径没被缝改坏）。变异核查：删掉 `Open` 里的 `a.refreshTerminal()`，第一条断言立刻红
> （`entering the terminal refreshed 0 times, want 1`）。
>
> **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 全绿、
> `GOOS=linux/windows/darwin go build ./...` 三平台通过。

> **上一轮：P7-dp 触发提示的下限是「提示自身不足 6ch」，不是「输入框被挤到 160px 以下」**
> —— P7-do 收尾后「尚未做（P7-do 重写）」第 1 条是最高优先级的具体项（已核实的偏离，带 file:line
> 与验收）。本轮把它落地。依赖检查：GitHub releases API 被限流，`go list -m -versions
> github.com/egoist/mygo` 最新仍是 **v0.4.8**（与 `go.mod` 一致），无版本可升。只对照 `main`
> （`backup/main-pre-mygo` = v0.3.14 原版树），未翻历史提交。
>
> **偏离点**：本构建的 `internal/launcher/triggerhint.go` 把「提示消失」的判据设在**输入框**上——
> `triggerHintMinField = 160`，判 `windowWidth(窗口整行宽) - hintNatural >= 160`。原版判的是
> **提示自己**这一遍布局拿到的**盒宽**：`main:src/launcher/trigger-hint.ts:31-38` 的
> `TRIGGER_HINT_MIN_CELLS = 6`（注释：低于 6 `ch` 时省略号只剩两个字形，提示不如整体消失）与
> `:44-48` 的 `triggerHintFits(width, cellWidth)`（`!(cellWidth > 0) || width >= cellWidth*6`）；
> `main:src/hooks/useTriggerHintFit.ts:37-52` 在 layout 阶段量 `getBoundingClientRect().width`
> 与 canvas 的 `measureText("0")`。两个阈值不同：**提示够 6ch 但输入框不到 160px 时，原版显示、
> 本构建隐藏**（200px 窗口下 old 判据 `200-151=49 < 160` → 隐藏，原版盒宽 ~92px 远大于 6ch → 显示）。
> 另有一处：原版 `white-space: nowrap` + `text-overflow: ellipsis`
> （`main:src/styles/launcher.css:534-569`），本构建的 `ui.Text` 默认**换行**——窄行里提示折成多行，
> 把 R48 省掉的那条 band 又花了回来（200px 实测盒高 40 = 两行）。
>
> **修法**（`internal/launcher/triggerhint.go` + `launcher.go:961-976` 调用点）：
> - `triggerHintMinCells = 6` 取代 `triggerHintMinField = 160`；`triggerHintFits` 改成**纯函数**
>   `triggerHintFits(room, natural, cell float32) bool`：盒宽 `box = min(natural, room)`，
>   `box >= cell*6`；`cell <= 0`（量不到字体）保留提示——「错的隐藏是唯一永久丢失路径」。
> - `(a *App) triggerHintRoom(c, t)` 算**这一行留给提示的盒宽**：行内宽（窗口宽 − 2×`t.Space(4)`）
>   减去保持尺寸的控件与间隙。行是 `[输入框][提示][清除][按钮]`，**输入框是 grower，把整份余量先让给
>   提示**（框架自己的顺序，对应原版 `flex: 1 1 0` 旁的 `flex: 0 1 auto`：挤窄时输入框先塌到 0），
>   所以只有清除键、按钮、以及它们之间的 gap/margin 计入。
> - `(a *App) triggerHintShown(c, t, text)` 在同一帧量 `cell`（`"0"`）与 `natural`（提示全文，都用
>   提示自己的字号 `t.FontSize-1`）再判定。
> - 调用点加 `.SingleLine().Ellipsis("\u2026")`，对齐原版的 `nowrap` + `ellipsis`，也保证提示永远
>   一行、永不夺回 band；隐藏仍是唯一丢失路径。
> - **诚实记录**：原版提示还有 `max-width: 50%` 与 `cellWidthOf` 的 half-em fallback。前者在本构建
>   无落点——启动器窗口固定 720 DIP、UI 步长最大 1.1，提示自然宽 ~166 远低于半行 306；后者对应
>   `MeasureText` 返回 ≤0 即「量不到」，已由 `cell <= 0` 保留分支覆盖。
>
> **测试**（`internal/launcher/triggerhint_test.go` 的 `TestTriggerHintFits` 重写）：6ch 边界
> （`room=60/cell=10` 留、`59.5` 丢）、提示自身短于下限也丢（`natural=59.5`）、`cell<=0` 保命；
> 真实行里 200px 窗口**盒宽 `== triggerHintRoom`** 且只有一行（把解析式与 mygo 的真实布局对钉），
> 120px 无 room → 隐藏。变异核查：把 `cell*triggerHintMinCells` 换回 `160` 或 `cell*5`，边界断言
> 立刻红；去掉 `.SingleLine()`，盒高断言立刻红。
>
> **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 全绿、
> `GOOS=linux/windows/darwin go build ./...` 三平台通过。

> **上一轮：P7-do 任何 reveal 都重读安装目录（原版 `src/App.tsx:2453-2457` 的
> `floter://revealed` 监听 + `src/extensions/tool-catalog-store.ts:52-63`）**
> —— P7-dn 收尾后清单只剩 P7-dn 第 3 条（深链/托盘 reveal 也重读）。本轮把它落地：给 `Show()` 的
> reveal 判定加一个可注入的可见性缝，把刷新放进 `Show()` 本身——它是本构建每一条 reveal 的唯一门口。
> 依赖检查：GitHub releases API 被限流，`go list -m -versions github.com/egoist/mygo` 最新仍是
> **v0.4.8**（与 `go.mod` 一致），无版本可升。只对照 `main`（`backup/main-pre-mygo` = v0.3.14
> 原版树），未翻历史提交。
>
> **偏离点**：P7-dn 把刷新接在 `summon()` 与 `returnToInput()` 两处，原版的 reveal 监听却是
> **凡窗口从隐藏变可见就刷新**——深链（`HandleURL` → `Open` → `Show`）、第二实例启动
> （`cmd/floter/main.go:64` 的 `app.Show()`）、托盘/全局快捷键（`Toggle` → `summon`）都算。其中深链
> 与第二实例此刻都不经过 `summon()`，用户在窗口隐藏期间用自己的 shell 装了工具，召回后旧安装行
> 还在。P7-dn 已把这一点记为待办第 3 条，但当时卡在「`Show()` 在 `a.Win == nil` 时提前 return，
> 测试没有窗口可造」。
>
> **修法**：判定与落点都放进 `Show()`，因为它是本构建每一条 reveal 的唯一门口（与框架
> `Window.Show()` 内部的 `if !visible { fire(onShow) }` 同一判定，见 `mygo@v0.4.8:window.go:624-637`）：
> - `internal/shell/shell.go` 的 `Show()`：先取 `rehome := placementForReveal(a.windowWasVisible())`，
>   `rehome` 为真时 `a.scanToolCatalog()`，再走窗口操作。于是 `summon()`/`returnToInput()` 已有的
>   显式刷新与这一次天然共享（在飞去重），深链/第二实例/托盘 reveal 也第一次有了刷新。
> - 新增 `App.windowVisible func() bool` 缝（nil → `a.Win == nil || a.Win.IsVisible()`）；测试无窗口
>   可读可见性，用它驱动 reveal 与非 reveal 两侧。nil 窗口读作「已在屏上」，因为没窗口就无从
>   reveal，也避免既有测试里每次 `Open()` 都起一次真实 `tools.Look`。
> - **诚实记录**：没有改用 `mygo.Window.OnShow` 注册回调——那需要真实窗口，shell 测试（`Win == nil`）
>   无法接线，本轮验收就落不了地。`Show()` 的判定与 `OnShow` 内部同源，覆盖的 reveal 集合一致。
>
> **测试**：`internal/shell/shell_test.go` 新增 `TestARevealRefreshesTheInstallCatalog`（窗口隐藏 →
> `Show()` 扫一次）、`TestASurfaceSwitchOnAVisiblePanelKeepsTheCatalogMemo`（窗口已可见 → 不扫）、
> `TestAVisiblePanelIsNotRehomed`（reveal 判定与 re-home 判定同一读数）。变异核查：把 `Show()` 里的
> `if rehome { a.scanToolCatalog() }` 删掉/改成 `if false`，第一条立刻红；改成 `if true`，第二条立刻红。
>
> **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 27 包全绿、
> `GOOS=linux/windows/darwin go build ./...` 三平台通过；`go test -race ./internal/shell -run
> 'Catalog|Reveal|SurfaceSwitch'` 无竞态。

> **上一轮：P7-dn 安装目录在召回时重读机器（原版 `src/App.tsx:2453-2457` 的
> `floter://revealed` 监听 + `:781-788` 的 `returnToInputMode`（R74），
> `src/extensions/tool-catalog-store.ts:52-63` 的 `refreshToolCatalog`）**
> —— P7-dm 收尾后「尚未做」清单为空，本轮按惯例换入口：逐文件对照原版**安装目录的读取/刷新**
> （`extensions/tool-catalog-store.ts`，此前未被任一待办覆盖）。依赖检查：GitHub releases API
> 仍被限流，`go list -m -versions github.com/egoist/mygo` 最新仍是 **v0.4.8**（与 `go.mod`
> 一致），无版本可升。只对照 `main`（`backup/main-pre-mygo` = v0.3.14 原版树），未翻历史提交。
>
> **偏离点（三处，一处本轮修）**：原版把安装目录的读取当成一件**有 memo、有在飞去重、每次 reveal
> 重读**的事：
> 1. **每次原生 reveal 重读一次**（`main:src/App.tsx:2453-2457`，注释「A tool installed in the
>    user's own shell while the window was hidden must stop being offered as an install row on the
>    next summon; between reveals the module memo serves every query. No polling, no timer.」）。
>    本构建只在启动（`internal/shell/shell.go:1107`）与托盘 Rescan（`internal/shell/system.go:495`）
>    读，`summon()`（`shell.go:2652`）从不重读——用户在隐藏窗口期间用自己的 shell 装了工具，召回后
>    旧安装行还在，直到重启或手动 Rescan。
> 2. **回到输入框再重读一次**（`main:src/App.tsx:781-788`，R74：安装行自己的终端门**不隐藏窗口**，
>    所以 reveal 监听看不到这次返回）。本构建的 `returnToInput()`（`shell.go:3012`）同样不读。
> 3. **在飞请求共享**（`tool-catalog-store.ts:52-63`，`refreshToolCatalog` 复用 `inFlight`）。本构建
>    的 `scanToolCatalog` 每次调用各起一个 goroutine，没有去重。
>
> **修法**（第 1、2、3 条同轮落地——同一个刷新机制）：
> - `internal/shell/shell.go`：`Options` 新增 `LookTools func() []tools.State`（nil 用真的
>   `tools.Look`，测试替换掉文件系统）；`scanToolCatalog` 走 `a.lookTools()`，并以
>   `catalogMu`/`catalogScanning` 做**在飞去重**（在飞则直接返回，共享那一次扫描）。
> - `summon()`（reveal，`shell.go:2657`）与 `returnToInput()`（R74 的归位，`shell.go:3018`）各加
>   一处 `a.scanToolCatalog()`，正对原版仅有的两个刷新点。
> - **诚实记录**：原版注释里的「每次 reveal」是原生 `floter://revealed` 事件，凡让窗口从隐藏变可见
>   都算（含深链 `HandleURL` → `Open` → `Show`）；本构建只接了 **summon** 与 **归位** 两条最常用的
>   reveal（也是用户报的那条「装了工具还显示安装行」的路径），深链 reveal 走 `Show()` 的 rehome
>   分支，留作待办第 3 条。
> - **顺带修掉一条既有竞态**：`TestToolCatalogReachesTheLauncher`（`internal/shell/shell_test.go:2230`）
>   轮询 `a.Launcher.InstallCatalog` 时没有同步，`-race` 下报数据竞态（属既有问题，非本轮引入；已用
>   新的 `awaitCatalogIdle` 修掉）。
>
> **测试**：`internal/shell/shell_test.go` 新增 `TestASummonRefreshesTheInstallCatalog`、
> `TestTheWayBackRefreshesTheInstallCatalog`、`TestOneCatalogScanRunsAtATime`、
> `TestARefreshReplacesTheInstallCatalog`，另有 `awaitCatalogStart`/`awaitCatalogIdle` 两个等待
> 助手。变异核查：删掉 `summon()` 那行，summon 测试立刻红；删掉 `returnToInput()` 那行，归位测试立刻
> 红；把在飞去重改成 `if false`，`TestOneCatalogScanRunsAtATime` 立刻红。
>
> **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 27 包全绿、
> `GOOS=linux/windows/darwin go build ./...` 三平台通过；`go test -race ./internal/shell -run Catalog`
> 无竞态。
>
> **上一轮：P7-dm 脚本工具链探测改成异步，状态行终于有「正在检查」（原版
> `src/ExtensionsPanel.tsx:925-961` 的 `probeScriptRuntime` 与
> `src/extensions/CustomIntegrationDrawer.tsx:112-121` 的 `ScriptRuntimeLine`）**
> —— P7-dl 收尾后「尚未做」只剩 P7-di 第 3 条里那半条「异步 checking 态」，先前被写成「卡框架」。
> 本轮核对框架后确认**并不卡**：mygo 的 `Window.Invalidate` 注释明写「safe from any goroutine」
> （`mygo@v0.4.8:content.go:145-147`），所以 `go func` + 重绘通道一直可用，之前只是没有接线。
> 依赖检查：GitHub releases API 仍被限流，`go list -m -versions github.com/egoist/mygo` 最新仍是
> **v0.4.8**（与 `go.mod` 一致），无版本可升。只对照 `main`（`backup/main-pre-mygo` = v0.3.14
> 原版树），未翻历史提交。
>
> **偏离点（一处，探测线程）**：原版的探测是 `invoke(...)` 异步；本构建的
> `extensions.ScriptRuntimeFor` 是同步 `exec`。改前它在渲染帧里直接调用（P7-di 已缓存，但第一帧
> 仍会阻塞到 3s 超时）。本轮把调用挪到自己的 goroutine，缓存条目变成「在飞 / 已落」两态。
>
> **修法**：
> - `internal/settingsui/settingsui.go`：`scriptRuntime` 从 `map[string]scriptlang.Check` 改成
>   `map[string]*runtimeProbe`（`runtimeProbe{done, check}`）+ `scriptRuntimeMu` + `scriptRuntimeEpoch`。
>   `scriptRuntimeStart` 起 goroutine（去重、epoch 守卫，落盘后 `Actions.Invalidate()`）；
>   `scriptRuntimeCheck`（渲染读）返回 `*Check`（nil = 在飞）；新增 `scriptRuntimeVerdict`（保存等待，
>   对应原版 `await probeScriptRuntime`）；`clearScriptRuntimeCache` 改成 epoch+1 并清空。
>   `Actions` 新增 `Invalidate func()`，`internal/shell/shell.go` 接到 `a.Win.Invalidate`。
> - `scriptRuntimeLine`：`check == nil` → `copy.LocalToolRuntimeChecking`（原版 `!check` → checking）；
>   没接线探测仍只说工具链名字。
> - `internal/i18n`：新增 `LocalToolRuntimeChecking`，en/zh 逐字对齐旧字典
>   （"Checking the local toolchain…" / "正在检查本机工具链…"）。
> - **诚实记录**：本构建的保存路径等探测时**阻塞当前帧**（原版异步 + busy 标记），因为
>   `scriptRuntimeVerdict` 走的是同一帧里的点击处理；探测通常早已落（表单打开时就起了），最坏
>   情况是 `--version` 卡到 3s 超时。没做成异步保存是刻意的：同步等待才对得上原版「先 await 再
>   决定警告」的语义，也避免了「保存时还没判完」这种半态。
>
> **测试**：`internal/settingsui/pluginsheet_test.go` 新增
> `TestScriptRuntimeLineSaysCheckingUntilTheProbeLands`（阻塞探测 → 检查中 → 放行 → missing）、
> `TestAStaleProbeFromAClosedFormIsDropped`（epoch 守卫：关表单后落地的旧探测不进缓存）；
> 既有 `TestScriptRuntimeProbeIsCachedPerLanguage` / `TestReopeningTheLocalToolFormReprobesTheToolchain`
> / `TestLocalToolFormBuildsAScript` 增加 `awaitRuntime` 等待（探测已不在渲染帧内同步完成）。
> `internal/i18n/i18n_test.go` 的 `TestScriptRuntimeWordsMatchTheOldDictionary` 加 en/zh 两句 checking。
> 变异核查：把 `scriptRuntimeLine` 的 `check == nil` 分支改回走 `StatusOf`，两条 checking 断言立刻红；
> 去掉 epoch 守卫，`TestAStaleProbeFromAClosedFormIsDropped` 立刻红。
>
> **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 27 包全绿（1094 项）、
> `GOOS=linux/windows/darwin go build ./...` 三平台通过；`go test -race ./internal/settingsui` 无竞态。
>
> **上一轮：P7-dl 脚本工具链状态行的措辞与「没问过」语义（原版
> `src/extensions/CustomIntegrationDrawer.tsx:112-121` 的 `ScriptRuntimeLine` +
> `src/i18n.ts:696-699,1576-1579`）**
> —— P7-dk 收尾后「尚未做」的 P7-di 第 3 条是最高优先级的具体项（P7-dg 第 3 条是原版自己都没接线的
> 死函数、P7-dd 第 3 条卡 mygo 框架），本轮取它。依赖检查：`go list -m -versions
> github.com/egoist/mygo` 最新仍是 **v0.4.8**（与 `go.mod` 一致），无版本可升。只对照 `main`
> （`backup/main-pre-mygo` = v0.3.14 原版树），未翻历史提交。
>
> **偏离点（两处，同一状态行）**：
> 1. **三条词自造**。原版 ready 行是 `${name} ${version}` 或 `name`（**没有** "is ready" 后缀，
>    后缀留给 `customScriptCompiled` 的 `<em>` 与 `path` 的 `<code>`）；missing 行是
>    "Not found on PATH: {names}. Install it, then reopen this form to check again."（zh
>    "在 PATH 中未找到：{names}。安装后重新打开此表单即可再次检查。"）；compiled 是两词
>    "built before running"（zh "运行前先编译"）。本构建的 `LocalToolRuntimeReady` 自造了 `+ " is
>    ready"`/`+ " 可用"`，missing 与 compiled 也各改写成自己的句子。
> 2. **零值 `Check` 被读成 missing**。`scriptlang.Check` 的注释明写「zero Check means 'not asked
>    yet' — never 'missing'」，但 `StatusOf` 对零值返回 `Missing`，与自己的注释相矛盾。
>
> **修法**：
> - `internal/scriptlang/scriptlang.go`：新增 `Check.Asked()`（有 observation 才算问过：
>   `Available`/`Path`/`Version`/`Candidates` 任一；`Compiled` 是语言的投影、不是观测，单有它仍是
>   没问过），`StatusOf` 对「没问过」返回 `Status{State: Unknown, Name: toolchain}`。
> - `internal/i18n/copy.go`：三条词逐字对齐旧字典（en/zh 各三处）。
> - `internal/settingsui/settingsui.go` 的 `scriptRuntimeLine`：走一个 `StatusOf` 的 `switch`，
>   没接线探测与「问过但零值」现在同一条 `default` 分支 → 只说工具链名字。
> - **诚实记录**：原版的 checking 态（`check === null` → `customScriptRuntimeChecking`）在本构建仍
>   无落点——探测是同步的，渲染帧里不存在「在飞」的时刻。按清单的告示，没有异步探测前**不新增**
>   checking 词，避免再造死旋钮；异步化（`go func` + 重绘）留作独立一轮。
>
> **测试**：`internal/scriptlang/scriptlang_test.go` 的 `TestStatusOf`（零值 → `Unknown`、不报 missing、
> go 的零值仍带 `Compiled`）与新增 `TestCheckAsked`（7 行表，含「只有 Compiled」仍是没问过）；
> `internal/i18n/i18n_test.go` 的 `TestScriptRuntimeWordsMatchTheOldDictionary` 逐字钉 en/zh 三句
> （含 ready 无版本）。变异核查：把 `StatusOf` 的零值分支改回 `Missing`，`TestStatusOf` 的
> 「zero check」断言立刻红。
>
> **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 27 包全绿、
> `GOOS=linux/windows/darwin go build ./...` 三平台通过。

> **上一轮：P7-dk 动作栏的「存在性」与「默认选择」拆开（原版 `src/App.tsx:1547-1552` 的
> `visibleActionBar` + `src/hooks/useLauncherCatalog.ts:1432-1443` 的 `defaultsToActionBar`）**
> —— P7-dj 收尾后「尚未做（P7-dj 本轮发现）」只此一条（本轮落地）。依赖检查：
> `go list -m -versions github.com/egoist/mygo` 最新仍是 **v0.4.8**（与 `go.mod` 一致），无版本可升。
> 只对照 `main`（`backup/main-pre-mygo` = v0.3.14 原版树），未翻历史提交。
>
> **偏离点**：本构建把动作栏的「存在性」与「默认选择」绑成了一件事：改前
> `internal/launcher/actionbar.go:96-102` 的 `actionShell && !defaultsToShell(other)` 直接
> `return nil`，于是查一个**能跑的命令**（catalog 命中、`defaultsToShell` 为假）时整个终端行消失——
> 用户报告的「底部应固定出现「在终端中打开」选项…目前缺失」。原版两件事分开：
> `visibleActionBar` 只看列表非空就画（拖入文件走 `fileRows` 支），`defaultsToActionBar` 只决定
> Enter 落在哪一行。
>
> **修法**：`actionBarItemsWith(other, gated)` 的 shell 闸门改成
> `gated && actionShell && len(other)==0`（`actionbar.go:124`）——匹配到东西就画、匹配不到就不画
> （对齐 `visibleActionBar` 的 `launcherResults.length > 0`），只有 search 传 `gated=true`；
> `actionBarItems()`（不过闸）供 held-modifier 与 Enter 兜底。`activate` 补上原版的
> `!launcherResults.length` 兜底：列表为空且 query 是命令行时直接跑 `actionBarItems()[0]`
> （`launcher.go:1849-1856`），所以匹配不到的命令行仍能 Enter 运行、只是页面上不多出一行。
> `assignNumbers` 跳过 `bar:` 行（`launcher.go:432-470`，原版 `⌘N` 不占动作栏），`defaultSelection`
> 从第一条非 `bar:` 的可运行行起步（`launcher.go:2047-2069`）。query 不经输入框变化时的帧内对齐由
> `defaultedQuery` 承担（`launcher.go:789-795`、`:947-952`）。
>
> **测试**：13+ 项既有测试重新对齐——`search_test.go` 改用 `indexOfID`；`resultnumbers_test.go` /
> `view_test.go` 的编号断言跳过头部 `bar:` 行；匹配不到任何东西的那几项改断言页面**没有** shell 行，
> 并靠 Enter 兜底验证仍会运行。变异核查：把闸门恢复成 `actionShell && len(other)==0`（shell 行永远
> 隐藏），`TestActionBarRows` 立刻红。
>
> **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 全绿、
> `GOOS=linux/windows/darwin go build ./...` 三平台通过。

> **上一轮：P7-dj 缺工具链的脚本保存时警告一次（原版 `src/ExtensionsPanel.tsx:1863-1875`
> 的 `createCustomIntegration` + `src/i18n.ts:698,1578`）**
> —— P7-di 收尾后「尚未做（P7-di 重写）」还剩第 2 条（本轮落地）与第 3 条（措辞对齐，卡「同步探测
> 没有在飞态」）。依赖检查：GitHub releases API 被限流，用
> `go list -m -versions github.com/egoist/mygo` 核对，最新仍是 **v0.4.8**（与 `go.mod` 一致），无版本
> 可升。只对照 `main`（`backup/main-pre-mygo` = v0.3.14 原版树），未翻历史提交。
>
> **偏离点**：原版保存脚本前 `await probeScriptRuntime(language)`，`!check.available` 就
> `onNotify("warning", customScriptRuntimeSaveWarning)` —— 注释原话：缺工具链**不阻止保存**（用户可以
> 装完再回来），但只警告一次，「so the integration is not silently dead on arrival」。本构建的提交路径
> （`internal/settingsui/settingsui.go:2796-2807`）只做 `customIssues` 内联校验，缺工具链既不是 issue
> 也没有任何提示：集成照常写入、在启动器里照常出现，第一次运行才失败。
>
> **修法**：`internal/settingsui` 的 `Actions` 新增 `NotifyWarning func(text)`（nil 就静默丢弃）；提交
> 路径在 script 模式下读**表单自己缓存的那次探测**（P7-di 的 `scriptRuntimeCheck`，所以这句提示不会
> 再起一次 PATH 扫描），`scriptlang.StatusOf(...).State == Missing` 时才发；探测没接线（`ok == false`）
> 或工具链在 → 一个字都不说。措辞进 i18n：`LocalToolRuntimeSaveWarning(language)`，en/zh 逐字对齐原版
> 字典（`{language}` 用 `scriptlang.Lookup(id).Label`，即原版 `scriptLanguageLabel`）。落点是 app 自己的
> **notice**（`internal/shell/notices.go` 的 `warnNotice` → `ui.Toast`），不是启动器那行 docked 状态行
> ——用户此刻在设置页，而那行只在启动器表面画；`queueNotice` 顺手把窗口做成参数（error 8s，对齐原版
> `TOAST_DISMISS_MS.error`），warning 用原版的 6s。
>
> **测试**：`internal/settingsui/pluginsheet_test.go` 的
> `TestSavingAScriptWithoutItsToolchainWarnsAndStillSaves`（JS 缺工具链 → 一条 warning 且**仍然连上**；
> 换成 python 在 PATH 上 → 一个字不说）与 `TestAScriptSaveWarnsOnlyFromAVerdict`（没接线探测 → 不猜）；
> `internal/shell/notices_test.go` 的 `TestWarningNoticeShowsWhereTheUserIs`（warning 排队一个、窗口是 6s、
> 画在设置表面且不改表面、空串不成 toast）。变异核查：把 `NotifyWarning` 那段分支关掉，settingsui 那条
> 测试立刻红。
>
> **顺带发现（未落地 → 已落地，见 P7-dk）**：启动器的「在终端中打开」行在本构建里**只有 Enter 会跑它时才存在**
> （`internal/launcher/actionbar.go:96-102` 的 `actionShell && !defaultsToShell` 直接返回 nil），原版
> `visibleActionBar`（`src/App.tsx:1547-1552`）的条件是**匹配结果非空**就画，`defaultsToActionBar`
> （`src/hooks/useLauncherCatalog.ts:1432-1443`）只决定选择落在哪一行。这条既是用户报告的
> 「底部应固定出现「在终端中打开」选项…目前缺失」，也是一处需要改 13 项既有测试的独立改动，已在
> P7-dk 落地。

> **上一轮：P7-di 脚本工具链探测按语言缓存（原版 `src/ExtensionsPanel.tsx:822-828,925-961`
> 的 `runtimeChecksRef`/`probeScriptRuntime` + `src/extensions/script-languages.ts:158-178`）**
> —— P7-dh 收尾后「尚未做（P7-dg 重写）」只剩第 3 条（原版自己都没接线的死函数，明确「没有落点前
> 不做」）与 P7-dd 的第 3 条（卡 mygo 框架），清单又一次为空。本轮换入口：逐文件对照原版的**脚本
> 工具链状态行**（`src/extensions/script-languages.ts` 的 `scriptRuntimeStatus` + 它在
> `CustomIntegrationDrawer.tsx` 的渲染 + `ExtensionsPanel.tsx` 的探测缓存），把清单写成下面三条带
> file:line 的具体待办（下方「具体待办（P7-di 重写）」），并取第 1 条落地。依赖检查：
> GitHub releases API 被限流，用 `go list -m -versions github.com/egoist/mygo` 核对，最新仍是
> **v0.4.8**（与 `go.mod` 一致），无版本可升。只对照 `main`（`backup/main-pre-mygo` = v0.3.14
> 原版树），未翻历史提交。
>
> **偏离点**：原版把工具链探测当成一件**有缓存、有在飞态**的异步事：`probeScriptRuntime`
> （`ExtensionsPanel.tsx:925-951`）先查 `runtimeChecksRef`（命中即返回）、再查在飞 Map（去重）、
> 然后 `setRuntimeChecks(language: null)`（渲染成「检查中」）并发起 `extensions_script_runtime_check`，
> 完成后写回缓存；`useEffect`（`:958-961`）只在语言变化时探测；打开/编辑表单（`:1558-1566`、
> `:1593-1595`）把 `runtimeChecksEpoch += 1` 并清空缓存——注释原话「the probe runs once per language
> per drawer session rather than per keystroke」「install it, then reopen this form to check again」。
> 本构建的 `internal/settingsui/settingsui.go:2945` 在**每一帧渲染**里直接调 `a.scriptRuntimeLine`，
> 后者（`:2974`）调 `a.Actions.ScriptRuntime(language)`；而 `internal/shell/shell.go:583-585` 的
> `ScriptRuntime` 是同步的 `extensions.ScriptRuntimeFor`，会 `exec.Command(path, "--version")`
> （`scriptVersionTimeout` 3s，`internal/extensions/scriptruntime.go:33-47`）。于是脚本表单每画一帧、
> 每敲一个字符都起一次子进程，还可能在 UI 线程上阻塞到 3 秒。
>
> **修法**：`internal/settingsui/settingsui.go` 新增 `App.scriptRuntime map[string]scriptlang.Check`
> （对齐 `runtimeChecksRef`）与 `scriptRuntimeCheck(language) (scriptlang.Check, bool)`——命中缓存直接
> 返回，未命中探测一次后落缓存，`Actions.ScriptRuntime` 为 nil 时返回 `false` 让 `scriptRuntimeLine`
> 退回工具链名；新增 `clearScriptRuntimeCache()`（对齐 `runtimeChecksEpoch` 的清空）。清缓存的三个
> 落点：`localToolSection` 打开表单的 toggle（`:2606` 附近）、`beginEdit`、`endEdit`——「重新打开
> 表单」正是原版要求重探的时刻。
>
> —— **后续（P7-dm）**：这里的同步缓存被改成异步（`runtimeProbe` goroutine + `Actions.Invalidate`），
> 渲染帧里不再 `exec.Command`；上面的 `map[string]scriptlang.Check` 已换成 `map[string]*runtimeProbe`。
>
> **测试**：`internal/settingsui/pluginsheet_test.go` 的 `TestScriptRuntimeProbeIsCachedPerLanguage`
> （同语言读两次只探一次、换语言各探一次、切回命中、清缓存后重探）与
> `TestReopeningTheLocalToolFormReprobesTheToolchain`（打开表单探一次、多帧重绘不再探、关闭重开再探
> 一次）。变异核查：把 `scriptRuntimeCheck` 的缓存查找删掉（每次直接 `a.Actions.ScriptRuntime`），
> 两条测试立刻红。
>
> **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 全绿、
> `GOOS=linux/windows/darwin go build ./...` 三平台通过。

> **上一轮：P7-dh 动作失败不再冒充写入失败（原版 `src/plugins/controls.tsx:206-260` 的
> `ActionControl` + `PluginConfigOverlay.tsx:299-309` 的失败行）**
> —— P7-dg 收尾后「尚未做（P7-dg 重写）」还剩第 2 条（失败来源合并）与第 3 条（原版自己都没接线的
> 死函数，低优先级）。本轮取第 2 条落地。依赖检查：GitHub releases API 被限流，用
> `go list -m -versions github.com/egoist/mygo` 核对，最新仍是 **v0.4.8**（与 `go.mod` 一致），
> 无版本可升。只对照 `main`（`backup/main-pre-mygo` = v0.3.14 原版树），未翻历史提交。
>
> **偏离点**：原版把两条失败线分得清清楚楚。写入线是 overlay 的 `persistFailed`（由
> `handleChange` 的 `rollback` 置起，渲染成 `settings.saveFailed`，`loadFailed` 优先于它，见
> `PluginConfigOverlay.tsx:299-309`）；动作线属于控件自己——`ActionControl`（`controls.tsx:209-260`）
> 有独立的 `failed` 状态，`onRun` 解析为 `false` 时 disarm 并在按钮旁打印字段自己的
> `failedKey`（`.plugin-config-action__status`），**不碰** overlay 的失败行；重新 arm 时
> `setFailed(false)`。本构建 `internal/launcher/config.go` 的 `Config.Failed bool` 同时被
> `ChangeConfig`（写入）与 `RunConfig`（动作）置起，渲染在 `internal/settingsui/pluginsheet.go:70-75`
> 的 `copy.SaveFailed`——于是 `clear_history` 失败会说「无法保存该修改」，而设置其实没动。
>
> **修法**：`launcher.Config` 新增 `ActionFailed string`（被拒动作的 key，空为无）；`RunConfig`
> 改置 `ActionFailed`、不再动 `Failed`（`ConfigAction` 为 nil 时按原版 `onRun?.()` 的 no-op 不报）；
> `ArmConfig` 在重新 arm 时清掉该行（对齐 `ActionControl` 的 `setFailed(false)`）。
> `settingsui.PluginSheet` 新增 `ActionFailed`，`sheetAction` 在未 arm 时把 `field.Failed` 画在
> 按钮旁（Warning 色），arm 后让位给 confirm/cancel；`shell.drawPluginConfig` 透传。`Failed` 仍是
> 写入线（`settings.saveFailed`）。
>
> **测试**：`internal/launcher/config_test.go` 的 `TestRefusedConfigActionReportsOnItsOwnLine`
> （拒绝→`ActionFailed=key`、`Failed` 假、已 disarm；重新 arm 清行）、`TestAcceptedConfigActionClearsTheLine`、
> `TestWriteAndActionFailuresAreSeparateLines`（写失败不碰动作行）；
> `internal/settingsui/pluginsheet_test.go` 的 `TestPluginSheetActionFailureHasItsOwnLine`
> （画 `field.Failed`、不画 `SaveFailed`、arm 后让位）；`internal/shell/pluginconfig_test.go` 的
> `TestPluginConfigActionFailureReportsOnTheActionLine`（端到端：让清空必然失败——store 的父路径是
> 一个文件，`mkdir` 报 ENOTDIR，连 root 也绕不过——渲染 `ClipboardClearFailed` 而非 `SaveFailed`）。
> 变异核查：`RunConfig` 改回 `a.config.Failed = true` → launcher 两条红；`sheetAction` 的
> `if sheet.ActionFailed == field.Key` 关掉 → settingsui 与 shell 各一条红。
>
> **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 全绿（1085 项）、
> `GOOS=linux/windows/darwin go build ./...` 三平台通过。

> **上一轮：P7-dg 被拒绝的配置写入要回落（原版 `src/plugins/config-persist.ts:44-50` 的
> `configChangeOutcome` + `src/plugins/PluginConfigOverlay.tsx:233-249` 的 `handleChange`）**
> —— P7-df 收尾后「尚未做（P7-dd 重写）」只剩第 3 条（卡 mygo 框架），清单又一次为空。
> 本轮换入口：逐文件对照原版**插件配置的写入路径**（`plugins/config-persist.ts`、
> `plugins/config-schema.ts`、`plugins/PluginConfigOverlay.tsx`），把清单写成下面三条带
> file:line 的具体待办（下方「具体待办（P7-dg 重写）」），并取第 1 条落地。依赖检查：
> `curl .../releases/latest` 被限流，用 `proxy.golang.org` 的 `@v/list` 与
> `go list -m -versions` 双核对，最新仍是 **v0.4.8**（与 `go.mod` 一致），无版本可升。
> 只对照 `main`（`backup/main-pre-mygo` = v0.3.14 原版树），未翻历史提交。
>
> **偏离点**：原版 R29 先乐观上屏、再异步写；写失败被 `.catch(() => undefined)` 吞掉，于是控件
> 继续显示后端没存的值。R113 给写入加了结果：`writeConfigChange` 把拒绝变成 `false`，
> `configChangeOutcome` 被拒绝时把上屏值**回落到上次确认的快照**（`rollback`）并给出失败行
> （`settings.saveFailed`），被接受时才推进快照。本构建 `internal/launcher/config.go:98-115` 的
> `ChangeConfig` 把上屏值写进 `a.config.Values`、调 `ConfigChange`，被拒绝（返回 nil）时只置
> `a.config.Failed = true` 就 `return`——**上屏值留在屏幕上**，与 R29 的「静默失效」同病。
> 注释「keeps the value it painted and says so」正是这条偏离的自我合理化。
>
> **修法**：新增纯函数 `internal/plugincfg/persist.go` 的
> `ConfigChangeOutcome(next, committed Values, accepted bool) (values Values, failed bool)`——
> 接受→返回上屏值、`failed=false`；拒绝→返回上次确认的 `committed`、`failed=true`。
> `ChangeConfig` 在改写前把 `a.config.Values` 记为 `committed`（每条路径都让它等于后端最后确认的
> 值，所以无需再加一个会漂移的字段），拒绝时 `a.config.Values = values` 回落；接受时仍以插件归一化
> 后的 `stored` 为准（既有行为，`changePluginConfig` 的返回值）。
>
> **测试**：`internal/plugincfg/persist_test.go` 的 `TestConfigChangeOutcomeCommitsOrRollsBack`
> 与 `TestConfigChangeOutcomeRollsBackEveryField`；`internal/launcher/config_test.go` 的
> `TestRefusedConfigChangeRollsBack`（拒绝→回落 `true` 且 `Failed`）、
> `TestAcceptedConfigChangeCommitsTheStoredValues`（接受→提交 `stored` 且清失败行）、
> `TestRefusedChangeRollsBackToTheLastAccepted`（回落目标是**上一次被接受**的快照）。
> 变异核查：把 `ChangeConfig` 的 `a.config.Values = values` 删回 `return`，
> `TestRefusedConfigChangeRollsBack` 的 `enabled` 一行立刻红。
>
> **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 全绿、
> `GOOS=linux/windows/darwin go build ./...` 三平台通过。

> **上一轮：P7-df 剪贴板清空历史的完成反馈（原版 `src-tauri/src/clipboard_history/mod.rs:277-303`
> 的 `clipboard_clear_history` → `notifications.rs` 的 `notify_completion`）**
> —— P7-de 收尾后「尚未做（P7-dd 重写）」只剩第 2 条（具体可做）与第 3 条（卡 mygo 框架），
> 本轮取第 2 条落地。依赖检查：`curl .../releases/latest` 与 `go.mod` 都停在 **v0.4.8**，无版本可升。
> 只对照 `main`（`backup/main-pre-mygo` = v0.3.14 原版树），未翻历史提交。
>
> **偏离点**：原版 `clipboard_clear_history` 在清空后调 `notify_completion(&app,
> Subject::Plugin(CLIPBOARD_PLUGIN_ID), CompletionAction::ClearHistory, outcome_of(&result))`——
> 插件页的这条长动作走与其他后台任务同一条完成规则：可见→面板自己的反馈，隐藏→系统通知，
> `{name}` 是插件页的双语名（`{name} cleared` / `{name} could not be cleared`）。本构建两条清空路径
> （`internal/shell/pluginconfig.go:141` 的 `clearClipboardHistory`、`internal/shell/shell.go:1744` 的
> `clearClipboardHistoryAsks`）都只 `Win.Invalidate()`/`log.Printf`，从不调 `notifyCompletion`，
> 面板隐藏时没有任何通知；i18n 的 `Notifications` 也没有这两条词。
>
> **修法**：`internal/shell/pluginconfig.go` 的 `clearClipboardHistory` 成为唯一清空核心（先
> `a.notifyClipboardClear(err == nil)` 再落盘/日志），`clearClipboardHistoryAsks` 确认后直接委托它；
> 新增 `internal/shell/shell.go` 的 `notifyClipboardClear(success)`——`{name}` 取
> `i18n.For(lang).Settings.ClipboardPlugin`（插件页的本地化名），走既有 `notifyCompletion`。
> 为让「可见→面板反馈 / 隐藏→通知」两条通道都能在无真窗口下测到，`notifyCompletion`/`notifyRun`
> 的「在屏」判定改走 `a.panelOnScreen()`（默认读窗口两个信号，测试可注入 `a.onScreen`）。
> `internal/i18n` 的 `Notifications` 补 `ClipboardCleared`/`ClipboardClearFailed`（en/zh 逐字对齐旧字典）。
>
> **测试**：`internal/shell/shell_test.go` 的 `TestClipboardClearReportsWhereTheUserIsLooking`
> （隐藏→系统通知带 `Clipboard history cleared`；可见→面板 `Feedback` 且零通知；失败→
> `Clipboard history could not be cleared`）。变异核查：删掉 `clearClipboardHistory` 里的
> `a.notifyClipboardClear(err == nil)`，`notifications = []` 一行立刻红。
>
> **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 全绿（31 包）、
> `GOOS=linux/windows/darwin go build ./...` 三平台通过。

> **上一轮：P7-de 前台判定补上「已聚焦即在场」（原版 `src-tauri/src/notifications.rs:168-190`
> 的 `panel_foreground_from`）**
> —— P7-dd 收尾后「尚未做」只剩第 1、2 条（具体可做）与第 3 条（卡 mygo 框架），本轮取第 1 条落地。
> 依赖检查：`go list -m -versions github.com/egoist/mygo` 停在 **v0.4.8**（与 `go.mod` 一致），
> 无版本可升。只对照 `main`（`backup/main-pre-mygo` = v0.3.14 原版树），未翻历史提交。
>
> **偏离点**：原版 `panel_foreground`（`notifications.rs:168-190`）组合三个信号——
> `AppState::window_visible`（应用自己的记录）、`Window::is_visible()`、`Window::is_focused()`——
> **任一**说「在屏」就算 `Visible`，注释明写「a focused window is on screen by definition, even on
> the platforms where the panel's visibility is reported late（macOS `NSPanel` before its first
> map）」。本构建只有窗口的两个信号，且两条完成路径（`notifyCompletion`、`notifyRun`）都只看了
> `IsVisible()`——于是「窗口已聚焦、但 `IsVisible` 尚未报真」的那一帧会被当成隐藏，在用户正看着的
> 面板上再叠一条系统通知。
>
> **修法**：`internal/shell/shell.go` 新增纯函数 `panelForeground(visible, focused bool) bool`
> （`visible || focused`），`notifyCompletion`（`:2386`）与 `notifyRun`（`:2338`）都改走它。
> 未引入原版的第三个信号（应用自己的 `window_visible` 记录）：mygo 只暴露
> `IsVisible`/`IsFocused` 两个旋钮（`mygo@v0.4.8/window.go`），本构建也没有等价的应用级记录，
> 而这条规则的方向（任一为真即在场）不受影响——这也正是本轮要修的偏差。
>
> **测试**：`internal/shell/appkeys_test.go` 的 `TestPanelForegroundAnyOnScreenSignalWins`——
> 四象限表（可见/聚焦任一为真 → 面板内反馈、不通知；都假 → 通知）。
> 变异核查：把 `return visible || focused` 改成 `return visible`，
> `focused but visibility reported late` 一行立刻红。
>
> **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 31 包全绿、
> `GOOS=linux/windows/darwin go build ./...` 三平台通过。

> **上一轮：P7-dd 单行文本字段自己认领 ↑/↓（原版 `src/settings-nav.ts:24-38`
> 的 `isArrowKeyEditableTarget`）**
> —— P7-dc 收尾后「尚未做」只剩第 2 条（具体可做）与第 3 条（卡 mygo 框架），本轮取第 2 条落地。
> 依赖检查：GitHub releases API 仍被限流，用 `go list -m -versions github.com/egoist/mygo`
> 核对，停在 **v0.4.8**（与 `go.mod` 一致），无版本可升。只对照 `main`
> （`backup/main-pre-mygo` = v0.3.14 原版树），未翻历史提交。
>
> **偏离点**：原版 `isArrowKeyEditableTarget`（`src/settings-nav.ts:24-38`）把**单行** `INPUT`
> 与 `TEXTAREA`/`SELECT`/contenteditable 一视同仁——焦点在文本框里时 ↑/↓ 不切页。本构建的
> mygo 编辑器只对**多行**字段认领 ↑/↓（`ui/editor_navigation.go:46-47`：
> `return ed.multiline || m != 0`），单行 `ui.TextInput` 主动放开它们；而设置面的窗口快捷键
> （`settingsui.go:662-664`）接住 ↑/↓，于是焦点在 npm 包名、本地工具名/程序/命令、驻留自定义
> 秒数、扩展配置文本这类单行字段里按 ↑/↓ 会切页。
>
> **为何不是 `Element.HandleInput`**：框架的 `keyDownPhysical`（`ui/input.go:578`）对聚焦元素的
> `HandleInput` 有一道 `!rt.claimed(k)` 守卫，而 `claimed` 会把**窗口快捷键**算进去
> （`ui/input.go:526`）——设置面自己注册了 `c.Shortcut(0, ui.KeyDown/KeyUp)`，所以这道缝对
> 纯 ↑/↓ **不可达**（P7-v 的字段删除键是带修饰键的，窗口没注册，才够得到）。可用的缝是
> **元素级快捷键**：`routeKeys`（`ui/input.go:730`）把一次按键交给焦点链上**最内层**认领它的
> 元素，聚焦字段自己 `e.Shortcut(0, ui.KeyDown/KeyUp)` 即先于窗口接住。
>
> **修法**：新文件 `internal/settingsui/textinput.go` 的 `textInput(c, &value)` = `ui.TextInput`
> + 两条元素级快捷键（只认 mods==0 的 ↑/↓，字段未聚焦时快捷键不在焦点链上，窗口照旧走页）。
> `config.go` / `custom.go` / `settingsui.go` 里 18 处 `ui.TextInput(c, …)` 全部改走它。
> **顺带**：启动器的插件配置浮层走的是同一个 `settingsui` 助手
> （`settingsui.go:2342` 的 `a.text` ← `pluginsheet.go:109` 的 `sheetField`），所以浮层里的
> 单行字段也一并拿回了自己的箭头；启动器的**搜索字段**不在 `settingsui`，不受影响。
>
> **测试**：`internal/settingsui/view_test.go` 的 `TestSingleLineFieldsKeepTheirArrows`——进
> Integrations 页、点进 npm 包名字段、按 ↑/↓ 后页号不变且字段仍获焦、`SetPage` 无新记录；
> 再把焦点交回侧栏按 ↓，仍按 P7-dc 的规则走页（证明认领只属于字段自己的焦点）。
> 变异核查：删掉 `textinput.go` 的两行 `e.Shortcut(…)`，
> `↓ in a text field walked to page 4` / `moved the keyboard off it` 立刻红。
>
> **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 31 包全绿、
> `GOOS=linux/windows/darwin go build ./...` 三平台通过。

> **上一轮：P7-dc 设置侧栏 ↑/↓ 从任意处走页、环绕、落盘（原版 `src/settings-nav.ts`、
> `src/hooks/useAppKeyboard.ts:288-303`）**
> —— P7-db 收尾后「尚未做」只剩第 3 条（卡 mygo 框架），本轮先把它重写为 3 条带 file:line 的具体
> 待办（下方「具体待办（P7-dc 重写）」），再取第 1 条落地。依赖检查：GitHub releases API 被限流，
> 用 `proxy.golang.org` 的 `@v/list` 核对，最新仍是 **v0.4.8**（与 `go.mod` 一致），无版本可升。
> 只对照 `main`（`backup/main-pre-mygo` = v0.3.14 原版树），未翻历史提交。
>
> **偏离点**：原版在 `mode === "settings"` 的窗口级处理器里，只要焦点不在可编辑控件上，就
> `nextSettingsPage` 环绕走页、把焦点移到新页的侧栏按钮、`changeSettingsPage` 落盘
> （`useAppKeyboard.ts:288-303`）。本构建的侧栏是个 `ui.List`，`New` 里
> `a.Sidebar.Selected = &a.Page`（`settingsui.go:571`）把走页交给框架的列表导航
> （`ui/list.go:882` 的 `navigate`），于是：**只有侧栏自己持有焦点时** ↑/↓ 才走页
> （`Open(SurfaceSettings)` 会 `FocusSidebar`）、**不环绕**（首页 ↑ 不动）、**不落盘**
> （`navigate` 走 `choose`，不经过 `selectPage` 的 `SetPage`）；焦点一移到页内控件，↑/↓ 完全无效。
>
> **修法**：`internal/settingsui/settingsui.go`
> - `New`（`:569`）不再给侧栏 `Selected`：侧栏退成纯点击列表，框架的列表导航不再认领 ↑/↓
>   （`ListState.cursor()` 为 nil 时 `navigate` 不会被调用）。
> - `View`（`:652-664`）注册窗口级 `c.Shortcut(0, KeyDown/KeyUp)`，记录器等待键时让出；
>   `stepPage`/`nextPage`（`:868-877`）按侧栏顺序环绕走页，`stepPage` 复用 `selectPage`
>   （落盘 + `SetPage`）并把焦点交回侧栏（原版的 `settingsSidebarButtons.get(next)?.focus()`）。
> - 让出规则由框架保证：编辑器的 `wants` 与控件自己的元素级快捷键都在窗口快捷键之前，
>   多行编辑器与 `Select`/`Segmented` 因此保留自己的箭头。
>
> **测试**：`internal/settingsui/view_test.go` 的 `TestSidebarArrowsWalkThePagesFromAnywhere`
> （焦点在侧栏时 ↓ 走页；无控件获焦时同样走页；首页 ↑ 环绕到 About、末页 ↓ 环绕回 General；
> 每一步都 `SetPage` 记录）与 `TestNextPageWraps`（六行环绕表）。
> 变异核查：恢复 `a.Sidebar.Selected` → 「首页 ↑ 不环绕」红；把窗口快捷键短路 → 「焦点在侧栏时 ↓
> 不走页」红。
>
> **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 全绿（31 包）、
> `GOOS=linux/windows/darwin go build ./...` 三平台通过。
>
> **已知未做（已写成下方第 2 条）**：原版的 `isArrowKeyEditableTarget` 把**单行** `INPUT` 也
> 算作可编辑目标，mygo 的 `editor.wants`（`editor_navigation.go:46-47`）对单行编辑器主动放开
> ↑/↓，所以焦点在 npm 包名这类单行文本框里时 ↑/↓ 仍会走页。多行 `ui.TextArea` 不受影响。

> **上一轮：P7-db Hyprland 窗口整形对齐原版（原版 `src-tauri/src/hyprland.rs`、
> `src-tauri/src/lib.rs:1109-1127`）**
> —— P7-da 之后「尚未做」又一次为空，本轮按上节建议换入口做「逐文件对照原版」的审计（与
> P7-ci/P7-ck/P7-ct/P7-cw/P7-cz/P7-da 同一手法）：拿 `internal/settings` + `internal/shell` 的
> 窗口整形对原版 `hyprland.rs`。依赖检查：GitHub releases API 本轮仍被限流，用
> `go list -m -versions github.com/egoist/mygo` 代替，停在 **v0.4.8**（与 `go.mod` 一致），
> 无版本可升。只对照 `main`（`backup/main-pre-mygo` = v0.3.14 原版树），未翻历史提交。
>
> **偏离点（两处可做 + 一处卡框架）**：
> 1. **空白的 `HYPRLAND_INSTANCE_SIGNATURE` 被当成 Hyprland**：原版
>    `hyprland_signature_detected`（`hyprland.rs:48-50`）先 `trim()` 再判空，只含空白的导出
>    不是会话；本构建 `internal/settings/settings.go:586-591` 的 `DefaultHideOnBlur` 用
>    `os.Getenv(...) == ""`，于是 `"   "` 会错误地关掉 `hide_on_blur`。
> 2. **`ensure_floating` 完全没有移植**：原版 `FLOATING_DISPATCHES`（`hyprland.rs:41-43`）与
>    `ensure_floating`（`:77-88`）在 `reveal_window`（`lib.rs:1109-1127`）的 `show()`+`set_focus()`
>    之后、后续几何调用之前逐条跑 `hyprctl dispatch setfloating` / `centerwindow`；本构建仓库里
>    `grep -rn hyprctl internal cmd` 为空，Hyprland 下新 toplevel 被平铺铺满，`placeWindow` 设的
>    尺寸被合成器覆盖。
> 3. **托盘身份是固定字面量**（卡框架）：原版 `tray_identity.rs:28-30` 用
>    `com.v.floter.tray`（bundle id + `.tray`）避免两个 appindicator 应用撞 SNI 对象路径；本构建
>    `mygo.NewTray` 的 `TrayOptions`（`mygo@v0.4.8`）没有 id 字段，改不了。规格写进待办第 3 条。
>
> **修法**：`internal/settings/settings.go` 新增 `HyprlandSignatureEnv` / `HyprlandDetected`
> （`strings.TrimSpace(...) != ""`）/ `HyprlandSession`，`DefaultHideOnBlur` 改走它；
> 新增 `internal/shell/hyprland.go`——`hyprlandFloatingDispatches`（setfloating 后 centerwindow，
> 不用 togglefloating）、`runHyprlandFloating(detected, run)`（`run` 注入）与
> `(*App).ensureHyprlandFloating`（Linux-only，best-effort）；`Show()` 在 `Focus()` 后调用它，
> 命中时再 `placeWindow()` 一次，让面板尺寸落在浮动窗口上。
>
> **测试**：`internal/settings/settings_test.go` 的
> `TestHyprlandSignatureBlanksAreNotASession`（`"abc123"`/`" abc123 "` → true，`""`/`"   "`/
> `"\t\n"` → false；`t.Setenv` 钉住 `DefaultHideOnBlur` 的两种取值）；
> `internal/shell/hyprland_test.go` 的 `TestHyprlandFloatingIsASetNotAToggle`（顺序 + 不含
> `togglefloating`）、`TestHyprlandFloatingSkipsOtherSessions`、
> `TestHyprlandFloatingIgnoresAFailedDispatch`（首条失败仍跑第二条）。变异核查：把
> `HyprlandDetected` 换回 `== ""`、把 `FLOATING_DISPATCHES` 换成 `togglefloating`，新用例各自
> 立刻变红。
> 门槛：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 全绿、
> `GOOS=linux/windows/darwin go build ./...` 三平台通过。

> **上一轮：P7-da 计算器历史的裁剪与归一化对齐原版（原版 `src-tauri/src/calculator_history.rs`、
> `src-tauri/src/commands/config.rs`）**
> —— P7-cz 之后清单再次为空，本轮按上节建议换入口做「逐文件对照原版」的审计（与
> P7-ci/P7-ck/P7-ct/P7-cw/P7-cz 同一手法）：拿 `internal/calculator` + `internal/settings` 的
> 计算器块对原版 Rust。依赖检查：GitHub releases API 本轮被限流（`curl` 返回 rate limit），
> 用 `go list -m -versions github.com/egoist/mygo` 代替，仍停在 **v0.4.8**（与 `go.mod` 一致），
> 无版本可升。只对照 `main`（`backup/main-pre-mygo` = v0.3.14 原版树），未翻历史提交。
>
> **偏离点（四处）**：
> 1. **容量的裁剪按「位置」而不是按「年龄」**：原版 `prune_entries`（`calculator_history.rs:127-167`）
>    先把非收藏项的下标按 `created_at` 升序排，再丢掉最老的 `excess` 个，注释明写
>    「the cap is applied by age, not by position, so a hand-edited out-of-order index still prunes
>    the truly oldest」；本构建 `internal/calculator/calculator.go:351-379` 的 `pruneLocked` 走列表
>    位置、留下前 `maxItems` 个非收藏项。两者只在「手工改乱顺序的索引」上分叉，但那正是原版
>    这条规则存在的理由。
> 2. **`Delete` 不裁剪**：原版每条写路径（新增 / 取消收藏 / 删除 / 改设置）都走同一个 `update`
>    （`calculator_history.rs:169-186`），在同一临界区里裁剪后落盘；本构建的 `Delete`
>    （`calculator.go:299-318`）直接存盘，漏了「窗口过期后没有别的写入、一次删除也要把过期的行
>    一并带走」这条。
> 3. **`copy_mode` 归一化没有 trim/lowercase**：原版 `normalize_calculator_copy_mode`
>    （`commands/config.rs:457-463`）先 `value.trim().to_lowercase()` 再比对；本构建
>    `internal/settings/calculator.go` 直接 `mode == "result"`，手改文件里的 `" Result "` 落回 `full`。
> 4. **`max_items` 的 `0` 落到默认而不是下界**：原版 `commands/config.rs:1211-1214` 是
>    `max_items.clamp(MIN, MAX)`，`0` → `MIN`(10)；本构建把 `<=0` 当「未设」回默认 100
>    （`internal/settings/calculator.go:96-107` 与 `internal/calculator/calculator.go:471-483` 两处）。
>
> **修法**：`pruneLocked` 收成原版形状（非收藏项下标 + `sort.SliceStable` by `CreatedAt` +
> `indices[:excess]` 丢最老）；`Delete` 补一次 `pruneLocked()`；`NormalizeCalculatorCopyMode`
> （settings 与 calculator 两份）都先 `strings.ToLower(strings.TrimSpace(...))`；
> `NormalizeCalculatorMaxItems` 与 `clampMaxItems` 改成对 `[MIN, MAX]` 的夹取（`0` → 10）。
>
> **测试**：`internal/calculator/calculator_test.go` 新增
> `TestTheCapacityDropsTheOldestByAgeNotByPosition`（12 行、最老的两行排在**前面**：按年龄裁剪
> 丢它们、留下的行保持原顺序）与 `TestADeleteAlsoPrunes`（窗口过期后没有别的写入，一次删除也要
> 把过期的行带走）；`TestNormalizers` 与 `internal/settings/calculator_test.go` 的
> `TestCalculatorNormalizers` / `TestCalculatorPluginDefaultsAndRoundTrip` 改成钉新规则
> （`0` → 下界、`" Result "` → `result`）。变异核查：把 `pruneLocked` 换回按位置、把 `Delete` 的
> `pruneLocked()` 去掉，两个新用例各自立刻变红。
> 门槛：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 全绿（**1062** 项
> `--- PASS`，含子测试）、`GOOS=linux/windows/darwin go build ./...` 三平台通过。
>
> **上一轮：P7-cz 拖放路径的解析对齐原版（原版 `src-tauri/src/commands/drops.rs:55-105`）**
> —— P7-cy 之后 P7-cw 清单三条全部落地、清单再次为空，本轮按上节建议换入口做「逐文件对照原版」的
> 审计（与 P7-ci/P7-ck/P7-ct/P7-cw 同一手法）：拿 `internal/drops` 对原版 Rust 的
> `src-tauri/src/commands/drops.rs`。依赖检查：`go list -m -versions github.com/egoist/mygo` 仍停在
> **v0.4.8**（与 `go.mod` 一致；GitHub releases API 本轮被限流，用 `go list` 代替），无版本可升。
> 只对照 `main`（`backup/main-pre-mygo` = v0.3.14 原版树），未翻历史提交。
>
> **偏离点（三处）**：
> 1. **trim 被当成了修正**：原版 `normalize_dropped_path` 只把 `path.trim()` 用在空串判断上
>    （`drops.rs:55-58`），随后把**原始** `path` 交给 `resolve`。本构建过去
>    `path = strings.TrimSpace(path)` 之后再解析，于是 `" /tmp/a "` 被悄悄修成 `/tmp/a`
>    并当成一个存在的文件——一个用户没拖的东西变成了行。
> 2. **存在性用了 `Stat` 而不是 `symlink_metadata`**：原版 `:60-65` 用
>    `std::fs::symlink_metadata`（不跟链），悬空符号链接仍是一份可以打开/`cd`/复制的掉落；本构建用
>    `os.Stat`，链断了整行就消失。目录判定是**反面**的问题、走 `resolved.is_dir()`（`:82`，跟链），
>    所以悬空链接应是「留着，但不是目录」。
> 3. **`~alice` 被当相对路径**：原版 `resolve`（`:93-105`）对**任何** `~` 开头都进分支，只有 `~`
>    或 `~` + 分隔符才是 home；`~alice` 是 `home.join("~alice")`（字面名字的目录，不是别人的
>    home）。本构建只认 `~`/`~/`，`~alice` 落到 `cwd/~alice`。
>
> **修法**：`internal/drops/drops.go` 的 `normalize`——空判改为 `strings.TrimSpace(path) == ""`
> 但解析用原始 `path`；存在性改 `os.Lstat`，目录判定单独 `os.Stat`（失败即 `IsDirectory=false`）；
> `~` 分支改为 `strings.HasPrefix(path, "~")` + `os.IsPathSeparator(rest[0])`，字面 `~alice` 走
> `filepath.Join(home, path)`，其余走 `filepath.Join(home, rest)`。
>
> **测试**：`internal/drops/drops_test.go` 新增
> `TestNormalizeKeepsADanglingSymlinkButNotAsADirectory`（悬空链保留且非目录 + 指向目录的链是目录）、
> `TestNormalizeDoesNotTrimThePathItResolves`（带空白的路径被丢弃、去空白后仍在）、
> `TestNormalizeTreatsALiteralTildeNameAsAFileUnderHome`（`home/~alice` 保留、`cwd/~bob` 丢弃）。
> 变异核查：把 `os.Lstat` 换回 `os.Stat`、把 `strings.TrimSpace(path) == ""` 换回
> `path = strings.TrimSpace(path)`、把 `~` 分支换回只认 `~`/`~/`，对应用例各自立刻变红。
> 门槛：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 全绿（**1060** 项
> `--- PASS`，含子测试）、`GOOS=linux/windows/darwin go build ./...` 三平台通过。
>
> **上一轮：P7-cy 应用按键录制的冲突拒绝（原版 `src/hooks/useShortcutCapture.ts:112-136`、
> `src/settings/ShortcutsPage.tsx:382,399-407`）**
> —— P7-cw 重写的清单第 1、2 条已由 P7-cw / P7-cx 落地，本轮取第 3 条（最后一条，也是清单里唯一
> 剩余项），至此 P7-cw 三条**全部落地**，清单再次为空（余下只有三条卡 mygo 框架项，规格见
> 「具体待办（P7-cm 重写）」与「已知差距」）。依赖检查：`go list -m -versions github.com/egoist/mygo`
> 仍停在 **v0.4.8**（与 `go.mod` 一致；GitHub releases API 本轮被限流，用 `go list` 代替），无版本可升。
> 只对照 `main`（`backup/main-pre-mygo` = v0.3.14 原版树），未翻历史提交。
>
> **偏离点**：原版录制一个应用按键时先归一（`select_result` 走 `normalizeResultShortcut`）、再扫
> `SHORTCUT_ACTIONS` 看**别的动作**是否已用同一组合（`:127-129` 的小写比较），冲突就 `setRejectedAction`
> 并整条放弃；`next === previous`（正是该动作自己现有的组合）则当作无变化、连写都不写。设置页把被拒
> 状态画两处：页顶一条 `settings-save-alert`（`:382`，非 toast 那条）与**该行**的 warning 副标题
> （`:399-407`），文案同是 `settings.shortcut.rejected`（"Already used by another app" /
> "该组合键已被其他应用占用"）；开始下一次录制（`toggle`）或重置成功时清空。本构建
> `internal/settingsui/settingsui.go:1795-1815` 的 `shortcutControl` 一录到键就直接 `Actions.SetShortcut`，
> `internal/settings/shortcutmap.go` 的 `SetShortcut` 也不看别的动作——于是两个动作可以绑同一个组合
> （自定义快捷键那条路有 `shortcuts.Duplicate`，应用按键这条没有），页面上也不会说一句话。
>
> **修法**：`internal/settings/shortcutmap.go` 抽出 `normalizeShortcut`（`SetShortcut` 共用，行为不变），
> 新增 `ShortcutRecording`（`ShortcutRecorded` / `ShortcutUnchanged` / `ShortcutRefused`）与
> `ShortcutRecordingOf(s, action, accelerator)`——先把组合收成**会落库的绑定**（家族头在内），再拿
> `shortcuts.Equal` 比该动作自己的现值（同值=无变化）与其余每个动作（冲突=拒绝），正是原版 `capture` 的
> 顺序。`internal/settingsui/settingsui.go` 新增 `recordShortcut`（只有 `ShortcutRecorded` 才转给
> `Actions.SetShortcut`，`ShortcutRefused` 记进新字段 `rejectedShortcut`）与 `shortcutWarning`；
> `shortcutMapRow` / `shortcutRow` 改用 `rowNote` 把拒收句画在该行名下；`shortcuts` 页顶新增
> `shortcutRejectedAlert`（`internal/settingsui/savealert.go`，原版那条**非 toast** 的
> `.settings-save-alert`：控制材质 + 边框 + warning 文字）；点 Record 开始录制时清空 `rejectedShortcut`。
> i18n 新增 `ShortcutRejected`（`internal/i18n/i18n.go` + `copy.go` 的 en/zh），文案逐字取原版的
> `settings.shortcut.rejected`。
>
> **一处与上轮留言的差异**：上轮把验收写成「复用现有 `ShortcutsRejected`」——那是**自定义快捷键**那条
> 路用的参数化句子（"Could not bind X: Y"），而原版这一屏是**另一句**不带参数的
> `settings.shortcut.rejected`，所以新增了字段而不是硬把加速键塞进那句话。两条路各自保持原样。
> 另：原版查冲突只比 `shortcuts[select_result]`（家族头）一个字符串，所以家族的**其余数字**（如
> `Cmd+7`）不算别人占用——本构建逐字保持这个盲区（测试里明钉 `Cmd+Shift+5` 仍是「可录」），没有
> 自作主张加严。
>
> **测试**：`internal/settings/shortcutmap_test.go` 新增
> `TestShortcutRecordingRefusesAChordAnotherActionHolds`（6 行表：别的动作持有的组合 / 同组合换大小写 /
> 该动作自己的组合=无变化 / 没人占的组合=可录 / 坏拼写 / 未知动作，外加家族头三条：别的数字=无变化、
> 家族头被别人抢=拒绝、头之外的数字=可录）；`internal/settingsui/view_test.go` 新增
> `TestShortcutRecorderRefusesAChordAnotherActionHolds`（录 `Cmd+V`（paste 持有）→ `SetShortcut` 一次都
> 没被调、页顶 + 行内共 **2** 处 warning、store 未变；录 `Ctrl+Space`（该行自己的值）→ 不写不报；录
> `Cmd+Shift+K` → 写入一次且 warning 归零）。变异核查：把 `ShortcutRecordingOf` 的冲突循环删掉 →
> settings 三条断言红；把 `recordShortcut` 的 `ShortcutRefused` 分支改成照写不误 → settingsui 五条全红。
> 门槛：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 全绿（**1057** 项
> `--- PASS`，含子测试）、`GOOS=linux/windows/darwin go build ./...` 三平台通过。
>
> **上一轮：P7-cx 快捷键显示与角标的拼写、平台名与键符号（原版 `src/shortcuts.ts:66-100,278-310`）**
> —— P7-cw 重写的清单第 1 条已落地，本轮取第 2 条（第 3 条「录制拒绝冲突」留单独一轮）。依赖检查：
> `curl .../releases/latest` 与 `go list -m -versions github.com/egoist/mygo` 均为 **v0.4.8**
> （与 `go.mod` 一致），无版本可升。只对照 `main`（`backup/main-pre-mygo` = v0.3.14 原版树），未翻历史提交。
>
> **偏离点**：原版 `formatShortcut`（`shortcuts.ts:278-296`）非 macOS 用 `parts.join("+")`
> （`Ctrl+Shift+C`，**无空格**），meta 键在 Linux 显示 `Super`、在 Windows 显示 `Win`（`:299`
> 的 `IS_LINUX ? "Super" : "Win"`），键名经 `KEY_SYMBOLS`（`:82-100`）把 `Backspace` 画成 `⌫`、
> 四个方向键画成 `↑↓←→`；`formatResultShortcut`（`:299-310`）的角标复用同一套。本构建
> `internal/shortcuts/shortcuts.go` 的 `Display` 非 macOS 写 `"Ctrl + "` / `"Super + "`（带空格，
> 且 Windows 也是 `Super`），`displayKey` 只大写单字符、不查符号表，于是 macOS 上 `Cmd+Up` 显示
> `⌘Up`、`Cmd+Backspace` 显示 `⌘Backspace`；`Badge` 同病。
>
> **修法**：`internal/shortcuts/shortcuts.go` 的 `Display`/`Badge` 拆出 `displayFor(goos,…)` /
> `badgeFor(goos,…)`（平台显式传入，三张表任何宿主都能钉），非 macOS 用 `strings.Join(words,"+")`
> 无空格；新增 `metaKeyWord(goos)`（Linux→`Super`，其余→`Win`）与 `keySymbols` 表
> （`Backspace`→`⌫`、`Up/Down/Left/Right`→`↑↓←→`，标点在词表里本就是自身符号），`displayKey` 先查
> 符号表再大写单字符；`Badge` 的键也过 `displayKey`。
>
> **测试**：`internal/shortcuts/shortcuts_test.go` 的 `TestDisplay` 改为 14 行表（三平台 + 标点 +
> 单字符 + 空格键），`TestBadge` 改为 7 行表（Windows `Super+1`→`Win+1`、Linux→`Super+1`、macOS
> 箭头/删除符号、Enter→↩）；`internal/settingsui/view_test.go` 三处硬写的 `"Alt + Space"` /
> `"Ctrl + W"` 改成 `shortcuts.Display(...)`，钉设置页跟随新拼写。变异核查：把
> `strings.Join(words, "+")` 换回 `" + "`、把 `metaKeyWord` 写死 `Super`、把 `keySymbols` 删掉，
> 对应用例立刻红。
> 门槛：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 全绿、
> `GOOS=linux/windows/darwin go build ./...` 三平台通过。
>
> **上一轮：P7-cw `select_result` 家族头对齐（原版 `src/shortcuts.ts:248-255,317-326`、`src/hooks/useShortcutCapture.ts:120-128`）**
> —— P7-cv 收尾后「尚未做」又一次为空（P7-ct 三条全部落地，余下只有三条卡 mygo 框架项）。本轮换
> 入口：逐文件对照原版的**快捷键**（`src/shortcuts.ts`）与设置里的快捷键地图，把清单重写成 3 条带
> file:line 的具体待办（下方「具体待办（P7-cw 重写）」），再取第 1 条落地。依赖检查：
> `go list -m -versions github.com/egoist/mygo` 仍停在 **v0.4.8**（与 `go.mod` 一致），无版本可升。
> 只对照 `main`（`backup/main-pre-mygo` = v0.3.14 原版树），未翻历史提交。
>
> **偏离点**：原版把 `select_result` 当**家族**——`normalizeResultShortcut`（`shortcuts.ts:248-255`）
> 解析后要求 `hasModifier`（Ctrl/Alt/Shift/Meta 至少一个），把键换成 `1`，无修饰键返回 `null`；
> `withShortcutDefaults`（`:317-326`）读设置时对 `select_result` 走它、失败回默认；录制路径的
> `capture`（`useShortcutCapture.ts:120-128`）写入前也走它、失败即拒。本构建
> `internal/settings/shortcutmap.go:82-98` 的 `ShortcutsOf` 对 `select_result` 只调
> `shortcuts.Normalize`，于是 `"Alt+Shift+7"` 原样落库（家族头应是 `Alt+Shift+1`），而 `"7"`
> （无修饰键）也落库——`SelectResultDigit`（`:145-154`）在无 `+` 时回裸数字，**裸数字键**于是能
> 触发结果行，正是原版明写要拒收的情形。
>
> **修法**：`internal/shortcuts/shortcuts.go` 新增 `NormalizeResultShortcut`（解析 → 无修饰键
> 即拒 → `join(modifierList(mods), "1")`）与 `modifierList`（`FromKey` 共用，行为不变）；
> `internal/settings/shortcutmap.go` 的 `ShortcutsOf` 对 `select_result` 走它、失败保留出厂默认，
> `SetShortcut` 同样收头并拒收无修饰键。
>
> **测试**：`internal/shortcuts/shortcuts_test.go` 的 `TestNormalizeResultShortcut`
> （`Cmd+7`→`Cmd+1`、`Ctrl+Shift+0`→`Ctrl+Shift+1`、`Alt+Shift+9`→`Alt+Shift+1`、
> `Cmd+Delete`→`Cmd+1`、`Shift+3`→`Shift+1`，以及 `7`/`A`/`not a key`/空 被拒）；
> `internal/settings/shortcutmap_test.go` 的 `TestSelectResultStoredBindingIsReducedToItsHead`
> （读 `Alt+Shift+7`→`Alt+Shift+1`、`SelectResultDigit(s,4)`→`Alt+Shift+4`；读 `7`→默认且第三
> 位仍带默认修饰键；`SetShortcut("Cmd+Shift+5")`→`Cmd+Shift+1`、`SetShortcut("5")` 被拒且不改值）。
> 变异核查：把 `ShortcutsOf` 的 `select_result` 分支去掉（回到只 `Normalize`），
> `TestSelectResultStoredBindingIsReducedToItsHead` 两条读断言立刻红；把 `SetShortcut` 的分支
> 去掉，写断言红。
> 门槛：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 全绿、
> `GOOS=linux/windows/darwin go build ./...` 三平台通过。
>
> **上一轮：P7-cv 计算器的函数、常量与阶乘（原版 `src/calculator.ts:100-200`）**
> —— P7-ct 重写的清单第 1、2 条已由 P7-ct / P7-cu 落地，本轮取第 3 条（最后一条，也是清单里唯一
> 可落地项）：原版 `evaluateExpression` 交给 `expr-eval-fork` 的加固 parser，**保留**它的内置函数与
> 常量，本构建的递归下降 parser 只认数字与四则/幂，于是 `sqrt(4)`、`PI` 在启动器里根本不是计算器行。
> 依赖检查：`go list -m -versions github.com/egoist/mygo` 与 GitHub releases latest 均为 **v0.4.8**
> （与 `go.mod` 一致），无版本可升。只对照 `main`（`backup/main-pre-mygo` = v0.3.14 原版树），未翻历史提交。
>
> **偏离点**：原版 parser 的默认表（`expr-eval-fork@3.0.3` 的 `unaryOps`/`functions`/`consts`）是
> 常量 `PI`/`E`、一元函数 `sin cos tan asin acos atan sinh cosh tanh asinh acosh atanh sqrt cbrt
> log log2 ln lg log10 expm1 log1p abs ceil floor round trunc exp sign`、具名函数 `random`（原版
> `delete` 掉）、`fac gamma roundTo pow atan2 hypot pyt min max if map fold filter indexOf join sum`，
> 外加后缀 `!`（阶乘）。本构建 `internal/calc/calc.go` 的 `hasArithmetic` 见到除 `e`/`E` 以外的字母
> 就返回 false，`Eval` 的 parser 也只认数字、`+ - * / % ^`、括号与一元符号。于是 `sqrt(4)`、`sin(PI/2)`、
> `5!`、`PI` 全不是计算器行。
>
> **修法**：`internal/calc/calc.go` 新增 `calcConstants`（`PI`/`E`）与 `calcFunctions` 表（与 fork 默认集
> 逐名对齐：一元 28 个 + 具名 `fac gamma roundTo pow atan2 hypot pyt min max`；`random` 与原版一样不收，
> 数组/条件助手 `if map fold filter indexOf join sum` 因本语言没有数组与布尔字面量而无法喂）。
> `hasArithmetic` 重写成按标识符扫描：`sqrt(4)`（函数调用）与 `PI`（常量）算 intent，未知名字仍是搜索，
> `1e3` 的 `e` 仍算指数；parser 增 `parseName`/`parseArgs`（常量、函数调用、逗号参数、arity 检查）、
> `parsePostfix`（后缀 `!`，比 `^` 与一元符号更紧，可叠 `3!!`）与 `parseUnary` 的前缀 `!`。数值语义
> 逐条对齐：`round`/`roundTo` 用 JS 的 `Math.round`（半值朝 +∞，`round(-2.5) = -2`）、`roundTo` 走
> 十进制字符串移位（`roundTo(2.345,2) = 2.35`，不是浮点乘法给的 2.34）、`fac(x) = gamma(x+1)`、
> `sign` 保留零的符号、`hypot`/`pyt`/`min`/`max` 变参折叠。
>
> **测试**：`internal/calc/calc_test.go` 新增 `TestEvalBuiltins`（44 行表：每个函数一例、`PI`/`E` 取值、
> `5!`/`!5`/`3!!`/`2^3!`/`-3!` 的优先级与叠用）、`TestEvalFunctionTableMatchesTheFork`（按名字钉住函数表
> 全集：不得少一个、不得多一个，`random`/`map`/`sum` 等明钉不在表里）、`TestEvalRejectsUnknownNames`
> （`foo(4)`/`sign` 仍是搜索，`sqrt(4`/`sqrt()`/`pow(2)` 是解析错）；
> `internal/launcher/search_test.go` 新增 `TestCalculatorRowAcceptsBuiltinFunctions`（`sqrt(16)`/`sin(PI/2)`/
> `5!`/`min(2,3)` 在启动器里出现答案行且尾值是 `4`/`1`/`120`/`2`；裸 `sign` 不出行）。
> 变异核查：把 `hasArithmetic` 的名字分支改回「见字母即 false」→ 启动器用例四条红；从表里删掉 `min`
> → 全集用例红；`round` 换回 `math.Round` → `round(-2.5)` 红；`roundTo` 换回浮点乘法 → `2.345` 用例红。
> 至此 P7-ct 清单三条**全部落地**，清单再次为空；余下只有三条卡 mygo 框架项（复制反馈 /
> 键入 Unicode 空格缝 / 外观四个死旋钮），规格见「具体待办（P7-cm 重写）」。
> 门槛：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 全绿、
> `GOOS=linux/windows/darwin go build ./...` 三平台通过。
>
> **上一轮：P7-cu 触发词与首词的空白切分用 JS `\s`（原版 `src/plugins/mode-entry.ts:44-52`、`src/plugins/external.ts:81-96`）**
> —— P7-ct 清单第 1 条已落地，本轮取第 2 条（第 3 条要扩求值器，留单独一轮）。依赖检查：
> `curl .../releases/latest` 与 `go list -m -versions github.com/egoist/mygo` 均停在 **v0.4.8**
> （与 `go.mod` 一致），无版本可升。只对照 `main`（`backup/main-pre-mygo` = v0.3.14 原版树），未翻历史提交。
>
> **偏离点**：原版的触发词切分用的是 JavaScript 正则——`splitTriggerWord` 的
> `/^(\S+)\s+([\s\S]*)$/`（`mode-entry.ts:44`）与 `pluginCommandTriggers` 的
> `source.trim().split(/\s+/)[0]`（`external.ts:81`）——`\s` 是 JS 的全空白集（25 个码点：六个
> ASCII 空白 + Zs 的 U+00A0/U+1680/U+2000–200A/U+202F/U+205F/U+3000 + 行分隔 U+2028/U+2029 +
> BOM U+FEFF，**不含** U+0085 与 U+180E）。本构建 `internal/launcher/launcher.go:1292` 的
> `modeWord` 只 `strings.IndexAny(query, " \t")`，`internal/launcher/argv.go` 的 `splitArgs` 也只把
> `' '`/`'\t'` 当词缝，于是中文输入法打出的全角空格 U+3000 既不触发模式，也不切分首词——而原版
> 里 `clip\u3000foo` 与 `clip foo` 是同一件事。
>
> **修法**：新文件 `internal/launcher/wordgap.go` 把 JS `\s` 写成一处：常量 `jsWhitespace`（25 个码点）、
> `isWordGap`（快路径 switch）、`indexWordGap`/`lastWordGapEnd`/`trimTrailingWordGaps`。
> `splitArgs` 改成按 rune 解码（多字节词缝不再被按字节切开）；`modeWord` 走 `indexWordGap`；
> `replaceLastWord` 走 `lastWordGapEnd`（保留整颗词缝 rune，`jv\u3000-f` → `jv\u3000file`）；
> `commandTokens`/`currentWord`/`appendWord` 的「行尾是不是词缝」统一走 `trimTrailingWordGaps`。
> `firstWord`/`commandTriggers` 经 `splitArgs` 自动跟上。
>
> **一处与原版记录的差异（本轮不改）**：原版 `parseClipboardMode("clip foo")` 的 needle 是 `foo`
> （rest 留在字段里）；本构建的进入设计是「触发词被消耗、字段清空，needle 是用户接着打的字段文本」
> （`internal/launcher/launcher.go:1242` 的注释与 `TestCalculatorModeWords` 都钉着它），所以
> `clip\u3000foo` 进入模式后字段为空。本轮只对齐**词缝字符集**（进入/切分/首词），不动进入设计；
> 命令模式仍保留 rest 为 argv（`jv\u3000-f` → argv `-f`）。若日后要对齐原版的「rest 即 needle」，
> 那是独立一轮。
>
> **测试**：新文件 `internal/launcher/wordgap_test.go`——
> `TestWordGapsAreJavaScriptsWhitespace`（独立写出的 25 码点字面量 + 全码点范围逐一比对 +
> 明钉 U+0085/U+180E/U+200B 不是词缝）、`TestWordGapsSeparateACommandLine`（NBSP/U+3000/U+2028/
> U+FEFF/换行各一行表 + 引号内的词缝 + 每个 token 是合法 UTF-8）、
> `TestEveryWordGapEntersTheMode`（七种词缝各进一次剪贴板模式）、
> `TestAFullWidthSpaceEntersACommandMode`（`jv\u3000-f` 进命令模式、argv `-f`）、
> `TestWordGapsReduceATriggerToItsFirstWord`（多词名用 U+3000 连接仍取首词）、
> `TestTrailingWordGapsCount`（`replaceLastWord` 保留整颗多字节词缝 + 行尾词缝算新词）。
> 变异核查：把 `modeWord` 换回 `strings.IndexAny(query, " \t")`、`splitArgs` 换回只认 `' '`/`'\t'`，
> 后五条立刻红（`splitArgs` 把整行当一个 token、五个词缝全不进模式、`commandTriggers` 带出整串）。
> 门槛：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 全绿、
> `GOOS=linux/windows/darwin go build ./...` 三平台通过。
>
> **上一轮：P7-ct 计算器结果的打印策略对齐原版（原版 `src/calculator.ts:78-96`）**
> —— P7-cs 收尾后「尚未做」又一次为空，本轮先把清单重写成 3 条带 file:line 的具体待办
> （下方「具体待办（P7-ct 重写）」），再取第 1 条落地。依赖检查：
> `go list -m -versions github.com/egoist/mygo` 仍停在 **v0.4.8**（与 `go.mod` 一致），无版本可升。
> 只对照 `main`（`backup/main-pre-mygo` = v0.3.14 原版树），未翻历史提交。
>
> **偏离点**：原版 `formatCalculatorResult` 的策略是「12 位有效数字 + JS 自己的数字打印」：
> 非有限值 → 空串；`|v| >= 1e15` 或 `< 1e-9` → `toExponential(11)` 再去尾零；否则
> `Number(v.toPrecision(12))` 后交给 JS `String(number)`（十进制指数在 `[-6, 20]` 内用十进制，
> 否则科学计数）。本构建 `internal/calc/calc.go:51` 的 `Format` 用 `FormatFloat(v, 'g', 10, 64)`：
> 10 位有效数字（不是 12）、Go `%g` 的科学阈值是 `[-4, 10)`，于是 `1e-5` 打成 `1e-05`（原版
> `0.00001`）、`1/3` 打成 `0.3333333333`（原版 `0.333333333333`）；整数分支 `math.Trunc(v)==v &&
> |v|<1e15` 还绕过 12 位舍入，`999999999999999` 打成自身（原版 `1000000000000000`）。
>
> **修法**：`internal/calc/calc.go` 的 `Format` 复刻原版四条分支，新增 `roundSignificant`
> （`Number(x.toPrecision(12))` 的 Go 等价）、`trimExponential`、`jsNumberString`（JS
> `String(number)` 的十进制/科学阈值）、`decimalExponent`、`normalizeExponent`；常量
> `calculatorPrecision`(12)/`calculatorScientificHigh`(1e15)/`_Low`(1e-9)。
>
> **测试**：`internal/calc/calc_test.go` 的 `TestFormat` 扩到 20 行表，逐分支钉住
> `0/-0/整数/1e-5/1e-6/1e-7/1e-9/9.9e-10/1e15/1e20/1.23456789012345e20/999999999999999/NaN/±Inf`。
> 变异核查：把 `Format` 换回 `FormatFloat(v, 'g', 10, 64)`，`1e-5`/`1e-7`/`999999999999999`
> 三条立刻红。
> 门槛：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 全绿、
> `GOOS=linux/windows/darwin go build ./...` 三平台通过。

> **上一轮：P7-cs 直出文本的 dim / inverse 规则对齐原版（原版 `src/launcher/PluginTextView.tsx:46-63`）**
> —— P7-cq 清单第 1、2 条已落地，本轮取第 3 条（最后一条，也是清单里唯一 Go 侧可落地项）。依赖检查：
> `go list -m -versions github.com/egoist/mygo` 与 GitHub releases latest 均为 **v0.4.8**（与 `go.mod`
> 一致），无版本可升。只对照 `main`（`backup/main-pre-mygo` = v0.3.14 原版树），未翻历史提交。
>
> **偏离点**：原版 `spanStyle` 把直出文本的一个 run 转成 CSS：`dim` 对**任何**颜色一律 `opacity: 0.62`
> （整 run 半透明，背景也在内）；`inverse` 先互换两侧再**只画互换后确有值的一侧**（`if (fg)`/`if (bg)`），
> 未命名的一侧保持表面默认（默认前景 / 透明背景）。本构建 `internal/launcher/output.go` 的 `outputSpans`
> 把这两条都写偏了：`dim` 只在 `FG.Kind == ansi.Default` 时生效、且用 `Alpha(0.7)`（既不是 0.62，也
> 不作用于具名/索引/真彩色，更不作用于背景）；`inverse` 无条件互换并用 `c.Theme().Background` 补齐空的
> 那一侧，于是「只给背景色 + inverse」多出一块不透明底。
>
> **修法**：抽出纯函数 `resolveOutputSpan`（`internal/launcher/output.go`）：互换只取命令命名的两侧、
> 未命名侧落回 `base`（默认前景）/ `ui.Transparent`；`dim` 对前景与背景一律 `Alpha(0.62)`（新常量
> `dimAlpha`）。`outputSpans` 逐 run 调用它。
>
> **测试**：`internal/launcher/outputstyle_test.go`——`TestDimFadesTheWholeRunNotJustTheDefaultColour`
> （默认/具名前景 + 具名背景都到 alpha 158）、`TestInverseSwapsOnlyTheSidesTheCommandNamed`（只前景 →
> 默认前景 + 具名背景；只背景 → 具名前景 + 透明背景；两侧都具名 → 互换）、
> `TestDimAndInverseComposeInSpanStylesOrder`（先互换再一起 0.62）。变异核查：dim 改回「只在默认前景」
> → dim 两条红；inverse 空背景补成表面底色 → `only_the_background_is_named` 红。
> 至此 P7-cq 清单三条**全部落地**，清单再次为空；余下只有三条卡 mygo 框架项（复制反馈 / 键入
> Unicode 空格缝 / 外观四个死旋钮），规格见「具体待办（P7-cm 重写）」。
> 门槛：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 全绿、
> `GOOS=linux/windows/darwin go build ./...` 三平台通过。

> **上一轮：P7-cr 输出列表的编号快捷键（原版 `src/App.tsx:1602-1611`、`src/launcher/plugin-mode.ts:659-661`、`src/launcher/result-budget.ts:883-900`）**
> —— P7-cq 清单第 1 条已落地，本轮取第 2 条（最高优先级）。依赖检查：`go list -m -versions
> github.com/egoist/mygo` 仍停在 **v0.4.8**（与 `go.mod` 一致），GitHub releases latest 也是 v0.4.8，无版本可升。
> 只对照 `main`（`backup/main-pre-mygo` = v0.3.14 原版树），未翻历史提交。
>
> **偏离点**：原版把插件视图的行喂进与普通结果**同一个**编号家族——`displayedShortcutSlots`
> （`App.tsx:1602-1611`）在 `pluginInteractive` 为真时走 `resultShortcutSlots(displayedResults, …)`，给插件列表的行发
> `⌘1`–`⌘9`/`⌘0`；`pluginViewInteractive`（`plugin-mode.ts:659-661`）只在列表形且 `tier === "interactive"` 时为真。
> 本构建 `internal/launcher/launcher.go` 的 `assignNumbers(results)` 只编号 `a.Results()`，输出视图在屏时
> `modeShortcuts` 只认方向键/Enter/Escape 就 `return`，**从不**问 `⌘N`；`outputRow`（`internal/launcher/output.go:518`）
> 也不画徽标。于是「跑一条命令、输出若干可运行行」时，键盘上的编号键对输出列表完全无效。
>
> **修法**：`internal/launcher/launcher.go` 的 `resultShortcuts` 抽出 `pressedResultDigit`（按键 → 数字，绑定取自
> `select_result`）；新增 `outputShortcuts`（用 `a.outputNumbers` 跑输出列表的第 N 行），并在输出视图的按键分支里
> 接上；`View` 每帧调 `assignOutputNumbers()`。`internal/launcher/output.go` 新增 `assignOutputNumbers`
> （可运行行才编号、`1`–`9` 前九个 + `0` 第十个、超过 `maxResultSlots` 时按 `OutputList.Visible()` 重新编号 = R34/R72），
> `outputRow` 在行尾画 `resultNumber` 徽标（与启动器行同构），`leaveOutput` 清 `outputNumbers`。
>
> **测试**：`internal/launcher/outputnumbers_test.go`——`TestOutputRowsCarryTheNumberedFamily`（三行 1/2/3 + 徽标画出）、
> `TestOutputStatusRowsAreNotNumbered`（状态行不编号、家族不落空）、`TestTheOutputNumberRunsItsRow`（⌘2 插入第二行文本）、
> `TestOutputNumbersFollowTheOutputViewport`（20 行窗口：编号只落在可见区；滚动后重新编号）。变异核查：摘掉按键分支 →
> 按键用例红；把 `!row.Runnable()` 改成 `row.ID == ""`（连状态行一起编号）→ 状态用例三条红；去掉 `OutputList.Visible()` 分支 →
> 视口用例三条红；删掉徽标绘制 → 徽标三条红。
> 至此 P7-cq 清单第 2 条落地，只剩第 3 条（直出文本 ANSI 的 dim/inverse 规则）待做，规格见下一节。
> 门槛：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 全绿、
> `GOOS=linux/windows/darwin go build ./...` 三平台通过。

> **上一轮：P7-cq 输出视图的窗口高度按输出自己的内容计价（原版 `src/App.tsx:2133-2137,2185-2200`、
> `src/launcher/plugin-mode.ts:238-267,647-651`）**
> —— P7-co 清单只剩三条卡框架项（`Options.OnCopy` 复制反馈、终端键入的 `HandleInput` 空格缝、外观四个
> 死旋钮），所以本轮先把「尚未做」重新写实为 3 条带 file:line 的具体待办（下方「具体待办（P7-cq 重写）」），
> 再取第 1 条落地。依赖检查：GitHub releases API 被限流，改用 `go list -m -versions github.com/egoist/mygo`，
> 仍停在 **v0.4.8**，与 `go.mod` 一致，无版本可升。只对照 `main`（`backup/main-pre-mygo` = v0.3.14 原版树），
> 不翻历史提交。本轮换入口：逐文件对照原版的**插件视图几何**。
>
> **偏离点**：原版把插件视图（文本/列表）自己的 `pluginViewRows` 交给窗口高度
> （`App.tsx:2133-2137` 的 `launcherListRows = pluginView ? pluginViewRows(pluginView) : displayedResults.length`；
> `:2197-2200` 文本形按 `pluginViewRows × ROW_HEIGHT_TWO_LINE` 计费）。本构建 `internal/launcher/launcher.go`
> 的 `View` 无论输出视图是否在屏，都用 `a.Results()`（它身后的搜索结果）建 `rows`、把 `len(results)` 交给
> `HoldRows`——于是「跑一条命令、输出 12 行」的窗口高度取决于查询命中了几条命令，而不是输出有多长；
> 输出列表也不受 `min(items, MAX_RESULTS)` 上限约束。实测（临时用例）：同一份 12 行输出，查询 `list`
> （1 条结果）与空查询（多条结果）分别是 166 / 454 像素，而输出本身的长度对高度毫无影响。
>
> **修法**：`internal/launcher/launcher.go` 的 `View` 改调 `a.windowRows(results)`；`internal/launcher/output.go`
> 新增 `windowRows`/`pluginTextRows`/`outputRows`——文本形按 `PLUGIN_TEXT_LINE_UNITS`(24u) 计行、夹进
> `PLUGIN_TEXT_MIN_UNITS`(72u)/`PLUGIN_TEXT_MAX_UNITS`(400u) 后向上取整成 `ROW_HEIGHT_TWO_LINE`(40u) 带行
> （复刻 `pluginTextMetrics.rows`），列表形取前 `MAX_RESULTS`(10) 行、状态行按 30u（`LAUNCHER_STATUS_UNITS`）、
> 其余按 40u，分组标题另计一条 26u 带（`LAUNCHER_SECTION_TITLE_UNITS`）。`internal/launcher/height.go` 的
> `RowInfo` 新增 `Status`/`Band`、`Geometry` 新增 `Sections`，各自按原版的带价计费。
>
> **测试**：`internal/launcher/outputheight_test.go`——`TestTheOutputViewPricesItsOwnContent`（同一输出在
> 一条结果与多条结果下高度相同；12 行高于 3 行）、`TestTheOutputListWindowIsCappedAtTheBudget`（3 行 < 10 行，
> 25 行 == 10 行）；`internal/launcher/height_test.go` 追加 `TestPluginTextRows`（空串/1/2/3/4 行与 100/200 行
> 的带行数）、`TestStatusAndSectionBands`、`TestBandRowsArePricedAsTheirBand`。变异核查：把 `windowRows` 的
> `if a.output != nil` 改成恒假，前两条立刻红（`166` vs `454`；12 行与 3 行同为 `166`）。
> 门槛：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 全绿（1036 项）、
> `GOOS=linux/windows/darwin go build ./...` 三平台通过。

> **最新一轮：P7-cp 输出列表的分组标题与启动器列表同一个寄存器（原版 `src/launcher/LauncherResults.tsx:837-839`、`src/styles/launcher.css:1007-1024`）**
> —— P7-co 清单第 3 条（唯一不卡框架的余项）。依赖检查：`go list -m -versions github.com/egoist/mygo`
> 仍为 **v0.4.8**（与 `go.mod` 一致），GitHub releases latest 也是 v0.4.8，无版本可升。只对照 `main`
> （`backup/main-pre-mygo` = v0.3.14 原版树），未翻历史提交。原版给启动器列表块标题与插件列表分组标题
> 用的是**同一个** `.launcher-section-title`（`LauncherResults.tsx:837-839`，`font-size: var(--text-body)`、
> `font-weight: 620`、`line-height: 1.4`、muted、`6u/12u/4u` 内边距），而本构建 `internal/launcher/launcher.go:1442`
> 的启动器列表标题已对齐这个寄存器，`internal/launcher/output.go:520` 的输出视图标题却是 `FontSize-1`、无字重、
> 且把内边距直接加在 `Text` 上（于是行把文字拉满整宽）。现在 `outputRow` 的标题改成与启动器列表完全同构的
> `ui.Row(...).Padding(Space(1.5), Space(3), Space(1), Space(3))` + `Text(...).FontSize(t.FontSize).FontWeight(620).TextColor(t.TextMuted)`。
> 测试：`internal/launcher/outputheading_test.go` 的 `TestOutputGroupHeadingSharesTheLauncherSectionRegister`
> 把插件列表的分组标题与启动器列表的「最近启动」块标题逐项对照（同一字号行高：两处标签框高都是 22；标题是文字自己的框
> 而不是整行宽）。变异核查：把 `output.go` 的标题改回旧写法，两条断言立刻红（`24` vs `22`、`704` vs `652`）。
> 至此 P7-co 清单只剩第 4 条（三条卡框架项：`Options.OnCopy` 复制反馈、终端键入的 `HandleInput` 空格缝、
> 外观四个死旋钮），规格见下一节与「已知差距」。
> 门槛：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 27 包全绿、
> `GOOS=linux/windows/darwin go build ./...` 三平台通过。

> **最新一轮：P7-co 行内容规则与列表分组标题对齐原版（`src/launcher/row-content.ts`、`src/launcher/list-sections.ts`）**
> —— P7-cm 清单余下三条（复制反馈 / 键入 Unicode-空格缝 / 外观四个死旋钮）仍全部卡 mygo 框架。
> 本轮换入口：逐文件对照原版**行内容与列表分组**这两个纯模块，把「尚未做」重写成新的具体待办
> （下方），再取前两条落地。依赖检查：`go list -m -versions github.com/egoist/mygo` 仍为
> **v0.4.8**（与 `go.mod` 一致；GitHub releases API 本轮被限流，用 `go list` 代替），无版本可升。
> 只对照 `main`（`backup/main-pre-mygo` = v0.3.14 原版树），未翻历史提交。
> （1）**行内容 R12/R14 的转写规则**：原版 `row-content.ts:105-118` 的 `isTranscription` 把副标题里
> 只重复标题的（大小写/全角/空格/标点变体）丢掉，`resultRowContent` 是渲染与高度记账共用的唯一判据；
> 本构建 `launcher.go` 直接画 `item.Detail`，于是 Linux `.desktop` 的 `Name`/`Comment` 同名、命令的
> `Description` 复读 `Name` 时会多出一行零信息的副标题。现在新增 `internal/launcher/rowcontent.go`
> （`transcriptionKey` 手写 NFKC 的全角折叠 + `isTranscription` + `rowSubtitle`），`View` 的行信息与
> `rowBody` 的地板/渲染都改走 `rowSubtitle`——画出的行与记账的行不可能再不一致。
> （2）**列表分组 R39/R72 的首行标题**：原版 `list-sections.ts:49-66` 对 index 0 也算「起一组」
> （`previous` 为 `null` → `pluginGroup`），所以插件列表**第一行**带 `group` 时也印标题；本构建
> `output.go:513` 的 `outputRow` 要求 `i > 0`，首行分组标题从不出现。现在条件改为
> `row.Group != "" && (i == 0 || row.Group != a.output.Rows[i-1].Group)`。
> 测试：`internal/launcher/rowcontent_test.go` 的 `TestTranscriptionKeyFoldsWidthCaseAndPunctuation`、
> `TestARowDropsASubtitleThatOnlyRepeatsItsTitle`、`TestADroppedSubtitleIsChargedOneLine`（画/记两断言
> 一起翻转）、`TestAnOutputListsOpeningGroupPrintsItsHeading`。变异核查：`rowSubtitle` 去掉转写分支 →
> 副标题两条断言变红（`"zebra"` 被画出、行被记 2 行）；`outputRow` 改回 `i > 0` → 首行分组标题缺失。

> **上一轮：P7-cn ANSI 解析器对齐原版 `src/launcher/ansi.ts`（`internal/ansi/ansi.go`）**
> —— P7-cm 清单余下三条（复制反馈 / 键入 Unicode-空格缝 / 外观四个死旋钮）全部卡 mygo 框架，
> 无法在 Go 侧落地。本轮换入口：逐文件对照原版的**直出 ANSI 解析器**，找到并修掉三个可实现偏离点。
> （1）非数字 SGR 参数：原版 `parseSgrParams` 读成 `NaN`、`applySgr` 整段忽略；本构建的 `atoi`
> 返回 0，于是 `ESC[1;?m` 把已点亮的样式**重置**了。现在 `sgrValue` 用 `invalidParam =
> math.MinInt` 标记非数字参数，`applySGR` 跳过它。（2）扩展色分量坏掉：原版 `extendedColor`
> 用 `Number.isInteger` 校验、坏就整条返回 `null`；本构建把 `ESC[38;5;?m` 读成 index 0、把
> `ESC[38;2;255;?;9m` 读成 `RGB{255,0,9}` 还吞掉了后面的 `9`。现在 `readExtended` 遇 `invalidParam`
> 整条返回「无颜色」，后续参数照常处理。（3）非 CSI：原版在 `ESC [` 后字节越出参数/中间字节
> （0x20-0x3f）范围时判定「不是 CSI」、只丢 ESC；本构建只找最终字节，会吞掉真实文本
> （`a\x1b[\x07b` → `a`）。现在 CSI 扫描新增范围检查，越界即 `i++`。测试：
> `internal/ansi/ansi_test.go` 的 `TestMalformedParametersAreIgnored`、
> `TestNotAControlSequenceKeepsTheText`。变异核查：`sgrValue` 错误返回改回 0 → 两条扩展色断言变红；
> 去掉 CSI 范围检查 → 两条文本断言变红。依赖检查：`go list -m -versions github.com/egoist/mygo`
> 仍为 **v0.4.8**（与 `go.mod` 一致），无版本可升。

> **上一轮：P7-cm R90 pin 标题/正文对齐（原版 `src/plugin-window/pin-entry.ts:43-80`）**
> —— 本轮先落地上一轮写好但未提交的 P7-cl 三条（剪贴板行预览/副标题、图片按自己的词可搜、
> 缩略图 memo 有界），再对照原版找到并修掉一个可实现的偏离点。本构建的 `pinClip` 用
> `entry.Label()` 当箱口标题（多行文本被折叠成一句），且对 files 条目钉 `paths` 拼接、对无 caption
> 图片钉 `[image]`；原版 `pinTextFor` 只有携单块文本的行可钉：`title` = 行标题（行预览），`text` =
> `entry.text`，files 无 text 不可钉、无 caption 图片同样不可钉。现在新增 `clipboardPinApplies`
> （text 恒可钉、image 需 caption、files 不可钉），`pinClip` 改用 `clipboardRowTitle` 命名窗口并以
> `entry.Text` 为正文，`launcher.go:1682` 的 pin 控件同步加门。测试：`filters_test.go` 的
> `TestOnlySingleTextClipboardRowsArePinnable`（四子例）、`TestAnEmptyTextCaptureStillPins`、
> `TestAFileRowShowsNoPinControl`。依赖检查：`go list -m -versions` 与 GitHub latest 均为 **v0.4.8**
> （与 `go.mod` 一致），无版本可升。余下三条（复制反馈 / 键入文本钩子 / 四个死旋钮）均卡 mygo 框架，
> 已在「尚未做」写成可执行规格（上游 API → 接线点 → 验收）。

> **上一轮：P7-ck Linux `.desktop` 的可用性判定与 `DBusActivatable`（原版 `linux.rs:395-408,86-140`）**
> —— P7-ci 的 3 条待办里第 3 条（最后一条）落地。本构建的 `ParseDesktopFile` 不看 `Type=`（`Link`/
> `Directory` 条目也被当应用列出），`ScanLinux` 不探测 `TryExec=`（会列出没装的条目），`App.Open` 对
> 所有条目一律 `exec.Command(Exec…)`——而原版在 `into_application` 里丢掉非 `Application` 类型、用
> `executable_available` 丢掉 `TryExec` 不在 PATH 的条目，并在 `open` 里让 `DBusActivatable=true` 的
> 条目走 `gio launch`（session bus 启动 = 自己的 scope + 单实例激活）。现在：`ParseDesktopFile` 增读
> 无 locale 的 `Type`（仅 `Application`，缺省保留）与 `DBusActivatable`，`App` 增 `DBusActivatable` 与
> 未导出的 `tryExec`；`Type` 的丢弃留在纯读取器里，`TryExec` 的可用性探测放进 `ScanLinux`（需要文件
> 系统/PATH，原版也在 `into_application`）——新增 `executableAvailable`（绝对/相对路径看可执行位，裸名
> 走 `PATH`）与 `isExecutableFile`；`App.Open` 抽出 `launchCommand`（D-Bus → `gio launch <path>`，
> 其余 → 自己的 argv），`launchDetached` 统一独立进程 spawn，`gio` 起不来时回退到 argv。测试：
> `TestDesktopEntryTypeGate`、`TestExecutableAvailable`、`TestScanLinuxDropsUnavailableTryExec`、
> `TestDesktopLaunchCommand`。变异核查：摘掉 `Type` 门、`executableAvailable` 恒真、D-Bus 改回 argv，
> 三组断言各自变红。依赖检查：GitHub latest release 与 `proxy.golang.org/.../@v/list` 均确认 **v0.4.8**
> 仍是最新，无版本可升。至此 P7-ci 三条全部落地，清单再次为空（剩余仅卡框架项，见「已知差距」）。

> **上一轮：P7-cj Linux `.desktop` 的搜索别名（原版 `linux.rs:423-440` / `mod.rs:326`）**
> —— P7-ci 的 3 条待办里第 1 条已落地，本轮取第 2 条（最高优先级）。本构建的 `ScanLinux` 从不给
> `App.Aliases` 赋值，`ParseDesktopFile` 也不读 `GenericName`/`Keywords`/`StartupWMClass`/`TryExec`，于是
> 一个中文名 Linux 应用只有拼音首字母能命中，`nautilus`、`org.gnome.Nautilus` 一概搜不到——而 macOS 侧
> `internal/apps/darwin.go` 早就这样做了。现在照抄原版 `into_application` 的候选表：`ParseDesktopFile`
> 增读那四个无 locale 键（本地化变体忽略，与原版一致），新增 `desktopAliases`（候选顺序：GenericName →
> StartupWMClass → `executableName(Exec)` → `executableName(TryExec)` → `Keywords` 按 `;` 切分 →
> `identifierAliases(desktopFileStem)`）、`executableName`（argv[0] 的 base）、`desktopFileStem`（Rust
> `file_stem` 语义：末扩展名且不吞整个 `.desktop`），以及原版 `build_aliases` 的 Go 版 `buildAliases`
> （去空白、丢弃无 ASCII 字母数字者、丢弃与 name/localized 等值者、大小写不敏感去重）与 `hasASCIIAlnum`。
> `identifierAliases` 直接复用 `darwin.go` 里那份（同包）。搜索侧无需改动：`internal/launcher/search.go:524`
> 的 `appItem` 早已把 `Aliases` 拼进 `Search`。测试：`internal/apps/apps_test.go` 的 `TestDesktopSearchAliases`
> （四个子用例：六个来源齐收 + 名称/中文关键词被丢 + 大小写不敏感去重 + desktop-id 应用半段 + `executableName`）
> 与 `TestScanLinuxFillsAliases`（`Name=文件`/`Exec=nautilus` 的扫描产物含 nautilus/gnome/files/folder，
> 且 Initials 仍只吃 name+localized = `wj`）；`internal/launcher/search_test.go` 的
> `TestLinuxDesktopAliasesAreSearchable`（真 `.desktop` → `ParseDesktopFile` → 搜索，`nautilus`、
> `org.gnome.Nautilus`、`folder`、`manager` 四问都命中）。变异核查：把 `buildAliases` 改成 `return nil` →
> 三条用例全红；去掉 `hasASCIIAlnum` 过滤 → 中文关键词 `文件管理` 进别名（红）；去掉 name 等值过滤 →
> `files` 进别名（红）。依赖检查：`proxy.golang.org/.../@v/list` 确认 **v0.4.8** 仍是最新，与 `go.mod`
> 一致，无版本可升。至此 P7-ci 第 2 条落地，第 3 条（Type/TryExec/DBusActivatable）待做。

> **上一轮：P7-ci Linux `.desktop` 的本地化名与 `Comment=`（原版 `linux.rs` / R14 行副标题）**
> —— P7-ch 收尾后「尚未做」又一次为空。本轮换入口：**逐文件对照原版的平台扫描器**。原版
> `src-tauri/src/commands/apps/linux.rs:312-372` 用 `["zh_CN","zh_SG","zh","zh_TW","zh_HK"]` 的阶梯读
> `Name[locale]`/`Comment[locale]`，本构建 `internal/apps/apps.go` 的 `ParseDesktopFile` 却只读无 locale 的
> `Name`/`Exec`/`Icon`/`Hidden`/`NoDisplay`（`Name[zh_CN]=…` 的 key 落到 default），`Comment=` 根本没有
> case；而 `internal/launcher/search.go` 的 `appItem` 也从不设 `Detail`。现在：新增 `desktopLocales` 阶梯 +
> `desktopLocaleRank` + `unescapeDesktop`（`\s`/`\n`/`\t`/`\r`/`\\`），`ParseDesktopFile` 按原版规则取本地化
> 名（`localized == name` 时不计）与 Comment（无 locale 先到先得、可被本地化覆盖，空串丢弃），`ScanLinux`
> 用 `computeInitials(name, localized)` 填拼音；`App` 新增 `Comment` 字段；`appItem` 只在 `Localized == ""`
> 时把 Comment 画成副标题——这正是原版 `useLauncherCatalog.ts:1162-1166` 的
> `(localizedName && name) || comment || …` 经 `row-content.ts:125-171` 的 `isAlternateAppName` 丢掉别名后的净效果。
> 测试：`TestDesktopLocalizedNameAndComment`（7 个子用例：zh_CN 胜 zh_TW、fr 忽略、同名不计、Comment 阶梯、
> 无 locale 回退、空 Comment、转义）+ `TestScanLinux` 补 Files 的 `Localized`/`Initials` 断言 +
> `TestApplicationRowSubtitleIsTheComment`（comment 单独显示 / 本地化名遮蔽 / 都没有则空）。变异核查：
> 把 `if app.Localized == ""` 换成 `if true` → 行副标题用例红（`Detail:"Browse your files"`）；把阶梯改成
> `zh_TW` 在前 → `Localized:"檔案"`、`Comment:"瀏覽檔案"` 两条红。依赖检查：
> `proxy.golang.org/github.com/egoist/mygo/@v/list` 确认 **v0.4.8** 仍是最新，与 `go.mod` 一致，无版本可升。
> 同轮把「尚未做」重写为 3 条具体待办（下方），第 2 条（搜索别名）、第 3 条（Type/TryExec/DBusActivatable）待做。

> **上一轮：P7-ch 捕获输出的 UTF-8 边界与内存上限（P7-cg 清单第 1、2 条）**
> —— 上一轮 P7-cg 把空清单重写成两条「Go 捕获实现自己的缺陷」，工作区里已经躺着一半实现
> （`internal/extensions/output.go` / `output_test.go` 未提交，且 `clip` 与 `run.go` 重定义冲突、`go build`
> 直接红）。本轮把它收口：`output.go` 的 `capturedOutput`（每个流的限长 writer，只留最前 `MaxRunOutputBytes`
> = 1 MiB，`Write` 仍返回完整输入长度，防止子进程拿到 short write/SIGPIPE；只有真超限才置 `truncated`）
> 与 `completeUTF8Prefix`（只在末尾丢弃**不完整**的 UTF-8 编码，完整字符即便正好压在上限上也保留，
> 非法源字节原样保留）；`run.go` 的两个 `bytes.Buffer` 换成 `capturedOutput`，`Truncated` 取两个 writer 的
> 或，`clip`/`utf8Start` 从 `run.go` 删除（`probes.go:180` 继续用 `output.go` 的 `clip`）。至此
> **输出在捕获期间就有界**（不再先全量缓存、退出后才裁剪），且**截断不再产出坏 UTF-8**。
> 测试：`TestClipPreservesUTF8Boundaries`（2/3/4 字节字符的每个切分位置 + ASCII/空/未超限）、
> `TestCapturedOutputIsBoundedWhileWriting`（分块写、恰好满不截、超限后重复写仍只留前缀）、
> `TestCapturedOutputProcess`+`TestRunCapturedBoundsEachStream`（真子进程分别写爆 stdout/stderr/两者，
> 断言正常退出、`Truncated`、两流各自截断且 `utf8.ValidString`）。变异核查：把 `Write` 的
> `min(len(p), cap-buffer.Len())` 换成 `len(p)` → 三条红（`retained 2097152 bytes`、`truncated=false`）；
> 把 `clip` 的 `completeUTF8Prefix` 去掉 → `TestClipPreservesUTF8Boundaries` 红（`valid UTF-8 = false`）。
> 依赖检查：`go list -m -versions github.com/egoist/mygo` 确认 **v0.4.8** 仍是最新，与 `go.mod` 一致，无版本可升。

> **上一轮：P7-cf 删除确认走页内 bar（原版 `RemovalConfirmation` / `.extension-discard-bar`）**
> —— P7-cd 清单第 3 条（P7-cc 记下的残留）。本构建的两条删除路径（普通卸载、删除孤儿）原来用原生
> `mygo.Dialog.Message`，与同页**停用**确认的页内形状不一致。现在照抄停用确认的页内形状：
> `settingsui` 新增 `removeConfirm` 状态、`requestRemove` 武装与 `removeConfirmation` 页内条
> （标题 + 一句后果，Cancel 在前、危险按钮在后；`Generated` 的本地连接换自己的词并追加
> `deleteCustomWarning` 那句「Floter 无法撤销」）；`internal/shell/shell.go` 三条拆除路径里的
> `mygo.Dialog.Message` 全部删除——到达 shell 即是答案，先 `Runs.Kill(id)` 再动文件的顺序不变。
> Escape 在任何确认条（停用 / 删除 / 组件化卸载）打开时先回答问题本身、不顺手离开页面
> （原版的文档层 `stopImmediatePropagation`）；顺带修掉这轮自己引入的一个 bug：Escape 分支原来
> 挂在 `Actions.Close != nil` 里，无 Close 的宿主（测试、嵌入）按 Escape 会吞掉问题。
> i18n：`IntegrationsRemoveTitle/Detail` 两个键换成 `RemoveTitle`/`RemoveDetail`/
> `RemoveCustomTitle`/`RemoveCustomDetail`/`RemoveCustomWarning`/`RemoveCancel`/`RemoveConfirm`/
> `RemoveDelete` 八个（en/zh 全）。测试：`TestIntegrationsPage` 的卸载段重写成「先问 → Cancel →
> 再问 → Escape 取消且不离开页面 → 再问 → 危险按钮确认」，`mygo` 的 `Find` 只回首个同文案元素，
> 危险按钮的坐标从同行唯一的 Cancel 钮读出；新增 `TestGeneratedIntegrationRemovalBar` 钉
> Generated 两态（自己的标题/警告/危险按钮词）。变异核查：删掉页内条则两段断言立刻红。

> **上一轮：P7-cd 命令启动的会话结束即带走终端页（原版 R62）**
> —— P7-cc 收尾后「尚未做」又一次为空。本轮不去翻旧提交，而是**顺着一条原版明写、本构建只做了一半
> 的契约**往回找：原版 `src/terminal/command-session.ts` 的 `shouldExitWithCommandSession` 规定
> **命令启动的会话**（`initialCommand` 或结构化 execution plan）进程一结束就离开终端页、回到启动器，
> 只有**裸** shell（系统 Terminal 行、⌘ 持有行、空态回车、安装行）才保留终帧；`useTerminalView.ts:476`
> 在 `term://exit` 里调它，再走 `returnToInputMode()`（关会话 + 回启动器）。本构建只做了 R9-2 的
> 前半：`internal/shell/shell.go:834` 的 `Exit` 回调**无条件** `Resident(code)` 持有终帧，命令会话也
> 留着——而 `docs/mygo-rewrite-plan.md:746`（P2-p）与 `tool-install.ts:190` 都写着「安装行故意开裸会话，
> 好让 R62 判定为 false」，说明这条契约被引用过却从未实现。顺带一个潜在 bug：`Exit` 回调不携带会话
> 身份，`Resident` 标的是 `Current()`——后台会话退出会把**当前**会话标成 resident。
> 现在：`Session.CommandStarted`（`RunCommand` 与新的 `RunCommandLine` 标 true，安装行的
> `RunShellWithCommand` 不标）、`Actions.Exit(session, code)` 携带会话本身（插件回调在终端自己的
> goroutine 上，索引在主线程用 `IndexOf` 解回，顺带修掉上面那个 bug）、纯判定
> `ShouldExitWithCommandSession(commandStarted, terminalOnScreen)`、`CommandSessionEnded(index,
> terminalOnScreen)` 关会话（**不写 snapshot**：一次性命令的输出不该进下一个 shell 的 scrollback）
> 并报告是否回启动器；shell 用 `a.Surf == SurfaceTerminal` 当「页面在屏上」，启动器自己的 shell 行
> （`ls | grep go`）走 `openShellCommand`（command-started），安装行仍走 `openInstallSession`（裸）。
> 测试：`internal/terminalui/commandsession_test.go` 4 条（判定表、逐会话归属、三种启动方式、规则全表）
> + `internal/shell/commandsession_test.go` 2 条（结构化命令与 shell 行各自带走页面、安装会话留下）。
> 变异核查：把判定改成恒假，三组断言立刻红。

> **上一轮：P7-cc R98「停用即停」接线 + R74 `.cmd`/`.bat` 包装；并订正 P7-cc 清单第 2 条**
> —— P7-cc 那轮把空清单写成 3 条待办，其中第 1 条（R98）落地了却没提交（工作区里躺着）。本轮先把那份
> 工作验绿并收进仓库：`internal/extensions/runregistry.go` 的 `RunRegistry`（一个 id → 一组 cancel，
> `Add` 返回幂等注销函数，`Kill` 返回杀掉的个数，nil 接收者全是 no-op）、`RunKilledFailure` 发
> `run_killed:{extension_id}`、`runCaptured` 在父 ctx 被取消（非超时）且条目有 `IntegrationID` 时回它；
> `internal/shell/shell.go` 的 `App.Runs` 在 `runCommandCaptured`/`loadOutputPage` 注册可取消 ctx，
> `SetIntegrationEnabled(false)`/组件化卸载/普通卸载/删除孤儿**先 `Runs.Kill(id)` 再碰文件**。测试
> `runregistry_test.go`（4 条）+ `TestDisablingStopsARunInFlight`（停用真的把 `sleep 5` 变成 `run_killed`）。
> 然后落地第 3 条（R74）：探测层会匹配 `.cmd`/`.bat` shim，而**执行的四处都没包**——
> `internal/extensions/run.go` 以前直接 `exec.CommandContext(ctx, entry.Program, …)`，CreateProcess
> 根本起不来一个脚本。新增 `internal/extensions/cmdshim.go` 的纯函数 `cmdShimWrap(shim, program, args)`
> （`/D` 跳 AutoRun、`/S` 固定 `/C` 引号、`/C` 执行），**平台判定当参数传进来**，于是整条决定是纯的：
> false 分支逐字节等同（连 slice 都是同一个，测试用 `&argv[0] != &args[0]` 钉住），true 分支在 Linux
> 上也测得到；接线用既有的 `ProgramIsCmdScript`（`paramrun.go:104`，参数校验一直用它）。测试 4 条：
> 真值表、`TestCmdScriptWrapFollowsThePlatformGate`、`TestRunCapturedDoesNotWrapCommandScriptsOffWindows`
> （Linux 上真跑一个名字叫 `runner.cmd` 的 POSIX 脚本，包了就会找不到 `cmd.exe`）、
> `TestRunCapturedWrapsWindowsCommandScripts`（Windows 上真写一个 `.cmd` 跑通，CI 的 windows-latest
> 会执行它——比原版的源码守卫更硬）。变异核查：把 `ProgramIsCmdScript` 换成 `IsCmdScriptName`（抽掉
> 平台门），Linux 那条立刻红（`exec: "cmd.exe": executable file not found in $PATH`）。**另外订正：清单
> 第 2 条（R100 停用先问）在本构建里早就落地了**（`a03b40a`／P4-ac「禁用前确认」，见下方清单里加的订正），
> 那轮审计把它记成「待做」是误判。至此 P7-cc 的 3 条全部落地，清单再次为空。

> **上一轮：P7-cb page-residency 收成一行（原版 `SurfaceResidencyControl` / `GeneralPage.tsx:157-250`）**
> —— 取 P7-ca 重写清单剩下的第 3 条：原版是**一行** `<select>`（off / 预设 / never / custom 四类），
> custom 选中时在同一行内联出数字框 + 单位 + Apply（`.residency-control__custom`），存储值本身是自定义
> 数字时 custom 那条的标签回显 `surfaceResidencyCustomValue({seconds})`；本构建却把它拆成两行——
> 选择器把当前自定义值伪装成第一条选项（`residencyOptions`），下面再无条件画一行输入框 + Set。现在
> `general` 只调 `a.residencyRow(c, copy)`：选择器按 `residencySelectValue` 的纯映射显示存储值，仅 custom
> 时在同一行内联 `TextInput`（Enter 或 Set 提交）。`residencyOptions`/`parseResidency` 换成参考那四个纯
> 函数（`residencySelectValue`/`residencyFromSelect`/`residencyCustomSeedSeconds`/`residencyCustomLabel`）
> 加提交时的 `parseResidencyDraft`（空 = off、非数字 = 默认、数字夹到一天）。i18n 补
> `SurfaceResidencyCustom`（`Custom…`/`自定义…`）与 `SurfaceResidencyCustomValue`（`Custom ({seconds} s)`）。
> 测试 `TestResidencySelectMapping` 钉住纯映射的每个分支；`TestSurfaceResidencyControlIsOneRow` 钉住非
> custom 值下没有内联字段、选 custom 后出现且不写入、提交后回显在 custom 选项标签里、落到预设值时收起。
> 变异核查：去掉「仅 custom 时画字段」的早退，两条断言立刻变红（preset 下出现 `Set`、预设值下字段不收敛）。
> 至此 P7-ca 的 3 条具体待办全部落地，「尚未做」清单再次为空。

> **上一轮：P7-ca 界面大小搬回 Appearance 卡 + Window behaviour 行序（原版 R7-13c / `GeneralPage.tsx:370-390`、`:450`/`:464`）**
> —— P7-bz 收尾后「尚未做」又一次为空，本轮先把它重写成 3 条带 file:line 的具体待办（下方「具体待办
> （P7-ca 重写）」），再取第 1、2 条落地。两条都是 General 页的**归类与行序**偏差：
> （R7-13c）原版把界面大小（`settings.uiScale`）画在 Appearance 卡里、theme/language **之后**，注释写明
> 「它和 theme/language 是同一类选择——界面长什么样」；本构建把它放在 Window behaviour 卡首行，于是
> 一个缩放整个界面的外观选项和一个「窗口失焦隐藏」的开关同处一区。现在 `general` 把 `a.pick(c, copy.Scale,
> copy.ScaleHint, copy.Scales, s.UIScale, …)` 移进 Appearance 卡（Language 之后）。
> （原版 `:410`/`:431`/`:450`/`:464`）原版 Window behaviour 卡的行序是 launch-at-startup → hide-on-blur →
> **page-residency** → **recent-items**；本构建把 recent-items 画在 page-residency 之前，两个相邻但有
> 先后关系的开关顺序颠倒。现在 residency（连同自定义秒数行）排到 recent switch 之前。测试
> `TestInterfaceSizeSitsWithAppearance` 钉住：Scale 行在 Appearance 标题之下、Window behaviour 标题之上，
> 且在 Theme/Language 之后；residency 行在 recent switch 之上。变异核查：把 Scale 移回 Window 卡、或把
> recent switch 挪到 hide-on-blur 之后，两条断言分别立刻变红（后者实测 `residency ... is not above recent`）。
> 至此 P7-ca 第 1、2 条落地，第 3 条（page-residency 收成原版那一行控件）由下一轮 P7-cb 落地。

> **上一轮：P7-bz General 页独立的「菜单栏」区（原版 `settings.group.menuBar`）** —— P7-by 之后
> 「尚未做」只剩 P7-bx 重写的第 3 条，本轮取它：原版给菜单栏自己的 section（标题
> `settings.group.menuBar`），把**显示菜单栏图标**开关（R7-10c）与**应用图标**明暗选择（R150）放进
> **同一张卡**，注释写明理由是「两者说的是同一个图标」（`GeneralPage.tsx:486-489`）。本构建把
> `app_icon` 拆在 Appearance 卡、`show_menubar_icon` 拆在 Window behaviour 卡，同一个决定的两端隔了
> 一屏。现在 `general` 在 Window behaviour 与 Terminal appearance 之间插入
> `a.section(c, copy.GroupMenuBar, "", …)`：卡内先画开关、后画图标选择器（**与原版一致**；P7-bx
> 重写的待办把顺序写成「图标在前」，与原版 `GeneralPage.tsx:509`/`:529` 相反，本轮按原版更正），
> Appearance 卡只剩主题/语言、Window behaviour 卡去掉开关。i18n 新增 `GroupMenuBar`（en `Menu bar` /
> zh `菜单栏`，旧版原文）。测试 `TestGeneralMenuBarGroup` 钉住：区标题只出现一次；两个控件都在标题
> 之下；开关在图标之上（原版顺序）；其余区标题都不夹在两控件之间（即两控件同一张卡）；Appearance
> 卡最后一行（语言）在菜单栏区之上。变异核查：把图标行换到开关之前，「开关在图标之上」立刻变红。

> **上一轮：P7-by General 页独立的「玻璃」区与区脚注（原版 `settings.material` / `settings.group.transparency`）**
> —— P7-bx 重写的 3 条具体待办第 1 条已落地，本轮取第 2 条：原版把材质分成自己的 section
>（标题 `settings.material` = Glass），里面两张卡——玻璃强度卡与透明度卡（卡的无障碍名
> `settings.group.transparency`）——区尾用两句 `.settings-section__hint` 交代玻璃强度与透明度
>（`glassIntensityHint`、`transparencyHint`），**不重复在每一行**。本构建把玻璃控件折进 Appearance
> 卡、透明度两条滚轴放第二张卡且每条各带一句行内 hint（`MainOpacityHint`/`TerminalOpacityHint`）。
> 现在 `general` 把玻璃控件搬出 Appearance 卡：Appearance 卡只剩主题/图标/语言；新增
> `a.sectionFoot(c, copy.Material, "", []string{copy.GlassHint, copy.TransparencyHint}, …)` 装两张卡
>（玻璃强度卡 + 用 `a.cardLabeled(copy.GroupTransparency, …)` 画的透明度卡），两条滚轴的 description
> 传空；`rows.go` 新增 `sectionFoot`（`section` 现在是它的无脚注包装）与 `cardLabeled`（给卡挂无障碍名）。
> i18n 删掉 `MainOpacityHint`/`TerminalOpacityHint`（原版只有一句 `transparencyHint`），新增
> `Material`/`TransparencyHint`/`GroupTransparency`（en/zh 原文：Glass/玻璃、原 transparencyHint、
> Transparency/透明度）。测试 `TestGeneralMaterialGroupFootnotes` 钉住：区标题与两句脚注各恰好一次、
> 玻璃控件在区标题之下（不再在 Appearance 卡）、两条透明度行在区标题之下且脚注在末行之下；变异核查：
> 脚注列表多塞一句 `TransparencyHint` 后计数断言立刻变红。

> **上一轮：P7-bx 四条滚轴的轴两端词与 R162 玻璃说明（原版 `SettingsScale` / `settings.transparency.scaleLow`）**
> —— P7-bw 收尾后「尚未做」又一次为空，本轮先把它重写成 3 条带 file:line 的具体待办
>（下方「具体待办（P7-bx 重写）」），再取第 1 条落地：原版每一条 range 行都是同一形状——**标题行 = 标签
> + 实时读数**（space-between），**轨道下 = 轴两端各一个词**（`SettingsScale`）；透明度两条滚轴是
> Transparent/Solid（`settings.transparency.scaleLow/.scaleHigh`），终端外观的字号与行高两条是
> Small/Large（`settings.scale.small/.large`，与 Interface size 分段控件共用词）。本构建的 `a.slider`
> 把读数放在轨道行尾、**从不画两端词**，一个孤零零的 `47%` 没有在说 10%/100% 各意味着什么。现在
> `a.slider` 增加 `low`/`high` 两个参数（都空则不画轴词，不发明一根轴），读数移到标题行右端、改
> `TextStrong`，轨道下用 `FontSize-2`/`TextMuted`/weight 500 画两端词；四个调用点分别传透明度两词与
> 终端两词。顺带把 `GlassHint`（en/zh）从 R162 之前的旧句换成 R162 措辞（点明扭曲/折射/透镜质感）。
> i18n 新增 `TransparencyScaleLow/High`（Transparent/Solid、透明/不透明）与 `ScaleSmall/Large`
>（Small/Large、小/大）。测试 `TestSliderRowsNameTheirAxis` 钉住 Transparent/Solid 各恰好 2 次、
> Small/Large 各 ≥3 次；变异核查：轴词行改成 `if false` 后四条断言全红。

> **上一轮：P7-bw 本机连接请求的后端校验补齐（原版 `create_custom_integration_locked` / `settings.extensions.form.*`）**
> —— P7-bu 重写的 3 条具体待办只剩第 3 条，本轮取它：原版在写包前逐条校验并回**本地化键**，名字
> 1..80（`form.nameLength`）、命令形状 `[a-z0-9][a-z0-9_-]{0,63}`（`form.commandShape`）、版本必须能
> 解析（`Version::parse`）、平台非空（`form.platformRequired`）、模式仅 executable/script
>（`form.modeInvalid`）、脚本内容非空（`form.scriptEmpty`）、`script_language` 只对脚本合法
>（`form.scriptLanguageInvalid`）。本构建的 `normalizeCustom` 以前只查「名字非空 / 脚本有语言且非空 /
> 模式合法 / 权限已知 / params 合法」，其余不查：空版本直接填 `0.0.1` 而不看格式、`Platforms` 允许为空、
> executable 也照收 `ScriptLanguage`（而表单恰好把语言默认成 `js` 一起发出来，只是后端不查才没暴露）。
> 现在新增 `internal/extensions/customvalidate.go`：`CustomIssues(CustomRequest) []CustomIssue` 是**后端与
> 表单共用**的那套规则（`CustomField` 指出问题属于哪一行、`CustomIssueKey` 是旧构建七个 i18n 键的稳定
> 身份），`normalizeCustom` 先在默认值填完后跑它并按第一个键拒绝（`ErrBadRequest: commandShape` …），
> 派生出来的命令（程序自己的文件名）另用 `validCustomCommand` 兜一遍；`internal/shell/shell.go` 的
> `customRequest` 给非脚本清掉 `ScriptLanguage`/`ScriptContent`（表单的语言状态会活过「切回程序」），
> `internal/settingsui/settingsui.go` 里表单抽出 `customDraft`（一处决定每个字段的含义）并在提交时先跑
> `customIssues`，失败的行用 `toolIssueUnder` 把对应句子印在行下（首次提交前不印刷，`toolIssue` 开关）；
> 版本框「空 = 兜底 0.0.1」的既有行为保留，只补上**格式非法就拒**。i18n 新增六个字段（`LocalToolNameLength`
> /`LocalToolCommandShape`/`LocalToolVersionShape`/`LocalToolModeInvalid`/`LocalToolScriptEmpty`/
> `LocalToolScriptLanguageInvalid`，en/zh）。测试 5 条：`TestCustomIssuesNamesEveryRule`（逐条合法通过 /
> 非法拒绝，并断言后端错误串里带同一个键）、`TestCustomCommandShapeAppliesToTheDerivedWord`（派生命令也被
> 查、且小写化）、`TestCustomDefaultsFillWithoutInventingAPlatform`、`TestLocalToolFormNamesTheShapeRules`
>（表单逐行印句子 + 一个都不发到后端 + executable 不带脚本字段）、`TestAnExecutableCarriesNoScriptFields`。
> 已做证伪式核查：抽掉 `validCustomCommand` 的循环后三条命令用例与派生命令测试同时变红，恢复后全绿。
> 落地见下方「P7-bw 后端校验」。

> **上一轮：P7-bv 本机连接表单的「支持的平台」勾选（R7-5 / 原版 `customPlatforms`）** —— P7-bu 重写的
> 3 条具体待办第 1 条已落地，本轮取第 2 条：原版的自定义集成表单有一组 darwin/linux/windows 三选，
> 默认只勾**当前平台**（`ExtensionsPanel.tsx:374` 的 `CURRENT_PLATFORM`、`:393` 的
> `platforms: [CURRENT_PLATFORM]`），一个都不勾时提交按钮禁用。本构建 `CustomTool` 里根本没有
> `Platforms` 字段，`customRequest` 也不传，于是本机连接出来的集成 `Platforms` 恒为空（= 所有平台），
> 用户无法限定；后端 `extensions.CustomRequest.Platforms` 其实一直会写进 manifest
>（`internal/extensions/custom.go:435`），缺的只是表单与映射。现在 `CustomTool` 加 `Platforms`，表单在
> 「输出」行下画三个勾选（`customPlatforms`：macOS/Linux/Windows ↔ darwin/linux/windows），未触碰时
> 默认勾当前平台（`currentPlatform()`，未知 GOOS 回落 linux，同原版 `CURRENT_PLATFORM` 的三元），
> 一个都不勾时拒绝提交并把 `form.platformRequired` 那句印在该行下（`a.issueUnder`），`beginEdit` 按
> manifest 回填、`toolForEdit` 从 `manifest.Platforms` 读回、`customRequest` 逐字透传。落地见下方
> 「P7-bv 支持的平台」。

> **上一轮：P7-bu 自定义集成描述控件的三条契约（R158 + R159 / 原版 ebb20ca）** —— P7-bt 收尾后
> 「尚未做」又一次为空，本轮先把它重写为 3 条带 file:line 的具体待办（下方「具体待办（P7-bu 重写）」），
> 再取第 1 条落地。原版 v0.3.14（`main` 的 `ebb20ca`）把描述输入收成 **executable-only**：脚本清单写死
> 自己的内部句子（`"Local script integration"`）、从不读请求里的描述，所以脚本模式下那个框是「可编辑但
> 被静默丢弃」的死控件（R158）；可执行分支则拿到两条 R159 契约——留空诚实存 `""`（不再落
> `Local integration for <exe>` 这句用户永远清不掉的兜底），输入框配 placeholder 写明「留空则不显示
> 描述」、label 改成「描述（可选）」。本构建的 `customManifest`/`customDescriptor` 脚本模式照读请求描述
> （`internal/extensions/custom.go`），`toolForEdit` 对脚本也把内部句子读回表单（`internal/shell/shell.go`），
> 表单描述行无条件绘制且无 placeholder（`internal/settingsui/settingsui.go`），label 只是 `"Description"`
> （`internal/i18n/copy.go`）。现在抽出 `customDescription`、按模式读回、只在 `mode != routeScript` 画那
> 行并补 placeholder 与新 label。落地见下方「P7-bu 描述契约」。

> **上一轮：P7-bt Windows 冒烟作业（原版 ad843f3）** —— P7-br 重写的 4 条具体待办里只剩第 3 条：真实
> windows-latest 上跑一次那个 .exe、截屏。本构建的每个门都是 Linux 门，`cfg(windows)` 那几段
> （`internal/shell` 的 `vibrancyForStop` windows 分支、玻璃雾层、窗口圆角、终端原生库）编译过一百次、
> 执行过零次。本轮新增 `.github/workflows/windows-smoke.yml`（只有 `workflow_dispatch`，**故意不进
> release 链**）：checkout + setup-go → `go tool mygo build -platform windows/amd64` →
> `FLOTER_OPEN=settings` 后台启动 `dist/windows-amd64/floter.exe` → 等 20s 确认进程还活着 → PowerShell
> `CopyFromScreen` 截屏 → `upload-artifact`。配套守卫测试 `internal/ci/workflows_test.go` 钉住「只有
> 手动触发、`go.yml` 不引用它、产物是截图」。并已用 `gh` 真 dispatch 一次（run `38349260181`，2m14s
> 全绿，截图确认是真实窗口）。落地见下方「P7-bt Windows 冒烟作业」。

> **上一轮：P7-bs `prefers-contrast: more` 接进主题与玻璃（原版 base.css:1387-1574）** —— P7-br 重写的
> 具体待办只剩两条：第 3 条（Windows 冒烟作业）要真机跑一次，第 4 条是纯代码、可本机验证，本轮取第 4 条。
> 原版把「提高对比度」当作**材质**的事而不只是边框：`--glass-frame-floor: 0.86`（透明度滑杆的底端被抬到
> 近乎不透明）、`--glass-step-dim: 0`（去掉雾层）、`--glass-lens-scale: 0.6`（反光阻尼），外加
> `--stroke-contrast` / `--stroke-contrast-soft` / `--text-placeholder-contrast` 三个 token。本构建此前对
> `HighContrast` 零引用，且 `theme.go:212-227` 恰好把框架自己的高对比度加固覆盖掉。现在新增
> `theme.ForPreferences(settings, dark, ui.Preferences)`（`For` 变成它的无偏好包装）与
> `glassmap.SpecForPreferences` / `Spec.Contrast()`，全部带 `*ui.Context` 的绘制点改读
> `c.Preferences()`。落地见下方「P7-bs 提高对比度接进主题与玻璃」。

> **上一轮：P7-br 分页页脚的三态 + 两个环境循环遵守减少动效（R29 / R8+R161）** —— P7-bp 重写的具体待办
> 第 1–3 条全落地后清单又空，本轮先把它重写为 4 条带 file:line 的具体待办（下方「具体待办（P7-br 重写）」），
> 再取前两条落地。第一条回到原版 R29 的页脚三态：`more` 状态只有一行**空行**（注释写明「the scroll itself
> is the affordance」）、`loading` 状态是 **spinner + Loading more…**、`end` 状态是 `No more results`；
> 本构建以前在 `more` 画了原版没有的 “Scroll for more”、`loading` 只有字没有环。现在抽出纯函数
> `outputFooterState`，`more` 只留高度、`loading` 画 12px 圆环（`Hairline` 整圈 + `TextMuted` 四分之一弧，
> 0.7s 一圈），并删掉原版没有的 `OutputMore` 文案。第二条是原版 `prefers-reduced-motion` 命名块点名关掉的
> **两个环境循环**（`.launcher-plugin-footer__spinner` 与 `.terminal-bar__dot`）：框架的 `Loop` 明确不跟这个
> 偏好，所以新增 `internal/motion` 让 App 自己读，终端状态点顺手回到原版 keyframes（0.5 与 0.88 缩放，
> 此前是 0.75..1 且不缩放）。落地见下方「P7-br 分页页脚三态与减少动效」。

> **上一轮：P7-bq 版本兜底提示 + 选项「一行一个」（R153 剩余两条）** —— P7-bp 重写的具体待办
> 第 2、3 条（都是 R153 的遗留）：原版给可编辑的版本框印一条 hint，说明探测到真实版本会自动覆盖、
> 探测失败时才用作者填的值兜底；又把选项编辑器的分隔符从逗号改成换行，因为一个选项值本身可能含逗号
> （`1,000`），逗号分隔会在下一次往返时把它拆成两个。本构建以前版本行只有占位符 `0.0.1`、选项仍用
> 逗号分隔。现在版本行补上本地化 hint（`LocalToolVersionHint`），选项编辑器改成多行 `ui.TextArea`、
> 按换行拆分（trim、去空行）、`beginEdit` 按换行写回，提示改成「每行一个」。落地见下方
> 「P7-bq 版本兜底提示与逐行选项」。

> **上一轮：P7-bp 本地工具表单的声明确认收进「高级」披露（R161）** —— 原版最近一轮
> （`main` 的 v0.3.14，commit 4b2c168）把自定义集成编辑器里两个**只做声明/审计**的权限字段集收进一个
> 默认折叠的 `<details>`「Advanced · Permissions & audit」，理由写在注释里：它们自己的提示就承认
> 「不提供操作系统隔离」，画成与宿主真正读取的字段同权，会让一个全新的集成一打开就是一张仪式表单。
> 本构建的 `localToolSection` 以前把 `extensions.AllPermissions` 逐条画成开关行、且没有那条边界提示。
> 现在它们一起收进默认折叠的「高级」披露：折叠只是表现，勾选仍住在 `a.toolPermissions` 里、提交时照读，
> 所以折叠不丢任何东西。落地见下方「P7-bp 本地工具权限收进高级披露」。

> **上一轮：P7-bo 删掉两处经核实无用的死代码（browser profile key + CommandInfo.ExtensionName）** ——
> P7-bm 重写的具体待办第 3、4 条。两条都不卡框架，是「写而不读」的冗余：本构建的浏览器**开页**走
> `Result.BrowserID`（`internal/launcher/search.go` 的 `openResult` 直接把它交给 `browser.OpenURL`），
> 从不经 profile key 反解；设置面集成分组卡片直接用 `Integration.Name`，命令开关表只用 `CommandInfo` 的
> `ExtensionID`/`CommandID`。两处删除后行为不变、无需测试改写（只删了两条为死符号写的测试）。落地
> 见下方「P7-bo 删除两处死代码」。

> **上一轮：P7-bn 删除双方共有的死代码（capability 报告探针）** —— P7-bm 重写的具体待办第 2 条。
> 本构建 `internal/extensions/capability.go` 的 `ProbeCapabilities`/`CapabilityReport` 只有自己的
> 测试调用，原版 `capability_probe.rs` 的 `CapabilityScanner::scan` 同样只有测试调用，且原版
> reprobe 路径传的是 `HealthReport::new(Default::default())`（`probe_executor.rs:17`）——所以原版卡片
> 里的 `capabilities.version` 恒为 `unknown`、features/limitations 恒空，两版都不展示。既然这是
> **双方共有的死代码**、接线又要改磁盘格式并偏离原版，本轮选择**删除**：清掉死探针，把文件里还活着的
> `--help` 派生与 semver 提取重命名为 `helpderive.go`。

> **上一轮：P7-bm 健康报告的探针明细与检查时间** —— 「尚未做」清单为空，本轮先把它重写为 4 条
> 带 file:line 的具体待办，再取第 1 条落地。仓库里持久化的健康报告（`probeReport`）只被投影成一句
> 状态词，`ProbeRecord.DurationMs`/`ExitCode`/`Stderr` 与 `HealthReport.CheckedAt` 全部算完就丢；
> 现在卡片在 Health 行下逐条画出探针（记号 + id + 耗时）、失败项（退出码 + stderr + 可重试）与检查
> 时间，`Retryable` 由清单 `required` 取反投影，磁盘格式不变。

> **上一轮：P7-bl 清单的严格解析与组合规则（`deny_unknown_fields` + `validate_paths` 的组合那半）** ——
> P7-bh 重写的具体待办第 5 条（最后一条）。清单的未知字段拒绝与组合规则已由 8f99926 落地，但
> `validate_param_definitions` 那一半还缺：`ValidateParams` 没在**解析期**跑、也不查 default 是否合
> kind，且这套规则一条测试都没有。现在 `Manifest.Validate()` 在解析期跑完组合 + 参数声明 + 路径 +
> lifecycle 四套规则，四组新测试钉住「未知字段/尾随数据拒绝」「五种合法组合 + 非法组合拒绝」
> 「坏声明在解析期被拒」「default 逐 kind 校验」。至此 P7-bh 的具体待办 5 条**全部落地**。

> **上一轮：P7-bk 运行表单的逐参数本地化错误（`paramValueIssues`）** —— P7-bh 重写的具体待办第 4 条。
> P7-bj 把后端拒绝变成 `run_param_*` 键，但表单自己的预检只回一句泛化的
> `RunIncomplete`，且 number 用 `ParseFloat`（放过 `inf`/`NaN`/`0x1p-2`）、boolean 不校验、
> required 不认 default 豁免。现在 `ParamIssues` 返回 `{Index,Key,Label}`，每个非法值
> 的句子印在**它自己那一行下面**，Run 按钮在有错时禁用；输出视图也按语言渲染 `run_param_*`。

**P7-bw 后端校验（原版 `create_custom_integration_locked` / `settings.extensions.form.*`）（本轮）**

- **依赖检查**：GitHub releases API 被限流，改用 `git ls-remote --tags` 确认 `v0.4.8` 仍是最新 tag
  （`go.mod` 已是 v0.4.8），无版本可升，故本轮无升级、无用点。
- **入口**：P7-bu 重写的具体待办第 1、2 条已分别由 P7-bv 落地，只剩第 3 条（`normalizeCustom` 缺的
  五条后端校验），本轮取它。
- **原版对照**：`src-tauri/src/extensions/install.rs:240-300` 的
  `create_custom_integration_locked`：`name.is_empty() || name.chars().count() > 80` → `form.nameLength`；
  `command` 先 `trim().to_ascii_lowercase()` 再按 `[a-z0-9][a-z0-9_-]{0,63}` 校验 → `form.commandShape`；
  `Version::parse(request.version.trim())` 失败即拒（错误串是临时拼的，没有 i18n 键）；
  `request.platforms.is_empty()` → `form.platformRequired`；`mode` 只允许 `script`/`executable` →
  `form.modeInvalid`；脚本分支 `script_content` 空 → `form.scriptEmpty`；且
  `!script_mode && script_language.is_some()` → `form.scriptLanguageInvalid`。键的英文/中文见
  `src/i18n.ts:840-845` / `:1695-1700`。
- **本构建（改前）**：`internal/extensions/custom.go` 的 `normalizeCustom` 只查「名字非空」「脚本有语言且
  非空」「模式合法」「权限已知」「params 合法」；空 `Version` 直接填 `0.0.1` 不校验格式、`Platforms` 允许
  为空、executable 照收 `ScriptLanguage`。更隐蔽的是：`internal/settingsui` 的表单把 `ScriptLanguage`
  默认成 `scriptlang.Languages[0].ID`（`js`）并对 executable 一起发出去，只因后端不查才没暴露；
  `internal/shell/shell.go` 的 `customRequest` 也原样透传。
- **改动**：
  - 新增 `internal/extensions/customvalidate.go`：`CustomField`（问题属于哪一行）、`CustomIssueKey`
    （七个旧 i18n 键的稳定身份，另加 `versionShape`）、`CustomIssue`、
    `CustomIssues(CustomRequest) []CustomIssue`（按表单行序返回，未填的命令/版本视为默认值、不报）、
    `validCustomCommand`（`[a-z0-9][a-z0-9_-]{0,63}`）。
  - `internal/extensions/custom.go`：`normalizeCustom` 先 trim/lowercase、填 `Mode`/`Version` 默认值，再跑
    `CustomIssues` 并按第一个键以 `ErrBadRequest: <key>` 拒绝（在 I/O 检查之前，保证表单与后端看到同一条
    规则）；`Mode` 未知不再走 `default` 分支而是由 `CustomIssues` 报 `modeInvalid`；派生出来的命令
    （`executableCommand` + 小写化）另用 `validCustomCommand` 兜一遍。
  - `internal/shell/shell.go`：`customRequest` 对非脚本清空 `ScriptLanguage`/`ScriptContent`（脚本字段只属于
    脚本运行时）。
  - `internal/settingsui/settingsui.go`：抽出 `customDraft`（一处决定每个字段含义，非脚本不带语言）与
    `customIssues`/`customIssueMessage`/`toolIssueUnder`；`App` 加 `toolIssue`（首次提交前不印刷，提交失败
    才在对应行下印句子），表单在提交时先跑 `customIssues` 再发后端。
  - `internal/i18n`：新增 `LocalToolNameLength`/`LocalToolCommandShape`/`LocalToolVersionShape`/
    `LocalToolModeInvalid`/`LocalToolScriptEmpty`/`LocalToolScriptLanguageInvalid`（en/zh 逐字对齐原版；
    `versionShape` 是本构建自拟的句子，原版那处没有 i18n 键）。
- **刻意保留的一处差异**：版本框留空仍然是「兜底 `0.0.1`」而不是原版的「空即拒」——这是本构建既有的、
  与表单 placeholder 一致的契约，本轮只补「**填了就必须是有效版本**」。原版有键的是七个，我们的
  `versionShape` 是唯一自拟的键。
- **测试**：`internal/extensions/custom_test.go` 的 `TestCustomIssuesNamesEveryRule`（10 条非法兄弟 + 4 条
  合法对照，逐条断言「恰好一条、字段与键正确」且 `normalizeCustom` 的错误串里带同一个键）、
  `TestCustomCommandShapeAppliesToTheDerivedWord`（程序文件名带空格 → 派生命令被拒；带连字符的大写文件名 →
  命令小写化）、`TestCustomDefaultsFillWithoutInventingAPlatform`；`internal/settingsui/view_test.go` 的
  `TestLocalToolFormNamesTheShapeRules`（初次打开发型不印、逐次提交把版本/命令/名字的句子印在行下、修好即
  连上、executable 不带脚本字段）；`internal/shell/scripttool_test.go` 的
  `TestAnExecutableCarriesNoScriptFields`（`customRequest` 清脚本字段后 `PrepareCustom` 接受）。已做证伪式核查：
  抽掉 `validCustomCommand` 的循环，三条命令用例与派生命令测试同时变红，恢复后全绿。
- **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 29 包全绿（919 项）、
  `GOOS=linux/windows/darwin go build ./...` 三平台通过。

**P7-bv 支持的平台（原版 `customPlatforms` / R7-5）（上一轮）**

- **依赖检查**：GitHub releases API 返回 `v0.4.8`（`go.mod` 已是 v0.4.8），go proxy 的
  `go list -m -versions` 也停在 v0.4.8，无版本可升，故本轮无升级、无用点。
- **入口**：P7-bu 重写的 3 条具体待办里第 1 条已落地，本轮取第 2 条（最高优先级）。
- **原版对照**：`src/ExtensionsPanel.tsx:374` 的 `CURRENT_PLATFORM`（Mac→darwin，Win→windows，其余
  linux）与 `:393`/`:1573` 的 `platforms: [CURRENT_PLATFORM]` 默认、`:1621` 编辑时
  `platforms: [...definition.platforms]`；`CustomIntegrationDrawer.tsx:23` 的 `PLATFORMS` 三元组与
  `:158` 的 fieldset（macOS/Linux/Windows 三个 checkbox）、`:178` 提交按钮的
  `disabled={… || integration.platforms.length === 0 || …}`；`src/i18n.ts:694`
  `settings.extensions.customPlatforms`、`:842` `settings.extensions.form.platformRequired`。
- **本构建（改前）**：`internal/settingsui` 的 `CustomTool` 没有 `Platforms` 字段、表单没有平台行、
  `internal/shell/shell.go` 的 `customRequest` 不传 `Platforms`、`toolForEdit` 不回填
  `manifest.Platforms`；后端 `customManifest`（`internal/extensions/custom.go:435`）其实早就把
  `request.Platforms` 落盘，所以缺的全在前端与映射。
- **改动**：
  - `internal/settingsui/settingsui.go`：新增 `customPlatforms`（id ↔ 显示名，macOS/Linux/Windows）与
    `currentPlatform()`（未知 GOOS 回落 `linux`）；`CustomTool` 加 `Platforms []string`；App 加
    `toolPlatforms map[string]bool`，为 `nil`（表单还没被碰过）时默认只勾当前平台，`beginEdit` 按
    manifest 填、`endEdit` 清回 `nil`；表单在「输出」行下画三个 `ui.Checkbox`，一个都不勾时在行下印
    `LocalToolPlatformRequired` 并在提交处提前 `return`。
  - `internal/shell/shell.go`：`customRequest` 透传 `Platforms`（脚本模式也保留，平台与模式无关），
    `toolForEdit` 从 `manifest.Platforms` 回填。
  - `internal/i18n`：新增 `LocalToolPlatforms`（Supported platforms / 支持的平台）与
    `LocalToolPlatformRequired`（Select at least one supported platform / 请至少选择一个支持的平台），
    逐字对齐原版两个键。
- **刻意对齐原版的一处语义**：编辑一个 manifest 里 `platforms` 为空的既有集成时，表单是**一个都不勾**
  （原版 `platforms: [...definition.platforms]` + 按钮禁用），而不是替用户勾上全部——空列表在
  manifest 里是「无约束」，回写三个平台名会把它窄化成「已知三平台」。用户必须显式选一个才能保存，
  行下的句子会说明原因。
- **测试**：`internal/settingsui/view_test.go` 的 `TestLocalToolPlatformsRow`（新表单只剩当前平台勾选 →
  取消后提交被拒且行下出现 `LocalToolPlatformRequired`、`connected` 为空 → 勾 linux+windows 后提交，
  `draft.Platforms` 逐字为 `linux,windows`；已做证伪式核查：删掉提交前的空列表 `return` 该测试即变红）；
  `TestEditGeneratedIntegration` 补平台回填/回写断言；`internal/shell/scripttool_test.go` 的
  `TestPlatformsTravelIntoTheManifest`（`customRequest` → `PrepareCustom`/`Commit` → manifest 逐字落盘 →
  `toolForEdit` 逐字读回；列表含本机平台，因为宿主只装支持本机的包）。
- **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 29 包全绿（914 项）、
  `GOOS=linux/windows/darwin go build ./...` 三平台通过。

**P7-bu 描述契约（原版 `ebb20ca` / R158 + R159）（上一轮）**

- **依赖检查**：GitHub releases API 被限流，改用 go proxy（`go list -m -versions github.com/egoist/mygo`）
  确认 v0.4.8 仍是最新（`go.mod` 已是 v0.4.8），无版本可升。
- **入口**：P7-bt 之后「尚未做」又一次为空（P7-br 的 4 条全落地），本轮先把它重写为 3 条带 file:line 的
  具体待办（下方「具体待办（P7-bu 重写）」），再取第 1 条实现——原版最近一个未复刻的提交 `ebb20ca`
  （v0.3.12，R159）与它依赖的 R158 规则。
- **原版对照**：`src/extensions/CustomIntegrationDrawer.tsx` 把描述输入写成
  `integration.mode === "executable" && <label>…` 并带 `placeholder={t("…customDescriptionPlaceholder")}`；
  `src-tauri/src/extensions/install.rs:341-342` 的脚本分支写死 `"Local script integration"`、`:1306` 的读回
  是 `(mode == "executable").then_some(manifest.description)`；`src/i18n.ts:679`/`:682` 是
  `Description (optional)` / `Leave blank to show no description`（zh 见 `:1561`/`:1562`）。
- **本构建（改前）**：`internal/extensions/custom.go` 的 `customManifest`/`customDescriptor` 两种模式都照
  读 `request.Description`；`internal/shell/shell.go` 的 `toolForEdit` 也把 `manifest.Description` 无条件读回；
  `internal/settingsui/settingsui.go` 的描述行无条件绘制且无 placeholder；`internal/i18n/copy.go:658` 的
  label 只是 `"Description"`、没有 placeholder 字段。
- **改动**：
  - `internal/extensions/custom.go`：新增 `customDescription(request)`——脚本 → `"Local script integration"`，
    可执行 → trim 后的用户原文；`customManifest` 与 `customDescriptor` 都改走它（脚本的 manifest 与
    descriptor 因此都不再泄漏用户描述）。
  - `internal/shell/shell.go`：`toolForEdit` 只对非脚本把 `manifest.Description` 读回表单。
  - `internal/settingsui/settingsui.go`：`mode != routeScript` 时才绘制描述行，并挂上 R159 placeholder
    （值仍留在 `a.toolDescription`，切回可执行即恢复）。
  - `internal/i18n`：`LocalToolDescription` 改成「Description (optional)」/「描述（可选）」，新增
    `LocalToolDescriptionPlaceholder`（「Leave blank to show no description」/「留空则不显示描述」）。
- **测试**：`internal/extensions/custom_test.go` 的 `TestCustomScriptDescriptionIsInternal`（脚本 manifest +
  descriptor 都是内部句子）与 `TestCustomExecutableDescriptionStaysHonest`（trim 后保留 / 纯空白 → `""`）；
  `internal/settingsui/view_test.go` 的 `TestLocalToolDescriptionIsExecutableOnly`（可执行画 label、脚本不画，
  并钉住两语言的 placeholder 原文——headless tester 不把 placeholder 记进 text 树，见 mygo
  `ui/editor/paint.go:228`）；`internal/shell/scripttool_test.go` 的 `TestConnectAScriptTool` 加一条「脚本
  描述读回为空」的断言。
- **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 29 包全绿（912 项）、
  `GOOS=linux/windows/darwin go build ./...` 三平台通过。

**P7-bt Windows 冒烟作业（原版 `ad843f3`）（上一轮）**

- **依赖检查**：GitHub releases API 仍被限流，改用 go proxy（`go list -m -versions github.com/egoist/mygo`）
  确认 v0.4.8 仍是最新（`go.mod` 已是 v0.4.8），无版本可升。
- **入口**：P7-br 重写的 4 条具体待办里第 1、2、4 条已落地，只剩第 3 条（Windows 冒烟作业）。它之所以
  留到最后，是因为验收要「真跑一次」，而此前各轮只能改仓库、不能动 CI；本轮 `gh` 已认证（`vst93`），
  所以除了写文件，还能真的 dispatch 并等人看截图。
- **原版对照**：`main` 的 `.github/workflows/windows-smoke.yml`（v0.3.14，67 行）只有 `workflow_dispatch`，
  构建 exe、用应用自己的揭示方式启动它、PowerShell 截屏并 `upload-artifact`，文件头注释写明「This is
  deliberately NOT part of the release chain」以及「compiled a hundred times and executed zero」。
- **本构建（改前）**：`.github/workflows/` 只有 `go.yml`；它的 test 矩阵在 windows-latest 上跑
gofmt/vet/build/test，package 矩阵在 Windows 上只上传 `dist/*/*.exe` 就结束——**从不启动那个 exe**。
- **改动**：
  - 新增 `.github/workflows/windows-smoke.yml`：`on: workflow_dispatch: {}`、`permissions: contents: read`、
    `runs-on: windows-latest`、`timeout-minutes: 40`。步骤 = checkout + setup-go（`go-version-file`）→
    `go tool mygo build -platform windows/amd64` → pwsh 里挑出非 Setup 的 exe、置 `FLOTER_OPEN=settings`、
    `Start-Process`、等 20s、用 `dist\windows-amd64` 路径确认进程还活着 → `CopyFromScreen` 存
    `smoke-desktop.png` → `Stop-Process` → `upload-artifact`（`if-no-files-found: error`）。
  - 新增 `internal/ci/workflows_test.go`（test-only 包）：`TestWindowsSmokeIsDispatchOnly` 钉住只有
    `workflow_dispatch`（无 `push:`/`pull_request:`/`release:`/`schedule:`）、构建 `windows/amd64`、
    `CopyFromScreen` + `smoke-desktop.png` + `upload-artifact` 三件套都在；
    `TestWindowsSmokeStaysOutOfTheReleaseChain` 钉住 `go.yml` 不引用 `windows-smoke`。
- **验证**（已满足）：`gh workflow run windows-smoke.yml --ref mygo-rewrite` 真跑一次，run
  [`38349260181`](https://github.com/vst93/floter/actions/runs/38349260181) 在 2m14s 内 `smoke` 全绿
  （含 `Launch, reveal, and capture the desktop`），artifact `smoke-desktop` 下载后是 1024×768 PNG：
  27 503 种颜色、亮度均值 147.3 / 标准差 94.3，16×12 网格里中间是一个亮面矩形（~220-234）被暗边框
  （~30-50）围着、底行是任务栏——是一张真桌面，不是全黑/全白。
- **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 29 包全绿（909 项）、
  `GOOS=linux/windows/darwin go build ./...` 三平台通过。

**P7-bs 提高对比度接进主题与玻璃（原版 `base.css:1387-1574`）（上一轮）**

- **依赖检查**：GitHub releases API 仍被限流，改用 go proxy（`go list -m -versions github.com/egoist/mygo`）
  确认 v0.4.8 仍是最新（`go.mod` 已是 v0.4.8），无版本可升。
- **入口**：P7-br 重写的 4 条具体待办里第 1、2 条已落地，第 3 条（Windows 冒烟作业）要真机跑一次才算验收，
  第 4 条是纯代码可本机验证，本轮取第 4 条。
- **原版对照**：`main` 的 `src/styles/base.css:1387-1427` 是第一个 `@media (prefers-contrast: more)` 块：
  `:root` 与三个效果 stop 各把 `--glass-frame-floor: 0.86`、`--glass-step-dim: 0`、`--glass-lens-scale: 0.6`
  覆写一遍（注释写「Increase Contrast has to mean the material, not only the borders … a user who has asked
  the system for contrast must not have to also go find the slider」）；同块把 `.collapsed-card` 等换成
  `--stroke-contrast`、`.settings-option`/`.settings-section__card` 换成 `--stroke-contrast-soft`、
  `.collapsed-card__input::placeholder` 换成 `--text-placeholder-contrast`；`:1576-1587` 是第二个块
  （Windows 三张面壳的描边），`:386-388`/`:753-756`/`:1004-1006` 是 token 的取值与理由。
- **本构建（改前）**：`grep -rn 'HighContrast' internal cmd` 为零；`internal/theme/theme.go:212-227` 用
  base.css 的普通值整套覆写框架主题的 `Border`/`Text`/`Surface` 等，恰好把框架自己的高对比度加固
  （`ui/preferences.go` 的 `follow`：`Border.Mix(Text, .45)` 等）覆盖掉；`glassmap.SpecFor` 只有
  `SolidTop = 0.98` 上限、没有下限。
- **改动**：
  - `internal/theme/theme.go`：新增 `ForPreferences(s, systemDark, ui.Preferences)`，`For` 变成它的
    无偏好包装（供 `control` 等无 context 的 headless 渲染用）；新增 `palette.contrast(appearance)`，
    按外观换掉三个 token——`stroke` ← `--stroke-contrast`（浅 0,0,0,.5 / 深 255,255,255,.42）、
    `controlEdge` ← `--stroke-contrast-soft`（浅 .36 / 深 .34）、`textMuted` ← `--text-placeholder-contrast`
    （浅 28,28,32,.68 / 深 242,243,245,.58）；玻璃 spec 改用 `glassmap.SpecForPreferences`。
  - `internal/glassmap/glassmap.go`：新增 `ContrastFrameFloor = 0.86`、`ContrastLensScale = 0.6`、
    `Spec.Contrast()`（frame alpha 取 `max(0.86, min(SolidTop, alpha))`、`HazeAlpha = 0`、`LensScale ×= 0.6`，
    **blur/saturate/style 不动**，与原版「The blur is left alone」一致；`!Translucent` 的初始值原样返回）与
    布尔包装 `SpecForPreferences`。
  - 绘制点：`launcher` 11 处 `theme.For`、`surface.go` 的 `Resolve`（新增 `prefs` 参数）、`settingsui.rows`、
    `terminalui` 3 处改 `ForPreferences(..., c.Preferences())`；`shell.View` 的终端框从
    `glassmap.SpecFor(s.GlassStep, s.TerminalOpacity)` 改 `SpecForPreferences(..., c.Preferences().HighContrast)`；
    三处 `launcher.Resolve`（`shell.go`/`pinned.go`/`detach.go`）传入 `c.Preferences()`。
- **诚实记录（框架缺口）**：`--text-placeholder-contrast` 没有独立的 `ui.Theme` 字段——框架把输入框占位符
  画在 `editor.Theme.LineNumber = TextMuted.Alpha(0.75)`（mygo `ui/editor/theme.go:61`），所以这个 token 只能
  落在 `TextMuted` 上；这是框架能接到的唯一处，也顺手把安静文案（toast/notice 等）抬了一级，正是原版
  `.app-toast, .settings-notice { color: var(--text-primary) }` 的意图。
- **测试/验收**：`internal/glassmap/glassmap_test.go` 的 `TestContrastRaisesTheFloorDropsHazeAndDampsTheLens`
  （四个 stop：floor ≥ 0.86 且 ≤ solid top、haze = 0、lens = 0.6×、blur/saturate/style 逐字段不变）；
  `TestContrastFloorNeverLowersATransparentFrame`（100% 时仍取 `min(solid_top, opacity)`，floor 不下拉；
  无偏好时逐字节等于 `SpecFor`）；`internal/theme/theme_test.go` 的
  `TestHighContrastSwapsTheStrokeAndPlaceholderTokens`（两个外观的十个精确值：三个 token + 框架 `Border` +
  其余 token 逐字段不变（涂掉对比度/materials 相关字段后结构体相等）+ 内容面与浮动面跟随抬升的 frame）。
  mutation 复现红：把 `Spec.Contrast()` 改成 no-op → `go test ./internal/glassmap ./internal/theme` 22 行失败。
- **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 全绿（907 项）、
  `GOOS=linux/windows/darwin go build ./...` 三平台通过。

**P7-br 分页页脚三态与减少动效（R29 / R8+R161）（上一轮）**

- **依赖检查**：GitHub releases API 仍被限流，改用 go proxy（`go list -m -versions github.com/egoist/mygo`）
  确认 v0.4.8 仍是最新（`go.mod` 已是 v0.4.8），无版本可升。
- **入口**：P7-bp 重写的 3 条具体待办全部落地（第 1 条 P7-bp，第 2、3 条 P7-bq）后清单又空；本轮先把它
  重写为 4 条带 file:line 的具体待办（下方「具体待办（P7-br 重写）」），再取第 1、2 条实现。
- **改动（第 1 条，页脚三态）**：原版 `src/launcher/plugin-mode.ts:613-626` 的 `pluginFooterState` 是纯函数
  （`null` / `"more"` / `"loading"` / `"end"`），`src/launcher/LauncherResults.tsx:1024-1040` 只给
  `loading` 画 spinner、给 `loading` 与 `end` 画字，`src/styles/launcher.css:1676-1705` 写着「只有两个状态带字：
  `more` 是空的——滚动本身就是提示」。本构建 `internal/launcher/output.go` 的 `outputFooter`（旧 `:221-243`）
  相反：默认文案是原版没有的 “Scroll for more”，`loading` 只有字没有环。现在抽出同构的纯函数
  `outputFooterState`（`footerNone`/`footerMore`/`footerLoading`/`footerEnd`）；`more` 只留行高
  （`MinHeight(Space(5.5))` = 原版 `min-height: calc(var(--u) * 22)`）；`loading` 在字左侧画环：12px 方框上
  `Path.Circle` 描 `Hairline` 一圈，再描 `Path.Circle` + `Path.Trim(0, 0.25)` 的 `TextMuted` 四分之一弧
  （即原版的 `border: 1.5px solid var(--hairline); border-top-color: var(--text-muted)`），`ui.Linear` 0.7s
  一圈；字号/间距/居中照原版（`--text-caption`、`gap 8`、`padding 0 12`）。i18n 删掉原版没有的 `OutputMore`
  （en「Scroll for more」/zh「滚动加载更多」，`deadcopy_test.go` 的 `TestNoDeadCopy` 会盯着），zh 的
  `OutputEnd` 从「没有更多了」回到原版的「没有更多结果」。
- **改动（第 2 条，两个环境循环遵守减少动效）**：原版 `src/styles/base.css:1305-1362` 的
  `prefers-reduced-motion` 命名块（`base.css:1305-1362`）把两个**环境循环**点名关掉（`animation: none`）：
  `.launcher-plugin-footer__spinner` 与 `.terminal-bar__dot`（后者的 keyframes `terminal.css:177-187` 是
  `0%/100% opacity 1 scale 1`、`50% opacity 0.5 scale 0.88`，所以静止态就是全不透明、满尺寸）；R161
  （`base.css:1364-1385`）又补了通配块（`animation-iteration-count: 1`）。框架的 `Loop` 明确不跟这个偏好——
  `ui/preferences.go` 写着「`Loop`, which shows that something is going on, goes on」——所以 App 自己读。新增
  `internal/motion`：`Value(c, phase)` 在减少动效时回静止值 0 且**从不调用** `phase`（注册过的 loop 会一直要
  帧，读了不用等于没接）。终端状态点改用它，并回到原版 keyframes：`Opacity(1 - 0.5*breath)` +
  `Scale(1-0.12*breath)`（此前是 `0.75 + 0.25*loop` 且不缩放）；页脚圆环同样经它取相位（减少动效时环静止）。
- **测试/验收**：`internal/motion` 的 `TestValueRestsUnderReduceMotion`（静止值 + 不询问相位）；
  `TestOutputFooterStates`（四个状态一张表）；`TestOutputFooterLoadingDrawsTheRing`（loading 画字**且**字左侧
  有环的墨，环的存在从渲染帧的像素上读，不是看内部标志）；`TestStatusDotRestsUnderReduceMotion`（同一时刻
  （循环 400ms 处）：减少动效是 7×7 的纯 accent 静止点，允许动效则是混色缩小的点）。两个 mutation 复现红：
  `motion.Value` 忽略偏好 → 终端测试报「the resting dot draws no accent pixels」；`motion.Value` 永远返回 0 →
  终端测试报「the breath never ran」；`outputFooter` 不画环 → 页脚测试报「the ring is missing」。
- **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 全绿（902 项）、
  `GOOS=linux/windows/darwin go build ./...` 三平台通过。

**P7-bq 版本兜底提示与逐行选项（R153 剩余两条）（上一轮）**

- **依赖检查**：GitHub releases API 仍被限流，改用 go proxy（`go list -m -versions github.com/egoist/mygo`）
  确认 v0.4.8 仍是最新（`go.mod` 已是 v0.4.8），无版本可升。
- **入口**：P7-bp 重写的 3 条具体待办，第 1 条（P7-bp）已落地，本轮取剩余第 2、3 条（都是 R153）。
- **改动（第 2 条，版本兜底提示）**：原版 `main` 的 `src/extensions/CustomIntegrationDrawer.tsx:133-135`
  在版本输入下印一条 `extension-custom-form__hint`（文案 `src/i18n.ts:685` / zh `:1565`）：
  “A successful version probe overwrites this value; if the probe fails, this is the fallback.”——
  一个可编辑的版本框挣要说清自己是什么。本构建 `internal/settingsui/settingsui.go` 的版本行
  （原 `:2302`）只有占位符 `0.0.1`。现在补 `LocalToolVersionHint`（en/zh，原文照搬），版本行的
  `a.row` 第二个参数（hint）接上它，与表单里其他提示同一套画法。
- **改动（第 3 条，选项「一行一个」）**：原版 `src/extensions/script-params.ts:92-107` 的
  `parseParamOptions` 用 `split("\n")`（trim、去空行）、`formatParamOptions` 用 `join("\n")`，理由写在
  注释里：一个选项值本身可能含逗号（`1,000`），逗号分隔会在下一次往返时把它拆成两个。本构建
  `settingsui.go` 的 `editorParams`（原 `:3140` `strings.Split(param.Options, ",")`）、`beginEdit`
  （原 `:3206` `strings.Join(param.Options, ", ")`）与占位符（原 `:3083` `"fast, slow"`）都还按逗号。
  现在：拆分改成 `"\n"`、写回改成 `"\n"`；输入控件由单行 `ui.TextInput` 换成多行 `ui.TextArea`
  （对齐原版的 `<textarea rows={3}>`），占位符 `"fast\nslow"`；`ParamsOptionsHint` 从
  “Separated by commas.”/“用逗号分隔。” 改成 “One per line.”/“每行一个。”。磁盘格式不变
  （线上仍是字符串数组）。
- **测试/验收**：新增 `TestLocalToolVersionHint`（表单未开时 hint 不在、打开后印出）、
  `TestParamOptionsNewlineCodec`（含逗号的选项 `["1,000", "1,000,000"]` 经 `beginEdit`/`editorParams`
  一次往返后仍是两个值）；`TestConnectFormNamesAnUnfinishedInputAndRefusesToSave` 里的
  `"fast, slow"` 改成 `"fast\nslow"`。`grep -rn 'CustomVersionHint' internal --include=*.go` 不再是零，
  `grep -rn 'Separated by commas\|fast, slow' internal --include=*.go` 为零。
- **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 27 包全绿、
  `GOOS=linux/windows/darwin go build ./...` 三平台通过。

**P7-bp 本地工具权限收进高级披露（R161）（上一轮）**

- **依赖检查**：GitHub releases API 被限流，改用 go proxy（`go list -m -versions github.com/egoist/mygo`）
  确认 v0.4.8 仍是最新（`go.mod` 已是 v0.4.8），无版本可升。
- **入口**：P7-bm 的 4 条具体待办全部落地（第 1 条 P7-bm、第 2 条 P7-bn、第 3·4 条 P7-bo）后清单又空；
  本轮先把它重新写实为 3 条带 file:line 的具体待办（见「具体待办（P7-bp 重写）」），再取第 1 条实现。
- **偏离点**：原版最近一轮（`main` 的 v0.3.14，commit 4b2c168，R161）把自定义集成编辑器里两个
  **只做声明/审计**的权限字段集连同它们的边界提示一起收进一个默认折叠的 `<details>`「Advanced ·
  Permissions & audit」。理由写在自己的注释里：这两个字段集的提示就承认「不提供操作系统隔离」，画成与
  宿主真正读取的字段同权，会让一个全新的集成一打开就是一张仪式表单；折叠后不卸装，勾选仍住在同一份
  表单状态里，由一个 author 自己展开编辑。本构建的 `localToolSection`（`internal/settingsui/settingsui.go`）
  以前在卡片里逐条画 `extensions.AllPermissions` 的开关行，且没有那条 `permissionBoundary` 边界提示。
- **修法**：
  - `internal/settingsui/settingsui.go`：新增 `App.toolAdvanced bool`（默认 false，`beginEdit`/`endEdit`
    都复位）；`localToolSection` 的权限开关改成一个「Advanced · Permissions & audit」的折叠行
    （与卡片里 config 折叠同一套 `a.row` + `below` 写法），新增 `toolPermissionFields` 助手：只在
    `toolAdvanced` 为真时画边界 note 与开关行。勾选仍写 `a.toolPermissions`，提交时照读——折叠不丢状态。
  - `internal/i18n/i18n.go` + `copy.go`：`LocalToolAdvanced`（“Advanced”/“高级”）、
    `LocalToolAdvancedPermissions`（“Permissions & audit”/“权限声明与审计”）、`PermissionBoundary`
    （原版 `src/i18n.ts:783` 的原文，en/zh）。
  - `internal/settingsui/view_test.go`：`TestLocalToolForm` 改为「先验默认折叠、展开后再验开关行」；
    新增 `TestLocalToolAdvancedDisclosure` 钉住默认折叠、展开后边界 note + 开关行、折叠后勾选仍在
    状态且能提交。
- **测试/验收**：`grep -rn 'CustomAdvanced' internal --include=*.go` 不再是零（已接线）；
  `internal/settingsui`、`internal/i18n` 测试全绿。两条关键断言直接钉住：开关行与边界 note
  在 `toolAdvanced` 为假时不被绘制（默认折叠），以及展开时勾选后折叠、提交仍能带上该权限。
- **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 27 包全绿
  （894 项）、`GOOS=linux/windows/darwin go build ./...` 三平台通过。

**P7-bo 删掉两处经核实无用的死代码（browser profile key + CommandInfo.ExtensionName）（上一轮）**

- **依赖检查**：GitHub releases API 被限流，改用 go proxy（`go list -m -versions github.com/egoist/mygo`）
  确认 v0.4.8 仍是最新（`go.mod` 已是 v0.4.8），无版本可升。
- **入口**：P7-bm 重写的 4 条具体待办，第 1 条（P7-bm）、第 2 条（P7-bn）已落地，本轮取第 3、4 条
  （两处都是「写而不读」的字段/函数，量小，一并清完）。
- **改动（第 3 条）**：
  - `internal/browser/browser.go`：删 `Result` 结构体里的 `ProfileKey string` 字段及其注释；随之删去
    history 读取（原 `:447`）、Chromium 书签（原 `:616`）、Firefox 书签（原 `:650`）三处
    `ProfileKey: profile.Key()` 赋值。
  - `internal/browser/safari.go`：删 Safari 书签（原 `:61`）的 `ProfileKey: profile.Key()` 赋值。
  - `internal/browser/target.go`：删 `FindProfile`（原 `:48-57`，全仓仅自身测试调用）。
  - `internal/browser/safari_test.go:33`：从断言里去掉 `result.ProfileKey != "safari/Safari"`。
  - `internal/browser/browser_test.go:236-242`：删 `FindProfile` 测试块。
  - **为何删除而非接线**：原版 `browser_open_url`（`src-tauri/src/browser_data/mod.rs:354`）用 `profile_key`
    反解 browser id，本构建改成 `Result.BrowserID` 直接开（`internal/launcher/search.go` 的 `openResult`
    把它交给 `browser.OpenURL`），行为等价；`OpenURL` 只按 browser id 选 App，profile key 从不参与，所以
    这份字段/函数在本构建里确为死代码。`Profile.Key()` 本身仍被测试与 target 解析使用，未删。
- **改动（第 4 条）**：`internal/extensions/store.go`：删 `CommandInfo.ExtensionName` 字段与
  `CommandRegistry` 里（原 `:242`）的 `ExtensionName: integration.Name` 赋值；设置面的集成卡片用
  `Integration.Name`（`settingsui.go:1267` 等），命令开关表只按 `ExtensionID`/`CommandID` 键控，从不读它。
- **测试/验收**：`grep -rn 'ProfileKey\|FindProfile\|ExtensionName' internal cmd --include=*.go` 为零；
  `internal/browser`、`internal/extensions`、`internal/settingsui`、`internal/shell` 的现有测试全绿。
- **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 27 包全绿、
  `GOOS=linux/windows/darwin go build ./...` 三平台通过。

**P7-bn 删除双方共有的死代码（capability 报告探针）（上一轮）**

- **依赖检查**：GitHub releases API 被限流，改用 go proxy（`@v/list` / `@latest`）确认 v0.4.8 仍是
  最新（`go.mod` 已是 v0.4.8），无版本可升。
- **入口**：P7-bm 重写的 4 条具体待办，第 1 条已落地，本轮取第 2 条。
- **决策（删除而非接线）**：原版 `src-tauri/src/extensions/capability_probe.rs` 的
  `CapabilityScanner::scan`/`scan_with` 只被自身测试调用（`git grep CapabilityScanner main` 无生产
  调用者），reprobe 走的是 `probe_executor.rs:17` 的 `HealthReport::new(Default::default())`，所以原版
  卡片（`ExtensionsPanel.tsx:2924/2926/2934`）里 `capabilities.version` 恒 falsy、features/limitations
  恒空——**渲染代码在，数据永远不来**。本构建改前有同一份死代码：`ProbeCapabilities`/`CapabilityReport`/
  `CapabilityProbe`/`ProbeResult`（`internal/extensions/capability.go`）只有 `capability_test.go` 调用
  （`grep -rn 'ProbeCapabilities' internal cmd | grep -v _test` 为空）。接线需要给 `HealthReport` 加
  `capabilities` 字段（改磁盘格式）并让 `Reprobe` 跑探针，那会**偏离原版行为**；所以本轮按原版的
  有效行为选择删除。
- **改动**：
  - `git mv internal/extensions/capability.go internal/extensions/helpderive.go`：该文件除死探针外还
    装着**活着**的 `ProbeHelpText`/`ProbeDerive`（连接时 `--help` 派生，`reprobe.go:198`、`custom.go:493`
    在用）与 `SemverFromVersionOutput`，所以只做定点手术，不删文件。
  - 删除死符号：常量 `ProbeVersion`/`ProbeHelp`/`UnknownVersion`/`probeTimeout`（以及同样无人用的
    `subcommandProbeTimeout`）、类型 `CapabilityProbe`/`ProbeResult`/`CapabilityReport`、函数
    `ProbeCapabilities`/`capabilityProbes`/`(*CapabilityProbe).Run`/`firstLine`/`capIntPtr`；随之
    清掉 `errors`/`fmt` 两个不再用到的 import。
  - 删除 `internal/extensions/capability_test.go`（四个测试全部只测 `ProbeCapabilities`；`ProbeHelpText`/
    `ProbeDerive` 的测试在 `helpargs_test.go`，不受影响）。
  - 文件头补一段说明，记下这份死代码的来历与为何删除。
- **测试/验收**：`grep -rn '\bCapabilityReport\b\|\bCapabilityProbe\b\|\bProbeResult\b\|\bProbeCapabilities\b\|\bProbeVersion\b\|\bProbeHelp\b\|\bUnknownVersion\b\|\bprobeTimeout\b\|\bsubcommandProbeTimeout\b\|\bfirstLine\b\|\bcapIntPtr\b' internal cmd --include=*.go`
  为零；`ProbeHelpText`/`ProbeDerive` 的现有测试（`helpargs_test.go`）仍绿。
- **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 27 包全绿、
  `GOOS=linux/windows/darwin go build ./...` 三平台通过。

**P7-bm 健康报告的探针明细与检查时间（上一轮）**

- **依赖检查**：GitHub releases API 被限流，改用 `go list -m -versions github.com/egoist/mygo` 确认
  v0.4.8 仍是最新（`go.mod` 已是 v0.4.8），无版本可升。
- **入口**：「尚未做」清单为空（P7-bh 的 5 条全部落地，其余只剩卡框架项），本轮先把清单重写为 4 条
  带 file:line 的具体待办（见下方「具体待办（P7-bm 重写）」），再取第 1 条实现。
- **偏离点（报告算完就丢）**：原版 `extension-health`（`ExtensionsPanel.tsx:2922-2964`）把
  `healthReport.probes` 逐条画出来——通过/失败记号 + `probeId` + `{durationMs}ms`；失败项另列
  `(exit: {code})` + stderr + `healthRetryable`；末尾 `healthCheckedAt`。本构建的集成卡片只把报告
  的 `Status` 映射成一句本地化状态词（`shell.go:1306` 的 `integration.Health`），而 `ProbeRecord`
  的 `DurationMs`/`ExitCode`/`Stderr`（`internal/extensions/probes.go:38-50`）与 `HealthReport.CheckedAt`
  （`probes.go:51`）**没有任何视图读取**——报告进了仓库、也读了回来，就是没人画。
- **修法**：
  - `internal/settingsui` 新增 `HealthReport`/`HealthProbe` 视图与 `Integration.HealthReport`；
    `healthDetail` 在 Health 行下画三块——`Checks`（每条探针一行：右侧记号 + `{ms}ms`）、`Failures`
    （每条失败一行：`exit: {code}` + stderr + `Retryable`）、以及 `Checked {time}` 时间戳。
    `healthDuration`/`healthExitCode`/`healthCheckedAt` 三个纯助手（退出码 `-1` 画 `-`，时间戳按
    RFC 3339 解析成 `2006-01-02 15:04` 本地时间，解析不了就原样返回）。
  - `internal/shell` 新增 `healthReportView`：把仓库里的 `ProbeReport` 投影成视图，`Retryable` 取
    清单 `Lifecycle.Probes` 的 `required` 取反（**不落盘**，磁盘格式与原版一致——原版的 `retryable`
    也只在失败记录里、不写进 `ProbeRecord`）。
  - `internal/i18n` 补五句（`IntegrationsHealthChecks`/`Failures`/`Retryable`/
    `HealthExitCode(code)`/`HealthCheckedAt(time)`，en/zh 用原版 `healthProbes`/`healthFailures`/
    `healthRetryable`/`healthExitCode`/`healthCheckedAt` 的原文）。
- **测试**：`settingsui/view_test.go` 的 `TestStoredHealthReportShowsItsProbes`（探针 id、耗时、
  `Checks`/`Failures`/`exit: 3`/stderr/`Retryable`/`Checked …` 全部画出来；没有报告时一行明细都不画）；
  `shell_test.go` 的 `TestHealthShowsOnTheRow` 扩写——夹具的清单加一条 optional 探针，核对报告到达卡片、
  通过项的耗时与时间戳在、`Retryable` 取 `required` 的反（required 失败不可重试、optional 可重试）。
  变异核查：把 `integrationList` 里的 `item.HealthReport = ...` 删掉，settingsui 与 shell 两条立刻红；
  把 `Retryable: !required[...]` 写成 `required[...]`，`TestHealthShowsOnTheRow` 两条断言翻转。
- **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 27 包全绿
  （894 项）、`GOOS=linux/windows/darwin go build ./...` 三平台通过。

**P7-bl 清单的严格解析与组合规则（上一轮）**

- **依赖检查**：`curl .../releases/latest` 返回 v0.4.8，`go.mod` 已是 v0.4.8，无版本可升。
- **入口**：P7-bh 重写的 5 条具体待办，第 1–4 条已落地，本轮取第 5 条（清单里最后一条）。
- **状态核对（第 5 条已落地一半）**：严格解析（`decodeManifest` 的 `DisallowUnknownFields` + 尾随
  数据拒绝）与组合规则（`ValidateCombination`）已由 8f99926 落地（那次提交的说明只写了玻璃与
  super+n）。但原版 `validate_paths`（`manifest.rs:565`）在解析期还调用 `validate_param_definitions`
  （`manifest.rs:145`），本构建的 `ValidateParams` 只在安装期（`install.go:268`）与连接表单
  （`custom.go:278`）跑，`LoadManifest` 从不查；且 `ValidateParams` 缺 default 的逐 kind 校验
  （number/boolean/select）。这两处是本轮补的。
- **偏离点（参数声明不在解析期校验、default 不查 kind）**：原版 `validate_param_definitions` 在
  `validate_paths` 里与路径规则一起跑，所以一个坏声明根本进不了安装/导出/运行；它还逐 kind 校验
  `default`（number 用 `parse::<f64>()`、boolean 必须 `true|false`、select 必须在选项里）。本构建
  改前 `Manifest.Validate()` 只跑组合 + 路径 + lifecycle，参数声明要等到 `install.go` 才查；
  `ValidateParams` 也不看 default。
- **修法**：
  - `internal/extensions/params.go`：`ValidateParams` 补 default 逐 kind 校验——number 用运行期的
    `IsFiniteDecimal`（刻意比原版的 `parse::<f64>()` 严：原版放过 `inf`/`NaN`，运行期又拒绝，本构建
    让声明期就拒绝，一个声明不可能带运行不了的值）、boolean 必须 `true|false`、select 必须在选项里。
  - `internal/extensions/validate.go`：`Manifest.Validate()` 在组合规则之后、路径规则之前调用
    `ValidateParams(m.Params)`，把 `validate_paths` 的 `validate_param_definitions` 那半接回解析期。
- **测试**：`validate_test.go` 的 `TestManifestValidateCombination`（四种 local 配对 + 一种 built-in
  合法；managed/linked/未知 provider/built-in script/built-in executable/缺 descriptor 六种非法）、
  `TestLoadManifestRejectsUnknownFieldsAndTrailingData`（未知顶层键、未知嵌套键、尾随数据）、
  `TestLoadManifestRejectsABadParamDeclaration`（空 id/重复 id/select 无选项/boolean 无 flag/坏 number
  default/坏 select default 都在 `LoadManifest` 就被 `ErrBadParam` 拒绝）；`params_test.go` 的
  `TestValidateParamsDefaults`（number/boolean/select 的合法与非法 default，含 `inf` 被拒）。
  变异核查：拿掉 `Validate()` 里的 `ValidateParams` 调用，端到端那条立刻红（6 个坏声明全部 `<nil>`）；
  拿掉 `DisallowUnknownFields`，未知字段两条立刻红；把 number default 检查改成 `false`，
  `abc`/`inf` 两条立刻红。
- **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 27 包全绿
  （901 项）、`GOOS=linux/windows/darwin go build ./...` 三平台通过。

**P7-bk 运行表单的逐参数本地化错误（上一轮）**

- **依赖检查**：GitHub API 被限流，改用 `go list -m -versions github.com/egoist/mygo` 确认 v0.4.8
  仍是最新（`go.mod` 已是 v0.4.8），无版本可升。
- **入口**：P7-bh 重写的 5 条具体待办，第 1、2、3 条已落地，本轮取第 4 条（同一套 `run_param_*`
  键的另一半：表单自己怎么判、句子印在哪）。
- **偏离点（表单预检比后端弱、句子不落在出错行）**：原版 `src/extensions/run-params.ts:72` 的
  `paramValueIssues` 逐参数返回 `{index,key,label}`：required 且无值**且无 default** 才算缺；number
  用 `isFiniteDecimal`；select 必须在选项里；boolean 必须是 `true|false`。`RunParamForm.tsx:39` 每帧
  重算，`issueByIndex` 把句子印在**出错输入下面**，并 `disabled={!canRun}`。本构建
  `internal/extensions/params.go` 的 `ParamIssues` 只回 `[]string` 的 id、number 用
  `strconv.ParseFloat`（放过非有限小数与 radix 前缀）、boolean 不查、required 不认 default 豁免；
  `internal/settingsui/settingsui.go:2725` 把非空结果一律换成 `copy.RunIncomplete` 印在按钮下。
  另：`internal/launcher/runfailure.go` 对 `run_param_*` 原样返回，输出视图不本地化。
- **修法**：
  - `internal/extensions/params.go`：新增 `ParamIssueKey`（`required`/`invalid`）与
    `ParamValueIssue{Index,Key,Label}`；`ParamIssues` 返回它，补 required-有-default 豁免、
    boolean 校验，number 改用 `IsFiniteDecimal`（与后端 `ParamArguments` 同一个文法）；label 空白回 id。
  - `internal/settingsui/settingsui.go`：`runForm` 每帧算 `ParamIssues`，`issueByIndex` 让每一行用
    `a.row(..., below...)` 在行内印本地化句子（`paramValueIssueMessage`：required→`RunParamRequired`，
    invalid→`RunParamInvalid`）；有错时 `ui.Button(...).Disabled(true)`，并保留点击守卫。
  - `internal/launcher/runfailure.go`/`output.go`：`failureReason` 在 `ParseRunFailure` 之后认
    `ParseParamFailure`，`paramRunMessage` 用 id 作 label（启动器手里没有声明，正是原版
    `paramRunErrorMessage` 找不到参数时的回退）。
  - i18n 删除已死的 `RunIncomplete`（en/zh）；`RunParam*` 四句 P7-bj 已有，复用。
- **测试**：`params_test.go` 的 `TestParamIssues`（结构化 Index/Key/Label、required/invalid/select/
  boolean）、`TestParamIssuesRefusesWhatTheBackendRefuses`（`inf`/`NaN`/`Infinity`/`0x1p-2`/`1_000`
  拒绝，`2`/`-1.5`/`.5`/`1e3`/带空白通过）、`TestARequiredDefaultIsNotMissing`；
  `settingsui/view_test.go` 的 `TestRunParamForm`（改用行内 `Required: fill in Target before running`、
  禁用时点 Run 不触发）与新增 `TestRunParamFormShowsPerRowReasons`（number/select 各自行内句子、
  修好后消失、出错时 Run 不跑）；`launcher/runfailure_test.go` 的
  `TestRunParamFailureIsLocalizedInTheOutputView`（en/zh、键不外泄）。变异核查：删掉 boolean 分支，
  `TestParamIssues` 立刻红（`issues = [{0 required Target} {1 invalid count} {2 invalid mode}]`）；
  把行内错误渲染改成 `&& false`，两条 settingsui 表单测试立刻红。
- **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 27 包全绿、
  `GOOS=linux/windows/darwin go build ./...` 三平台通过。

> **上一轮：P7-bj 运行期参数校验与 argv 权威（`param_arguments`）** —— P7-bh 重写的具体待办第 3 条。
> 原来 `ParamArgv` 只把答案拼成 argv：未声明的 id 被静默丢掉、required 不查、number/select/boolean
> 不校验、`inf`/`NaN` 照收、`.cmd`/`.bat` 的 cmd.exe 元字符也直接传下去。现在只有
> `ParamArguments` 一个 argv 构建点，非法组合发 `run_param_*` 键，面板按语言把句子印在按钮下。

**P7-bj 运行期参数校验与 argv 权威（本轮）**

- **依赖检查**：GitHub API 被限流，改用 `go list -m -versions github.com/egoist/mygo` 确认 v0.4.8
  仍是最新（`go.mod` 已是 v0.4.8），无版本可升。
- **入口**：P7-bh 重写的 5 条具体待办，第 1、2 条已落地，本轮取第 3 条（`param_arguments`）。
- **偏离点（argv 构建点从不拒绝）**：原版 `src-tauri/src/extensions/run.rs:295` 的 `param_arguments`
  是**唯一**的 argv 构建点，并在运行前重新校验：未声明的 id → `run_param_unknown:{id}`；required 且
  无值无默认 → `run_param_required:{id}`；flag 非法 / number 非有限小数 / select 不在选项 / boolean
  不是 `true|false` → `run_param_invalid:{id}`；`.cmd`/`.bat`（经 cmd.exe）且值含 `&|<>^%`
  （`run.rs:231` 的 `WINDOWS_CMD_UNSAFE_CHARS`）→ `run_param_windows_unsafe:{id}`；缺值回填 `default`。
  本构建 `internal/extensions/params.go:163` 的 `ParamArgv` 静默丢未知 id、不查 required、不校验
  number/select/boolean、不回填 default、无 windows-unsafe；`shell.go:1787` 直接把它交给运行。
- **修法**：新文件 `internal/extensions/paramrun.go`——`ParamFailure{Key,Param}` 与四个键、`Error()`
  发 `key:{id}`、`ParseParamFailure`（只认四个键、id 非空）、`ParamArguments(params, values, cmdScript)`
  复刻全部规则（未知键按排序确定；缺值回填 default；按 kind 转换；flag 与 value 各自成项）、
  `IsFiniteDecimal`（复刻原版前端 `FINITE_DECIMAL_PATTERN`，拒绝 `inf`/`NaN`/`0x1p-2`/`1_000`）、
  `WindowsCmdUnsafeChar`、`IsCmdScriptName`/`ProgramIsCmdScript`。删除 `ParamArgv` 与 `truthy`——
  原版明说「argv 只在一个地方构建」，第二处会漂移。`shell.go` 的 `runWithParams` 改用它，并把
  `RunWithParams` 回调改为返回 `*extensions.ParamFailure`；`settingsui` 新增 `paramFailureMessage`
  （label 优先、缺省回 id）把句子印在按钮下。i18n 补四句 `RunParam*`（en/zh 逐字对齐旧字典 774-777 /
  1645-1648）。
- **测试**：`internal/extensions/paramrun_test.go`——声明顺序与 flag/位置参数、缺省回填、required、
  empty=未给、按 kind 转换、非有限小数（`inf`/`NaN`/`0x1p-2`/`1_000`）、未声明键、坏 flag、
  cmd 元字符（同一值对非 cmd 程序放行）、键往返与保守解析（`run_param_required:note` 不被
  `ParseRunFailure` 认）、`IsFiniteDecimal`/`WindowsCmdUnsafeChar`/`IsCmdScriptName` 表；
  `internal/settingsui/view_test.go` 的 `TestRunParamRefusalIsWorded`（四键 en/zh、label 回退 id、
  键不外泄）与 `TestRunParamFormShowsTheBackendRefusal`（后端拒绝印在表单上）。变异核查：把
  `if cmdScript` 改成 `if false`，cmd 元字符那条立刻红（`accepted map[target:a&b]`）。
- **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 全绿（887 项）、
  `GOOS=linux/windows/darwin go build ./...` 三平台通过。

> **上一轮：P7-bi lifecycle 的结构校验（`Lifecycle::validate`）** —— P7-bh 重写的具体待办第 2 条。
> 原版解析清单时对 lifecycle 有一整套规则（每 shell 一个 completion、source/args 恰有其一、参数与
> 超时上下界、模板 target 与 probe id 唯一、完成文件名安全、launch 的 restorePolicy 闭集与 NUL），
> 本构建只解析不校验，夹具可带任意 timeout 与重复 id 通过。现在 `LoadManifest` 会跑
> `Manifest.Validate()`，坏 lifecycle 根本进不来；`restorePolicy` 字段也补上了（缺省 `reattach`）。

**P7-bi lifecycle 的结构校验（本轮）**

- **依赖检查**：`curl .../releases/latest` 返回 v0.4.8，`go.mod` 已是 v0.4.8，无版本可升。
- **入口**：P7-bh 重写的 5 条具体待办，第 1 条已落地，本轮取第 2 条（`Lifecycle::validate`）。
- **偏离点（lifecycle 只解析不校验）**：原版 `src-tauri/src/extensions/lifecycle.rs:39` 的
  `ToolLifecycle::validate`——每个 shell 至多一个 completion（`:41`）、completion 的 `source`/`args`
  恰有其一（`:52`）、`args ≤ 16`（`:63`）、`timeoutMs ∈ 100..=10000`（`:66`）、`fileName` 合法
  （`:645` 的 `validate_file_name`：非空、≤128 字节、仅 ASCII 字母数字与 `.`/`_`/`-`）、template
  `target` 不重复（`:145`）、probe `id` 不重复（`:173`）、probe `timeoutMs ∈ 100..=30000`、`args ≤ 32`、
  以及 `LaunchConfig::validate`（`:119` 的 `restorePolicy ∈ {reattach,restart,none}` 与
  `LaunchCommand::validate` 的 NUL 检查）。本构建 `internal/extensions/manifest.go` 的
  `ShellCompletion`/`Template`/`Probe`/`Launch` 只有字段，`LoadManifest` 只查 ID/Name + 路径，
  `launch_test.go`/`probes_test.go` 的夹具因此可以带着任意 timeout 与重复 id 通过；`restorePolicy`
  甚至没有字段（`grep -rn RestorePolicy internal` 为空）。
- **修法**：`internal/extensions/validate.go`——新增 `Manifest.Validate()`（own paths → lifecycle）、
  `Lifecycle.Validate()`（原版全部规则，含 shell 闭集）、`Launch.Validate()`（restorePolicy 闭集 +
  program/args 的 NUL），以及 `validateCompletionFileName`；`Manifest.ValidatePaths()` 拆出
  `validateDeclaredPaths` 供两处复用。`internal/extensions/manifest.go`——`Launch` 补
  `RestorePolicy` 字段与 `EffectiveRestorePolicy()`；`ShellCompletion`/`Probe` 补
  `EffectiveTimeoutMs()` 与 serde 缺省（3000 / 2000，原版 `default_completion_timeout` /
  `default_probe_timeout`），`probes.go` 改用它。`LoadManifest` 改调 `m.Validate()`。
- **测试**：`internal/extensions/validate_test.go`——`TestLifecycleValidate`（干净 + 17 条非法：重复/未知
  shell、source+args 同给、两者皆无、args=17、超时 99/10001、坏/超长文件名、重复 target、重复 probe id、
  probe 超时 99/30001、args=33、未知 restorePolicy、program/args 含 NUL；外加边界 100/10000/30000 合法与
  缺省合法）、`TestEffectiveLifecycleDefaults`（3000/2000/reattach）、`TestLoadManifestRefusesABadLifecycle`
  （端到端：干净 probe 可载、重复 probe 与未知 restorePolicy 拒绝）。变异核查：把 `LoadManifest` 的
  `Validate()` 换回 `ValidatePaths()`，端到端那条立刻红。
- **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 全绿、
  `GOOS=linux/windows/darwin go build ./...` 三平台通过。

> **上一轮：P7-bh 清单路径安全（`validate_relative_path`）** —— P7-bg 后 P7-bd 的清单已全部落地，
> 本轮先把「尚未做」重新写实为 5 条带 file:line 的具体待办，再取第 1 条落地：原版在**解析清单时**
> 就拒绝任何绝对/`..`/空路径，本构建只在使用时兜住一部分，导出的 staging 写入完全裸奔。现在
> `LoadManifest` 会校验 `icon`、脚本路径、描述符与 lifecycle 的四处路径，坏包根本进不来。

**P7-bh 清单路径安全（本轮）**

- **依赖检查**：`curl .../releases/latest` 返回 v0.4.8，`go.mod` 已是 v0.4.8，无版本可升。
- **入口**：P7-an/P7-bb/P7-bd 三份清单都已全部落地或只剩一次性维护者动作，所以先把「尚未做」重写成
  5 条具体待办（见「### 具体待办（P7-bh 重写）」），再取第 1 条实现。
- **偏离点（解析期不查路径）**：原版 `src-tauri/src/extensions/manifest.rs:606` 的 `validate_relative_path`
  （非空、非绝对、每个 component 必须是 `Component::Normal`）由 `validate_paths`（`:565`）用在 `icon`、
  `Runtime::Script.path`、`provider.descriptor`，由 `lifecycle.rs:39/145/173` 用在 completion `source`、
  template `source`/`target`、`launch.command.program`，并在 `parse()`（`:492`）反序列化后立刻调用。
  本构建 `internal/extensions/manifest.go:225` 的 `LoadManifest` 只查 ID/Name；`internal/extensions/sync.go:417,427`
  把 `runtime.path`/`provider.descriptor` 直接 `filepath.Join` 进 staging（`..` 可越界写文件）；
  `description.go:139` 的 `joinWithin` 只在读取时兜住描述符/脚本/模板，`icon` 全仓无人查。
- **修法**：新文件 `internal/extensions/validate.go`——`ErrBadManifest`、`ValidateRelativePath(value, field)`
  （拒空/绝对/盘符前缀/`.`/`..`；同时按 `/` 与 `\` 切分，Windows 与 Unix 拼写同一规则；`a//b` 与
  尾随分隔符照原版 `Path::components` 折叠为合法）、`Manifest.ValidatePaths()`（icon / script path /
  descriptor）与 `Lifecycle.ValidatePaths()`（completion source、template source/target、launch program
  非 `self`）。`LoadManifest` 在 ID/Name 之后调用 `ValidatePaths()`。零新 API 面，只加校验。
- **测试**：`internal/extensions/validate_test.go`——`TestValidateRelativePath`（6 条合法含中文与 `a//b`、
  12 条非法含 `C:\…`、`a\..\b`、`./x`）、`TestManifestValidatePaths`（三处字段 + 缺省不报错 + 脚本无路径报错）、
  `TestLifecycleValidatePaths`（四处路径 + `self` 豁免）、`TestLoadManifestRefusesAnEscapingPath`（干净清单仍可载、
  `../../../../tmp/evil.sh` 与绝对 icon 端到端拒绝）。变异核查：把 `LoadManifest` 里的 `ValidatePaths()` 调用
  摘掉，端到端那条立刻红。
- **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 27 包全绿、
  `GOOS=linux/windows/darwin go build ./...` 三平台通过。

> **上一轮：P7-bg 不可用运行时的失败原因本地化（R11）** —— P7-bd 重写的具体待办第 5 条（最后一条）
> 落地。运行时无法解析原来只让集成行变灰、原因是一句英文 prose 且从不落盘；现在失败是**稳定键 +
> JSON 负载**（`binding-changed:{"path":…}` / `binding-missing:…` / `runtime-unavailable:…`），探测把它
> 连同 `lastErrorCode`/`brokenReason` 写进 repository，集成卡片按语言渲染原因（认旧版英文句，认不出的
> prose 原样保留）。

**P7-bg 运行时绑定的失败原因本地化（本轮）**

- **依赖检查**：`curl .../releases/latest` 与 go proxy 都返回 v0.4.8，`go.mod` 已是 v0.4.8，无版本可升。
- **偏离点（一整条会灰但不会说话的路）**：原版 `src/extensions/binding-errors.ts` 的
  `failureReason(code, detail, t)` 把后端发的 `binding-changed:{"path":…}` 本地化，并认旧版英文句
  `Executable fingerprint changed at …` 以迁移已落盘记录；`error_codes.rs` 的 `ProviderErrorCode` 是码的
  闭集，`src/i18n.ts` 的 `bindingChangedDetail`/`bindingMissingDetail`/`errorCode.*` 是词。本构建
  `internal/extensions/provider.go` 的 `ResolveRuntime` 只返回英文 prose（`is not on the search path` /
  `no %s interpreter is available`），既不落 `lastErrorCode`/`brokenReason`，`internal/settingsui` 也只画
  一个状态词——`grep -rn BrokenReason internal/settingsui` 为空。
- **修法**：
  - 新文件 `internal/extensions/binding.go`（纯）：三个码常量（`binding-changed`/`binding-missing`/
    `runtime-unavailable`，另加旧宿主的兜底码 `binding-check-failed`）、`BindingFailure{Code,Detail}`、
    `Error()`、`NewBindingFailure`/`AsBindingFailure`（类型化读取）、三个 keyed 构造器
    （`BindingChangedDetail`/`BindingMissingDetail`/`RuntimeUnavailableDetail` → `code:{json}`，marshal
    失败退 `code:{}`），以及**保守**解析 `ParseBindingDetail`（码必须在闭集、余下必须是 JSON 对象、该码
    插值的字段必须非空；散文、别的子系统的 `run_…` 键、坏 JSON 都不认）与 `LegacyBindingDetail`
    （两条旧英文句前缀）。
  - `internal/extensions/provider.go`：`ResolveRuntime` 的三条失败改发键——记录的可执行文件不在了 →
    `binding-missing`（带 path）、搜索路径找不到 system 工具或脚本解释器 → `runtime-unavailable`（带
    names，取 `InterpreterNames(language)`）、结构性问题（越界的脚本路径、未知 runtime）→
    `binding-check-failed`（保留底层 prose，不 keyed）。
  - `internal/extensions/probes.go`：新增 `recordBindingFailure`——探测在解析运行时这一步就失败时，像
    一次失败的必需探测那样把 code/detail/broken 状态写进 repository 条目（健康报告不动，因为还没有
    任何探测跑过）；`Reprobe` 在 `ResolveRuntime` 出错时调用它。
  - `internal/settingsui/failure.go`：`failureReason(copy, code, detail)` 是本仓唯一的措辞处——认 keyed
    detail（三句）、认旧英文句（迁移），都不认时把码的标签与原文用 ` · ` 串起来（绝不吞掉），标签来自
    `copy.ErrorCodeLabel`；`integrationRow` 在状态行下画一行 `Failure detail`。
  - `internal/shell/shell.go`：`integrationList` 把 `LastErrorCode` 与 `BrokenReason`（缺则
    `LastErrorDetail`）交给卡片。
  - i18n：`Settings` 新增 `IntegrationsFailureDetail` 与 `ErrorCodeLabel`（旧宿主 19 个码 + 本构建的
    `probe_failed`，两语言逐字对齐旧字典）、`BindingChangedDetail`/`BindingMissingDetail`/
    `RuntimeUnavailableDetail`。
- **测试**：`internal/extensions/binding_test.go`——`TestBindingDetailRoundTrip`、
  `TestParseBindingDetailIsConservative`、`TestLegacyBindingDetail`、`TestBindingFailureDetailPrefersTheKey`、
  `TestResolveRuntimeReportsKeyedBindingFailures`（记录文件已失 → missing、system 工具不在 PATH →
  runtime-unavailable、脚本无解释器 → 列出候选、未知 runtime → check-failed 保留 prose）、
  `TestAsBindingFailureOnlyClaimsItsOwn`、`TestEveryBindingCodeHasALabel`（闭集是 3、返回副本不可篡改）；
  `internal/settingsui/failure_test.go`——`TestFailureReasonWordsABindingDetail`（en/zh 三句、键与 JSON
  绝不外泄）、`TestFailureReasonMigratesALegacySentence`、`TestFailureReasonKeepsProseItCannotWord`、
  `TestEveryBindingCodeHasADictionaryLabel`、`TestTheRowRendersTheRecordedReason`（卡片上四行、raw 形态不在屏上）；
  `internal/extensions/probes_test.go` 的 `TestReprobeRecordsAnUnresolvableRuntime`（真探测：写 code +
  broken + keyed reason，行可解）、`internal/shell/shell_test.go` 的
  `TestABrokenIntegrationCarriesItsRecordedReason`（keyed 与 legacy 两种记录都到达卡片）。变异核查：把
  `recordBindingFailure` 摘掉 → probes 那条红；把 `failureReason` 的 keyed 分支摘掉 → settingsui 三条红。
- **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 27 包全绿（840 项）、
  `GOOS=linux/windows/darwin go build ./...` 三平台通过。

> **上一轮：P7-bf 面板手动 Run 按清单 `output` 分流 + 「最近一次输出」块** —— P7-bd 重写的具体
> 待办第 3、4 条一起落地（两条互相依赖）。面板里点 Run 原来**无条件**开终端，清单写着
> `output: background` 的集成也照开不误，捕获结果也从不落回该行；现在路由由清单决定，后台运行
> 的捕获输出存在集成的行上，卡片里多了一个可折叠、固定高度的「最近一次输出」块。

**P7-bf 面板 Run 的路由与输出块（本轮）**

- **依赖检查**：`curl .../releases/latest` 返回 v0.4.8，`go.mod` 已是 v0.4.8，无版本可升。
- **偏离点（两处，同一个功能的上下半）**：
  1. **面板的 Run 从不读路由**。旧版 `src/ExtensionsPanel.tsx:1746-1795` 拿后端返回的
     `outcome.route`：`terminal` 才开终端，否则把 `outcome.output` 存进 `runOutputs[extension.id]`
     并在成功时给带 `customViewOutput` 的 toast。本构建 `internal/shell/shell.go:1769` 的
     `runWithParams` **无条件** `a.runCommand(entry, argv)`（`:2158` 直接 `RunCommand` +
     `Open(SurfaceTerminal)`），于是清单声明 `output: background` 的集成在面板里点 Run 也开终端。
  2. **捕获得来的输出无处可放**。`extensions.CapturedRun` 只进启动器的输出视图
     （`internal/launcher/output.go:109`），面板的卡片里没有旧版 `.extension-row__output` 那个块，
     i18n 里那六个键（`customLastOutput`/`customViewOutput`/…）一个都没有。
- **修法**：
  - `internal/settingsui/settingsui.go` 新增 `RunResult{Stdout, Stderr, Truncated, Success}` 与
    `Text()`/`HasOutput()`；`App` 增加 `runOutputs`/`outputOpen`/`outputScroll` 三个映射，
    `SetRunResult`/`LastRun`/`setRunOutputOpen`/`runOutputScroll` 一组存取器。成功运行把块折起
    （由「View output」行展开），失败运行立即展开——旧版 run 处理器同此。
  - 新方法 `runOutput`：折起时画一行 `View output`（行标签与按钮都能点开），展开时是卡片内的
    固定高度滚动块（`MaxHeight` 的 `ui.Scroll`）——头部 `Last output` + 截断标记 + `Hide output`，
    正文是 `stdout+stderr` 原样拼接（空则 `(No output)`），尾部在截断时补 `… output truncated at 64 KB`。
  - `internal/shell/shell.go` 的 `runWithParams` 读 `integration.Manifest.Route()`：`terminal` 走
     `runCommand`（原状），`background` 走新的 `runCommandFromPanel` —— `runCommandCaptured` 跑完把
     `CapturedRun` 写进 `a.Settings.SetRunResult(entry.IntegrationID, …)`；面板隐藏时照旧发完成通知。
  - `internal/i18n` 的 `Settings` 补六个句子（en/zh 逐字对齐旧字典 742-747 / 1620-1625）。
- **测试**：`internal/settingsui/view_test.go` 的 `TestRunOutputBlock`（成功折起、点开见正文、
  Hide 收起、空捕获说 `(No output)`、截断在头尾两处标记、失败立即展开）；
  `internal/shell/shell_test.go` 的 `TestPanelRunFollowsTheManifestOutput`（缺省清单 → 捕获结果落在
  行上且 `Success`/stdout 正确；改写成 `output: terminal` 的新实例 → 开终端面且**不**记录捕获输出）。
  变异核查：把 `runWithParams` 里的路由判断去掉，后台那条 `waitForRun` 立刻超时、终端那条记录也不会
  出现。
- **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 27 包全绿
  （823 项）、`GOOS=linux/windows/darwin go build ./...` 三平台通过。

> **上一轮：P7-be 后台完成通知的时长词形与输出行数（R9-2）** —— P7-bd 重写的具体待办第 2 条落地。
> 后台运行成功时的系统通知原来是一句「名字 + 状态」的公文（时长 `3.4s`、没有行数），现在与旧版
> 完成 toast 同一串法：**时长词形**（`420 ms` / `3.4 s`，坏时钟退 `0 ms`）与**输出行数**
> （`2 lines of output` / 截断改口），失败只说退出码。

**P7-be 后台完成通知的摘要（本轮）**

- **依赖检查**：`curl .../releases/latest` 返回 v0.4.8，`go.mod` 已是 v0.4.8，无版本可升。
- **偏离点**：旧版完成 toast（`src/ExtensionsPanel.tsx:1759-1775`）把 `formatRunDuration`
  （`src/extensions/run-routing.ts:41`：<1000ms → `420 ms`，否则 → `3.4 s`，非有限/负 → `0 ms`）与
  `runOutputSummary`（`:99-111` 的 `runOutputLineCount`：两流按序拼接、一个尾换行不额外算行、恰好
  一个换行仍是一行、空捕获 0 行；截断时换成 `truncated at 64 KB`）串成
  `{name} finished in {duration} · {lines} lines of output`（`src/i18n.ts:740-749` 四键）。本构建
  的 `internal/launcher/output.go:329` 只 `fmt.Sprintf("%.1fs")`，`internal/shell/shell.go:2204` 的通知是
  `CommandFinished(name) + " · " + CommandStatus(status)`——**没有时长词形、没有行数**。
- **修法**：
  - 新文件 `internal/extensions/runsummary.go`（纯）：`FormatRunDuration(ms float64)` 复刻
    `formatRunDuration`（NaN/Inf/负 → `0 ms`；<1000 用 `Math.round` 的整数毫秒；否则一位小数秒）、
    `RunHasOutput`（两流任一非空）、`RunOutputLineCount`（拼接两流、去掉恰好一个尾换行再数 `\n`）。
  - i18n：`Settings` 新增四个句子 `RunSucceeded`/`RunFailed`/`RunOutputLines`/`RunOutputLinesTruncated`
    （en/zh 逐字对齐旧字典）。
  - `internal/shell/shell.go`：`runNotificationBody` 按「完成且成功」→ 时长 + （有输出才）行数；
    「完成但非零退出」（`err == nil`）→ 退出码；「没起来/被杀」（`err != nil`，P7-bd 的键）→ 保留
    `RunStatus` 的本地化理由——退出码不是那里用户需要的事实。
  - `internal/launcher/output.go`：输出视图的时长也走同一 `FormatRunDuration`，两处词形不可能走样。
- **测试**：`internal/extensions/runsummary_test.go`——`TestFormatRunDuration`（NaN/±Inf/负/0/420/
  999.4/999.6/1000/3400/9999）、`TestRunOutputLineCount`（空/`"\n"`/`"a\nb"`/尾换行/两个尾换行/
  两流拼接/截断）、`TestRunHasOutput`；`internal/shell/shell_test.go` 的
  `TestRunNotificationSummary`（en 成功 + 行数、秒级 + 截断改口、静默无行数、zh 各一句、失败只说退出码）
  与改写的 `TestCapturedRunNotifiesOnlyWhenHidden`（真实后台跑：成功带行数、失败不带）。
  变异核查：把 `RunOutputLineCount` 的去尾换行摘掉 → 五条边界立刻红。
- **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 27 包全绿、
  `GOOS=linux/windows/darwin go build ./...` 三平台通过。

> **上一轮：P7-bd 运行失败的「键 + JSON 负载」与逐语言渲染（R9-5）** —— P7-bc 后「尚未做」
> 清单已空，本轮先把清单重写成 5 条具体待办（见下方「具体待办（P7-bd 重写）」），再取第 1 条实现。
> 后台运行的失败原来是一句英文 prose（`extensions: run failed: fork/exec …`），现在与旧版同一形状：
> 后端只发**稳定键 + JSON 负载**，界面按语言渲染。

**P7-bd 运行失败的键 + 负载（本轮）**

- **依赖检查**：`curl .../releases/latest` 返回 v0.4.8，`go.mod` 已是 v0.4.8，无版本可升。
- **偏离点**：`src-tauri/src/extensions/run_error.rs`（旧版后端）只发 `key:{json}` 形态的失败——
  `run_program_missing` / `run_program_not_executable` / `run_spawn_failed` / `run_timeout` / …，
  负载字段 `path`/`language`/`names`/`searched`/`detail`/`seconds`/`extension_id`/`reason`/`error`；
  `spawn_failed`（`run_error.rs:135`）把 `NotFound` 映射成 `run_program_missing`、`PermissionDenied`
  映射成 `run_program_not_executable`，其余保留 OS 原文。前端 `src/extensions/run-errors.ts` 的
  `parseRunError`/`runErrorMessage` 负责本地化，`src/i18n.ts:751-764` 是 10 条 `settings.extensions.runError*`。
  本构建 `internal/extensions/run.go` 的超时与 spawn 失败都返回英文 prose（`fmt.Errorf` / `&ProviderError{}`），
  `internal/launcher/output.go` 的 `RunStatus` 与输出视图文本把 prose 直接交出；`grep runError internal/i18n`
  为空——10 个键一个都没有。
- **修法**：
  - 新文件 `internal/extensions/runfailure.go`（纯）：`RunErrorKey` 常量集（旧版 10 个键，顺序照旧）、
    `RunFailure{Key,Payload}`、`Error()` 发 `key:{json}`（marshal 失败退 `key:{}`，键不丢）、
    `ParseRunFailure` 复刻旧前端那份**保守**解析（键必须 `run_` 起头且在闭集内、负载必须是 JSON 对象；
    `run_param_required:note` 这类键+id 不认、prose 不认），以及 `RunSpawnFailure` 复刻 OS 错误→键的映射、
    `RunTimeoutFailure`。
  - `internal/extensions/run.go`：超时返回 `RunTimeoutFailure(int(timeout/time.Second))`；非 ExitError 的
    生成失败返回 `RunSpawnFailure(entry.Program, runErr)`；非零退出仍是「答案不是失败」（`nil`）。
  - `internal/launcher/runfailure.go`：`runErrorMessage(copy i18n.Settings, failure)` 逐键渲染；输出视图的
    `view.Text` 与一行状态经 `RunStatus`/`failureReason` 走同一渲染，未识别的消息**原样返回**。
  - i18n：`Settings` 新增 10 个 `RunError*` 句子（en/zh 逐字对齐旧字典，含解释器缺失时
    `names.join(" / ")` 与 `searched.join("\n")` 的两种连接）。
- **测试**：`internal/extensions/runfailure_test.go`——`TestRunFailureRoundTrips`、
  `TestParseRunFailureIsConservative`（9 行：合法/无体/键+id/未知 run 键/prose/非对象/坏 JSON/数组/空）、
  `TestRunFailurePayloadAccessors`（缺字段不造假）、`TestRunSpawnFailureMapsTheOsError`、
  `TestRunCapturedNamesAMissingProgram`（真跑一个不存在的程序，失败是键）；
  `internal/launcher/runfailure_test.go`——`TestRunErrorMessageIsLocalized`（en/zh 同一键）、
  `TestInterpreterMissingJoinsItsLists`（两种连接）、`TestFailureReasonKeepsUnrecognizedProse`、
  `TestRunStatusLocalizesAKeyedFailure`（随 store 语言切换）。
  变异核查：把 `run.go` 的 `RunSpawnFailure` 换回 `&ProviderError{}` → `TestRunCapturedNamesAMissingProgram` 立刻红。
- **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 27 包全绿、
  `GOOS=linux/windows/darwin go build ./...` 三平台通过。

> **上一轮：P7-bc 漂移通知门与相对时间本地化（R7-7）** —— P7-bb 写实的第 2、3 条落地。
> 后台上游工具改版后的重探（G2）原来一句话都不说；现在它的「一次真变化一句」判定是一个纯函数
> （`DecideDriftNotice`），三条规则**有顺序**：无变化不发（门都不问，不烧名额）、无名不发（门同样
> 不问）、同一变化一次。顺带把 freshness 明细那截 `3d`/`3h` 缩写换成原版 `Intl.RelativeTimeFormat`
> 的本地化词形（en/zh，分钟→年）。

**P7-bc 漂移通知门 + 相对时间本地化（本轮）**

- **依赖检查**：go proxy 显示 v0.4.8（GitHub releases 没有对应 release，只有 tag）。已
  `go get github.com/egoist/mygo@v0.4.8 && go mod tidy`。对照 `v0.4.7...v0.4.8` 只有三条提交：
  浏览器输入框触摸键盘、新增 `ui/editor`（代码编辑器 + diff 查看器）、发版。**无自然用点**——
  floter 是启动器/设置/终端三表面，登记审查里的变更是一行权限清单而非文本 diff，描述符是原地
  重生成的 JSON 也无处 diff，故本轮只升级框架，不改应用。三条提交已读，无 API 破坏。
- **偏离点 1（R7-7 的可见那一半）**：原版 `src/extensions/freshness.ts:206` 的
  `createReprobeNoticeGate`（会话内 `seen` Set，`allow(key)` 首次真）、`reprobeNoticeKey`
  （`id\u0000toolVersion\u0000kind:count` 稳定 key）、`decideDriftNotice(frame, name, gate)`——
  三条**有顺序**的规则：「无变化不发」（**先于门**，不能烧掉名额）、「无名不发」（门只在有名字时
  才问）、「同一变化一次」（key 相同则沉默，不同变化仍发）；`src/ExtensionsPanel.tsx:2078` 的
  `handleDriftNotice` 把 op-progress 帧写回列表行（`lastProbeAt`/`commandCount`/`previousCommandCount`，
  缺字段保留旧值、不填 0），并向唯一一个 `onNotify` 发 `settings.extensions.reprobeNotice`。
  本构建 `internal/extensions/reprobe.go` 的 `ReprobeCommands` 是**手动、同步**的；全仓
  `grep -i drift` 只在无关注释里命中，没有通知门、没有 op-progress 帧、也没有后台重探调度。
- **偏离点 2（freshness 明细的相对时间）**：原版 `src/extensions/freshness.ts:165` 的 `relativeTime`
  用 `Intl.RelativeTimeFormat`（本地化「3 minutes ago」/「3分钟前」，覆盖到月/年；< 45s 走 caller
  自己的 `justNow`）；本构建 `internal/settingsui/settingsui.go` 的 `humanAge` 产出 `3d`/`3h`/`3m`
  缩写、到「天」就封顶，再被 `copy.FreshnessAgo` 包成「3d ago」/「3d前」——不是原版的词形。
- **修法**：
  - 新文件 `internal/extensions/driftnotice.go`：`ReprobeNoticeFrame`、`DriftGate` 接口与
    `DriftNoticeGate`（`Allow`/`Size`）、`ReprobeNoticeKey`、`DecideDriftNotice` 复刻三条顺序与 key
    （`DriftNoticeDecision{Display, Announce}` 两个独立结果；缺计数保留 nil，不假造 0）。**纯半**：
    不排时钟、不发通知，调度决策留给后续轮次。
  - i18n：`Settings` 新键 `FreshnessRelative(value, unit)`，`FreshnessAgo` 字段删除；新增
    `RelativeUnit`（minute/hour/day/month/year）。en/zh 各一个实现，**逐字对齐** `Intl.RelativeTimeFormat`
    （`numeric: "auto"`, `style: "short"`）的输出——含它缩短的形式：en「yesterday」「last mo.」
    「last yr.」，zh「昨天」「前天」「上个月」「去年」（用 node 实测输出校准）。
  - `internal/settingsui/settingsui.go`：`humanAge` → `relativeAge(age) (int, i18n.RelativeUnit)`
    （45s 判 justNow 的阈值从原来的 `> time.Minute` 改成 `< 45s`，与旧版一致；阈值 3600/86400/
    2592000/31536000 与 `math.Round` 半进位照搬）与 `freshnessWhen(copy, age)`。
- **测试**：`internal/extensions/driftnotice_test.go`——`TestDecideDriftNoticeAnnouncesAChange`、
  `TestNoChangeDoesNotBurnTheGate`（无变化后真变化仍发）、`TestNoNameWaitsWithoutBurningTheGate`、
  `TestSameChangeAnnouncesOncePerSession`（同帧静默 / 不同量仍发 / 不同版本仍发）、
  `TestMissingCountsFabricateNothing`、`TestReprobeNoticeKeyIdentity`、`TestFreshGateRearms`；
  `internal/settingsui/freshnesswhen_test.go`——`TestRelativeAgeThresholds`（45s/59s/60s/90s/3599s/
  3600s/86399s/86400s/172800s/2591999s/2592000s/31535999s/31536000s/63072000s 边界）、
  `TestFreshnessWhenIsLocalized`（en/zh 各 11 行，含 44s→just now、2 天→前天、30 天→上个月、
  400 天→去年）。
  变异核查：把 `DecideDriftNotice` 的「无变化」分支挪到门之后 → `TestNoChangeDoesNotBurnTheGate` 红；
  把 day 的 `value==1` 特例删掉 → en「yesterday」与 zh「昨天」两行红。
- **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 全绿、
  `GOOS=linux/windows/darwin go build ./...` 三平台通过。

> **上一轮：P7-bb 集成行的 freshness 点（R7-7b）** —— 原版列表行上那个「一句话新鲜度」的
> 状态点（synced / changed / unknown）从没落地：本构建只在展开的卡里有一行 freshness 明细。
> 现在同一份纯投影 `Freshness.DotState()` 既喂那一行、也喂名字旁的点，还顺手修了 `FreshnessOf`
> 把「探过、但没有健康报告」错报成 unknown 的结果推导。

**P7-bb 集成行的一句话 freshness（本轮）**

- **依赖检查**：`curl .../releases/latest` 返回 v0.4.7（`go.mod` 已是 v0.4.7），无版本可升；本轮走
  「对照原版排查偏离点」，同时把「尚未做」里已全部完成/卡框架的项目换成 P7-bb 的新具体待办。
- **偏离点（两处，同一条链）**：
  1. **列表行的 freshness 点没有**。原版 `src/extensions/ExtensionRow.tsx:161` 在名字行（version 与
     meta 之间）画一个 8u 圆点，它的状态由纯函数 `freshnessDotState`（`src/extensions/freshness.ts:143`）
     从**同一个** `Freshness` 投出来，三态闭集 `synced` / `changed` / `unknown`：只有「一次真的比较过
     且计数动了」才是 `changed`（唯一带暖色的态），比较过且相等才是 `synced`，其余（没探过、只探过
     一次、在飞、失败/降级、发布者描述符）都是 `unknown`——**「第一次探测」不许写成「已同步」**，那正是
     `commandDelta` 已经拒绝制造的假信心。点的颜色是态（synced=`--text-secondary`、changed=`--text-warning`、
     unknown=`--text-muted`，`src/styles/extensions.css:926`），标签与 tooltip 是同一句
     `settings.extensions.freshnessDot`（「Command status: {state}」）。本构建 `internal/settingsui`
     有 `freshnessState`/`freshnessDetail` 的**明细行**，却没有任何三态投影，也没有行级标记。
  2. **`FreshnessOf` 的结果推导漏了「探过即成功」**。原版 `freshnessOf` 的决定链是
     `running → failed(errorCode 或 unhealthy) → degraded → (atSeconds !== null ? success : unknown)`；
     本构建 `internal/extensions/freshness.go` 的 `FreshnessOf` 只在**有健康报告**时才写 `Result`，
     于是「宿主生成、探过成功、但没声明 lifecycle probes」的集成结果永远停在 `unknown`——明细行显示
     「未知」而时间/计数都在，点也永远到不了 `synced`。
- **修法**：
  - `internal/extensions/freshness.go`：新增 `FreshnessDot` 三态类型与 `Freshness.DotState()`，复刻
    `freshnessDotState`（先看 delta 的增/减 → changed；再 `Result==success && delta==unchanged` → synced；
    否则 unknown）。`FreshnessOf` 的结果改成原版那条链：失败/降级的健康报告直接胜出，否则**有过探测
    时间就是 success**（健康报告的 CheckedAt 也计入时间来源），全无探测才是 unknown。
  - `internal/settingsui/settingsui.go`：`freshnessDot(c, copy, freshness)` 在集成名旁画
    `t.Space(2)` 的圆点，颜色按 `DotState()`，`Label`/`Tooltip` 同为 `freshnessDotLabel(copy, dot)`
    （把态词塞进 `copy.FreshnessDot` 的共享句框）；`integrationRow` 把 `freshnessOf` 读一次，
    喂名字旁的点与下面的明细行——**一个值，两处画，不可能互相矛盾**。
  - i18n：`Settings` 新键 `FreshnessDot(state)` / `FreshnessDotSynced` / `FreshnessDotChanged` /
    `FreshnessDotUnknown`（en/zh，原版原文）。
- **测试**：`internal/extensions/freshness_test.go` 的 `TestFreshnessDotState`（9 行表：增/减 → changed、
  相等与 0==0 → synced、首探/失败/降级/在飞/没探过 → unknown）、`TestFreshnessOfGeneratedIntegration`
  追加「探过即 success」断言；`internal/settingsui/view_test.go` 的 `TestFreshnessDotProjectsTheRowState`
  （in sync → changed → unknown 三态逐帧、tooltip 与 label 同句、没探过则没有任何点）。
  变异核查：把 `DotState` 的 delta 分支删掉 → extensions 三条立刻红；把 `freshnessDot` 从名字行摘掉 →
  settingsui 那条立刻红；把 `FreshnessOf` 的 `AtSeconds > 0` 分支删掉 → 「探过即 success」立刻红。
- **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 27 包全绿
  （789 项）、`GOOS=linux/windows/darwin go build ./...` 三平台通过。

> **上一轮：P7-ba `platformOverrides.minimumOsVersion` 的执行** —— P7-az 顺带记下的那条：
> override 的 `minimumOsVersion` 已解析进结构却**没人执行**。现在新增宿主 OS 版本探测
> （macOS `sw_vers`、Linux `uname -r`、Windows `ver`，可注入），命中的 override 在安装期
> 比较宿主版本，低于下限的包在落盘前被拒收并说明原因；已装的包不因宿主升级/降级而失效。

**P7-ba override 的 OS 下限在落盘前生效（本轮）**

- **依赖检查**：`curl .../releases/latest` 返回 v0.4.7（`go.mod` 已是 v0.4.7），无版本可升
  （本轮只有 game 相关能力，floter 无自然用点）。本轮取具体待办里**唯一不卡框架**的第 8 条，
  即 P7-az 自己记下的新偏离。
- **偏离点（解析了、没人执行）**：`PlatformOverride.MinimumOSVersion`
  （`internal/extensions/platform.go`）跟着 `platformOverrides` 一起进了 Go 结构，`EffectiveProvider`
  也会命中 override，但**没有任何地方读这个下限**——声明 `minimumOsVersion: "6.8"` 的包在 5.15 内核
  上照装照跑。原版 `manifest.rs:442` 的 `ResolvedManifest::validate_minimum_os_version` 在
  `install.rs:2524`（right after `resolve`）比较宿主 OS 版本与下限；`current_os_version()`
  （`manifest.rs:657`）在 macOS 读 `sw_vers -productVersion`、Linux 读 `uname -r`、Windows 读
  `GetVersionExW`；`numeric_version`（`manifest.rs:638`）只取第一个数字起头的点分数字段。
  它**只在 install 期查**（`extensions.rs` 的 pre-install review 与 registry 选版本都只跑
  `resolve()`，不跑这条），已装包不因宿主 OS 变化而失效——与 `compatibility` 同一取向。
- **修法**：新文件 `internal/extensions/osversion.go`。
  - `HostOSVersion()` 按宿主平台取版本（macOS `sw_vers -productVersion`、Linux `uname -r`、
    Windows `cmd /c ver`），读不到就返回空串——**不猜**。
  - `parseOSVersion` 复刻原版 `numeric_version`：第一个数字起头的点分数字段、去掉尾部非数字、
    缺 minor/patch 读作 0（`6.8.0-45-generic` = 6.8.0，`14.5` = `14.5.0`）。
  - `Manifest.MinimumOSVersion(target)`（`platform.go`）复刻 `resolve()` 的挑法：按
    `Identifiers()` 顺序走，**最后一个命名下限的 override 胜**（`<os>-any` 压过完整 target，
    且后面没有下限的 override 不会抹掉前面的）。
  - `Manifest.ValidateMinimumOSVersion(target, hostVersion)`：空下限不查；下限读不出来就拒收
    （清单要了、我们给不了）；宿主版本读不出来**不拒收**（与 `ValidateCompatibility` 对无名构建
    的同一宽容）。
  - `Paths` 新增 `OSVersion`（零值 = 探测宿主，见 `HostOSVersion()`），让测试不靠机器；
    `prepare()` 与 `install()` 在 `ValidatePlatform` 之后各调用一次（原版 `install.rs:2524` 的
    同一位置），路径都在错误分支清掉 staging。
- **测试**：`internal/extensions/osversion_test.go`——`TestParseOSVersion`（9 行表，含 tail、缺位、
  溢出）、`TestMinimumOSVersionFollowsTheOverrideOrder`（OS 宽胜、无下限的 override 不抹掉、无
  override 为空）、`TestValidateMinimumOSVersion`（8 行表：低于/等于/高于、缺 patch、未声明、
  另一平台的底线不管本机、宿主读不到不拒、下限读不到拒）；`TestPrepareRefusesAnOsBelowTheOverrideFloor`
  （注入 target/OS 版本：低于下限拒收且不留 staging）、
  `TestAMatchingOsVersionLetsACompatibleManifestThrough`（达标照装）。变异核查：把 `prepare` 里的
  `ValidateMinimumOSVersion` 调用摘掉，第一条立刻红。
- **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 27 包全绿（787 项）、
  `GOOS=linux/windows/darwin go build ./...` 三平台通过。

> **上一轮：P7-az 清单的 `platforms` 与 `platformOverrides` 真正生效** —— schema 里的
> `platforms` 一直只被 JSON 读进来、从没人查，`platformOverrides` 更是整块没进 Go 结构；
> 声明 `platforms: ["darwin"]` 的包在 Linux/Windows 上照列照跑。现在清单按宿主平台解析，
> 越界的包在安装期与库存期都被拒收并说明原因，命中的 `platformOverrides` 改写 provider 的
> argsPrefix / environment。

**P7-az 清单的平台约束（本轮）**

- **依赖检查**：`curl .../releases/latest` 返回 v0.4.7（`go.mod` 已是 v0.4.7），无版本可升。本轮走
  「对照原版排查偏离点」，找到的是同一类「声明了、没人读」的化石，而且比 `compatibility` 更彻底：
  两个字段一个只写不读、一个根本没解析。
- **偏离点（两个字段，零消费者）**：
  - `Manifest.Platforms`（`internal/extensions/manifest.go:32`）只有 `custom.go:420` 写入，**全仓没有
    读取**；原版 `manifest.rs:503` 的 `resolve()` 在 `platforms` 非空且不含当前 OS 时返回
    `Extension {id} does not support {os}`，install（`install.rs:2523`）与 registry 选版本
    （`registry.rs:63`）都先跑它。于是本构建里 `platforms: ["darwin"]` 的包在 Linux 上照样列出、照样运行。
  - `platformOverrides`（schema `floter-extension.schema.json:178`）**整个字段没有进 Go 结构**：原版
    `resolve()` 按 `override_identifiers()` 顺序（完整 target → `<os>-any`）把命中的
    `providerArgsPrefix` 替换、`environment` 合并进 provider。本构建一个都不认。
- **修法**：新文件 `internal/extensions/platform.go`。
  - `Platform{OS, Arch, Libc, Abi}` + `CurrentPlatform()`：`runtime.GOOS/GOARCH` 映射成 schema 的
    `darwin`/`linux`/`windows` 与 `aarch64`/`x86_64`/`armv7`（接受旧版 `PlatformArch::parse` 的多种
    拼写）；Linux 的 libc 从 `ld-musl-*.so.1` / `*-linux-gnu` / `ld-linux-*` 探测，armv7 的 ABI 从
    `ld-linux-armhf.so.3` 探测，读不到就是 `unknown`（**不猜 gnu**）。
  - `Platform.Identifiers()` 复刻原版顺序（完整 target → `<os>-any`）；`Manifest.ValidatePlatform`
    复刻 `resolve()` 的拒绝；`Manifest.EffectiveProvider` 复刻 override 的 argsPrefix 替换与 env 合并
    （后应用者胜，且**不改动原 manifest**）。
  - `Paths` 新增 `Platform`（零值 = 宿主，见 `Target()`），让跨平台断言不必靠机器；`LoadInventory`
    在 `LoadManifest` 之后按 `paths.Target()` 校验，`prepare()` 与 `install()` 在落盘前各查一次
    （原版 `install.rs:2523` 的同一位置）；`ProtocolArgs`/`providerEnv` 改读 `EffectiveProvider(CurrentPlatform())`，
    让 describe/complete 拿到解析后的 provider。
- **测试**：`internal/extensions/platform_test.go`——`TestManifestPlatformsGateTheHost`（8 行表，含空表、
  多选、大小写）、`TestPlatformIdentifiers`（5 行表，含缺 libc/abi 读作 unknown、armv7 四段）、
  `TestPlatformOverridesResolveToTheProvider`（argsPrefix 被替换、env 合并、`<os>-any` 胜、原 manifest 不被污染、
  无命中照旧）、`TestProtocolArgsReadThePlatformOverride`（argv 与 env 都取到 override）、
  `TestInventoryRefusesAnUnsupportedPlatform`（注入 target：越界 `ManifestErr` 且 `Running()` 假、
  合规照跑）、`TestInventoryKeepsAnUnconstrainedManifest`（空 `platforms` 不误伤）、
  `TestInstallRefusesAnUnsupportedPlatform`（`PrepareLocal` 落盘前拒绝）。变异核查：把 `LoadInventory` 里的
  `ValidatePlatform` 摘掉、或让 `EffectiveProvider` 忽略 override，对应用例立刻红。
- **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 29 包全绿
  （`go test ./... -v | grep -c '^=== RUN'` = 765）、`GOOS=linux/windows/darwin go build ./...` 三平台通过。
- **顺带记下（本轮未做，见「具体待办」）**：override 的 `minimumOsVersion` 已解析进结构，但**没有执行**
  ——原版 `install.rs:2524` 的 `resolved.validate_minimum_os_version()` 会读当前 OS 版本（macOS `sw_vers`、
  Windows build）再比较，本构建还没有采集宿主 OS 版本的地方。

> **上一轮：P7-ay `output` 缺省回到 schema 的 `background`** —— P7-as 顺带记下、说「留待单独
> 一轮」的那条：`Manifest.Route()` 把未声明的 `output` 当作 `terminal`，而原版与 schema 的缺省是
> `background`；现在未声明/不可识别的 `output` 一律走 `background`，只有显式 `terminal` 才进 PTY。

**P7-ay `output` 缺省对齐原版（本轮）**

- **依赖检查**：`go.mod` 已是 v0.4.7；`git ls-remote --tags https://github.com/egoist/mygo.git` 的
  列表里 v0.4.7 仍是最新（GitHub 的 releases API 被限流，用 tags 复核），无版本可升。本轮走
  「对照原版排查偏离点」，取 P7-as 明确「留待单独一轮」的那条。
- **偏离点（缺省取向反了）**：原版 `src-tauri/src/extensions/manifest.rs:207` 的 `OutputMode` 用
  `#[default] Background`，字段带 `#[serde(default, skip_serializing_if = "OutputMode::is_background")]`
  （`manifest.rs:49`）——**一份在 `output` 出现之前写的清单必须继续解析成 background**
  （`manifest.rs:705` 的 `output_defaults_to_background_and_round_trips_through_schema`，改动
  `#[default]` 即红）；schema 的 `output.default` 同样是 `"background"`
  （`docs/extensions/schemas/floter-extension.schema.json:225`）。而本构建 `internal/extensions/manifest.go:50` 的
  `Manifest.Route()` 只认 `"background"` 字符串、其余一律 `terminal`（plan P3-h 的选择）。于是
  所有**未声明 `output`** 的扩展命令都落进终端面，而原版是静默跑完、在启动器里回看输出——正是
  `store.go:357`（`CommandEntries` 写 `Route`）与启动器 `search.go:183,196` 消费的那条链。
- **修法**：
  - `Manifest.Route()` 反过来：只有显式 `terminal`（大小写与首尾空白容忍）走 `RouteTerminal`，
    空、`background` 与不可识别的词都落 `RouteBackground`（缺省即 schema 缺省）。
  - `Manifest.Output` 加 `omitempty`，并把 `custom.go` 的 `customManifest` 改成**只写显式的
    `terminal`**——保留缺省的自定义工具清单从此不再写出 `"output": ""`，与原版
    `skip_serializing_if = is_background` 的落盘拼写一致。
- **测试**：`internal/extensions/run_test.go` 的 `TestManifestRoute` 表改为「空/background/杂词 →
  background，terminal（含大小写与空白）→ terminal」；`route_test.go` 的
  `TestCommandRouteFollowsTheManifestOutput` 改成「显式 `terminal` 的集成 → terminal、不声明的集成 →
  background（schema 缺省）」；新 `TestCustomToolOutputSpelling` 钉住自定义工具**缺省不写
  `output` 字段**、`LoadManifest` 后 `Route()` 是 background，声明 `terminal` 才写出字段；
  `internal/shell/shell_test.go` 的 `TestCapturedCommandReachesTheLauncher` 不再手动注入
  `"output": "background"`，直接靠夹具的缺省断言背景路由（更强：测的是缺省本身）。
  变异核查：把 `Route()` 换回旧的「缺省 terminal」，`TestManifestRoute`、
  `TestCommandRouteFollowsTheManifestOutput`、`TestCustomToolOutputSpelling` 三条立刻红。
- **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 27 包全绿
  （750 项）、`GOOS=linux/windows/darwin go build ./...` 三平台通过。

> **上一轮：P7-ax 终端面的失败提示落地** —— 旧版 `useTimedFeedback` 的终端那一半：终端面板自己的
> 「在系统终端打开」失败时，现在在面板角上说话（4.5s 后自清），而不是静默地什么都不发生。

**P7-ax 终端面板的失败提示（本轮）**

- **依赖检查**：`go.mod` 已是 v0.4.7；`git ls-remote --tags https://github.com/egoist/mygo.git` 的
  列表里 v0.4.7 仍是最新（GitHub 的 releases API 被限流，用 tags 复核），无版本可升。本轮继续
  「对照原版排查偏离点」。
- **偏离点（一个静默的失败）**：旧版终端面板的「在系统终端打开」在 `open_in_default_terminal`
  失败时，`src/hooks/useTerminalView.ts:437` 会 `showTerminalFeedback("launcher.error.externalTerminal")`
  并 `focusTerminalView()` —— 面板左上角浮出一条警告色的提示（`src/App.tsx:3896` 的
  `.terminal-feedback`，`src/styles/terminal.css:393`：accent 键线、`--glass-float`、`--elev-3`、
  15u 警示图标、进入用 `feedback-enter` 的 160ms），这条提示由 `src/hooks/useTimedFeedback.ts`
  在 **4500ms** 后清掉。本构建 `internal/shell/shell.go` 的 `termActions.OpenExternal` 在失败时
  **只 `log.Printf`**：`a.Terminal` 上没有任何反馈通道，用户点了「在终端打开」而机器没有终端模拟器
  时，面板一点反应都没有（`OpenTermFailed` 这句文案只走启动器那句 `WarnFeedback`，见
  `internal/shell/launch.go:73`）。
- **修法**：`internal/terminalui/terminalui.go` 新增面板级浮出提示：`ShowFeedback(message)` /
  `FeedbackVisible()`，`feedbackNote(c)` 在 `View` 里画旧版 `.terminal-feedback` 同一形态
  （左上角 12u、accent 键线、浮动材质与两层阴影、警示色 560 的正文），并按 `useTimedFeedback`
  的 4500ms 计时——`feedback != feedbackShown` 时起表、过期即清，`c.AnimationFrame()` 让静置的
  面板也会到点；空态分支也画，因为标题栏的控件在没有会话时也在。`internal/shell/shell.go` 的
  `termActions.OpenExternal` 失败时 `ShowFeedback(copy.OpenTermFailed)` + `FocusTerminal()`。
- **测试**：`internal/terminalui/feedback_test.go` 的 `TestTerminalFeedbackShowsAndExpires`（显示、
  起表、过窗自清）、`TestTerminalFeedbackRestartsOnANewerMessage`（已过期的旧消息不会吞掉新消息，
  时钟重启）、`TestTerminalFeedbackShowsOverTheEmptyState`（空态也画）；
  `internal/shell/shell_test.go` 的 `TestTerminalOpenExternalFailureSpeaksInThePanel`（失败才说话，
  且面板真的画出那句）。变异核查：把 `feedbackNote(c)` 从 `View` 摘掉，terminalui 三条立刻红；
  把 shell 的 `ShowFeedback` 摘掉，shell 那条立刻红。
- **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 全绿、
  `GOOS=linux/windows/darwin go build ./...` 三平台通过。

> **上一轮：P7-aw 插件模式的驻留真正生效** —— 具体待办第 7 条（R35 报告点名的那半条）落地：
> 插件模式与配置覆盖层现在是「可驻留的表面」，唤回时按时钟保留或收线。

**P7-aw 插件模式的驻留（本轮）**

- **依赖检查**：`go.mod` 已是 v0.4.7，`curl .../releases/latest` 返回 v0.4.7（本轮唯一的新功能是
  game 相关：`Bitmap.Pixelated` / `Painter.ImageRegion` / `Painter.Transform` / gamepad /
  `game.View`/`Camera` / `Player.SetLoopRange` / `ui.Canvas`——都是游戏插件的旋钮，floter 是启动器
  没有自然用点），无版本可升。本轮取具体待办里唯一不卡框架的第 7 条。
- **偏离点（一整条设置只服务两个面）**：R35 的报告原文是「所在插件和页面应该维持一段时间」,
  原版 `src/surface-residency.ts` 的 `residencySurface` 把 collapsed 面分成 `plugin-mode`
  （browser/clipboard/calculator/外部命令)与 `plugin-config`,与 settings/terminal 同级。本构建
  的 `residencyHolds()`（`internal/shell/shell.go`）对 `SurfaceLauncher` 直接返回 false,而
  `Toggle()` 的收线分支只调 `Open(SurfaceLauncher)`——它既不重置查询、也不离开插件模式,所以
  插件模式**永不被保留计算、也永不被收线**（等价于「永远保留」）。于是「保留页面」这条设置在
  启动器面上完全没有可观察效果；i18n 提示（`internal/i18n/copy.go:324`）还把原版的「插件页面」
  那半句删掉了以掩盖。
- **原版对照**：`src/surface-residency.ts:201` 的 `residencySurface`（settings → terminal →
  collapsed 下的 plugin-config → plugin-mode → null)与 `src/App.tsx:2480-2530` 的唤回处理——
  时钟成立时 `residency.refresh()` + `clearDrops()` 并保留该面,否则 `setQueryExitingPlugin("")`
  + `clearDrops()` 回到全新搜索页。时钟本身由 `useSurfaceResidency` 每次渲染 note 面变化。
- **修法**：
  - `internal/launcher/fieldkeys.go`：`PluginModeActive()` 导出 `pluginModeActive()`（shell 读不到
    未导出的谓词）；`internal/launcher/files.go`：`ClearDropped()`（原版 `clearDrops()` 的等价物）。
  - `internal/shell/shell.go`：`currentResidencySurface()` 按原版 `residencySurface` 的同一顺序
    推导当前面（settings / terminal / plugin-config / plugin-mode / 空）；`noteResidency()` 在
    `View` 每帧对比上帧的面,进入可驻留面时起表、离开时清记录（时钟由状态推导,所以遗留的时钟
    永远不公 claim 下一个唤回）；`residencyHolds()` 改成先问当前面是不是可驻留面;
    `Toggle` 的收线分支收成 `summon()`：时钟成立则 `enteredAt = now()`（原版 `residency.refresh()`）
    并 `Show()`,否则 `ResetQuery()` + `ClearDropped()` + `Open(SurfaceLauncher)`。
  - `internal/launcher/launcher.go`：`ResetQuery()` 补 `syncConfig()`——搜索页没有插件覆盖层,
    离开模式就该把配置表一起收走（与 `View` 每帧的 `syncConfig` 同一规则,只是不再等到下一帧）。
  - i18n：`SurfaceResidencyHint` 中英文回到原版原文（「A plugin page, its settings, or the
    terminal…」/「插件页面、其设置或终端……」）。
- **测试**：`internal/shell/shell_test.go` 的 `TestPluginModeResidency`（普通搜索页唤回总是清查询;
  进入插件模式起表;窗内唤回保留模式与字段;窗外唤回回空搜索页）、
  `TestPluginModeResidencyClearsDroppedRows`（窗内连拖放行一起留、窗外连行一起清）、
  `TestPluginModeResidencyFollowsTheSetting`（0 关、never 永久）、
  `TestPluginConfigResidency`（覆盖层是更具体的面,窗内保留、窗外连同模式收走）、
  `TestLeavingASurfaceClearsTheResidencyClock`（显式离开后遗留时钟不再生效）；
  `internal/launcher/fieldkeys_test.go` 的 `TestPluginModeActiveNamesTheModes`（导出谓词与内部谓词
  在五种模式上一致）与 `TestClearDroppedForgetsTheRows`。变异核查：把 `a.noteResidency()` 从
  `View` 摘掉,三条插件模式测试立刻红；把 `summon()` 的 `ResetQuery()`+`ClearDropped()` 摘掉,
  四条立刻红。
- **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 全绿（27 包）、
  `GOOS=linux/windows/darwin go build ./...` 三平台通过。

> **上一轮：P7-av 旧版两个 `useTimedReset` 落地** —— 「尚未做」清单已空（1–3 卡框架、4–6 完成），
> 本轮继续「对照原版排查偏离点」，取旧版两处**会自己归位**的状态：电源确认的时间窗和设置写失败的横幅。

**P7-av 旧版会自己归位的两个状态（本轮）**

- **依赖检查**：`go.mod` 已是 v0.4.7；`git ls-remote --tags https://github.com/egoist/mygo.git` 列表里
  v0.4.7 仍是最新（GitHub releases API 被限流），无版本可升。本轮走「对照原版排查偏离点」。
- **偏离点（两处，都来自旧版唯一的两个 `useTimedReset`）**：
  1. **电源确认没有时间窗**。原版 `src/App.tsx:546` 的
     `useTimedReset(pendingSystemAction, () => setPendingSystemAction(null))`，默认超时 3000ms
     （`src/hooks/useTimedReset.ts`）——**没人回答的电源确认三秒后自己退场**（确认键/取消
     键之外的第三条出路）。本构建的 `pendingPower`（`internal/launcher/power.go`）只被 Esc、
     确认/取消按钮和换模式清掉；一个「重启还是关机」的提问可以一整晚挂在列表下面。
  2. **设置写失败没有横幅**。原版 `src/App.tsx:596` 的
     `useTimedReset(settingsSaveFailed, dismissSaveError, 5000)` 给了同一条时间窗：磁盘写失败时
     `settings.saveFailed`（「无法应用更改，请检查设置后重试。」）这个横幅画在设置内容之上
     （`src/App.tsx:2938` 的 `.settings-save-alert--toast`，带一个 ✕ 提前清掉），5 秒后自带收起。
     本构建 `internal/settingsui/settingsui.go:1998` 的 `set()` 是 `_ = a.Store.Update(mutate)`——
     **写失败静默丢弃**：内存里改了、文件里没写，用户什么都看不到，下次启动发现改动丢了。
- **修法**：
  - 电源（`internal/launcher/power.go`）：新增 `powerConfirmWindow = 3 * time.Second`；`runPower`
    记 `pendingPowerAt`；`expirePowerConfirm(c)` 在 `View` 每帧（挨着 P7-au 的 `expireArmedDelete`）
    过期，装填期间 `c.AnimationFrame()` 让帧继续来，所以静置时也会到点；`confirmPower`/`cancelPower`
    一并清掉时刻。
  - 设置（新文件 `internal/settingsui/savealert.go`）：`set()` 捕获 `Store.Update` 的错误并置
    `saveFailed`/`saveFailedAt`；`expireSaveFailed(c)` 五秒到期（同样 `AnimationFrame`）；
    `saveAlert(c, copy)` 把警告底色的横幅连同 ✕ 画在页面头部之上；`App` 增加可注入的 `Now`；
    i18n 新键 `Settings.SaveFailedAlert`（en/zh 旧版原文），✕ 复用现成的 `Settings.Close` 作为标签。
- **测试**：`internal/launcher/view_test.go` 的 `TestThePowerConfirmationExpiresOnItsOwn`（注入时钟：
  窗内确认仍在、窗外确认与它的行一起消失且**什么都没运行**）；新文件
  `internal/settingsui/savealert_test.go` 的 `TestTheSaveFailureBannerAppearsAndExpires`（写失败才出现、
  内存仍改了、窗内留驻、窗外消失）与 `TestTheSaveFailureBannerDismisses`（✕ 提前清、能写的 store
  从不报）。变异核查：把 `expirePowerConfirm(c)` 从 `View` 摘掉，第一条立刻红；把 `set()` 改回
  `_ = a.Store.Update(mutate)`，后两条立刻红。
- **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 全绿、
  `GOOS=linux/windows/darwin go build ./...` 三平台通过。
- **顺带记下的新偏离（本轮未做，见「具体待办」第 7 条）**：`residency_seconds` 对**插件模式**
  （browser/clipboard/calculator/命令模式）从来没有生效——`internal/shell/shell.go:2569` 的
  `residencyHolds()` 对 `SurfaceLauncher` 直接返回 false，而本构建的启动器**从不**在唤回时收线
  （原版 `src/App.tsx:2516` 的 `setQueryExitingPlugin("")` + `clearDrops()`），所以插件模式既不在
  窗内被保留、也不在窗外被重置；i18n 的提示（`internal/i18n/copy.go:324`）也把原版的「插件页面」
  那半句悄悄删了。

**P7-au 两步删除的时间窗**

- **依赖检查**：`go list -m -versions github.com/egoist/mygo` 与 `proxy.golang.org` 都确认 v0.4.7 仍是最新
  （`go.mod` 已是 v0.4.7），无版本可升；本轮走「对照原版排查偏离点」。
- **偏离点（一个消失的边界）**：旧版的两步删除有一条**时间窗**（`src/plugins/history-actions.ts` 的
  `HISTORY_DELETE_CONFIRM_MS = 5000`）：`useLauncherActions` 在 `armedId` 上挂一个
  `HISTORY_DELETE_CONFIRM_MS + 50` 的 `setTimeout`，到点以 `{type:"expire"}` 归约——静置的装填
  **自己退场**，取消路径是「确认键 / Esc / 其它键 / 焦点丢失 / 超时」五条。本构建的 `armedDelete`
  （`internal/launcher/launcher.go`）只是一个 id 字符串：`move`、离开模式、`DisarmDelete`（blur/换面）
  都会清，**唯独超时这条从未存在**——装填可以一直挂着，直到某次无关的第二次 ⌘⌫ 落在它上面。
  连带地，旧版 `reduceHistoryDelete` 的确认还要求 `now - armedAt <= CONFIRM_MS`：过了窗再按同一行
  **重新装填**而不是删除；本构建只比 id，于是「超时后仍会误删」是这个洞的真实后果。
- **修法**：
  - `App` 新增 `armedDeleteAt time.Time` 与可注入的 `clock func() time.Time`（`now()` 缺省走墙钟），
    装填时记时刻；`disarmDelete()` 把「清 id + 清时刻」收成一处，`DisarmDelete` 与各清点改走它
    （`launcher.go` 的 Escape/离开模式/移动、`calculator.go` 离开计算器模式）。
  - `expireArmedDelete(c)`：在 `View` 顶部每帧检查，超过 `historyDeleteConfirm`（5s）就退场；
    装填期间 `c.AnimationFrame()` 让帧继续来（与停靠状态行的 `feedbackRow` 同一形状），
    所以静置窗口里超时也真的会发生。
  - `historyDeleteKey` 的确认加回旧版的时间条件：同一行且 `now - armedAt <= 5s` 才算确认，
    否则按「不同行」处理——**重新装填**，不是删除。
- **测试**（`internal/launcher/fieldkeys_test.go`）：`TestTheArmedDeleteExpiresOnItsOwn`（注入时钟推进 6s，
  下一帧装填与屏幕上的「Press again to delete」一起消失、一行没删）、
  `TestAPressAfterTheWindowReArmsInsteadOfDeleting`（过窗后按同一行：条目数不变、仍是同一行装填）、
  `TestAConfirmInsideTheWindowStillDeletes`（2s 内第二次按仍删）。变异核查：注掉 `View` 的
  `expireArmedDelete(c)` 并去掉 `historyDeleteKey` 的时间条件，前两条立刻红。
- **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 全绿、
  `GOOS=linux/windows/darwin go build ./...` 三平台通过。

**P7-al 被拒绝的深链说出来（本轮）**
- **依赖检查**：`go list -m -versions github.com/egoist/mygo` 确认 v0.4.5 仍是最新（go.mod 已是 v0.4.5），
  无版本可升；本轮走第 3 条（对照原版排查偏离点）。
- **偏离点**：旧版深链有一条**「拒绝要说话」**的规则（`src-tauri/src/deep_link.rs` 的 `Reject::is_silent`）：
  畸形 URL、缺 `manifest` 的 `connect`、manifest 验证/加载失败会把面板带到前面（`focus_main`，**不换正在
  显示的面**）并发一条 `settings.deepLinkRejected`（「无法打开该链接」），且**按 key 30 秒去重**
  （`createFailureDeduper` + `FAILURE_NOTIFY_DEDUP_MS`——外部触发会自己重试）；只有未知 action 与不支持的
  scheme 保持安静。本构建对畸形 URL 静默打开启动器、对准备失败的 connect 只写日志——用户点一条坏链接
  什么也看不到；而 `connect`/`register` 还先把用户带到集成页，让拒绝像是「页面开了但什么都没发生」。
- **修法**：
  - 新文件 `internal/shell/notices.go`：`refusalGate`（按 key 的 30 秒窗口，时钟可注入，对应旧版
    `createFailureDeduper`）；`queueNotice`/`flushNotices` 在 `View` 第一遍把排队的 toast 交给框架自己的
    `ui.Context.AddToast`（框架的 toast 栈在所有面之上，正是启动器那条**停靠行**服务不了的场景）；
    `refuseLink` 先过窗口，再 `Show()`（旧版 `focus_main`：带到前面不换面），最后一句 8 秒错误 toast
    （旧版 `TOAST_DISMISS_MS.error`）。
  - `HandleURL`：解析失败 → 拒绝；scheme 不匹配 / 未知 action → 保持安静；`connect` 缺 `manifest`、
    `register` 缺名字 → 在**碰设置页之前**拒绝；`connectFromLink` 的 `stat` / `PrepareLocal` 失败走同一条。
  - i18n：新增 `Settings.DeepLinkRejected`（en/zh，旧版原文）。
- **顺带**：`ui.Context.AddToast` 是框架 v0.4.5 就有的能力，此前全仓零调用，而 `P5` 的文档一直写着
  「反馈走框架 toast」——拒绝是第一个真正需要跨面显示的用户可感知消息。
- **测试**：`notices_test.go`（去重窗口三段纯测试、拒绝只排一条并画一次、窗口内的重复与畸形 URL 不再排、
  未处理的 scheme 与未知 action 保持安静、后台失败的 connect 仍画出来）；摘掉 `allow` 守卫两条「重复不再
  排」会红，摘掉 `flushNotices` 两条「画出来」会红。

**P7-ak 空字段的退格离开插件模式**
- **偏离点**：旧版 R32 的规则是「退格删空即停」：把 needle 退到空的**那一按**停在模式里（那是编辑器的
  编辑），空字段上**再按一次**退格才离开模式（`src/launcher.ts` 的 `pluginModeExitOnBackspace`——
  `mode !== null && needle.length === 0`）。本构建的字段只有 Escape 一条出路，空字段上的退格被编辑器
  吃掉、什么也不做。这条规则还被本仓库自己的协议文档 `docs/plugin-development.md` §7 的「离开（R31）」
  明写着——**文档在承诺一个从未接线的转换**（P7-t/P7-u 之后的同一类漏洞）。
- **修法**：`internal/launcher/fieldkeys.go` 的 `fieldKeys`（字段自己的 `HandleInput`，唯一先于编辑器的
  那道缝，见 P7-v）加第三条规则：`ev.Mods` 不含 ⌘/⌃/⌥、`Query` 为空、且有模式持有字段时，离开它。
  Shift 不被拦（旧版只读 `metaKey || ctrlKey || altKey`），带修饰键的退格全部留给编辑器。
- **一条梯子**：新增 `App.leavePluginMode()`（files → calculator → browser → clipboard → command，
  返回「有没有」），Escape 的五段 switch 与这里的按键读的是同一条，模式的顺序不可能写两遍而走样；
  顺带 `pluginModeActive()` 把此前四处重复的同一串模式判定收成一个谓词。
- **测试**：`fieldkeys_test.go` 新增 8 项（空字段离开剪贴板模式且不动历史、两步的「删空停 / 再按走」、
  有 needle 时编辑器的手势不变、五种模式逐一、⌘/⌃/⌥ + 退格不离开、⇧⌫ 是普通键、普通页什么都不做、
  梯子唯一）；把新分支换成 `if false` 跑一遍，除两条「断言否定」外全红。

**P7-aj 会话列表回到旧版自己的轴（本轮）**
- **偏离点**：旧版会话页（`src/settings/SessionsPage.tsx`）的列表按 `createdAt` 倒序（默认
  `newest`），页头带「Newest first / Oldest first」翻转控件；每行是 `名字` + `cwd` + `状态 · 尺寸 ·
  创建时间`。本构建把列表按 `Index`（创建顺序，最旧在前）直出，页头没有任何控件，行下只有一个
  状态词——**默认顺序与旧版相反**，行丢掉了 cwd / 尺寸 / 起始时间。
- **修法**：
  - `settingsui.Session` 补上 `Dir` / `Size` / `Created`；`shell.go` 从 `terminalui` 读出来
    （新增 `DirOf` / `SizeOf`，`Created` 取会话自己的 `Session.Created`）。
  - `sessions()` 先过 `sortedSessions()`：默认最新优先（`Created` 倒序），页头的 `sectionWithAction`
    把当前顺序当控件（按下翻转，旧版 `session-manager__sort-toggle`），排序 **stable**——没有
    起始时间的 fixture 保持 shell 交来的顺序。
  - 行改用新的 `rowLines`：sublabel 现在是多行——shell 报的 cwd（有才画）在上，`状态 · 尺寸 ·
    2006-01-02 15:04` 在下（时间格式与启动器的浏览记录同一写法）。
  - **轮询不做**：会话在本进程内，列表就是表面当帧持有的东西，刷新无从谈起（i18n 里那三个
    从未被读过的状态词 `SessionsRunning/SessionsShowing/SessionsExited` 随之删掉，新增旧版的
    `SessionsSortNewest`/`SessionsSortOldest`）。
- **测试**：`TestSessionsListOrdersAndToggles`（默认最新在前、控件写「Newest first」、行里有
  cwd / `100×30` / 格式化后的起始时间、按下后写「Oldest first」且列表翻到最早在前）、
  `TestSessionsWithoutAStartKeepTheShellsOrder`（无起始时间时顺序不变）；把 `sortedSessions`
  的 `sort.SliceStable` 摘掉会红。shell 的 `TestSessionsListReportsEverySession` 追加
  Dir/Size/Created 到达页面的断言。

**P7-ai 分离窗口属于它的内容（本轮）**
- **偏离点**：P7-d 的分离输出窗口按每次按下都 `append` 一个新窗口（`Options.OpenDetached` 每次
  都调），于是「再钉同一个命令」会开出**两个一模一样的窗口**，而不是像旧版那样把已经在显示它的
  那个窗口抬到前面并换上新一次运行的内容。旧版 R91（`plugin_windows.rs` 的 `content_key` +
  `detach_plugin_window`）写明：**同一份内容 → 原窗换内容**（R84-R90 的行为，为它而留），
  **不同内容 → 另开一窗**（R91 的新行为，本构建的 slice 已经支持）。几何记忆也应是**按内容**的
  （旧版 `plugin-window-geometry.json` v2 按 `content_key` 存 `{width,height,x,y}`），而不是所有
  分离窗口共用一块记忆。
- **修法**：`internal/shell/detach.go` 新增纯函数 `detachContentKey`（命令按 `IntegrationID` +
  NUL + `Command.ID`，无命令的运行按 trim 过的标题；NUL 让 `ab`+`c` 与 `a`+`bc` 不碰撞）与
  `detachStateKey`；`detachOutput` 先 `findDetached(key)`——命中就替换该窗的 entry/view/title、
  `SetTitle`/`Show`/`Focus`/`Invalidate` 并返回，未命中才开新窗。新窗的 `StateKey` 取
  `detachStateKey(key)`，交给框架的 `window-state.json` 按内容记住位置与尺寸（旧版那个 store 的等价物）。
- **最新一次运行获胜**：窗口多一个 `generation`，替换内容时自增；`rerunDetached` 捕获启动时的
  generation，回调走 `applyDetachedRun`——世代对不上就丢弃，旧运行不会盖掉用户后钉的内容。
- **测试**：`TestDetachReusesTheWindowForTheSameContent`（同命令换 argv/文本复用一窗、无命令
  的运行按标题复用与另开、第三窗是另一条 note；把 `findDetached` 改回总是 nil 会红）、
  `TestDetachContentKey`（命令不跟标题、不同命令不同 key、NUL 消歧、文本 key 去空格、
  几何 key 随内容且互不相同）、`TestAStaleDetachedRunDoesNotOverwriteNewerContent`
  （stale 运行被丢弃、当前运行落地；去掉 `existing.generation++` 会让复用测试红）。

**P7-ah 外观行只有一个来源**
- **偏离点**：P7-ab 给终端加了它自己的外观抽屉后，同一批控件在**两处**各写了一份：设置页
  General 的「终端外观」卡片和终端抽屉都手写了全部七行（字号、字体族、行距、内边距、光标形状、
  光标闪烁、配色）。旧版 R42/R55 只有**一份 schema**，并规定两个面各画哪些行——设置页的卡片
  只留用户点名的四项（字体族、字号、行距、配色），更细的轴（内边距、光标形状、闪烁）只画在
  抽屉里。本构建于是把抽屉里的控件又原样搬了一份到设置页，两个面没有任何机制保证它们不漂移。
- **修法**：`internal/control` 新增 `AppearanceAxis` 与三个纯函数——`AppearanceAxes()`（抽屉画的
  全集，也就是 schema 的顺序）、`OnSettingsCard()`（旧版 `page: "card" | "strip"` 的规则）、
  `SettingsCardAxes()`（全集按规则过滤）。`settingsui.general` 与 `terminalui.drawer` 都改成
  遍历 schema、按轴查自己的行构造器，行序不再由各视图各自维护；设置卡片因此少掉内边距/光标形状/
  光标闪烁三行，它们仍然只在终端自己的抽屉里（值照旧、从抽屉改的设置照旧对会话生效）。
- **测试**：`internal/control/TestAppearanceAxesSplit`（卡片是无重复 schema 的严格子集、顺序就是
  schema 过滤后的顺序、抽屉专属恰好是那三项）；`settingsui/TestTerminalAppearanceControls` 与
  `terminalui/TestTerminalDrawerShowsTheAppearanceRows` 各自从 schema 读取应画的行拼标签表
  （schema 多一个轴而某视图没画就红），并断言对方那一面的行在自己这里**缺席**。

**P7-ag 行内标记与孤儿接管带上自己的说明（本轮）**
- **偏离点**：上一批 tooltip（P7-aa 设置/终端、P7-af 字段行）之后，旧版 `title=` 的最后一批仍没补：
  启动器行自己的**警告点**（旧版 `extensions.conflict`「命令冲突」——本构建仓库里**没有**这个词条）、
  **收藏星**、**行内钉住控件**、**来源词**（`MaxWidthPercent(30)` 会被省略号吃掉），以及输出视图的两个
  图标控件（旧版 `pluginWindow.detachHint`「在独立窗口中打开这份输出，不再随主面板消失」）。
  同一处还有 `IntegrationsAdoptHint`——孤儿行只有一个「接管」按钮，没有一个字说明它做什么，
  而这条文案写好了整整一轮没人读。
- **修法**：`launcher` 的警告点挂 `Label`+`Tooltip`、来源文本与星/钉控件挂 `Tooltip`、
  `rowControl` 带上自己 `Label` 作为 hover 说明；`output` 视图的 `control` 增加 hint 参数，
  分离控件挂新的 `OutputDetachHint`；`settingsui` 的孤儿行把 `IntegrationsAdoptHint` 落到第二行
  （可见说明，而不是藏进 tooltip）。新增 `CommandConflict`、`OutputDetachHint`（en/zh，旧版原文）。
- **测试**：`TestCommandRowWarningCarriesItsSentence`、`TestHistoryRowMarksCarryTheirSentences`、
  `TestCommandRowSourceCarriesItsWholeWord`、`TestOutputViewControlsCarryTheirHints`
  （悬停后 tooltip 文本多出一个节点；把对应 `Tooltip` 摘掉跑一遍全红）；
  `TestOrphanRowsOfferAdoptAndDelete` 追加「孤儿行解释接管」。

**P7-af 启动器字段行的图标控件带上自己的说明**
- **偏离点**：P7-aa 只补了设置页头部与终端标题栏的图标 tooltip，**启动器字段行行尾的三个图标**
  （插件齿轮、会话、设置）仍是光秃秃的字形：旧版每个 `.collapsed-card__settings` 都带 `title`，且设置那
  句里的快捷键是用户当前的 `open_settings` 绑定。`i18n.Launcher.ShortcutSettings`（「Cmd+,」）是这个
  加速键标签的残骸，注释写着「the accelerator labels the rows show」，却从没有一行视图读过它。
- **修法**：`fieldButtons`/`pluginGear` 各挂 `Element.Tooltip`；新增 `FieldSessionsHint`、
  `FieldSettingsHint(shortcut)`、`ConfigurePluginHint`、`ConfigurePluginCloseHint`（旧版原文，en/zh），
  删掉死的 `ShortcutSettings`——设置那句话的加速键从用户当前绑定算出，改键跟着走。
- **测试**：`TestFieldRowButtonsCarryTheirHints`、`TestPluginGearCarriesItsHints`；摘掉 `Tooltip`
  跑一遍四条断言全红。

**P7-ae 深链的复制确认与命令列表的说明（本轮）**
- **偏离点**：两处「文案早已写好、没有一个视图画过它」的同一类漏洞：
  1. About 页的深链卡片每行一个「Copy link」按钮，按下后**毫无反馈**——剪贴板写入本来就是无声的。
     旧版 `DeepLinkRow` 把按钮文字翻成 `settings.deepLinkCopied`（「Link copied」）1600ms，还带一个对勾。
     本构建的 `DeepLinkCopied` 字段在 `i18n` 里躺了一整轮没人读。
  2. Integrations 页每条命令一个开关，但**没有说明这一列开关是干什么的**。旧版 R39 在列表上方印了
     `settings.extensions.commands`（「Commands」）标题与 `commandsHint`（讲「直出」：搜索框敲命令名加
     空格、其后即参数、输出就印在搜索界面），并且明确是「印一次，不重复到每一行」。本构建把这两条
     `IntegrationsCommands`/`IntegrationsCommandsHint` 也留着没画。
- **修法**：
  - `settingsui.App` 新增 `copiedLink`/`copiedLinkUntil`：复制成功（`CopyText` 非 nil）后按链接记下
    1600ms 的确认窗，`about()` 的按钮在窗内改读 `DeepLinkCopied`；`c.After(...)` 在窗口到期时
    要一帧，按钮自己回到「Copy link」，不靠下一次无关事件。确认只属于**被复制的那一行**。
  - `integrationRow` 的命令列表前加一行 caption + hint（`a.row` 的空控件行），把两条文案落到列表上方。
  - 顺带把 `IntegrationsCommandsHint` 的中英文案对齐旧版原文（原来是一句自造的「开关关掉就不出现在
    启动器里」，语义窄了；旧版讲的是「直出」这条真实路径，本构建 P7-t 已接线）。zh 的标题也从「命令」
    改成旧版的「命令列表」。
- **测试**：`TestDeepLinkCopyConfirmsOnTheButton`（复制前两行都是「Copy link」；点第一行后剪贴板拿到
  该链接、只有它翻成「Link copied」、另一行仍是「Copy link」；`linkCopied` 在窗口内真、越过窗口假）；
  `TestIntegrationCommandSwitches` 追加命令列表的 caption 与 hint 在（把 caption 行摘掉会红）。

**P7-ad 存储的健康状态真正画在行上**
- **偏离点**：P3-p 记的是「`probeReport.status` 持久化并显示在集成行上」，但那次提交只往
  `settingsui.Integration` 加了一个 `Health` 字段、在 shell 里把它算出来（跨运行的探针报告 + 本次
  检查的结果），**没有任何视图代码读过它**。于是仓库里持久化的健康状态（healthy / degraded /
  not working）算完就丢，用户只能在「检查」那一瞬间看到结果；而原本标着「Health」的那一行其实
  画的是 diagnose 协议的文本，不是健康状态。
- **修法**：集成卡里新增一条**自己的**行——标签 `IntegrationsHealth`，值是该工具最近一次探针的
  状态词（`healthy` 中性、`degraded` 警告色、`not working` 危险色），跨运行可见、检查后刷新。
  diagnose 协议与 provider 报错的行改挂新标签 `IntegrationsDiagnostics`（旧版
  `settings.extensions.diagnostics`「诊断结果」的原文），两个事实不再共用一个「Health」。
- **测试**：`TestStoredHealthShowsOnTheRow`（健康行与状态词在、broken 工具显示 not working、
  diagnose 文本在「诊断结果」下、无报告无检查时两行都不画；把健康行摘掉跑一遍会红）。

**P7-ac 打开设置的快捷键**
- **偏离点**：设置页 Shortcuts 的「App shortcuts」把 `open_settings`（默认 ⌘, / Ctrl+,）列成一条
  可录制的绑定，自定义快捷键的 `action:open_settings` 也走同一张表——但**内置的那条从来没被读过**。
  旧版在 collapsed（启动器）里 `matchesShortcut(event, shortcuts.open_settings)` → `openSettings()`。
  本构建只靠 macOS 应用菜单里硬编码的 `CmdOrCtrl+,` 兜着：Linux/Windows 没有菜单，⌘/Ctrl+, 什么
  都不做；改了键也只有菜单那条冻结的旧键还活着，新键彻底无人接。又一例「UI 卖了一个从不回应的键」。
- **修法**：
  - `internal/launcher/launcher.go` 的 `appShortcuts` 加上 `ShortcutOpenSettings` →
    `Actions.OpenSettings()`。启动器就是旧版回答它的那个面，其余面维持旧版行为。
  - `internal/shell/system.go` 的 Settings… 菜单项加速键改为读用户存储的绑定
    （`menuActions.SettingsAccelerator`，空则回退 `CmdOrCtrl+,`）；`shell.setShortcut` 改
    `open_settings` 时重装菜单，菜单里不再留一个冻结的 Cmd+,。
- **测试**：`TestOpenSettingsShortcutOpensSettings`（默认键开设置、改键后旧键失效新键生效；把接线
  摘掉跑一遍会红），`TestMenuSettingsAcceleratorFollowsTheBinding`（存储的绑定进菜单，空则回退）。

**P7-ab 终端自己的外观抽屉**
- **偏离点**：旧版终端页的齿轮打开的是**终端自己**的外观抽屉（`.terminal-settings-drawer`，R42/R54）：
  一条停靠在面板下缘的条带，画布是它的**兄弟**而不是被它盖住，所以打开它只会把 PTY 挤小，
  正在跑的会话（连同滚动缓冲）原地不动。本构建把齿轮接到「跳到设置页的 General 页」——
  用户为了改字体大小离开终端，改动虽然会实时应用，但“在同一页里调外观”这条交互没有了。
- **修法**：`internal/terminalui/drawer.go`。`App.Drawer` + `ToggleDrawer`/`CloseDrawer`；
  `View` 把画布包进一个 `Grow(1)` 的 body，抽屉作为它的**兄弟**（`Shrink(0)`）排在下面，
  resident 提示也挪进 body（否则会浮在抽屉上）。齿轮在开启时变成 ✕（旧版
  `toolbar-button--settings-open`），抽屉自己右上角也有一个 ✕，两个入口同一状态。
  行是旧版 `repeat(auto-fit, minmax(230px, 1fr))` 的等价物：`Wrap()` + `MinWidth` 的格子，
  窗口窄了就换行（860px 三列、720px 两列，实测）。调色板那行带它自己的小型终端预览。
- **只画真能生效的轴**：字体大小、字体族、行距、内边距、光标形状、光标闪烁、配色。
  旧版条带里还有 bold 模式、滚轮步长、选中即复制、粘贴安全、滚动条开关——框架的
  `plugins/terminal` 没有这些旋钮（`Options` 里没有，也没有运行期的 setter），
  **画一个什么都不做的控件是撒谎**，所以不画（见「已知差距」）。
- **一处共享，不是两份实现**：新增 `internal/control`（`Segmented`）——设置页与抽屉的
  分段控件此前只存在于 `settingsui`，抽屉需要同一个材质，所以把它提出来；调色板预览提成
  `terminalui.PalettePreview`（`settingsui` 反向调用，包方向不变，无环）。
- **i18n**：`i18n.Terminal` 的 `SettingsLabel` 拆成 `SettingsOpen` / `SettingsOpenHint` /
  `SettingsClose`（旧版 `terminal.settingsOpen` / `settingsOpenHint` / `settingsClose`），
  抽屉的组名复用 `settings.GroupTerminal`。
- **测试**：`drawer_test.go`（七行都在、每个控件写回 store、两个 ✕ 同名且各自能关、
  齿轮翻转并来回、抽屉不吞掉画布且停在画布之下、跟随语言）、`internal/control` 的分段控件
  三项（画全、回报下标、选中项有盒子）、shell 的 `TestTerminalGearOpensTheDrawer`
  （齿轮翻转抽屉且**不离开终端面**）。

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

**P3-p 健康状态行上显示**
- `probeReport.status` 持久化并显示在集成行上（跨运行可见；「检查」刷新它），broken 时不贡献命令。
- **订正（P7-ad）**：这一轮只把状态算进了 `Integration.Health`，视图从未读过它——健康状态实际
  不可见；P7-ad 才把它画成行，并把 diagnose/报错的行改挂「诊断结果」标签。

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
- **占位符颜色没有自己的框架字段**：原版 `prefers-contrast: more` 块里
  `.collapsed-card__input::placeholder { color: var(--text-placeholder-contrast) }`，而框架把输入框占位符
  画在 `editor.Theme.LineNumber`，由 `ui/editor/theme.go:61` 从 `TextMuted.Alpha(0.75)` 派生，`ui.Theme`
  没有占位符字段、`ui.Input` 也没有设色的 API。P7-bs 把这个 token 落在 `TextMuted` 上（框架能接到的
  唯一处，顺带把安静文案抬了一级，正是原版 `.app-toast { color: var(--text-primary) }` 的意图）。
  **要什么**：`ui.Theme` 上一个占位符 token（或 `ui.Input.PlaceholderColor`）。
- ~~toast 没有色调~~ **已完成（P7-dy）**：原版三档 toast 各有色/字形（`src/toast-state.ts:36-40` 的
  `TOAST_DISMISS_MS` 与 `src/components/ToastStack.tsx` 的 kind 图标/样式），本构建之前用的框架默认
  外观（`ui/toast.go:343` 的 `buildToasts`）只读 `fill, text := t.inverse()`，完全不看
  `ui.Toast.Type`。P7-dy 用框架已支持的 `ui.ToastViewportBase` 自绘视口
  （`internal/shell/notices.go:169` 的 `toastViewport`）：error 与 warning 共用 amber
  （`Theme.Warning`，与旧版 CSS 自己的 R9-1 注释一致）、success 用 accent，每档带自己的字形
  （AlertCircle / TriangleAlert / Check），并按表面定位（折叠启动器左下、全高页右上）。
  **仍余一个框架口子**：error toast 的 `role="alert"`（相对其它档的 `role="status"`）在
  `ui.Role` 里没有枚举（只有 `RoleStatus` / `RoleAlertDialog`），`ToastBase` 固定 `RoleStatus`，
  故不接线，如实留在此。
- **终端选中即复制 / 滚动条 / bracketed paste**：见下面的终端外观条目。**复制反馈**是另一回事
  （P7-an 定位）：旧版 R44/R63 的规则是**每一次复制都在终端自己的状态行里说一句**
  （`src/terminal/copy-notice.ts`：`visible` 1500ms → `fade` 260ms → idle；失败也说，
  `terminal.copyNotice.failed`），启动器的直出面共用同一套词（`useLauncherTextCopy`）。本构建
  什么都看不到：复制发生在框架内部（mygo `plugins/terminal/view.go:467` 的 `v.copy()` 直接
  `v.services.WriteClipboard(text)`），而 `ui/services.go:14` 的 `Services` 由窗口宿主拥有、
  应用无法包一层；`ui/handler.go:20` 的 `InputCommand` 只由菜单 role 发出，没有公开的发送/观察
  API（应用菜单用的是 `mygo.RoleEditMenu`，`internal/shell/system.go:459`）。
  **要什么**：`terminal.Options.OnCopy(text string, ok bool)`（或让 Edit role 的 Copy 可观察）。
  拿到了就能按旧版接：i18n 补 `Terminal.CopyNoticeCopied/Failed`，连续复制重启 visible 不叠加计时器。
- **键入文本的 Unicode 空格接缝**（P7-an 定位）：原版在 `src/hooks/useTerminalView.ts:917-925`
  的 `sendTerminalText` 里对**每一次键入**（含 IME 提交）都做规范化。本构建已覆盖**粘贴**
  （`Options.OnPaste`）与 **App 自己拼的命令行**（`ParseCommandLine` / `RunShellWithCommand`），
  但框架终端自己处理的键入文本够不到：`plugins/terminal/view.go:60` 的 `e.HandleInput(v.input)`
  是终端视图自己的，而 `ui/input.go:568` 的 `rt.focusHandler()` 只把 `HandleInput` 交给**聚焦
  元素**，应用无法在外面包一层改写它；`Options.OnKey`（`plugins/terminal/terminal.go:124`）的
  文档也写明「只打字的键在 Linux 上它看不到」。**要什么**：一个文本钩子（或让 `OnKey` 看到
  text-only 键）。在那之前，键入一个全角空格仍然会走到 shell 里（P7-an 的三个接缝已封住其余三面）。
- ~~插件模式的 scope 标记~~ **已完成（P7-m）**：进入模式消耗触发词并清空字段，输入框左侧画
  `图标 + 模式名`（`.collapsed-card__scope`）；模式内的搜索/补全把字段当 needle；Esc 离开时
  needle 保留为普通查询（R31），**运行**则清空字段（旧版自己的行为）；模式只由用户自己离开。
- ~~会话排序 / 轮询~~ **排序已完成（P7-aj）**：旧版会话页有"最新/最早"排序与 5 秒轮询（会话由
  broker 托管，可能跨天存活）；本构建的会话在进程内、生命周期与 App 相同，所以**轮询不做**（列表
  就是表面当帧持有的东西），但**排序做了**——默认最新优先（旧版自己的默认），页头有翻转控件，
  行下也补上了旧版的 cwd / 状态 / 尺寸 / 起始时间。
- **设置里的 Plugins 页已删**：内置插件的配置现在是启动器里的通用表单（P7-c），与旧版 R33 一致；
  遗留的 `last_settings_page: "plugins"` 会落到第一页。旧版 Integrations 页自己的「基础插件」区
  （三个内置插件 + 开关 + 「配置」）已在 P7-u 补上，它也是浏览器模式空态文案指向的那扇门。

- 终端外观的**四个**设置磁盘上存在、会被保留，但当前无 UI、也不起作用：
  `terminal_bold`（加粗模式：粗体字/亮色）、`terminal_scrollbar`、`terminal_select_copy`（选中即复制）、
  `terminal_wheel_lines`。mygo 的 terminal 插件（`plugins/terminal`）
  没有暴露这些旋钮（内部固定：滚动条总画、滚轮步长固定、复制仅 ⌘C、
  bold 由字体权重决定），所以 Go 侧既不能渲染（渲染了就是骗人）也不能接线。等框架暴露后再补。
  **订正（P7-an）**：`terminal_paste_safe`（粘贴不执行）曾和这四个记在一起，是**错的**——
  `Options.OnPaste`（`plugins/terminal/terminal.go:116`，v0.4.6 就有）一直存在，这个设置是死的
  只因为本构建从未设过那个 hook。P7-an 接上了它，它现在有自己的抽屉行（`control.AxisPasteSafe`）。
  字号/字体族/光标形状/闪烁/行高/内边距/调色板/透明度**已全部接线**，字体族还带真实安装探测。
  注：光标形状与闪烁只在**新建会话**时生效（框架的 `terminal.Terminal` 只有 `SetFont`/`SetTheme`，
  没有运行期的光标 setter），所以终端抽屉里这两行也是下一会话才看得到；字号/行距/内边距对打开的
  会话实时生效。调色板与透明度**从 P7-dr 起也实时**：背景不再由插件画，而是本应用自己的一层
  （`options()` 里 `Transparent` 是常量，`View` 每帧读设置画 `canvasFill`）。终端抽屉（P7-ab）
  因此不画 bold/滚轮/选中即复制/滚动条四行（粘贴安全那一行 P7-an 已补上）。
- **块光标的字形颜色是插件的**（P7-dr 核实，P7-ds 确认为框架无口子）：原版 `main:src/terminal/render.ts:586-591`
  在块光标铺满后，用**该单元格自己的前景色**重绘字形（`base = cellFg === WIRE_FG ? this.fg : cellFg`，
  加粗的亮色模式还会 `brighten(base)`），所以光标下的字保持它本来的颜色；mygo 插件固定用
  `color(v.colors.Background)`（`plugins/terminal/paint.go:500-505`），除非 `Theme.CursorText.A > 0`
  （`plugins/terminal/theme.go:13-14` 写明这是**一个**颜色），于是光标块下的字用的是主题背景色。
  P7-ds 已走完框架、确认没有逐单元格钩子（`CursorText` 是一个全局色），故不半接（给全局前景色
  会在 `amber` 这类光标与前景都偏亮的调色板上把字压没）。**要什么**：一个逐单元格的光标文字色口子。
  注：光标**自身**的颜色不在此列，已由 P7-ds 修回 `--terminal-cursor`。

**P1-ad 版式与交互对齐（多轮，本轮）**
- 设置面板全量改造为分组卡片语言 + 页面头部 + 侧栏字形/选中 + 权限两层 + 每集成一卡 + 本地工具连接表单。
- 启动器：图标底板/字形集、触发提示、分节标题、模式空状态、平台化徽标、动作栏 Enter 徽标、停靠状态行、
  筛选 chip 轴、共享命令词警告点、调用行。
- 终端：浮动栏 + 呼吸点 + 已结束会话保留 + 字体系探测。

**已完成（P7-v）：历史行的删除键**
- 此前记为框架限制（`editor.wants` 对带修饰键的 Backspace 返回 true，且编辑器分支没有 `claimed`
  守卫，窗口快捷键够不到）。现在由**字段自己的 `HandleInput`** 回答（见 P7-v）：`⌘⌫` / `Ctrl+⌫`
  装填并确认选中历史行的两步删除，`⌃⇧⌫` 清空命令历史。行尾的 ✕ 控件此前保留，P7-ge 对照
  原版移除（原版只有键盘 ⌘⌫ 与装填后的 `deleteConfirm` 文本，没有点击删除）。
- 教训：`c.Shortcut` 够不到的键，`Element.HandleInput` 够得到——前者排在编辑器之后，后者排在最前。

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

### 具体待办（P7-hb 重写；3 条已全部落地：第 1 条 P7-hb、第 2 条 P7-hc、第 3 条 P7-hd）

审计入口：原版设置页与终端抽屉共用的**分段控件** `main:src/settings/SettingsRows.tsx` 的
`SegmentedChoice`（`:152-197`）与 `main:src/styles/settings.css` 的
`.settings-options`/`.settings-option`/`.settings-option__label`（`:763-765`、`:1012-1035`、
`:1067-1120`），本构建对应 `internal/control/control.go` 的 `Segmented`/`SegmentedFill`。此前
`grep -c` 复核：`SegmentedChoice` 在 `docs/` 全文只出现 1 次（P7-ab 提 `internal/control` 时），
`settings-option` 的**度量/字级/对齐/字色**从未逐字段比对——P7-ab 只对过材质（轨道 `LensInset`、
选中段 `Lens` + accent 键线），P7-em 只对过透镜 census。依赖检查：`go.mod` 已是 **v0.5.1** =
`go list -m -versions` 的最新（`api.github.com` 本轮被限流），无版本可升；只对照 `main`
（`backup/main-pre-mygo` = v0.3.14 原版树），未翻历史提交。

#### P7-hb-1 · 段盒缺原版的 30u 下限（**本轮落地，P7-hb**）

- **原版对照**：`main:src/styles/settings.css:1031-1055` 的 `.settings-option`：
  `min-height: calc(var(--u) * 30); padding: 0 calc(var(--u) * 9)`（R43 的注释就在行内：34u 收到
  30u）；轨道 `.settings-options--inline`（`:1016-1030`）的 `padding/gap: 2u`、`border-radius: 999px`。
- **本构建（改前偏离）**：`internal/control/control.go:47` 的
  `Padding(t.Space(0.75), t.Space(2.25))`（竖直 3u + 水平 9u）**没有下限**，段盒高度随标签行盒走——
  默认界面档实测 **22.4px**，原版 30u = **27px**。
- **落地 file:line（已落地）**：`internal/control/control.go:47` 改
  `Padding(0, t.Space(2.25)).MinHeight(t.Space(7.5))`（= `padding: 0 9u; min-height: 30u`），
  `AlignItems(ui.Center)` 居中原样保留；`Segmented`/`SegmentedFill` 共用一处。
- **验收（已过）**：`internal/control/segmentmetrics_test.go` 的 `TestTheSegmentBoxIsThirtyUnitsHigh`
  （三条段的盒高都 == `Space(7.5)` = 30u 且互等）。变异核查：摘掉 `MinHeight` → 三条 17.0 ≠ 27.0 红。

#### P7-hb-2 · 未选段的标签被压暗到 muted/500，原版整条轨道都是 `--text-strong`/580（**本轮落地，P7-hc**）

- **原版对照**：`.settings-option__label { color: var(--text-strong); font-weight: 580 }`
  （`main:src/styles/settings.css:1111-1120`）——对**每个**段都一样；选中态只由
  `.settings-option--active`（`:1095-1098`）的 raised pane + accent 键线承担，`color`/`font-weight`
  不随选中改变；裸 `.settings-option` 的字色是 `--text-primary`（`:1060`），也没有按选中的压暗规则。
- **本构建（改前偏离）**：`internal/control/control.go:64-70` 的非当前段走 `t.TextMuted` + 500，
  只有当前段 `TextStrong` + 580——原版的字色/字重差是本构建发明的。
- **落地 file:line（已落地，P7-hc）**：`internal/control/control.go` 的 `segmented` 删掉按选中的
  `TextMuted`/500 分支，标签统一 `tokens.TextStrong` / 580；选中态仍由段上的 `ControlHover` +
  `Lens` + `AccentEdge` 键线承担，未动。
- **验收（已过，P7-hc）**：`internal/control/segmentlabel_test.go` 的
  `TestEverySegmentLabelIsTextStrong`——用 `tt.Image()` 采每条标签自己的墨迹像素（离该段表面最远的
  像素 = 字形自身的颜色），断言它与 `tokens.TextStrong` 合成到该段表面后的值相等、且不等于
  `TextMuted` 的合成值（选中/未选三段都测）。变异核查：把 `TextMuted`/500 分支加回 → 未选段读成
  `{146 147 150}` 而目标是 `{236 237 239}`，红。

#### P7-hb-3 · 基础段的标签被居中，原版把非材质段当「行」左对齐，只有材质选择器居中（**本轮落地，P7-hd**）

- **原版对照**：`.settings-option { text-align: left }`（`main:src/styles/settings.css:1060`）——
  `SegmentedChoice` 的按钮里只有一个 `.settings-option__label`，落在 `grid-template-columns:
  minmax(0,1fr) auto` 的第一列，因此**左对齐**；唯一居中的是材质选择器
  `.glass-intensity__segments .settings-option { text-align: center; justify-items: center }`
  （`:792-799`），CSS 自己的注释点明理由：「A stop's label is the effect's name; centered because
  the segment is a choice, not a row」——即基础段是「行」，材质段才是「选择」。该变体同时把水平内缩
  收到 `0 6u`（基础是 `0 9u`，`:1055`），但两形都是 `repeat(auto-fit, minmax(0,1fr))` 的等宽列
  （`:1017-1019`），所以 6u vs 9u 只在标签触到省略号时才看得出。
- **本构建（改前偏离）**：`internal/control/control.go:47` 的段一律 `Justify(ui.Center)`，它的两个
  调用方（`internal/settingsui/settingsui.go:2860` 的 `pick` 与 `internal/terminalui/drawer.go` 的
  外观选择器）都因此居中，没有左对齐与材质专用的居中两态。
- **落地 file:line（已落地，P7-hd）**：`internal/control/control.go` 的 `segmented` 加 `glass` 形参
  （基础态 `Justify(ui.Start)` + `Space(2.25)` = 9u；材质态 `Justify(ui.Center)` + `Space(1.5)` = 6u），
  新增导出 `SegmentedGlassFill`；`internal/settingsui/settingsui.go` 的 `pick` 拆出 `pickWith`，
  新增仅材质段用的 `glassPick`/`a.glassSegmented`，General 页的材质 pick 改走 `a.glassPick`；
  别的调用方调用未改，默认回到原版的「行」。
- **验收（已过，P7-hd）**：`internal/control/segmentalign_test.go` 的
  `TestTheBaseSegmentsAreRowsAndOnlyTheGlassTrackIsCentered`——基础段标签墨迹左缘离段左边 == 9u 且
  远小于右留白；材质段左右墨迹留白相等（居中）且左留白 > 9u；两形段高仍是 30u。
  变异核查：基础态改回 `Justify(ui.Center)` → 基础段左留白 80.2、右 81.4，与 9u/「右留白 > 半宽」
  两条同时红。

### 具体待办（P7-gx 重写；第 1 条 P7-gx 已落地，余下第 2·3·4 条）

审计入口：原版终端表面 chrome 的**度量与颜色**表 `main:src/styles/terminal.css`（浮栏
`.terminal-bar`/`.terminal-bar__title`、状态点 `.terminal-bar__dot`、抽屉 `.terminal-settings*`、驻留条
`.terminal-resident`、空态 `.terminal-empty`），本构建对应 `internal/terminalui/terminalui.go` 的
`titleBar`/`statusDot`/`residentNote`/`View` 与 `internal/terminalui/drawer.go`。此前 `grep -c` 复核：
`terminal-bar__title`、`terminal-settings__row`、`terminal-setting-range`、`terminal-resident`、
`terminal-empty`、`terminal-copy-notice` 在 `docs/` 全为 0——`terminal.css` 只对照过浮栏的**词与
tooltip**（P7-fl/fm）与空态两行（P7-cm），度量与颜色从未逐条比对。依赖检查：`go.mod` 已是
**v0.5.1** = `go list -m -versions` 的最新（`api.github.com` 本轮被限流），无版本可升；只对照 `main`
（`backup/main-pre-mygo` = v0.3.14 原版树），未翻历史提交。

#### P7-gx-1 · 浮栏身份行与状态点的字级/颜色/环（**本轮落地，P7-gx**）

- **原版对照**：`main:src/styles/terminal.css:165-175`（`.terminal-bar__title { color: var(--text-secondary);
  font-size: var(--text-body); font-weight: 580; line-height: 1; letter-spacing: -0.01em }`）、
  `:148-157`（`.terminal-bar__dot { width:7px; height:7px; background: var(--accent);
  box-shadow: var(--elev-halo) }`）、`:158-163`（`.terminal-bar__dot--exited { background:
  var(--text-tertiary); box-shadow: none; opacity: 0.7 }`）；`main:src/styles/base.css:206-207`
  （`--elev-halo: 0 0 0 2px var(--accent-halo)`）、`:693`（dark `--accent-halo: rgba(143,183,255,0.18)`）、
  `:958`（light `rgba(6,74,166,0.24)`）。
- **本构建（改前偏离）**：`internal/terminalui/terminalui.go:1094-1096` 的身份文字是
  `FontSize(t.FontSize - 1)` + `TextColor(t.TextMuted)`（小一档、且是 muted 而非 secondary，无
  `letter-spacing`）；`statusDot`（`:1136-1152`）退出态落 `t.TextMuted`（亮一档）、活态完全没有
  `--elev-halo` 环，且 `theme.Tokens` 没有 `--accent-halo` 这个 token。
- **落地 file:line（已落地）**：`internal/theme/theme.go` 新增 `Tokens.AccentHalo`（palette 两个外观
  各一份，`theme_test.go` 钉值）；`internal/terminalui/terminalui.go:1095` 的身份文字改
  `FontSize(t.FontSize)` + `TextColor(tokens.TextSecondary)` + `LetterSpacing(-0.01*t.FontSize)`，
  `:1144` 退出点改 `tokens.TextTertiary`，`:1150` 活点加 `dot.Shadow(0, 0, 0, t.Space(0.5),
  tokens.AccentHalo)`（2u 环）。详见「最近几轮」P7-gx 记录。
- **验收（已过）**：`internal/terminalui/barchrome_test.go` 的
  `TestTheTerminalBarTitleIsTheReferenceSecondaryBody` 与
  `TestTheTerminalBarDotCarriesItsHaloAndWearsTertiary`（点用**中心像素**法与参考实心填充逐字节比）。
  变异核查：改回 `FontSize-1`/`TextMuted` → 两条红；退出点换回 `TextMuted` → 中心
  `{33 33 37}` ≠ `{30 30 34}` 红；去掉 `dot.Shadow` → 活点宽 6 == 裸点宽 6 红。

#### P7-gx-2 · 抽屉 slider 的宽度、读数与格宽都不是原版的数（**本轮落地，P7-gy**）

- **原版对照**：`main:src/styles/terminal.css:654-658`（`.terminal-settings { display:grid;
  grid-template-columns: repeat(auto-fit, minmax(230px, 1fr)); gap: 6px 18px }`）、`:660-675`
  （`.terminal-settings__row { gap:10px; min-height:30px }`、`.terminal-settings__label
  { color: var(--text-secondary); font-size: var(--text-body); font-weight: 560 }`）、`:677-692`
  （`.terminal-settings__control { gap:8px }`、`.terminal-setting-range { width: 110px; accent-color:
  var(--accent) }`）；读数 `main:src/styles/settings.css:854-865`（`.terminal-setting-control__value
  { color: var(--accent); font-family: "SF Mono", Menlo…; font-size: var(--text-body);
  font-variant-numeric: tabular-nums; font-weight: 600 }`）。原版的行由 `TerminalAppearance.tsx:367-380`
  的 strip 变体渲染（label + control + `<output class="terminal-setting-control__value">`）。
- **本构建（现状偏离）**：`internal/terminalui/drawer.go:72,100` 的两个 range 是
  `Width(t.Space(22))` = 88px（原版 110px = `Space(27.5)`）；`:76-78`、`:104-106` 的读数是
  `FontSize(tokens.Emphasis).TextColor(t.TextMuted)`（原版是 body 步 + accent + tabular + 600）；
  `:68` 起的格下限 `MinWidth(t.Space(60))` = 240px（原版 230px = `Space(57.5)`）。抽屉的其它度量
  （`Padding(10u,12u)`、`GapX(18u)/GapY(6u)`、`MinHeight(30u)`、label 的 body/560/TextSecondary、
  control 的 8u 沟）都已对齐，只剩这三处。
- **落地 file:line（已落地，P7-gy）**：新增 `internal/terminalui/drawer.go` 的 `drawerRangeWidth`（27.5）、
  `drawerRange`（固定 `Width`）与 `drawerReadout`（等宽栈 + body + accent + 600 + `tnum`），两个 slider
  与两处读数改走它们；`drawerCellFloor`（57.5）与 `paletteCellFloor`（95）成为常量，`drawerGrid`
  接受 `cellFloor` 参数、`drawer` 传 `t.Space(drawerCellFloor)`。
- **验收（已过，P7-gy）**：新增 `internal/terminalui/drawermetrics_test.go` 三条（活 slider 的旋钮行程
  对 27.5u/22u 参考、读数的 box 对 body/emphasis 参考 + 最强像素的蓝−红/绿−红、`drawerGrid` 在
  两 floor 下的折点与中间窗口）；变异核查：`drawerRangeWidth` → 22 / `drawerReadout` → Emphasis+Muted /
  `drawerCellFloor` → 60，三条各红。详见上文 P7-gy 记录。

#### P7-gx-3 · 浮栏悬停时缺原版的 `--elev-bar` 阴影（**本轮落地，P7-gz**）

- **原版对照**：`main:src/styles/terminal.css:99-104`——`.terminal-bar { box-shadow: none;
  transition: box-shadow var(--dur-3) ease }`，`@media (hover:hover) { .terminal-bar:hover
  { box-shadow: var(--elev-bar) } }`；`--elev-bar` 是 `main:src/styles/base.css:224-226` 的
  `0 6px 16px rgba(0,0,0,calc(0.18 * var(--elev-shadow-scale)))`——CSS 注释点名它是「把深度重新投在
  下面滚动的内容上」的轻阴影，是「这里可以拖」的暗示，不是第四级 elevation。
- **本构建（改前偏离）**：`internal/terminalui/terminalui.go` 的 `titleBar` 在 hover 时
  只把身份行与按钮 `revealed`，从不给浮栏自己加阴影；浮栏是纯透明。
- **落地 file:line（已落地，P7-gz）**：`internal/terminalui/terminalui.go:1076-1078` 的 `titleBar` 在
  `bar.Hovered()` 时加 `bar.Shadow(0, t.Space(1.5), t.Space(4), 0, ui.RGBA(0, 0, 0, 0.18))`（offset/blur
  取原版 `0 6px 16px` 折成 `Space`、alpha 取 base `0.18`），未悬停时不加；键盘焦点仍只揭示身份行、
  不抬升表面。
- **验收（已过，P7-gz）**：`internal/terminalui/barshadow_test.go` 的
  `TestTheTerminalBarLiftsItsShadowOnHover`——灰画布上「指针在浮栏上 / 指针离开」两帧同列对比：
  浮栏正下方 `Space(7)` 起的第一行悬停帧更暗（实测 184 → 158）、未悬停帧那一行是裸画布、离浮栏
  30px 外两帧无差。变异核查：删掉 `bar.Shadow` → 两帧同值红。详见上文 P7-gz 记录。

#### P7-gx-4 · 空态多画了一行原版没有的说明（**本轮落地，P7-ha**）

- **原版对照**：`main:src/App.tsx:3852-3872` 的空态只有 `<p class="terminal-empty__title">`
  （`terminal.emptyTitle`，`main:src/styles/terminal.css:346-350` 的 `--text-emphasis`）加两条
  `.terminal-empty__line`（Enter / 用户自己的返回键，`:356-368`），**没有**第二段说明。
- **本构建（现状偏离）**：`internal/terminalui/terminalui.go:736-742` 在标题之后、两条动作行之前
  多画了一句 `ui.Text(c, hint)`（`copy.Hint` = “The session has not started yet.”），与 `EmptyTitle`
  （“No terminal session yet”）语义重复，原版没有这一行。`copy.Hint` 全仓库只有这一个调用方。
- **落地 file:line（已落地，P7-ha）**：`internal/terminalui/terminalui.go` 的空态分支删掉 `hint` 变量与
  那句 `ui.Text(c, hint)`，只留 `EmptyTitle` 与两条 `emptyLine`；启动失败时唯一保留的一行是
  `a.Err.Error()`（原版这页没有启动错误面，本构建如实说出开不了会话的原因）。`internal/i18n/i18n.go`
  删 `Terminal.Hint` 字段、`internal/i18n/copy.go` 删 en/zh 两份值。
- **验收（已过，P7-ha）**：`internal/terminalui/view_test.go` 的 `TestTerminalEmptyState` 断言空态有
  `EmptyTitle`/`EmptyNew`/`EmptyBack` 且**不含** “The session has not started yet.”；
  `TestTerminalSessionErrorShows` 继续覆盖失败原因；`internal/shell/shell_test.go` 的终端快照 want 改
  `No terminal session yet`。变异核查：把 `copy.Hint` 那一行加回 → 红。

> 「具体待办（P7-gx 重写）」4 条全部落地：第 1 条 P7-gx、第 2 条 P7-gx-2（P7-gy）、第 3 条 P7-gx-3
> （P7-gz）、第 4 条 P7-gx-4（P7-ha）。**清单归零**，下一轮回到「对照原版排查偏离点」。

审计入口：原版 About 页的深链行 `main:src/settings/DeepLinkRow.tsx`（它吃的行基元
`main:src/settings/SettingsRows.tsx` 的 `SettingsAction`，以及 `main:src/styles/settings.css:682-730`
的 `.settings-reset`、`:1524-1535` 的 `.settings-deep-link__value`）。本构建对应
`internal/settingsui/settingsui.go` 的 `about`（deep-link 区，`:2583-2636`）与
`internal/settingsui/rows.go` 的 `action`（`:330-350`），词条在 `internal/i18n/copy.go`。全文档此前
从未提过 `DeepLinkRow` / `.settings-deep-link__value`（`grep -c` = 0）。依赖检查：`go.mod` 已是
**v0.5.1** = GitHub releases latest（本轮 `releases/latest` 复核），无版本可升；只对照 `main`
（`backup/main-pre-mygo` = v0.3.14 原版树），未翻历史提交。

1. **（本轮落地，P7-gi）深链复制控件是带填充的 `ui.Button`，且复制后没有全局成功提示。**
   - 原版对照：`DeepLinkRow.tsx:47-58` 的 `SettingsAction`（= `SettingsRows.tsx:200-216` 的
     `.settings-reset`）在 1.6s 内用 `copied === value` 在 lucide 的 `Check` 与 `Copy`（均 13px、
     `strokeWidth={2}`）之间切换，文本取 `settings.deepLinkCopy` / `deepLinkCopied`；`:31-36` 的
     `copy()` 成功写剪贴板后调 `onCopied(t("settings.deepLinkCopied"))`，让确认同时落进应用自己的
     toast 栈。CSS：`.settings-reset`（`settings.css:682-730`）是**无填充的 accent 文本动作**，
     `gap: 5u`、`min-height: 24u`、`padding: 0 2u`、hover 只降透明度；`.settings-reset__icon`
     宽 12u、随字级缩放。
   - 本构建（改前）：`settingsui.go:2608` 的控件是 `ui.Button(c, label)`——框架的填充按钮，既没有
     图标，也不在复制成功后通知任何地方；`Actions` 只有 `NotifyWarning`，没有 success 通道。
   - 落地 file:line（已按此落地）：`internal/launcher/glyphs.go` 新增 `glyphCopy` 与
     `Glyph("copy")`（lucide Copy 的两页形状，区别于既有 clipboard 记号）；
     `internal/settingsui/rows.go` 把 `action` 拆出 `actionWithGlyph`（可选前导图标、`gap: Space(1.25)`
     = 5u、`Space(3.25)` = 13u 图标，accent 墨色）；`internal/settingsui/settingsui.go` 的 deep-link
     行改走 `actionWithGlyph(linkCopyGlyph(copied), …)`，新增 `linkCopyGlyph` 与两个示例常量，
     `CopyText` 后调 `Actions.NotifySuccess`；`internal/settingsui` 新增 `Actions.NotifySuccess`，
     `internal/shell/notices.go` 新增 `notifySuccess`（success 类型、`successNotice` 4s 窗口），
     `internal/shell/shell.go` 把它接到 `onMain`。
   - 验收（已过）：`internal/settingsui/deeplinkcopy_test.go` 三条——
     `TestDeepLinkCopyRaisesTheSuccessToast`（复制值 + `NotifySuccess` 逐字收到 `DeepLinkCopied`）、
     `TestDeepLinkCopyUsesTheReferenceGlyphs`（`linkCopyGlyph` 两态 + 两个名字都在闭集里）、
     `TestTheDeepLinkCopyActionWearsItsIcon`（标签前 20u 带内最大通道差 ≥60；填充按钮只会是 ≤28）、
     `internal/shell/notices_test.go` 的 `TestTheSuccessNoticeCarriesItsKindAndWindow`（Type 为
     `success`、窗口为 `successNotice`、空消息不排队）。变异核查：换回 `ui.Button` 且不调
     `NotifySuccess` → 前两条当场红（`notified = []`；band delta 28）。
2. **（本轮落地，P7-gj）深链值不是原版的代码块 chip。**
   - 原版对照：`DeepLinkRow.tsx:67,79` 的 label 是
     `<code className="settings-deep-link__value">{value}</code>`；`settings.css:1524-1535` 给它
     `display:block`、`padding: 4u 8u`、`--radius-xs`、`background: var(--surface-sunken)`、
     `border: 1px solid var(--glass-control-edge)`、`color: --text-secondary`、等宽字体
     （SF Mono/Menlo/Monaco/Consolas）、`--text-caption`、`overflow-wrap:anywhere`。
   - 本构建（改前）：`settingsui.go` 的 `a.row(c, link, "", …)` 把链接当普通行 label 画成
     `--text-emphasis` 的 strong 文本（`rows.go` 的 `rowAligned`），没有 sunken 底、等宽或长串换行。
   - 落地 file:line（已按此落地）：`internal/settingsui/rows.go` 把 `rowAligned` 的行框抽成共享的
     `cardRow`（分隔线 / `cardRows` 计数 / 44u 底 / 标签列 / 尾随控件槽 / `below` 槽只此一份），
     `rowAligned` 与新的 `codeRow` 都走它；`codeRow` 只替换标签列的绘制，字号/行高/底色/边框/圆角/
     等宽字体与 4u/8u 内边距按上面 token 逐条对齐；`settingsui.go` 的 deep-link 行改走 `a.codeRow`。
   - 验收（已过）：`internal/settingsui/deeplinkcopy_test.go` 的
     `TestTheDeepLinkValueIsTheReferenceCodeChip`（暗色下 chip 底与卡片底像素差 ≥12；文字上下填充带 =
     4u）与 `TestTheDeepLinkValueUsesTheReferenceCodeFont`（等宽栈逐字常量）。变异核查：改回 `a.row`
     的普通 label → 底色断言红（差 0）。
3. **（本轮落地，P7-gk）共享文本动作的度量与交互不是 `.settings-reset` 本身。**
   - 原版对照：`settings.css:682-730` —— `min-height:24u`、`padding:0 2u`、`gap:5u`、
     `border-radius:var(--radius-xs)`、`background:transparent`、hover `opacity:.78`、
     active `:active opacity:.6`、disabled `--text-tertiary`。
   - 本构建（改前）：`rows.go:385` 的 `actionWithGlyph` 用 `Padding(Space(1), Space(1.5))` =
     4u/6u、`RadiusSM` = 9px，hover 用 `row.Background(t.SurfaceHover)` 画底色带，没有 active
     与 `min-height`；`Restore defaults`（`settingsui.go:2338` 一带）与深链复制共用它，因此两处
     一起偏离（实测：盒子 30u 高、左内缩 6u、hover 平面 delta 21）。
   - 落地 file:line（已按此落地）：`internal/settingsui/rows.go` 的 `actionWithGlyph` ——
     `MinHeight(t.Space(6))`、`Padding(0, t.Space(0.5))`、`Radius(tokens.RadiusXS)`、
     `AlignItems(ui.Center)`；hover/active 改为 `row.Opacity(0.78)`/`row.Opacity(0.6)`，次序与
     原版一致（pressed 优先）。`action`/`actionWithGlyph` 改返回 `ui.Element`，测试用
     `Handle.Bind` + `Resolve` 读盒子。
   - 验收（已过）：`internal/settingsui/actionmetrics_test.go` 的
     `TestTheTextActionsWearTheReferenceResetMetrics`（左/右内缩各 2u、盒高 23.5–25）与
     `TestTheTextActionHoverDimsTheInkOnly`（hover 后墨色最小通道上升、平面像素差 ≤2）。变异核查：
     改回 `Padding(4u,6u)`+`RadiusSM`+`SurfaceHover` → 两条当场红（inset 6/6、高 30；平面 delta 21）。

> P7-gb 清单三条已全部结清（P7-gb/P7-gd/P7-gh），本轮回到「对照原版排查偏离点」，换到从未审计的
> About 页深链行，把清单重写为上面 3 条带 file:line / 原版对照 / 验收标准的具体待办，并取第 1 条
> 落地（P7-gi）。第 2·3 条随后由 P7-gj / P7-gk 结清，本清单归零。

### 具体待办（P7-gk 重写；第 1 条 P7-gl、第 2 条 P7-gm、第 3 条 P7-gn 均已落地；清单归零）

审计入口：原版 `main:src/settings/SettingsRows.tsx` 的 `SettingsEmpty`（`:218-244`）与它的 CSS
`main:src/styles/settings.css:633-676`、`:1393`（`.session-manager > .settings-empty` 的整身接管）、
以及三个调用方 `main:src/settings/SessionsPage.tsx:142-167`、`main:src/settings/ShortcutsPage.tsx:240-252`、
`main:src/ExtensionsPanel.tsx:3177-3191`。本构建对应 `internal/settingsui/settingsui.go` 的 `sessions`
（`:1142-1152`）/`integrations`（`:1317-1321`）与 `internal/settingsui/custom.go` 的空态（`:35-45`），
词条在 `internal/i18n/copy.go`。此前只有 P7-gc 碰过 `ShortcutsPage` 的空态，且只是「回到
`SettingsEmpty` 形状」的一行；`.settings-empty` 的度量（92u/160u 底、24u padding、8u gap、图标 22px
@0.72、title `--text-secondary` 580、hint `--text-caption` 320px）全文档此前从未逐条对照。依赖检查：
`go.mod` 已是 **v0.5.1** = latest tag（本轮以 `go list -m -versions` 复核），无版本可升；只对照
`main`，未翻历史提交。

1. **（本轮落地，P7-gl）Sessions 页的空态是一行裸文本，不是原版的三态空态。**
   - 原版对照：`SessionsPage.tsx:142-167` 在 `sessions.length === 0` 时走 `SettingsEmpty`：
     `alert={error}`、图标三态（error → `AlertCircle`、loading → `RefreshCw`、默认 `SquareTerminal`，
     均 22px、`strokeWidth 1.6`、`aria-hidden`）、title 三态（`terminal.sessionsError` /
     `terminal.sessionsLoading` / `terminal.sessionsEmpty`）、hint 两态（error →
     `terminal.sessionsRefreshHint`、loading → 无、默认 `terminal.sessionsEmptyHint`）；CSS
     `settings.css:1399` 给这条列表空态 `min-height:160u`（整身接管，因为空态*就是*整页 body），
     其余走基类 92u。
   - 本构建（现状）：`settingsui.go:1144-1152` 是
     `ui.Column(...).Gap(Space(3)).Padding(Space(4)).Center()`，只有一行 `copy.SessionsNone`
     （“No sessions are running.”）与一个 `ui.Button(copy.SessionsNew)`；没有图标、没有 hint、没有
     160u 底，且词条与 `terminal.sessionsEmpty`（“No persistent sessions”/“暂无持久化会话”）不同。
   - 落地 file:line：在 `internal/settingsui` 抽出共享的 `emptyState` 绘制（供本清单三条复用），
     `sessions` 空态改走它，并新增 tri-state 图标/词条；`internal/i18n/copy.go` 补 `SessionsLoading`/
     `SessionsEmpty`/`SessionsEmptyHint`/`SessionsRefreshHint`（en/zh 逐字取 `i18n.ts:162-166,1069-1073`）；
     整身接管的 160u 底作为 `emptyState` 的一个参数（`SettingsEmpty` 的 `className="session-manager__list"`
     对应物）。
   - 验收：一条 Sessions 空页渲染测试 —— 空态区域高 ≥160u、有 22px 的图标记号、hint 在 title 下 8u、
     且 `New session` 仍是空态里的出路（原版的 `SettingsEmpty` 没有 action 槽，出路是页面 own 的
     `ui.Button`，与原版 `SessionsPage` 一致）；一条词表测试钉住四条新词逐字。变异核查：去掉 160u 底
     或图标任一 → 对应断言红。
   - **P7-gl 落地**：`internal/settingsui/empty.go`（新，共享 `emptyState` + 三态纯函数
     `sessionsEmptyFaceFor`）、`internal/settingsui/settingsui.go` 的 `sessions` 空支、
     `internal/launcher/glyphs.go` 的 `square-terminal`/`refresh` 记号、`internal/i18n` 的五条原版词
     （删自造 `SessionsNone`）。测试：`internal/settingsui/empty_test.go` 三条 +
     `internal/i18n/i18n_test.go` 的 `TestSessionsEmptyWordsMatchTheOldDictionary`。三项都与验收
     对应；三态里 loading/failed 在本构建无来源（会话列表进程内同步），如实记录在顶部 P7-gl 记录。

2. **（本轮落地，P7-gm）Integrations 页的空态是一行裸文本，不是原版的 `SettingsEmpty`。**
   - 原版对照：`ExtensionsPanel.tsx:3177-3191` 的 `EmptyState` 一律走 `SettingsEmpty`：`icon` 是
     面板给的记号、`title` 是状态行、`hint` 是过滤时的查询回显（`<strong
     className="extensions-empty__query">"{query}"</strong>`，仅 `font-weight:600`，
     `extensions.css:1435-1444`）——过滤空态也仍然报出它匹配空的查询串。基类给它 92u 底、24u padding、
     8u gap、title `--text-secondary` 580、hint `--text-caption` 320px。
   - 本构建（现状）：`settingsui.go:1317-1321` 是 `ui.Column(...).Padding(Space(4)).Center()` +
     一行 `copy.IntegrationsEmpty`，没有图标、没有 hint、没有 92u 底，title 还是 `TextMuted`
     （原版是 `--text-secondary`）。
   - 落地 file:line：`integrations` 的空态改走上面第 1 条抽出的 `emptyState`，补面板自己的记号；
     若本构建的过滤路径已有查询串（`internal/settingsui` 的 installed 过滤），把查询回显接上 hint 线
     （没有就如实不画 hint，并在文档记下取舍）。
   - 验收：一条渲染测试 —— 空态区域高 ≥92u、有图标记号、title 用 `--text-secondary` 墨色（与
     `TextMuted` 的像素差可测）；过滤到空时 hint 里逐字含查询串。变异核查：改回裸 `ui.Text` →
     高度/墨色断言红。
   - **P7-gm 落地**：`internal/settingsui/empty.go` 的共享 `emptyState` 增加 `iconSize` 参数
     （原版 `SettingsEmpty` 的 icon 是调用方给的节点：Sessions/快捷键 22px、集成列表 20px；0 → 22u）；
     `settingsui.go` 的 `integrations` 空支改走 `a.emptyState(c, "package", copy.IntegrationsEmpty,
     "", t.Space(5), 0, nil)`；`internal/launcher/glyphs.go` 新增 `package` 记号（lucide-react 1.31.0
     的 Package 路径，逐字取自该版本 tarball）；`internal/i18n` 的 `IntegrationsEmpty` en/zh 改成原版
     `settings.extensions.emptyInstalled` 的逐字文案（`i18n.ts:443,1330`）。**取舍**：原版 `EmptyState`
     还有首帧加载支与过滤空态的 hint 查询回显；本构建的安装列表是进程内同步读、集成页无过滤输入
     （原版 v0.3.14 的 `query` 参数本就无人传），两支都无来源，只画 resting 一支、不画 hint。测试见
     `internal/settingsui/integrationsempty_test.go` 两条；详见「最近几轮」P7-gm 记录。

3. **（本轮落地，P7-gn）自定义快捷键空态仍是填充按钮 + 自造度量。**
   - 原版对照：`ShortcutsPage.tsx:240-252` 的 `SettingsEmpty`：`icon={<Keyboard size={22}
     strokeWidth={1.6} aria-hidden />}`、`title=customShortcutsEmpty`、`hint=customShortcutsEmptyHint`、
     `action=<SettingsAction title=customShortcutsAddHint><span className="settings-reset__icon">＋</span>
     {customShortcutsAddFirst}</SettingsAction>`；CSS 同基类：92u 底、24u padding、8u gap、
     图标 `--text-tertiary` @ `opacity:.72`、title `--text-secondary` 580、hint `--text-caption`
     320px、action 是 `.settings-reset` 文本动作（现已有 `actionWithGlyph`），不是填充按钮。
   - 本构建（改前）：`custom.go:35-45` 用 `Gap(Space(2)).Padding(Space(4)).Center()`，图标是
     `mustGlyph("text")` @ `Space(5)`、title @ `TextMuted`、hint @ `FontSize-1`、出路是
     `ui.Button(copy.ShortcutsAddFirst)`（填充按钮，无 ＋ 记号）；title 词尾还多一个句号。
   - 落地 file:line：`custom.go` 的空态改走共享 `emptyState`，图标换 `keyboard` 记号
     （`internal/launcher/glyphs.go` 新增 lucide Keyboard）、title 改 `--text-secondary`、hint 改
     `--text-caption`、出路改 `a.actionWithGlyph(c, "plus", copy.ShortcutsAddFirst, …)`
     （`glyphs.go` 新增 lucide Plus 的 `plus` 记号）。
   - 验收：一条渲染测试 —— 空态高 ≥92u、title 墨色是 `--text-secondary`、出路是文本动作（墨色最小
     通道带内 20u 有 accent 记号、且不是填充 pill）。变异核查：把出路改回 `ui.Button` → 记号断言红。
   - **P7-gn 落地**：`internal/settingsui/custom.go` 的空支、`internal/launcher/glyphs.go` 的
     `keyboard`/`plus` 记号、`internal/i18n` 的 `ShortcutsNone`（去句号回原版）。测试：
     `internal/settingsui/customempty_test.go` 两条 + `internal/i18n/i18n_test.go` 的
     `TestCustomShortcutsEmptyWordsMatchTheOldDictionary`。变异核查：出路改回 `ui.Button` →
     记号/平面两条断言红；空态记号名改 `""` → 记号条红。详见「最近几轮」P7-gn 记录。

### 具体待办（P7-gb 重写；3 条已全部落地：第 1 条 P7-gb、第 2 条 P7-gd、第 3 条 P7-gh）

审计入口：原版 `main:src/plugin-window/DetachedPluginApp.tsx`（独立插件窗口的 React 表面，
`backup/main-pre-mygo` = v0.3.14 原版树）与它吃的两个纯模块 `main:src/plugin-window/detach.ts`、
`main:src/plugin-window/pin-entry.ts`，本构建对应 `internal/shell/detach.go`（命令输出分离）与
`internal/shell/pinned.go`（文本钉住），词条在 `internal/i18n/copy.go`。这个入口**从未进入任何
历史待办**（全文档仅一处顺带提及）。依赖检查：`go.mod` 已是 **v0.4.8** = GitHub releases latest，
无版本可升；只对照 `main`，未翻历史提交。

1. **（本轮落地，P7-gb）独立窗口的词是自造的、任务栏标题没有 app 尾巴。**
   - 原版对照：`DetachedPluginApp.tsx:120-124` 的 `` getCurrentWindow().setTitle(`${windowTitle}
     · floter`) ``（U+00B7 中点）；`detach.ts:140` 的 `PLUGIN_WINDOW_FALLBACK_TITLE = "floter
     plugin"`；词条 `i18n.ts:46-50,969-976`（`pluginWindow.fallbackTitle` “floter plugin”/
     “floter 插件”、`pluginWindow.idle` “No command pinned yet”/“尚未固定任何命令”、
     `pluginWindow.rerun` “Rerun”/“重新执行”）。
   - 本构建（改前）：`internal/i18n/copy.go:208,209,211,1060,1063` 意译成 “floter command” /
     “Re-run” / “Nothing has run yet” 与 “floter 命令” / “还没有执行过”；
     `internal/shell/detach.go` 的 `WindowOptions.Title` 与复用分支的 `SetTitle` 都用裸标签。
   - 落地 file:line：`internal/i18n/copy.go` 三条词改回原版；`internal/shell/detach.go` 新增
     `detachWindowTitleSuffix`（`" \u00b7 floter"`）与 `detachWindowTitle(label)`，窗口创建与复用
     两处改走它——**窗口内的 header 仍用裸标签**（原版的 `headerLabel` 也不带尾巴）。
   - 验收：`internal/shell/detachwords_test.go` 的 `TestDetachedWindowWordsMatchTheOldDictionary`
     （en/zh 三条词逐字）与 `TestDetachedWindowTitleCarriesTheAppTail`。变异核查：把
     `detachWindowTitle` 换回裸标签 → 标题条红；三个词任一改回旧译 → 词表条红。

2. **（已落地，P7-gd）独立窗口 bar 的关闭键不是原版的 X 图标、rerun 没有运行中禁用态。**
   - 原版对照：`DetachedPluginApp.tsx:284-315` —— bar 的两个键都是 `plugin-window__bar-button`：
     rerun 是一个文本键，`disabled={state.status === "running"}`，`title`/`aria-label` 取
     `pluginWindow.rerun`；close 是一个**图标键** `<X size={16} strokeWidth={1.8} aria-hidden>`，
     只有 `title`/`aria-label` 取 `pluginWindow.close`，键面不画文字。
   - 本构建（现状）：`internal/shell/detach.go:145-169` 的 `barButton(label, run)` 对两个键都画
     **文本**，没有 X 图形；`rerunDetached`（`:184-193`）内部用 `detached.running` 挡了一下，但
     `barButton` 没有禁用态，键面看不出运行中。
   - 落地 file:line（P7-gd）：`internal/shell/detach.go` 的 `detachedView` 的 bar——把 close 换成
     图形键（`mustGlyph`/`glyph` 里补一个 X 记号，参考 `internal/launcher/glyphs.go` 现有记号的写法），
     rerun 在 `detached.running` 时画成禁用墨色并把点击短路。`internal/i18n` 无需新词（复用
     `OutputWindowRerun`/`OutputWindowClose`，后者只留给 `aria`/tooltip）。
   - 验收：一条渲染测试，`detached.entry != nil && detached.running` 时：close 键不再画出 “Close”
     文本（换成记号），rerun 文本键的墨色 == 禁用 token（或点击不触发第二次 run）；`detached.running`
     为 false 时 rerun 键恢复可点。变异核查：把禁用态去掉 → 墨色/点击断言红。
   - 风险：mygo 的禁用态 API 与本构建的 `barButton` 手写 hover 需要对齐；`pinned.go` 的文本窗
     **没有 bar**（它走的是独立的 `PinText` 窗口），本条是否顺带给文本窗补上原版的 bar（header +
     Close 图标）是独立决定——原版两条 arm 共用同一个 bar，本构建拆成了两个窗口，留作第 2 条内的
     取舍记录。

3. **（本轮落地，P7-gh）独立窗口 body 缺 running/failed/empty 三条状态行。**
   - 原版对照：`DetachedPluginApp.tsx:322-338` 的 body 是一台状态机——`status === "running"` →
     `launcher.externalRunning`（“Running…”/“正在执行…”，`i18n.ts:37,965`）；`status === "failed"` →
     `runErrorMessage(state.failure, t)` 本地化（`src/extensions/run-errors.ts`，认不出才回落
     `failure ?? launcher.externalFailed`）；`status === "done" && view === null` →
     `launcher.externalEmpty`（“The command produced no output”/“命令没有输出”，`i18n.ts:39,967`）。
   - 本构建（现状）：`internal/shell/detach.go:170-179` 只画 `detached.view.Text`（空则
     `OutputWindowIdle`）；`rerunDetached` 在飞的整段时间 `detached.running = true` 却不改画面，
     用户看不到「正在执行」；失败由 `launcher.OutputFor` 直接拼进文本，没有 `failed` 行。
   - **P7-gh 落地**：`internal/i18n` 新增 `OutputWindowRunning`/`OutputWindowFailed`/
     `OutputWindowEmpty`（en/zh 逐字取 `i18n.ts:37-39,965-967`）；`detachedOutput` 加 `failed`/`ran`，
     `detachedBody` 是新的三态状态机（running → failed → 有文本 → ran → idle），failed 走
     `tokens.Warning`；`rerunDetached` 起跑清 failed + `Invalidate()`，`applyDetachedRun` 收 `runErr`
     并在 `view.Text == ""` 时用 `Launcher.RunStatus` 记句子（空则回落通用句）。验收与变异见顶部
     P7-gh 记录。
   - 验收（P7-gh）：`detach_test.go` 的 `TestTheDetachedBodyFollowsTheRunStateMachine` /
     `TestARerunMarksTheWindowRunningUntilTheAnswerLands` / `TestARunThatNeverStartedCarriesItsFailureSentence`
     + `detachwords_test.go` 的三态逐字。
   - 风险：`launcher.OutputFor` 已经给失败输出拼了 `OutputFailed`（“Exit code N”）文本，本条要确认
     两者不重复（原版 failed 行只在**运行管线**失败时出现，退出码非零仍是 `done`）。

> P7-ga 之后「尚未做」归零，本轮按审计入口换到**独立插件窗口**（此前从未逐字段对照），把清单重写为
> 上面 3 条带 file:line / 原版对照 / 验收标准的具体待办，并取第 1 条落地（P7-gb）；第 2 条
> （bar 的 X/禁用态）由 P7-gd 落地，第 3 条（body 的三条状态行）由 P7-gh 落地——**本清单归零**。

### 具体待办（P7-fz 重写；2 条已全部落地：第 1·2 条 P7-ga）

审计入口：原版 `main:src/settings/TerminalAppearance.tsx` 的 `card` 变体与 `main:src/styles/settings.css`
的行语言（本轮只覆盖了 segmented 行；同一批行的 palette 行与 stacked 行的字级尚未逐字段比对）。
依赖检查：`go.mod` 已是 **v0.4.8** = GitHub releases latest，无版本可升；只对照 `main`
（`backup/main-pre-mygo` = v0.3.14 原版树），未翻历史提交。

1. **（已落地，P7-ga）终端的 palette 行不是原版的「标签一行 + 全宽 select/预览一行」。**
   - 原版对照：`TerminalAppearance.tsx:271-292` 的 `terminal_theme` 行 `stacked: true`，control 是
     `.terminal-palette`（`settings.css:871-878`：`width:100%; display:flex; align-items:center;
     justify-content:space-between`），左边 `select.settings-select`（`max-width:220px`）、右边
     `TerminalPalettePreview`（`min-width:148u`）。
   - **P7-ga 落地**：`paletteRow` 改走 `a.stackedRow`，control 内 `ui.Row(...).FillWidth().
     Justify(ui.SpaceBetween)`——左 select（`Width(220)`）、右 `terminalui.PalettePreview(...)`
     固定 `t.Space(37)`（实测 148u）。验收与变异见顶部 P7-ga 记录。
2. **（已落地，P7-ga）stacked 行的标签/说明字级比尾随行小一档，原版是同一档。**
   - 原版对照：`.settings-row__label` 对 stacked 与尾随是同一条规则（`--text-emphasis`），
     `.settings-row__sublabel` 同一条 `--text-body`（`settings.css:520-546`），`SettingsRow` 不因
     `stacked` 改字级（`SettingsRows.tsx:104-135`）。
   - **P7-ga 落地**：`rows.go` 的 `stackedRow` 与 `settingsui.go` 的 `slider` 标签改 `t.FontSize+1`、
     说明改 `t.FontSize`（读数保持 `--text-body`）。验收与变异见顶部 P7-ga 记录。

> P7-fy 之后「尚未做」只剩第 3 条，本轮 P7-fz 落地后归零；按审计入口把清单重写为上面 2 条带
> file:line / 原版对照 / 验收标准的具体待办，留给下一轮。

### 具体待办（P7-fy 重写；3 条已全部落地：第 1·2 条 P7-fy、第 3 条 P7-fz）

审计入口：原版 `main:src/settings/TerminalAppearance.tsx` 的 `card` 变体（本构建对应
`internal/settingsui/settingsui.go` 的 `general` 里喂给 `control.SettingsCardAxes()` 的 rows map）
与 `main:src/settings/AboutPage.tsx` 的分组结构（本构建对应同文件的 `about`/`updateRow`）。此前
General 页只对照过分组顺序（P7-ca）与终端外观的布局分流（P7-aj），而**同一份 schema 两个面的读数
/ 提示本身**、以及 About 页的**分组**从未逐字段比对。依赖检查：`go.mod` 已是 **v0.4.8** = GitHub
releases latest，无版本可升；只对照 `main`（`backup/main-pre-mygo` = v0.3.14 原版树），未翻历史提交。

1. **（本轮落地，P7-fy）设置卡的字号/行距读数与提示跟抽屉不一致**。
   - 原版对照：`TerminalAppearance.tsx:190-195` 的字号行 `value: ${fontSize} px`、
     `sublabel: variant === "card" ? t("settings.terminalAppearanceHint") : undefined`；
     `:210-215` 的行距行 `value: ${lineHeight.toFixed(2)}×`。抽屉（`variant === "strip"`）不画
     sublabel，两个读数却是同一个字符串。
   - 本构建（改前）：`internal/settingsui/settingsui.go` 的 `AxisFontSize` format = `"%d"`（漏 ` px`）、
     `AxisLineHeight` format = `"%.1f\u00d7"`（漏第二位），字号行 sublabel 传空；抽屉
     （`internal/terminalui/drawer.go:69-71`）已经是 `14 px` / `1.20×`，两面各说一套。
   - 落地 file:line：`settingsui.go` 的两处 format + 字号行传 `copy.TerminalHint`。
   - 验收：`internal/settingsui/view_test.go` 的 `TestTerminalCardReadoutsMatchTheDrawer`
     （`14 px`、`1.20×` 与 `TerminalHint` 三项都在）。变异核查：format 改回 `%d` / `%.1f×` → 红。
2. **（本轮落地，P7-fy）About 页的更新行没有原版自己的分组**。
   - 原版对照：`AboutPage.tsx:62-70` 的更新行在 `<section>` 里，标题 `settings.group.update`
     （`i18n.ts:213,1115`），卡带 `label={t("settings.group.update")}`。
   - 本构建（改前）：`about` 把 `updateRow` 直接放在一张无标题卡里，与身份行同层。
   - 落地 file:line：`internal/i18n` 新增 `GroupUpdate`；`about` 改 `a.section(c, copy.GroupUpdate, "", …)`。
   - 验收：`view_test.go` 的 `TestAboutUpdateRowFollowsTheOldStateMachine` 断更新行位于
     `en.GroupUpdate` 标题下方。变异核查：换回裸 `a.card` → 红。
3. **（已落地，P7-fz）Appearance 卡的三个分段行未按原版的 `stacked` 全宽布局绘制**。
   - 原版对照：`GeneralPage.tsx:371-395` 的 Theme / Language / UI scale 三行都是 `<SettingsRow
     stacked …>`（标签与说明一行、分段控件在**下一行全宽**），与 `TerminalAppearance.tsx:388` 的
     `stacked={row.slider === true || row.stacked === true}` 同一形状；CSS `.settings-row--stacked`
     是 `grid-template-columns: minmax(0, 1fr)`（`src/styles/settings.css:557-577`），控件占满整行。
   - 本构建（现状）：`internal/settingsui/settingsui.go:2677-2690` 的 `a.pick` 把分段控件放在标签同行的
     control 槽里（`rows.go:174-176` 的 `rowAligned`），所以 Theme / Language / UI scale 的控件被
     挤在右端而不是占一行；`a.cardRows` 的规则与行分隔依赖 `rowAligned`，需要新增一个 `stackedRow`
     （仿 `a.slider` 的行计数与分隔）而不重写整张卡。
   - 落地 file:line（建议）：`rows.go` 新增 `func (a *App) stackedRow(c, label, sublabel string, control func()) bool`
     ——调 `a.cardRows` 计数与分隔，标签/说明一列，`control` 在下一行 `FillWidth()`；`settingsui.go`
     的 `a.pick` 改用它（三个调用点带上 `copy.ThemeHint`/`copy.LanguageHint`/`copy.ScaleHint` 仍在行内）。
   - 验收：`view_test.go` 新增 `TestAppearanceChoicesStackUnderTheirLabels`——取 Theme 分段
     （`"Dark"` 段）与 Theme 标签（`copy.Theme`）的 rect，断言分段段的 `Y` **不小于**标签行的底
     （即在其下方），且分段控件的宽度接近卡宽（≥ 标签行宽的一半）而不再贴右端；对比 `HideOnBlur`
     这类非 stacked 行，其控件仍与标签同一行（`|control.Y-lagel.Y|` 小）。变异核查：把 `a.pick` 改回
     `rowAligned`（控件同行）→ 第一条断言红。
   - 风险：`a.pick` 也被 Integrations 页的 PATH 开关……（核实为否：`grep -n "a\.pick\("` 只有
     General 页五处，其中 Glass/AppIcon 也是分段，同样按原版 stacked 处理）。
   - **P7-fz 落地**：`rows.go` 的 `stackedRow` + `control.SegmentedFill`；`pick` 改走
     `a.stackedRow`；验收与变异见顶部 P7-fz 记录。

> P7-fx 之后「尚未做」再次只剩历史清单，本轮按审计入口换到「终端外观卡的读数 + About 分组 + Appearance
> 分段行布局」，把清单重写为上面 3 条带 file:line / 原版对照 / 验收标准的具体待办，并取第 1·2 条
> 落地（P7-fy）；第 3 条（Appearance 分段行的 stacked 布局）是下一轮最高优先级。

### 具体待办（P7-fw 重写；第 1 条 P7-fw 落地，第 2·3 条本轮 P7-fx 落地；清单归零）

审计入口：原版 `main:src/settings/SessionsPage.tsx` 的会话行状态词与行交互（`state`
`:173-177`、`session-state` 类 `:189`、整行 `onActivate` `:197`），本构建对应
`internal/terminalui/terminalui.go` 的 `NoteOf`/`LabelOf`、`internal/settingsui/settingsui.go` 的
`sessions`/`sessionLines`、`internal/settingsui/rows.go` 的 `rowLines`。此前 P7-aj 只对照了排序轴、
行事实（cwd/尺寸/起始时间）与关闭武装，**状态词与行激活本身未逐字段比对**。依赖检查：`go.mod`
已是 **v0.4.8** = GitHub releases latest，无版本可升；只对照 `main`（`backup/main-pre-mygo` =
v0.3.14 原版树），未翻历史提交。

1. **（本轮落地，P7-fw）Sessions 行把退出码画在干净退出上**。
   - 原版对照：`SessionsPage.tsx:173-177` 的 `state` 为
     `session.exited ? t("terminal.sessionExited") + (session.exitCode !== 0 ? ` · exit ${session.exitCode}` : "") : session.attached ? t("terminal.sessionAttached") : t("terminal.sessionDetached")`；
     词条 `i18n.ts:179,1085`（“Exited”/“已退出”）。终端表面的 resident note 是**另一条**词
     `terminal.processExited`（“Process exited · code {code}”，`i18n.ts:171,1078`）。
   - 本构建（改前）：`internal/terminalui/terminalui.go:338-356` 的 `NoteOf` 对任何结束的会话都返回
     `copy.ProcessExited(session.ExitCode)`，于是干净退出画成 “Process exited · code 0”；
     `internal/i18n` 没有 `SessionExited`，两条词在 Go 侧被并成一条。
   - 落地 file:line：`internal/i18n/i18n.go` 新增 `Terminal.SessionExited`（en/zh 取原版两条词）；
     `internal/terminalui/terminalui.go` 的 `NoteOf` 仅在 `ExitCode != 0` 时拼
     `" · exit " + strconv.Itoa(code)`，0 只返回 `SessionExited`；`ProcessExited` 继续只服务
     `residentNote`。
   - 验收：`internal/terminalui/sessions_test.go` 的
     `TestSessionNoteNamesTheExitCodeOnlyWhenItIsNotZero`（en/zh：0 → “Exited”/“已退出”，3 →
     “Exited · exit 3”/“已退出 · exit 3”，运行中两态与越界空）；`internal/shell/scripttool_test.go`
     的 `TestSessionsListReportsEverySession` 追加 ended 行 Note 逐字断言；
     `internal/i18n/i18n_test.go` 的 `TestTerminalBarWordsMatchTheOldDictionary` 补 en/zh
     `SessionExited`。变异核查：把 `NoteOf` 改回 `ProcessExited` → 新测试红。
2. **（本轮落地，P7-fx）exited 行没有原版的 `session-state--exited` 状态色**。
   - 原版对照：`SessionsPage.tsx:188-191` 把状态词单独放在
     `<span className={session.exited ? "session-state session-state--exited" : "session-state"}>`，
     由 `src/styles/settings.css` 的 `.session-state--exited` 给它一个弱化的退出色；行内其它
     meta（尺寸/时间）仍是普通 muted。
   - 本构建（改前/现状）：`internal/settingsui/settingsui.go:1237-1260` 的 `sessionLines` 把状态词与
     size/created 用 `" · "` 拼成一个字符串数组，`internal/settingsui/rows.go:131-133` 的
     `rowLines` 对每一行一律 `TextColor(t.TextMuted)`——**状态词与其它 meta 同色**，exited 行没有
     自己的色。
   - 落地 file:line（建议）：`settingsui.Session` 增加 `Exited bool`（shell 投影 `Running == false`），
     `sessionLines` 改为返回带色标的状态段（或新增 `rowStateLines` 只给状态行 `TextMuted` 之外的
     一个 token）；`rows.go` 的 `rowLines` 增加「每个 sublabel 可选色」的口子。
   - 验收（已过）：`internal/settingsui/sessionsstate_test.go` 的
     `TestSessionStateWordsCarryTheOldStateColours`——逐词取墨色（最远离底色的像素，抗锯齿边缘
     不参与）：running/exited 状态词 RGB 不同、running 状态词 ≠ muted 尺寸词、exited 状态词 = 同色
     muted 尺寸词、两行尺寸词相同。框架的文本节点**能**逐段上色（每段是一个 `ui.Text`），无需
     「已知差距」条目。变异核查：把段的颜色改回 `t.TextMuted` → 两条像素断言立刻红。
3. **（本轮落地，P7-fx）整行点击不激活会话，只有显式的 Show 按钮**。
   - 原版对照：`SessionsPage.tsx:197-198` 的 `SettingsRow` 带
     `onActivate={resumable ? () => onResume(session) : undefined}`，即 **resumable 的行点哪里都
     恢复会话**；`Play` 按钮是同一动作的显式控件。
   - 本构建（现状）：`internal/settingsui/settingsui.go:1175-1200` 只把 `rowLines` 的
     `clicked` 返回值丢弃，恢复只由 `copy.SessionsShow` 按钮触发；`rows.go:174-176` 已经把
     `names.Clicked()` 记进 `clicked`，但没有接线。
   - 落地 file:line（建议）：`sessions` 里接住 `a.rowLines(...)` 的返回值，`session.Running` 且
     非 `session.Active` 时调 `a.Actions.SelectSession(session.Index)`；`Active`/exited 行保持不响应
     （原版 `resumable = !session.exited && !session.attached`）。
   - 落地（已按此落地）：`settingsui.go` 的 `sessions` 接住 `a.rowLines(...)` 的返回值，
     `resumable := session.Running && !session.Active` 为真且 `Actions.SelectSession != nil` 时调
     `SelectSession(session.Index)`；`Active` 行与 exited 行保持不响应（原版
     `resumable = !session.exited && !session.attached`）。
   - 验收（已过）：`internal/settingsui/sessionsstate_test.go` 的
     `TestClickingAResumableSessionRowResumesIt`（点 resumable 行标题 → `SelectSession(1)`；再点
     active 行与 exited 行 → 仍只有一次调用；显式 Show 按钮仍可用并再触发一次）。变异核查：把
     `resumable` 放宽为 `true` → 测试在「active/exited 行不响应」处红。

> P7-fv 之后「尚未做」再次只剩历史清单，本轮按审计入口换到 Sessions 行，把清单重写为上面
> 3 条带 file:line / 原版对照 / 验收标准的具体待办，并取第 1 条落地（P7-fw）；第 2·3 条由
> 下一轮 P7-fx 一并结清。

### 具体待办（P7-fv 重写；第 1·2·3 条本轮 P7-fv 一并落地；清单归零）

审计入口：原版 `main:src/settings/AboutPage.tsx`（`AboutPage` `:35-113`）与 `main:src/App.tsx`
的更新状态（`:548-552`、`:2763-2809`），本构建对应 `internal/settingsui/settingsui.go` 的
`updateRow`/`about`、`internal/shell/update.go` 的 `checkForUpdates`/`installUpdate`、
`internal/i18n` 的 `Settings.Update*`。此前 About 页只做过「有无更新源」的收尾，更新行本身
从未逐字段对照。依赖检查：`go.mod` 已是 **v0.4.8** = GitHub releases latest，无版本可升；
只对照 `main`（`backup/main-pre-mygo` = v0.3.14 原版树），未翻历史提交。

1. **（本轮落地，P7-fv）更新行不是原版的一行状态机**。
   - 原版对照：`AboutPage.tsx:71-111` 是**一行** `SettingsRow`：label
     `` `${t("settings.currentVersion")}: v${appVersion}` ``，sublabel 为
     `updateFailed ? t("settings.updateFailed") : updateInfo ? `${t("settings.latestVersion")}: v${updateInfo.version}` : t("settings.upToDate")`，
     control 按 `updateProgress → updateDownloading → updateFailed → updateInfo` 四态选进度条 /
     禁用的 `settings.installing` / `settings.retry` / `settings.downloadUpdate`；词条在
     `i18n.ts:416-422,1303-1309`。`App.tsx:2763-2769` 在挂载时自动 `check()`，页面没有
     「检查更新」按钮。
   - 本构建（改前）：`internal/settingsui/settingsui.go:2396-2420` 的 `updateRow` 是
     「Check for updates」按钮 + 可选「Download and install」+ 自由句 `UpdateStatus`；
     `internal/shell/update.go:25-88` 只把本地化字符串塞进 `UpdateStatus`/`UpdateReady`，
     版本、下载中、失败、进度都不可区分；启动时不检查。
   - 落地 file:line：`internal/settingsui/settingsui.go` 新增 `UpdateState`/`UpdateProgress` 与
     `updateRow` 四态；`internal/shell/update.go` 改为写状态机并在 `Start()`（`shell.go:1182`）
     自动 `checkForUpdates()`；`internal/i18n` 换成原版八条词。
   - 验收：`internal/settingsui/view_test.go` 的
     `TestAboutUpdateRowFollowsTheOldStateMachine`（五行/四态逐字与按钮落点）；变异核查：把
     control 分支换回「Check for updates」→ 测试红。
2. **（本轮落地，P7-fv）下载进度没有原版的字节条与读数**。
   - 原版对照：`AboutPage.tsx:45-48,75-88` 的 `updatePercent`（`Math.min(100, downloaded/total*100)`）、
     `formatBytes`（`:9-15`，B/KB/MB/GB，整数字节、其余一位小数、`0 B`）与
     `` `${Math.round(updatePercent)}% · ${formatBytes(downloaded)} / ${formatBytes(total)}` ``，
     无 total 时只显示 `formatBytes(downloaded)`；CSS 见 `src/styles/settings.css:1570-1599`。
   - 本构建（改前）：`UpdateInstalling func(percent int) string` 只给一句
     “Downloading… N%”，没有条、没有 bytes/total，也没有 `formatBytes`。
   - 落地 file:line：`internal/settingsui/settingsui.go` 新增 `updateProgress`、
     `updateProgressLabel`、`formatBytes`（逐字复刻原版四单位与取整规则）。
   - 验收：`TestUpdateProgressFormatting`（`formatBytes` 七例、两条 label、越界 100% 夹取）；
     变异核查：把 `formatBytes` 换回 `strconv.Itoa` → 测试红。
3. **（本轮落地，P7-fv）失败后没有原版的 Retry，成功后也不重启**。
   - 原版对照：`AboutPage.tsx:93-100` 失败态画 `settings.retry` 按钮，点击调
     `onDownloadUpdate`（即 `App.tsx:2771-2809` 的 `downloadAndInstallUpdate`：重新 `check()`，
     失败置 `updateFailed`，成功 `await update.install(); await relaunch()`）。
   - 本构建（改前）：`internal/settingsui/settingsui.go:2415-2417` 失败只画一行自由文本；
     `internal/shell/update.go:49-78` 成功后只写 “installed; restarts on the next launch”，
     不重启。
   - 落地 file:line：`internal/shell/update.go` 的 `installUpdate` 先重查再装、失败态可重试、
     成功后 `relaunchApp()`（新增 `Options.Relaunch` 注入缝，默认 `mygo.App.Relaunch`）。
   - 验收：`TestAboutUpdateRowFollowsTheOldStateMachine` 的 Retry 分支；`internal/shell` 侧由
     注入缝保持可测（成功路径不替换测试进程）。变异核查：删掉 `installUpdate` 的重查 → 无 feed
     测试不再落到 `Disabled`。

> P7-fu 之后「尚未做」再次只剩历史清单，本轮按审计入口换到 About 页更新行，把清单重写为上面
> 3 条带 file:line / 原版对照 / 验收标准的具体待办，并**三条一并落地**。

### 具体待办（P7-fs 重写；第 1 条 P7-fs、第 2 条 P7-ft、第 3 条 P7-fu 均已落地；清单归零）

审计入口：原版 `main:src-tauri/src/commands/actions.rs`（`open_url` `:44-52`、`is_web_url`
`:57-68`、`open_path` `:73-98`、`resolve_path` `:107-135`、`run_silent_command` `:137-160`、
`system_spawn_detached` `:162-...`），本构建对应 `internal/shell/launch.go` 的
`openPath`/`openTerminalLink`、`internal/shell/shell.go` 的 `openURL`/`openHomepage`、
`internal/launcher/output.go` 的 `runOutputRow`、`internal/browser/target.go` 的 `OpenURL`。
前几轮已落地：`system_power`（P7-fr）与 `drops.rs` 的路径规范化（P7-cz）。依赖检查：`go.mod`
已是 **v0.4.8** = GitHub releases latest，无版本可升。只对照 `main`
（`backup/main-pre-mygo` = v0.3.14 原版树），未翻历史提交。

1. **（本轮落地，P7-fs）通用 `open_url` 没有原版的 web 协议 allow-list**。
   - 原版对照：`src-tauri/src/commands/actions.rs:44-68` 的 `open_url` 先用 `is_web_url` 校验，
     只有 `http://`/`https://`/`ftp://`（大小写不敏感）才 `spawn_opener`；否则返回
     `settings.extensions.openNonWebUrl:{url}`（`i18n.ts:851,1706`）。调用点包括 action bar 的
     URL 行、插件 output 的 `open` 动作、扩展 homepage。
   - 本构建（改前）：`internal/shell/shell.go:2522-2535` 的 `openURL` 对空 browserID 直接
     `mygo.Shell.OpenExternal(url)`；`openHomepage`（`:1742-1753`）与
     `internal/launcher/output.go:359-364` 的 output `open` 动作都走它，`file://`/`javascript:`/
     `mailto:` 一律交给系统 opener。`internal/launcher/actionbar.go:26` 的 `URLQuery` 只在
     action bar 这一条路挡住，另外两条没有。
   - 落地 file:line：`internal/shell/weburl.go`（新，`webSchemes`/`isWebURL`）；
     `internal/shell/shell.go` 的 `openURL` 与 `openHomepage`；`internal/launcher/output.go` 的
     `runOutputRow`；`internal/i18n/i18n.go` 与 `copy.go` 的 `Settings.OpenNonWebURL`。
   - 验收：`internal/shell/weburl_test.go` 的 `TestIsWebURLMatchesTheOldAllowList`、
     `TestHomepageRefusesANonWebURL`、`TestBrowserSpecificOpenKeepsItsOwnSchemes`；
     `internal/launcher/outputopen_test.go` 的 `TestOutputOpenRefusesWithoutClosing`；
     `internal/i18n/i18n_test.go` 的 `TestOpenNonWebURLWordMatchesTheOldDictionary`。
2. **（本轮落地，P7-ft）`open_path` 不解析 `~`/相对路径，也不要求路径存在**。
   - 原版对照：`actions.rs:73-98` 的 `open_path` 先 `resolve_path(&path, &home)`，再
     `resolved.exists()`，不存在返回 `Path does not exist: …`；`resolve_path`（`:107-135`）只把
     整段 `~`/`~/…` 解析成 home，`~alice` 与相对路径按 `home.join` 处理。
   - 本构建（改前）：`internal/shell/launch.go:23-40` 的 `openPath` 直接
     `spawn.Program(systemOpener(), path)`；`openTerminalLink`（`:86-107`）对 `~/…` 只做
     `HasPrefix` 判断就原样交给 opener，路径不存在也静默启动。
   - 落地 file:line（已按此落地）：`internal/shell/launch.go` 新增 `resolvePath(path, home)`；
     `openPath` 先取 home（`a.homeDir`，空则 `os.UserHomeDir`，取不到即拒）→ `resolvePath` →
     `os.Stat`，不存在发 `OpenPathFailed` 且不触达 opener；注入的 `OpenPath` 命令收到解析后的路径。
     `openTerminalLink` 不用改（解析与存在性集中在 `openPath`）。
   - 验收（已过）：`internal/shell/launch_test.go` 的 `TestOpenPathResolvesAndRequiresExistence`、
     `TestResolvePathMatchesTheOldResolver`、`TestTerminalLinkResolvesTilde`。变异核查：删掉存在性
     `os.Stat` → 第一条立刻红。
3. **（本轮落地，P7-fu）homepage 的非白名单类打开失败仍然只记日志**。
   - 原版对照：`ExtensionsPanel.tsx:2577-2580` 的 `void invoke("open_url", …).catch((error) => {
     onNotify("error", errorMessage(error, t)); })`——homepage 的任何打开失败都会成为一条 error
     通知，不只是非 web 那一种。
   - 本构建（改前）：`internal/shell/shell.go:1742-1753` 的 `openHomepage` 只对
     `errors.Is(err, errNonWebURL)` 发 notice，其余错误仅 `log.Printf`，用户看不到。
   - 落地 file:line（已按此落地）：`internal/shell/shell.go` 的 `openHomepage` 任何错误都
     `queueNotice`（默认 `err.Error()`，非 web 时换 i18n `Settings.OpenNonWebURL(url)`）；
     `Options.OpenURL`/`openURLCommand` 改为 `func(browserID, url string) error` 以便测试注入普通
     spawn 失败。
   - 验收（已过）：`internal/shell/weburl_test.go` 的 `TestHomepageReportsAnOpenerFailure`
     （注入 `opener exploded` 后恰有一条 error 通知且标题含该串）；
     `TestHomepageRefusesANonWebURL` 继续覆盖非 web 本地化与成功安静。变异核查：把错误分支改回
     只处理 `errNonWebURL` → 新测试立刻红。


审计入口：原版 `main:src-tauri/src/commands/config.rs` 的写盘路径（`write_settings_to`
`:1473-1491`、`write_settings_file` `:1493-1506`）与电源命令（`main:src-tauri/src/commands/system.rs`
的 `system_power` `:21-72`、`run_first` `:72-118`），本构建对应 `internal/settings/settings.go` 的
`Save`/`Load` 与 `internal/shell/power.go` 的 `runPower`/`powerCommands`。前几轮已落地 settings 的
解析、规范化、未知键保留、失败回写落与 180ms 去抖（P7-fn/P7-fo），下面两条是本轮逐函数比对后新查出的
真实缺口。依赖检查：`go.mod` 已是 **v0.4.8** = GitHub releases latest，无版本可升。只对照 `main`
（`backup/main-pre-mygo` = v0.3.14 原版树），未翻历史提交。

1. **（本轮落地，P7-fq）settings 写盘不是原子替换，也从不刷新恢复副本**。
   - 原版对照：`src-tauri/src/commands/config.rs:1473-1506` 的 `write_settings_to` 先把完整内容写进
     `settings.json.backup` 并 `sync_directory`，再用 `write_settings_file` 把同一份内容写进
     `settings.json`；`write_settings_file` 用同目录 `NamedTempFile` + `persist` 原子替换，并在
     `persist` 前 `flush`/`sync_all`。`load_settings_from`（`:945-970`）在 primary 读不到或解析失败时
     读这个备份，所以恢复副本始终等于最近一次成功保存的内容。
   - 本构建（改前）：`internal/settings/settings.go:474-485` 的 `Save` 直接
     `os.WriteFile(path, data, 0o644)`——没有临时文件、没有 rename、没有 fsync，也没有更新
     `settings.json.backup`；`Load`（`:256-275`）虽然会回退读备份，但备份只在首次迁移时写一次。
   - 落地 file:line（已按此落地）：`internal/settings/write.go`（新）的
     `writeFileAtomic(path, data, mode)`；`internal/settings/settings.go` 的 `Save` 先写
     `path+backupSuffix` 再写 primary，两者都是 0600 权限、都走原子替换。
   - 验收（已过）：`internal/settings/write_test.go` 两条——
     `TestSaveWritesTheRecoveryCopyBesideThePrimary`（主文件与备份字节相同、0600、无临时文件残留）、
     `TestSaveRefreshesTheRecoveryCopyOnEveryWrite`（连写两次后截断 primary，`Load` 恢复第二次的值）；
     `internal/settings/write_unix_test.go` 的 `TestWriteFileAtomicLeavesTheOldFileOnFailure`
     （目标目录不可写时返回错误且旧文件仍在）。
     变异核查：把 `Save` 改回 `os.WriteFile` → 前两条立刻红。

2. **（本轮落地，P7-fr）电源动作只看 spawn 成功，不等命令的退出结果**。
   - 原版对照：`main:src-tauri/src/commands/system.rs:21-118` 的 `system_power` 调 `run_first`：对每个
     候选 `Command::output()` 加 30s 超时；**只有** `ErrorKind::NotFound` 才落到下一个候选，候选一旦
     启动，非零退出（polkit/loginwindow 拒绝）或超时都原样返回错误，不再尝试更特权的后备命令。
   - 本构建（改前）：`internal/shell/power.go` 的 `runPower` 用 `spawn.Program`（fire-and-forget）逐个
     候选尝试，返回 nil 的条件只是「spawn 成功」；一个启动后立刻被 polkit 拒绝的 `systemctl` 会被报成
     成功，而一次权限类 spawn 错误又会继续落到 `reboot`，与「启动后拒绝不降级」的原版规则相反。
   - 落地 file:line（已按此落地）：`internal/shell/power.go` 的 `runPower` 抽成
     `runPowerCandidates(candidates, timeout)`，用 `context.WithTimeout`（`powerActionTimeout` = 30s）+ 
     `exec.CommandContext` 运行候选、单独缓冲 stderr；`errors.Is(err, exec.ErrNotFound)` 才 `continue`，
     超时返回 `timed out while requesting power action`，非零退出返回 `program: exit code N: stderr`。
     `spawn.Program` 仍留给真正的 fire-and-forget 路径。
   - 验收（已过）：`internal/shell/power_test.go`（新，`//go:build unix`）六条——候选一缺失时落到候选二
     （`sh -c "exit 0"`）；候选一启动后 `exit 7` 且 stderr 有「permission denied」时返回错误、不跑候选二
     （候选二写 sentinel 文件断言不存在）、错误串含两者；挂起工具被 20ms 超时切断；无 stderr 的 `exit 3`
     仍报「exit code 3」；未知动作被拒；空候选表报「not supported」。变异核查：换回 `spawn.Program` 的
     「spawn 成功即返回 nil」→「refusal」与「timed out」两条立刻红。

### 具体待办（P7-fp 重写；第 1·2 条均由本轮 P7-fp 落地；清单归零）

审计入口：P7-fe 收尾时如实记录的最后一处有意偏离——「system/package 删完的通知是
`disconnectedNotice`/`packageRemovedNotice`，本构建两条都走 shell 的 `IntegrationRemoved(name)`，
`UninstallIntegration(id, name)` 的签名不带 kind，改它要动跨包接口，留待后续单独一轮」。本轮把这句
残留重写成 2 条带 file:line 的具体待办并全部落地。只对照 `main`（`backup/main-pre-mygo` = v0.3.14
原版树），未翻历史提交。

1. **（本轮落地，P7-fp）纯删除动作的完成通知按 system/package 说话**。
   - 原版对照：`src/ExtensionsPanel.tsx:1286-1304` 的 `confirmRemoval` 对 system/package 调
     `extensions_uninstall {removeData:false}`，成功后 `showSuccess(t(kind === "system" ?
     "settings.extensions.disconnectedNotice" : "settings.extensions.packageRemovedNotice", {name}))`
     （词条 `src/i18n.ts:546-547,1431-1432`）。
   - 本构建（改前）：`internal/shell/shell.go:761-775` 的 `UninstallIntegration` 回调签名是
     `func(id, name string)`，成功一律 `c.IntegrationRemoved(name)`；settingsui 的
     `removeConfirmation`（`internal/settingsui/settingsui.go:4203-4204`）拿得到 `removalKind()` 却
     传不过去。
   - 落地 file:line：`internal/settingsui/settingsui.go` 的 `Actions.UninstallIntegration` 改为
     `func(id, name, kind string)`、调用点补 `integration.removalKind()`；`internal/shell/shell.go`
     新增 `integrationRemovalNotice(copy, kind, name)` 纯函数，system →
     `IntegrationDisconnected`、package → `IntegrationPackageRemoved`。
   - 验收：`internal/shell/removalnotice_test.go` 的
     `TestIntegrationRemovalNoticeFollowsTheKind`（四 kind 纯映射）；`internal/i18n/i18n_test.go` 的
     `TestRemovalNoticesMatchTheOldDictionary`（en/zh 逐字）；
     `internal/settingsui/integrationremoval_test.go` 的
     `TestOnlyOwnedKindsOpenTheComponentizedDialog` 钉住 system 行传 `RemovalSystem`。

2. **（本轮落地，P7-fp）组件化删除动作的完成通知按 custom/npm 说话**。
   - 原版对照：`src/ExtensionsPanel.tsx:1320-1340` 的 `confirmComponentizedUninstall` 成功后
     `showSuccess(t(kind === "custom" ? "settings.extensions.customDeleted" :
     "settings.extensions.uninstalledNotice", {name}))`（词条 `src/i18n.ts:545,547,1430,1432`）。
   - 本构建（改前）：`internal/shell/shell.go:738-760` 的 `UninstallComponents` 回调同样只有
     `(id, name, components)`，成功一律 `c.IntegrationRemoved(name)`；`internal/settingsui/uninstall.go:86`
     的对话框确认拿得到 `integration.removalKind()` 却传不过去。
   - 落地 file:line：`Actions.UninstallComponents` 改为 `func(id, name, kind string, components)`，
     `uninstall.go` 补 `integration.removalKind()`；shell 共用同一 `integrationRemovalNotice`，custom →
     `IntegrationCustomDeleted`、npm → `IntegrationUninstalled`。
   - 验收：同上 shell 纯映射；`TestAnNpmRowGoesStraightToTheComponentizedDialog` 钉住 npm 行传
     `RemovalNPM`。

> **清单状态（P7-fp 重写）**：两条均落地，**清单归零**。下一轮回到「对照原版排查偏离点」，重新
> 写实清单。

### 具体待办（P7-fn 重写；第 1 条 P7-fn、第 2 条 P7-fo 均已落地；清单归零）

审计入口：原版 `main:src/settings-persistence.ts` 与 `main:src/hooks/useSettings.ts` 的设置落盘/失败
路径，本构建对应 `internal/settings/store.go` 的 `Store`、`internal/settingsui/settingsui.go` 的
`set()` / `slider()`。逐函数核对结论：`createSettingsHydration`（原版 `settings-persistence.ts:36-73`）
在本构建不适用——设置在任何视图构建前已在 `OpenStore` 读完，不存在「读盘未完成时编辑」的窗口；
`createSerialSettingsWriter`（`:75-88`）由 `Store` 的互斥锁 + 同步 `Save` 天然等价（写序不会倒挂）；
`normalizeSettingsPage`（`:13-17`）已由 `internal/settings/plugins.go:216-223` + `settingsui` 的
页面路由覆盖。下面两条是本轮新查出的真实缺口。依赖检查：`go.mod` 已是 **v0.4.8** = GitHub
releases latest（release notes 的 `ui/editor` 用点已由 P7-ev 的 F-new 落地），无版本可升。只对照
`main`（`backup/main-pre-mygo` = v0.3.14 原版树），未翻历史提交。

1. **（本轮落地，P7-fn）设置写入被拒时，内存里仍保留被拒值；原版回落到上次确认的快照**。
   - 原版对照：`src/hooks/useSettings.ts:220-236` 的失败分支——写失败后重读一次权威设置
     （`get_settings`），`confirmedSettings.current = confirmed`，再
     `rollbackRejectedSettings(settingsRef.current, next, confirmed)` 把仍等于被拒值的字段写回确认值；
     `src/settings-persistence.ts:90-98` 的 `rollbackRejectedSettings` 只回落
     `Object.is(current[field], attempted[field])` 的字段，因此不会覆盖那之后的新编辑。
     `settingsSaveFailed` 只负责 5s 横幅（P7-av）。
   - 本构建（改前）：`internal/settings/store.go:55-81` 的 `Store.Update` 在 `Save` 出错后仍把
     `st.current = next` 留在内存里并照常通知监听器；`internal/settingsui/settingsui.go:2841-2846`
     的 `set()` 只置横幅。结果：文件没写成，界面和之后的读都当成新值——与 P7-av 的横幅说的
     「改动没落盘」互相矛盾。
   - 落地 file:line（已按此落地）：`Store.Update` 先算 `previous`/`next`，只在 `Save` 成功后才把
     `st.current = next` 并通知监听器；`Save` 失败时保持 `previous`、记 `LastError`、不通知、返回
     错误。`settingsui.set` 与 `savealert.go` 的注释同步改成「回落后由横幅报错」。
   - 验收（已过）：`internal/settings/store_test.go` 的 `TestStoreRollsBackWhenTheWriteFails`（失败后
     快照与写前相等、`LastError` 仍记、监听器一次都不跑）；`internal/settingsui/savealert_test.go`
     的 `TestTheSaveFailureBannerAppearsAndExpires` 改成断言失败写回落后 `HideOnBlur` 与 `before`
     相等且横幅仍出现。变异核查：把 `Store.Update` 改回「失败也 `st.current = next`」→ 两条立刻红。

2. **（本轮落地，P7-fo）滑块的落盘没有原版的 180ms 去抖**。
   - 原版对照：`src/hooks/useSettings.ts:149` 的 `SETTINGS_DEBOUNCE_MS = 180`；`changeOpacity`
     （`:396-412`，两个透明度滑块）、`changeFontSize`（`:444-459`）、`changeTerminalLineHeight`
     （`:486-501`）与 `changeCommandAlias`（`:515-530`）都先把新值上屏、清掉旧 timer，再
     `setTimeout(..., 180)` 才 `persistSettings()`；拖动一次不会写几十遍文件。
   - 本构建（现状）：`internal/settingsui/settingsui.go:2775` 的 `ui.Slider(...).Changed()` 每帧命中
     一次就 `apply(v)` → `a.set(...)` → `Store.Update` → 同步 `Save`；
     `settingsui.go:2542/2548/2612/2621` 四处滑块（主/终端透明度、字号、行高）都在拖动过程中逐帧
     落盘。
   - 落地 file:line（已按此落地）：`internal/settings/store.go` 新增 `Preview`/`Commit`（内存先动、
     写盘一次、失败回滚 `confirmed`）；`internal/settingsui/debounce.go`（新）的 `settingsDebounce`
     = 180ms、`previewSetting`/`debounceAlias`/`expireSettingsWrites`/`flushSettingsWrites`/`Flush`；
     `settingsui.go` 四处滑块改走 `previewSetting`，命令别名输入框改 `.Changed()` + `debounceAlias`，
     `View` 每帧过期、`set`/`selectPage` 先 flush；`internal/shell/shell.go` 的 `Open`/`Hide`/`quit`
     三条离开路径调 `Settings.Flush()`。
   - 验收（已过）：`internal/settings/store_test.go` 三条（三次 preview 写 0 次、commit 写 1 次；
     失败回滚 + 监听器先新后旧；preview + 立即 Update 合成一次写）；
     `internal/settingsui/debounce_test.go` 四条（窗口内文件不存在、过窗写最后一次、空转不再写；
     三次别名编辑合并一次动作且切页 flush；真敲键盘一个字符不写、过窗才写；`set` 先 flush 预览）。
     变异核查：`previewSetting` 换回 `a.set` → 第一条红；去掉 `selectPage` 的 flush → 别名条红。

> **清单状态（P7-fn 重写）**：P7-fn 收尾后第 2 条（滑块/别名的 180ms 去抖）由 P7-fo 落地，
> **2 条全部结清、清单归零**。下一轮回到「对照原版排查偏离点」，重新写实清单。依赖检查：
> `go.mod` 已是 **v0.4.8** = GitHub releases latest（release notes 的 `ui/editor` 用点已由 P7-ev
> 落地），无版本可升；只对照 `main`（`backup/main-pre-mygo` = v0.3.14 原版树），未翻历史提交。

> **清单状态（P7-fl 重写）**：P7-fk 收尾后「具体待办（P7-fj 重写）」3 条全部结清、清单归零。本轮换
> 入口：逐字段对照**终端表面浮栏与输入区**（原版 `main:src/App.tsx:3740-3805` 的
> `.terminal-panel__actions` 与 `.terminal-text-input`，本构建对应
> `internal/terminalui/terminalui.go` 的 `titleBar`/`View`），把清单重新写实为下面 **3 条带 file:line
> 的具体待办**：第 1 条（浮栏两个控件的词与绑定）与第 2 条（终端视图的无障碍名）P7-fl 落地，
> 第 3 条（浮栏身份行的退出码）P7-fm 落地——**3 条全部结清、清单归零**。依赖检查：
> `go list -m -versions github.com/egoist/mygo` 与 GitHub releases latest 都是 **v0.4.8**（与 `go.mod`
> 一致），无版本可升、无新 API 用点。只对照 `main`（`backup/main-pre-mygo` = v0.3.14 原版树），
> 未翻历史提交。

### 具体待办（P7-fl 重写；第 1·2 条 P7-fl、第 3 条 P7-fm 均已落地；清单归零）

审计入口：原版 `main:src/App.tsx` 的终端浮栏（`:3713-3796` 的 `.terminal-bar` 与
`.terminal-panel__actions`）与输入区（`:3803-3805` 的 `.terminal-text-input`），本构建对应
`internal/terminalui/terminalui.go` 的 `titleBar`（`:1042`）、`View`（`:707`）、`barHints`
（`:846`）与 `barButton`。前几轮已落地：外观抽屉（P7-ab）、粘贴安全（P7-an）、空态两行（P7-cm）。

1. **（本轮落地，P7-fl）浮栏两个控件的 tooltip 没有原版的 `{shortcut}` 插值，popout 的词也不对**。
   - 原版对照：`main:src/App.tsx:3771` 的 popout `title={t("terminal.openInTerminalHint", { shortcut:
     formatShortcut(shortcuts.open_external_terminal) })}`、`:3781` 的关闭键
     `title={t("terminal.newCommandHint", { shortcut: formatShortcut(shortcuts.new_command) })}`；
     aria-label 分别是 `terminal.openInTerminal` = “Continue in System Terminal” /
     `terminal.newCommand` = “New command”（`i18n.ts:146-149,1054-1057`）。
   - 本构建（改前）：`internal/i18n/copy.go:942,948` 的 `NewCommandHint` = “Back to the search field”、
     `OpenExternalHint` = “Opens the system's own terminal window”，`OpenExternal` = “Open in terminal”；
     `internal/terminalui/terminalui.go:1078,1081` 直接传这两个常量，绑定从未被读；`zh` 的
     `NewCommand` 还是“新命令”而非原版的“新建命令”。
   - 落地 file:line（已按此落地）：`internal/i18n/i18n.go:1108` 一带把 `NewCommandHint`/
     `OpenExternalHint` 改成 `func(shortcut string) string`，`copy.go` 的 en/zh 逐字取模板，
     `OpenExternal` 改词、`NewCommand` 的 zh 修正；`internal/terminalui/terminalui.go` 新增
     `barHints(copy, settings.Settings)`（`settings.Shortcut` → `shortcuts.Display`，即原版的
     `formatShortcut`），`titleBar` 改用它的两个返回值。
   - 验收（已过）：`internal/i18n/i18n_test.go` 的 `TestTerminalBarWordsMatchTheOldDictionary`
     （en/zh 十句逐字）；`internal/terminalui/view_test.go` 的
     `TestTerminalBarHintsFollowTheUserBinding`（改绑定 → tooltip 跟着变）与
     `TestTerminalBarButtonsCarryTheirHints`（期望值从绑定算出）。变异核查：把 `barHints` 的
     open-external 绑定换成常量 `"Ctrl+N"` → 立刻红（`want it to name "Ctrl+Alt+J"`）。

2. **（本轮落地，P7-fl）终端视图没有 `terminal.input` 的无障碍名**。
   - 原版对照：`main:src/App.tsx:3805` 的终端文本输入带 `aria-label={t("terminal.input")}` =
     “Terminal input”/“终端输入”（`i18n.ts:150,1058`）。
   - 本构建（改前）：`internal/terminalui/terminalui.go:776` 的
     `terminal.View(c, session.Term).Fill().Padding(pad).Bind(&a.Focus)` 没有 `Label`，
     `internal/i18n/i18n.go` 的 `Terminal` 结构也没有 `Input` 字段——聚焦到一个无名区域。
   - 落地 file:line（已按此落地）：`internal/i18n` 新增 `Terminal.Input`（en/zh）；
     `internal/terminalui/terminalui.go:776` 链上 `.Label(copy.Input)`。
   - 验收（已过）：`internal/terminalui/view_test.go` 的 `TestTerminalInputCarriesItsAccessibleName`
     （终端视图的标签找得到）。变异核查：摘掉 `.Label(copy.Input)` → 立刻红。

3. **（本轮落地，P7-fm）浮栏的身份行在会话结束后不报退出码**。
   - 原版对照：`main:src/App.tsx:3713-3722` 的 `identityTitle` —— 会话 ended 时是
     `` `${title} · ${t("terminal.headerExited", { code: exitCode ?? 0 })}` ``，这个字符串既是
     `.terminal-bar__identity` 的 `title=` 也是它读出来的身份文本；词条 `terminal.headerExited` =
     “Exited · code {code}”/“已退出 · 退出码 {code}”（`i18n.ts:154,1062`）。它与 resident note 的
     `terminal.processExited`（“Process exited · code {code}”）是**两条词**（`App.tsx:3891-3893`）。
   - 本构建（改前/现状）：`internal/terminalui/terminalui.go:1059-1066` 的 `titleBar` 身份只画
     `a.sessionTitle()`（空则 `copy.Title`），置灰点（`statusDot`）区分死活，但它**不报退出码**；
     `i18n.Terminal` 只有 `ProcessExited`（`i18n.go:1119`），退出码只出现在 `residentNote`。
   - 落地 file:line（已按此落地）：`internal/i18n` 的 `Terminal` 新增
     `HeaderExited func(code int) string`（en/zh 逐字取 `i18n.ts:170,1077`）；
     `internal/terminalui/terminalui.go` 的 `titleBar` 身份文本改成——`Session.Resident` 时
     `title + " \u00b7 " + copy.HeaderExited(session.ExitCode)`（标题为空时回落 `copy.Title`）——并
     把同一句放上 `.Label(...)`/`.Tooltip(...)`（原版的 `title=`）。
   - 验收（已过）：`internal/terminalui/view_test.go` 的 `TestTerminalBarIdentityNamesTheExitCode`——
     用 `withSession` 造一个会话、`a.ResidentAt(0, 3)` 后帧里出现 “Exited · code 3”（标题非空时是
     “zsh · Exited · code 3”），且 `residentNote` 的 “Process exited · code 3” 仍在（两条词都在，
     不互相顶掉）；`internal/i18n/i18n_test.go` 的 `TestTerminalBarWordsMatchTheOldDictionary`
     钉住 en/zh 两条词逐字。变异核查：把 `HeaderExited` 换成 `copy.ProcessExited` → 立刻红。

> **历史清单（保留作「哪些已经做过」的记录）**：

> **清单状态（P7-fj 重写）**：P7-fi 收尾后「具体待办（P7-fh 重写）」归零。本轮换入口：逐文件对照原版的
> **自定义快捷键的落盘/读盘路径**（`main:src/custom-shortcuts.ts` + `src-tauri/src/commands/config.rs`
> 的 `normalize_custom_shortcuts`/`normalize_shortcut`/`set_custom_shortcuts`），把清单重新写实为下面
> **3 条带 file:line 的具体待办**：第 1 条 P7-fj 落地，第 2·3 条（`conflict` 拒收与两个词）P7-fk 落地，
> **3 条全部结清、清单归零**。依赖检查：
> `go list -m -versions github.com/egoist/mygo` 最新仍是 **v0.4.8**（与 `go.mod` 一致），无版本可升、
> 无新 API 用点；GitHub releases API 限流。只对照 `main`（`backup/main-pre-mygo` = v0.3.14 原版树），
> 未翻历史提交。

### 具体待办（P7-fj 重写；第 1 条 P7-fj、第 2·3 条 P7-fk 均已落地；清单归零）

审计入口：原版 `main:src/custom-shortcuts.ts`（前端）与 `main:src-tauri/src/commands/config.rs` 的
`normalize_custom_shortcuts`（`:1385-1404`）、`normalize_shortcut`（`:1408-1434`）、
`set_custom_shortcuts`（`:1666-1718`）；本构建对应 `internal/settings/customshortcuts.go` +
`internal/shell/custom.go` + `internal/settingsui/custom.go`。前几轮已落地：应用快捷键地图的
`select_result` 家族头（P7-cw）、`Display`/`Badge` 拼写（P7-cx）、录制冲突拒收（P7-cy）。**下面
三条是本轮逐函数比对后新查出的真实缺口**。

1. **（本轮落地，P7-fj）`NormalizeCustomShortcuts` 收键时不校验形状、也不收拼写**。
   - 原版对照：`main:src-tauri/src/commands/config.rs:1385-1404` 的 `normalize_custom_shortcuts`
     把每个 `key` 过一遍 `normalize_shortcut`（`:1408-1434`：任一段 `trim` 后为空 → `None`、
     修饰键重复 → `None`、两个键 → `None`、无键 → `None`），再用 `.filter(|key| key.contains('+'))`
     丢掉裸键；命中的键以规范拼写（`modifiers.join("+")`）落盘。
   - 本构建（改前）：`internal/settings/customshortcuts.go:87-107` 的 `NormalizeCustomShortcuts`
     只 `strings.TrimSpace(key)` + `shortcuts.Duplicate` 去重——`"F5"`（裸键）、`"Ctrl++A"`、
     `"Ctrl+Ctrl+A"`、`"Ctrl+A+B"`、`"+"` 全部原样落盘；`internal/shortcuts/shortcuts.go:97-128`
     的 `Normalize` 对空段是 `continue`、对重复修饰键静默吞并，所以连那条宽松路径也修不出原版判据。
   - 落地 file:line（已按此落地）：`internal/shortcuts/shortcuts.go` 新增 `NormalizeBinding`
     （`:144`，与 `Normalize` 共用修饰键/键名表与 `join`，但空段/重复修饰键/两键/无修饰键一律拒绝）；
     `internal/settings/customshortcuts.go` 的 `NormalizeCustomShortcuts`（`:87`）改走它，
     `CustomShortcutsOf`（`:46`）与 `SetCustomShortcuts`（`:65`）读写两处都受用。
   - 验收（已过）：`internal/shortcuts/shortcuts_test.go` 的 `TestNormalizeBinding`（六例收、十一例
     拒，并钉住宽松 `Normalize("Ctrl++A") == "Ctrl+A"` 而严格路径拒绝）；
     `internal/settings/customshortcuts_test.go` 的
     `TestCustomShortcutsDropShapesTheRecorderCouldNotMake`（八条入、两条留、键与动作都规范/裁剪）与
     `TestCustomShortcutsOfDropsAHandEditedBadKey`（手改文件里的裸键读盘即丢）。变异核查：把
     `NormalizeCustomShortcuts` 的 `NormalizeBinding` 换回 `strings.TrimSpace` → 三条全红。

2. **（本轮落地，P7-fk）自定义快捷键与应用自身按键撞车时，原版判 `conflict` 并拒收**。
   - 原版对照：`main:src-tauri/src/commands/config.rs:1666-1718` 的 `set_custom_shortcuts`——
     `reserved` 取 `resolved_shortcuts(&settings)` 的全部值，逐条与 `shortcut_binding_eq` 比对，
     撞上内置组合即 `CustomShortcutRejection { reason: "conflict" }` 并跳过（`:1682-1695`）。
   - 本构建（改前）：`internal/shell/custom.go:24-40` 的 `setCustomShortcuts` 无条件
     `s.SetCustomShortcuts(entries)` 落盘；`ApplyCustomShortcuts`（`:54-96`）只挡
     `accelerator == a.summonKey`（`:71`），**不比对** `settings.ShortcutsOf` 的其它动作值——把自定义
     键绑到 `Cmd+W`（`new_command` 的默认）会照常落盘并注册，而应用快捷键那条与此同时仍生效。
   - 落地 file:line（已按此落地）：`internal/settings/shortcutmap.go` 新增
     `CustomShortcutConflict`（`:118`，逐 action 过 `ShortcutsOf`，`shortcuts.Equal` 等值比对 +
     `select_result` 家族按修饰键判撞；`toggle_window` 交给 shell 的 summon 检查）；
     `internal/shortcuts/shortcuts.go` 新增 `SameResultFamily`（`:281`）。
     `internal/shell/custom.go` 的注册收敛成唯一一条 `applyCustomShortcuts`（`:80`）——
     `ApplyCustomShortcuts`（`:67`）、`setCustomShortcuts`（`:28`）与启动/文件变更三条路都走它，
     冲突/占用不可再各说各话；`setCustomShortcuts` 只把注册成功的那几条写回磁盘（原版
     `:1716-1717` 的 `updated.custom_shortcuts = registered`），并在写盘前记下
     `customSignature`，以免 Store 自己的监听把同一批键再 claim 一次。
   - 验收（已过）：`internal/settings/shortcutmap_test.go` 的 `TestCustomShortcutConflict`
     （等值/异拼写/家族任一数字 = 冲突，别的修饰键/空闲键/toggle 键 = 不冲突）；
     `internal/shell/shell_test.go` 的 `TestCustomShortcutsRefuseConflictsAndKeepOnlyRegistered`
     （`Cmd+W` 报 `conflict` 且既不注册也不落盘、`Cmd+Shift+T` 报 `occupied` 且不落盘、
     空闲键注册并落盘）。

3. **（本轮落地，P7-fk）拒收原因用自由句而不是原版的两个词，设置页文案对不上原版**。
   - 原版对照：`reason` 只有 `"conflict"`/`"occupied"` 两个 token（`config.rs:1691,1707`），
     前端 `main:src/settings/ShortcutsPage.tsx:293-296` 取 `settings.customShortcutsOccupied`
     /`settings.customShortcutsConflict` 两个词（`i18n.ts:412-413`）。
   - 本构建（改前）：`internal/shell/custom.go:68,72,77` 塞的是英文自由句（"the accelerator is not one the
     system accepts" / "the key already summons floter" / `err.Error()`），`internal/i18n` 的
     `ShortcutsRejected`（`copy.go:6,28`；`internal/settingsui/custom.go:212` 调用）是把 key +
     自由句拼起来，没有那两个词。
   - 落地 file:line（已按此落地）：`internal/settingsui/settingsui.go` 新增
     `CustomShortcutReasonConflict`/`CustomShortcutReasonOccupied` 两个 token 常量（`:688`）；
     `internal/i18n/copy.go` 的 `shortcutsRejectedEN`/`shortcutsRejectedZH` 按 token 选词
     （逐字取 `i18n.ts:412-413,1299-1300`）；`internal/shell/custom.go` 的 `applyCustomShortcuts`
     只发这两个 token。非法形状在 `NormalizeCustomShortcuts` 已丢，不再需要第三个原因。
   - 验收（已过）：`internal/settingsui/view_test.go` 的 `TestCustomShortcutRejectionWording`
     （en 两词）与 `TestCustomShortcutsPage`（`occupied` 那句）；`internal/i18n/i18n_test.go` 的
     `TestCustomShortcutRejectionWordsMatchTheOldDictionary`（en/zh 四句 + 未知 token 原样透传）。
     变异核查：把 zh 映射退回直通 → 该测试立刻红（`无法绑定 Cmd+J：conflict`）。

> **历史清单（保留作「哪些已经做过」的记录）**：

> **清单状态（P7-fe 结清）**：P7-ew 一路到 P7-fa 的 5 条全部落地、上一条清单归零后，P7-fb 按
> 审计入口 `main:src/extensions/ComponentizedUninstallDialog.tsx`（本构建对应
> `internal/settingsui/uninstall.go`）逐字段比对，把清单重新写实为下面 **4 条带 file:line 的具体
> 待办**（第 1–3 条 P7-fb 落地）；本轮 P7-fe 取剩下的第 4 条（卸载路由）落地，**4 条至此全部
> 结清，本清单归零**。下一轮回到「对照原版排查偏离点」，重新写实清单。
> 依赖检查：`go list -m -versions github.com/egoist/mygo` 最新仍是 **v0.4.8**（与 `go.mod` 一致），
> 无版本可升；GitHub releases API 限流。只对照 `main`（`backup/main-pre-mygo` = v0.3.14 原版树），
> 未翻历史提交。

### 具体待办（P7-fb 重写；第 1·2·3 条 P7-fb、第 4 条 P7-fe——4 条全部落地）

审计入口：原版 `main:src/extensions/ComponentizedUninstallDialog.tsx`（组件化卸载对话框，本构建对应
`internal/settingsui/uninstall.go`）与它的调用点 `main:src/ExtensionsPanel.tsx:1286-1323,2150-2165`，
逐字段对照。P3-p 首次落地时把对话框压成「标题 + 说明 + 三个勾选」（见 plan `P3-p`），下面四条是本
轮逐字段比对后新查出的真实缺口。

1. **（本轮落地 P7-fb）对话框没有原版的第一行「程序文件」（选中且禁用）**。
   - 原版对照：`main:src/extensions/ComponentizedUninstallDialog.tsx:84-98` 的第一个
     `.extensions-uninstall-component`——`uninstallProgram`（“Program files”）+ `uninstallProgramHint`
     （“Integration package and runtime (always removed)”，`i18n.ts:532-533`），checkbox `checked` 且
     `disabled`（`:88-90`）；它正是对话框「程序文件始终会被移除」这句披露的落点。
   - 本构建（改前）：`internal/settingsui/uninstall.go:44-51` 的 `uninstallDialog` 只画 HostConfig/
     ToolData/Artifacts 三行，`grep -rn 'UninstallProgram' internal` 为空——用户看不到程序文件会被移除。
   - 落地 file:line：`internal/i18n` 新增 `UninstallProgram`/`UninstallProgramHint`（en/zh 逐字取
     `i18n.ts:532-533`）；`internal/settingsui/uninstall.go` 的 `uninstallParts`（四行的值函数，
     program 行 `Fixed`、无 `Value`）+ `uninstallPartRow`（fixed 行 `ui.Checkbox(...).Label(label).Disabled(true)`）。
   - 验收：`internal/settingsui/uninstall_test.go` 的 `TestComponentizedUninstallListsAllFourComponents`
     （`uninstallParts` 四行、program 固定、其余三行指 draft 字段、两语言不同）与
     `TestComponentizedUninstallDrawsTheReferenceRows`（屏幕上四行按 program/host/tool/artifacts 顺序、
     四条 hint 逐字）。变异核查：抽掉 `uninstallParts` 的 program 行 → 两条都红。

2. **（本轮落地 P7-fb）对话框标题不是原版的 `uninstallComponentsTitle`**。
   - 原版对照：`main:src/extensions/ComponentizedUninstallDialog.tsx:70-72` 的 `<h3>` 读
     `settings.extensions.uninstallComponentsTitle` = “Uninstall {name}”（`i18n.ts:530`）。
   - 本构建（改前）：`internal/settingsui/uninstall.go:46` 用自造句 `IntegrationsUninstallTitle`
     （“Uninstall this integration”）+ `"  ·  "` + 名字（`internal/i18n/copy.go`），既不是原版词条、
     也没有 `{name}` 插值。
   - 落地 file:line：`internal/i18n/i18n.go:525` 的 `IntegrationsUninstallTitle` 改成
     `func(name string) string`，`copy.go` en/zh 逐字 “Uninstall {name}”/“卸载 {name}”；`uninstall.go` 改调用。
   - 验收：`TestComponentizedUninstallDrawsTheReferenceRows` 断言标题逐字 “Uninstall NPM Tool”，且
     自造句 “Uninstall this integration” 不出现。

3. **（本轮落地 P7-fb）对话框两个动作的顺序与原版相反**。
   - 原版对照：`main:src/extensions/ComponentizedUninstallDialog.tsx:119-141` 的
     `.extensions-dialog-actions`——Cancel 在前、danger confirm（`data-destructive-confirm`）在后。
   - 本构建（改前）：`internal/settingsui/uninstall.go:52-61` 先画 confirm（`removalLabel`）再画 Cancel。
   - 落地 file:line：`uninstall.go` 的 `ui.Row` 里 Cancel 先、confirm 后（同原版，也与本仓库
     `removeConfirmation`（`settingsui.go:4025-4035`）的 cancel-first 一致）。
   - 验收：`TestComponentizedUninstallKeepsCancelFirst`——Cancel 的 x < confirm 的 x。

4. **（已落地 P7-fe）组件化对话框对任何 kind 都展开，原版只给 npm/custom**。
   - 原版对照：`main:src/ExtensionsPanel.tsx:1286-1323` 的 `confirmRemoval`——**所有** kind 先走
     `RemovalConfirmation` 条（`removalTarget`）；确认后 npm/custom 才 `setUninstallDialogTarget`
     （组件化对话框），system/package 直接 `extensions_uninstall {removeData:false}`
     （通知文案 `packageRemovedNotice`/`disconnectedNotice`）。
   - 本构建（改前）：`internal/settingsui/settingsui.go:1826-1832`——只要 `Actions.UninstallComponents
     != nil`，任何 kind 的卸载按钮都直接 `<verb>…` 展开组件化对话框（`uninstall.go:25`）；shell
     （`shell.go:735,761`）两个入口都接线后，`removeConfirmation`/`requestRemove` 在集成行成了死代码，
     system/package 被提供了原版不会给的数据删除勾选。
   - 落地 file:line（已落地）：`internal/settingsui/settingsui.go` 新增
     `Integration.componentizedRemoval()`（`removalKind()` ∈ {npm, custom}）；集成行（`:1819` 一带）的
     动作改成三分支——npm/custom 且接了组件化动作走 `uninstallComponentsRow`；否则若接了普通卸载动作
     走 `requestRemove` → `removeConfirmation` → `UninstallIntegration`；仅接组件化动作（测试夹具）
     回退到组件化门。
   - 验收（已满足）：`internal/settingsui/integrationremoval_test.go` 的
     `TestOnlyOwnedKindsTakeTheComponentizedRemoval`（四 kind 纯映射）与
     `TestOnlyOwnedKindsOpenTheComponentizedDialog`（system 行点 Disconnect 先画
     `IntegrationsDisconnectTitle` 条、四行组件化文案不出现、确认前不调动作；位置点 Cancel 右侧的
     danger 后只调 `UninstallIntegration`）。变异核查：`componentizedRemoval()` 改成 `return true` → 两条红。
     残留（有意偏离）：system/package 成功后的通知仍走 shell 的 `IntegrationRemoved`
     （`UninstallIntegration(id, name)` 签名不带 kind），未分 `disconnectedNotice`/`packageRemovedNotice`。

### 具体待办（P7-ew 重写；第 1 条 P7-ew、第 2 条 P7-ex、第 3 条 P7-ey、第 4 条 P7-ez、第 5 条 P7-fa——5 条全部落地）

审计入口：原版 `main:src/extensions/CustomIntegrationDrawer.tsx`（自定义工具表单，本构建对应
`internal/settingsui/settingsui.go` 的 `localToolSection`/`scriptRows`/`toolPermissionFields`/
`toolParamsEditor`）与 `main:src/ExtensionsPanel.tsx` 的调用点，逐字段对照。前几轮已落地：权限披露
（R161，P7-bp）、版本兜底 hint（R153，P7-bq）、选项换行码（R153，P7-bq）、脚本编辑器（F-new，
P7-ev）、执行引擎/参数校验（P7-bh）等。**下面五条是本轮逐字段比对后新查出的真实缺口**。

1. **（本轮落地，P7-ew）自定义工具表单没有原版的内容动作行（复制脚本 / 复制命令 / 复制执行计划）**。
   - 原版对照：`main:src/extensions/CustomIntegrationDrawer.tsx:145-146` 的
     `.extension-custom-form__actions` 行——script 模式一个「Copy script」（`onCopy(scriptContent,
     customScriptCopied)`），executable 模式「Copy command」（`onCopy(executablePath,
     customCommandCopied)`）+「Copy execution plan」（`onCopyPlan`）；文案 `i18n.ts:648-655`，
     行为在 `src/ExtensionsPanel.tsx:1934-1964`（`copyCustomContent` 单飞行 + 1.6s 确认，
     `copyExecutionPlan` 把 `{program, args, mode:"pty"}` 美化 JSON）。
   - 本构建（改前）：`internal/settingsui/settingsui.go:3009-3054` 的表单只有字段行、脚本编辑器、
     Output 行，**没有任何复制按钮**；`grep -rn 'LocalToolCopyScript\|copyScript\|copyPlan' internal/settingsui`
     为空，i18n 也没有对应词条，用户只能手动选中/复制。
   - 落地 file:line（已落地）：i18n 六个键 `LocalToolCopyScript/ScriptCopied/CopyCommand/CommandCopied/
     CopyPlan/PlanCopied`（`internal/i18n/i18n.go:840-850`、`copy.go` en/zh）；`internal/settingsui/rows.go`
     的 `actionRow`（无标签的卡内行，通风其余行的 hairline 节奏）；`internal/settingsui/settingsui.go`
     的 `toolContentActions`/`contentAction`/`contentCopied`/`executionPlanJSON`/`splitPlanArgs` 与
     `copiedContent/copiedContentUntil` 跨帧确认状态（`:3054` 接线）；复用已有的 `Actions.CopyText`。
   - 验收：`internal/settingsui/toolcontent_test.go` 三条——`TestLocalToolFormCopiesItsContent`
     （program 模式两个按钮、点击复制出命令词与 `{program,args,mode}` JSON、逐字确认 1.6s 只点亮
     被按的那个）、`TestLocalToolFormCopiesTheScript`（script 模式只有 Copy script、复制出模板、程序
     按钮不画）、`TestExecutionPlanJSON`（空白参数读作 `[]` 而非 `null`）。既有 `TestLocalToolForm`
     因表单高一行为 1700 渲染高度（注释说明原因）。变异核查见「最近几轮」P7-ew。

2. **（已落地，P7-ex）可执行表单的「可执行文件」字段缺少原版的下方说明与占位符**。
   - 原版对照：`main:src/extensions/CustomIntegrationDrawer.tsx:145` 的 `customExecutable` label 下
     一条 `customExecutablePlaceholder`（“Type a path or search installed tools”，`i18n.ts:687`）与一条
     `changeExecutableHint`（“Search the tools installed on this device… re-runs verification when you
     save”，`:665`）。
   - 本构建（改前）：`internal/settingsui/settingsui.go:3009-3014` 的 Program 行只有 `Placeholder(
     "/usr/bin/rg")`，`grep -rn 'ChangeExecutable\|LocalToolProgramHint' internal` 为空——没有原版的
     那句「改可执行文件会在保存时重跑验证」的提示，占位符也是自造路径。
   - 落地 file:line（已落地）：`internal/i18n` 新增 `LocalToolProgramPlaceholder` + `LocalToolProgramHint`
     （en/zh 逐字取 `i18n.ts:665,687`）；`settingsui.go:3009` 的 Program 行改用这两个键（`row` 的
     sublabel 位改成 hint，`textInput` 的 `Placeholder` 改成原版句）。
   - 验收（已满足）：`internal/settingsui/toolcontent_test.go` 的
     `TestLocalToolProgramRowCarriesItsWord`——Program 行下印出 hint、占位符不出现在 `Texts()`
     （编辑器渲染的占位符不注册文本，同 `view_test.go:2178` 的注释）、script 模式不画；既有
     `TestLocalToolForm` 因表单又高一行为 1800 渲染高度。变异核查：把 hint 参数改回 `""` → 红。
     详见「最近几轮」P7-ex。

3. **（已落地，P7-ey）「集成 ID」在编辑态是只读说明行，本构建根本没画**。
   - 原版对照：`main:src/extensions/CustomIntegrationDrawer.tsx:144` 的 `editingId &&` 分支——编辑时
     一条 `customId`（“Integration ID”，`i18n.ts:683`）后跟不可编辑的 `<code>{id}</code>`；创建时整行
     隐藏（还没有 id）。原因写在原版注释里（R9-3：id 由后端铸造且此后不可变，前端不展示可推导的值）。
   - 本构建（改前）：`internal/settingsui/settingsui.go` 的 `localToolSection` 在 `a.toolEditing != ""`
     时**只改按钮文案**（`LocalToolSave`），没有任何一行点名正在改的是哪个 id；
     `grep -rn 'CustomId\|LocalToolId\|Integration ID' internal/settingsui` 为空。
   - 落地 file:line（已落地）：`internal/i18n` 新增 `LocalToolId`（en/zh，原版 `customId` 原文，
     `i18n.go:806-810`、`copy.go:708,1529`）；`settingsui.go:2987-2992` 的 `localToolSection` 在卡片
     顶部（Name 行之前）`a.toolEditing != ""` 时走 `a.keyValue(c, copy.LocalToolId, a.toolEditing)`——
     值在行尾且 `Selectable()`，没有可编辑控件（同 About 页只读值的读法）；创建态不画。
   - 验收（已满足）：`internal/settingsui/toolcontent_test.go` 的
     `TestLocalToolFormNamesTheIntegrationBeingEdited`——en/zh 标签逐字断言、列表态不画、点「Edit…」
     进编辑态后 `HasText(copy.LocalToolId)` 且 `a.toolEditing == "local.abc"`、`endEdit()` 后消失。
     删掉该行 → 红。详见「最近几轮」P7-ey。

4. **（已落地，P7-ez）参数编辑器的「必需」开关与行头控件顺序与原版不一致**。
   - 原版对照：`main:src/extensions/CustomIntegrationDrawer.tsx:83-104` 的 `ScriptParamEditor` 每个参数
     一行：label/id/type/placeholder/default/flag 字段 + `required` 开关 + 删除按钮；`boolean` 种类**不画
     placeholder 字段**（`param.kind !== "boolean" && …`），`select` 种类在行末展开 `options` 的 textarea。
   - 本构建（改前）：`internal/settingsui/settingsui.go` 的 `toolParamsEditor` 把每行压成
     「一行控件」（`row` 的 control 里放 id/kind/flag 三个控件，options/required 放 `below`），控件顺序
     与可见平面与原版不同；**且 `placeholder`/`default` 两个字段在 `toolParam`/`editorParams`/`beginEdit`
     里根本不存在**，编辑一个带这两值的集成再保存会把它们静默丢掉。
   - 落地 file:line（已落地）：`internal/settingsui/settingsui.go` 的 `toolParamsEditor`——行头只放原版的
     controls 组（`Required` 开关 + `Remove` 按钮），字段按 label/id/kind (+placeholder for
     non-boolean)/default/flag 逐行铺开，select 的 Choices 行仍在 Flag 之后，行末是这一行自己的报错；
     `toolParam`/`editorParams`/`beginEdit` 读写 `Placeholder`/`Default`；`internal/i18n` 新增
     `ParamsPlaceholder`/`ParamsDefault`（en/zh 逐字取 `i18n.ts:723-724`）。不改 `extensions.ParamRowIssues`
     的规则（它们已与后端一致，P7-w）。
   - 验收（已满足）：`internal/settingsui/params_test.go` 的
     `TestConnectFormKeepsTheReferenceParamFields`——非 boolean 参数画 placeholder 字段、boolean 不画；
     字段顺序按原版（label/id/kind/placeholder/default/flag）；`declaredParams`/`beginEdit` 带出两个字段。
     变异核查：去掉非 boolean 的 placeholder 行 → 红；把 Default 行提到 Kind 之前 → 红。既有
     `TestParamRowIssues`/`TestConnectFormDeclaresInputs` 保持绿。详见「最近几轮」P7-ez。

5. **（已落地，P7-fa）脚本模式的「导出脚本」动作（写文件）缺失**。
   - 原版对照：`main:src/extensions/CustomIntegrationDrawer.tsx:145` 的「Export script」按钮 →
     `src/ExtensionsPanel.tsx:1951-1964` 的 `exportCustomScript`，调后端
     `extensions_custom_export_script`（`src-tauri/src/commands/extensions.rs:1516-1560`）让用户选路径并写出
     脚本（成功 toast `customScriptExported`，`i18n.ts:651`；文件名 `{id}-script.{extension}`，id 取
     `editingCustomId ?? customIntegration.command`）。
   - 本构建（改前）：`grep -rn 'ExportScript\|custom_export_script\|ScriptExported' internal` 为空——
     第 1 条本轮只落了「复制」，写文件这一支完全没有（原版是复制+导出两个按钮）。
   - 落地 file:line（已落地）：`internal/settingsui` 新增 `Actions.ExportScript(name, content)`
     （`settingsui.go:170-177`）；`toolContentActions`（`:3258`）收 `language`，script 模式在 Copy script
     之后走 `exportScriptAction`（无 exporter 或脚本为空不画）与 `scriptExportName`
     （复刻 `{id}-script.{extension}`）；`internal/shell/sync.go:167-200` 的 `exportScript` 复用
     `saveFile` 对话框 + `os.WriteFile` 写盘、成功发 `script-export` toast、关闭对话框静默、失败发
     `LocalToolExportFailed`；`saveFile` 的 filter 改为跟随文件名后缀；i18n 新增
     `LocalToolExportScript`/`LocalToolScriptExported`/`LocalToolExportFailed`。
   - 验收（已满足）：`internal/settingsui/toolcontent_test.go` 的
     `TestLocalToolFormExportsTheScript`（program 模式不画、script 模式画、点击后收到
     `notes-tool-script.js` 与表单脚本、不占剪贴板确认窗）、`TestScriptExportName`（四例）；
     `internal/shell/shell_test.go` 的 `TestTheScriptExportWritesTheFile`（对话框名字逐字、文件内容、
     成功 toast）、`TestTheScriptExportStaysQuietWhenItWritesNothing`（关闭/失败两支）。变异核查：
     拿掉导出按钮 → 红；去掉 `os.WriteFile` → 红。详见「最近几轮」P7-fa。

### 具体待办（P7-ee 重写；第 1·2·3 条全部落地：第 1 条 P7-ee、第 2·3 条 P7-ef）

1. **（已落地，P7-ee）剪贴板模式只持有 50 行，而且上限花在 chip 过滤之前**。
   - 原版对照：`main:src/hooks/useLauncherCatalog.ts:833-834` 把 `MAX_CLIPBOARD_MAX_ITEMS`
     （`main:src/clipboard-history.ts:440` = 500，正是历史自己的容量上限）当 **抓取深度** 传给
     `clipboardModeRows`/`clipboardSearchRows`；`main:src/plugins/clipboard/mode.ts:34` 的
     `CLIPBOARD_FETCH_LIMIT = MAX_RESULTS`（10）只是**插件输出视图**那个固定高度盒子的分组上限。
     过滤顺序也在同一处：`clipboardModeRows` 先 `filterClipboardEntries(needle)`、再
     `filter(clipboardEntryMatchesFilter)`（chip），最后才 `matched.slice(0, limit)`。
   - 本构建（改前）：`internal/launcher/search.go:351` 把 `maxClipboardResults = 50`（`:437`）
     当 `Store.Search` 的 limit——**上限在 chip 过滤之前花掉**，于是空查询最多 50 行，且
     `images`/`favorites` 这类 chip 在命中数超过 50 时会看不到本该存在的行（与 P7-ec 修掉的浏览器
     同一个毛病：上限切在 chip 之前）。这个 50 全仓没有出处，也没写进任何待办或文档。
   - **已落地（P7-ee）file:line 规格**：`internal/launcher/search.go` 新增导出
     `ClipboardFetchLimit = clipboard.MaxMaxItems`（500；注释逐字指向
     `useLauncherCatalog.ts:833-834` 与 `clipboard-history.ts:440`，并写明为何只取两个天花板里的
     这一个）；`clipboardItems` 改成 `Search(query, 0)`（只上 needle，不花上限）→ chip 过滤 →
     `len(out) >= ClipboardFetchLimit` 收手（对应原版的 `matched.slice(0, limit)`）；`pruneThumbs`
     的 keep 集合改为**这一帧真正建出的行**（对应原版 R97 传渲染自己的 id），不再是 needle 命中的全部。
   - 验收：`internal/launcher/clipboarddepth_test.go` 的
     `TestTheClipboardModeHoldsTheReferenceFetchDepth` 钉 `ClipboardFetchLimit == 500 ==
     clipboard.MaxMaxItems`、600 条历史（容量 500）下模式恰好持 500 行且最新一行在最前；
     `TestTheClipboardChipFilterSeesTheWholeHeldSet` 把唯一的（且最老的、被收藏豁免淘汰的）
     图片摆在 600 条文本之后，钉 `images` chip 仍能拿到它。变异核查：把常量换回 50 → 第一条红；
     把上限挪回 `Search` 里（上限先花）→ 第二条红（`images chip shows 0 rows, want 1`）。
     详见「最近几轮」P7-ee。

2. **（已落地，P7-ef）剪贴板模式没有「已关闭」这一档**。
   - 原版对照：`main:src/hooks/useLauncherCatalog.ts:814-816` 在插件关闭时发一条
     `clipboardStatusRow("clipboard-disabled", "clipboard.pageUnavailable")`
     （词条见 `main:src/i18n.ts:291` “Clipboard history is off”），即**插件关掉时模式自己说发生了
     什么**，而不是说「暂无复制记录」。
   - 本构建（改前）：`internal/shell/shell.go:792` 无条件建 store，插件开关只影响**监听**
     （`internal/shell/system.go:274` 的 `watchClipboard`），于是关掉插件后仍能进模式、仍列出历史，
     空态走 `internal/launcher/launcher.go:585-591` 的 `ClipboardEmpty`（“暂无复制记录”）——
     一句与事实相反的话。浏览器模式有对应档（`BrowserResults.Disabled` →
     `internal/launcher/launcher.go:593-598` 的 `BrowserDisabled`），剪贴板没有对应物。
   - **已落地（P7-ef）**：`internal/launcher/search.go` 新增导出 `ClipboardDisabled`（原版 id 逐字）
     与 `Item.status`；`clipboardItems` 先读 `settings.ClipboardOf(a.settings()).Enabled`，关闭时返回
     那一条 status 行（`Title` = 新 i18n 键 `ClipboardDisabled`，无 `Run`）；`Item.status` 经
     `windowRows` 设 `RowInfo.Status`、`launcher.row()` 按 `.launcher-status` 画像（无底板、lucide
     `info`、30u 带），`rowSelectable` 把 note 从箭头/数字族里排掉。
   - 验收：`internal/launcher/clipboardoff_test.go` 四条（见「最近几轮」P7-ef）；新键 en/zh 各一行，
     `TestNoDeadCopy` 与完整性测试守卫。

3. **（已落地，P7-ef）计算器模式没有模式层行数上限**。
   - 原版对照：`main:src/plugins/calculator/mode.ts:79-80` 的 `calculatorModeRows` 在 chip 过滤后
     `matched.slice(0, limit)`，内联启动器传 `MAX_CALCULATOR_MAX_ITEMS`
     （`main:src/calculator.ts:250` = 500）；`CALCULATOR_FETCH_LIMIT = MAX_RESULTS` 同样只是输出视图的。
   - 本构建（改前）：`internal/launcher/calculator.go:128-144` 遍历 `a.calculatorFound` 全量、不设上限；
     `internal/calculator` 的 `maxItems`（默认 100、上限 500）只约束**非收藏**项，而收藏永不淘汰，
     所以一份收藏极多的历史会比原版多出行。
   - **已落地（P7-ef）**：`internal/launcher/calculator.go` 新增导出
     `CalculatorFetchLimit = calculator.MaxMaxItems`（= 500）；`calculatorItems` 保持 chip 过滤在前，
     过滤后 `history >= CalculatorFetchLimit` 收手。验收：`internal/launcher/calculatorcap_test.go`
     两条（600 条下恰好 500 行、收藏 chip 仍能看到被上限跳过的唯一收藏）。

4. **（记录，非待办）原版的十行客户端翻页（`paginatePluginRows`）本构建不实现**。
   - 原版对照：`main:src/launcher/paginate-plugin-rows.ts` 把内联插件列表切成每页十行、滚到底加载下一页；
     它配合的是固定高度盒子里没有滚动条的列表。
   - 本构建：启动器列表是虚拟化滚动列表（`internal/launcher/launcher.go:871-881` 的滚动视口），
     所以持有 500 行即可滚到；再加一层分页会是「两个滚动机制打架」的半截行为。P7-ec 已按此决定落地；
     若将来要复刻翻页，须先拆掉列表自身的滚动，届时应整体替换而非叠加。

### 具体待办（P7-ec 重写；第 1·2·3 条已落地：第 1 条 P7-eb、第 2 条 P7-ec、第 3 条 P7-ed）

> 第 1 条（浏览器模式行内容）已由 P7-eb 落地；第 2 条（数量上限与抓取深度）P7-ec 落地；
> 第 3 条（plugincfg 值规范化）本轮 P7-ed 落地。本节至此全部结清。依赖检查同上，无版本可升。

1. **（已落地，P7-eb）浏览器结果行的字形与副标题偏离原版**。
   - 原版对照：`main:src/launcher/LauncherResults.tsx:290-294` 的 `BrowserRowIcon`——bookmark →
     书签、history → 时钟、live tab → `app-window`，只有来源未知才落到地球；
     `main:src/plugins/browser/mode.ts:195,208` 给每条搜索结果与每个标签页的 `subtitle` 都是**纯 URL**
     （`last_visit`/`date_added` 只被 `browser-page.ts:37` 解析、从不渲染）；
     `main:src/browser-page.ts:216` 的 `tabRowKey` 是 `tab:${browser_id}:${window_index}:${tab_index}`
     （注释：「Position, not URL: the same page can be open in two windows and the two rows must stay
     distinct」）。
   - 本构建（改前）：`internal/launcher/glyphs.go:76-80` 的 `rowGlyph` 只判 bookmark，其余一律地球；
     `internal/launcher/search.go:246-248` 给历史行拼访问时间、`:252-256` 给标签页拼 `· Open tab`，
     且标签页 id 是 `"tab:" + tab.BrowserID + ":" + tab.URL`。
   - **已落地（P7-eb）**：`rowGlyph` 按 `browser.KindBookmark` / `browser.KindHistory` 分派；
     两处 `Detail` 回到纯 URL；标签页 id 改成 `tab:<browserID>:<window>:<index>`；i18n 的
     `BrowserTab` 成为死键，从 `Copy` 与 en/zh 字典各删一行（`TestNoDeadCopy` 守卫）。
     测试：`internal/launcher/browserrows_test.go` 的 `TestBrowserRowsFollowTheReferenceRowContent`；
     改写 `view_test.go` 的 `TestBrowserModeOpensAndCopies`。详见「最近几轮」P7-eb。

2. **（已落地，P7-ec）浏览器结果的数量上限与抓取深度不对齐原版**。
   - 原版对照：`main:src/plugins/browser/mode.ts:70` 的 `BROWSER_FETCH_LIMIT = 500` 是内联启动器
     **拉取并持有**的深度；`:84` 的 `BROWSER_GROUP_LIMIT = MAX_RESULTS`（`result-budget.ts:46` 的
     `MAX_RESULTS = 10`，「一个分组最多 10 行」）只是**插件输出视图**那个不能滚动的盒子自己的分组上限。
     内联模式把分组的 `cap` 提到抓取深度，合并组按 `cap` 截断（`:199` 的 `if (rows.length >= cap) break`）、
     标签页组按 `cap * 2` 的**总数**截断（`:218`），再用 `paginatePluginRows` 每次翻十行显示。
   - 本构建（改前）：`internal/shell/system.go:251` 的 `browserResultLimit = 50` 同时当
     `browser.Options.Limit`（`internal/shell/shell.go:1590`，搜索**合并后**的后置截断，**不是** SQL LIMIT）
     与标签页上限（`shell.go:1605-1606`）；`internal/launcher/search.go` 的 `browserItems` 不二次截断。
     于是空查询的浏览器列表最多 50 行（原版 500），而且截断在范围 chip **之前**：书签一多，
     `history` chip 就什么都看不到。
   - **决定（P7-ec）**：**不复刻「每分组 10 行 + 翻页」**——本构建的启动器列表虚拟化可滚动，原版的十行
     分页是它自己固定高度盒子的补丁，硬塞 `BROWSER_GROUP_LIMIT = 10` 只会藏住滚动本可到达的行；
     保留滚动，只把「模式持有多少行」的判据搬回模式层，与原版 `browserSearchRows` 同位。
   - **已落地（P7-ec）file:line 规格**：
     - 模式层上限（原版 `mode.ts:70` 的落点）：`internal/launcher/search.go` 新增导出
       `BrowserFetchLimit = 500`；`browserItems` 的合并组在 `len(out) >= BrowserFetchLimit` 收手
       （对应 `mode.ts:199-200`）、标签页组在 `len(out) >= 2*BrowserFetchLimit` 收手（对应
       `mode.ts:218`，标签页**不**与第一组合并，`mode.ts:90`）。
     - 后置截断移除：`internal/shell/system.go` 删 `browserResultLimit` 与 `browserOptions.limit`；
       `internal/shell/shell.go` 的 `searchBrowser` 不再给 `browser.Search` 传 `Limit`（0 = 不截断，
       后端自己的每 profile SQL 窗口 `internal/browser/browser.go:411` 的 4000 行仍是读入上限，比原版更深），
       也不在 shell 里截断标签页。
     验收：`internal/launcher/browsercap_test.go` 的 `TestTheBrowserModeHoldsTheReferenceFetchDepth`
     钉 `BrowserFetchLimit == 500`、空查询第一组 500、标签页组 500、整表 1000；
     `TestTheBrowserTabGroupIsNotMergedWithTheFirstGroup` 钉同一 URL 作为书签与标签页时两行都在、id 不同。
     行数上限的**反面**（不做原版的十行翻页）写入下面第 4 条。

4. **（记录，非待办）原版的十行客户端翻页（`paginatePluginRows`）本构建不实现**。
   - 原版对照：`main:src/launcher/paginate-plugin-rows.ts` 把内联插件列表切成每页十行、滚到底加载下一页；
     它配合的是固定高度盒子里没有滚动条的列表。
   - 本构建：启动器列表是虚拟化滚动列表（`internal/launcher/launcher.go:871-881` 的滚动视口），
     所以持有 500 行即可滚到；再加一层分页会是「两个滚动机制打架」的半截行为。P7-ec 已按此决定落地；
     若将来要复刻翻页，须先拆掉列表自身的滚动，届时应整体替换而非叠加。
3. **（已落地，P7-ed）`internal/plugincfg` 的值规范化与原版 `normalizeConfigValue` 有四处口径差**。

   > **已落地（P7-ed）file:line 规格**：决定**保留拒绝语义**（形状不符 / select·radio 未知值仍拒绝，
   > 不改 `TestApplyRejectsAValueOutsideTheOptions`），只补齐其余三处：
   > `internal/plugincfg/schema.go` 的 `coerce` 里 text 加 `strings.TrimSpace`（空→`nil`）、
   > slider/number 走新的 `clampStep`（按 `min` 锚定 step 网格 + clamp + 六位舍入，NaN/Inf → `min`）、
   > checkboxes 去重并按 schema 选项顺序输出；加载路径（typed settings）经
   > `TestTheShippedValuesAlreadyFitTheSchema` 钉住形状已合规。测试表见下方原验收。详见「最近几轮」P7-ed。
   - 原版对照：`main:src/plugins/config-schema.ts:448-500` 的 `normalizeConfigValue` 是**全函数**：
     toggle → `raw === true`；text → `trim` 后空为 `null`；slider/number → 非有限回退 `field.min` 并
     `clampStep`（按 `step` 从 `min` 对齐，最后 `Math.round(x*1e6)/1e6`）；select/radio → 未知值回退
     **第一个选项**；checkboxes → 过滤、**去重并按 schema 选项顺序**。它同时用在写入
     （`applyConfigChange`）与**加载**（`configValues`）上。
   - 本构建：`internal/plugincfg/schema.go:160-210` 的 `coerce` 是**偏函数**（形状不符即拒绝、保留旧值），
     slider/number 只 `clamp` 不按 step 对齐，text 不 trim，select/radio 未知值不回退，checkboxes 不去重/不排序；
     加载路径 `internal/shell/pluginconfig.go:40-56` 直接搬 settings 的值，不过 `coerce`。
   - 验收（P7-ed 已按此落地）：先决定口径——现有 `internal/plugincfg/schema_test.go` 的
     `TestApplyRejectsAValueOutsideTheOptions` 把「拒绝越界选项」钉成有意行为，若改为原版的回退语义
     必须同时改该测试并说明理由；若保留拒绝，则把 step 对齐、text trim、checkboxes 去重排序这三处
     **补齐**（它们不与现有测试冲突），并加一张表测试逐条钉住。不要半改。
     **P7-ed 选择保留拒绝分支**，三处已补齐，表测试为
     `TestCoerceShapesValuesLikeTheReferenceNormalizer`。

### 具体待办（P7-ea 重写；第 1 条本轮落地 P7-ea）

> 本轮入口：逐文件对照原版的命令别名模块（`main:src/command-aliases.ts`）。它的搜索半边
> （`candidateMatchScore` / `commandMatchScore` / `resolveCommandAliases` / `matchedCommandAlias`）
> 本构建早已接好（`internal/launcher/search.go:809-850` 的 `aliasHint` / `aliasRank`、
> `internal/settings/plugins.go:159` 的 `ResolveCommandAliases`），但**还原半边**
> （`rebaseAliasCommandLine`）没有对应物。依赖检查同上，无版本可升。

1. **（已落地，P7-ea）PATH 命令的别名带参数时没有被还原成真命令，会去跑一个不存在的程序**。
   - 原版对照：`main:src/command-aliases.ts:184-201` 的 `rebaseAliasCommandLine`（把行首那个
     **恰好**等于别名的 token 换回真命令名）；由 `main:src/hooks/useLauncherCatalog.ts:1328-1334`
     使用——`typedAlias` 时 `rowCommandLine = rebaseAliasCommandLine(query, typedAlias, entry.command)`，
     原版注释把这条规则写死：「`gfm -m x` therefore executes `git -m x`, never a nonexistent shell
     command named `gfm`」。`matchedCommandAlias`（`:167-175`）限定只有**整 token、大小写不敏感**
     的相等才还原，前缀/包含命中仍是「发现面」。
   - 本构建（改前）：`internal/launcher/search.go:533` 的 PATH 行 `Run: func() { a.runTool(tool) }`
     只跑 `tool.Path`、不带参数；真正的带参路径是 `internal/launcher/actionbar.go:164` 的
     `ParseCommandLine(value)`，`argv` 里的命令 token 原样进终端（`:193-226` 两条 Run 闭包）。
     全仓没有还原半边：`grep -rni "rebase" internal cmd` 无命中。于是 `gfm status`
     （别名表 `{"git":"gfm"}`）送进终端的是 `gfm status`，一个不存在的程序。
   - **已落地（P7-ea）**：新增 `internal/launcher/aliasrebase.go`——`aliasToCommand`（按
     `settings.ResolveCommandAliases` 反查，冲突时名字升序的第一个命令拥有别名）与
     `RebaseAliasArgv`（只换行首命令 token，其余字节不动）。`internal/launcher/actionbar.go`
     在算出 `argv` 后、两条 Run 路线之前调一次 `RebaseAliasArgv(a.Aliases, argv)`，环境变量路线
     与应用命令名路线共用同一个 argv。shell 语法行（`gfm | grep x`）按原版范围**不还原**。
     测试：`internal/launcher/aliasrebase_test.go` 六条（纯函数五组含大小写/前缀/包含/冲突/空；
     经运行路径的 `gfm status`、`FOO=gfm gfm status` 环境路线、shell 语法不还原）。详见「最近几轮」P7-ea。

> 本轮 P7-dz 落地了第 1 条（删掉 `Item.section` 死代码）。第 2 条是框架死口子（非待办，仅记录）。
> 依赖检查同上，无版本可升。

1. **（已落地，P7-dz）`launcher.Item.section` 是全仓死代码**。
   - 原版对照：`main:src/launcher/list-sections.ts` 的 `pluginGroup` 规则——外部插件自己的
     `group` 字段要在列表行上方印一条标题（`:39-49` 的 `listSectionStarts`）。
   - 本构建（改前）：`internal/launcher/search.go:77` 声明了 `Item.section`、
     `internal/launcher/launcher.go:1532-1541` 读取它并画标题，但**全仓没有任何非测试代码写入
     `section`**（`grep -rn "section:" internal/launcher` 只有 `section:recent` / `section:dropped`
     两个 `heading: true` 的项）。原因是外部插件的行在本构建走 captured-output 视图，那里的
     `outputRow`（`internal/launcher/output.go:514-530`）已经按 `row.Group` 画了同一条标题；启动器
     列表里从不放外部插件行，所以分支不可达。
   - 验收：确认无写入后删除 `Item.section` 字段与 `launcher.go` 的读取分支（或保留时补一条注释说明
     其未来的生产者），`go test -count=1 ./internal/launcher` 全绿，不回归 `outputRow` 的 group 标题。
   - **已落地（P7-dz）**：删除 `Item.section` 字段与 `launcher.go` 的读取分支（及两处过时注释）；
     新增 `internal/launcher/rowsection_test.go` 的 `TestTheOrdinaryListChargesNoSectionBands`
     钉住「普通列表报告 0 条 section band、块标题只以 `heading` 行存在」。详见「最近几轮」P7-dz。

2. **（记录，非待办）error toast 的 `role="alert"` 是框架无枚举的死口子**。
   - 原版对照：`main:src/components/ToastStack.tsx:62` 的 `role={toast.kind === "error" ? "alert" : "status"}`。
   - 本构建：`mygo@v0.4.8:ui/access.go:21-56` 的 `ui.Role` 只有 `RoleStatus`（和 `RoleAlertDialog`），
     `ui/toast.go:289-297` 的 `ToastBase` 固定 `RoleStatus`。要让 error 档带 `RoleAlert` 需要框架先
     暴露该角色（或 `ToastBase` 读 `Type` 选角色），故不接死旋钮。P7-dy 已把可见的色调/字形接回。

### 具体待办（P7-dw 重写；三条已全部结清：第 1 条 P7-dx、第 2 条 P7-dw、第 3 条 P7-dv 无落点）

> 本轮 P7-dx 落地第 1 条（sessions 武装改用稳定 ID），并补齐终端层的身份字段与测试。依赖检查
> 同上，无版本可升。

1. **（已落地，P7-dx）sessions 的关闭武装用位置 `Index` 当身份，删会话后会落到别的会话上**。
   - 原版对照：`main:src/settings/SessionsPage.tsx:54` 的 `killArmedId` 是
     `string | null`——**稳定的 `session.sessionId`**（`:199` 的 `key={session.sessionId}`、
     `:224/:241` 的 `killArmedId === session.sessionId`），列表重排/增删都不改它的指向。
   - 本构建（改前）：`internal/settingsui/settingsui.go:490` 的 `armedSession` 存的是
     `session.Index`（驱动它的 `internal/shell/shell.go:812-829` 的 `Sessions` 提供者按切片位置
     编号）。会话被删时切片收缩、后面的 index 前移。
   - **已落地（P7-dx）**：`terminalui.Session` 加稳定 `ID`（`nextSessionID` 计数器，
     `startSession` 每建一个 ++ 一次）；`settingsui.Session` 加 `ID`，`armedSession` 改存 ID，
     confirm 用当前帧的 `session.Index`；列表里行消失（`sessionInList` 为 false）即自动解除。
     测试：`internal/terminalui/sessionid_test.go` 的 `TestSessionIDsAreStableAcrossDeletion`、
     `internal/settingsui/sessionsidentity_test.go` 两条。详见「最近几轮」P7-dx。

### 具体待办（P7-dt 重写；三条已全部结清：第 1 条 P7-dt、第 2 条 P7-du、第 3 条 P7-dv）

> 本轮把「逐文件对照原版」的入口放在终端的空态与其快捷键归属上，第 1 条是已核实的真实偏离
> （本轮落地），第 2·3 条是同一张表里剩下的具体项。依赖检查同上，无版本可升。

1. **（已落地，P7-dt）终端空态的「返回」那行印的不是配置里的 new-command 键，绑的也不是它**。
   - 原版对照：`main:src/App.tsx:3863-3872`——空态第二行的 keycap 是
     `formatShortcut(shortcuts.new_command)`，点击是 `returnToInputMode`；`main:src/surface-policy.ts:207-215`
     的 `DISMISS_TABLE.terminal.escape` 是 **null**（Escape 归运行中的 shell），`new-command` 才
     `return-to-input`。
   - 本构建（改前）：`internal/terminalui/terminalui.go:711-724` 印字面量 `"esc"`、点击
     `Actions.Close`，并在视图里 `c.Shortcut(0, ui.KeyEscape)` 抢走 Escape；而 new-command 键由
     `internal/shell/shell.go:3031-3033` 的 `appShortcuts` → `returnToInput` 单独回答。
   - **已落地（P7-dt）**：`emptyLine` 改收已排好的 keycap 文本，新增 `newCommandBadge`
     （`shortcuts.Display(settings.Shortcut(s, ShortcutNewCommand))`）；返回行改用
     `Actions.NewCommand`，shell 的该 action 改接 `returnToInput`（与 header 的「New command」
     同一入口）；删掉空态对 Escape 的认领。详见「最近几轮」P7-dt。
2. **（已落地，P7-du）launcher 上被改绑后，字面 ⌘W 不应该再隐藏窗口**。
   - 原版对照：`main:src/surface-policy.ts:207-211`——`collapsed.mod-w` 是 **null**（只有可配置的
     `new_command` 触发折叠；默认同为 ⌘W 掩盖了差别），`collapsed.new-command` → `hide-window`。
   - 本构建（改前）：`internal/shell/shell.go` 的 default（launcher）分支无条件
     `c.Shortcut(ui.Cmd, ui.KeyW)` → `Hide()`，把字面 ⌘W 折进了 new-command（注释自认
     「the same key by default」），于是用户把 `new_command` 改绑到别处后，⌘W 仍隐藏。
   - **已落地（P7-du）**：删掉该字面 ⌘W 认领，launcher 只认 new-command 绑定；settings 分支的
     字面 ⌘W（原版确有 close-settings 规则）保留；加 `App.hideWindow` 缝让测试能看见 `Hide()`。
     测试：`TestDismissTable` 加隐藏计数 + 新增 `TestALiteralModWDoesNothingAfterARebind`。
     详见「最近几轮」P7-du。
3. **（已结清，P7-dv）terminal 的 new-command 没有 key-up reassert**。
   - 原版对照：`main:src/surface-policy.ts:214` 的 `terminal.new-command` 带 `reassertOnKeyUp: true`
     ——松开修饰键后重新把焦点压回启动器输入框（`main:src/App.tsx:1789-1812`）。
   - 本构建：`internal/shell/shell.go:3040-3044` 只有 key-down 的 `c.Shortcut` 分支，没有 key-up
     处理。
   - **结论（P7-dv）**：核实为**无落点**——mygo 的焦点层每帧重压启动器输入框
     （`internal/launcher/launcher.go:762` 的 `FocusSearch`），且 NSView 是整个 surface 唯一的原生
     first responder（`mygo@v0.4.8:internal/darwin/surface.go:667,800-809`），没有「Cmd 松手再动
     first responder」这个事件可挂。详见「最近几轮」P7-dv。
4. **（记录，非待办）ansi 解析器有三处是原版的超集**（`internal/ansi/ansi.go`）：
   - `skipEscape`（`:170`）把 `ESC P/X/^/_` 当字符串序列整体吞掉；原版 `main:src/launcher/ansi.ts:283`
     只对 `ESC ]` 这样做，其余两字节转义之后的内容会当文本印出。
   - `applySGR`（`:222`）支持冒号分隔的颜色（`38:5:1`）；原版 `parseSgrParams` 只按 `;` 拆，
     `Number("38:5:1")` 是 NaN → 忽略。
   - `sgrValue`（`:283`）用 `strconv.Atoi`；原版用 `Number()`，认 `0x10`/`1e2` 这类写法。
   - 三处都是「本构建更宽」，不回归对齐（对齐会把 DCS 载荷印成可见文本）；在此记录为**有意的
     超集**，不是待办。

### 具体待办（P7-ds 重写；两条已全部结清：第 1 条 P7-ds、第 2 条 P7-dt 核实为框架无口子）

> 本轮关闭了 P7-dr 第 1 条（核实为框架无逐单元格口子，已记「已知差距」），并落地了同一段代码里
> 新发现的 inherit 光标色偏离。下面只留本轮新发现、仍未做的一条；P7-dq 清单的第 2·3 条
> （终端空态的 240ms 延迟、自定义集成的脏比较/溢出菜单）仍未做，仍在下一份清单里。依赖检查：
> GitHub releases API 限流，`go list -m -versions github.com/egoist/mygo` 最新仍是 **v0.4.8**
> （与 `go.mod` 一致），无版本可升。只对照 `main`（`backup/main-pre-mygo` = v0.3.14 原版树），
> 未翻历史提交。

1. **（已落地，P7-ds）`inherit` 的光标色应取 `--terminal-cursor`，不是前景色**。
   - 原版对照：`main:src/terminal/render.ts:242-244`——`updateTheme()` 对 `inherit` 读
     `packedColor(style, "--terminal-cursor", …)`；`main:src/styles/base.css:718` 暗色 **#8bd5ca**、
     `:975/:1107` 亮色 **#007c86**，都与 `--terminal-fg` 不同。
   - 本构建（改前）：`palette("inherit")` 返回 nil，插件回落到零光标规则（前景色）。
   - **已落地（P7-ds）**：`themeFor` 返回亮/暗一对、只覆盖 `Cursor`（`inheritCursor`），
     `options()` 同时给 `Theme`/`DarkTheme`，预览共用同一 token。详见「最近几轮」P7-ds。
2. **（待比对，疑框架无口子）块光标的字形颜色应取单元格自己的前景色**（P7-ds 核实为框架无口子，
   已记「已知差距」）。
   - 原版对照：`main:src/terminal/render.ts:586-591`——块光标用**该单元格自己的前景色**重绘字形。
   - 本构建：`plugins/terminal/paint.go:500-505` 固定 `text = color(v.colors.Background)`，
     `Theme.CursorText.A > 0` 时才能换成一个**全局**色。无逐单元格口子，不半接。

### 具体待办（P7-dr 重写；已全部结清：第 1 条 P7-ds 核实为框架无口子）

> P7-ds 核实第 1 条无逐单元格钩子，写实进「已知差距」；P7-dq 清单第 2·3 条仍未做，见下面那份。
> 依赖检查：GitHub releases API 限流，`go list -m -versions github.com/egoist/mygo` 最新仍是
> **v0.4.8**（与 `go.mod` 一致），无版本可升。只对照 `main`（`backup/main-pre-mygo` = v0.3.14
> 原版树），未翻历史提交。

1. **块光标的字形颜色应取单元格自己的前景色**（**已核实为框架无口子，P7-ds**）。
   - 原版对照：`main:src/terminal/render.ts:586-591`——块光标先用自己的颜色铺满单元格，再用
     **该单元格自己的前景色**重绘字形（`base = cellFg === WIRE_FG ? this.fg : cellFg`，加粗的
     亮色模式还会 `brighten(base)`）；这是逐单元格的值。
   - 本构建：mygo 插件固定 `text = color(v.colors.Background)`（`plugins/terminal/paint.go:504`），
     除非 `Theme.CursorText.A > 0`（本构建从不设）。于是光标块下的字用的是**主题背景色**，原版
     用的是该字原本的颜色。
   - 验收：先确认框架是否真有逐单元格的口子（`Theme.CursorText` 只能给一个全局色，且 `inherit`
     根本没有主题）；若有则接，若无则在「已知差距」里写实（已初记）。

### 具体待办（P7-dq 重写；第 1 条已核对关闭于 P7-dr，第 2·3 条仍未做）

> 第 1 条在 P7-dr 核对为「无落空」（并发现落地了 `Transparent` 那条偏离）；第 2·3 条仍未做，
> 留在这里，做完上面 P7-dr 清单那条后回来取。

1. **（待比对，本轮新发现）终端外观除「进入」之外还有没有别的写入时机**（**已核对关闭，P7-dr**）。
   - 原版对照：`main:src/hooks/useTerminalView.ts:502-521`——进入终端（`mode` 变到 `terminal`、
     renderer 为 null）就新建 `TerminalCanvas`，构造参数是当帧的 `fontFamily`/`fontSize`/
     `lineHeight`/`padding`/`theme`/`cursorBlink`/`showScrollbar`/`wheelLines`/`boldMode`。
   - 本构建（P7-dq 后）：`internal/shell/shell.go:2561` 的 `Open(SurfaceTerminal)` 在
     `EnsureSession()` 后 `a.refreshTerminal()`；但 `RunShellIn`（`shell.go:538` 附近）、命令会话
     （`shell.go:869` 的 `CommandSessionEnded`）等会自行建会话，未必经过一次「进入」。
   - **结论（P7-dr）**：逐条走完所有 `Open(SurfaceTerminal)` 与 `a.Terminal.Run*` 调用点，它们
     都不新建第二个入口——所有会话都经 `App.startSession` → `options()` 读**当帧**
     `Store.Snapshot()`，`a.NewTerminal` 全仓只出现一次（`terminalui.go:206`）。新会话必得当帧外观，
     旧会话由入场刷新（P7-dq）覆盖，**无落空**。核对时发现并落地的真实偏离见 P7-dr（`Transparent`
     是创建期选项，跨 inherit/命名配色必然过期）。
2. **（已关闭，P7-dt）终端空态的 240ms 延迟——无落点**。
   - 原版对照：`main:src/terminal/empty-state.ts:39` 的 `TERMINAL_EMPTY_HINT_DELAY = 240`——防的是
     「异步 `term_spawn` 往返期间页面短暂为空 → 空态闪一下」。
   - 本构建：`EnsureSession`（`internal/terminalui/terminalui.go:256`）同步调 `startSession`
     （`:167`）同步拿回 `*terminal.Terminal`（PTY 在插件内部自行异步启动，但页面从第一帧就有
     会话对象），空态只在**同步失败**（`a.Err`）或所有会话被关时出现（`:698-728`），没有那个
     「建好了但身份还没回来」的窗口。
   - **结论（P7-dt）**：全仓没有异步入队后再翻身份的 spawn 路径，延迟无处可挂，**无落点**。
3. **（已关闭，P7-dt）自定义集成的脏比较（R9-4）与溢出菜单（R7-3b）——无落点**。
   - 原版对照：`main:src/extensions/integration-dirty.ts` 的 `customIntegrationDirty`（防「打开旧
     manifest 就被判脏 → 关闭被 discard bar 拦」）与 `main:src/ExtensionsPanel.tsx:2227-2242` 的
     `OverflowMenu`（菜单里只有 Export/Import 两项，`main:src/ExtensionsPanel.tsx:2338` 才渲染）。
   - 本构建：**没有自定义集成抽屉**——创建走 `floter connect` 深链/CLI，集成页只有运行时表单
     （`internal/settingsui/settingsui.go:3781` 的 `toolParamsEditor`），没有「编辑一份 manifest
     再保存」的模态，也没有 discard bar，故脏比较无处可挂；导出/导入由 `transferRow` 的两个直按钮
     直接暴露（与菜单同一对动作），菜单那一层本来就是为「把两个按钮搬进去」而存在。
   - **结论（P7-dt）**：两个动作都可达，脏比较没有对应模态，**无落点**。

### 具体待办（P7-do 重写；三条已全部结清：第 1 条 P7-dp，第 2·3 条 P7-dq 比对为覆盖/无落点）

> 本轮换入口：清点原版 `src/` 中在 status/plan 两份文档里零命名的模块（`git ls-tree` + 逐文件 grep），
> 得到 `src/components/OverflowMenu.tsx`、`src/hooks/useSessionManagement.ts`、`src/deferred-repaint.ts`、
> `src/launcher/trigger-hint.ts`、`src/launcher/command-row.ts`。逐个对照本构建后：`command-row.ts`
> 与 `useSessionManagement.ts` 是 React/JS 特有的重构与轮询卫生（Go 的具名字段结构体与即时模式
> 渲染已天然避开），**无落点、不新增死旋钮**；`trigger-hint.ts` 有下面第 1 条的真实偏离。
> 依赖检查：GitHub releases API 被限流，`go list -m -versions github.com/egoist/mygo` 最新仍是
> **v0.4.8**（与 `go.mod` 一致），无版本可升。只对照 `main`（`backup/main-pre-mygo` = v0.3.14 原版树）。

1. **触发提示的下限是「提示自身不足 6ch」，不是「输入框被挤到 160px 以下」**（已落地，P7-dp）。
   - 原版对照：`main:src/launcher/trigger-hint.ts:31-38` 的 `TRIGGER_HINT_MIN_CELLS = 6`（注释：
     低于 6 `ch` 时省略号只剩两个字形，提示不如整体消失）与 `:44-48` 的
     `triggerHintFits(width, cellWidth, minCells)`——`width` 是**提示自己**这一遍布局拿到的像素盒宽，
     `cellWidth` 是它自己字体里一个 `ch` 的推进宽度；`main:src/hooks/useTriggerHintFit.ts:37-52`
     在 layout 阶段量 `getBoundingClientRect().width` 与 canvas 的 `measureText("0")`。
   - 本构建（改前）：`internal/launcher/triggerhint.go:88` 的 `triggerHintMinField = 160` 是一个固定的
     **输入框**下限，`:94-104` 的 `triggerHintFits(c, text)` 量的是**提示全文**宽度，判据是
     `width - hint >= 160`（`width` 是整行窗口宽）。于是「输入框 < 160px」与「提示 < 6ch」是两个不同
     阈值：提示够 6ch 但输入框不到 160px 时，原版显示、本构建隐藏。
   - 验收：在 `ui.NewTester` 下，提示被隐藏当且仅当它在当前字体步长下的盒宽不足 6 个 `ch`；
     字体不可量（`MeasureText` 返回 ≤ 0）时保留提示。渲染侧 `internal/launcher/launcher.go:965`
     的调用点要保持「隐藏是唯一的丢失路径」这条注释语义。
   - 既有测试：`internal/launcher/triggerhint_test.go` 的 `TestTriggerHintFits`（宽窗有、200px 无）
     要重新钉在 6ch 边界，而不是 160px 边界。
   - **已落地（P7-dp）**：`triggerHintMinCells = 6` + 纯函数 `triggerHintFits(room, natural, cell)`
     + `(a *App) triggerHintRoom`（行留给提示的盒宽，输入框是 grower 不占它）+ `triggerHintShown`；
     调用点加 `.SingleLine().Ellipsis("\u2026")`（对齐原版 `nowrap`+`ellipsis`，提示永远一行）。
     测试 `TestTriggerHintFits` 重写：6ch 边界、提示自身短于下限、`cell<=0` 保命、200px 盒宽 ==
     `triggerHintRoom` 且一行、120px 隐藏。详见「最近几轮」P7-dp。
2. **ExtensionsPanel 工具栏的溢出菜单（R7-3b）是否在本构建有落点**（已比对：已用另一种形式覆盖，P7-dq）。
   - 原版对照：`main:src/components/OverflowMenu.tsx:1-26`（频率拆分：高频动作留按钮，稀有动作连同
     其说明搬进菜单）与它在 `main:src/ExtensionsPanel.tsx:2338` 的**唯一**用法（触发键、↑/↓ 环绕、
     Home/End、Escape 自己 `stopPropagation`）。
   - 本构建：`internal/settingsui` 的集成页（`settingsui.go` 的 `PageIntegrations` 及其 view）没有
     任何溢出菜单。
   - **结论（P7-dq）**：原版那处 `OverflowMenu` 里只装了 export/import 两项（其余工具区动作留在
     按钮上）；本构建 `internal/settingsui/settingsui.go` 的 `transferRow` 给 Export/Import 各一个
     直按钮，`integrations` 的 recommended/localTool/transfer/install/installFolder 各段也都是直接
     控件，每个动作都能触达。「把两项搬进菜单」是呈现形式而非能力，**不新增一个只有两项、还得
     自己实现 ↑/↓/Home/End/Escape 的菜单**。若以后工具栏真被挤爆再加。
3. **透明度拖拽时的终端重绘合并（GLASS-CLIP / `src/deferred-repaint.ts`）**（已比对：无落点，P7-dq）。
   - 原版对照：`main:src/deferred-repaint.ts:1-90`（CSS 变量逐 tick、终端重绘 140ms 尾沿合并，注释：
     「透明度的滑杆拖动会中断」）与 `TERMINAL_REPAINT_DEBOUNCE_MS = 140`。
   - 本构建：终端由 mygo 逐帧绘制，透明度滑杆写的是 settings。
   - **结论（P7-dq）**：两个理由说明它没有落点。(1) 原版合并的是**不改变任何像素**的
     透明度（CSS 变量）；mygo 的 `SetFont`/`SetTheme`（`plugins/terminal/terminal.go:507,516`）只把
     插件标脏，真正的重绘由**帧循环天然合并**（每一帧最多一次），没有原版那种「逐 tick 强制重绘」
     的成本。 (2) 本构建的透明度滑杆在 **Settings 面**（`settingsui.go:2136` 的终端外观区），终端
     面上没有任何透明度控件；把它烘进插件背景 alpha 的 `palette()`（`terminalui.go:1107`）只在
     `Surf == SurfaceTerminal` 时应用，拖动透明度时根本走不到终端。加一个 140ms 的尾沿合并只是
     给一个不存在的问题加复杂度，**不落地**（已在 P7-dq 记录）。

### 具体待办（P7-dn 重写；三条已全部落地：第 1·2 条 P7-dn、第 3 条 P7-do）

> 本轮换入口：逐文件对照原版的**安装目录缓存**（`src/extensions/tool-catalog-store.ts` 的
> `loadToolCatalog`/`refreshToolCatalog`/`useToolCatalog`，以及它的两个刷新调用点
> `src/App.tsx:2453-2457` 与 `:781-788`）。此模块此前未被任一历史待办覆盖。依赖检查：GitHub
> releases API 被限流，用 `go list -m -versions github.com/egoist/mygo` 核对，最新仍是 **v0.4.8**
> （与 `go.mod` 一致），无版本可升。只对照 `main`（`backup/main-pre-mygo` = v0.3.14 原版树），
> 未翻历史提交。

1. **revel 与从终端页归位时重读安装目录**（已落地，P7-dn）。
   - 原版对照：`main:src/App.tsx:2453-2457`——`floter://revealed` 监听里 `void refreshToolCatalog()`，
     注释明写“一次 reveal 一次刷新”、无轮询无定时器；`main:src/App.tsx:781-788`（R74）——
     `returnToInputMode` 再 `void refreshToolCatalog()`，因为安装行自己的终端门“never hides the
     window”，reveal 监听看不到这次返回；`main:src/extensions/tool-catalog-store.ts:52-63` 的
     `refreshToolCatalog` 在飞则复用 `inFlight`。
   - 本构建（改前）：`internal/shell/shell.go:1208` 的 `scanToolCatalog` 只在启动
     （`shell.go:1107`）与托盘 Rescan（`internal/shell/system.go:495`）被调；`summon()`
     （`shell.go:2652`）与 `returnToInput()`（`shell.go:3012`）都不读，也没有在飞去重。
   - 验收：一次 summon 重读一次机器；从终端页 `returnToInput` 也重读（即使窗口从未隐藏）；两次
     紧挨的刷新只跑一次扫描；扫描落地后重绘。
   - **已落地**：`Options.LookTools` + `scanToolCatalog` 的在飞去重（`catalogMu`/`catalogScanning`）；
     `summon()`、`returnToInput()` 各加一处调用。测试 `TestASummonRefreshesTheInstallCatalog`、
     `TestTheWayBackRefreshesTheInstallCatalog`、`TestOneCatalogScanRunsAtATime`、
     `TestARefreshReplacesTheInstallCatalog`。详见「最近几轮」P7-dn。
2. **“一次进程一次”的启动 memo 与“刷新替换 memo”语义**（已随第 1 条落地，P7-dn）。
   - 原版对照：`tool-catalog-store.ts:38-46` 的 `loadToolCatalog`（首次 fetch，之后 memo）与
     `:31-37` 的 `fetchReport`（失败→`report = null`，软着陆）。
   - 本构建：`InstallCatalog` 就是 memo，启动扫一次（`shell.go:1107`）；刷新整体替换
     （`SetToolCatalog`），与 `fetchReport` 的“整份替换”一致。`tools.Look` 永远返回整表，没有
     nil 失败分支，软着陆无处可落——不新增死旋钮。
   - 验收：刷新后的行来自最新一份扫描，不合并旧表；测试 `TestARefreshReplacesTheInstallCatalog`。
3. **深链/托盘 reveal 也走一次重读**（已落地，P7-do）。
   - 原版对照：`floter://revealed` 是原生 reveal 事件，凡窗口从隐藏变可见都发（`HandleURL` →
     `Open` → `Show` 也算）。
   - 本构建（P7-dn 时）：只接了 `summon()` 与 `returnToInput()` 两条；`HandleURL` 走 `Open()` →
     `Show()`，而 `Show()` 的 reveal 判定（`rehome := placementForReveal(a.Win.IsVisible())`）
     已经知道“窗口刚刚从隐藏变可见”，但那里没有去重后的刷新调用。
   - 验收：窗口隐藏时到达的 `floter://settings` / `floter://terminal` 深链把它带上前台后，安装目录
     也重读一次；已可见的 surface 切换（`rehome == false`）不重读。
   - **已落地（P7-do）**：刷新搬进 `Show()` 本身（`rehome := placementForReveal(a.windowWasVisible())`
     为真时 `a.scanToolCatalog()`），并新增 `App.windowVisible` 缝让 `Win == nil` 的测试能驱动两侧；
     测试 `TestARevealRefreshesTheInstallCatalog`、
     `TestASurfaceSwitchOnAVisiblePanelKeepsTheCatalogMemo`、`TestAVisiblePanelIsNotRehomed`。
     详见「最近几轮」P7-do。

### 待办（P7-dk 已落地：启动器动作栏的「在终端中打开」只在 Enter 会跑它时存在）

> P7-dj 收尾后处理用户报告的「底部应固定出现「在终端中打开」选项…目前缺失」。一条独立改动，它让
> 既有的 `internal/launcher` 测试重新对齐：动作栏行进入结果列表的**头部**，编号跳过它，
> `Results(…)[0]` 断言改为按 id 找行。

1. **动作栏的闸门与默认选择拆开**（已落地，P7-dk）。
   - 原版对照：`main:src/App.tsx:1547-1552` 的 `visibleActionBar`——`launcherResults.length > 0 ||
     fileRows.length > 0` 就画（拖入文件时走 `fileDropActionBar` 那支），**与 Enter 无关**；
     `main:src/hooks/useLauncherCatalog.ts:1432-1443` 的 `defaultsToActionBar` 才是选择：它调
     `shouldDefaultToActionBar(query, actionBar.type, launcherResults.length, runnableResultCount,
     hasRunnableCommandResult)`，而 `main:src/App.tsx:2019-2042` 的 effect 把它交给
     `setSelectedResultIndex(firstRunnableResultIndex …)` + `setSelectedActionBar(defaultsToActionBar)`；
     `main:src/App.tsx:3254-3260` 的 keyup 在按住修饰键松手时 `setSelectedActionBar(false)`。
   - 本构建：`internal/launcher/actionbar.go:96-102` 的 `actionShell && !a.defaultsToShell(other)` 直接
     `return nil`，于是「查一个能跑的命令」时整个终端行消失（用户看到的现象）；`defaultsToShell`
     （`:292-315`）已经是原版那条链的忠实翻译，错的是它被当成了**存在性**闸门。
   - 验收：query 命中可运行结果时终端行仍在列表里（`Results` 含 `bar:shell`），但默认选择仍是那条
     结果（`App.Selected` 不指向动作栏）；URL / 路径 query 默认选择仍是动作栏；空 query 无动作栏。
   - 落地：`actionBarItemsWith(other, gated)` 的 shell 闸门改成 `gated && actionShell && len(other)==0`
     （`internal/launcher/actionbar.go:124`），只有 search 传 `gated=true`；`actionBarItems()`
     （`actionbar.go:89`）不过闸，供 held-modifier 与下面的 Enter 兜底。`activate` 补上原版的
     `!launcherResults.length` 兜底：结果为空且 query 是命令行时直接跑 `actionBarItems()[0]`
     （`internal/launcher/launcher.go:1849-1856`），所以匹配不到任何东西的命令行仍能 Enter 运行，
     只是页面上不多出一行。`assignNumbers` 跳过 `bar:` 行（`launcher.go:432-470`），`defaultSelection`
     从第一条**非** `bar:` 的可运行行起步（`launcher.go:2047-2069`）。
   - 注意：原版的闸门看的是**匹配结果**而非空查询，所以“非空 query 就画”是错的（那会让一个匹配
     不到任何东西的命令行在“没有结果”时多出一行，且与 `visibleActionBar` 不符）。选择必须在 query
     变化时重算：本构建有大量不经过输入框的 `a.Query = …` 赋值（`history.go:53,70`、`calculator.go:62`、
     `output.go:372` 等），帧内对齐由 `defaultedQuery` 承担（`launcher.go:789-795`，`field.Changed()`
     分支 `:947-952`），它们与输入框三条路径共用同一个 `defaultSelection`。
   - 测试对齐：`search_test.go` 改用 `indexOfID`；`resultnumbers_test.go`/`view_test.go` 的编号断言
     跳过头部 `bar:` 行；匹配不到任何东西的那几项（`TestNoResultsShowsTheEmptyMessage`,
     `TestActionBarRows` 的 `git status` 段、`TestShellSyntax*`、`TestShellLineCarriesEnvironment`,
     `TestSystemCommandsJoinTheSearchWhenEnabled` 关设置那段）断言页面**没有** shell 行、并靠 Enter
     兜底验证仍会运行。本轮试过的“非空 query 就画”版本已丢弃，未提交。

### 具体待办（P7-di 重写；三条已全部落地：第 1 条 P7-di、第 2 条 P7-dj、第 3 条措辞 P7-dl + 异步 checking P7-dm）

> 本轮换入口：逐文件对照原版的**脚本工具链状态行**（`src/extensions/script-languages.ts` 的
> `scriptRuntimeStatus`、`CustomIntegrationDrawer.tsx` 的 `ScriptRuntimeLine`、`ExtensionsPanel.tsx`
> 的 `probeScriptRuntime` 缓存），把清单写成下面三条带 file:line 的具体待办，并取第 1 条落地。
> 依赖检查：GitHub releases API 被限流，用 `go list -m -versions github.com/egoist/mygo` 核对，
> 最新仍是 **v0.4.8**（与 `go.mod` 一致），无版本可升。只对照 `main`
> （`backup/main-pre-mygo` = v0.3.14 原版树），未翻历史提交。

1. **脚本工具链探测每帧重跑，原版按语言缓存一次**（已落地，P7-di）。
   - 原版对照：`main:src/ExtensionsPanel.tsx:822-828` 的 `runtimeChecksRef`/`runtimeChecksPending`/
     `runtimeChecksEpoch`，`:925-951` 的 `probeScriptRuntime`（缓存命中直接返回、在飞去重、
     `setRuntimeChecks(language: null)` 表示在飞），`:958-961` 的 effect（key 是语言），
     `:1558-1566`/`:1593-1595` 的打开/编辑时 `epoch += 1` 并清空缓存。
   - 本构建（改前）：`internal/settingsui/settingsui.go:2945` 的 `ui.Text(c, a.scriptRuntimeLine(...))`
     在每帧渲染里直接调 `a.Actions.ScriptRuntime(language)`（`:2974`），而
     `internal/shell/shell.go:583-585` 的 `ScriptRuntime` 是同步 `extensions.ScriptRuntimeFor`，会
     `exec.Command(path, "--version")`（3s 超时）——每帧/每次键入都起一个子进程。
   - 验收：同一语言在一次表单会话里只探测一次；切走再切回命中缓存；重新打开表单重新探测。
   - **已落地**：`internal/settingsui/settingsui.go` 的 `App.scriptRuntime map[string]scriptlang.Check`
     + `scriptRuntimeCheck`（命中返回、未命中探测后落缓存、`ScriptRuntime` 为 nil 时返回 false）
     + `clearScriptRuntimeCache`；打开表单的 toggle、`beginEdit`、`endEdit` 都清缓存。测试
     `internal/settingsui/pluginsheet_test.go` 的 `TestScriptRuntimeProbeIsCachedPerLanguage` /
     `TestReopeningTheLocalToolFormReprobesTheToolchain`。变异核查：删掉缓存查找，两条测试立刻红。
2. **缺工具链的脚本保存时应警告一次**（已落地，P7-dj，不阻止保存）。
   - 原版对照：`main:src/ExtensionsPanel.tsx:1863-1875` 的 `createCustomIntegration`——script 模式下
     `await probeScriptRuntime(customIntegration.scriptLanguage)`，`!check.available` 时
     `onNotify("warning", t("settings.extensions.customScriptRuntimeSaveWarning", { language }))`；
     i18n key 在 `main:src/i18n.ts`（en/zh）。注释明写：缺工具链**不阻止保存**，只警告一次，
     「so the integration is not silently dead on arrival」。
   - 本构建：`internal/settingsui/settingsui.go:2796-2804` 的提交路径只做 `customIssues` 内联校验，
     缺工具链既不是 issue 也没有 toast（`settingsui.Actions` 没有 Notify/Toast 动作）。
   - 验收：script 提交且工具链缺失 → 一条 warning toast（`{language}` 用语言名），**仍提交**；
     工具链在 → 无 toast。需要给 settingsui 加一个 Notify 动作（或复用 shell 的 `notifyCompletion`）
     并补 i18n 词 `LocalToolRuntimeSaveWarning`。
   - **已落地（P7-dj）**：`settingsui.Actions` 新增 `NotifyWarning func(text)`；`settingsui.go` 的提交
     路径在 script 模式下读 `scriptRuntimeCheck(tool.ScriptLanguage)`（复用 P7-di 的缓存，不再起子进程），
     `scriptlang.StatusOf(...).State == Missing` 时发一句，**不阻止** `ConnectLocalTool`/`EditCustomTool`；
     没接线探测或工具链在 → 不说。i18n 新增 `LocalToolRuntimeSaveWarning(language)`
     （`internal/i18n/i18n.go:830`、`copy.go` en/zh），`{language}` 用 `scriptlang.Lookup(id).Label`。
     落点：`internal/shell/notices.go` 的 `warnNotice`（`ui.Toast`，6s = 原版 `TOAST_DISMISS_MS.warning`；
     `queueNotice` 的窗口改成参数，refusal 仍是 8s），由 `shell.go` 的 `NotifyWarning` 接线。测试：
     `pluginsheet_test.go` 的 `TestSavingAScriptWithoutItsToolchainWarnsAndStillSaves` /
     `TestAScriptSaveWarnsOnlyFromAVerdict`，`notices_test.go` 的 `TestWarningNoticeShowsWhereTheUserIs`。
     变异核查：关掉 `NotifyWarning` 那段分支，settingsui 那条测试立刻红。
     **诚实记录**：框架默认的 toast 外观只画一种色调（`ui/engine.buildToasts` 不读 `Toast.Type`），
     原版的 warning 色调在这里无处可落，所以这条 notice 不设 `Type`——设了就是个没有实现的旋钮。
3. **状态行措辞与旧字典不一致，且缺 checking 态**（已全部落地：措辞 P7-dl，异步 checking P7-dm）。
   - 原版对照：`main:src/extensions/CustomIntegrationDrawer.tsx:112-121` 的 `ScriptRuntimeLine`——
     `check === null` → `customScriptRuntimeChecking`（en "Checking the local toolchain…" / zh
     "正在检查本机工具链…"，`main:src/i18n.ts:696,1576`）；missing → `customScriptRuntimeMissing`
     （en "Not found on PATH: {names}. Install it, then reopen this form to check again." / zh
     "在 PATH 中未找到：{names}。安装后重新打开此表单即可再次检查。"，`:697,1577`）；ready →
     `${name} ${version}` 或 `name`（**没有** "is ready" 后缀），后接 `customScriptCompiled`
     （en "built before running" / zh "运行前先编译"，`:699,1579`）与可选的 `path`。
   - 本构建：`internal/i18n/copy.go:818-825`（en）/`:1567-1574`（zh）的 `LocalToolRuntimeReady`
     自造了 "is ready"，`LocalToolRuntimeMissing` 改写成 "{names} is not on PATH. Install it and come
     back."，`LocalToolRuntimeCompiled` 改写成 "The toolchain builds this script at install time."，
     且没有 checking 词；`internal/scriptlang/scriptlang.go` 定义了 `Unknown` 态，但 `StatusOf` 对零值
     `Check` 返回 `Missing`（与它自己「zero Check means not asked yet」的注释相矛盾）。
   - 验收（措辞，可独立落地）：三条词逐字对齐旧字典（ready 去掉 "is ready"）；`StatusOf` 对「没问过」
     的 `Check` 返回 `Unknown`。**卡点**：本构建的探测是同步的，渲染帧里不存在「在飞」的时刻，所以
     `Unknown`/checking 的渲染没有真实落点——要么先引入异步探测（`go func` + `Invalidate`，对齐原版
     的 `null` → 检查中），要么只做措辞对齐；没有异步前不新增 checking 词，避免再造死旋钮。
   - **已落地（P7-dl，措辞与 `Unknown` 语义）**：`scriptlang.Check.Asked()` + `StatusOf` 的零值 →
     `Unknown`；`copy.go` 的 en/zh 三句逐字对齐旧字典；`scriptRuntimeLine` 走 `StatusOf` 的 switch，
     `Unknown` 落到工具链名字。
   - **已落地（P7-dm，异步 checking）**：探测移出渲染帧（`runtimeProbe` goroutine +
     `Actions.Invalidate` 接 `a.Win.Invalidate`），`scriptRuntimeLine` 对「在飞」返回
     `LocalToolRuntimeChecking`（新增 en/zh 词，逐字对齐旧字典）；`clearScriptRuntimeCache` 用 epoch
     守卫丢弃关表单后落地的旧探测。保存路径走 `scriptRuntimeVerdict` 等待（原版 `await`）。
     详见「最近几轮」P7-dm。

### 具体待办（P7-dg 重写；第 1 条已落地 P7-dg，第 2 条已落地 P7-dh，余下第 3 条低优先级）

> 本轮换入口：逐文件对照原版的**插件配置写入路径**（`src/plugins/config-persist.ts`、
> `src/plugins/config-schema.ts`、`src/plugins/PluginConfigOverlay.tsx` 的 `handleChange`/`persist`），
> 把清单写成下面三条带 file:line 的具体待办，并取第 1 条落地（见「最近几轮」P7-dg）。
> 依赖检查：GitHub releases API 被限流，用 `proxy.golang.org` 的 `@v/list` 与
> `go list -m -versions` 双核对，最新仍是 **v0.4.8**（与 `go.mod` 一致），无版本可升。
> 只对照 `main`（`backup/main-pre-mygo` = v0.3.14 原版树），未翻历史提交。

1. **一次被拒绝的配置写入不回落到上次确认的值**（已落地，P7-dg）。
   - 原版对照：`main:src/plugins/config-persist.ts:44-50` 的 `configChangeOutcome`——`accepted` 为假时
     返回 `{kind:"rollback", values: committed, feedback: "settings.saveFailed"}`；调用点
     `main:src/plugins/PluginConfigOverlay.tsx:233-249` 的 `handleChange` 在 `rollback` 时把
     `valuesRef.current`/`setValues` 都改回 `outcome.values` 并置 `setPersistFailed(true)`。
   - 本构建（改前）：`internal/launcher/config.go:98-115` 的 `ChangeConfig` 先 `a.config.Values = next`，
     被拒绝（`stored == nil`）时只 `a.config.Failed = true; return`，上屏值留在 `a.config.Values` 里。
   - 验收：被拒绝→`Values` 回到改写前的快照且 `Failed` 为真；被接受→`Values` 为插件归一化后的 `stored`
     且 `Failed` 为假；第二次拒绝回到**上一次被接受**的快照，而不是打开时的初值。
   - **已落地**：`internal/plugincfg/persist.go` 的
     `ConfigChangeOutcome(next, committed Values, accepted bool) (values Values, failed bool)`；
     `internal/launcher/config.go` 的 `ChangeConfig` 在改写前把 `a.config.Values` 记为 `committed`
     （每条路径都让它等于后端最后确认的值，所以无需再加一个会漂移的 `Committed` 字段），拒绝时
     `a.config.Values = values` 回落，接受时仍以 `stored` 为准。
     测试 `internal/plugincfg/persist_test.go` 的 `TestConfigChangeOutcomeCommitsOrRollsBack` /
     `TestConfigChangeOutcomeRollsBackEveryField`，`internal/launcher/config_test.go` 的
     `TestRefusedConfigChangeRollsBack` / `TestAcceptedConfigChangeCommitsTheStoredValues` /
     `TestRefusedChangeRollsBackToTheLastAccepted`。变异核查：把 `a.config.Values = values` 改成
     `a.config.Values = next`（继续显示上屏值），`TestRefusedConfigChangeRollsBack` 与
     `TestRefusedChangeRollsBackToTheLastAccepted` 立刻红。
2. **`writeConfigChange` 把「写被拒绝」变成 `false` 而不是抛出，且拒绝的反馈行是 `settings.saveFailed`**
   （已落地，P7-dh）。
   - 原版对照：`main:src/plugins/config-persist.ts:25-33` 的 `writeConfigChange`（`try { await write(); return true }
     catch { return false }`）与 `:44-50` 的 `feedback: "settings.saveFailed"`；渲染点
     `main:src/plugins/PluginConfigOverlay.tsx:299-309` 的 `plugin-config-action__status`（`loadFailed` 时
     优先 `settings.pluginConfigLoadFailed`）。
   - 本构建：`internal/launcher/config.go` 的 `Config.Failed bool` 由 `internal/shell/pluginconfig.go` 的
     `changePluginConfig`（返回 nil 表示拒绝）与 `runPluginAction` 共同驱动，渲染在
     `internal/settingsui/pluginsheet.go:70-75`（`copy.SaveFailed`）。**尚缺**：`Failed` 把「写失败」与「动作失败」
     混成一个 bool，且没有原版 R126 的「读失败优先」那条线（本构建 `pluginConfig` 同步读、不会失败，
     所以这条只有在框架/异步读出现后才有落点）。
   - 验收：一次拒绝后失败行是 `settings.saveFailed`；`Failed` 由动作（clear_history）置起时
     不误报成写失败。
   - **已落地（P7-dh）**：没有合并两个来源，而是按原版**分开**——`launcher.Config.Failed` 只由
     `ChangeConfig` 的 rollback 置起（写入线，`settings.saveFailed`）；新增 `Config.ActionFailed string`
     由 `RunConfig` 置起，`settingsui` 的 `sheetAction` 在未 arm 时画 `field.Failed`（`ActionControl`
     的独立 `failed` 状态），`ArmConfig` 在重新 arm 时清掉它。原版 R126 的「读失败优先」在本构建
     仍无落点（`pluginConfig` 同步读、不会失败），留待异步读出现；理由与规格写在上一条清单里。
     测试与变异核查见「最近几轮」P7-dh。
3. **`configValuesEqual` 的「无变化不写」没有落点**（待做，优先级低）。
   - 原版对照：`main:src/plugins/config-schema.ts:552-563` 的 `configValuesEqual`（逐 schema 字段比较，
     列表按下标）。原版里它**没有被任何调用点引用**（`git grep configValuesEqual` 只命中定义），
     所以这是一条「原版自己也没接线」的死函数。
   - 本构建：`internal/plugincfg/schema.go` 的 `Schema.Apply` 总是返回新 `Values`，`ChangeConfig` 每次
     都发一次写。
   - 验收（若做）：把 `configValuesEqual` 移植成 `plugincfg.ValuesEqual(schema, a, b)` 并先论证一个真实
     落点（例如「写前的值与后端已存值相同就跳过写」）；**没有落点前不做**，避免再造死旋钮。

### 具体待办（P7-dd 重写；第 1 条已落地 P7-de，第 2 条已落地 P7-df，余下第 3 条卡 mygo 框架）

> 本轮换入口：逐文件对照原版的**系统通知**（`src-tauri/src/notifications.rs` + 它的四个触发点），
> 把清单写成下面三条带 file:line 的具体待办。第 1 条（前台判定）已由 P7-de 落地，第 2 条
> （剪贴板清空历史的完成反馈）已由 P7-df 落地，第 3 条卡 mygo 框架。依赖检查：
> GitHub releases API 被限流，用 `go list -m -versions github.com/egoist/mygo` 核对，最新仍是
> **v0.4.8**（与 `go.mod` 一致），无版本可升。只对照 `main`（`backup/main-pre-mygo` = v0.3.14
> 原版树），未翻历史提交。

1. **`notifyCompletion` 的前台判定只看 `IsVisible()`，漏了 `IsFocused()`**（已落地，P7-de）。
   - 原版对照：`main:src-tauri/src/notifications.rs:133-165`——`should_notify` 之上是
     `panel_foreground_from(flag_visible, window_visible, window_focused)`：**任一**信号说「在屏」
     就算 `Visible`；`panel_foreground`（`:168-190`）组合 `AppState::window_visible` +
     `Window::is_visible()` + `Window::is_focused()`，注释明写「a focused window is on screen by
     definition, even on the platforms where the panel's visibility is reported late（macOS
     `NSPanel` before its first map）」；没有窗口算 `Hidden`。
   - 本构建：`internal/shell/shell.go:2386` 的 `notifyCompletion` 只
     `a.Win != nil && a.Win.IsVisible()`；`a.Win.IsFocused()` 在同一文件的 `:2549`/`:2960` 已在用，
     证明旋钮存在（`mygo@v0.4.8/window.go:674`）。
   - 验收：抽纯函数 `panelForeground(visible, focused bool) bool`（或三参带 app 记录），
     `notifyCompletion` 用它；表测试四象限（visible/focused 任一为真 → 面板内反馈、不通知；
     都假 → 通知）。变异核查：删掉 `|| focused` → 「focused 但 IsVisible 为假」一行红。
     **已落地**：`shell.go:2375-2393` 的 `panelForeground`（`visible || focused`），
     `notifyCompletion`（`:2399`）与 `notifyRun`（`:2337`）都改走它；未引入原版的第三个信号
     （应用自己的 `window_visible` 记录），因为 mygo 只暴露 `IsVisible`/`IsFocused` 两个旋钮
     （`mygo@v0.4.8/window.go`）、本构建也没有等价的应用级记录。
     测试 `internal/shell/appkeys_test.go` 的 `TestPanelForegroundAnyOnScreenSignalWins`（四象限）。
     变异核查：`return visible || focused` → `return visible`，
     `focused but visibility reported late` 一行红。
2. **剪贴板清空历史没有完成反馈/通知（原版四个触发点之一）**（已落地，P7-df）。
   - 原版对照：`main:src-tauri/src/clipboard_history/mod.rs:277-303` 的
     `clipboard_clear_history` 在写入后调 `notify_completion(&app, Subject::Plugin(CLIPBOARD_PLUGIN_ID),
     CompletionAction::ClearHistory, outcome_of(&result))`；`notifications.rs:288-299` 的成功/失败词是
     `{name} cleared` / `{name} could not be cleared`（`{name}` = 插件页的双语名）；
     `:357` 的 `notify_completion` 再走 `should_notify`。
   - 本构建（改前）：`internal/shell/shell.go:1745-1760` 的 `clearClipboardHistoryAsks` 成功只
     `Win.Invalidate()`、失败只 `log.Printf`——它不调 `notifyCompletion`（`:2380`），所以面板隐藏时
     没有任何通知。可见的那一路已有：`plugincfg/schema.go:322-326` 的 Action 字段带
     `Failed: copy.ClipboardClearFailed`（`internal/i18n` 的 `Settings.ClipboardClearFailed`，
     en/zh 都有），但**只有失败词、没有成功词**。
   - 验收：清空历史走 `notifyCompletion`（可见→面板内反馈，隐藏→通知）；
     `internal/i18n` 的 `Notifications` 补两条成功/失败（en/zh 对齐旧字典的 `{name} cleared` /
     `{name} could not be cleared`，`{name}` 用剪贴板插件页的本地化名）；测试钉「隐藏→通知、
     可见→反馈」两条路径（注入 `Notify`）。
     **已落地**：`internal/shell/pluginconfig.go:141` 的 `clearClipboardHistory` 把两条清空路径
     （插件面板的 Action 与设置面 `clearClipboardHistoryAsks`）统一到一处，先 `a.notifyClipboardClear`
     再按结果落盘/日志；`internal/shell/shell.go` 新增 `notifyClipboardClear(success)`——
     `{name}` 取 `i18n.For(lang).Settings.ClipboardPlugin`（插件页的本地化名），走既有的
     `notifyCompletion`（可见→`Launcher.Feedback`，隐藏→`a.Notify`）。为让两条通道都可测，
     `notifyCompletion`/`notifyRun` 的「在屏」判定改走 `a.panelOnScreen()`，它默认读窗口两个信号、
     测试可注入 `a.onScreen`。i18n 的 `Notifications` 补 `ClipboardCleared`/`ClipboardClearFailed`
     （en `{name} cleared` / `{name} could not be cleared`，zh `{name} 已清空` / `{name} 清空失败`）。
     测试 `internal/shell/shell_test.go` 的 `TestClipboardClearReportsWhereTheUserIsLooking`
     （隐藏→通知带成功词、可见→面板反馈且不通知、失败词）。变异核查：删掉 `clearClipboardHistory`
     里的 `a.notifyClipboardClear(err == nil)`，`notifications = []` 一行红。
3. **托盘图标的身份是固定字面量，会和别的 appindicator 应用撞车**（卡 mygo 框架，未做）。
   - 原版对照：`main:src-tauri/src/tray_identity.rs:28-30` 的 `TRAY_ICON_ID = "com.v.floter.tray"`
     （bundle identifier + `.tray`），`:36-45` 的 `appindicator_id` 复刻 `tray-icon` 拼出的
     `"tray-icon tray app <id>"`；`tray-icon` 0.24.2 的 GTK 后端拿这一个字符串派生出 SNI `Id`、
     对象路径、菜单路径和 `$XDG_RUNTIME_DIR` 下的图标文件，所以两个 Tauri 应用传同一 tray id
     会发布同一身份、后注册者顶掉先注册者。
   - 本构建：`internal/shell/system.go:108-112` 的 `mygo.NewTray(mygo.TrayOptions{…})` 没有 id 字段
     （`mygo@v0.4.8` 的 `TrayOptions` 只有 Icon/IconIsTemplate/Title/ToolTip/Menu），身份由框架
     决定，无法从应用侧改成 bundle id。
   - 验收（框架给 `TrayOptions.ID` 之后）：Linux 上 `mygo.NewTray` 收到 `com.v.floter.tray`，
     SNI 对象路径为 `/org/ayatana/NotificationItem/<clean id>`；另一进程注册不同 id 时不互相顶替。
     规格先记在这里，等 mygo 提供旋钮再落地。

### 具体待办（P7-dc 重写；第 1·2 条已落地，余下第 3 条卡 mygo 框架）

> 本轮换入口：逐文件对照原版的**设置页键盘导航**（`src/settings-nav.ts` + 它在
> `src/hooks/useAppKeyboard.ts` 的调用点），把清单写成下面三条带 file:line 的具体待办，
> 并取第 1 条落地（见「最近几轮」P7-dc）。依赖检查：GitHub releases API 被限流，用
> `proxy.golang.org` 的 `@v/list` 核对，最新仍是 **v0.4.8**（与 `go.mod` 一致），无版本可升。
> 只对照 `main`（`backup/main-pre-mygo` = v0.3.14 原版树），未翻历史提交。
> P7-da 建议的三个候选里，`src/plugins/filter-axis.ts` 已由 P7 系列的 `internal/launcher/filters.go`
> 落地，`cwd_policy.rs` 已由 P7-ar 落地，`src/collapsed-focus.ts` 经本轮核对**不适用**
> （原版的触发源是 iframe / DOM 焦点外逃，mygo 是元素级焦点，没有对应物；窗口 OS 焦点由
> `shell.go:1013` 的 `Win.OnFocus` 记录，输入焦点由各表面的 `Focus*` 接缝收回）。

1. **设置侧栏的 ↑/↓ 从页面任意处走页、两端环绕、记住选择**（已落地，P7-dc）。
   - 原版对照：`main:src/hooks/useAppKeyboard.ts:288-303`——`mode === "settings"` 时，只要
     `!isArrowKeyEditableTarget(target)` 就 `preventDefault()`、`nextSettingsPage(...)`、
     把焦点移到新页的侧栏按钮并 `changeSettingsPage(next)`；`src/settings-nav.ts:24-38` 的
     `isArrowKeyEditableTarget`（INPUT/TEXTAREA/SELECT/contenteditable 让出）、`:47-54` 的
     `nextSettingsPage`（`wrapIndex` 环绕）。
   - 本构建（改前）：`internal/settingsui/settingsui.go:571` 的 `a.Sidebar.Selected = &a.Page`
     把侧栏交给 `ui.List` 的内建键盘导航（mygo `ui/list.go:882` 的 `navigate`），于是
     **只有侧栏自己持有焦点时** ↑/↓ 才走页（`shell.go:2446` 的 `Open(SurfaceSettings)` 会
     `FocusSidebar`），**不环绕**（首页 ↑ 不动）、**不落盘**（`navigate` 走 `choose`，不经过
     `selectPage` 的 `SetPage`），焦点移到页内控件后 ↑/↓ 完全无效。
   - 验收：焦点在侧栏、在窗口、在页内普通控件时，↑/↓ 都按侧栏顺序走页并在两端环绕；
     每次切换都调用 `SetPage`；侧栏仍可点击切页。
     **已落地**：`settingsui.go:652-664` 的窗口级 `c.Shortcut(0, KeyDown/KeyUp)`（记录器等待键时让出）、
     `:868-877` 的 `stepPage`/`nextPage`；`New`（`:569`）不再给侧栏游标，让出框架的列表导航。
     测试 `internal/settingsui/view_test.go` 的 `TestSidebarArrowsWalkThePagesFromAnywhere`
     （焦点在侧栏 / 无控件 / 环绕 / `SetPage` 记录）与 `TestNextPageWraps`。
     变异核查：恢复 `a.Sidebar.Selected` → 「首页 ↑ 不环绕」红；禁用窗口快捷键 → 「焦点在侧栏时 ↓ 不走页」红。
2. **单行文本框仍然让出 ↑/↓（原版规则的一半没复刻）**（已落地，P7-dd）。
   - 原版对照：`main:src/settings-nav.ts:24-38` 的 `isArrowKeyEditableTarget` 把 `INPUT`
     （含单行）与 `TEXTAREA` 一视同仁——焦点在文本框里时 ↑/↓ 不切页。
   - 本构建（改前）：mygo 的 `editor.wants`（`ui@v0.4.8/editor_navigation.go:46-47`）对单行编辑器
     `return ed.multiline || m != 0`，即单行 `ui.TextInput` **主动放开** ↑/↓，于是窗口快捷键接住它、
     切了页。受影响的行：`settingsui.go:1251`（npm 包名/版本）、`:2528`（驻留自定义标签）、
     `custom.go:101`（自定义快捷键命令）、`:2644/:2667/:2679`（本地工具名/程序/命令）、
     `config.go:165`（扩展配置文本字段）。多行 `ui.TextArea` 不受影响（`wants` 为真）。
   - 验收：焦点在单行文本字段里时 ↑/↓ 不切页、字段仍获焦；字段未聚焦时窗口快捷键照旧走页。
     **已落地**：`internal/settingsui/textinput.go` 的 `textInput(c, &value)`——`ui.TextInput`
     + 两条元素级快捷键 `e.Shortcut(0, ui.KeyUp/KeyDown)`（只认 mods==0）。18 处 `ui.TextInput`
     全部改走它。为何不是 `HandleInput`：窗口快捷键让 `rt.claimed` 为真，`HandleInput` 那道缝
     对纯 ↑/↓ 不可达；元素级快捷键在焦点链上、先于窗口。
     测试 `internal/settingsui/view_test.go` 的 `TestSingleLineFieldsKeepTheirArrows`。
     变异核查：删掉 `textinput.go` 的两行 `e.Shortcut(…)`，该用例立刻红。
     钉法：npm 包名字段获焦后按 ↓，页号不变、字段仍获焦。
3. **托盘图标的身份是固定字面量，会和别的 appindicator 应用撞车**（卡 mygo 框架，未做）。
   - 原版对照：`main:src-tauri/src/tray_identity.rs:28-30` 的 `TRAY_ICON_ID = "com.v.floter.tray"`
     （bundle identifier + `.tray`），`:36-45` 的 `appindicator_id` 复刻 `tray-icon` 拼出的
     `"tray-icon tray app <id>"`；`tray-icon` 0.24.2 的 GTK 后端拿这一个字符串派生出 SNI `Id`、
     对象路径、菜单路径和 `$XDG_RUNTIME_DIR` 下的图标文件，所以两个 Tauri 应用传同一 tray id
     会发布同一身份、后注册者顶掉先注册者。
   - 本构建：`internal/shell/system.go:108-112` 的 `mygo.NewTray(mygo.TrayOptions{…})` 没有 id 字段
     （`mygo@v0.4.8` 的 `TrayOptions` 只有 Icon/IconIsTemplate/Title/ToolTip/Menu），身份由框架
     决定，无法从应用侧改成 bundle id。
   - 验收（框架给 `TrayOptions.ID` 之后）：Linux 上 `mygo.NewTray` 收到 `com.v.floter.tray`，
     SNI 对象路径为 `/org/ayatana/NotificationItem/<clean id>`；另一进程注册不同 id 时不互相顶替。
     规格先记在这里，等 mygo 提供旋钮再落地。

### 具体待办（P7-da 重写；四条已全部落地，同为 P7-da）

> 本轮换入口：逐文件对照原版的**计算器历史**（`src-tauri/src/calculator_history.rs`）与它读写的
> 设置块（`src-tauri/src/commands/config.rs` 的 `CalculatorPluginSettings`），把清单写成下面四条
> 带 file:line 的具体待办，并一次全部落地（见「最近几轮」P7-da）。四条都落地后清单为空。
> 下一轮仍建议换一个区域做「逐文件对照原版」的审计（与
> P7-ci/P7-ck/P7-ct/P7-cw/P7-cz/P7-da 同一手法）；尚未碰过的纯模块候选：
> `src/collapsed-focus.ts`、`src/settings-nav.ts`、`src/plugins/filter-axis.ts`、
> `src-tauri/src/extension*/cwd_policy.rs` 一类。

1. **容量的裁剪按「年龄」而不是按「位置」**（已落地，P7-da）。
   - 原版对照：`main:src-tauri/src/calculator_history.rs:127-167` 的 `prune_entries`——先把非收藏项的
     下标按 `created_at` 升序排（`non_favorite_indices.sort_by_key(...)`），再
     `take(excess)` 丢掉最老的；注释明写「the cap is applied by age, not by position, so a
     hand-edited out-of-order index still prunes the truly oldest」。
   - 本构建（改前）：`internal/calculator/calculator.go:351-379` 的 `pruneLocked` 按位置走列表，
     留下前 `maxItems` 个非收藏项——顺序正常的索引上与按年龄等价，手改乱顺序时分叉。
   - 验收：一个最老的行排在**前面**的乱序索引，容量裁剪丢掉真正最老的那几行，留下的行保持原顺序。
2. **`Delete` 也要裁剪**（已落地，P7-da）。
   - 原版对照：`main:src-tauri/src/calculator_history.rs:169-186` 的 `update`——每条写路径（新增、
     取消收藏、删除、改设置）都在同一临界区里裁剪后落盘；`calculator_delete`（`:233-246`）走它。
   - 本构建（改前）：`internal/calculator/calculator.go:299-318` 的 `Delete` 直接存盘，不调
     `pruneLocked`。
   - 验收：窗口过期后没有别的写入，一次删除也要把过期的行一并带走。
3. **`copy_mode` 归一化先 trim + lowercase**（已落地，P7-da）。
   - 原版对照：`main:src-tauri/src/commands/config.rs:457-463` 的 `normalize_calculator_copy_mode`
     ——`value.trim().to_lowercase()` 之后再与 `CALCULATOR_COPY_MODES` 比对。
   - 本构建（改前）：`internal/settings/calculator.go` 与 `internal/calculator/calculator.go:495`
     的同名死副本都直接 `mode == "result"`，手改文件里的 `" Result "` 落回 `full`。
   - 验收：`NormalizeCalculatorCopyMode("  Result ")` == `"result"`；未知值仍是 `"full"`。
4. **`max_items` 的 `0` 落到下界（10），不是默认（100）**（已落地，P7-da）。
   - 原版对照：`main:src-tauri/src/commands/config.rs:1211-1214`——
     `max_items.clamp(MIN_CALCULATOR_MAX_ITEMS, MAX_CALCULATOR_MAX_ITEMS)`，`0` → `MIN`(10)；
     只有「键缺失」（`#[serde(default)]`）才给 `DEFAULT`(100)。
   - 本构建（改前）：`internal/settings/calculator.go:96-107` 与
     `internal/calculator/calculator.go:471-483` 都把 `<=0` 当「未设」回默认 100。
   - 验收：`NormalizeCalculatorMaxItems(0)` == `MinCalculatorMaxItems`、`clampMaxItems(0)` == `MinMaxItems`；
     `1000` 仍夹到 `Max*`；键缺失仍是默认 100。

### 具体待办（P7-cz 重写；三条已全部落地，同为 P7-cz）

> 本轮换入口：逐文件对照原版的**拖放规范化**（`src-tauri/src/commands/drops.rs`），把清单写成下面
> 三条带 file:line 的具体待办，并一次全部落地（见「最近几轮」P7-cz）。三条都落地后清单为空。
> 下一轮仍建议换一个区域做「逐文件对照原版」的审计（与 P7-ci/P7-ck/P7-ct/P7-cw/P7-cz 同一手法）。

1. **`normalize_dropped_path` 的 trim 只能用于空串判断，不能拿来修路径**（已落地，P7-cz）。
   - 原版对照：`main:src-tauri/src/commands/drops.rs:55-58` 用 `path.trim()` 判空，
     `:59` 的 `resolve(path, …)` 收**原始**串。
   - 本构建（改前）：`internal/drops/drops.go` 先 `path = strings.TrimSpace(path)`，`" /tmp/a "`
     被修成 `/tmp/a` 后通过存在性检查。
   - 验收：带首尾空白的路径被丢弃（除非真存在同名文件），去空白后仍是同一文件。
2. **存在性走 `symlink_metadata`（`os.Lstat`），目录走跟链的 `is_dir`（`os.Stat`）**（已落地，P7-cz）。
   - 原版对照：`main:src-tauri/src/commands/drops.rs:60-65` 的 `symlink_metadata` 与 `:82` 的
     `resolved.is_dir()`。
   - 本构建（改前）：`internal/drops/drops.go` 统一 `os.Stat`，悬空符号链接整行消失。
   - 验收：悬空链接保留且 `IsDirectory=false`；指向目录的链接 `IsDirectory=true`。
3. **`~alice` 是字面目录名，join 到 home，不是相对 cwd 的路径**（已落地，P7-cz）。
   - 原版对照：`main:src-tauri/src/commands/drops.rs:93-105` 的 `resolve`——只有 `~` 或
     `~` + 分隔符才是 home，其余 `~xxx` 走 `home.join(path)`。
   - 本构建（改前）：`internal/drops/drops.go` 只认 `~`/`~/`，`~alice` 落到 `filepath.Join(cwd, "~alice")`。
   - 验收：`home/~alice` 被保留为 `home/~alice`，`cwd/~bob` 不因 cwd 下存在而被收。

### 具体待办（P7-cw 重写；三条已全部落地：第 1 条 P7-cw、第 2 条 P7-cx、第 3 条 P7-cy）

> P7-cw 的三条至此全部落地，本节只剩历史记录。下一轮要做的只有三条卡 mygo 框架项
> （`Options.OnCopy` 复制反馈、终端键入的 `HandleInput` 空格缝、外观四个死旋钮，规格见
> 「具体待办（P7-cm 重写）」与「已知差距」），因此建议换一个区域做「逐文件对照原版」的审计
> （与 P7-ci/P7-ck/P7-ct/P7-cw 同一手法），再把清单写实成带 file:line 的具体待办。

P7-cv 收尾后「尚未做」又一次为空（P7-ct 三条全部落地，余下只有三条卡 mygo 框架项：
`Options.OnCopy` 复制反馈、终端键入的 `HandleInput` 空格缝、外观四个死旋钮，规格见
「具体待办（P7-cm 重写）」与「已知差距」）。本轮换入口：逐文件对照原版的**快捷键**
（`src/shortcuts.ts`）与设置里的快捷键地图（`src/settings/ShortcutsPage.tsx`、
`src/hooks/useShortcutCapture.ts`），把清单重写成下面三条带 file:line 的具体待办，并取第 1 条
落地（见「最近几轮」P7-cw）。依赖检查：`go list -m -versions github.com/egoist/mygo` 仍停在
**v0.4.8**（与 `go.mod` 一致），无版本可升。只对照 `main`（`backup/main-pre-mygo` = v0.3.14
原版树），未翻历史提交。

1. **`select_result` 家族在设置读取/写入时没有被收成家族头**（已落地，P7-cw）。
   - 原版对照：`main:src/shortcuts.ts:248-255` 的 `normalizeResultShortcut`（解析后要求
     `hasModifier`，把键换成 `1`，无修饰键返回 `null`）与 `:317-326` 的 `withShortcutDefaults`
     （`select_result` 的存值走它、失败回默认）；`main:src/hooks/useShortcutCapture.ts:120-128`
     的 `capture` 在写入前也走它、失败即 `setRejectedAction`。
   - 本构建（改前）：`internal/settings/shortcutmap.go:82-98` 的 `ShortcutsOf` 对
     `select_result` 只调 `shortcuts.Normalize`，`"Alt+Shift+7"` 原样落库、`"7"`（无修饰键）
     也落库；`SelectResultDigit`（`:145-154`）在无 `+` 时直接回裸数字，于是手改/旧版文件里的
     `"7"` 会让**裸数字键**触发结果行（原版明写要拒收：「normal number entry … can never
     launch something unexpectedly」），`"Alt+Shift+7"` 也带着 `7` 而不是家族头 `1`。
   - 验收：`ShortcutsOf` 把 `select_result` 收成「修饰键 + 1」，无修饰键回默认；`SetShortcut`
     同样收头并拒收无修饰键。测试钉 `Alt+Shift+7` → `Alt+Shift+1`、`7` → 默认、
     `SetShortcut("Cmd+Shift+5")` → `Cmd+Shift+1`、`SetShortcut("5")` 被拒。
     **已落地**：`internal/shortcuts/shortcuts.go` 新增 `NormalizeResultShortcut` 与
     `modifierList`（`FromKey` 共用）；`internal/settings/shortcutmap.go` 的 `ShortcutsOf` /
     `SetShortcut` 对 `select_result` 走它；测试 `internal/shortcuts/shortcuts_test.go` 的
     `TestNormalizeResultShortcut`、`internal/settings/shortcutmap_test.go` 的
     `TestSelectResultStoredBindingIsReducedToItsHead`。
2. **`shortcuts.Display` / `Badge` 的拼写、平台名与键符号和原版不一致**（已落地，P7-cx）。
   - 原版对照：`main:src/shortcuts.ts:278-296` 的 `formatShortcut`——非 macOS 用
     `parts.join("+")`（`Ctrl+Shift+C`，无空格），meta 键在 Linux 显示 `Super`、在 Windows 显示
     `Win`（`:292`），键名经 `KEY_SYMBOLS`（`:66-84`）把 `Backspace` 画成 `⌫`、四个方向键画成
     `↑↓←→`；`formatResultShortcut`（`:299-310`）的角标复用同一套。
   - 本构建：`internal/shortcuts/shortcuts.go:292-345` 的 `Display` 非 macOS 写
     `"Ctrl + "` / `"Alt + "` / `"Shift + "` / `"Super + "`（带空格，且 Windows 也是 `Super`
     而不是 `Win`），`displayKey` 只大写单字符、不查符号表，于是 macOS 上 `Cmd+Up` 显示 `⌘Up`、
     `Cmd+Backspace` 显示 `⌘Backspace`，而非 `⌘↑`、`⌘⌫`；`Badge`（`:433-460`）同病。调用点四处：
     `internal/settingsui/settingsui.go:1814`、`internal/settingsui/custom.go:40,90`、
     `internal/launcher/launcher.go:2167`。
   - 验收：非 macOS `Display("Ctrl+Shift+C")` == `"Ctrl+Shift+C"`；Windows
     `Display("Super+K")` == `"Win+K"`、Linux == `"Super+K"`；macOS `Display("Cmd+Up")` ==
     `"⌘↑"`、`Display("Cmd+Backspace")` == `"⌘⌫"`；单字符仍大写；`Badge` 与 `Display` 同规则。
3. **应用按键录制没有冲突拒绝**（已落地，P7-cy）。
   - 原版对照：`main:src/hooks/useShortcutCapture.ts:133-140` 的 `capture` 在写入前扫
     `SHORTCUT_ACTIONS`，除本动作外任一动作已用同一组合（大小写不敏感）即 `setRejectedAction`
     并放弃；`main:src/settings/ShortcutsPage.tsx:382,399-407` 用一句 `settings.shortcut.rejected`
     在页顶与行内画出被拒状态。
   - 本构建（改前）：`internal/settingsui/settingsui.go:1780-1812` 的 `shortcutControl` 录到键就直接
     `SetShortcut`，`internal/settings/shortcutmap.go` 的 `SetShortcut` 也不看其它动作，于是两个
     动作可以绑同一个组合（自定义快捷键那条路有 `shortcuts.Duplicate`，应用按键这条没有）。
   - 验收：录制一个已被别的动作占用的组合时不写入、该行/页顶显示拒收那一句；
     录制本动作自己的原值视为无变化；录制未占用的组合照常写入。
     **已落地**：`internal/settings/shortcutmap.go` 新增 `ShortcutRecording` 与
     `ShortcutRecordingOf`（`normalizeShortcut` 与 `SetShortcut` 共用）；
     `internal/settingsui/settingsui.go` 的 `recordShortcut` + `rejectedShortcut` +
     `shortcutWarning`，`savealert.go` 的 `shortcutRejectedAlert`；i18n 新增
     `ShortcutRejected`（原版 `settings.shortcut.rejected` 的逐字文案，未复用参数化的
     `ShortcutsRejected`）。测试：`internal/settings/shortcutmap_test.go` 的
     `TestShortcutRecordingRefusesAChordAnotherActionHolds`、`internal/settingsui/view_test.go` 的
     `TestShortcutRecorderRefusesAChordAnotherActionHolds`。

### 具体待办（P7-ct 重写；三条已全部落地：第 1 条 P7-ct、第 2 条 P7-cu、第 3 条 P7-cv）

P7-cs 收尾后「尚未做」又一次为空（P7-cq 三条全部落地，余下只有三条卡 mygo 框架项：
`Options.OnCopy` 复制反馈、终端键入的 `HandleInput` 空格缝、外观四个死旋钮，规格见
「具体待办（P7-cm 重写）」与「已知差距」）。本轮换入口：逐文件对照原版的**计算器**
（`src/calculator.ts`、`src/plugins/calculator/mode.ts`）与**触发词切分**
（`src/plugins/mode-entry.ts`、`src/plugins/external.ts`），把清单重写成下面三条带
file:line 的具体待办，并取第 1 条落地（见「最近几轮」P7-ct）。依赖检查：
`go list -m -versions github.com/egoist/mygo` 仍停在 **v0.4.8**（与 `go.mod` 一致），无版本可升。
只对照 `main`（`backup/main-pre-mygo` = v0.3.14 原版树），未翻历史提交。

1. **计算器结果的打印策略与原版不同**（已落地，P7-ct）。
   - 原版对照：`main:src/calculator.ts:87-96` 的 `formatCalculatorResult`——先拒收非有限值
     （返回空串）；`|v| >= 1e15` 或 `|v| < 1e-9` 走 `toExponential(CALCULATOR_PRECISION-1)`
     （12 位有效数字）再 `trimExponential` 去掉尾零；否则 `Number(v.toPrecision(12))` 后再用
     JS 的 `String(number)` 打印。常量 `CALCULATOR_PRECISION`(12)/`CALCULATOR_SCIENTIFIC_HIGH`(1e15)/
     `_LOW`(1e-9) 在 `:66-74`。JS `String(number)` 的十进制/科学阈值是十进制指数 `[-6, 20]`
     （`1e-6` → `0.000001`，`1e-7` → `1e-7`）。
   - 本构建（改前）：`internal/calc/calc.go:51` 的 `Format` 用 `strconv.FormatFloat(v, 'g', 10, 64)`
     —— 10 位有效数字（不是 12）、科学阈值是 Go `%g` 的 `[-4, 10)`（`1e-5` 打成 `1e-05`，
     原版是 `0.00001`），整数分支还绕过 12 位舍入（`999999999999999` 打成自身，原版是
     `1000000000000000`）。调用点两处：`internal/launcher/calculator.go:117`、
     `internal/launcher/search.go:729`。
   - 验收：新增纯函数 `Format` 复刻原版四条分支——非有限 → `""`；高/低带 → 12 位科学计数并
     去尾零（`1e+20`、`9.9e-10`）；带内 → 12 位有效数字后按 JS `String` 打印（`1/3` →
     `0.333333333333`、`1e-5` → `0.00001`、`1e-7` → `1e-7`、`1e15` → `1e+15`）；
     测试钉 `0/-0/整数/1e-6/1e-7/1e-9/9.9e-10/1e15/1e20/15 位整数/NaN/±Inf`。
     **已落地**：`internal/calc/calc.go` 的 `Format` + `roundSignificant`/`trimExponential`/
     `jsNumberString`/`decimalExponent`/`normalizeExponent`；`internal/calc/calc_test.go` 的
     `TestFormat` 20 行表。
2. **触发词/首词的空白切分只认空格与制表符**（已落地，P7-cu）。
   - 原版对照：`main:src/plugins/mode-entry.ts:44-52` 的 `splitTriggerWord` 用
     `/^(\S+)\s+([\s\S]*)$/`——`\s` 是 JS 的全部空白（含 `\n`/`\r`/`\f`/`\v`、NBSP、
     U+2000–U+200A、U+3000 等），所以 `clip\u3000foo`（全角空格）在原版里进入剪贴板模式；
     `main:src/plugins/external.ts:81-96` 的 `pluginCommandTriggers` 用 `source.trim().split(/\s+/)[0]`
     取首词，`\s` 同样全空白。
   - 本构建：`internal/launcher/launcher.go:1292` 的 `modeWord` 只 `strings.IndexAny(query, " \t")`，
     `internal/launcher/argv.go:12` 的 `splitArgs`（`firstWord` 在 `:54`）也只把 `' '`/`'\t'`
     当词缝，`internal/launcher/triggerhint.go:25` 的 `commandTriggers` 经 `firstWord` 取首词。
     于是全角空格（以及 NBSP 等）既不触发模式，也不切分首词。
   - 验收：新增一个纯函数（或复用 `internal/inputtext` 的 Zs 表）把 JS `\s` 的全集作为词缝，
     `modeWord`/`splitArgs`/`commandTriggers` 三处共用；`clip\u3000foo` 进入剪贴板模式且 needle 是
     `foo`，`jv\u3000-f` 进入命令模式，`\u00a0`/`\u3000`/`\u2028` 与 `' '` 等价；测试钉
     「表与 JS `\s` 互为全集」与三个入口各一例。
     **已落地**：`internal/launcher/wordgap.go` 的 `jsWhitespace`/`isWordGap`/`indexWordGap`/
     `lastWordGapEnd`/`trimTrailingWordGaps`；`splitArgs` 改为按 rune 解码，`modeWord`/
     `replaceLastWord`/`commandTokens`/`currentWord`/`appendWord` 共用这套词缝；
     `internal/launcher/wordgap_test.go` 六条测试。
     **一处与验收原文的偏差**：本构建的进入设计把触发词消耗掉、字段清空（needle 是接着打的字段
     文本），所以 `clip\u3000foo` 进模式后 needle 为空；命令模式保留 rest 为 argv。只对齐了
     词缝字符集，未改进入设计（详见「最近几轮」P7-cu）。
3. **计算器求值没有原版的函数与常量**（已落地，P7-cv）。
   - 原版对照：`main:src/calculator.ts:100-200` 的 `evaluateExpression` 交给 `expr-eval-fork` 的
     加固 parser（只关掉成员访问与赋值/函数定义/比较/逻辑/拼接/随机，**保留**内置函数），所以
     `sqrt(4)`、`sin(PI/2)`、`log(100)`、`abs(-3)`、`2^10`、`5!`、`PI`、`E` 都是合法算式；
     `hasArithmetic` 的对应物只是「拒收非算式」。
   - 本构建（改前）：`internal/calc/calc.go:162` 的 `hasArithmetic` 见到除 `e`/`E` 以外的字母就返回 false，
     `Eval`（`:27`）的递归下降 parser 只认数字、`+ - * / % ^`、括号与一元符号，
     **没有函数、常量与阶乘**。于是 `sqrt(4)`/`PI` 在启动器里根本不是计算器行。
   - 验收：`hasArithmetic` 认 `sqrt`/`sin`/`cos`/`tan`/`asin`/`acos`/`atan`/`log`/`ln`/`abs`/`ceil`/
     `floor`/`round`/`min`/`max`/`pow` 等原版内置函数名与 `PI`/`E`；parser 增 `parseCall`/`parseConstant`
     与后缀 `!`（阶乘），函数表与原版 `expr-eval-fork` 的默认集逐名对齐；测试钉每个函数一例、
     `PI`/`E` 的取值、`5!`、非法函数名仍拒收。
     **已落地**：`calcConstants`/`calcFunctions`（`internal/calc/calc.go`，一元 28 名 + 具名 9 名，
     `random` 与原版一样不收、数组/条件助手因无字面量而缺）、`hasArithmetic` 重写、`parseName`/`parseArgs`/
     `parsePostfix`/前缀 `!`；测试 `calc_test.go` 的 `TestEvalBuiltins`（44 行）/
     `TestEvalFunctionTableMatchesTheFork`/`TestEvalRejectsUnknownNames` 与 `search_test.go` 的
     `TestCalculatorRowAcceptsBuiltinFunctions`。

> 至此 P7-ct 清单三条**全部落地**，清单再次为空；卡框架项未落地前不假装接线；
> 其余可实现的偏离点继续逐区域对照 `main` 排查。

### 具体待办（P7-cq 重写；三条已全部落地：第 1 条 P7-cq、第 2 条 P7-cr、第 3 条 P7-cs）

P7-cp 之后「尚未做」只剩三条卡框架项（`Options.OnCopy` 复制反馈、终端键入的 `HandleInput` 空格缝、
外观四个死旋钮），无 Go 侧可落地项。本轮换入口：逐文件对照原版的**插件视图几何**，把清单重写成
下面三条带 file:line 的具体待办，并取第 1 条落地（见「最近几轮」P7-cq）。依赖检查：
`go list -m -versions github.com/egoist/mygo` 仍停在 **v0.4.8**（与 `go.mod` 一致），无版本可升。
只对照 `main`（`backup/main-pre-mygo` = v0.3.14 原版树），未翻历史提交。

1. **输出视图的窗口高度按输出自己的内容计价**（已落地，P7-cq）。
   - 原版对照：`main:src/App.tsx:2133-2137` 的 `launcherListRows = pluginView ? pluginViewRows(pluginView)
     : displayedResults.length` 与 `:2185-2200` 的文本形 `pluginViewRows × ROW_HEIGHT_TWO_LINE`；
     `main:src/launcher/plugin-mode.ts:647-651` 的 `pluginViewRows`（文本 → `metrics.rows`，列表 →
     `min(items.length, MAX_RESULTS)`）与 `:238-267` 的 `PLUGIN_TEXT_LINE_UNITS`(24u)/
     `PLUGIN_TEXT_MIN_UNITS`(72u)/`PLUGIN_TEXT_MAX_UNITS`(400u)/`pluginTextMetrics`。
   - 本构建（改前）：`internal/launcher/launcher.go` 的 `View` 无条件用 `a.Results()` 建 `rows`、
     `HoldRows(a.HeldRows, len(results))`——输出视图在屏时窗口仍按身后的搜索结果计价。
   - 验收：同一份输出在「一条结果」与「多条结果」的查询下高度相同；12 行高于 3 行；3 行列表 < 10 行，
     25 行列表 == 10 行（`min(items, MAX_RESULTS)`）。**已落地**：`windowRows`/`pluginTextRows`/`outputRows`
     （`internal/launcher/output.go`）+ `RowInfo.Status`/`Band`、`Geometry.Sections`（`internal/launcher/height.go`）；
     测试 `internal/launcher/outputheight_test.go`、`height_test.go` 的三条。
2. **输出列表的行要有编号快捷键（原版 R39 的 interactive 层）**（已落地，P7-cr）。
   - 原版对照：`main:src/App.tsx:1602-1611` 的 `displayedShortcutSlots`——`pluginInteractive` 为真时
     走 `resultShortcutSlots(displayedResults, …)`，给插件列表的行发 `⌘1`–`⌘9`/`⌘0`；
     `main:src/launcher/plugin-mode.ts:659-661` 的 `pluginViewInteractive`（`form === "list" && tier === "interactive"`）；
     `main:src/launcher/result-budget.ts:883-900` 的 `resultShortcutSlots`（可运行行才编号、前九 + `0`、
     超出预算才看 viewport）。
   - 本构建（改前）：`internal/launcher/launcher.go` 的 `assignNumbers(results)` 只编号 `a.Results()`，
     输出视图在屏时 `modeShortcuts` 只认方向键/Enter/Escape 就 `return`，**从不**问 `⌘N`；`outputRow`
     也不画徽标。
   - 验收：交互层的输出列表前 10 行按顺序带 `⌘1`–`⌘9`/`⌘0` 徽标，按 `⌘N` 跑第 N 行；display 层
     （状态行、不可运行行）不编号；超出预算时按 `OutputList.Visible()` 重新编号（R72）。**已落地**：
     `pressedResultDigit`/`outputShortcuts`（`launcher.go`）+ `assignOutputNumbers`/徽标（`output.go`）；
     测试 `internal/launcher/outputnumbers_test.go` 四条。
3. **直出文本的 ANSI 渲染：dim 与 inverse 的规则与原版不同**（已落地，P7-cs）。
   - 原版对照：`main:src/launcher/PluginTextView.tsx:46-63` 的 `spanStyle`——`dim` 对**任何**颜色
     一律 `opacity: 0.62`（整个 run 半透明，背景也包括）；`inverse` 先做 `fg = invert ? style.bg :
     style.fg` / `bg = invert ? style.fg : style.bg` 的互换，但**只画互换后确实有值的那一侧**
     （`if (fg)` / `if (bg)`），未命名的一侧保持表面自己的默认（默认前景 / 透明背景），不会补出颜色。
   - 本构建（改前）：`internal/launcher/output.go` 的 `outputSpans`——`style.Dim` 只在
     `style.FG.Kind == ansi.Default` 时生效，且用 `fg.Alpha(0.7)`（不是 0.62，也不作用于具名/索引/真彩
     色、不作用于背景）；`style.Inverse` **无条件**互换，且用 `c.Theme().Background` 补齐空的那一侧
     （于是「只给背景色 + inverse」会多出一块不透明底）。
   - 验收：`dim` 把整个 run（前景**与**背景）都乘 0.62，具名/索引/真彩色一视同仁；`inverse` 只给
     前景色时 → 前景为默认、背景为那个具名色，只给背景色时 → 前景为那个具名色、背景透明，两侧都
     具名时照旧互换；`dim` 与 `inverse` 叠加时先互换再一起乘 0.62。**已落地**：新增纯函数
     `resolveOutputSpan`（`internal/launcher/output.go`，`outputSpans` 改为逐 run 调用）；测试
     `internal/launcher/outputstyle_test.go` 四条（`TestDimFadesTheWholeRunNotJustTheDefaultColour`、
     `TestInverseSwapsOnlyTheSidesTheCommandNamed`、`TestDimAndInverseComposeInSpanStylesOrder`）。
     变异核查：把 dim 改回「只在默认前景」→ dim 两条红；把 inverse 的空背景补成表面底色 →
     `only_the_background_is_named` 红。

> 至此 P7-cq 清单三条**全部落地**，清单再次为空；卡框架项未落地前不假装接线；
> 其余可实现的偏离点继续逐区域对照 `main` 排查。

### 具体待办（P7-co 重写；第 1–3 条已落地，余下第 4 条卡框架但已写成可执行规格）

前一份 P7-cn 清单三条已全部落地（本轮提交），清单又空。本轮检查依赖：GitHub releases API 被限流，
改用 `go list -m -versions github.com/egoist/mygo`，仍停在 **v0.4.8**，与 `go.mod` 一致，无版本可升。
只对照 `main`（`backup/main-pre-mygo` = v0.3.14 原版树），不翻历史提交。本轮换入口：逐文件对照原版的
**行内容**与**列表分组**两个纯模块（`src/launcher/row-content.ts`、`src/launcher/list-sections.ts`）。

1. **行内容 R12/R14：只重复标题的副标题必须丢掉**（已落地）。
   - 原版对照：`main:src/launcher/row-content.ts:99-110` 的 `isTranscription`（`transcriptionKey` =
     NFKC + 小写 + 去掉空白/标点/符号）与 `:164-167` 的 `resultRowContent`（`subtitle === typeWord || transcription`
     → 丢）。原版把这条规则放在**渲染时**算，所以 `launcherRowHeightUnits` 读的是同一结果（R43）。
   - 本构建（改前）：`internal/launcher/launcher.go:1591` 直接画 `item.Detail`，`View` 的
     `rows[i].Lines`（`launcher.go:746`）与 `rowBody` 的地板（`launcher.go:1472`）也读 `item.Detail`；
     没有任何去重。Linux `.desktop` 的 `Comment=` 等于 `Name=`、命令 `Description` 复读 `Name` 时，
     副标题就是标题的第二遍。
   - 验收：`rowSubtitle` 对 `{Title:"Zebra",Detail:"zebra"}` 返回 `""`、对 `{Title:"企业微信",Detail:"WeCom"}`
     保留（不同名字）、对 `{Title:"Zebra",Detail:"A striped animal"}` 保留；渲染出来的行同时只记 1 行。
     **已落地**：`internal/launcher/rowcontent.go`（`transcriptionKey`/`isTranscription`/`rowSubtitle`），
     接线到 `View` 与 `rowBody`；测试 `rowcontent_test.go` 三条。
2. **列表分组 R39/R72：插件列表首行的 `group` 也要印标题**（已落地）。
   - 原版对照：`main:src/launcher/list-sections.ts:49-66` 的 `listSectionStarts`——index 0 的
     `previous` 是 `null`，`pluginGroupOf(item) !== null` 即返回 `"pluginGroup"`，所以首行带
     `group` 时标题照样画（`LauncherResults.tsx:837-839`）。
   - 本构建（改前）：`internal/launcher/output.go:519` 的 `outputRow` 要求 `i > 0`，首行分组标题从不出现。
   - 验收：列表 `[{group:"First"}, {no group}, {group:"Second"}]` 渲染后 `First` 与 `Second` 两个标题
     都在。**已落地**：条件改为 `row.Group != "" && (i == 0 || row.Group != a.output.Rows[i-1].Group)`；
     测试 `TestAnOutputListsOpeningGroupPrintsItsHeading`。
3. **输出视图的分组标题要与启动器列表同一个寄存器**（已落地，P7-cp）。
   - 原版对照：`main:src/launcher/LauncherResults.tsx:837-839` 用的是同一个 `.launcher-section-title`
     （`main:src/styles/launcher.css:1007-1024`：`--text-body` 字号、`font-weight: 620`、
     `line-height: 1.4`、muted、`6u/12u/4u` 内边距）。本构建启动器列表的标题（`internal/launcher/launcher.go:1442`）
     已经对齐这个寄存器，但 `internal/launcher/output.go:520-521` 的输出视图标题是 `FontSize-1`、无字重、
     `1,2,0,2` 内边距——同一个元素两个样子。
   - 接线点：`outputRow` 的标题改用 `ui.Row(...).Padding(t.Space(1.5), t.Space(3), t.Space(1), t.Space(3))`
     包住 `ui.Text(...).FontSize(t.FontSize).FontWeight(620).TextColor(t.TextMuted)`（与启动器列表同构）。
   - 验收：插件列表的分组标题与启动器列表（空查询的「最近启动」块）同字号同字重同内边距；
     `internal/launcher/outputheading_test.go` 的 `TestOutputGroupHeadingSharesTheLauncherSectionRegister`
     钉住（标签框高一致、标题不是整行宽）；现有 `TestAnOutputListsOpeningGroupPrintsItsHeading` 仍绿。
4. **卡框架项（三条，规格见下一节 P7-cm）**：终端/钉住窗口的复制反馈（`Options.OnCopy`）、键入文本的
   Unicode 空格接缝（文本钩子 / `Options.OnKey` 的 text-only 键）、终端外观四个死旋钮（`plugins/terminal`
   暴露 bold/scrollbar/select-copy/wheel-lines）。

> 卡框架项未落地前不假装接线；可实现项逐文件对照 `main` 继续排查。

### 具体待办（P7-cn 重写；三条已全部落地）

前一份 P7-cm 清单的三条（复制反馈 / 键入 Unicode-空格缝 / 外观四个死旋钮）**全部卡在 mygo
框架**，规格已写实、无法在 Go 侧落地（不假装接线），故本轮换一个入口：**逐文件对照原版的
ANSI 解析器**——`internal/ansi/ansi.go` 对 `main:src/launcher/ansi.ts`。这不是框架问题，是
Go 侧把原版的三条规则写偏了。依赖检查：`go list -m -versions github.com/egoist/mygo` 仍是
**v0.4.8**（与 `go.mod` 一致），无版本可升。只对照 `main`（`backup/main-pre-mygo` = v0.3.14
原版树），未翻历史提交。三条均已在 P7-cn 落地（见下）。

1. **非数字 SGR 参数必须被忽略，而不是读成 0（=重置）**。
   - 本构建（改前）：`internal/ansi/ansi.go` 的 `atoi` 对 `strconv.Atoi` 失败返回 0，于是
     `ESC[1;?m`（`?` 是合法的参数字节 0x3f、最终字节是 `m`）里那个坏参数被当成 0，把已经点亮的
     bold/颜色**重置**掉。
   - 原版对照：`main:src/launcher/ansi.ts` 的 `parseSgrParams` 把非数字段读成 `NaN`，
     `applySgr` 的任一分支都不匹配 `NaN`，于是**整段忽略**、保留在力的样式。
   - 验收：`ESC[1;?mX` 仍 bold；`ESC[31;?mX` 仍是 red；空参数（`ESC[1;;4m`）仍按 0 处理（原版
     `Number("")` = 0）。
2. **扩展色的任一分量坏掉时整条丢掉，而不是补 0 成一种颜色**。
   - 本构建（改前）：`readExtended` 用同一个 `atoi`，`ESC[38;5;?m` 变成 index 0（黑），
     `ESC[38;2;255;?;9m` 变成 `RGB{255,0,9}`（并把后面本来独立的参数 `9` 当成了蓝色分量吃掉）。
   - 原版对照：`main:src/launcher/ansi.ts` 的 `extendedColor` 用 `Number.isInteger` 校验每个
     分量，坏掉就返回 `null`，`38`/`48` 整条被忽略、后续参数照常处理。
   - 验收：`ESC[38;5;?mX` 的前景保持进入前的值；`ESC[38;2;255;?;9mX` 前景保持原值且 `9` 仍被
     读成 strike（而不是被吞掉）。
3. **`ESC [` 之后不是参数字节也不是最终字节时，这不是控制序列：只丢 ESC，其余按文本读**。
   - 本构建（改前）：CSI 扫描只找「最终字节」（0x40-0x7e），于是一个游离的控制字节会让扫描器
     一路吃到下一个看起来像最终字节的普通字符，**吞掉真实文本**（`a\x1b[\x07b` → `a`，`b` 没了）。
   - 原版对照：`main:src/launcher/ansi.ts` 的 `parseAnsi` 在 `ESC [` 后的字节落在参数/中间字节
     （0x20-0x3f）之外时判定「不是 CSI」，只前进一个字符（丢掉 ESC），`[` 及其后按文本继续。
   - 验收：`a\x1b[\x07b` → `a[\x07b`；`a\x1b[世b` → `a[世b`；真正的 CSI（`a\x1b[2Kb` →
     `ab`）与截断序列（`before\x1b[31` → `before`）行为不变。

> 三条均已在 **P7-cn** 落地：`sgrValue` 用 `invalidParam = math.MinInt` 标记非数字参数，
> `applySGR` 跳过它、`readExtended` 遇到它整条返回「无颜色」；CSI 扫描新增「参数/中间字节」
> 范围检查，越界即只丢 ESC。测试：`internal/ansi/ansi_test.go` 的
> `TestMalformedParametersAreIgnored`、`TestNotAControlSequenceKeepsTheText`（含真正的 CSI 与
> 截断序列的回归断言）。变异核查：把 `sgrValue` 的错误返回改回 0 → 索引/真彩两条断言变红
> （`Kind:2`、`Kind:3 R:255 B:9`）；去掉 CSI 的范围检查 → 文本两条断言变红（`Plain = "a"`）。
> 余下仍是 P7-cm 的三条卡框架项（`Options.OnCopy` 复制反馈、终端键入的 `HandleInput` 空格缝、
> 外观四个死旋钮）与一次性维护者动作，规格见下一节与「已知差距」。

### 具体待办（P7-cm 重写；R90 pin 已落地，余下三条卡框架但已写成可执行规格）

前一份 P7-cl 清单三条已全部落地（本轮提交）。依赖检查：`go list -m -versions github.com/egoist/mygo`
仍是 **v0.4.8**（与 `go.mod` 一致），GitHub releases latest 也是 v0.4.8，无版本可升。只对照 `main`
（`backup/main-pre-mygo` = v0.3.14 原版树），未翻历史提交。

**本轮已落地：R90 pin 的窗口标题/正文与原版对齐**（原版 `main:src/plugin-window/pin-entry.ts:43-80`）。
本构建 `internal/launcher/search.go` 的 `pinClip` 过去用 `entry.Label()` 当窗口标题（把多行文本折叠成
一句），并对 **files 条目**钉 `paths` 拼接、对**无 caption 的图片**钉 `[image]`——而原版 `pinTextFor`
只有**携带单块文本**的行可钉：`title` = 行的标题（行预览），`text` = `entry.text`（图片 = caption、
文本 = 正文）；files 无 text → 不可钉，无 caption 的图片同样不可钉。现在新增 `clipboardPinApplies`
（text 恒可钉、image 需有 caption、files 不可钉），`pinClip` 用 `clipboardRowTitle` 命名窗口并以
`entry.Text` 为正文，`internal/launcher/launcher.go:1682` 的 pin 控件同步加门。测试：
`internal/launcher/filters_test.go` 的 `TestOnlySingleTextClipboardRowsArePinnable`、
`TestAnEmptyTextCaptureStillPins`、`TestAFileRowShowsNoPinControl`。

**余下三条都卡在 mygo 框架**（背景见「已知差距」，此处给出可执行规格：上游 API → 本构建接线点 → 验收）：

1. **终端/钉住窗口的复制反馈**（原版 R44/R63：`main:src/terminal/copy-notice.ts`，1500ms visible →
   260ms fade；失败也说 `terminal.copyNotice.failed`）。要什么：`terminal.Options.OnCopy(text string,
   ok bool)`（或让 Edit role 的 Copy 可观察）。接线点：`internal/terminalui/` 装配终端 Options 处设
   hook → i18n 补 `Terminal.CopyNoticeCopied/Failed` → 终端状态行与 `internal/shell/pinned.go` 的钉住
   窗口共用。验收：⌘C/选中复制后状态行说一句；连续复制重启 visible 而不叠计时器；失败也报。
2. **键入文本的 Unicode 空格接缝**（原版 `main:src/hooks/useTerminalView.ts:917-925`）。要什么：一个
   文本钩子，或让 `Options.OnKey` 看到 text-only 键（`plugins/terminal/terminal.go:124` 文档写明 Linux
   上看不到）。接线点：`Options.OnKey` 里对 text-only 键做规范化（粘贴与 App 自拼命令行两条既有路不动）。
   验收：键入的全角空格在到达 shell 前被规范化；既有两条路不回退。
3. **终端外观四个死旋钮**（`terminal_bold` / `terminal_scrollbar` / `terminal_select_copy` /
   `terminal_wheel_lines`）。要什么：`plugins/terminal` 暴露这四个旋钮（滚动条开关、滚轮步长、选中即
   复制、bold 模式）。接线点：`internal/terminalui/` 的抽屉行 + settings 已存盘字段。验收：四行回到终端
   抽屉、改动实时生效（选中即复制/滚动条/滚轮步长）或下一会话生效（bold 若随 SetFont），且不再有
   「磁盘上有、UI 上没有」的字段。

> 卡框架项未落地前不假装接线（渲染了就是骗人）；其余可实现的偏离点继续逐区域对照 `main` 排查。

### 具体待办（P7-cl 重写；三条已全部落地：第 1·2·3 条 P7-cl，R90 pin 对齐 P7-cm）

前一份 P7-ci 清单三条已全部落地，清单又空。本轮检查依赖：GitHub releases API 被限流，改用
`go list -m -versions github.com/egoist/mygo`，仍停在 **v0.4.8**，与 `go.mod` 一致，无版本可升。
只对照 `main`（`backup/main-pre-mygo` = v0.3.14 原版树），不翻历史提交。本轮换一个入口：**逐文件对照
原版的剪贴板行渲染与搜索**——本构建的剪贴板行把原版的行预览/副标题整套丢了，只留一个
`Entry.Label()` 与一个绝对时间戳。这不是「框架不支持」，是 Go 侧没写完。

1. **剪贴板行的预览与副标题（类型词 + 紧凑年龄）**。
   - 本构建（改前）：`internal/launcher/search.go:312-341` 的 `clipboardItems` 用
     `entry.Label()`（`internal/clipboard/clipboard.go:139-151`）当标题，`Detail` 是绝对时间
     `2006-01-02 15:04`。于是：文本条目把**所有行**用空白折叠成一句（原版只取第一行、并截到 96）；
     文件条目只显示第一个文件名、**丢掉 `+N`**；图片条目恒为 `[image]`、**丢掉 `WxH` 与 caption**；
     副标题里**没有类型词**（文字/图片/链接/文件）、也**没有紧凑年龄**（`42m`/`5h`）。
   - 原版对照：`main:src/plugins/clipboard/mode.ts:52-68` 的 `clipboardRow`——标题 = files 的
     `basename + (extra>0 ? " +N" : "")`，否则 `clipboardPreview(entry, 96)`，为空时回落类型词；
     副标题 = `typeWord` + （有年龄时）` · ${age}`，类型词取自 `CLIPBOARD_KIND_KEYS`
     （`clipboard.typeText/typeLink/typeImage/typeFiles`，本构建 i18n 里就是
     `ClipboardTypeText/Image/Link/Files`，逐字相同）；`clipboardPreview` 见
     `main:src/clipboard-history.ts:221-240`（文本取第一行、`cap = max(1, min(maxLength, 120))`；
     图片/文件取 caption 第一行、截到 60，无 caption 则 `[image WxH]`，维度未知写 `?`）；
     `formatFilesPreview` 见 `:196-215`；`formatClipboardAge` 见 `:305-330`（`<60m` → `42m`，
     `<24h` → `5h`，再老 → 本地 `YYYY-MM-DD HH:mm`，秒从不出现）。
   - 验收：文本条目只印第一行、>96 字符截断加 `…`；多行文本不折叠成一行；文件条目印
     `a.png +2`；图片条目有 caption 时印 caption、无 caption 时印 `[image 1920x1080]`、维度未知印
     `[image ?x?]`；预览为空时标题回落类型词；副标题恒以类型词开头，且年龄按 `42m`/`5h`/
     `YYYY-MM-DD HH:mm` 三段取词。
2. **图片条目按自己的词可搜（image / img / 图片）**。
   - 本构建（改前）：`internal/clipboard/clipboard.go:238-250` 的 `matches` 把
     `Text + Paths` 拼成一个 haystack，图片条目（没有 caption 的）两边都空，于是搜 `image`、`img`、
     `图片` 一个都命中不了——而 chip 里明明有「图片」这一档。
   - 原版对照：`main:src/clipboard-history.ts:70-80` 的 `clipboardEntrySearchFields`——files 只吃
     `paths`、其余吃 `text`，且 image 条目**额外**塞进 `"image"`, `"img"`, `"图片"`；
     `filterClipboardEntries`（`:95-101`）把查询切成 token 后**每个 token 都要落在某个字段里**。
   - 验收：`Store.Search` 对无 caption 的图片条目，`image`/`img`/`图片` 三个词都能命中；有 caption
     的图片条目按 caption 命中；文本条目不因 `image` 一词被误命中（除非它真的含这个词）；files 条目按
     路径命中。
3. **图片缩略图 memo 有界（R97）**。
   - 本构建（改前）：`internal/launcher/thumb.go:19-40` 的 `clipThumb` 只往
     `a.thumbs[path]` 里塞、**从不淘汰**：一个长会话每张看过的图都留一份解码后的 `*ui.Bitmap`
     （1920×1080 就是 8 MB），而历史本身是有上限的。
   - 原版对照：`main:src/clipboard-history.ts:441-465` 的 `pruneThumbnailCache`（R97）——memo 超过
     历史的 `MAX_CLIPBOARD_MAX_ITEMS`（本构建的 `clipboard.MaxMaxItems` = 500）时，只保留**当前渲染
     仍引用**的 key，其余丢掉（下次需要再取）；不超过上限时返回同一个对象、不做拷贝。
   - 验收：memo 超过 500 时收敛到当前结果集引用的那些路径；不超过上限时不动；被丢掉的条目再次出现
     时能重新解码。

> 三条已全部落地（P7-cl 提交 4 条改动 + 本轮 P7-cm 的 R90 pin 对齐）。余下三条卡在 mygo 框架
> （`Options.OnCopy`、终端键入的 `HandleInput` 缝、外观四个死旋钮），其可执行规格见顶部 P7-cm 清单；
> 一次性维护者动作（`mygo keygen`、`mygo.json` 的 `updates`/签名、CI 私钥、发布到 GitHub Release）
> 见「已知差距」。

### 具体待办（P7-ci 重写；三条已全部落地：第 1 条 P7-ci、第 2 条 P7-cj、第 3 条 P7-ck）

前一份 P7-cg 清单两项已全部落地，清单又空。本轮检查 GitHub latest release：仍为 **v0.4.8**，与 go.mod
一致，无需升级。只对照 `main`，不翻历史提交。这轮换一个入口：**逐文件对照原版的平台扫描器**，发现本
构建的 Linux `.desktop` 读取器只是一个占位——`internal/apps/apps.go:276` 的 `ParseDesktopFile` 只读
`Name`/`Exec`/`Icon`/`Hidden`/`NoDisplay`，而原版 `main:src-tauri/src/commands/apps/linux.rs` 读的是一份
完整的 Desktop Entry。这不是「框架不支持」，是 Go 侧没写完。

1. **Linux `.desktop` 的本地化名与 `Comment=`（P7-ci 已落地）**。
   - 本构建（改前）：`internal/apps/apps.go:276-350` 的 `ParseDesktopFile` 用 `strings.Cut(line, "=")` 后
     直接 `switch key`，于是 `Name[zh_CN]=…` 的 key 是 `"Name[zh_CN]"`，落到 default 被丢；`Comment=`
     根本没有 case。`internal/launcher/search.go:515` 的 `appItem` 也不设 `Detail`，所以 Linux 应用行
     永远没有副标题、也永远没有本地化标题。
   - 原版对照：`main:src-tauri/src/commands/apps/linux.rs:312-372` 用
     `locales = ["zh_CN","zh_SG","zh","zh_TW","zh_HK"]` 给 `Name[locale]`/`Comment[locale]` 排名取最优
     （`best_name_locale`/`best_comment_locale`，无 locale 的 `Comment` 是先到先得、可被本地化覆盖），
     `:447` 过滤空 Comment；`main:src/hooks/useLauncherCatalog.ts:1162-1166` 应用行
     `subtitle = (localizedName && name) || comment || t(appSubtitleKey(path))`；`main:src/launcher/row-content.ts:125-171`
     的 `isAlternateAppName` 又把「本地化名 ↔ 拉丁名」那条副标题丢掉，所以**实际只有 comment 会被画出来**，
     且只在没有本地化名时。
   - 验收：`Name[zh_CN]` 胜过 `Name[zh_TW]`、不在阶梯里的 `Name[fr]` 被忽略、本地化名与 `Name` 相同则不计；
     `Comment[zh_CN]` 胜过 `Comment[zh_TW]` 与无 locale 的 `Comment`；空 Comment 丢弃；`\s`/`\n` 等
     Desktop Entry 转义按规范解开；应用行只在 `Localized == ""` 时把 Comment 画成副标题。
2. **Linux `.desktop` 的搜索别名（GenericName / Keywords / StartupWMClass / Exec 程序名 / desktop-id 片段）（P7-cj 已落地）**。
   - 本构建：`internal/apps/apps.go:185-202` 的 `ScanLinux` 从不给 `App.Aliases` 赋值，`ParseDesktopFile`
     也不读这几个键。于是一个中文名 Linux 应用只有拼音首字母能命中；`nautilus`、`org.gnome.Nautilus`、
     `StartupWMClass` 一概搜不到——而 macOS 侧 `internal/apps/darwin.go:65` 已经这样做了。
   - 原版对照：`main:src-tauri/src/commands/apps/linux.rs:423-440` 把 `generic_name`、`startup_wm_class`、
     `executable_name(exec)`、`executable_name(try_exec)`、`Keywords` 按 `;` 切分、`identifier_aliases(file_stem)`
     全部收进 `candidates`，再 `build_aliases(name, localized_name, candidates)`
     （`main:src-tauri/src/commands/apps/mod.rs:326`）。
   - 验收：`ScanLinux` 生成的 `App.Aliases` 含上述六个来源，且 `computeInitials` 仍只吃 name+localized；
     一个 `Name=文件`/`Exec=nautilus` 的条目能被 `nautilus`、`org.gnome.Nautilus`、`Keywords` 命中。
3. **Linux `.desktop` 的可用性判定与 `DBusActivatable` 启动路径（P7-ck 已落地）**。
   - 本构建（改前）：`ParseDesktopFile` 不看 `Type=`（`Link`/`Directory` 条目也会被当应用列出）、不探测 `TryExec=`
     （会列出没装的条目），`App.Open`（`internal/apps/apps.go:58`）对所有条目一律 `exec.Command(Exec…)`。
   - 原版对照：`main:src-tauri/src/commands/apps/linux.rs:395-408`（`Type != "Application"` 丢弃、
     `TryExec` 不在 PATH 丢弃）、`:86-140` 的 `open`（非 D-Bus 条目直接 spawn argv，`DBusActivatable=true`
     的条目走 `gio launch <desktop-id>`，交给 session bus 做单实例激活）。
   - 验收：`Type` 非 Application 丢弃；`TryExec` 不可执行（`exec.LookPath`）丢弃；`DBusActivatable=true`
     的条目仍列出但 `Open` 走 `gio launch`。

> **P7-ck 更新（本轮）**：P7-ci 的三条待办至此全部落地。本轮检查 GitHub latest release：仍为
> **v0.4.8**，与 `go.mod` 一致，无需升级（release notes 无自然用点）。实现要点：`ParseDesktopFile`
> 新增 `Type`（仅 `Application`，缺省保留）与 `DBusActivatable` 两个无 locale 键，`App` 新增
> `DBusActivatable` 与未导出的 `tryExec`；`Type` 的丢弃留在纯读取器里，`TryExec` 的可用性探测放进
> `ScanLinux`（需要文件系统/PATH，原版也在 `into_application` 而非 `parse`）——新增 `executableAvailable`
> 与 `isExecutableFile`（绝对/相对路径看可执行位，裸名走 `PATH`）；`App.Open` 抽出 `launchCommand`
> （D-Bus 条目 → `gio launch <path>`，其余 → 自己的 argv），`launchDetached` 统一「独立进程」的
> spawn，且 `gio` 起不来时回退到 argv。测试：`TestDesktopEntryTypeGate`、`TestExecutableAvailable`、
> `TestScanLinuxDropsUnavailableTryExec`、`TestDesktopLaunchCommand`。变异核查：摘掉 `Type` 门、
> 让 `executableAvailable` 恒真、把 D-Bus 改回 argv，三组断言各自变红。依赖检查：GitHub latest
> release 与 `proxy.golang.org/.../@v/list` 均确认 **v0.4.8** 仍是最新。
>
> **下一轮入口**：P7-ci 清单已全部落地，清单再次为空。经快速审计，`internal/apps` 与
> `internal/extensions` 里此前记录的「未接线」项（`plugin:calculator` 模式、`terminal.color` 环境、
> 剪贴板 kind 过滤、工具多来源清点、应用扫描签名缓存）均已落地。剩余未做项都卡框架（`Options.OnCopy`
> 复制反馈、终端键入的 `HandleInput` Unicode-空格缝、外观四个死旋钮），见「已知差距」；下一轮建议换一个
> 区域做「逐文件对照原版」的审计（与 P7-ci/P7-ck 同一手法），再把它写成带 file:line 的具体待办。

### 具体待办（P7-cg 重写；两项已于 P7-ch 全部落地，已由 P7-ci 取代）

> 本轮 P7-ci 已把清单重写（见上一节）；下面保留 P7-cg 的原文，只作记录。

前一份 P7-cd 清单已全部完成。本轮检查 GitHub latest release：仍为 **v0.4.8**，与 go.mod 一致，
无需升级。只对照 `main`，不翻历史提交；以下两项是 Go 捕获实现自己的缺陷，不伪称原版已有内存上限。

1. **修正输出截断的 UTF-8 边界（P7-ch 已落地）**。
   - 本构建：`internal/extensions/run.go:165-177` 的 `clip` 从末尾删 continuation byte，却保留多字节
     字符的首字节；字符刚好完整落在上限内时也会被破坏。`internal/extensions/probes.go:180` 共用它。
   - 原版对照：`main:src-tauri/src/extensions/run.rs:750-752` 用 `String::from_utf8_lossy` 返回文本，
     不截断（`truncated: false`）；Go 自己增加的截断不能把原本合法的 UTF-8 变成坏串。
   - 验收：对 2/3/4 字节字符测试每个切分位置；完整字符恰好落在上限内时保留、不完整字符整个丢弃；
     ASCII、空串、未超限保持不变；合法输入截断后 `utf8.ValidString` 必须为真。
2. **在捕获期间限制内存，而不是进程退出后才裁剪（P7-ch 已落地）**。
   - 本构建：`internal/extensions/run.go:120-135` 把两个流全部写进 `bytes.Buffer`，`clip` 只在
     `cmd.Run()` 返回后调用；持续输出的程序可在 300 秒超时前耗尽宿主内存。
   - 原版对照：`main:src-tauri/src/extensions/run.rs:724-734,750-752` 管道捕获后完整返回输出；
     本构建 `internal/extensions/run.go:12-24` 额外承诺了有界捕获，需兑现而非照搬原版的无限缓存。
   - 验收：每个流使用限长 writer，保留最前 1 MiB、持续接收并丢弃多余字节，`Write` 返回完整输入长度
     防止 short-write/SIGPIPE；只有真正超限才置 `Truncated`。单测验证分块写入、超限后重复写、两流独立；
     通过真实子进程验证大输出仍正常退出，stdout/stderr 都正确截断，UTF-8 边界沿用第 1 条。


> **P7-cd 更新**：P7-cc 之后「尚未做」清单再次为空。这轮不去翻旧提交，而是**顺着原版一条明写、
> 本构建只做了一半的契约往回找**（与 P7-cc 同一手法）：原版 `src/terminal/command-session.ts` 的
> R62 规定「命令启动的会话结束就离开终端页」，本构建只做了 R9-2 的「持有终帧」一半，命令会话也留着
> （而 plan P2-p 与 `tool-install.ts:190` 都引用过这条判定）。同一族又找到两个口子——集成卡里
> **完整缺失**的审批记录区块（`ApprovedAt`/`ApprovedManifestDigest`/`ApprovedPermissions`
> 存了没人读），与 P7-cc 自己记下的残留（删除确认走原生弹窗而非原版的页内 bar）。三条都写成本
> 清单（下方「具体待办（P7-cd 重写）」）。
> **P7-cd 落地结果**：第 1 条（R62）已落地。第 2（审批记录）、第 3（删除确认页内 bar）留待下一轮。
> **P7-ce 落地了第 2 条，P7-cf 落地了第 3 条——本清单三条已全部落地。**
> 其余仍是卡框架的三条（`Options.OnCopy`、终端键入的 `HandleInput` 缝、外观四个死旋钮）与一次性
> 维护者动作（`mygo keygen`、`mygo.json` 的 `updates`/签名、CI 私钥、发布到 GitHub Release），
> 另见「已知差距」。

### 具体待办（P7-cd 重写；3 条已全部落地：第 1 条 P7-cd、第 2 条 P7-ce、第 3 条 P7-cf）

1. **命令启动的会话结束时把终端页一起带走（原版 R62）（本轮 P7-cd 已落地）** ——
   原版 `src/terminal/command-session.ts` 的 `shouldExitWithCommandSession(commandStarted, mode === "terminal")`：
   命令启动的会话（`initialCommand` 或结构化 execution plan）结束就离开终端页、回到启动器；**裸** shell
   （系统 Terminal 行、⌘ 持有行、空态回车、安装行）永不带走页面。`useTerminalView.ts:476` 在
   `term://exit` 里调它，再走 `returnToInputMode()`（= 关会话 + 回启动器）。本构建只做了 R9-2 的前半：
   `Actions.Exit` 一律 `Resident(code)` 持有终帧，命令会话也留着——而 `docs/mygo-rewrite-plan.md:746`
   （P2-p）与 `src/extensions/tool-install.ts:190` 都写着「安装行故意开裸会话，好让 R62 判定为 false」，
   说明这条契约被引用过却从未实现。顺带一个潜在 bug：`Exit` 回调不携带会话身份，`Resident` 标的
   是 `Current()`——后台会话退出会把当前会话标成 resident。
   - 原版：`src/terminal/command-session.ts`、`src/hooks/useTerminalView.ts:363-367`/`:476-479`、
     `src/App.tsx:779`（returnToInputMode）、`src/extensions/tool-install.ts:190-200`。
   - 本构建（改前）：`internal/terminalui/terminalui.go:44`（`Exit func(int)`）、`:767`（`Resident` 标
     Current）、`internal/shell/shell.go:834`（无条件 `Resident`）。
   - 验收（已满足）：`Session.CommandStarted`；`Actions.Exit(session, code)` 携带会话（`IndexOf` 在
     主线程解回索引）；`ShouldExitWithCommandSession` 纯判定；
     `CommandSessionEnded(index, terminalOnScreen)` 关会话（不写 snapshot）并报告是否回启动器；
     `RunCommand`/`RunCommandLine` 标 command-started，`RunShellWithCommand`（安装行）不标；shell 用
     `a.Surf == SurfaceTerminal` 当「页面在屏上」。测试：`internal/terminalui/commandsession_test.go`
     4 条 + `internal/shell/commandsession_test.go` 2 条。变异核查：判定改成恒假，立刻红。

2. **集成卡里的「审批记录」区块（原版 R7-8 / R7-8b），字段存了没人读** ——
   原版在每个集成的详情里画一条审批记录：`approvedAt`（本地化时间）、`approvedManifestDigest`
   （`shortDigest` 截到 12 hex、包在 `<code>` 里，未知则一句 `approvalRecordDigestUnknown`）、**并排**的
   在盘 `currentManifestDigest`（R7-8b：让「我批准了什么」与「现在磁盘上是什么」是两行可读事实，而不是
   一个真假值）、已批准的权限列表（按 enforced / disclosure 分组，空时 `approvalRecordPermissionsNone`）、
   以及 `approvalIsStale(a, b)` 为真时的 `approvalRecordChanged` 提示；没有任何记录时一句
   `approvalRecordNone`。本构建把 `ApprovedPermissions`/`ApprovedAt`/`ApprovedManifestDigest` 三个字段
   **存进仓库、改键时原样保留，却只有写入端**（`grep -rn "ApprovedAt\|ApprovedManifestDigest" internal`
   只有 `repository.go:53-55` 的声明与 `install.go` 的赋值），卡片里也没有这条区块。
   - 原版：`src/ExtensionsPanel.tsx:2707-2763`（区块）、`src/extensions/approval-record.ts`
     （`shortDigest` 12 hex / `approvalIsStale` 的两侧未知规则）、`src/extensions/permission-tiers.ts`、
     `src/i18n.ts` 的 8 个 `settings.extensions.approvalRecord*`。
   - 本构建（改前）：`internal/extensions/repository.go:53-55`、`internal/extensions/permissions.go:89`
     （只用来判「是否还需要审批」）；`internal/settingsui.Integration` 无对应字段；i18n 无这 8 个键。
   - 验收：`Integration` 带 `ApprovedAt`/`ApprovedDigest`/`CurrentDigest`/`ApprovedPermissions`；纯函数
     `ShortDigest`（>12 hex 截断、`sha256-` 前缀短则原样、空则空）与 `ApprovalIsStale`（两侧都有且不同
     才 true）；卡片在权限区下画区块（无记录 → 一句空态）；en/zh 8 个键；测试钉短摘要边界、stale 的两侧
     未知、权限分级、以及卡片渲染（有/无记录两态）。

3. **删除确认应走原版的页内 bar，不是原生弹窗（P7-cc 记下的残留）** ——
   原版的删除/组件化卸载确认是页内一条**警示 bar**（`role="alert"`）：焦点落在**取消**按钮、Escape 在
   文档层捕获（`stopImmediatePropagation`）、破坏性确认按钮按 `busy` 门控、系统运行时用 `Unplug` 图标、
   包运行时用 `Trash` 图标，生成的本地连接再追加一句 `deleteCustomWarning`。本构建的两条删除路径都用
   原生 `mygo.Dialog.Message`（`internal/shell/shell.go:629-647` 与 `:668` 前后），与同页的**停用**确认
   （页内卡片 + 两个 `ui.Button`，`internal/settingsui/settingsui.go:3024`）形状不一致；P7-cc 的残留段
   已如实记下这一差异。
   - 原版：`src/extensions/RemovalConfirmation.tsx`（+ `ComponentizedUninstallDialog.tsx` 同一契约）、
     `src/styles/extensions.css` 的 `.extension-discard-bar`、`src/ExtensionsPanel.tsx` 两处接线、
     `src/i18n.ts` 的 `deleteCustomWarning` / `cancel` / `uninstall` 等。
   - 本构建：`internal/shell/shell.go:629`/`:668`（`mygo.Dialog.Message`）、
     `internal/settingsui/settingsui.go:3012-3040`（停用确认的页内形状可以照抄）。
   - 验收（已满足，P7-cf）：`settingsui` 里删除走页内确认行（标题 + 一句后果 + Cancel 在前、危险按钮
     在后；生成的本地连接带 `deleteCustomWarning`）；Escape 先关它、不顺手离开页面；原生弹窗从三条
     拆除路径（组件化卸载 / 普通卸载 / 删除孤儿）删除；`TestIntegrationsPage` 钉先问/Cancel/Escape/
     确认四步，`TestGeneratedIntegrationRemovalBar` 钉 Generated 两态。残留：原版的焦点落取消、
     `busy` 门控与运行态图标是 Web 契约，mygo 无对应原语，未搬。

### 具体待办（P7-cc 重写；3 条已全部落地：第 1·3 条 P7-cc，第 2 条早已在 P4-ac 落地）

1. **停用/卸载不杀死进行中的后台运行（原版 R98；`run_killed` 已定义却从不发出）（P7-cc 已落地）** ——
   原版 R98 的结论是：一个后台运行无论背后发生什么都要活到 300s 超时——停用它、甚至卸载它，子进程照跑，
   启动器/独立窗口的 running 状态也照留。原版建 `RunAbortRegistry`（一个 id → `Vec` 个 abort handle，因为
   外部插件路由没有单槽守卫、启动器与独立窗口的运行可以真的重叠），`set_enabled` 的停用分支与两条卸载
   路径在**碰任何文件之前**调 `kill_extension_runs`，每个 abort drop 掉 future，复用既有的 `kill_on_drop`
   + `ChildCleanup` 进程组拆除，并发出新的 `run_killed` 键（「因为 {id} 被停用或卸载，运行已停止」）。
   本构建的 `internal/extensions/runfailure.go:42` 定义了 `RunErrorKilled`、`internal/launcher/runfailure.go:38`
   会把它渲染成人话、en/zh 两语言都在（`internal/i18n/copy.go:538`/`:1272`），但**没有任何代码发出它**：
   `internal/shell/shell.go:729-732` 的 `SetIntegrationEnabled` 只写标记，`:634`/`:664`/`:711` 的卸载/删除孤儿
   也不取消任何运行；`internal/extensions/run.go:84` 的 `runCaptured` 只把父 ctx 的取消投影成裸 `ctx.Err()`。
   - 原版：`src-tauri/src/extensions/run.rs`（`RunAbortRegistry`、`kill_extension_runs`）、
     `src-tauri/src/commands/extensions.rs`（disable 分支 + 两条卸载路径先杀后动文件）、
     `src-tauri/src/extensions/run_error.rs` 的 `run_killed`、`src/extensions/run-errors.ts`、`src/i18n.ts` 的
     `run_killed`。
   - 本构建（改前）：`internal/extensions/run.go:84`、`internal/shell/shell.go:729`、`:634`/`:664`/`:711`。
   - 验收（已满足）：新增 `extensions.RunRegistry`（按 id 记录每个在飞运行的 cancel，`Kill(id)` 返回杀掉的个数，
     `Add` 返回一个幂等的注销函数）；`extensions.RunKilledFailure(id)` 建 `run_killed:{extension_id}`；
     `runCaptured` 在父 ctx 被取消（非超时）且条目有 IntegrationID 时回 `run_killed`；shell 的 `App` 持有
     `Runs`，`runCommandCaptured`/`loadOutputPage` 用可取消 ctx 注册，停用/组件化卸载/普通卸载/删除孤儿
     先 `Runs.Kill(id)` 再动文件。测试钉住 registry 语义、真跑一次取消、以及停用把运行变成 `run_killed`。
     **P7-cc 落地位置**：新增 `internal/extensions/runregistry.go`（`RunRegistry`：`Add`/`Kill`/`InFlight`，
     nil 接收者 no-op）、`internal/extensions/runfailure.go:185` 的 `RunKilledFailure`、
     `internal/extensions/run.go:147` 的取消分支；`internal/shell/shell.go:221` 的 `App.Runs`，
     `:2287`/`:2304` 在 `runCommandCaptured`/`loadOutputPage` 注册可取消 ctx，
     `:641`/`:671`/`:732`/`:744` 在四条拆除路径先杀后动文件。测试：`runregistry_test.go` 4 条 +
     `internal/shell/shell_test.go` 的 `TestDisablingStopsARunInFlight`（真跑 `sleep 5`，停用后拿到
     `run_killed:{"extension_id":"test.ext"}` 且注册表清空）。
2. **停用集成时不先问（原版 R100）（订正：本构建早已落地于 P4-ac `a03b40a`，无需再动）** ——
   **P7-cc 订正（2026-10-14）**：P7-cc 那轮把这行记成「待做」，是把原版审计直接当成了本构建的差距。
   本轮核查：`internal/settingsui/settingsui.go:3012` 的 `requestDisable`（武装）与 `:3024` 的
   `disableConfirmation`（一句标题 + `DisableDescription` 同时说「运行中的命令立即停止」与
   「只是标记，文件与数据都保留、随时可再启用」+ Cancel/Disable 两个按钮），接线在 `:1459`（开关
   关向）与 `:1496`（标签点击），`internal/i18n/copy.go:617-620`（en）/`:1348-1351`（zh）；
   不对称由 `internal/settingsui/view_test.go:2390` 的 `TestDisableAsksFirst` 钉住，`:961` 另有一条。
   下面这段「改前」分析保留，用于记住「原版有 =/= 本构建缺」这个误判。
   残留：原版要求确认条沿用删除条的无障碍契约（焦点在取消、Escape 捕获、busy 门控、中性确认按钮）；
   本构建的删除确认本身走的是原生 `mygo.Dialog.Message`（`internal/shell/shell.go:629`），停用确认是
   页内卡片 + 两个 `ui.Button`（`:3024`）。核心事实（先问、不对称、两句事实都在）已满足，与删除条
   共享同一套无障碍契约这件事未核对，作为可能的后续对照点留在此处。
   —— 原版把「停用」定义为**标记**、「删除」定义为**真删**，两者在生效前
   都要交代后果；审计发现删除侧已合规，缺口只在前面：扩展开关直接发 `extensions_disable`、没有任何提示。
   原版让开关的两个方向由纯投影 `toggleIntent` 拥有——**停用**先武装一个确认（文案同时说两件事：进行中的
   命令立即停止〔R98〕、这只是标记，文件与数据都保留、随时可再启用），**启用**保持即时（一条负向守卫钉住
   这个不对称）；确认条沿用删除条的无障碍契约（焦点在取消、Escape 捕获、busy 门控、中性确认按钮因为可逆）。
   - 原版：`src/ExtensionsPanel.tsx` 的 `toggleIntent`/`DisableConfirmation`、`tests/r100-disable-delete.test.ts`、
     `src/i18n.ts` 的两句新文案。
   - 本构建（改前）：`internal/settingsui/settingsui.go:1501-1502`（开关直接调动作）、
     `internal/shell/shell.go:729-732`（动作直接写标记）。
   - 验收：`SetIntegrationEnabled` 在 `enabled == false` 时先弹一条本地化确认（两句事实都在，en/zh），
     `enabled == true` 不弹；一条测试钉住不对称，一条钉住两句事实确实被渲染而不只是被声明。
3. **Windows 上 `.cmd`/`.bat` 程序没有 `cmd /c` 包装（原版 R74 的 latent-failure 守卫）（P7-cc 已落地）** —— 原版审查发现
   `CreateProcess` 不能直接执行 `.cmd`/`.bat`，而探测层会匹配这些 shim，于是给 Windows 的应用启动分支加了一个
   **cfg-free 的纯函数** `windows_cmd_shim_wrap()`（后缀是 `.cmd`/`.bat` 就包 `cmd /c`），真值表测试在 Linux 上跑，
   另用一条源码守卫钉住接线（`open_url` 的 `cmd /c start` 不受影响）。本构建里 `internal/extensions/paramrun.go:88`
   的 `IsCmdScriptName`/`ProgramIsCmdScript` 就是这个纯后缀判定的现成件、`internal/extensions/paramrun.go:70-77`
   的注释甚至写着「A `.cmd`/`.bat` program is launched through `cmd.exe /D /S /C`」，但**没有一处真的包**：
   `internal/extensions/run.go:98`、`provider.go:162`、`complete.go:99`、`probes.go:164` 都直接
   `exec.CommandContext(..., entry.Program, ...)`，参数校验用 `ProgramIsCmdScript` 而执行不用。
   - 原版：`src-tauri/src/extensions/run.rs` 的 `windows_cmd_shim_wrap()` + 真值表测试 +
     `tests/r74-catalog-refresh-and-cmd-shim.test.ts` 的源码守卫。
   - 本构建（改前）：`internal/extensions/paramrun.go:88-108`、`internal/extensions/run.go:98`。
   - 验收（已满足）：新增 `internal/extensions/cmdshim.go` 的纯函数
     `cmdShimWrap(shim bool, program string, args []string) (string, []string)`——`shim` 为 false 时
     逐字节原样返回（连 slice 都是同一个），true 时回 `cmd.exe` + `/D /S /C program args…`；
     `internal/extensions/run.go:103` 用既有的 `ProgramIsCmdScript`（OS 门）当 `shim` 接进 spawn，
     `Program`/`argv` 就地改名，`CapturedRun.Command` 现在记录真跑的那条 argv（与它自己的注释
     「the argv that ran」一致）。测试（`cmdshim_test.go`）：`TestCmdShimWrapTruthTable` 三个子测试
     钉真假两支与 nil；`TestCmdScriptWrapFollowsThePlatformGate` 钉平台门（两个平台都跑）；
     `TestRunCapturedDoesNotWrapCommandScriptsOffWindows` 在 Linux 上真跑一个名为 `runner.cmd` 的
     POSIX 脚本并断言 argv 未被改写；`TestRunCapturedWrapsWindowsCommandScripts` 在 Windows 上真写
     一个 `.cmd` 跑通（CI 的 windows-latest `go test ./...` 会执行它，比原版的源码守卫更硬）。
     变异核查：把门换成 `IsCmdScriptName`（抽掉平台判定），Linux 那条立即变红；恢复后全绿。
     残留（诚实记录，不属于本条）：原版的删除确认是页内 bar（焦点在取消、Escape 捕获、busy 门控），
     本构建的删除确认走 `mygo.Dialog.Message` 原生弹窗（`internal/shell/shell.go:629`），两者的无障碍形状
     不同；本条只管停用侧，那一差异未列入本轮。

### 具体待办（P7-ca 重写；3 条已全部落地：第 1·2 条 P7-ca、第 3 条 P7-cb）

1. **界面大小住在 Window behaviour 卡，应回 Appearance 卡（原版 R7-13c）（本轮已落地）** —— 原版把
   `uiScale` 的 `SettingsRow` 放在 Appearance 卡里、theme/language **之后**，注释写明理由是「它和 theme/
   language 是同一类选择——界面长什么样」（`GeneralPage.tsx:369-390`，R7-13c）。本构建把它画在
   Window behaviour 卡首行：一个缩放整个界面的外观选项，和一个「窗口失焦隐藏」的开关同处一区。
   - 原版：`src/settings/GeneralPage.tsx:370-390`（Appearance 卡内 `settings.uiScale`）+ `:333-400`（卡
     的其余两行）。
   - 本构建（改前）：`internal/settingsui/settingsui.go:1871-1873`（`copy.Scale` 在 `GroupWindow` 卡首行）。
   - 验收（已满足）：`a.pick(c, copy.Scale, copy.ScaleHint, copy.Scales, s.UIScale, …)` 移进 Appearance 卡
     （Language 之后），Window behaviour 卡去掉它；测试 `TestInterfaceSizeSitsWithAppearance` 钉住 Scale 行
     在 Appearance 标题之下、Window behaviour 标题之上，且在 Theme/Language 之后。变异核查：把 Scale 移回
     Window 卡后位置断言立刻变红。

2. **Window behaviour 的行序与参考相反（原版 `GeneralPage.tsx:410/431/450/464`）（本轮已落地）** —— 原版四
   行顺序是 launch-at-startup → hide-on-blur → **page-residency** → **recent-items**（`:410` switch、`:431`
   blur switch、`:450` residency control、`:464` recent switch）；本构建把 `showRecentInLauncher` 画在
   `surfaceResidency` **之前**，两个相邻但有先后关系的开关顺序颠倒。（`Scale` 移出后剩下的四行仍是这个
   错序。）
   - 原版：`src/settings/GeneralPage.tsx:410`/`:431`/`:450`/`:464`。
   - 本构建（改前）：`internal/settingsui/settingsui.go:1877-1912`（recent switch 在 residency 之前）。
   - 验收（已满足）：`surfaceResidency`（连同自定义秒数行）排到 `showRecentInLauncher` 之前；`TestInterfaceSizeSitsWithAppearance`
     同时钉住这一行序。变异核查：把 recent switch 挪到 hide-on-blur 之后，`residency is not above recent` 立刻变红。

3. **page-residency 被拆成两行，原版是**一行**带内联自定义字段（原版 `SurfaceResidencyControl`）（本轮 P7-cb 已落地）** —— 原版是
   单行 `<select>`：off / presets / never / **custom** 四类选项，仅当选中 custom 时在同一行里内联出数字框 +
   单位 + Apply（`.residency-control__custom`，`GeneralPage.tsx:157-250`）；存储值本身是自定义数字时，custom
   那一条的标签用 `surfaceResidencyCustomValue({seconds})` 回显；内联框初值由 `residencyCustomSeedSeconds`
   取（自定义值＝自己，其余＝默认）；提交落在 preset 上就收起内联框。本构建把选择器与自定义输入拆成**两行**：
   选择器把当前自定义值伪装成第一条选项（`residencyOptions`），下面再**无条件**画一行输入框 + Apply。
   - 原版：`src/settings/GeneralPage.tsx:157-250`（`SurfaceResidencyControl`）+ `src/surface-residency.ts`
     （`residencySelectValue`/`residencyFromSelect`/`residencyCustomSeedSeconds`）。
   - 本构建（改前）：`internal/settingsui/settingsui.go` 的选择器一行 + 自定义行（旧 `:1889-1917`）+
     `residencyOptions` 以存储值造伪选项（旧 `:2222-2243`）。
   - 验收（已满足）：`general` 只调 `a.residencyRow(c, copy)`，收成一行（select 带 custom 项；仅 custom 时画内联
     `TextInput` + 单位 + Set，Enter 也提交）；`residencyOptions`/`parseResidency` 换成纯映射
     `residencySelectValue`/`residencyFromSelect`/`residencyCustomSeedSeconds`/`residencyCustomLabel`，提交走
     `parseResidencyDraft`（空 = off、非数字 = 默认、数字夹到一天）。i18n 补 `SurfaceResidencyCustom` 与
     `SurfaceResidencyCustomValue`（en `Custom…`/`Custom ({seconds} s)`，zh `自定义…`/`自定义（{seconds} 秒）`）。
     测试钉住非 custom 值下页面上没有自定义输入框、选 custom 后出现（且不写入），且自定义存储值回显在 custom
     选项标签里。变异核查：去掉「仅 custom 时画字段」的早退，preset 下出现 `Set`、预设值下字段不收敛，断言立刻变红。

### 具体待办（P7-bx 重写；3 条已全部落地：第 1 条 P7-bx、第 2 条 P7-by、第 3 条 P7-bz）

1. **General 页四条滚轴缺「轴的两端词」，读数也不在标题行（R162 后的原版 `SettingsScale`）（本轮已落地）** ——
   原版每一条 range 行都是同一形状：**标题行 = 标签 + 实时读数**（`.settings-slider__head`，
   space-between、baseline），**轨道下方 = 轴两端各一个词**（`SettingsScale`）。两端词按轴而取：
   两条透明度滚轴是 `settings.transparency.scaleLow/.scaleHigh`（Transparent / Solid），终端外观的
   字号与行高滚轴是 `settings.scale.small/.large`（Small / Large）。本构建的 `a.slider` 把读数放在
   轨道行尾、**从不画两端词**，于是用户看到一个孤零零的百分比，不知道 10% 与 100% 各意味着什么。
   顺带：`settings.glassIntensityHint` 在 R162 改过（点明 off 之后的「扭曲/折射/透镜质感」），本构建的
   `GlassHint` 还是 R162 之前的旧句。
   - 原版：`src/settings/GeneralPage.tsx:97-125`（`OpacityControl`）+ `src/settings/SettingsRows.tsx:268-277`
     （`SettingsScale`），透明度行 `GeneralPage.tsx:580-592`；终端滚轴 `src/settings/TerminalAppearance.tsx:399-407`；
     `src/styles/settings.css:735-834`（`.settings-slider__head/__scale`、`.opacity-control__value`）；
     `src/i18n.ts:346`/`:350-351`（hint 与两个轴词）、`:221-222`/`:335`。
   - 本构建（改前）：`internal/settingsui/settingsui.go:2063-2095` 的 `a.slider`（读数在轨道行、无轴词）；
     四个调用点 `:1853`/`:1858`/`:1930`/`:1939`；`internal/i18n/copy.go:305` 的旧 `GlassHint`。
   - 验收（已满足）：`a.slider` 增加 `low`/`high` 两个参数（都空则不画轴词，不发明一根轴）；读数移到标题行
     右端、改 `TextStrong`；轨道下画两端词（`FontSize-2`、`TextMuted`、weight 500）；四个调用点分别传
     透明度两词与终端两词；i18n 新增 `TransparencyScaleLow/High`、`ScaleSmall/Large`（en/zh）并把
     `GlassHint` 换成 R162 措辞。测试 `TestSliderRowsNameTheirAxis`：Transparent/Solid 各恰好 2 次，
     Small/Large 各 ≥3 次（含 Interface size 分段控件那 1 次）。变异核查：把轴词行改成 `if false`，四条断言全红。

2. **General 页没有独立的「玻璃」区与区脚注（原版 `settings.material`）（本轮 P7-by 已落地）** —— 原版把材质分成
   一个自己的 section（标题 `settings.material` = Glass/玻璃），里面两张卡：**玻璃强度**卡与**透明度**卡
   （卡标题 `settings.group.transparency`），两条说明（`glassIntensityHint`、`transparencyHint`）画成
   **区脚注**、不重复在每一行。本构建把玻璃控件折进 Appearance 卡、透明度两条滚轴放第二张卡，并且
   每条滚轴各带一句行内 hint（`MainOpacityHint`/`TerminalOpacityHint`）。
   - 原版：`src/settings/GeneralPage.tsx:558-594`（material section）+ `src/i18n.ts:338`/`:209`/`:346`/`:352`。
   - 本构建（改前）：`internal/settingsui/settingsui.go:1836-1866`（玻璃在 Appearance 卡、透明度卡带行内 hint）；
     `internal/i18n/i18n.go` 无 `Material`/`GroupTransparency` 字段。
   - 验收（已满足）：新增 `Material`（Glass/玻璃）、`TransparencyHint`（原 transparencyHint）、
     `GroupTransparency`（Transparency/透明度）三个 i18n 字段，删掉原版没有的 `MainOpacityHint`/
     `TerminalOpacityHint`；General 页新增 `a.sectionFoot(c, copy.Material, "", []string{copy.GlassHint,
     copy.TransparencyHint}, …)`：内含玻璃强度卡（不再画在 Appearance 卡里）与透明
     度卡（`a.cardLabeled(copy.GroupTransparency, …)`，两条滚轴的 description 传空），两句说明画在两张卡
     之下。`rows.go` 新增 `sectionFoot`（`section` 是其无脚注包装）与 `cardLabeled`（卡的无障碍名）。
     注：`Scale`（界面大小）在本构建住在 Window behaviour 卡、本轮不动；Appearance 卡去掉玻璃后只剩
     主题/图标/语言。测试 `TestGeneralMaterialGroupFootnotes` 钉住：material 区标题与两句脚注各恰好一次（不再有
     行内 hint），玻璃控件在区标题之下、两条透明度行也在区标题之下，两句脚注在各自卡片末行之下。

3. **General 页没有独立的「菜单栏」区（原版 `settings.group.menuBar`）（本轮 P7-bz 已落地）** —— 原版给菜单栏自己的 section
   （标题 `settings.group.menuBar`），把**应用图标**选择器（R150）与**显示菜单栏图标**开关放在同一张卡里
   （它们说的是同一件事：那个图标）。本构建把 `app_icon` 拆到 Appearance 卡、`show_menubar_icon` 拆到
   Window behaviour 卡，两个相关控件隔了一屏。
   - 原版：`src/settings/GeneralPage.tsx:482-530`（R7-10c 开关 `:509` 在前、R150 图标 `:529` 在后，同一张卡）。
   - 本构建（改前）：`internal/settingsui/settingsui.go:1840`（`copy.AppIcon` 在 Appearance 卡）与`:1878`
     （`copy.ShowMenubarIcon` 在 Window behaviour 卡）。
   - 验收（已满足）：新增 `GroupMenuBar` i18n 字段（en `Menu bar` / zh `菜单栏`，旧版原文）；两个控件搬进新的
     菜单栏 section（**开关在前、图标在后，即原版顺序**——P7-bx 重写的待办误写成「图标在前」，本轮按原版
     更正），Appearance 卡去掉图标行、Window behaviour 卡去掉开关；测试 `TestGeneralMenuBarGroup` 钉住两控件
     在同一 section（其间不出现别的区标题、开关在图标之上）且 Appearance 卡不再含 `copy.AppIcon`。
     变异核查：图标行换到开关之前，顺序断言立刻变红。

> 卡框架的三条（`Options.OnCopy` 复制反馈、终端键入的 `HandleInput` Unicode-空格缝、外观四个死旋钮）
> 与一次性维护者动作（`mygo keygen`、`mygo.json` 的 `updates`/签名、CI 私钥、发布到 GitHub Release）
> 仍见「已知差距」，不重复列入待办。

### 具体待办（P7-bu 重写；3 条已全部落地：第 1 条 P7-bv、第 2 条 P7-bv、第 3 条 P7-bw）

1. **自定义集成描述控件的三条契约（R158 + R159）（本轮已落地）** —— 原版 v0.3.14 把描述输入收成
   **executable-only**（R158）：脚本清单写死自己的内部句子、从不读请求里的描述，所以脚本模式下那个
   输入框是「可编辑但被静默丢弃」的死控件；可执行分支同时拿两条 R159 契约——留空就诚实存 `""`（不再
   落 `Local integration for <exe>` 这句「用户永远清不掉」的兜底），输入框配 placeholder 说明「留空则
   不显示描述」、label 改成「描述（可选）」。
   - 原版：`src/extensions/CustomIntegrationDrawer.tsx`（`integration.mode === "executable" && <label>…`
     + `placeholder={t("…customDescriptionPlaceholder")}`）；`src-tauri/src/extensions/install.rs:341-342`
     （脚本写死 `"Local script integration"`）、`:1306`（读回 `(mode == "executable").then_some(...)`）；
     `src/i18n.ts:679`/`:682`（`Description (optional)` / `Leave blank to show no description`，zh `:1561`/`:1562`）。
   - 本构建（改前）：`internal/settingsui/settingsui.go` 的描述行（旧 `:2304-2306`）无条件绘制、无
     placeholder；`internal/i18n/copy.go:658` 的 label 是 `"Description"`、没有 placeholder 字段；
     `internal/extensions/custom.go` 的 `customManifest`/`customDescriptor` 脚本模式照读请求描述；
     `internal/shell/shell.go` 的 `toolForEdit` 对脚本也把 `manifest.Description` 读回表单。
   - 验收（已满足）：`customDescription`（脚本 → `"Local script integration"`，可执行 → trim 后的用户
     原文）同时喂 manifest 与 descriptor；`toolForEdit` 只对非脚本读回描述；`settingsui` 的
     `LocalToolDescription` 行仅在 `mode != routeScript` 时绘制、带 placeholder；i18n 补
     `LocalToolDescriptionPlaceholder` 并把 label 改成「(optional)」。测试：
     `TestCustomScriptDescriptionIsInternal`、`TestCustomExecutableDescriptionStaysHonest`、
     `TestLocalToolDescriptionIsExecutableOnly`，`TestConnectAScriptTool` 加读回断言。

2. **本机连接表单缺「支持的平台」勾选（原版 `customPlatforms`）（本轮已落地）** —— 原版的自定义集成表单有一组三选
   （darwin / linux / windows）的平台勾选，默认只勾**当前平台**，且没有任何一个勾选时提交按钮禁用；
   本构建的 `CustomTool` 里根本没有 `Platforms` 字段，`customRequest` 也不传，于是本机连接出来的集成
   `Platforms` 恒为空（= 所有平台），用户无法限定。后端 `extensions.CustomRequest.Platforms` 已经会写进
   manifest（`internal/extensions/custom.go`），缺的只是表单与映射。
   - 原版：`src/ExtensionsPanel.tsx:369`（`platforms` 字段形状）、`:393`（`platforms: [CURRENT_PLATFORM]`
     默认）、`:1621`（编辑时 `platforms: [...definition.platforms]`）；`CustomIntegrationDrawer.tsx` 的
     `PLATFORMS` + `customPlatforms` fieldset 与提交按钮的 `integration.platforms.length === 0` 禁用；
     `src/i18n.ts:694`（`Supported platforms` / `支持的平台`）。
   - 本构建：`internal/settingsui/settingsui.go:2205-2224`（`CustomTool` 无 `Platforms`）、
     `:2246-2360`（表单无平台行）；`internal/shell/shell.go:3179`（`customRequest` 不传 `Platforms`）；
     `internal/shell/shell.go:1952`（`toolForEdit` 不回填 `manifest.Platforms`）。
   - 验收（已满足）：`CustomTool` 加 `Platforms []string`；表单画三个勾选（默认 `[runtime.GOOS]`）并在一个都没勾时
     拒绝提交（错误句子印在行下）；`customRequest` 透传；`toolForEdit` 从 `manifest.Platforms` 回填；测试
     钉住默认值、无勾选拒绝与一次往返（含后端 `Platforms` 逐字落盘）。落地见下方「P7-bv 支持的平台」。

3. **本机连接请求的后端校验只做了一半（原版 `settings.extensions.form.*`）（本轮已落地）** —— 原版
   `create_custom_integration_locked` 在写包前逐条校验并回**本地化键**：名字 1..80（`form.nameLength`）、
   命令形状 `[a-z0-9][a-z0-9_-]{0,63}`（`form.commandShape`）、版本必须能 `Version::parse`、平台非空
   （`form.platformRequired`）、模式仅 executable/script（`form.modeInvalid`）、脚本内容非空
   （`form.scriptEmpty`）、`script_language` 只对脚本合法（`form.scriptLanguageInvalid`）。本构建的
   `normalizeCustom`（`internal/extensions/custom.go:223-280`）只有「名字非空」「脚本有语言且非空」
   「模式合法」「权限已知」「params 合法」，其余不查：`version` 为空时直接填 `0.0.1` 而不校验格式，
   `Platforms` 允许为空，executable 也接受 `ScriptLanguage`。
   - 原版：`src-tauri/src/extensions/install.rs:249-300`（七条校验与各自的键）；`src/i18n.ts` 的
     `settings.extensions.form.*` 七个键。
   - 本构建：`internal/extensions/custom.go:223`（`normalizeCustom`）缺 name 长度、command 形状、version
     semver、platforms 非空、script-language-only-for-script 五条。
   - 验收（已满足）：新增 `internal/extensions/customvalidate.go` 的 `CustomIssues`（`CustomField` + `CustomIssueKey`，
     旧构建七个 i18n 键的稳定身份）同时喂后端与表单；`normalizeCustom` 在默认值填完后按第一个键拒绝，
     派生命令另用 `validCustomCommand` 兜一遍；`customRequest` 给非脚本清掉脚本字段（否则表单默认语言会把
     每个 executable 都判成 `scriptLanguageInvalid`）；表单在提交前跑同一函数并把句子用 `toolIssueUnder`
     印在行下（`toolIssue` 开关，首次提交前不印）；i18n 新增六个字段（en/zh）。测试：
     `TestCustomIssuesNamesEveryRule`（逐条合法/非法 + 错误串里的键）、
     `TestCustomCommandShapeAppliesToTheDerivedWord`、`TestCustomDefaultsFillWithoutInventingAPlatform`、
     `TestLocalToolFormNamesTheShapeRules`、`TestAnExecutableCarriesNoScriptFields`。证伪：抽掉命令形状循环后
     三条命令用例与派生命令测试同时变红。落地见下方「P7-bw 后端校验」。

### 具体待办（P7-br 重写；4 条已全部落地：第 1·2 条 P7-br、第 4 条 P7-bs、第 3 条 P7-bt）

1. **分页页脚的三态（R29）（本轮已落地）** —— 原版 `pluginFooterState`（`src/launcher/plugin-mode.ts:613-626`）
   把页脚收成四个状态，`LauncherResults.tsx:1024-1040` 只给 `loading` 画 spinner、给 `loading` 与
   `end` 画字，`launcher.css:1676-1705` 写明「`more` 是空的——滚动本身就是提示」。本构建以前在 `more`
   画了原版没有的 “Scroll for more”、`loading` 只有字没有环。
   - 本构建（改前）：`internal/launcher/output.go:221-243` 的 `outputFooter` 默认 `text := copy.OutputMore`；
     `internal/i18n/copy.go:188` 的 `OutputMore` 不在原版词表里；`grep -rn 'Rotate\|spinner' internal/launcher --include=*.go` 为空。
   - 验收（已满足）：`outputFooterState`（`footerNone`/`footerMore`/`footerLoading`/`footerEnd`）；`more` 只留
     行高；`loading` 画 12px 环（`Hairline` 整圈 + `TextMuted` 四分之一弧，`ui.Linear` 0.7s 一圈）；
     `OutputMore` 字段与两份文案已删（`TestNoDeadCopy` 守住）；zh 的 `OutputEnd` 改回「没有更多结果」；
     `TestOutputFooterStates`、`TestOutputFooterLoadingDrawsTheRing` 钉住。
2. **两个环境循环在减少动效下应当静止（R8 + R161）（本轮已落地）** —— 原版把
   `.launcher-plugin-footer__spinner` 与 `.terminal-bar__dot` 两个循环在 `prefers-reduced-motion` 下
   `animation: none`（`base.css:1305-1362`），R161 又加通配块；框架的 `Loop` 明确不跟这个偏好
   （`ui/preferences.go`：「`Loop` … goes on」），所以 App 必须自己读。本构建以前两个循环都无条件跑，
   且状态点 keyframes 与原版不一致（0.75..1 且不缩放）。
   - 原版：`src/styles/base.css:1305-1340`（命名块）、`src/styles/terminal.css:148-187`（keyframes）、
     `src/styles/launcher.css:1692-1705`（环）。
   - 本构建（改前）：`internal/terminalui/terminalui.go:946-957` 的 `dot.Opacity(0.75 + 0.25*dot.Loop(...))`；
     `grep -rn 'ReduceMotion\|Preferences()' internal --include=*.go` 为空。
   - 验收（已满足）：`internal/motion.Value`（减少动效时回 0 且不调用 phase）；状态点
     `Opacity(1-0.5*breath)` + `Scale(1-0.12*breath)`；页脚环经同一函数取相位；
     `TestValueRestsUnderReduceMotion`、`TestStatusDotRestsUnderReduceMotion` 钉住（同一帧像素对比，
     两个 mutation 复现红）。
3. **Windows 冒烟作业：编译过一百次、执行过零次（原版 ad843f3）（本轮已落地）** —— 原版新增
   `.github/workflows/windows-smoke.yml`（仅 `workflow_dispatch`，**故意不进 release 链**）：在真实
   windows-latest 上构建 exe、用 CLI 自己的冷启动揭示启动它、截屏桌面并上传截图供人看。理由写在提交里：
   「Every gate this repo runs is a Linux gate: the cfg(windows) code has been compiled a hundred times and
   executed zero」，而 R156 的材质与 R157 的圆角只有用户重测这一条运行期证据。本构建同样如此：`go.yml`
   的矩阵在 windows-latest 上只跑 gofmt/vet/build/test，**从不启动那个 .exe**；而 Windows 专属的运行期
   代码（`internal/shell` 的 `vibrancyForStop` 的 windows 分支、玻璃雾层、窗口圆角、终端的原生库）正是
   只有跑起来才看得见的。
   - 原版：`.github/workflows/windows-smoke.yml`（v0.3.14 的 67 行）；截图用 runner 自带的 PowerShell
     （`Add-Type -AssemblyName System.Windows.Forms,System.Drawing` + `CopyFromScreen`），启动前不要求
     产品代码改动。
   - 本构建：`.github/workflows/go.yml:20-50`（test 矩阵）、`:63-93`（package 矩阵，Windows 只上传
     `dist/*/*.exe` 就结束）；`cmd/floter/main.go:73-84` 的 `FLOTER_OPEN`（`launcher` / `settings` /
     `terminal`，由 `shell.ParseSurface` 解析）就是现成的揭示方式，与终端的原生库一起由
     `go tool mygo build -platform windows/amd64` 打进 exe。
   - 验收（已满足）：`internal/ci/workflows_test.go` 钉住触发面与产物；`gh workflow run windows-smoke.yml
     --ref mygo-rewrite` 真跑一次、下载 artifact 确认截图里能看到窗口（见下方「P7-bt」记录）。
4. **`prefers-contrast: more` 一整块没接线（原版 `base.css:1387-1574`）（本轮已落地）** —— 原版把「提高对比度」当作
   **材质**的事，不只是边框：`--glass-frame-floor: 0.86`（透明的底端被抬到近乎不透明）、
   `--glass-step-dim: 0`（去掉雾层）、`--glass-lens-scale: 0.6`（反光阻尼到近平面），再加
   `--stroke-contrast` / `--stroke-contrast-soft` / `--text-placeholder-contrast` 三个 token 换掉
   卡片、抽屉、toast、选中行、`settings-option--active`、快捷键录制等的描边与占位符颜色（浅色下是
   `rgba(0,0,0,.5)`/`.36`/`.28`，深色下是 `rgba(255,255,255,.42)`/`.34`/`.58`）。本构建对
   `HighContrast` **没有任何引用**：`internal/theme/theme.go:212-227` 反而在这里把框架主题的
   `Border`/`Text` 等整套覆写成 base.css 的普通值，恰好把框架自己的高对比度加固覆盖掉；
   `internal/glassmap/glassmap.go:148-175` 的 `SpecFor` 也只有一个 `SolidTop = 0.98` 上限、没有下限。
   - 原版：`src/styles/base.css:1387-1574` 与 `:1576-1587`（两个 `@media (prefers-contrast: more)` 块，后者是 Windows 三张面壳的描边）、
     `:386-388`（`--glass-frame-floor` 的取值与理由）、`:753-756`/`:1004-1006`（两个外观的 token）。
   - 本构建（改前）：`grep -rn 'HighContrast' internal cmd --include=*.go` 为零；
     `internal/theme/theme.go:212-227`、`internal/glassmap/glassmap.go:130-175`。
   - 验收（已满足）：`theme.ForPreferences(s, dark, prefs)`（`For` 是它的无偏好包装，供无 context 的 headless 渲染）；
     `palette.contrast(appearance)` 换掉 `stroke` / `controlEdge` / `textMuted` 三个 token（两套外观各四套值）；
     `glassmap.Spec.Contrast()` 把 frame alpha 抬到 `max(0.86, min(SolidTop, opacity))`、`HazeAlpha` 归 0、
     `LensScale ×0.6`，`SpecForPreferences` 是它的布尔包装；全部绘制点（launcher 11 处、settingsui、terminalui、
     shell 的终端框与三处 `launcher.Resolve`）改读 `c.Preferences()`。`TestHighContrastSwapsTheStrokeAndPlaceholderTokens`
     钉住三个 token 的精确值 + 其余 token 逐字段不变 + 材质抬升；`TestContrastRaisesTheFloorDropsHazeAndDampsTheLens`
     钉住四个 stop 的 floor/haze/lens 与「blur 不动」，`TestContrastFloorNeverLowersATransparentFrame` 钉住
     不清空偏好时逐字节等于 `SpecFor`。一处诚实记录：`--text-placeholder-contrast` 落在 `TextMuted` 上，
     因为框架把输入框占位符画在 `editor.Theme.LineNumber = TextMuted.Alpha(0.75)`（mygo `ui/editor/theme.go`），
     `ui.Theme` 没有独立的占位符字段；这是框架能接到的唯一处。

> P7-bp 的 3 条具体待办已全部落地，下方旧清单保留作为「哪些已经做过」的记录。

### 具体待办（P7-bp 重写；3 条已全部落地：第 1 条 P7-bp、第 2·3 条 P7-bq）

1. **本地工具表单的权限开关与边界提示收进「高级」披露（本轮已落地）** —— 原版 R161 把自定义集成
   编辑器里两个**只做声明 / 审计**的权限字段集（`ENFORCED` / `DECLARED`）连同它们上方的
   `permissionBoundary` 提示一起包进一个默认折叠的 `<details>`，标题「Advanced · Permissions & audit」；
   理由写在自己的注释里：这两个字段集的提示就承认「不提供操作系统隔离」，画成与宿主真正读取的字段
   同权，会让一个全新的集成一打开就是一张仪式表单。本构建以前把 `extensions.AllPermissions` 逐条画成
   开关行、也没有那条边界提示。
   - 原版：`src/extensions/CustomIntegrationDrawer.tsx`（v0.3.14 的 `<details className="extension-custom-advanced">`
     包住 `permissionBoundary` 与两个 `<fieldset>`）；i18n `src/i18n.ts:704-705`（`customAdvanced` /
     `customAdvancedPermissions`）与 `src/i18n.ts:783`（`permissionBoundary`）。
   - 本构建（改前）：`internal/settingsui/settingsui.go` 的 `localToolSection` 在卡片里 `for _, permission
     := range extensions.AllPermissions` 直接画开关行（旧行号 `:2317-2331`）；`grep -rn 'CustomAdvanced'
     internal --include=*.go` 为空。
   - 验收（已满足）：新增 `a.toolAdvanced`（默认 false，`beginEdit`/`endEdit` 复位）；
     `toolPermissionFields` 只在展开时画 `PermissionBoundary` note 与开关行；勾选仍住在
     `a.toolPermissions`，折叠后提交照带（折叠只是表现，不卸装）；i18n 补
     `LocalToolAdvanced`/`LocalToolAdvancedPermissions`/`PermissionBoundary`（en/zh）；
     `TestLocalToolAdvancedDisclosure` 与新改写句的 `TestLocalToolForm` 钉住。
2. **版本字段缺「兜底」提示（R153）（P7-bq 已落地）** —— 原版在自定义集成的版本输入下印一条 hint，说明「探测到真实
   版本会自动覆盖；探测失败时用你填的值兜底」，让一个可编辑的版本框诚实说明自己是什么。本构建的版本行
   只有一个占位符 `0.0.1`，没有这行提示。
   - 原版：`src/i18n.ts:685` 的 `settings.extensions.customVersionHint`（zh 在 `:1565`），画在
     `src/extensions/CustomIntegrationDrawer.tsx:133-135` 的 `extension-custom-form__hint`。
   - 本构建：`internal/settingsui/settingsui.go:2302` 的版本行 `ui.TextInput(c, &version).Label(...).Placeholder("0.0.1")`
     没有 hint；`grep -rn 'CustomVersionHint' internal --include=*.go` 为空。
   - 验收（已满足）：版本行补一行本地化 hint（`LocalToolVersionHint`，en/zh），`TestLocalToolVersionHint` 钉住。
3. **select 选项的编辑用「一行一个」而非逗号（R153）（P7-bq 已落地）** —— 原版把选项编辑器的分隔符从逗号改成换行，
   理由直接写在代码里：一个选项值自身可能含逗号，用逗号分隔会在下一次往返时把一个值拆成两个。本构建的
   编辑器仍用逗号。
   - 原版：`src/extensions/script-params.ts:92-107` 的 `parseParamOptions`（`split("\n")`，trim 后去空行）
     与 `formatParamOptions`（`join("\n")`）。
   - 本构建：`internal/settingsui/settingsui.go:3140` 的 `strings.Split(param.Options, ",")` 与
     `:3206` 的 `strings.Join(param.Options, ", ")`；占位符 `:3083` 的 `"fast, slow"`。
   - 验收（已满足）：选项按换行拆分（trim、去空行），`beginEdit` 按换行 join，占位符改成「一行一个」，
     多行 `ui.TextArea`；`TestParamOptionsNewlineCodec` 钉住一个含逗号的选项能一次往返。

> P7-bm 的 4 条具体待办已全部落地，下方旧清单保留作为「哪些已经做过」的记录。

### 具体待办（P7-bm 重写；4 条已全部落地：第 1 条 P7-bm、第 2 条 P7-bn、第 3·4 条 P7-bo）

1. **健康报告的探针明细与检查时间从不展示（本轮已落地）** —— 原版 `extension-health` 会逐条画出
   `healthReport.probes`（通过/失败的记号 + `probeId` + `{durationMs}ms`）、失败项（`healthExitCode` +
   stderr + `healthRetryable`）与 `healthCheckedAt` 时间戳；本构建只画一句状态词，报告里的
   `ProbeRecord.DurationMs`/`ExitCode`/`Stderr` 与 `HealthReport.CheckedAt` 全部算完就丢。
   落地见下方「P7-bm 健康报告的探针明细」。
   - 原版：`src/ExtensionsPanel.tsx:2922-2964` 的 `extension-health` 块——`healthReport.probes.map`
     每条画记号 + `probeId` + `{probe.durationMs}ms`；`healthReport.failures.map` 画 `probe` +
     `(exit: {code})` + stderr + `healthRetryable`；末尾 `healthCheckedAt`。i18n 键
     `settings.extensions.healthProbes`/`healthFailures`/`healthExitCode`/`healthRetryable`/
     `healthCheckedAt`（`src/i18n.ts:584-590`）。
   - 本构建（改前）：`internal/settingsui/settingsui.go:1199` 的 `integration.Health` 只画状态词；
     `extensions.ProbeRecord.DurationMs`（`internal/extensions/probes.go:41`）、`ExitCode`、`Stderr`
     与 `HealthReport.CheckedAt`（`probes.go:51`）**没有任何视图读取**（`grep '\.DurationMs'
     internal/settingsui` 为空）；`settingsui.Integration` 也没有承载报告的字段。
   - 验收（已满足）：`settingsui` 新增 `HealthReport`/`HealthProbe` 视图并画在 Health 行下；shell 的
     `healthReportView` 把仓库里的报告投影进来，`Retryable` 取清单 `required` 的反（不落盘，磁盘格式
     不变）；i18n 补五句；测试钉住渲染与投影。
2. **`extensions.CapabilityReport` / `ProbeCapabilities` 是双方都有的死代码（P7-bn 已删除）** ——
   `internal/extensions/capability.go` 的 `ProbeCapabilities`/`SupportedFeatures`/`Limitations`
   只有自己的测试调用（`grep -rn 'ProbeCapabilities' internal cmd --include=*.go | grep -v _test` 为空）。
   原版 `capability_probe.rs` 的 `CapabilityScanner::scan` 同样只有测试调用，且 reprobe 路径传的是
   `HealthReport::new(Default::default())`（`probe_executor.rs:17`），所以原版卡片里的
   `capabilities.version` 恒为 `unknown`、features/limitations 恒空——**两版都不展示**。
   这不是偏离，是一份双方共有的死代码，需要一个决策：接线（让 `Reprobe` 也跑 `ProbeCapabilities`，
   把 `Capabilities` 补回 `HealthReport` 并画出 `v{version}`/features/limitations，同时补磁盘字段）
   或删除（`capability.go` 与其测试一起移除）。验收：接线则卡片有 version/features/limitations 且
   测试钉住；删除则 `grep` 为零。**P7-bn 选择删除**（接线要改磁盘格式且偏离原版行为），落地见下方
   「P7-bn 删除双方共有的死代码」；`capability.go` 里仍活着的 `--help` 派生重命名为 `helpderive.go`。
3. **`browser.Entry.ProfileKey` 与 `browser.FindProfile` 是死字段/死函数（P7-bo 已删除）** ——
   `internal/browser/browser.go:60` 的 `ProfileKey` 只在构造时写入（`:447,616,650` 与 `safari.go:61`），
   全仓无 `.ProfileKey` 读取（`grep -rn '\.ProfileKey' internal --include=*.go | grep -v _test` 为空）；
   `FindProfile`（`target.go:50`）除自身测试外无调用者。原版用 `profile_key` 在 `browser_open_url`
   （`src-tauri/src/browser_data/mod.rs:354`）里反解 browser id；本构建改用 `Entry.BrowserID` 直接开
   （`internal/launcher/search.go:273` 的 `openResult`），行为等价，所以这是**冗余清理**而非缺口。
   验收：删除 `ProfileKey`/`FindProfile` 及其测试（或让 `OpenURL` 真的经 `FindProfile` 解析），
   `grep` 为零，`OpenURL` 测试仍绿。**P7-bo 选择删除**（保留 `Profile.Key()` 本体，它仍被测试与
   target 解析使用）；落地见下方「P7-bo 删除两处死代码」。
4. **`extensions.CommandInfo.ExtensionName` 是死字段（P7-bo 已删除）** —— `internal/extensions/store.go:211`
   只写不读（`shell.go:1239` 的 `integrationList` 映射不用它，卡片已按集成分组）。验收：删除字段与赋值，
   `grep` 为零，测试不受影响。**P7-bo 已落地**；落地见下方「P7-bo 删除两处死代码」。

> 这 4 条都不卡框架，且已全部落地（第 1 条 P7-bm，第 2 条 P7-bn，第 3·4 条 P7-bo）。上面
> P7-bh/P7-an/P7-bb/P7-bd 的旧清单保留在下方，作为「哪些已经做过」的记录。

### 具体待办（P7-bh 重写；第 1–5 条全部已落地）

1. **清单的路径安全（`validate_relative_path`）**（P7-bh 已落地）—— 原版在**解析清单时**就拒绝
   任何非空、非相对、含 `.`/`..` 的声明路径；本构建只在使用时用 `joinWithin` 兜住一部分，导出
   的 staging 写入（`sync.go:417,427`）完全裸奔。落地见下方「P7-bh 清单路径安全」。（原分析保留在下方。）
   - 原版：`src-tauri/src/extensions/manifest.rs:606` 的 `validate_relative_path`（非空、非绝对、
     每个 component 必须是 `Component::Normal`），由 `validate_paths`（`:565`）用在 `icon`、
     `Runtime::Script.path`、`provider.descriptor`，并由 `lifecycle.rs:39/145/173` 用在 completion
     `source`、template `source`/`target`、`launch.command.program`（`self` 豁免）。`parse()`
     （`:492`）在反序列化后立刻调用，所以坏包根本进不来。
   - 本构建（改前）：`internal/extensions/manifest.go:225` 的 `LoadManifest` 只查 ID/Name；
     `internal/extensions/sync.go:417,427` 直接把 `manifest.Runtime.Path` / `Provider.Descriptor`
     `filepath.Join` 进 staging（`..` 可越界写文件）；`internal/extensions/description.go:139` 的
     `joinWithin` 只在描述符/脚本/模板被**读取**时兜住，`icon` 全仓无人查。
   - 验收：新增 `ValidateRelativePath(value, field)` 与 `Manifest.ValidatePaths()`（含 lifecycle 的
     路径），接进 `LoadManifest`；绝对路径、空、`.`/`..` component 报错；测试钉合法/非法表、三处
     manifest 字段、lifecycle 四处路径、以及一个 `LoadManifest` 端到端拒绝的夹具。
2. **lifecycle 的结构校验（`Lifecycle::validate`）**（P7-bi 已落地）—— 原版对 completion / template /
   probe / launch 有一整套规则；本构建的 `Lifecycle`（`manifest.go:117`）原来只解析，无任何校验。
   落地见下方「P7-bi lifecycle 的结构校验」。（原分析保留在下方。）
   - 原版：`src-tauri/src/extensions/lifecycle.rs:39`（同一 shell 的 completion 不重复、`source` 与
     `args` 恰有其一、`args ≤ 16`、`timeoutMs ∈ 100..=10000`、`fileName` 合法）、`:145`（template
     `target` 不重复）、`:173`（probe `id` 不重复、`timeoutMs ∈ 100..=30000`、`args ≤ 32`）、
     `LaunchConfig::validate`（`restorePolicy ∈ {reattach,restart,none}`、launch args 无 NUL）。
   - 本构建（改前）：`internal/extensions/manifest.go` 的 `ShellCompletion`/`Template`/`Probe`/`Launch`
     只有字段；`launch_test.go`/`probes_test.go` 的夹具因此可以带着任意 timeout 与重复 id 通过。
   - 验收：新增 `Lifecycle.Validate()` 复刻全部规则（含 `restorePolicy` 字段的解析），接进
     `LoadManifest`；逐条测试（重复 shell、`source`+`args` 同时给、`args=17`、timeout 越界、重复
     probe id、probe timeout 越界、`args=33`、未知 `restorePolicy`、NUL）。
3. **运行期参数校验与 argv 权威（`param_arguments`）**（P7-bj 已落地）—— 原版把声明 + 答案在
   **运行前**重新校验并转成 argv，任何非法组合发稳定键；本构建原来的 `ParamArgv` 只拼 argv，从不拒绝。
   落地见下方「P7-bj 运行期参数校验与 argv 权威」。（原分析保留在下方。）
   - 原版：`src-tauri/src/extensions/run.rs:290` 的 `param_arguments`——未声明的 id →
     `run_param_unknown:{id}`；required 且无值无默认 → `run_param_required:{id}`；flag 非法 /
     number 非有限 / select 不在选项 / boolean 不是 true|false → `run_param_invalid:{id}`；
     Windows `.cmd`/`.bat` 且值含 `WINDOWS_CMD_UNSAFE_CHARS`（`&|<>^%`，`run.rs:231`）→
     `run_param_windows_unsafe:{id}`；缺值回填 `default`。
   - 本构建（改前）：`internal/extensions/params.go:163` 的 `ParamArgv` 静默丢未知 id、不查 required、
     不校验 number/select/boolean、不回填 default、无 windows-unsafe；`shell.go:1787` 直接把它交给运行。
   - 验收：新增 `ParamArguments(params, values, cmdScript) ([]string, error)` 发 `run_param_*` 键，
     `shell.go` 改用它；测试钉每个键与边界（含 default 回填、`inf`/`NaN` 拒绝、cmd 脚本不安全字符）。
4. **运行表单的逐参数本地化错误（`paramValueIssues` / `paramRunErrorMessage`）**（P7-bk 已落地）——
   原版把「哪个参数、哪一条规则」本地化后印在出错行下；本构建只有一句泛化的 `RunIncomplete`。
   落地见下方「P7-bk 运行表单的逐参数本地化错误」。（原分析保留在下方。）
   - 原版：`src/extensions/run-params.ts` 的 `paramValueIssues`（返回 `{index,key,label}`，
     required-有默认不报缺）与 `paramRunErrorMessage`（后端 `run_param_*` → i18n），
     `src/i18n.ts` 的 `customParamRequiredMissing`/`InvalidValue`/`Unknown`/`WindowsUnsafe`。
   - 本构建（改前）：`internal/settingsui/settingsui.go:2725` 把 `ParamIssues` 的非空结果换成
     `copy.RunIncomplete`；`grep -rn customParamRequiredMissing internal/i18n` 为空；
     `internal/launcher/runfailure.go:17` 对 `run_param_*` 原样返回。
   - 验收：`ParamIssues` 返回 `{Index, Key, Label}`（并补 required-有默认豁免、boolean 校验）；
     面板在出错行下印本地化句子；输出视图按语言渲染 `run_param_*` 键；i18n 补四句（en/zh）。
5. **清单的严格解析与组合规则（`deny_unknown_fields` + `validate_paths` 的组合那半）**（P7-bl 已落地）——
   原版每个 manifest struct 都 `deny_unknown_fields`，且只允许三组 distribution/runtime/provider 组合，
   并在解析期跑 `validate_param_definitions`；本构建原来 `json.Unmarshal` 静默忽略未知字段，也没有
   组合与参数声明校验。落地见下方「P7-bl 清单的严格解析与组合规则」。（原分析保留在下方。）
   - 原版：`src-tauri/src/extensions/manifest.rs` 的 `#[serde(deny_unknown_fields)]`（`:17,66,230,
     237,251,258,265,335,366`）；`validate_paths`（`:565`）只放行
     `(Local, System, Executable|StaticDescriptor)`、`(Local, Script, Executable|StaticDescriptor)`、
     `(BuiltIn, System, StaticDescriptor)`，且 `Local + StaticDescriptor` 必须有 `descriptor`；
     `validate_paths` 还调 `validate_param_definitions`（`:145`）。
   - 本构建（改前）：`LoadManifest`（`manifest.go:225`）用 `json.Unmarshal`，未知键静默忽略；
     `platform_test.go:250` 的 `runtime.type: "static-descriptor"` 这种非法组合照收；
     `ValidateParams` 只在 `install.go:268`/`custom.go:278` 跑，解析期不查 default。
   - 验收（已满足）：严格解析（`json.Decoder.DisallowUnknownFields`）拒绝未知字段与尾随数据；
     组合规则拒绝并修正非法夹具；`Manifest.Validate()` 在解析期跑 `ValidateParams`（含 default 逐
     kind）；测试钉「未知字段拒绝 / 五种合法组合 / 其余组合拒绝 / Local+StaticDescriptor 缺 descriptor
     拒绝 / 坏参数声明在解析期拒绝 / default 逐 kind」。

> 这 5 条都不卡框架，**第 1–5 条已全部落地**。上面 P7-an/P7-bb/P7-bd 的旧清单保留在下方，
> 作为「哪些已经做过」的记录。

### 具体待办（P7-an 把清单写实；每项带 file:line、原版对照、验收标准）

1. **终端复制的反馈（R44/R63）** —— 唯一一条要等框架的能力。
   - 原版：`src/terminal/copy-notice.ts`（`COPY_NOTICE_VISIBLE_MS = 1500`、`COPY_NOTICE_FADE_MS = 260`、
     `visible → fade → idle`，第二次复制重启 visible 而不叠计时器）、`src/hooks/useTerminalView.ts`
     的 `copySelection`（写失败也 `showCopyNotice("terminal.copyNotice.failed")`）、
     `src/hooks/useLauncherTextCopy.ts`（直出面共用同一套词）。
   - 本构建：`internal/terminalui/terminalui.go` 里没有任何 copy 路径；`internal/i18n` 没有
     `Terminal.CopyNoticeCopied/Failed`。
   - 卡在哪（file:line）：mygo `plugins/terminal/view.go:467` `v.copy()` 直接写剪贴板；
     `ui/services.go:14` 的 `Services` 宿主拥有；`ui/handler.go:20` 的 `InputCommand` 只能由菜单
     role 发出；应用菜单用 `mygo.RoleEditMenu`（`internal/shell/system.go:459`）。
   - 验收：mygo 给出 `Options.OnCopy`（或可观察的 Edit-Copy）后，`internal/terminalui` 接线；
     测试钉「成功一句 / 失败一句」「连续两次复制只有一个计时器」「1.5s 后进 fade、再 260ms 清空」。
2. **键入文本的 Unicode 空格接缝**（粘贴与 App 拼的命令行已在 P7-an 封住，只剩框架键入这一面）。
   - 原版：`src/hooks/useTerminalView.ts:917-925`。
   - 卡在哪：`plugins/terminal/view.go:60` + `ui/input.go:568`（`HandleInput` 只给聚焦元素）。
   - 验收：钩子到位后，键入 `go<U+3000>version` 到 shell 里的字节是两个词；测试从字段层发键断言。
3. **终端外观剩下的四个死旋钮**：`internal/settings/settings.go:195-199` 的 `TerminalBold` /
   `TerminalWheelLines` / `TerminalSelectCopy` / `TerminalScrollbar`；原版
   `src/settings/TerminalAppearance.tsx:302,320,330,355`（都是 `page: "strip"`）。
   验收：框架暴露旋钮后按 `internal/control` 加轴 + 抽屉行（P7-an 的 `AxisPasteSafe` 就是模板：
   加轴 → `AppearanceAxes()` → 抽屉行 → `control`/`settingsui`/`terminalui` 三处测试自动要求）。
4. **~~死文案清理~~**：已完成（P7-ao）——10 个死键已删，并加了 `TestNoDeadCopy` 守卫
   （反射走 `Copy` 的每个叶子字段，在 `internal/i18n` 之外零 `.字段名` 引用即失败）。
5. **~~`cwdPolicy` 的对象形（`projectRoot` / `fixed`）在加载期就让清单失效~~**：已完成（P7-ar）。
   （原分析保留在下方。）
   - 原版：`src-tauri/src/extensions/cwd_policy.rs:64` 的 `CwdPolicy::from_manifest` 接受**两种拼写**——
     字符串（`inheritActiveSession` / `toolData` / `home`）与对象（`{"policy":"projectRoot",
     "markers":[...], "maxDepth":n}`、`{"policy":"fixed","path":"/abs"}`）；schema 也是这么写的
     （`docs/extensions/schemas/floter-extension.schema.json:380-421`）。
   - 本构建：`internal/extensions/manifest.go:141` 的 `Launch.CWDPolicy` 是 `string`——
     `json.Unmarshal` 遇到对象直接报错（`cannot unmarshal object into ... type string`），
     于是**一份 schema 合法的清单被整体拒收**（`manifest.go:198` 的 `LoadManifest`）。
     顺带 `FixedPath()`（`manifest.go:154`）只认旧的 `"fixed:<abs>"` 字符串，不认对象形。
   - 验收：`CWDPolicy` 改成带 `UnmarshalJSON` 的类型（字符串与对象两种拼写都能进），
     `projectRoot` 按 `markers`/`maxDepth` 从「活动会话的 cwd」向上找根（无会话 cwd 时按原版回退链
     toolData → home）；`fixed` 对象形读到 `path`；非法对象（缺 `policy`、`fixed` 相对路径）在加载期报错。
     测试：两种拼写的夹具都能 `Refresh` 出命令；对象形非法时 `LoadManifest` 有错。
     **难点**：本构建没有“活动会话 cwd”（`internal/launcher/search.go:429` 用的是 App 自己的 cwd），
     所以 `ProjectRoot` 的 marker 行走只能靠注入的 cwd 测；先把解析与回退链做对，会话 cwd 单独开一项。
6. **~~清单的 `compatibility` 区间从未被检查~~**：已完成（P7-at）。
   - 原版：`src-tauri/src/extensions/manifest.rs:533` 的 `validate_compatibility`——host 版本剥离
     pre-release 后对 `compatibility.floter` 做 `VersionReq` 匹配，provider 协议固定为 `1.0` 对
     `compatibility.providerProtocol` 匹配；在 install（`extensions/install.rs:2522`）与 pre-install
     review（`commands/extensions.rs:1456,1477`）各跑一次。
   - 本构建：`internal/extensions/manifest.go:66` 的 `Compatibility{Floter, ProviderProtocol}` 只被
     `json.Unmarshal` 读进来（`custom.go` 生成工具时写死 `>=0.3.0` / `^1.0`），**没有任何调用者**；
     `Satisfies`（`semver.go:138`）早已存在，却只服务 registry 选版本（`registry.go:122`）。
     声明 `floter: ">=99.0.0"` 的包照装照跑。
   - 卡在哪：`Paths`（`paths.go`）不携带 host 版本；shell 的 `appVersion()`（`shell.go:2815`）
     是唯一知道自己的地方。
   - 验收：`Paths.HostVersion` 由 `shell.New` 从 `appVersion()` 注入；`prepare()` 与 `install()`
     在落盘前调用 `Manifest.ValidateCompatibility(paths.HostVersion)`，两个区间都查，空 host 版本跳过；
     测试钉「越界拒绝 / 协议不匹配拒绝 / 兼容照常 / 空与不可解析 host 不拒绝 / preview host 按 release 行匹配」。
7. **~~插件模式的驻留（P7-av 定位）~~**：已完成（P7-aw）——`residencyHolds` 现在按原版
   `residencySurface` 的同一顺序认 plugin-mode / plugin-config；`View` 每帧 `noteResidency` 随面起表/清表；
   `Toggle` 收成 `summon()`：时钟成立时刷新并保留，否则 `ResetQuery()`+`ClearDropped()` 回空搜索页；
   i18n 提示回到原版原文。（原分析保留在下方。）
   - 原版：`src/surface-residency.ts:201` 的 `residencySurface` 把 collapsed 面分成 `plugin-mode`
     （browser/clipboard/calculator/external）与 `plugin-config`；`src/App.tsx:2480-2530` 的唤回处理
     在时钟成立时保留该面（`residency.refresh()`），否则 `setQueryExitingPlugin("")` + `clearDrops()`
     回到全新搜索页。提示文案 `src/i18n.ts:305` 也写着「A plugin page, its settings, or the terminal
     stays put…」。
   - 本构建：`internal/shell/shell.go:2569` 的 `residencyHolds()` 对 `SurfaceLauncher` 直接 false，
     而 `Toggle()`（`shell.go:2366`）在时钟不成立时只 `Open(SurfaceLauncher)`——**从不收线**，所以
     插件模式/查询永远保留（等价于「永不」），驻留设置在启动器面上完全没有可观察效果；i18n 提示
     （`internal/i18n/copy.go:324`）也已把「插件页面」删掉以掩盖。
   - 验收：`residencyHolds` 认 plugin-mode（`launcher.ConfigOpen()` / 一个新的导出谓词如
     `PluginModeActive()`）与 `plugin-config`；时钟随面变化在 `View` 每帧 `noteResidency`（进入模式起表、
     离开清表）；`Toggle` 在时钟不成立时 `Launcher.ResetQuery()`（含 `leaveFiles`）再回启动器；唤回落到
     同一面时 `refresh` 重启窗口。测试（注入时钟）：插件模式窗内唤回被保留并刷新、窗外唤回回到空搜索
      页（查询与拖放行都清）、普通搜索页唤回永远清空、0/never 边界；把 i18n 提示改回原版原文。
8. **~~`platformOverrides.minimumOsVersion` 的强制执行~~**：已完成（P7-ba）——`internal/extensions/osversion.go`
   新增 `HostOSVersion()` 探测与 `parseOSVersion`（复刻原版 `numeric_version`），`Manifest.MinimumOSVersion`
   按 override 顺序取最后一个下限，`ValidateMinimumOSVersion` 在 `prepare`/`install` 落盘前比较；
   `Paths.OSVersion` 可注入。（原分析保留在下方。）
   - 原版：`src-tauri/src/extensions/install.rs:2524` 的 `resolved.validate_minimum_os_version()`，
     在 install 期比较宿主的 OS 版本与 override 声明的下限；`manifest.rs` 的 `resolve()` 把命中的
     override 的 `minimumOsVersion` 挑出来。
   - 本构建：`internal/extensions/platform.go` 的 `PlatformOverride.MinimumOSVersion` 已解析进来，
     但**没有执行**——宿主 OS 版本还没有采集点（macOS `sw_vers -productVersion`、Windows build 号）。
   - 验收：新增宿主 OS 版本探测（可注入，三平台纯函数），`Manifest.EffectiveProvider`/一个新方法在
     命中 override 且宿主版本低于下限时拒绝（安装期，与 `ValidatePlatform` 同位置）；测试
     「低于下限拒绝 / 等于即通过 / 无声明不查 / 版本读不到时不拒绝」；越界时 `prepare`/`install` 不留 staging。

> 至此具体待办里的 1–3 **三条都卡在框架能力上**（`Options.OnCopy`、终端键入的 `HandleInput`
> 缝、外观旋钮的暴露），不是应用侧能单方面推进的；第 4–8 条已全部完成（第 8 条在 P7-ba 落地）。
> 下一轮仍应回到「对照原版排查偏离点」这条路。

### 具体待办（P7-bb 重写；两项已在 P7-bc 落地，本条清单已空 → 下一轮回到「对照原版排查偏离点」）

1. **~~集成行的 freshness 点（R7-7b）~~**：已完成（P7-bb）——`Freshness.DotState()` 三态投影 +
   名字旁的点（颜色=态、label/tooltip 同句）；`FreshnessOf` 补上「探过即 success」的推导。
   （原分析保留在上面的 P7-bb 记录里。）
2. **~~漂移通知门（R7-7 的 `createReprobeNoticeGate` / `decideDriftNotice`）~~**：已完成（P7-bc）——
   `internal/extensions/driftnotice.go` 的 `DriftNoticeGate`/`ReprobeNoticeKey`/`DecideDriftNotice`
   复刻三条**有顺序**的规则；**纯半**已就位。剩余（需先决策，不算本轮待办）：是否移植后台重探调度器，
   移植后把 `DecideDriftNotice` 接在 shell 唯一一个通知点（一帧一 toast）。
3. **~~freshness 明细的相对时间本地化~~**：已完成（P7-bc）——`humanAge` 换成 `relativeAge` + `freshnessWhen`
   （i18n 的 `FreshnessRelative`，en/zh 逐字对齐 `Intl.RelativeTimeFormat` 的短词形，含「yesterday」
   「前天」「上个月」「去年」）；45s 阈值回到旧版的 `< 45s`。

### 具体待办（P7-bd 重写；第 1、2 条已落地）

> P7-bb 的清单已全部落地、P7-an 的 1–8 只剩一次性维护者动作，所以 P7-bd 先把「尚未做」重新写实。
> 每条都带 file:line、原版对照与验收标准，供后续轮次逐条取用。第 1 条（P7-bd）、第 2 条（P7-be）
> 已实现；下一轮从第 3 条开始，第 3、4 条互相依赖，宜连着做。

1. **运行失败的「键 + JSON 负载」与逐语言渲染（P7-bd 已落地）**
   - 原版：`src-tauri/src/extensions/run_error.rs`——后端只发**稳定键 + JSON 负载**的字符串
     `key:{json}`（`program_missing` / `program_not_executable` / `spawn_failed` / `timed_out` …），
     负载字段是 `path`/`language`/`names`/`searched`/`detail`/`seconds`/`extension_id`/`reason`/`error`；
     `spawn_failed`（`run_error.rs:135`）把 `NotFound` 映射成 `run_program_missing`、`PermissionDenied`
     映射成 `run_program_not_executable`，其余保留 OS 原文进 `run_spawn_failed`；`run-errors.ts` 的
     `parseRunError`（保守：键必须 `run_` 起头、负载必须是 JSON 对象、未知键不认）、`runErrorMessage`，
     `src/i18n.ts:751-764` 的 10 条 `settings.extensions.runError*`（en/zh）。
   - 本构建：`internal/extensions/run.go:133` 超时返回 `fmt.Errorf("extensions: %s timed out after %s")`、
     `run.go:144` 生成失败返回 `&ProviderError{}`（`provider.go:104` 一律英文 prose）；`internal/launcher/output.go:80`
     的 `RunStatus` 与 `output.go:132` 的 `view.Text = err.Error()` 直接把 prose 交出去；`grep runError
     internal/i18n` 为空——10 个键一个都没有。
   - 验收：新增 `internal/extensions/runfailure.go`（纯：`RunErrorKey` 常量集、`RunFailure{Key,Payload}`、
     `Error()` 发 `key:{json}`、`ParseRunFailure` 复刻保守解析、`RunSpawnFailure` 复刻 OS 错误→键的映射）；
     `runCaptured` 的超时与 spawn 失败改为发键；launcher 在输出视图文本与一行状态里按语言渲染，未识别的
     消息**原样返回**；`i18n.Settings` 新增 10 个 `RunError*` 句子（en/zh 逐字对齐旧字典）；测试钉解析
     （含 `run_param_required:note` 不认）、OS 错误映射、en/zh 各一句。
2. **后台运行完成通知的时长词形与输出行数（P7-be 已落地）**
   - 原版：`src/extensions/run-routing.ts:41` 的 `formatRunDuration`（<1000ms → `420 ms`，否则 → `3.4 s`，
     非有限/负 → `0 ms`）、`:99-111` 的 `runOutputSummary` + `runOutputLineCount`（两个流按序拼接、
     一个尾换行不额外算行、恰好一个换行仍是一行、空捕获是 0 行、截断时尾巴换成 `truncated at 64 KB`）；
     `src/ExtensionsPanel.tsx:1759-1775` 的成功 toast 串成 `{name} finished in {duration} · {lines} lines of output`；
     `src/i18n.ts:740-749` 的四个键。
   - 本构建：`internal/launcher/output.go:316-318` 只 `fmt.Sprintf("%.1fs")`；`internal/shell/shell.go:2204`
     的通知是 `CommandFinished(name) + " · " + CommandStatus(status)`，没有时长词形也没有行数；i18n 无四个键。
   - 验收：新增纯函数 `FormatRunDuration`/`RunOutputLineCount`/`RunHasOutput` 与 i18n 四键，成功通知按原版
     串法（失败通知不带行数）；测试钉 `<1000/≥1000/NaN/负` 与 `""`/`"\n"`/`"a\nb"`/尾换行 边界。
3. **~~面板手动 Run 按 manifest 的 `output` 路由分流~~**：已完成（P7-bf）——`runWithParams` 读
   `Manifest.Route()`，`terminal` 走 `runCommand`（原状），`background` 走 `runCommandFromPanel`
   （`runCommandCaptured` 跑完把 `CapturedRun` 写进 `Settings.SetRunResult`）；测试
   `TestPanelRunFollowsTheManifestOutput` 钉两种路由各走各的。（原分析保留在下方。）
   - 原版：`src/ExtensionsPanel.tsx:1746-1795`——`outcome.route === "terminal"` 才打开终端，否则把捕获的
     output 存进该行、成功时 toast（带 `customViewOutput`）；`run-routing.ts:52` 的 `DEFAULT_OUTPUT_MODE`。
   - 本构建（改前）：`internal/settingsui/settingsui.go:2641` 的 Run 按钮 → `shell.go:1769` 的 `runWithParams`
     **无条件** `a.runCommand(entry, argv)`（`shell.go:2158` 直接 `Terminal.RunCommand` + `Open(SurfaceTerminal)`），
     完全忽略 manifest 的 `output`——`background` 的集成在面板里点 Run 也开终端。
4. **~~集成行的「最近一次输出」块~~**：已完成（P7-bf）——`internal/settingsui/settingsui.go` 的 `runOutput`
   在卡片里画可折叠、固定高度滚动的输出块：折起一行 `View output`，展开是头部（`Last output` +
   截断标记 + `Hide output`）、`stdout+stderr` 正文（空则 `(No output)`）、尾部截断行；失败运行立即展开。
   六条 i18n（en/zh 对齐旧字典）。测试 `TestRunOutputBlock` 钉四态。（原分析保留在下方。）
   - 原版：`src/extensions/ExtensionRow.tsx:480-520`——可折叠、固定高度滚动的输出块，空输出有
     `customNoOutput`、截断在头部有 `customOutputTruncated`、尾部有 `customOutputTruncatedTail`，
     隐藏按钮 aria 是 `customHideOutput`。
   - 本构建（改前）：`internal/settingsui/settingsui.go` 的卡片里没有输出块；`grep -i lastOutput internal/i18n` 空；
     后台运行的 `CapturedRun`（`extensions/run.go:28`）根本不进面板（只进 launcher 的输出视图）。
5. **~~不可用运行时的失败原因本地化（R11）~~**：已完成（P7-bg）——`ResolveRuntime` 的失败变键+负载
   （`internal/extensions/binding.go`），探测把 `lastErrorCode`/`brokenReason` 写进条目
   （`internal/extensions/probes.go`），卡片在状态行下画 `Failure detail` 并按语言渲染
   （`internal/settingsui/failure.go`），认旧英文句、认不出的 prose 原样保留。（原分析保留在下方。）
   - 原版：`src/extensions/binding-errors.ts`——后端发 `binding-changed:{"path":…}` /
     `binding-missing:{"path":…}`，`failureReason(code, detail, t)` 本地化并认旧版英文句（`Executable
     fingerprint changed at …`）以迁移已落盘记录；`src-tauri/src/extensions/error_codes.rs` 的
     `ProviderErrorCode`（`binding-missing`/`binding-changed`/…）；`src/i18n.ts` 的
     `bindingChangedDetail`/`bindingMissingDetail`/`errorCode.*`。
   - 本构建：`internal/extensions/provider.go:37` 的 `ResolveRuntime` 返回 prose（`is not on the search path`
     / `no %s interpreter is available`），既没有键也不落进 repository 的 `lastErrorCode`/`brokenReason`；
     `internal/settingsui/settingsui.go:1126,1425` 只画状态词（`IntegrationsBroken`），`BrokenReason` 全仓无人读
     （`grep -rn BrokenReason internal/settingsui` 空）。
   - 验收：`ResolveRuntime` 的失败变键+负载，健康检查/探测失败时写 `lastErrorCode`+`brokenReason`；卡片状态行
     渲染本地化原因（认旧英文句，认不出则保留原句）；测试钉键与两语言渲染。

> 下一轮：第 5 条已由 P7-bg 落地，P7-bd 的清单至此 1–5 全部完成。**下一轮先把「尚未做」重新写实**
> （逐条 file:line、原版对照、验收标准），再取第 1 条实现。

**P7-bf 面板手动 Run 按 `output` 分流 + 「最近一次输出」块（本轮）**

- **依赖检查**：`curl .../releases/latest` 返回 v0.4.8，`go.mod` 已是 v0.4.8，无版本可升。
- **偏离点**：旧版 `ExtensionsPanel.tsx:1746-1795` 按 `outcome.route` 分流；本构建的 `runWithParams`
  （`internal/shell/shell.go:1769`）**无条件**开终端，且 `ExtensionRow.tsx:480-520` 的「最近一次输出」块
  完全没有对应物——`CapturedRun` 只进启动器的输出视图。
- **修法**：`internal/settingsui/settingsui.go` 的 `RunResult` + `SetRunResult`/`LastRun` + `runOutput`
  （折起/展开、固定高度滚动、四态）；`internal/shell/shell.go` 的 `runWithParams` 按 `Manifest.Route()`
  分流，新增 `runCommandFromPanel`；`internal/i18n` 补六条。
- **测试**：`TestPanelRunFollowsTheManifestOutput`（shell）、`TestRunOutputBlock`（settingsui）。
- **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 27 包全绿（823 项）、
  `GOOS=linux/windows/darwin go build ./...` 三平台通过。

**P7-at manifest 的 `compatibility` 区间在落盘前生效（本轮）**

- **依赖检查**：`go.mod` 已是 v0.4.7，`curl .../releases/latest` 与 go proxy 都确认 v0.4.7 仍是最新，
  无版本可升；本轮走「对照原版排查偏离点」，取本轮写实的第 6 条。
- **偏离点（解析了、没人查）**：`Manifest.Compatibility`（`internal/extensions/manifest.go:66`）的
  `Floter` 与 `ProviderProtocol` 两个字段只有 `json.Unmarshal` 读入与 `custom.go:409` 写入，
  **没有任何校验调用者**；`Satisfies`（`semver.go:138`）这套区间匹配只服务 registry 选版本。
  于是声明 `floter: ">=99.0.0"` 或 `providerProtocol: "^2.0"` 的包照装照跑。
- **原版对照**：`manifest.rs:533` 的 `validate_compatibility`——host 版本剥离 pre-release 后匹配
  `compatibility.floter`，provider 协议固定 `1.0` 匹配 `compatibility.providerProtocol`；
  install（`install.rs:2522`）与 pre-install review（`extensions.rs:1456,1477`）各跑一次。
  它**不在**后续 load/refresh 上跑——已装的包不会因 host 升级而失效，本轮同样只在 install 期查。
- **修法**：
  - 新文件 `internal/extensions/compatibility.go` 的 `Manifest.ValidateCompatibility(hostVersion)`：
    空/不可解析的 host 版本**跳过**（「一个说不清自己版本号的构建没有立场拒收」）；preview host
    剥掉 pre-release 后按 release 行匹配（旧版同规则）；`floter` 与 `providerProtocol` 各自空区间即无约束。
  - `Paths` 新增 `HostVersion`（`paths.go`）；`shell.New`（`shell.go:385`）在发现路径后从
    `appVersion()` 注入（测试注入的值不被覆盖）。
  - `prepare()`（落盘前）与 `install()`（真正写入前）各调用一次；路径都在错误分支清掉 staging。
- **测试**：`internal/extensions/compatibility_test.go`——`TestValidateCompatibility` 的 12 行表
  （越界、交集、`||`、协议版本、空区间、空/不可解析 host、preview host、不可解析区间）；
  `TestPrepareRefusesAManifestBuiltForAnotherRelease`（拒绝且不留 staging）、
  `TestInstallLocalRefusesAnIncompatibleProtocol`、
  `TestAHostVersionLetsACompatibleManifestThrough`（注入 host 版本不会误伤正常安装）。
  变异核查：把 `prepare`/`install` 里的 `ValidateCompatibility` 调用删掉，前两条立刻红。
- **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 27 包全绿、
  `GOOS=linux/windows/darwin go build ./...` 三平台通过。

**P7-as manifest 的 `output` 路由终于到达启动器（本轮）**

- **依赖检查**：`go list -m -versions` 确认 v0.4.7 仍是最新（`go.mod` 已是 v0.4.7），无版本可升；
  GitHub releases 被限流，用 go proxy 核对。本轮走「对照原版排查偏离点」。
- **偏离点（死字段的化石）**：`Manifest.Route()`（`internal/extensions/manifest.go:51`）有实现、
  有测试（`run_test.go:121` 的 `TestManifestRoute`），但**从来没有调用者**；构造 `CommandEntry` 的
  `CommandEntries()`（`store.go:299`）**从不写 `Route`**。而 `Route` 是启动器读的字段：
  `search.go:183` 的 `direct`（行尾 `␣` 键帽）与 `search.go:196` 的 `runCommandWith`（决定输出去
  terminal 还是 captured 视图）。于是**清单里写 `output: "background"` 的命令永远走终端**，
  `␣` 键帽从不出现；P3-h 自己写的设计（plan:408「`CommandEntry.Route` 带上它」）在实现里断成两截。
  测试把它掩盖了：`view_test.go:1277`、`filters_test.go:923`、`shell_test.go:1509` 都手动
  `entry.Route = RouteBackground` 再断言，绕过了生产路径。
- **原版对照**：`src-tauri/src/extensions/run.rs:420` 的 `resolve_route(manifest) = manifest.output.into()`
  ——「The manifest's declared `output` mode is the only authority」，路由只有一个来源。
- **修法**：`CommandEntries()` 构造条目时补 `Route: integration.Manifest.Route()`。零新 API。
- **测试**：新文件 `internal/extensions/route_test.go` 的 `TestCommandRouteFollowsTheManifestOutput`
  （一个声明 `output: "background"` 的集成、一个不声明的集成，各出一个命令，核对 Route）；
  改写 `internal/shell/shell_test.go` 的 `TestCapturedCommandReachesTheLauncher`：把
  `output: "background"` 写进**清单文件**（不再手写 `entry.Route`），断言启动器收到的 entry **自己**
  带 `RouteBackground`——manifest → CommandEntries → launcherCommands → SetCommands 整条链一次钉住。
  变异核查：`git stash` 掉 `store.go` 那一行，`route_test.go` 两条断言与 shell 那条立刻红。
- **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 全绿、
  `GOOS=linux/windows/darwin go build ./...` 三平台通过。
- **顺带记下的已知差异（本轮不改）**：schema 的 `output.default` 是 `"background"`
  （`docs/extensions/schemas/floter-extension.schema.json:225`），而 Go 的 `Manifest.Route()` 缺省
  terminal（plan P3-h 的选择，已被 `TestManifestRoute` 钉住）。缺省取向是独立的决定，改它会翻转所有
  未声明命令的落点，留待单独一轮。**已在 P7-ay 落地**：`Route()` 缺省改回 `background`，
  `TestManifestRoute` 与 `route_test.go` 一并翻转。

**P7-ar `cwdPolicy` 的两种拼写都进得来（本轮）**

- **依赖检查**：`curl .../releases/latest` 确认 v0.4.7 仍是最新（`go.mod` 已是 v0.4.7），无版本可升；
  本轮取具体待办里唯一不卡框架的第 5 条。
- **偏离点**：`Launch.CWDPolicy` 是 `string`（`manifest.go:143`）——schema 合法的对象形
  （`{"policy":"projectRoot",...}` / `{"policy":"fixed","path":"/abs"}`，见
  `docs/extensions/schemas/floter-extension.schema.json:380-440`）会被 `json.Unmarshal` 直接拒收，
  于是**整份清单读不进来**（`LoadManifest` 报 `cannot unmarshal object into ... type string`）。
  旧的 `FixedPath()` 只认 Go 自己发明的 `"fixed:<abs>"` 字符串，对象形永远读不到。
- **修法**：新文件 `internal/extensions/cwdpolicy.go`。
  - `CWDPolicy` 类型（`Kind` / `Markers` / `MaxDepth` / `Path`）带 `UnmarshalJSON`：字符串拼写
    （`inheritActiveSession` / `toolData` / `home`，以及向后兼容的 `fixed:<abs>`）与对象拼写
    （`projectRoot` 带 `markers`/`maxDepth`、`fixed` 带 `path`）都能进；`MarshalJSON` 让本构建写的
    清单（自定义工具）原样往返，未设策略仍写空串（`declared` 判定不变）。
  - `fromObject` 复刻原版 `CwdPolicy::from_manifest`：缺 `policy`、`fixed` 相对路径 / 空路径 / 含 NUL 都是
    加载期错误；`projectRoot` 缺省 markers 用原版的五个（`.git`、`Cargo.toml`、`go.mod`、`package.json`、
    `pyproject.toml`）、缺省 `maxDepth` 32。
  - `Resolve(CwdContext)` 复刻原版 `CwdPolicy::resolve` 的回退链：`inheritActiveSession` = 会话 cwd（是目录）
    → 项目根（默认 markers）→ 数据目录 → home；`projectRoot` = 项目根（自带 markers/maxDepth）→ 会话 cwd
    → 数据目录 → home；`toolData` 先建目录；`fixed` 直接给声明的绝对路径；`home` 给主目录。
    `findProjectRoot` 从会话 cwd 向上走至多 `maxDepth` 层找任一 marker。
  - `store.go` 的 `launchOverrides` 改为交给 `config.CWDPolicy.Resolve(CwdContext{DataDir, Home})`；本构建
    仍无「活动会话 cwd」，所以 `inheritActiveSession`/`toolData`/无会话的 `projectRoot`/缺省都落到集成的数据目录
    （原版回退链的同一落点），数据目录照旧先 `MkdirAll`。`FixedPath()` 删除。
- **测试**（新文件 `cwdpolicy_test.go` + 改写 `launch_test.go` 的 `TestLaunchFixedPath` 为 `TestLaunchFixedPolicy`）：
  两种拼写的合法解析与默认值、非法对象（缺 policy / 相对 fixed 路径 / 未知词 / 非字符串非对象）报错；
  `TestCWDPolicyResolve` 的回退链表；`TestProjectRootWalksUpFromSessionCwd` 与 `TestFindProjectRoot`
  用**注入的 cwd** 验 marker 行走与 `maxDepth` 截断；`TestObjectCwdPolicyRefreshes` 让两种对象形夹具真的
  `Refresh` 出命令并核对 `Dir`；`TestInvalidCwdPolicyFailsTheManifest` 钉 `LoadManifest` 报错。
  变异核查：把 `fromObject` 的 `fixed` 校验去掉、或让 `Resolve` 的 `projectRoot` 跳过 `findProjectRoot`，
  对应用例立刻红。
- **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 27 包全绿
  （`go test ./... -v | grep -c '^=== RUN'` = 714）、`GOOS=linux/windows/darwin go build ./...` 三平台通过。

**P7-ao 死文案清理 + `TestNoDeadCopy` 守卫（本轮）**

- **依赖检查**：`go.mod` 已是 v0.4.7（P7-an 升的），`curl .../releases/latest` 确认 v0.4.7 仍是最新；
  无版本可升。本轮取具体待办里**唯一不卡框架**的第 4 条（1–3 都要框架先给钩子/旋钮）。
- **删除的 10 个死键**（`internal/i18n/i18n.go` 的声明 + `copy.go` 的 en/zh 值）：`BrowserCDP`、
  `BrowserHistoryAll`、`CalculatorPluginHint`、`OpenFilesFailed`、`PagePlugins`、`PagePluginsHint`、
  `PowerConfirmButton`、`PowerConfirmDetail`、`PowerConfirmTitle`、`ShortcutsAction`。
  三组来源：`PowerConfirm*` 是 P7-d 改回面板内确认行后的遗留（现用 `PowerConfirmMessage` +
  动作自己的词）；`PagePlugins*` 是 P7-u 删掉内置插件页后的遗留；其余是随视图重写停用的旧行。
- **守卫**：新文件 `internal/i18n/deadcopy_test.go`。
  - `copyLeafNames`：反射走 `Copy` 树，收下每个 `string` / `func` 叶子的名字（结构体与结构体切片
    递归，去重排序）。
  - `selectedNamesOutsideI18n`：`go/parser` 解析 `cmd/` 与 `internal/` 下全部 `.go`（跳过
    `internal/i18n`——声明自己不算读者），收集每个 `SelectorExpr` 的 `Sel.Name`。
  - `moduleRoot`：从测试工作目录上溯到 `go.mod`，扫描不依赖调用者 cwd；扫不到任何 selector
    会直接 `Fatal`，不会静默通过。
  - **方向是保守的**：同名于其他结构体的字段读作「在用」——守卫可能漏掉一个死键，但**不会**
    误报一个活键。
- **测试**：`TestNoDeadCopy`（本体）。变异核查：往 `Copy` 里加一个没人读的 `ZzzDeadProbe string`，
  测试立刻红（`i18n.Copy.ZzzDeadProbe is declared but never read outside internal/i18n`）；
  删掉后绿。
- **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 27 包全绿
  （700 项）、`GOOS=linux/windows/darwin go build ./...` 三平台通过。

**P7-ap 别名说清自己命中了什么 + 扩展命令的别名真的可达（本轮）**

- **依赖检查**：`go.mod` 已是 v0.4.7（P7-an 升的），无版本可升；本轮走「对照原版排查偏离点」。
- **偏离点（两处，同一个功能的两半）**：
  - 旧版被别名命中的行会在副标题前置 `launcher.aliasMatch`（`src/hooks/useLauncherCatalog.ts:1344-1350`），
    且**只在别名得分严格高于命令名时**出现（`candidateMatchScore(needle, alias) > candidateMatchScore(needle, command)`，
    `src/command-aliases.ts:39` 的 exact 1000 > prefix 800-罚分）：名字本身就是答案时不画蛇添足。
  - P4-y 的别名编辑器把别名写进 `command_aliases`（键是**命令 id**），但启动器只把别名喂给 PATH 命令行
    （`toolItems` 的 `ToolAliases[tool.Name]`），扩展命令的别名**存了却永远匹配不到**——编辑框承诺的能力
    从未接线。
- **修法**：
  - 别名升格为**目录级事实**：`App.Aliases` + `SetCommandAliases`（`SetTools` 不再携带它），`scanTools`
    在 PATH 开关判定**之前**先把别名交给启动器，所以关掉 PATH 行也照样能用别名找到扩展命令；
    `commandItems` 按 `command.ID` 取别名（`jv` → `j`）。
  - `Match` 记录真正命中的别名（`Item.matchedAlias`），`aliasHint`/`aliasRank` 实现旧版「别名严格更好才说」
    的比较（exact 压 prefix，其余沿用 `candidateScore` 的梯子）；`matchedCatalog` 把 `copy.AliasMatch`
    织进副标题（`别名：j · 描述`）。措辞归应用（`i18n.AliasMatch`，en/zh 旧版原文），排名只回答「是不是别名」。
- **测试**：`view_test.go` 的 `TestToolAliasesJoinTheSearch`（`gfm` 命中并写 `Alias: gfm`、`gf` 前缀同样写、
  `git` 命中不写）与新增 `TestExtensionCommandAliasesJoinTheSearch`（命令 id 别名可达 + 副标题前缀）；
  `shell_test.go` 的 `TestCommandAliasFlow` 追加「别名到达启动器、清空后消失」，钉住别名与 PATH 开关解耦。
- **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 27 包全绿
  （701 项）、`GOOS=linux/windows/darwin go build ./...` 三平台通过。

**P7-aq 启动的工作目录与终端要求按清单执行（本轮）**

- **依赖检查**：`curl .../releases/latest` 确认 v0.4.7 仍是最新（`go.mod` 已是 v0.4.7，P7-an 升的），
  无版本可升；本轮走「对照原版排查偏离点」。
- **偏离点（两处，都在 `lifecycle.launch` 上）**：
  1. **`execution.workingDirectory` 是行为词，不是路径**。原版 `provider.rs:655` 把它映射成
     `current → 启动自己的 cwd`、`home → 用户主目录`、`inherit → 不设 Dir（随宿主）`。本构建
     `internal/extensions/store.go:315` 把 `"current"`/`"home"`/`"inherit"` 原样写进
     `CommandEntry.Dir`，于是后台运行（`run.go:101` 的 `cmd.Dir = entry.Dir`）在名为 “current” 的
     目录上 chdir 失败，而终端路径只是靠 `internal/terminalui/terminalui.go:209` 的 `isKeywordDir`
     **把词丢掉**——顺带把 `home` 策略也丢了（会话不是从主目录起，而是从框架默认位置）。
     这个 bug 还被 `store_test.go` 的 `entry.Dir != "current"` 钉着，`shell_test.go:1512` 则
     到处手动 `entry.Dir = ""` 绕开它。
  2. **`terminal.color` 从未被读**。原版 `launch.rs` 的 `terminal_environment` 按 `none`/`8`/`256`/其它
     决定 `TERM`/`COLORTERM`；本构建无论清单怎么写都固定 `floter-256color`+`truecolor`，
     清单里的 `terminal` 块根本没进 Go 结构。
- **修法**：
  - 新文件 `internal/extensions/workingdir.go` 的 `WorkingDirectory(mode, launchDir, home)`：
    `home → home`（找不到就是空 = 继承）、`inherit → ""`、其余（含 `current` 与未知词）→ 启动的
    cwd。两条路由现在同一个答案。
  - `store.go` 的 `launchOverrides` 改成返回 `launchDefaults`，并让**默认策略落到集成自己的数据
    目录**：原版在没有活动会话 cwd 时走的正是 `InheritActiveSession → toolData → home` 这条回退
    链，`run.rs` 的 `run_cwd` 也明说「tool data 就是 run 的 cwd，写在旁边的脚本不能写进宿主的 cwd」；
    并且那里会先 `create_dir_all`（cwd 不存在会让 spawn 失败），本构建同样先建。
  - `manifest.go` 的 `Launch` 增加 `Terminal Terminal`（只读 `color`），`terminalEnvironment(color)`
    复刻旧版四项映射；`launch.declared` 决定要不要带上这套 env（旧版只要清单声明了 launch 就带）。
- **测试**：`workingdir_test.go` 的 `TestWorkingDirectory`（11 行表，含大小写、空白、未知词、home 缺失）、
  `TestTerminalEnvironment`（7 行表）、`TestDescriptorWorkingDirectory`（一个 provider 四条命令：
  `current` 落到数据目录、缺省同 `current`、`home` 落到主目录、`inherit` 为空；并核对 `terminal.color:"none"`
  真的给出 `TERM=dumb`/`COLORTERM=""`）；`launch_test.go` 的 `TestNoLaunchOverrides` 追加默认 cwd 的断言；
  `store_test.go` 的那条错断言改成数据目录。变异核查：把 `WorkingDirectory(...)` 换回
  `command.Execution.WorkingDirectory`，`TestDescriptorWorkingDirectory` 四条断言全红（`here dir = "current"`）。
- **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 27 包全绿
  （`go test ./... -v | grep -c '^=== RUN'` = 709）、`GOOS=linux/windows/darwin go build ./...` 三平台通过。

**P7-an 命令行的两个接缝、粘贴边界与拖放上限（本轮）**

- **依赖检查**：`proxy.golang.org` 显示 v0.4.7（`go list -m -versions` 走不通时用得上）。
  release notes 逐条评估（`#275–#287`）：`Bitmap.Pixelated`、`Painter.ImageRegion`、
  `Painter.Transform`、`ui.Canvas`、`examples/farm`、`game` 插件的 gamepad / `game.View` 固定步长 /
  `game.Camera`、`Player.SetLoopRange`——全是**游戏与精灵图**向的能力（离屏画布、图集切片、
  变换、逐帧循环）。floter 没有一帧里反复重画而“很少变”的层（玻璃是插件的 shader，文本/图标
  是框架自己的缓存），也没有精灵图；像素化放大只会让照片式的应用图标更糟。**无自然用点，只升级**
  （`go.mod` → v0.4.7，`plugins/terminal` 与 `ui` 的 API 面未变）。
- **本轮是第 3 条（对照原版排查）**，一次找到四类偏离，全部实现。

1 · **带 shell 语法的命令行从没交给 shell**（功能性 bug）
- **原版**：`src/hooks/useLauncherActions.ts:262-272` 的 `runCommand` 把**行本身**（`commandLine.trim()`）
  交给终端，由交互式 shell 读它；结构化 argv 只用于目录里带 execution plan 的行。
  `src/launcher.ts:199` 的 `shellSyntax` 正是为区分这两条路而存在。
- **本构建**：`internal/launcher/commandline.go:30` 算出了 `ShellSyntax`，但**全仓零读取**
  （`grep -rn ShellSyntax` 只有它自己的声明与赋值）。`internal/launcher/actionbar.go` 的 shell 行
  一律走 argv，于是 `ls -la | grep go` 变成 `exec ls "-la" "|" "grep" "go"`——管道不是管道。
- **修法**：`Actions` 新增 `RunInShell func(line string)`（`internal/launcher/launcher.go:141`），
  `actionbar.go` 在 `parsed.ShellSyntax` 时返回「把这行交给用户自己的 shell」的行；shell 侧接到
  `a.openInstallSession`（`internal/shell/shell.go:520`）——就是安装行用的那条「开一个裸会话并打进命令」
  的路。历史仍记用户打的原行（不是第二个规范化点）。
- **测试**：`view_test.go` 的 `TestShellSyntaxRunsTheLineInAShell`（`ls -la | grep go` → `RunInShell`
  拿到整行、`RunInTerminal` 零调用；`git status` → 仍是 argv 两段）、
  `TestShellSyntaxRowKeepsItsLineAndItsHistory`（历史留原行）、shell 的
  `TestAShellSyntaxLineOpensTheTerminalSurface`。把 `if parsed.ShellSyntax` 换成 `if false`：两条红。

2 · **命令行的 Unicode 空格：`go<U+3000>version` 是一个词**
- **原版**：`src/terminal/inputNormalize.ts` 的 `normalizeTerminalInputSpaces`——shell 只把 U+0020
  当词缝，其余 16 个 Zs（含中文输入法的全角空格 U+3000）会把两个词粘成一个 token，zsh 报
  `command not found: go version`。原版在**字符串变成 PTY 字节**的那一处统一规范化。
- **本构建**：`internal/launcher/commandline.go` 的 tokenizer 按**字节**只认 `' '`/`'\t'`，
  于是 `go<U+3000>version` 成为单个 token 进 argv；`terminalui.RunShellWithCommand` 原样 `Send`。
- **修法**：新包 `internal/inputtext`（`NormalizeSpaces` + `StripPasteNewline`，干净串原样返回不分配）；
  `ParseCommandLine` 第一行就规范化（结构化路径的唯一接缝）；`RunShellWithCommand` 规范化 + trim
  （`shellCommandLine`，字符串路径的唯一接缝）。
- **测试**：`internal/inputtext` 的 16 个 Zs 逐行表 + 「表与 `unicode.Zs` 互为全集」+ 非 Zs
  （tab/换行/U+200B/U+2028/U+2029/U+0085/U+180E）直通；`commandline_test.go` 的
  `TestUnicodeSeparatorsAreWordGaps`（含引号内、env 前缀、管道旁）、
  `TestTheTokenizerNormalizesThroughOneFunction`；`terminalui` 的 `TestShellCommandLine`。
  把规范化那一行删掉：`ideographic space: tokens = ["go<U+3000>version"]` 红。

3 · **粘贴边界（`terminal_paste_safe` 一直是死的）**
- **原版**：`src/hooks/useTerminalView.ts:917` 的 `isPaste && pasteSafe ? stripPasteNewline(text) : text`
  再规范化——粘进来的命令自带「当初运行它的那个换行」，不拿掉就会在用户读到之前执行；
  开关是 `src/settings/TerminalAppearance.tsx:340` 的 `terminal_paste_safe`（`page: "strip"`）。
- **本构建**：`Options.OnPaste`（mygo `plugins/terminal/terminal.go:116`，**v0.4.6 就有**）从未被设过，
  所以这个设置既不显示也不起作用——旧文档把它记成「框架没暴露」，是错的，本轮订正。
- **修法**：`internal/terminalui/terminalui.go` 的 `startSession` 给每个会话装 `OnPaste`：
  `pasteText(text, safe)` = 安全粘贴时 `StripPasteNewline` + 无条件 `NormalizeSpaces`，
  有改动才接管（用终端自己的 `Paste` 送，bracketed paste 仍是插件的），没改动就交回框架的原路。
  设置**每次粘贴现读**，所以抽屉里拨一下对正在显示的会话立即生效。
  抽屉补一行（`internal/control` 新轴 `AxisPasteSafe`，抽屉专属；`internal/terminalui/drawer.go`
  的行；i18n `Settings.PasteSafe` = 旧版原文 `Safe paste` / `粘贴不执行`）。
- **测试**：`internal/terminalui/paste_test.go` 的 `TestPasteText`（10 行表）、
  `TestEverySessionCarriesThePasteBoundary`（每个会话都装 hook；关时只规范化、开时连尾换行一起拿）、
  `TestSafePasteAndSeparatorsAreOneBoundary`；`control`/`settingsui`/`terminalui` 三处 schema 测试
  随之要求新轴（卡片必须**不**画它）。把 `pasteText` 调用换成 `text, false`：两条红。

4 · **拖放超过 5 个文件不再埋掉列表**
- **原版**：`src/launcher/file-drops.ts:64` 的 `MAX_VISIBLE_DROPPED_FILES = 5` 与 `fileDropRows`
  的「…and N more」行——它**是一行普通结果**（方向键与编号快捷键都够得到），运行它只展开、
  不碰任何文件；`useFileDrops` 在新一次拖入时把展开状态收回。
- **本构建**：`internal/launcher/files.go` 的 `filesItems()` 无上限、无展开行——拖一个文件夹进来
  就是几百行。
- **修法**：`MaxVisibleDroppedFiles = 5` + `dropsExpanded` 状态（`SetDropped`/`enterFiles` 都收回）；
  按**文件**计数（行数是一个文件 ×3 的既有设计差异），隐藏数 >0 时追加一行 `drop:more`
  （`Run` 只置 `dropsExpanded = true`）。针过滤仍在**行**级（`fileRows`），所以 `copy the path`
  这类词照旧只留下对应的行。i18n 补 `FilesMore(count)` / `FilesMoreHint`（旧版原文）。
- **测试**：`view_test.go` 的 `TestALargeDropCollapsesBehindOneRow`（第 6 个文件不列、展开行是列表
  最后一行、运行后 8×3+1 行且仍在模式里）、`TestTheDropExpanderFollowsTheDrop`（新拖入/重进模式
  都收回展开）、`TestTheDropCapCountsFilesNotRows`（`…and 2 more files` 数文件、针仍按行动作过滤）。
  把上限那三行删掉：三条全红。

- **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 27 包全绿
  （699 项）、`GOOS=linux/windows/darwin go build ./...` 三平台通过。

**P7-am PATH 命令行的来源词与路径（本轮）**
- **依赖检查**：`go list -m -versions github.com/egoist/mygo` 确认 v0.4.5 仍是最新（go.mod 已是
  v0.4.5），无版本可升；本轮走第 3 条（对照原版排查偏离点）。
- **偏离点**：旧版目录给每条 PATH 命令行两个行级事实（`src-tauri/src/extensions/catalog.rs` 的
  `system_command_entries`，约 :1033）：`source_name: "PATH"`（与 provider 集成的 `source_name`
  同一字段）与 `description: path.to_string_lossy()`（可执行的完整路径）。Go 侧
  `internal/launcher/search.go` 的 `toolItems()` 两样都丢：行只有 `Title` 与 `alias`，来源词槽
  （P7-z 建的 `Item.Source`）对 PATH 命令从不出现，`/usr/bin/git` 与
  `/opt/homebrew/bin/git` 画的行一模一样。
- **修法**：`toolItems()` 补上 `Detail: tool.Path` 与 `Source: pathSourceWord`（新常量 `"PATH"`，
  旧版原文）；路径同时是可搜索文本（`Detail` 本就在 `itemScore` 的搜索路径里），所以查询能经
  路径里的目录词到达命令——同目录词不再把行顶出去。P7-z 的行级来源槽与右端成组对这批行
  自动生效，无需新视图代码。命令模式补全行的来源（`argumentItems`）核对过：旧版给它的也是
  所属集成的名字（`entry.sourceName`），不是 PATH——保持原状，未改动。
- **测试**：`search_test.go` 的 `TestToolRowNamesItsSourceAndItsPath`（来源词、路径第二行、路径
  可达搜索）、`TestToolRowsFollowTheSetting`（开关关掉时行与来源词都不画）；`view_test.go` 的
  `TestThePathRowNamesItsSourceOnScreen`（画出来的行：source 在标题右外侧、路径文本在）。
  把 `Source: pathSourceWord` 与 `Detail` 摘掉各跑一遍，对应断言全红。

**P7-v 字段自己的两个键（本轮）**
- **根因**：旧版有两个手势是**搜索字段自己**回答的——`CmdOrCtrl+Backspace`（⌘⌫ / Ctrl+⌫）装填并
  确认选中历史行的两步删除，`Ctrl+Shift+Backspace` 清空输入过的命令历史。本构建把这两个键都记成
  「框架不可达」：框架的 `keyDownPhysical` 顺序是「聚焦元素的 `HandleInput` → 编辑器的 `wants` →
  快捷键注册」，而 `editor.wants` 对带修饰键的 Backspace 一律返回 true，且编辑器那一支**没有**
  `claimed` 守卫——所以 `c.Shortcut` 永远够不到它。删除的能力此前只留在行尾的 ✕ 控件上，命令历史
  则**根本没有清空的方式**。
- **修法**：新文件 `internal/launcher/fieldkeys.go`。字段挂 `Element.HandleInput`（框架里唯一先于
  编辑器的那道缝），两个键在这里裁决，其余一律 `return false`，字段自己的编辑原样继续。
- **两步删除**：`historyDeleteClaimable` 只在「历史模式（剪贴板/计算器）持有字段 + 字段为空 + 选中行
  是一条历史行」时认领——旧版 R55 的结构性缓解，手在删文字时 ⌘⌫ 仍是「删到行首」。装填改为**按 id**
  （`armedDelete string`）而不是按行号：列表在装填期间移位（剪贴板监听器插入新条目）只会让这一按
  在新行上**重新装填**，不会把确认落到用户没看的行上。装填随「任何其他键」解除（旧版规则），
  Escape 先回答它、不顺手离开模式；离开模式也清掉装填。
- **删除后的选中**：回到旧版 `selectionAfterRemoval` 的「邻居接管选中，不跳顶」（此前是跳回第一行）。
- **命令历史清空**：`Ctrl+Shift+Backspace` 清掉 `History` 并给出状态行文案（`i18n.Launcher`
  新增 `HistoryCleared`，en/zh 都有；旧版 `launcher.historyCleared`）。
- **测试**：`fieldkeys_test.go`（装填/确认、计算器行、字段有字时让出编辑手势、普通页与计算器答案行
  不动、任何键解除装填、Escape 只解除、邻居接管选中、移位不误删、装填随模式退场、命令历史清空、
  键位与 `selectionAfterRemoval` 的表格）。把 `HandleInput` 摘掉跑一遍，除「断言否定」的三项外全红
  ——测试确实钉在机制上。

**P7-w 透明底（第三次报告）+ Super 键交互（本轮）**

用户第三次报告：「floter 透明底还是有问题」；并报告 Super（应用修饰键）的交互与原版有差异。
两件都对照 `backup/main-pre-mygo` 定位，透明度用**无头生产帧的像素 alpha** 验收。

### 1 · 透明底：内容面把帧的 tint 又画了一遍（叠成两层）

- **前两次为何无效**：`7caa60b` 只改了**窗口材质**（Windows Mica→Acrylic），但材质在场景下面，
  动不了像素；`622efae` 把内容面 `--surface-sunken` 从固定带改成**骑滑块**，却把它设成
  **等于帧的 tint**（`glassmap.SpecFor` 的 `ContentAlpha() == TintAlpha`），而 `theme.For` 的
  `Sunken` 又是在**卡片已经铺了帧 tint 之后**再铺一层（`shell.go:2734` 的 `face.Background(fill)` +
  `launcher.go:710` / `settingsui.go:574,582` 的 `Background(tk.Sunken)`）。合成 alpha = `1-(1-t)²`：
  dark 10% 时帧只有 0.505，可见底部却是 **0.761**；再叠上**不骑滑块**的 sheen 渐变
  （light `sheen-top` 0.66），滑块只把底部从 0.76 推到 1.0，用户读作「底部还是不透明的，拖了没反应」。
- **真根因（机制级）**：可见底部 = 帧 tint（`glassmap` + `shell.go:2734`）**再叠**一层同样厚的
  内容 tint（`glassmap.go:157` 旧 `ContentAlpha`）**再叠**固定 sheen（`theme.go:315` 旧值）。
  三层的 alpha 只在滑块两端动 `1-(1-t)²` 这条被压缩过的带。不是设置没到、也不是框架画了不透明底——
  是应用自己的内容面把帧又盖了一遍。
- **原版怎么做**：`backup/main-pre-mygo:src/glass-material.ts` 把 `main_opacity` 定为「帧的唯一
  alpha 真值」，`base.css:417` 的 `--glass-frame-alpha = min(0.98, max(0, opacity))`；
  `base.css:461` 的 `--surface-sunken = rgba(17,18,20, 0.62+0.18*op)` 只是内容面的**一层**，
  `base.css:633` 的 `.launcher-bottom { background: var(--surface-sunken) }` 叠在卡片 tint 上。
  即原版的底部也是「帧 + 内容面」两层，但内容面的 alpha 远小于 1，且 sheen 折进同一层，
  滑块仍是主项。（原版 launcher 另有 `--launcher-glass-floor: 0.74` 的地板。）
- **修法**：内容面改成**只带 `frame + 0.06` 凹槽**的过帧 alpha（`glassmap.go:137,141,157` 的
  `contentStep`/`overFrame`/`ContentAlpha`）：`S = (target-frame)/(1-frame)`，于是可见底部 =
  `frame + 0.06`，是滑块拥有的**一张**底。sheen 也按帧 tint 缩放（`theme.go:315,444` 的
  `scaleAlpha`），不再用固定高光把背景顶到滑块动不了。终端 `inherit` 主题此前交给插件的不透明
  默认背景，改为 `Transparent: true`（`terminalui.go:533`），让卡片的终端透明度 tint 透出来。

**各档位底部区域平均 alpha（无头生产帧实测，band y=350..400，`shell/transparency_test.go`）**：

| 档位 | dark 改前 | dark 改后 | light 改前 | light 改后 |
|---|---|---|---|---|
| 10% | 0.761 | **0.570** | 0.820 | **0.622** |
| 25% | 0.785 | **0.597** | 0.837 | **0.649** |
| 50% | 0.862 | **0.687** | 0.895 | **0.734** |
| 75% | 0.953 | **0.840** | 0.965 | **0.872** |
| 100% | 1.000 | 1.000 | 1.000 | 1.000 |
| **幅度** | 0.239 | **0.430** | 0.180 | **0.378** |

- **验收测试**：`internal/shell/transparency_test.go` 的 `TestTheSliderMovesTheRenderedBackground`
  在每个档位渲染真实窗口内容（`ui.NewTester` 走生产 `surfaceFrame`），直接读渲染结果像素的
  alpha，测底部/中部/顶部三个带：底部必须单调上升、幅度 ≥0.25、最低档 <0.68、最高档 ≥0.95。
  **把改动 stash 掉重跑，dark 底部 0.785、light 0.837、幅度 0.215/0.163——测试确实红**。
  `TestTheContentPaneIsNotASecondFullTint` 另钉住「内容面不是第二层同厚 tint」。
- **窗口层核对**（回答「设置有没有到窗口层」）：`shell.go:946` 设 `Transparent: true`，
  `shell.go:962` 设 `Vibrancy: vibrancyFor(GOOS)`（Windows→Acrylic、macOS→UnderWindow、
  Linux→None）；框架以透明色 `scene.Color{}` 清屏（mygo `ui/paint.go` `s.Reset(..., scene.Color{})`），
  Linux 的 GL area 拿到 `hasAlpha`（mygo `internal/linux/surface.go`）。**Windows 的 surface 路径只
  在 `Vibrancy != "" && systemBackdrops()`（Windows 11 22H2，build ≥22621）`&& DirectComposition`
  时才开 `WS_EX_NOREDIRECTIONBITMAP`（mygo `internal/windows/window.go:126`）；否则 `PresentPixels`
  走 `SetDIBitsToDevice`（忽略 alpha）**——这是框架在 Win10/<22H2 上的限制，本应用侧无法绕过，
  报告于此。

### 2 · Super 键：松手后窗口高度不回退（`HoldRows` 的一行余量）

- **原版怎么做（file:line）**：
  - 按住修饰键的可见性由 `backup/main-pre-mygo:src/launcher/terminal-row.ts` 的
    `bareTerminalPlacement`（第 77 行）判定：`if (!modifierHeld || scope !== null) return null;`
    再 `return query.trim().length > 0 ? null : "appended";`——**有 query 就不画**（R64 明确：
    「输入框不为空时按住 cmd 不出现」）。行落在列表**末尾**（App.tsx `displayedResults` 追加）。
  - 修饰键的按下/松开由 `src/hooks/useAppModifierHeld.ts` 的 window 捕获监听维护（keydown/keyup/blur）。
  - **窗口高度**：`src/App.tsx`（约 2198 行）直接 `launcherListUnits(displayedResults.map(...))`
    算高度，**没有** `resolveLauncherRows` 的余量——`result-budget.ts` 里那个函数是死代码，
    R43/R66 的注释写明「窗口精确跟随内容；顺滑来自 `useLauncherHeight` 的动画，不是保留高度」。
    所以松手 → 行消失 → 高度立刻回退。
- **本构建两处偏离**：
  1. `internal/launcher/height.go:71` 的 `HoldRows` 带 `Hysteresis = 1`：内容只掉 1 行时
     `target < current-1` 为假 → 保留旧高度。而持有行恰好只加/减 1 行，于是**松手高度不回退**。
  2. `internal/launcher/launcher.go:1920` 的 `heldTerminalItem` 用 `strings.TrimSpace(a.Query) != ""`
     挡住有 query 的情况——**这与原版一致**（原版 `terminal-row.ts:81` 同样按空字段判定）。用户
     「有 query 时不出现」是对原版记忆的偏差，不是偏离；保留。
- **修法**：`HoldRows` 改为精确跟随内容（`return RowCount(rows)`，删掉 `Hysteresis`）——对齐原版
  R66。松手时 `len(results)` 少 1 → `HeldRows` 少 1 → `ResizeTo` 变矮 → `walkLauncherHeight` 的
  弹簧动画收回。
- **测试**：`internal/shell/heldmodifier_test.go`：按住 → 出现 + 选中最后一行 + `launcherTarget` 变大；
  松开 → 消失 + `launcherTarget` 回到原值；有 query → 不出现（钉住原版语义）。
  `internal/launcher/height_test.go` 的 `TestHoldRows` 改成钉精确跟随。

**P7-c 四个用户报告（本轮）**

1. **开玻璃时透明底失效**。上一轮把 `off` 档改成帧自己的 alpha，但效果档仍用
   `--glass-tint-alpha`（帧 alpha 再叠档位 haze），于是开玻璃比关玻璃更不透明，滑块低端失效
   （「关闭液态玻璃效果时透明底可以了，但是开启时又没有透明了」）。修法：`glassmap.SpecFor`
   的 `TintAlpha` 在所有档位都取帧自己的 alpha（`min(SolidTop, opacity)`），haze 只作为档位的
   材质记录保留、不再进填充。实测（无头生产帧，底部 band）：10% 时 off 0.163、frosted 0.163、
   regular 0.163、liquid 0.163（dark），滑块到 100% 都到 1.000。
2. **最近打开列表**。`glassmap` 无关；`internal/launcher` 的 `recentItems` 对齐原版
   `src/launcher.ts` 的 `recentItems`：上限从 5 改成 `MAX_RESULTS` = 10；并列时按应用扫描顺序
   （`b.count - a.count || a.index - b.index`），此前 `usage.Top` 按 `Last` 排（`refreshRecents`
   重排）。过滤（count>0）、去重（map）、点击（开应用）、空态（无最近项 → 无块）逐项对齐。
3. **玻璃 3 档看不出区别**。档位到像素的唯一 rung 之前只有 tint（且很薄）；现在把
   `--glass-lens-scale`（0.5/1/1.6）也压到帧的 sheen 上（`theme.For` 的 `sheen = tint * lens`），
   于是帧的亮面随档位变：dark 顶带 RGB 21/24/28 → 28/31/35（sum 74→95），light 125→180。
   档位的 blur/saturate 仍是窗口材质（`vibrancyForStop`）。
4. **设置页头部栏**。`settingsui` 的头部从写死的 `t.Space(14)` 改成 `launcher.FieldRowUnits`
   （新导出），gap 对齐原版 `.settings-card__header` 的 10u；像素验收：两表面的第一带中心都在
   y=39.8（`geometry_test.go` 的 `TestSettingsHeaderSharesTheLauncherFieldRowBand`）。

**P7-d 第四次透明度报告 + super+n 复刻原版（本轮）**

用户报告：「floter 的开玻璃透明底问题还是没修好。同时现在列表页的 super+n 快捷键逻辑也没有
复刻原版，现在是固定打到前几项上」。

### 1 · 透明底：帧的 sheen 层（不是 tint）

- **定位**：`internal/shell/shell.go:2955` 的 `View` 在卡片 tint 之上又画了一层
  `ui.Box(...).Absolute().LinearGradient(SheenTop, SheenBottom)`，而 `theme.For`
  （`internal/theme/theme.go:266`）只对效果档给 sheen 非零（`sheen = TintAlpha * LensScale`，
  `off` 为 0）。这层自带 alpha，composite 到 tint 上就变厚——**它就是「开玻璃」时多出来的那一层**。
  前几轮改的 `SpecFor.TintAlpha` / 内容面 / 窗口材质都在它下面，动不了这个数字。
- **原版对照**：`main:src/styles/launcher.css:119` 的卡片是
  `linear-gradient(180deg, var(--sheen-top), var(--sheen-bottom)), var(--launcher-tint)`；
  原版的 `--sheen-*` 是固定 token（`base.css:628/906`，dark 0.046/0.01、light 0.66/0.22），
  档位不改它。档位的可见 rung 是 `--glass-step-blur`（窗口材质）与 `--glass-lens-scale`
  （控件透镜），不是帧的 sheen。
- **修法**：新增 `theme.Tokens.FaceFill(alpha)`：按 `1-(1-base)(1-sheen) == alpha` 反解出
  帧 tint 的 base alpha，sheen 重多少 tint 就减多少，于是 sheen 不再增加帧的 alpha；sheen 自身
  也被 `capAlpha` 夹在帧 alpha 以内（极薄的帧顶端不会比滑块更厚）。`shell.View` 用
  `FaceFill(spec.TintAlpha)` 作底色，sheen 层保留（档位仍让亮面变色）。
- **像素实测**（无头生产帧，`shell/glass_test.go` 的 `TestTheFrameAlphaIsTheSlidersOnEveryStop`；
  band 顶 y=90..120 / 中 y=200..260 / 底 y=350..400，均值 alpha）：

  | 外观 / 滑块 | band | off | frosted | regular | liquid | 改前 regular/liquid |
  |---|---|---|---|---|---|---|
  | dark 47% | 顶 | 0.563 | 0.563 | 0.566 | 0.566 | +0.008 / +0.012 |
  | dark 47% | 底 | 0.530 | 0.530 | 0.530 | 0.530 | +0.004 / +0.004 |
  | light 47% | 顶 | 0.564 | 0.565 | 0.561 | 0.550 | **+0.114 / +0.182** |
  | light 47% | 底 | 0.531 | 0.531 | 0.531 | 0.524 | **+0.056 / +0.091** |
  | light 10% | 底 | 0.163 | 0.163 | 0.164 | 0.162 | +0.023 / +0.035 |

  效果档在每一档滑块、每一个 band 都**不高于** off（线性渐变中间段最多低 0.021，是更薄不是更厚）；
  滑块仍拥有整张底（10% 底 0.163、47% 底 0.53、100% 底 1.000）。
- **测试**：新表测试钉住「效果档不得比 off 更厚」；`TestTheGlassStopsAreVisiblyDifferent`
  仍钉亮面颜色随档位上升（dark RGB sum 74→95、light 125.3→133.2）。

### 2 · super+n：编号跟滚动视口

- **原版对照表**（`main:src/launcher/result-budget.ts`）：

  | 规则 | 原版 | 本构建原来 | 现在 |
  |---|---|---|---|
  | 编号范围 | 视口内（`resultShortcutSlots(rows, flags, visible)`，`start=floor(visible.start)`、`end=ceil(visible.end)`） | 整表前 10 个可运行行 | 超过 10 行时取 `List.Visible()` |
  | 数字 | 前九个 `1`-`9`，第十个 `0` | 同 | 同 |
  | 视口外 | `null`，`⌘N` 够不到 | 前 10 行永远有号 | 视口外无号 |
  | 滚动 | 重新编号到屏上（R34） | 不动（钉在头部） | 下一帧重新编号 |
  | 短表（≤10） | 不查视口，整表顺序编号（R72） | 整表 | 整表（同） |
  | 按键映射 | `resultIndexForSlot(slots, slot) = slots.indexOf(slot)` | `a.numbers` 反查 | 同 |

- **修法**：`launcher.assignNumbers` 在 `len(results) > maxResultSlots`(=10) 时读
  `a.List.Visible()`，只给视口内的可运行行编号；短表仍整表编号。视口是上一帧的，和原版按键
  处理器读上一次渲染的 slot 表一致。
- **测试**：`launcher/resultnumbers_test.go` 的 `TestResultShortcutsFollowTheScrollViewport`
  （30 行：滚前 `numbers[0]==1`；滚后 `numbers[vf]==1`、`numbers[0]` 消失、所有编号行都在视口内、
  modifier+1 跑可见行而非 `App 00`）与 `TestAShortListIsNumberedWhole`。

## 注意事项（踩过的坑）

- **Tester 的 `Type("\b")` 是插入文本，不是按退格**：测试里删字符要用 `tt.Key(0, ui.KeyBackspace)`。
- **视图里的快捷键先于聚焦元素的 `HandleInput`**：录制器必须在自己录的时候让出 Escape（已处理）。
- **反过来：编辑器取走的键，只有 `Element.HandleInput` 能先拿到**。框架的按键顺序是「聚焦元素的
  `HandleInput` → 编辑器的 `wants` → 快捷键注册」，而编辑器那一支没有 `claimed` 守卫——所以
  `c.Shortcut` 永远够不到带修饰键的 Backspace（字段的删除/清空历史键因此挂在字段自己身上，见 P7-v）。
- **`Tester.Type` 只发文本、不发 keydown**：要测「按键」的规则得用 `tt.TypeKey(0, ui.KeyX, "x")`，
  否则 `HandleInput` 根本看不到那次输入。
- **程序设置文本后要显式把光标移到末尾**（`SetTextSelection`），否则打字会插进命令词中间导致模式退出。
- **测试不要碰真实系统资源**（剪贴板、登录项、窗口、原生库）：相关能力都在 `shell.Options` 里可注入（`WriteClipboard`、`RegisterShortcut`、`OpenAtLogin`、`OpenPinned`、`ConfirmPermissions`、`Registry`、`NewTerminal`、`Paths`）。
- **`mygo build` 曾因旧 `package.json` 生成空的 `src/mygo.ts`**；旧前端删除后已消失。
- **平台表要写成纯函数**：`internal/browser` 的浏览器路径表以前按宿主平台返回，Linux CI 直接红；
  现在是 `(goos, home, …)` 的纯函数，任何宿主都能断言三个平台的布局。运行期解析同理
  （`internal/extensions` 的 `lookTool`/`findInterpreter` 可注入，测试不看机器 PATH）。
- **原生库依赖**：终端插件要 libghostty-vt、浏览器插件要 libsqlite3；`go test` 首次会按需下载到 `~/Library/Caches/mygo/natives/`，打包时由 CLI 内嵌。
- **“框架做不到”这个结论要先查 `Options` 的每个字段**：`terminal_paste_safe` 被记了两轮的框架限制，
  其实 `Options.OnPaste` 一直都在——死的只是应用从未设过它（P7-an 订正）。把「有没有 hook」与
  「有没有旋钮」分开：前者往往已经存在（`OnPaste`/`OnKey`/`OnScroll`/`OpenLink`/`OnNotify`），
  后者才是真缺（滚动条、滚轮步长、选中即复制、bold 模式）。
- **算出来没人读的字段是 bug 的化石**：`ParseCommandLine` 的 `ShellSyntax` 从建库起就是死值，
  而它正是「带管道的一行要交给 shell」的唯一判据（P7-an）。新加的字段如果没人读，宁可不加。
- **字符串要变成 PTY 字节，就得过 `internal/inputtext`**：shell 只把 U+0020 当词缝，其余 Zs
  （尤其全角空格 U+3000）会把两个词粘成一个 token。结构化 argv（`ParseCommandLine`）、App 拼的
  命令行（`RunShellWithCommand`）、粘贴（`Options.OnPaste`）三个接缝现在都走它；框架自己处理的
  键入文本还够不到（见「已知差距」）。


## ⚠️ 用户实测反馈（2026-10-14 早，最高优先级）

> 用户原话：「floter 项目当前存在以下问题」——以下四项**优先于上面所有待办**。本轮（P7-eg）先把它
> 逐条写实成带 file:line / 原版对照 / 验收标准的待办（下面四项），再取第 1 条（F1）落地；F2 复核为
> 已落地（P7-d），F3/F4 拆成可执行子项留给后续轮次。

### F1 · 搜索框 Super 键 → 底部「打开终端」行（**本轮落地，P7-eg**）

- **现象**：输入内容之后按住 Super（App 修饰键），底部应固定出现「在终端中打开」的行。
- **原版对照（用户要求与参考的取舍）**：`main:src/launcher/terminal-row.ts:77-84` 的
  `bareTerminalPlacement` 在 **R64** 把行窄化成「空字段才画」（`query.trim().length > 0 ? null`）；
  更早的 **R60** 是「按住修饰键就追加底部终端执行项」。两者都是用户自己的措辞（R64 的注释里逐字记着）。
  本构建此前完全按 R64，并把「有 query 不画」钉成参考行为
  （`internal/shell/heldmodifier_test.go` 的旧 `TestHeldModifierRowFollowsTheReferenceQueryGate`）。
- **决定**：**用户的最新要求优先**——恢复 R60 的更宽手势：按住修饰键时该行**无论字段是否为空**都是
  列表最后一行（这就是「底部应固定出现」）。deviation 与理由写在 `internal/launcher/launcher.go` 的
  `heldTerminalItem` 注释里。
- **落地 file:line**：
  - `internal/launcher/search.go` 的 `Results()`：`other` 之后追加 `heldTerminalItem()`，保证它是
    **最后**一行，且动作栏的存在性闸门只读 `other`（不含该行）；
  - `internal/launcher/search.go` 的 `otherResults()`：删掉空字段分支里的追加（避免该行污染动作栏闸门）；
  - `internal/launcher/launcher.go` 的 `heldTerminalItem()`：删掉 `strings.TrimSpace(a.Query) != ""`
    挡板；`syncHeldModifier` 的按下沿照旧把选中放到最后一行。
- **验收**：`internal/launcher/filters_test.go` 的 `TestHeldModifierBareTerminalRow` 末段（先键入
  「zzz」再按住修饰键 → 行在、在最后、选中在其上）；`internal/shell/heldmodifier_test.go` 的
  `TestHeldModifierRowStaysOverAQuery`（端到端；旧 `…QueryGate` 已随本轮改写）。变异核查：把
  `heldTerminalItem` 的 query 挡板加回去，两条立刻红（`the held row hid over a query`）。

### F2 · 列表滚动时编号快捷键跟随视口（**已落地 P7-d；本轮复核为参考行为**）

- **现象**：滚动时应始终给**当前视口内**的行编号（`1`~`9` 与 `0`），「不跳屏、不滞后」。
- **原版对照**：`main:src/launcher/result-budget.ts:883-901` 的 `resultShortcutSlots(rows, flags, visible)`
  （`start=floor(visible.start)`、`end=ceil(visible.end)`，前九行 `1`-`9`、第十行 `0`）；`:909-912` 的
  `resultIndexForSlot` 是「键 ↔ 徽章」的唯一来源。
- **本构建**：`internal/launcher/launcher.go:417-470` 的 `assignNumbers` 已实现（溢出时读
  `a.List.Visible()`，`internal/launcher/resultnumbers_test.go` 三条钉住），P7-d 落地。
- **复核对齐**：编号家族只覆盖**可运行行**（`Run != nil` 且非 `bar:`/`status`/`heading`），与
  `result-budget.ts:837-855` 的 runnable flags 同义；视口是**上一帧**的（原版键处理读上一次渲染的 slot
  表），列表在**下一帧**重编号——这是即时模式与原版 React 的同一个「键滞后一帧」，不是偏差。
- **残留（若用户仍读作「滞后」）**：把编号改成在 `ui.List` 的行回调里按**当前**视口赋号，可去掉这
  一帧；但那会让 overscan 行（mygo `listOverscan=2`）也被编号，与 `resultShortcutSlots` 的精确
  `floor/ceil` 不完全等价，需要单独一轮权衡。**列为可选后续，非本轮。**
- **验收（现状）**：`TestResultShortcutsFollowTheScrollViewport`（30 行滚动后编号落到视口首行、键跑
  可见行而非 `App 00`）、`TestAShortListIsNumberedWhole`。

### F3 · 毛玻璃效果配置（**拆成 F3a/F3b 两个可执行子项**）

- **F3a · 液态玻璃两态观感差。**
  - **原版对照**：`main:src/glass-material.ts` 的 `GLASS_STEP_TOKENS`（dim off 0 / frosted 0.6 /
    regular 0.5 / liquid 0.2）与 `main:src/styles/base.css:780-800` 的 `[data-glass]` 三档
    （blur 10/22/28、saturate 130/170/200、lens 0.5/1/1.6）。
  - **本构建**：`internal/glassmap/glassmap.go:88-101` 的 `stops` 三档**数值与原版逐项相等**；可见差异
    只剩两处落点：窗口材质 `internal/shell/shell.go:2738` 的 `vibrancyForStop`（macOS 三档
    UnderWindow/…；Windows 只有 Acrylic；Linux None）与控件透镜
    `internal/theme/theme.go:347-349` 的 `LensRim/LensSheen/LensEdge × LensScale`。
  - **验收/待做（P7-ej 实测后重写）**：**已实测**——把帧 sheen 临时改成固定 token（F3b 的字面做法）
    后渲染生产帧：帧专属带（y=90..120，不含控件）frosted/regular/liquid 的 RGB sum 为 dark
    100.3/100.3/100.5、light 397.2/397.8；以 20px 为步长扫全画布，regular 与 liquid 的 RGB sum 差
    **处处 ≤0.4**。即**控件透镜在本构建的渲染里几乎不贡献像素**——它只在搜索框 1px 内影与图标底板上
    （`internal/launcher/launcher.go:1724`、`glyphs.go:164`）。所以「Linux 上仅靠透镜」**不成立**。
  - **重写后的待做（P7-el 定案）**：取 **(a) 让控件透镜真的成为可测像素**。定案依据是这回找到了原版
    自己的一份清单：`main:tests/glass-reaxis.test.ts:47-90` 的 lens census **逐条点名**了必须画透镜的
    控件（raised/recessed/hover-only 三类），而本构建的 `Tokens.Lens` 只被四处调用——设置面与启动器的
    **按钮/分段控件/开关轨道**（多为框架 `ui.Button`/`ui.Switch` 直接绘制）根本没有透镜，这正是
    ≤0.4 RGB 的根因。完整的落点与验收标准见文末「具体待办（P7-el 重写）」的 **F3a**。
  - **P7-em 已落地**：新增 `Tokens.LensInset` + `TrackEdge`，把扩展页/设置面的按钮（`actionButton`）、
    开关（`switchControl`）、自定义模式分段控件（`segmentedControl`）、侧栏/头栏/保存横幅的 hover、
    控制台 `Segmented`、toast 动作 chip、终端栏按钮 hover 全部接上透镜；`internal/shell/glasscensus_test.go`
    把原版 13 条选择器逐条映射（file + 透镜调用 + 像素测试）。**控件一侧的验收已成立**（常背景隔离帧
    sheen 后，控件 RGB sum 随 0.5/1/1.6 严格上升，regular→liquid ≥ 数百到数千）；F3b（**帧** sheen 回固定口径）已由 P7-en 落地，见下。
    改回固定口径（已落地，P7-en）。
- **F3b · 玻璃作用范围：控件 vs 整体；整体弱化、阴影细节加强（**P7-en 已落地**）。**
  - **原版对照**：`main:src/styles/launcher.css:119` 的卡片是
    `linear-gradient(180deg, var(--sheen-top), var(--sheen-bottom)), var(--launcher-tint)`，而
    `--sheen-top/-bottom` 是**固定 token**（`main:src/styles/base.css:628-629` dark 0.046/0.01、
    `:906-907` light 0.66/0.22），**不随 `[data-glass]` 变**；档位的可见 rung 只在控件透镜
    （`base.css:569-572` 的 `--glass-lens-*` 乘 `--glass-lens-scale`）与窗口材质上。
  - **本构建（已修）**：`internal/theme/theme.go` 曾把帧的 sheen 写成
    `sheen = spec.TintAlpha * spec.LensScale`（P7-d 为「3 档看不出区别」加的耦合），于是**帧**的亮面随
    档位变，而原版的帧 sheen 固定。
  - **P7-en 落地**：`theme.go` 改为 `spec.Glass` 时取按外观固定的
    `capAlpha(p.sheenTop/p.sheenBottom, spec.TintAlpha)`（`off` 为 0），`scaleAlpha` 收窄为只管控件透镜。
    落地步骤与验收见文末「具体待办（P7-el 重写）」的 **F3b** 与上文 P7-en 记录。
  - **验收（已过）**：`internal/theme/theme_test.go` 两条新测试钉「三档 sheen 完全相等且等于固定 token
    的 cap 值」与「固定 token 而非 tint 缩放」；`internal/shell/glass_test.go` 的
    `TestTheGlassStopsAreVisiblyDifferent` 改写并改名为 `TestTheFrameSheenIsFixedAcrossStops`（帧顶带
    三档同色、off 无 sheen），而**控件**的 lens 强度仍按 0.5/1/1.6 单调上升（P7-em 的四组像素测试）。
  - **风险**：帧 sheen 参与透明度的 `FaceFill` 补偿（`theme.go` 的 `baseAlpha`/`capAlpha` 与
    `internal/shell/transparency_test.go`）——P7-en 已重跑该套像素测试，全绿。

### F4 · 集成组件逻辑差异（**P7-eh 落地 F4a；P7-ei 落地 F4b；P7-ek 落地 F4c；P7-ej 落地 F4d；P7-el 落地 F4e 的 Reconnect 一支**）

- **现象**：集成组件逻辑与原版差异仍大，需要梳理、可能重构。
- **原版对照**：`main:src/ExtensionsPanel.tsx`（面板与安装/权限/运行流程，3269 行）与
  `main:src/extensions/ExtensionRow.tsx`（每行卡片，532 行）。
- **本构建入口**：`internal/settingsui/settingsui.go:1120` 的 `integrations`（列表页）、`:1485` 的
  `integrationRow`（卡片）、`:1326` 的 `stateMark`、`:1741` 的 `permissionAudit`，数据来自
  `internal/extensions` 的 inventory/repository。
- **P7-eh 做法**：以 `ExtensionRow.tsx` 为蓝本逐函数对照（`generatedCustom`/`distributionSource`/
  `runtimeOwnership`/`connected`/`runtimeAvailable` 五组判据），落成下面四条**带 file:line 的具体待办**，
  并取第 1 条（F4a）落地。P7-ei 取第 2 条（F4b）落地。`ExtensionsPanel.tsx` 的安装/审批/运行流程
  留作后续轮（见「残留」）。

#### F4a · 卡片的「移除」动词全按一种说法（**本轮落地，P7-eh**）

- **原版对照**：`main:src/extensions/ExtensionRow.tsx:109-114` 的 `removalKind`（`generatedCustom` →
  `custom`；`distributionSource==="npm"` → `npm`；`built-in` + `runtimeOwnership==="system"` → `system`；
  否则 `package`）与 `:409-418` 用它选菜单项文案 + 图标：custom →
  `settings.extensions.deleteCustom`（`main:src/i18n.ts:540` 「Delete integration」）+ `Trash2`；npm →
  `uninstall`（`:521`）+ `Trash2`；system → `disconnect`（`:522`）+ **`Unplug`**；package →
  `removePackage`（`:525` 「Remove package」）+ `Trash2`。
- **本构建（偏离）**：`internal/settingsui/uninstall.go:27,45` 与 `settingsui.go:1698-1701` 三个移除入口
  **一律**用 `copy.IntegrationsUninstall`（「Uninstall」）——一个 `system` 行说「Uninstall」而它的运行时由
  系统拥有、包根本不是 floter 的，一个 `local` 链入包说「Uninstall」而文件并不在 floter 目录里。
  数据侧也缺字段：`settingsui.Integration` 没有 `distributionSource`/`runtimeOwnership`。
- **落地 file:line**：
  - `internal/settingsui/settingsui.go` 的 `Integration` 结构体新增 `DistributionSource`、
    `RuntimeOwnership` 两个字段；新增 `removalKind()`（原版 `:109-114` 逐行）与
    `removalLabel(copy)`（原版 `:409-418` 的四选一；custom 复用已有的 `RemoveDelete`＝原版
    `deleteCustom` 的同一句话）；
  - 三个移除入口改读 `integration.removalLabel(copy)`；
  - `internal/shell/shell.go` 的 `integrationList` 把 `Entry.DistributionSource`/`Entry.RuntimeOwnership`
    投影进这两个字段。
- **验收**：新增 `internal/settingsui/integrationremoval_test.go`——
  `TestTheRemovalVerbFollowsTheIntegrationKind` 六条表（含「generated 胜过 npm」的顺序断言）钉
  `removalKind` 与 en/zh 两句话；`TestEachIntegrationCardShowsItsOwnRemovalVerb` 同屏渲染四种行，在
  **像素上**钉四个动词都在（纯函数对了而卡片仍硬编码一个词时这条红）。变异核查：把
  `removalLabel` 的 system/package 两支改回 `IntegrationsUninstall` → 两条立刻红
  （`removalLabel(en) = "Uninstall", want "Disconnect"`）。
- **本轮未改**：确认条的**标题/正文/确认按钮**仍走 `RemoveTitle`/`RemoveDetail`/`RemoveConfirm`
  （除 custom 走 `RemoveDelete`）。原版 `RemovalConfirmation.tsx:41` 的确认按钮与菜单项用**同一个
  kind key**，而 `removalTextKey` 还给 package/system 各自的 `removePackageTitle`/`disconnectTitle`
  词——这把确认条也按 kind 说话留给后续轮（需要新增配对 i18n 键），不塞进本条。

#### F4b · 卡片 meta 行补回「集成种类」词（**本轮落地，P7-ei**）

- **原版对照**：`ExtensionRow.tsx:77-101` 的 `integrationKindKey`（custom / npm / manifest / recommended /
  tool / system / package 七种，判据是 `generatedCustom`、`distributionSource`、`!connected &&
  manifestSuggestion`、`!connected && recommended`、`!connected && distributionSource==="local"`、
  `built-in` + `system`），在 `:220-222` 与 `:255` 作为 meta 行的第二个 `<span>` 画出
  （词条 `i18n.ts:446-453`）。
- **本构建（偏离）**：`internal/settingsui/settingsui.go` 的 `Integration.identity()` 只印
  publisher · version · ID，没有种类词；`integrationRow` 的 meta 行
  （原 `a.row(c, integration.identity(), "", func(){})`）因此缺这一格。
- **落地 file:line**：`internal/settingsui/settingsui.go` 新增 `Integration.integrationKind(copy)` 与
  `identityLine(copy)`（identity 在前、类型词在后，两格都是原版的槽位与顺序）；卡片那句改读
  `identityLine(copy)`。i18n 新增四键 `IntegrationsKindCustom` / `KindNPM` / `KindSystem` /
  `KindPackage`（en/zh 用原版原文）。判据读 `Generated` / `DistributionSource` / `RuntimeOwnership`
  （F4a 已投影），不需要新字段：原版的 `recommended` / `manifest` / `tool` 三支都要求
  `!connected`，而本构建把这三类行放在 Detected / Recommended 两个独立段、用各自的渲染器
  （`DetectedTool` / `RecommendedTool`），所以在「卡片是 connected」前提下七分退化为四分；
  多声明三个词只会留下无人读的 i18n 键（`TestNoDeadCopy` 会红）。段合并是 F4c 的独立决定。
- **验收**：新增 `internal/settingsui/integrationkind_test.go` 三条（六行表 + 拼接 + 四行同屏像素
  断言）。变异核查：卡片 `identityLine(copy)` 换回 `identity()` → 像素条立刻红。

#### F4c · detected（未连接）行的动作语义不是原版那张脸（**本轮落地，P7-ek**）

- **落地（P7-ek，取「接通目录」这一支）**：`settingsui.DetectedTool` 新增 `RuntimeAvailable` /
  `InstallCommand`；新增 `detectedActionFor`（三态选择，`runtimeAvailable` 优先）与 `detectedLabel`
  （Connect / Install in terminal / Install tool first，i18n 新增后两键）；`detectedSection` 的行改走
  `detectedAction`（不可运行且有配方 → 跑 `InstallDetected(command)`，都没有 → 禁用）。`internal/shell`
  的 `detectedTools` 用可执行文件名匹配 `tools.Catalog` 的 `Probes`，经 `ChosenRecipe`+`InstallCommand`
  算出配方命令；`InstallDetected` 接到 `a.openShellCommand`。验收见文件末尾「本轮（P7-ek）」的测试说明。
  仍是「退回态」而非三态齐全的原因已在上文 P7-ej 实测里说清（detected 全为 `Available`；「本机没有、
  目录能装」的历史落点是启动器的 install 行）；布局上保留段式（与本区其它段一致），不合并进 Installed。

- **P7-ej 实测补充**：本构建的 `detectedTools`（`internal/shell/shell.go:1897`）只列出
  `candidate.Available && candidate.Path != ""` 且**未连接**的项，所以 detected 行恒为
  `runtimeAvailable` → Connect；`homepage` / `installInTerminal` 在本构建的 inventory 投影里**根本
  没有数据**（`inventory.Candidate` 无 homepage；`tools.Catalog` 有 `Recipes` 但从未接进 detected
  投影）。所以 F4c 的真实缺口不是「补三态」（三态会退化成单态），而是：要么把 `tools.Catalog` 的
  `ChosenRecipe`+`InstallCommand` 接进 detected 投影（installInTerminal 态真的可判），要么承认本
  构建的 detected 就是「已在 PATH、只需 Connect」并只补布局（合并进 Installed 段 / 补 disabled
  原因）。这个决定与 F4d 无关，留后续轮。

- **原版对照**：`ExtensionRow.tsx:299-330`——同一张卡片，`!connected` 时右侧只有一个按钮：
  `runtimeAvailable` → Connect（`Link2`，`aria-label=connect`）；`installInTerminal` → 终端安装
  （`installInTerminal`）；否则 → 去主页（`ExternalLink`，`installTool`），且
  `disabled = busy || (!runtimeAvailable && !homepage && !installInTerminal)`。卡片 class 也随
  `!connected` 切 `extension-row--detected`（`:290`），且 detected 行**没有**开关槽（`:430`）。
- **本构建（偏离）**：`internal/settingsui/settingsui.go:3348` 的 `detectedSection` 把发现项放进
  **独立的一段卡片**（`copy.DetectedTitle`），每行一个恒为 `IntegrationsConnect` 的按钮，没有
  runtimeAvailable / installInTerminal / homepage 三态，也没有 disabled 的原因；它不在「Installed」
  段里，因此与原版「同一列表、同一卡片」的结构不同。
- **待做 file:line**：`detectedSection` 的行补三态动作（需 `Detected` 的投影带上
  `RuntimeAvailable`/`Homepage`/`InstallInTerminal`，见 `internal/shell` 的 detected 投影）；是否合并
  进 Installed 段是独立决定（本构建的段式布局与该区其它段一致，建议**保留段、只补动作三态**）。
- **验收**：一条渲染测试，三条 detected 项分别钉按钮文案/禁用态；`busy` 与无主页时按钮禁用。

#### F4d · register 深链高亮标记（**本轮落地，P7-ej**）

- **原版对照**：`ExtensionRow.tsx:196-205`（R8-3）——`floter://register` 深链点名的行画一个
  `extension-row__register-dot`（`role="img"` + `registerHighlight` 文案），是**标记不是控件**；
  卡片 class 追加 `extension-row--register`（`:290`）。深链处理在 `ExtensionsPanel.tsx:1500-1548`：命中
  未连接项 → 高亮（+args hint），已连接 → `registerAlreadyConnected`，否则 → `registerNotFound`；
  高亮按 `useTimedReset` 默认 3s 自清。
- **本构建（P7-ej 前缺失）**：`grep -rn RegisterHighlight internal/` 无命中，`Integration` 上没有
  highlight 字段，卡片也不接受 `highlighted`；且 `connectDetected` 构造的 `CustomTool` 没带
  `Platforms`，`PrepareCustom` 一律 `platformRequired`——register/Detected 的连接**从未落地**。
- **落地 file:line**：i18n 三键 `RegisterHighlight`/`RegisterNotFound`/`RegisterAlreadyConnected`；
  `internal/settingsui/settingsui.go` 的 `Integration.Highlighted` 与 `integrationRow` 的 `rowNote`；
  `internal/shell/system.go` 的 `registerFromLink`/`markRegisterTarget`/`hasExecutable`/`sameExecutable`、
  `internal/shell/shell.go` 的 `registerTarget`/`registerHighlight` 与 `integrationList` 投影；
  `connectDetected` 补 `settingsui.CurrentPlatform()`。
- **验收**：新增 `internal/settingsui/integrationregister_test.go`（被点名卡片画标记、其余不画、全屏
  只一次）与 `internal/shell/register_test.go`（未命中/已连接两种句子、连接落地后卡片被标记、标记
  到点自清、`sameExecutable` 符号链接表）。变异核查：摘掉卡片 `highlight` → 像素条红；摘掉
  `connectDetected` 的 `Platforms` → 连接失败、shell 条红。

- **残留**：`main:src/ExtensionsPanel.tsx`（安装文件夹/权限审批/运行参数/输出回看/导入导出的**流程**）
  尚未逐函数对照；F4a–F4d 全部落地（F4c 由 P7-ek 收尾）；**F4e 全部落地**（Reconnect 一支 P7-el、
  `extensions_repair` 一支 P7-ep）；F4f 由 P7-eo 落地；F3a/F3b 由 P7-em/P7-en 落地。余下 **F4g**（逐
  入口对照）与 **F-new**（脚本编辑器）——已写成文末「具体待办（P7-el 重写）」的条目。

### 具体待办（P7-el 重写；F3a P7-em、F3b P7-en、F4f P7-eo、F4e 的 repair 一支 P7-ep、F4g 第 1·2·3·4·5 条入口 P7-eq/P7-er/P7-es/P7-et/P7-eu、F-new P7-ev 全部落地；现无未落地条目）

P7-ej/P7-ek 之后清单上只剩 F3a（一个待决策的分叉）与 F3b（已实测标为「先决于 F3a」）。P7-el 按规则
把清单写实为五条带 file:line / 原版对照 / 验收标准的待办，并取其中新查出的真实缺口（F4e）落地。
**P7-em 又取清单里文档点名的最高优先级 F3a 并落地**（详见上文 P7-em 记录）：控件透镜已成可测像素，
F3b 因此解除「先决」。**P7-en 再取 F3b 并落地**（详见上文 P7-en 记录）：帧 sheen 回固定 token，
用户反馈 F3「毛玻璃效果配置」整条收口。**P7-eo 取 F4f 并落地**（详见上文 P7-eo 记录）：移除确认条
按 kind 说话，F4a 留下的尾巴收口。**P7-ep 取 F4e 的 `extensions_repair`（Recheck）一支并落地**
（详见上文 P7-ep 记录）：broken 行回到原版的 Wrench，repair 校验包身份并在需要时重写 `manifestPath` /
兜底 Reconnect，`repairKind()` 里那处有意偏离随之删除。**P7-eq 取 F4g 的第 1 条入口并落地**
（安装文件夹的 manifest review）；**P7-er 取 F4g 的第 2 条入口并落地**（推荐工具的权限审阅，见上文
P7-er 记录）：推荐工具的 connect 现在先出两层权限审阅（标题 / 可执行路径 / `permissionsRequired` /
每层 label+hint+每条说明 / 边界句），确认才提交全部 declared。**P7-es 取 F4g 的第 3 条入口并落地**
（运行工具选择，见上文 P7-es 记录）：清单 `executableNames` 命中多个程序时先弹 `chooseSystemTool`
（标题 / hint / 每个候选名称+路径+来源），选中路径交给 connect 或 reconnect。**P7-et 取 F4g 的第 4 条
入口并落地**（漂移提示，见上文 P7-et 记录）：列表驱动的版本漂移检查（后台单飞行循环）在工具版本移
动时重生成命令表，并对**真实变化**弹一条 `reprobeNotice` toast（带 `viewDetails` 动作）。**P7-eu 取
F4g 的最后一条入口并落地**（导入导出的调用点，见上文 P7-eu 记录）：单飞行闸 + 忙态行/禁用按钮 + 导出
success toast（原版 `exportedNotice`）+ 导入报告块（excluded 警告/失败明细/summary）。**F4g 至此收口，
下轮最高优先级是 F-new**（脚本编辑器换 `ui/editor`）。下面每条的「原版对照」都只读 `main`
（v0.3.14 原版树）。

#### F4e · 已连接卡片的修复入口（**全部落地：Reconnect 一支 P7-el、repair 一支 P7-ep**）

- 见上文 P7-el 与 P7-ep 记录。原版的另一支 **`extensions_repair`（Wrench「Recheck」）** 已由 P7-ep 落地：
  原版 `ExtensionRow.tsx:313-330` 对 `connected && state === "broken"` 画一个 Wrench，走
  `ExtensionsPanel.tsx:1255-1267` 的 `extensions_repair`，它**校验已装包并按需修复**（不是重跑探测），
  toast 按 `report.repaired ? repairedNotice : recheckedNotice` 二选一
  （词条 `i18n.ts:507` / `recheckedNotice` 旁边一条）。
- **本构建**：没有 repair。broken 行只有一个 `copy.IntegrationsCheck` 按钮 →
  `internal/shell/shell.go:1489` 的 `diagnoseIntegration`（跑 `extensions.Reprobe` + `extensions.Diagnose`），
  它**只报告**、不校验包完整性也不修。i18n 也没有 `repairedNotice`/`recheckedNotice`。
- **落地 file:line（P7-ep 已按此落地）**：`internal/extensions/repair.go`（新）`Repair(paths, id) (RepairReport, error)`
  ——校验清单与包目录（`ManifestPath()` / `PackageDir()` 的存在性与 `ManifestDigest` 与
  `ApprovedManifestDigest` 是否一致）、必要时重写 `manifestPath`；`RepairReport{Repaired bool}`；
  `internal/settingsui` 新增 `Actions.RepairIntegration` + broken 分支的文案
  （`IntegrationsRecheck`，原版 `settings.extensions.recheck`）；`internal/shell` 接
  `a.repairIntegration` 并加两条通知 `IntegrationRepaired` / `IntegrationRechecked`（原版
  `repairedNotice`/`recheckedNotice`）。**与本轮 F4e 的关系**：`repairKind` 现在对绑定类失败复用
  Reconnect；repair 落地后可把「broken 行的入口」拆回原版的两支，并在 `repairKind` 里去掉那处有意偏离。
- **验收**：`internal/extensions/repair_test.go`——包目录缺失/清单 digest 与批准记录不一致/两者都正常
  三条表，钉 `Repaired` 的真假与仓库条目的改动；`internal/settingsui` 一条像素断言 broken 行画
  Recheck 且点击落到 `RepairIntegration`；`internal/shell` 一条钉两条通知按 `Repaired` 二选一。
  变异核查：把 `Repair` 的「digest 不一致」支改成返回 `Repaired=false` → 第一条红。

#### F4f · 移除确认条按 kind 说话（**本轮落地，P7-eo**）

- **原版对照**：`main:src/extensions/RemovalConfirmation.tsx:41`——确认条的**标题/正文/确认按钮**与
  菜单项用**同一个 kind key**（`ExtensionsPanel.tsx:2140-2151` 的 `removalTextKey` 按 `removalKind` 选
  `deleteCustom|uninstall|disconnect|removePackage` 词干），所以 package/system 各自有
  `removePackageTitle` / `disconnectTitle` 这类词（`i18n.ts:521-542` 一带）。
- **P7-eo 落地**：`internal/settingsui/settingsui.go` 新增 `Integration.removalWords(copy)`（四分支取
  标题/正文，custom 仍追加 `RemoveCustomWarning`），`removeConfirmation` 改用它，确认按钮改用
  `integration.removalLabel(copy)`（与菜单同一个词干）；`i18n.go`/`copy.go` 新增 disconnect/removePackage
  两对 Title/Detail 并删掉变死的 `RemoveConfirm`。详见上文 P7-eo 记录。
- **验收（已过）**：`integrationremoval_test.go` 两条新测试（纯映射 + 像素反向断言）；既有的
  `view_test.go:TestIntegrationsPage`、`shell/settingsescape_test.go:TestEscapeAnswersTheSettingsQuestionFirst`
  按新口径改写（它们原本钉的是旧的错口径）；变异核查：system 支改回 `RemoveTitle` → 立刻红。

#### F4g · `ExtensionsPanel.tsx` 尚未逐函数对照的流程

- **原版对照（逐个入口，行号已钉）**：
  - 安装文件夹：`main:src/ExtensionsPanel.tsx:1430-1440` 的 `reviewLocalManifest`
    （`extensions_local_manifest_review` → 名字/runtime/platforms/source + 权限清单）与
    `:1554` 的 `extensions_install` 提交；—— **已落地 P7-eq**（见下）：`internal/shell/system.go`
    的 `localInstallReview`/`localInstallReviewText` + `internal/shell/shell.go` 的
    `installPackageFolder`/`reviewLocalInstall`（无条件先审后装），文案取原版
    `LocalInstallDialog.tsx` / `i18n.ts:640-645,553`。**残留（有意偏离）**：原版
    `extensions_local_manifest_review`（`commands/extensions.rs:1480-1482`）对**已安装**的 id 直接报
    “Extension is already installed” 而拒绝审阅；本构建把同 id 的本地包当作**更新**走同一 review
    （`PrepareLocal` → `Commit`），因为本构建支持就地更新，且这是更常见的意图。
  - 权限审批：`:1349-1400` 的 `connectRecommendedAt`（先 `extensions_recommended_permissions`，
    有权限才弹 `pendingPermissionReview`）与 `:2594-2620` 的 `extension-permission-backdrop`
    弹窗（标题按 `manifestSuggestion` / `recommended` 二选一 + `PermissionTierList` + 边界句）；
    —— **已落地 P7-er**（见上）：`internal/shell/system.go` 的
    `recommendedInstallReview`/`recommendedInstallReviewText` + `permissionTierLines`，
    `internal/shell/shell.go` 的 `connectRecommended`/`reviewRecommendedConnect`（`Declared` 非空先审），
    文案取原版 `install.rs:2175-2240` / `i18n.ts:465-469,553`。**残留（有意偏离）**：原版标题的
    `manifestSuggestion` 一支（`confirmConnectManifest`）在本构建不可达（没有“约定位置清单”这类
    detected 行）；可执行路径行只在 `Prepared.ExecutablePath` 非空时出现（推荐流程审阅时不解析候选）。
  - 运行参数（运行工具选择）：`:1393-1401` 的 `connectRecommended`、`:1975-1988` 的 `reconnectSystem`
    与 `:2623-2648` 的 `pendingToolSelection` 弹窗（多个候选才弹，`chooseSystemTool`）；
    —— **已落地 P7-es**（见上）：`internal/extensions/candidates.go` 的
    `ExecutableCandidate`/`LookTools`/`SystemRuntimeCandidates`、`internal/extensions/reconnect.go` 的
    `ReconnectAt`、`internal/shell/system.go` 的 `chooseSystemTool`/`chooseSystemToolText`、
    `internal/shell/shell.go` 的 `recommendedToolPath`（connect 先选再审、reconnect 选中即绑），
    文案取原版 `i18n.ts:472-473`。**残留（有意偏离）**：候选只有 PATH 一种来源（`Sources` 恒 `["path"]`）；
    打分只有「记录路径优先，否则全列」；用原生多按钮 message box 而非可聚焦 listbox。
  - 输出回看：`:2078-2132` 的 `handleDriftNotice`（`OperationProgress` 漂移提示）；
    —— **已落地 P7-et**（见上）：`internal/extensions/driftversion.go` 的 `CheckToolVersionDrift`
    （记录版本 vs 当前版本的比较 + 资格闸 + 失败软着陆），`internal/shell/driftreprobe.go` 的
    `reprobeDriftedIntegrations`（列表驱动的后台单飞行循环，复刻原版 `try_start_drift_reprobe`）+
    `raiseDriftNotice`（一条 `reprobeNotice` success toast，带 `viewDetails` 动作），
    `RefreshIntegrations` 末尾接线；判定与 gate 用已落地的 `extensions.DecideDriftNotice`。
    **残留（有意偏离）**：行数据来自磁盘刷新而非原版的 `setExtensions` 就地改写；
    `viewDetails` 动作落到 Integrations 页而非逐集成抽屉（本构建无抽屉）。
  - 导入导出：`extensions_export` / `extensions_import` 的调用点（`onImport`/`onExport` 一带）；
    —— **已落地 P7-eu**（见上）：`internal/settingsui/transfer.go` 的 `ImportReport`/`transferReport`、
    `internal/shell/sync.go` 的 `beginTransfer`/`finishTransfer`（单飞行闸）/导出 success toast/导入
    报告块，`internal/extensions/sync.go` 的 `SyncReport.ExcludedFields`；文案逐字取
    `i18n.ts:509-520`。**残留（有意偏离）**：原版导入全或无（失败回滚，失败通知以 `importRolledBack`
    打头），本构建 `ImportSync` 按条应用（P3-l），失败不回滚，故失败通知只给原版的
    `importFailedDetail`/`importFailedMore`，不加回滚抬头。
- **本构建入口**：`internal/settingsui/settingsui.go` 的 `localToolSection`（`:2860` 一带）、
  `installationReview`/`InstallFromFolder`、`confirmInstallPermissions`（shell）、
  `permissionAudit`（`:1741`）、`runForm`/`runOutput`、`importIntegrations`/`exportIntegrations`（shell）。
- **待做**：按上面五个入口逐条比对，每条都要么「已有等价实现 → 在文档里记下 file:line 与结论」，
  要么写成 F4g-n 的具体条目。**验收**：每条至少一条测试（对已有等价实现的条目，测试写「原版那句话/那个
  状态在本构建的哪个函数被谁读」）。
- **注意**：这是**对照任务**，不要一次性大改；每轮只取一条入口（与 F4a–F4e 的节奏一致）。
- **剩余**：无——F4g 五条入口（安装文件夹 P7-eq、权限审批 P7-er、运行工具选择 P7-es、输出回看
  P7-et、导入导出 P7-eu）与 **F-new（脚本编辑器 P7-ev）** 全部对照落地。清单上再无未落地条目；
  下一轮候选是回到 `localToolSection` / `runForm` 等其他入口的逐函数审计（`ExtensionsPanel.tsx`
  尚有未逐函数对照的流程），或先由本轮把新条目写实。

#### F3a · 控件透镜（**本轮落地，P7-em**；原版有一份可逐条对齐的 lens census）

- **原版对照（不用猜：原版自己有一份清单）**：`main:tests/glass-reaxis.test.ts:47-90`
  的 `every named control draws a lens rung, not a bare ring` 逐条点名了必须画透镜的控件，
  并区分 raised（`--elev-0`）与 recessed（`--elev-track`）：
  - raised：`.extensions-action-button`、`.app-toast__action`、`.settings-save-alert button`、
    `.settings-option--active`、`.settings-sidebar__item:hover`、`.settings-section__card`、
    `.launcher-result__icon`、`.launcher-action-bar__icon`；
  - recessed：`.settings-options--inline`、`.extension-custom-mode`、`.settings-switch`；
  - 只有 hover 才亮透镜：`.extensions-icon-button:hover`、`.toolbar-button:hover`。
  token 定义在 `main:src/styles/base.css:188`（`--elev-0: var(--glass-lens-stack)`）、`:231`
  （`--elev-track: var(--glass-lens-stack-inset)`），档位倍率是 `:784/:791/:798` 的
  `--glass-lens-scale: 0.5/1/1.6`（本构建 `internal/glassmap/glassmap.go:88-101` 数值已逐项相等）。
- **本构建**：`internal/theme/theme.go:577-584` 的 `Tokens.Lens` 就是 `--glass-lens-stack`，
  但只被四处调用（`internal/launcher/glyphs.go:164`、`internal/settingsui/rows.go:89`、
  `internal/settingsui/settingsui.go:877`、`internal/terminalui/terminalui.go:812/946/991`），
  而设置面/启动器的**按钮、分段控件、开关轨道**大多由框架的 `ui.Button`/`ui.Switch` 直接绘制
  （例如 `internal/settingsui/settingsui.go:1723` 的 `ui.Button(c, …)`），**不带透镜**——这就是
  P7-ej 实测「控件透镜几乎不贡献像素（≤0.4 RGB）」的根因。
- **落地 file:line（建议顺序）**：
  1. 在 `internal/settingsui` 加 `func (a *App) actionButton(c *ui.Context, label string) ui.Element`
     = `tokens.Lens(ui.Button(c, label))`（先验证框架允许对 Button 加 inset shadow；不行则用
     `ui.ButtonBase` + 自己填 `tokens.Control`/`ControlEdge`），把所有 `ui.Button(c, …)` 调用点换成它；
  2. `internal/control/control.go:20-45` 的轨道只画 `LensRim` 一层，补成
     `--glass-lens-stack-inset` 的三层（rim/sheen/edge 反向，见 `base.css:593-597`）；
  3. 分段控件（`internal/settingsui` 的 options/segmented 行）按 `.extension-custom-mode` 的 recessed 透镜处理。
- **验收**：改写/新增 `internal/shell/glass_test.go` 的像素测试——同一 `main_opacity` 下，
  **控件**（图标底板 / 设置面按钮）在 regular 与 liquid 之间的 RGB sum 差 **≥ 一个明确的阈值**
  （P7-ej 的实测是 ≤0.4，即「不可测」），而**帧专属带**（y=90..120，不含控件）的差 ≤0.4；
  另加一张对照表把 `glass-reaxis.test.ts:47-90` 的每个选择器映射到本构建的绘制点
  （写成 Go 表 + 注释，逐条点名），任何一个落点丢失透镜就红。
- **风险**：按钮外观会变（透镜是 inset 阴影，不是边框），需目视确认暗/亮两套主题；改动只碰
  `Tokens.Lens` 的**使用点**，不动 `internal/theme` 的数值。
- **P7-em 落地结果**：新增 `Tokens.LensInset` + `TrackEdge`（`theme.go`）；`settingsui` 新增
  `actionButton`/`switchControl`/`segmentedControl` 并把扩展页的按钮/开关/分段控件全部改走它们，
  另补侧栏 hover、头栏图标按钮 hover、保存失败横幅 ✕ hover；`control.Segmented` 轨道/选中段、
  `shell` 的 toast 动作 chip、`terminalui` 的栏按钮 hover 均接上。`internal/shell/glasscensus_test.go`
  把原版 13 条选择器逐条映射到绘制点（file + 透镜调用 + 像素测试，或如实标 pending 理由）。
  像素验收在**常背景**隔离帧 sheen 的测试里完成（theme/control/settingsui/launcher 四包），
  控件 RGB sum 随 0.5/1/1.6 严格上升、regular→liquid 差值远超 P7-ej 的 ≤0.4。
  `.extensions-icon-button:hover` 一条如实记为 pending（本构建扩展页动作都是
  `.extensions-action-button`，没有「静止透明、hover 才亮」的独立图标按钮）。
  **剩下的是 F3b**：把**帧** sheen 改回固定口径（控件一侧已成立）。

#### F3b · 帧 sheen 回固定口径（**本轮落地，P7-en**）

- **原版对照**：`main:src/styles/launcher.css:119` 的卡片是
  `linear-gradient(180deg, var(--sheen-top), var(--sheen-bottom)), var(--launcher-tint)`，而
  `--sheen-top/-bottom` 是**固定 token**（`base.css:628-629` dark 0.046/0.01、`:906-907` light 0.66/0.22），
  **不随 `[data-glass]` 变**；档位的可见 rung 只在控件透镜与窗口材质上。
- **本构建（已修）**：`internal/theme/theme.go` 曾把帧的 sheen 写成
  `sheen = spec.TintAlpha * spec.LensScale`（P7-c/P7-d 为「3 档看不出区别」加的耦合），于是帧的亮面
  随档位变，而原版的帧 sheen 固定。
- **P7-en 落地**：`internal/theme/theme.go` 改为 `spec.Glass` 时取按外观固定的
  `capAlpha(p.sheenTop/p.sheenBottom, spec.TintAlpha)`（`off` 为 0），`scaleAlpha` 收窄为只管
  控件透镜；`internal/theme/theme_test.go` 两条新测试钉住「三档 sheen 完全相等且等于固定 token 的
  cap 值」与「固定 token 而非 tint 缩放」；`internal/shell/glass_test.go` 的
  `TestTheGlassStopsAreVisiblyDifferent` 改写并改名为 `TestTheFrameSheenIsFixedAcrossStops`
  （帧顶带三档同色、off 无 sheen；控件一侧由 P7-em 的四组像素测试钉住）。详见上文 P7-en 记录。
- **验收（已过）**：同一 `main_opacity` 下**帧**亮面在 regular 与 liquid 之间相同，而**控件** lens
  强度按 0.5/1/1.6 单调上升；`internal/shell/transparency_test.go` 的 `FaceFill`/`baseAlpha` 像素测试
  全绿（帧 sheen 参与透明度的补偿计算，改完重跑过这一套）。

#### F-new · 脚本内容编辑器换用 mygo `ui.editor`（v0.4.8 新包的自然用点；**已落地，P7-ev**）

- **原版对照**：`main:src/ExtensionsPanel.tsx` 的自定义工具表单里，脚本内容是一个带高亮的代码编辑区
  （`src/extensions/` 下的 script 编辑器组件；词条 `settings.extensions.customScript*`）。
- **本构建**：`internal/settingsui/settingsui.go:3258` 是一个纯粹的 `ui.TextArea(c, script)`（多行文本
  框，无行号/无高亮/无折行控制）。
- **落地（P7-ev，见「最近几轮」P7-ev 记录）**：`internal/settingsui/scripteditor.go` 的
  `scriptEditor`/`scriptEditorView`（跨帧持久的 `*editor.State`，语言同步 + 外部 seed 同步 + 渲染后写回），
  `settingsui.go` 的 `scriptRows` 换 `editor.View`。落地前三件事的结论：
  (1) inputtext 边界**不适用**（脚本内容是 provider 文件，不是 PTY 字节——边界存在的理由是终端
  字节必须匹配键盘字节，见 `scripteditor.go` 注释）；(2) 编辑器处理自己的键输入，表单提交按钮
  点击触发，`Submitted()` 链不打断（`TestScriptEditorTakesAnOutsideDraft` 端到端验证）；
  (3) 不引入 LSP（补全由 app 主动 `ShowCompletions` 触发，不接 language server）。
- **验收（已过）**：`scripteditor_test.go` 五条——`TestScriptEditorDrawsLineNumbers`（gutter 行号
  像素断言：hint 下方左缘 48×110 条带内与背景色差 >25 的像素 ≥30；tester 不把 gutter 数字注册为
  文本元素，同 `view_test.go:2178` 的 placeholder 注释）、`TestEveryScriptLanguageHasAnEditorLanguage`
  （8 语言全有语法，ps1 走内嵌 `editor.Language`）、`TestScriptEditorTakesAnOutsideDraft`（外部 seed
  → connect 提交逐字相等）、`TestScriptEditorFollowsTheLanguageSwitch`（grammar 跟随选择器）、
  `TestScriptEditorHandsTheUsersTypingOn`（键入 → connect 提交含键入内容）。
  原「粘贴带尾换行」边界回归不适用（同上 (1)），如实记录；变异核查的更正见上文 P7-ev 记录
  （单删渲染末回写不会让 outside-draft 条红，钉它的是入向的 `st.Buffer.Set(*script)`）。

### 具体待办（P7-ff 重写；第 1 条 P7-ff、第 2 条 P7-fg 均已落地；清单归零）

P7-fe 收尾后「尚未做（P7-fb 重写）」归零。P7-ff 按审计入口 `main:src/extensions/RunParamForm.tsx`
（运行参数表单）与 `main:src/extensions/ExtensionRow.tsx` 的 Run 披露逐字段对照，把清单重写为下面
两条带 file:line / 原版对照 / 验收标准的具体待办，并取第 1 条落地。

#### F-run-1 · 运行参数表单的框（**本轮落地，P7-ff**）

- **原版对照**：`main:src/extensions/RunParamForm.tsx` —— `:40-43` 的 `customRunValues` 标题 +
  `customRunValuesHint` 提示；`:53-60` 每行 label 旁的 `<small>customParamType.{kind}` 与必填
  `<em>customParamRequired`（词条 `i18n.ts:725-731`）；`:131` 的 `customParamInjectionNote`；
  `:136-138` 按 `outputMode` 二选一的 `customRunOutputHintTerminal`/`Background`
  （词条 `i18n.ts:772-773`）。
- **本构建（改前偏离）**：`internal/settingsui/settingsui.go:4165` 的 `runForm` 直接画卡片行，
  没有标题/提示；每行第二行用 **placeholder** + `ConfigRequired`（“required”）而不是种类词 +
  “Required”；底部只有一句自造 `RunOutputHint`（“The values are passed as separate arguments.”）。
- **落地 file:line（已按此落地）**：`internal/i18n/i18n.go` 与 `copy.go` 新增 `RunValues`/
  `RunValuesHint`/`RunRequired`/`RunInjectionNote`/`RunOutputHintTerminal`/`RunOutputHintBackground`
  并删掉 `RunOutputHint`；`settingsui.go` 的 `runForm` 加标题+提示、把 `sublabel` 改读新
  `runParamKindLabel`、卡片后加注入说明与按 `integration.OutputMode` 二选一的路由提示；
  `Integration.OutputMode` 新字段 + `internal/shell/shell.go` 的 `integrationList` 用
  `integration.Manifest.Route()` 投影。
- **验收（已过）**：`internal/settingsui/runform_test.go` 两条、`internal/shell/runroute_test.go` 一条
  （详见「最近几轮」P7-ff 记录）。变异核查：`sublabel` 换回 placeholder / 标题换掉 → 像素测三处红。

#### F-run-2 · Run 披露与 Cancel（**本轮落地，P7-fg**）

- **原版对照**：`main:src/extensions/ExtensionRow.tsx:381-399` —— connected 行有一个
  `extensions-icon-button--row` 的 Run 按钮（`Play` 图标，`aria-expanded={hasParams ? runFormOpen : undefined}`，
  不可运行时 disabled 且 title 说明），`:459-471` 的表单只在 `runFormOpen` 时画、并给 `onCancel`；
  `main:src/ExtensionsPanel.tsx:1712-1727` 的 `openRunForm`（有参数 → 打开表单并按会话内存 seed）、
  `:1740-1744`（无参数 → 直接 `runExtension`）；`RunParamForm.tsx:96-99` 的 Cancel 动作
  （词条 `customRunCancel` = “Cancel”）。
- **本构建（偏离）**：`internal/settingsui/settingsui.go:1723-1725` 的集成行**无条件**调 `runForm`，
  于是有参数的集成表单**常驻**、没有关闭入口；`runForm` 的动作行只有 Run，没有 Cancel；
  `grep -rn 'RunWithParams' internal/settingsui` 只有 `runForm` 一处调用 → **无参数的已连接集成在设置页
  根本无法手动运行**（原版无参数时 Run 直接跑）。
- **落地 file:line（已按此落地）**：
  1. `App` 新增 `runOpen map[string]bool`（仿 `outputOpen`/`configOpen`）；`runForm` 开头加
     `if !a.runOpen[integration.ID] { return }`；
  2. `integrationRow` 的动作区（enable 行 `a.row(copy.IntegrationsEnable, …, func(){…})` 内、与
     Check/Reprobe/移除按钮同排）加 Run 按钮：`a.Actions.RunWithParams != nil && integration.Running
     && !integration.Broken` 时——`len(integration.Params) > 0` → `a.runOpen[id] = !a.runOpen[id]`（并清
     `runError`）；否则直接 `a.Actions.RunWithParams(id, nil)`；不可运行的行保留按钮但 `Disabled(true)` +
     `Tooltip(copy.RunActionUnavailable)`。新增三个词 `RunAction`（`customRun` = “Run”/“运行”）、
     `RunActionUnavailable`（`customRunUnavailable`）、`RunCancel`（`customRunCancel` = “Cancel”/“取消”）；
     现有 `RunLabel`（“Run with these values”）仍是表单确认按钮的词，两者不混；
  3. `runForm` 动作行加 Cancel（`copy.RunCancel`）→ `a.runOpen[id] = false` 并清 `a.runError`。
- **验收（已过）**：`internal/settingsui/runform_test.go` 的 `TestRunButtonRunsOrOpensTheForm` 两条子测
  （无参数的 Running 行点 Run → `RunWithParams(id, nil)`；有参数的行初始**不画**表单、点 Run 后画、点
  Cancel 后收起）。既有 `internal/settingsui/view_test.go` 的三条 run 表单测试
  （`TestRunParamForm` / `TestRunParamFormShowsPerRowReasons` / `TestRunParamFormShowsTheBackendRefusal`）
  与 `runform_test.go` 的 `TestRunFormDrawsTheReferenceFrame` 已改为先点 Run 展开。变异核查：把 `runForm`
  的 `runOpen` 挡板摘掉 → “初始不画表单”条立刻红。
- **残留（随本条的取舍记录）**：原版表单值来自 `runParamMemory`（会话内存，仅在**新打开**表单时
  `seedParamValues`），本构建的 `a.runValues[id]` 常驻且不因关闭重置。收起/展开时是否重置为默认值，
  在本条一起决定并写进注释（本构建常驻编辑态、不丢用户输入，倾向保留）。

### 具体待办（P7-fh 重写；第 1 条 P7-fh、第 2·3 条 P7-fi 均已落地；清单归零）

P7-fg 收尾后「具体待办（P7-ff 重写）」归零。本轮按审计入口 `main:src/extensions/DisableConfirmation.tsx`
（`src/extensions/` 下最后一个尚未逐字段对照的组件；其余七个组件都已在前几轮对照落地）逐字段比对，
把清单重写为下面 3 条带 file:line / 原版对照 / 验收标准的具体待办，并取第 1 条落地。

#### F-disable-1 · 停用确认条回到原版的横条与词（**本轮落地，P7-fh**）

- **原版对照**：`main:src/extensions/DisableConfirmation.tsx` —— `:60` 的
  `className="extensions-notice extension-discard-bar"`（横条：`AlertCircle` 15px + **一个** `<span>`
  连成一句 + 同排 Cancel/Disable）；`:63-64` 的
  `{t("settings.extensions.disableTitle", {name})}{" "}{t("settings.extensions.disableDescription")}`；
  `:66` 的 Cancel 用 `settings.extensions.cancel`；`:67-68` 的 confirm 用 `settings.extensions.disable`
  （“Disable integration”/“禁用集成”）并带 `PowerOff` 14px 图标。CSS：`src/styles/extensions.css:146-172`
  的 `.extensions-notice`（`color: --text-secondary`、`background: --glass-control`、`padding: 8px 12px`）
  与 `:2455-2460` 的 `.extension-discard-bar`（`margin: 0 12px; padding: 8px 10px`）。
- **本构建（改前偏离）**：`internal/settingsui/settingsui.go` 的 `disableConfirmation` 是纵向卡片
  （`ui.Column`：加粗标题 + 弱化描述 + 按钮行）；无前导图标；confirm 词自造 “Disable”/“停用”；
  英文 description 意译（`internal/i18n/copy.go:662`）。
- **落地 file:line（已按此落地）**：`internal/settingsui/settingsui.go` 的 `disableConfirmation`
  改横条（`ui.Row` + `mustGlyph("alert")` + 一个 `ui.Text` 连成一句 + 同排两个 `ui.Button`）；
  `internal/i18n/copy.go` 的 `DisableDescription`（en 逐字）、`DisableConfirm`（en/zh = “Disable
  integration”/“禁用集成”）。详见「最近几轮」P7-fh 记录。
- **验收（已过）**：`internal/settingsui/disablebar_test.go` 的 `TestDisableBarIsOneNoticeSentence`
  （整句一个文本节点 + confirm 逐字词 + 两个按钮同排且在其右 + 点 confirm 落动作）；
  `view_test.go:1010` 的 `Click("Disable")` 改 `Click("Disable integration")`。

#### F-disable-2 · 删除确认条也走同一条横条（**本轮落地，P7-fi**）

- **原版对照**：`main:src/extensions/RemovalConfirmation.tsx:40-44` 与 `DisableConfirmation.tsx` 共用
  同一个 `.extensions-notice .extension-discard-bar` 横条形状（删除条多一个 `--error` 底色与
  `extensions-action-button--danger` 危险按钮），只是文案按 kind 不同（P7-eo 已落地文案）。
- **本构建（改前）**：`internal/settingsui/settingsui.go` 的 `removeConfirmation` 仍是纵向卡片（P7-cf 当年
  「照抄停用确认的页内形状」，而当时停用条本身就是卡片）。P7-fh 把停用条改成横条后，两条确认条形状
  不再一致。
- **落地 file:line（已按此落地）**：把 P7-fh 在 `disableConfirmation` 里写的横条抽成共享 helper
  `func (a *App) noticeBand(c *ui.Context, danger bool, sentence string, answers func())`，
  `removeConfirmation` 改用它（图标仍取 alert，危险按钮仍在最后且带 `t.Danger` 墨色）；顺带按
  `RemovalConfirmation.tsx:41` 把句子改成 `Integration.removalSentence`（标题 + 仅生成项才有的警告），
  不再画原版从不显示的 `*Description` 正文。详见「最近几轮」P7-fi 记录。
- **验收（已过）**：`internal/settingsui/disablebar_test.go` 的 `TestRemovalBarIsTheSameNoticeBand`
  （整句一个节点、旧正文不出现、两个按钮同排且危险按钮在最后）与 `TestTheRemovalBandWearsTheErrorTint`
  （整句墨色 R−B 均值：停用 −0.64 / 删除 +11.86）；`view_test.go` 的
  `TestGeneratedIntegrationRemovalBar` 改成原版的一句警告。变异核查：`noticeBand` 忽略 `danger` → 红。

#### F-disable-3 · 两条确认条共享一个 Cancel 词（**本轮落地，P7-fi**）

- **原版对照**：原版 Cancel 都取同一个键 `settings.extensions.cancel`（`i18n.ts:554`）；本构建把它拆成
  两个字段 `DisableCancel`（`internal/i18n/copy.go:663`）与 `RemoveCancel`（同文件），两个值恰好都是
  “Cancel”/“取消”。
- **落地 file:line（已按此落地）**：`internal/i18n/i18n.go` 合并为一个 `IntegrationsCancel`，
  `copy.go` en/zh 各留一份，`settingsui.go` 两处调用点改读它。
- **验收（已过）**：`TestTheRemovalBandWearsTheErrorTint` 里钉住两条横条都取同一个 `IntegrationsCancel`；
  结构化 Copy 缺字段会在编译期报错。

### 具体待办（P7-go 重写；第 1 条 P7-go、第 2 条 P7-gp 已落地，余下第 3 条）

P7-gn 收尾后清单归零。本轮按审计入口 `main:src/styles/settings.css` 的 `.settings-section__heading` /
`__heading-main` / `__label` / `__hint`（此前只对照过行、文本动作与空态，标题行本身没有 file:line
对照）逐字段比对，把清单重写为下面 3 条带 file:line / 原版对照 / 验收标准的具体待办，并取第 1 条落地。

#### P7-go-1 · 分组标题行回到原版的 `.settings-section__heading`（**本轮落地，P7-go**）

- **原版对照**：`main:src/styles/settings.css:411-419`（`.settings-section`：`display:grid;
  gap: calc(var(--u)*8)`）、`:421-429`（`.settings-section__heading`：`display:flex;
  align-items:flex-end; justify-content:space-between; gap:calc(var(--u)*12); padding:0 calc(var(--u)*2)`）、
  `:430-434`（`.settings-section__heading-main`：`display:grid; gap:1px`）、`:440-448`
  （`.settings-section__label`：`--text-strong`/`--text-title`/620）、`:1198-1206`
  （`.settings-section__hint`：`padding:0 calc(var(--u)*2)`、`--text-tertiary`/`--text-body`/500）；
  `main:src/settings/ShortcutsPage.tsx:383-395`（heading 里 label + SettingsAction）、`:424`
  （说明在 `</SettingsCard>` 之后）；`main:src/settings/SettingsRows.tsx:200-216`（`SettingsAction`）。
- **本构建（改前偏离）**：`internal/settingsui/rows.go` 的 `section`/`sectionFoot` 把标题直接画在
  内容左边、说明紧贴标题下（8u）；`sectionWithAction` 的标题行 `AlignItems(ui.Center)`。
- **落地 file:line（已按此落地）**：`internal/settingsui/rows.go:61` 的 `sectionHeading`（flex 行 +
  End + SpaceBetween + Space(3) gap + 2u padding）、`:83` 的 `sectionHint`（卡片之后、2u 内缩）、
  `:40`/`:363` 的两个调用方。详见「最近几轮」P7-go 记录。
- **验收（已过）**：`internal/settingsui/sectionheading_test.go` 三条 + `actionmetrics_test.go` 的
  `TestSectionHeadingActionSitsAtTheRightEndBottomAligned`。

#### P7-go-2 · 深链标题的说明是原版的 `--inline` 说明，应留在 heading-main 里（**P7-gp 已落地**）

- **原版对照**：`main:src/settings/DeepLinkRow.tsx:61-64` 的 heading-main 里放 label 与
  `<p className="settings-section__hint settings-section__hint--inline">{t("settings.deepLinkHint")}</p>`；
  `main:src/styles/settings.css:1209-1211` 的 `--inline { padding: 0 }`。也就是说，原版**唯一**一条
  骑在标题块内的说明就是深链页这条。
- **本构建（改前偏离）**：`internal/settingsui/settingsui.go:2606` 的
  `a.section(c, copy.DeepLinkTitle, copy.DeepLinkHint, ...)` 在 P7-go 之后把这条说明画成卡片之后的
  `sectionHint` 行，与「标题块内第二行」的原版位置不符。
- **落地 file:line（已落地）**：`internal/settingsui/rows.go:83` 的 `sectionHeadingInline` 给
  `heading-main` 用 `ui.Column(...).Gap(1)` 复刻 1px 栈、`:56` 的 `sectionInline` 把卡片画在
  inline 说明之后，`internal/settingsui/settingsui.go:2606` 的深链段改走它。详见「最近几轮」P7-gp 记录。
- **验收（已过）**：`internal/settingsui/sectionheading_test.go` 的
  `TestTheDeepLinkHintRidesInsideItsHeading`（说明在卡片之前、与标题同列、间距 == 1px）。

#### P7-go-3 · Sessions 标题行缺原版的排序记号（刷新图标键按已记录偏离保留缺席）（**P7-gq 已落地**）

- **原版对照**：`main:src/settings/SessionsPage.tsx:116-135` 的 heading 里除标题外还有两个键——
  `session-manager__sort-toggle`（`ArrowUpDown` 13px + 排序词）与 `session-manager__icon-button`
  （`RefreshCw` 14px，`aria-label`/`title` 取 `terminal.sessionsRefresh`，`disabled={loading}`）；
  CSS 见 `main:src/styles/settings.css` 的 `.session-manager__sort-toggle` / `__icon-button`。
- **本构建（改前偏离）**：`internal/settingsui/settingsui.go:1178-1180` 的
  `sectionWithAction(c, copy.PageSessions, copy.SessionsActive, order, ...)` 只画**一个**文本动作
  （排序词），既没有 `ArrowUpDown` 记号，也没有独立的刷新图标键。
- **落地 file:line（已落地）**：`internal/launcher/glyphs.go` 新增 `arrow-up-down` 记号；
  `internal/settingsui/rows.go:405` 的 `sectionWithControls`（`sectionWithAction` 委托它）把标题行
  动作槽放宽到多控件；`internal/settingsui/settingsui.go:1181` 的 Sessions 段改走
  `sectionWithControls` + `actionWithGlyph(c, "arrow-up-down", order, ...)`。刷新图标键**不画**：本构建
  的会话在进程内、列表当帧由 `a.Sessions()` 取得（P7-aj 已记「轮询不做」），没有可重跑的取回，画一个
  永不生效的键比不画更骗人。详见「最近几轮」P7-gq 记录。
- **验收（已过）**：`internal/settingsui/sessionssort_test.go` 的
  `TestTheSessionsSortControlLeadsWithTheReferenceMark`（排序词左侧 5u 沟里是 accent 记号墨迹；
  `arrow-up-down` 在字形表里；按下后词翻面）。刷新键缺席部分不再有测试——它是不画的控件。

### 具体待办（P7-gr 重写；3 条已全部落地：第 1 条 P7-gr、第 2 条 P7-gs、第 3 条 P7-gt）

P7-gq 收尾后清单归零。本轮按审计入口 `main:src/settings/GeneralPage.tsx` 的**顶层 section 顺序**
（此前只对照过单卡内的行序 P7-ca、分组归类 P7-by/P7-bz，页面级的 section 先后从未有 file:line
对照）逐段比对，把清单重写为下面 3 条带 file:line / 原版对照 / 验收标准的具体待办，并取第 1 条落地。

#### P7-gr-1 · General 页 section 顺序：材质最后（**本轮落地，P7-gr**）

- **原版对照**：`main:src/settings/GeneralPage.tsx:322`（Appearance）、`:405`
  （`settings.group.startup` = Window behaviour）、`:482`（`settings.group.menuBar`）、`:533`
  （`terminal-appearance-settings`）、`:558`（`settings.material`）；容器
  `main:src/styles/settings.css:379-387`（`.settings-page` 网格，`gap: 22u`）。
- **本构建（改前偏离）**：`internal/settingsui/settingsui.go:2712` 的 `general` 把材质 sectionFoot
  画在 Appearance 之后、Window behaviour 之前。
- **落地 file:line（已落地）**：`internal/settingsui/settingsui.go` 的材质 `sectionFoot` 移到
  GroupTerminal section 之后，成为页面最后一个 section。详见「最近几轮」P7-gr 记录。
- **验收（已过）**：`internal/settingsui/view_test.go` 的
  `TestTheGeneralSectionsKeepTheReferenceOrder`（五个 section 标题的顺序 + 材质最后 + 玻璃控件/脚注
  在材质标题之下）。变异核查：移回 Appearance 之后，四条断言同时红。

#### P7-gr-2 · General 页 section 之间的沟是原版的 22u，本构建是 16u（**本轮落地，P7-gs**）

- **原版对照**：`.settings-page { display:grid; gap: calc(var(--u) * 22) }`
  （`main:src/styles/settings.css:379-387`）——页头与每个 section 都是它的网格子项，所以相邻 section
  之间也是 22u。
- **本构建（改前偏离）**：`internal/settingsui/settingsui.go:2715` 的 `general` 顶层列用
  `Gap(c.Theme().Space(4))` = 16u；其它页（`sessions:1172`、`integrations:1311`、`shortcuts:2347`、
  `about:2583`）都用 `Space(5.5)` = 22u，General 是唯一的例外。
- **落地 file:line（已落地）**：新增共享助手 `internal/settingsui/rows.go` 的
  `pageColumn(c, fn)`（`Space(5.5)`），五个页（`general`/`sessions`/`integrations`/`shortcuts`/
  `about`）全部改走它；8u section 列也收敛为 `sectionGroup` 并挂上 `sectionLabel(title)`。详见
  「最近几轮」P7-gs 记录。
- **验收（已过）**：`internal/settingsui/view_test.go` 的
  `TestTheGeneralPageKeepsTheReferenceSectionGap`——General 相邻 section 的沟逐条 == `Space(5.5)`，
  且与 Shortcuts 页的沟相等（用 section 组盒子测量，非文字盒）。变异核查：`pageColumn` 改回
  `Space(4)` → 5 条断言同时红（16.0 ≠ 22.0）。

#### P7-gr-3 · 材质 section 自己的 10u 间距与 10u 内边距（**本轮落地，P7-gt**）

- **原版对照**：`.settings-section--material { padding: calc(var(--u) * 10) 0 }` 且
  `{ gap: calc(var(--u) * 10) }`（`main:src/styles/settings.css:767-772`）、`.settings-cards
  { gap: calc(var(--u) * 10) }`（`:491-495`）——材质区比普通 section（`gap: 8u`，`:411-419`）更松。
- **本构建（改前偏离）**：`internal/settingsui/rows.go` 的 `sectionFoot` 对所有 section 都用同一个
  `Gap(t.Space(2))`（8u），没有材质区自己的 10u gap 与 `padding: 10u 0`。
- **落地 file:line（已落地）**：`internal/settingsui/rows.go` 新增 `sectionGroupSpaced`（可选 gap +
  `Padding(padding, 0)`）与材质专用的 `sectionMaterial`（`gap: 10u`、`padding: 10u 0`，脚注在两张卡
  之下）；`sectionFoot` 被 `sectionMaterial` 取代并删除；`internal/settingsui/settingsui.go` 的
  `general` 材质段改走 `a.sectionMaterial`。10u 内边距落在组盒子**内部**，所以它不改变与相邻 section
  的 22u 页沟（`pageColumn`）。详见「最近几轮」P7-gt 记录。
- **验收（已过）**：`internal/settingsui/materialspacing_test.go` 两条——
  `TestTheMaterialSectionKeepsItsOwnSpacing`（heading→卡、卡→卡、卡→脚注、脚注→脚注四沟 == 10u；
  组上/下内边距 == 10u；同法构建的普通 section 仍是 8u 且无内边距）与
  `TestTheLiveMaterialSectionUsesItsOwnSpacing`（General 页上的材质组同法测量）。变异核查：
  `sectionMaterial` 改回 `Space(2)`/无内边距 → 两条共 8 处断言红。
