# floter × mygo 重构总计划（mygo-rewrite 分支）

## 决策记录（2026-10-09）

- 用户指令：停止 v0.3.x 迭代；用 mygo（github.com/egoist/mygo）全量重构；主分支备份；新分支推进。
- 备份：`backup/main-pre-mygo` = `09fd4a3`（v0.3.14 收尾版），已 push。
- 工作分支：`mygo-rewrite`（本分支）。
- mygo 三卖点已核实：原生 GPU 自绘 UI（无 webview）、`plugins/glass`（GPU 着色器液态玻璃）、`plugins/terminal`（ghostty 渲染）。
- 文档：<https://mygo.egoist.dev/docs/ui>、`/docs/native`、`/docs/bindings`、仓库内 `docs/plugins/terminal.md`、`docs/plugins/glass.md`。
- 本机 Go 1.26.3（go.mod 声明 1.27+ 时工具链自动下载）。

## 架构映射（旧 → 新）

| 旧（tauri + React + Rust） | 新（mygo + Go） |
|---|---|
| tauri 窗口/WebviewWindow | `mygo.NewWindow`（native UI，无 webview） |
| React 三壳（launcher/settings/terminal 页） | `ui` 包视图函数（view = 状态的纯函数） |
| CSS backdrop-filter 玻璃 | `plugins/glass`（GPU shader；性能问题的根治） |
| 手写 canvas 终端渲染器 + Rust pty | `plugins/terminal`（ghostty 内核） |
| Rust `#[tauri::command]` ~90 个 | Go 服务方法（原生 UI 直接调用，无需 IPC 层） |
| settings JSON（config.rs 白名单） | 读取同一 JSON 文件、同一批 key（用户数据无缝迁移），Go 侧结构体 + normalize |
| 深链 floter://、全局快捷键、托盘、剪贴板历史 | mygo Desktop APIs（global shortcuts / tray / clipboard / deep links） |
| 独立插件窗口（detach） | 多窗口（原生） |
| extensions 目录/manifest/tool-lock | Go 移植（同一磁盘格式，兼容既有用户安装） |

## 插件参考（官方文档已读，2026-10-09：docs/plugins/glass.md + terminal.md）

**glass**（比 P0 探针结论丰富，P2 按此升级保真）：
- `glass.Glass{Style: Regular|Clear, Tint, Interactive}` —— Liquid Glass 本体；Regular 面积越大越 frost（macOS 行为）；Interactive = 按压生长动画（AppKit 27 同款）
- `glass.Blur{Radius, Mask}` —— **纯背景模糊（无 tint/rim/shadow）**，Radius=标准差（同 CSS blur()）；Mask=LinearGradient alpha 渐进模糊（同 CSS mask-image）→ **旧版 10/22/28px blur 的忠实对位物，frosted/regular/liquid 可逐档复刻，且可与 Glass 叠加**（"glass over it shows the blurred content through"）
- `glass.ScrollEdge{Soft|Hard}` —— macOS 26 滚动边缘：内容滚到栏下渐隐(Soft: 85%→0)或均匀磨砂(Hard: blur 6 + 饱和 1.25 + 82% 底) → 设置页/列表滚动条边缘的原生替代（旧 CSS 边缘效果退役）
- 性能：GPU = 每 pane 小几趟 pass，pane 随其下内容变化重绘；**CPU 路径慢（渐进模糊一条 1360×176=9ms@M5）→ 全程锁 GPU 渲染路径**；pane=圆角矩形、相邻不合体

**terminal**（P3 直接按此实现）：
- `terminal.New(Options{Command, Dir, Env, Font{Family,Size,Weight,Features,LineHeight,Thicken}, Theme, Cursor, OnTitle, OnExit})` + `terminal.View(c, term).Fill().AutoFocus()`
- 方法：`Send/Paste/Feed/Title/Dir/Size/Text/SetFont/SetTheme/Resize/Snapshot/Close`
- **`Snapshot`** = 滚回+屏幕+光标+模式的转义序列快照，喂给同尺寸新终端即还原 → 旧版「会话恢复/attach」语义可重建
- **`Conn`** = 无子进程的自定义连接（Feed 什么显示什么）→ 后台运行输出、远端会话可喂进终端视图
- libghostty-vt 原生库：`go run`/`go test` 首次自动下载到 `<cache>/mygo/natives/`（SHA-256 校验）；CLI 打包时嵌入 resources；`terminal.Load` 预加载可提前报缺库；`$MYGO_GHOSTTY_VT` 可指自编库
- 选中/复制粘贴语义内置（Ghostty 同款）

## 阶段（2026-10-09 修订：用户已在新项目验证过 mygo，砍验证环节，主体先行）

- **P0 地基（在飞）**：脚手架（native 模板）+ settings JSON 读取子集（theme/glass_step/language/opacity，保未知键）+ 启动器第一窗口 + glass 插件挂载。
- **P1 主体成型**：三壳全部立起来 —— 启动器（搜索框常聚焦 + 结果列表骨架 + 键盘 ↑↓/Enter/Esc）、设置页（页面路由 + General 页真控件 + settings **写回**仍保未知键）、终端页（plugins/terminal 一条会话）、主题 token 体系（light/dark 子集）、全局快捷键或托盘唤回。门槛含三窗截图。
- **P2 细节·启动器与设置**：真实搜索（应用扫描/计算器/表达式）、结果虚拟化、完整设置页、i18n en/zh 全量、**玻璃保真升级**（`glass.Blur{Radius}` 逐档复刻 10/22/28px，可与 Glass 叠加；ScrollEdge 替代旧滚动边缘效果；玻璃保真缺口在 P0 已登记）。
- **P3 细节·扩展**：extensions manifest/目录/安装 graft 的 Go 移植（磁盘格式兼容）。
- **P4 细节·系统集成**：托盘菜单、深链 floter://、自启动、剪贴板历史、独立插件窗口。
- **P5 打磨**：动效、无障碍、性能对照（官方数字：原生 UI ~44MB / ~7MB 二进制）。
- **P6 发布链**：CI Go 矩阵、打包（dmg/nsis/deb）、旧代码删除收尾、预发布通道。

## 纪律（继承）

- 承包 runner：禁 commit/push，树留脏主线复核；门槛实跑；报告落 /tmp。
- 主线：复核 → commit → push（本分支）。
- 用户数据零迁移成本优先于代码优雅：settings/扩展/tool-lock 的磁盘格式不变。
- 性能基线记录在案：mygo 官方数字（原生 UI ~44MB 内存、~7MB 二进制）作为 P6 验收参照。
