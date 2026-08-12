# FEP-5：Permissions and Security

状态：Draft 1

Floter extensions are native programs. A permission declaration is a disclosure
and consent boundary for the user; it is not a kernel sandbox. The Provider still
runs with the operating-system identity and rights of the Floter process.

## Permission Model

Permissions are a provider and Host execution contract, not an operating-system sandbox. The Host reviews them during installation and update. `environment` controls environment inheritance, and `process-spawn` controls a descriptor asking the Host to run a program other than its own runtime. File, network, clipboard, and any child processes started directly by local code are currently disclosure and audit boundaries.

The install approval records the complete declared permission set, but approval
does not imply that every item is technically blocked at runtime. The current
Host-enforced subset is `environment` and descriptor-driven `process-spawn`;
`filesystem-*`, `network-fetch`, and `clipboard-*` remain declarations for user
review and audit.

User-created executables and JS, Shell, or PowerShell scripts still run with the current user's operating-system rights. They can call native APIs directly, so permission checkboxes must not be treated as malicious-code protection. Strong isolation would require a separate OS sandbox implementation for each platform.

The extension manifest declares the capabilities it needs in the
`permissions` array of `floter.extension.json`. During installation and update,
the Host resolves the manifest, displays the requested capabilities, and asks
the user to confirm. An update that adds a capability MUST pause for explicit
confirmation. Removing capabilities does not require confirmation, but the
change is recorded with the installed version.

The protocol names capabilities using dotted notation. The JSON schema uses the
corresponding kebab-case wire values shown in the table.

| Capability | Meaning | Draft 1 manifest value |
| --- | --- | --- |
| `filesystem.read` | Read files and directories available to the process | `filesystem-read` |
| `filesystem.write` | Create, modify, or remove files | `filesystem-write` |
| `network.fetch` | Make outbound network requests | `network-fetch` |
| `process.spawn` | Start child processes | `process-spawn` |
| `clipboard.read` | Read the system clipboard | `clipboard-read` |
| `clipboard.write` | Write the system clipboard | `clipboard-write` |
| `environment` | Inherit Host environment variables | `environment` |

```json
{
  "schemaVersion": "2.0",
  "id": "io.example.v-tools",
  "permissions": [
    "filesystem-read",
    "network-fetch",
    "process-spawn",
    "environment"
  ]
}
```

The canonical enum and parsing behavior are defined in
`docs/extensions/schemas/floter-extension.schema.json` and
`src-tauri/src/extensions/manifest.rs` (`Permission`).

## Isolation Boundary

The Host launches each Provider in an independent operating-system process and
communicates through the Provider protocol over stdin/stdout. An extension:

- MUST NOT read or modify Floter internal state, lock files, provider caches, or
  another extension's program/data directories;
- MUST NOT modify another extension's data, even when the process has filesystem
  access;
- MUST treat all Host input as structured JSON/argv data, not as a shell script.

Without `environment`, the Host clears inherited environment variables before
starting the Provider or an extension command, then applies only explicit
manifest and configuration entries. An execution descriptor whose `program` is
not `self` requires `process-spawn`. The latter check governs what the Host
resolves and starts from the descriptor; it cannot stop a native executable or
script from starting a child process by calling the operating system directly.
These checks are Host execution controls, not an OS sandbox.

These are protocol requirements and packaging policy. Permission declarations do
not grant access to Floter internals and do not make native code safe by
themselves.

## Provider Resource Limits

The Host bounds every protocol call, captures stdout/stderr with size limits,
and terminates a process that exceeds its deadline:

| Operation | Protocol budget |
| --- | ---: |
| `describe` | 5 s |
| `complete` | 800 ms |
| `diagnose` | 10 s |

The current Phase 1-6 implementation uses a stricter 5 s deadline for
`diagnose` as well (`src-tauri/src/extensions/provider.rs`, `diagnose`); a
provider MUST therefore be correct when stopped after 5 s. `describe` and
`complete` use manifest-configured values within their schema bounds
(`describeTimeoutMs` up to 5,000 ms and `completeTimeoutMs` up to 3,000 ms);
the default complete budget remains 800 ms.

## Package and Archive Security

Before extraction, `src-tauri/src/extensions/install.rs` verifies the tarball's
NPM SRI digest and rejects absolute paths, `..` components, entries outside the
`package/` root, symbolic links, hard links, unsupported entry types, and archive
size/entry-count limits. This is the path-escape protection implemented by
`safe_unpack` and `safe_archive_path`.

The preferred integrity algorithm is SHA-512 in NPM `dist.integrity` form
(`sha512-<base64>`). The Host compares digests in constant time and rejects a
missing or unsupported digest. NPM lifecycle scripts and package JavaScript are
never executed.

## Consent and Updates

The permission set is part of the install/update review. A package that changes
permissions without a new version is invalid in practice because its published
tarball digest must remain stable. Hosts SHOULD show the old and new sets side by
side and retain the user's decision in the extension lock/audit record.
File imports MUST perform the same review against the resolved package manifest;
permissions embedded in an import document are not user approval.

## References

- Manifest permission enum and compatibility validation: `src-tauri/src/extensions/manifest.rs`
- Provider process, timeout, and output limits: `src-tauri/src/extensions/provider.rs`
- Tar extraction and SRI verification: `src-tauri/src/extensions/install.rs`
- Lifecycle security baseline: `docs/extensions/FEP-3-lifecycle.md`
