use serde::{Deserialize, Serialize};
use std::collections::{BTreeMap, BTreeSet};
use std::path::{Component, Path, PathBuf};

const MAX_SOURCE_FILE_BYTES: u64 = 2 * 1024 * 1024;
const STAGING_SENTINEL: &str = "/__floter_stage";

#[derive(Debug, Clone, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct SourceInferenceReport {
    pub project_root: String,
    pub cargo: Option<CargoBuildPlan>,
    pub make: Option<MakeBuildPlan>,
}

#[derive(Debug, Clone, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct CargoBuildPlan {
    pub manifest_path: String,
    pub lock_file_present: bool,
    pub binaries: Vec<CargoBinaryPlan>,
    pub risks: Vec<SourceRisk>,
}

#[derive(Debug, Clone, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct CargoBinaryPlan {
    pub package: String,
    pub name: String,
    pub manifest_path: String,
    pub source_path: Option<String>,
    pub required_features: Vec<String>,
    pub build_arguments: Vec<String>,
}

#[derive(Debug, Clone, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct MakeBuildPlan {
    pub makefile_path: String,
    pub variables: BTreeMap<String, String>,
    pub targets: Vec<String>,
    pub install_actions: Vec<MakeInstallAction>,
    pub accepted: bool,
    pub risks: Vec<SourceRisk>,
}

#[derive(Debug, Clone, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct MakeInstallAction {
    pub command: String,
    pub source: String,
    pub staging_path: String,
    pub mode: Option<String>,
}

#[derive(Debug, Clone, Serialize, PartialEq, Eq)]
#[serde(rename_all = "camelCase")]
pub struct SourceRisk {
    pub code: String,
    pub detail: String,
    pub blocking: bool,
}

#[derive(Debug, Deserialize)]
struct CargoManifest {
    package: Option<CargoPackage>,
    workspace: Option<CargoWorkspace>,
    #[serde(default)]
    bin: Vec<CargoBin>,
    #[serde(flatten)]
    remaining: BTreeMap<String, toml::Value>,
}

#[derive(Debug, Deserialize)]
struct CargoPackage {
    name: String,
    #[serde(rename = "default-run")]
    default_run: Option<String>,
    build: Option<toml::Value>,
    links: Option<String>,
}

#[derive(Debug, Deserialize)]
struct CargoWorkspace {
    #[serde(default)]
    members: Vec<String>,
}

#[derive(Debug, Deserialize)]
struct CargoBin {
    name: Option<String>,
    path: Option<String>,
    #[serde(default, rename = "required-features")]
    required_features: Vec<String>,
}

pub fn infer(path: &Path) -> Result<SourceInferenceReport, String> {
    let (project_root, selected_file) = resolve_input(path)?;
    let cargo_path = selected_file
        .as_deref()
        .filter(|path| path.file_name().and_then(|name| name.to_str()) == Some("Cargo.toml"))
        .map(Path::to_path_buf)
        .or_else(|| {
            project_root
                .join("Cargo.toml")
                .is_file()
                .then(|| project_root.join("Cargo.toml"))
        });
    let make_path = selected_file
        .as_deref()
        .filter(|path| {
            matches!(
                path.file_name().and_then(|name| name.to_str()),
                Some("Makefile" | "makefile" | "GNUmakefile")
            )
        })
        .map(Path::to_path_buf)
        .or_else(|| find_makefile(&project_root));

    if cargo_path.is_none() && make_path.is_none() {
        return Err("No Cargo.toml or Makefile was found at the selected path".to_string());
    }

    Ok(SourceInferenceReport {
        project_root: project_root.to_string_lossy().into_owned(),
        cargo: cargo_path
            .as_deref()
            .map(|path| infer_cargo(&project_root, path))
            .transpose()?,
        make: make_path
            .as_deref()
            .map(|path| infer_make(&project_root, path))
            .transpose()?,
    })
}

