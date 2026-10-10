package extensions

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func argumentNames(arguments []DerivedArgument) []string {
	var names []string
	for _, argument := range arguments {
		names = append(names, strings.Join(argument.Names, ","))
	}
	return names
}

// The clap-style block: two-column option definitions with placeholders and
// descriptions.
func TestDerivesClapStyleArguments(t *testing.T) {
	help := "Usage: demo [OPTIONS]\n\n" +
		"Options:\n" +
		"  -o, --output <FILE>    Write result to FILE\n" +
		"      --verbose          Enable verbose logging\n" +
		"  -j, --threads N        number of threads\n" +
		"  -C, --dir <DIR>        Working directory\n"
	arguments := DeriveArguments(help)
	if got := argumentNames(arguments); len(got) != 4 ||
		got[0] != "-o,--output" || got[1] != "--verbose" || got[2] != "-j,--threads" || got[3] != "-C,--dir" {
		t.Fatalf("names = %v", got)
	}
	if !arguments[0].TakesValue || arguments[0].ValueHint != "FILE" || arguments[0].Kind != "path" {
		t.Errorf("output = %+v", arguments[0])
	}
	if arguments[0].Description != "Write result to FILE" {
		t.Errorf("description = %q", arguments[0].Description)
	}
	if arguments[1].TakesValue || arguments[1].Kind != "flag" || arguments[1].Description != "Enable verbose logging" {
		t.Errorf("verbose = %+v", arguments[1])
	}
	if !arguments[2].TakesValue || arguments[2].ValueHint != "N" {
		t.Errorf("threads = %+v", arguments[2])
	}
	if arguments[3].Kind != "directory" {
		t.Errorf("dir kind = %q", arguments[3].Kind)
	}
}

// The argparse style: a wrapped definition whose description is the
// indented line that follows.
func TestDerivesArgparseStyleArguments(t *testing.T) {
	help := "usage: prog [-h] [--count COUNT] [--mode MODE] source\n\n" +
		"options:\n" +
		"  -h, --help            show this help message and exit\n" +
		"  -c COUNT, --count COUNT\n" +
		"                        number of items to process\n" +
		"  --mode MODE           operation mode\n"
	arguments := DeriveArguments(help)
	if got := argumentNames(arguments); len(got) != 2 || got[0] != "-c,--count" || got[1] != "--mode" {
		t.Fatalf("names = %v", got)
	}
	if arguments[0].Description != "number of items to process" {
		t.Errorf("count description = %q", arguments[0].Description)
	}
	if arguments[1].Description != "operation mode" {
		t.Errorf("mode description = %q", arguments[1].Description)
	}
}

// The Go flag style: an indented definition whose prose follows on the next
// line, with a `(default …)` trailer that is dropped.
func TestDerivesGoFlagStyleArguments(t *testing.T) {
	help := "Usage of demo:\n" +
		"  -j int\n" +
		"    \tnumber of threads (default 4)\n" +
		"  -verbose\n" +
		"    \tenable verbose logging\n"
	arguments := DeriveArguments(help)
	if got := argumentNames(arguments); len(got) != 2 || got[0] != "-j" || got[1] != "-verbose" {
		t.Fatalf("names = %v", got)
	}
	if !arguments[0].TakesValue || arguments[0].Description != "number of threads" {
		t.Errorf("j = %+v", arguments[0])
	}
	if arguments[1].TakesValue || arguments[1].Description != "enable verbose logging" {
		t.Errorf("verbose = %+v", arguments[1])
	}
}

// Chinese usage lines, URLs and section headers never become options.
func TestSkipsChineseUsageLinesURLsAndHeaders(t *testing.T) {
	help := "用法：demo [选项]\n\n" +
		"选项：\n" +
		"  -o, --output <FILE>    输出文件\n" +
		"  文档见 https://example.com/docs\n"
	arguments := DeriveArguments(help)
	if got := argumentNames(arguments); len(got) != 1 || got[0] != "-o,--output" {
		t.Fatalf("names = %v", got)
	}
	if arguments[0].Description != "输出文件" {
		t.Errorf("description = %q", arguments[0].Description)
	}
}

// Help and version options stay excluded per module policy.
func TestExcludesHelpAndVersionOptions(t *testing.T) {
	help := "Options:\n" +
		"  -h, --help       Print help\n" +
		"  -V, --version    Print version\n" +
		"  -v, --verbose    Verbose output\n"
	arguments := DeriveArguments(help)
	if got := argumentNames(arguments); len(got) != 1 || got[0] != "-v,--verbose" {
		t.Fatalf("names = %v", got)
	}
}

