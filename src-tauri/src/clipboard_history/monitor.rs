//! The background clipboard monitor.
//!
//! A tokio task polls `arboard` for text and images on a fixed interval for
//! as long as floter runs and the setting is on. Every poll hashes what it
//! sees so unchanged clipboards and consecutive copies of the same content
//! cost nothing, and every clipboard access failure is logged and survived —
//! another application holding the clipboard is a normal Tuesday, not an
//! error worth crashing over.

use super::store;
use super::ClipboardEntry;
use std::io::Cursor;
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::Arc;
use std::time::{Duration, SystemTime, UNIX_EPOCH};

use tauri::{AppHandle, Manager};

/// How often the system clipboard is sampled.
pub const POLL_INTERVAL_MS: u64 = 900;
/// Text above this size is not captured (512 KB).
pub const MAX_TEXT_BYTES: usize = 512 * 1024;
/// Images whose PNG encoding exceeds this are not captured (~20 MB).
pub const MAX_IMAGE_PNG_BYTES: usize = 20 * 1024 * 1024;

fn now_ms() -> i64 {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap_or_default()
        .as_millis() as i64
}

/// One sampling pass. Returns the hash of whatever was last seen so the
/// caller can skip identical follow-up polls; `None` means the clipboard was
/// unreadable or held nothing capturable.
///
/// Runs on a blocking thread: arboard talks to the window server, which can
/// stall, and a stalled clipboard must never hold up the async runtime.
pub fn poll_once(app: &AppHandle, last_hash: Option<String>) -> Option<String> {
    let mut clipboard = match arboard::Clipboard::new() {
        Ok(clipboard) => clipboard,
        Err(error) => {
            eprintln!("floter: clipboard unavailable: {error}");
            return None;
        }
    };

    // Text first; only when there is no readable text do we look at images,
    // which keeps the common text case at one syscall-ish round trip.
    if let Ok(text) = clipboard.get_text() {
        if !text.trim().is_empty() {
            if text.len() > MAX_TEXT_BYTES {
                // Too big to keep. Not recorded in `last_hash` either, so the
                // user paying the cost of a smaller copy still gets captured.
                return None;
            }
            let hash = store::content_hash(text.as_bytes());
            if last_hash.as_deref() == Some(hash.as_str())
                || capture_text(app, text, &hash).is_err()
            {
                // Either already the newest entry (duplicate copy) or storage
                // failed; either way do not re-record the same content.
                return Some(hash);
            }
            return Some(hash);
        }
    }

    let image = match clipboard.get_image() {
        Ok(image) => image,
        Err(_) => return None,
    };
    let width = image.width as u32;
    let height = image.height as u32;
    if width == 0 || height == 0 {
        return None;
    }
    let rgba = image.bytes.into_owned();
    let png = match encode_png(width, height, &rgba) {
        Ok(png) => png,
        Err(error) => {
            eprintln!("floter: clipboard image encoding failed: {error}");
            return None;
        }
    };
    if png.len() > MAX_IMAGE_PNG_BYTES {
        eprintln!(
            "floter: clipboard image too large ({} bytes), skipped",
            png.len()
        );
        return None;
    }
    // Dimensions go into the hash: identical pixels at different sizes are
    // different content, and RGBA bytes alone would collide across resizes
    // far less often than a plain byte hash suggests they should be kept.
    let mut hashed = Vec::with_capacity(rgba.len() + 8);
    hashed.extend_from_slice(&width.to_be_bytes());
    hashed.extend_from_slice(&height.to_be_bytes());
    hashed.extend_from_slice(&rgba);
    let hash = store::content_hash(&hashed);
    let _ = capture_image(app, &png, width, height, &hash);
    Some(hash)
}

fn capture_text(app: &AppHandle, text: String, hash: &str) -> Result<(), String> {
    super::mutate_history(app, |entries| {
        if store::is_duplicate_of_newest(hash, entries) {
            return Ok(());
        }
        entries.insert(
            0,
            ClipboardEntry {
                id: uuid::Uuid::new_v4().to_string(),
                kind: "text".to_string(),
                text: Some(text),
                image_file: None,
                width: None,
                height: None,
                hash: hash.to_string(),
                created_at: now_ms(),
                favorite: false,
            },
        );
        prune_and_save(entries)
    })
}

fn capture_image(
    app: &AppHandle,
    png: &[u8],
    width: u32,
    height: u32,
    hash: &str,
) -> Result<(), String> {
    let paths = store::app_store_paths().ok_or("No app data directory")?;
    super::mutate_history(app, |entries| {
        if store::is_duplicate_of_newest(hash, entries) {
            return Ok(());
        }
        let id = uuid::Uuid::new_v4().to_string();
        store::write_image(&paths, &id, png)?;
        entries.insert(
            0,
            ClipboardEntry {
                id: id.clone(),
                kind: "image".to_string(),
                text: None,
                image_file: Some(format!("{id}.png")),
                width: Some(width),
                height: Some(height),
                hash: hash.to_string(),
                created_at: now_ms(),
                favorite: false,
            },
        );
        if let Err(error) = prune_and_save(entries) {
            // Do not leave the freshly written PNG behind without an index row.
            store::delete_image(&paths, &format!("{id}.png"));
            return Err(error);
        }
        Ok(())
    })
}

