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

## Screenshots

![Terminal mode](docs/screenshots/screenshot-1.png)

![App launcher](docs/screenshots/screenshot-2.png)

## Install

Download the latest version from [GitHub Releases](https://github.com/vst93/floter/releases).

| Platform | Download | Install |
| --- | --- | --- |
| macOS | `.dmg` | Open the image and drag **floter** to **Applications**. |
| Linux | `.deb` or `.rpm` | Install with your package manager. |
| Arch / CachyOS / Manjaro | — | `cd packaging/arch && makepkg -si` ([PKGBUILD](packaging/arch/PKGBUILD), repackages the release `.deb`). |
| Windows | `.exe` | Run the setup file. |

An `.AppImage` is published as well, for distributions none of the packages fit.
Prefer a native package where you can: the AppImage carries the GTK and WebKit
libraries it was built with, and on a rolling distribution those are older than
the graphics drivers installed on the system — which is what the EGL failure
below is usually about.

### macOS: unsigned app

floter is not currently code-signed. If macOS reports that the app is damaged after installation, run:

```bash
xattr -cr /Applications/floter.app
```

Then open floter again from **Applications**.

### Linux: floter does not start

If the window never appears and the terminal shows something like

```text
Could not create default EGL display: EGL_BAD_PARAMETER
```

then WebKitGTK could not reach the GPU. This is common on AMD hardware under
Wayland with a recent Mesa.

floter notices a start that never reached its window and runs the next one
without the GPU by itself, so **try starting it a second time** first. To decide
for it:

```bash
floter --software-rendering   # never use the GPU
floter --gpu                  # always use it, even after a failed start
```

The same choice is available as `FLOTER_SOFTWARE_RENDERING=1` (or `=0`), for a
desktop entry or a service file. If software rendering does not help either,
run floter through XWayland:

```bash
GDK_BACKEND=x11 floter
```

And if you are on the AppImage, install the package for your distribution
instead — a WebKit built against your own system's libraries is the real fix.

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
