# Decision Gateway

Self-hosted service exposing reusable, versioned decision contracts through provider-independent REST APIs.

> Status: integration baseline complete. Registry, validation, discovery, execution, fallback, typed provider errors, OpenTelemetry, hardened container/Kubernetes deployment, and release automation are implemented.

## Run

Requirements: Go 1.24+.

```sh
go run ./cmd/decision-gateway
```

```sh
curl -s http://localhost:8080/healthz
curl -s http://localhost:8080/v1/decisions
curl -s http://localhost:8080/v1/decisions/task-triage/versions/v1
```

Configure at least one provider, then execute a decision:

```sh
# Self-hosted Laya. Add DG_LAYA_API_KEY when LAYA_API_KEY is enabled.
export DG_LAYA_BASE_URL=http://localhost:8000

# Optional fallback through local Ollama. Use https://ollama.com plus
# DG_CLEF_API_KEY for Ollama cloud.
export DG_CLEF_BASE_URL=http://localhost:11434
export DG_CLEF_MODEL=clef

go run ./cmd/decision-gateway
```

```sh
curl -sS -X POST \
  http://localhost:8080/v1/decisions/task-triage/versions/v1:execute \
  -H 'content-type: application/json' \
  -d '{"title":"Add distributed tracing","description":"Instrument the API gateway with OpenTelemetry"}' | jq
```

Both adapters call `POST /v1/systemone`. Laya may omit `model`; Ollama requires a decision model such as `clef`, `clef-flash`, or a cloud model reference.

## Adding decisions and endpoints

Each YAML definition automatically creates describe and execute endpoints from `metadata.name` and `metadata.version`. Questions come from `spec.decisions`; each question defines an instruction and 2–32 allowed options. No Go route changes are required.

See [`docs/adding-decisions.md`](docs/adding-decisions.md) for a complete example, supported input schema, question/option guidance, versioning, routing, Docker Compose, and Kubernetes deployment steps.

## Configuration

| Variable | Default | Purpose |
|---|---:|---|
| `DG_LISTEN_ADDR` | `:8080` | HTTP listen address |
| `DG_DEFINITIONS_DIR` | `./decisions` | startup-loaded YAML definitions |
| `DG_MAX_BODY_BYTES` | `1048576` | request body limit |
| `DG_MAX_CONCURRENT` | `64` | concurrent execution limit |
| `DG_LAYA_BASE_URL` | empty | Laya server root URL |
| `DG_LAYA_API_KEY` | empty | optional Laya bearer token |
| `DG_LAYA_MODEL` | empty | optional Laya model override |
| `DG_CLEF_BASE_URL` | empty | Ollama server root URL or `https://ollama.com` |
| `DG_CLEF_API_KEY` | empty | optional Ollama bearer token |
| `DG_CLEF_MODEL` | `clef` | Ollama decision model |
| `OTEL_SERVICE_NAME` | `decision-gateway` | OpenTelemetry service name |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | empty | shared OTLP endpoint; enables traces and metrics |
| `OTEL_EXPORTER_OTLP_TRACES_ENDPOINT` | empty | trace-only OTLP endpoint |
| `OTEL_EXPORTER_OTLP_METRICS_ENDPOINT` | empty | metric-only OTLP endpoint |
| `OTEL_EXPORTER_OTLP_PROTOCOL` | `grpc` | `grpc` or `http/protobuf`; signal-specific variants override it |
| `OTEL_SDK_DISABLED` | `false` | disable all telemetry when `true` |

A provider is registered only when its base URL is configured. Startup fails when any loaded decision has neither its primary nor fallback provider configured. `/readyz` returns `503` for the same condition.

## OpenTelemetry

Telemetry stays disabled unless a shared or signal-specific OTLP endpoint is configured. HTTP server/client instrumentation emits standard spans and metrics. Decision spans add decision name, version, selected provider, and fallback usage without recording request payloads. Provider failure logs include trace/span IDs and typed error metadata.

```sh
export OTEL_SERVICE_NAME=decision-gateway
export OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4317
export OTEL_EXPORTER_OTLP_PROTOCOL=grpc
go run ./cmd/decision-gateway
```

Health and readiness probes are excluded from tracing. Trace context and baggage propagate through provider requests.

## n8n

Import [`examples/n8n/task-triage.json`](examples/n8n/task-triage.json), update its gateway URL, and run the workflow. See [`examples/n8n/README.md`](examples/n8n/README.md).

## Releases

Pushing a `v*` tag builds checksummed Linux, macOS, and Windows archives plus multi-architecture GHCR images. Build metadata is embedded in startup logs and OCI image labels.

## Layout

```text
cmd/decision-gateway/  process entrypoint
internal/api/          REST API and error envelope
internal/config/       environment configuration
internal/definition/   contract types and supported schema subset
internal/engine/       timeout, fallback, output normalization
internal/provider/     provider interface, registry, System One adapter
internal/registry/     immutable startup registry
internal/telemetry/    OTLP trace and metric providers
decisions/             example contracts
deploy/kubernetes/     hardened baseline manifests
docs/                   decision authoring and operations guides
examples/n8n/          importable workflow example
```

## Development

```sh
make test
make lint
make build
docker compose up --build
```

## Milestones

- **M0:** complete — verified System One protocol for Laya and Ollama-hosted Clef
- **M1:** complete — contract, registry, validation, and discovery
- **M2:** complete — execution API, request IDs, deadlines, fallback, and mock tests
- **M3:** complete — live-tested System One adapters and typed provider errors
- **M4:** complete — OpenTelemetry, release hardening, and n8n example

## License

MIT
