package termsession

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// CommandName is the hidden subcommand the app starts a session host with.
//
// It is a subcommand rather than a second executable because the app has to
// be one binary on every platform it ships (a Windows install is one
// executable; a macOS bundle signs what is inside it), and because a host
// that is a copy of this binary is a host with the same platform code — the
// pseudo-terminal is already here.
const CommandName = "__termhost"

// AttachCommandName is the subcommand that attaches to a host and prints what
// it shows: the smoke test's half of the session arc, and what S1b wires the
// terminal surface to. It exists so a session can be proved alive on a real
// machine — start a host, run a long command, quit the app, attach again —
// without a GUI.
const AttachCommandName = "__termattach"

// ListCommandName is the subcommand that shows this user's sessions: what a
// list is, for the app that will draw one and for a user at a shell. It
// answers the question the host arc could not: which sessions are there, what
// became of them, and which of them nobody is watching (P13-c).
const ListCommandName = "__termlist"

// KillCommandName is the subcommand that ends a session by id or socket — and
// reaps one whose host is gone.
const KillCommandName = "__termkill"

// IsCommand reports whether argv is the host subcommand, and the arguments
// that follow it.
func IsCommand(args []string) ([]string, bool) {
	if len(args) == 0 || args[0] != CommandName {
		return nil, false
	}
	return args[1:], true
}

// IsAttachCommand is IsCommand for the attach subcommand.
func IsAttachCommand(args []string) ([]string, bool) {
	if len(args) == 0 || args[0] != AttachCommandName {
		return nil, false
	}
	return args[1:], true
}

// IsListCommand is IsCommand for the list subcommand.
func IsListCommand(args []string) ([]string, bool) { return splitCommand(args, ListCommandName) }

// IsKillCommand is IsCommand for the kill subcommand.
func IsKillCommand(args []string) ([]string, bool) { return splitCommand(args, KillCommandName) }

// IsSessionCommand reports whether argv is one of the session subcommands —
// and the arguments to hand the command that owns them.
//
// It exists so that the entry point asks **one** question about the whole
// family: every one of them has to be dispatched before the single-instance
// lock, and a new subcommand that forgets is a new subcommand that a running
// app swallows. The contract test reads this list (see
// internal/ci/termsession_contract_test.go).
func IsSessionCommand(args []string) (int, []string, bool) {
	for i, name := range SessionCommandNames {
		if rest, ok := splitCommand(args, name); ok {
			return i, rest, true
		}
	}
	return 0, nil, false
}

// SessionCommandNames is every session subcommand, in the order
// IsSessionCommand returns them, and SessionCommands is what runs each of
// them. Two parallel slices rather than a switch because the entry point
// dispatches from them: "is this the host, or a client, or a list?" has one
// answer in one place, and the contract test that keeps every subcommand
// ahead of the single-instance lock reads the same list.
var SessionCommandNames = []string{CommandName, AttachCommandName, ListCommandName, KillCommandName}

// SessionCommands are the bodies, one per name.
var SessionCommands = []func([]string) int{Command, AttachCommand, ListCommand, KillCommand}

// splitCommand is the one-word test every subcommand shares: the name is a
// word of its own, never a prefix of the app's own arguments.
func splitCommand(args []string, name string) ([]string, bool) {
	if len(args) == 0 || args[0] != name {
		return nil, false
	}
	return args[1:], true
}

// ListCommand prints this user's sessions, one line each, oldest first:
//
//	floter __termlist [-dir <path>] [-json]
//
// A session whose host is gone prints as `lost`, which is the one line a user
// cannot get any other way: attaching to it can only fail, and failing says
// nothing about what it was.
func ListCommand(args []string) int {
	fs := flag.NewFlagSet(ListCommandName, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	dir := fs.String("dir", "", "where sessions live (empty: the user's own)")
	asJSON := fs.Bool("json", false, "print records as JSON")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	recs, err := ListRecords(*dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "floter: %v\n", err)
		return 1
	}
	if *asJSON {
		out, err := json.MarshalIndent(recs, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "floter: %v\n", err)
			return 1
		}
		fmt.Println(string(out))
		return 0
	}
	for _, rec := range recs {
		fmt.Printf("%s\t%s\t%s\t%d clients\tpid %d\t%s\n",
			rec.ID, rec.State, rec.Socket, rec.Clients, rec.HostPid, rec.Dir)
	}
	return 0
}

