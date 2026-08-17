use crate::extensions::lock::sync_directory;
use reqwest::header::{CONTENT_LENGTH, CONTENT_RANGE, ETAG, IF_RANGE, LAST_MODIFIED, RANGE};
use reqwest::{Response, StatusCode, Url};
use serde::{Deserialize, Serialize};
use sha2::{Digest, Sha256};
use std::fs::{File, OpenOptions};
use std::io::{Read, Seek, SeekFrom, Write};
use std::path::{Path, PathBuf};

const DOWNLOAD_JOURNAL_SCHEMA_VERSION: u32 = 1;

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
struct DownloadJournal {
    schema_version: u32,
    url: String,
    integrity: String,
    downloaded_bytes: u64,
    expected_bytes: Option<u64>,
    etag: Option<String>,
    last_modified: Option<String>,
}

struct DownloadFiles {
    directory: PathBuf,
    part: PathBuf,
    journal: PathBuf,
}

struct ResumeState {
    journal: DownloadJournal,
    offset: u64,
}

pub(crate) async fn download_with_resume<F>(
    client: &reqwest::Client,
    cache_root: &Path,
    url: Url,
    integrity: &str,
    limit: usize,
    label: &str,
    verify: F,
) -> Result<Vec<u8>, String>
where
    F: Fn(&[u8], &str) -> Result<(), String>,
{
    let files = download_files(cache_root, &url, integrity);
    std::fs::create_dir_all(&files.directory)
        .map_err(|error| format!("Cannot create {label} download cache: {error}"))?;

    let mut resume = load_resume_state(&files, &url, integrity, limit, label)?;
    let mut restarted = false;
    loop {
        let mut request = client.get(url.clone());
        if resume.offset > 0 {
            request = request.header(RANGE, format!("bytes={}-", resume.offset));
            if let Some(validator) = resume
                .journal
                .etag
                .as_deref()
                .or(resume.journal.last_modified.as_deref())
            {
                request = request.header(IF_RANGE, validator);
            }
        }

        let response = request
            .send()
            .await
            .map_err(|error| format!("Cannot download {label}: {error}"))?;
        ensure_https_response(&response, label)?;

        if response.status() == StatusCode::RANGE_NOT_SATISFIABLE && resume.offset > 0 {
            if unsatisfied_total(&response) == Some(resume.offset) {
                return finish_download(&files, integrity, limit, label, verify);
            }
            if restarted {
                return Err(format!(
                    "Cannot resume {label}: server rejected the byte range"
                ));
            }
            reset_download(&files, &mut resume, &url, integrity, label)?;
            restarted = true;
            continue;
        }

        let response = response
            .error_for_status()
            .map_err(|error| format!("Cannot download {label}: {error}"))?;
        let append = resume.offset > 0 && response.status() == StatusCode::PARTIAL_CONTENT;
        if append {
            let (start, total) = parse_content_range(&response, label)?;
            if start != resume.offset {
                return Err(format!(
                    "Cannot resume {label}: server returned byte {start}, expected {}",
                    resume.offset
                ));
            }
            resume.journal.expected_bytes = total;
        } else if response.status() == StatusCode::PARTIAL_CONTENT {
            let (start, total) = parse_content_range(&response, label)?;
            if start != 0 {
                return Err(format!(
                    "Cannot download {label}: initial response starts at byte {start}"
                ));
            }
            resume.offset = 0;
            resume.journal.expected_bytes = total;
        } else {
            resume.offset = 0;
            resume.journal.downloaded_bytes = 0;
            resume.journal.expected_bytes = response.content_length();
        }

        update_validators(&mut resume.journal, &response);
        validate_response_size(&response, resume.offset, limit, label)?;
        write_journal(&files, &resume.journal, label)?;
        stream_response(response, &files, &mut resume, append, limit, label).await?;
        return finish_download(&files, integrity, limit, label, verify);
    }
}

