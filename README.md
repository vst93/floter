# floter

A floating terminal and application launcher for macOS, Linux, and Windows.

[English](#overview) | [中文](#中文)

## Overview

Built with Tauri v2 (Rust + React/TypeScript), using alacritty_terminal as the PTY backend.

### Features

- **Floating panel** — summon with a global hotkey, stays on top, dismissed on blur
- **Application launcher** — fuzzy-search installed apps, launch with one key
- **Embedded terminal** — full alacritty-backed PTY with 256-color support
- **Multi-monitor aware** — appears on the screen you're working on
- **Customizable shortcuts** — rebind every action to your preference
- **Cross-platform** — macOS, Linux (X11 & Wayland), Windows

### Installation

#### From source

```bash
git clone https://github.com/vst93/floter.git
cd floter
npm install
npm run tauri dev    # development
npm run tauri build  # production build
```

#### Requirements

- [Rust](https://rustup.rs/) (stable)
- [Node.js](https://nodejs.org/) 18+
- **Linux:** `gtk3`, `librsvg`, `xdotool` (optional, for multi-monitor focus on X11)
- **macOS:** Xcode Command Line Tools

### Usage

#### Global Hotkey

The default hotkey is `Ctrl+Shift+Space` (macOS: `Cmd+Shift+Space`).

Rebind it in Settings → Shortcuts.

#### Wayland (Linux)

On Wayland, X11 global key grabs don't work — the compositor owns all global
shortcuts. floter provides a `--toggle` CLI flag as a workaround:

1. Open your compositor's shortcut settings:
   - **GNOME:** Settings → Keyboard → Custom Shortcuts
   - **KDE:** System Settings → Shortcuts → Custom
2. Add a new custom shortcut
3. Set the command to `floter --toggle`
4. Assign your preferred key combination

The `--toggle` command connects to the running floter instance via a Unix
domain socket and toggles the panel — it's fast (~30ms) and doesn't start
a second app instance.

#### Launcher

Type to search installed applications. Press `Enter` to launch the top result,
or `Ctrl+1`–`Ctrl+9` to select by index. Anything that doesn't match an app
is run as a shell command in the embedded terminal.

#### Terminal

Toggle between the launcher input and the full terminal with the terminal
shortcut (`Ctrl+W` / `Cmd+W` by default). Open the current session in an
external terminal with `Ctrl+N` / `Cmd+N`.

### Development

```bash
npm run tauri dev     # hot-reload dev build
cargo check           # Rust type check
npx tsc --noEmit      # TypeScript type check
cargo test            # run unit tests
```

## License

MIT

---

## 中文

floter 是一个悬浮终端与应用启动器，支持 macOS、Linux 和 Windows。

基于 Tauri v2（Rust + React/TypeScript）构建，使用 alacritty_terminal 作为 PTY 后端。

### 功能

- **悬浮面板** — 全局快捷键呼出，置顶显示，失焦自动隐藏
- **应用启动器** — 模糊搜索已安装应用，一键启动
- **内嵌终端** — 基于 alacritty 的完整 PTY，支持 256 色
- **多屏感知** — 在你当前工作的屏幕上出现
- **自定义快捷键** — 所有操作均可重新绑定
- **跨平台** — macOS、Linux（X11 和 Wayland）、Windows

### 安装

```bash
git clone https://github.com/vst93/floter.git
cd floter
npm install
npm run tauri dev    # 开发模式
npm run tauri build  # 生产构建
```

#### 依赖

- [Rust](https://rustup.rs/)（stable）
- [Node.js](https://nodejs.org/) 18+
- **Linux：** `gtk3`、`librsvg`、`xdotool`（可选，用于 X11 下多屏焦点检测）
- **macOS：** Xcode Command Line Tools

### 使用

#### 全局快捷键

默认快捷键为 `Ctrl+Shift+Space`（macOS：`Cmd+Shift+Space`）。

可在 设置 → 快捷键 中重新绑定。

#### Wayland（Linux）

在 Wayland 下，X11 全局快捷键不可用 — 全局快捷键由 compositor 管理。
floter 提供了 `--toggle` 命令行参数作为解决方案：

1. 打开 compositor 的快捷键设置：
   - **GNOME：** 设置 → 键盘 → 自定义快捷键
   - **KDE：** 系统设置 → 快捷键 → 自定义
2. 添加一个新的自定义快捷键
3. 命令填写 `floter --toggle`
4. 绑定你喜欢的快捷键组合

`--toggle` 命令通过 Unix domain socket 连接正在运行的 floter 实例并切换面板，
速度很快（约 30ms），不会启动第二个实例。

#### 启动器

输入即可搜索已安装的应用。按 `Enter` 启动第一个结果，
或按 `Ctrl+1`–`Ctrl+9` 按序号选择。不匹配任何应用的输入会作为 shell 命令
在内嵌终端中执行。

#### 终端

使用终端快捷键（默认 `Ctrl+W` / `Cmd+W`）在启动器和终端之间切换。
按 `Ctrl+N` / `Cmd+N` 在外部终端中打开当前会话。
