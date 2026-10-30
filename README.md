<p align="center">
  <img src="assets/decision-gateway.svg" alt="Decision Gateway icon" width="152" height="152">
</p>

<h1 align="center">Decision Gateway</h1>

<p align="center">
  <strong>Decisions as APIs.</strong><br>
  Turn structured inference into stable, reusable, provider-independent endpoints.
</p>

<p align="center">
  <a href="https://github.com/mvaldes14/decision-gateway/actions/workflows/ci.yaml"><img alt="CI" src="https://github.com/mvaldes14/decision-gateway/actions/workflows/ci.yaml/badge.svg"></a>
  <img alt="Go 1.24+" src="https://img.shields.io/badge/Go-1.24%2B-00ADD8?logo=go&logoColor=white">
  <a href="LICENSE"><img alt="License: MIT" src="https://img.shields.io/badge/License-MIT-22C55E.svg"></a>
  <img alt="Status: integration baseline" src="https://img.shields.io/badge/status-integration%20baseline-8B5CF6">
</p>

Decision Gateway is a self-hosted service for publishing reusable, versioned **decision contracts** over REST. Applications send domain data instead of provider prompts and receive validated choices through a consistent response envelope.

Define classifications, routing rules, severity levels, effort estimates, or other bounded decisions once. Call them unchanged from n8n, backend services, automation, or agents while the gateway owns provider translation, validation, fallback, and telemetry.

## Why Decision Gateway

| Capability | Product value |
|---|---|
| **Contract-first** | Versioned YAML defines inputs, questions, allowed options, routing, and timeout behavior. |
| **Provider-independent** | One API works across Laya and Ollama-hosted Clef through the System One protocol. |
| **Validated end to end** | Inputs are schema-checked and provider outputs must match configured option IDs. |
| **Resilient execution** | Deadlines, concurrency limits, typed errors, and primary/fallback routing are built in. |
| **Observable by default** | Standard HTTP metrics cover inbound and provider traffic; decision spans record routing and fallback. |
| **Operations-ready** | Stateless Go binary, hardened container, Kubernetes baseline, CI, and release automation. |

## How it works

```text
n8n / services / agents
          │  domain JSON
          ▼
┌──────────────────────────────┐
│ Decision Gateway             │
│ contract → validate → route  │
│ execute → verify → normalize │
└──────────────┬───────────────┘
               │ System One
        ┌──────┴──────┐
        ▼             ▼
      Laya      Ollama / Clef
```

A decision definition automatically creates discoverable describe and execute endpoints. Adding a new use case requires YAML, not another model integration or HTTP handler.

### Good fits

- Issue, ticket, and incident triage
- Ownership and queue routing
- Severity or risk classification
- Bounded effort estimation
- Automation steps that must return one of approved choices

### Product scope

Decision Gateway handles bounded decisions with approved outputs. Chat model routing, workflow side effects, and agent orchestration stay outside the service. Agents and automation tools consume its contracts as regular HTTP APIs.

> **Project status:** M0 through M4 integration baseline complete. Live Clef verification and MCP exposure remain on the roadmap.

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

Each YAML definition automatically creates describe and execute endpoints from `metadata.name` and `metadata.version`. Questions come from `spec.decisions`; each question defines an instruction and 2 to 32 allowed options. No Go route changes are required.

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
assets/                project icon and visual assets
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

- **M0:** complete; verified System One protocol for Laya and Ollama-hosted Clef
- **M1:** complete; contract, registry, validation, and discovery
- **M2:** complete; execution API, request IDs, deadlines, fallback, and mock tests
- **M3:** complete; live-tested System One adapters and typed provider errors
- **M4:** complete; OpenTelemetry, release hardening, and n8n example

## License

MIT
