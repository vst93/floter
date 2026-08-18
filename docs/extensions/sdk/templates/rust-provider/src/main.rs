use serde::Deserialize;
use serde_json::{json, Value};
use std::io::{Read, Write};
use std::process::ExitCode;

const DESCRIPTION: &str = include_str!("../provider-description.json");

#[derive(Deserialize)]
struct CompleteRequest {
    command: String,
    args: Vec<String>,
    cwd: String,
}

fn protocol(
    operation: &str,
    mut input: impl Read,
    mut output: impl Write,
) -> Result<(), String> {
    let response: Value = match operation {
        "describe" => serde_json::from_str(DESCRIPTION)
            .map_err(|error| format!("invalid embedded description: {error}"))?,
        "complete" => {
            let request: CompleteRequest = serde_json::from_reader(&mut input)
                .map_err(|error| format!("invalid completion request: {error}"))?;
            let completions = if request.command == "search" {
                vec![json!({
                    "label": "--type",
                    "kind": "flag",
                    "detail": format!(
                        "Filter resource type ({} argument(s), cwd {})",
                        request.args.len(),
                        request.cwd
                    )
                })]
            } else {
                Vec::new()
            };
            json!({ "completions": completions })
        }
        "diagnose" => json!({
            "status": "ok",
            "checks": [{
                "id": "runtime",
                "status": "ok",
                "message": "Provider is ready"
            }]
        }),
        "config" => json!({
            "configuration": {
                "configVersion": 1,
                "owner": "host",
                "schema": [{
                    "key": "endpoint",
                    "type": "text",
                    "label": "API endpoint",
                    "default": "https://api.example.com"
                }]
            }
        }),
        other => return Err(format!("unsupported Floter operation: {other}")),
    };
    serde_json::to_writer(&mut output, &response)
        .map_err(|error| format!("cannot write protocol response: {error}"))?;
    writeln!(output).map_err(|error| format!("cannot finish protocol response: {error}"))
}

fn main() -> ExitCode {
    let args = std::env::args().skip(1).collect::<Vec<_>>();
    if args.first().map(String::as_str) != Some("--floter") {
        eprintln!("run the wrapped tool here");
        return ExitCode::SUCCESS;
    }
    if args.len() != 4 || args[2] != "--protocol" || args[3] != "1" {
        eprintln!("usage: --floter <operation> --protocol 1");
        return ExitCode::from(2);
    }
    match protocol(&args[1], std::io::stdin(), std::io::stdout()) {
        Ok(()) => ExitCode::SUCCESS,
        Err(error) => {
            eprintln!("{error}");
            ExitCode::from(2)
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn describe_is_one_json_document() {
        let mut output = Vec::new();
        protocol("describe", &b""[..], &mut output).unwrap();
        let value: Value = serde_json::from_slice(&output).unwrap();
        assert_eq!(value["protocolVersion"], "1.0");
    }

    #[test]
    fn complete_accepts_the_protocol_request() {
        let input = br#"{"command":"search","args":[],"cwd":"/tmp"}"#;
        let mut output = Vec::new();
        protocol("complete", &input[..], &mut output).unwrap();
        let value: Value = serde_json::from_slice(&output).unwrap();
        assert_eq!(value["completions"][0]["label"], "--type");
    }
}
