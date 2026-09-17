//! External triggers: the `floter://` URL scheme, and the CLI spellings of the
//! very same actions.
//!
//! A URL scheme is an *untrusted input channel*: any web page, any shell
//! script, any other application on the machine can make the operating system
//! hand this process a string. So the surface here is deliberately tiny —
//! exactly two actions exist — and every one of them is decided by a pure
//! function that the tests drive directly:
//!
//! ```text
//!   floter://open
//!       Focus (or summon) the main window. Nothing else.
//!
//!   floter://connect?manifest=<url-encoded value>
//!       Open the manifest-connect *review* dialog for a local absolute
//!       `.json` manifest path or an `https://` manifest URL.
//! ```
//!
//! Everything else is refused. An unknown action is dropped with one log line
//! and no UI noise (it is not a thing the user asked for — it is noise from
//! somewhere else). A *malformed* `floter://` URL, or a `connect` whose
//! manifest fails validation, is worth telling the user about: the window
//! comes forward and one toast rides the app's existing stack.
//!
//! Two invariants the tests pin:
//!
//! * **One allow-list.** [`ACTIONS`] is the crate's only list of external
//!   action names, and both the URL router and the CLI normalizer consult
//!   [`is_action`]. A second copy anywhere is the drift this module exists to
//!   prevent.
//! * **A deep link never installs.** [`connect`] validates the manifest, then
//!   emits an event that makes the frontend open the review dialog. The only
//!   path from there to an installed extension is the user pressing Connect,
//!   which runs the ordinary `extensions_install` pipeline and therefore
//!   leaves the ordinary approval record (`approvedPermissions` /
//!   `approvedAt` / `approvedManifestDigest`).

use std::path::{Component, Path, PathBuf};

use serde::Serialize;
use tauri::{AppHandle, Emitter, Manager, Url};

use crate::extensions::manifest::ExtensionManifest;
use crate::extensions::ExtensionState;

/// The registered URL scheme, without `://`.
pub const SCHEME: &str = "floter";

/// How a `floter://` URL reached the router.
///
/// This is the one place the cold-start / live distinction is made, because it
/// decides whether a resolved `connect` request is *parked* on
/// `AppState.pending_deep_link` for the frontend to pick up later.
///
/// * A **cold start** (`floter connect …` with no instance listening) runs in
///   `setup`, before the webview has mounted its listeners. The request is
///   parked and the frontend consumes it through `take_pending_deep_link`
///   exactly once.
/// * A **live** delivery (the control socket, the single-instance forward, or
///   a macOS `RunEvent::Opened`) finds the listeners already attached, so the
///   event itself is the delivery. Parking it as well would leave a slot that
///   is never taken — until a webview reload (dev HMR) mounts the frontend
///   again and re-opens a dialog the user already dismissed.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Delivery {
    /// The process that received the URL is the one that just started.
    ColdStart,
    /// An already-running instance received the URL.
    Live,
}

/// The complete set of actions this app accepts from outside itself.
///
/// This is the only place the set is written down. The URL router
/// ([`parse_url`]) and the CLI normalizer ([`canonical_argument`]) both call
/// [`is_action`]; nothing else may spell the names out.
pub const ACTIONS: &[&str] = &["open", "connect"];

/// Event asking the frontend to open the manifest-connect review dialog.
pub const CONNECT_EVENT: &str = "floter://deep-link-connect";

/// Event carrying the dictionary key of a refusal the user should see.
pub const REJECT_EVENT: &str = "floter://deep-link-rejected";

/// The one user-facing string a refused deep link produces. The technical
/// reason goes to the log, never into the toast: the person who clicked the
/// link cannot act on "path traversal", and the person who can reads logs.
pub const REJECT_MESSAGE_KEY: &str = "settings.deepLinkRejected";

/// Ceiling on a remotely fetched manifest. Same order of magnitude as the
/// official-index limit; a manifest is kilobytes.
pub const MAX_MANIFEST_BYTES: usize = 1024 * 1024;

/// Where a `connect` manifest comes from. Both variants are validated before
/// they exist: this type cannot be constructed from an unchecked string.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum ManifestSource {
    /// An absolute local path ending in `.json`.
    Local(PathBuf),
    /// An `https://` URL ending in `.json`, with no embedded credentials.
    Remote(String),
}

/// One accepted external trigger. Reaching this type means every check passed.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum Trigger {
    Open,
    Connect(ManifestSource),
}

