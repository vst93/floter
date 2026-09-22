//! Live browser tabs — the third and most perishable of the plugin's sources.
//!
//! Bookmarks and history live in files, so reading them is a file read. The
//! tabs a browser has open right now live in the browser process, and there is
//! no file to open. Two mechanisms cover the platforms, and neither is required
//! for the plugin to be useful:
//!
//! * **macOS** — AppleScript. Every Chromium-family browser on the Mac answers
//!   `tell application "X"` with its window and tab structure, so one script
//!   reads all four browsers and needs no configuration at all. The script
//!   emits one tab per line, tab-separated, because a line-oriented payload is
//!   deterministic to parse while AppleScript's native nested-list rendering
//!   (`{{"a", "b"}, …}`) is a quoting puzzle with no schema.
//! * **Windows / Linux** — the Chrome DevTools Protocol. A browser started with
//!   `--remote-debugging-port=9222` answers `GET /json/list` with its page
//!   targets. This only works when the user has opted in (the plugin setting)
//!   *and* started the browser with the flag, so an unreachable endpoint is an
//!   expected state, not an error: it is reported as a one-line hint and never
//!   blocks bookmarks or history.
//!
//! Every fetch is soft-failing and time-boxed. A browser that is not running, a
//! missing app, a hung `osascript`, a dead port: each returns an error *string*
//! the caller turns into an empty group plus a note. Nothing here panics, and
//! nothing here waits forever.

use serde::Serialize;
use std::time::Duration;

/// How long a single fetch may take before it is killed. AppleScript against a
/// busy browser can be slow, but three seconds is already far past the point
/// where the user has stopped expecting a result.
const FETCH_TIMEOUT: Duration = Duration::from_secs(3);

/// The port Chromium's debug endpoint listens on unless the user picked another.
pub const DEFAULT_CDP_PORT: u16 = 9222;

/// One open tab.
///
/// `window_index` and `tab_index` are 1-based on macOS (AppleScript's own
/// numbering) and 0-based on the CDP path, where a window is not part of the
/// protocol at all and `window_index` is always `0`. `active` is only known on
/// macOS — the DevTools Protocol does not expose which page target is focused —
/// so the CDP path reports every tab as inactive rather than guessing.
#[derive(Debug, Clone, PartialEq, Eq, Serialize)]
pub struct BrowserTab {
    pub browser_id: String,
    pub window_index: usize,
    pub tab_index: usize,
    pub title: String,
    pub url: String,
    pub active: bool,
}

/// Map a browser id to its macOS application bundle name.
///
/// The same table `browser_open_url` uses. Kept here as well so the tab reader
/// does not reach into the opener's internals; the two are deliberately
/// identical, and both are covered by the platform table's own test.
#[cfg_attr(not(target_os = "macos"), allow(dead_code))]
pub fn mac_app_name(browser_id: &str) -> Option<&'static str> {
    match browser_id {
        "chrome" => Some("Google Chrome"),
        "edge" => Some("Microsoft Edge"),
        "brave" => Some("Brave Browser"),
        "chromium" => Some("Chromium"),
        _ => None,
    }
}

/// The AppleScript that lists every tab of every window.
///
/// One line per tab: `window \t tab \t active \t url \t title`. `active` is `1`
/// for the window's active tab. Titles may themselves contain a tab (they come
/// from page markup), so the reader splits on the first four separators only —
/// the title keeps the rest of the line verbatim.
#[cfg_attr(not(target_os = "macos"), allow(dead_code))]
pub fn applescript_list_tabs(app_name: &str) -> String {
    let sep = "(ASCII character 9)";
    let eol = "(ASCII character 10)";
    format!(
        concat!(
            "set sep to {sep}\n",
            "set eol to {eol}\n",
            "set out to \"\"\n",
            "tell application \"{app}\"\n",
            "  set wi to 0\n",
            "  repeat with w in windows\n",
            "    set wi to wi + 1\n",
            "    set at to active tab index of w\n",
            "    set ti to 0\n",
            "    repeat with t in tabs of w\n",
            "      set ti to ti + 1\n",
            "      set flag to \"0\"\n",
            "      if ti is at then set flag to \"1\"\n",
            "      set out to out & wi & sep & ti & sep & flag & sep & (URL of t) & sep & (title of t) & eol\n",
            "    end repeat\n",
            "  end repeat\n",
            "end tell\n",
            "return out\n",
        ),
        sep = sep,
        eol = eol,
        app = app_name,
    )
}

