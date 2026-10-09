# floter × mygo 重构：当前进度与后续规划

> 交接文档。状态时间：2026-10-09，分支 `mygo-rewrite`（已推送）。
> **仓库现在只有 Go 实现**：Tauri/Rust/React 时代的代码已删除（P6-b）。
> 详细的分轮记录见 `docs/mygo-rewrite-plan.md`（含每轮的决策与「未做」清单）。

## 一句话状态

用 mygo（原生 GPU 自绘 UI + Go）重写的 floter **已经是一个可打包、可运行的完整应用**：单窗口三表面（启动器 / 设置 / 终端）、搜索内核、扩展平台内核、剪贴板、浏览器、系统集成、打包与 CI 全部就位；**测试全绿**（三平台矩阵），macOS 打包产物 16.0 MB（app）/ 4.9 MB（dmg），并已在本机真实数据上验证（旧扩展仓库、剪贴板历史）。**旧的 `src/`（React）与 `src-tauri/`（Rust）已删除**（P6-b），仓库里只有 `cmd/` + `internal/` 的 Go 实现。

## 快速上手（换到新电脑后）

```sh
git clone git@github.com:vst93/floter.git && cd floter
git checkout mygo-rewrite

# 门槛（全部必须过）
gofmt -l cmd internal          # 必须为空
go vet ./...
go test -count=1 ./...         # 194 项
GOOS=linux go build ./...  && GOOS=windows go build ./...

# 开发运行（打开真实窗口）
go run ./cmd/floter                 # 默认启动器
FLOTER_OPEN=settings go run ./cmd/floter
FLOTER_OPEN=terminal go run ./cmd/floter

# 打包（CLI 是 go.mod 里的 tool，无需 npm/bun/node）
go tool mygo build                  # dist/darwin-arm64/floter.app + .dmg

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
| `internal/apps` | 应用扫描（.app/.lnk/.desktop）+ PATH 命令扫描 |
| `internal/clipboard` | 剪贴板历史（text/image/files、去重、容量/30 天保留、原子写） |
| `internal/browser` | 浏览器历史/书签（Chrome/Safari/Firefox，只读 SQLite） |
| `internal/extensions` | 扩展平台：manifest、仓库（未知键保留的原子读写）、inventory、运行时解析、describe/complete/diagnose 协议、命令模式数据、配置注入、本地/npm 安装、权限审批 |
| `internal/shortcuts` | 加速键两种拼写的互转与录制 |

## 已完成（分轮，全部已提交并推送）

**P6-b 清理 + 三平台修复**（本轮）
- 删除旧实现：`src/`、`src-tauri/`、`tests/`、`package*.json`、`node_modules/`、`vite/tsconfig`、
  `public/`、`packaging/arch`、旧 CI、`docs/phase-reports/`、旧审计文档与截图；旧设计文档移入
  `docs/archive/`。注释里对已删文件的引用改写成行为描述。
- README（中英）与 `docs/AGENT-NOTES.md` 重写为纯 mygo 版。
- 修掉两处只在作者机器成立的断言：浏览器 profile 表（补齐 Windows/Linux + Edge 渠道 + profile
  排序与显示名 + 自定义 base 目录）与扩展运行时解析（可注入，不依赖机器装了 `v`）。

**P0/P1 地基与三壳**（`6a6d65c`、`281c600`）
settings 读取子集（保未知键）、glass 映射、单窗口三表面（启动器/设置/终端）、窗口尺寸与聚焦。

**P2 收尾**（`92da843` … `463f6ab`）
- 玻璃保真：读源码证伪「用 `glass.Blur` 复刻 10/22/28px」（Blur 读窗口内场景而非桌面），改为材质阶梯 + 雾层 + `ScrollEdge`。
- 真实搜索：表达式求值 + 三平台应用扫描 + 后台扫描。
- 终端外观：字号/字体/光标/行距/边距/九档调色板/尺寸接线，可热应用。
- 设置页成型：三组 General、Sessions/Shortcuts/About 真内容、滚动边缘、`FLOTER_OPEN`。
- 结果虚拟化；失焦隐藏 + 页面驻留。

**P3 扩展平台**（`5753a2f` … `4244a2f`）
- 读既有安装：与旧版同一份 `extension-repository.json`，未知键写回不丢；本机 9 集成 / 21 命令解析成功。
- 命令参数模式（Tab 展开、静态补全、argv 运行）。
- 配置注入（旧 `config.json` + 密钥 + envVar/argument，口令只进环境）。
- 本地安装/更新/卸载（staging + 备份 + rename + 原子写仓库 + 审计保留）。
- npm 安装（semver 子集、registry 客户端、SRI 校验、安全解包）。
- 权限审批（七权限、两条 host 强制、批准绑定清单 sha256、两阶段安装）。
- `complete`/`diagnose` 协议 + 动态补全合并。

**P4 系统集成与内置插件**（`c136945` … `41d1a0f`）
- 单实例、`floter://` 深链、登录项、托盘、应用菜单（Edit 角色）。
- 剪贴板历史（text/image/files 捕获、去重、保留、启动器模式、Tab 钉住）。
- 浏览器历史/书签（只读 SQLite，Chrome/Safari/Firefox，异步搜索）。
- 钉住输出到独立窗口。
- 空查询的最近使用（`usage.json`）。
- PATH 系统命令搜索（`show_commands_in_search`）。
- 快捷键录制（`hotkey` 写回 + 重新注册）。

