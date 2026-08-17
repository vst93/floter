# FEP-6：Declarative Configuration

状态：Draft 1

FEP-6 lets an extension describe configuration as data. The Host owns the
settings UI, validation, persistence, and injection into Provider invocations;
the extension supplies a schema and consumes the resulting values.

## Configuration Descriptor

An extension declares configuration in the `configuration` field of its
`provider-description.json`. In the current executable protocol, the Provider's
`config` operation returns that field in a JSON envelope. A host-managed
descriptor has `owner: "host"` and a non-empty `schema`; a tool-managed
descriptor has `owner: "tool"`, no host schema, and an `openCommand` that the
Host runs in a PTY. This is the same descriptor consumed by
`src-tauri/src/extensions/config.rs` and rendered by `src/ExtensionsPanel.tsx`.

```json
{
  "configuration": {
    "configVersion": 1,
    "owner": "host",
    "schema": [
      {
        "key": "endpoint",
        "type": "text",
        "label": "API endpoint",
        "description": "Base URL used by the provider",
        "required": true,
        "default": "https://api.example.com"
      },
      {
        "key": "token",
        "type": "password",
        "label": "Access token",
        "required": true,
        "envVar": "EXAMPLE_TOKEN"
      },
      {
        "key": "retries",
        "type": "number",
        "label": "Retries",
        "minimum": 0,
        "maximum": 10,
        "default": 3
      },
      {
        "key": "region",
        "type": "select",
        "options": ["us", "eu"],
        "default": "us"
      },
      {
        "key": "telemetry",
        "type": "boolean",
        "default": false,
        "argument": "--telemetry"
      }
    ]
  }
}
```

The required Draft 1 field types are `text`, `number`, `password`, `select`,
and `boolean`. The current Rust and TypeScript implementations additionally
support `path` and `multi-select`; these extensions use the same validation and
rendering rules.

Field keys are unique non-empty identifiers. `select` and `multi-select` fields
provide an `options` array. `required`, `label`, and `description` are optional
metadata except that a required field must have a value (or a valid default).

## Defaults and Validation

Each field MAY provide a `default` value. In prose this is the **defaultValue**
of the field; `default` is the Draft 1 wire name emitted by `config.rs` and
consumed by `ExtensionsPanel.tsx`. Defaults are validated exactly like user
values and are used when no stored value exists.

The Host validates, in order:

1. Schema shape: field keys, duplicate keys, field types, required fields, and
   the presence of options for `select`.
2. Value types: strings for text/password/path/select, arrays of strings for
   multi-select, booleans for boolean, and JSON numbers for number.
3. Constraints: `minimum`/`maximum` for numbers, `minLength`/`maxLength` for
   text, and membership in `options` for select values. A descriptor MAY also
   name a custom `validate` function for validation that cannot be expressed by
   these constraints.

Unknown value keys and invalid values are rejected by `validate_values` in
`src-tauri/src/extensions/config.rs`. It enforces type, required, enum, numeric
range, and text length checks. A custom validation rule, when supported by a
future protocol version, MUST be deterministic, side-effect free, and run before
persistence.

## Persistence and Scope

Host-managed values are persisted atomically as JSON and kept separate from the
installed program. The logical FEP path is:

```text
extensions/<id>/config.json
```

In the current Floter layout, the data root is deliberately separate from the
program root, so the concrete path is
`extension-data/<id>/config.json` (see `ExtensionPaths::from_root` and
`values_path` in `src-tauri/src/extensions/config.rs`). The stored object
contains `configVersion`, redacted public `values`, and the schema snapshot used
for migration and compatibility:

```json
{
  "configVersion": 2,
  "values": {
    "endpoint": "https://api.example.com",
    "retries": 3,
    "telemetry": false
  },
  "schema": []
}
```

Password values are written as `[REDACTED]` in `config.json`. Their actual values
are stored in an access-restricted immutable generation under `config-secrets/`;
`config.json` names the committed `secretGeneration` and is the single atomic
commit pointer. A save writes, protects, and fsyncs the secret generation before
atomically replacing `config.json`, so public values and secrets cannot be mixed
across saves. Floter serializes configuration mutations and, at startup, removes
password placeholders safely when a referenced generation is missing or invalid.
The legacy adjacent `config.secrets.json` is read for migration and removed after
the next successful save.

Configuration is global to an installed extension by default. A future revision
may add a session/terminal scope; scoped values MUST be layered over global
values without changing the persisted global object.

## Schema Versions and Migration

Providers SHOULD expose a positive `configVersion` in their configuration descriptor.
When the version changes, the Host compares the stored schema
snapshot with the new descriptor before rendering or injecting values:

1. Keep values whose keys and types remain compatible.
2. Replace removed or incompatible values with their new defaults.
3. Add new fields using `default` (or leave them unset when optional).
4. If a provider supplies a migration operation, run it in a bounded,
   side-effect-free `config` call; otherwise preserve compatible values and
   report the migration in the UI.

Descriptors that omit `configVersion` default to version 1. The Host persists
the version and schema snapshot. When either changes, compatible values are
retained, incompatible values are discarded, and newly introduced defaults are
materialized automatically.

## Injection into Provider Execution

Fields may map to an environment variable with `envVar` and/or to an argv flag
with `argument`. The legacy `environment` spelling remains accepted. A
descriptor-level `environmentMapping` object may also map field keys to variable
names. Before `describe`, `complete`, `diagnose`, or command
execution, the Host applies persisted values: scalar values are stringified,
arrays are comma-joined, false booleans and null values do not emit an argument,
and environment entries are added to the structured execution environment.
No value is interpolated into a shell string.

Password fields MUST be rendered as secret inputs. Hosts SHOULD avoid exposing
their values in logs, diagnostics, command previews, IPC responses, or crash
reports. Floter returns password placeholders from configuration IPC and keeps
extension command program/argv/environment data behind a single-use backend
execution-plan token, so injected secrets are resolved only when the terminal
session is spawned.

## Configuration UI

For `owner: "host"`, the Host automatically renders labels, descriptions,
required markers, text/password/number inputs, select controls, multi-select
controls, and a boolean switch. `ExtensionsPanel.tsx` submits values through the
`extensions_config_set` command only after the same server-side validation. For
`owner: "tool"`, the panel presents an “open configuration” action and does not
attempt to parse or persist tool-owned settings.

## References

- Configuration model, validation, storage, and injection: `src-tauri/src/extensions/config.rs`
- Provider process protocol and `config` operation: `src-tauri/src/extensions/provider.rs`
- Automatic form rendering: `src/ExtensionsPanel.tsx`
