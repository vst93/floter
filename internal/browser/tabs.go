package browser

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// Tab is one tab a browser has open right now — the plugin's third and most
// perishable source: history and bookmarks live in files, a tab lives in the
// browser process.
type Tab struct {
	// BrowserID is the browser's id, Window and Index identify the tab for
	// activation, and Active is whether it is the window's focused tab (only
	// macOS knows).
	BrowserID string
	Window    int
	Index     int
	Title     string
	URL       string
	Active    bool
}

// Label is the one-line text a list shows.
func (t Tab) Label() string {
	if strings.TrimSpace(t.Title) != "" {
		return strings.TrimSpace(t.Title)
	}
	return t.URL
}

// DefaultCDPPort is the port Chromium's debug endpoint listens on unless the
// user picked another.
const DefaultCDPPort = 9222

// fetchTimeout is how long one tab read may take. AppleScript against a busy
// browser can be slow, but three seconds is already far past the point where
// the user has stopped expecting a result.
const fetchTimeout = 3 * time.Second

// ErrTabsUnavailable is what a tab read reports when it cannot ask: the
// browser is not running, the debug port is closed, the opt-in is off. It is
// the expected state, not a failure of the plugin.
var ErrTabsUnavailable = errors.New("browser: the browser's tabs are not readable")

// Tabs reads the tabs a browser is showing: through AppleScript on macOS,
// through the DevTools Protocol elsewhere (which needs the user's opt-in and
// a browser started with `--remote-debugging-port`).
//
// Every failure is soft: the caller renders an empty group and a one-line
// note. A tab read never blocks the bookmarks and history beside it.
func Tabs(ctx context.Context, browserID string, cdpEnabled bool, cdpPort int) ([]Tab, error) {
	switch runtime.GOOS {
	case "darwin":
		return macTabs(ctx, browserID)
	default:
		if !cdpEnabled {
			return nil, fmt.Errorf("%w: tab capture is off in the browser plugin's settings", ErrTabsUnavailable)
		}
		if cdpPort <= 0 {
			cdpPort = DefaultCDPPort
		}
		return cdpTabs(ctx, browserID, cdpPort)
	}
}

// ActivateTab brings one tab to the front. When the platform cannot focus it
// — the debug port is closed, the browser is not running — the call degrades
// to opening the tab's URL, so pressing Enter on a tab row always does
// something the user asked for.
func ActivateTab(ctx context.Context, tab Tab, cdpEnabled bool, cdpPort int) error {
	if runtime.GOOS == "darwin" {
		if app, ok := macAppName(tab.BrowserID); ok {
			if err := runAppleScript(ctx, appleScriptActivateTab(app, tab.Window, tab.Index)); err == nil {
				return nil
			}
		}
	} else if cdpEnabled {
		if cdpPort <= 0 {
			cdpPort = DefaultCDPPort
		}
		if err := cdpActivate(ctx, cdpPort, tab.Index); err == nil {
			return nil
		}
	}
	if tab.URL == "" {
		return errors.New("browser: could not focus the tab, and it has no URL to open")
	}
	return OpenURL(tab.BrowserID, tab.URL)
}

// macAppName maps a browser id to its macOS application bundle name. The Edge
// channels are separate bundles, not one bundle with a flag, so each id names
// its own.
func macAppName(browserID string) (string, bool) {
	switch browserID {
	case BrowserChrome:
		return "Google Chrome", true
	case BrowserEdge:
		return "Microsoft Edge", true
	case BrowserEdgeBeta:
		return "Microsoft Edge Beta", true
	case BrowserEdgeDev:
		return "Microsoft Edge Dev", true
	case BrowserEdgeCanary:
		return "Microsoft Edge Canary", true
	case BrowserBrave:
		return "Brave Browser", true
	case BrowserChromium:
		return "Chromium", true
	default:
		return "", false
	}
}

// appleScriptNotRunning is what the list script returns when the browser is
// not running: a single line with no separators, so it can never be read as a
// tab, and the caller can say *why* the group is empty.
const appleScriptNotRunning = "__floter_browser_not_running__"

