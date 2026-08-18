use floter_lib::extensions::conformance::{
    validate_configuration_response, validate_documents,
};
use floter_lib::extensions::manifest::{validate_relative_path, ExtensionManifest};
use floter_lib::extensions::provider::{ProviderInvocation, ProviderManager};
use semver::Version;
use serde::Deserialize;
use serde_json::json;
use std::path::{Path, PathBuf};

const USAGE: &str = "\
Usage:
  floter-extension-check files <package-dir> [--description <path>]
  floter-extension-check provider <manifest> <executable> [--complete <command>] [--diagnose] [--config]";

#[derive(Deserialize)]
struct PackageJson {
    name: String,
    version: String,
    #[serde(default)]
    keywords: Vec<String>,
    floter: FloterMetadata,
}

#[derive(Deserialize)]
struct FloterMetadata {
    manifest: String,
}

struct ProviderOptions {
    manifest: PathBuf,
    executable: PathBuf,
    complete: Option<String>,
    diagnose: bool,
    config: bool,
}

#[tokio::main]
async fn main() {
    if let Err(error) = run(std::env::args().skip(1).collect()).await {
        eprintln!("error: {error}");
        std::process::exit(1);
    }
}

async fn run(args: Vec<String>) -> Result<(), String> {
    match args.first().map(String::as_str) {
        Some("files") => check_files(&args[1..]),
        Some("provider") => check_provider(parse_provider_options(&args[1..])?).await,
        _ => Err(USAGE.to_string()),
    }
}

fn check_files(args: &[String]) -> Result<(), String> {
    let package_dir = args
        .first()
        .map(PathBuf::from)
        .ok_or_else(|| USAGE.to_string())?;
    let mut description = package_dir.join("provider-description.json");
    let mut index = 1;
    while index < args.len() {
        match args[index].as_str() {
            "--description" if index + 1 < args.len() => {
                description = resolve_from(&package_dir, &args[index + 1]);
                index += 2;
            }
            unknown => return Err(format!("Unknown files option: {unknown}\n{USAGE}")),
        }
    }

    let package_path = package_dir.join("package.json");
    let package: PackageJson = read_json(&package_path, "package.json")?;
    if !package.keywords.iter().any(|keyword| keyword == "floter-extension") {
        return Err("package.json keywords must contain floter-extension".to_string());
    }
    Version::parse(&package.version)
        .map_err(|error| format!("package.json version is not SemVer: {error}"))?;
    let relative_manifest = validate_relative_path(&package.floter.manifest, "floter.manifest")?;
    let manifest_path = package_dir.join(relative_manifest);
    let manifest_bytes = read(&manifest_path, "extension manifest")?;
    let description_bytes = read(&description, "provider description")?;
    let report = validate_documents(
        &manifest_bytes,
        &description_bytes,
        env!("CARGO_PKG_VERSION"),
    )?;

    println!(
        "ok: package {}@{}; extension {}; provider {}; {} command(s)",
        package.name,
        package.version,
        report.extension_id,
        report.provider_version,
        report.command_count
    );
    Ok(())
}

fn parse_provider_options(args: &[String]) -> Result<ProviderOptions, String> {
    if args.len() < 2 {
        return Err(USAGE.to_string());
    }
    let mut options = ProviderOptions {
        manifest: PathBuf::from(&args[0]),
        executable: PathBuf::from(&args[1]),
        complete: None,
        diagnose: false,
        config: false,
    };
    let mut index = 2;
    while index < args.len() {
        match args[index].as_str() {
            "--complete" if index + 1 < args.len() => {
                options.complete = Some(args[index + 1].clone());
                index += 2;
            }
            "--diagnose" => {
                options.diagnose = true;
                index += 1;
            }
            "--config" => {
                options.config = true;
                index += 1;
            }
            unknown => return Err(format!("Unknown provider option: {unknown}\n{USAGE}")),
        }
    }
    Ok(options)
}

async fn check_provider(options: ProviderOptions) -> Result<(), String> {
    let manifest = ExtensionManifest::load(&options.manifest)?;
    manifest.validate_compatibility(env!("CARGO_PKG_VERSION"))?;
    let executable = options.executable.canonicalize().map_err(|error| {
        format!(
            "Cannot resolve provider executable {}: {error}",
            options.executable.display()
        )
    })?;
    let cache = tempfile::tempdir()
        .map_err(|error| format!("Cannot create conformance cache: {error}"))?;
    let manager = ProviderManager::new(cache.path().to_path_buf());
    let invocation = ProviderInvocation {
        extension_id: manifest.id.clone(),
        runtime_root: executable.parent().map(Path::to_path_buf),
        executable,
        executable_prefix: Vec::new(),
        package_version: "conformance-check".to_string(),
        tool_version_hint: None,
        version_args: Vec::new(),
        config: manifest.provider.clone(),
        permissions: manifest.permissions.clone(),
    };
    let response = manager.describe(&invocation, true).await?;

    if let Some(command) = options.complete {
        manager
            .complete(
                &invocation,
                &json!({
                    "command": command,
                    "args": [],
                    "cwd": std::env::current_dir()
                        .map_err(|error| format!("Cannot determine current directory: {error}"))?
                }),
            )
            .await?;
    }
    if options.diagnose {
        manager.diagnose(&invocation).await?;
    }
    if options.config {
        let value: serde_json::Value = manager.call_config(&invocation).await?;
        let bytes = serde_json::to_vec(&value)
            .map_err(|error| format!("Cannot serialize configuration response: {error}"))?;
        validate_configuration_response(&bytes)?;
    }

    println!(
        "ok: provider {} {}; {} command(s)",
        response.description.provider.id,
        response.description.provider.version,
        response.description.commands.len()
    );
    Ok(())
}

fn resolve_from(root: &Path, value: &str) -> PathBuf {
    let path = PathBuf::from(value);
    if path.is_absolute() {
        path
    } else {
        root.join(path)
    }
}

fn read(path: &Path, label: &str) -> Result<Vec<u8>, String> {
    std::fs::read(path).map_err(|error| format!("Cannot read {label} {}: {error}", path.display()))
}

fn read_json<T: for<'de> Deserialize<'de>>(path: &Path, label: &str) -> Result<T, String> {
    serde_json::from_slice(&read(path, label)?)
        .map_err(|error| format!("Invalid {label} {}: {error}", path.display()))
}
