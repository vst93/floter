// Command next-version prints the version the next release should carry.
//
// It exists so the release workflow stops deriving that number from
// `mygo.json`: the file is a snapshot of the branch, not of the releases (the
// workflow writes the version into its own workspace and never commits it
// back), so growing a version out of it republished one users already had —
// with `--clobber`, i.e. new bytes under an installed version number, which the
// updater then reports as "current". See internal/versioning for the rule.
//
// Usage:
//
//	next-version -current "$(jq -r .version mygo.json)" mygo-v0.3.24 mygo-v0.3.25
//
// Existing versions come in as arguments (tags, release names, or bare
// numbers); anything unparsable is ignored, so a caller can pass the whole
// `git ls-remote --tags` listing through `sed` without being careful.
package main

import (
	"flag"
	"fmt"
	"os"

	"floter/internal/versioning"
)

func main() {
	current := flag.String("current", "", "the branch's mygo.json version (the floor)")
	flag.Parse()
	next, err := versioning.Next(*current, flag.Args())
	if err != nil {
		fmt.Fprintf(os.Stderr, "next-version: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(next)
}
