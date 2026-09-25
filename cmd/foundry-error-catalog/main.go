// foundry-error-catalog regenerates the canonical JSON artifact describing
// every error code the foundry_vtt plugin can emit, consumed by Foundry-side
// docs that cross-reference the Chronicle error contract.
//
// Run via `make foundry-error-catalog` from the repo root. Writes the JSON to
// internal/plugins/foundry_vtt/error-catalog.json by default; pass -o to
// redirect. Shares its parser with errors_catalog.go so generation and drift
// checks stay in sync. See internal/plugins/foundry_vtt/.ai.md "Adding a new
// error code".
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/keyxmakerx/chronicle/internal/plugins/foundry_vtt"
)

const (
	defaultSourcePath = "internal/plugins/foundry_vtt/errors.go"
	defaultOutputPath = "internal/plugins/foundry_vtt/error-catalog.json"
)

func main() {
	source := flag.String("source", defaultSourcePath, "path to errors.go (relative to CWD)")
	output := flag.String("o", defaultOutputPath, "output JSON path (relative to CWD; use - for stdout)")
	flag.Parse()

	sourceBytes, err := os.ReadFile(*source)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read source %s: %v\n", *source, err)
		os.Exit(1)
	}

	constructors, err := foundry_vtt.ParseConstructors(sourceBytes, *source)
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse: %v\n", err)
		os.Exit(1)
	}

	jsonBytes, err := foundry_vtt.BuildJSONArtifact(constructors)
	if err != nil {
		fmt.Fprintf(os.Stderr, "build JSON: %v\n", err)
		os.Exit(1)
	}

	if *output == "-" {
		_, err = os.Stdout.Write(jsonBytes)
	} else {
		err = os.WriteFile(*output, jsonBytes, 0o644)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "write output: %v\n", err)
		os.Exit(1)
	}
	if *output != "-" {
		fmt.Fprintf(os.Stderr, "wrote %d constructor(s) to %s\n", len(constructors), *output)
	}
}
