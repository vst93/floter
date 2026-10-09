package browser

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The exact shape appleScriptListTabs emits, from a browser with two windows:
// window 1 has two tabs (the second active), window 2 has one.
const appleScriptFixture = "1\t1\t0\thttps://example.com/\tExample Domain\n1\t2\t1\thttps://rust-lang.org/\tRust Programming Language\n2\t1\t1\thttps://news.ycombinator.com/\tHacker News\n"

func TestAppleScriptTabsParse(t *testing.T) {
	tabs := parseAppleScriptTabs(BrowserChrome, appleScriptFixture)
	if len(tabs) != 3 {
		t.Fatalf("tabs = %+v", tabs)
	}
	first := tabs[0]
	if first.BrowserID != BrowserChrome || first.Window != 1 || first.Index != 1 ||
		first.URL != "https://example.com/" || first.Title != "Example Domain" || first.Active {
		t.Errorf("first tab = %+v", first)
	}
	if !tabs[1].Active || tabs[1].Window != 1 {
		t.Errorf("the second tab of window 1 is the active one: %+v", tabs[1])
	}
	if tabs[2].Window != 2 || tabs[2].URL != "https://news.ycombinator.com/" {
		t.Errorf("third tab = %+v", tabs[2])
	}

	// A title may hold a tab: only the first four separators are structural.
	tabs = parseAppleScriptTabs(BrowserBrave, "1\t1\t1\thttps://example.com/\tweird\ttitle\n")
	if len(tabs) != 1 || tabs[0].Title != "weird\ttitle" {
		t.Errorf("a tabbed title = %+v", tabs)
	}

	// A malformed line is skipped, not fatal; an untitled tab falls back to
	// its URL; an empty payload is an empty list.
	tabs = parseAppleScriptTabs(BrowserEdge, "1\t1\t1\thttps://ok.example/\tFine\nnot-a-row\n1\tx\t0\thttps://bad.example/\tBad\n\n")
	if len(tabs) != 1 || tabs[0].Title != "Fine" {
		t.Errorf("a malformed line = %+v", tabs)
	}
	if tabs := parseAppleScriptTabs(BrowserChromium, "1\t1\t1\thttps://example.com/\t\n"); len(tabs) != 1 || tabs[0].Title != "https://example.com/" {
		t.Errorf("an untitled tab = %+v", tabs)
	}
	if tabs := parseAppleScriptTabs(BrowserChrome, ""); tabs != nil {
		t.Errorf("an empty payload = %+v", tabs)
	}
	if tabs := parseAppleScriptTabs(BrowserChrome, appleScriptNotRunning); tabs != nil {
		t.Errorf("the sentinel became a tab: %+v", tabs)
	}
}

// The two defects that made the old build's macOS tab read fail on every
// machine: a reserved word as a variable name, and a `tell application "X"`
// that launches the browser to ask it a question.
func TestAppleScriptSource(t *testing.T) {
	source := appleScriptListTabs("Google Chrome")
	for _, want := range []string{
		`tell application "Google Chrome"`,
		"active tab index of w",
		"set activeIndex to active tab index of w",
		"if ti is activeIndex then",
		"(ASCII character 9)",
		`application "Google Chrome" is running`,
		appleScriptNotRunning,
	} {
		if !strings.Contains(source, want) {
			t.Errorf("the list script lacks %q:\n%s", want, source)
		}
	}
	// `at` is an AppleScript parameter name: `set at to …` is a compile
	// error that rejects the whole script.
	if strings.Contains(source, "set at to") || strings.Contains(source, "is at then") {
		t.Error("`at` is reserved: AppleScript refuses the whole script")
	}
	if activate := appleScriptActivateTab("Microsoft Edge", 2, 3); !strings.Contains(activate, "set active tab index of window 2 to 3") {
		t.Errorf("the activate script = %q", activate)
	}
}

