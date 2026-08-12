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

协议内部保留 `managed` 和 `linked` 两种运行时所有权，但管理页面统一称为
“集成”，用户不需要理解安装器内部术语：

- **Floter 托管**：从 NPM 发现、安装和更新，集成与工具运行时都由 Floter
  管理。
- **系统工具**：工具由 Homebrew、Cargo、系统包管理器或用户维护；Floter
  只连接 manifest 和 Provider，断开连接不会卸载工具。
- **内置集成 · 系统工具**：Floter 自带适配器描述，检测到对应工具后仍需用户
  确认连接。未连接的适配器不会进入命令目录。

管理页统一展示集成版本、工具版本、运行来源和当前可用性。NPM 是默认的发现
与分发渠道；已有工具通过“设置 > 集成 > 连接本地工具”选择 manifest 接入。
