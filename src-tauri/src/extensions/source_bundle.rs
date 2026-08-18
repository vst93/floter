use crate::extensions::source_inference;
use crate::extensions::source_resolver::{BundleSourceRequest, SourceProvider, SourceResolution};
use chrono::{SecondsFormat, Utc};
use flate2::read::GzDecoder;
use flate2::write::GzEncoder;
use flate2::Compression;
use serde::{Deserialize, Serialize};
use sha2::{Digest, Sha256};
use std::collections::{BTreeMap, BTreeSet};
use std::fs::{File, OpenOptions};
use std::io::{BufReader, Read, Write};
use std::path::{Component, Path, PathBuf};

const BUNDLE_SCHEMA_VERSION: u32 = 1;
const BUNDLE_FORMAT: &str = "floter-source-bundle";
const BUNDLE_MANIFEST_PATH: &str = "floter-bundle.json";
const SOURCE_PREFIX: &str = "source";
const CACHE_RECORD_PATH: &str = "bundle.json";
const MAX_BUNDLE_BYTES: u64 = 512 * 1024 * 1024;
const MAX_EXTRACTED_BYTES: u64 = 2 * 1024 * 1024 * 1024;
const MAX_MANIFEST_BYTES: u64 = 32 * 1024 * 1024;
const MAX_ENTRIES: usize = 200_000;

#[derive(Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct SourceBundleExportRequest {
    pub source_path: String,
    pub bundle_path: String,
    #[serde(default)]
    pub project: Option<String>,
    #[serde(default)]
    pub revision: Option<String>,
}

