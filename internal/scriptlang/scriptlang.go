// Package scriptlang is the script-language vocabulary of a local tool: the
// eight languages a provider can be written in, and what the editor needs to
// know about each — its file extension, whether it is compiled or interpreted,
// the toolchain that must be on PATH, and the template a new script starts
// from.
//
// The old build's R9-1 put the same table in one place for the same reason:
// three surfaces used to answer "what is this language" separately, so a fourth
// language needed four edits and missing one produced a silently wrong
// extension. The backend stays the source of truth for what can actually run
// (`extensions.ScriptRuntimeFor`); this package is the description.
//
// Language names are proper nouns and are never translated, which is why no
// dictionary key exists for them.
package scriptlang

import "strings"

// Language is one script language.
type Language struct {
	// ID is the wire id: what a manifest stores and what the interpreter
	// lookup is keyed by.
	ID string
	// Label is the name the picker shows: a proper noun, never translated.
	Label string
	// Extension is the file extension without the dot, for the provider file
	// (`provider.py`) and the export dialog's filter.
	Extension string
	// Compiled languages are source-distributed build scripts: the toolchain
	// produces an artifact and *that* is what runs.
	Compiled bool
	// Toolchain is the binary that must be on PATH: the interpreter, or for a
	// compiled language the compiler.
	Toolchain string
	// Template is a minimal runnable script, in both languages' comments.
	Template string
}

// Languages are the eight a provider may be written in, in the order the
// picker lists them.
var Languages = []Language{
	{
		ID: "js", Label: "JavaScript", Extension: "js", Toolchain: "node",
		Template: "#!/usr/bin/env node\n\n// Floter provider script\n// Parameters you declare in the editor arrive as argv, e.g. --target example.com.\n// Read them with process.argv.slice(2).\n// 编辑器里声明的参数会作为 argv 传入，可用 process.argv.slice(2) 读取。\n",
	},
	{
		ID: "shell", Label: "Shell", Extension: "sh", Toolchain: "sh",
		Template: "#!/bin/sh\n\n# Floter provider script\n# Parameters you declare in the editor arrive as argv, e.g. --target example.com.\n# Read them as \"$1\", \"$2\", … or \"$@\".\n# 编辑器里声明的参数会作为 argv 传入，可用 \"$1\"、\"$2\" 或 \"$@\" 读取。\n",
	},
	{
		ID: "powershell", Label: "PowerShell", Extension: "ps1", Toolchain: "pwsh",
		Template: "#!/usr/bin/env pwsh\n\n# Floter provider script\n# Parameters you declare in the editor arrive as argv, e.g. --target example.com.\n# Read them with $args.\n# 编辑器里声明的参数会作为 argv 传入，可用 $args 读取。\n",
	},
	{
		ID: "python", Label: "Python", Extension: "py", Toolchain: "python3",
		Template: "#!/usr/bin/env python3\n\n# Floter provider script\n# Parameters you declare in the editor arrive as argv, e.g. --target example.com.\n# Read them with sys.argv[1:].\n# 编辑器里声明的参数会作为 argv 传入，可用 sys.argv[1:] 读取。\n",
	},
	{
		ID: "ruby", Label: "Ruby", Extension: "rb", Toolchain: "ruby",
		Template: "#!/usr/bin/env ruby\n\n# Floter provider script\n# Parameters you declare in the editor arrive as argv, e.g. --target example.com.\n# Read them with ARGV.\n# 编辑器里声明的参数会作为 argv 传入，可用 ARGV 读取。\n",
	},
	{
		ID: "php", Label: "PHP", Extension: "php", Toolchain: "php",
		Template: "<?php\n\n// Floter provider script\n// Parameters you declare in the editor arrive as argv, e.g. --target example.com.\n// Read them with $argv.\n// 编辑器里声明的参数会作为 argv 传入，可用 $argv 读取。\n",
	},
	{
		ID: "go", Label: "Go", Extension: "go", Compiled: true, Toolchain: "go",
		Template: "package main\n\nimport \"fmt\"\n\nfunc main() {\n\t// Floter provider script\n\t// Parameters you declare in the editor arrive as argv, e.g. --target example.com.\n\t// Read them with os.Args[1:] (add the \"os\" import).\n\t// 编辑器里声明的参数会作为 argv 传入，可用 os.Args[1:] 读取（需引入 \"os\"）。\n\tfmt.Println(\"floter provider\")\n}\n",
	},
	{
		ID: "rust", Label: "Rust", Extension: "rs", Compiled: true, Toolchain: "rustc",
		Template: "fn main() {\n    // Floter provider script\n    // Parameters you declare in the editor arrive as argv, e.g. --target example.com.\n    // Read them with std::env::args().skip(1).\n    // 编辑器里声明的参数会作为 argv 传入，可用 std::env::args().skip(1) 读取。\n    println!(\"floter provider\");\n}\n",
	},
}

