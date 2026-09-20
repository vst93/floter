//! External triggers: the `floter://` URL scheme, and the CLI spellings of the
//! very same actions.
//!
//! A URL scheme is an *untrusted input channel*: any web page, any shell
//! script, any other application on the machine can make the operating system
//! hand this process a string. So the surface here is deliberately tiny —
//! exactly three actions exist — and every one of them is decided by a pure
//! function that the tests drive directly:
//!
//! ```text
//!   floter://open
//!       Focus (or summon) the main window. Nothing else.
//!
//!   floter://connect?manifest=<url-encoded value>
//!       Open the manifest-connect *review* dialog for a local absolute
//!       `.json` manifest path or an `https://` manifest URL.
//!
//!   floter://register?cmd=<basename>[&args=<inert tokens>]
//!       Resolve a CLI *name* against the live discovery inventory and hand
//!       the candidate to the integrations review surface. `cmd` is a bare
//!       basename, never a path and never a shell string; `args` is an
//!       optional, strictly shell-inert argument hint carried to the review
//!       surface for display only. Nothing is bound, installed or executed.
//! ```
//!
//! The terminal spellings of the same actions are normalized into these URLs by
//! [`canonical_argument`] and routed by [`parse_url`] — one parser, two
//! spellings. `floter register <cmd>` is the one place where a spelling carries
//! something a URL cannot say: `--yes`, the user's own confirmation, folded
//! into a `confirm=1` query parameter. It is what lets a terminal invocation
//! *finish* the connection (see below) instead of stopping at the offer.
//!
//! Everything else is refused. An unknown action is dropped with one log line
//! and no UI noise (it is not a thing the user asked for — it is noise from
//! somewhere else). A *malformed* `floter://` URL, or a `connect` whose
//! manifest fails validation, is worth telling the user about: the window
//! comes forward and one toast rides the app's existing stack.
//!
//! Three invariants the tests pin:
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
//! * **A deep link never binds either.** [`register`] resolves a command to a
//!   [`ToolCandidate`] and, when the trigger arrived as a URL, stops at the
//!   same review surface. The one exception is the *terminal* spelling with
//!   the user's confirmation: [`register_may_bind`] is true only when the
//!   name is on the curated allow-list or the invocation carried `--yes`, and
//!   even then the binding runs through `install::connect_tool` — the
//!   *existing* connect path, with the *fixed* disclosure set — never through
//!   a private lock write. No download, no install, and a name that does not
//!   resolve to an available executable binds nothing.
//! * **The lock has one writer.** This module never names `ToolLock`,
//!   `bind_locator` or `lock.save`; the only binding it can cause goes through
//!   `install::connect_tool`.

use std::path::{Component, Path, PathBuf};

use serde::Serialize;
use tauri::{AppHandle, Emitter, Manager, Url};

use crate::extensions::inventory::ToolCandidate;
use crate::extensions::manifest::ExtensionManifest;
use crate::extensions::{resolver, ExtensionState, ResolveRequest, ResolveResult};

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
pub const ACTIONS: &[&str] = &["open", "connect", "register"];

/// Event asking the frontend to open the manifest-connect review dialog.
pub const CONNECT_EVENT: &str = "floter://deep-link-connect";

/// Event asking the frontend to highlight a discovered tool on the review
/// surface. Deliberately distinct from [`CONNECT_EVENT`]: the payload is a
/// resolved [`RegisterRequest`], not a manifest path.
pub const REGISTER_EVENT: &str = "floter://deep-link-register";

/// Ceiling on the `cmd` basename, matching the custom-integration command
/// pattern (`[a-z0-9][a-z0-9_-]{0,63}`) rather than inventing a second bound.
pub const MAX_COMMAND_CHARS: usize = 64;

/// Ceiling on one `args` token.
pub const MAX_ARGUMENT_CHARS: usize = 64;

/// Ceiling on how many `args` tokens a link may carry.
pub const MAX_ARGUMENT_COUNT: usize = 16;

/// Ceiling on the whole `args` value.
pub const MAX_ARGUMENTS_TOTAL_CHARS: usize = 256;

/// Event carrying the dictionary key of a refusal the user should see.
pub const REJECT_EVENT: &str = "floter://deep-link-rejected";

/// The one user-facing string a refused deep link produces. The technical
/// reason goes to the log, never into the toast: the person who clicked the
/// link cannot act on "path traversal", and the person who can reads logs.
pub const REJECT_MESSAGE_KEY: &str = "settings.deepLinkRejected";

/// Ceiling on a remotely fetched manifest. A manifest is kilobytes, so this
/// is a generous bound that still stops a hostile server from streaming.
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

/// A `register` payload: the bare name of a CLI tool and an optional,
/// strictly shell-inert argument hint.
///
/// `args` is carried to the review surface as *context only*. It is never
/// executed by the backend, never passed to a shell, and never reaches a
/// binding without the user pressing Connect on the ordinary zero-form path —
/// which is why the validation below refuses every character a shell would
/// interpret. See [`validate_arguments`].
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct CommandRequest {
    pub command: String,
    pub args: Option<Vec<String>>,
    /// The user's explicit `--yes` on the terminal spelling of this trigger.
    /// It is content, not permission: the *origin* decides whether a bind may
    /// happen at all, and `--yes` is one of the two ways the terminal path may
    /// ask for one (see [`register_may_bind`]). A URL cannot produce it except
    /// by spelling `confirm=1`, which is inert without a terminal origin.
    pub confirmed: bool,
    /// The transport this trigger arrived on.
    pub origin: RegisterOrigin,
}

/// Where a `register` trigger came from.
///
/// This is the bit that decides whether the trigger may *finish* a connection,
/// and it is deliberately **not** a URL parameter: a `floter://` URL is an
/// untrusted input channel (any page can make the OS hand one over), so a URL
/// that could ask for a bind would be a way to make this machine bind a
/// stranger's tool from a click in a browser. The origin is therefore decided
/// by the *transport* — the process' own argv, or the control socket line,
/// which only the user's own processes can write.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum RegisterOrigin {
    /// A `floter://register?…` link, from the OS or from the CLI's URL spelling.
    /// Resolves and stops at the review surface.
    Link,
    /// `floter register <cmd>` typed in a terminal. The same resolution, and
    /// then — only when [`register_may_bind`] says so — the ordinary connect
    /// pipeline.
    Terminal,
}

