# floter × mygo 重构：当前进度与后续规划

> 交接文档。状态时间：2026-10-12，分支 `mygo-rewrite`（已推送）。框架：mygo v0.4.8。
> **仓库现在只有 Go 实现**：Tauri/Rust/React 时代的代码已删除（P6-b）。
> 详细的分轮记录见 `docs/mygo-rewrite-plan.md`（含每轮的决策与「未做」清单）。

## 一句话状态

用 mygo（原生 GPU 自绘 UI + Go）重写的 floter **已经是一个可打包、可运行的完整应用**：单窗口三表面
（启动器 / 设置 / 终端）、搜索内核、扩展平台（含后台运行、列表协议、导入导出、权限审计）、剪贴板、
浏览器、计算器、系统集成、打包与 CI 全部就位。**887 项测试全绿**，三平台都能构建，Linux 打包
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
go test -count=1 ./...         # 887 项测试
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

> **最新一轮：P7-bk 运行表单的逐参数本地化错误（`paramValueIssues`）** —— P7-bh 重写的具体待办第 4 条。
> P7-bj 把后端拒绝变成 `run_param_*` 键，但表单自己的预检只回一句泛化的
> `RunIncomplete`，且 number 用 `ParseFloat`（放过 `inf`/`NaN`/`0x1p-2`）、boolean 不校验、
> required 不认 default 豁免。现在 `ParamIssues` 返回 `{Index,Key,Label}`，每个非法值
> 的句子印在**它自己那一行下面**，Run 按钮在有错时禁用；输出视图也按语言渲染 `run_param_*`。

**P7-bk 运行表单的逐参数本地化错误（本轮）**

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
  没有运行期的光标 setter），所以终端抽屉里这两行也是下一会话才看得到；字号/行距/内边距/配色
  对打开的会话实时生效。终端抽屉（P7-ab）因此不画 bold/滚轮/选中即复制/滚动条四行（粘贴安全
  那一行 P7-an 已补上）。

**P1-ad 版式与交互对齐（多轮，本轮）**
- 设置面板全量改造为分组卡片语言 + 页面头部 + 侧栏字形/选中 + 权限两层 + 每集成一卡 + 本地工具连接表单。
- 启动器：图标底板/字形集、触发提示、分节标题、模式空状态、平台化徽标、动作栏 Enter 徽标、停靠状态行、
  筛选 chip 轴、共享命令词警告点、调用行。
- 终端：浮动栏 + 呼吸点 + 已结束会话保留 + 字体系探测。

**已完成（P7-v）：历史行的删除键**
- 此前记为框架限制（`editor.wants` 对带修饰键的 Backspace 返回 true，且编辑器分支没有 `claimed`
  守卫，窗口快捷键够不到）。现在由**字段自己的 `HandleInput`** 回答（见 P7-v）：`⌘⌫` / `Ctrl+⌫`
  装填并确认选中历史行的两步删除，`⌃⇧⌫` 清空命令历史。行尾的 ✕ 控件仍在，两条路共用同一个装填状态。
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

> **P7-bk 更新**：P7-bh 重写的 5 条里第 1（路径安全）、第 2（lifecycle 结构校验）、第 3
> （运行期参数校验与 argv 权威）、第 4（表单逐参数本地化错误）已落地。第 5 条（严格解析 + 组合规则）
> 独立，是清单里最后一条。三条明确卡在框架上的能力（`Options.OnCopy`、
> 终端键入的 `HandleInput` 缝、外观四个死旋钮）与一次性维护者动作（`mygo keygen`、`mygo.json` 的
> `updates`/签名、CI 私钥、发布到 GitHub Release）仍未动。

### 具体待办（P7-bh 重写；第 1、2、3、4 条已落地）

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
5. **清单的严格解析与组合规则（`deny_unknown_fields` + `validate_paths` 的组合那半）** —— 原版每个
   manifest struct 都 `deny_unknown_fields`，且只允许三组 distribution/runtime/provider 组合；
   本构建的 `json.Unmarshal` 静默忽略未知字段，也没有组合校验。
   - 原版：`src-tauri/src/extensions/manifest.rs` 的 `#[serde(deny_unknown_fields)]`（`:17,66,230,
     237,251,258,265,335,366`）；`validate_paths`（`:565`）只放行
     `(Local, System, Executable|StaticDescriptor)`、`(Local, Script, Executable|StaticDescriptor)`、
     `(BuiltIn, System, StaticDescriptor)`，且 `Local + StaticDescriptor` 必须有 `descriptor`。
   - 本构建（改前）：`LoadManifest`（`manifest.go:225`）用 `json.Unmarshal`，未知键静默忽略；
     `platform_test.go:250` 的 `runtime.type: "static-descriptor"` 这种非法组合照收。
   - 验收：严格解析（`json.Decoder.DisallowUnknownFields`）拒绝未知字段；组合规则拒绝并修正非法夹具；
     测试钉「未知字段拒绝 / 三组合法组合 / 其余组合拒绝 / Local+StaticDescriptor 缺 descriptor 拒绝」。

> 这 5 条都不卡框架。第 1、2 条已落地；第 3、4 条互相依赖（同一套 `run_param_*` 键的两半），宜连着做。
> 上面 P7-an/P7-bb/P7-bd 的旧清单保留在下方，作为「哪些已经做过」的记录。

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
- **门槛**：`gofmt -l cmd internal` 空、`go vet ./...` 净、`go test -count=1 ./...` 28 包全绿、
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