/// Why a trigger was refused. Every variant carries the offending value so the
/// log line is diagnosable; none of them reaches the user verbatim.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum Reject {
    MalformedUrl(String),
    UnsupportedScheme(String),
    UnknownAction(String),
    UnknownParameter(String),
    MissingManifest,
    ManifestNotAbsolute(String),
    ManifestTraversal(String),
    ManifestNotJson(String),
    ManifestInsecure(String),
}

impl Reject {
    /// Whether this refusal is worth a toast. An unknown action is somebody
    /// else's bug or a probe, and the product decision is to stay quiet about
    /// it; a malformed link is a user-visible dead end worth one line.
    pub fn is_silent(&self) -> bool {
        matches!(self, Self::UnknownAction(_) | Self::UnsupportedScheme(_))
    }

    /// One log line, with the value that was refused.
    pub fn reason(&self) -> String {
        match self {
            Self::MalformedUrl(value) => format!("malformed URL: {value}"),
            Self::UnsupportedScheme(scheme) => format!("unsupported scheme: {scheme}"),
            Self::UnknownAction(action) => format!("unknown action: {action}"),
            Self::UnknownParameter(name) => format!("unknown parameter: {name}"),
            Self::MissingManifest => "connect is missing its manifest parameter".to_string(),
            Self::ManifestNotAbsolute(value) => {
                format!("manifest is not an absolute path: {value}")
            }
            Self::ManifestTraversal(value) => format!("manifest path traverses upwards: {value}"),
            Self::ManifestNotJson(value) => format!("manifest is not a .json file: {value}"),
            Self::ManifestInsecure(value) => {
                format!("manifest must be a local path or an https URL: {value}")
            }
        }
    }
}

/// Whether `name` is one of the actions this app accepts from outside.
///
/// The URL router and the CLI normalizer both call this. It reads [`ACTIONS`]
/// rather than restating it, so the two entry points cannot drift into two
/// different allow-lists.
pub fn is_action(name: &str) -> bool {
    ACTIONS.contains(&name)
}

/// Validate a `connect` manifest parameter.
///
/// The allowed shapes are exactly two, and the order of the checks is the
/// order of the refusal matrix in the round's report:
///
/// | input                                   | verdict |
/// |-----------------------------------------|---------|
/// | `/home/u/tool.json`                     | local   |
/// | `C:\tools\tool.json`                    | local   |
/// | `https://example.com/tool.json`         | remote  |
/// | `tool.json` (relative)                  | refuse  |
/// | `/home/u/../../etc/tool.json`           | refuse  |
/// | `/home/u/tool.txt`                      | refuse  |
/// | `file:///home/u/tool.json`              | refuse  |
/// | `http://example.com/tool.json`          | refuse  |
/// | `https://user:pw@example.com/tool.json` | refuse  |
///
/// The structural check ("is this actually a manifest?") is *not* here: it
/// needs to read the file or the network, and keeping this function pure is
/// what lets the matrix be a unit test. [`connect`] runs the structural check
/// before anything reaches the frontend.
pub fn validate_manifest_source(raw: &str) -> Result<ManifestSource, Reject> {
    let value = raw.trim();
    if value.is_empty() {
        return Err(Reject::MissingManifest);
    }
    // Control characters never belong in a path or a URL, and a NUL would
    // truncate the value the moment it reached the filesystem or a header.
    if value.chars().any(char::is_control) {
        return Err(Reject::ManifestInsecure(value.to_string()));
    }

    if let Some((scheme, _)) = value.split_once("://") {
        // An explicit scheme must be `https`. `file://`, `http://` and every
        // other transport are refused here rather than later, so the refusal
        // is identical on every platform and cannot be undone by a redirect:
        // the extension client refuses non-HTTPS redirects too.
        if !scheme.eq_ignore_ascii_case("https") {
            return Err(Reject::ManifestInsecure(value.to_string()));
        }
        let url = Url::parse(value).map_err(|_| Reject::ManifestInsecure(value.to_string()))?;
        if url.host_str().is_none() {
            return Err(Reject::ManifestInsecure(value.to_string()));
        }
        if !url.username().is_empty() || url.password().is_some() {
            return Err(Reject::ManifestInsecure(value.to_string()));
        }
        if !is_json_manifest_path(&url) {
            return Err(Reject::ManifestNotJson(value.to_string()));
        }
        return Ok(ManifestSource::Remote(url.into()));
    }

    // No scheme: a local absolute path. `..` is refused by component rather
    // than by substring, so a file legitimately named `..notes.json` is not
    // caught by accident and `/a/../b.json` is.
    let path = Path::new(value);
    if !path.is_absolute() {
        return Err(Reject::ManifestNotAbsolute(value.to_string()));
    }
    if path
        .components()
        .any(|component| matches!(component, Component::ParentDir))
    {
        return Err(Reject::ManifestTraversal(value.to_string()));
    }
    if !value.ends_with(".json") {
        return Err(Reject::ManifestNotJson(value.to_string()));
    }
    Ok(ManifestSource::Local(path.to_path_buf()))
}

