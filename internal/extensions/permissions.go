package extensions

import (
	"errors"
)

// The permission ids a manifest may declare, in the schema's order.
const (
	PermissionFilesystemRead  = "filesystem-read"
	PermissionFilesystemWrite = "filesystem-write"
	PermissionNetworkFetch    = "network-fetch"
	PermissionProcessSpawn    = "process-spawn"
	PermissionClipboardRead   = "clipboard-read"
	PermissionClipboardWrite  = "clipboard-write"
	PermissionEnvironment     = "environment"
)

// AllPermissions lists every declared permission in the schema's order.
var AllPermissions = []string{
	PermissionFilesystemRead,
	PermissionFilesystemWrite,
	PermissionNetworkFetch,
	PermissionProcessSpawn,
	PermissionClipboardRead,
	PermissionClipboardWrite,
	PermissionEnvironment,
}

// The two permissions the host itself decides. The rest are disclosed to the
// user and not intercepted: the host is not a sandbox, and saying so is the
// point of the split.
var hostEnforcedPermissions = map[string]bool{
	PermissionEnvironment:  true,
	PermissionProcessSpawn: true,
}

// PermissionEnforced reports whether the host itself decides a permission
// (true) or merely discloses it (false).
func PermissionEnforced(permission string) bool { return hostEnforcedPermissions[permission] }

// knownPermission reports whether a permission id is one of the schema's.
func knownPermission(permission string) bool {
	for _, candidate := range AllPermissions {
		if candidate == permission {
			return true
		}
	}
	return false
}

// ErrPermissionApprovalRequired is what an install reports when a manifest
// declares permissions the user has not approved for it.
var ErrPermissionApprovalRequired = errors.New("extensions: the package needs its permissions approved")

// PermissionApproval is what an install asks the user about: the permissions
// the package declares that are not already approved for this manifest.
type PermissionApproval struct {
	// ID and Name identify the integration.
	ID, Name string
	// PackageVersion is the version being installed.
	PackageVersion string
	// Added are the permissions that need approval, in the schema's order.
	Added []string
	// Declared is everything the manifest declares.
	Declared []string
	// Digest is the manifest digest the approval would apply to.
	Digest string
}

// RequiresApproval reports what a manifest needs the user to approve, given
// what the existing entry already approved.
//
// The rule is the old build's: an approval is bound to the exact manifest
// bytes, so a changed manifest (a new permission, a changed description)
// needs a fresh one; an unchanged manifest keeps its approval.
func RequiresApproval(previous *Entry, manifest Manifest, digest string) PermissionApproval {
	declared := filterKnownPermissions(manifest.Permissions)
	approval := PermissionApproval{
		ID:             manifest.ID,
		Name:           manifest.Name,
		PackageVersion: manifestPackageVersion(manifest),
		Declared:       declared,
		Digest:         digest,
	}
	if len(declared) == 0 {
		return approval
	}
	approved := map[string]bool{}
	if previous != nil && previous.ApprovedManifestDigest != nil && *previous.ApprovedManifestDigest == digest {
		for _, permission := range previous.ApprovedPermissions {
			approved[permission] = true
		}
	}
	for _, permission := range declared {
		if !approved[permission] {
			approval.Added = append(approval.Added, permission)
		}
	}
	return approval
}

// NeedsApproval reports whether an approval asks the user anything.
func (a PermissionApproval) NeedsApproval() bool { return len(a.Added) > 0 }

// filterKnownPermissions keeps the schema's ids, in the schema's order, so a
// hand-edited manifest cannot smuggle an unknown id into the record.
func filterKnownPermissions(declared []string) []string {
	present := map[string]bool{}
	for _, permission := range declared {
		if knownPermission(permission) {
			present[permission] = true
		}
	}
	var out []string
	for _, permission := range AllPermissions {
		if present[permission] {
			out = append(out, permission)
		}
	}
	return out
}
