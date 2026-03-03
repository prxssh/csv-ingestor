package upload

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/prxssh/csv-ingestor/ingest-service/config"
	"github.com/prxssh/csv-ingestor/ingest-service/internal/storage"
)

type Service struct {
	store storage.BlobStorage
	repo  *Repository
}

func NewService(store storage.BlobStorage, repo *Repository) *Service {
	return &Service{
		store: store,
		repo:  repo,
	}
}

func (s *Service) CreateIndexes(ctx context.Context) error {
	return s.repo.createIndexes(ctx)
}

func (s *Service) GetJob(ctx context.Context, jobID string) (*UploadJob, error) {
	job, err := s.repo.findByID(ctx, jobID)
	if err != nil {
		return nil, fmt.Errorf("find job: %w", err)
	}
	if job == nil {
		return nil, ErrJobNotFound
	}
	return job, nil
}

func (s *Service) MarkProcessing(ctx context.Context, jobID string) error {
	return s.repo.updateStatus(ctx, jobID, JobStatusProcessing)
}

func (s *Service) MarkProcessed(ctx context.Context, jobID string) error {
	return s.repo.updateStatus(ctx, jobID, JobStatusProcessed)
}

func (s *Service) MarkFailed(ctx context.Context, jobID string, errMsg string) error {
	return s.repo.updateJobError(ctx, jobID, errMsg)
}

func (s *Service) InitUpload(
	ctx context.Context,
	req InitUploadRequest,
) (*InitUploadResponse, error) {
	key := buildKey(req.Filename)

	result, err := s.store.InitMultipartUpload(ctx, key, req.ContentType)
	if err != nil {
		return nil, fmt.Errorf("init multipart upload: %w", err)
	}

	totalParts := int32(math.Ceil(float64(req.TotalSize) / float64(storage.PartSize)))
	if totalParts < 1 {
		totalParts = 1
	}

	parts := make([]PartRecord, totalParts)
	for i := int32(0); i < totalParts; i++ {
		parts[i] = PartRecord{
			PartNumber: i + 1,
			Status:     PartStatusPending,
			UpdatedAt:  time.Now().UTC(),
		}
	}

	job, err := s.repo.create(ctx, &UploadJob{
		Key:         key,
		UploadID:    result.UploadID,
		Filename:    req.Filename,
		ContentType: req.ContentType,
		TotalParts:  totalParts,
		PartSize:    storage.PartSize,
		TotalSize:   req.TotalSize,
		Parts:       parts,
		Status:      JobStatusPending,
	})
	if err != nil {
		if abortErr := s.store.AbortMultipartUpload(ctx, key, result.UploadID); abortErr != nil {
			slog.ErrorContext(ctx, "failed to abort orphaned multipart upload",
				"key", key, "upload_id", result.UploadID, "error", abortErr)
		}
		return nil, fmt.Errorf("create upload job: %w", err)
	}

	presigned, err := s.presignParts(ctx, key, result.UploadID, partsRange(1, totalParts))
	if err != nil {
		if abortErr := s.store.AbortMultipartUpload(ctx, key, result.UploadID); abortErr != nil {
			slog.ErrorContext(ctx, "failed to abort orphaned multipart upload",
				"key", key, "upload_id", result.UploadID, "error", abortErr)
		}
		return nil, fmt.Errorf("presign parts: %w", err)
	}

	return &InitUploadResponse{
		JobID:    job.ID.Hex(),
		UploadID: result.UploadID,
		Parts:    presigned,
	}, nil
}

func (s *Service) GetPresignedParts(
	ctx context.Context,
	jobID string,
	partNumbers []int32,
) (*PresignPartsResponse, error) {
	job, err := s.repo.findByID(ctx, jobID)
	if err != nil {
		return nil, fmt.Errorf("find job: %w", err)
	}
	if job == nil {
		return nil, ErrJobNotFound
	}
	if job.Status == JobStatusCompleted || job.Status == JobStatusAborted {
		return nil, ErrJobFinished
	}

	presigned, err := s.presignParts(ctx, job.Key, job.UploadID, partNumbers)
	if err != nil {
		return nil, fmt.Errorf("presign parts: %w", err)
	}

	return &PresignPartsResponse{
		JobID: jobID,
		Parts: presigned,
	}, nil
}

func (s *Service) GetUploadStatus(ctx context.Context, jobID string) (*UploadJob, error) {
	job, err := s.repo.findByID(ctx, jobID)
	if err != nil {
		return nil, fmt.Errorf("find job: %w", err)
	}
	if job == nil {
		return nil, ErrJobNotFound
	}
	return job, nil
}