/// Route one `floter://` URL. This is the single authority on which external
/// actions exist and what each one may carry.
pub fn parse_url(raw: &str) -> Result<Trigger, Reject> {
    let url = Url::parse(raw).map_err(|_| Reject::MalformedUrl(raw.to_string()))?;
    if url.scheme() != SCHEME {
        return Err(Reject::UnsupportedScheme(url.scheme().to_string()));
    }
    // `floter://connect?…` — the action is the authority. A URL without one
    // (`floter:connect`) is malformed for this scheme rather than a second
    // spelling to support.
    let action = url
        .host_str()
        .filter(|host| !host.is_empty())
        .ok_or_else(|| Reject::MalformedUrl(raw.to_string()))?;
    if !is_action(action) {
        return Err(Reject::UnknownAction(action.to_string()));
    }

    let mut manifest = None;
    for (key, value) in url.query_pairs() {
        if action == "connect" && key == "manifest" && manifest.is_none() {
            manifest = Some(value.into_owned());
            continue;
        }
        return Err(Reject::UnknownParameter(key.into_owned()));
    }

    match action {
        "open" => Ok(Trigger::Open),
        _ => Ok(Trigger::Connect(validate_manifest_source(
            &manifest.ok_or(Reject::MissingManifest)?,
        )?)),
    }
}

/// The suffix rule a remote manifest URL must satisfy.
///
/// Applied twice, on purpose: once to the URL the user handed over (in
/// [`validate_manifest_source`]) and once to whatever URL a redirect actually
/// landed on (in [`stage_remote`]). A link whose path ends in `.json` is not a
/// promise that the *redirect target* does, and the two checks must not be
/// allowed to drift into two different rules.
fn is_json_manifest_path(url: &Url) -> bool {
    url.path().ends_with(".json")
}

/// Normalize a process argument list into the one URL form.
///
/// This is the CLI half of the merge: `floter open` and `floter connect
/// <value>` are spelled as the URLs they mean, and then routed by [`parse_url`]
/// exactly like a link that arrived from the operating system. The action name
/// is *not* re-checked against a second list — it is copied into the URL and
/// the router decides — and no manifest validation happens here either. One
/// parser, one validator, two spellings.
pub fn canonical_argument(args: &[String]) -> Option<String> {
    // A URL handed over by the OS is already in the canonical form.
    for argument in args.iter().skip(1) {
        if argument.starts_with(&format!("{SCHEME}://")) {
            return Some(argument.clone());
        }
    }
    // `floter <action> [value]`. Flags are skipped, so `floter --background
    // open` and `floter open` normalize identically.
    let mut words = args
        .iter()
        .skip(1)
        .filter(|argument| !argument.starts_with('-'));
    let action = words.next()?;
    if !is_action(action) {
        return None;
    }
    let mut url = Url::parse(&format!("{SCHEME}://{action}")).ok()?;
    if let Some(value) = words.next() {
        // A value on `open` is carried along and refused by the router, which
        // is the point: there is one place that decides what each action may
        // carry.
        url.query_pairs_mut().append_pair("manifest", value);
    }
    // A third word is a malformed invocation, not a trigger.
    if words.next().is_some() {
        return None;
    }
    Some(url.into())
}

/// A URL normalized before the app finished starting (`floter connect …` with
/// no instance listening yet) is stored on `AppState.pending_deep_link` by
/// [`connect`] itself, and the frontend consumes it through this command once
/// its listeners are up — the same contract `pending_plugin_open` uses for a
/// cold-start `floter clip`.
#[tauri::command]
pub(crate) fn take_pending_deep_link(
    state: tauri::State<'_, crate::AppState>,
) -> Option<ConnectRequest> {
    take_parked(&state.pending_deep_link)
}