fn download_files(cache_root: &Path, url: &Url, integrity: &str) -> DownloadFiles {
    let directory = cache_root.join("downloads");
    let mut digest = Sha256::new();
    digest.update(url.as_str().as_bytes());
    digest.update(b"\n");
    digest.update(integrity.as_bytes());
    let key = format!("{:x}", digest.finalize());
    DownloadFiles {
        part: directory.join(format!("{key}.part")),
        journal: directory.join(format!("{key}.journal.json")),
        directory,
    }
}

fn new_journal(url: &Url, integrity: &str) -> DownloadJournal {
    DownloadJournal {
        schema_version: DOWNLOAD_JOURNAL_SCHEMA_VERSION,
        url: url.as_str().to_string(),
        integrity: integrity.to_string(),
        downloaded_bytes: 0,
        expected_bytes: None,
        etag: None,
        last_modified: None,
    }
}

fn load_resume_state(
    files: &DownloadFiles,
    url: &Url,
    integrity: &str,
    limit: usize,
    label: &str,
) -> Result<ResumeState, String> {
    let fresh = || ResumeState {
        journal: new_journal(url, integrity),
        offset: 0,
    };
    if !files.journal.exists() || !files.part.exists() {
        remove_if_exists(&files.journal, label)?;
        remove_if_exists(&files.part, label)?;
        return Ok(fresh());
    }

    let bytes = match std::fs::read(&files.journal) {
        Ok(bytes) => bytes,
        Err(_) => {
            reset_files(files, label)?;
            return Ok(fresh());
        }
    };
    let journal: DownloadJournal = match serde_json::from_slice(&bytes) {
        Ok(journal) => journal,
        Err(_) => {
            reset_files(files, label)?;
            return Ok(fresh());
        }
    };
    let part_length = std::fs::metadata(&files.part)
        .map_err(|error| format!("Cannot inspect partial {label}: {error}"))?
        .len();
    let valid = journal.schema_version == DOWNLOAD_JOURNAL_SCHEMA_VERSION
        && journal.url == url.as_str()
        && journal.integrity == integrity
        && part_length <= limit as u64
        && journal
            .expected_bytes
            .is_none_or(|expected| part_length <= expected && expected <= limit as u64);
    if !valid {
        reset_files(files, label)?;
        return Ok(fresh());
    }

    let mut journal = journal;
    journal.downloaded_bytes = part_length;
    Ok(ResumeState {
        journal,
        offset: part_length,
    })
}

fn reset_download(
    files: &DownloadFiles,
    resume: &mut ResumeState,
    url: &Url,
    integrity: &str,
    label: &str,
) -> Result<(), String> {
    reset_files(files, label)?;
    resume.journal = new_journal(url, integrity);
    resume.offset = 0;
    Ok(())
}

fn reset_files(files: &DownloadFiles, label: &str) -> Result<(), String> {
    remove_if_exists(&files.part, label)?;
    remove_if_exists(&files.journal, label)
}

fn remove_if_exists(path: &Path, label: &str) -> Result<(), String> {
    match std::fs::remove_file(path) {
        Ok(()) => Ok(()),
        Err(error) if error.kind() == std::io::ErrorKind::NotFound => Ok(()),
        Err(error) => Err(format!("Cannot clean partial {label}: {error}")),
    }
}

fn write_journal(
    files: &DownloadFiles,
    journal: &DownloadJournal,
    label: &str,
) -> Result<(), String> {
    let bytes = serde_json::to_vec_pretty(journal)
        .map_err(|error| format!("Cannot serialize {label} download journal: {error}"))?;
    let mut temporary = tempfile::NamedTempFile::new_in(&files.directory)
        .map_err(|error| format!("Cannot create {label} download journal: {error}"))?;
    temporary
        .write_all(&bytes)
        .and_then(|_| temporary.flush())
        .and_then(|_| temporary.as_file().sync_all())
        .map_err(|error| format!("Cannot write {label} download journal: {error}"))?;
    temporary
        .persist(&files.journal)
        .map_err(|error| format!("Cannot persist {label} download journal: {error}"))?;
    sync_directory(&files.directory)
        .map_err(|error| format!("Cannot sync {label} download cache: {error}"))
}

