# FEP-1：扩展包与平台运行时

状态：Draft 1

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

## 安装模式

### managed

由 Floter 拥有运行时。基础包保存清单、图标和文档，`platformPackages`
按照当前目标选择实际二进制包。同一个扩展版本的基础包与平台包版本必须
一致。

平台包解压到扩展版本目录的 `runtime/` 下，`executable` 相对于该目录。
Unix Host 在完整性验证后补充用户可执行位；Windows 不修改 ACL。

### linked

关联 PATH 或用户指定的外部程序。`executableNames` 按顺序探测；Windows
自动考虑 `.exe`、`.cmd` 和 `.bat`，Unix 只接受普通可执行文件。

删除 linked 扩展只解除注册，绝不能删除外部程序。外部程序的升级由其包
管理器负责。

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
  作为主入口，避免隐式 `cmd.exe` 和不可控转义；linked 模式可以识别它们，
  但执行时必须显式选择对应宿主。

机器可读定义见 `schemas/floter-extension.schema.json`。

