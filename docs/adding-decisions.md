# Adding decision endpoints

Decision Gateway generates REST endpoints from YAML decision definitions. You do not add Go handlers for each use case.

Place one definition per `.yaml` or `.yml` file in `DG_DEFINITIONS_DIR`:

- Local default: `./decisions`
- Container/Kubernetes default: `/etc/decision-gateway/decisions`

Definitions load once during startup. Add or change a file, then restart the gateway. Any malformed definition or decision without an available primary/fallback provider stops startup.

## Endpoint generation

For this metadata:

```yaml
metadata:
  name: incident-triage
  version: v1
```

the gateway exposes:

```text
GET  /v1/decisions
GET  /v1/decisions/incident-triage/versions/v1
POST /v1/decisions/incident-triage/versions/v1:execute
```

The filename does not control the URL. `metadata.name` and `metadata.version` do.

Use a new version such as `v2` for contract changes after clients consume a version. Keep both files when both versions must remain callable.

## Complete example

Create `decisions/incident-triage.yaml`:

```yaml
apiVersion: decisiongateway.dev/v1alpha1
kind: Decision
metadata:
  name: incident-triage
  version: v1
spec:
  description: Classify an operational incident and select its owning team
  input:
    type: object
    additionalProperties: false
    required: [service, summary, customerImpact]
    properties:
      service:
        type: string
        minLength: 1
        maxLength: 100
      summary:
        type: string
        minLength: 1
        maxLength: 2000
      customerImpact:
        type: boolean
      affectedRegions:
        type: array
        maxItems: 20
        items:
          type: string
          minLength: 1
          maxLength: 50
  decisions:
    severity:
      type: choice
      instruction: Select the incident severity from the impact and scope in the supplied state.
      options:
        sev1: Critical outage or severe widespread customer impact
        sev2: Major degradation or substantial customer impact
        sev3: Limited degradation with a workaround or narrow impact
        sev4: Minor issue with little or no customer impact
    owner:
      type: choice
      instruction: Select the team that should own initial incident response.
      options:
        platform: Kubernetes, networking, compute, or shared infrastructure
        observability: Telemetry ingestion, storage, querying, or alerting
        application: Product behavior, business logic, or application APIs
  routing:
    primary: laya
    fallback: clef
    timeout: 5s
  review:
    enabled: false
```

Restart, discover, and execute it:

```sh
curl -sS http://localhost:8080/v1/decisions | jq
curl -sS http://localhost:8080/v1/decisions/incident-triage/versions/v1 | jq
curl -sS -X POST \
  http://localhost:8080/v1/decisions/incident-triage/versions/v1:execute \
  -H 'content-type: application/json' \
  -d '{
    "service": "otel-gateway",
    "summary": "OTLP ingestion failing in two regions",
    "customerImpact": true,
    "affectedRegions": ["us-east-1", "eu-west-1"]
  }' | jq
```

Results are keyed by entries under `spec.decisions`:

```json
{
  "results": {
    "severity": {"choice": "sev1"},
    "owner": {"choice": "observability"}
  }
}
```

## Inputs

`spec.input` defines caller-supplied state and is validated before any provider call.

Supported schema types:

- `object`
- `string`
- `number`
- `integer`
- `boolean`
- `array`

Supported constraints:

- `required`
- `additionalProperties`
- `enum`
- `minLength` / `maxLength`
- `minimum` / `maximum`
- `minItems` / `maxItems`
- nested `properties`
- array `items`

Set `additionalProperties: false` on objects when unknown fields should be rejected.

## Questions and options

Each entry under `spec.decisions` becomes one System One choice question.

```yaml
decisions:
  severity:                 # Question/result ID
    type: choice            # Only supported decision type today
    instruction: ...        # Provider instruction for this question
    options:                # Allowed output IDs and their meanings
      sev1: ...
      sev2: ...
```

Rules:

- A definition must contain at least one decision.
- Decision names must be nonempty and are returned as keys under `results`.
- `type` must be `choice`.
- `instruction` must be nonempty.
- Each question requires 2–32 options.
- Option IDs and descriptions must be nonempty.
- Providers must return an exact configured option ID. Unknown choices are rejected.
- Multiple questions in one definition are sent together in one System One request.

Authoring guidance:

- Use short, stable, machine-friendly option IDs such as `sev1` or `platform`.
- Treat option IDs as API values; changing one is a contract change.
- Make descriptions mutually distinguishable, not synonyms.
- State the selection rule in `instruction`; do not restate the entire input payload.
- Keep caller data in `spec.input`. Do not create an option for every possible input value.
- Avoid provider-specific prompt syntax. Definitions should remain portable between Laya and Clef.

## Routing

Available provider names are currently:

- `laya`, configured with `DG_LAYA_BASE_URL`
- `clef`, configured with `DG_CLEF_BASE_URL`

Example:

```yaml
routing:
  primary: laya
  fallback: clef
  timeout: 5s
```

At least one named route must be configured when the process starts. If only the fallback is configured, execution attempts the missing primary and then uses the fallback. `timeout` is one total deadline shared by both attempts.

## Deployment

### Docker Compose

The repository Compose file mounts `./decisions` read-only. Add the YAML file there and recreate the service:

```sh
docker compose up -d --build --force-recreate
```

### Kubernetes

Add another key under the `decision-gateway-definitions` ConfigMap in `deploy/kubernetes/configmap.yaml`, or patch/replace that resource from a Kustomize overlay. The ConfigMap volume projects every key into the definitions directory.

Render and apply the manifests:

```sh
kubectl kustomize deploy/kubernetes
kubectl apply -k deploy/kubernetes
kubectl rollout status deployment/decision-gateway
```

The registry loads definitions only at process startup. After changing the ConfigMap, restart the pods unless an overlay or GitOps controller already changed the pod template and triggered a rollout:

```sh
kubectl rollout restart deployment/decision-gateway
kubectl rollout status deployment/decision-gateway
```

Verify loaded decisions:

```sh
kubectl port-forward service/decision-gateway 8080:8080
curl -sS http://localhost:8080/v1/decisions | jq
```

## Startup troubleshooting

Common failures:

- `no YAML decision definitions found`: definitions directory is empty or mounted incorrectly.
- `duplicate decision name/version`: two files use the same metadata pair.
- `provider "..." is not configured` during execution: route name has no configured base URL.
- `no configured provider available for decisions`: neither primary nor fallback is configured.
- Schema validation error: unsupported type, invalid constraint, or `required` references a missing property.
- Provider response rejected: returned question name or option ID does not match the definition.
