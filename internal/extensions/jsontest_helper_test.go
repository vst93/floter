package extensions

import "encoding/json"

// jsonEscaped renders s for the inside of a JSON string: the fixtures that
// record a program or a manifest write their JSON as a raw string with the
// path spliced in between its own quotes, and a Windows path is full of
// backslashes — `"C:\Users\..."` is not a JSON string but an invalid escape
// (`\U`), so the repository the test meant to build did not load at all and
// every test that went through it failed on Windows with "no such
// integration", while the product itself was fine. The fixture is the thing
// that has to quote, so this is the escape and the caller keeps the quotes.
func jsonEscaped(s string) string {
	b, err := json.Marshal(s)
	if err != nil || len(b) < 2 {
		return ""
	}
	return string(b[1 : len(b)-1])
}
