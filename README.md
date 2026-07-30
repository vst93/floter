# floter

A floating terminal and app launcher — one hotkey away. macOS, Linux, Windows.

[English](#features) · [中文](#中文)

## Features

- **One hotkey away** — summon from anywhere, hides when you're done
- **App launcher** — fuzzy-search installed apps, launch instantly
- **Built-in terminal** — full shell with 256-color support
- **Run anything** — not an app? It runs as a shell command
- **Multi-monitor** — appears on the screen you're on
- **Pop out** — hand off the current session to your system terminal
- **Customizable** — rebind every shortcut to your liking

## Download

Pre-built binaries: [Releases](https://github.com/vst93/floter/releases)

Build from source: see [Development](#development).

## Shortcuts

| Action | macOS | Linux / Windows |
|--------|-------|-----------------|
| Show / hide | `Cmd+Shift+Space` | `Ctrl+Shift+Space` |
| New command | `Cmd+W` | `Ctrl+W` |
| External terminal | `Cmd+N` | `Ctrl+N` |
| Copy / Paste | `Cmd+C` / `Cmd+V` | `Ctrl+C` / `Ctrl+V` |
| Settings | `Cmd+,` | `Ctrl+,` |
| Select result | `Cmd+1`–`9` | `Ctrl+1`–`9` |

All rebindable in **Settings → Shortcuts**.

## Wayland

Global hotkeys are managed by the compositor. Bind `floter --toggle` as a custom shortcut:

- **GNOME:** Settings → Keyboard → Custom Shortcuts
- **KDE:** System Settings → Shortcuts → Custom

---

## 中文

悬浮终端与应用启动器 — 一个快捷键随叫随到。支持 macOS、Linux、Windows。

[English](#features)

### 功能

- **一键呼出** — 全局快捷键随时召唤，用完即藏
- **应用搜索** — 模糊搜索已安装应用，即时启动
- **内嵌终端** — 完整 shell，支持 256 色
- **万能输入** — 不是应用名？直接作为命令执行
- **多屏感知** — 在你当前的屏幕上出现
- **无缝切换** — 一键将当前会话转到系统终端继续
- **随心定制** — 所有快捷键均可重新绑定

### 下载

预构建二进制文件：[Releases](https://github.com/vst93/floter/releases)

从源码构建：见 [Development](#development)。

### 快捷键

| 操作 | macOS | Linux / Windows |
|------|-------|-----------------|
| 显示 / 隐藏 | `Cmd+Shift+Space` | `Ctrl+Shift+Space` |
| 新建命令 | `Cmd+W` | `Ctrl+W` |
| 外部终端 | `Cmd+N` | `Ctrl+N` |
| 复制 / 粘贴 | `Cmd+C` / `Cmd+V` | `Ctrl+C` / `Ctrl+V` |
| 设置 | `Cmd+,` | `Ctrl+,` |
| 选择结果 | `Cmd+1`–`9` | `Ctrl+1`–`9` |

均可在 **设置 → 快捷键** 中重新绑定。

### Wayland

Wayland 下全局快捷键由 compositor 管理。请将 `floter --toggle` 绑定为自定义快捷键：

- **GNOME：** 设置 → 键盘 → 自定义快捷键
- **KDE：** 系统设置 → 快捷键 → 自定义

---

## Development

```bash
git clone https://github.com/vst93/floter.git
cd floter
npm install
npm run tauri dev      # dev
npm run tauri build    # build
```

Requires [Rust](https://rustup.rs/), Node.js 18+. Linux: `gtk3`, `librsvg`. macOS: Xcode CLT.

```bash
cargo check --all-targets && npx tsc --noEmit && cargo test
```

## License

MIT
