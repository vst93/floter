use crate::extensions::source_resolver::{GitlabSourceRequest, SourceProvider, SourceResolution};
use crate::extensions::ExtensionState;
use flate2::read::GzDecoder;
use reqwest::header::HeaderValue;
use reqwest::{Response, Url};
use serde::{Deserialize, Serialize};
use sha2::{Digest, Sha256};
use std::ffi::OsString;
use std::io::Write;
use std::path::{Component, Path, PathBuf};

const MAX_GITLAB_METADATA_BYTES: usize = 1024 * 1024;
const MAX_GITLAB_ARCHIVE_BYTES: usize = 256 * 1024 * 1024;
const MAX_GITLAB_EXTRACTED_BYTES: u64 = 1024 * 1024 * 1024;
const MAX_GITLAB_ARCHIVE_ENTRIES: usize = 200_000;
const CACHE_RECORD_FILE: &str = ".floter-source.json";

#[derive(Debug, Deserialize)]
struct GitlabProject {
    id: u64,
    path_with_namespace: String,
    web_url: String,
    default_branch: Option<String>,
}

#[derive(Debug, Deserialize)]
struct GitlabCommit {
    id: String,
}

#[derive(Debug, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "camelCase")]
struct SourceCacheRecord {
    schema_version: u32,
    server_url: String,
    project_id: u64,
    project: String,
    requested_reference: Option<String>,
    resolved_reference: String,
    revision: String,
    archive_sha256: String,
}

pub(super) async fn resolve(
    state: &ExtensionState,
    request: GitlabSourceRequest,
) -> Result<SourceResolution, String> {
    let server = validate_server_url(&request.server_url)?;
    let project_path = validate_project_path(&request.project)?;
    let token = validate_token(request.access_token.as_deref())?;
    let api_root = api_root(&server)?;

    let project_url = project_endpoint(&api_root, &project_path)?;
    let project: GitlabProject = get_json(
        &state.client,
        project_url,
        token.as_ref(),
        "GitLab project metadata",
    )
    .await?;
    validate_project(&project, &server)?;

    let requested_reference = request
        .reference
        .as_deref()
        .map(validate_reference)
        .transpose()?
        .map(str::to_owned);
    let resolved_reference = requested_reference
        .clone()
        .or(project.default_branch.clone())
        .ok_or_else(|| "GitLab project has no default branch; specify a reference".to_string())?;

    let commit_url = commit_endpoint(&api_root, project.id, &resolved_reference)?;
    let commit: GitlabCommit = get_json(
        &state.client,
        commit_url,
        token.as_ref(),
        "GitLab commit metadata",
    )
    .await?;
    validate_revision(&commit.id)?;

    let cache_parent = cache_parent(&state.paths.cache, &server, project.id);
    let target = cache_parent.join(&commit.id);
    if let Some(mut record) = read_cached_resolution(&target, &server, project.id, &commit.id)? {
        record.project = project.path_with_namespace;
        record.requested_reference = requested_reference;
        record.resolved_reference = resolved_reference;
        return Ok(resolution_from_record(&target, record, true));
    }

    let archive_url = archive_endpoint(&api_root, project.id, &commit.id)?;
    let archive = get_bytes(
        &state.client,
        archive_url,
        token.as_ref(),
        MAX_GITLAB_ARCHIVE_BYTES,
        "GitLab source archive",
    )
    .await?;
    let archive_sha256 = format!("{:x}", Sha256::digest(&archive));
    let record = SourceCacheRecord {
        schema_version: 1,
        server_url: normalized_server_url(&server),
        project_id: project.id,
        project: project.path_with_namespace,
        requested_reference,
        resolved_reference,
        revision: commit.id,
        archive_sha256,
    };
    cache_archive(&cache_parent, &target, &archive, &record)?;
    Ok(resolution_from_record(&target, record, false))
}

