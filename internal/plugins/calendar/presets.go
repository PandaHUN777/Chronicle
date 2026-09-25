package calendar

import (
	"embed"
	"fmt"
	"io/fs"
	"sort"
)

// presets.go — the shipped calendar presets, kept alive through the importer.
//
// A preset IS an export: it goes through the exact same DetectAndParse path a
// user's uploaded file takes, so a malformed preset fails exactly where a
// malformed upload fails.

//go:embed presets/*.json
var presetFS embed.FS

// PresetNames returns the available preset ids, sorted for determinism (see
// import_order_determinism_test.go).
func PresetNames() ([]string, error) {
	entries, err := fs.Glob(presetFS, "presets/*.json")
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, e[len("presets/"):len(e)-len(".json")])
	}
	sort.Strings(out)
	return out, nil
}

// LoadPreset parses one shipped preset through the ordinary import path.
//
// The return type is the importer's own ImportResult on purpose — a caller
// cannot tell a preset from an uploaded file, which is the whole point of the
// ruling above.
func LoadPreset(name string) (*ImportResult, error) {
	data, err := presetFS.ReadFile("presets/" + name + ".json")
	if err != nil {
		return nil, fmt.Errorf("unknown preset %q: %w", name, err)
	}
	return DetectAndParse(data)
}
