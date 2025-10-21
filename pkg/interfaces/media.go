package interfaces

import "context"

type MediaProvider interface {
	GetURL(ctx context.Context, path string) (string, error)
	GetMetadata(ctx context.Context, id string) (MediaMetadata, error)
}

type MediaMetadata struct {
	ID       string
	MimeType string
	Size     int64
	Width    int
	Height   int
}
