# Rust Provider 实现指南

Rust Provider 与 Go Provider 使用同一 argv 和 JSON 协议。一个小型 wrapper 通常
只需要 `serde` 与 `serde_json`：

```toml
[dependencies]
serde = { version = "1", features = ["derive"] }
serde_json = "1"
```

```rust
use serde::{Deserialize, Serialize};
use std::process::ExitCode;

#[derive(Deserialize)]
struct CompleteRequest {
    command: String,
    args: Vec<String>,
    cwd: String,
}

#[derive(Serialize)]
struct Completion<'a> {
    label: &'a str,
    kind: &'a str,
    detail: &'a str,
}

fn write_json(value: &impl Serialize) -> Result<(), String> {
    serde_json::to_writer(std::io::stdout(), value).map_err(|error| error.to_string())
}

fn protocol(operation: &str) -> Result<(), String> {
    match operation {
        "describe" => {
            let description: serde_json::Value = serde_json::from_str(include_str!(
                "../provider-description.json"
            ))
            .map_err(|error| error.to_string())?;
            write_json(&description)
        }
        "complete" => {
            let request: CompleteRequest = serde_json::from_reader(std::io::stdin())
                .map_err(|error| format!("invalid completion request: {error}"))?;
            let completions = if request.command == "search" {
                vec![Completion {
                    label: "--type",
                    kind: "flag",
                    detail: "Filter resource type",
                }]
            } else {
                Vec::new()
            };
            let _ = (&request.args, &request.cwd);
            write_json(&serde_json::json!({ "completions": completions }))
        }
        "diagnose" => write_json(&serde_json::json!({
            "status": "ok",
            "checks": [{ "id": "runtime", "status": "ok", "message": "Provider is ready" }]
        })),
        _ => Err(format!("unsupported Floter operation: {operation}")),
    }
}

fn main() -> ExitCode {
    let args = std::env::args().skip(1).collect::<Vec<_>>();
    if args.first().is_some_and(|arg| arg == "--floter") {
        if args.len() != 4 || args[2] != "--protocol" || args[3] != "1" {
            eprintln!("usage: --floter <operation> --protocol 1");
            return ExitCode::from(2);
        }
        return match protocol(&args[1]) {
            Ok(()) => ExitCode::SUCCESS,
            Err(error) => {
                eprintln!("{error}");
                ExitCode::from(2)
            }
        };
    }
    // Run normal CLI commands here.
    ExitCode::SUCCESS
}
```

不要用 `println!` 输出调试信息；stdout 仅供协议 JSON 使用。日志写到 stderr。
建议用 `include_str!` 内嵌经过 schema 验证的描述文件，并在 CI 中分别运行三个
协议操作。跨平台构建可用原生 CI runner 或经过验证的交叉编译工具链；最终文件
名必须与 manifest 的 `runtime.executable` 一致。
