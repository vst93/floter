package extensions

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestSemverParseAndCompare(t *testing.T) {
	versions := map[string]Semver{}
	for _, text := range []string{"1.2.3", "v1.2.3", "1.2.3-beta.1", "1.2.3+build.5", "0.0.1"} {
		parsed, err := ParseSemver(text)
		if err != nil {
			t.Fatalf("ParseSemver(%q): %v", text, err)
		}
		versions[text] = parsed
	}
	if versions["1.2.3"].Compare(versions["v1.2.3"]) != 0 {
		t.Error("a leading v changed the version")
	}
	if versions["1.2.3-beta.1"].Compare(versions["1.2.3"]) >= 0 {
		t.Error("a prerelease is not lower than its release")
	}
	if versions["0.0.1"].Compare(versions["1.2.3"]) >= 0 {
		t.Error("0.0.1 is not lower than 1.2.3")
	}
	if got := versions["1.2.3+build.5"].String(); got != "1.2.3+build.5" {
		t.Errorf("String = %q", got)
	}
	// Prerelease identifiers: numeric before alphanumeric, fewer first.
	alpha, _ := ParseSemver("1.0.0-alpha")
	alpha1, _ := ParseSemver("1.0.0-alpha.1")
	alphaB, _ := ParseSemver("1.0.0-alpha.beta")
	if alpha.Compare(alpha1) >= 0 || alpha1.Compare(alphaB) >= 0 {
		t.Error("prerelease ordering is wrong")
	}

	for _, text := range []string{"", "x", "1.2.3.4", "1.a"} {
		if _, err := ParseSemver(text); err == nil {
			t.Errorf("ParseSemver(%q) accepted", text)
		}
	}
}

func TestSatisfies(t *testing.T) {
	cases := []struct {
		version    string
		constraint string
		want       bool
	}{
		{"1.2.3", "*", true},
		{"1.2.3", "", true},
		{"1.2.3", "1.2.3", true},
		{"1.2.4", "1.2.3", false},
		{"1.2.3", "^1.0.0", true},
		{"1.5.0", "^1.0.0", true},
		{"2.0.0", "^1.0.0", false},
		{"0.2.5", "^0.2.0", true},
		{"0.3.0", "^0.2.0", false},
		{"1.2.9", "~1.2.0", true},
		{"1.3.0", "~1.2.0", false},
		{"1.2.3", ">=1.0.0 <2.0.0", true},
		{"2.0.0", ">=1.0.0 <2.0.0", false},
		{"1.0.0", "1.0.0 || 2.0.0", true},
		{"3.0.0", "1.0.0 || 2.0.0", false},
		{"1.0.0", ">=0.3.0", true},
		{"1.2.3", "=1.2.3", true},
		{"1.2.3", "^2.0.0 || ~1.2.0", true},
	}
	for _, tc := range cases {
		version, err := ParseSemver(tc.version)
		if err != nil {
			t.Fatal(err)
		}
		if got := Satisfies(version, tc.constraint); got != tc.want {
			t.Errorf("Satisfies(%s, %q) = %v, want %v", tc.version, tc.constraint, got, tc.want)
		}
	}
}

