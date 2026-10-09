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

## 阶段（2026-10-09 修订：用户已在新项目验证过 mygo，砍验证环节，主体先行）

- **P0 地基（在飞）**：脚手架（native 模板）+ settings JSON 读取子集（theme/glass_step/language/opacity，保未知键）+ 启动器第一窗口 + glass 插件挂载。
- **P1 主体成型**：三壳全部立起来 —— 启动器（搜索框常聚焦 + 结果列表骨架 + 键盘 ↑↓/Enter/Esc）、设置页（页面路由 + General 页真控件 + settings **写回**仍保未知键）、终端页（plugins/terminal 一条会话）、主题 token 体系（light/dark 子集）、全局快捷键或托盘唤回。门槛含三窗截图。
- **P2 细节·启动器与设置**：真实搜索（应用扫描/计算器/表达式）、结果虚拟化、完整设置页、i18n en/zh 全量。
- **P3 细节·扩展**：extensions manifest/目录/安装 graft 的 Go 移植（磁盘格式兼容）。
- **P4 细节·系统集成**：托盘菜单、深链 floter://、自启动、剪贴板历史、独立插件窗口。
- **P5 打磨**：动效、无障碍、性能对照（官方数字：原生 UI ~44MB / ~7MB 二进制）。
- **P6 发布链**：CI Go 矩阵、打包（dmg/nsis/deb）、旧代码删除收尾、预发布通道。

## 纪律（继承）

- 承包 runner：禁 commit/push，树留脏主线复核；门槛实跑；报告落 /tmp。
- 主线：复核 → commit → push（本分支）。
- 用户数据零迁移成本优先于代码优雅：settings/扩展/tool-lock 的磁盘格式不变。
- 性能基线记录在案：mygo 官方数字（原生 UI ~44MB 内存、~7MB 二进制）作为 P6 验收参照。