fn validate_server_url(value: &str) -> Result<Url, String> {
    let mut url =
        Url::parse(value).map_err(|error| format!("Invalid GitLab server URL: {error}"))?;
    if url.scheme() != "https" {
        return Err("GitLab server URL must use HTTPS".to_string());
    }
    if !url.username().is_empty() || url.password().is_some() {
        return Err("GitLab server URL must not contain credentials".to_string());
    }
    if url.query().is_some() || url.fragment().is_some() {
        return Err("GitLab server URL must not contain a query or fragment".to_string());
    }
    if url.host_str().is_none() || url.cannot_be_a_base() {
        return Err("GitLab server URL must be an absolute HTTPS URL".to_string());
    }
    if !url.path().ends_with('/') {
        let path = format!("{}/", url.path());
        url.set_path(&path);
    }
    Ok(url)
}

fn normalized_server_url(url: &Url) -> String {
    url.as_str().trim_end_matches('/').to_string()
}

fn validate_project_path(value: &str) -> Result<String, String> {
    let value = value.trim().trim_matches('/');
    let value = value.strip_suffix(".git").unwrap_or(value);
    if value.is_empty() || value.len() > 512 {
        return Err("GitLab project path must contain 1 to 512 bytes".to_string());
    }
    if value.split('/').any(|segment| {
        segment.is_empty() || matches!(segment, "." | "..") || segment.chars().any(char::is_control)
    }) {
        return Err("GitLab project path contains an invalid segment".to_string());
    }
    Ok(value.to_string())
}

fn validate_reference(value: &str) -> Result<&str, String> {
    let value = value.trim();
    if value.is_empty() || value.len() > 1024 || value.chars().any(char::is_control) {
        return Err("GitLab reference must contain 1 to 1024 printable bytes".to_string());
    }
    Ok(value)
}

fn validate_token(value: Option<&str>) -> Result<Option<HeaderValue>, String> {
    let Some(value) = value else {
        return Ok(None);
    };
    if value.is_empty() {
        return Err("GitLab access token must not be empty".to_string());
    }
    let mut value = HeaderValue::from_str(value)
        .map_err(|_| "GitLab access token contains invalid header characters".to_string())?;
    value.set_sensitive(true);
    Ok(Some(value))
}

fn validate_revision(value: &str) -> Result<(), String> {
    if matches!(value.len(), 40 | 64) && value.bytes().all(|byte| byte.is_ascii_hexdigit()) {
        Ok(())
    } else {
        Err("GitLab returned an invalid commit revision".to_string())
    }
}

fn validate_project(project: &GitlabProject, server: &Url) -> Result<(), String> {
    validate_project_path(&project.path_with_namespace)?;
    let web_url = Url::parse(&project.web_url)
        .map_err(|error| format!("GitLab returned an invalid project URL: {error}"))?;
    if web_url.scheme() != "https" || web_url.host_str() != server.host_str() {
        return Err("GitLab project URL does not match the requested server".to_string());
    }
    Ok(())
}

fn api_root(server: &Url) -> Result<Url, String> {
    server
        .join("api/v4/")
        .map_err(|error| format!("Cannot construct GitLab API URL: {error}"))
}

fn project_endpoint(api_root: &Url, project: &str) -> Result<Url, String> {
    let mut url = api_root
        .join("projects/")
        .map_err(|error| format!("Cannot construct GitLab project URL: {error}"))?;
    {
        let mut segments = url
            .path_segments_mut()
            .map_err(|_| "GitLab API URL cannot contain path segments".to_string())?;
        segments.pop_if_empty().push(project);
    }
    Ok(url)
}

fn commit_endpoint(api_root: &Url, project_id: u64, reference: &str) -> Result<Url, String> {
    let mut url = api_root
        .join(&format!("projects/{project_id}/repository/commits/"))
        .map_err(|error| format!("Cannot construct GitLab commit URL: {error}"))?;
    {
        let mut segments = url
            .path_segments_mut()
            .map_err(|_| "GitLab API URL cannot contain path segments".to_string())?;
        segments.pop_if_empty().push(reference);
    }
    Ok(url)
}