// The same option reached through wrapped or repeated listings is only
// suggested once.
func TestDedupesRepeatedDefinitions(t *testing.T) {
	help := "Options:\n" +
		"  -o, --output <FILE>    Write result to FILE\n" +
		"  --output <FILE>        Aliased listing of the same option\n" +
		"  --input <FILE>         Read from FILE\n"
	arguments := DeriveArguments(help)
	if got := argumentNames(arguments); len(got) != 2 || got[0] != "-o,--output" || got[1] != "--input" {
		t.Fatalf("names = %v", got)
	}
}

// Value placeholders across the bracketed, bare-uppercase and Go-type
// styles.
func TestDetectsValuePlaceholdersAcrossStyles(t *testing.T) {
	for _, pair := range [][2]string{
		{"<FILE>", "FILE"}, {"[COUNT]", "COUNT"}, {"(DIR)", "DIR"},
		{"N", "N"}, {"int", "int"},
	} {
		if got := placeholderValue(pair[0]); got != pair[1] {
			t.Errorf("placeholderValue(%q) = %q, want %q", pair[0], got, pair[1])
		}
	}
	for _, token := range []string{"enable", "--flag", ""} {
		if got := placeholderValue(token); got != "" {
			t.Errorf("placeholderValue(%q) = %q, want none", token, got)
		}
	}
}

// A flag without a value takes none.
func TestFlagsWithoutValuesTakeNoValue(t *testing.T) {
	arguments := DeriveArguments("--quiet                   suppress output")
	if len(arguments) != 1 || arguments[0].TakesValue || arguments[0].ValueHint != "" {
		t.Errorf("arguments = %+v", arguments)
	}
}

// The cap holds and garbage never panics.
func TestCapsResultsAndNeverPanicsOnGarbage(t *testing.T) {
	var builder strings.Builder
	builder.WriteString("Options:\n")
	for index := 0; index < 60; index++ {
		builder.WriteString("  --opt-" + strings.Repeat("x", index+1) + "            option\n")
	}
	if arguments := DeriveArguments(builder.String()); len(arguments) != maxDerivedArguments {
		t.Errorf("arguments = %d, want the cap", len(arguments))
	}
	for _, garbage := range []string{"", "---- ==== <<<>>>", "-", "random text"} {
		if arguments := DeriveArguments(garbage); len(arguments) != 0 {
			t.Errorf("garbage %q derived %v", garbage, arguments)
		}
	}
}

// The v-style plugin listing: glyph-decorated rows with version tokens,
// alias groups and indented descriptions.
func TestParsesVStylePluginRows(t *testing.T) {
	help := "v - Gadgets under the terminal\n" +
		"Version: dev  🏠 https://github.com/vst93/v\n\n" +
		"Available Plugins\n" +
		"==================================================\n" +
		"📦 json2excel 0.0.1 👤 vst  (aliases: j2e)\n" +
		"  convert json data to excel file\n\n" +
		"📦 jv 1.0.0 👤 vst\n" +
		"  JSON Viewer & Formatter - format, compress, escape, ...\n" +
		"📦 codec 0.3 (aliases: cc, enc)\n\n" +
		"Run v <command> -h for detailed help.\n"
	subcommands := DeriveSubcommands(help)
	var names []string
	for _, sub := range subcommands {
		names = append(names, sub.Name)
	}
	if len(names) != 3 || names[0] != "json2excel" || names[1] != "jv" || names[2] != "codec" {
		t.Fatalf("names = %v", names)
	}
	if len(subcommands[0].Aliases) != 1 || subcommands[0].Aliases[0] != "j2e" {
		t.Errorf("aliases = %v", subcommands[0].Aliases)
	}
	if subcommands[0].Description != "convert json data to excel file" {
		t.Errorf("description = %q", subcommands[0].Description)
	}
	if subcommands[1].Description != "JSON Viewer & Formatter - format, compress, escape, ..." {
		t.Errorf("jv description = %q", subcommands[1].Description)
	}
	if len(subcommands[2].Aliases) != 2 || subcommands[2].Description != "" {
		t.Errorf("codec = %+v", subcommands[2])
	}
}