fn resolve_input(path: &Path) -> Result<(PathBuf, Option<PathBuf>), String> {
    let canonical = path
        .canonicalize()
        .map_err(|error| format!("Cannot resolve source path {}: {error}", path.display()))?;
    if canonical.is_dir() {
        return Ok((canonical, None));
    }
    if !canonical.is_file() {
        return Err(format!(
            "Source path is not a regular file: {}",
            path.display()
        ));
    }
    let root = canonical
        .parent()
        .ok_or_else(|| format!("Source file has no parent: {}", canonical.display()))?
        .to_path_buf();
    Ok((root, Some(canonical)))
}

fn find_makefile(root: &Path) -> Option<PathBuf> {
    ["GNUmakefile", "Makefile", "makefile"]
        .into_iter()
        .map(|name| root.join(name))
        .find(|path| path.is_file())
}

fn read_source(path: &Path) -> Result<String, String> {
    let metadata = std::fs::metadata(path)
        .map_err(|error| format!("Cannot inspect {}: {error}", path.display()))?;
    if !metadata.is_file() || metadata.len() > MAX_SOURCE_FILE_BYTES {
        return Err(format!(
            "Source definition must be a regular file no larger than {} bytes: {}",
            MAX_SOURCE_FILE_BYTES,
            path.display()
        ));
    }
    std::fs::read_to_string(path)
        .map_err(|error| format!("Cannot read {} as UTF-8: {error}", path.display()))
}

fn infer_cargo(root: &Path, manifest_path: &Path) -> Result<CargoBuildPlan, String> {
    let mut risks = Vec::new();
    let mut binaries = Vec::new();
    let mut visited = BTreeSet::new();
    infer_cargo_manifest(root, manifest_path, &mut visited, &mut binaries, &mut risks)?;
    binaries.sort_by(|left, right| {
        left.package
            .cmp(&right.package)
            .then(left.name.cmp(&right.name))
            .then(left.manifest_path.cmp(&right.manifest_path))
    });
    binaries.dedup_by(|left, right| {
        left.package == right.package
            && left.name == right.name
            && left.manifest_path == right.manifest_path
    });
    risks.sort_by(|left, right| {
        left.code
            .cmp(&right.code)
            .then(left.detail.cmp(&right.detail))
    });
    risks.dedup();
    let lock_file_present = root.join("Cargo.lock").is_file();
    if !lock_file_present && !binaries.is_empty() {
        risks.push(SourceRisk {
            code: "cargo-lock-missing".into(),
            detail: "Cargo.lock is missing; an application build cannot be strictly reproduced"
                .into(),
            blocking: false,
        });
    }
    for binary in &mut binaries {
        binary.build_arguments = vec!["build".into(), "--release".into()];
        if lock_file_present {
            binary.build_arguments.push("--locked".into());
        }
        binary
            .build_arguments
            .extend(["--bin".into(), binary.name.clone()]);
        if !binary.required_features.is_empty() {
            binary
                .build_arguments
                .extend(["--features".into(), binary.required_features.join(",")]);
        }
    }
    Ok(CargoBuildPlan {
        manifest_path: display_relative(root, manifest_path),
        lock_file_present,
        binaries,
        risks,
    })
}

fn infer_cargo_manifest(
    root: &Path,
    manifest_path: &Path,
    visited: &mut BTreeSet<PathBuf>,
    binaries: &mut Vec<CargoBinaryPlan>,
    risks: &mut Vec<SourceRisk>,
) -> Result<(), String> {
    let canonical = manifest_path
        .canonicalize()
        .map_err(|error| format!("Cannot resolve {}: {error}", manifest_path.display()))?;
    if !canonical.starts_with(root) {
        return Err(format!(
            "Workspace manifest escapes the selected project: {}",
            manifest_path.display()
        ));
    }
    if !visited.insert(canonical.clone()) {
        return Ok(());
    }
    let text = read_source(&canonical)?;
    let manifest: CargoManifest = toml::from_str(&text)
        .map_err(|error| format!("Cannot parse {}: {error}", canonical.display()))?;
    let manifest_dir = canonical.parent().ok_or("Cargo manifest has no parent")?;
    let relative_manifest = display_relative(root, &canonical);

    if let Some(package) = &manifest.package {
        collect_package_risks(package, manifest_dir, &relative_manifest, risks);
        collect_dependency_risks(&manifest.remaining, &relative_manifest, risks);
        collect_cargo_binaries(package, &manifest.bin, root, &canonical, binaries);
    }

    if let Some(workspace) = manifest.workspace {
        for member in workspace.members {
            if member.contains(['*', '?', '[', ']']) {
                risks.push(SourceRisk {
                    code: "cargo-workspace-glob".into(),
                    detail: format!(
                        "{relative_manifest}: workspace member pattern {member:?} requires explicit resolution"
                    ),
                    blocking: true,
                });
                continue;
            }
            let member_manifest = manifest_dir.join(&member).join("Cargo.toml");
            infer_cargo_manifest(root, &member_manifest, visited, binaries, risks)?;
        }
    }
    Ok(())
}

