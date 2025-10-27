package scheduler

import "github.com/google/uuid"

const (
	JobTypeContentPublish   = "cms.content.publish"
	JobTypeContentUnpublish = "cms.content.unpublish"
	JobTypePagePublish      = "cms.page.publish"
	JobTypePageUnpublish    = "cms.page.unpublish"
)

func ContentPublishJobKey(id uuid.UUID) string {
	return "content:" + id.String() + ":publish"
}

func ContentUnpublishJobKey(id uuid.UUID) string {
	return "content:" + id.String() + ":unpublish"
}

func PagePublishJobKey(id uuid.UUID) string {
	return "page:" + id.String() + ":publish"
}

func PageUnpublishJobKey(id uuid.UUID) string {
	return "page:" + id.String() + ":unpublish"
}
