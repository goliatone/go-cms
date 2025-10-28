package markdowncmd

import (
	"testing"

	"github.com/google/uuid"
)

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

func TestImportDirectoryCommandValidateTemplateID(t *testing.T) {
	id := uuid.Nil
	cmd := ImportDirectoryCommand{
		Directory:  "content",
		TemplateID: &id,
	}
	if err := cmd.Validate(); err == nil {
		t.Fatal("expected error when template id is nil uuid")
	}

	valid := uuid.New()
	cmd.TemplateID = &valid
	if err := cmd.Validate(); err != nil {
		t.Fatalf("unexpected error with valid template id: %v", err)
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

func TestSyncDirectoryCommandValidateTemplateID(t *testing.T) {
	id := uuid.Nil
	cmd := SyncDirectoryCommand{
		Directory:  "content",
		TemplateID: &id,
	}
	if err := cmd.Validate(); err == nil {
		t.Fatal("expected error when template id is nil uuid")
	}

	valid := uuid.New()
	cmd.TemplateID = &valid
	if err := cmd.Validate(); err != nil {
		t.Fatalf("unexpected error with valid template id: %v", err)
	}
}
