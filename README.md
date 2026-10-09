# floter

A cross-platform floating launcher and terminal, always one shortcut away.

**English** · [简体中文](README.zh-CN.md)

floter is a native app: one frameless panel that swaps between a launcher, a
settings surface and a real terminal. The UI is drawn by
[mygo](https://mygo.egoist.dev) on the GPU — there is no webview and no browser
engine in the binary — and everything else is Go.

## Features

- **Launcher** — one shortcut (default `Ctrl+Space`) summons a search field that
  matches installed applications (`.app` / `.lnk` / `.desktop`), expressions
  (`0.1+0.2` answers `0.3`), your clipboard history, browser history and
  bookmarks, the commands installed extensions declare, and optionally the
  executables on your `PATH`.
- **Terminal** — a real terminal with the [ghostty](https://ghostty.org) VT core:
  font, size, cursor, line height, padding and nine colour palettes are settings,
  and a change applies to the running session.
- **Settings** — theme (dark / light / auto), interface size, the glass material
  step, panel opacity, language (English / 简体中文), window behaviour (hide on
  blur, how long a surface survives a hide), terminal appearance, the global
  shortcut (recorded by pressing it) and the integration list.
- **Extensions** — a tool ships a manifest and a provider program; floter reads
  its `describe` answer to learn its commands, runs them in the terminal
  surface, injects the configuration the user filled in, and asks for approval
  before installing anything that declares permissions. Local packages and npm
  packages are both supported, with SRI verification and a safe tarball
  extractor.
- **Clipboard history** — text, images and file lists are captured as you copy,
  deduplicated, searchable from the launcher, and pinnable into their own window.
- **System integration** — one instance, `floter://` deep links, a tray icon, a
  login item, a standard application menu, and a pin-to-window action for
  terminal output.

## Install

Build from source (Go 1.27 or newer):

```sh
go run ./cmd/floter        # run it
go tool mygo build         # package it (dist/<platform>/floter.app + .dmg on macOS)
```

`go tool mygo build` uses the mygo CLI, which is a tool of this module — no
Node, Bun or Rust toolchain is involved.

## Development

```sh
gofmt -l cmd internal      # must print nothing
go vet ./...
go test -count=1 ./...     # the whole suite
GOOS=linux go build ./... && GOOS=windows go build ./...
```

Some tests drive real resources and are opt-in:

```sh
FLOTER_TERMINAL_TEST=1 go test ./internal/terminalui   # a real libghostty-vt session
FLOTER_SQLITE_TEST=1  go test ./internal/browser ./internal/extensions
FLOTER_REGISTRY_TEST=1 go test ./internal/extensions -run TestRealRegistry
```

Set `FLOTER_OPEN=settings` (or `terminal`) to start the packaged app on a
surface other than the launcher.

## Your data

floter reads and writes the same files earlier builds used, so nothing has to be
migrated: `settings.json`, `extension-repository.json`, the installed extension
directories, the clipboard history and the usage record all keep their format,
and every key floter does not understand is preserved verbatim when it writes.
See [`docs/AGENT-NOTES.md`](docs/AGENT-NOTES.md) for the file-by-file list.

## Documentation

- [`docs/mygo-rewrite-status.md`](docs/mygo-rewrite-status.md) — current state,
  the package map, what is left to do.
- [`docs/mygo-rewrite-plan.md`](docs/mygo-rewrite-plan.md) — the round-by-round
  decision record.
- [`docs/extensions/`](docs/extensions/README.zh-CN.md) — the extension format
  (manifest, provider protocol, permissions, declarative configuration).
- [`docs/plugin-development.md`](docs/plugin-development.md) — writing a tool
  that floter can drive.

## License

[GPL-3.0](LICENSE)