// An indented row inside a Commands/Plugins section follows the cobra shape
// and requires the wide definition-column gap; an indented single-spaced row
// matches neither documented listing style and must yield no entry.
func TestRejectsIndentedSectionRowWithoutWideGap(t *testing.T) {
	help := "Available Plugins\n" +
		"  alpha 1.0.0 (aliases: al)\n" +
		"    First gadget\n"
	if subcommands := DeriveSubcommands(help); len(subcommands) != 0 {
		t.Errorf("indented single-spaced row derived %v", subcommands)
	}
	// The same row unindented is the documented v-style listing shape.
	help = "Available Plugins\n" +
		"alpha 1.0.0 (aliases: al)\n" +
		"    First gadget\n"
	subcommands := DeriveSubcommands(help)
	if len(subcommands) != 1 || subcommands[0].Name != "alpha" || len(subcommands[0].Aliases) != 1 {
		t.Errorf("unindented row = %+v", subcommands)
	}
}

// The cobra `Available Commands:` section: name + wide gap + description.
func TestParsesCobraAvailableCommandsSections(t *testing.T) {
	help := "Usage:\n" +
		"  mycli [command]\n\n" +
		"Available Commands:\n" +
		"  get         Get something from somewhere\n" +
		"  set         Set something useful\n" +
		"  completion  Generate the autocompletion script\n" +
		"  help        Help about any command\n\n" +
		"Flags:\n" +
		"  -h, --help   help for mycli\n\n" +
		"Use \"mycli [command] --help\" for more information.\n"
	subcommands := DeriveSubcommands(help)
	if len(subcommands) != 2 || subcommands[0].Name != "get" || subcommands[0].Description != "Get something from somewhere" {
		t.Fatalf("subcommands = %+v", subcommands)
	}
	if subcommands[1].Name != "set" || subcommands[1].Description != "Set something useful" {
		t.Errorf("set = %+v", subcommands[1])
	}
}

// Flag lines, URLs and banners never become subcommands.
func TestFlagLinesURLsAndBannersNeverBecomeSubcommands(t *testing.T) {
	help := "Options:\n" +
		"  -f         Format JSON\n" +
		"  -sort      Sort keys\n" +
		"Docs: https://example.com/docs\n" +
		"mytool - A banner description\n" +
		"usage: mytool [options]\n"
	if subcommands := DeriveSubcommands(help); len(subcommands) != 0 {
		t.Errorf("derived %v", subcommands)
	}
}

// Invalid or prose names are skipped by the strict charset check.
func TestInvalidOrProseNamesAreSkipped(t *testing.T) {
	help := "🚀 Bad-Name 1.0\n" +
		"@@@ !!!\n" +
		"Run demo <command> -h now\n" +
		"ok-name 1.2.3\n" +
		"  indented prose line without a section\n"
	var names []string
	for _, sub := range DeriveSubcommands(help) {
		names = append(names, sub.Name)
	}
	if len(names) != 1 || names[0] != "ok-name" {
		t.Errorf("names = %v", names)
	}
}

// Rows dedupe by name and the result caps.
func TestDedupesSubcommandsAndCapsTheResult(t *testing.T) {
	var builder strings.Builder
	builder.WriteString("Available Commands:\n")
	builder.WriteString("  dup        First occurrence\n")
	builder.WriteString("  dup        Second occurrence\n")
	for index := 0; index < maxDerivedSubcommands+5; index++ {
		builder.WriteString("  cmd" + strings.Repeat("x", index+1) + "       Command\n")
	}
	subcommands := DeriveSubcommands(builder.String())
	if len(subcommands) != maxDerivedSubcommands {
		t.Fatalf("subcommands = %d, want the cap", len(subcommands))
	}
	if subcommands[0].Name != "dup" || subcommands[0].Description != "First occurrence" {
		t.Errorf("first = %+v", subcommands[0])
	}
}

// Version tokens are ignored but numeric names survive.
func TestVersionTokensAreIgnoredButNumericNamesSurvive(t *testing.T) {
	if !isVersionToken("0.0.1") || !isVersionToken("1.10") {
		t.Error("a version token was not recognized")
	}
	if isVersionToken("dev") || isVersionToken("42") {
		t.Error("a non-version token was taken for one")
	}
	subcommands := DeriveSubcommands("📦 7zip 1.0.0 👤 vst\n  zip tool\n")
	if len(subcommands) != 1 || subcommands[0].Name != "7zip" || subcommands[0].Description != "zip tool" {
		t.Errorf("subcommands = %+v", subcommands)
	}
}

