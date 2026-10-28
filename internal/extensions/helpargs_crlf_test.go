package extensions

import (
	"reflect"
	"strings"
	"testing"
)

// TestHelpFromWindowsParsesLikeHelpFromUnix pins the one byte that differs
// between a Windows host and a Unix one: the CR of a CRLF. `--help` output from
// a Windows tool ends every line with it, and the parser used to hand the raw
// line (CR included) to the tokenizer, where it glued itself onto the last
// token of the line. Measured before the fix, on the text below:
//
//	LF:   -A/--after-context kind=string value-hint=NUM description="Show NUM…"
//	      -n kind=string value-hint=int; --quiet kind=flag
//	CRLF: -A/--after-context kind=flag, no value, description="<NUM>"
//	      -n kind=flag, no value hint, description="int"; --quiet gone
//
// i.e. on Windows every option that takes a value was offered as a bare switch
// (the assembled argv lost its argument), the real description was replaced by
// the placeholder, and a switch with no description disappeared outright.
//
// Untagged on purpose: the pin has to run on the platform where the CR exists.
func TestHelpFromWindowsParsesLikeHelpFromUnix(t *testing.T) {
	unix := "Usage:\n" +
		"  rg [OPTIONS]\n" +
		"\n" +
		"  -A, --after-context <NUM>\n" +
		"          Show NUM lines after each match.\n" +
		"\n" +
		"  -n int\n" +
		"          count\n" +
		"\n" +
		"  --quiet\n" +
		"          be quiet\n"
	windows := strings.ReplaceAll(unix, "\n", "\r\n")

	fromUnix := DeriveArguments(unix)
	fromWindows := DeriveArguments(windows)
	if !reflect.DeepEqual(fromUnix, fromWindows) {
		t.Fatalf("CRLF help parsed differently:\n unix   = %+v\n windows= %+v", fromUnix, fromWindows)
	}
	if len(fromUnix) != 3 {
		t.Fatalf("arguments = %+v, want the three options", fromUnix)
	}
	// The three facts the CR used to destroy, asserted on their own so a future
	// regression names which one broke.
	if !fromWindows[0].TakesValue || fromWindows[0].ValueHint != "NUM" {
		t.Errorf("--after-context = %+v, want it to take a NUM", fromWindows[0])
	}
	if fromWindows[0].Description != "Show NUM lines after each match." {
		t.Errorf("--after-context description = %q", fromWindows[0].Description)
	}
	if !fromWindows[1].TakesValue || fromWindows[1].ValueHint != "int" {
		t.Errorf("-n = %+v, want it to take an int", fromWindows[1])
	}
	if fromWindows[2].Names[0] != "--quiet" || fromWindows[2].Kind != "flag" {
		t.Errorf("--quiet = %+v, want a flag", fromWindows[2])
	}
}

// TestSubcommandsFromWindowsParseLikeUnix is the same pin for the listing
// reader: its section headers and rows are compared as whole strings, so a
// trailing CR used to make `Commands:` an unknown header and every row in the
// section invisible.
func TestSubcommandsFromWindowsParseLikeUnix(t *testing.T) {
	unix := "Usage:\n  tool [command]\n\nAvailable Commands:\n" +
		"  build       Build the thing\n" +
		"  serve       Serve the thing\n"
	windows := strings.ReplaceAll(unix, "\n", "\r\n")

	fromUnix := DeriveSubcommands(unix)
	fromWindows := DeriveSubcommands(windows)
	if !reflect.DeepEqual(fromUnix, fromWindows) {
		t.Fatalf("CRLF listing parsed differently:\n unix   = %+v\n windows= %+v", fromUnix, fromWindows)
	}
	if len(fromUnix) != 2 {
		t.Fatalf("subcommands = %+v, want build and serve", fromUnix)
	}
}
