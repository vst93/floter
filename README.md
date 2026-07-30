# floter

A floating terminal and app launcher — one hotkey away. macOS, Linux, Windows.

[English](#features) · [中文文档](README.zh-CN.md)

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
