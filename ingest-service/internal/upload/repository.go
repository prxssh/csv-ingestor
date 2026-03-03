package upload

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Repository struct {
	coll *mongo.Collection
}

func NewRepository(coll *mongo.Collection) *Repository {
	return &Repository{coll: coll}
}

func (r *Repository) create(ctx context.Context, job *UploadJob) (*UploadJob, error) {
	job.ID = primitive.NewObjectID()
	now := time.Now().UTC()
	job.CreatedAt = now
	job.UpdatedAt = now

	if _, err := r.coll.InsertOne(ctx, job); err != nil {
		return nil, fmt.Errorf("insert upload job: %w", err)
	}
	return job, nil
}

func (r *Repository) findByID(ctx context.Context, id string) (*UploadJob, error) {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, fmt.Errorf("invalid job id: %w", err)
	}

	var job UploadJob
	err = r.coll.FindOne(ctx, bson.M{"_id": oid}).Decode(&job)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find upload job: %w", err)
	}
	return &job, nil
}

func (r *Repository) updatePartStatus(
	ctx context.Context,
	jobID string,
	partNumber int32,
	status PartStatus,
) error {
	oid, err := primitive.ObjectIDFromHex(jobID)
	if err != nil {
		return fmt.Errorf("invalid job id: %w", err)
	}

	now := time.Now().UTC()
	_, err = r.coll.UpdateOne(
		ctx,
		bson.M{"_id": oid, "parts.part_number": partNumber},
		bson.M{
			"$set": bson.M{
				"parts.$.status":     status,
				"parts.$.updated_at": now,
				"updated_at":         now,
			},
		},
	)
	if err != nil {
		return fmt.Errorf("update part status: %w", err)
	}
	return nil
}

func (r *Repository) markPartCompleted(
	ctx context.Context,
	jobID string,
	partNumber int32,
	etag string,
) error {
	oid, err := primitive.ObjectIDFromHex(jobID)
	if err != nil {
		return fmt.Errorf("invalid job id: %w", err)
	}

	now := time.Now().UTC()
	_, err = r.coll.UpdateOne(
		ctx,
		bson.M{"_id": oid, "parts.part_number": partNumber},
		bson.M{
			"$set": bson.M{
				"parts.$.status":     PartStatusCompleted,
				"parts.$.etag":       etag,
				"parts.$.updated_at": now,
				"updated_at":         now,
			},
		},
	)
	if err != nil {
		return fmt.Errorf("mark part completed: %w", err)
	}
	return nil
}

func (r *Repository) updateStatus(ctx context.Context, jobID string, status JobStatus) error {
	oid, err := primitive.ObjectIDFromHex(jobID)
	if err != nil {
		return fmt.Errorf("invalid job id: %w", err)
	}

	_, err = r.coll.UpdateOne(
		ctx,
		bson.M{"_id": oid},
		bson.M{"$set": bson.M{"status": status, "updated_at": time.Now().UTC()}},
	)
	if err != nil {
		return fmt.Errorf("update job status: %w", err)
	}
	return nil
}

func (r *Repository) updateJobError(ctx context.Context, jobID string, errMsg string) error {
	oid, err := primitive.ObjectIDFromHex(jobID)
	if err != nil {
		return fmt.Errorf("invalid job id: %w", err)
	}

	_, err = r.coll.UpdateOne(
		ctx,
		bson.M{"_id": oid},
		bson.M{"$set": bson.M{
			"status":        JobStatusFailed,
			"error_message": errMsg,
			"updated_at":    time.Now().UTC(),
		}},
	)
	if err != nil {
		return fmt.Errorf("update job error: %w", err)
	}
	return nil
}

func (r *Repository) createIndexes(ctx context.Context) error {
	indexes := []mongo.IndexModel{
		{
			Keys:    bson.D{bson.E{Key: "status", Value: 1}},
			Options: options.Index().SetName("idx_status"),
		},
		{
			Keys:    bson.D{bson.E{Key: "upload_id", Value: 1}},
			Options: options.Index().SetName("idx_upload_id").SetSparse(true),
		},
	}
	_, err := r.coll.Indexes().CreateMany(ctx, indexes)
	return err
}
