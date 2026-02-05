package markdowncmd

import "testing"

func TestImportDirectoryCommandValidateRequiresDirectory(t *testing.T) {
	cmd := ImportDirectoryCommand{}
	if err := cmd.Validate(); err == nil {
		t.Fatal("expected error when directory missing")
	}

	cmd.Directory = "content"
	if err := cmd.Validate(); err != nil {
		t.Fatalf("unexpected error when directory provided: %v", err)
	}
}

func TestSyncDirectoryCommandValidateRequiresDirectory(t *testing.T) {
	cmd := SyncDirectoryCommand{}
	if err := cmd.Validate(); err == nil {
		t.Fatal("expected error when directory missing")
	}

	cmd.Directory = "content"
	if err := cmd.Validate(); err != nil {
		t.Fatalf("unexpected error when directory provided: %v", err)
	}
}
