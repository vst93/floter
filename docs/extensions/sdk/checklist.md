# 扩展发布检查清单

## 包与元数据

- [ ] 基础包 `package.json` 含精确 keyword `floter-extension`。
- [ ] `package.json` 的 `floter.manifest` 指向包内安全相对路径。
- [ ] `floter.extension.json` 通过 `floter-extension.schema.json` 验证。
- [ ] `provider-description.json` 通过 `provider-description.schema.json` 验证。
- [ ] manifest ID 与 Provider `provider.id` 完全一致。
- [ ] 基础包和所有平台包使用同一个版本号。
- [ ] 版本号遵循 SemVer，升级范围与破坏性变化相符。
- [ ] `README` 包含 Floter 安装说明和所需外部依赖。

## Provider 与平台

- [ ] `describe` 输出单个 UTF-8 JSON，stdout 不含日志或 ANSI。
- [ ] `complete` 和 `diagnose` 的支持状态及降级行为已测试。
- [ ] 所有 `execution.program` 和 `runtime.executable` 均为安全相对路径。
- [ ] 权限声明覆盖 Provider 和实际命令可能访问的资源。
- [ ] macOS、Linux、Windows 三平台均已测试（发布相应平台时）。
- [ ] 每种发布架构均在真实或等效 CI 环境中启动并执行协议测试。
- [ ] Unix 文件可执行，各平台包都提供 manifest 声明的同一相对文件名。

## 完整性与发布

- [ ] NPM registry 为每个包提供受支持的 `dist.integrity`（推荐 SHA-512）。
- [ ] 基础包 tarball 已签名并声明 Ed25519 `signatures`（推荐）。
- [ ] 签名针对 `npm pack` 的原始 `.tgz` 字节，签名 URL 使用 HTTPS。
- [ ] 公钥已通过项目网站、仓库或其他独立可信渠道发布。
- [ ] 签名 URL 和所有平台包在公开环境中可下载。
- [ ] 先发布同版本平台包，再发布引用它们的基础包。
- [ ] 从干净环境按 registry 包名和版本完成一次安装、运行、更新与卸载测试。
