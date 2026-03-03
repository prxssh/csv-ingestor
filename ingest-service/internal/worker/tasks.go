package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/hibiken/asynq"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

const (
	TaskProcessCSV = "csv:process"
)

type ProcessCSVPayload struct {
	JobID        string            `json:"job_id"`
	TraceHeaders map[string]string `json:"trace_headers,omitempty"`
}

func EnqueueProcessCSV(ctx context.Context, client *asynq.Client, jobID string) error {
	tracer := otel.Tracer(tracerName)
	ctx, span := tracer.Start(ctx, "asynq.enqueue/"+TaskProcessCSV,
		trace.WithSpanKind(trace.SpanKindProducer),
		trace.WithAttributes(
			attribute.String("messaging.system", "asynq"),
			attribute.String("messaging.operation", "publish"),
			attribute.String("asynq.task.type", TaskProcessCSV),
			attribute.String("job_id", jobID),
		),
	)
	defer span.End()

	payload, err := json.Marshal(ProcessCSVPayload{
		JobID:        jobID,
		TraceHeaders: InjectTraceContext(ctx),
	})
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	task := asynq.NewTask(TaskProcessCSV, payload, asynq.MaxRetry(3))
	info, err := client.EnqueueContext(ctx, task)
	if err != nil {
		return fmt.Errorf("enqueue task: %w", err)
	}

	slog.InfoContext(
		ctx,
		"enqueued csv processing task",
		"task_id",
		info.ID,
		"queue",
		info.Queue,
		"job_id",
		jobID,
	)
	return nil
}

func RegisterHandlers(proc *Processor) *asynq.ServeMux {
	mux := asynq.NewServeMux()
	mux.Use(OtelMiddleware())
	mux.HandleFunc(TaskProcessCSV, proc.HandleProcessCSV)
	return mux
}
