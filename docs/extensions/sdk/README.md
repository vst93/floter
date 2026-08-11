# Floter 第三方扩展 SDK

Floter 扩展是一个 NPM 分发包加一个原生 Provider 程序。NPM 只负责发现、版本
解析和下载；Floter 不执行 JavaScript，也不运行 NPM lifecycle scripts。

## 选择入口

- [Agent 打包指南](../agent-packaging.zh-CN.md)：将现有工具交给编码 Agent 分析、打包和验证。
- [TypeScript Wrapper 模板](typescript-wrapper.md)：组织 NPM 基础包、平台包和两份 JSON 描述。
- [Go Provider 指南](go-wrapper.md)：用单文件原生程序实现 Provider 协议。
- [Rust Provider 指南](rust-wrapper.md)：用 `serde`/`serde_json` 实现相同协议。
- [发布检查清单](checklist.md)：发布前逐项验证包内容、平台和签名。

协议的机器可读来源是
[`floter-extension.schema.json`](../schemas/floter-extension.schema.json) 和
[`provider-description.schema.json`](../schemas/provider-description.schema.json)。行为约定见
[FEP-1](../FEP-1-package.md)、[FEP-2](../FEP-2-provider.md) 和
[FEP-4](../FEP-4-npm-registry.md)。

## 最小工作流

1. 为 Provider 选择稳定 ID，例如 `com.example.acme-tools`。manifest 与 Provider
   输出必须使用同一个 ID。
2. 实现 `--floter describe --protocol 1`。需要上下文补全时再实现 `complete`，
   需要健康检查时再实现 `diagnose`。
3. 为每个支持的目标编译原生可执行文件，并分别放入平台 NPM 包。
4. 在基础包中放入 `package.json` 和 `floter.extension.json`，用
   `runtime.platformPackages` 关联同版本的平台包。
5. 本地验证 JSON、协议输出和三平台产物，再发布平台包和基础包。

基础包和平台包必须使用同一个 SemVer 版本。Provider 报告的工具版本可以独立
演进。安装器始终先校验 NPM `dist.integrity`；manifest 声明 `signatures` 时，
还会校验基础包原始 tarball 的 Ed25519 签名。
