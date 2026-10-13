package extensions

import (
	"bytes"
	"unicode/utf8"
)

// capturedOutput drains the entire stream but retains only its prefix. Returning
// the full write length lets even a chatty child finish without a broken pipe.
// stdout and stderr each own a writer; os/exec serializes writes to each one.
type capturedOutput struct {
	buffer    bytes.Buffer
	truncated bool
}

func (w *capturedOutput) Write(p []byte) (int, error) {
	n := min(len(p), MaxRunOutputBytes-w.buffer.Len())
	w.buffer.Write(p[:n])
	if n < len(p) {
		w.truncated = true
	}
	return len(p), nil
}

func (w *capturedOutput) String() string {
	data := w.buffer.Bytes()
	if w.truncated {
		data = completeUTF8Prefix(data)
	}
	return string(data)
}

// clip cuts a stream at the cap, without splitting a UTF-8 character.
func clip(data []byte) string {
	if len(data) <= MaxRunOutputBytes {
		return string(data)
	}
	return string(completeUTF8Prefix(data[:MaxRunOutputBytes]))
}

// completeUTF8Prefix drops only an incomplete trailing encoding. Complete
// characters (including a literal replacement rune) and invalid source bytes
// are preserved; truncation must not corrupt an otherwise valid stream.
func completeUTF8Prefix(data []byte) []byte {
	if len(data) == 0 {
		return data
	}
	start := len(data) - 1
	for start > 0 && !utf8.RuneStart(data[start]) {
		start--
	}
	if !utf8.FullRune(data[start:]) {
		return data[:start]
	}
	return data
}
