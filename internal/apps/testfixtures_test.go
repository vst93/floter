package apps

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

// 这一族夹具**不属于某个平台**：`apps_test.go` 是 `//go:build !windows` 的
// （它测的是 `.desktop` 与 POSIX 路径），但无标签的 `icnsfallback_test.go`
// 用它们造 `.icns` 与 PNG。夹具放在 gated 文件里，整个测试包在 Windows 上就
// 编译不过（`GOOS=windows go vet ./...` 报 `undefined: pngBytes`），
// `.github/workflows/go.yml` 的 windows-latest 那一格于是永远跑不到测试。
// 由 `internal/ci` 的 `TestTestHelpersAreVisibleOnEveryPlatform` 守着。

// icnsEntry is one entry of the container a test builds.
type icnsEntry struct {
	kind    string
	payload []byte
}

// icnsContainer builds an .icns file around the given entries.
func icnsContainer(t *testing.T, entries []icnsEntry) []byte {
	t.Helper()
	var body bytes.Buffer
	for _, entry := range entries {
		if len(entry.kind) != 4 {
			t.Fatalf("kind %q is not four bytes", entry.kind)
		}
		length := make([]byte, 4)
		binary.BigEndian.PutUint32(length, uint32(len(entry.payload)+8))
		body.WriteString(entry.kind)
		body.Write(length)
		body.Write(entry.payload)
	}
	out := bytes.NewBufferString("icns")
	total := make([]byte, 4)
	binary.BigEndian.PutUint32(total, uint32(body.Len()+8))
	out.Write(total)
	out.Write(body.Bytes())
	return out.Bytes()
}

// pngBytes is a real PNG of the given square size.
func pngBytes(t *testing.T, size int) []byte {
	t.Helper()
	var buffer bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	if err := png.Encode(&buffer, img); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func writeBytes(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}