#[derive(Debug, Clone, Serialize, PartialEq, Eq)]
#[serde(rename_all = "camelCase")]
pub struct SourceBundleExportResult {
    pub path: String,
    pub project: String,
    pub revision: String,
    pub bundle_sha256: String,
    pub source_sha256: String,
    pub file_count: usize,
    pub uncompressed_bytes: u64,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
struct BundleManifest {
    schema_version: u32,
    format: String,
    created_at: String,
    project: String,
    revision: String,
    source_sha256: String,
    files: Vec<BundleFile>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
struct BundleFile {
    path: String,
    size: u64,
    mode: u32,
    sha256: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
struct BundleCacheRecord {
    schema_version: u32,
    bundle_sha256: String,
    manifest: BundleManifest,
}

pub fn export(request: &SourceBundleExportRequest) -> Result<SourceBundleExportResult, String> {
    let source = Path::new(&request.source_path)
        .canonicalize()
        .map_err(|error| format!("Cannot resolve source directory: {error}"))?;
    if !source.is_dir() {
        return Err(format!(
            "Source path is not a directory: {}",
            source.display()
        ));
    }
    source_inference::infer(&source)?;

    let destination = absolute_destination(Path::new(&request.bundle_path))?;
    if destination
        .parent()
        .is_some_and(|parent| parent.starts_with(&source))
    {
        return Err("Source bundle must be written outside the source directory".to_string());
    }

    let mut files = Vec::new();
    collect_files(&source, &source, &mut files)?;
    files.sort_by(|left, right| left.path.cmp(&right.path));
    if files.is_empty() {
        return Err("Source bundle cannot be empty".to_string());
    }
    let uncompressed_bytes = files.iter().try_fold(0_u64, |total, file| {
        total
            .checked_add(file.size)
            .ok_or_else(|| "Source bundle size overflow".to_string())
    })?;
    if uncompressed_bytes > MAX_EXTRACTED_BYTES {
        return Err(format!(
            "Source directory exceeds the {MAX_EXTRACTED_BYTES} byte bundle limit"
        ));
    }
    let source_sha256 = source_digest(&files);
    let project = normalized_label(
        request
            .project
            .as_deref()
            .or_else(|| source.file_name().and_then(|name| name.to_str())),
        "project",
    )?;
    let revision = normalized_label(
        request.revision.as_deref().or(Some(source_sha256.as_str())),
        "revision",
    )?;
    let manifest = BundleManifest {
        schema_version: BUNDLE_SCHEMA_VERSION,
        format: BUNDLE_FORMAT.to_string(),
        created_at: Utc::now().to_rfc3339_opts(SecondsFormat::Secs, true),
        project: project.clone(),
        revision: revision.clone(),
        source_sha256: source_sha256.clone(),
        files,
    };
    validate_manifest(&manifest)?;
    let manifest_bytes = serde_json::to_vec_pretty(&manifest)
        .map_err(|error| format!("Cannot serialize source bundle manifest: {error}"))?;

    let parent = destination
        .parent()
        .ok_or("Invalid source bundle destination")?;
    let mut temporary = tempfile::NamedTempFile::new_in(parent)
        .map_err(|error| format!("Cannot create source bundle: {error}"))?;
    {
        let encoder = GzEncoder::new(temporary.as_file_mut(), Compression::default());
        let mut archive = tar::Builder::new(encoder);
        append_bytes(&mut archive, BUNDLE_MANIFEST_PATH, &manifest_bytes, 0o644)?;
        for file in &manifest.files {
            let archive_path = format!("{SOURCE_PREFIX}/{}", file.path);
            let path = source.join(path_from_bundle(&file.path)?);
            let mut input = File::open(&path)
                .map_err(|error| format!("Cannot read source file {}: {error}", path.display()))?;
            append_reader(
                &mut archive,
                &archive_path,
                &mut input,
                file.size,
                file.mode,
            )?;
        }
        let encoder = archive
            .into_inner()
            .map_err(|error| format!("Cannot finish source bundle archive: {error}"))?;
        encoder
            .finish()
            .map_err(|error| format!("Cannot finish source bundle compression: {error}"))?;
    }
    temporary
        .as_file_mut()
        .flush()
        .and_then(|_| temporary.as_file().sync_all())
        .map_err(|error| format!("Cannot flush source bundle: {error}"))?;
    let bundle_sha256 = hash_file_limited(temporary.path(), MAX_BUNDLE_BYTES, "source bundle")?;
    temporary
        .persist(&destination)
        .map_err(|error| format!("Cannot persist source bundle: {error}"))?;
    sync_parent(parent)?;

    Ok(SourceBundleExportResult {
        path: destination.to_string_lossy().into_owned(),
        project,
        revision,
        bundle_sha256,
        source_sha256,
        file_count: manifest.files.len(),
        uncompressed_bytes,
    })
}

pub fn import(
    cache_root: &Path,
    request: &BundleSourceRequest,
) -> Result<SourceResolution, String> {
    let bundle = Path::new(&request.path)
        .canonicalize()
        .map_err(|error| format!("Cannot resolve source bundle: {error}"))?;
    if !bundle.is_file() {
        return Err(format!(
            "Source bundle is not a regular file: {}",
            bundle.display()
        ));
    }
    let bundle_sha256 = hash_file_limited(&bundle, MAX_BUNDLE_BYTES, "source bundle")?;
    let cache_parent = cache_root.join("sources").join("bundle");
    let target = cache_parent.join(&bundle_sha256);
    if target.is_dir() {
        let record = read_cached(&target, &bundle_sha256)?;
        verify_cached_source(&target.join(SOURCE_PREFIX), &record.manifest)?;
        return Ok(resolution(&target, &bundle_sha256, record.manifest, true));
    }

    std::fs::create_dir_all(&cache_parent)
        .map_err(|error| format!("Cannot create source bundle cache: {error}"))?;
    let staging = tempfile::Builder::new()
        .prefix("bundle-")
        .tempdir_in(&cache_parent)
        .map_err(|error| format!("Cannot create source bundle staging directory: {error}"))?;
    let payload = staging.path().join("payload");
    let source = payload.join(SOURCE_PREFIX);
    std::fs::create_dir_all(&source)
        .map_err(|error| format!("Cannot create source bundle staging directory: {error}"))?;
    let manifest = extract_bundle(&bundle, &source)?;
    source_inference::infer(&source)?;
    let record = BundleCacheRecord {
        schema_version: BUNDLE_SCHEMA_VERSION,
        bundle_sha256: bundle_sha256.clone(),
        manifest: manifest.clone(),
    };
    write_json(
        &payload.join(CACHE_RECORD_PATH),
        &record,
        "source bundle cache record",
    )?;
    match std::fs::rename(&payload, &target) {
        Ok(()) => sync_parent(&cache_parent)?,
        Err(_) if target.is_dir() => {
            let cached = read_cached(&target, &bundle_sha256)?;
            verify_cached_source(&target.join(SOURCE_PREFIX), &cached.manifest)?;
            return Ok(resolution(&target, &bundle_sha256, cached.manifest, true));
        }
        Err(error) => return Err(format!("Cannot commit source bundle cache: {error}")),
    }
    Ok(resolution(&target, &bundle_sha256, manifest, false))
}

fn extract_bundle(path: &Path, destination: &Path) -> Result<BundleManifest, String> {
    let file = File::open(path).map_err(|error| format!("Cannot open source bundle: {error}"))?;
    let decoder = GzDecoder::new(BufReader::new(file));
    let mut archive = tar::Archive::new(decoder);
    let mut manifest: Option<BundleManifest> = None;
    let mut expected = BTreeMap::new();
    let mut seen = BTreeSet::new();
    let mut extracted_bytes = 0_u64;
    let entries = archive
        .entries()
        .map_err(|error| format!("Invalid source bundle archive: {error}"))?;
    for (index, entry) in entries.enumerate() {
        if index > MAX_ENTRIES {
            return Err(format!(
                "Source bundle contains more than {MAX_ENTRIES} files"
            ));
        }
        let mut entry = entry.map_err(|error| format!("Invalid source bundle entry: {error}"))?;
        let entry_path = entry
            .path()
            .map_err(|error| format!("Invalid source bundle path: {error}"))?
            .into_owned();
        let kind = entry.header().entry_type();
        if kind.is_symlink() || kind.is_hard_link() {
            return Err(format!(
                "Source bundle links are not allowed: {}",
                entry_path.display()
            ));
        }
        if entry_path == Path::new(BUNDLE_MANIFEST_PATH) {
            if manifest.is_some() || !kind.is_file() {
                return Err("Source bundle has an invalid duplicate manifest".to_string());
            }
            if entry.size() > MAX_MANIFEST_BYTES {
                return Err("Source bundle manifest is too large".to_string());
            }
            let mut bytes = Vec::with_capacity(entry.size() as usize);
            entry
                .read_to_end(&mut bytes)
                .map_err(|error| format!("Cannot read source bundle manifest: {error}"))?;
            let parsed: BundleManifest = serde_json::from_slice(&bytes)
                .map_err(|error| format!("Invalid source bundle manifest: {error}"))?;
            validate_manifest(&parsed)?;
            expected = parsed
                .files
                .iter()
                .cloned()
                .map(|file| (file.path.clone(), file))
                .collect();
            manifest = Some(parsed);
            continue;
        }
        let relative = source_entry_path(&entry_path)?;
        if kind.is_dir() {
            std::fs::create_dir_all(destination.join(&relative))
                .map_err(|error| format!("Cannot create bundled source directory: {error}"))?;
            continue;
        }
        if !kind.is_file() {
            return Err(format!(
                "Unsupported source bundle entry: {}",
                entry_path.display()
            ));
        }
        let key = bundle_path(&relative)?;
        let expected_file = expected
            .get(&key)
            .ok_or_else(|| format!("Source bundle file is not declared: {key}"))?;
        if !seen.insert(key.clone()) {
            return Err(format!("Source bundle contains duplicate file: {key}"));
        }
        if entry.size() != expected_file.size {
            return Err(format!(
                "Source bundle file size does not match manifest: {key}"
            ));
        }
        extracted_bytes = extracted_bytes
            .checked_add(entry.size())
            .ok_or("Source bundle extracted size overflow")?;
        if extracted_bytes > MAX_EXTRACTED_BYTES {
            return Err("Source bundle expands beyond the source size limit".to_string());
        }
        let target = destination.join(&relative);
        if let Some(parent) = target.parent() {
            std::fs::create_dir_all(parent)
                .map_err(|error| format!("Cannot create bundled source directory: {error}"))?;
        }
        let mut output = OpenOptions::new()
            .create_new(true)
            .write(true)
            .open(&target)
            .map_err(|error| format!("Cannot create bundled source file {key}: {error}"))?;
        let digest = copy_and_hash(&mut entry, &mut output, expected_file.size)?;
        output
            .flush()
            .and_then(|_| output.sync_all())
            .map_err(|error| format!("Cannot flush bundled source file {key}: {error}"))?;
        if digest != expected_file.sha256 {
            return Err(format!("Source bundle checksum mismatch: {key}"));
        }
        set_mode(&target, expected_file.mode)?;
    }
    let manifest = manifest.ok_or("Source bundle manifest is missing")?;
    if seen.len() != expected.len() {
        let missing = expected
            .keys()
            .find(|path| !seen.contains(*path))
            .cloned()
            .unwrap_or_else(|| "unknown".to_string());
        return Err(format!("Source bundle file is missing: {missing}"));
    }
    Ok(manifest)
}

fn collect_files(root: &Path, directory: &Path, files: &mut Vec<BundleFile>) -> Result<(), String> {
    let mut entries = std::fs::read_dir(directory)
        .map_err(|error| {
            format!(
                "Cannot read source directory {}: {error}",
                directory.display()
            )
        })?
        .collect::<Result<Vec<_>, _>>()
        .map_err(|error| format!("Cannot read source directory entry: {error}"))?;
    entries.sort_by_key(|entry| entry.file_name());
    for entry in entries {
        let path = entry.path();
        let file_type = entry
            .file_type()
            .map_err(|error| format!("Cannot inspect source path {}: {error}", path.display()))?;
        if file_type.is_symlink() {
            return Err(format!(
                "Source bundles do not allow symbolic links: {}",
                path.display()
            ));
        }
        if file_type.is_dir() {
            collect_files(root, &path, files)?;
            continue;
        }
        if !file_type.is_file() {
            return Err(format!("Unsupported source file type: {}", path.display()));
        }
        if files.len() >= MAX_ENTRIES {
            return Err(format!(
                "Source directory contains more than {MAX_ENTRIES} files"
            ));
        }
        let relative = path
            .strip_prefix(root)
            .map_err(|_| "Source file escaped the source directory")?;
        let portable_path = bundle_path(relative)?;
        let metadata = entry
            .metadata()
            .map_err(|error| format!("Cannot inspect source file {}: {error}", path.display()))?;
        files.push(BundleFile {
            path: portable_path,
            size: metadata.len(),
            mode: file_mode(&metadata),
            sha256: hash_file_limited(&path, MAX_EXTRACTED_BYTES, "source file")?,
        });
    }
    Ok(())
}

fn validate_manifest(manifest: &BundleManifest) -> Result<(), String> {
    if manifest.schema_version != BUNDLE_SCHEMA_VERSION || manifest.format != BUNDLE_FORMAT {
        return Err("Unsupported source bundle format or schema version".to_string());
    }
    chrono::DateTime::parse_from_rfc3339(&manifest.created_at)
        .map_err(|error| format!("Invalid source bundle timestamp: {error}"))?;
    normalized_label(Some(&manifest.project), "project")?;
    normalized_label(Some(&manifest.revision), "revision")?;
    validate_sha256(&manifest.source_sha256, "source bundle tree")?;
    if manifest.files.is_empty() || manifest.files.len() > MAX_ENTRIES {
        return Err("Source bundle has an invalid file count".to_string());
    }
    let mut paths = BTreeSet::new();
    let mut total = 0_u64;
    for file in &manifest.files {
        let path = path_from_bundle(&file.path)?;
        if bundle_path(&path)? != file.path || !paths.insert(file.path.as_str()) {
            return Err(format!(
                "Invalid or duplicate source bundle path: {}",
                file.path
            ));
        }
        validate_sha256(&file.sha256, "source bundle file")?;
        total = total
            .checked_add(file.size)
            .ok_or("Source bundle size overflow")?;
        if total > MAX_EXTRACTED_BYTES {
            return Err("Source bundle expands beyond the source size limit".to_string());
        }
    }
    let mut files = manifest.files.clone();
    files.sort_by(|left, right| left.path.cmp(&right.path));
    if source_digest(&files) != manifest.source_sha256 {
        return Err("Source bundle tree checksum does not match its manifest".to_string());
    }
    Ok(())
}

fn verify_cached_source(root: &Path, manifest: &BundleManifest) -> Result<(), String> {
    validate_manifest(manifest)?;
    for file in &manifest.files {
        let path = root.join(path_from_bundle(&file.path)?);
        let metadata = std::fs::metadata(&path)
            .map_err(|error| format!("Cached source file is missing {}: {error}", file.path))?;
        if !metadata.is_file() || metadata.len() != file.size {
            return Err(format!(
                "Cached source file does not match bundle: {}",
                file.path
            ));
        }
        if hash_file_limited(&path, file.size, "cached source file")? != file.sha256 {
            return Err(format!("Cached source checksum mismatch: {}", file.path));
        }
    }
    source_inference::infer(root)?;
    Ok(())
}

fn read_cached(target: &Path, bundle_sha256: &str) -> Result<BundleCacheRecord, String> {
    let bytes = std::fs::read(target.join(CACHE_RECORD_PATH))
        .map_err(|error| format!("Cannot read source bundle cache record: {error}"))?;
    let record: BundleCacheRecord = serde_json::from_slice(&bytes)
        .map_err(|error| format!("Invalid source bundle cache record: {error}"))?;
    if record.schema_version != BUNDLE_SCHEMA_VERSION || record.bundle_sha256 != bundle_sha256 {
        return Err("Source bundle cache record does not match its cache path".to_string());
    }
    Ok(record)
}

fn resolution(
    target: &Path,
    bundle_sha256: &str,
    manifest: BundleManifest,
    cached: bool,
) -> SourceResolution {
    SourceResolution {
        provider: SourceProvider::Bundle,
        server_url: String::new(),
        project: manifest.project,
        requested_reference: None,
        resolved_reference: manifest.revision.clone(),
        revision: manifest.revision,
        project_root: target.join(SOURCE_PREFIX).to_string_lossy().into_owned(),
        archive_sha256: bundle_sha256.to_string(),
        cached,
    }
}

fn append_bytes<W: Write>(
    archive: &mut tar::Builder<W>,
    path: &str,
    bytes: &[u8],
    mode: u32,
) -> Result<(), String> {
    append_reader(archive, path, &mut &bytes[..], bytes.len() as u64, mode)
}

fn append_reader<W: Write, R: Read>(
    archive: &mut tar::Builder<W>,
    path: &str,
    reader: &mut R,
    size: u64,
    mode: u32,
) -> Result<(), String> {
    let mut header = tar::Header::new_gnu();
    header.set_entry_type(tar::EntryType::Regular);
    header.set_size(size);
    header.set_mode(mode);
    header.set_uid(0);
    header.set_gid(0);
    header.set_mtime(0);
    header.set_cksum();
    archive
        .append_data(&mut header, path, reader)
        .map_err(|error| format!("Cannot append {path} to source bundle: {error}"))
}

fn copy_and_hash<R: Read, W: Write>(
    reader: &mut R,
    writer: &mut W,
    size: u64,
) -> Result<String, String> {
    let mut remaining = size;
    let mut digest = Sha256::new();
    let mut buffer = [0_u8; 64 * 1024];
    while remaining > 0 {
        let read_length = buffer.len().min(remaining as usize);
        let count = reader
            .read(&mut buffer[..read_length])
            .map_err(|error| format!("Cannot read bundled source file: {error}"))?;
        if count == 0 {
            return Err("Source bundle file ended before its declared size".to_string());
        }
        writer
            .write_all(&buffer[..count])
            .map_err(|error| format!("Cannot write bundled source file: {error}"))?;
        digest.update(&buffer[..count]);
        remaining -= count as u64;
    }
    Ok(format!("{:x}", digest.finalize()))
}

fn hash_file_limited(path: &Path, limit: u64, label: &str) -> Result<String, String> {
    let metadata = std::fs::metadata(path)
        .map_err(|error| format!("Cannot inspect {label} {}: {error}", path.display()))?;
    if !metadata.is_file() || metadata.len() > limit {
        return Err(format!("{label} exceeds the {limit} byte limit"));
    }
    let mut file = File::open(path)
        .map_err(|error| format!("Cannot read {label} {}: {error}", path.display()))?;
    let mut digest = Sha256::new();
    let mut buffer = [0_u8; 64 * 1024];
    loop {
        let count = file
            .read(&mut buffer)
            .map_err(|error| format!("Cannot read {label}: {error}"))?;
        if count == 0 {
            break;
        }
        digest.update(&buffer[..count]);
    }
    Ok(format!("{:x}", digest.finalize()))
}

fn source_digest(files: &[BundleFile]) -> String {
    let mut digest = Sha256::new();
    for file in files {
        digest.update((file.path.len() as u64).to_be_bytes());
        digest.update(file.path.as_bytes());
        digest.update(file.size.to_be_bytes());
        digest.update(file.mode.to_be_bytes());
        digest.update(file.sha256.as_bytes());
    }
    format!("{:x}", digest.finalize())
}

fn source_entry_path(path: &Path) -> Result<PathBuf, String> {
    let mut components = path.components();
    match components.next() {
        Some(Component::Normal(root)) if root == SOURCE_PREFIX => {}
        _ => return Err(format!("Unexpected source bundle path: {}", path.display())),
    }
    let mut relative = PathBuf::new();
    for component in components {
        match component {
            Component::Normal(value) => relative.push(value),
            _ => return Err(format!("Unsafe source bundle path: {}", path.display())),
        }
    }
    if relative.as_os_str().is_empty() {
        return Err("Source bundle file path is empty".to_string());
    }
    Ok(relative)
}

fn bundle_path(path: &Path) -> Result<String, String> {
    let mut parts = Vec::new();
    for component in path.components() {
        match component {
            Component::Normal(value) => parts.push(
                value
                    .to_str()
                    .ok_or_else(|| format!("Source path is not valid UTF-8: {}", path.display()))?,
            ),
            _ => return Err(format!("Source path is not portable: {}", path.display())),
        }
    }
    if parts.is_empty() {
        return Err("Source path is empty".to_string());
    }
    Ok(parts.join("/"))
}

fn path_from_bundle(value: &str) -> Result<PathBuf, String> {
    if value.is_empty() || value.starts_with('/') || value.contains('\\') {
        return Err(format!("Invalid source bundle path: {value}"));
    }
    let mut path = PathBuf::new();
    for segment in value.split('/') {
        if segment.is_empty()
            || matches!(segment, "." | "..")
            || segment.chars().any(char::is_control)
        {
            return Err(format!("Invalid source bundle path: {value}"));
        }
        path.push(segment);
    }
    Ok(path)
}

fn normalized_label(value: Option<&str>, label: &str) -> Result<String, String> {
    let value = value.unwrap_or("").trim();
    if value.is_empty() || value.len() > 1024 || value.chars().any(char::is_control) {
        return Err(format!(
            "Source bundle {label} must contain 1 to 1024 printable bytes"
        ));
    }
    Ok(value.to_string())
}

fn validate_sha256(value: &str, label: &str) -> Result<(), String> {
    if value.len() == 64 && value.bytes().all(|byte| byte.is_ascii_hexdigit()) {
        Ok(())
    } else {
        Err(format!("Invalid {label} SHA-256 digest"))
    }
}

fn absolute_destination(path: &Path) -> Result<PathBuf, String> {
    let name = path
        .file_name()
        .ok_or("Source bundle destination has no file name")?;
    let parent = path.parent().unwrap_or_else(|| Path::new("."));
    let parent = parent
        .canonicalize()
        .map_err(|error| format!("Cannot resolve source bundle destination: {error}"))?;
    Ok(parent.join(name))
}

fn write_json<T: Serialize>(path: &Path, value: &T, label: &str) -> Result<(), String> {
    let bytes = serde_json::to_vec_pretty(value)
        .map_err(|error| format!("Cannot serialize {label}: {error}"))?;
    let mut file = OpenOptions::new()
        .create_new(true)
        .write(true)
        .open(path)
        .map_err(|error| format!("Cannot create {label}: {error}"))?;
    file.write_all(&bytes)
        .and_then(|_| file.flush())
        .and_then(|_| file.sync_all())
        .map_err(|error| format!("Cannot write {label}: {error}"))
}

#[cfg(unix)]
fn file_mode(metadata: &std::fs::Metadata) -> u32 {
    use std::os::unix::fs::PermissionsExt;
    metadata.permissions().mode() & 0o777
}

#[cfg(not(unix))]
fn file_mode(_metadata: &std::fs::Metadata) -> u32 {
    0o644
}

#[cfg(unix)]
fn set_mode(path: &Path, mode: u32) -> Result<(), String> {
    use std::os::unix::fs::PermissionsExt;
    std::fs::set_permissions(path, std::fs::Permissions::from_mode(mode & 0o777))
        .map_err(|error| format!("Cannot set bundled source file mode: {error}"))
}

#[cfg(not(unix))]
fn set_mode(_path: &Path, _mode: u32) -> Result<(), String> {
    Ok(())
}

fn sync_parent(path: &Path) -> Result<(), String> {
    crate::extensions::lock::sync_directory(path)
        .map_err(|error| format!("Cannot sync source bundle directory: {error}"))
}

#[cfg(test)]
mod tests {
    use super::*;

