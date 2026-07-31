# floter

A cross-platform floating terminal and app launcher, always one shortcut away.

**English** · [简体中文](README.zh-CN.md)

## Features

- **Floating terminal** — summon a full 256-color shell from anywhere, then hide it when you are done.
- **Smart app launcher** — scans installed apps on macOS, Linux, and Windows; supports fuzzy matching and pinyin initials for Chinese names.
- **Action bar** — opens URLs in your browser, paths in your file manager, and other input as shell commands.
- **Multi-monitor support** — opens on the display you are currently using, with an option to continue in the system terminal.
- **Personalized controls** — choose dark, light, or system theme and rebind shortcuts in Settings.
- **Built-in updates** — checks for new releases and installs them from inside the app.

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

floter checks for updates automatically. When a new version is available, open **Settings** and choose **Download & Install**; floter relaunches after the update completes.

## Wayland

Wayland compositors manage global shortcuts. Bind the following command as a custom shortcut:

```bash
floter --toggle
```

- **GNOME:** Settings → Keyboard → Custom Shortcuts
- **KDE:** System Settings → Shortcuts → Custom Shortcuts

## License

[GPL-3.0](LICENSE)
