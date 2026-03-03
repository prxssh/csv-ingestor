package worker

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hibiken/asynq"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const tracerName = "github.com/prxssh/csv-ingestor/ingest-service/internal/worker"

// carrier adapts a map[string]string for OTel context propagation.
type carrier map[string]string

func (c carrier) Get(key string) string { return c[key] }
func (c carrier) Set(key, value string) { c[key] = value }
func (c carrier) Keys() []string {
	keys := make([]string, 0, len(c))
	for k := range c {
		keys = append(keys, k)
	}
	return keys
}

// InjectTraceContext serializes the current span context into a map
// for embedding in asynq task payloads (producer side).
func InjectTraceContext(ctx context.Context) map[string]string {
	c := make(carrier)
	otel.GetTextMapPropagator().Inject(ctx, c)
	return c
}

// OtelMiddleware creates an asynq middleware that:
//  1. Extracts trace context propagated through the task payload
//  2. Creates a consumer span wrapping the task handler
//  3. Records task type as span attributes
//  4. Marks the span as error on handler failure
func OtelMiddleware() asynq.MiddlewareFunc {
	tracer := otel.Tracer(tracerName)
	propagator := otel.GetTextMapPropagator()

	return func(next asynq.Handler) asynq.Handler {
		return asynq.HandlerFunc(func(ctx context.Context, t *asynq.Task) error {
			// Extract parent trace context from task payload
			traceHeaders := extractTraceHeaders(t)
			if len(traceHeaders) > 0 {
				ctx = propagator.Extract(ctx, carrier(traceHeaders))
			}

			spanName := fmt.Sprintf("asynq.task/%s", t.Type())
			ctx, span := tracer.Start(ctx, spanName,
				trace.WithSpanKind(trace.SpanKindConsumer),
				trace.WithAttributes(
					attribute.String("messaging.system", "asynq"),
					attribute.String("messaging.operation", "process"),
					attribute.String("asynq.task.type", t.Type()),
				),
			)
			defer span.End()

			err := next.ProcessTask(ctx, t)
			if err != nil {
				span.SetStatus(codes.Error, err.Error())
				span.RecordError(err)
			} else {
				span.SetStatus(codes.Ok, "")
			}

			return err
		})
	}
}

// extractTraceHeaders reads trace_headers from the task payload JSON.
func extractTraceHeaders(t *asynq.Task) map[string]string {
	var envelope struct {
		TraceHeaders map[string]string `json:"trace_headers"`
	}
	if err := json.Unmarshal(t.Payload(), &envelope); err != nil {
		return nil
	}
	return envelope.TraceHeaders
}
