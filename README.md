# floter

A floating terminal and app launcher — one hotkey away. macOS, Linux, Windows.

[English](#features) · [中文文档](README.zh-CN.md)

## Features

- **One hotkey away** — summon from anywhere, hides when you're done
- **App launcher** — fuzzy-search installed apps, pinyin initials supported
- **Smart action bar** — types a URL? opens browser. A path? opens file manager. Anything else? runs as a shell command
- **Built-in terminal** — full shell with 256-color support
- **Multi-monitor** — appears on the screen you're on
- **Pop out** — hand off to your system terminal
- **System commands** — restart or shut down right from the launcher
- **Dark / Light / Auto** — follows your system or pick your own
- **Customizable** — rebind every shortcut to your liking

## Download & Installation

Pre-built binaries: [Releases](https://github.com/vst93/floter/releases)

### macOS

Download the `.dmg`, drag floter to Applications. Since the app is not code-signed, macOS shows "damaged":

```bash
xattr -cr /Applications/floter.app
```

### Linux

Download `.deb` (Debian/Ubuntu), `.rpm` (Fedora), or `.AppImage`.

### Windows

Download the `.exe` setup file and run.

Build from source: see [Development](#development).

## Shortcuts

| Action | macOS | Linux / Windows |
|--------|-------|-----------------|
| Show / hide | `Cmd+Shift+Space` | `Ctrl+Shift+Space` |
| Run as command | `Tab` | `Tab` |
| Navigate results | `↑` `↓` | `↑` `↓` |
| Select result | `Cmd+1`–`5` | `Ctrl+1`–`5` |
| New command | `Cmd+W` | `Ctrl+W` |
| External terminal | `Cmd+N` | `Ctrl+N` |
| Copy / Paste | `Cmd+C` / `Cmd+V` | `Ctrl+C` / `Ctrl+V` |
| Settings | `Cmd+,` | `Ctrl+,` |

All rebindable in **Settings → Shortcuts**.

## Wayland

Global hotkeys are managed by the compositor. Bind `floter --toggle` as a custom shortcut:

- **GNOME:** Settings → Keyboard → Custom Shortcuts
- **KDE:** System Settings → Shortcuts → Custom

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

GPL-3.0
