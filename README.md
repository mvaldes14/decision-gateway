# Decision Gateway

Self-hosted service exposing reusable, versioned decision contracts through provider-independent REST APIs.

> Status: scaffold. Registry, schema validation, discovery API, execution engine, fallback, container, and Kubernetes baseline exist. Laya/CLEF adapters and OpenTelemetry remain gated on provider protocol validation.

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

Execution currently returns `PROVIDER_ERROR` because real adapters are intentionally not stubbed with unverified protocols.

```sh
curl -sS -X POST \
  http://localhost:8080/v1/decisions/task-triage/versions/v1:execute \
  -H 'content-type: application/json' \
  -d '{"title":"Add distributed tracing","description":"Instrument the API gateway with OpenTelemetry"}'
```

## Configuration

| Variable | Default | Purpose |
|---|---:|---|
| `DG_LISTEN_ADDR` | `:8080` | HTTP listen address |
| `DG_DEFINITIONS_DIR` | `./decisions` | startup-loaded YAML definitions |
| `DG_MAX_BODY_BYTES` | `1048576` | request body limit |
| `DG_MAX_CONCURRENT` | `64` | concurrent execution limit |

Provider and standard `OTEL_*` variables are reserved in `.env.example`; code will consume them when adapters and telemetry land.

## Layout

```text
cmd/decision-gateway/  process entrypoint
internal/api/          REST API and error envelope
internal/config/       environment configuration
internal/definition/   contract types and supported schema subset
internal/engine/       timeout, fallback, output normalization
internal/provider/     provider interface and registry
internal/registry/     immutable startup registry
decisions/             example contracts
deploy/kubernetes/     baseline manifests
```

## Development

```sh
make test
make lint
make build
docker compose up --build
```

## Milestones

- **M0:** verify Laya/CLEF protocols and capture fixtures
- **M1:** contract and registry
- **M2:** execution API, request IDs, deadlines, mock-provider integration tests
- **M3:** real adapters and typed provider errors
- **M4:** OpenTelemetry, release hardening, n8n example

## License

MIT
