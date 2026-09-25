package campaigns

// topbar_image_test.go pins that the topbar Image control is first-class
// server-rendered markup: TopbarImageSection renders both states with the
// correct HTMX swap wiring, and a saved image reads back into the form.

import (
	"context"
	"strings"
	"testing"
)

func renderAppearanceTab(t *testing.T, settings string) string {
	t.Helper()
	cc := &CampaignContext{
		Campaign:   &Campaign{ID: "camp-1", Name: "Test Campaign", Settings: settings},
		MemberRole: RoleOwner,
	}
	var sb strings.Builder
	if err := appearanceTab(cc, "tok").Render(context.Background(), &sb); err != nil {
		t.Fatalf("render appearanceTab: %v", err)
	}
	return sb.String()
}

// TestAppearanceTab_ImageModeIsFirstClass proves the Image button + upload panel
// are in the server-rendered markup, not injected by JS at runtime.
func TestAppearanceTab_ImageModeIsFirstClass(t *testing.T) {
	html := renderAppearanceTab(t, "")

	if !strings.Contains(html, `data-mode="image"`) {
		t.Error(`Top Bar Style card must render a first-class data-mode="image" button (was JS-injected before the rescue)`)
	}
	if !strings.Contains(html, `id="appearance-topbar-image"`) {
		t.Error("Top Bar Style card must render the #appearance-topbar-image panel")
	}
	if !strings.Contains(html, `id="appearance-topbar-image-section"`) {
		t.Error("the image panel must contain the TopbarImageSection swap target")
	}
}

// TestTopbarImageSection_States pins both render states + their HTMX swap wiring.
func TestTopbarImageSection_States(t *testing.T) {
	t.Run("no image → upload dropzone posting to the topbar-image endpoint", func(t *testing.T) {
		var sb strings.Builder
		if err := TopbarImageSection("camp-1", "", "tok").Render(context.Background(), &sb); err != nil {
			t.Fatalf("render: %v", err)
		}
		html := sb.String()
		if !strings.Contains(html, `hx-post="/campaigns/camp-1/topbar-image"`) {
			t.Error("empty state must offer an hx-post upload to the topbar-image endpoint")
		}
		if !strings.Contains(html, `hx-target="#appearance-topbar-image-section"`) {
			t.Error("upload must swap the section in place (no reload)")
		}
		if !strings.Contains(html, `data-topbar-image-path=""`) {
			t.Error("empty state must carry an empty data-topbar-image-path for JS state sync")
		}
	})

	t.Run("image set → thumbnail + hx-delete remove", func(t *testing.T) {
		// The fixture matches what the upload path actually stores:
		// MediaUploader.UploadBackdrop returns MediaFile.Filename as
		// filepath.Join("2006/01", uuid+ext) — a value containing slashes.
		const stored = "2026/09/b7c17bb1-6563-462c-8b49-5b2e8bd57108.png"
		var sb strings.Builder
		if err := TopbarImageSection("camp-1", stored, "tok").Render(context.Background(), &sb); err != nil {
			t.Fatalf("render: %v", err)
		}
		html := sb.String()

		// /media/:id matches ONE path segment. A src carrying the stored
		// value verbatim cannot be routed by Echo and answers 404.
		if strings.Contains(html, "/media/"+stored) {
			t.Errorf("src renders the raw stored path %q under /media/, which Echo's single-segment /media/:id route cannot match — this is a 404", stored)
		}
		if !strings.Contains(html, `/media/b7c17bb1-6563-462c-8b49-5b2e8bd57108`) {
			t.Error("set state must render the image through MediaURL, which reduces the stored path to the media id")
		}
		if !strings.Contains(html, `hx-delete="/campaigns/camp-1/topbar-image"`) {
			t.Error("set state must offer an hx-delete remove")
		}
		// The data- attribute keeps the raw stored value: JS round-trips it
		// back to the server, which stores paths, not ids.
		if !strings.Contains(html, `data-topbar-image-path="`+stored+`"`) {
			t.Error("set state must carry the stored path in data-topbar-image-path for JS state sync")
		}
	})
}

// TestAppearanceTab_TopbarImageReadsBack proves a saved topbar image renders
// back into the form (the sweep's read-back check, item (c)).
func TestAppearanceTab_TopbarImageReadsBack(t *testing.T) {
	const stored = "2026/09/b7c17bb1-6563-462c-8b49-5b2e8bd57108.png"
	html := renderAppearanceTab(t, `{"topbar_style":{"mode":"image","image_path":"`+stored+`"}}`)
	if !strings.Contains(html, `/media/b7c17bb1-6563-462c-8b49-5b2e8bd57108`) {
		t.Error("a saved topbar image must read back into the Image panel thumbnail, through MediaURL")
	}
	if strings.Contains(html, "/media/"+stored) {
		t.Errorf("read-back renders the raw stored path %q — a 404 under /media/:id", stored)
	}
	if !strings.Contains(html, `data-topbar-image-path="`+stored+`"`) {
		t.Error("the saved image path must round-trip into data-topbar-image-path")
	}
}
