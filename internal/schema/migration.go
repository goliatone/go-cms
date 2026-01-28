package schema

import "fmt"

// MigrationFunc transforms a payload between schema versions.
type MigrationFunc func(map[string]any) (map[string]any, error)

// Migrator manages ordered migration steps per schema slug.
type Migrator struct {
	steps map[string]map[string]MigrationStep
}

// MigrationStep describes a single migration hop.
type MigrationStep struct {
	From  string
	To    string
	Apply MigrationFunc
}

// NewMigrator constructs an empty migrator registry.
func NewMigrator() *Migrator {
	return &Migrator{steps: map[string]map[string]MigrationStep{}}
}

// Register adds a migration step for a slug.
func (m *Migrator) Register(slug, from, to string, fn MigrationFunc) error {
	if m == nil {
		return fmt.Errorf("schema: migrator not configured")
	}
	if slug == "" || from == "" || to == "" || fn == nil {
		return fmt.Errorf("schema: migration registration invalid")
	}
	if m.steps == nil {
		m.steps = map[string]map[string]MigrationStep{}
	}
	if m.steps[slug] == nil {
		m.steps[slug] = map[string]MigrationStep{}
	}
	m.steps[slug][from] = MigrationStep{From: from, To: to, Apply: fn}
	return nil
}

// Migrate applies migration steps until the target version is reached.
func (m *Migrator) Migrate(slug, from, to string, payload map[string]any) (map[string]any, error) {
	if m == nil || m.steps == nil {
		return nil, fmt.Errorf("schema: migrator not configured")
	}
	if from == to {
		return payload, nil
	}
	seen := map[string]struct{}{}
	current := from
	out := cloneMap(payload)
	for current != to {
		if _, ok := seen[current]; ok {
			return nil, fmt.Errorf("schema: migration cycle detected")
		}
		seen[current] = struct{}{}
		step, ok := m.steps[slug][current]
		if !ok || step.Apply == nil {
			return nil, fmt.Errorf("schema: migration step missing")
		}
		nextPayload, err := step.Apply(out)
		if err != nil {
			return nil, err
		}
		out = cloneMap(nextPayload)
		current = step.To
	}
	return out, nil
}
