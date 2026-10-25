package apps

import (
	"encoding/binary"
	"io/fs"
	"os"
	"runtime"
	"sort"
	"sync"
	"time"

	"floter/internal/xdg"
)

// The application scan's cache: the launcher opens with the previous scan
// immediately and re-scans only when the sources changed, which is what makes
// a summon cheap instead of a filesystem walk.
//
// The signature is the old build's `paths_signature`: the paths of the roots
// and of every entry the platform's scan reads, sorted and hashed (FNV-1a) with
// each path's length and modification time. A signature that matches says the
// sources the scan observed are the ones still on disk.
//
// "Every entry the platform's scan reads" is the point: the depth is the
// platform's own (see appWalkDepth), and the walk is the scan's own
// (`xdg.WalkEntries`), so the cache cannot be blind to an entry the scan lists.

// signatureCheckInterval is how long a computed signature is trusted before
// the disk is asked again.
const signatureCheckInterval = 30 * time.Second

// scanMaxAge is how long a cached scan is trusted at all, whatever its
// signature says. The signature covers the filesystem; on Windows it cannot see
// the uninstall registry, which is where a program that ships no shortcut lives,
// so a cached list would never learn about one (`max_cache_age`,
// `main:src-tauri/src/commands/apps/windows.rs:85`, the reference's own fallback
// "for installers that only edit values in an existing child key").
const scanMaxAge = 30 * time.Minute

// scanCache holds the last scan, its signature, and when the signature was
// computed.
var (
	scanMu      sync.Mutex
	scanApps    []App
	scanSig     uint64
	scanSigAt   time.Time
	scanHasSig  bool
	scanHasApps bool
	scanStored  time.Time
)

// SourceSignature is the FNV-1a hash of the paths the scan would observe —
// the roots and the entries their platform's scan reads — with each path's
// length and modification time mixed in.
func SourceSignature(roots []string) uint64 {
	return signatureFor(runtime.GOOS, roots)
}

// appWalkDepth is how deep this platform's scan reads, and therefore how deep
// the signature has to look: a signature that stopped above an entry the scan
// reads would leave the cache trusting a list the disk no longer matches. The
// numbers are the scanners' own (see darwinWalkDepth); Linux reads desktop
// entries through `xdg.DesktopWalkDepth`.
func appWalkDepth(platform string) int {
	switch platform {
	case "darwin":
		return darwinWalkDepth
	case "windows":
		return windowsWalkDepth
	default:
		return xdg.DesktopWalkDepth
	}
}

// signatureFor is SourceSignature for one platform's scanners. It takes the
// platform rather than reading it, so the depth table can be pinned from any
// host (the scans themselves are pure I/O over explicit roots).
func signatureFor(platform string, roots []string) uint64 {
	var sources []string
	sources = append(sources, roots...)
	for _, root := range roots {
		collectSignaturePaths(root, appWalkDepth(platform), &sources)
	}
	return pathsSignature(sources)
}

// collectSignaturePaths gathers the paths whose identity the signature covers:
// every entry the platform's scan reads, through the same walk the scan itself
// uses (`xdg.WalkEntries`) and to the same depth. A symlinked folder is
// descended into here exactly as it is there, so the signature covers what the
// scan saw.
func collectSignaturePaths(root string, maxDepth int, sources *[]string) {
	*sources = append(*sources, root)
	xdg.WalkEntries(root, maxDepth, func(path string, d fs.DirEntry, isDir bool) bool {
		*sources = append(*sources, path)
		return true
	})
}

// pathsSignature is the FNV-1a hash of sorted, deduplicated paths, with each
// path's length and modification time mixed in.
func pathsSignature(paths []string) uint64 {
	const (
		fnvOffset = 0xcbf29ce484222325
		fnvPrime  = 0x00000100000001b3
	)
	sorted := append([]string{}, paths...)
	sort.Strings(sorted)
	deduplicated := sorted[:0]
	for i, path := range sorted {
		if i == 0 || path != sorted[i-1] {
			deduplicated = append(deduplicated, path)
		}
	}
	hash := uint64(fnvOffset)
	for _, path := range deduplicated {
		for _, b := range []byte(path) {
			hash ^= uint64(b)
			hash *= fnvPrime
		}
		var modified int64
		if info, err := os.Stat(path); err == nil {
			modified = info.ModTime().UnixNano()
		}
		var bytes [16]byte
		binary.LittleEndian.PutUint64(bytes[0:8], uint64(len(path)))
		binary.LittleEndian.PutUint64(bytes[8:16], uint64(modified))
		for _, b := range bytes {
			hash ^= uint64(b)
			hash *= fnvPrime
		}
	}
	return hash
}

// CachedScan is the previous scan and whether the sources it observed are
// still the ones on disk. The signature check is rate-limited (the old
// build's 30-second interval), so a summon during a burst of reveals costs
// one `stat` batch at most.
func CachedScan(roots []string) (found []App, upToDate bool) {
	scanMu.Lock()
	defer scanMu.Unlock()
	if !scanHasApps {
		return nil, false
	}
	now := time.Now()
	if !scanStored.IsZero() && now.Sub(scanStored) >= scanMaxAge {
		// Stale however fresh the filesystem looks (see scanMaxAge).
		return scanApps, false
	}
	if !scanHasSig || now.Sub(scanSigAt) >= signatureCheckInterval {
		scanSig = SourceSignature(roots)
		scanSigAt = now
		scanHasSig = true
	}
	return scanApps, scanSig == signatureOf(roots)
}

// signatureOf computes the signature now, for the up-to-date comparison.
func signatureOf(roots []string) uint64 {
	return SourceSignature(roots)
}

// StoreScan remembers a scan and the signature of the sources it observed.
func StoreScan(roots []string, found []App) {
	scanMu.Lock()
	defer scanMu.Unlock()
	scanApps = found
	scanSig = SourceSignature(roots)
	scanSigAt = time.Now()
	scanHasSig = true
	scanHasApps = true
	scanStored = time.Now()
}
