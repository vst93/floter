# floter × mygo 重构：当前进度与后续规划

> 交接文档。状态时间：2026-10-09，分支 `mygo-rewrite`（已推送）。
> **仓库现在只有 Go 实现**：Tauri/Rust/React 时代的代码已删除（P6-b）。
> 详细的分轮记录见 `docs/mygo-rewrite-plan.md`（含每轮的决策与「未做」清单）。

## 一句话状态

用 mygo（原生 GPU 自绘 UI + Go）重写的 floter **已经是一个可打包、可运行的完整应用**：单窗口三表面
（启动器 / 设置 / 终端）、搜索内核、扩展平台（含后台运行、列表协议、导入导出、权限审计）、剪贴板、
浏览器、计算器、系统集成、打包与 CI 全部就位。**343 项测试全绿**，三平台都能构建，Linux 打包
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
go test -count=1 ./...         # 343 项测试
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

## 尚未做（按建议优先级）

1. **列表协议的分页**：`page.cursor`/`hasMore` 与滚动加载（需要带着 cursor 重新执行命令）；旧版还有
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
8. **小项**：开机自启需要在真实打包应用上验证一次；集成的**命令别名编辑**（`command_aliases`
   的匹配已实现，设置页的逐命令输入框未做）；**多来源工具清点**（dpkg/rpm/flatpak/snap/nix/…）；
   **能力探测**（manifest `lifecycle.probes` 的执行与 `probeReport`，Go 侧「检查」目前走 diagnose）；
   旧版 `package.json` 归档字段。（导入导出的权限审批已改为**一次汇总**，
   与旧版一致；旧版 `launch_counts` 已在启动时一次性导入 `usage.json`，本构建的记录优先。）

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
