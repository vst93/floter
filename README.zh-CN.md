# floter

跨平台的浮动启动器与终端，一个快捷键随叫随到。

[English](README.md) · **简体中文**

floter 是原生应用：一个无边框面板在**启动器**、**设置**与**终端**三个表面之间切换。
界面由 [mygo](https://mygo.egoist.dev) 在 GPU 上自绘——二进制里没有 webview、没有浏览器
内核——其余全部是 Go。

## 功能

- **启动器**：一个快捷键（默认 `Ctrl+Space`）唤出搜索框，可匹配已安装应用
  （`.app` / `.lnk` / `.desktop`）、算式（`0.1+0.2` 得 `0.3`）、剪贴板历史、浏览器历史与
  书签、已装扩展声明的命令，以及（可选的）`PATH` 上的可执行文件。
- **终端**：真正的终端，VT 内核来自 [ghostty](https://ghostty.org)：字体、字号、光标、
  行距、边距与九档调色板都是设置项，改动会作用到正在运行的会话。
- **设置**：主题（深/浅/跟随系统）、界面大小、玻璃档、面板透明度、语言（English / 简体中文）、
  窗口行为（失焦隐藏、表面驻留时长）、终端外观、全局快捷键（按键录制）、集成列表。
- **扩展**：工具提供 manifest + provider 程序，floter 通过 `describe` 得知它的命令，在终端
  表面运行，注入用户填写的配置，并在安装声明权限的包之前请求批准。本地包与 npm 包都支持，
  带 SRI 校验与安全的解包器。
- **剪贴板历史**：文本、图片、文件列表在你复制时入库、去重、可在启动器搜索、可钉成独立窗口。
- **系统集成**：单实例、`floter://` 深链、托盘图标、开机自启、标准应用菜单，以及把终端输出
  钉进独立窗口。

## 安装

从源码构建（需要 Go 1.27 或更新）：

```sh
go run ./cmd/floter                      # 直接运行
go tool mygo build                       # 为本机打包
go tool mygo build -platform linux/amd64,windows/amd64,darwin/universal
```

`go tool mygo build` 用的是本模块声明的 mygo CLI 工具，不需要 Node、Bun 或 Rust 工具链，
且任何平台都能在任意机器上构建。产物在 `dist/<platform>/`：

| 平台 | 产物 |
| --- | --- |
| macOS | `floter.app`（原生库已内嵌）与 `.dmg` |
| Linux | `floter`、`.deb`，以及带 `install.sh` 的 `.tar.gz`（为当前用户装到 `~/.local`） |
| Windows | `floter.exe`（有 NSIS 时另有安装程序 `.exe`） |

发布前需要维护者做一次的事：`mygo keygen` 生成更新签名密钥、在 `mygo.json` 写 `updates`
（仓库与公钥）、`macos.signingIdentity` 与 `macos.notarize` 做签名与公证，构建时提供
`MYGO_UPDATER_PRIVATE_KEY`。在此之前构建不带更新源，About 页会如实说明。

## 开发

```sh
gofmt -l cmd internal      # 必须没有输出
go vet ./...
go test -count=1 ./...     # 全量测试
GOOS=linux go build ./... && GOOS=windows go build ./...
```

部分测试要驱动真实资源，需显式开启：

```sh
FLOTER_TERMINAL_TEST=1 go test ./internal/terminalui   # 真 libghostty-vt 会话
FLOTER_SQLITE_TEST=1  go test ./internal/browser ./internal/extensions
FLOTER_REGISTRY_TEST=1 go test ./internal/extensions -run TestRealRegistry
```

设 `FLOTER_OPEN=settings`（或 `terminal`）可让打包后的应用直接开在某个表面。

## 你的数据

floter 读写的是既有版本用的同一批文件，无需迁移：`settings.json`、`extension-repository.json`、
已安装扩展目录、剪贴板历史、使用记录都保持原格式；写回时 floter 不认识的键一律原样保留。
逐文件清单见 [`docs/AGENT-NOTES.md`](AGENT-NOTES.md)。

## 文档

- [`docs/mygo-rewrite-status.md`](mygo-rewrite-status.md) —— 当前状态、包导览、未做清单。
- [`docs/mygo-rewrite-plan.md`](mygo-rewrite-plan.md) —— 分轮决策记录。
- [`docs/extensions/`](extensions/README.zh-CN.md) —— 扩展格式（manifest、provider 协议、
  权限、声明式配置）。
- [`docs/plugin-development.md`](plugin-development.md) —— 如何写一个 floter 能驱动的工具。

## 许可

[GPL-3.0](../LICENSE)
