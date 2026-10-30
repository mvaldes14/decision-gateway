package telemetry

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

type Shutdown func(context.Context) error

func Setup(ctx context.Context, serviceVersion string, logger *slog.Logger) (Shutdown, bool, error) {
	tracesEnabled := signalEnabled("TRACES")
	metricsEnabled := signalEnabled("METRICS")
	if !tracesEnabled && !metricsEnabled {
		return func(context.Context) error { return nil }, false, nil
	}

	serviceName := strings.TrimSpace(os.Getenv("OTEL_SERVICE_NAME"))
	if serviceName == "" {
		serviceName = "decision-gateway"
	}
	res, err := resource.New(ctx,
		resource.WithFromEnv(),
		resource.WithTelemetrySDK(),
		resource.WithAttributes(
			attribute.String("service.name", serviceName),
			attribute.String("service.version", serviceVersion),
		),
	)
	if err != nil {
		return nil, false, err
	}

	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))
	otel.SetErrorHandler(otel.ErrorHandlerFunc(func(err error) {
		logger.Error("OpenTelemetry error", "error", err)
	}))

	shutdowns := make([]Shutdown, 0, 2)
	if tracesEnabled {
		exporter, err := newTraceExporter(ctx)
		if err != nil {
			return nil, false, err
		}
		provider := sdktrace.NewTracerProvider(
			sdktrace.WithBatcher(exporter),
			sdktrace.WithResource(res),
		)
		otel.SetTracerProvider(provider)
		shutdowns = append(shutdowns, provider.Shutdown)
	}
	if metricsEnabled {
		exporter, err := newMetricExporter(ctx)
		if err != nil {
			_ = shutdownAll(ctx, shutdowns)
			return nil, false, err
		}
		provider := metric.NewMeterProvider(
			metric.WithReader(metric.NewPeriodicReader(exporter)),
			metric.WithResource(res),
		)
		otel.SetMeterProvider(provider)
		shutdowns = append(shutdowns, provider.Shutdown)
	}

	return func(ctx context.Context) error { return shutdownAll(ctx, shutdowns) }, true, nil
}

func newTraceExporter(ctx context.Context) (sdktrace.SpanExporter, error) {
	switch signalProtocol("TRACES") {
	case "grpc":
		return otlptracegrpc.New(ctx)
	case "http/protobuf":
		return otlptracehttp.New(ctx)
	default:
		return nil, fmt.Errorf("unsupported OTLP traces protocol %q", signalProtocol("TRACES"))
	}
}

func newMetricExporter(ctx context.Context) (metric.Exporter, error) {
	switch signalProtocol("METRICS") {
	case "grpc":
		return otlpmetricgrpc.New(ctx)
	case "http/protobuf":
		return otlpmetrichttp.New(ctx)
	default:
		return nil, fmt.Errorf("unsupported OTLP metrics protocol %q", signalProtocol("METRICS"))
	}
}

func signalProtocol(signal string) string {
	if value := strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_" + signal + "_PROTOCOL")); value != "" {
		return strings.ToLower(value)
	}
	if value := strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_PROTOCOL")); value != "" {
		return strings.ToLower(value)
	}
	return "grpc"
}

func signalEnabled(signal string) bool {
	if disabled, _ := strconv.ParseBool(os.Getenv("OTEL_SDK_DISABLED")); disabled {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(os.Getenv("OTEL_"+signal+"_EXPORTER")), "none") {
		return false
	}
	return strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")) != "" ||
		strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_"+signal+"_ENDPOINT")) != ""
}

func shutdownAll(ctx context.Context, shutdowns []Shutdown) error {
	var result error
	for i := len(shutdowns) - 1; i >= 0; i-- {
		result = errors.Join(result, shutdowns[i](ctx))
	}
	return result
}
