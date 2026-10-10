package apps

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// The application scan's cache: the launcher opens with the previous scan
// immediately and re-scans only when the sources changed, which is what makes
// a summon cheap instead of a filesystem walk.
//
// The signature is the old build's `paths_signature`: the paths of the roots
// and of their first two levels of entries, sorted and hashed (FNV-1a) with
// each path's length and modification time. A signature that matches says the
// sources the scan observed are the ones still on disk.

// signatureCheckInterval is how long a computed signature is trusted before
// the disk is asked again.
const signatureCheckInterval = 30 * time.Second

// scanCache holds the last scan, its signature, and when the signature was
// computed.
var (
	scanMu      sync.Mutex
	scanApps    []App
	scanSig     uint64
	scanSigAt   time.Time
	scanHasSig  bool
	scanHasApps bool
)

// SourceSignature is the FNV-1a hash of the paths the scan would observe —
// the roots and their first two levels of entries — with each path's length
// and modification time mixed in.
func SourceSignature(roots []string) uint64 {
	var sources []string
	sources = append(sources, roots...)
	for _, root := range roots {
		collectSignaturePaths(root, 0, &sources)
	}
	return pathsSignature(sources)
}

// collectSignaturePaths gathers the paths whose identity the signature covers:
// the roots and their first two levels of entries, which is the depth the
// scanners themselves read.
func collectSignaturePaths(root string, depth int, sources *[]string) {
	if depth > 2 {
		return
	}
	*sources = append(*sources, root)
	entries, err := os.ReadDir(root)
	if err != nil {
		return
	}
	for _, entry := range entries {
		path := filepath.Join(root, entry.Name())
		*sources = append(*sources, path)
		if depth < 1 && entry.IsDir() {
			collectSignaturePaths(path, depth+1, sources)
		}
	}
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
}
