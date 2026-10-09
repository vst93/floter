package extensions

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// VerifyIntegrity checks a payload against a subresource-integrity string,
// as an npm packument records it ("sha512-<base64>", several hashes allowed,
// space separated). An empty integrity is not verified: the caller decides
// whether that is acceptable.
func VerifyIntegrity(data []byte, integrity string) error {
	integrity = strings.TrimSpace(integrity)
	if integrity == "" {
		return nil
	}
	sum512 := sha512.Sum512(data)
	sum256 := sha256.Sum256(data)

	var checked int
	for _, field := range strings.Fields(integrity) {
		algorithm, encoded, ok := strings.Cut(field, "-")
		if !ok {
			continue
		}
		// "sha512-<base64>?foo" — the parameters are the registry's, not
		// ours.
		encoded, _, _ = strings.Cut(encoded, "?")
		want, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			continue
		}
		checked++
		switch strings.ToLower(algorithm) {
		case "sha512":
			if bytes.Equal(want, sum512[:]) {
				return nil
			}
		case "sha256":
			if bytes.Equal(want, sum256[:]) {
				return nil
			}
		}
	}
	if checked == 0 {
		return fmt.Errorf("extensions: unsupported integrity %q", integrity)
	}
	return errors.New("extensions: the package's integrity does not match")
}

// ExtractTarball unpacks a gzipped npm tarball into dir: the leading path
// component ("package/") is stripped, and every entry is checked so nothing
// can be written outside dir.
//
// Entries that are not files or directories (symlinks, hardlinks, devices)
// are skipped rather than followed: a package that ships one loses it, which
// is a smaller problem than a link that points anywhere.
func ExtractTarball(data []byte, dir string) error {
	gzipReader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("extensions: the package is not a gzipped tarball: %w", err)
	}
	defer gzipReader.Close()

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	reader := tar.NewReader(gzipReader)
	for {
		header, err := reader.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("extensions: reading the package: %w", err)
		}
		if strings.HasPrefix(header.Name, "/") || filepath.IsAbs(header.Name) {
			return fmt.Errorf("extensions: the package holds an absolute path: %s", header.Name)
		}
		relative := stripFirstComponent(header.Name)
		if relative == "" {
			continue
		}
		target, ok := resumeWithin(dir, relative)
		if !ok {
			return fmt.Errorf("extensions: the package tries to write outside itself: %s", header.Name)
		}
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := writeTarFile(reader, target, os.FileMode(header.Mode)); err != nil {
				return err
			}
		default:
			// Links and special files are dropped, never followed.
			continue
		}
	}
}

// writeTarFile writes one regular file, with the package's mode plus the
// owner's execute bits (a shipped program must be runnable, as the old
// installer's make_executable did).
func writeTarFile(reader io.Reader, target string, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	file, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode.Perm()|0o700)
	if err != nil {
		return err
	}
	if _, err := io.Copy(file, reader); err != nil {
		file.Close()
		return err
	}
	return file.Close()
}

// stripFirstComponent removes the tarball's wrapper directory ("package/"),
// as npm packs every package that way.
func stripFirstComponent(name string) string {
	cleaned := strings.TrimPrefix(strings.ReplaceAll(name, "\\", "/"), "./")
	cleaned = strings.TrimPrefix(cleaned, "/")
	if cleaned == "" {
		return ""
	}
	if index := strings.IndexByte(cleaned, '/'); index >= 0 {
		cleaned = cleaned[index+1:]
	} else {
		return "" // the wrapper itself
	}
	return strings.TrimSuffix(cleaned, "/")
}

// resumeWithin joins a relative path under root, refusing absolute paths and
// any component that would leave root.
func resumeWithin(root, relative string) (string, bool) {
	if relative == "" || filepath.IsAbs(relative) {
		return "", false
	}
	clean := filepath.Clean(relative)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", false
	}
	for _, part := range strings.Split(clean, string(filepath.Separator)) {
		if part == ".." {
			return "", false
		}
	}
	target := filepath.Join(root, clean)
	cleanRoot := filepath.Clean(root)
	if target != cleanRoot && !strings.HasPrefix(target, cleanRoot+string(filepath.Separator)) {
		return "", false
	}
	return target, true
}