/// The AppleScript that focuses one tab.
#[cfg_attr(not(target_os = "macos"), allow(dead_code))]
pub fn applescript_activate_tab(app_name: &str, window_index: usize, tab_index: usize) -> String {
    format!(
        "tell application \"{app}\"\n  set active tab index of window {window} to {tab}\n  activate\nend tell\n",
        app = app_name,
        window = window_index,
        tab = tab_index,
    )
}

/// Parse the [`applescript_list_tabs`] payload.
///
/// Pure, so the fixture in this module's tests is the whole contract: the
/// platform script only has to keep emitting this shape. A malformed line (a
/// missing field, a non-numeric index) is skipped rather than failing the
/// whole read — one odd row must not hide the other thirty.
#[cfg_attr(not(target_os = "macos"), allow(dead_code))]
pub fn parse_applescript_tabs(browser_id: &str, output: &str) -> Vec<BrowserTab> {
    let mut tabs = Vec::new();
    for line in output.lines() {
        let line = line.trim_end_matches('\r');
        if line.trim().is_empty() {
            continue;
        }
        let mut parts = line.splitn(5, '\t');
        let (Some(window), Some(tab), Some(flag), Some(url), Some(title)) = (
            parts.next(),
            parts.next(),
            parts.next(),
            parts.next(),
            parts.next(),
        ) else {
            continue;
        };
        let (Ok(window_index), Ok(tab_index)) =
            (window.trim().parse::<usize>(), tab.trim().parse::<usize>())
        else {
            continue;
        };
        let url = url.trim().to_string();
        let title = title.trim().to_string();
        if url.is_empty() && title.is_empty() {
            continue;
        }
        tabs.push(BrowserTab {
            browser_id: browser_id.to_string(),
            window_index,
            tab_index,
            title: if title.is_empty() { url.clone() } else { title },
            url,
            active: flag.trim() == "1",
        });
    }
    tabs
}

/// One page target from `/json/list`, reduced to the fields the tab list uses.
struct CdpPage {
    id: String,
    title: String,
    url: String,
}

/// Every `type == "page"` target, in the order the browser reported them.
///
/// A DevTools payload that is not a JSON array, or that carries a malformed
/// entry, yields an empty or shortened list rather than an error: the endpoint
/// is an optional convenience and a version difference must not look like a
/// crash.
fn cdp_pages(json: &str) -> Vec<CdpPage> {
    let Ok(value) = serde_json::from_str::<serde_json::Value>(json) else {
        return Vec::new();
    };
    let Some(entries) = value.as_array() else {
        return Vec::new();
    };
    entries
        .iter()
        .filter(|entry| entry.get("type").and_then(|kind| kind.as_str()) == Some("page"))
        .map(|entry| {
            let text = |key: &str| {
                entry
                    .get(key)
                    .and_then(|value| value.as_str())
                    .unwrap_or_default()
                    .trim()
                    .to_string()
            };
            CdpPage {
                id: text("id"),
                title: text("title"),
                url: text("url"),
            }
        })
        .filter(|page| !page.url.is_empty() || !page.title.is_empty())
        .collect()
}

/// Parse a `/json/list` payload into tabs.
pub fn parse_cdp_targets(browser_id: &str, json: &str) -> Vec<BrowserTab> {
    cdp_pages(json)
        .into_iter()
        .enumerate()
        .map(|(index, page)| BrowserTab {
            browser_id: browser_id.to_string(),
            // The protocol has no window concept; one flat list is the truth
            // the endpoint gives us, so every row reports window 0.
            window_index: 0,
            tab_index: index,
            title: if page.title.is_empty() {
                page.url.clone()
            } else {
                page.title
            },
            url: page.url,
            // `/json/list` does not say which target is focused.
            active: false,
        })
        .collect()
}

