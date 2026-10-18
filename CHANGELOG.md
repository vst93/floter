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

- **启动器空页的内置命令块对齐原版**：只剩原版 `SYSTEM_COMMANDS` 的 6 行，且顺序与原版
  一致——重启、关机、剪贴板历史、浏览器历史、计算器、终端。「打开设置」与「退出 floter」
  不再占用搜索结果（原版同样没有这两行）：两者都在托盘菜单（显示面板 / 打开设置 / 重新扫描 /
  退出）里，macOS 另有应用菜单的「退出」。三个平台的启动器列表因此是同一张表。

## [0.3.16] - 2026-10-19

这一版是 Windows 可用性的收口：0.3.15 预览版实测报的四条（背景透明、外圈底色与系统三大
金刚、终端快捷键反复弹出导致卡死、安装包形态）都在这一版里改到根因上。

### Fixed

- **Windows 背景透明 / 外圈底色 / 三大金刚按钮**：launcher 主窗口改为 panel 形态（Windows
  上是无 caption 的 `WS_POPUP` 工具窗），并显式剥掉框架默认带上的 `WS_CAPTION` /
  `WS_SYSMENU` / 最小化与最大化位；DWM 不再在右上角画系统按钮，外圈也不再是窗口自己的
  边框色。窗口没有 material 时卡片满窗且不透明，那圈「额外底色」没有立足之地。
- **搜索页左右空白**：卡片不再按窗口宽度算一个显式宽度，而是直接填满窗口减平台 inset 之后
  的整条内容带（`.collapsed-card { width: 100% }` 的字面实现），首帧尺寸未知、DPI 取整、
  平台上报的 client 与请求的 DIP 不一致都不会再画出两条空白。
- **「在终端中打开」项**：输入框有内容时不再把终端执行行当成普通结果摆在最前面——原版
  （R64）对非空 query 根本不画那一行；空查询页按住修饰键时它才是列表的最后一行，并带选中
  面，松手即消失。动作栏（`在终端中打开` / `在浏览器打开` / `打开文件`）仍固定在结果之后。
- **卡死**：终端交接全路径单飞（同一时刻只有一次 spawn）、面板内 shell 交接进闸、summon
  快捷键防抖、剪贴板缩略图解码与动作栏路径 stat 都搬出绘制线程（限流 + 落地要帧）。

### Changed

- **打包**：release 工作流补齐三平台安装包形态（Windows NSIS `*-setup.exe`、macOS dmg +
  app.zip、Linux deb / rpm / pkg.tar.zst + 通用 tar.gz + `install.sh`），amd64 与 arm64
  双架构，附 `update-*.json` 更新清单；详见 `docs/release.md`。

## [Unreleased]

### Added

- **应用图标**：设置中可在深色（黑）与浅色（白）两套应用图标之间切换，默认深色；
  两套图标内嵌在二进制里，切换后即时作用于菜单栏 / 托盘图标（Windows / Linux 上同时
  作用于任务栏图标），打包图标不受影响。

### Changed

- **桌面手感**：移除交互控件上的手型光标（桌面工具包的按钮与列表用箭头），
  控件文字不可选中（输入框、终端与命令输出仍可选中），图标不再触发浏览器的图片拖拽，
  全局关掉拼写检查与触屏高亮；开关的「开」态改成 macOS 式 accent 填充轨道 + 白色滑块。
  风格整体向 macOS 靠齐，但不引入 macOS 专属的实现（独立原生设置窗、原生弹层）。
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

- **设置侧栏**：进入设置页时不再画出蓝色焦点框——那是程序性聚焦被当成 `:focus-visible` 的副作用，
  当前页已有 pill 作为位置标记；只有真正用键盘 Tab 回来时才画焦点环。
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