// ANSI color codes are stripped before parsing: tools that colorize
// regardless of stdout being a TTY would otherwise poison every token.
func TestANSIColorCodesAreStrippedBeforeParsing(t *testing.T) {
	colored := "\x1b[33;1m📦 jv\x1b[0m \x1b[32m1.0.0\x1b[0m 👤 dev\n  JSON viewer\n"
	subcommands := DeriveSubcommands(colored)
	if len(subcommands) != 1 || subcommands[0].Name != "jv" || subcommands[0].Description != "JSON viewer" {
		t.Errorf("subcommands = %+v", subcommands)
	}
	coloredFlags := "Options:\n" +
		"  \x1b[0;32m-sort\x1b[0m   Sort keys\n" +
		"  \x1b[0;32m-out\x1b[0m <path>  Write path\n"
	arguments := DeriveArguments(coloredFlags)
	if got := argumentNames(arguments); len(got) != 2 || got[0] != "-sort" || got[1] != "-out" {
		t.Fatalf("names = %v", got)
	}
	if arguments[1].ValueHint != "path" {
		t.Errorf("value hint = %q", arguments[1].ValueHint)
	}
}

// The decoration strip keeps option markers and word characters.
func TestDecorationStripKeepsOptionMarkersAndWordCharacters(t *testing.T) {
	for _, pair := range [][2]string{
		{"📦 json2excel", "json2excel"},
		{"\ufe0f👤 vst", "vst"},
		{"=====", ""},
		{"-sort", "-sort"},
		{"--output", "--output"},
		{"plain", "plain"},
	} {
		if got := stripLeadingDecoration(pair[0]); got != pair[1] {
			t.Errorf("stripLeadingDecoration(%q) = %q, want %q", pair[0], got, pair[1])
		}
	}
}

// Punctuation runs and meta lines are ignored, alone and together.
func TestPunctuationRunsAndMetaLinesAreIgnored(t *testing.T) {
	for _, line := range []string{"====", "----", "───"} {
		if !isPunctuationRun(line) {
			t.Errorf("%q is not a punctuation run", line)
		}
	}
	if isPunctuationRun("-sort   Sort keys") {
		t.Error("an option line is a punctuation run")
	}
	for _, line := range []string{"Version: dev  🏠 https://example.com", "Run v <command> -h for detailed help.", "run demo --help now"} {
		if !isMetaLine(line) {
			t.Errorf("%q is not a meta line", line)
		}
	}
	if isMetaLine("jv - JSON Viewer v1.0.0") {
		t.Error("a plugin row is a meta line")
	}
	noise := "Version: 2.0\n============\n----\nRun tool -h for details.\n"
	if subcommands := DeriveSubcommands(noise); len(subcommands) != 0 {
		t.Errorf("noise derived %v", subcommands)
	}
	if arguments := DeriveArguments(noise); len(arguments) != 0 {
		t.Errorf("noise derived %v", arguments)
	}
}

// Single-dash long options accept placeholders.
func TestSingleDashLongOptionsAcceptPlaceholders(t *testing.T) {
	plain, ok := parseOptionLine("  -sort   Sort object keys alphabetically")
	if !ok || len(plain.names) != 1 || plain.names[0] != "-sort" || plain.valueHint != "" {
		t.Errorf("plain = %+v ok=%v", plain, ok)
	}
	bracketed, ok := parseOptionLine("  -k <a.b.c>   Key path description")
	if !ok || bracketed.valueHint != "a.b.c" {
		t.Errorf("bracketed = %+v ok=%v", bracketed, ok)
	}
	upper, ok := parseOptionLine("  -k VALUE   Key description")
	if !ok || upper.valueHint != "VALUE" {
		t.Errorf("upper = %+v ok=%v", upper, ok)
	}
}

