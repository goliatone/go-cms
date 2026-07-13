package content

import (
	"testing"

	"github.com/google/uuid"
)

func TestParseContentListOptionsPreservesMultipleFamilyIDs(t *testing.T) {
	first := uuid.New()
	second := uuid.New()
	opts := parseContentListOptions(WithFamilyIDs(first, second, first))
	if len(opts.familyIDs) != 2 || opts.familyIDs[0] != first || opts.familyIDs[1] != second {
		t.Fatalf("family IDs = %#v", opts.familyIDs)
	}
}

func TestServiceAdvertisesOnlyExecutableContentListOptions(t *testing.T) {
	first := uuid.New()
	second := uuid.New()
	svc := &service{}
	for _, option := range []ContentListOption{
		WithFamilyID(first),
		WithFamilyIDs(first, second),
		WithContentTypeID(first),
		WithTranslations(),
		WithDerivedFields(),
	} {
		if !svc.SupportsContentListOption(option) {
			t.Fatalf("expected supported option %q", option)
		}
	}
	for _, option := range []ContentListOption{
		"",
		"content:list:families:",
		"content:list:families:not-a-uuid",
		ContentListOption("content:list:families:" + first.String() + ",bad"),
		"content:list:unknown:value",
	} {
		if svc.SupportsContentListOption(option) {
			t.Fatalf("expected unsupported option %q", option)
		}
	}
}
