# floter

悬浮终端与应用启动器 — 一个快捷键随叫随到。支持 macOS、Linux、Windows。

[English](README.md) · [中文](#功能)

## 功能

- **一键呼出** — 全局快捷键随时召唤，用完即藏
- **应用搜索** — 模糊搜索已安装应用，支持拼音首字母匹配
- **智能行动条** — 输入 URL 自动打开浏览器，输入路径打开文件管理器，其他则作为命令执行
- **内嵌终端** — 完整 shell，支持 256 色
- **多屏感知** — 出现在光标所在的屏幕上
- **外部终端** — 一键交接给系统终端
- **系统命令** — 直接从启动器重启或关机
- **深色 / 浅色 / 自动** — 跟随系统或手动选择
- **自定义快捷键** — 所有快捷键均可重绑

## 下载

预编译二进制：[Releases](https://github.com/vst93/floter/releases)

源码构建：见[开发说明](#开发说明)。

## 快捷键

| 操作 | macOS | Linux / Windows |
|------|-------|-----------------|
| 显示 / 隐藏 | `Cmd+Shift+Space` | `Ctrl+Shift+Space` |
| 切换到命令行 | `Tab` | `Tab` |
| 上下导航 | `↑` `↓` | `↑` `↓` |
| 选择结果 | `Cmd+1`–`5` | `Ctrl+1`–`5` |
| 新建命令 | `Cmd+W` | `Ctrl+W` |
| 外部终端 | `Cmd+N` | `Ctrl+N` |
| 复制 / 粘贴 | `Cmd+C` / `Cmd+V` | `Ctrl+C` / `Ctrl+V` |
| 设置 | `Cmd+,` | `Ctrl+,` |

所有快捷键均可在 **设置 → 快捷键** 中重绑。

## Wayland

全局快捷键由合成器管理。将 `floter --toggle` 绑定为自定义快捷键：

- **GNOME：** 设置 → 键盘 → 自定义快捷键
- **KDE：** 系统设置 → 快捷键 → 自定义

---

## 开发说明

```bash
git clone https://github.com/vst93/floter.git
cd floter
npm install
npm run tauri dev      # 开发
npm run tauri build    # 构建
```

需要 [Rust](https://rustup.rs/)、Node.js 18+。Linux 需 `gtk3`、`librsvg`。macOS 需 Xcode CLT。

```bash
cargo check --all-targets && npx tsc --noEmit && cargo test
```

## 许可证

MIT
