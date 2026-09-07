# FEP-3：安装生命周期与安全

状态：Draft 2 · **部分实现**。Phase 3 完成 repository-only 状态、journal-first 卸载/编辑和投影重建（`repository.rs`、`transaction.rs`、`install.rs`）。`ExtensionsLock` 保留为内存 API；`extensions.lock.json` 及 `.migrated` 仅供启动迁移恢复，必须先持久化 repository 才能继续。`.corrupt` 阻止损坏状态被当作空安装。NPM 安装/更新 pipeline 已移除，下述相关步骤是历史协议设计；旧 installation journals 的生产 replay 保留，用于升级前崩溃恢复。官方签名索引和 broken 状态仍有实现，版本保留策略未在本阶段扩展。

## 持久状态与操作阶段

```text
not-installed -> enabled <-> disabled
                     \       /
                       broken
```

`enabled`、`disabled` 和 `broken` 是写入 `extension-repository.json` 的持久状态。解析、下载、
校验、安装、更新、回滚和删除是 Host 操作或事务阶段，不写成扩展的持久状态；
操作失败时，Host 保留原来的可用版本和持久状态。它们都不是 Provider 自定义
命令。

安装事务内部依次经过：

```text
resolving -> downloading -> verifying -> installing -> complete
```

## 安装事务

1. 从 NPM Registry 解析精确版本和 `dist.integrity`。
2. 下载基础包；bundled runtime 还需下载当前平台包，system runtime 则从 PATH
   解析外部工具。
3. 在临时目录验证 integrity 并安全解包。
4. 验证 Package Manifest、Host 版本、协议版本、OS 和架构。
5. 启动 Provider `describe`，校验 Provider ID 与扩展 ID 相同。
6. 可选执行 `diagnose`。
7. 原子移动到 `extensions/<id>/versions/<version>`。
8. 原子写入 `extension-repository.json` 并重建命令目录和 current pointer 投影。

任何一步失败都不得改变当前可用版本。

## 更新与回滚

新版本安装到并列目录，通过 repository 的 current version 原子切换。至少保留
一个 previous version，直到新版本成功运行。默认自动更新只允许 patch；
major 更新需要用户确认。用户可以固定版本或选择 stable/beta dist-tag。

## 删除

程序目录与数据目录必须分开。bundled runtime 集成删除时提供：

- 删除程序，保留数据；
- 删除程序和数据。

system runtime 集成只删除集成文件或解除本地注册，绝不能删除外部工具。删除前
必须先禁用 Provider、取消补全请求并从命令目录移除所有命令。

实际持久化顺序：写入并同步 removal journal，暂存程序目录，提交 repository，
清理暂存目录和 journal。编辑复用 removal journal 保存原始条目和文件，失败时
恢复或留给下一次 startup recovery；current pointer 和 shims 不作为状态源。

## 安全基线

- 不执行任何 NPM 生命周期脚本或包内 JavaScript。
- 所有 tar 路径在解包前规范化，拒绝绝对路径、`..` 和目录外链接。
- 使用 Registry 提供的 Subresource Integrity 校验 tarball。
- 分别锁定基础包和平台运行时包的 SRI，并记录已安装版本树的内容摘要；修复、
  重装和回滚不得静默接受同版本内容变化。
- Provider ID、包名、版本和发布者与 repository 记录一致。
- 执行计划使用结构化 program/argv/cwd/env。
- Provider 超时后终止子进程，stdout 大小设上限。
- 官方索引必须是签名的包名白名单；普通 NPM 搜索结果标记为未验证。
- 权限是披露而不是沙箱。原生 CLI 仍拥有启动它的用户权限。