// KillCommand ends a session: the program and its host. It is the one way a
// session whose host is gone stops being listed —
//
//	floter __termkill -id <id>|-socket <path> [-dir <path>]
//
// because a record whose host has died has nothing left to kill: the session
// is lost, and the only thing to do about it is to stop offering it. A kill
// that cannot reach the host therefore **reaps** rather than fails, and says
// so (herdr-core's keep semantics, which this follows: a lost program is
// never re-run, it is reported and forgotten).
func KillCommand(args []string) int {
	fs := flag.NewFlagSet(KillCommandName, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	id := fs.String("id", "", "the session's id, as the list shows it")
	socket := fs.String("socket", "", "the session's socket")
	dir := fs.String("dir", "", "where sessions live (empty: the user's own)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	target := *socket
	if target == "" {
		target = *id
	}
	if target == "" {
		fmt.Fprintln(os.Stderr, "floter: __termkill needs -id or -socket")
		return 2
	}
	rec, ok, err := LookupRecord(*dir, target)
	if err != nil {
		fmt.Fprintf(os.Stderr, "floter: %v\n", err)
		return 1
	}
	if !ok {
		fmt.Fprintf(os.Stderr, "floter: no session %q\n", target)
		return 1
	}
	if rec.State == StateLost {
		// Nothing to end: the host is gone, so the program is too. Take the
		// record and the socket's leftover away and say what happened.
		_ = RemoveRecord(rec.Socket)
		_ = os.Remove(rec.Socket)
		fmt.Printf("%s lost\n", rec.ID)
		return 0
	}
	cl, err := Dial(rec.Socket)
	if err != nil {
		// The host answered the pid probe and not the socket: it is on its
		// way out. The record is what is stale, so it goes.
		_ = RemoveRecord(rec.Socket)
		_ = os.Remove(rec.Socket)
		fmt.Printf("%s unreachable: %v\n", rec.ID, err)
		return 0
	}
	defer func() { _ = cl.Close() }()
	if err := cl.Kill(); err != nil {
		fmt.Fprintf(os.Stderr, "floter: %v\n", err)
		return 1
	}
	// The host removes its own record when it stops; this waits for that so
	// that a kill followed by a list does not see the session it just ended.
	for i := 0; i < 200; i++ {
		if _, err := os.Stat(RecordPath(rec.Socket)); err != nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	fmt.Printf("%s killed\n", rec.ID)
	return 0
}

// AttachCommand attaches to a host, types what it was given, prints the
// screen it is handed, and detaches — leaving the program running.
//
//	floter __termattach -socket <path> [-type "make build\r"] [-wait 5s]
//
// It is the arc's own smoke test, not a user-facing feature: the terminal
// surface is the client (S1b), and this is how the session is proved alive
// between two launches of the app.
func AttachCommand(args []string) int {
	fs := flag.NewFlagSet(AttachCommandName, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	socket := fs.String("socket", "", "the host to attach to")
	typ := fs.String("type", "", "bytes to type before reading")
	wait := fs.Duration("wait", 2*time.Second, "how long to read")
	kill := fs.Bool("kill", false, "end the program instead of detaching")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *socket == "" {
		fmt.Fprintln(os.Stderr, "floter: __termattach needs -socket")
		return 2
	}
	cl, err := Dial(*socket)
	if err != nil {
		fmt.Fprintf(os.Stderr, "floter: %v\n", err)
		return 1
	}
	defer func() { _ = cl.Close() }()
	if *typ != "" {
		if _, err := cl.Write([]byte(*typ)); err != nil {
			fmt.Fprintf(os.Stderr, "floter: %v\n", err)
			return 1
		}
	}
	if *kill {
		_ = cl.Kill()
	}
	out := readFor(cl, *wait)
	info := cl.Info()
	fmt.Printf("%s %s seq=%d %dx%d pid=%d\n", info.Name, info.State, info.Seq, info.Cols, info.Rows, info.Pid)
	os.Stdout.Write(out)
	return 0
}

// readFor reads what the host sends for the window, or until it ends.
func readFor(cl *Client, window time.Duration) []byte {
	var out []byte
	deadline := time.Now().Add(window)
	buf := make([]byte, 4096)
	for time.Now().Before(deadline) {
		if err := cl.c.SetReadDeadline(deadline); err != nil {
			break
		}
		n, err := cl.Read(buf)
		if n > 0 {
			out = append(out, buf[:n]...)
		}
		if err != nil {
			break
		}
	}
	_ = cl.c.SetReadDeadline(time.Time{})
	return out
}

// Command runs the host subcommand and returns the process's exit code.
//
// The flags are the ones a client needs to describe the session: what to run,
// where, at what size, and where to listen. Everything else about the session
// (its ring, its linger, its name) has a default, because a command line is
// not a settings page.
func Command(args []string) int {
	fs := flag.NewFlagSet(CommandName, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var (
		program = fs.String("program", "", "the program to host (empty: the user's shell)")
		dir     = fs.String("dir", "", "the working directory")
		socket  = fs.String("socket", "", "where to listen (empty: a name of its own)")
		cols    = fs.Int("cols", 80, "the screen's columns")
		rows    = fs.Int("rows", 24, "the screen's rows")
		name    = fs.String("name", "", "what a list shows")
		linger  = fs.Duration("linger", 0, "how long to wait with no client (0: the default)")
		ring    = fs.Int("ring", 0, "bytes of output to keep (0: the default)")
		envs    multiFlag
	)
	fs.Var(&envs, "env", "an environment entry KEY=value (repeatable)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	path, argv := *program, []string(nil)
	if path == "" {
		var err error
		if path, argv, err = UserShell(); err != nil {
			fmt.Fprintf(os.Stderr, "floter: %v\n", err)
			return 1
		}
	}
	o := Options{
		Path:   path,
		Args:   argv,
		Dir:    *dir,
		Env:    Environment(envs),
		Name:   *name,
		Cols:   *cols,
		Rows:   *rows,
		Socket: *socket,
		Linger: *linger,
		Ring:   *ring,
	}
	if o.Name == "" {
		o.Name = filepath.Base(path)
	}
	if err := Run(o); err != nil {
		fmt.Fprintf(os.Stderr, "floter: %v\n", err)
		return 1
	}
	return 0
}

// multiFlag collects a repeatable string flag.
type multiFlag []string

func (m *multiFlag) String() string { return strings.Join(*m, ",") }

func (m *multiFlag) Set(v string) error {
	*m = append(*m, v)
	return nil
}

// UserShell is the program a bare session runs: $SHELL, else the platform's
// own, as a terminal app starts one — a **login** shell, whose argv[0] carries
// the leading dash the shells read. That is the plugin's own rule
// (`plugins/terminal/terminal.go` `command`), and it matters here because the
// host's shell is the one the user comes back to.
func UserShell() (string, []string, error) {
	shell := strings.TrimSpace(os.Getenv("SHELL"))
	if shell == "" {
		for _, candidate := range []string{"/bin/zsh", "/bin/bash", "/bin/sh"} {
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				shell = candidate
				break
			}
		}
	}
	if shell == "" {
		return "", nil, fmt.Errorf("no shell to run: set $SHELL")
	}
	return shell, []string{"-" + filepath.Base(shell)}, nil
}

// Environment is the environment a hosted program starts with: this host's
// own, with the terminal's variables set — the same three the plugin sets, so
// that a program sees the terminal it is in whatever launched the host (a
// Windows app started from the Finder has no locale, and a program that reads
// TERM=="" draws nothing).
func Environment(extra []string) []string {
	env := os.Environ()
	set := func(kv string) {
		k, _, _ := strings.Cut(kv, "=")
		for i, e := range env {
			if strings.HasPrefix(e, k+"=") {
				env[i] = kv
				return
			}
		}
		env = append(env, kv)
	}
	set("TERM=xterm-256color")
	set("COLORTERM=truecolor")
	if os.Getenv("LANG") == "" && os.Getenv("LC_ALL") == "" {
		set("LANG=en_US.UTF-8")
	}
	for _, kv := range extra {
		set(kv)
	}
	return env
}
