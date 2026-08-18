//! Terminal capability negotiation for the terminal Floter renders into.
//!
//! [`TerminalCapability`] describes what the *outer* terminal (the one a
//! session's output eventually lands in) can do: how rich its color support
//! is, and whether it implements the optional extension modes TUIs rely on.
//!
//! Two escape-sequence families drive [`TerminalCapability::negotiate`]:
//!
//! * **DA1** — Primary Device Attributes, `CSI c`. The terminal answers with
//!   `CSI ? params c`; a parameter of `22` reports ANSI color support. This is
//!   the only DA1 bit we act on.
//! * **DECRQM** — DEC Request Mode, `CSI ? Ps $ p` where `Ps` is the mode
//!   number. The terminal answers with `CSI ? Ps ; Pm $ y` where `Pm` is:
//!   0 = not recognized, 1 = set, 2 = reset, 3 = permanently set,
//!   4 = permanently reset. Following tmux's `tty-features` interpretation, a
//!   *recognized* mode (`Pm != 0`) means the terminal supports the feature;
//!   whether it happens to be toggled on right now is irrelevant to us.
//!
//! # What DA1/DECRQM cannot tell us
//!
//! * **Color depth.** DA1's `22` bit proves 8-color ANSI support at most;
//!   there is no DEC mode for 256-color or truecolor. [`TerminalCapability::conservative`]
//!   therefore starts from the bundled `floter-256color` terminfo promise
//!   (`Palette256`), and [`TerminalCapability::negotiate`] upgrades it to
//!   [`TerminalColor::Truecolor`] with one supplementary, non-mutating OSC 4
//!   palette query (`OSC 4 ; 0 ; ?`): every terminal that answers that query
//!   with an `rgb:` value exposes a 24-bit palette and accepts truecolor SGR.
//! * **Unicode.** No DA1/DECRQM bit reports UTF-8 support, so `unicode` is
//!   carried over unchanged from the baseline capability. For
//!   `floter-256color` the conservative assumption is UTF-8 (true).

use serde::{Deserialize, Serialize};
use std::time::{Duration, Instant};

/// How rich a terminal's color support is, ordered weakest to strongest.
#[derive(Debug, Clone, Copy, PartialEq, Eq, PartialOrd, Ord, Hash, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub enum TerminalColor {
    /// Monochrome: no ANSI colors at all.
    None,
    /// The 8/16-color ANSI palette.
    Palette8,
    /// The 256-color palette (`xterm-256color` style).
    Palette256,
    /// 24-bit RGB (truecolor) SGR.
    Truecolor,
}

/// DECRQM mode numbers probed by [`TerminalCapability::negotiate`].
pub mod modes {
    /// SGR mouse reporting (`CSI < ... M/m`).
    pub const SGR_MOUSE: u16 = 1006;
    /// Bracketed paste (`CSI ? 2004 h/l`).
    pub const BRACKETED_PASTE: u16 = 2004;
    /// Synchronized output / "pending update" (`CSI ? 2026 h/l`).
    pub const SYNCHRONIZED_OUTPUT: u16 = 2026;
    /// Enhanced keyboard protocol / modifyOtherKeys (`CSI ? 2048 h/l`).
    pub const KEYBOARD_PROTOCOL: u16 = 2048;
}

/// State reported by DECRQM for a queried mode.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub enum DecrqmState {
    /// Mode not recognized — the terminal has no idea it exists.
    NotRecognized,
    /// Mode recognized and currently set.
    Set,
    /// Mode recognized and currently reset.
    Reset,
    /// Mode recognized and permanently set (cannot be reset).
    PermanentlySet,
    /// Mode recognized and permanently reset (cannot be set).
    PermanentlyReset,
}

impl DecrqmState {
    /// Decode the `Pm` parameter of a `CSI ? Ps ; Pm $ y` response.
    pub fn from_u16(value: u16) -> Option<Self> {
        Some(match value {
            0 => Self::NotRecognized,
            1 => Self::Set,
            2 => Self::Reset,
            3 => Self::PermanentlySet,
            4 => Self::PermanentlyReset,
            _ => return None,
        })
    }

    /// Whether the mode is *supported* by the terminal. Any recognized state —
    /// including `Reset` — means the feature exists; only `NotRecognized`
    /// denies it.
    pub fn supported(self) -> bool {
        !matches!(self, Self::NotRecognized)
    }
}

