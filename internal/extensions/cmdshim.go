package extensions

// cmdExe is the interpreter a Windows command script is run through.
const cmdExe = "cmd.exe"

// cmdShimWrap decides how a program is spawned.
//
// A `.cmd`/`.bat` file is not a program CreateProcess can start: it is a script
// the shell re-parses, and asking the OS to execute it fails with "not a valid
// application". The execution host therefore spells out the wrap — cmd.exe /D
// /S /C with the script and its arguments — where /D skips the AutoRun commands
// and /S fixes the /C quoting to what follows verbatim.
//
// shim is the caller's answer to "is this a Windows command script?" (see
// ProgramIsCmdScript). Spelling it as an argument rather than asking the
// platform again keeps the whole decision pure, so the wrap is testable on
// every platform and the false branch is provably a byte-for-byte no-op: the
// program and the very same argv slice come back.
func cmdShimWrap(shim bool, program string, args []string) (string, []string) {
	if !shim {
		return program, args
	}
	argv := make([]string, 0, len(args)+4)
	argv = append(argv, "/D", "/S", "/C", program)
	argv = append(argv, args...)
	return cmdExe, argv
}