// A real /json/list body, trimmed to the fields that matter, with one
// non-page target that must not become a row.
const cdpFixture = `[
  {"id":"A1","type":"page","title":"Example Domain","url":"https://example.com/","webSocketDebuggerUrl":"ws://x"},
  {"id":"A2","type":"background_page","title":"Service Worker","url":"https://example.com/sw.js"},
  {"id":"A3","type":"page","title":"Rust","url":"https://www.rust-lang.org/"},
  {"id":"A4","type":"page","title":"","url":"https://untitled.example/"}
]`

func TestCDPTargets(t *testing.T) {
	tabs := parseCDPTargets(BrowserChrome, cdpFixture)
	if len(tabs) != 3 {
		t.Fatalf("the background page is not a tab: %+v", tabs)
	}
	if tabs[0].Index != 0 || tabs[0].Window != 0 || tabs[0].Title != "Example Domain" {
		t.Errorf("first tab = %+v", tabs[0])
	}
	if tabs[1].Index != 1 || tabs[1].URL != "https://www.rust-lang.org/" {
		t.Errorf("second tab = %+v", tabs[1])
	}
	for _, tab := range tabs {
		if tab.Active {
			t.Error("CDP does not report focus, so nothing claims to be active")
		}
	}
	if tabs[2].Title != "https://untitled.example/" {
		t.Errorf("an empty title falls back to the URL: %+v", tabs[2])
	}

	ids := parseCDPTargetIDs(cdpFixture)
	if len(ids) != 3 || ids[0] != "A1" || ids[2] != "A4" {
		t.Errorf("ids = %v", ids)
	}
	for _, body := range []string{"not json", "{}", "[]", "{"} {
		if tabs := parseCDPTargets(BrowserChrome, body); tabs != nil {
			t.Errorf("a broken payload %q = %+v", body, tabs)
		}
		if ids := parseCDPTargetIDs(body); ids != nil {
			t.Errorf("a broken payload %q = %v", body, ids)
		}
	}
}

func TestCDPTabsOverHTTP(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/json/list":
			w.Write([]byte(cdpFixture))
		case r.Method == http.MethodPost && r.URL.Path == "/json/activate/A3":
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	port := serverPort(t, server.URL)

	tabs, err := cdpTabs(context.Background(), BrowserChrome, port)
	if err != nil || len(tabs) != 3 {
		t.Fatalf("cdpTabs = %+v, %v", tabs, err)
	}
	if err := cdpActivate(context.Background(), port, 1); err != nil {
		t.Errorf("cdpActivate = %v", err)
	}
	if err := cdpActivate(context.Background(), port, 9); err == nil {
		t.Error("an out-of-range index activated something")
	}

	// A closed port is the expected state, reported as such.
	if _, err := cdpTabs(context.Background(), BrowserChrome, port+1); !errors.Is(err, ErrTabsUnavailable) {
		t.Errorf("a closed port = %v", err)
	}
	// The opt-in gate is a soft error, never a fetch.
	if _, err := Tabs(context.Background(), BrowserChrome, false, port); err == nil && !runtimeDarwin() {
		t.Error("the CDP path ran without the opt-in")
	}
}

// serverPort reads the port out of an httptest server URL.
func serverPort(t *testing.T, rawURL string) int {
	t.Helper()
	index := strings.LastIndex(rawURL, ":")
	if index < 0 {
		t.Fatalf("no port in %q", rawURL)
	}
	port, err := atoi(rawURL[index+1:])
	if err != nil {
		t.Fatal(err)
	}
	return port
}

func TestOpenURLRefusesAnEmptyPage(t *testing.T) {
	if err := OpenURL(BrowserChrome, "   "); err == nil {
		t.Error("an empty URL opened")
	}
}

func TestTabLabel(t *testing.T) {
	if got := (Tab{URL: "https://example.com"}).Label(); got != "https://example.com" {
		t.Errorf("label without a title = %q", got)
	}
	if got := (Tab{URL: "https://example.com", Title: "  Docs "}).Label(); got != "Docs" {
		t.Errorf("label = %q", got)
	}
}
