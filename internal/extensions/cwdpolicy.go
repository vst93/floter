package extensions

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// The names a manifest's `lifecycle.launch.cwdPolicy` can carry. The three
// words are spelled as strings, `projectRoot` and `fixed` as objects; the
// parser accepts both spellings for the three words as a convenience, and the
// legacy Go-only `fixed:<abs>` string keeps working so an installed package
// does not stop loading.
const (
	CWDInheritActiveSession = "inheritActiveSession"
	CWDProjectRoot          = "projectRoot"
	CWDToolData             = "toolData"
	CWDHome                 = "home"
	CWDFixed                = "fixed"
)

// The projectRoot defaults are the old host's (cwd_policy.rs): the marker set
// is the schema default plus `pyproject.toml`, the depth cap is 32.
var defaultProjectMarkers = []string{
	".git", "Cargo.toml", "go.mod", "package.json", "pyproject.toml",
}

const defaultProjectMaxDepth = 32

// ErrCWDUnknown is what an unreadable cwd policy reports while a manifest
// loads.
var ErrCWDUnknown = errors.New("extensions: invalid lifecycle.launch.cwdPolicy")

// CWDPolicy is where a command starts, from the manifest's
// `lifecycle.launch.cwdPolicy`. It replaces the bare string the build used to
// hold: a schema-legal object (`{"policy":"projectRoot", ...}` /
// `{"policy":"fixed","path":"/abs"}`) used to make json.Unmarshal reject the
// whole manifest, so a valid package was refused at load time.
//
// The zero value behaves as `inheritActiveSession`, the schema's default.
type CWDPolicy struct {
	// Kind is one of the CWD* names, or "" for the schema default.
	Kind string
	// Markers and MaxDepth belong to a projectRoot policy.
	Markers  []string
	MaxDepth int
	// Path is the absolute directory a fixed policy names.
	Path string
}

// UnmarshalJSON accepts both spellings the schema and the old host took: the
// three string words, and the projectRoot / fixed objects.
func (p *CWDPolicy) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return fmt.Errorf("%w: empty value", ErrCWDUnknown)
	}
	switch trimmed[0] {
	case '"':
		var word string
		if err := json.Unmarshal(trimmed, &word); err != nil {
			return err
		}
		return p.fromWord(word)
	case '{':
		var object struct {
			Policy   string   `json:"policy"`
			Markers  []string `json:"markers"`
			MaxDepth int      `json:"maxDepth"`
			Path     string   `json:"path"`
		}
		if err := json.Unmarshal(trimmed, &object); err != nil {
			return err
		}
		return p.fromObject(object.Policy, object.Markers, object.MaxDepth, object.Path)
	default:
		return fmt.Errorf("%w: %s", ErrCWDUnknown, trimmed)
	}
}

func (p *CWDPolicy) fromWord(word string) error {
	word = strings.TrimSpace(word)
	switch {
	case word == "":
		// The empty string is how this build used to write an absent policy;
		// it stays the zero value rather than an error.
		*p = CWDPolicy{}
	case word == CWDInheritActiveSession:
		*p = CWDPolicy{Kind: CWDInheritActiveSession}
	case word == CWDToolData:
		*p = CWDPolicy{Kind: CWDToolData}
	case word == CWDHome:
		*p = CWDPolicy{Kind: CWDHome}
	case strings.HasPrefix(word, CWDFixed+":"):
		path := strings.TrimPrefix(word, CWDFixed+":")
		if !validFixedPath(path) {
			return fmt.Errorf("%w: path must be an absolute directory", ErrCWDUnknown)
		}
		*p = CWDPolicy{Kind: CWDFixed, Path: path}
	default:
		return fmt.Errorf("%w: %q", ErrCWDUnknown, word)
	}
	return nil
}

// MarshalJSON writes the policy back in the schema's spellings, so a manifest
// this build writes (a custom tool) round-trips. An unset policy writes as the
// empty string, which is what the build emitted before and what UnmarshalJSON
// reads back as unset.
func (p CWDPolicy) MarshalJSON() ([]byte, error) {
	switch p.Kind {
	case CWDProjectRoot:
		return json.Marshal(struct {
			Policy   string   `json:"policy"`
			Markers  []string `json:"markers,omitempty"`
			MaxDepth int      `json:"maxDepth,omitempty"`
		}{CWDProjectRoot, p.Markers, p.MaxDepth})
	case CWDFixed:
		return json.Marshal(struct {
			Policy string `json:"policy"`
			Path   string `json:"path"`
		}{CWDFixed, p.Path})
	default:
		return json.Marshal(p.Kind)
	}
}

