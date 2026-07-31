# floter

跨平台悬浮终端与应用启动器，一个快捷键随叫随到。

[English](README.md) · **简体中文**

## 功能

- **悬浮终端：** 默认按 `Ctrl+Space`，即可随时显示或隐藏完整的 256 色 shell。
- **智能应用启动器：** 自动扫描 macOS、Linux 和 Windows 中已安装的应用，支持模糊匹配及中文应用名的拼音首字母搜索。
- **操作栏：** 自动识别 URL、文件系统路径和 shell 输入；按 `Cmd+Enter`（macOS）或 `Ctrl+Enter`（其他平台）即可打开或运行。
- **多显示器支持：** 在当前使用的显示器上出现，也可在系统终端中继续处理终端任务。
- **个性化设置：** 可选择深色、浅色或自动主题，调整透明度和界面语言，并重新绑定快捷键。
- **内置更新：** 自动检查新版本，并可直接在应用内完成安装。

## 安装

前往 [GitHub Releases](https://github.com/vst93/floter/releases) 下载最新版本。

| 平台 | 下载文件 | 安装方式 |
| --- | --- | --- |
| macOS | `.dmg` | 打开镜像，将 **floter** 拖入「应用程序」。 |
| Linux | `.deb`、`.rpm` 或 `.AppImage` | 选择适合发行版的安装包；AppImage 添加执行权限后即可运行。 |
| Windows | `.exe` | 运行安装程序。 |

### macOS：未签名应用

floter 目前尚未进行代码签名。若安装后 macOS 提示应用「已损坏」，请运行：

```bash
xattr -cr /Applications/floter.app
```

然后从「应用程序」中重新打开 floter。

## 更新

floter 会在启动时检查更新。发现新版本后，打开「设置」并选择「下载并安装」。更新完成后，floter 会自动重新启动。

## Wayland

Wayland 的全局快捷键由合成器管理。请将以下命令绑定为自定义快捷键：

```bash
floter --toggle
```

- **GNOME：** 设置 → 键盘 → 自定义快捷键
- **KDE：** 系统设置 → 快捷键 → 自定义快捷键

## 许可证

[GPL-3.0](LICENSE)
