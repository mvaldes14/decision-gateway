package telemetry

import "testing"

func TestSignalEnabled(t *testing.T) {
	for _, name := range []string{
		"OTEL_SDK_DISABLED",
		"OTEL_TRACES_EXPORTER",
		"OTEL_METRICS_EXPORTER",
		"OTEL_EXPORTER_OTLP_ENDPOINT",
		"OTEL_EXPORTER_OTLP_TRACES_ENDPOINT",
		"OTEL_EXPORTER_OTLP_METRICS_ENDPOINT",
		"OTEL_EXPORTER_OTLP_PROTOCOL",
		"OTEL_EXPORTER_OTLP_TRACES_PROTOCOL",
		"OTEL_EXPORTER_OTLP_METRICS_PROTOCOL",
	} {
		t.Setenv(name, "")
	}
	if signalEnabled("TRACES") {
		t.Fatal("traces enabled without endpoint")
	}

	t.Setenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", "http://collector:4317")
	if !signalEnabled("TRACES") || signalEnabled("METRICS") {
		t.Fatal("signal-specific endpoint enabled wrong signals")
	}

	t.Setenv("OTEL_TRACES_EXPORTER", "none")
	if signalEnabled("TRACES") {
		t.Fatal("traces enabled with exporter=none")
	}

	t.Setenv("OTEL_TRACES_EXPORTER", "")
	t.Setenv("OTEL_SDK_DISABLED", "true")
	if signalEnabled("TRACES") {
		t.Fatal("traces enabled with SDK disabled")
	}
}

func TestSignalProtocol(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_PROTOCOL", "")
	t.Setenv("OTEL_EXPORTER_OTLP_TRACES_PROTOCOL", "")
	if got := signalProtocol("TRACES"); got != "grpc" {
		t.Fatalf("default protocol = %q", got)
	}

	t.Setenv("OTEL_EXPORTER_OTLP_PROTOCOL", "http/protobuf")
	if got := signalProtocol("TRACES"); got != "http/protobuf" {
		t.Fatalf("shared protocol = %q", got)
	}

	t.Setenv("OTEL_EXPORTER_OTLP_TRACES_PROTOCOL", "grpc")
	if got := signalProtocol("TRACES"); got != "grpc" {
		t.Fatalf("signal protocol = %q", got)
	}
}
