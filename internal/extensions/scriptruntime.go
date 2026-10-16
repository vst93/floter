package extensions

import (
	"bytes"
	"context"
	"os/exec"
	"strings"
	"time"

	"floter/internal/scriptlang"
)

// The runtime check behind the script editor's status line: is the toolchain
// this language needs on PATH, which one, and what does it report?
//
// It never fails. "Not found" is an answer — a user may install Python and come
// back — so the caller's line says so rather than blocking a save, which is
// what the old build's `extensions_script_runtime_check` was for.

// scriptVersionTimeout bounds the version probe: a toolchain that hangs must
// not hold the editor.
const scriptVersionTimeout = 3 * time.Second

// ScriptRuntimeFor looks one script language up. `language` may be a language
// id or one of its interpreter names; an unknown value resolves to the first
// language, as the editor's picker does.
func ScriptRuntimeFor(ctx context.Context, language string) scriptlang.Check {
	option := scriptlang.Lookup(language)
	check := scriptlang.Check{Compiled: option.Compiled}

	names := InterpreterNames(option.ID)
	if len(names) == 0 {
		// A compiled language: the toolchain builds the source at install
		// time, so the thing that has to be there is the compiler.
		names = []string{option.Toolchain}
	}
	check.Candidates = names

	path, ok := lookTool(names...)
	if !ok {
		return check
	}
	check.Available, check.Path = true, path
	check.Version = probeProgramVersion(ctx, path)
	return check
}

// probeProgramVersion runs a program's own `--version` and returns the semver
// it reported, or "" when it reported none. Garbage in, empty out: this never
// guesses a version.
func probeProgramVersion(ctx context.Context, path string) string {
	probeCtx, cancel := context.WithTimeout(ctx, scriptVersionTimeout)
	defer cancel()
	cmd := exec.CommandContext(probeCtx, path, "--version")
	isolateProbe(cmd)
	cmd.Stdin = nil
	cmd.WaitDelay = time.Second
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	_ = cmd.Run()
	if probeCtx.Err() != nil {
		return ""
	}
	text := stdout.String()
	if strings.TrimSpace(text) == "" {
		text = stderr.String()
	}
	return SemverFromVersionOutput(text)
}