/// The target ids of a `/json/list` payload, in the same order as
/// [`parse_cdp_targets`], so an index into one is an index into the other.
pub fn parse_cdp_page_ids(json: &str) -> Vec<String> {
    cdp_pages(json).into_iter().map(|page| page.id).collect()
}

/// One minimal HTTP/1.0 round trip to the local debug endpoint.
///
/// Written by hand rather than through the HTTP client the updater uses: this
/// talks to `127.0.0.1` only, wants no redirects, no TLS and no connection
/// pool, and a request whose entire surface is four headers is clearer as four
/// headers. The socket has a connect and a read timeout, so a port that accepts
/// but never answers cannot hang the fetch.
fn http_request(port: u16, method: &str, path: &str) -> Result<String, String> {
    use std::io::{Read, Write};
    use std::net::{Ipv4Addr, SocketAddr, TcpStream};

    let address = SocketAddr::from((Ipv4Addr::LOCALHOST, port));
    let mut stream = TcpStream::connect_timeout(&address, FETCH_TIMEOUT)
        .map_err(|error| format!("debug port {port} is not reachable: {error}"))?;
    stream
        .set_read_timeout(Some(FETCH_TIMEOUT))
        .map_err(|error| error.to_string())?;
    stream
        .set_write_timeout(Some(FETCH_TIMEOUT))
        .map_err(|error| error.to_string())?;
    let request = format!(
        "{method} {path} HTTP/1.0\r\nHost: 127.0.0.1:{port}\r\nConnection: close\r\nContent-Length: 0\r\n\r\n"
    );
    stream
        .write_all(request.as_bytes())
        .map_err(|error| format!("could not write to debug port {port}: {error}"))?;
    let mut response = Vec::new();
    stream
        .read_to_end(&mut response)
        .map_err(|error| format!("debug port {port} did not answer: {error}"))?;
    let text = String::from_utf8_lossy(&response).into_owned();
    let (headers, body) = text.split_once("\r\n\r\n").unwrap_or((text.as_str(), ""));
    let status = headers.lines().next().unwrap_or_default();
    if !status.contains(" 200") {
        return Err(format!("debug port {port} answered {status}"));
    }
    Ok(body.to_string())
}

/// Read the tabs the browser is showing, through the DevTools Protocol.
///
/// Only reached on Windows and Linux, and only when the user turned the setting
/// on; a failure here is the expected state on a browser started without the
/// flag.
pub fn fetch_tabs_cdp(browser_id: &str, port: u16) -> Result<Vec<BrowserTab>, String> {
    let body = http_request(port, "GET", "/json/list")?;
    let tabs = parse_cdp_targets(browser_id, &body);
    if tabs.is_empty() {
        return Err(format!(
            "debug port {port} answered but reported no page targets"
        ));
    }
    Ok(tabs)
}

/// The DevTools path for this platform, gated on the user's opt-in.
#[cfg(not(target_os = "macos"))]
fn list_tabs_cdp_for(browser_id: &str) -> Result<Vec<BrowserTab>, String> {
    let settings = crate::commands::config::load_settings().browser_plugin;
    if !settings.cdp_enabled {
        return Err(
            "Tab capture is off. Turn on the debug port in this plugin's settings, then start the browser with --remote-debugging-port."
                .to_string(),
        );
    }
    fetch_tabs_cdp(browser_id, settings.cdp_port)
}

/// Focus a tab through the DevTools Protocol, by its position in `/json/list`.
#[cfg(not(target_os = "macos"))]
fn activate_cdp_tab(tab_index: usize) -> Result<(), String> {
    let settings = crate::commands::config::load_settings().browser_plugin;
    if !settings.cdp_enabled {
        return Err("Tab capture is off".to_string());
    }
    let body = http_request(settings.cdp_port, "GET", "/json/list")?;
    let ids = parse_cdp_page_ids(&body);
    let id = ids
        .get(tab_index)
        .ok_or_else(|| format!("no tab at index {tab_index}"))?;
    http_request(settings.cdp_port, "POST", &format!("/json/activate/{id}")).map(|_| ())
}