fn collect_package_risks(
    package: &CargoPackage,
    manifest_dir: &Path,
    relative_manifest: &str,
    risks: &mut Vec<SourceRisk>,
) {
    let build_script = match package.build.as_ref() {
        Some(toml::Value::Boolean(false)) => None,
        Some(toml::Value::String(path)) => Some(path.as_str()),
        Some(_) => Some("build.rs"),
        None if manifest_dir.join("build.rs").is_file() => Some("build.rs"),
        None => None,
    };
    if let Some(build_script) = build_script {
        risks.push(SourceRisk {
            code: "cargo-build-script".into(),
            detail: format!(
                "{relative_manifest}: package {} executes {build_script} during build",
                package.name
            ),
            blocking: false,
        });
    }
    if let Some(links) = &package.links {
        risks.push(SourceRisk {
            code: "cargo-native-links".into(),
            detail: format!(
                "{relative_manifest}: package {} links native library {links}",
                package.name
            ),
            blocking: false,
        });
    }
}

fn collect_dependency_risks(
    table: &BTreeMap<String, toml::Value>,
    manifest_path: &str,
    risks: &mut Vec<SourceRisk>,
) {
    fn visit_table(
        table: &toml::map::Map<String, toml::Value>,
        manifest_path: &str,
        risks: &mut Vec<SourceRisk>,
    ) {
        for (key, value) in table {
            if matches!(
                key.as_str(),
                "dependencies" | "build-dependencies" | "dev-dependencies"
            ) {
                collect_sources(value, manifest_path, risks);
            }
            if let Some(nested) = value.as_table() {
                visit_table(nested, manifest_path, risks);
            }
        }
    }

    fn collect_sources(value: &toml::Value, manifest_path: &str, risks: &mut Vec<SourceRisk>) {
        let Some(table) = value.as_table() else {
            return;
        };
        for (name, spec) in table {
            if let Some(spec) = spec.as_table() {
                for source in ["git", "path"] {
                    if let Some(location) = spec.get(source).and_then(toml::Value::as_str) {
                        risks.push(SourceRisk {
                            code: format!("cargo-{source}-dependency"),
                            detail: format!(
                                "{manifest_path}: dependency {name} uses {source} source {location}"
                            ),
                            blocking: false,
                        });
                    }
                }
            }
        }
    }

    for (key, value) in table {
        if matches!(
            key.as_str(),
            "dependencies" | "build-dependencies" | "dev-dependencies"
        ) {
            collect_sources(value, manifest_path, risks);
        }
        if let Some(nested) = value.as_table() {
            visit_table(nested, manifest_path, risks);
        }
    }
}

