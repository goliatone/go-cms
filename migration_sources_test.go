package cms_test

import (
	"io/fs"
	"testing"

	cms "github.com/goliatone/go-cms"
	persistence "github.com/goliatone/go-persistence-bun"
)

func TestDefaultMigrationSourceDescriptor(t *testing.T) {
	descriptor, err := cms.DefaultMigrationSourceDescriptor(
		cms.WithMigrationSourceDependencies("go-users"),
	)
	if err != nil {
		t.Fatalf("default migration source descriptor: %v", err)
	}

	if descriptor.Name != cms.MigrationSourceName {
		t.Fatalf("source name mismatch: got=%q want=%q", descriptor.Name, cms.MigrationSourceName)
	}
	if descriptor.SourceKey != cms.MigrationSourceKey {
		t.Fatalf("source key mismatch: got=%q want=%q", descriptor.SourceKey, cms.MigrationSourceKey)
	}
	if descriptor.Order != cms.MigrationSourceOrder {
		t.Fatalf("source order mismatch: got=%d want=%d", descriptor.Order, cms.MigrationSourceOrder)
	}
	if len(descriptor.ValidationTargets) != 2 ||
		descriptor.ValidationTargets[0] != cms.MigrationDialectPostgres ||
		descriptor.ValidationTargets[1] != cms.MigrationDialectSQLite {
		t.Fatalf("validation targets mismatch: got=%v", descriptor.ValidationTargets)
	}
	if len(descriptor.DependsOn) != 1 || descriptor.DependsOn[0] != "go-users" {
		t.Fatalf("dependencies mismatch: got=%v", descriptor.DependsOn)
	}

	matches, err := fs.Glob(descriptor.Root, "*.up.sql")
	if err != nil {
		t.Fatalf("glob postgres migrations: %v", err)
	}
	if len(matches) == 0 {
		t.Fatalf("expected postgres migration files")
	}
	matches, err = fs.Glob(descriptor.Root, "sqlite/*.up.sql")
	if err != nil {
		t.Fatalf("glob sqlite migrations: %v", err)
	}
	if len(matches) == 0 {
		t.Fatalf("expected sqlite migration files")
	}
}

func TestStableOrderedMigrationSource(t *testing.T) {
	source, err := cms.StableOrderedMigrationSource(
		cms.WithMigrationSourceDependencies("go-users"),
	)
	if err != nil {
		t.Fatalf("stable ordered migration source: %v", err)
	}

	if source.IdentityMode != persistence.OrderedMigrationIdentitySourceStable {
		t.Fatalf("identity mode mismatch: got=%s", source.IdentityMode)
	}
	if source.Name != cms.MigrationSourceName {
		t.Fatalf("source name mismatch: got=%q", source.Name)
	}
	if source.SourceKey != cms.MigrationSourceKey {
		t.Fatalf("source key mismatch: got=%q", source.SourceKey)
	}
	if source.Order != cms.MigrationSourceOrder {
		t.Fatalf("source order mismatch: got=%d", source.Order)
	}
	if len(source.DependsOn) != 1 || source.DependsOn[0] != "go-users" {
		t.Fatalf("dependencies mismatch: got=%v", source.DependsOn)
	}
	if len(source.Options) == 0 {
		t.Fatalf("expected dialect options")
	}
}

func TestStableOrderedMigrationSourcePreservesInvalidOrderForRegistrationValidation(t *testing.T) {
	source, err := cms.StableOrderedMigrationSource(
		cms.WithMigrationSourceOrder(0),
	)
	if err != nil {
		t.Fatalf("stable ordered migration source: %v", err)
	}
	if source.Order != 0 {
		t.Fatalf("expected caller-provided invalid order to be preserved, got %d", source.Order)
	}
}

func TestLegacyOrderedMigrationSource(t *testing.T) {
	source, err := cms.LegacyOrderedMigrationSource()
	if err != nil {
		t.Fatalf("legacy ordered migration source: %v", err)
	}

	if source.IdentityMode != persistence.OrderedMigrationIdentityPositional {
		t.Fatalf("identity mode mismatch: got=%s", source.IdentityMode)
	}
	if source.SourceKey != "" || source.Order != 0 {
		t.Fatalf("expected positional source without stable identity, got key=%q order=%d", source.SourceKey, source.Order)
	}
	if source.Name != cms.MigrationSourceName {
		t.Fatalf("source name mismatch: got=%q", source.Name)
	}
	if len(source.Options) == 0 {
		t.Fatalf("expected dialect options")
	}
}