// Compact summary lines yield flag arguments.
func TestCompactSummaryLinesYieldFlagArguments(t *testing.T) {
	raws := parseSummaryFlags("I/O: -pipe (auto) \u00b7 -file <path> \u00b7 -url <u> \u00b7 -clip \u00b7 -copy")
	var extracted [][2]string
	for _, raw := range raws {
		extracted = append(extracted, [2]string{raw.names[0], raw.valueHint})
	}
	want := [][2]string{
		{"-pipe", ""}, {"-file", "path"}, {"-url", "u"}, {"-clip", ""}, {"-copy", ""},
	}
	if len(extracted) != len(want) {
		t.Fatalf("extracted = %v", extracted)
	}
	for index, pair := range want {
		if extracted[index] != pair {
			t.Errorf("extracted[%d] = %v, want %v", index, extracted[index], pair)
		}
	}
	// Too few flags, overlong labels, stray prose words, and priority
	// chains without dashes are rejected wholesale.
	for _, line := range []string{
		"I/O: -a -b",
		"Longer-label: -a -b -c",
		"Note: see -a and -b below",
		"Priority: pipe > -file > -url > clipboard",
		"no label at all",
	} {
		if raws := parseSummaryFlags(line); len(raws) != 0 {
			t.Errorf("summary %q derived %v", line, raws)
		}
	}
}

// Author tokens after a version never become the name.
func TestAuthorTokensAfterVersionNeverBecomeTheName(t *testing.T) {
	subcommands := DeriveSubcommands("📦 toolname 1.2.3 👤 someone\n  does things\n")
	if len(subcommands) != 1 || subcommands[0].Name != "toolname" || subcommands[0].Description != "does things" {
		t.Errorf("subcommands = %+v", subcommands)
	}
}