fn collect_cargo_binaries(
    package: &CargoPackage,
    declared: &[CargoBin],
    root: &Path,
    manifest_path: &Path,
    binaries: &mut Vec<CargoBinaryPlan>,
) {
    for binary in declared {
        if let Some(name) = binary.name.as_deref() {
            binaries.push(cargo_binary_plan(
                package,
                name,
                binary.path.as_deref(),
                &binary.required_features,
                root,
                manifest_path,
            ));
        }
    }
    if let Some(name) = package.default_run.as_deref() {
        if !binaries.iter().any(|binary| {
            binary.package == package.name
                && binary.name == name
                && binary.manifest_path == display_relative(root, manifest_path)
        }) {
            binaries.push(cargo_binary_plan(
                package,
                name,
                None,
                &[],
                root,
                manifest_path,
            ));
        }
    }
    let main_path = manifest_path.parent().map(|dir| dir.join("src/main.rs"));
    if main_path.as_ref().is_some_and(|path| path.is_file())
        && !binaries.iter().any(|binary| {
            binary.package == package.name
                && binary.name == package.name
                && binary.manifest_path == display_relative(root, manifest_path)
        })
    {
        binaries.push(cargo_binary_plan(
            package,
            &package.name,
            Some("src/main.rs"),
            &[],
            root,
            manifest_path,
        ));
    }
}

fn cargo_binary_plan(
    package: &CargoPackage,
    name: &str,
    source_path: Option<&str>,
    required_features: &[String],
    root: &Path,
    manifest_path: &Path,
) -> CargoBinaryPlan {
    CargoBinaryPlan {
        package: package.name.clone(),
        name: name.to_string(),
        manifest_path: display_relative(root, manifest_path),
        source_path: source_path.map(str::to_string),
        required_features: required_features.to_vec(),
        build_arguments: Vec::new(),
    }
}

fn infer_make(root: &Path, makefile_path: &Path) -> Result<MakeBuildPlan, String> {
    let text = read_source(makefile_path)?;
    let logical_lines = make_logical_lines(&text);
    let mut variables = BTreeMap::new();
    let mut targets = Vec::new();
    let mut install_recipes = Vec::new();
    let mut current_targets: Vec<String> = Vec::new();
    let mut risks = Vec::new();

    for line in &logical_lines {
        let trimmed = line.trim();
        if trimmed.is_empty() || trimmed.starts_with('#') {
            continue;
        }
        if !line.starts_with('\t') {
            current_targets.clear();
            if trimmed.starts_with("include ")
                || trimmed.starts_with("-include ")
                || trimmed.starts_with("sinclude ")
            {
                risks.push(make_risk("make-include", "Makefile uses include", true));
                continue;
            }
            if let Some((name, value)) = parse_make_assignment(trimmed) {
                if matches!(name, "PREFIX" | "DESTDIR" | "BINDIR" | "bindir") {
                    variables.insert(name.to_string(), value.to_string());
                }
                continue;
            }
            if let Some((names, _dependencies)) = trimmed.split_once(':') {
                if !names.contains('=') {
                    current_targets = names
                        .split_whitespace()
                        .filter(|name| !name.starts_with('.'))
                        .map(str::to_string)
                        .collect();
                    for target in &current_targets {
                        if matches!(target.as_str(), "build" | "install")
                            && !targets.contains(target)
                        {
                            targets.push(target.clone());
                        }
                    }
                }
            }
            continue;
        }

        let recipe = trimmed.trim_start_matches(['@', '-', '+']).trim();
        if recipe.contains("$(shell") || recipe.contains("${shell") {
            risks.push(make_risk(
                "make-shell-expansion",
                "Makefile recipe uses shell expansion",
                true,
            ));
        }
        if recipe_invokes_recursive_make(recipe) {
            risks.push(make_risk(
                "make-recursive",
                "Makefile recipe invokes make recursively",
                true,
            ));
        }
        if current_targets.iter().any(|target| target == "install") {
            install_recipes.push(recipe.to_string());
        }
    }

    if text.contains("$(shell") || text.contains("${shell") {
        risks.push(make_risk(
            "make-shell-expansion",
            "Makefile uses shell expansion",
            true,
        ));
    }
    if !targets.iter().any(|target| target == "install") {
        risks.push(make_risk(
            "make-install-target-missing",
            "Makefile has no install target",
            true,
        ));
    }

    let mut install_actions = Vec::new();
    for recipe in install_recipes {
        match parse_install_action(&recipe, &variables) {
            Ok(action) => install_actions.push(action),
            Err(detail) => risks.push(make_risk("make-install-rejected", &detail, true)),
        }
    }
    if targets.iter().any(|target| target == "install") && install_actions.is_empty() {
        risks.push(make_risk(
            "make-install-actions-missing",
            "Install target contains no accepted copy or install actions",
            true,
        ));
    }
    targets.sort();
    risks.sort_by(|left, right| {
        left.code
            .cmp(&right.code)
            .then(left.detail.cmp(&right.detail))
    });
    risks.dedup();
    let accepted = !risks.iter().any(|risk| risk.blocking);
    Ok(MakeBuildPlan {
        makefile_path: display_relative(root, makefile_path),
        variables,
        targets,
        install_actions,
        accepted,
        risks,
    })
}

