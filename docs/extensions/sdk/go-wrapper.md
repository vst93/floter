# Go Provider 实现指南

Provider 接收如下 argv，其中 `argsPrefix` 来自 manifest：

```text
acme-provider --floter describe --protocol 1
acme-provider --floter complete --protocol 1
acme-provider --floter diagnose --protocol 1
```

`describe` 和 `diagnose` 不接收 stdin。`complete` 从 stdin 读取一个 UTF-8 JSON
对象。所有成功响应只向 stdout 写一个 JSON 值；日志只能写 stderr。成功退出码
是 0，协议或参数错误使用 2，工具自身错误使用其他非零值。

## 最小实现

下面的程序与 V Provider 使用相同的分派方式：协议入口与工具自身命令共享一个
二进制，`--floter` 只进入机器可读分支。

```go
package main

import (
	"encoding/json"
	"fmt"
	"os"
)

type completeRequest struct {
	Command string   `json:"command"`
	Args    []string `json:"args"`
	CWD     string   `json:"cwd"`
}

func writeJSON(value any) {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func protocolMain(args []string) int {
	if len(args) != 4 || args[0] != "--floter" ||
		args[2] != "--protocol" || args[3] != "1" {
		fmt.Fprintln(os.Stderr, "usage: --floter <operation> --protocol 1")
		return 2
	}

	switch args[1] {
	case "describe":
		writeJSON(map[string]any{
			"protocolVersion": "1.0",
			"provider": map[string]any{
				"id": "com.example.acme-tools", "name": "Acme Tools",
				"version": "2.4.0", "description": "Acme CLI integration",
			},
			"commands": []any{map[string]any{
				"id": "search", "name": "Search Acme",
				"description": "Search resources by query",
				"execution": map[string]any{
					"program": "self", "argsPrefix": []string{"search"},
					"mode": "pty", "workingDirectory": "current",
				},
				"arguments": []any{map[string]any{
					"names": []string{"query"}, "kind": "string",
					"description": "Search text", "required": true,
				}},
			}},
		})
	case "complete":
		var request completeRequest
		if err := json.NewDecoder(os.Stdin).Decode(&request); err != nil {
			fmt.Fprintln(os.Stderr, "invalid completion request:", err)
			return 2
		}
		items := []any{}
		if request.Command == "search" {
			items = append(items, map[string]any{
				"label": "--type", "kind": "flag", "detail": "Filter resource type",
			})
		}
		writeJSON(map[string]any{"completions": items})
	case "diagnose":
		writeJSON(map[string]any{
			"status": "ok",
			"checks": []any{map[string]string{
				"id": "runtime", "status": "ok", "message": "Provider is ready",
			}},
		})
	default:
		fmt.Fprintln(os.Stderr, "unsupported Floter operation:", args[1])
		return 2
	}
	return 0
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--floter" {
		os.Exit(protocolMain(os.Args[1:]))
	}
	// Run the tool's normal CLI commands here.
}
```

生产实现应使用具体 struct 代替大量 `map[string]any`，并为输出加协议快照测试。
可把 `provider-description.json` 用 `//go:embed` 编入二进制，在 `describe` 中解码
后输出；这样静态目录仍与 schema 测试共用同一份文件。

## JSON 要求

- `describe` 必须符合 `provider-description.schema.json`，且 `provider.id` 与
  `floter.extension.json` 的 `id` 完全一致。
- `complete` 返回 `{"completions":[...]}`。每项必须包含字符串 `label`、
  `kind`、`detail`；没有结果时返回空数组。
- `diagnose` 返回 `status` 和 `checks`。每个检查包含字符串 `id`、`status`、
  `message`，且只能报告，不得自行安装或修改系统。
- stdout 必须是 UTF-8 JSON，不能含 ANSI、进度条、日志或多个 JSON 文档。
- `describe` 应在 5 秒内完成，`complete` 默认应在 800 ms 内完成，并应能在
  Host 取消进程时立即退出。

构建时为每个发布目标生成独立文件，例如：

```bash
GOOS=linux GOARCH=amd64 go build -trimpath -o dist/linux-x64/bin/acme-provider .
GOOS=darwin GOARCH=arm64 go build -trimpath -o dist/darwin-arm64/bin/acme-provider .
GOOS=windows GOARCH=amd64 go build -trimpath -o dist/windows-x64/bin/acme-provider .
```
