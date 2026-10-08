# Changelog

floter 面向用户的变更记录。格式参考 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/)，
版本号遵循 [Semantic Versioning](https://semver.org/lang/zh-CN/)：`MAJOR.MINOR.PATCH`。
这个版本号在仓库里有六个载体（`src-tauri/tauri.conf.json`、`src-tauri/Cargo.toml`、
`src-tauri/Cargo.lock`、`package.json`、`package-lock.json`、`packaging/arch/PKGBUILD`），
发布工作流会同时改写它们；一致性由 `tests/r146-release-hygiene.test.ts` 守卫。

维护约定：每个迭代周期收官时按「用户可感知的影响」补账，按主题归组，不逐条搬运 commit。
`[Unreleased]` 累积下一次发布的内容，发版时改名为版本号并落日期。

## [Unreleased]

### Changed

- **扩展工具安装**：launcher 可以列出可安装与已安装的外部工具，安装经终端交接完成，
  返回 launcher 时刷新目录；Windows 上生成 `.cmd` shim。工具目录的发现改读共享搜索路径，
  而不是进程 PATH，PATH 被改写的会话也能识别已装工具；TUI 工具的 invoke 同样走终端交接。
- **独立插件窗口**：插件页可 detach 成持久的第二窗口，几何与位置被记住；支持多个窗口，
  列表分页与 launcher 一致，单条结果可钉成文本窗口，原生关闭后不再残留 detached 标记。
- **剪贴板**：列表 IPC 只传 8KB 文本前缀，搜索改为在服务端对全量文本进行。
- **launcher / 插件面板**：被关闭的插件显示为状态说明行，而不是变暗的搜索结果；
  禁用扩展前先询问，删除保持二次确认。
- **浏览器历史**：搜索下推到 500 行 fetch 上限之下，结果不再受截断影响。
- **框架与授权**：tauri 家族升到 2.12.1，随之清掉五个不再维护的 `unic-*` crate；
  webview 的 capabilities 收敛到实际调用的授权。
- **插件容器**：三类容器的错误、复制与回滚规则统一为一份。

### Fixed

- **界面语言**：用户可见的英文清零——错误码标签、表单校验、包选择器、config 与 dialog
  错误全部接入双语字典，并加上字典与可见面的永久扫描守卫。
- **子进程生命周期**：每次 spawn 都离开应用的进程组，daemon 正确 detach，
  孤儿进程被回收，SIGHUP 有宽限期。
- **设置与启动**：修掉设置读写的竞态；启动冷路径只读一次 settings 文件，
  稳态 autostart 不再写盘。
- **渲染与写入**：渲染失败落到可重试行，被拒绝的设置写入回滚；
  同步命令链上不再有 panic abort 点。
- **快捷键**：监听器全应用只注册一次，每个事件 disposer 都有守卫。
- **插件行与运行**：subtitle 为空时按 title 钉住；禁用或卸载会终止进行中的运行；
  损坏的 `tool-lock.json` 被隔离而不是让应用失败。
- **打包与安装**：改用 `libayatana-appindicator`；修正 asset URL 的版本前缀不匹配、
  PKGBUILD 的 pkgver 合法性、brew cask 的 post-check。
- **依赖**：可触达的 rustls advisory 打补丁，六个死依赖出清，控制 socket 关闭；
  表达式求值器换到维护中的 fork；浏览器数据 schema reader 移到 rusqlite 0.40。

### Performance

- **settings**：读操作走进程内快照，写入使其失效——launcher 呼出与浏览器搜索不再回盘。
- **扩展目录**：重建时每条目只读一次 manifest，`--version` 探测只在缓存 miss 时发生。
- **导出**：导出 IO 移出 async worker，不阻塞事件循环。
- **launcher**：load-more 不再等待一帧。

### Security

- **本地 socket**：daemon socket 收紧为 owner-only，与应用自身的控制 socket 一致。
- **依赖**：可触达的 rustls advisory 打补丁；依赖声明收紧到已审计的下限。

### Internal

- 守卫矩阵扩展到 r135–r144；每轮的普查方法固定，本文件不逐轮登记。

## [0.3.6] - 2026-09-30

回溯摘要（按主题归组，不逐条抄 commit）。这一版把 launcher 的尺寸与交互打磨到稳定形状，
补上计算器与剪贴板两条内置能力，并重做终端与快捷键设置。

### Added

- 内置计算器插件：表达式求值、历史记忆、星标保护，以及三种找到被隐藏插件的方式。
- 剪贴板历史：收藏、分类 chips、缩略图与按类型区分的图标。
- 直接输出行：命令输出可内联作答并在选中时复制。
- 快捷键页重做：自上而下的阅读顺序、草稿态、单条历史的删除键。
- 托盘身份：不能被另一个 Tauri 应用借用的标识。
- 插件配置 overlay 与开发者文档；外部插件的逐命令开关与两种输出形态。
- 平台专属应用图标重设计。

### Changed

- launcher 尺寸收敛：行高、空态间距、窗口与列表高度自动化统一；
  数字键槽位跟随 viewport；插件模式去掉搜索 chrome，改为 filter + AND needle。
- 终端：外观面板即时重绘、选择即复制默认开启、更宽松的行高下限、裸路径入口；
  固定终端模式退役。
- 窗口与列表：按行自适应高度、尾部直接输出行的位置固定、表面跨窗口存活。

### Fixed

- 剪贴板与插件页的反馈统一走宿主 toast；插件输出与错误处理的一致性。
- 面板高度抖动、空态缝隙、反馈行的定位等一批视觉修复。

[Unreleased]: https://github.com/vst93/floter/compare/v0.3.6...HEAD
[0.3.6]: https://github.com/vst93/floter/releases/tag/v0.3.6