fn make_logical_lines(text: &str) -> Vec<String> {
    let mut lines = Vec::new();
    let mut pending = String::new();
    for raw in text.lines() {
        let continued = raw.trim_end().ends_with('\\');
        let part = if continued {
            raw.trim_end().trim_end_matches('\\')
        } else {
            raw
        };
        if pending.is_empty() {
            pending.push_str(part);
        } else {
            pending.push(' ');
            pending.push_str(part.trim_start());
        }
        if !continued {
            lines.push(std::mem::take(&mut pending));
        }
    }
    if !pending.is_empty() {
        lines.push(pending);
    }
    lines
}

fn parse_make_assignment(line: &str) -> Option<(&str, &str)> {
    let (left, value) = line.split_once('=')?;
    let name = left.trim_end_matches([' ', '\t', ':', '?', '+']).trim();
    (!name.is_empty()
        && name
            .bytes()
            .all(|byte| byte.is_ascii_alphanumeric() || byte == b'_'))
    .then(|| (name, value.trim()))
}

fn recipe_invokes_recursive_make(recipe: &str) -> bool {
    shlex::split(recipe).is_some_and(|tokens| {
        tokens
            .iter()
            .any(|token| matches!(token.as_str(), "make" | "gmake" | "$(MAKE)" | "${MAKE}"))
    })
}

fn parse_install_action(
    recipe: &str,
    declared_variables: &BTreeMap<String, String>,
) -> Result<MakeInstallAction, String> {
    let tokens = shlex::split(recipe)
        .ok_or_else(|| format!("Install recipe has invalid shell quoting: {recipe}"))?;
    if tokens.is_empty() {
        return Err("Install recipe is empty".into());
    }
    if tokens.iter().any(|token| {
        matches!(
            token.as_str(),
            ";" | "&&" | "||" | "|" | ">" | ">>" | "<" | "sudo" | "su" | "curl" | "wget"
        )
    }) {
        return Err(format!(
            "Install recipe contains a forbidden shell operation: {recipe}"
        ));
    }
    let command = tokens[0].rsplit('/').next().unwrap_or(&tokens[0]);
    if !matches!(command, "install" | "cp") {
        return Err(format!(
            "Install target command is not an accepted copy action: {command}"
        ));
    }
    let mut operands = Vec::new();
    let mut mode = None;
    let mut index = 1;
    while index < tokens.len() {
        let token = &tokens[index];
        if token == "-m" {
            index += 1;
            let value = tokens
                .get(index)
                .ok_or_else(|| "install -m has no mode".to_string())?;
            mode = Some(value.clone());
        } else if command == "install" && token.starts_with("-Dm") && token.len() > 3 {
            let value = &token[3..];
            if !value.bytes().all(|byte| byte.is_ascii_digit()) {
                return Err(format!("Install recipe has invalid mode {value}"));
            }
            mode = Some(value.to_string());
        } else if command == "install" && token.starts_with("-m") && token.len() > 2 {
            let value = &token[2..];
            if !value.bytes().all(|byte| byte.is_ascii_digit()) {
                return Err(format!("Install recipe has invalid mode {value}"));
            }
            mode = Some(value.to_string());
        } else if token.starts_with('-') {
            if !token
                .trim_start_matches('-')
                .bytes()
                .all(|byte| matches!(byte, b'D' | b'p' | b'v' | b'f'))
            {
                return Err(format!("Install recipe uses unsupported option {token}"));
            }
        } else {
            operands.push(token.clone());
        }
        index += 1;
    }
    if operands.len() != 2 {
        return Err(format!(
            "Install recipe must have exactly one source and one destination: {recipe}"
        ));
    }
    let staging_path = expand_staging_path(&operands[1], declared_variables)?;
    Ok(MakeInstallAction {
        command: command.to_string(),
        source: operands[0].clone(),
        staging_path,
        mode,
    })
}

