# FEP-3：安装生命周期与安全

状态：Draft 1

## 状态机

```text
not-installed -> resolving -> downloading -> verifying -> installing
                                                        -> enabled
enabled <-> disabled
enabled/disabled -> updating -> enabled | rollback | broken
enabled/disabled/broken -> removing -> not-installed
```

安装、更新、启停、回滚和删除都是 Host 操作，不是 Provider 自定义命令。

## 安装事务

1. 从 NPM Registry 解析精确版本和 `dist.integrity`。
2. 下载基础包以及当前平台包。
3. 在临时目录验证 integrity 并安全解包。
4. 验证 Package Manifest、Host 版本、协议版本、OS 和架构。
5. 启动 Provider `describe`，校验 Provider ID 与扩展 ID 相同。
6. 可选执行 `diagnose`。
7. 原子移动到 `extensions/<id>/versions/<version>`。
8. 原子写入 lock 文件并启用命令目录。

任何一步失败都不得改变当前可用版本。

## 更新与回滚

新版本安装到并列目录，通过 lock 文件的 current version 原子切换。至少保留
一个 previous version，直到新版本成功运行。默认自动更新只允许 patch；
major 更新需要用户确认。用户可以固定版本或选择 stable/beta dist-tag。

## 删除

程序目录与数据目录必须分开。托管扩展删除时提供：

- 删除程序，保留数据；
- 删除程序和数据。

linked 扩展只能解除关联。删除前必须先禁用 Provider、取消补全请求并从命令
目录移除所有命令。

## 安全基线

- 不执行任何 NPM 生命周期脚本或包内 JavaScript。
- 所有 tar 路径在解包前规范化，拒绝绝对路径、`..` 和目录外链接。
- 使用 Registry 提供的 Subresource Integrity 校验 tarball。
- Provider ID、包名、版本和发布者与 lock 记录一致。
- 执行计划使用结构化 program/argv/cwd/env。
- Provider 超时后终止子进程，stdout 大小设上限。
- 官方索引必须是签名的包名白名单；普通 NPM 搜索结果标记为未验证。
- 权限是披露而不是沙箱。原生 CLI 仍拥有启动它的用户权限。