func (p *CWDPolicy) fromObject(policy string, markers []string, maxDepth int, path string) error {
	switch strings.TrimSpace(policy) {
	case CWDProjectRoot:
		if len(markers) == 0 {
			markers = append([]string{}, defaultProjectMarkers...)
		}
		if maxDepth <= 0 {
			maxDepth = defaultProjectMaxDepth
		}
		*p = CWDPolicy{Kind: CWDProjectRoot, Markers: markers, MaxDepth: maxDepth}
	case CWDFixed:
		if !validFixedPath(path) {
			return fmt.Errorf("%w: path must be an absolute directory", ErrCWDUnknown)
		}
		*p = CWDPolicy{Kind: CWDFixed, Path: path}
	case CWDInheritActiveSession, CWDToolData, CWDHome:
		*p = CWDPolicy{Kind: policy}
	default:
		return fmt.Errorf("%w: %q", ErrCWDUnknown, policy)
	}
	return nil
}

// validFixedPath is the old host's fixed-path check: a directory that is
// absolute and free of NUL.
func validFixedPath(path string) bool {
	return path != "" && !strings.ContainsRune(path, 0) && filepath.IsAbs(path)
}

// CwdContext is what a cwd policy resolves against, the old host's
// `CwdContext` without the filesystem-permission sandbox this build does not
// have. SessionCwd is the active terminal session's cwd, "" when there is
// none; Home is "" when it cannot be found.
type CwdContext struct {
	SessionCwd string
	DataDir    string
	Home       string
}

// Resolve returns the directory the command runs in; "" means inherit the
// host's own cwd. The fallback chains are the old host's
// (cwd_policy.rs `CwdPolicy::resolve`).
func (p CWDPolicy) Resolve(ctx CwdContext) string {
	switch p.Kind {
	case CWDHome:
		return ctx.Home
	case CWDFixed:
		return p.Path
	case CWDToolData:
		_ = os.MkdirAll(ctx.DataDir, 0o755)
		return ctx.DataDir
	case CWDProjectRoot:
		if root := findProjectRoot(ctx.SessionCwd, p.markers(), p.maxDepth()); root != "" {
			return root
		}
		if isDir(ctx.SessionCwd) {
			return ctx.SessionCwd
		}
		return fallbackDir(ctx)
	default: // inheritActiveSession, and anything the manifest left unset
		if isDir(ctx.SessionCwd) {
			return ctx.SessionCwd
		}
		if root := findProjectRoot(ctx.SessionCwd, defaultProjectMarkers, defaultProjectMaxDepth); root != "" {
			return root
		}
		return fallbackDir(ctx)
	}
}

func (p CWDPolicy) markers() []string {
	if len(p.Markers) == 0 {
		return defaultProjectMarkers
	}
	return p.Markers
}

func (p CWDPolicy) maxDepth() int {
	if p.MaxDepth <= 0 {
		return defaultProjectMaxDepth
	}
	return p.MaxDepth
}

// findProjectRoot walks up from start looking for a directory that holds any
// of the markers, at most maxDepth levels. It is the old host's own walk
// (cwd_policy.rs `find_project_root`).
func findProjectRoot(start string, markers []string, maxDepth int) string {
	if start == "" || maxDepth <= 0 {
		return ""
	}
	current := filepath.Clean(start)
	for depth := 0; depth < maxDepth; depth++ {
		for _, marker := range markers {
			if marker == "" {
				continue
			}
			if _, err := os.Stat(filepath.Join(current, marker)); err == nil {
				return current
			}
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}
	return ""
}

// fallbackDir is the old host's last two rungs: the tool's data directory,
// then the home directory.
func fallbackDir(ctx CwdContext) string {
	if isDir(ctx.DataDir) {
		return ctx.DataDir
	}
	return ctx.Home
}

func isDir(path string) bool {
	if path == "" {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
