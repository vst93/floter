package extensions

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// DefaultRegistryURL is where npm packages come from.
const DefaultRegistryURL = "https://registry.npmjs.org"

// Registry is the npm registry client: metadata, versions and tarballs, as
// the old installer used it (the registry HTTP API, never `npm install`).
type Registry struct {
	// BaseURL is the registry root; DefaultRegistryURL when empty.
	BaseURL string
	// Client is the HTTP client; a default with a timeout when nil.
	Client *http.Client
}

// NewRegistry builds a registry client with the shipped defaults.
func NewRegistry() *Registry {
	return &Registry{
		BaseURL: DefaultRegistryURL,
		Client:  &http.Client{Timeout: 60 * time.Second},
	}
}

// VersionInfo is what a packument says about one version.
type VersionInfo struct {
	Version    string `json:"version"`
	Dist       Dist   `json:"dist"`
	Deprecated string `json:"deprecated"`
}

// Dist is a version's tarball and its integrity.
type Dist struct {
	Tarball   string `json:"tarball"`
	Integrity string `json:"integrity"`
	Shasum    string `json:"shasum"`
}

// Packument is a package's metadata.
type Packument struct {
	Name     string                 `json:"name"`
	DistTags map[string]string      `json:"dist-tags"`
	Versions map[string]VersionInfo `json:"versions"`
}

// ErrNoPackage is what a lookup reports for a package the registry does not
// have.
var ErrNoPackage = errors.New("extensions: the registry has no such package")

func (r *Registry) base() string {
	if r.BaseURL == "" {
		return DefaultRegistryURL
	}
	return r.BaseURL
}

func (r *Registry) client() *http.Client {
	if r.Client == nil {
		return &http.Client{Timeout: 60 * time.Second}
	}
	return r.Client
}

// Packument fetches a package's metadata. A scoped name's slash is encoded,
// as the registry requires.
func (r *Registry) Packument(ctx context.Context, name string) (Packument, error) {
	if strings.TrimSpace(name) == "" {
		return Packument{}, errors.New("extensions: no package name")
	}
	endpoint := r.base() + "/" + url.PathEscape(name)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return Packument{}, err
	}
	request.Header.Set("Accept", "application/json")

	response, err := r.client().Do(request)
	if err != nil {
		return Packument{}, err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		return Packument{}, fmt.Errorf("%w: %s", ErrNoPackage, name)
	}
	if response.StatusCode != http.StatusOK {
		return Packument{}, fmt.Errorf("extensions: the registry answered %s for %s", response.Status, name)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 32<<20))
	if err != nil {
		return Packument{}, err
	}
	var packument Packument
	if err := json.Unmarshal(body, &packument); err != nil {
		return Packument{}, fmt.Errorf("extensions: invalid registry answer for %s: %w", name, err)
	}
	if packument.Versions == nil {
		packument.Versions = map[string]VersionInfo{}
	}
	return packument, nil
}

// Resolve picks the version a constraint names, from the packument's
// dist-tags and versions.
func (p Packument) Resolve(constraint string) (VersionInfo, error) {
	names := make([]string, 0, len(p.Versions))
	for name := range p.Versions {
		names = append(names, name)
	}
	chosen, err := SelectVersion(names, p.DistTags, constraint)
	if err != nil {
		return VersionInfo{}, err
	}
	info, ok := p.Versions[chosen]
	if !ok {
		return VersionInfo{}, fmt.Errorf("extensions: the registry names %s but does not describe it", chosen)
	}
	return info, nil
}

