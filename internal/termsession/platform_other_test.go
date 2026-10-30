//go:build !darwin && !linux

package termsession

import (
	"path/filepath"
	"testing"
)

// P13-a: where there is no pseudo-terminal the host says so.
//
// The Windows arm of this arc is S1b (ConPTY), and until it lands the one
// thing this package must not do is hand out a session that only looks like
// one: a client would attach, get a record, and find nothing behind it.
func TestTheHostRefusesToPretend(t *testing.T) {
	if _, err := startPTY(Options{Path: "cmd.exe"}, 80, 24); err != ErrUnsupported {
		t.Fatalf("startPTY = %v, want ErrUnsupported on a platform with none", err)
	}
	if _, err := Start(Options{Path: "cmd.exe", Socket: filepath.Join(t.TempDir(), "s.sock")}); err != ErrUnsupported {
		t.Errorf("Start = %v, want ErrUnsupported", err)
	}
}