/// Run a program with a hard deadline and capture its output.
///
/// `Command::output()` has no timeout, and a browser that stops answering
/// AppleScript would leave the launcher waiting on it. stdout and stderr go to
/// temporary files rather than pipes so a chatty script cannot fill a pipe
/// buffer and deadlock before the deadline is even reached.
#[cfg(target_os = "macos")]
fn run_with_timeout(program: &str, args: &[&str], timeout: Duration) -> Result<String, String> {
    use std::io::{Read, Seek, SeekFrom};

    let stdout_file = tempfile::tempfile().map_err(|error| error.to_string())?;
    let stderr_file = tempfile::tempfile().map_err(|error| error.to_string())?;
    let stdout_clone = stdout_file.try_clone().map_err(|error| error.to_string())?;
    let stderr_clone = stderr_file.try_clone().map_err(|error| error.to_string())?;
    let mut child = std::process::Command::new(program)
        .args(args)
        .stdin(std::process::Stdio::null())
        .stdout(std::process::Stdio::from(stdout_clone))
        .stderr(std::process::Stdio::from(stderr_clone))
        .spawn()
        .map_err(|error| format!("could not run {program}: {error}"))?;

    let deadline = std::time::Instant::now() + timeout;
    let status = loop {
        match child.try_wait() {
            Ok(Some(status)) => break status,
            Ok(None) => {
                if std::time::Instant::now() >= deadline {
                    let _ = child.kill();
                    let _ = child.wait();
                    return Err(format!(
                        "{program} did not answer within {}s",
                        timeout.as_secs()
                    ));
                }
                std::thread::sleep(Duration::from_millis(25));
            }
            Err(error) => return Err(format!("could not wait for {program}: {error}")),
        }
    };

    let mut stdout = stdout_file;
    stdout
        .seek(SeekFrom::Start(0))
        .map_err(|error| error.to_string())?;
    let mut text = String::new();
    stdout
        .read_to_string(&mut text)
        .map_err(|error| error.to_string())?;
    if status.success() {
        return Ok(text);
    }
    let mut stderr = stderr_file;
    stderr
        .seek(SeekFrom::Start(0))
        .map_err(|error| error.to_string())?;
    let mut message = String::new();
    let _ = stderr.read_to_string(&mut message);
    let message = message.trim();
    Err(if message.is_empty() {
        format!("{program} exited with {status}")
    } else {
        message.to_string()
    })
}

/// The AppleScript read, time-boxed and soft-failing.
#[cfg(target_os = "macos")]
fn fetch_tabs_macos(browser_id: &str) -> Result<Vec<BrowserTab>, String> {
    let app = mac_app_name(browser_id)
        .ok_or_else(|| format!("no AppleScript reader for browser {browser_id}"))?;
    let source = applescript_list_tabs(app);
    let output = run_with_timeout("osascript", &["-e", &source], FETCH_TIMEOUT)?;
    let tabs = parse_applescript_tabs(browser_id, &output);
    if tabs.is_empty() {
        return Err(format!("{app} has no open windows"));
    }
    Ok(tabs)
}

/// Turn the command's argument into a browser id.
///
/// The caller may pass a `profile_key` (`"chrome/Default"`), a bare browser id
/// (`"chrome"`), or `"auto"`/`"default"`/nothing — the same placeholder the
/// launcher's `profileKey` uses for the system browser. Anything that does not
/// resolve to a browser the platform table knows is left as-is; the fetch then
/// reports it as unsupported rather than guessing.
fn resolve_browser_id(value: &str) -> String {
    let value = value.trim();
    if value.is_empty() || value == "auto" || value == "default" {
        return super::browser_default_profile()
            .and_then(|key| super::discover::parse_profile_key(&key).map(|(id, _)| id))
            .unwrap_or_default();
    }
    match super::discover::parse_profile_key(value) {
        Some((browser_id, _)) => browser_id,
        None => value.to_string(),
    }
}

/// Every tab the browser has open, newest position first, capped at `limit`.
///
/// Errors are the soft kind: the caller renders an empty group and the error's
/// one line. A tab read never blocks the bookmark or history read that sits
/// beside it.
#[tauri::command]
pub fn browser_list_tabs(
    profile_key_or_browser: String,
    limit: Option<usize>,
) -> Result<Vec<BrowserTab>, String> {
    let browser_id = resolve_browser_id(&profile_key_or_browser);
    if browser_id.is_empty() {
        return Err("No browser profile was found".to_string());
    }
    #[cfg(target_os = "macos")]
    let mut tabs = fetch_tabs_macos(&browser_id)?;
    #[cfg(not(target_os = "macos"))]
    let mut tabs = list_tabs_cdp_for(&browser_id)?;
    tabs.truncate(super::normalize_limit(limit));
    Ok(tabs)
}

