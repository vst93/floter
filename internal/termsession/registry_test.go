package termsession

import (
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// P13-c: the record on disk, and the commands that read it.
//
// These are the pins that need no pseudo-terminal: a record is a file, and a
// list is a directory read. The one that needs a real session — a live host's
// record tracking its clients and its state — lives in host_test.go with the
// rest of the host's pins.

// TestARecordRoundTripsWithTheWholeSession pins the shape of what a list
// reads: the wire record plus the socket, the host's pid, the client count and
// when it started. A field that silently stops being written is a field a list
// shows as zero, which is indistinguishable from "no clients" and "no pid".
func TestARecordRoundTripsWithTheWholeSession(t *testing.T) {
	socket := filepath.Join(t.TempDir(), "session-1.sock")
	started := time.Date(2026, 10, 31, 9, 30, 0, 0, time.UTC)
	want := Record{
		Info:    Info{ID: "zsh-9", Name: "zsh", State: StateRunning, Seq: 3, Cols: 120, Rows: 40, Pid: 9, Dir: "/tmp"},
		Socket:  socket,
		HostPid: 4242,
		Clients: 2,
		Started: started,
	}
	if err := WriteRecord(want); err != nil {
		t.Fatal(err)
	}
	got, err := ReadRecord(socket)
	if err != nil {
		t.Fatal(err)
	}
	if got.Info != want.Info || got.Socket != socket || got.HostPid != 4242 || got.Clients != 2 || !got.Started.Equal(started) {
		t.Errorf("record = %+v, want %+v", got, want)
	}
	// The record lives beside the socket, and nothing else: a list is a
	// directory read, so a name that does not follow is a session that is
	// never found.
	if RecordPath(socket) != socket+".json" {
		t.Errorf("record path = %q, want the socket with .json", RecordPath(socket))
	}
	entries, err := os.ReadDir(filepath.Dir(socket))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != filepath.Base(RecordPath(socket)) {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("the sessions directory holds %v, want just the record (no .tmp left behind)", names)
	}
	// Forgetting a session is idempotent: two readers may reap the same lost
	// one, and the second is not an error.
	if err := RemoveRecord(socket); err != nil {
		t.Fatal(err)
	}
	if err := RemoveRecord(socket); err != nil {
		t.Errorf("the second remove = %v, want nil", err)
	}
}

// TestAListReadsOnlyRecordsAndOrdersThemOldestFirst pins the two halves of a
// list: which files are sessions (a socket's record, not the temporary file of
// a write in flight, not a socket without a record) and in what order they
// come back. Ordering by name would put an older session after a younger one
// whenever their pids said so.
func TestAListReadsOnlyRecordsAndOrdersThemOldestFirst(t *testing.T) {
	dir := t.TempDir()
	base := time.Date(2026, 10, 31, 9, 0, 0, 0, time.UTC)
	mk := func(name string, started time.Time) string {
		socket := filepath.Join(dir, name)
		rec := Record{Info: Info{ID: name, Name: name, State: StateRunning}, Socket: socket, HostPid: os.Getpid(), Started: started}
		if err := WriteRecord(rec); err != nil {
			t.Fatal(err)
		}
		return socket
	}
	mk("b.sock", base.Add(2*time.Minute))
	mk("a.sock", base)
	mk("c.sock", base.Add(time.Minute))
	// Things that are not sessions: a socket with no record, a record of
	// something that is not a socket, and a half-written file.
	for _, name := range []string{"bare.sock", "notes.json", "half.sock.json.tmp"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("{}"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	recs, err := ListRecords(dir)
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, rec := range recs {
		ids = append(ids, rec.ID)
	}
	if got := strings.Join(ids, ","); got != "a.sock,c.sock,b.sock" {
		t.Errorf("the list = %v, want the three sessions oldest first", ids)
	}
	// No directory at all is no sessions, not an error: a first launch has no
	// cache directory yet.
	if recs, err := ListRecords(filepath.Join(dir, "absent")); err != nil || recs != nil {
		t.Errorf("a list of a directory that does not exist = (%v, %v), want (nothing, nil)", recs, err)
	}
}

// TestAListReportsAHostThatIsGoneAsLost pins the state this slice exists to
// make real: a record whose host process is no longer there is a session that
// is lost, whatever state the record was written in. The file is not touched —
// the reader did not write it — so a second list gives the same answer.
func TestAListReportsAHostThatIsGoneAsLost(t *testing.T) {
	dir := t.TempDir()
	socket := filepath.Join(dir, "dead.sock")
	pid := deadPid(t)
	rec := Record{Info: Info{ID: "dead", State: StateRunning}, Socket: socket, HostPid: pid}
	if err := WriteRecord(rec); err != nil {
		t.Fatal(err)
	}
	recs, err := ListRecords(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 1 {
		t.Fatalf("the list holds %d sessions, want the one record", len(recs))
	}
	if recs[0].State != StateLost {
		t.Errorf("the session's state = %q, want %q: its host is gone", recs[0].State, StateLost)
	}
	// The record is the writer's, not the reader's.
	again, err := ListRecords(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(again) != 1 || again[0].State != StateLost {
		t.Errorf("the second list = %+v, want the same lost session", again)
	}
	if _, err := os.Stat(RecordPath(socket)); err != nil {
		t.Errorf("the list buried a record it only read: %v", err)
	}
	// And Alive says the same thing about this pid, which is what makes the
	// two halves one rule rather than two.
	if Alive(pid) {
		t.Errorf("Alive(%d) = true for a process that has ended", pid)
	}
	if Alive(0) || Alive(-1) {
		t.Error("a pid that is not a process was reported alive")
	}
}

// TestALookupFindsASessionBySocketOrById pins how a kill names a session: the
// socket a caller already has, or the id (or name) a user read off a list. A
// name that matches nothing is not an error — "no such session" is a normal
// answer for one that has already ended.
func TestALookupFindsASessionBySocketOrById(t *testing.T) {
	dir := t.TempDir()
	socket := filepath.Join(dir, "one.sock")
	rec := Record{Info: Info{ID: "zsh-77", Name: "zsh", State: StateRunning}, Socket: socket, HostPid: os.Getpid()}
	if err := WriteRecord(rec); err != nil {
		t.Fatal(err)
	}
	bySocket, ok, err := LookupRecord(dir, socket)
	if err != nil || !ok || bySocket.ID != "zsh-77" {
		t.Errorf("lookup by socket = (%+v, %v, %v), want the session", bySocket, ok, err)
	}
	byID, ok, err := LookupRecord(dir, "zsh-77")
	if err != nil || !ok || byID.Socket != socket {
		t.Errorf("lookup by id = (%+v, %v, %v), want the session", byID, ok, err)
	}
	byName, ok, err := LookupRecord(dir, "zsh")
	if err != nil || !ok || byName.Socket != socket {
		t.Errorf("lookup by name = (%+v, %v, %v), want the session", byName, ok, err)
	}
	if _, ok, err := LookupRecord(dir, "nobody"); ok || err != nil {
		t.Errorf("lookup of a session that is not there = (%v, %v), want (false, nil)", ok, err)
	}
	if _, ok, err := LookupRecord(dir, ""); ok || err != nil {
		t.Errorf("lookup of nothing = (%v, %v), want (false, nil)", ok, err)
	}
}

// TestTheKillForgetsASessionWhoseHostIsGone pins the one thing a kill can do
// about a lost session: stop offering it. The program went with its host, so
// there is nothing to end — and failing instead would leave the session in
// every list for ever, which is the state the record exists to prevent.
func TestTheKillForgetsASessionWhoseHostIsGone(t *testing.T) {
	dir := t.TempDir()
	socket := filepath.Join(dir, "dead.sock")
	if err := os.WriteFile(socket, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	rec := Record{Info: Info{ID: "dead-1", State: StateRunning}, Socket: socket, HostPid: deadPid(t)}
	if err := WriteRecord(rec); err != nil {
		t.Fatal(err)
	}
	if out := captureStdout(t, func() int { return KillCommand([]string{"-id", "dead-1", "-dir", dir}) }); !strings.Contains(out, "lost") {
		t.Errorf("the kill of a lost session printed %q, want it to say lost", out)
	}
	if _, err := os.Stat(RecordPath(socket)); !os.IsNotExist(err) {
		t.Errorf("the record survived the kill: %v", err)
	}
	if _, err := os.Stat(socket); !os.IsNotExist(err) {
		t.Errorf("the socket survived the kill: %v", err)
	}
	recs, err := ListRecords(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 0 {
		t.Errorf("the list still holds %+v after the kill", recs)
	}
	// A session nobody has is a kill that says so rather than one that
	// pretends: the exit code is the difference.
	if code := KillCommand([]string{"-id", "nobody", "-dir", dir}); code != 1 {
		t.Errorf("the kill of a session that does not exist = %d, want 1", code)
	}
	if code := KillCommand([]string{"-dir", dir}); code != 2 {
		t.Errorf("the kill with nothing to kill = %d, want 2 (a usage error)", code)
	}
}

// TestTheListCommandPrintsTheSessions pins the command's two shapes: a line a
// user reads and a JSON a program reads. The line carries the state, because
// "which of these is lost" is the question a list answers.
func TestTheListCommandPrintsTheSessions(t *testing.T) {
	dir := t.TempDir()
	socket := filepath.Join(dir, "one.sock")
	rec := Record{Info: Info{ID: "zsh-1", Name: "zsh", State: StateRunning, Dir: "/tmp"}, Socket: socket, HostPid: os.Getpid(), Clients: 1}
	if err := WriteRecord(rec); err != nil {
		t.Fatal(err)
	}
	out := captureStdout(t, func() int { return ListCommand([]string{"-dir", dir}) })
	if !strings.Contains(out, "zsh-1") || !strings.Contains(out, string(StateRunning)) || !strings.Contains(out, "1 clients") {
		t.Errorf("the list printed %q, want the session's id, state and client count", out)
	}
	js := captureStdout(t, func() int { return ListCommand([]string{"-dir", dir, "-json"}) })
	var recs []Record
	if err := json.Unmarshal([]byte(js), &recs); err != nil {
		t.Fatalf("the JSON list = %q: %v", js, err)
	}
	if len(recs) != 1 || recs[0].ID != "zsh-1" || recs[0].Socket != socket {
		t.Errorf("the JSON list = %+v, want the session", recs)
	}
	if code := ListCommand([]string{"-dir", t.TempDir()}); code != 0 {
		t.Errorf("the list of no sessions = %d, want 0", code)
	}
	if code := ListCommand([]string{"-nonsense"}); code != 2 {
		t.Errorf("the list with a flag it does not have = %d, want 2", code)
	}
}

// TestEverySessionSubcommandIsOneFamily pins the dispatch table against the
// predicate the entry point uses: every name is recognised, the index it hands
// back is the body that runs it, and there are as many bodies as names. The
// contract test in internal/ci keeps the family ahead of the single-instance
// lock; this is the half that keeps the table itself honest.
func TestEverySessionSubcommandIsOneFamily(t *testing.T) {
	if len(SessionCommandNames) != len(SessionCommands) {
		t.Fatalf("%d subcommand names and %d bodies: they are one family", len(SessionCommandNames), len(SessionCommands))
	}
	for i, name := range SessionCommandNames {
		if name == "" || !strings.HasPrefix(name, "__term") {
			t.Errorf("subcommand %d is %q, want a __term name", i, name)
		}
		which, rest, ok := IsSessionCommand([]string{name, "-x"})
		if !ok || which != i || len(rest) != 1 || rest[0] != "-x" {
			t.Errorf("IsSessionCommand(%q) = (%d, %q, %v), want (%d, [-x], true)", name, which, rest, ok, i)
		}
		// The name is a word of its own: a longer word that starts with it
		// belongs to the app.
		if _, _, ok := IsSessionCommand([]string{name + "ile"}); ok {
			t.Errorf("%qile was read as the subcommand %q", name, name)
		}
	}
	if _, _, ok := IsSessionCommand(nil); ok {
		t.Error("no arguments were read as a session subcommand")
	}
	if _, _, ok := IsSessionCommand([]string{"floter://register?cmd=rg"}); ok {
		t.Error("a deep link was read as a session subcommand")
	}
}

// deadPid is a pid that is not a live process: this test's own child, run and
// waited for. Signal 0 then says "done" — which is the same answer Alive gives
// for a host that crashed, and the one a list has to read as lost.
func deadPid(t *testing.T) int {
	t.Helper()
	cmd := exec.Command("/bin/true")
	if err := cmd.Run(); err != nil {
		t.Fatalf("running /bin/true: %v", err)
	}
	return cmd.Process.Pid
}

// captureStdout runs fn with stdout redirected and returns what it printed.
func captureStdout(t *testing.T, fn func() int) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	saved := os.Stdout
	os.Stdout = w
	code := fn()
	os.Stdout = saved
	_ = w.Close()
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	_ = r.Close()
	if code != 0 {
		t.Fatalf("the command = %d, want 0 (output %q)", code, out)
	}
	return string(out)
}