fn archive_endpoint(api_root: &Url, project_id: u64, revision: &str) -> Result<Url, String> {
    let mut url = api_root
        .join(&format!("projects/{project_id}/repository/archive.tar.gz"))
        .map_err(|error| format!("Cannot construct GitLab archive URL: {error}"))?;
    url.query_pairs_mut().append_pair("sha", revision);
    Ok(url)
}

async fn get_json<T: for<'de> Deserialize<'de>>(
    client: &reqwest::Client,
    url: Url,
    token: Option<&HeaderValue>,
    label: &str,
) -> Result<T, String> {
    let response = send(client, url, token, label).await?;
    let bytes = read_response_limited(response, MAX_GITLAB_METADATA_BYTES, label).await?;
    serde_json::from_slice(&bytes).map_err(|error| format!("Invalid {label}: {error}"))
}

async fn get_bytes(
    client: &reqwest::Client,
    url: Url,
    token: Option<&HeaderValue>,
    limit: usize,
    label: &str,
) -> Result<Vec<u8>, String> {
    let response = send(client, url, token, label).await?;
    read_response_limited(response, limit, label).await
}

async fn send(
    client: &reqwest::Client,
    url: Url,
    token: Option<&HeaderValue>,
    label: &str,
) -> Result<Response, String> {
    let mut request = client.get(url);
    if let Some(token) = token {
        request = request.header("private-token", token.clone());
    }
    let response = request
        .send()
        .await
        .map_err(|error| format!("Cannot request {label}: {error}"))?
        .error_for_status()
        .map_err(|error| format!("Cannot request {label}: {error}"))?;
    if response.url().scheme() != "https" {
        return Err(format!("{label} redirected to a non-HTTPS URL"));
    }
    Ok(response)
}

async fn read_response_limited(
    mut response: Response,
    limit: usize,
    label: &str,
) -> Result<Vec<u8>, String> {
    if response
        .content_length()
        .is_some_and(|length| length > limit as u64)
    {
        return Err(format!("{label} exceeds {limit} bytes"));
    }
    let mut bytes = Vec::new();
    while let Some(chunk) = response
        .chunk()
        .await
        .map_err(|error| format!("Cannot read {label}: {error}"))?
    {
        let next_length = bytes
            .len()
            .checked_add(chunk.len())
            .ok_or_else(|| format!("{label} size overflow"))?;
        if next_length > limit {
            return Err(format!("{label} exceeds {limit} bytes"));
        }
        bytes.extend_from_slice(&chunk);
    }
    Ok(bytes)
}

fn cache_parent(cache_root: &Path, server: &Url, project_id: u64) -> PathBuf {
    let server_hash = format!(
        "{:x}",
        Sha256::digest(normalized_server_url(server).as_bytes())
    );
    cache_root
        .join("sources")
        .join("gitlab")
        .join(&server_hash[..16])
        .join(project_id.to_string())
}

fn read_cached_resolution(
    target: &Path,
    server: &Url,
    project_id: u64,
    revision: &str,
) -> Result<Option<SourceCacheRecord>, String> {
    if !target.exists() {
        return Ok(None);
    }
    if !target.is_dir() {
        return Err(format!(
            "GitLab source cache target is not a directory: {}",
            target.display()
        ));
    }
    let bytes = std::fs::read(target.join(CACHE_RECORD_FILE))
        .map_err(|error| format!("Cannot read cached GitLab source metadata: {error}"))?;
    let record: SourceCacheRecord = serde_json::from_slice(&bytes)
        .map_err(|error| format!("Invalid cached GitLab source metadata: {error}"))?;
    if record.schema_version != 1
        || record.server_url != normalized_server_url(server)
        || record.project_id != project_id
        || record.revision != revision
    {
        return Err("Cached GitLab source metadata does not match its cache path".to_string());
    }
    Ok(Some(record))
}