fn expand_staging_path(
    destination: &str,
    declared_variables: &BTreeMap<String, String>,
) -> Result<String, String> {
    let mut variables = BTreeMap::from([
        ("DESTDIR".to_string(), STAGING_SENTINEL.to_string()),
        ("PREFIX".to_string(), "/usr".to_string()),
        ("BINDIR".to_string(), "$(PREFIX)/bin".to_string()),
        ("bindir".to_string(), "$(PREFIX)/bin".to_string()),
    ]);
    for name in ["BINDIR", "bindir"] {
        if let Some(value) = declared_variables.get(name) {
            variables.insert(name.to_string(), value.clone());
        }
    }
    let mut expanded = destination.to_string();
    for _ in 0..8 {
        let Some((start, end, name)) = next_make_variable(&expanded) else {
            break;
        };
        let value = variables
            .get(name)
            .ok_or_else(|| format!("Install destination uses unsupported variable {name}"))?;
        expanded.replace_range(start..end, value);
    }
    if next_make_variable(&expanded).is_some() || expanded.contains('$') {
        return Err(format!(
            "Install destination cannot be statically expanded: {destination}"
        ));
    }
    let normalized = normalize_absolute_path(Path::new(&expanded))?;
    let staging = Path::new(STAGING_SENTINEL);
    let relative = normalized.strip_prefix(staging).map_err(|_| {
        format!("Install destination does not resolve inside staging: {destination}")
    })?;
    if relative.as_os_str().is_empty() {
        return Err("Install destination cannot be the staging root".into());
    }
    Ok(format!("/{}", relative.to_string_lossy()))
}

fn next_make_variable(value: &str) -> Option<(usize, usize, &str)> {
    let start = value.find("$(").or_else(|| value.find("${"))?;
    let closing = if &value[start..start + 2] == "$(" {
        ')'
    } else {
        '}'
    };
    let tail = &value[start + 2..];
    let offset = tail.find(closing)?;
    let end = start + 2 + offset + 1;
    Some((start, end, &value[start + 2..end - 1]))
}

fn normalize_absolute_path(path: &Path) -> Result<PathBuf, String> {
    if !path.is_absolute() {
        return Err(format!(
            "Install destination is not absolute: {}",
            path.display()
        ));
    }
    let mut normalized = PathBuf::new();
    for component in path.components() {
        match component {
            Component::RootDir => normalized.push(component.as_os_str()),
            Component::Normal(value) => normalized.push(value),
            Component::CurDir => {}
            Component::ParentDir => {
                if !normalized.pop() {
                    return Err(format!(
                        "Install destination escapes root: {}",
                        path.display()
                    ));
                }
            }
            Component::Prefix(_) => {
                return Err(format!(
                    "Unsupported install destination: {}",
                    path.display()
                ))
            }
        }
    }
    Ok(normalized)
}

fn make_risk(code: &str, detail: &str, blocking: bool) -> SourceRisk {
    SourceRisk {
        code: code.to_string(),
        detail: detail.to_string(),
        blocking,
    }
}

