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