fn cache_archive(
    cache_parent: &Path,
    target: &Path,
    archive: &[u8],
    record: &SourceCacheRecord,
) -> Result<(), String> {
    std::fs::create_dir_all(cache_parent)
        .map_err(|error| format!("Cannot create GitLab source cache: {error}"))?;
    let temporary = tempfile::Builder::new()
        .prefix("resolve-")
        .tempdir_in(cache_parent)
        .map_err(|error| format!("Cannot create GitLab source staging directory: {error}"))?;
    let source = temporary.path().join("source");
    std::fs::create_dir(&source)
        .map_err(|error| format!("Cannot create GitLab source staging directory: {error}"))?;
    extract_archive(archive, &source)?;
    write_cache_record(&source, record)?;
    match std::fs::rename(&source, target) {
        Ok(()) => Ok(()),
        Err(_error) if target.is_dir() => {
            read_cached_resolution(
                target,
                &Url::parse(&record.server_url)
                    .map_err(|parse_error| format!("Invalid cached GitLab URL: {parse_error}"))?,
                record.project_id,
                &record.revision,
            )?;
            Ok(())
        }
        Err(error) => Err(format!("Cannot commit GitLab source cache: {error}")),
    }
}

fn write_cache_record(root: &Path, record: &SourceCacheRecord) -> Result<(), String> {
    let bytes = serde_json::to_vec_pretty(record)
        .map_err(|error| format!("Cannot serialize GitLab source metadata: {error}"))?;
    let path = root.join(CACHE_RECORD_FILE);
    let mut file = std::fs::OpenOptions::new()
        .create_new(true)
        .write(true)
        .open(&path)
        .map_err(|error| format!("Cannot create GitLab source metadata: {error}"))?;
    file.write_all(&bytes)
        .and_then(|_| file.flush())
        .and_then(|_| file.sync_all())
        .map_err(|error| format!("Cannot write GitLab source metadata: {error}"))
}

fn extract_archive(bytes: &[u8], destination: &Path) -> Result<(), String> {
    let decoder = GzDecoder::new(bytes);
    let mut archive = tar::Archive::new(decoder);
    let mut archive_root: Option<OsString> = None;
    let mut extracted_bytes = 0_u64;
    let entries = archive
        .entries()
        .map_err(|error| format!("Invalid GitLab source archive: {error}"))?;
    for (index, entry) in entries.enumerate() {
        if index >= MAX_GITLAB_ARCHIVE_ENTRIES {
            return Err(format!(
                "GitLab source archive contains more than {MAX_GITLAB_ARCHIVE_ENTRIES} entries"
            ));
        }
        let mut entry =
            entry.map_err(|error| format!("Invalid GitLab source archive entry: {error}"))?;
        let path = entry
            .path()
            .map_err(|error| format!("Invalid GitLab source archive path: {error}"))?;
        let relative = safe_archive_path(&path, &mut archive_root)?;
        if relative.as_os_str().is_empty() {
            continue;
        }
        let target = destination.join(&relative);
        let kind = entry.header().entry_type();
        if kind.is_symlink() || kind.is_hard_link() {
            return Err(format!(
                "GitLab source archive links are not allowed: {}",
                path.display()
            ));
        }
        if kind.is_dir() {
            std::fs::create_dir_all(&target)
                .map_err(|error| format!("Cannot create source directory: {error}"))?;
            continue;
        }
        if !kind.is_file() {
            return Err(format!(
                "Unsupported GitLab source archive entry: {}",
                path.display()
            ));
        }
        extracted_bytes = extracted_bytes
            .checked_add(entry.size())
            .ok_or("GitLab source archive extracted size overflow")?;
        if extracted_bytes > MAX_GITLAB_EXTRACTED_BYTES {
            return Err("GitLab source archive expands beyond the source size limit".to_string());
        }
        if let Some(parent) = target.parent() {
            std::fs::create_dir_all(parent)
                .map_err(|error| format!("Cannot create source directory: {error}"))?;
        }
        let mut file = std::fs::OpenOptions::new()
            .create_new(true)
            .write(true)
            .open(&target)
            .map_err(|error| format!("Cannot create source file {}: {error}", target.display()))?;
        std::io::copy(&mut entry, &mut file)
            .map_err(|error| format!("Cannot extract {}: {error}", target.display()))?;
        file.flush()
            .map_err(|error| format!("Cannot flush {}: {error}", target.display()))?;
        #[cfg(unix)]
        {
            use std::os::unix::fs::PermissionsExt;
            let mode = entry.header().mode().unwrap_or(0o644) & 0o777;
            std::fs::set_permissions(&target, std::fs::Permissions::from_mode(mode))
                .map_err(|error| format!("Cannot set source file mode: {error}"))?;
        }
    }
    if archive_root.is_none() {
        return Err("GitLab source archive is empty".to_string());
    }
    Ok(())
}