/// Parsed DA1 (Primary Device Attributes) response.
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct Da1Report {
    /// Numeric parameters from `CSI ? params c`.
    pub params: Vec<u16>,
}

impl Da1Report {
    /// DA1 parameter reporting ANSI color support (the one we act on).
    pub const ANSI_COLOR_PARAM: u16 = 22;

    /// Whether the terminal reported ANSI color support (DA1 param 22).
    pub fn has_ansi_color(&self) -> bool {
        self.params.contains(&Self::ANSI_COLOR_PARAM)
    }
}

/// One DECRQM probe and its parsed outcome.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct DecrqmResult {
    /// The queried mode (see [`modes`]).
    pub mode: u16,
    /// Parsed state, or `None` when the terminal never answered.
    pub state: Option<DecrqmState>,
}

/// Everything a single negotiation pass observed.
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct ProbeReport {
    /// Parsed DA1 response; `None` when the terminal did not answer in time.
    pub da1: Option<Da1Report>,
    /// DECRQM answers, one entry per mode that answered. Modes that timed out
    /// are absent.
    pub decrqm: Vec<DecrqmResult>,
    /// Whether the OSC 4 color query received any reply.
    pub color_query_answered: bool,
    /// Whether the color query reply advertised an `rgb:` (24-bit) palette.
    pub truecolor_palette: bool,
}

impl ProbeReport {
    /// The state reported for `mode`, or `None` if that mode never answered.
    pub fn mode_state(&self, mode: u16) -> Option<DecrqmState> {
        self.decrqm
            .iter()
            .find(|result| result.mode == mode)
            .and_then(|result| result.state)
    }
}

/// Result of a negotiation: the resolved capability plus the raw report.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Negotiation {
    /// The capability derived from the baseline plus everything the probes
    /// measured.
    pub capability: TerminalCapability,
    /// Per-probe observations backing [`Negotiation::capability`].
    pub report: ProbeReport,
}

/// Byte transport for a probe session.
///
/// Implementations wrap a live PTY (or, in tests, a scripted fake). The two
/// callbacks borrow disjoint resources in production — one side sends to the
/// PTY master, the other drains its output.
type TerminalWriter<'a> = dyn FnMut(&[u8]) -> Result<(), String> + 'a;
type TerminalReader<'a> = dyn FnMut(&mut [u8], Duration) -> Result<usize, String> + 'a;

pub struct TerminalIo<'a> {
    /// Send `data` to the terminal. Errors abort the negotiation.
    pub write: Box<TerminalWriter<'a>>,
    /// Block up to `timeout` for terminal output; copy it into `buf` and
    /// return the byte count. `Ok(0)` means the timeout elapsed with no data
    /// (never an error — silence is how probes time out). Errors abort the
    /// negotiation.
    pub read: Box<TerminalReader<'a>>,
}

/// Decoded capabilities of a terminal.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct TerminalCapability {
    /// Color depth the terminal can reproduce.
    pub color: TerminalColor,
    /// UTF-8 support. No DA1/DECRQM probe exists for this, so it always comes
    /// from the baseline capability; [`TerminalCapability::conservative`]
    /// assumes UTF-8 for `floter-256color`.
    pub unicode: bool,
    /// SGR mouse reporting (mode 1006).
    pub mouse: bool,
    /// Bracketed paste (mode 2004).
    pub bracketed_paste: bool,
    /// Synchronized output (mode 2026).
    pub synchronized_output: bool,
    /// Enhanced keyboard protocol (mode 2048).
    pub keyboard_protocol: bool,
}

/// Default budget for one [`TerminalCapability::negotiate`] pass.
pub const DEFAULT_NEGOTIATION_BUDGET: Duration = Duration::from_millis(750);

