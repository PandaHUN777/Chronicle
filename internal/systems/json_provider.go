package systems

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// JSONProvider loads reference data from JSON files in a module's data
// directory. Each file corresponds to one category (e.g., spells.json →
// "spells" category). Files contain arrays of ReferenceItem objects.
// Data is loaded into memory at construction time and served read-only.
type JSONProvider struct {
	moduleID string
	dataDir  string
	items    map[string][]ReferenceItem
}

// NewJSONProvider scans dataDir for *.json files, loading each as a
// category of ReferenceItem objects. The filename stem becomes the
// category slug (e.g., "spells.json" → "spells"). Returns an error
// if the directory cannot be read or a JSON file is malformed.
func NewJSONProvider(moduleID, dataDir string) (*JSONProvider, error) {
	return newJSONProvider(moduleID, dataDir, RecordEvent)
}

// newJSONProvider is the shared constructor behind NewJSONProvider. sink
// receives one aggregated diagnostic per skipped-item file (see
// normalizeReferenceItems); pass nil to suppress diagnostics entirely, as
// preview/dry-run paths must not mutate the global admin-diagnostics ring
// as a side effect of inspecting a package.
func newJSONProvider(moduleID, dataDir string, sink func(LoadEvent)) (*JSONProvider, error) {
	p := &JSONProvider{
		moduleID: moduleID,
		dataDir:  dataDir,
		items:    make(map[string][]ReferenceItem),
	}

	entries, err := os.ReadDir(dataDir)
	if err != nil {
		if os.IsNotExist(err) {
			// No data directory is valid — widget-only systems don't need
			// reference data. Return an empty provider.
			return p, nil
		}
		return nil, fmt.Errorf("reading data dir %s: %w", dataDir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		category := strings.TrimSuffix(entry.Name(), ".json")
		filePath := filepath.Join(dataDir, entry.Name())

		data, err := os.ReadFile(filePath)
		if err != nil {
			slog.Warn("skipping unreadable data file",
				slog.String("file", filePath),
				slog.String("error", err.Error()),
			)
			continue
		}

		var items []ReferenceItem
		if err := json.Unmarshal(data, &items); err != nil {
			// Skip files that aren't arrays of ReferenceItem. Game system
			// packages may include widget-specific data files in formats
			// that don't match the reference item schema.
			slog.Warn("skipping non-reference data file",
				slog.String("file", filePath),
				slog.String("module", moduleID),
				slog.String("error", err.Error()),
			)
			continue
		}

		items = normalizeReferenceItems(items, moduleID, category, filePath, sink)

		p.items[category] = items
	}

	return p, nil
}

// normalizeReferenceItems stamps each item with moduleID/category and
// normalizes ID: prefer the source's own "id", else fall back to "slug"
// (Chronicle-Draw-Steel docs/DATA-SCHEMA.md; source data keys by slug and
// never sets "id"). Shared by the on-disk loader and ZIP preview; Get()
// consumes only the normalized ID.
//
// An item with neither id nor slug is unaddressable and is skipped rather
// than loaded with a colliding blank ID; a duplicate normalized ID within a
// category is also skipped, since Get() is first-match-wins and a kept
// duplicate would resolve links to the wrong item.
//
// Skips are aggregated into one sink call per invocation (with a count) so
// one bad file can't evict the whole fixed-capacity diagnostics ring. sink
// may be nil (preview/dry-run paths must not mutate global state).
func normalizeReferenceItems(items []ReferenceItem, moduleID, category, source string, sink func(LoadEvent)) []ReferenceItem {
	seen := make(map[string]bool, len(items))
	var missing, duplicate int

	normalized := items[:0]
	for i := range items {
		items[i].SystemID = moduleID
		items[i].Category = category
		if items[i].ID == "" {
			items[i].ID = items[i].Slug
		}
		if items[i].ID == "" {
			missing++
			continue
		}
		if seen[items[i].ID] {
			duplicate++
			continue
		}
		seen[items[i].ID] = true
		normalized = append(normalized, items[i])
	}

	if missing > 0 || duplicate > 0 {
		slog.Warn("skipped reference items during normalization",
			slog.String("source", source),
			slog.String("module", moduleID),
			slog.String("category", category),
			slog.Int("missing_id_and_slug", missing),
			slog.Int("duplicate_id", duplicate),
		)
		if sink != nil {
			sink(LoadEvent{
				SystemID: moduleID,
				Kind:     EventSkipped,
				Source:   "data-item",
				Error: fmt.Sprintf("category %q: skipped %d item(s) (%d missing id/slug, %d duplicate id)",
					category, missing+duplicate, missing, duplicate),
				Dir: source,
			})
		}
	}

	return normalized
}

// List returns all reference items in the given category.
// Returns an empty slice if the category does not exist.
func (p *JSONProvider) List(category string) ([]ReferenceItem, error) {
	items, ok := p.items[category]
	if !ok {
		return []ReferenceItem{}, nil
	}
	return items, nil
}

// Get returns a single reference item by category and ID (slug).
// Returns nil and no error if the item does not exist.
func (p *JSONProvider) Get(category string, id string) (*ReferenceItem, error) {
	items, ok := p.items[category]
	if !ok {
		return nil, nil
	}
	for i := range items {
		if items[i].ID == id {
			return &items[i], nil
		}
	}
	return nil, nil
}

// Search returns reference items matching the query string across all
// categories. Matches case-insensitively against Name, Summary, and Tags.
func (p *JSONProvider) Search(query string) ([]ReferenceItem, error) {
	if query == "" {
		return []ReferenceItem{}, nil
	}

	q := strings.ToLower(query)
	var results []ReferenceItem

	for _, items := range p.items {
		for _, item := range items {
			if matchesQuery(item, q) {
				results = append(results, item)
			}
		}
	}

	return results, nil
}

// Categories returns the list of available data category slugs, sorted.
func (p *JSONProvider) Categories() []string {
	cats := make([]string, 0, len(p.items))
	for k := range p.items {
		cats = append(cats, k)
	}
	sort.Strings(cats)
	return cats
}

// matchesQuery checks if a reference item matches the query (lowercase)
// against name, summary, or any tag.
func matchesQuery(item ReferenceItem, query string) bool {
	if strings.Contains(strings.ToLower(item.Name), query) {
		return true
	}
	if strings.Contains(strings.ToLower(item.Summary), query) {
		return true
	}
	for _, tag := range item.Tags {
		if strings.Contains(strings.ToLower(tag), query) {
			return true
		}
	}
	return false
}