fn update_validators(journal: &mut DownloadJournal, response: &Response) {
    journal.etag = response
        .headers()
        .get(ETAG)
        .and_then(|value| value.to_str().ok())
        .map(str::to_owned);
    journal.last_modified = response
        .headers()
        .get(LAST_MODIFIED)
        .and_then(|value| value.to_str().ok())
        .map(str::to_owned);
}

fn parse_content_range(response: &Response, label: &str) -> Result<(u64, Option<u64>), String> {
    let value = response
        .headers()
        .get(CONTENT_RANGE)
        .and_then(|value| value.to_str().ok())
        .ok_or_else(|| format!("Cannot resume {label}: response has no Content-Range"))?;
    let range = value
        .strip_prefix("bytes ")
        .ok_or_else(|| format!("Cannot resume {label}: invalid Content-Range"))?;
    let (bounds, total) = range
        .split_once('/')
        .ok_or_else(|| format!("Cannot resume {label}: invalid Content-Range"))?;
    let (start, end) = bounds
        .split_once('-')
        .ok_or_else(|| format!("Cannot resume {label}: invalid Content-Range"))?;
    let start = start
        .parse::<u64>()
        .map_err(|_| format!("Cannot resume {label}: invalid Content-Range start"))?;
    let end = end
        .parse::<u64>()
        .map_err(|_| format!("Cannot resume {label}: invalid Content-Range end"))?;
    if end < start {
        return Err(format!(
            "Cannot resume {label}: invalid Content-Range bounds"
        ));
    }
    let total = if total == "*" {
        None
    } else {
        Some(
            total
                .parse::<u64>()
                .map_err(|_| format!("Cannot resume {label}: invalid Content-Range total"))?,
        )
    };
    if total.is_some_and(|total| end >= total) {
        return Err(format!(
            "Cannot resume {label}: invalid Content-Range total"
        ));
    }
    Ok((start, total))
}

fn unsatisfied_total(response: &Response) -> Option<u64> {
    response
        .headers()
        .get(CONTENT_RANGE)?
        .to_str()
        .ok()?
        .strip_prefix("bytes */")?
        .parse()
        .ok()
}

fn validate_response_size(
    response: &Response,
    offset: u64,
    limit: usize,
    label: &str,
) -> Result<(), String> {
    let content_length = response
        .headers()
        .get(CONTENT_LENGTH)
        .and_then(|value| value.to_str().ok())
        .and_then(|value| value.parse::<u64>().ok())
        .or_else(|| response.content_length());
    if content_length
        .and_then(|length| offset.checked_add(length))
        .is_some_and(|length| length > limit as u64)
    {
        return Err(format!("{label} exceeds {limit} bytes"));
    }
    Ok(())
}