func TestSelectVersion(t *testing.T) {
	versions := []string{"1.0.0", "1.2.0", "1.2.3", "2.0.0-beta.1", "not-a-version"}
	tags := map[string]string{"latest": "1.2.3", "beta": "2.0.0-beta.1"}

	cases := map[string]string{
		"":         "1.2.3",
		"latest":   "1.2.3",
		"beta":     "2.0.0-beta.1",
		"^1.0.0":   "1.2.3",
		"~1.2.0":   "1.2.3",
		">=2.0.0":  "", // only a prerelease satisfies it, and none is selected
		"1.0.0":    "1.0.0",
		"^2.0.0":   "",
		"not-real": "",
	}
	for constraint, want := range cases {
		got, err := SelectVersion(versions, tags, constraint)
		if want == "" {
			if err == nil {
				t.Errorf("SelectVersion(%q) = %q, want an error", constraint, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("SelectVersion(%q): %v", constraint, err)
			continue
		}
		if got != want {
			t.Errorf("SelectVersion(%q) = %q, want %q", constraint, got, want)
		}
	}
}

func sri(data []byte) string {
	sum := sha512.Sum512(data)
	return "sha512-" + base64.StdEncoding.EncodeToString(sum[:])
}

func TestVerifyIntegrity(t *testing.T) {
	data := []byte("a package")
	if err := VerifyIntegrity(data, sri(data)); err != nil {
		t.Errorf("a correct integrity failed: %v", err)
	}
	if err := VerifyIntegrity(data, ""); err != nil {
		t.Errorf("an empty integrity is not verified: %v", err)
	}
	// Several hashes: one matching is enough.
	other := sri([]byte("something else"))
	if err := VerifyIntegrity(data, other+" "+sri(data)); err != nil {
		t.Errorf("a matching hash among several failed: %v", err)
	}
	if err := VerifyIntegrity(data, other); err == nil {
		t.Error("a wrong integrity was accepted")
	}
	if err := VerifyIntegrity(data, "sha512-"+base64.StdEncoding.EncodeToString([]byte("short"))); err == nil {
		t.Error("a wrong digest was accepted")
	}
	if err := VerifyIntegrity(data, "md5-abc"); err == nil {
		t.Error("an unsupported algorithm was accepted")
	}
	// Parameters after the digest are the registry's business.
	if err := VerifyIntegrity(data, sri(data)+"?foo=bar"); err != nil {
		t.Errorf("integrity with parameters failed: %v", err)
	}
}

// tarEntry is one file for the test tarball builder.
type tarEntry struct {
	name string
	body string
	link string
	mode int64
	dir  bool
}

func buildTarball(t *testing.T, entries []tarEntry) []byte {
	t.Helper()
	var buffer bytes.Buffer
	gzipWriter := gzip.NewWriter(&buffer)
	writer := tar.NewWriter(gzipWriter)
	for _, entry := range entries {
		header := &tar.Header{Name: entry.name, Mode: entry.mode}
		if header.Mode == 0 {
			header.Mode = 0o644
		}
		switch {
		case entry.dir:
			header.Typeflag, header.Mode = tar.TypeDir, 0o755
		case entry.link != "":
			header.Typeflag, header.Linkname = tar.TypeSymlink, entry.link
		default:
			header.Typeflag, header.Size = tar.TypeReg, int64(len(entry.body))
		}
		if err := writer.WriteHeader(header); err != nil {
			t.Fatal(err)
		}
		if header.Typeflag == tar.TypeReg {
			if _, err := writer.Write([]byte(entry.body)); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func TestExtractTarball(t *testing.T) {
	dir := t.TempDir()
	data := buildTarball(t, []tarEntry{
		{name: "package/", dir: true},
		{name: "package/floter.extension.json", body: exampleManifest},
		{name: "package/bin/tool", body: "#!/bin/sh\n", mode: 0o755},
		{name: "package/link", link: "/etc/passwd"},
	})
	if err := ExtractTarball(data, dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "floter.extension.json")); err != nil {
		t.Errorf("the manifest was not extracted: %v", err)
	}
	if info, err := os.Stat(filepath.Join(dir, "bin", "tool")); err != nil {
		t.Errorf("the tool was not extracted: %v", err)
	} else if runtime.GOOS != "windows" && info.Mode().Perm()&0o100 == 0 {
		t.Errorf("the tool is not executable: %v", info.Mode())
	}
	// The link is dropped, never created.
	if _, err := os.Lstat(filepath.Join(dir, "link")); !os.IsNotExist(err) {
		t.Errorf("a symlink was extracted: %v", err)
	}

	// A path that leaves the directory is refused, and nothing is written
	// outside it.
	root := t.TempDir()
	escape := buildTarball(t, []tarEntry{{name: "package/../../escaped.txt", body: "x"}})
	if err := ExtractTarball(escape, root); err == nil {
		t.Error("an escaping entry was extracted")
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(root), "escaped.txt")); !os.IsNotExist(err) {
		t.Error("an escaping entry was written")
	}

	absolute := buildTarball(t, []tarEntry{{name: "/tmp/absolute.txt", body: "x"}})
	if err := ExtractTarball(absolute, t.TempDir()); err == nil {
		t.Error("an absolute entry was extracted")
	}

	if err := ExtractTarball([]byte("not a tarball"), t.TempDir()); err == nil {
		t.Error("junk was extracted")
	}
}

// registryFixture serves a packument and a tarball for one package and
// returns the server and the tarball it serves.
func registryFixture(t *testing.T, entries []tarEntry, integrity string) (*httptest.Server, []byte) {
	t.Helper()
	tarball := buildTarball(t, entries)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/@vst93/floter-v":
			packument := map[string]any{
				"name":      "@vst93/floter-v",
				"dist-tags": map[string]string{"latest": "1.2.0", "beta": "2.0.0-beta.1"},
				"versions": map[string]any{
					"1.0.0": map[string]any{"version": "1.0.0", "dist": map[string]string{"tarball": serverURL(r) + "/tarball/1.0.0"}},
					"1.2.0": map[string]any{"version": "1.2.0", "dist": map[string]string{
						"tarball":   serverURL(r) + "/tarball/1.2.0",
						"integrity": integrity,
					}},
					"2.0.0-beta.1": map[string]any{"version": "2.0.0-beta.1", "dist": map[string]string{"tarball": serverURL(r) + "/tarball/beta"}},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(packument)
		case "/tarball/1.2.0", "/tarball/1.0.0", "/tarball/beta":
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Write(tarball)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	return server, tarball
}

// serverURL is the server's own base URL, from the request.
func serverURL(r *http.Request) string {
	return "http://" + r.Host
}

func npmPackageEntries(version string) []tarEntry {
	manifest := strings.Replace(exampleManifest, `"distribution": { "type": "local" }`, `"distribution": { "type": "npm" }`, 1)
	return []tarEntry{
		{name: "package/package.json", body: `{"name": "@vst93/floter-v", "version": "` + version + `", "floter": {"manifest": "floter.extension.json"}}`},
		{name: "package/floter.extension.json", body: manifest},
		{name: "package/tool", body: "#!/bin/sh\nexit 0\n", mode: 0o755},
	}
}

func TestInstallFromRegistry(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fixture's runtime is resolved through sh")
	}
	entries := npmPackageEntries("1.2.0")
	tarball := buildTarball(t, entries)
	server, served := registryFixture(t, entries, sri(tarball))

	paths := FromRoot(t.TempDir())
	registry := &Registry{BaseURL: server.URL, Client: server.Client()}
	entry, err := InstallFromRegistry(context.Background(), paths, registry, "@vst93/floter-v", "^1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if entry.DistributionSource != "npm" || entry.PackageVersion != "1.2.0" {
		t.Errorf("entry = %+v", entry)
	}
	if entry.PackageName == nil || *entry.PackageName != "@vst93/floter-v" {
		t.Errorf("package name = %v", entry.PackageName)
	}
	if entry.Integrity == nil || *entry.Integrity != sri(served) {
		t.Errorf("integrity = %v", entry.Integrity)
	}
	// The package landed, the manifest is where the record points, and no
	// download or staging directory is left behind.
	installed := filepath.Join(paths.Extensions, "io.github.vst93.v")
	if _, err := os.Stat(filepath.Join(installed, manifestFileName)); err != nil {
		t.Errorf("the package did not land: %v", err)
	}
	if entry.ManifestPath != filepath.Join(installed, manifestFileName) {
		t.Errorf("manifestPath = %q", entry.ManifestPath)
	}
	leftovers, err := os.ReadDir(paths.Root)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range leftovers {
		if strings.HasPrefix(item.Name(), ".download-") {
			t.Errorf("a download directory was left: %s", item.Name())
		}
	}

	// The install reaches the inventory, and the manifest's distribution
	// type is the package's own.
	inventory := LoadInventory(paths)
	if len(inventory.Integrations) != 1 {
		t.Fatalf("inventory = %+v", inventory.Integrations)
	}
	if inventory.Integrations[0].Manifest.Distribution.Type != "npm" {
		t.Errorf("manifest distribution = %q", inventory.Integrations[0].Manifest.Distribution.Type)
	}
}

func TestInstallFromRegistryRefusesBadPackages(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fixture's runtime is resolved through sh")
	}
	entries := npmPackageEntries("1.2.0")

	// A tarball whose integrity does not match is refused, and nothing is
	// installed.
	server, _ := registryFixture(t, entries, sri([]byte("something else")))
	paths := FromRoot(t.TempDir())
	registry := &Registry{BaseURL: server.URL, Client: server.Client()}
	if _, err := InstallFromRegistry(context.Background(), paths, registry, "@vst93/floter-v", "1.2.0"); err == nil {
		t.Error("a package with a bad integrity installed")
	}
	if inventory := LoadInventory(paths); len(inventory.Integrations) != 0 {
		t.Errorf("an install landed anyway: %+v", inventory.Integrations)
	}

	// A package whose tar escapes is refused too.
	escape := append(entries, tarEntry{name: "package/../../../escaped.txt", body: "x"})
	escaping := buildTarball(t, escape)
	server2, _ := registryFixture(t, escape, sri(escaping))
	paths2 := FromRoot(t.TempDir())
	registry2 := &Registry{BaseURL: server2.URL, Client: server2.Client()}
	if _, err := InstallFromRegistry(context.Background(), paths2, registry2, "@vst93/floter-v", "1.2.0"); err == nil {
		t.Error("an escaping package installed")
	}

	// An unknown package and an unsatisfiable range are errors, not empty
	// installs.
	paths3 := FromRoot(t.TempDir())
	registry3 := &Registry{BaseURL: server.URL, Client: server.Client()}
	if _, err := InstallFromRegistry(context.Background(), paths3, registry3, "@vst93/nope", "1.0.0"); !errors.Is(err, ErrNoPackage) {
		t.Errorf("an unknown package = %v", err)
	}
	if _, err := InstallFromRegistry(context.Background(), paths3, registry3, "@vst93/floter-v", "^9.0.0"); err == nil {
		t.Error("an unsatisfiable range installed")
	}
	// A package without package.json's floter entry is refused.
	noEntry := buildTarball(t, []tarEntry{
		{name: "package/package.json", body: `{"name": "x", "version": "1.0.0"}`},
		{name: "package/floter.extension.json", body: exampleManifest},
	})
	server3, _ := registryFixture(t, []tarEntry{}, sri(noEntry))
	paths4 := FromRoot(t.TempDir())
	registry4 := &Registry{BaseURL: server3.URL, Client: server3.Client()}
	// The fixture serves its own tarball, so install the hand-built one
	// through the same code path: download, verify, extract.
	if err := VerifyIntegrity(noEntry, sri(noEntry)); err != nil {
		t.Fatal(err)
	}
	download := filepath.Join(paths4.Root, "download")
	if err := os.MkdirAll(download, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := ExtractTarball(noEntry, download); err != nil {
		t.Fatal(err)
	}
	if _, _, err := LoadPackageManifest(download); err == nil {
		t.Error("a package.json without floter.manifest was accepted")
	}
	_ = registry4
}

func TestPackageManifestRejectsEscapes(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "package.json"), `{"name": "x", "version": "1.0.0", "floter": {"manifest": "../outside.json"}}`)
	if _, _, err := LoadPackageManifest(root); err == nil {
		t.Error("an escaping manifest path was accepted")
	}
	writeFile(t, filepath.Join(root, "package.json"), `{"name": "x", "version": "1.0.0"}`)
	if _, _, err := LoadPackageManifest(root); err == nil {
		t.Error("a package.json without floter was accepted")
	}
	writeFile(t, filepath.Join(root, "package.json"), "{not json")
	if _, _, err := LoadPackageManifest(root); err == nil {
		t.Error("malformed package.json was accepted")
	}
}

// TestRealRegistry exercises the client against the real npm registry:
// metadata, version selection, the tarball and its integrity, and the
// extraction. The package it fetches is a real, tiny one — a floter package
// is not published yet (`@vst93/floter-v`, the docs' example, answers 404),
// so the full install path is the fixture's job and this one proves the
// network half against real data.
//
// Opt-in, because it needs the network:
//
//	FLOTER_REGISTRY_TEST=1 go test ./internal/extensions -run TestRealRegistry
func TestRealRegistry(t *testing.T) {
	if os.Getenv("FLOTER_REGISTRY_TEST") == "" {
		t.Skip("set FLOTER_REGISTRY_TEST=1 to reach the npm registry")
	}
	context, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	registry := NewRegistry()
	packument, err := registry.Packument(context, "is-number")
	if err != nil {
		t.Fatal(err)
	}
	if len(packument.Versions) == 0 || packument.DistTags["latest"] == "" {
		t.Fatalf("packument = %+v", packument)
	}
	info, err := packument.Resolve("^7.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if info.Dist.Tarball == "" || info.Dist.Integrity == "" {
		t.Fatalf("dist = %+v", info.Dist)
	}
	t.Logf("resolved is-number %s, integrity %s", info.Version, info.Dist.Integrity[:16]+"...")

	data, err := registry.Download(context, info)
	if err != nil {
		t.Fatalf("download: %v", err)
	}
	dir := t.TempDir()
	if err := ExtractTarball(data, dir); err != nil {
		t.Fatalf("extract: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "package.json")); err != nil {
		t.Errorf("the package.json was not extracted: %v", err)
	}
	// It is not a floter package, and saying so is the right answer.
	if _, _, err := LoadPackageManifest(dir); err == nil {
		t.Error("a package with no floter manifest was accepted")
	}

	// The docs' example package is not published, and the client says so.
	if _, err := registry.Packument(context, "@vst93/floter-v"); !errors.Is(err, ErrNoPackage) {
		t.Errorf("the example package = %v, want ErrNoPackage", err)
	}
}
