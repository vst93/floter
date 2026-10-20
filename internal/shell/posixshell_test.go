package shell

import (
	"testing"

	"floter/internal/extensions"
)

// requirePOSIXShell skips a test whose fixture is a `sh` script.
//
// The extension runtime resolves `language: shell` to the interpreter
// `extensions.InterpreterNames` names — `sh` — and Windows ships none (the
// platform's own script language there is `powershell`, which has its own
// coverage). A fixture that cannot start leaves nothing for these tests to
// observe: every one of them reads the provider's answer, so the whole test is
// about a process this host will never spawn.
//
// The question is asked of the product's own resolver rather than of the OS
// name, so a Windows machine that does have a POSIX shell on the path still
// runs them — the fixture is what needs `sh`, not "not Windows".
func requirePOSIXShell(t *testing.T) {
	t.Helper()
	if _, ok := extensions.FindInterpreter("shell"); !ok {
		t.Skip("the fixture is a sh script and this platform has no sh interpreter")
	}
}
