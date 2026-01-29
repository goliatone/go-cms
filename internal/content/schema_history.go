package content

import (
	"time"

	"github.com/google/uuid"
)

func newContentTypeSchemaSnapshot(ct *ContentType, actor uuid.UUID, updatedAt time.Time) ContentTypeSchemaSnapshot {
	snapshot := ContentTypeSchemaSnapshot{
		Version:      ct.SchemaVersion,
		Schema:       cloneMap(ct.Schema),
		UISchema:     cloneMap(ct.UISchema),
		Capabilities: cloneMap(ct.Capabilities),
		Status:       ct.Status,
		UpdatedAt:    updatedAt,
	}
	if actor != uuid.Nil {
		actorCopy := actor
		snapshot.UpdatedBy = &actorCopy
	}
	return snapshot
}

func appendSchemaHistory(history []ContentTypeSchemaSnapshot, snapshot ContentTypeSchemaSnapshot) []ContentTypeSchemaSnapshot {
	if snapshot.Version == "" {
		return history
	}
	if len(history) == 0 {
		return []ContentTypeSchemaSnapshot{snapshot}
	}
	last := history[len(history)-1]
	if last.Version == snapshot.Version {
		history[len(history)-1] = snapshot
		return history
	}
	return append(history, snapshot)
}

func cloneSchemaHistory(history []ContentTypeSchemaSnapshot) []ContentTypeSchemaSnapshot {
	if len(history) == 0 {
		return nil
	}
	out := make([]ContentTypeSchemaSnapshot, len(history))
	for i, snapshot := range history {
		out[i] = ContentTypeSchemaSnapshot{
			Version:      snapshot.Version,
			Schema:       cloneMap(snapshot.Schema),
			UISchema:     cloneMap(snapshot.UISchema),
			Capabilities: cloneMap(snapshot.Capabilities),
			Status:       snapshot.Status,
			UpdatedAt:    snapshot.UpdatedAt,
		}
		if snapshot.UpdatedBy != nil {
			actor := *snapshot.UpdatedBy
			out[i].UpdatedBy = &actor
		}
	}
	return out
}