// Lookup resolves a language by its id, falling back to the first one so a
// manifest value this build does not know still renders a usable editor
// instead of an empty picker.
func Lookup(id string) Language {
	id = strings.ToLower(strings.TrimSpace(id))
	for _, language := range Languages {
		if language.ID == id {
			return language
		}
	}
	// A manifest may name an interpreter rather than a language id.
	switch id {
	case "javascript", "node":
		return Languages[0]
	case "sh", "bash", "zsh":
		return Languages[1]
	case "pwsh":
		return Languages[2]
	case "python3":
		return Languages[3]
	}
	return Languages[0]
}

// Template is the starting script of a language.
func Template(id string) string { return Lookup(id).Template }

// Extension is a language's file extension, without the dot.
func Extension(id string) string { return Lookup(id).Extension }

// Templates are every language's template, for the "is the editor still
// holding an untouched one?" test a language switch uses to decide whether to
// swap the content.
func Templates() []string {
	out := make([]string, 0, len(Languages))
	for _, language := range Languages {
		out = append(out, language.Template)
	}
	return out
}

// Check is what this machine has for one language, as the runtime lookup
// answers it: whether the interpreter (or, for a compiled language, the
// toolchain) is there, which one, and what it reports as its version. The
// zero Check means "not asked yet" — never "missing".
type Check struct {
	Available bool
	Path      string
	Version   string
	Compiled  bool
	// Candidates are the names that were looked for, in order: what a user
	// without the toolchain needs to install.
	Candidates []string
}

// State is the three claims the status line may make: we have not looked, we
// looked and it is there, or we looked and it is not.
type State int

const (
	// Unknown is "not asked yet".
	Unknown State = iota
	// Available is "the toolchain is on PATH".
	Available
	// Missing is "it is not on PATH".
	Missing
)

// Status is the inline line under the language picker, as data. An absent
// toolchain is deliberately *not* an error: a user may install Python and come
// back, so it projects to a muted line rather than a red banner — and the three
// states are distinct because "we have not looked" and "we looked and it is
// missing" are different claims.
type Status struct {
	State State
	// Name is the interpreter's own file name when it is there, and the
	// language's toolchain name otherwise.
	Name string
	// Version is what the toolchain reported, empty when it reported none.
	Version string
	// Path is where it was found, for a tooltip.
	Path string
	// Compiled reports a language whose toolchain builds the source.
	Compiled bool
	// Names are what a missing toolchain needs installed.
	Names []string
}

// StatusOf projects a check for a language into the line the editor shows.
func StatusOf(language string, check Check) Status {
	option := Lookup(language)
	if check.Path == "" && !check.Available {
		names := check.Candidates
		if len(names) == 0 {
			names = []string{option.Toolchain}
		}
		return Status{State: Missing, Name: option.Toolchain, Names: names, Compiled: option.Compiled}
	}
	name := option.Toolchain
	if check.Path != "" {
		if base := baseName(check.Path); base != "" {
			name = base
		}
	}
	return Status{
		State:    Available,
		Name:     name,
		Version:  check.Version,
		Path:     check.Path,
		Compiled: check.Compiled || option.Compiled,
	}
}

// baseName is the last segment of a path, in either platform's spelling.
func baseName(path string) string {
	if index := strings.LastIndexAny(path, `/\`); index >= 0 {
		return path[index+1:]
	}
	return path
}
