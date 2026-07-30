# floter

悬浮终端与应用启动器 — 一个快捷键随叫随到。支持 macOS、Linux、Windows。

[English](README.md) · [中文](#功能)

## 功能

- **一键呼出** — 全局快捷键随时召唤，用完即藏
- **应用搜索** — 模糊搜索已安装应用，即时启动
- **内嵌终端** — 完整 shell，支持 256 色
- **万能输入** — 不是应用名？直接作为命令执行
- **多屏感知** — 在你当前的屏幕上出现
- **无缝切换** — 一键将当前会话转到系统终端继续
- **随心定制** — 所有快捷键均可重新绑定

## 下载

预构建二进制文件：[Releases](https://github.com/vst93/floter/releases)

从源码构建：见 [开发](#开发)。

## 快捷键

| 操作 | macOS | Linux / Windows |
|------|-------|-----------------|
| 显示 / 隐藏 | `Cmd+Shift+Space` | `Ctrl+Shift+Space` |
| 新建命令 | `Cmd+W` | `Ctrl+W` |
| 外部终端 | `Cmd+N` | `Ctrl+N` |
| 复制 / 粘贴 | `Cmd+C` / `Cmd+V` | `Ctrl+C` / `Ctrl+V` |
| 设置 | `Cmd+,` | `Ctrl+,` |
| 选择结果 | `Cmd+1`–`9` | `Ctrl+1`–`9` |

均可在 **设置 → 快捷键** 中重新绑定。

## Wayland

Wayland 下全局快捷键由 compositor 管理。请将 `floter --toggle` 绑定为自定义快捷键：

- **GNOME：** 设置 → 键盘 → 自定义快捷键
- **KDE：** 系统设置 → 快捷键 → 自定义

---

## 开发

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

## License

MIT