// appleScriptListTabs is the AppleScript that lists every tab of every
// window: one line per tab, `window \t tab \t active \t url \t title`.
//
// Two things about it are load-bearing:
//
//   - no reserved word as a variable name: `at` is an AppleScript parameter
//     name (a preposition), so `set at to …` makes the compiler reject the
//     whole script — the old build's tab group was never populated and the
//     failure was invisible because the reader is soft-failing by design.
//     The index lives in `activeIndex`.
//   - no `tell application "X"` without a running check: sending an Apple
//     Event to a quit application *launches* it, so asking for tabs would open
//     the user's browser as a side effect.
func appleScriptListTabs(app string) string {
	return strings.Join([]string{
		`set sep to (ASCII character 9)`,
		`set eol to (ASCII character 10)`,
		`set out to ""`,
		`if not (application "` + app + `" is running) then return "` + appleScriptNotRunning + `"`,
		`tell application "` + app + `"`,
		`  set wi to 0`,
		`  repeat with w in windows`,
		`    set wi to wi + 1`,
		`    set activeIndex to active tab index of w`,
		`    set ti to 0`,
		`    repeat with t in tabs of w`,
		`      set ti to ti + 1`,
		`      set flag to "0"`,
		`      if ti is activeIndex then set flag to "1"`,
		`      set out to out & wi & sep & ti & sep & flag & sep & (URL of t) & sep & (title of t) & eol`,
		`    end repeat`,
		`  end repeat`,
		`end tell`,
		`return out`,
	}, "\n")
}

// appleScriptActivateTab is the AppleScript that focuses one tab.
func appleScriptActivateTab(app string, window, tab int) string {
	return fmt.Sprintf("tell application %q\n  set active tab index of window %d to %d\n  activate\nend tell\n", app, window, tab)
}

// parseAppleScriptTabs reads the list script's payload. A malformed line is
// skipped rather than failing the whole read: one odd row must not hide the
// other thirty.
//
// Titles may themselves contain a tab (they come from page markup), so the
// reader splits on the first four separators only — the title keeps the rest
// of the line verbatim.
func parseAppleScriptTabs(browserID, output string) []Tab {
	var tabs []Tab
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSuffix(line, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 5)
		if len(parts) < 5 {
			continue
		}
		window, err := atoi(strings.TrimSpace(parts[0]))
		if err != nil {
			continue
		}
		index, err := atoi(strings.TrimSpace(parts[1]))
		if err != nil {
			continue
		}
		url := strings.TrimSpace(parts[3])
		title := strings.TrimSpace(parts[4])
		if url == "" && title == "" {
			continue
		}
		if title == "" {
			title = url
		}
		tabs = append(tabs, Tab{
			BrowserID: browserID,
			Window:    window,
			Index:     index,
			Title:     title,
			URL:       url,
			Active:    strings.TrimSpace(parts[2]) == "1",
		})
	}
	return tabs
}

// macTabs runs the AppleScript read, time-boxed and soft-failing.
func macTabs(ctx context.Context, browserID string) ([]Tab, error) {
	app, ok := macAppName(browserID)
	if !ok {
		return nil, fmt.Errorf("%w: no AppleScript reader for browser %s", ErrTabsUnavailable, browserID)
	}
	output, err := runAppleScriptOutput(ctx, appleScriptListTabs(app))
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(output) == appleScriptNotRunning {
		return nil, fmt.Errorf("%w: %s is not running", ErrTabsUnavailable, app)
	}
	tabs := parseAppleScriptTabs(browserID, output)
	if len(tabs) == 0 {
		return nil, fmt.Errorf("%w: %s has no open windows", ErrTabsUnavailable, app)
	}
	return tabs, nil
}

// runAppleScript runs a script, discarding its output.
func runAppleScript(ctx context.Context, source string) error {
	_, err := runAppleScriptOutput(ctx, source)
	return err
}

// runAppleScriptOutput runs `osascript -e source` under the fetch deadline and
// returns its stdout.
func runAppleScriptOutput(ctx context.Context, source string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()
	output, err := exec.CommandContext(ctx, "osascript", "-e", source).Output()
	if ctx.Err() != nil {
		return "", fmt.Errorf("%w: osascript did not answer within %s", ErrTabsUnavailable, fetchTimeout)
	}
	if err != nil {
		return "", fmt.Errorf("%w: osascript: %v", ErrTabsUnavailable, err)
	}
	return string(output), nil
}

// cdpTarget is one page target from `/json/list`, reduced to the fields the
// tab list uses.
type cdpTarget struct {
	ID    string `json:"id"`
	Type  string `json:"type"`
	Title string `json:"title"`
	URL   string `json:"url"`
}

