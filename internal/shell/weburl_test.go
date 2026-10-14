package shell

import (
	"strings"
	"testing"
)

// The old host's `open_url` allow-list: only the three web schemes, matched
// case-insensitively, and nothing that merely resembles one
// (`src-tauri/src/commands/actions.rs:44-68` `is_web_url`).
func TestIsWebURLMatchesTheOldAllowList(t *testing.T) {
	accept := []string{
		"http://example.com",
		"https://example.com/a?b=c",
		"ftp://example.com/file",
		"HTTP://EXAMPLE.COM",
		"hTtPs://example.com",
	}
	for _, raw := range accept {
		if !isWebURL(raw) {
			t.Errorf("isWebURL(%q) = false, want true", raw)
		}
	}
	refuse := []string{
		"",
		"file:///etc/passwd",
		"javascript:alert(1)",
		"mailto:someone@example.com",
		"example.com",
		"httpx://example.com",
		" ftp://example.com",
		"ftp:/example.com",
	}
	for _, raw := range refuse {
		if isWebURL(raw) {
			t.Errorf("isWebURL(%q) = true, want false", raw)
		}
	}
}

// The generic opener refuses a non-web URL before any opener is reached, and
// the integration homepage's refusal is the old host's own sentence.
func TestHomepageRefusesANonWebURL(t *testing.T) {
	a := launchTestApp(t, &launchCalls{})
	opened := []string{}
	a.openURLCommand = func(browserID, url string) { opened = append(opened, url) }

	a.openHomepage("file:///etc/passwd")

	if len(opened) != 0 {
		t.Fatalf("a non-web homepage reached the opener: %v", opened)
	}
	if len(a.notices) != 1 {
		t.Fatalf("notices = %+v, want one refusal", a.notices)
	}
	want := "Refusing to open a non-web URL: file:///etc/passwd"
	if !strings.Contains(a.notices[0].Title, want) {
		t.Errorf("notice = %q, want %q in it", a.notices[0].Title, want)
	}
	if a.notices[0].Type != "error" {
		t.Errorf("notice type = %q, want error", a.notices[0].Type)
	}

	// A web homepage still reaches the opener, case-insensitively.
	a.notices = nil
	opened = nil
	a.openHomepage("HTTPS://example.com/tool")
	if len(opened) != 1 || opened[0] != "HTTPS://example.com/tool" {
		t.Errorf("opened = %v", opened)
	}
	if len(a.notices) != 0 {
		t.Errorf("a web homepage queued a notice: %+v", a.notices)
	}
}

// A browser row carries the id it came from, so the browser-specific opener is
// not narrowed by the generic opener's allow-list.
func TestBrowserSpecificOpenKeepsItsOwnSchemes(t *testing.T) {
	a := launchTestApp(t, &launchCalls{})
	opened := []string{}
	a.openURLCommand = func(browserID, url string) { opened = append(opened, browserID+"|"+url) }

	if err := a.openURL("chrome", "file:///tmp/notes.html"); err != nil {
		t.Fatalf("browser-specific open: %v", err)
	}
	if len(opened) != 1 || opened[0] != "chrome|file:///tmp/notes.html" {
		t.Errorf("opened = %v", opened)
	}
}
