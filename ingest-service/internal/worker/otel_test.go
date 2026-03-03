package worker

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/hibiken/asynq"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func TestProcessCSVPayload_OmitsTraceHeadersWhenNil(t *testing.T) {
	payload := ProcessCSVPayload{JobID: "abc123"}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	// trace_headers must not appear in JSON — the worker uses its presence
	// to decide whether to extract trace context from the payload.
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, exists := raw["trace_headers"]; exists {
		t.Fatal(
			"trace_headers should be omitted from JSON when nil — would cause false trace extraction",
		)
	}
}

func TestInjectAndExtractTraceContext_RoundTrip(t *testing.T) {
	// Set up a real trace provider + propagator so we can verify round-trip
	tp := sdktrace.NewTracerProvider()
	defer tp.Shutdown(context.Background())

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	tracer := tp.Tracer("test")
	ctx, span := tracer.Start(context.Background(), "test-span")
	defer span.End()

	// Producer side: inject trace context
	headers := InjectTraceContext(ctx)
	if headers == nil {
		t.Fatal("InjectTraceContext returned nil")
	}
	if headers["traceparent"] == "" {
		t.Fatal("expected traceparent header to be set by propagator")
	}

	// Simulate: marshal payload with headers, create task, extract
	payload := ProcessCSVPayload{
		JobID:        "job-1",
		TraceHeaders: headers,
	}
	data, _ := json.Marshal(payload)
	task := asynq.NewTask(TaskProcessCSV, data)

	extracted := extractTraceHeaders(task)
	if extracted == nil {
		t.Fatal("extractTraceHeaders returned nil")
	}
	if extracted["traceparent"] != headers["traceparent"] {
		t.Errorf("traceparent mismatch: injected %q, extracted %q",
			headers["traceparent"], extracted["traceparent"])
	}
}

func TestExtractTraceHeaders_NoHeaders(t *testing.T) {
	payload := ProcessCSVPayload{JobID: "test"}
	data, _ := json.Marshal(payload)
	task := asynq.NewTask("test:task", data)

	headers := extractTraceHeaders(task)
	if headers != nil {
		t.Errorf("expected nil when no trace_headers in payload, got %v", headers)
	}
}

func TestExtractTraceHeaders_InvalidJSON(t *testing.T) {
	task := asynq.NewTask("test:task", []byte("{broken json"))
	headers := extractTraceHeaders(task)
	if headers != nil {
		t.Errorf("expected nil for invalid JSON, got %v", headers)
	}
}

func TestExtractTraceHeaders_EmptyPayload(t *testing.T) {
	task := asynq.NewTask("test:task", []byte("{}"))
	headers := extractTraceHeaders(task)
	if headers != nil {
		t.Errorf("expected nil for empty JSON object, got %v", headers)
	}
}
