# Floter 扩展平台规范

Floter 扩展不是网页插件。扩展提供者（Provider）是独立的 CLI/TUI
程序，Floter 负责发现命令、参数联想、安装管理以及在 PTY 中执行。

规范分为两个稳定边界：

- **扩展包协议**：定义 NPM 包的身份、平台运行时、权限和安装方式。
- **Provider 运行协议**：定义工具如何动态返回当前版本所包含的命令。

因此 Floter 只固定“如何询问 Provider”，不会固定 `jv -> v jv` 之类的
子命令映射。工具升级并增加子命令后，下一次 Provider 刷新就会进入命令目录。

## 文档

- [FEP-1：扩展包与平台运行时](FEP-1-package.md)
- [FEP-2：Provider 运行协议](FEP-2-provider.md)
- [FEP-3：安装生命周期与安全](FEP-3-lifecycle.md)
- [FEP-4：NPM Registry Convention](FEP-4-npm-registry.md)
- [使用 Agent 打包 Floter 工具插件](agent-packaging.zh-CN.md)
- [第三方 SDK / Wrapper 指南](sdk/README.md)
- [扩展包 JSON Schema](schemas/floter-extension.schema.json)
- [Provider 描述 JSON Schema](schemas/provider-description.schema.json)
- [V Tools 扩展包示例](examples/v/floter.extension.json)
- [V Tools Provider 输出示例](examples/v/provider-description.json)

## 平台目标

目标使用 `<os>-<arch>` 标识。目前 Host 必须识别：

| 目标 | 系统 | 架构 |
| --- | --- | --- |
| `darwin-arm64` | macOS 11+ | Apple Silicon |
| `darwin-x64` | macOS 10.15+ | Intel |
| `linux-arm64` | Linux | AArch64 |
| `linux-x64` | Linux | x86-64 |
| `windows-arm64` | Windows 10+ | ARM64 |
| `windows-x64` | Windows 10+ | x86-64 |

Provider 的稳定 ID、扩展包版本、外部工具版本和协议版本相互独立：

```text
io.github.vst93.v        Provider ID
@vst93/floter-v@0.0.12  NPM 扩展包版本
V Tools 0.0.12          工具版本
Provider Protocol 1.0   运行协议版本
```

## NPM 的角色

NPM 只承担索引、版本和 tarball 分发。Floter 通过 Registry HTTP API 下载
包，并校验 `dist.integrity`；不会调用 `npm install`，也不会运行
`preinstall`、`install`、`postinstall` 或包内 JavaScript。因此用户无需安装
Node.js，扩展也不能借助 NPM 生命周期脚本绕开管理器。

## 用户侧的统一集成模型

协议内部把分发来源、运行时所有权和 Provider 类型分开声明，但管理页面统一
称为“集成”，用户不需要理解安装器内部术语：

- **Floter 托管**：从 NPM 发现和更新集成，工具运行时也由 Floter 安装管理。
- **NPM 集成 · 系统工具**：从 NPM 发现和更新集成描述，工具仍由 Homebrew、
  Cargo 或系统包管理器维护。
- **系统工具**：工具由 Homebrew、Cargo、系统包管理器或用户维护；Floter
  只连接 manifest 和 Provider，断开连接不会卸载工具。
- **内置集成 · 系统工具**：Floter 自带适配器描述，检测到对应工具后仍需用户
  确认连接。未连接的适配器不会进入命令目录。

管理页统一展示集成版本、工具版本、运行来源和当前可用性。NPM 是默认的发现
与分发渠道；已有工具通过“设置 > 集成 > 连接本地工具”选择 manifest 接入。

### 可视化创建本地集成

“设置 > 集成 > 创建自定义集成”支持两种 Provider 来源：

- 系统可执行文件：可输入绝对路径，也可按命令名模糊搜索 Floter 当前进程的 `PATH`。
- 内置脚本：支持 JavaScript（Node.js）、Shell（`sh`）和 PowerShell（优先 `pwsh`，兼容 `powershell`）。脚本保存在本地集成目录中，不依赖原始编辑位置。

表单默认生成 `local.custom-tool`、`custom-tool`、`1.0.0` 和空的默认执行参数。每个参数按独立 argv 项编辑，不做 Shell 字符串拆分。创建时 Floter 会生成一份静态命令描述，并验证当前运行时可用；普通可执行文件和脚本不需要实现 Provider Protocol。要在当前设备完成验证，平台列表必须包含当前平台。

本地命令默认继承 Floter 的环境变量，以便使用 `PATH`、`HOME`、语言环境和用户凭据；用户可以关闭继承，此时 Host 会在启动进程前清空环境。其余文件、网络、剪贴板和脚本自行启动子进程的选项用于安装审核与审计，不能形成操作系统沙箱。本地代码始终以当前系统用户身份运行。

用户可多选 macOS、Linux 和 Windows。选择结果写入 manifest 的 `platforms` allow-list；当前系统不在列表中时，宿主不会加载该集成。已有插件省略 `platforms` 时仍视为支持全部平台。

由可视化创建器生成的单命令集成可以在集成详情或更多菜单中再次编辑。保存时 Floter 会重新验证清单、运行时和命令描述；验证失败会恢复原有文件与连接状态。集成 ID 是持久身份，创建后不能在编辑器中修改。

删除这类自定义集成会同时删除 Floter 保存的 manifest、静态命令描述和脚本，因此界面会使用“删除集成”并进行二次确认。通过“连接扩展包”接入的外部 manifest 只会断开连接，不会删除外部文件或系统工具。“连接扩展包”面向已有 `floter.extension.json` 的开发者包；连接单个命令或脚本应使用“创建自定义集成”。

```json
{
  "distribution": { "type": "local" },
  "runtime": { "type": "script", "language": "js", "path": "provider.js" },
  "platforms": ["darwin", "linux"]
}
```

## Manifest v2

新 manifest 使用 `schemaVersion: "2.0"`，分别声明集成分发来源、工具运行时
所有权和 Provider 类型。最重要的新组合是 `npm + system + executable`：
发布者可以通过 NPM 更新集成协议与权限声明，同时让 Homebrew、Cargo 或系统
包管理器继续负责工具本身。Host 仍兼容 v1 manifest，并在加载时归一化为 v2
内部模型。