/// Retention pass shared by every write path: prune, persist, and clean up
/// the image files of whatever was dropped or orphaned.
pub fn prune_and_save(entries: &mut Vec<ClipboardEntry>) -> Result<(), String> {
    let paths = store::app_store_paths().ok_or("No app data directory")?;
    let taken = std::mem::take(entries);
    let (kept, dropped) = store::prune_entries(taken, now_ms());
    *entries = kept;
    store::save_index(&paths, entries)?;
    for entry in &dropped {
        if let Some(file) = &entry.image_file {
            store::delete_image(&paths, file);
        }
    }
    store::remove_orphan_images(&paths, entries);
    Ok(())
}

/// Encode raw RGBA8 pixels as PNG.
pub fn encode_png(width: u32, height: u32, rgba: &[u8]) -> Result<Vec<u8>, String> {
    let expected = width as usize * height as usize * 4;
    if rgba.len() < expected {
        return Err(format!(
            "image buffer too small: {} bytes for {width}x{height}",
            rgba.len()
        ));
    }
    let mut output = Vec::new();
    {
        let mut encoder = png::Encoder::new(Cursor::new(&mut output), width, height);
        encoder.set_color(png::ColorType::Rgba);
        encoder.set_depth(png::BitDepth::Eight);
        let mut writer = encoder.write_header().map_err(|error| error.to_string())?;
        writer
            .write_image_data(&rgba[..expected])
            .map_err(|error| error.to_string())?;
    }
    Ok(output)
}

/// Decode a stored PNG back into `(width, height, RGBA8)` for restoring to
/// the system clipboard.
pub fn decode_png(bytes: &[u8]) -> Result<(u32, u32, Vec<u8>), String> {
    let decoder = png::Decoder::new(Cursor::new(bytes));
    let mut reader = decoder.read_info().map_err(|error| error.to_string())?;
    let mut buffer = vec![0u8; reader.output_buffer_size()];
    let info = reader
        .next_frame(&mut buffer)
        .map_err(|error| error.to_string())?;
    buffer.truncate(info.buffer_size());
    // Everything we encode is RGBA8, so anything else in a stored file means
    // it did not come from us; refuse rather than hand garbled pixels to the
    // clipboard.
    if info.color_type != png::ColorType::Rgba || info.bit_depth != png::BitDepth::Eight {
        return Err("Unsupported image format in history".to_string());
    }
    Ok((info.width, info.height, buffer))
}

/// Handle to one running monitor, cancelled and aborted on disable/quit.
pub struct MonitorHandle {
    cancel: Arc<AtomicBool>,
    handle: tauri::async_runtime::JoinHandle<()>,
}

/// Start polling, unless a monitor is already running. Idempotent by design:
/// settings syncs may ask for a start while one is live.
pub fn start(app: &AppHandle) {
    let state = app.state::<super::ClipboardState>();
    let mut slot = match state.monitor.lock() {
        Ok(slot) => slot,
        Err(_) => return,
    };
    if slot.is_some() {
        return;
    }
    let cancel = Arc::new(AtomicBool::new(false));
    let task_cancel = cancel.clone();
    let app_handle = app.clone();
    let handle = tauri::async_runtime::spawn(async move {
        let mut ticker = tokio::time::interval(Duration::from_millis(POLL_INTERVAL_MS));
        ticker.set_missed_tick_behavior(tokio::time::MissedTickBehavior::Delay);
        // The first tick fires immediately; skip it so floter does not read
        // the clipboard during startup before the window server is settled.
        ticker.tick().await;
        let mut last_hash: Option<String> = None;
        loop {
            ticker.tick().await;
            if task_cancel.load(Ordering::Relaxed) {
                return;
            }
            let poll_app = app_handle.clone();
            let poll_last = last_hash.take();
            let polled =
                tauri::async_runtime::spawn_blocking(move || poll_once(&poll_app, poll_last)).await;
            match polled {
                Ok(next) => last_hash = next,
                Err(error) => eprintln!("floter: clipboard poll panicked: {error}"),
            }
        }
    });
    *slot = Some(MonitorHandle { cancel, handle });
}

/// Stop a running monitor, if any. Also idempotent.
pub fn stop(app: &AppHandle) {
    let state = app.state::<super::ClipboardState>();
    let Ok(mut slot) = state.monitor.lock() else {
        return;
    };
    if let Some(monitor) = slot.take() {
        monitor.cancel.store(true, Ordering::Relaxed);
        monitor.handle.abort();
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn png_round_trips_pixels() {
        let width = 3u32;
        let height = 2u32;
        let rgba: Vec<u8> = (0..width * height * 4)
            .map(|byte| (byte % 251) as u8)
            .collect();

        let encoded = encode_png(width, height, &rgba).expect("encode");

        assert!(encoded.len() > 8);
        let (decoded_width, decoded_height, decoded) = decode_png(&encoded).expect("decode");
        assert_eq!((decoded_width, decoded_height), (width, height));
        assert_eq!(decoded, rgba);
    }

    #[test]
    fn decode_rejects_non_rgba_frames() {
        let mut output = Vec::new();
        {
            let mut encoder = png::Encoder::new(Cursor::new(&mut output), 1, 1);
            encoder.set_color(png::ColorType::Grayscale);
            encoder.set_depth(png::BitDepth::Eight);
            let mut writer = encoder.write_header().expect("header");
            writer.write_image_data(&[0u8]).expect("data");
        }
        assert!(decode_png(&output).is_err());
    }

    #[test]
    fn short_buffers_are_refused_before_encoding() {
        assert!(encode_png(4, 4, &[0u8; 4]).is_err());
    }
}
