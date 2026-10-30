# Decision Gateway

Self-hosted service exposing reusable, versioned decision contracts through provider-independent REST APIs.

> Status: early integration. Registry, validation, discovery, execution, fallback, and System One adapters for Laya and Ollama-hosted Clef exist. OpenTelemetry and release hardening remain.

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

A provider is registered only when its base URL is configured. Standard `OTEL_*` variables remain reserved for telemetry.

## Layout

```text
cmd/decision-gateway/  process entrypoint
internal/api/          REST API and error envelope
internal/config/       environment configuration
internal/definition/   contract types and supported schema subset
internal/engine/       timeout, fallback, output normalization
internal/provider/     provider interface, registry, System One adapter
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

- **M0:** complete — verified System One protocol for Laya and Ollama-hosted Clef
- **M1:** complete — contract, registry, validation, and discovery
- **M2:** complete — execution API, request IDs, deadlines, fallback, and mock tests
- **M3:** in progress — live System One adapters implemented; real endpoint testing and typed provider errors remain
- **M4:** OpenTelemetry, release hardening, and n8n example

## License

MIT
