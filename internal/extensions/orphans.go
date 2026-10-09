package extensions

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Orphans are the package directories under extensions/ that no repository
// entry names: a package an earlier install did not finish, or one dropped in
// by hand. Two operations cover them, and neither can touch an installed
// integration: adopting grafts the package into the repository (the same
// validation, runtime resolution and permission gate an install has), and
// deleting removes the directory.

// ErrNoOrphan is what an orphan operation reports for a directory that is not
// there.
var ErrNoOrphan = errors.New("extensions: no such package directory")

// ErrNotOrphan is what an orphan operation reports for a package the
// repository already names: an installed integration is uninstalled, not
// deleted behind the repository's back.
var ErrNotOrphan = errors.New("extensions: the package is installed, not an orphan")

// PrepareAdopt stages an orphan package directory for adoption. The returned
// Prepared is committed exactly as an install's is, so a package that declares
// permissions is still gated on the user's approval.
func PrepareAdopt(paths Paths, id string) (Prepared, error) {
	if err := validID(id); err != nil {
		return Prepared{}, err
	}
	dir, err := orphanDir(paths, id)
	if err != nil {
		return Prepared{}, err
	}
	prepared, err := PrepareLocal(paths, dir)
	if err != nil {
		return Prepared{}, err
	}
	// The directory names the integration; a manifest that claims another id
	// would be filed under the wrong one.
	if prepared.Manifest.ID != id {
		return Prepared{}, fmt.Errorf("extensions: the package in %s declares %s", id, prepared.Manifest.ID)
	}
	return prepared, nil
}

// Adopt grafts an orphan package into the repository, when nothing has to be
// approved. A package that declares permissions needs PrepareAdopt and Commit
// with the user's answer.
func Adopt(paths Paths, id string) (Entry, error) {
	prepared, err := PrepareAdopt(paths, id)
	if err != nil {
		return Entry{}, err
	}
	return prepared.Commit(!prepared.Approval.NeedsApproval())
}

// DeleteOrphan removes an orphan package directory. It refuses a package the
// repository names: that one goes through Uninstall, which keeps the
// repository and the filesystem in step.
func DeleteOrphan(paths Paths, id string) error {
	if _, err := orphanDir(paths, id); err != nil {
		return err
	}
	return os.RemoveAll(filepath.Join(paths.Extensions, id))
}

// orphanDir is the orphan's package directory, or the reason there is none.
func orphanDir(paths Paths, id string) (string, error) {
	if err := validID(id); err != nil {
		return "", err
	}
	repo, err := LoadRepository(paths.RepositoryFile)
	if err != nil {
		return "", err
	}
	if _, ok := repo.Extensions[id]; ok {
		return "", ErrNotOrphan
	}
	dir := filepath.Join(paths.Extensions, id)
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return "", fmt.Errorf("%w: %s", ErrNoOrphan, id)
	}
	return dir, nil
}
