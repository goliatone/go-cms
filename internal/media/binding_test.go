package media_test

import (
	"testing"

	"github.com/goliatone/go-cms/internal/media"
	"github.com/goliatone/go-cms/pkg/interfaces"
)

func TestCloneBindingSetNil(t *testing.T) {
	if media.CloneBindingSet(nil) != nil {
		t.Fatalf("expected nil clone for nil source")
	}
}

func TestCloneBindingSetDeepCopy(t *testing.T) {
	original := media.BindingSet{
		"hero_image": {
			{
				Slot: "hero",
				Reference: interfaces.MediaReference{
					ID:         "asset-1",
					Collection: "images",
				},
				Renditions: []string{"full", "thumb"},
				Required:   []string{"full"},
				Locale:     "en",
				Metadata: map[string]any{
					"role": "primary",
				},
			},
		},
	}

	cloned := media.CloneBindingSet(original)
	if len(cloned) != 1 {
		t.Fatalf("expected one entry in cloned set")
	}
	if cloned["hero_image"][0].Reference.ID != "asset-1" {
		t.Fatalf("expected reference id to be asset-1")
	}

	original["hero_image"][0].Renditions[0] = "mutated"
	original["hero_image"][0].Metadata["role"] = "secondary"

	if cloned["hero_image"][0].Renditions[0] != "full" {
		t.Fatalf("expected cloned renditions to remain unaffected")
	}
	if cloned["hero_image"][0].Metadata["role"] != "primary" {
		t.Fatalf("expected cloned metadata to remain unaffected")
	}
}