/// Take whatever a cold start parked, if anything. Extracted so the parking
/// contract can be driven directly by a unit test without an `AppHandle`.
fn take_parked(slot: &std::sync::Mutex<Option<ConnectRequest>>) -> Option<ConnectRequest> {
    slot.lock().ok().and_then(|mut slot| slot.take())
}

/// Park a resolved request for the frontend, **but only for a cold start**.
///
/// A live delivery is already an event the mounted listeners receive; writing
/// the slot on that path is what left it permanently non-empty (see
/// [`Delivery`]). The cold-start path keeps its slot so the frontend's
/// mount-time `take_pending_deep_link` still has something to consume.
fn park_for_frontend(
    slot: &std::sync::Mutex<Option<ConnectRequest>>,
    request: ConnectRequest,
    delivery: Delivery,
) {
    if delivery == Delivery::Live {
        return;
    }
    if let Ok(mut slot) = slot.lock() {
        *slot = Some(request);
    }
}

/// The shape the frontend receives for a `connect` request.
#[derive(Debug, Clone, PartialEq, Eq, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct ConnectRequest {
    pub manifest_path: String,
    pub extension_name: String,
    /// `local` or `https`, for the review dialog's source row.
    pub source: String,
}

/// Route a URL and run it. This is the one entry point shared by the
/// single-instance forward, the Linux control socket, the macOS
/// `RunEvent::Opened` delivery and a cold start. `delivery` says which of
/// those it is, and therefore whether a resolved `connect` is parked.
pub fn dispatch_url(app: &AppHandle, raw: &str, delivery: Delivery) {
    match parse_url(raw) {
        Ok(Trigger::Open) => focus_main(app),
        Ok(Trigger::Connect(source)) => {
            let handle = app.clone();
            tauri::async_runtime::spawn(async move {
                if let Err(reason) = connect(&handle, source, delivery).await {
                    tracing::warn!("refusing deep link: {reason}");
                    focus_main(&handle);
                    notify_reject(&handle);
                }
            });
        }
        Err(reject) => {
            tracing::warn!("ignoring deep link {raw}: {}", reject.reason());
            if reject.is_silent() {
                return;
            }
            focus_main(app);
            notify_reject(app);
        }
    }
}

/// Bring the panel forward without changing what it is showing.
fn focus_main(app: &AppHandle) {
    let Some(window) = app.get_webview_window("main") else {
        return;
    };
    let state = app.state::<crate::AppState>();
    let _ = crate::reveal_saved_mode(&window, &state);
}

/// Raise the one refusal toast. The frontend owns the dictionary; the backend
/// sends a key, never a sentence.
fn notify_reject(app: &AppHandle) {
    let _ = app.emit(REJECT_EVENT, REJECT_MESSAGE_KEY);
}

/// Resolve a validated source into a local manifest file, then ask the
/// frontend to review it.
///
/// Deliberately stops at the review dialog. There is no install, no approval
/// and no enable on this path — the user presses Connect in the dialog, which
/// runs `extensions_install` and therefore writes the ordinary approval
/// record. A deep link that could install by itself would be a way to make
/// the machine run a stranger's program from a click in a browser.
async fn connect(
    app: &AppHandle,
    source: ManifestSource,
    delivery: Delivery,
) -> Result<(), String> {
    let state = app.state::<ExtensionState>();
    let (path, label) = match &source {
        ManifestSource::Local(path) => {
            if !path.is_file() {
                return Err(format!("manifest is not a file: {}", path.display()));
            }
            (path.clone(), "local")
        }
        ManifestSource::Remote(url) => (stage_remote(&state, url).await?, "https"),
    };

    // The structural check. `ExtensionManifest::load` runs the bundled JSON
    // schema and the path validation, so a `.json` file that is not a manifest
    // is refused here rather than in the dialog.
    let manifest = ExtensionManifest::load(&path)?;
    let request = ConnectRequest {
        manifest_path: path.to_string_lossy().into_owned(),
        extension_name: manifest.name,
        source: label.to_string(),
    };

    // Parked only for a cold start: a live link finds the webview listening,
    // and writing the slot there would leave it non-empty forever (a webview
    // reload would then re-open this dialog). See [`Delivery`].
    park_for_frontend(
        &app.state::<crate::AppState>().pending_deep_link,
        request.clone(),
        delivery,
    );
    focus_main(app);
    app.emit(CONNECT_EVENT, request)
        .map_err(|error| error.to_string())
}