    fn sample_source(root: &Path) {
        std::fs::create_dir_all(root.join("src")).unwrap();
        std::fs::write(
            root.join("Cargo.toml"),
            b"[package]\nname = \"offline-sample\"\nversion = \"0.1.0\"\nedition = \"2021\"\n",
        )
        .unwrap();
        std::fs::write(root.join("src/main.rs"), b"fn main() {}\n").unwrap();
    }

    #[test]
    fn exports_imports_and_reuses_a_verified_source_bundle() {
        let temporary = tempfile::tempdir().unwrap();
        let source = temporary.path().join("project");
        std::fs::create_dir(&source).unwrap();
        sample_source(&source);
        let bundle = temporary.path().join("project.floter-bundle");
        let exported = export(&SourceBundleExportRequest {
            source_path: source.to_string_lossy().into_owned(),
            bundle_path: bundle.to_string_lossy().into_owned(),
            project: Some("group/project".into()),
            revision: Some("deadbeef".into()),
        })
        .unwrap();
        assert_eq!(exported.file_count, 2);

        let cache = temporary.path().join("cache");
        let request = BundleSourceRequest {
            path: bundle.to_string_lossy().into_owned(),
        };
        let imported = import(&cache, &request).unwrap();
        assert_eq!(imported.provider, SourceProvider::Bundle);
        assert_eq!(imported.project, "group/project");
        assert_eq!(imported.revision, "deadbeef");
        assert!(!imported.cached);
        assert_eq!(
            std::fs::read_to_string(Path::new(&imported.project_root).join("src/main.rs")).unwrap(),
            "fn main() {}\n"
        );
        assert!(import(&cache, &request).unwrap().cached);
    }