fn safe_archive_path(path: &Path, archive_root: &mut Option<OsString>) -> Result<PathBuf, String> {
    let mut components = path.components();
    let Some(Component::Normal(root)) = components.next() else {
        return Err(format!(
            "GitLab archive path is not relative: {}",
            path.display()
        ));
    };
    match archive_root {
        Some(expected) if expected != root => {
            return Err("GitLab source archive has multiple top-level roots".to_string())
        }
        None => *archive_root = Some(root.to_os_string()),
        Some(_) => {}
    }
    let mut relative = PathBuf::new();
    for component in components {
        match component {
            Component::Normal(value) => relative.push(value),
            _ => return Err(format!("Unsafe GitLab archive path: {}", path.display())),
        }
    }
    Ok(relative)
}

fn resolution_from_record(
    target: &Path,
    record: SourceCacheRecord,
    cached: bool,
) -> SourceResolution {
    SourceResolution {
        provider: SourceProvider::Gitlab,
        server_url: record.server_url,
        project: record.project,
        requested_reference: record.requested_reference,
        resolved_reference: record.resolved_reference,
        revision: record.revision,
        project_root: target.to_string_lossy().into_owned(),
        archive_sha256: record.archive_sha256,
        cached,
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn project_endpoint_encodes_nested_namespace_as_one_identifier() {
        let server = validate_server_url("https://gitlab.example/subpath").unwrap();
        let url = project_endpoint(&api_root(&server).unwrap(), "group/subgroup/project").unwrap();
        assert_eq!(
            url.as_str(),
            "https://gitlab.example/subpath/api/v4/projects/group%2Fsubgroup%2Fproject"
        );
    }

    #[test]
    fn server_requires_https_and_rejects_embedded_credentials() {
        assert!(validate_server_url("http://gitlab.example").is_err());
        assert!(validate_server_url("https://token@gitlab.example").is_err());
        assert!(validate_server_url("https://gitlab.example?token=secret").is_err());
    }

    #[test]
    fn archive_paths_share_one_root_and_strip_it() {
        let mut root = None;
        assert_eq!(
            safe_archive_path(Path::new("project-deadbeef/src/main.rs"), &mut root).unwrap(),
            Path::new("src/main.rs")
        );
        assert_eq!(
            safe_archive_path(Path::new("project-deadbeef/Cargo.toml"), &mut root).unwrap(),
            Path::new("Cargo.toml")
        );
        assert!(safe_archive_path(Path::new("other/file"), &mut root).is_err());
        assert!(safe_archive_path(Path::new("../outside"), &mut None).is_err());
    }

    #[test]
    fn extracts_regular_gitlab_archive() {
        let mut compressed = Vec::new();
        {
            let encoder =
                flate2::write::GzEncoder::new(&mut compressed, flate2::Compression::default());
            let mut builder = tar::Builder::new(encoder);
            let contents = b"[package]\nname = \"sample\"\n";
            let mut header = tar::Header::new_gnu();
            header.set_size(contents.len() as u64);
            header.set_mode(0o644);
            header.set_cksum();
            builder
                .append_data(&mut header, "sample-deadbeef/Cargo.toml", &contents[..])
                .unwrap();
            builder.finish().unwrap();
        }
        let output = tempfile::tempdir().unwrap();
        extract_archive(&compressed, output.path()).unwrap();
        assert_eq!(
            std::fs::read(output.path().join("Cargo.toml")).unwrap(),
            b"[package]\nname = \"sample\"\n"
        );
    }
}
