package storage

import (
	"context"
	"io"
)

type BlobStorage interface {
	InitMultipartUpload(ctx context.Context, key, contentType string) (*InitResult, error)

	PresignPartUpload(
		ctx context.Context,
		key, uploadID string,
		partNumber int32,
		expires Duration,
	) (*PresignedPart, error)

	CompleteMultipartUpload(
		ctx context.Context,
		key, uploadID string,
		parts []CompletedPart,
	) (string, error)

	AbortMultipartUpload(ctx context.Context, key, uploadID string) error

	GetObjectStream(ctx context.Context, key string) (io.ReadCloser, error)
}
