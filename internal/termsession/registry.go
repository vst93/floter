package termsession

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// P13-c: a session's record on disk — what a list reads instead of attaching.
//
// The wire record (Info) is what a client is handed; this is the same record
// plus the three facts a reader needs to find the session and judge whether it
// is still there: where its socket is, which process hosts it, and how many
// clients are attached. It lives beside the socket, named after it, so "the
// sessions of this user" is a directory read and nothing else.
//
// It is deliberately a second source of truth next to the socket, and that is
// the point: a socket that refuses a connection says nothing about why, while
// the record says what the session was. A record whose host is gone is a
// session that is lost (see ListRecords), which is exactly the state this
// slice exists to make visible — StateLost was declared in P13-a and nothing
// ever produced it.

// Record is a session as a list sees it.
type Record struct {
	Info
	// Socket is where the host listens.
	Socket string `json:"socket"`
	// HostPid is the process holding the program: the one whose death is the
	// session's death.
	HostPid int `json:"hostPid"`
	// Clients is how many clients are attached right now.
	Clients int `json:"clients"`
	// Started is when the host took the program. It orders a list, so that
	// the youngest session is where a user expects it.
	Started time.Time `json:"started"`
}

// RecordPath is where a session's record lives: beside its socket, named
// after it. A socket of its own therefore gets a record of its own, and a
// test's temporary directory carries both away together.
func RecordPath(socket string) string { return socket + ".json" }

// DefaultDir is where this user's sessions live: sockets and records together.
func DefaultDir() string {
	dir, err := os.UserCacheDir()
	if err != nil || dir == "" {
		dir = os.TempDir()
	}
	return filepath.Join(dir, "floter", "terminal")
}

// WriteRecord puts a session's record where a list will find it.
//
// It is written whole and renamed into place: a reader that catches the write
// halfway must see the old record, never half of a new one — the same rule the
// settings file follows, for the same reason.
func WriteRecord(rec Record) error {
	data, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	path := RecordPath(rec.Socket)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// ReadRecord reads one session's record.
func ReadRecord(socket string) (Record, error) {
	return readRecordFile(RecordPath(socket))
}

// RemoveRecord forgets a session. A record that is already gone is not an
// error: two readers may race to reap the same lost session.
func RemoveRecord(socket string) error {
	err := os.Remove(RecordPath(socket))
	if err != nil && os.IsNotExist(err) {
		return nil
	}
	return err
}

// ListRecords reads every session of this user, oldest first.
//
// A record whose host process is gone is a session that is lost: the socket
// has nothing behind it (the host's death closed the terminal, and the program
// went with it), so the record comes back as StateLost whatever state it was
// written in. The file is left alone — this reader did not write it and does
// not get to bury it; KillCommand is what reaps a lost session.
//
// A pid can be reused by another process, and then a lost session reads as
// running. That is the price of asking the operating system instead of
// connecting (a connect would disturb the session it is asking about), and it
// is what the record is for: state the session reported, not state guessed
// from a probe.
func ListRecords(dir string) ([]Record, error) {
	if dir == "" {
		dir = DefaultDir()
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []Record
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !isRecordName(name) {
			continue
		}
		rec, err := readRecordFile(filepath.Join(dir, name))
		if err != nil {
			// A record that cannot be read is not a session to report: the
			// alternative is inventing one out of a half-written file.
			continue
		}
		if !Alive(rec.HostPid) {
			rec.State = StateLost
		}
		out = append(out, rec)
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].Started.Equal(out[j].Started) {
			return out[i].Started.Before(out[j].Started)
		}
		return out[i].Socket < out[j].Socket
	})
	return out, nil
}

// isRecordName reports whether a directory entry is a session's record: the
// name of a socket with .json after it. The temporary file a write is renamed
// from ends in .json.tmp and is not one.
func isRecordName(name string) bool {
	if filepath.Ext(name) != ".json" {
		return false
	}
	return filepath.Ext(name[:len(name)-len(".json")]) == ".sock"
}

// readRecordFile reads one record by path.
func readRecordFile(path string) (Record, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Record{}, err
	}
	var rec Record
	if err := json.Unmarshal(data, &rec); err != nil {
		return Record{}, err
	}
	return rec, nil
}

// LookupRecord finds a session by its socket path or by the id (or name) a
// list shows. A socket path is what a caller who has one passes straight in;
// an id is what a user reads off a list.
func LookupRecord(dir, idOrSocket string) (Record, bool, error) {
	if idOrSocket == "" {
		return Record{}, false, nil
	}
	if filepath.Ext(idOrSocket) == ".sock" {
		rec, err := ReadRecord(idOrSocket)
		if err == nil {
			if !Alive(rec.HostPid) {
				rec.State = StateLost
			}
			return rec, true, nil
		}
	}
	recs, err := ListRecords(dir)
	if err != nil {
		return Record{}, false, err
	}
	for _, rec := range recs {
		if rec.ID == idOrSocket || rec.Name == idOrSocket {
			return rec, true, nil
		}
	}
	return Record{}, false, nil
}

// writeRecord is the host's half: it puts what the session is right now next
// to its socket, so that a list sees it without attaching.
//
// A write that fails is not reported to anyone: the record is for a list, not
// for the session, and there is no client to tell. The session works either
// way — which is why the record may be absent rather than fatal.
//
// The writes are serialised. The file is written whole, but two renames of two
// different snapshots still leave whichever landed last, and a client count is
// exactly the fact two goroutines report at once (an attach and a detach, or a
// resize while the program is ending).
func (h *Host) writeRecord() {
	h.regMu.Lock()
	defer h.regMu.Unlock()
	h.session.mu.Lock()
	info, closed := h.session.info, h.session.closed
	h.session.mu.Unlock()
	// A host that has stopped does not write again. Its clients are being
	// closed right now and their own detaches want to write one last count,
	// which would put the record back after Stop removed it — and a list
	// cannot tell that resurrected record from a session still running.
	if closed {
		return
	}
	_ = WriteRecord(Record{
		Info:    info,
		Socket:  h.opts.Socket,
		HostPid: os.Getpid(),
		Clients: h.clientCount(),
		Started: h.started,
	})
}