/// Download a remote manifest into the extension cache and return the staged
/// path. The URL was already constrained to HTTPS without credentials by
/// [`validate_manifest_source`]; the size ceiling and the parse are what keep
/// a hostile response from being handed onward.
async fn stage_remote(state: &ExtensionState, url: &str) -> Result<PathBuf, String> {
    let parsed = Url::parse(url).map_err(|error| format!("invalid manifest URL: {error}"))?;
    let mut response = state
        .client
        .get(parsed)
        .timeout(std::time::Duration::from_secs(10))
        .send()
        .await
        .map_err(|error| format!("cannot download manifest: {error}"))?
        .error_for_status()
        .map_err(|error| format!("cannot download manifest: {error}"))?;
    // A redirect is followed only within HTTPS by the shared client, but the
    // response URL is re-checked because that policy lives one module away.
    // The input URL's `.json` suffix was checked in `validate_manifest_source`;
    // the URL a redirect *landed on* is a second, independent input and gets
    // the same rule. Without this a link to `/tool.json` could 302 to
    // `/download/1234` and be accepted, which is not what "re-validated after
    // a redirect" means.
    validate_followed_manifest_url(response.url())?;
    if response
        .content_length()
        .is_some_and(|length| length > MAX_MANIFEST_BYTES as u64)
    {
        return Err("manifest exceeds the size limit".to_string());
    }
    let mut bytes = Vec::new();
    while let Some(chunk) = response
        .chunk()
        .await
        .map_err(|error| format!("cannot read manifest: {error}"))?
    {
        if bytes
            .len()
            .checked_add(chunk.len())
            .is_none_or(|length| length > MAX_MANIFEST_BYTES)
        {
            return Err("manifest exceeds the size limit".to_string());
        }
        bytes.extend_from_slice(&chunk);
    }
    // Parse before writing: nothing that is not a manifest reaches the disk.
    let manifest = ExtensionManifest::parse(&bytes)?;

    let directory = state.paths.cache.join("deep-link");
    std::fs::create_dir_all(&directory)
        .map_err(|error| format!("cannot create {}: {error}", directory.display()))?;
    let digest = ExtensionManifest::digest_of(url.as_bytes());
    let stem: String = manifest
        .id
        .chars()
        .map(|character| {
            if character.is_ascii_alphanumeric() || matches!(character, '.' | '-' | '_') {
                character
            } else {
                '-'
            }
        })
        .collect();
    let staged = directory.join(format!("{stem}-{}.json", &digest[7..23]));
    std::fs::write(&staged, &bytes)
        .map_err(|error| format!("cannot stage {}: {error}", staged.display()))?;
    Ok(staged)
}

/// Re-check the URL a redirect actually landed on.
///
/// Extracted from [`stage_remote`] so the redirect rule is a pure function the
/// tests drive directly: the `.json` suffix and the HTTPS constraint are the
/// same two checks the input URL passed, applied to the second URL the network
/// chose. The suffix refusal is spelled exactly like the input URL's
/// not-a-json refusal, so the log line is the same whichever URL was at fault.
fn validate_followed_manifest_url(url: &Url) -> Result<(), String> {
    if url.scheme() != "https" {
        return Err("manifest URL redirected away from HTTPS".to_string());
    }
    if !is_json_manifest_path(url) {
        return Err(format!("manifest is not a .json file: {url}"));
    }
    Ok(())
}

/// Register the `floter://` handler with the platform. Best-effort by
/// contract: a machine without `xdg-mime` is still fully usable, it just
/// cannot claim the scheme at runtime (a real install registers it from the
/// bundled `.desktop` file instead).
pub fn register_scheme(app: &AppHandle) {
    #[cfg(desktop)]
    {
        use tauri_plugin_deep_link::DeepLinkExt;
        if let Err(error) = app.deep_link().register_all() {
            tracing::info!("floter:// is not registered at runtime: {error}");
        }
    }
    #[cfg(not(desktop))]
    let _ = app;
}

/// macOS delivers links through the event loop, not through `argv`. Wired only
/// there, so the argument path stays the single route on Linux and Windows.
pub fn listen_for_url_events(app: &AppHandle) {
    #[cfg(target_os = "macos")]
    {
        use tauri_plugin_deep_link::DeepLinkExt;
        let handle = app.clone();
        app.deep_link().on_open_url(move |event| {
            for url in event.urls() {
                // macOS delivers through the running event loop: this is a
                // live delivery, not a cold start.
                dispatch_url(&handle, url.as_str(), Delivery::Live);
            }
        });
    }
    #[cfg(not(target_os = "macos"))]
    let _ = app;
}

