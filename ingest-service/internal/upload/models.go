package upload

import (
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

var (
	ErrJobNotFound         = errors.New("upload job not found")
	ErrJobFinished         = errors.New("upload job is already completed or aborted")
	ErrJobAlreadyCompleted = errors.New("upload job is already completed")
	ErrJobAborted          = errors.New("upload job has been aborted")
)

type InitUploadRequest struct {
	Filename    string `json:"filename"     binding:"required,min=1,max=255"`
	ContentType string `json:"content_type" binding:"required,eq=text/csv"`
	TotalSize   int64  `json:"total_size"   binding:"required,min=1,max=1073741824"` // 1GB max
}

type PresignedPartResponse struct {
	PartNumber int32  `json:"part_number"`
	URL        string `json:"url"`
}

type InitUploadResponse struct {
	JobID    string                  `json:"job_id"`
	UploadID string                  `json:"upload_id"`
	Parts    []PresignedPartResponse `json:"parts"`
}

type CompletedPartRequest struct {
	PartNumber int32  `json:"part_number" binding:"required"`
	ETag       string `json:"etag"        binding:"required"`
}

type CompleteUploadRequest struct {
	Parts []CompletedPartRequest `json:"parts" binding:"required,min=1"`
}

type CompleteUploadResponse struct {
	JobID    string `json:"job_id"`
	Location string `json:"location"` // final S3 object URL
}

type PresignPartsResponse struct {
	JobID string                  `json:"job_id"`
	Parts []PresignedPartResponse `json:"parts"`
}

type ReportPartUploadedRequest struct {
	PartNumber int32  `json:"part_number" binding:"required,min=1"`
	ETag       string `json:"etag"        binding:"required"`
}

type PartStatus string

const (
	PartStatusPending   PartStatus = "pending"
	PartStatusUploading PartStatus = "uploading"
	PartStatusCompleted PartStatus = "completed"
	PartStatusFailed    PartStatus = "failed"
)

type JobStatus string

const (
	JobStatusPending    JobStatus = "pending"
	JobStatusUploading  JobStatus = "uploading"
	JobStatusCompleted  JobStatus = "completed"
	JobStatusProcessing JobStatus = "processing"
	JobStatusProcessed  JobStatus = "processed"
	JobStatusFailed     JobStatus = "failed"
	JobStatusAborted    JobStatus = "aborted"
)

type PartRecord struct {
	PartNumber int32      `bson:"part_number"`
	Status     PartStatus `bson:"status"` // pending | uploading | completed | failed
	ETag       string     `bson:"etag"`   // populated when completed
	UpdatedAt  time.Time  `bson:"updated_at"`
}

type UploadJob struct {
	ID           primitive.ObjectID `bson:"_id"`
	Key          string             `bson:"key"`       // S3 object key
	UploadID     string             `bson:"upload_id"` // S3 multipart upload ID
	Filename     string             `bson:"filename"`
	ContentType  string             `bson:"content_type"`
	TotalParts   int32              `bson:"total_parts"`
	PartSize     int64              `bson:"part_size"` // always 5 MB
	TotalSize    int64              `bson:"total_size"`
	Parts        []PartRecord       `bson:"parts"`
	Status       JobStatus          `bson:"status"`
	ErrorMessage string             `bson:"error_message,omitempty"`
	CreatedAt    time.Time          `bson:"created_at"`
	UpdatedAt    time.Time          `bson:"updated_at"`
}
