//go:build !darwin && !linux

package termsession

// Alive reports whether pid names a live process.
//
// On Windows there are no signals to probe with, and this slice has no
// pseudo-terminal there either (S1b is ConPTY): a session is never hosted, so
// there is never a record to judge. Saying "not alive" is the honest answer —
// the alternative is to call every pid a session, which is how a list would
// invent sessions that cannot exist.
func Alive(int) bool { return false }