/// One accepted external trigger. Reaching this type means every check passed.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum Trigger {
    Open,
    Connect(ManifestSource),
    Register(CommandRequest),
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
    MissingCommand,
    CommandNotBasename(String),
    CommandInsecure(String),
    CommandTooLong(String),
    InvalidArguments(String),
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
            Self::MissingCommand => "register is missing its cmd parameter".to_string(),
            Self::CommandNotBasename(value) => {
                format!("register cmd is not a bare command name: {value}")
            }
            Self::CommandInsecure(value) => {
                format!("register cmd contains characters that are not allowed: {value}")
            }
            Self::CommandTooLong(value) => format!("register cmd is too long: {value}"),
            Self::InvalidArguments(value) => {
                format!("register args are not a plain argument list: {value}")
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

/// Validate a `register` `cmd` parameter.
///
/// The value is a **name**, not a command line and not a path. Everything a
/// shell, a path resolver or a launcher could read as "do something else" is
/// refused here, on the pure-function side, so the refusal matrix is a unit
/// test:
///
/// | input           | verdict |
/// |-----------------|---------|
/// | `rg`            | accept  |
/// | `rg.exe`        | accept  |
/// | `python3.11`    | accept  |
/// | `/usr/bin/rg`   | refuse  |
/// | `..\rg`         | refuse  |
/// | `rg;rm -rf /`   | refuse  |
/// | `rg --exec=ls`  | refuse  |
/// | `rg\n`          | refuse  |
/// | ``              | refuse  |
/// | 65 characters   | refuse  |
///
/// The rule is structural: no `/` and no `\` (so it is one path component), no
/// `..` (so it is not a traversal), no control character, no whitespace, at
/// most [`MAX_COMMAND_CHARS`] characters, and only `[A-Za-z0-9._+-]`. A `-` is
/// allowed inside a name (`my-tool`) but a leading `-` is refused, because a
/// leading dash is an option to whatever consumes the value, not a name.
pub fn validate_command(raw: &str) -> Result<String, Reject> {
    let value = raw.trim();
    if value.is_empty() {
        return Err(Reject::MissingCommand);
    }
    if value.chars().count() > MAX_COMMAND_CHARS {
        return Err(Reject::CommandTooLong(value.to_string()));
    }
    if value.contains('/') || value.contains('\\') {
        return Err(Reject::CommandNotBasename(value.to_string()));
    }
    if value.split('.').any(|segment| segment == "..") || value == ".." {
        return Err(Reject::CommandNotBasename(value.to_string()));
    }
    if value.starts_with('-') {
        return Err(Reject::CommandInsecure(value.to_string()));
    }
    if !value.chars().all(|character| {
        character.is_ascii_alphanumeric() || matches!(character, '.' | '_' | '+' | '-')
    }) {
        return Err(Reject::CommandInsecure(value.to_string()));
    }
    Ok(value.to_string())
}

/// Validate an optional `register` `args` parameter.
///
/// The value is split on ASCII spaces into tokens; each token must be
/// shell-inert. Refused per token: empty tokens, control characters, any of
/// the shell metacharacters `; | & $ ` < > ( ) { } [ ] * ? ! ~ \\ " '`, a
/// leading `-`, and anything over [`MAX_ARGUMENT_CHARS`]. The whole list is
/// bounded by [`MAX_ARGUMENT_COUNT`] tokens and
/// [`MAX_ARGUMENTS_TOTAL_CHARS`] characters.
///
/// These arguments are a *hint shown on the review surface*. The backend never
/// runs them and never joins them into a command line, so the refusal is about
/// keeping a hostile link from smuggling something a later reader might execute
/// — not about escaping.
pub fn validate_arguments(raw: &str) -> Result<Option<Vec<String>>, Reject> {
    let value = raw.trim();
    if value.is_empty() {
        return Ok(None);
    }
    if value.chars().count() > MAX_ARGUMENTS_TOTAL_CHARS {
        return Err(Reject::InvalidArguments(value.to_string()));
    }
    let mut tokens = Vec::new();
    for token in value.split(' ') {
        let token = token.trim();
        if token.is_empty() {
            continue;
        }
        if token.chars().count() > MAX_ARGUMENT_CHARS {
            return Err(Reject::InvalidArguments(value.to_string()));
        }
        if token.starts_with('-') {
            return Err(Reject::InvalidArguments(value.to_string()));
        }
        if !token.chars().all(is_inert_argument_char) {
            return Err(Reject::InvalidArguments(value.to_string()));
        }
        tokens.push(token.to_string());
    }
    if tokens.len() > MAX_ARGUMENT_COUNT {
        return Err(Reject::InvalidArguments(value.to_string()));
    }
    if tokens.is_empty() {
        return Ok(None);
    }
    Ok(Some(tokens))
}

/// The one character class an `args` token may use: letters, digits and the
/// punctuation that means nothing to a shell or to a path resolver. Everything
/// else (`;`, `|`, `$`, a backtick, a quote, a glob, a tilde, a slash) is a
/// refusal.
fn is_inert_argument_char(character: char) -> bool {
    character.is_ascii_alphanumeric()
        || matches!(character, '.' | '_' | '+' | '=' | ':' | ',' | '@' | '%')
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
    let mut command = None;
    let mut arguments = None;
    let mut confirmed = false;
    for (key, value) in url.query_pairs() {
        if action == "connect" && key == "manifest" && manifest.is_none() {
            manifest = Some(value.into_owned());
            continue;
        }
        if action == "register" && key == "cmd" && command.is_none() {
            command = Some(value.into_owned());
            continue;
        }
        if action == "register" && key == "args" && arguments.is_none() {
            arguments = Some(value.into_owned());
            continue;
        }
        // The fourth spelling of the *same* trigger: a terminal invocation
        // carries the user's explicit confirmation as a flag. It is not a
        // parameter of its own (nothing to validate) and not a second action;
        // it only sets the bit that lets the terminal path bind instead of
        // stopping at the review surface. Spelling `confirm=1` in a hand-typed
        // `floter://` URL is inert: the *origin*, not the parameter, is what
        // authorizes a bind (see [`RegisterOrigin`]).
        if action == "register" && key == "confirm" {
            confirmed = true;
            continue;
        }
        return Err(Reject::UnknownParameter(key.into_owned()));
    }

    match action {
        "open" => Ok(Trigger::Open),
        "register" => Ok(Trigger::Register(CommandRequest {
            command: validate_command(&command.ok_or(Reject::MissingCommand)?)?,
            args: arguments
                .as_deref()
                .map(validate_arguments)
                .transpose()?
                .flatten(),
            confirmed,
            // A URL is a URL: it resolves and stops. Only
            // [`parse_terminal_url`] upgrades this, and only for a trigger that
            // arrived on a transport a page cannot write.
            origin: RegisterOrigin::Link,
        })),
        _ => Ok(Trigger::Connect(validate_manifest_source(
            &manifest.ok_or(Reject::MissingManifest)?,
        )?)),
    }
}

/// Parse a URL that arrived on a **terminal** transport (the process' own
/// argv, or a control-socket line) rather than from the operating system.
///
/// It is [`parse_url`] plus one fact the URL cannot carry: the transport. The
/// validation is not duplicated — a `connect` or an `open` is returned exactly
/// as the router returns it — and the only change is
/// [`RegisterOrigin::Terminal`] on a `register` trigger, which is what
/// [`register_may_bind`] reads.
///
/// This exists so the distinction is made at the *edge*, where the transport is
/// still known, and never by looking at the URL's contents. A hostile page can
/// write any URL it likes; it cannot write this process' argv or the socket.
pub fn parse_terminal_url(raw: &str) -> Result<Trigger, Reject> {
    match parse_url(raw)? {
        Trigger::Register(mut request) => {
            request.origin = RegisterOrigin::Terminal;
            Ok(Trigger::Register(request))
        }
        other => Ok(other),
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
    //
    // `--yes` is the one flag with a meaning of its own (see
    // [`CommandRequest::confirmed`]): it is not a value and not an action, it
    // is the user's confirmation, and it is recorded as a query parameter
    // exactly like `cmd` and `args` so the router stays the only validator.
    // Position does not matter — `floter --yes register rg` and `floter
    // register rg --yes` produce the same URL.
    let confirmed = args.iter().skip(1).any(|argument| argument == "--yes");
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
        //
        // `register` names its first value `cmd` and folds every remaining
        // word into `args` (space-joined, exactly the shape the URL carries),
        // because `floter register rg --hidden src` is the natural spelling of
        // `floter://register?cmd=rg&args=--hidden+src`. The validation still
        // lives in the router: this function only builds the URL.
        if action == "register" {
            url.query_pairs_mut().append_pair("cmd", value);
            let rest: Vec<&String> = words.collect();
            if !rest.is_empty() {
                url.query_pairs_mut().append_pair(
                    "args",
                    &rest
                        .iter()
                        .map(|word| word.as_str())
                        .collect::<Vec<_>>()
                        .join(" "),
                );
            }
            if confirmed {
                url.query_pairs_mut().append_pair("confirm", "1");
            }
            return Some(url.into());
        }
        url.query_pairs_mut().append_pair("manifest", value);
    }
    // A third word is a malformed invocation, not a trigger.
    if words.next().is_some() {
        return None;
    }
    Some(url.into())
}

/// Whether an argument list is the **terminal** spelling of the `register`
/// action (`floter register <cmd> [--yes]`).
///
/// This only answers "which transport is this?" — what the invocation *means*
/// is still decided by [`canonical_argument`] and [`parse_terminal_url`], so the
/// CLI keeps one parser. It exists here rather than in `ipc` (where
/// `wants_toggle`/`wants_clip` live) because the answer is needed on every
/// platform, not just the ones with a control socket.
pub fn wants_register(args: &[String]) -> bool {
    args.iter().skip(1).any(|argument| argument == "register")
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
fn take_parked<T>(slot: &std::sync::Mutex<Option<T>>) -> Option<T> {
    slot.lock().ok().and_then(|mut slot| slot.take())
}

/// Park a resolved request for the frontend, **but only for a cold start**.
///
/// A live delivery is already an event the mounted listeners receive; writing
/// the slot on that path is what left it permanently non-empty (see
/// [`Delivery`]). The cold-start path keeps its slot so the frontend's
/// mount-time `take_pending_*` still has something to consume.
///
/// Generic over the request type so `connect` and `register` share one parking
/// contract rather than growing a second, subtly different one.
fn park_for_frontend<T>(slot: &std::sync::Mutex<Option<T>>, request: T, delivery: Delivery) {
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

/// The shape the frontend receives for a `register` request.
///
/// `candidate` is the *discovery* result, never a binding. `None` means the
/// validated name was not found on this device: the frontend renders an inline
/// reason rather than failing silently, and highlights nothing. Even a `Some`
/// candidate is only *offered* — the tool lock is written solely by the user
/// pressing Connect on the ordinary `extensions_connect_tool` path.
#[derive(Debug, Clone, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct RegisterRequest {
    pub command: String,
    /// Shell-inert argument hint, carried for display only.
    pub args: Option<Vec<String>>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub candidate: Option<ToolCandidate>,
    /// Whether the invocation carried an explicit `--yes`. The frontend uses it
    /// to render "bound" instead of "found" when a terminal `floter register
    /// rg --yes` already completed the connection; it is never an approval in
    /// itself (see [`register`]).
    #[serde(default)]
    pub confirmed: bool,
    /// The lock entry written by a terminal invocation that was allowed to
    /// bind. `None` on every other path — including every URL delivery, which
    /// stops at the offer.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub bound: Option<crate::extensions::ExtensionLockEntry>,
    /// Why a terminal invocation that was allowed to bind did not: the ordinary
    /// connect pipeline's own error, carried to the frontend so the row can
    /// explain itself. `None` when nothing was attempted.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub bind_error: Option<String>,
}

/// The second cold-start slot: a `register` request has a different shape from
/// a `connect` one, so it parks in its own cell rather than widening the
/// connect cell into an enum both paths would have to match on.
#[tauri::command]
pub(crate) fn take_pending_deep_link_register(
    state: tauri::State<'_, crate::AppState>,
) -> Option<RegisterRequest> {
    take_parked(&state.pending_deep_link_register)
}

/// Route a URL and run it. This is the one entry point shared by the
/// single-instance forward, the Linux control socket, the macOS
/// `RunEvent::Opened` delivery and a cold start. `delivery` says which of
/// those it is, and therefore whether a resolved `connect` is parked.
///
/// Every URL that reaches this function is treated as a **link**: it may
/// resolve and offer, never bind. The terminal spelling goes through
/// [`dispatch_terminal_url`], which is the same router plus the transport fact.
pub fn dispatch_url(app: &AppHandle, raw: &str, delivery: Delivery) {
    match parse_url(raw) {
        Ok(trigger) => dispatch_parsed(app, trigger, delivery),
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

/// Route a URL that arrived on a **terminal** transport.
///
/// The mirror of [`dispatch_url`] with one difference, made at the edge where
/// the transport is still known: [`parse_terminal_url`] marks a `register`
/// trigger [`RegisterOrigin::Terminal`], so it may finish the connection when
/// [`register_may_bind`] allows it. `connect` and `open` behave exactly as they
/// do for a link — a terminal `floter connect …` is still a review dialog, not
/// an install.
///
/// It is called for `floter register <cmd>` and for a `link ` line whose sender
/// was the user's own `floter register` process (see `ipc.rs`), never for an
/// OS-delivered URL.
pub fn dispatch_terminal_url(app: &AppHandle, raw: &str, delivery: Delivery) {
    match parse_terminal_url(raw) {
        Ok(Trigger::Register(request)) => {
            let handle = app.clone();
            tauri::async_runtime::spawn_blocking(move || register(&handle, request, delivery));
        }
        Ok(other) => dispatch_parsed(app, other, delivery),
        Err(reject) => {
            tracing::warn!("ignoring terminal trigger {raw}: {}", reject.reason());
            if reject.is_silent() {
                return;
            }
            focus_main(app);
            notify_reject(app);
        }
    }
}

/// Run an already-parsed trigger. Split out of [`dispatch_url`] so the
/// terminal entry point shares the `open`/`connect` arms rather than
/// re-implementing them.
fn dispatch_parsed(app: &AppHandle, trigger: Trigger, delivery: Delivery) {
    match trigger {
        Trigger::Open => focus_main(app),
        Trigger::Connect(source) => {
            let handle = app.clone();
            tauri::async_runtime::spawn(async move {
                if let Err(reason) = connect(&handle, source, delivery).await {
                    tracing::warn!("refusing deep link: {reason}");
                    focus_main(&handle);
                    notify_reject(&handle);
                }
            });
        }
        // `dispatch_terminal_url` handles this arm itself; reaching here means
        // a link, which never binds.
        Trigger::Register(request) => {
            let handle = app.clone();
            tauri::async_runtime::spawn_blocking(move || register(&handle, request, delivery));
        }
    }
}

/// Resolve a validated `register` command against the live discovery inventory
/// and hand the result to the integrations review surface.
///
/// On the **link** origin this function is exactly what R8-3 built: it reads
/// the inventory, asks the one resolver for a candidate, and emits an event. A
/// URL scheme is reachable from any web page, and a link that could bind a tool
/// would be a way to make this machine bind a stranger's program from a click
/// in a browser.
///
/// On the **terminal** origin — `floter register <cmd>`, which only the user
/// can type — it finishes the job when [`register_may_bind`] allows it (the
/// name is curated, or `--yes` was given) and the name resolved to a real,
/// available executable. Even then the binding is not done here: it runs
/// `install::connect_tool`, the *existing* connect path, with the *fixed*
/// disclosure set, so the lock, the approval record and the catalog stay on the
/// one pipeline every other connection uses. No download, no install, and a
/// name that did not resolve binds nothing.
fn register(app: &AppHandle, request: CommandRequest, delivery: Delivery) {
    let resolved = {
        let state = app.state::<ExtensionState>();
        connect_registered_command(&state, &request)
    };
    if resolved.candidate.is_none() {
        tracing::info!(
            "register found no executable named {} on this device",
            request.command
        );
    }
    if let Some(entry) = resolved.bound.as_ref() {
        tracing::info!(
            "register connected {} as {} ({})",
            request.command,
            entry.id,
            permission_disclosure()
        );
        // Same event as every other mutating extension command, so a window
        // that is already showing the list refreshes instead of going stale.
        let _ = app.emit("extensions-changed", ());
    }
    if let Some(reason) = resolved.bind_error.as_deref() {
        tracing::warn!("register could not connect {}: {reason}", request.command);
    }
    // Same fork as `connect`: a cold start runs before the webview has
    // listeners, so it parks the request; a live delivery is the event itself.
    park_for_frontend(
        &app.state::<crate::AppState>().pending_deep_link_register,
        resolved.clone(),
        delivery,
    );
    focus_main(app);
    if let Err(error) = app.emit(REGISTER_EVENT, resolved) {
        tracing::warn!("could not deliver a register request: {error}");
    }
}

/// Resolve a `register` trigger, and — only when [`register_may_bind`] allows
/// it — finish the connection through the ordinary pipeline.
///
/// This is the whole behaviour of `floter register <cmd>` as one function over
/// an [`ExtensionState`], with no `AppHandle` and no event loop, so the tests
/// can drive the real lock write against a temporary directory instead of
/// asserting a promise about it. [`register`] is the thin event/park wrapper.
///
/// The gate is checked *before* anything can be written, and the write itself is
/// `install::connect_tool` — the same entry point `extensions_connect_tool`
/// calls. There is no private lock write on this path: no `ToolLock`, no
/// `bind_locator`, no `lock.save`. A name that did not resolve to an available
/// executable, or a trigger that did not clear the gate, leaves the lock file
/// byte-for-byte untouched.
pub(crate) fn connect_registered_command(
    state: &ExtensionState,
    request: &CommandRequest,
) -> RegisterRequest {
    let mut resolved = resolve_registered_command(state, request);
    if !register_may_bind(request) {
        return resolved;
    }
    let Some(candidate) = resolved.candidate.clone().filter(|found| found.available) else {
        return resolved;
    };
    match tauri::async_runtime::block_on(crate::extensions::install::connect_tool(state, candidate))
    {
        Ok(entry) => resolved.bound = Some(entry),
        Err(reason) => resolved.bind_error = Some(reason),
    }
    resolved
}

/// The disclosure the terminal path prints and the log records: the tool, and
/// the exact permission set it will run with.
///
/// It reads [`crate::extensions::install::tool_binding_disclosure`] — which
/// reads [`crate::extensions::install::tool_binding_permissions`], the one
/// place the set is written down — rather than restating the three names, so
/// the sentence a user reads can never drift from the set the approval record
/// stores. The `--yes` flag does not skip it: a confirmation is not an
/// exemption from being told what was confirmed.
fn permission_disclosure() -> String {
    crate::extensions::install::tool_binding_disclosure()
}

/// Whether this trigger may *finish* a connection without a dialog.
///
/// Two facts must hold, and neither is reachable from a link:
///
/// * the origin is [`RegisterOrigin::Terminal`] — the invocation came from this
///   process' own argv or from a control-socket line, not from an OS-delivered
///   URL a web page can write;
/// * the user already said yes: the name is on the one curated allow-list, or
///   the invocation carried `--yes`.
///
/// It reads [`crate::extensions::curated_tools::is_curated_command`], so the CLI
/// gate and the discovery ranking share one list and one spelling rule.
pub fn register_may_bind(request: &CommandRequest) -> bool {
    request.origin == RegisterOrigin::Terminal
        && (request.confirmed
            || crate::extensions::curated_tools::is_curated_command(&request.command))
}

/// What `floter register <cmd> [--yes]` should do, decided without printing
/// anything so a test can assert the decision directly.
///
/// The three outcomes are the whole CLI contract: a refusal that exits
/// non-zero, an offer that falls back to the review surface, and a bind that
/// prints the disclosure first.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum RegisterCliPlan {
    /// Nothing can proceed, and the message says why. The caller prints it and
    /// exits non-zero — a command the user typed must never fail silently.
    Refused { message: String },
    /// The name is not curated and `--yes` was not given. Nothing is bound;
    /// the same name is handed to the review surface, and the message tells
    /// the user how to connect it in one step next time.
    Offer { url: String, message: String },
    /// The user already said yes and the name resolved. `url` is what a running
    /// instance receives; `disclosure` is the line printed either way.
    Bind {
        request: CommandRequest,
        url: String,
        disclosure: String,
    },
}

impl RegisterCliPlan {
    /// The process exit code this plan implies.
    pub fn exit_code(&self) -> i32 {
        match self {
            Self::Refused { .. } => 1,
            Self::Offer { .. } | Self::Bind { .. } => 0,
        }
    }

    /// The stdout line(s) this plan prints, in order. Never empty for a refusal
    /// or a bind: those are the two cases a user must be able to read off the
    /// terminal, and the disclosure is one of them.
    pub fn stdout_lines(&self) -> Vec<String> {
        match self {
            Self::Refused { message } => vec![message.clone()],
            Self::Offer { message, .. } => vec![message.clone()],
            Self::Bind {
                request,
                disclosure,
                ..
            } => vec![
                format!("floter: connecting {} ({disclosure})", request.command),
                "floter: connected through the ordinary integration pipeline".to_string(),
            ],
        }
    }
}

/// Decide what `floter register <cmd> [--yes]` means, against a real
/// [`ExtensionState`].
///
/// Returns `None` when `args` is not a register invocation at all, so the caller
/// can fall through to the ordinary launch. Everything it decides is either a
/// router verdict (the same [`parse_terminal_url`] a delivered URL goes
/// through) or a read-only lookup in the discovery inventory — it never writes.
/// The write, when there is one, belongs to [`connect_registered_command`].
pub(crate) fn plan_register_cli(
    state: &ExtensionState,
    args: &[String],
) -> Option<RegisterCliPlan> {
    if !wants_register(args) {
        return None;
    }
    let Some(url) = canonical_argument(args) else {
        return Some(RegisterCliPlan::Refused {
            message: "floter: register needs a command name, for example `floter register rg`"
                .to_string(),
        });
    };
    let request = match parse_terminal_url(&url) {
        Ok(Trigger::Register(request)) => request,
        // A refusal is the router's, verbatim, so the CLI cannot tell a
        // different story than a delivered URL would.
        Err(reject) => {
            return Some(RegisterCliPlan::Refused {
                message: format!("floter: cannot register this name ({})", reject.reason()),
            })
        }
        Ok(_) => return None,
    };
    if !register_may_bind(&request) {
        return Some(RegisterCliPlan::Offer {
            // The offer is a *link*: it opens the review surface, exactly as
            // `floter://register?cmd=…` does.
            url: url.replace("&confirm=1", ""),
            message: format!(
                "floter: {} is not on the curated list; showing it on the integrations review \
                 surface instead of connecting it (add --yes to connect it in one step)",
                request.command
            ),
        });
    }
    let resolved = resolve_registered_command(state, &request);
    let Some(candidate) = resolved.candidate.filter(|found| found.available) else {
        return Some(RegisterCliPlan::Refused {
            message: format!(
                "floter: no executable named {} was found on this device",
                request.command
            ),
        });
    };
    // The disclosure is part of the plan, not of the printing, so removing it
    // is a failing test rather than a silent loss of the one sentence the user
    // is owed.
    let disclosure = format!(
        "{} at {} ({})",
        candidate.name,
        candidate
            .locator
            .executable_path()
            .map(|path| path.display().to_string())
            .unwrap_or_else(|| candidate.id.clone()),
        permission_disclosure()
    );
    Some(RegisterCliPlan::Bind {
        request,
        url,
        disclosure,
    })
}

/// Run `floter register <cmd> [--yes]` as a synchronous terminal command.
///
/// Returns `None` when the caller should keep going — either because `args` is
/// not a register invocation at all, or because the decision was an *offer*,
/// which belongs on the review surface and therefore to the ordinary trigger
/// path (a live instance through the socket, a cold start through the GUI).
/// Otherwise every line has been printed and the returned value is the process
/// exit code.
///
/// `deliver` hands a URL to a running instance (the control socket on Linux).
/// When it succeeds the instance owns the write, which is the point: a second
/// process writing `tool-lock.json` behind a live instance would leave that
/// instance's in-memory lock stale. When it fails there is no instance, so this
/// process is the only writer and performs the connection itself.
pub fn register_cli(
    args: &[String],
    deliver: impl Fn(&str, RegisterOrigin) -> Result<(), String>,
) -> Option<i32> {
    if !wants_register(args) {
        return None;
    }
    let state = match ExtensionState::new() {
        Ok(state) => state,
        Err(reason) => {
            println!("floter: cannot read the integration state: {reason}");
            return Some(1);
        }
    };
    let plan = plan_register_cli(&state, args)?;
    match &plan {
        // A refusal is the one outcome a user must be able to read off the
        // terminal, and the exit code is what makes it a failure rather than a
        // log line.
        RegisterCliPlan::Refused { .. } => {
            for line in plan.stdout_lines() {
                println!("{line}");
            }
            Some(1)
        }
        // An offer is not a terminal outcome: the user asked for the tool to be
        // shown, so the window opens on it. The line is printed here because the
        // user typed a command and deserves an answer, and then the ordinary
        // trigger path takes over (socket, or GUI cold start).
        RegisterCliPlan::Offer { .. } => {
            for line in plan.stdout_lines() {
                println!("{line}");
            }
            None
        }
        RegisterCliPlan::Bind {
            request,
            url,
            disclosure,
        } => {
            // The disclosure is printed before anything is written, and `--yes`
            // does not exempt it.
            println!("floter: connecting {} ({disclosure})", request.command);
            if deliver(url, RegisterOrigin::Terminal).is_ok() {
                println!("floter: connected through the ordinary integration pipeline");
                return Some(0);
            }
            let resolved = connect_registered_command(&state, request);
            if let Some(reason) = resolved.bind_error.as_deref() {
                println!("floter: could not connect {}: {reason}", request.command);
                return Some(1);
            }
            println!("floter: connected through the ordinary integration pipeline");
            Some(0)
        }
    }
}

/// The pure half of [`register`], split out so a unit test can drive it with a
/// temporary [`ExtensionState`] and assert the red line directly: after a
/// resolve, the tool lock and the extension repository are byte-for-byte what
/// they were.
///
/// Resolution uses the *existing* discovery + resolver chain — a first pass
/// against the cached snapshot, then one forced refresh if the name was not
/// found, because a tool installed a minute ago may predate the 5-minute TTL.
/// No second PATH walk and no second name-matching rule is introduced here.
pub(crate) fn resolve_registered_command(
    state: &ExtensionState,
    request: &CommandRequest,
) -> RegisterRequest {
    let names = [request.command.clone()];
    let query = ResolveRequest {
        tool: request.command.clone(),
        required_version: None,
        preferred_locator: None,
    };
    let mut candidate = None;
    for attempt in 0..2 {
        let candidates = {
            let Ok(mut inventory) = state.tool_inventory.lock() else {
                break;
            };
            if attempt == 1 {
                inventory.refresh();
            }
            inventory.candidates()
        };
        match resolver::resolve_executable_names(&query, &names, &candidates) {
            ResolveResult::Selected {
                candidate: found, ..
            } => {
                candidate = Some(found);
                break;
            }
            // Several PATH directories can hold the same name. The resolver
            // reports that explicitly rather than picking; the review surface
            // shows one row per name, so the first tied candidate is the row
            // the user would have seen anyway. Still only an offer.
            ResolveResult::Ambiguous { candidates } => {
                candidate = candidates.into_iter().next().map(|scored| scored.candidate);
                break;
            }
            ResolveResult::NotFound { .. } => continue,
        }
    }
    RegisterRequest {
        command: request.command.clone(),
        args: request.args.clone(),
        candidate,
        confirmed: request.confirmed,
        bound: None,
        bind_error: None,
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
    fn the_external_action_table_is_exactly_three_actions() {
        assert_eq!(ACTIONS, &["open", "connect", "register"]);
        assert!(is_action("open"));
        assert!(is_action("connect"));
        assert!(is_action("register"));
        for refused in ["clip", "install", "settings", "Open", "", "open/", ".."] {
            assert!(!is_action(refused), "{refused} must not be an action");
        }
    }

    // ── register: the third action (R8-3) ─────────────────────────────────

    #[test]
    fn register_accepts_a_bare_command_name() {
        assert_eq!(
            parse_url("floter://register?cmd=rg"),
            Ok(Trigger::Register(CommandRequest {
                command: "rg".to_string(),
                args: None,
                confirmed: false,
                origin: RegisterOrigin::Link,
            }))
        );
        // A Windows launcher suffix and a dotted version are still one name.
        for name in ["rg.exe", "python3.11", "my-tool", "a_b+c"] {
            // `+` is form-encoded as `%2B` in a query string (a literal `+`
            // decodes to a space), so a name that contains one has exactly one
            // correct URL spelling. The CLI path percent-encodes it for free.
            let encoded = name.replace('+', "%2B");
            assert_eq!(
                parse_url(&format!("floter://register?cmd={encoded}")),
                Ok(Trigger::Register(CommandRequest {
                    command: name.to_string(),
                    args: None,
                    confirmed: false,
                    origin: RegisterOrigin::Link,
                })),
                "{name} is a command name"
            );
        }
    }

    #[test]
    fn register_refuses_an_argument_that_reads_as_an_option() {
        // `--hidden` starts with a dash: an argument that a tool would read as
        // an option is exactly what this parameter must not carry, so it is
        // refused rather than forwarded.
        assert!(matches!(
            parse_url("floter://register?cmd=rg&args=--hidden%20src"),
            Err(Reject::InvalidArguments(_))
        ));
    }

    #[test]
    fn register_passes_inert_arguments_through_untouched() {
        assert_eq!(
            parse_url("floter://register?cmd=rg&args=src%20lib"),
            Ok(Trigger::Register(CommandRequest {
                command: "rg".to_string(),
                args: Some(vec!["src".to_string(), "lib".to_string()]),
                confirmed: false,
                origin: RegisterOrigin::Link,
            }))
        );
        // An empty `args` is the same as no `args`.
        assert_eq!(
            parse_url("floter://register?cmd=rg&args="),
            Ok(Trigger::Register(CommandRequest {
                command: "rg".to_string(),
                args: None,
                confirmed: false,
                origin: RegisterOrigin::Link,
            }))
        );
    }

    #[test]
    fn register_refuses_a_path_a_traversal_or_a_shell_string() {
        for refused in [
            "%2Fusr%2Fbin%2Frg",
            "..%2Frg",
            "rg%3Brm%20-rf%20%2F",
            "rg%20--exec%3Dls",
            "rg%0Ax",
            "-rg",
            "rg%24(x)",
            "rg%60x%60",
            "",
        ] {
            let parsed = parse_url(&format!("floter://register?cmd={refused}"));
            assert!(parsed.is_err(), "cmd={refused} must be refused");
        }
        // A missing `cmd` is its own refusal, distinct from an empty one.
        assert_eq!(parse_url("floter://register"), Err(Reject::MissingCommand));
        assert_eq!(
            parse_url("floter://register?cmd="),
            Err(Reject::MissingCommand)
        );
    }

    #[test]
    fn register_refuses_an_over_long_name() {
        let long = "a".repeat(MAX_COMMAND_CHARS + 1);
        assert!(matches!(
            parse_url(&format!("floter://register?cmd={long}")),
            Err(Reject::CommandTooLong(_))
        ));
        let allowed = "a".repeat(MAX_COMMAND_CHARS);
        assert!(parse_url(&format!("floter://register?cmd={allowed}")).is_ok());
    }

    #[test]
    fn register_refuses_a_shell_metacharacter_in_args() {
        for refused in [
            "args=src%3Brm",
            "args=%7Ccat",
            "args=%24HOME",
            "args=*",
            "args=~%2Fetc",
            "args=%22quoted%22",
        ] {
            let parsed = parse_url(&format!("floter://register?cmd=rg&{refused}"));
            assert!(parsed.is_err(), "{refused} must be refused");
        }
    }

    #[test]
    fn register_refuses_an_unknown_parameter_and_keeps_the_action_silent_rule() {
        assert!(matches!(
            parse_url("floter://register?cmd=rg&manifest=/a/tool.json"),
            Err(Reject::UnknownParameter(_))
        ));
        // The new action is *known*, so its refusals are noisy: a user who
        // clicked a register link must be told when nothing happened.
        let rejected = parse_url("floter://register").expect_err("missing cmd");
        assert!(!rejected.is_silent());
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
        // `floter register <cmd> [args…]` normalizes into the same URL the
        // scheme spells, so the CLI has no second parser (red line 4).
        let register =
            canonical_argument(&args(&["floter", "register", "rg"])).expect("register normalizes");
        assert_eq!(
            parse_url(&register),
            Ok(Trigger::Register(CommandRequest {
                command: "rg".to_string(),
                args: None,
                confirmed: false,
                origin: RegisterOrigin::Link,
            }))
        );
        let with_args = canonical_argument(&args(&["floter", "register", "rg", "src", "lib"]))
            .expect("register normalizes with trailing words");
        assert_eq!(
            parse_url(&with_args),
            Ok(Trigger::Register(CommandRequest {
                command: "rg".to_string(),
                args: Some(vec!["src".to_string(), "lib".to_string()]),
                confirmed: false,
                origin: RegisterOrigin::Link,
            }))
        );
        // And the refusal still happens in the router, not in the normalizer:
        // a path smuggled through the CLI reaches `parse_url` and dies there.
        let bad_register =
            canonical_argument(&args(&["floter", "register", "/usr/bin/rg"])).expect("normalizes");
        assert!(matches!(
            parse_url(&bad_register),
            Err(Reject::CommandNotBasename(_))
        ));
        // A bare `floter register` still normalizes (the action is known) and
        // is refused by the router, so the user gets the one toast rather than
        // a silently dropped invocation.
        let bare = canonical_argument(&args(&["floter", "register"])).expect("normalizes");
        assert_eq!(bare, "floter://register");
        assert_eq!(parse_url(&bare), Err(Reject::MissingCommand));
        assert!(!Reject::MissingCommand.is_silent());
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

    // ── register never writes a binding (the hard red line) ──────────────

    /// The red line of this round, asserted against the real files: resolving
    /// a `floter://register` command reads the discovery inventory and stops.
    /// No `tool-lock.json` byte changes and no repository entry is created —
    /// the lock is written only by the user's explicit Connect
    /// (`extensions_connect_tool`), which a link never reaches.
    ///
    /// Mutation: call `install::connect_tool` (or `ToolLock::bind` +
    /// `save`) from [`resolve_registered_command`] and this goes red.
    #[cfg(unix)]
    #[test]
    fn a_register_resolution_never_writes_a_binding() {
        use std::os::unix::fs::PermissionsExt;

        let directory = tempfile::tempdir().expect("temp dir");
        let root = directory.path().join("config");
        let state = crate::extensions::ExtensionState::from_paths(
            crate::extensions::ExtensionPaths::from_root(root.clone()),
        )
        .expect("extension state");

        // A real executable, so the candidate the resolver returns is
        // `available` exactly as a PATH discovery would be.
        let bin = tempfile::tempdir().expect("bin dir");
        let executable = bin.path().join("register-probe");
        std::fs::write(&executable, "#!/bin/sh\nexit 0\n").expect("write");
        std::fs::set_permissions(&executable, std::fs::Permissions::from_mode(0o755))
            .expect("chmod");
        state
            .tool_inventory
            .lock()
            .expect("inventory")
            .set_candidates_for_test(vec![crate::extensions::inventory::executable_candidate(
                &executable,
                "register-probe",
            )]);

        // A pre-existing binding, so "unchanged" is a real comparison and not
        // merely "the file is still absent".
        let tool_lock_path = root.join("tool-lock.json");
        let before = br#"{
  "schemaVersion": 1,
  "tools": {
    "local.existing": {
      "tool": "local.existing",
      "locator": { "kind": "executable", "path": "/usr/bin/true" },
      "fingerprint": null,
      "lockedAt": 1,
      "state": "connected"
    }
  }
}
"#;
        std::fs::write(&tool_lock_path, before).expect("seed lock");
        let repository_path = root.join("extension-repository.json");

        let resolved = resolve_registered_command(
            &state,
            &CommandRequest {
                command: "register-probe".to_string(),
                args: Some(vec!["src".to_string()]),
                confirmed: false,
                origin: RegisterOrigin::Link,
            },
        );
        // The resolve succeeded — this is the success path, not a refusal that
        // trivially wrote nothing.
        assert!(resolved.candidate.is_some(), "the probe must resolve");
        assert_eq!(resolved.args.as_deref(), Some(&["src".to_string()][..]));

        // The lock is byte-for-byte what it was, and no repository entry was
        // created by the resolution.
        assert_eq!(
            std::fs::read(&tool_lock_path).expect("read lock"),
            before.to_vec(),
            "a register resolution must not touch the tool lock"
        );
        assert!(
            !repository_path.exists(),
            "a register resolution must not create an extension entry"
        );
    }

    /// A name this device does not have resolves to `candidate: None` rather
    /// than an error: the frontend needs a reason to render inline, and the
    /// resolve path itself must stay infallible (best-effort contract).
    #[cfg(unix)]
    #[test]
    fn a_register_for_an_unknown_command_resolves_to_no_candidate() {
        let directory = tempfile::tempdir().expect("temp dir");
        let state = crate::extensions::ExtensionState::from_paths(
            crate::extensions::ExtensionPaths::from_root(directory.path().to_path_buf()),
        )
        .expect("extension state");
        state
            .tool_inventory
            .lock()
            .expect("inventory")
            .set_candidates_for_test(Vec::new());
        let resolved = resolve_registered_command(
            &state,
            &CommandRequest {
                command: "definitely-not-a-real-tool-9d2f".to_string(),
                args: None,
                confirmed: false,
                origin: RegisterOrigin::Link,
            },
        );
        assert!(resolved.candidate.is_none());
        assert_eq!(resolved.command, "definitely-not-a-real-tool-9d2f");
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

    // ── R8-4 · `floter register <cmd>`: the terminal spelling ────────────

    fn terminal_args(list: &[&str]) -> Vec<String> {
        list.iter().map(|value| (*value).to_string()).collect()
    }

    /// The CLI folds into the *same* URL the scheme spells — there is no second
    /// parser — and `--yes` is the one thing it adds, as a query parameter the
    /// router validates like any other.
    #[test]
    fn the_register_cli_normalizes_into_the_same_url() {
        for spelling in [
            &["floter", "register", "rg", "--yes"][..],
            &["floter", "--yes", "register", "rg"][..],
        ] {
            let url = canonical_argument(&terminal_args(spelling)).expect("normalizes");
            assert_eq!(url, "floter://register?cmd=rg&confirm=1", "{spelling:?}");
            // And the router is what reads it, so the parameter is validated in
            // one place rather than parsed twice.
            match parse_url(&url) {
                Ok(Trigger::Register(request)) => {
                    assert_eq!(request.command, "rg");
                    assert!(request.confirmed, "--yes must survive normalization");
                    assert_eq!(request.origin, RegisterOrigin::Link, "a URL is a URL");
                }
                other => panic!("{spelling:?} produced {other:?}"),
            }
        }
        // Without the flag, the same words produce the same URL R8-3 already
        // accepted — the flag is additive, not a second shape.
        assert_eq!(
            canonical_argument(&terminal_args(&["floter", "register", "rg"])),
            Some("floter://register?cmd=rg".to_string())
        );
        // Trailing words still become the inert `args` hint, and `--yes` does
        // not leak into it.
        assert_eq!(
            canonical_argument(&terminal_args(&[
                "floter", "register", "rg", "src", "--yes"
            ])),
            Some("floter://register?cmd=rg&args=src&confirm=1".to_string())
        );
    }

    /// An unknown subcommand is not a trigger at all: the normalizer returns
    /// `None`, the process falls through to the ordinary launch, and nothing is
    /// ever resolved. The report's "unknown command" case.
    #[test]
    fn an_unknown_subcommand_normalizes_to_nothing() {
        assert_eq!(
            canonical_argument(&terminal_args(&["floter", "registr", "rg"])),
            None
        );
        assert_eq!(
            canonical_argument(&terminal_args(&["floter", "install", "rg"])),
            None
        );
        assert_eq!(canonical_argument(&terminal_args(&["floter"])), None);
    }

    /// The transport, not the URL, is what a bind depends on. This is the whole
    /// reason [`parse_terminal_url`] exists.
    #[test]
    fn only_the_terminal_transport_can_confirm_a_register() {
        let link = |url: &str| match parse_url(url) {
            Ok(Trigger::Register(request)) => request,
            other => panic!("{url} produced {other:?}"),
        };
        let terminal = |url: &str| match parse_terminal_url(url) {
            Ok(Trigger::Register(request)) => request,
            other => panic!("{url} produced {other:?}"),
        };

        // A link — even one spelling `confirm=1` by hand — may not bind.
        for url in [
            "floter://register?cmd=rg",
            "floter://register?cmd=rg&confirm=1",
            "floter://register?cmd=notacuratedtool",
        ] {
            assert_eq!(link(url).origin, RegisterOrigin::Link, "{url}");
            assert!(
                !register_may_bind(&link(url)),
                "a URL must never be allowed to bind ({url})"
            );
        }
        // The terminal transport may, when the user already said yes.
        assert!(register_may_bind(&terminal("floter://register?cmd=rg")));
        assert!(register_may_bind(&terminal(
            "floter://register?cmd=notacuratedtool&confirm=1"
        )));
        // …and not otherwise.
        assert!(!register_may_bind(&terminal(
            "floter://register?cmd=notacuratedtool"
        )));
        // `open` and `connect` are unchanged by the transport upgrade.
        assert_eq!(
            parse_terminal_url("floter://open"),
            parse_url("floter://open")
        );
        assert_eq!(
            parse_terminal_url("floter://connect?manifest=/opt/tool.json"),
            parse_url("floter://connect?manifest=/opt/tool.json")
        );
        // A refusal is the router's, not a second one.
        assert_eq!(
            parse_terminal_url("floter://register?cmd=%2Fusr%2Fbin%2Frg"),
            Err(Reject::CommandNotBasename("/usr/bin/rg".to_string()))
        );
    }

    /// The gate reads the *one* curated allow-list, with the same spelling rule
    /// the discovery ranking uses (`rg.exe` is `rg`).
    #[test]
    fn the_bind_gate_reads_the_one_curated_allow_list() {
        let request = |command: &str, confirmed: bool| CommandRequest {
            command: command.to_string(),
            args: None,
            confirmed,
            origin: RegisterOrigin::Terminal,
        };
        for curated in ["rg", "git", "docker", "rg.exe", "RG"] {
            assert!(
                register_may_bind(&request(curated, false)),
                "{curated} is on the allow-list"
            );
        }
        for uncurated in ["my-own-tool", "register-probe", "definitely-not-curated"] {
            assert!(
                !register_may_bind(&request(uncurated, false)),
                "{uncurated}"
            );
            assert!(
                register_may_bind(&request(uncurated, true)),
                "--yes is the user's own confirmation"
            );
        }
        // The list itself is the single source: the gate and the ranking answer
        // the same question about the same name.
        assert!(crate::extensions::curated_tools::is_curated_command(
            "rg.exe"
        ));
        assert!(!crate::extensions::curated_tools::is_curated_command(
            "rg-extra"
        ));
    }

    /// A register that is *not* allowed to bind leaves the lock byte-for-byte
    /// alone — including a curated name that simply is not on this device.
    ///
    /// Mutation: drop the [`register_may_bind`] check (or move it after the
    /// connect) and the non-curated case goes red; bind without resolving first
    /// and the missing-tool case goes red.
    #[cfg(unix)]
    #[test]
    fn a_register_that_may_not_bind_writes_nothing() {
        let fixture = register_fixture(&["my-own-tool"]);
        let before = fixture.lock_bytes();

        for (command, confirmed) in [
            // Not curated, and no `--yes`: the gate is closed.
            ("my-own-tool", false),
            // The gate is open, but this device has no such executable. A
            // `--yes` is a confirmation, not an installation.
            ("definitely-not-here-9d2f", true),
        ] {
            let resolved = connect_registered_command(
                &fixture.state,
                &CommandRequest {
                    command: command.to_string(),
                    args: None,
                    confirmed,
                    origin: RegisterOrigin::Terminal,
                },
            );
            assert!(
                resolved.bound.is_none(),
                "{command} (--yes={confirmed}) must not bind"
            );
            assert!(
                resolved.bind_error.is_none(),
                "nothing was attempted, so nothing failed"
            );
            assert_eq!(
                fixture.lock_bytes(),
                before,
                "{command} (--yes={confirmed}) must leave the lock untouched"
            );
        }
        assert!(!fixture.repository_path().exists());
    }

    /// A candidate whose executable disappeared is not connectable, and `--yes`
    /// does not change that. The resolver's own availability filter refuses it
    /// (so the plan refuses the command), and the belt-and-braces
    /// `.filter(|found| found.available)` in [`connect_registered_command`]
    /// keeps the connect pipeline from ever being handed a dead path if that
    /// filter is ever loosened.
    ///
    /// Mutation: relax `resolve_filtered`'s `candidate.available` filter *and*
    /// drop the `.filter(|found| found.available)` guard and this goes red —
    /// the connect pipeline would be handed a path that no longer exists.
    #[cfg(unix)]
    #[test]
    fn a_vanished_executable_is_never_handed_to_the_connect_path() {
        let fixture = register_fixture(&["my-own-tool"]);
        std::fs::remove_file(&fixture.executable).expect("remove the executable");
        // Re-inspect the path *after* the removal, so the inventory reports the
        // candidate as unavailable — the shape a fresh scan produces once the
        // file is gone.
        let unavailable =
            crate::extensions::inventory::executable_candidate(&fixture.executable, "my-own-tool");
        assert!(
            !unavailable.available,
            "the probe must see the file is gone"
        );
        fixture
            .state
            .tool_inventory
            .lock()
            .expect("inventory")
            .set_candidates_for_test(vec![unavailable]);

        let resolved = connect_registered_command(
            &fixture.state,
            &CommandRequest {
                command: "my-own-tool".to_string(),
                args: None,
                confirmed: true,
                origin: RegisterOrigin::Terminal,
            },
        );
        assert!(
            resolved.candidate.is_none(),
            "an unavailable candidate is not resolved"
        );
        assert!(resolved.bound.is_none(), "so it cannot bind");
        assert!(resolved.bind_error.is_none(), "nothing was attempted");
        assert!(!fixture.repository_path().exists());
        assert!(!fixture.lock_path().exists());

        // And the CLI refuses it out loud rather than reporting a connection.
        let args = ["floter", "register", "my-own-tool", "--yes"]
            .iter()
            .map(|value| (*value).to_string())
            .collect::<Vec<_>>();
        match plan_register_cli(&fixture.state, &args) {
            Some(RegisterCliPlan::Refused { message }) => {
                assert!(message.contains("my-own-tool"), "{message}");
                assert!(message.contains("no executable"), "{message}");
            }
            other => panic!("expected a refusal, got {other:?}"),
        }
    }

    /// The success path, through the real pipeline: a curated name resolves and
    /// the lock gains exactly the entry `install::connect_tool` writes, with the
    /// fixed disclosure set.
    #[cfg(unix)]
    #[test]
    fn a_curated_terminal_register_binds_through_the_connect_path() {
        let fixture = register_fixture(&["git"]);
        assert!(!fixture.lock_path().exists(), "nothing is bound yet");

        let resolved = connect_registered_command(
            &fixture.state,
            &CommandRequest {
                command: "git".to_string(),
                args: None,
                confirmed: false,
                origin: RegisterOrigin::Terminal,
            },
        );

        let entry = resolved.bound.expect("a curated tool binds");
        // R9-3 · the identity is minted at creation, not derived from the
        // registered command. The *command* still tracks the tool. Shape:
        // `^local\.[0-9a-f]{8,}$`.
        let bound_id = entry.id.clone();
        let suffix = bound_id.strip_prefix("local.").unwrap_or("");
        assert!(
            suffix.len() >= 8 && suffix.bytes().all(|byte| byte.is_ascii_hexdigit()),
            "the minted id is `local.<8+ hex>`: {bound_id}"
        );
        assert_eq!(
            entry.approved_permissions,
            crate::extensions::install::tool_binding_permissions()
                .into_iter()
                .collect::<std::collections::BTreeSet<_>>()
                .into_iter()
                .collect::<Vec<_>>(),
            "the three-permission disclosure set, fixed"
        );
        // The write is the ordinary one: the repository entry is on disk, and
        // the first binding it implies lands in `tool-lock.json` through the
        // same catalog load any other connection goes through — not through a
        // private write this module invented.
        assert!(fixture.repository_path().exists());
        tauri::async_runtime::block_on(
            crate::extensions::catalog::load_provider_commands_uncached(&fixture.state),
        )
        .expect("the catalog load succeeds");
        let stored = crate::extensions::ToolLock::load(&fixture.lock_path())
            .expect("the lock is readable")
            .tools
            .get(&bound_id)
            .cloned()
            .expect("the entry is in the lock");
        assert_eq!(stored.tool, bound_id);
        assert_eq!(
            stored.locator.executable_path().map(Path::to_path_buf),
            Some(fixture.executable.clone())
        );
    }

    /// `--yes` on a name that is not on the allow-list takes the same route: the
    /// user's confirmation substitutes for curation, and nothing else changes.
    #[cfg(unix)]
    #[test]
    fn an_explicit_yes_binds_an_uncurated_name_through_the_same_path() {
        let fixture = register_fixture(&["my-own-tool"]);

        let resolved = connect_registered_command(
            &fixture.state,
            &CommandRequest {
                command: "my-own-tool".to_string(),
                args: None,
                confirmed: true,
                origin: RegisterOrigin::Terminal,
            },
        );

        let entry = resolved.bound.expect("--yes binds");
        let bound_id = entry.id.clone();
        assert!(
            bound_id.starts_with("local.") && bound_id != "local.my-own-tool",
            "the id is minted, not derived from the command: {bound_id}"
        );
        assert!(fixture.repository_path().exists());
        tauri::async_runtime::block_on(
            crate::extensions::catalog::load_provider_commands_uncached(&fixture.state),
        )
        .expect("the catalog load succeeds");
        let lock = crate::extensions::ToolLock::load(&fixture.lock_path()).expect("the lock");
        assert!(lock.tools.contains_key(&bound_id));
    }

    /// A **link** for the very same curated name still stops at the offer: the
    /// transport is the difference, and this is the mutation that would erase
    /// it.
    #[cfg(unix)]
    #[test]
    fn the_same_curated_name_over_a_link_still_does_not_bind() {
        let fixture = register_fixture(&["git"]);

        let resolved = connect_registered_command(
            &fixture.state,
            &CommandRequest {
                command: "git".to_string(),
                args: None,
                confirmed: false,
                origin: RegisterOrigin::Link,
            },
        );

        assert!(resolved.candidate.is_some(), "it is still resolved");
        assert!(resolved.bound.is_none(), "but a link never binds");
        assert!(!fixture.lock_path().exists());
    }

    /// The disclosure is built from the one permission function, so a sentence
    /// the user reads cannot disagree with the record the lock stores.
    ///
    /// Mutation: delete the sentence (or hard-code a shorter one) and the
    /// permission names below stop matching.
    #[test]
    fn the_disclosure_names_every_permission_the_binding_gets() {
        let disclosure = crate::extensions::install::tool_binding_disclosure();
        assert!(disclosure.starts_with("permissions: "), "{disclosure}");
        for label in ["environment", "process-spawn", "filesystem-read"] {
            assert!(disclosure.contains(label), "{disclosure} must name {label}");
        }
        assert_eq!(
            disclosure.matches(',').count() + 1,
            crate::extensions::install::tool_binding_permissions().len(),
            "one label per permission, no more"
        );
    }

    /// The CLI decision itself, with no printing and no writing: a curated hit
    /// is a bind, an uncurated name without `--yes` is an offer, and a name that
    /// is not on the device is a refusal with a non-zero exit code.
    #[cfg(unix)]
    #[test]
    fn the_cli_plan_decides_bind_offer_or_refuse() {
        let fixture = register_fixture(&["git", "my-own-tool"]);
        let args = |list: &[&str]| {
            list.iter()
                .map(|value| (*value).to_string())
                .collect::<Vec<_>>()
        };

        // A curated name on this device: the terminal path may bind.
        match plan_register_cli(&fixture.state, &args(&["floter", "register", "git"])) {
            Some(RegisterCliPlan::Bind {
                request,
                url,
                disclosure,
            }) => {
                assert_eq!(request.command, "git");
                assert_eq!(url, "floter://register?cmd=git");
                assert!(disclosure.contains("permissions: "), "{disclosure}");
                assert!(disclosure.contains("environment"), "{disclosure}");
            }
            other => panic!("expected a bind, got {other:?}"),
        }

        // Not curated, no `--yes`: an offer, and the offer is a *link* (the
        // `confirm` parameter is dropped, so the review surface cannot bind).
        match plan_register_cli(
            &fixture.state,
            &args(&["floter", "register", "my-own-tool"]),
        ) {
            Some(RegisterCliPlan::Offer { url, message }) => {
                assert_eq!(url, "floter://register?cmd=my-own-tool");
                assert!(message.contains("--yes"), "{message} must say how to bind");
                assert!(!url.contains("confirm"), "the offer must not carry a bind");
            }
            other => panic!("expected an offer, got {other:?}"),
        }

        // `--yes` turns the same name into a bind, and the URL records the
        // confirmation for the instance that receives it.
        match plan_register_cli(
            &fixture.state,
            &args(&["floter", "register", "my-own-tool", "--yes"]),
        ) {
            Some(RegisterCliPlan::Bind { url, .. }) => {
                assert_eq!(url, "floter://register?cmd=my-own-tool&confirm=1");
            }
            other => panic!("expected a bind, got {other:?}"),
        }

        // A curated name this device does not have: refused, non-zero, and the
        // sentence names the command. Nothing falls back to a window.
        match plan_register_cli(&fixture.state, &args(&["floter", "register", "hg"])) {
            Some(RegisterCliPlan::Refused { message }) => {
                assert!(message.contains("hg"), "{message}");
                assert_eq!(
                    plan_register_cli(&fixture.state, &args(&["floter", "register", "hg"]))
                        .expect("a refusal")
                        .exit_code(),
                    1
                );
            }
            other => panic!("expected a refusal, got {other:?}"),
        }
        // A malformed name is refused by the router, and the message carries
        // the router's own reason.
        match plan_register_cli(
            &fixture.state,
            &args(&["floter", "register", "/usr/bin/rg"]),
        ) {
            Some(RegisterCliPlan::Refused { message }) => {
                assert!(message.contains("not a bare command name"), "{message}");
            }
            other => panic!("expected a refusal, got {other:?}"),
        }
        // A bare `register` has no command to resolve.
        assert!(matches!(
            plan_register_cli(&fixture.state, &args(&["floter", "register"])),
            Some(RegisterCliPlan::Refused { .. })
        ));
        // Not a register invocation at all: the caller falls through to the GUI.
        assert!(plan_register_cli(&fixture.state, &args(&["floter", "--toggle"])).is_none());
        assert!(plan_register_cli(&fixture.state, &args(&["floter"])).is_none());
    }

    /// Planning never writes: the lock is untouched by every branch, including
    /// the one that decides to bind. The write belongs to the execution step.
    ///
    /// Mutation: move the `connect_tool` call into `plan_register_cli` and this
    /// goes red.
    #[cfg(unix)]
    #[test]
    fn planning_a_register_never_writes_anything() {
        let fixture = register_fixture(&["git"]);
        let args = |list: &[&str]| {
            list.iter()
                .map(|value| (*value).to_string())
                .collect::<Vec<_>>()
        };
        for spelling in [
            &["floter", "register", "git"][..],
            &["floter", "register", "git", "--yes"][..],
            &["floter", "register", "nothing-here"][..],
        ] {
            let _ = plan_register_cli(&fixture.state, &args(spelling));
            assert!(!fixture.lock_path().exists(), "{spelling:?}");
            assert!(!fixture.repository_path().exists(), "{spelling:?}");
        }
    }

    /// A scratch home with one fake executable on its inventory, and the lock
    /// path the real pipeline would write.
    struct RegisterFixture {
        state: ExtensionState,
        executable: PathBuf,
        root: PathBuf,
        _bin: tempfile::TempDir,
        _home: tempfile::TempDir,
    }

    impl RegisterFixture {
        fn lock_path(&self) -> PathBuf {
            self.root.join("tool-lock.json")
        }

        fn repository_path(&self) -> PathBuf {
            self.root.join("extension-repository.json")
        }

        /// `None` while nothing has been written, so "unchanged" can be
        /// asserted as "still absent" on the zero-binding paths.
        fn lock_bytes(&self) -> Option<Vec<u8>> {
            std::fs::read(self.lock_path()).ok()
        }
    }

    fn register_fixture(names: &[&str]) -> RegisterFixture {
        use std::os::unix::fs::PermissionsExt;

        let home = tempfile::tempdir().expect("home");
        let root = home.path().join("config");
        let state =
            ExtensionState::from_paths(crate::extensions::ExtensionPaths::from_root(root.clone()))
                .expect("extension state");
        let bin = tempfile::tempdir().expect("bin");
        let mut candidates = Vec::new();
        for name in names {
            let executable = bin.path().join(name);
            std::fs::write(&executable, "#!/bin/sh\nexit 0\n").expect("write");
            std::fs::set_permissions(&executable, std::fs::Permissions::from_mode(0o755))
                .expect("chmod");
            candidates.push(crate::extensions::inventory::executable_candidate(
                &executable,
                *name,
            ));
        }
        let executable = bin.path().join(names[0]);
        state
            .tool_inventory
            .lock()
            .expect("inventory")
            .set_candidates_for_test(candidates);
        RegisterFixture {
            state,
            executable,
            root,
            _bin: bin,
            _home: home,
        }
    }
}