/// DECRQM query prefix: `CSI ? Ps $ p`.
const DECRQM_QUERY_PREFIX: &str = "\x1b[?";
/// DECRQM query suffix: `$ p`.
const DECRQM_QUERY_SUFFIX: &str = "$p";
/// DA1 query: `CSI c`.
const DA1_QUERY: &[u8] = b"\x1b[c";
/// OSC 4 color query: `OSC 4 ; 0 ; ? BEL`. Asking for palette index 0 and
/// reading whatever format the answer comes back in is all the depth probe
/// needs; it never mutates the palette.
const OSC4_QUERY: &[u8] = b"\x1b]4;0;?\x07";
/// Modes probed in order.
const DECRQM_MODES: [u16; 4] = [
    modes::SGR_MOUSE,
    modes::BRACKETED_PASTE,
    modes::SYNCHRONIZED_OUTPUT,
    modes::KEYBOARD_PROTOCOL,
];
/// Silence longer than this between bytes ends a response.
const QUIET_PERIOD: Duration = Duration::from_millis(40);
/// Upper bound on one collected response, so a chatty stream cannot balloon it.
const MAX_RESPONSE_BYTES: usize = 4096;

impl TerminalCapability {
    /// Conservative baseline matching the bundled `floter-256color` terminfo
    /// entry: it promises 256 colors and UTF-8, but cannot vouch for any
    /// optional extension mode, so mouse, bracketed paste, synchronized output
    /// and the keyboard protocol all start off until a probe confirms them.
    ///
    /// Use this as the pre-negotiation default and as the fallback when
    /// [`negotiate`](Self::negotiate) times out.
    pub fn conservative() -> Self {
        Self {
            color: TerminalColor::Palette256,
            unicode: true,
            mouse: false,
            bracketed_paste: false,
            synchronized_output: false,
            keyboard_protocol: false,
        }
    }

    /// Probe the terminal connected through `io`, starting from `self` (the
    /// baseline, normally [`conservative`](Self::conservative)) and resolving
    /// a capability from whatever the probes measure.
    ///
    /// Probing is sequential and stops early on a silent terminal: DA1 first,
    /// then DECRQM for each mode, then the OSC 4 color-depth query. If DA1
    /// never answers, the remaining probes are skipped and `self` is returned
    /// unchanged — silence means "assume the baseline".
    ///
    /// [`DEFAULT_NEGOTIATION_BUDGET`] bounds the whole pass; use
    /// [`negotiate_with_budget`](Self::negotiate_with_budget) to tune it.
    pub fn negotiate(&self, io: &mut TerminalIo) -> Result<Negotiation, String> {
        self.negotiate_with_budget(io, DEFAULT_NEGOTIATION_BUDGET)
    }

    /// Like [`negotiate`](Self::negotiate), with an explicit total budget
    /// shared across all probes.
    pub fn negotiate_with_budget(
        &self,
        io: &mut TerminalIo,
        budget: Duration,
    ) -> Result<Negotiation, String> {
        let deadline = Instant::now() + budget;
        let mut report = ProbeReport::default();

        // Primary Device Attributes. A response at all means we are talking to
        // a terminal worth questioning; silence means the baseline stands.
        (io.write)(DA1_QUERY)?;
        if let Some(data) = read_response(io, deadline)? {
            report.da1 = parse_da1(&data);
        }
        let Some(da1) = &report.da1 else {
            return Ok(Negotiation {
                capability: *self,
                report,
            });
        };

        // DECRQM for each optional mode. A recognized mode (any state except
        // NotRecognized) counts as supported, regardless of its current state.
        for &mode in &DECRQM_MODES {
            if Instant::now() >= deadline {
                break;
            }
            let query = format!("{DECRQM_QUERY_PREFIX}{mode}{DECRQM_QUERY_SUFFIX}");
            (io.write)(query.as_bytes())?;
            if let Some(data) = read_response(io, deadline)? {
                let state = parse_decrqm(&data, mode);
                report.decrqm.push(DecrqmResult { mode, state });
            }
        }

        // Color-depth upgrade: DA1's 22 proves 8 colors at most. Ask for one
        // palette entry; an `rgb:` answer means a 24-bit palette, i.e. a
        // terminal that also accepts truecolor SGR.
        if da1.has_ansi_color() && Instant::now() < deadline {
            (io.write)(OSC4_QUERY)?;
            if let Some(data) = read_response(io, deadline)? {
                report.color_query_answered = true;
                report.truecolor_palette = data.windows(4).any(|window| window == b"rgb:");
            }
        }

        Ok(Negotiation {
            capability: self.apply(&report),
            report,
        })
    }

