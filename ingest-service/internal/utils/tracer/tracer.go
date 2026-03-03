package tracer

import (
	"context"
	"encoding/binary"
	"log/slog"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

type customSamplerProcessor struct {
	next      sdktrace.SpanProcessor
	threshold uint64
}

func (c *customSamplerProcessor) OnStart(parent context.Context, s sdktrace.ReadWriteSpan) {
	c.next.OnStart(parent, s)
}

func (c *customSamplerProcessor) OnEnd(s sdktrace.ReadOnlySpan) {
	if s.Status().Code == codes.Error {
		c.next.OnEnd(s)
		return
	}

	// Sample % of successful requests based on threshold
	// Use the lower 64 bits of the trace ID for sampling decision
	traceID := s.SpanContext().TraceID()
	x := binary.BigEndian.Uint64(traceID[8:16])

	if x < c.threshold {
		c.next.OnEnd(s)
	}
}

func (c *customSamplerProcessor) Shutdown(ctx context.Context) error {
	return c.next.Shutdown(ctx)
}

func (c *customSamplerProcessor) ForceFlush(ctx context.Context) error {
	return c.next.ForceFlush(ctx)
}

func NewOtelTracer(
	ctx context.Context,
	serviceName, collectorURL string,
	samplingRate float64,
) func(context.Context) error {
	exporter, err := otlptrace.New(
		ctx,
		otlptracehttp.NewClient(
			otlptracehttp.WithEndpointURL(collectorURL),
		),
	)
	if err != nil {
		slog.Error("failed to create exporter", "error", err)
		panic(err)
	}

	resources, err := sdkresource.New(
		ctx,
		sdkresource.WithAttributes(
			attribute.String("service.name", serviceName),
			attribute.String("library.language", "go"),
		),
	)
	if err != nil {
		slog.Error("could not set resources", "error", err)
		panic(err)
	}

	batcher := sdktrace.NewBatchSpanProcessor(exporter)

	// Calculate threshold for probabilistic sampling
	// threshold = rate * 2^64
	const maxUint64 = ^uint64(0)
	threshold := uint64(samplingRate * float64(maxUint64))

	processor := &customSamplerProcessor{
		next:      batcher,
		threshold: threshold,
	}

	otel.SetTracerProvider(
		sdktrace.NewTracerProvider(
			// We MUST sample 100% at the head to ensure our processor sees every span
			// and can decide to keep/drop based on status (which is known at the end).
			sdktrace.WithSampler(sdktrace.AlwaysSample()),
			sdktrace.WithSpanProcessor(processor),
			sdktrace.WithResource(resources),
		),
	)
	return processor.Shutdown
}