async fn stream_response(
    mut response: Response,
    files: &DownloadFiles,
    resume: &mut ResumeState,
    append: bool,
    limit: usize,
    label: &str,
) -> Result<(), String> {
    let mut options = OpenOptions::new();
    options.create(true).write(true);
    if append {
        options.append(true);
    } else {
        options.truncate(true);
    }
    let mut part = options
        .open(&files.part)
        .map_err(|error| format!("Cannot open partial {label}: {error}"))?;
    if !append {
        sync_directory(&files.directory)
            .map_err(|error| format!("Cannot sync {label} download cache: {error}"))?;
    }

    while let Some(chunk) = response
        .chunk()
        .await
        .map_err(|error| format!("Cannot read {label}: {error}"))?
    {
        let next = resume
            .offset
            .checked_add(chunk.len() as u64)
            .ok_or_else(|| format!("{label} size overflow"))?;
        if next > limit as u64 {
            return Err(format!("{label} exceeds {limit} bytes"));
        }
        part.write_all(&chunk)
            .map_err(|error| format!("Cannot write partial {label}: {error}"))?;
        resume.offset = next;
        resume.journal.downloaded_bytes = next;
    }
    part.flush()
        .and_then(|_| part.sync_all())
        .map_err(|error| format!("Cannot persist partial {label}: {error}"))?;
    write_journal(files, &resume.journal, label)?;
    if resume
        .journal
        .expected_bytes
        .is_some_and(|expected| expected != resume.offset)
    {
        return Err(format!(
            "Cannot download {label}: received {} of {} bytes",
            resume.offset,
            resume.journal.expected_bytes.unwrap_or_default()
        ));
    }
    Ok(())
}

fn finish_download<F>(
    files: &DownloadFiles,
    integrity: &str,
    limit: usize,
    label: &str,
    verify: F,
) -> Result<Vec<u8>, String>
where
    F: Fn(&[u8], &str) -> Result<(), String>,
{
    let mut file = File::open(&files.part)
        .map_err(|error| format!("Cannot open downloaded {label}: {error}"))?;
    let length = file
        .metadata()
        .map_err(|error| format!("Cannot inspect downloaded {label}: {error}"))?
        .len();
    if length > limit as u64 {
        reset_files(files, label)?;
        return Err(format!("{label} exceeds {limit} bytes"));
    }
    file.seek(SeekFrom::Start(0))
        .map_err(|error| format!("Cannot read downloaded {label}: {error}"))?;
    let mut bytes = Vec::with_capacity(length as usize);
    file.read_to_end(&mut bytes)
        .map_err(|error| format!("Cannot read downloaded {label}: {error}"))?;
    if let Err(error) = verify(&bytes, integrity) {
        reset_files(files, label)?;
        return Err(error);
    }
    reset_files(files, label)?;
    sync_directory(&files.directory)
        .map_err(|error| format!("Cannot sync {label} download cache: {error}"))?;
    Ok(bytes)
}

fn ensure_https_response(response: &Response, label: &str) -> Result<(), String> {
    if response.url().scheme() == "https" {
        Ok(())
    } else {
        Err(format!("{label} redirected to a non-HTTPS URL"))
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn resume_uses_actual_part_length_after_interrupted_journal_update() {
        let directory = tempfile::tempdir().unwrap();
        let url = Url::parse("https://registry.example/package.tgz").unwrap();
        let files = download_files(directory.path(), &url, "sha512-example");
        std::fs::create_dir_all(&files.directory).unwrap();
        std::fs::write(&files.part, b"partial data").unwrap();
        let mut journal = new_journal(&url, "sha512-example");
        journal.downloaded_bytes = 3;
        write_journal(&files, &journal, "test tarball").unwrap();

        let resume =
            load_resume_state(&files, &url, "sha512-example", 1024, "test tarball").unwrap();
        assert_eq!(resume.offset, 12);
        assert_eq!(resume.journal.downloaded_bytes, 12);
    }

    #[test]
    fn mismatched_journal_is_discarded() {
        let directory = tempfile::tempdir().unwrap();
        let url = Url::parse("https://registry.example/package.tgz").unwrap();
        let files = download_files(directory.path(), &url, "sha512-current");
        std::fs::create_dir_all(&files.directory).unwrap();
        std::fs::write(&files.part, b"stale").unwrap();
        write_journal(
            &files,
            &new_journal(&url, "sha512-previous"),
            "test tarball",
        )
        .unwrap();

        let resume =
            load_resume_state(&files, &url, "sha512-current", 1024, "test tarball").unwrap();
        assert_eq!(resume.offset, 0);
        assert!(!files.part.exists());
        assert!(!files.journal.exists());
    }
}
