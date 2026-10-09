package apps

import (
	"bytes"
	"encoding/binary"
	"errors"
)

// Reading a macOS `.icns` icon: the container is a magic, a total length, and
// a list of typed entries. Modern icons hold PNG payloads (`ic07`…`ic14`),
// which is what this reads; the older raw-bitmap types are skipped, and a
// bundle whose icon is one of those simply shows no icon rather than a wrong
// one.

// icnsPNGTypes are the entry types whose payload is a PNG, from the smallest
// to the largest.
var icnsPNGTypes = [][]byte{
	[]byte("icp4"), // 16×16
	[]byte("icp5"), // 32×32
	[]byte("icp6"), // 64×64
	[]byte("ic07"), // 128×128
	[]byte("ic08"), // 256×256
	[]byte("ic09"), // 512×512
	[]byte("ic10"), // 1024×1024
	[]byte("ic11"), // 32×32 (2x 16)
	[]byte("ic12"), // 64×64 (2x 32)
	[]byte("ic13"), // 256×256 (2x 128)
	[]byte("ic14"), // 512×512 (2x 256)
}

// ErrNotICNS is what a document that is not an icon reports.
var ErrNotICNS = errors.New("apps: not an icns icon")

// LargestPNGFromICNS returns the largest PNG payload in an `.icns` icon, or
// nil when it holds none.
func LargestPNGFromICNS(data []byte) []byte {
	if len(data) < 8 || string(data[0:4]) != "icns" {
		return nil
	}
	total := int(binary.BigEndian.Uint32(data[4:8]))
	if total <= 8 || total > len(data) {
		total = len(data)
	}
	var best []byte
	for offset := 8; offset+8 <= total; {
		kind := data[offset : offset+4]
		length := int(binary.BigEndian.Uint32(data[offset+4 : offset+8]))
		if length < 8 || offset+length > total {
			break
		}
		payload := data[offset+8 : offset+length]
		if isPNGType(kind) && bytes.HasPrefix(payload, []byte("\x89PNG")) && len(payload) > len(best) {
			best = payload
		}
		offset += length
	}
	return best
}

// isPNGType reports whether an entry type holds a PNG.
func isPNGType(kind []byte) bool {
	for _, candidate := range icnsPNGTypes {
		if bytes.Equal(kind, candidate) {
			return true
		}
	}
	return false
}
