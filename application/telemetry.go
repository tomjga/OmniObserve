package main

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// initTracer wires the OpenTelemetry SDK and registers a global TracerProvider that
// exports spans via OTLP/gRPC to the OTel Collector. The endpoint comes from the
// standard OTEL_EXPORTER_OTLP_ENDPOINT env var (default localhost:4317 when unset);
// run the collector locally with:
//
//	docker compose -f collector/docker-compose.yaml up
//
// The gRPC client connects lazily, so startup never blocks when no collector is up —
// exports simply fail in the background until one is reachable. Returns the
// provider's Shutdown func so main can flush spans on exit.
func initTracer() (func(context.Context) error, error) {
	exporter, err := otlptracegrpc.New(context.Background(), otlptracegrpc.WithInsecure())
	if err != nil {
		return nil, err
	}

	res, err := resource.New(context.Background(),
		resource.WithAttributes(
			semconv.ServiceName("api-service"),
			semconv.ServiceVersion(version),
		),
	)
	if err != nil {
		return nil, err
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)

	// W3C TraceContext + Baggage so spans propagate across service boundaries.
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{}, propagation.Baggage{},
	))

	return tp.Shutdown, nil
}
