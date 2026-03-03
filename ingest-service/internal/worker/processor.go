package worker

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"sync/atomic"

	"github.com/hibiken/asynq"
	"github.com/prxssh/csv-ingestor/ingest-service/internal/movie"
	"github.com/prxssh/csv-ingestor/ingest-service/internal/storage"
	"github.com/prxssh/csv-ingestor/ingest-service/internal/upload"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/sync/errgroup"
)

const (
	batchSize  = 500
	maxWorkers = 4
)

type Processor struct {
	store   storage.BlobStorage
	uploads *upload.Service
	movies  *movie.Service
}

func NewProcessor(
	store storage.BlobStorage,
	uploads *upload.Service,
	movies *movie.Service,
) *Processor {
	return &Processor{
		store:   store,
		uploads: uploads,
		movies:  movies,
	}
}

func (p *Processor) HandleProcessCSV(ctx context.Context, t *asynq.Task) error {
	var payload ProcessCSVPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal payload: %w", err)
	}

	slog.InfoContext(ctx, "started processing csv", "job_id", payload.JobID)

	// Add job_id to current span (created by OtelMiddleware)
	span := trace.SpanFromContext(ctx)
	span.SetAttributes(attribute.String("job_id", payload.JobID))

	job, err := p.uploads.GetJob(ctx, payload.JobID)
	if err != nil {
		return fmt.Errorf("get job: %w", err)
	}

	if err := p.uploads.MarkProcessing(ctx, payload.JobID); err != nil {
		slog.WarnContext(ctx, "failed to update job status to processing", "error", err)
	}

	if err := p.processCSV(ctx, payload.JobID, job.Key); err != nil {
		slog.ErrorContext(
			ctx,
			"csv processing failed",
			"job_id",
			payload.JobID,
			"error",
			err,
		)
		span.SetStatus(codes.Error, err.Error())
		if updateErr := p.uploads.MarkFailed(ctx, payload.JobID, err.Error()); updateErr != nil {
			slog.ErrorContext(
				ctx,
				"failed to record job error",
				"job_id",
				payload.JobID,
				"error",
				updateErr,
			)
		}
		return fmt.Errorf("process csv: %w", err)
	}

	if err := p.uploads.MarkProcessed(ctx, payload.JobID); err != nil {
		slog.WarnContext(ctx, "failed to update job status to processed", "error", err)
	}

	slog.InfoContext(ctx, "finished processing csv", "job_id", payload.JobID)
	return nil
}

func (p *Processor) processCSV(ctx context.Context, jobID, key string) error {
	tracer := otel.Tracer(tracerName)

	// Span: stream CSV from S3
	ctx, streamSpan := tracer.Start(ctx, "worker.StreamCSV",
		trace.WithAttributes(
			attribute.String("job_id", jobID),
			attribute.String("s3.key", key),
		),
	)

	body, err := p.store.GetObjectStream(ctx, key)
	if err != nil {
		streamSpan.SetStatus(codes.Error, err.Error())
		streamSpan.End()
		return fmt.Errorf("get object stream: %w", err)
	}
	defer body.Close()

	reader := csv.NewReader(body)
	reader.LazyQuotes = true
	reader.TrimLeadingSpace = true
	reader.FieldsPerRecord = -1

	if _, err := reader.Read(); err != nil {
		streamSpan.SetStatus(codes.Error, err.Error())
		streamSpan.End()
		return fmt.Errorf("read csv header: %w", err)
	}

	streamSpan.End()

	// Span: parse and batch-write
	ctx, processSpan := tracer.Start(ctx, "worker.ParseAndUpsert",
		trace.WithAttributes(attribute.String("job_id", jobID)),
	)
	defer processSpan.End()

	var (
		totalInserted atomic.Int64
		totalModified atomic.Int64
		totalSkipped  atomic.Int64
		totalRows     atomic.Int64
	)

	batches := make(chan []movie.Movie, maxWorkers*2)

	g, gCtx := errgroup.WithContext(ctx)

	// Consumer workers
	for w := 0; w < maxWorkers; w++ {
		g.Go(func() error {
			for batch := range batches {
				inserted, modified, err := p.movies.BatchUpsert(gCtx, batch)
				if err != nil {
					return fmt.Errorf("batch upsert: %w", err)
				}
				totalInserted.Add(inserted)
				totalModified.Add(modified)
			}
			return nil
		})
	}

	// Producer
	g.Go(func() error {
		defer close(batches)

		batch := make([]movie.Movie, 0, batchSize)
		lineNum := 2 // header already consumed above, first data row is line 2

		for {
			record, err := reader.Read()
			if err == io.EOF {
				break
			}
			if err != nil {
				slog.WarnContext(gCtx, "skipping malformed csv line",
					"job_id", jobID, "line", lineNum, "error", err)
				totalSkipped.Add(1)
				lineNum++
				continue
			}

			m, err := movie.ParseRow(record)
			if err != nil {
				slog.WarnContext(gCtx, "skipping unparseable csv row",
					"job_id", jobID, "line", lineNum, "error", err)
				totalSkipped.Add(1)
				lineNum++
				continue
			}

			batch = append(batch, m)
			totalRows.Add(1)
			lineNum++

			if len(batch) >= batchSize {
				toSend := make([]movie.Movie, len(batch))
				copy(toSend, batch)

				select {
				case batches <- toSend:
				case <-gCtx.Done():
					return gCtx.Err()
				}

				batch = batch[:0]
			}
		}

		if len(batch) > 0 {
			select {
			case batches <- batch:
			case <-gCtx.Done():
				return gCtx.Err()
			}
		}

		return nil
	})

	if err := g.Wait(); err != nil {
		processSpan.SetStatus(codes.Error, err.Error())
		return err
	}

	// Record results as span attributes
	processSpan.SetAttributes(
		attribute.Int64("csv.total_rows", totalRows.Load()),
		attribute.Int64("csv.inserted", totalInserted.Load()),
		attribute.Int64("csv.modified", totalModified.Load()),
		attribute.Int64("csv.skipped", totalSkipped.Load()),
	)

	slog.InfoContext(ctx, "csv processing complete",
		"job_id", jobID,
		"total_rows", totalRows.Load(),
		"inserted", totalInserted.Load(),
		"modified", totalModified.Load(),
		"skipped", totalSkipped.Load(),
	)

	if totalRows.Load() == 0 && totalSkipped.Load() > 0 {
		return fmt.Errorf(
			"all %d rows were skipped due to parsing errors",
			totalSkipped.Load(),
		)
	}

	return nil
}