func (s *Service) CompleteUpload(
	ctx context.Context,
	jobID string,
	req CompleteUploadRequest,
) (*CompleteUploadResponse, error) {
	job, err := s.repo.findByID(ctx, jobID)
	if err != nil {
		return nil, fmt.Errorf("find job: %w", err)
	}
	if job == nil {
		return nil, ErrJobNotFound
	}
	if job.Status == JobStatusCompleted {
		return nil, ErrJobAlreadyCompleted
	}
	if job.Status == JobStatusAborted {
		return nil, ErrJobAborted
	}

	s3Parts := make([]storage.CompletedPart, len(req.Parts))
	for i, p := range req.Parts {
		s3Parts[i] = storage.CompletedPart{
			PartNumber: p.PartNumber,
			ETag:       p.ETag,
		}
		if err := s.repo.markPartCompleted(ctx, jobID, p.PartNumber, p.ETag); err != nil {
			slog.WarnContext(
				ctx,
				"failed to mark part completed in db",
				"job_id",
				jobID,
				"part",
				p.PartNumber,
				"error",
				err,
			)
		}
	}

	location, err := s.store.CompleteMultipartUpload(ctx, job.Key, job.UploadID, s3Parts)
	if err != nil {
		return nil, fmt.Errorf("complete multipart upload: %w", err)
	}

	if err := s.repo.updateStatus(ctx, jobID, JobStatusCompleted); err != nil {
		slog.WarnContext(
			ctx,
			"failed to mark job completed in db",
			"job_id",
			jobID,
			"error",
			err,
		)
	}

	return &CompleteUploadResponse{
		JobID:    jobID,
		Location: location,
	}, nil
}

func (s *Service) AbortUpload(ctx context.Context, jobID string) error {
	job, err := s.repo.findByID(ctx, jobID)
	if err != nil {
		return fmt.Errorf("find job: %w", err)
	}
	if job == nil {
		return ErrJobNotFound
	}
	if job.Status == JobStatusCompleted {
		return ErrJobAlreadyCompleted
	}
	if job.Status == JobStatusAborted {
		return nil // idempotent
	}

	if err := s.store.AbortMultipartUpload(ctx, job.Key, job.UploadID); err != nil {
		return fmt.Errorf("abort multipart upload: %w", err)
	}

	return s.repo.updateStatus(ctx, jobID, JobStatusAborted)
}

func (s *Service) ReportPartUploaded(
	ctx context.Context,
	jobID string,
	req ReportPartUploadedRequest,
) error {
	job, err := s.repo.findByID(ctx, jobID)
	if err != nil {
		return fmt.Errorf("find job: %w", err)
	}
	if job == nil {
		return ErrJobNotFound
	}
	if job.Status == JobStatusCompleted {
		return ErrJobAlreadyCompleted
	}
	if job.Status == JobStatusAborted {
		return ErrJobAborted
	}

	if job.Status == JobStatusPending {
		if err := s.repo.updateStatus(ctx, jobID, JobStatusUploading); err != nil {
			return fmt.Errorf("transition job to uploading: %w", err)
		}
	}

	return s.repo.markPartCompleted(ctx, jobID, req.PartNumber, req.ETag)
}

func (s *Service) presignParts(
	ctx context.Context,
	key, uploadID string,
	partNumbers []int32,
) ([]PresignedPartResponse, error) {
	ttl := time.Duration(config.Env.S3PresignTTLMins) * time.Minute
	out := make([]PresignedPartResponse, 0, len(partNumbers))
	for _, n := range partNumbers {
		p, err := s.store.PresignPartUpload(ctx, key, uploadID, n, ttl)
		if err != nil {
			return nil, fmt.Errorf("presign part %d: %w", n, err)
		}
		out = append(out, PresignedPartResponse{PartNumber: p.PartNumber, URL: p.URL})
	}
	return out, nil
}

func partsRange(from, to int32) []int32 {
	nums := make([]int32, 0, to-from+1)
	for i := from; i <= to; i++ {
		nums = append(nums, i)
	}
	return nums
}

func buildKey(filename string) string {
	now := time.Now().UTC()
	return fmt.Sprintf("uploads/%s/%d/%02d/%02d/%s/%s",
		config.Env.Environment,
		now.Year(), now.Month(), now.Day(),
		uuid.NewString(),
		filename,
	)
}
