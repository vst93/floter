package inventory

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"floter/internal/extensions"
)

// The tool inventory: every executable this machine offers from more than
// its bare PATH — the desktop entries a Linux publishes, the flatpak/snap/
// nix exports, Homebrew's and Scoop's bins, the LaunchServices apps — with
// a ranking that puts what a person would recognise at the top. Discovery
// is a stat walk, never a spawn; a connect still reviews its permissions.

// DiscoverySource names where a candidate came from. A candidate carries
// every source that found it, so `rg` on the PATH and `rg` with a desktop
// entry are one row, not two.
type DiscoverySource string

// The sources discovery reads.
const (
	SourcePath          DiscoverySource = "path"
	SourceDesktop       DiscoverySource = "desktop"
	SourceFlatpak       DiscoverySource = "flatpak"
	SourceSnap          DiscoverySource = "snap"
	SourceNix           DiscoverySource = "nix"
	SourceBrew          DiscoverySource = "brew"
	SourceLaunchService DiscoverySource = "launch-services"
	SourceChocolatey    DiscoverySource = "chocolatey"
	SourceScoop         DiscoverySource = "scoop"
	SourceWinGet        DiscoverySource = "winget"
)

// DiscoveryQuality is the trust the discovery has in a candidate: an OS
// that published a name for it outranks a bare PATH hit.
type DiscoveryQuality string

// The qualities, best first.
const (
	QualityOfficialAdapter DiscoveryQuality = "official-adapter"
	QualityNativeSupport   DiscoveryQuality = "native-support"
	QualityAutoDetected    DiscoveryQuality = "auto-detected"
	QualityUserDefined     DiscoveryQuality = "user-defined"
)

// Candidate is one executable the machine offers.
type Candidate struct {
	// ID is the source-independent identity: the executable's normalized
	// path, so the same tool found twice is one candidate.
	ID string
	// Name is the executable's own.
	Name string
	// Path is where it lives.
	Path string
	// Description came from discovery metadata (a desktop entry's Comment),
	// empty when no source had one.
	Description string
	// Sources lists everything that found it.
	Sources []DiscoverySource
	// Quality is the best of them.
	Quality DiscoveryQuality
	// Available is whether the file is executable right now.
	Available bool
	// Fingerprint identifies the binary (path:size:mtime), so a tool that
	// was upgraded in place is noticed.
	Fingerprint string
}

// environmentSignature is what a refresh depends on: the search path and
// the data dirs a version manager moves. A change asks for a new walk.
func environmentSignature() string {
	return strings.Join(extensions.SearchDirectories(), "|")
}

// Inventory is the discovered candidates, cached: discovery walks thousands
// of files, and the launcher is open for seconds at a time.
type Inventory struct {
	mu          sync.Mutex
	snapshot    []Candidate
	generated   time.Time
	environment string
	ttl         time.Duration
}

// DefaultTTL is how long a snapshot stands before the next open re-walks.
const DefaultTTL = 5 * time.Minute

// shared is the process's one inventory: the connect flow, the settings
// page and the launcher all ask for the same walk.
var shared = &Inventory{ttl: DefaultTTL}

// Shared returns the process's inventory.
func Shared() *Inventory { return shared }

// NeedsRefresh reports whether the snapshot is missing, stale, or from an
// environment that has since changed.
func (in *Inventory) NeedsRefresh() bool {
	in.mu.Lock()
	defer in.mu.Unlock()
	return in.needsRefresh()
}

func (in *Inventory) needsRefresh() bool {
	return in.snapshot == nil ||
		time.Since(in.generated) >= in.ttl ||
		in.environment != environmentSignature()
}

// Candidates returns the discovered candidates, refreshing first when the
// snapshot needs it. The list is ranked: the ones a person recognises at
// the top.
func (in *Inventory) Candidates() []Candidate {
	in.mu.Lock()
	defer in.mu.Unlock()
	if in.needsRefresh() {
		in.refresh()
	}
	return in.snapshot
}

// refresh re-walks the sources. The caller holds the lock.
func (in *Inventory) refresh() {
	directories := extensions.SearchDirectories()
	in.snapshot = discover(directories)
	in.environment = environmentSignature()
	in.generated = time.Now()
}

