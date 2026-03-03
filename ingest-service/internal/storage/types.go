package storage

import "time"

type Duration = time.Duration

const PartSize = 5 * 1024 * 1024

type InitResult struct {
	UploadID string
	Key      string
}

type PresignedPart struct {
	PartNumber int32
	URL        string
}

type CompletedPart struct {
	PartNumber int32
	ETag       string
}