/// Bring one tab to the front.
///
/// macOS sets the active tab index through AppleScript; Windows and Linux ask
/// the DevTools endpoint to activate the target at that position. When neither
/// path is available — the CDP port is closed, the browser is not running — the
/// call degrades to opening the tab's URL, so pressing Enter on a tab row
/// always does *something* the user asked for.
#[tauri::command]
pub fn browser_activate_tab(
    browser_id: String,
    window_index: usize,
    tab_index: usize,
    url: String,
) -> Result<(), String> {
    let browser_id = resolve_browser_id(&browser_id);
    let url = url.trim().to_string();

    #[cfg(target_os = "macos")]
    {
        if let Some(app) = mac_app_name(&browser_id) {
            let source = applescript_activate_tab(app, window_index, tab_index);
            if run_with_timeout("osascript", &["-e", &source], FETCH_TIMEOUT).is_ok() {
                return Ok(());
            }
        }
    }

    #[cfg(not(target_os = "macos"))]
    {
        // The DevTools endpoint has no window concept; only the tab's position
        // in the flat target list is meaningful there.
        let _ = window_index;
        if activate_cdp_tab(tab_index).is_ok() {
            return Ok(());
        }
    }

    if url.is_empty() {
        return Err("Could not focus the tab, and it has no URL to open".to_string());
    }
    super::open_url_with(&browser_id, &url)
}

#[cfg(test)]
mod tests {
    use super::*;

    /// The exact shape [`applescript_list_tabs`] emits, from a browser with two
    /// windows: window 1 has two tabs (the second active), window 2 has one.
    const APPLESCRIPT_FIXTURE: &str = "1\t1\t0\thttps://example.com/\tExample Domain\n1\t2\t1\thttps://rust-lang.org/\tRust Programming Language\n2\t1\t1\thttps://news.ycombinator.com/\tHacker News\n";

    #[test]
    fn applescript_output_parses_into_windows_and_tabs() {
        let tabs = parse_applescript_tabs("chrome", APPLESCRIPT_FIXTURE);
        assert_eq!(tabs.len(), 3);
        assert_eq!(
            tabs[0],
            BrowserTab {
                browser_id: "chrome".into(),
                window_index: 1,
                tab_index: 1,
                title: "Example Domain".into(),
                url: "https://example.com/".into(),
                active: false,
            }
        );
        assert!(
            tabs[1].active,
            "the second tab of window 1 is the active one"
        );
        assert_eq!(tabs[1].window_index, 1);
        assert_eq!(tabs[2].window_index, 2);
        assert_eq!(tabs[2].url, "https://news.ycombinator.com/");
    }

    #[test]
    fn a_title_containing_a_tab_keeps_the_rest_of_the_line() {
        // Page titles come from markup and may hold a literal tab; only the
        // first four separators are structural.
        let line = "1\t1\t1\thttps://example.com/\tweird\ttitle\n";
        let tabs = parse_applescript_tabs("brave", line);
        assert_eq!(tabs.len(), 1);
        assert_eq!(tabs[0].title, "weird\ttitle");
        assert_eq!(tabs[0].url, "https://example.com/");
    }

    #[test]
    fn a_malformed_applescript_line_is_skipped_not_fatal() {
        let payload =
            "1\t1\t1\thttps://ok.example/\tFine\nnot-a-row\n1\tx\t0\thttps://bad.example/\tBad\n\n";
        let tabs = parse_applescript_tabs("edge", payload);
        assert_eq!(tabs.len(), 1);
        assert_eq!(tabs[0].title, "Fine");
    }

    #[test]
    fn an_untitled_applescript_tab_falls_back_to_its_url() {
        let tabs = parse_applescript_tabs("chromium", "1\t1\t1\thttps://example.com/\t\n");
        assert_eq!(tabs.len(), 1);
        assert_eq!(tabs[0].title, "https://example.com/");
    }