    #[cfg(unix)]
    #[test]
    fn export_rejects_symbolic_links() {
        use std::os::unix::fs::symlink;
        let temporary = tempfile::tempdir().unwrap();
        let source = temporary.path().join("project");
        std::fs::create_dir(&source).unwrap();
        sample_source(&source);
        symlink("Cargo.toml", source.join("linked.toml")).unwrap();
        let request = SourceBundleExportRequest {
            source_path: source.to_string_lossy().into_owned(),
            bundle_path: temporary
                .path()
                .join("bundle")
                .to_string_lossy()
                .into_owned(),
            project: None,
            revision: None,
        };
        assert!(export(&request).unwrap_err().contains("symbolic links"));
    }

    #[test]
    fn manifest_rejects_parent_paths_and_tree_tampering() {
        let file = BundleFile {
            path: "../Cargo.toml".into(),
            size: 1,
            mode: 0o644,
            sha256: format!("{:x}", Sha256::digest(b"x")),
        };
        let manifest = BundleManifest {
            schema_version: BUNDLE_SCHEMA_VERSION,
            format: BUNDLE_FORMAT.into(),
            created_at: Utc::now().to_rfc3339(),
            project: "project".into(),
            revision: "revision".into(),
            source_sha256: source_digest(std::slice::from_ref(&file)),
            files: vec![file],
        };
        assert!(validate_manifest(&manifest).unwrap_err().contains("path"));
    }
}
