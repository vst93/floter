package termsession

import (
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