    /// Resolve a capability from the baseline (`self`) plus a probe report.
    /// Every field the probes measured overrides the baseline; fields they
    /// never measured (unicode, and color when DA1 stayed silent) keep it.
    fn apply(&self, report: &ProbeReport) -> TerminalCapability {
        let mut capability = *self;

        if let Some(da1) = &report.da1 {
            capability.color = if da1.has_ansi_color() {
                if report.truecolor_palette {
                    TerminalColor::Truecolor
                } else {
                    TerminalColor::Palette8
                }
            } else {
                TerminalColor::None
            };
        }
        if let Some(state) = report.mode_state(modes::SGR_MOUSE) {
            capability.mouse = state.supported();
        }
        if let Some(state) = report.mode_state(modes::BRACKETED_PASTE) {
            capability.bracketed_paste = state.supported();
        }
        if let Some(state) = report.mode_state(modes::SYNCHRONIZED_OUTPUT) {
            capability.synchronized_output = state.supported();
        }
        if let Some(state) = report.mode_state(modes::KEYBOARD_PROTOCOL) {
            capability.keyboard_protocol = state.supported();
        }
        capability
    }
}

/// Collect a terminal response with a quiet-period terminator.
///
/// Reads until either `deadline` passes, a quiet gap longer than
/// [`QUIET_PERIOD`] follows the last byte, or the response exceeds
/// [`MAX_RESPONSE_BYTES`]. Returns `None` when no byte ever arrived.
fn read_response(io: &mut TerminalIo, deadline: Instant) -> Result<Option<Vec<u8>>, String> {
    let mut buf = [0u8; 512];
    let mut response = Vec::new();
    loop {
        let now = Instant::now();
        if now >= deadline {
            return Ok((!response.is_empty()).then_some(response));
        }
        let wait = (deadline - now).min(QUIET_PERIOD);
        let n = (io.read)(&mut buf, wait)?;
        if n == 0 {
            // Quiet period after at least one byte: the response is complete.
            if !response.is_empty() {
                return Ok(Some(response));
            }
            continue;
        }
        response.extend_from_slice(&buf[..n]);
        if response.len() >= MAX_RESPONSE_BYTES {
            return Ok(Some(response));
        }
    }
}

/// Locate a CSI introducer (`ESC [`) in `data`.
fn find_csi(data: &[u8]) -> Option<usize> {
    data.windows(2).position(|window| window == [0x1b, b'['])
}

/// Parse `CSI ? params c` into its numeric parameters. The optional `?` is
/// tolerated either way, and the terminator accepts both cases for robustness.
fn parse_da1(data: &[u8]) -> Option<Da1Report> {
    let csi = find_csi(data)?;
    let mut rest = &data[csi + 2..];
    if rest.first() == Some(&b'?') {
        rest = &rest[1..];
    }
    let end = rest.iter().position(|&byte| byte == b'c' || byte == b'C')?;
    Some(Da1Report {
        params: parse_params(&rest[..end])?,
    })
}

/// Parse `CSI ? Ps ; Pm $ y` for `mode`, returning the reported state.
///
/// Returns `None` when the response does not answer `mode` at all (no CSI
/// sequence, a different mode echoed back, an unknown state value, or a
/// terminator other than `$ y`).
fn parse_decrqm(data: &[u8], mode: u16) -> Option<DecrqmState> {
    let csi = find_csi(data)?;
    let mut rest = &data[csi + 2..];
    if rest.first() == Some(&b'?') {
        rest = &rest[1..];
    }
    let dollar = rest.iter().position(|&byte| byte == b'$')?;
    let head = &rest[..dollar];
    let tail = &rest[dollar + 1..];
    if tail.first() != Some(&b'y') && tail.first() != Some(&b'Y') {
        return None;
    }
    let mut parts = head.split(|&byte| byte == b';');
    let answered_mode = std::str::from_utf8(parts.next()?)
        .ok()?
        .parse::<u16>()
        .ok()?;
    if answered_mode != mode {
        return None;
    }
    let state_value = std::str::from_utf8(parts.next()?)
        .ok()?
        .parse::<u16>()
        .ok()?;
    DecrqmState::from_u16(state_value)
}