#[cfg(test)]
mod tests {
    use super::*;

    fn source(value: &str) -> Result<ManifestSource, Reject> {
        validate_manifest_source(value)
    }

    // ── the allow-list ────────────────────────────────────────────────────

    #[test]
    fn the_external_action_table_is_exactly_two_actions() {
        assert_eq!(ACTIONS, &["open", "connect"]);
        assert!(is_action("open"));
        assert!(is_action("connect"));
        for refused in ["clip", "install", "settings", "Open", "", "open/", ".."] {
            assert!(!is_action(refused), "{refused} must not be an action");
        }
    }

    // ── the accepted shapes ───────────────────────────────────────────────

    #[test]
    fn open_is_the_only_action_with_no_payload() {
        assert_eq!(parse_url("floter://open"), Ok(Trigger::Open));
    }

    #[test]
    fn connect_accepts_an_absolute_local_manifest() {
        assert_eq!(
            parse_url("floter://connect?manifest=%2Fhome%2Fu%2Ftool.json"),
            Ok(Trigger::Connect(ManifestSource::Local(PathBuf::from(
                "/home/u/tool.json"
            ))))
        );
        // A literal absolute path needs no escaping.
        assert_eq!(
            parse_url("floter://connect?manifest=/opt/tools/tool.json"),
            Ok(Trigger::Connect(ManifestSource::Local(PathBuf::from(
                "/opt/tools/tool.json"
            ))))
        );
    }

    #[test]
    fn connect_accepts_an_https_manifest() {
        assert_eq!(
            parse_url("floter://connect?manifest=https%3A%2F%2Fexample.com%2Ftool.json"),
            Ok(Trigger::Connect(ManifestSource::Remote(
                "https://example.com/tool.json".to_string()
            )))
        );
    }

    #[test]
    fn connect_accepts_a_manifest_from_a_subdirectory() {
        assert_eq!(
            source("/home/u/.config/floter/tools/tool.json"),
            Ok(ManifestSource::Local(PathBuf::from(
                "/home/u/.config/floter/tools/tool.json"
            )))
        );
    }

    // ── the refused shapes ────────────────────────────────────────────────

    #[test]
    fn file_urls_are_refused() {
        assert!(matches!(
            source("file:///home/u/tool.json"),
            Err(Reject::ManifestInsecure(_))
        ));
    }

    #[test]
    fn plain_http_is_refused() {
        assert!(matches!(
            source("http://example.com/tool.json"),
            Err(Reject::ManifestInsecure(_))
        ));
    }

    #[test]
    fn traversal_is_refused() {
        assert!(matches!(
            source("/home/u/../../etc/tool.json"),
            Err(Reject::ManifestTraversal(_))
        ));
        assert!(matches!(
            source("../tool.json"),
            Err(Reject::ManifestNotAbsolute(_))
        ));
        // An interior `..` is refused even when it would cancel out.
        assert!(matches!(
            source("/home/u/.config/floter/tools/../tools/tool.json"),
            Err(Reject::ManifestTraversal(_))
        ));
        // A file legitimately named `..notes.json` is not caught by accident.
        assert_eq!(
            source("/home/u/..notes.json"),
            Ok(ManifestSource::Local(PathBuf::from("/home/u/..notes.json")))
        );
    }

    #[test]
    fn relative_paths_and_bad_suffixes_are_refused() {
        assert!(matches!(
            source("tool.json"),
            Err(Reject::ManifestNotAbsolute(_))
        ));
        assert!(matches!(
            source("/home/u/tool.txt"),
            Err(Reject::ManifestNotJson(_))
        ));
        assert!(matches!(
            source("https://example.com/tool.txt"),
            Err(Reject::ManifestNotJson(_))
        ));
    }

    #[test]
    fn embedded_credentials_and_empty_values_are_refused() {
        assert!(matches!(
            source("https://user:pw@example.com/tool.json"),
            Err(Reject::ManifestInsecure(_))
        ));
        assert_eq!(source("   "), Err(Reject::MissingManifest));
        assert!(matches!(
            source("/home/u/tool\n.json"),
            Err(Reject::ManifestInsecure(_))
        ));
    }