fn display_relative(root: &Path, path: &Path) -> String {
    path.strip_prefix(root)
        .unwrap_or(path)
        .to_string_lossy()
        .into_owned()
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn cargo_inference_preserves_required_features_and_reports_risks() {
        let temp = tempfile::tempdir().unwrap();
        std::fs::create_dir_all(temp.path().join("src")).unwrap();
        std::fs::write(temp.path().join("src/main.rs"), "fn main() {}\n").unwrap();
        std::fs::write(temp.path().join("build.rs"), "fn main() {}\n").unwrap();
        std::fs::write(
            temp.path().join("Cargo.toml"),
            r#"
[package]
name = "fixture"
version = "1.0.0"
default-run = "fixture"
build = "build.rs"
links = "ssl"

[[bin]]
name = "helper"
path = "src/helper.rs"
required-features = ["cli", "tls"]

[dependencies]
shared = { path = "../shared" }
remote = { git = "https://example.invalid/repo.git", rev = "deadbeef" }
"#,
        )
        .unwrap();

        let report = infer(temp.path()).unwrap();
        let cargo = report.cargo.unwrap();
        let helper = cargo
            .binaries
            .iter()
            .find(|binary| binary.name == "helper")
            .unwrap();
        assert_eq!(helper.required_features, ["cli", "tls"]);
        assert_eq!(
            helper.build_arguments,
            [
                "build",
                "--release",
                "--bin",
                "helper",
                "--features",
                "cli,tls"
            ]
        );
        assert!(cargo
            .risks
            .iter()
            .any(|risk| risk.code == "cargo-build-script"));
        assert!(cargo
            .risks
            .iter()
            .any(|risk| risk.code == "cargo-native-links"));
        assert!(cargo
            .risks
            .iter()
            .any(|risk| risk.code == "cargo-path-dependency"));
        assert!(cargo
            .risks
            .iter()
            .any(|risk| risk.code == "cargo-git-dependency"));
        assert!(cargo
            .risks
            .iter()
            .any(|risk| risk.code == "cargo-lock-missing"));
    }

    #[test]
    fn cargo_inference_reads_literal_workspace_members() {
        let temp = tempfile::tempdir().unwrap();
        std::fs::create_dir_all(temp.path().join("tools/demo/src")).unwrap();
        std::fs::write(
            temp.path().join("Cargo.toml"),
            "[workspace]\nmembers = [\"tools/demo\"]\n",
        )
        .unwrap();
        std::fs::write(
            temp.path().join("tools/demo/Cargo.toml"),
            "[package]\nname = \"demo\"\nversion = \"1.0.0\"\n",
        )
        .unwrap();
        std::fs::write(temp.path().join("tools/demo/src/main.rs"), "fn main() {}\n").unwrap();

        let report = infer(temp.path()).unwrap();
        let cargo = report.cargo.unwrap();
        assert_eq!(cargo.binaries.len(), 1);
        assert_eq!(cargo.binaries[0].name, "demo");
        assert_eq!(cargo.binaries[0].manifest_path, "tools/demo/Cargo.toml");
    }

    #[test]
    fn make_inference_accepts_staged_install_actions() {
        let temp = tempfile::tempdir().unwrap();
        std::fs::write(
            temp.path().join("Makefile"),
            "PREFIX ?= /usr/local\nBINDIR = $(PREFIX)/bin\nbuild:\n\tcc -o tool tool.c\ninstall: build\n\tinstall -Dm755 tool $(DESTDIR)$(BINDIR)/tool\n",
        )
        .unwrap();

        let report = infer(temp.path()).unwrap();
        let make = report.make.unwrap();
        assert!(make.accepted);
        assert_eq!(make.targets, ["build", "install"]);
        assert_eq!(make.install_actions.len(), 1);
        assert_eq!(make.install_actions[0].staging_path, "/usr/bin/tool");
        assert_eq!(make.install_actions[0].mode.as_deref(), Some("755"));
    }

    #[test]
    fn make_inference_rejects_shell_network_and_non_staged_writes() {
        let temp = tempfile::tempdir().unwrap();
        std::fs::write(
            temp.path().join("Makefile"),
            "VERSION := $(shell curl https://example.invalid/version)\ninstall:\n\tsudo cp tool /usr/bin/tool\n",
        )
        .unwrap();

        let report = infer(temp.path()).unwrap();
        let make = report.make.unwrap();
        assert!(!make.accepted);
        assert!(make
            .risks
            .iter()
            .any(|risk| risk.code == "make-shell-expansion"));
        assert!(make
            .risks
            .iter()
            .any(|risk| risk.code == "make-install-rejected"));
        assert!(make.install_actions.is_empty());
    }
}