// discover walks every source and merges what it finds into one ranked
// list.
func discover(directories []string) []Candidate {
	byPath := map[string]*Candidate{}
	merge := func(path, name, description string, source DiscoverySource, quality DiscoveryQuality) {
		available := isExecutable(path)
		id := normalized(path)
		existing, ok := byPath[id]
		if !ok {
			existing = &Candidate{
				ID:          id,
				Name:        name,
				Path:        path,
				Available:   available,
				Fingerprint: fingerprint(path),
			}
			byPath[id] = existing
		}
		if !hasSource(existing.Sources, source) {
			existing.Sources = append(existing.Sources, source)
		}
		if qualityBetter(quality, existing.Quality) {
			existing.Quality = quality
		}
		if existing.Description == "" && description != "" {
			existing.Description = description
		}
	}
	discoverPath(merge, directories)
	switch runtime.GOOS {
	case "linux":
		discoverLinux(merge)
	case "darwin":
		discoverMacOS(merge)
	case "windows":
		discoverWindows(merge)
	}
	out := make([]Candidate, 0, len(byPath))
	for _, candidate := range byPath {
		out = append(out, *candidate)
	}
	sort.Slice(out, func(i, j int) bool {
		left, right := out[i], out[j]
		if left.Quality != right.Quality {
			// Quality is a string; better first, so compare inverted.
			return qualityBetter(left.Quality, right.Quality)
		}
		if Priority(left) != Priority(right) {
			return Priority(left) > Priority(right)
		}
		return left.Name < right.Name
	})
	return out
}

// discoverPath walks the search path's directories.
func discoverPath(merge func(path, name, description string, source DiscoverySource, quality DiscoveryQuality), directories []string) {
	for _, directory := range directories {
		discoverExecutableDirectory(merge, directory, SourcePath, QualityAutoDetected)
	}
}

// discoverExecutableDirectory reads one directory's executables in.
func discoverExecutableDirectory(merge func(path, name, description string, source DiscoverySource, quality DiscoveryQuality), directory string, source DiscoverySource, quality DiscoveryQuality) {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		path := filepath.Join(directory, entry.Name())
		if !isExecutable(path) {
			continue
		}
		merge(path, entry.Name(), "", source, quality)
	}
}

// isExecutable reports whether a path is a file the user could run: the
// executable bit on Unix, an executable extension on Windows.
func isExecutable(path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	if runtime.GOOS == "windows" {
		switch strings.ToLower(filepath.Ext(path)) {
		case ".exe", ".cmd", ".bat", ".com":
			return true
		}
		return false
	}
	return info.Mode()&0o111 != 0
}

// normalized is a path's identity: absolute, cleaned, and on Windows
// lowercased (the filesystem is not case-sensitive there).
func normalized(path string) string {
	absolute, err := filepath.Abs(path)
	if err != nil {
		absolute = filepath.Clean(path)
	}
	if runtime.GOOS == "windows" {
		return strings.ToLower(absolute)
	}
	return absolute
}

// fingerprint identifies a binary: canonical path, size, and mtime, so a
// Homebrew upgrade that replaced the binary in place is a new tool.
func fingerprint(path string) string {
	target, err := filepath.EvalSymlinks(path)
	if err != nil {
		target = path
	}
	info, err := os.Stat(target)
	if err != nil {
		return ""
	}
	return target + ":" + itoa(info.Size()) + ":" + itoa(info.ModTime().UnixNano())
}

// itoa is strconv for the two numbers a fingerprint carries.
func itoa(value int64) string {
	if value == 0 {
		return "0"
	}
	negative := value < 0
	if negative {
		value = -value
	}
	var digits [20]byte
	position := len(digits)
	for value > 0 {
		position--
		digits[position] = byte('0' + value%10)
		value /= 10
	}
	if negative {
		position--
		digits[position] = '-'
	}
	return string(digits[position:])
}

// hasSource reports whether the sources already carry one.
func hasSource(sources []DiscoverySource, source DiscoverySource) bool {
	for _, existing := range sources {
		if existing == source {
			return true
		}
	}
	return false
}

// qualityOrder is the ranking of the qualities, better first.
var qualityOrder = []DiscoveryQuality{
	QualityOfficialAdapter,
	QualityNativeSupport,
	QualityAutoDetected,
	QualityUserDefined,
}

// qualityBetter reports whether left outranks right.
func qualityBetter(left, right DiscoveryQuality) bool {
	return qualityRank(left) < qualityRank(right)
}

// qualityRank is a quality's position in the order.
func qualityRank(quality DiscoveryQuality) int {
	for index, candidate := range qualityOrder {
		if candidate == quality {
			return index
		}
	}
	return len(qualityOrder)
}