    #[test]
    fn unknown_actions_are_refused_silently() {
        let rejected = parse_url("floter://clip").expect_err("clip is not an action");
        assert_eq!(rejected, Reject::UnknownAction("clip".to_string()));
        assert!(rejected.is_silent(), "an unknown action raises no toast");
        // `floter://` with no authority at all is malformed, not unknown.
        assert!(matches!(
            parse_url("floter://"),
            Err(Reject::MalformedUrl(_))
        ));
    }

    #[test]
    fn unknown_parameters_are_refused() {
        assert!(matches!(
            parse_url("floter://open?query=hello"),
            Err(Reject::UnknownParameter(_))
        ));
        assert!(matches!(
            parse_url("floter://connect?manifest=/a/tool.json&extra=1"),
            Err(Reject::UnknownParameter(_))
        ));
        assert_eq!(parse_url("floter://connect"), Err(Reject::MissingManifest));
    }

    #[test]
    fn other_schemes_are_refused() {
        assert!(matches!(
            parse_url("https://example.com/tool.json"),
            Err(Reject::UnsupportedScheme(_))
        ));
        assert!(matches!(
            parse_url("not a url at all"),
            Err(Reject::MalformedUrl(_))
        ));
    }

    #[test]
    fn a_refusal_only_tells_the_user_it_did_nothing() {
        let noisy = [
            Reject::MalformedUrl("x".into()),
            Reject::MissingManifest,
            Reject::ManifestTraversal("x".into()),
            Reject::ManifestInsecure("x".into()),
        ];
        for reject in noisy {
            assert!(!reject.is_silent(), "{reject:?} is worth one toast");
            // The reason is for the log, and it names the value that failed.
            assert!(reject.reason().contains('x') || reject.reason().contains("missing"));
        }
    }

    // ── the structural check ──────────────────────────────────────────────