/// Parse a `;`-separated list of decimal parameters.
fn parse_params(slice: &[u8]) -> Option<Vec<u16>> {
    if slice.is_empty() {
        return Some(Vec::new());
    }
    slice
        .split(|&byte| byte == b';')
        .map(|part| std::str::from_utf8(part).ok()?.parse::<u16>().ok())
        .collect()
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::cell::RefCell;
    use std::collections::{HashMap, VecDeque};

    /// Scripted stand-in for a terminal. The script maps each exact request
    /// byte sequence to the response chunks the fake emits — one chunk per
    /// `read` call, then silence (which is how `read_response` knows the
    /// response is complete). Requests absent from the script are answered
    /// with silence, like a terminal that does not know the query.
    ///
    /// `pending`/`served` use interior mutability so the two [`TerminalIo`]
    /// closures can share the fake behind an ordinary shared reference.
    struct FakeTerminal {
        script: HashMap<Vec<u8>, Vec<Vec<u8>>>,
        pending: RefCell<VecDeque<Vec<u8>>>,
        served: RefCell<Vec<Vec<u8>>>,
        fail_write: bool,
        fail_read: bool,
    }

    impl FakeTerminal {
        fn new(script: &[(&[u8], &[&[u8]])]) -> Self {
            let mut map = HashMap::new();
            for (request, chunks) in script {
                map.insert(
                    request.to_vec(),
                    chunks.iter().map(|chunk| chunk.to_vec()).collect(),
                );
            }
            Self {
                script: map,
                pending: RefCell::new(VecDeque::new()),
                served: RefCell::new(Vec::new()),
                fail_write: false,
                fail_read: false,
            }
        }

        fn write(&self, data: &[u8]) -> Result<(), String> {
            if self.fail_write {
                return Err("write failed".to_string());
            }
            if let Some(chunks) = self.script.get(data) {
                self.pending.borrow_mut().extend(chunks.iter().cloned());
            }
            self.served.borrow_mut().push(data.to_vec());
            Ok(())
        }

        fn read(&self, buf: &mut [u8], _timeout: Duration) -> Result<usize, String> {
            if self.fail_read {
                return Err("read failed".to_string());
            }
            match self.pending.borrow_mut().pop_front() {
                Some(chunk) => {
                    let n = chunk.len().min(buf.len());
                    buf[..n].copy_from_slice(&chunk[..n]);
                    Ok(n)
                }
                None => Ok(0),
            }
        }

        /// Wrap the fake in a [`TerminalIo`] sharing this fake by reference.
        fn io(&self) -> TerminalIo<'_> {
            TerminalIo {
                write: Box::new(move |data: &[u8]| self.write(data)),
                read: Box::new(move |buf: &mut [u8], timeout: Duration| self.read(buf, timeout)),
            }
        }
    }

    #[test]
    fn conservative_defaults_match_floter_256color_promise() {
        let capability = TerminalCapability::conservative();
        assert_eq!(capability.color, TerminalColor::Palette256);
        assert!(capability.unicode);
        assert!(!capability.mouse);
        assert!(!capability.bracketed_paste);
        assert!(!capability.synchronized_output);
        assert!(!capability.keyboard_protocol);
    }

    #[test]
    fn color_levels_are_ordered_weakest_to_strongest() {
        assert!(TerminalColor::None < TerminalColor::Palette8);
        assert!(TerminalColor::Palette8 < TerminalColor::Palette256);
        assert!(TerminalColor::Palette256 < TerminalColor::Truecolor);
        assert_eq!(
            TerminalColor::Truecolor.max(TerminalColor::None),
            TerminalColor::Truecolor
        );
    }

    #[test]
    fn capability_round_trips_through_json() {
        let capability = TerminalCapability::conservative();
        let json = serde_json::to_string(&capability).unwrap();
        let back: TerminalCapability = serde_json::from_str(&json).unwrap();
        assert_eq!(back, capability);
    }

    #[test]
    fn decrqm_state_semantics() {
        assert!(DecrqmState::Set.supported());
        assert!(DecrqmState::Reset.supported());
        assert!(DecrqmState::PermanentlySet.supported());
        assert!(DecrqmState::PermanentlyReset.supported());
        assert!(!DecrqmState::NotRecognized.supported());
        assert_eq!(DecrqmState::from_u16(0), Some(DecrqmState::NotRecognized));
        assert_eq!(
            DecrqmState::from_u16(4),
            Some(DecrqmState::PermanentlyReset)
        );
        assert_eq!(DecrqmState::from_u16(5), None);
    }

    #[test]
    fn parse_da1_extracts_params_and_color_bit() {
        let report = parse_da1(b"\x1b[?1;2;22;24;28c").unwrap();
        assert_eq!(report.params, vec![1, 2, 22, 24, 28]);
        assert!(report.has_ansi_color());
    }

    #[test]
    fn parse_da1_tolerates_missing_question_mark_and_uppercase() {
        assert_eq!(parse_da1(b"\x1b[1;2c").unwrap().params, vec![1, 2]);
        assert_eq!(parse_da1(b"\x1b[?1;2C").unwrap().params, vec![1, 2]);
    }

    #[test]
    fn parse_da1_rejects_garbage_and_foreign_sequences() {
        assert_eq!(parse_da1(b"hello"), None);
        assert_eq!(parse_da1(b"\x1b]4;0;rgb:0000/0000/0000\x07"), None);
        // An ANSI mode response (no `?`) with a non-DA terminator is not DA1.
        assert_eq!(parse_da1(b"\x1b[1006;1$y"), None);
    }

    #[test]
    fn parse_decrqm_decodes_all_states() {
        assert_eq!(
            parse_decrqm(b"\x1b[?1006;1$y", 1006),
            Some(DecrqmState::Set)
        );
        assert_eq!(
            parse_decrqm(b"\x1b[?1006;0$y", 1006),
            Some(DecrqmState::NotRecognized)
        );
        assert_eq!(
            parse_decrqm(b"\x1b[?2004;4$Y", 2004),
            Some(DecrqmState::PermanentlyReset)
        );
    }

    #[test]
    fn parse_decrqm_ignores_responses_for_other_modes() {
        assert_eq!(parse_decrqm(b"\x1b[?1006;1$y", 2004), None);
        assert_eq!(
            parse_decrqm(b"\x1b[?1006;1$y", 1006),
            Some(DecrqmState::Set)
        );
        assert_eq!(parse_decrqm(b"no response at all", 1006), None);
    }

    #[test]
    fn negotiate_full_modern_terminal() {
        let fake = FakeTerminal::new(&[
            (b"\x1b[c", &[b"\x1b[?1;2;22;24;28c"]),
            (b"\x1b[?1006$p", &[b"\x1b[?1006;1$y"]),
            (b"\x1b[?2004$p", &[b"\x1b[?2004;2$y"]), // supported, currently reset
            (b"\x1b[?2026$p", &[b"\x1b[?2026;3$y"]), // permanently set
            (b"\x1b[?2048$p", &[b"\x1b[?2048;1$y"]),
            (b"\x1b]4;0;?\x07", &[b"\x1b]4;0;rgb:0000/0000/0000\x07"]),
        ]);
        let negotiation = TerminalCapability::conservative()
            .negotiate(&mut fake.io())
            .unwrap();

        let capability = negotiation.capability;
        assert_eq!(capability.color, TerminalColor::Truecolor);
        assert!(capability.unicode, "unicode is baseline-carried");
        assert!(capability.mouse);
        assert!(capability.bracketed_paste);
        assert!(capability.synchronized_output);
        assert!(capability.keyboard_protocol);

        let report = negotiation.report;
        assert_eq!(report.da1.as_ref().unwrap().params, vec![1, 2, 22, 24, 28]);
        assert!(report.color_query_answered);
        assert!(report.truecolor_palette);
        assert_eq!(report.mode_state(modes::SGR_MOUSE), Some(DecrqmState::Set));
        assert_eq!(
            report.mode_state(modes::BRACKETED_PASTE),
            Some(DecrqmState::Reset)
        );
        assert_eq!(
            report.mode_state(modes::SYNCHRONIZED_OUTPUT),
            Some(DecrqmState::PermanentlySet)
        );
    }

    #[test]
    fn negotiate_legacy_terminal_reports_minimal_capabilities() {
        // DA1 without the color bit, SGR mouse explicitly not recognized, and
        // silence for every other DECRQM query and the color query.
        let fake = FakeTerminal::new(&[
            (b"\x1b[c", &[b"\x1b[?1;2c"]),
            (b"\x1b[?1006$p", &[b"\x1b[?1006;0$y"]),
        ]);
        let negotiation = TerminalCapability::conservative()
            .negotiate_with_budget(&mut fake.io(), Duration::from_millis(60))
            .unwrap();

        let capability = negotiation.capability;
        assert_eq!(capability.color, TerminalColor::None);
        assert!(!capability.mouse);
        assert!(!capability.bracketed_paste);
        assert!(!capability.synchronized_output);
        assert!(!capability.keyboard_protocol);

        let report = negotiation.report;
        assert!(!report.color_query_answered);
        assert_eq!(
            report.mode_state(modes::SGR_MOUSE),
            Some(DecrqmState::NotRecognized)
        );
        assert_eq!(report.mode_state(modes::BRACKETED_PASTE), None);
    }

    #[test]
    fn negotiate_keeps_baseline_when_terminal_is_silent() {
        let fake = FakeTerminal::new(&[]);
        let negotiation = TerminalCapability::conservative()
            .negotiate_with_budget(&mut fake.io(), Duration::from_millis(40))
            .unwrap();

        assert_eq!(
            negotiation.capability,
            TerminalCapability::conservative(),
            "silence must fall back to the baseline untouched"
        );
        assert!(negotiation.report.da1.is_none());
        assert!(negotiation.report.decrqm.is_empty());
        // Only the DA1 query should have been sent.
        assert_eq!(*fake.served.borrow(), vec![b"\x1b[c".to_vec()]);
    }

    #[test]
    fn negotiate_accumulates_chunked_responses() {
        // The terminal answers DA1 in two reads; the collector must join them.
        let fake = FakeTerminal::new(&[
            (b"\x1b[c", &[b"\x1b[?1;", b"22;7c"]),
            (b"\x1b[?1006$p", &[b"\x1b[?1006;1$y"]),
        ]);
        let negotiation = TerminalCapability::conservative()
            .negotiate_with_budget(&mut fake.io(), Duration::from_millis(200))
            .unwrap();
        let da1 = negotiation.report.da1.unwrap();
        assert_eq!(da1.params, vec![1, 22, 7]);
        assert!(da1.has_ansi_color());
        assert!(negotiation.capability.mouse);
    }

    #[test]
    fn negotiate_requires_da1_color_before_truecolor_upgrade() {
        // The terminal answers the color query, but DA1 never reported ANSI
        // color, so the OSC 4 probe must not even be sent.
        let fake = FakeTerminal::new(&[
            (b"\x1b[c", &[b"\x1b[?1;2c"]),
            (b"\x1b]4;0;?\x07", &[b"\x1b]4;0;rgb:0000/0000/0000\x07"]),
        ]);
        let negotiation = TerminalCapability::conservative()
            .negotiate_with_budget(&mut fake.io(), Duration::from_millis(100))
            .unwrap();
        assert_eq!(negotiation.capability.color, TerminalColor::None);
        assert!(!negotiation.report.color_query_answered);
        assert!(
            !fake
                .served
                .borrow()
                .iter()
                .any(|request| request == b"\x1b]4;0;?\x07"),
            "OSC 4 query must be gated on DA1 reporting ANSI color"
        );
    }

    #[test]
    fn negotiate_carries_unicode_from_baseline() {
        // A baseline that denies unicode keeps denying it no matter what the
        // terminal answers; everything else is still measured.
        let baseline = TerminalCapability {
            color: TerminalColor::None,
            unicode: false,
            ..TerminalCapability::conservative()
        };
        let fake = FakeTerminal::new(&[
            (b"\x1b[c", &[b"\x1b[?1;2;22c"]),
            (b"\x1b[?1006$p", &[b"\x1b[?1006;1$y"]),
        ]);
        let negotiation = baseline
            .negotiate_with_budget(&mut fake.io(), Duration::from_millis(100))
            .unwrap();
        let capability = negotiation.capability;
        assert!(!capability.unicode);
        assert_eq!(capability.color, TerminalColor::Palette8);
        assert!(capability.mouse);
    }

    #[test]
    fn negotiate_propagates_write_errors() {
        let mut fake = FakeTerminal::new(&[(b"\x1b[c", &[b"\x1b[?1;2c"])]);
        fake.fail_write = true;
        let error = TerminalCapability::conservative()
            .negotiate(&mut fake.io())
            .unwrap_err();
        assert!(error.contains("write failed"), "{error}");
    }

    #[test]
    fn negotiate_propagates_read_errors() {
        let mut fake = FakeTerminal::new(&[(b"\x1b[c", &[b"\x1b[?1;2c"])]);
        fake.fail_read = true;
        let error = TerminalCapability::conservative()
            .negotiate(&mut fake.io())
            .unwrap_err();
        assert!(error.contains("read failed"), "{error}");
    }
}
