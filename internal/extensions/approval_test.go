//go:build !windows

package extensions

import (
	"strings"
	"testing"
)

// The short form is a display truncation of exactly `sha256-` + 12 hex —
// never a value a comparison uses, and never a placeholder for an unknown
// one.
func TestShortDigest(t *testing.T) {
	full := "sha256-" + strings.Repeat("abcdef0123456789", 4) // 64 hex
	short := ShortDigest(full)
	if short != "sha256-abcdef012345" {
		t.Errorf("short = %q", short)
	}
	// Exactly at the width, or already shorter, is shown whole.
	atWidth := "sha256-abcdef012345"
	if got := ShortDigest(atWidth); got != atWidth {
		t.Errorf("at width = %q", got)
	}
	if got := ShortDigest("sha256-abc"); got != "sha256-abc" {
		t.Errorf("under width = %q", got)
	}
	// An unknown digest is no digest: the caller omits the value rather than
	// printing a placeholder.
	for _, unknown := range []string{"", "   ", "\t\n"} {
		if got := ShortDigest(unknown); got != "" {
			t.Errorf("unknown %q = %q", unknown, got)
		}
	}
	// The width counts characters, not bytes.
	long := "sha256-" + strings.Repeat("é", 20)
	if got := ShortDigest(long); got != "sha256-"+strings.Repeat("é", 12) {
		t.Errorf("multibyte = %q", got)
	}
}

// Stale means *both sides known and different*. A missing recorded digest
// (old data) or a missing current digest (the manifest could not be read) is
// unknown, not stale — the card says nothing rather than raising a false
// flag.
func TestApprovalIsStale(t *testing.T) {
	a := "sha256-" + strings.Repeat("a", 64)
	b := "sha256-" + strings.Repeat("b", 64)
	cases := []struct {
		approved, current string
		want              bool
	}{
		{a, a, false},
		{a, b, true},
		{"", a, false},
		{a, "", false},
		{"", "", false},
	}
	for _, tc := range cases {
		if got := ApprovalIsStale(tc.approved, tc.current); got != tc.want {
			t.Errorf("ApprovalIsStale(%q, %q) = %v, want %v", tc.approved, tc.current, got, tc.want)
		}
	}
	// The comparison sees the full strings: two digests that agree in the
	// first 12 hex but differ later are still stale.
	prefix := "sha256-" + strings.Repeat("0", 12)
	if !ApprovalIsStale(prefix+"aaa", prefix+"bbb") {
		t.Error("digests sharing the display prefix were reported unchanged")
	}
}
