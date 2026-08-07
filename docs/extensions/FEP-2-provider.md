# FEP-2：Provider 运行协议

状态：Draft 1

## 调用约定

扩展包声明 `provider.argsPrefix`。Host 将操作名附加在后面。例如前缀为
`["--floter"]` 时：

```bash
v --floter describe --protocol 1
v --floter complete --protocol 1
v --floter diagnose --protocol 1
```

Provider 必须把协议 JSON 写到 stdout，把诊断日志写到 stderr。成功返回 0，
协议错误返回 2，工具自身错误返回其他非零值。stdout 不能包含 ANSI 或额外
文本。UTF-8 是唯一允许的编码。

`describe` 必须在五秒内完成且不能修改用户配置。Host 会缓存成功结果，
并在可执行文件路径、mtime、包版本或工具版本变化时刷新。

## describe

输出包含 Provider 身份、工具版本和完整命令目录。命令的 `execution.program`
默认是 `self`，表示执行刚才响应 describe 的同一文件；也可以指向托管包
`runtime/` 下的相对路径。

命令参数必须使用结构化类型：

- `flag`：不取值的开关；
- `string`、`integer`、`number`：标量；
- `path`、`directory`：由 Host 提供本地路径联想；
- `url`：URL 输入；
- `enum`：由 `values` 提供候选；
- `command`：由 Provider 的动态 complete 提供候选。

命令可以选择 `pty`、`capture` 或 `external` 执行模式。交互式 TUI 必须使用
`pty`。Host 永远以 argv 生成执行计划，不把用户输入直接拼进 shell 字符串。

## complete

可选操作。请求 JSON 从 stdin 读取：

```json
{
  "command": "vc",
  "tokens": ["vc", "-join", "a"],
  "cursor": 11,
  "cwd": "/home/user"
}
```

动态补全应在 800ms 内返回。Host 必须防抖、取消过期请求，并对 cwd 与输入
建立短期缓存。静态 flag、enum 和文件路径不应调用 Provider。

## diagnose

可选操作，用于管理页健康检查。检查结果只能报告问题，不能自行安装依赖或
修改系统。Host 不把权限声明视为沙箱保证；它只是向用户透明展示原生程序
可能访问的资源。

机器可读定义见 `schemas/provider-description.schema.json`。