**P6 打包与 CI**（`b25e8d5`）
- `go tool mygo build` 产出 app + dmg；bundle 内嵌 `libghostty-vt.dylib` 与 `libmygo-sqlite3.dylib`；Info.plist 带 `floter` scheme；AppIcon.icns；应用图标跟随设置。
- `.github/workflows/go.yml`：三平台 gofmt/vet/build/test + macOS 打包上传产物。
- 已在 bundle 内验证：移走原生库缓存后终端仍能起（用的是 bundle 里的库）；深链与登录项在 bundle 下不再有「需要 bundle」报错。

**P4-j / P2-h 命令开关、别名与 Plugins 页**（本轮）
- `plugin_command_switches`（缺省即关闭，设置页每条命令一个开关，门禁启动器行）与
  `command_aliases`（先到先得的冲突策略，别名与命令名同阶梯打分）。
- 新设置页 Plugins：浏览器插件（目标/目录/范围/排序/搜索范围/调试端口）与剪贴板（开关/容量）。
- `show_menubar_icon`、`last_settings_page`、`launch_counts`、三个启动器开关收进类型化访问器。

**P4-k 自定义快捷键**（本轮）
- `custom_shortcuts`（plugin:/action:/命令行 三类动作，平台感知的按键去重），随设置重注册，
  系统拒绝时如实报告；新 `internal/spawn` 统一「启动即忘」的程序启动（浏览器/静默命令/终端模拟器）；
  Shortcuts 页可录制、改动作、移除。

## 尚未做（按建议优先级）

1. **终端会话快照/恢复**：`plugins/terminal` 的 `Snapshot()` 可把回滚+屏幕+光标序列化；旧版 `session_restore.rs` 是「工具会话 reattach」语义（本机无集成使用）。Sessions 页目前只能列出/关闭当前会话。
2. **i18n 全量对齐**：目前只覆盖实际用到的键（约 200 条），旧 `src/i18n.ts` 有 1700+ 条（含插件页文案）。
3. **浏览器插件剩余**：Safari 的 `Bookmarks.plist`（二进制 plist，Go 标准库不支持）；排序/目标/
   搜索范围/标签页已接线（见 P4-j）。
4. **扩展剩余**：orphan 的接管/删除、权限审计抽屉、导入/导出（sync）、后台运行与输出回看（旧版 run.rs 的 capture 通道）、完成提示。命令级开关已完成。
5. **系统通知**：长任务完成时通知（`mygo.NewNotification`，需打包应用）。
6. **P5 打磨**：动效（转场/按压）、无障碍（行 role、announcement）、性能对照（官方数字：原生 UI ~44MB 内存 / ~7MB 二进制）。
7. **P6 发布链**：预发布通道、签名/公证（`mygo.json` 的 `macos.sign`）、Linux/Windows 打包产物验证、删除旧 CI。
8. **小项**：终端窗口尺寸拖拽后写回 settings；设置页 General 的「开机自启」在打包应用下的真机验证；`show_menubar_icon` 等未接线的旧设置项。

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