// The verbatim second-level help of a hand-rolled Go tool with a Modes
// block and a compact one-line flag summary.
func TestParsesSingleDashOptionsAndSummaryLineVerbatim(t *testing.T) {
	help := "--------------------------------------------------\n" +
		"jv - JSON Viewer & Formatter v1.0.0\n\n" +
		"Usage:\n" +
		"  v jv [flags]           Read from clipboard (default)\n" +
		"  v jv -file       Read from file\n" +
		"  echo '{...}' | v jv    Read from pipe/stdin\n\n" +
		"Modes:\n" +
		"  (default)  Interactive tree viewer (browse, fold/unfold, copy, edit)\n" +
		"  -f         Format (pretty-print) JSON\n" +
		"  -c         Compress (minify) JSON\n" +
		"  -e         Escape non-ASCII to \\uXXXX\n" +
		"  -u         Unescape \\uXXXX to UTF-8\n\n" +
		"Options:\n" +
		"  -sort   Sort object keys alphabetically\n" +
		"  -raw    Disable colored output (with -f)\n\n" +
		"I/O: -pipe (auto) \u00b7 -file  \u00b7 -url  \u00b7 -clip \u00b7 -out  \u00b7 -copy \u00b7 -h\n" +
		"     Priority: pipe > -file > -url > clipboard\n\n" +
		"Non-JSON input is opened as plain editable text.\n" +
		"Press ? inside the viewer for the full key reference.\n" +
		"--------------------------------------------------\n"
	arguments := DeriveArguments(help)
	var flat []string
	for _, argument := range arguments {
		flat = append(flat, argument.Names...)
	}
	for _, expected := range []string{"-sort", "-raw", "-f", "-c", "-e", "-u", "-pipe", "-file", "-url", "-clip", "-out", "-copy"} {
		if !contains(flat, expected) {
			t.Errorf("missing %s in %v", expected, flat)
		}
	}
	// Help/version stay excluded, and neither usage rows nor prose become
	// arguments.
	if contains(flat, "-h") {
		t.Error("-h was derived")
	}
	if len(arguments) != len(flat) {
		t.Errorf("multi-name arguments leaked: %v", arguments)
	}
	for index, argument := range arguments {
		if argument.Kind != "flag" {
			t.Errorf("arguments[%d] kind = %q", index, argument.Kind)
		}
	}
	if arguments[4].Names[0] != "-sort" || arguments[4].Description != "Sort object keys alphabetically" {
		t.Errorf("sort = %+v", arguments[4])
	}
	if arguments[5].Description != "Disable colored output (with -f)" {
		t.Errorf("raw = %+v", arguments[5])
	}
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

// The connect-time derivation against a real tool: a shell script that
// answers --help with a plugin listing, and whose plugins answer their own
// help.
func TestProbeDerivesFromRealHelp(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fixture is a shell script")
	}
	dir := t.TempDir()
	program := filepath.Join(dir, "tool.sh")
	script := "#!/bin/sh\n" +
		`if [ "$1" = "--help" ]; then` + "\n" +
		"  cat <<'ROOT'\n" +
		"tool - Gadgets under the terminal\n" +
		"Available Plugins\n" +
		"==================================================\n" +
		"📦 json2excel 0.0.1 👤 vst  (aliases: j2e)\n" +
		"  convert json data to excel file\n" +
		"📦 jv 1.0.0 👤 vst\n" +
		"  JSON Viewer & Formatter\n" +
		"ROOT\n" +
		`elif [ "$2" = "--help" ]; then` + "\n" +
		"  cat <<'SUB'\n" +
		"Options:\n" +
		"  -o, --output <FILE>    Write result to FILE\n" +
		"SUB\n" +
		"else\n" +
		"  exit 3\n" +
		"fi\n"
	if err := os.WriteFile(program, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	paths := FromRoot(dir)
	if err := os.MkdirAll(paths.Extensions, 0o755); err != nil {
		t.Fatal(err)
	}
	integration := Integration{
		Entry: Entry{
			ID:             "dev.floter.tool",
			ExecutablePath: program,
		},
		Manifest: Manifest{
			ID:      "dev.floter.tool",
			Runtime: Runtime{Type: "system", ExecutableNames: []string{"tool.sh"}},
		},
		Paths: paths,
	}
	derivation := ProbeDerive(context.Background(), integration)
	if len(derivation.RootArguments) != 0 {
		t.Errorf("root arguments = %v", derivation.RootArguments)
	}
	if len(derivation.Subcommands) != 2 {
		t.Fatalf("subcommands = %+v", derivation.Subcommands)
	}
	first, second := derivation.Subcommands[0], derivation.Subcommands[1]
	if first.Name != "json2excel" || first.Description != "convert json data to excel file" {
		t.Errorf("first = %+v", first)
	}
	if second.Name != "jv" || second.Description != "JSON Viewer & Formatter" {
		t.Errorf("second = %+v", second)
	}
	// The second-level probe ran (the fixture answers `<name> --help`),
	// so the real flags came back.
	if len(second.Arguments) != 1 || second.Arguments[0].Names[0] != "-o" ||
		second.Arguments[0].ValueHint != "FILE" {
		t.Errorf("jv arguments = %+v", second.Arguments)
	}
}

// A tool whose runtime does not resolve derives nothing and does not fail.
func TestProbeDeriveToleratesAMissingRuntime(t *testing.T) {
	dir := t.TempDir()
	paths := FromRoot(dir)
	if err := os.MkdirAll(paths.Extensions, 0o755); err != nil {
		t.Fatal(err)
	}
	integration := Integration{
		Entry:    Entry{ID: "dev.floter.gone"},
		Manifest: Manifest{ID: "dev.floter.gone", Runtime: Runtime{Type: "system", ExecutableNames: []string{"no-such-tool"}}},
		Paths:    paths,
	}
	derivation := ProbeDerive(context.Background(), integration)
	if !derivation.Empty() {
		t.Errorf("derivation = %+v", derivation)
	}
}

// The first real semver in a version output is the tool's version: plain,
// v-prefixed, amid noise words, across lines, two-component completed,
// pre-release preserved, punctuation tolerated.
func TestSemverFromVersionOutputExtractsTheFirstRealVersion(t *testing.T) {
	for _, pair := range [][2]string{
		{"14.1.0", "14.1.0"},
		{"v1.2.3", "1.2.3"},
		{"git version 2.42.0", "2.42.0"},
		{"tool 3.4.5\nCopyright 2024", "3.4.5"},
		{"v1.2", "1.2.0"},
		{"app 2.0.0-rc.1", "2.0.0-rc.1"},
		{"(tool 1.0.0)", "1.0.0"},
	} {
		if got := SemverFromVersionOutput(pair[0]); got != pair[1] {
			t.Errorf("SemverFromVersionOutput(%q) = %q, want %q", pair[0], got, pair[1])
		}
	}
}

// Garbage in, empty out: no digits, no semver shape, or an empty output
// must all yield "" rather than a guessed constant.
func TestSemverFromVersionOutputNeverFabricatesAVersion(t *testing.T) {
	for _, output := range []string{"", "   \n\t ", "no version here", "build 20240101", "release 1", "a.b.c"} {
		if got := SemverFromVersionOutput(output); got != "" {
			t.Errorf("SemverFromVersionOutput(%q) = %q, want none", output, got)
		}
	}
}