    // The path/suffix checks are only the first half. A `.json` file that is
    // not a manifest must be refused before the frontend ever sees a dialog —
    // otherwise a link could put arbitrary local JSON in front of the user and
    // make the *next* screen's error the discovery mechanism.
    #[test]
    fn a_json_file_that_is_not_a_manifest_is_refused() {
        let directory = tempfile::tempdir().expect("temp dir");
        let not_a_manifest = directory.path().join("tool.json");
        std::fs::write(&not_a_manifest, br#"{ "hello": "world" }"#).expect("write");
        let error = ExtensionManifest::load(&not_a_manifest).expect_err("must refuse");
        assert!(
            error.contains("schema") || error.contains("Invalid extension manifest"),
            "a non-manifest JSON must fail the structural check, got: {error}"
        );
        // The path itself was fine, so the refusal came from the structure and
        // not from the validator this module owns.
        assert!(matches!(
            validate_manifest_source(&not_a_manifest.to_string_lossy()),
            Ok(ManifestSource::Local(_))
        ));
    }

    #[test]
    fn a_real_manifest_passes_the_structural_check() {
        let directory = tempfile::tempdir().expect("temp dir");
        let manifest_path = directory.path().join("tool.json");
        std::fs::write(
            &manifest_path,
            serde_json::to_vec(&serde_json::json!({
                "schemaVersion": "2.0",
                "id": "deep.linked",
                "name": "Deep Linked Tool",
                "publisher": { "id": "local-user", "name": "Local user" },
                "compatibility": { "floter": ">=0.0.1", "providerProtocol": "^1.0" },
                "distribution": { "type": "local" },
                "runtime": { "type": "system", "executableNames": ["true"] },
                "provider": { "type": "executable", "argsPrefix": [], "environment": {} },
            }))
            .expect("serialize"),
        )
        .expect("write");
        let manifest = ExtensionManifest::load(&manifest_path).expect("a real manifest loads");
        assert_eq!(manifest.id, "deep.linked");
        assert_eq!(manifest.name, "Deep Linked Tool");
    }

    // ── the CLI/scheme merge ──────────────────────────────────────────────

    #[test]
    fn cli_spellings_normalize_into_the_same_urls() {
        let args = |list: &[&str]| list.iter().map(|a| (*a).to_string()).collect::<Vec<_>>();
        assert_eq!(
            canonical_argument(&args(&["floter", "open"])),
            Some("floter://open".to_string())
        );
        let connect = canonical_argument(&args(&["floter", "connect", "/opt/tool.json"]))
            .expect("connect normalizes");
        assert_eq!(
            parse_url(&connect),
            Ok(Trigger::Connect(ManifestSource::Local(PathBuf::from(
                "/opt/tool.json"
            ))))
        );
        // Flags are skipped, not treated as the action.
        assert_eq!(
            canonical_argument(&args(&["floter", "--background", "open"])),
            Some("floter://open".to_string())
        );
        // A URL argument passes through untouched.
        assert_eq!(
            canonical_argument(&args(&["floter", "floter://open"])),
            Some("floter://open".to_string())
        );
    }

    #[test]
    fn cli_spellings_the_router_refuses_are_not_special_cased() {
        let args = |list: &[&str]| list.iter().map(|a| (*a).to_string()).collect::<Vec<_>>();
        // `clip` and `--toggle` keep their own CLI paths; they are not
        // triggers and the normalizer must not invent an action for them.
        assert_eq!(canonical_argument(&args(&["floter", "clip"])), None);
        assert_eq!(canonical_argument(&args(&["floter", "--toggle"])), None);
        assert_eq!(canonical_argument(&args(&["floter"])), None);
        // A refused manifest reaches the router as a URL and is refused there,
        // so the CLI has no validator of its own to keep in sync.
        let bad =
            canonical_argument(&args(&["floter", "connect", "../tool.json"])).expect("normalizes");
        assert!(matches!(
            parse_url(&bad),
            Err(Reject::ManifestNotAbsolute(_))
        ));
    }

    // ── the cold-start / live parking fork (m-1) ─────────────────────────

    fn request() -> ConnectRequest {
        ConnectRequest {
            manifest_path: "/home/u/tool.json".to_string(),
            extension_name: "Tool".to_string(),
            source: "local".to_string(),
        }
    }

    #[test]
    fn a_cold_start_parks_the_request_for_the_frontend() {
        // `setup` dispatches before the webview has mounted its listeners, so
        // the slot is the only thing the mount-time take can find.
        let slot = std::sync::Mutex::new(None);
        park_for_frontend(&slot, request(), Delivery::ColdStart);
        assert_eq!(take_parked(&slot), Some(request()));
        // Consumed once: the frontend's `take_pending_deep_link` is a take, not
        // a read, so a later mount cannot replay it.
        assert_eq!(take_parked(&slot), None);
    }

    #[test]
    fn a_live_delivery_does_not_park_the_request() {
        // A live link finds the listeners attached and the event is the whole
        // delivery. Parking it would leave the slot non-empty for the rest of
        // the process' life, and a webview reload (dev HMR) would re-open the
        // dialog the user already dismissed.
        let slot = std::sync::Mutex::new(None);
        park_for_frontend(&slot, request(), Delivery::Live);
        assert_eq!(take_parked(&slot), None);
    }

    #[test]
    fn a_live_delivery_clears_nothing_but_a_cold_start_after_it_still_parks() {
        // The two paths are independent: a live delivery neither writes nor
        // erases what a preceding cold start parked (there is no reload between
        // them), and a later cold start parks normally.
        let slot = std::sync::Mutex::new(None);
        park_for_frontend(&slot, request(), Delivery::Live);
        park_for_frontend(&slot, request(), Delivery::ColdStart);
        assert_eq!(take_parked(&slot), Some(request()));
    }

    // ── the redirect suffix re-check (m-2) ───────────────────────────────

    #[test]
    fn a_redirect_to_an_https_json_path_is_accepted() {
        assert_eq!(
            validate_followed_manifest_url(
                &Url::parse("https://example.com/downloads/tool.json").expect("url")
            ),
            Ok(())
        );
    }

    #[test]
    fn a_redirect_to_a_non_json_path_is_refused_like_the_input_url() {
        // The same suffix rule as the input URL, and the same wording, so the
        // log line cannot tell the user two different stories about one rule.
        let error = validate_followed_manifest_url(
            &Url::parse("https://example.com/download/1234").expect("url"),
        )
        .expect_err("a non-.json redirect target must be refused");
        let expected =
            Reject::ManifestNotJson("https://example.com/download/1234".to_string()).reason();
        assert_eq!(error, expected);
    }

    #[test]
    fn a_redirect_away_from_https_is_refused() {
        assert!(validate_followed_manifest_url(
            &Url::parse("http://example.com/tool.json").expect("url")
        )
        .is_err());
    }
}
