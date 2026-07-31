# floter

A cross-platform floating terminal and app launcher, always one shortcut away.

**English** · [简体中文](README.zh-CN.md)

## Features

- **Floating terminal:** toggle a full 256-color shell from anywhere with `Ctrl+Space` by default.
- **Smart app launcher:** scans installed apps on macOS, Linux, and Windows, with fuzzy matching and pinyin-initial search for Chinese names.
- **Action bar:** detects URLs, filesystem paths, and shell input; press `Cmd+Enter` on macOS or `Ctrl+Enter` elsewhere to open or run it.
- **Multi-monitor workflow:** appears on the display you are using and can continue terminal work in your system terminal.
- **Personalized settings:** choose Dark, Light, or Auto theme, adjust opacity and language, and rebind shortcuts.
- **Built-in updater:** checks for new releases and installs them from inside the app.

## Install

Download the latest version from [GitHub Releases](https://github.com/vst93/floter/releases).

| Platform | Download | Install |
| --- | --- | --- |
| macOS | `.dmg` | Open the image and drag **floter** to **Applications**. |
| Linux | `.deb`, `.rpm`, or `.AppImage` | Use the package for your distribution, or make the AppImage executable and run it. |
| Windows | `.exe` | Run the setup file. |

### macOS: unsigned app

floter is not currently code-signed. If macOS reports that the app is damaged after installation, run:

```bash
xattr -cr /Applications/floter.app
```

Then open floter again from **Applications**.

## Updates

floter checks for updates when it starts. When a new version is available, open **Settings** and choose **Download & Install**. floter relaunches after the update completes.

## Wayland

Wayland compositors manage global shortcuts. Bind the following command as a custom shortcut:

```bash
floter --toggle
```

- **GNOME:** Settings → Keyboard → Custom Shortcuts
- **KDE:** System Settings → Shortcuts → Custom Shortcuts

## License

[GPL-3.0](LICENSE)
