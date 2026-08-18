# FEP-1：扩展包与平台运行时

状态：Draft 2

Host 继续读取 Draft 1 manifest，但新包应使用 `schemaVersion: "2.0"`。

## 包入口

NPM `package.json` 必须包含：

```json
{
  "keywords": ["floter-extension"],
  "floter": { "manifest": "floter.extension.json" }
}
```

`manifest` 必须是包根目录内的相对路径，不能包含 `..`。Host 解包时必须
拒绝绝对路径、符号链接逃逸以及写出安装目录的 tar 条目。

## 三个正交维度

Draft 2 不再用一个 `managed / linked` 字段同时表达分发与运行时所有权：

| 字段 | 可选值 | 负责回答 |
| --- | --- | --- |
| `distribution.type` | `npm` / `local` / `built-in` | 集成描述由谁分发和更新 |
| `runtime.type` | `bundled` / `system` / `script` | Provider 运行文件由 Floter、系统还是本地脚本提供 |
| `provider.type` | `executable` / `static-descriptor` | 命令描述来自运行协议还是 Host 内置文件 |

Host 当前支持：

| distribution | runtime | provider | 用途 |
| --- | --- | --- | --- |
| `npm` | `bundled` | `executable` | 集成与工具都由 Floter 安装 |
| `npm` | `system` | `executable` | NPM 更新集成，系统包管理器更新工具 |
| `local` | `system` | `executable` | 连接本地 manifest 和已有工具 |
| `local` | `script` | `executable` | 使用本地 JS、Shell 或 PowerShell Provider 脚本 |
| `local` | `script` | `static-descriptor` | 使用本地脚本执行已生成的静态命令 |
| `built-in` | `system` | `static-descriptor` | Floter 内置静态适配器 |

其他组合必须被拒绝。特别是第三方 NPM 包不能声明 `static-descriptor`。

这些字段是协议概念，不要求原样暴露给用户。管理页统一呈现为“集成”，以
“Floter 托管”或“系统工具”说明运行来源。

### bundled runtime

由 Floter 拥有运行时。基础包保存清单、图标和文档，`platformPackages`
按照当前目标选择实际二进制包。同一个扩展版本的基础包与平台包版本必须
一致。

平台包解压到扩展版本目录的 `runtime/` 下，`executable` 相对于该目录。
Unix Host 在完整性验证后补充用户可执行位；Windows 不修改 ACL。

多二进制平台包使用顶层 `artifacts.binaries` 声明每个程序的 `name`、相对于
`runtime/` 的 `path`、`role`（`provider` / `public` / `helper`）、可选
`versionArgs` 和 `required`。Host 会逐个验证 required 二进制；带
`versionArgs` 的程序还必须在两秒内成功退出。`public` 程序会获得 Floter 管理的
稳定 shim，shim 每次运行都读取 `current.json`，因此更新或回滚只需切换一次版本
指针即可让整套公开命令同步生效。

### system runtime

关联 PATH 或用户指定的外部程序。`executableNames` 按顺序探测；Windows
自动考虑 `.exe`、`.cmd` 和 `.bat`，Unix 只接受普通可执行文件。

删除 system runtime 集成只解除注册或删除集成描述，绝不能删除外部程序。
外部程序的升级由其包管理器负责。

从带 `package.json` 的本地包连接时，集成版本取包版本；直接选择 manifest
连接时，集成版本取 Provider `describe` 报告的版本。`versionArgs` 探测到的
外部工具版本单独记录，两者不得在管理页混为一个字段。

Host 内置静态适配器也属于 system runtime 集成。Host 可以在管理页显示已检测到的
适配器，但必须得到用户确认并写入 lock 后才能把命令加入目录。静态适配器的
描述由 Host 提供，不要求外部工具实现 Provider `describe`。

### script runtime

仅用于 `local` 分发。`language` 支持 `js`、`shell`、`powershell`，`path` 是相对 manifest 所在目录的安全路径。Host 分别通过 PATH 中的 `node`、`sh`、`pwsh`（或 `powershell`）启动脚本。可视化创建生成 `static-descriptor`，让普通脚本直接作为一个静态命令运行；需要动态能力时再使用 `executable` Provider。

顶层 `platforms` 是可选的操作系统 allow-list，可包含 `darwin`、`linux`、`windows`。当前系统不在列表内时，Host 不加载该扩展；省略时表示支持全部平台。

## 平台覆写

`platformOverrides` 可以覆写 Provider 参数、环境变量和最低系统版本，
但不能覆写扩展 ID、发布者或权限。解析顺序是：

1. 通用 `provider` 配置；
2. 当前 `<os>-<arch>` 的精确覆写；
3. 当前 `<os>-any` 的系统覆写。

环境变量只允许显式键值；Host 不展开 shell 表达式。所有参数以 argv 数组
保存，不接受拼接的 shell 字符串。

## 三平台要求

- macOS 必须区分 `darwin-arm64` 与 `darwin-x64`。托管包可以是未签名程序，
  但管理页必须显示签名状态；正式源应提供公证产物。
- Linux 托管包应尽量使用静态链接或声明 `diagnose` 检查系统动态库。Host
  不假设发行版，也不自动运行 `apt`、`dnf` 或 `pacman`。
- Windows 的可执行入口可以是 `.exe`。第一版托管包不接受 `.cmd`/`.bat`
  作为主入口，避免隐式 `cmd.exe` 和不可控转义；system runtime 可以识别它们，
  但执行时必须显式选择对应宿主。

机器可读定义见 `schemas/floter-extension.schema.json`。
