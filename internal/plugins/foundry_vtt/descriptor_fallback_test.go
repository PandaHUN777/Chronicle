// Pins Chronicle's loadDescriptor fallback (defaultDescriptor) against
// the canonical chronicle-package.json shipped by the Foundry-Module
// repo. testdata/chronicle-package.json is a committed snapshot, not a
// live fetch; the Foundry-Module repo's tools/check-package-descriptor.mjs
// instructs maintainers to regenerate it whenever the canonical changes.
// A drift failure names the specific field that diverged.
package foundry_vtt

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// TestLoadDescriptor_CanonicalMatchesFallback asserts every field of
// the canonical chronicle-package.json matches defaultDescriptor()
// except package.id, which defaultDescriptor() deliberately leaves
// empty since it's a descriptor-required field, not a default.
// Per-field assertions (not deep-equal) name the diverged field.
func TestLoadDescriptor_CanonicalMatchesFallback(t *testing.T) {
	bytes, err := os.ReadFile(filepath.Join("testdata", "chronicle-package.json"))
	if err != nil {
		t.Fatalf("read canonical fixture: %v", err)
	}

	var canonical PackageDescriptor
	if err := json.Unmarshal(bytes, &canonical); err != nil {
		t.Fatalf("parse canonical fixture as PackageDescriptor: %v", err)
	}

	fallback := defaultDescriptor()

	if canonical.SchemaVersion != fallback.SchemaVersion {
		t.Errorf("schemaVersion drift: canonical=%d fallback=%d",
			canonical.SchemaVersion, fallback.SchemaVersion)
	}

	// package.id is deliberately excluded: descriptor-required field,
	// not a default.

	if canonical.Package.Kind != fallback.Package.Kind {
		t.Errorf("package.kind drift: canonical=%q fallback=%q",
			canonical.Package.Kind, fallback.Package.Kind)
	}
	if canonical.Package.ModuleJSONPath != fallback.Package.ModuleJSONPath {
		t.Errorf("package.moduleJsonPath drift: canonical=%q fallback=%q",
			canonical.Package.ModuleJSONPath, fallback.Package.ModuleJSONPath)
	}

	if !reflect.DeepEqual(canonical.Serving.RewriteFields, fallback.Serving.RewriteFields) {
		t.Errorf("serving.rewriteFields drift: canonical=%v fallback=%v",
			canonical.Serving.RewriteFields, fallback.Serving.RewriteFields)
	}
	if canonical.Serving.ManifestEndpoint != fallback.Serving.ManifestEndpoint {
		t.Errorf("serving.manifestEndpoint drift: canonical=%q fallback=%q",
			canonical.Serving.ManifestEndpoint, fallback.Serving.ManifestEndpoint)
	}
	if canonical.Serving.DownloadEndpoint != fallback.Serving.DownloadEndpoint {
		t.Errorf("serving.downloadEndpoint drift: canonical=%q fallback=%q",
			canonical.Serving.DownloadEndpoint, fallback.Serving.DownloadEndpoint)
	}
	if canonical.Serving.PerCampaignSignedToken != fallback.Serving.PerCampaignSignedToken {
		t.Errorf("serving.perCampaignSignedToken drift: canonical=%v fallback=%v",
			canonical.Serving.PerCampaignSignedToken, fallback.Serving.PerCampaignSignedToken)
	}
	if canonical.Serving.ZipContentRoot != fallback.Serving.ZipContentRoot {
		t.Errorf("serving.zipContentRoot drift: canonical=%q fallback=%q",
			canonical.Serving.ZipContentRoot, fallback.Serving.ZipContentRoot)
	}
}
