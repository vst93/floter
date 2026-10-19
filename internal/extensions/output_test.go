//go:build !windows

package extensions

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestClipPreservesUTF8Boundaries(t *testing.T) {
	for _, char := range []string{"é", "界", "😀"} {
		for remaining := 1; remaining <= len(char); remaining++ {
			t.Run(fmt.Sprintf("%s/%d", char, remaining), func(t *testing.T) {
				prefix := strings.Repeat("a", MaxRunOutputBytes-remaining)
				input := prefix + char + "tail"
				want := prefix
				if remaining == len(char) {
					want += char
				}
				got := clip([]byte(input))
				if got != want || !utf8.ValidString(got) {
					t.Fatalf("clipped length = %d, want %d; valid UTF-8 = %v", len(got), len(want), utf8.ValidString(got))
				}
			})
		}
	}
	for _, input := range []string{"", "hello 界", strings.Repeat("a", MaxRunOutputBytes)} {
		if got := clip([]byte(input)); got != input {
			t.Errorf("changed an untruncated stream of %d bytes", len(input))
		}
	}
	if got := clip([]byte(strings.Repeat("a", MaxRunOutputBytes+1))); got != strings.Repeat("a", MaxRunOutputBytes) {
		t.Error("ASCII truncation lost the last byte")
	}
}

func TestCapturedOutputIsBoundedWhileWriting(t *testing.T) {
	var output capturedOutput
	write := func(p []byte) {
		t.Helper()
		if n, err := output.Write(p); n != len(p) || err != nil {
			t.Fatalf("Write = %d, %v; want %d, nil", n, err, len(p))
		}
		if output.buffer.Len() > MaxRunOutputBytes {
			t.Fatalf("retained %d bytes", output.buffer.Len())
		}
	}
	write([]byte("first"))
	write(bytes.Repeat([]byte("a"), MaxRunOutputBytes-5))
	write(nil)
	if output.truncated || len(output.String()) != MaxRunOutputBytes {
		t.Fatal("exactly full stream marked truncated or lost bytes")
	}
	for i := 0; i < 8; i++ {
		write(bytes.Repeat([]byte("b"), MaxRunOutputBytes))
	}
	if !output.truncated || output.String() != "first"+strings.Repeat("a", MaxRunOutputBytes-5) {
		t.Fatal("overflow did not retain the prefix and mark truncation")
	}
}

// Re-enter the test binary instead of requiring a platform-specific shell.
func TestCapturedOutputProcess(t *testing.T) {
	mode := os.Getenv("FLOTER_TEST_CAPTURE_OUTPUT")
	if mode == "" {
		return
	}
	prefix := strings.Repeat("a", MaxRunOutputBytes-1)
	if mode == "stdout" || mode == "both" {
		fmt.Fprint(os.Stdout, prefix+strings.Repeat("界", MaxRunOutputBytes))
	}
	if mode == "stderr" || mode == "both" {
		fmt.Fprint(os.Stderr, prefix+strings.Repeat("😀", MaxRunOutputBytes))
	}
	os.Exit(0)
}

func TestRunCapturedBoundsEachStream(t *testing.T) {
	program, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"stdout", "stderr", "both"} {
		t.Run(mode, func(t *testing.T) {
			entry := CommandEntry{
				Program: program,
				Args:    []string{"-test.run=^TestCapturedOutputProcess$"},
				Env:     []string{"FLOTER_TEST_CAPTURE_OUTPUT=" + mode},
			}
			run, err := RunCaptured(context.Background(), entry, nil)
			if err != nil || !run.Success || run.ExitCode != 0 || !run.Truncated {
				t.Fatalf("err=%v, success=%v, exit=%d, truncated=%v", err, run.Success, run.ExitCode, run.Truncated)
			}
			for name, stream := range map[string]string{"stdout": run.Stdout, "stderr": run.Stderr} {
				want := ""
				if mode == name || mode == "both" {
					want = strings.Repeat("a", MaxRunOutputBytes-1)
				}
				if stream != want || !utf8.ValidString(stream) {
					t.Errorf("%s: got %d bytes, want %d; valid UTF-8=%v", name, len(stream), len(want), utf8.ValidString(stream))
				}
			}
		})
	}
}