// parseCDPTargets reads a `/json/list` payload into tabs. A payload that is
// not a JSON array, or that carries a malformed entry, yields an empty or
// shortened list rather than an error: the endpoint is an optional
// convenience and a version difference must not look like a crash.
func parseCDPTargets(browserID, body string) []Tab {
	var targets []cdpTarget
	if err := json.Unmarshal([]byte(body), &targets); err != nil {
		return nil
	}
	var tabs []Tab
	for _, target := range targets {
		if target.Type != "page" {
			continue
		}
		title := strings.TrimSpace(target.Title)
		url := strings.TrimSpace(target.URL)
		if title == "" && url == "" {
			continue
		}
		if title == "" {
			title = url
		}
		tabs = append(tabs, Tab{
			BrowserID: browserID,
			// The protocol has no window concept; one flat list is the truth
			// the endpoint gives us, so every row reports window 0.
			Window: 0,
			Index:  len(tabs),
			Title:  title,
			URL:    url,
			// `/json/list` does not say which target is focused.
			Active: false,
		})
	}
	return tabs
}

// parseCDPTargetIDs reads the target ids of a `/json/list` payload, in the
// same order as parseCDPTargets, so an index into one is an index into the
// other.
func parseCDPTargetIDs(body string) []string {
	var targets []cdpTarget
	if err := json.Unmarshal([]byte(body), &targets); err != nil {
		return nil
	}
	var ids []string
	for _, target := range targets {
		if target.Type != "page" {
			continue
		}
		if strings.TrimSpace(target.Title) == "" && strings.TrimSpace(target.URL) == "" {
			continue
		}
		ids = append(ids, target.ID)
	}
	return ids
}

// cdpTabs reads the browser's tabs through the DevTools Protocol.
func cdpTabs(ctx context.Context, browserID string, port int) ([]Tab, error) {
	body, err := cdpRequest(ctx, port, http.MethodGet, "/json/list")
	if err != nil {
		return nil, err
	}
	tabs := parseCDPTargets(browserID, body)
	if len(tabs) == 0 {
		return nil, fmt.Errorf("%w: debug port %d answered but reported no page targets", ErrTabsUnavailable, port)
	}
	return tabs, nil
}

// cdpActivate focuses the target at a position in `/json/list`.
func cdpActivate(ctx context.Context, port, index int) error {
	body, err := cdpRequest(ctx, port, http.MethodGet, "/json/list")
	if err != nil {
		return err
	}
	ids := parseCDPTargetIDs(body)
	if index < 0 || index >= len(ids) {
		return fmt.Errorf("browser: no tab at index %d", index)
	}
	_, err = cdpRequest(ctx, port, http.MethodPost, "/json/activate/"+ids[index])
	return err
}

// cdpRequest is one minimal round trip to the local debug endpoint. It talks
// to 127.0.0.1 only, wants no redirects and no TLS, and the socket has a
// deadline, so a port that accepts but never answers cannot hang the read.
func cdpRequest(ctx context.Context, port int, method, path string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()

	client := &http.Client{Timeout: fetchTimeout}
	request, err := http.NewRequestWithContext(ctx, method, fmt.Sprintf("http://127.0.0.1:%d%s", port, path), nil)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrTabsUnavailable, err)
	}
	response, err := client.Do(request)
	if err != nil {
		var netErr net.Error
		if errors.As(err, &netErr) {
			return "", fmt.Errorf("%w: debug port %d is not reachable", ErrTabsUnavailable, port)
		}
		return "", fmt.Errorf("%w: %v", ErrTabsUnavailable, err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%w: debug port %d answered %s", ErrTabsUnavailable, port, response.Status)
	}
	body := make([]byte, 0, 32<<10)
	buffer := make([]byte, 16<<10)
	for {
		read, err := response.Body.Read(buffer)
		body = append(body, buffer[:read]...)
		if err != nil || len(body) > 4<<20 {
			break
		}
	}
	return string(body), nil
}

// atoi parses a decimal index, refusing everything else.
func atoi(text string) (int, error) {
	value := 0
	if text == "" {
		return 0, errors.New("browser: empty index")
	}
	for _, r := range text {
		if r < '0' || r > '9' {
			return 0, errors.New("browser: not an index")
		}
		value = value*10 + int(r-'0')
	}
	return value, nil
}