// Download fetches a tarball and verifies its integrity.
func (r *Registry) Download(ctx context.Context, info VersionInfo) ([]byte, error) {
	if info.Dist.Tarball == "" {
		return nil, errors.New("extensions: the version names no tarball")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, info.Dist.Tarball, nil)
	if err != nil {
		return nil, err
	}
	response, err := r.client().Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("extensions: the tarball answered %s", response.Status)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, 256<<20))
	if err != nil {
		return nil, err
	}
	integrity := info.Dist.Integrity
	if integrity == "" && info.Dist.Shasum != "" {
		// An older packument carries only the SHA-1; check it as a SHA-1
		// SRI rather than skipping verification.
		integrity = "sha1-" + base64OfHex(info.Dist.Shasum)
	}
	if err := VerifyIntegrity(data, integrity); err != nil {
		return nil, err
	}
	return data, nil
}

// PackageManifest is the entry of a package's own package.json.
type PackageManifest struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Floter  struct {
		Manifest string `json:"manifest"`
	} `json:"floter"`
}

// LoadPackageManifest reads package.json and the manifest path it names.
func LoadPackageManifest(root string) (PackageManifest, string, error) {
	path := filepath.Join(root, "package.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return PackageManifest{}, "", fmt.Errorf("extensions: the package has no package.json: %w", err)
	}
	var pkg PackageManifest
	if err := json.Unmarshal(data, &pkg); err != nil {
		return PackageManifest{}, "", fmt.Errorf("extensions: invalid package.json: %w", err)
	}
	name := strings.TrimSpace(pkg.Floter.Manifest)
	if name == "" {
		return PackageManifest{}, "", errors.New("extensions: package.json names no floter manifest")
	}
	manifestPath, ok := joinWithin(root, name)
	if !ok {
		return PackageManifest{}, "", fmt.Errorf("extensions: floter.manifest %q leaves the package", name)
	}
	return pkg, manifestPath, nil
}

// InstallFromRegistry installs (or updates) a package from the npm registry:
// metadata, version selection, the tarball, its integrity, extraction, and
// then the same graft a local install uses.
func InstallFromRegistry(ctx context.Context, paths Paths, registry *Registry, name, constraint string) (Entry, error) {
	if registry == nil {
		registry = NewRegistry()
	}
	packument, err := registry.Packument(ctx, name)
	if err != nil {
		return Entry{}, err
	}
	info, err := packument.Resolve(constraint)
	if err != nil {
		return Entry{}, err
	}
	data, err := registry.Download(ctx, info)
	if err != nil {
		return Entry{}, err
	}
	if err := paths.Ensure(); err != nil {
		return Entry{}, err
	}

	// Extract beside the installed packages, so the graft is a rename on
	// one filesystem, and remove the download either way.
	download, err := os.MkdirTemp(paths.Root, ".download-*")
	if err != nil {
		return Entry{}, err
	}
	defer os.RemoveAll(download)
	if err := ExtractTarball(data, download); err != nil {
		return Entry{}, err
	}
	pkg, manifestPath, err := LoadPackageManifest(download)
	if err != nil {
		return Entry{}, err
	}
	if _, err := LoadManifest(manifestPath); err != nil {
		return Entry{}, err
	}

	spec := installSpec{
		PackageDir:   download,
		ManifestPath: manifestPath,
		Distribution: "npm",
		PackageName:  name,
		Version:      pkg.Version,
		Integrity:    info.Dist.Integrity,
	}
	return install(paths, spec)
}

// base64OfHex converts a hexadecimal digest to the base64 SRI spelling.
func base64OfHex(hexDigest string) string {
	raw := make([]byte, 0, len(hexDigest)/2)
	for i := 0; i+1 < len(hexDigest); i += 2 {
		var value byte
		for j := 0; j < 2; j++ {
			c := hexDigest[i+j]
			value <<= 4
			switch {
			case c >= '0' && c <= '9':
				value |= c - '0'
			case c >= 'a' && c <= 'f':
				value |= c - 'a' + 10
			case c >= 'A' && c <= 'F':
				value |= c - 'A' + 10
			default:
				return ""
			}
		}
		raw = append(raw, value)
	}
	if len(raw) == 0 {
		return ""
	}
	return base64.StdEncoding.EncodeToString(raw)
}