    #[test]
    fn an_empty_applescript_payload_is_an_empty_list() {
        assert!(parse_applescript_tabs("chrome", "").is_empty());
        assert!(parse_applescript_tabs("chrome", "\n\n").is_empty());
    }

    #[test]
    fn the_applescript_source_names_the_app_and_the_separators() {
        let source = applescript_list_tabs("Google Chrome");
        assert!(source.contains("tell application \"Google Chrome\""));
        assert!(source.contains("active tab index of w"));
        assert!(source.contains("(ASCII character 9)"));
    }

    #[test]
    fn the_activation_source_targets_one_window_and_tab() {
        let source = applescript_activate_tab("Microsoft Edge", 2, 3);
        assert!(source.contains("set active tab index of window 2 to 3"));
        assert!(source.contains("activate"));
    }

    /// A real `/json/list` body, trimmed to the fields that matter, with one
    /// non-page target that must not become a row.
    const CDP_FIXTURE: &str = r#"[
        {"id":"A1","type":"page","title":"Example Domain","url":"https://example.com/","webSocketDebuggerUrl":"ws://x"},
        {"id":"A2","type":"background_page","title":"Service Worker","url":"https://example.com/sw.js"},
        {"id":"A3","type":"page","title":"Rust","url":"https://www.rust-lang.org/"},
        {"id":"A4","type":"page","title":"","url":"https://untitled.example/"}
    ]"#;

    #[test]
    fn cdp_targets_keep_only_pages_and_are_indexed_in_order() {
        let tabs = parse_cdp_targets("chrome", CDP_FIXTURE);
        assert_eq!(tabs.len(), 3, "the background page is not a tab");
        assert_eq!(tabs[0].tab_index, 0);
        assert_eq!(tabs[0].window_index, 0);
        assert_eq!(tabs[0].title, "Example Domain");
        assert_eq!(tabs[1].tab_index, 1);
        assert_eq!(tabs[1].url, "https://www.rust-lang.org/");
        // CDP does not report focus, so nothing claims to be active.
        assert!(tabs.iter().all(|tab| !tab.active));
        // An empty title falls back to the URL.
        assert_eq!(tabs[2].title, "https://untitled.example/");
    }

    #[test]
    fn cdp_page_ids_line_up_with_the_parsed_targets() {
        let ids = parse_cdp_page_ids(CDP_FIXTURE);
        let tabs = parse_cdp_targets("chrome", CDP_FIXTURE);
        assert_eq!(ids, vec!["A1", "A3", "A4"]);
        assert_eq!(ids.len(), tabs.len());
    }

    #[test]
    fn a_broken_cdp_payload_is_an_empty_list_not_a_panic() {
        assert!(parse_cdp_targets("chrome", "not json").is_empty());
        assert!(parse_cdp_targets("chrome", "{}").is_empty());
        assert!(parse_cdp_targets("chrome", "[]").is_empty());
        assert!(parse_cdp_page_ids("{").is_empty());
    }

    #[test]
    fn a_bare_browser_id_and_a_profile_key_resolve_the_same_way() {
        // `auto`/`default` reach the settings-driven default; a profile key and
        // a bare id both keep their browser part.
        assert_eq!(resolve_browser_id("chrome/Default"), "chrome");
        assert_eq!(resolve_browser_id("brave"), "brave");
        assert_eq!(resolve_browser_id("  edge  "), "edge");
        // Unknown ids survive so the fetch can name them as unsupported.
        assert_eq!(resolve_browser_id("nope"), "nope");
    }

    #[test]
    fn the_app_name_table_covers_every_chromium_browser() {
        for (id, app) in [
            ("chrome", "Google Chrome"),
            ("edge", "Microsoft Edge"),
            ("brave", "Brave Browser"),
            ("chromium", "Chromium"),
        ] {
            assert_eq!(mac_app_name(id), Some(app));
        }
        assert_eq!(mac_app_name("firefox"), None);
    }

    #[test]
    fn the_default_debug_port_is_the_documented_one() {
        assert_eq!(DEFAULT_CDP_PORT, 9222);
    }
}
