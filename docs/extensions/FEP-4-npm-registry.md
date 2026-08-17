# FEP-4：NPM Registry Convention

状态：Draft 2

This document defines how Floter extensions are named, indexed, selected, and
downloaded from the public NPM registry. NPM is a distribution and metadata
service only; it is not an extension runtime.

## Package Names

An extension has one base package and, for a bundled runtime, one platform
package for each supported target. A base package MUST use the `floter-`
prefix (for example, `floter-v-tools`) or the `@floter/*` scope (for example,
`@floter/v-tools`). A platform package appends the target to the base package:

```text
<base-package>-<platform>-<arch>
```

For example, the Linux x86-64 package for `floter-v-tools` is
`floter-v-tools-linux-x64`. Targets use the identifiers defined by FEP-1:
`darwin-arm64`, `darwin-x64`, `linux-arm64`, `linux-x64`, `windows-arm64`, and
`windows-x64`.

The current installer accepts normal unscoped and scoped NPM syntax and relies
on publisher conformance for this prefix rule; it enforces the metadata and
manifest requirements below. Publishers MUST retain the naming convention when
a private registry is used.

## Required Metadata

Every base package MUST contain a root `package.json` with the exact keyword
`floter-extension` and a `floter.manifest` path. The path is relative to the
package root and MUST NOT be absolute or contain `..` components. The platform
package MUST contain a `package.json` whose name and version match the registry
record; it does not need to contain an extension manifest. A system runtime
extension has no platform package because the external tool remains under its
system package manager.

```json
{
  "name": "floter-v-tools",
  "version": "1.4.2",
  "description": "V Tools for Floter",
  "keywords": ["floter-extension"],
  "floter": {
    "manifest": "floter.extension.json"
  }
}
```

## Version Coupling

For a bundled runtime, the base package and selected platform package MUST
publish the same SemVer version. For `floter-v-tools@1.4.2`, the selected package
is therefore `floter-v-tools-linux-x64@1.4.2`, not a separately versioned
runtime. The Host rejects a pair with different versions before installing it.
For a system runtime, the NPM version is the integration version; the external
tool version is detected independently and need not match it.

Provider/tool versions are independent metadata and need not equal the NPM
package version.

## Discovery

Floter discovers candidates with the NPM search API. The protocol query is:

```text
https://registry.npmjs.org/-/v1/search?text=keywords:floter-extension
```

When the user enters a search term, the Host appends it to the `text` query and
limits the result size. Search results are unverified until the package has
passed manifest, compatibility, and integrity checks; an official signed index
may provide a separate verified allow-list.

## Resolution and SemVer

An install request may name an exact version, a dist-tag such as `latest`, or a
valid npm semver range such as `^1.4.0`. Exact versions and dist-tags are
resolved first. For a range, the Host selects the highest matching published
version. Normal SemVer precedence applies: prereleases do not satisfy a range
unless the range explicitly includes a prerelease identifier.

Updates retain the selected channel and package lock information. A publisher
MUST NOT republish different bytes under the same version; the tarball digest is
part of the installed identity.

## Download and Verification

The Host reads `dist.tarball` and `dist.integrity` from registry metadata,
downloads the tarball directly over HTTPS, and verifies the NPM Subresource
Integrity value before extraction. It does not run `npm install`, NPM lifecycle
scripts, or package JavaScript, and it does not require Node.js. The current
implementation performs this in `src-tauri/src/extensions/install.rs`
(`download_tarball`, `verify_integrity`, and `safe_unpack`).

Publishers SHOULD provide a `sha512-<base64>` digest in NPM's
`dist.integrity` format. The current Host also recognizes sha384 and sha256
tokens for compatibility, but a package without a supported SRI digest is
rejected.

## Signature Index

Floter's official index is a signed JSON envelope. `payload` is the standard
Base64 encoding of the exact UTF-8 JSON bytes that are signed. Keeping the
payload opaque during signature verification avoids JSON canonicalization
ambiguities. `signatures` may contain signatures from multiple pinned root
keys while the root key is being rotated:

```json
{
  "payload": "<base64 payload>",
  "signatures": [
    {
      "keyId": "floter-release-2026",
      "algorithm": "ed25519",
      "signature": "<base64 Ed25519 signature over decoded payload bytes>"
    }
  ]
}
```

The decoded payload has this format:

```json
{
  "schemaVersion": 1,
  "indexVersion": 1,
  "expiresAt": "2027-01-01T00:00:00Z",
  "entries": [
    {
      "extensionId": "io.github.vst93.v",
      "npmPackage": "v-tools",
      "publisher": "vst93",
      "signingKeys": ["ed25519:<base64 32-byte public key>"]
    }
  ]
}
```

`publisher` is the extension manifest publisher ID. Multiple `signingKeys`
allow an overlap window for publisher-key rotation. The Host pins a minimal set
of Floter root public keys and accepts an envelope when at least one signature
matches a pinned root. It then requires schema version 1, a positive index
version, and a future RFC 3339 expiry. Before release, the development root key
and example entry embedded in `official_index.rs` MUST be replaced with the
production release key and reviewed index data.

Discovery uses the NPM search publisher only as an early candidate match. The
installer does not rely on that result: after SRI verification it independently
binds the manifest extension ID, requested NPM package, manifest publisher ID,
and declared signing key to one valid index entry. It then verifies the
publisher's tarball signature. An unavailable, malformed, tampered, unsupported,
or expired index degrades to community/unverified status and never reuses an
official result from discovery.

### Publisher tarball signature

The root `floter.extension.json` MAY declare an Ed25519 signature for the base
package tarball:

```json
{
  "signatures": {
    "url": "https://example.com/floter-v-tools-1.4.2.sig",
    "publicKey": "ed25519:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
    "algorithm": "ed25519"
  }
}
```

`url` MUST use HTTPS. `publicKey` contains the 32-byte Ed25519 public key in
standard Base64 with the `ed25519:` prefix. The signature resource is UTF-8
text containing the standard Base64 encoding of the 64-byte signature; it MAY
also use the `ed25519:` prefix. The signed message is the exact base-package
tarball response body, before decompression or any other transformation.

The Host first verifies `dist.integrity`, reads the manifest from the verified
archive, downloads the declared signature, and verifies it before resolving or
executing the platform Provider. A missing declaration remains valid for
backward compatibility. An invalid declaration, failed download, or failed
signature rejects the installation. The signature is an additional publisher
signal and does not replace NPM integrity verification. Publishers should also
publish or pin their public key through an independently trusted channel.

## Deprecation

A publisher MAY mark a version or package as deprecated with:

```json
{
  "floter": {
    "manifest": "floter.extension.json",
    "deprecated": true
  }
}
```

`floter.deprecated: true` tells discovery and management UIs to warn before a
new install. It does not silently remove an installed version. Deprecation is
not the same as an NPM unpublish and does not alter lock-file rollback behavior.

## References

- FEP-1 package entry and platform package resolution (`docs/extensions/FEP-1-package.md`)
- FEP-3 install transaction and integrity baseline (`docs/extensions/FEP-3-lifecycle.md`)
- Registry parsing and download implementation (`src-tauri/src/extensions/install.rs`)
