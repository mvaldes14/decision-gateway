# Kubernetes deployment

This directory is a Kustomize base composed from separate resources:

```text
configmap.yaml      decision definitions
deployment.yaml     gateway workload and provider configuration
pdb.yaml            disruption budget
service.yaml        ClusterIP service
kustomization.yaml  resource assembly and image selection
```

## Render and apply

Review the rendered resources before applying them:

```sh
kubectl kustomize deploy/kubernetes
kubectl apply -k deploy/kubernetes
kubectl rollout status deployment/decision-gateway
```

Verify the service locally:

```sh
kubectl port-forward service/decision-gateway 8080:8080
curl -sS http://localhost:8080/readyz | jq
curl -sS http://localhost:8080/v1/decisions | jq
```

## Provider configuration

`deployment.yaml` defaults to these in-cluster services:

```text
Laya: http://laya:8000
Clef: http://ollama:11434
```

Change them directly for a standalone deployment or patch them from an environment overlay.

Provider keys are read from the optional `decision-gateway-providers` Secret:

```sh
kubectl create secret generic decision-gateway-providers \
  --from-literal=laya-api-key='<key>' \
  --from-literal=clef-api-key='<key>'
```

Do not commit real credentials.

## Image selection

The base uses `ghcr.io/mvaldes14/decision-gateway:latest`. Pin a release or commit image in `kustomization.yaml`:

```yaml
images:
  - name: ghcr.io/mvaldes14/decision-gateway
    newTag: v0.1.0
```

For GitOps, override the image in an overlay instead of modifying the base.

## Adding decisions

Add each definition as another key in `configmap.yaml`. The key becomes the mounted filename; `metadata.name` and `metadata.version` determine the generated REST endpoints.

The registry loads definitions at startup, so restart the Deployment after ConfigMap changes:

```sh
kubectl rollout restart deployment/decision-gateway
kubectl rollout status deployment/decision-gateway
```

See [`../../docs/adding-decisions.md`](../../docs/adding-decisions.md) for schema, question, option, versioning, and routing guidance.

## Overlay example

Create an environment overlay outside the base:

```text
deploy/kubernetes/overlays/lab/
├── kustomization.yaml
└── deployment-patch.yaml
```

```yaml
# deploy/kubernetes/overlays/lab/kustomization.yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
  - ../..
patches:
  - path: deployment-patch.yaml
images:
  - name: ghcr.io/mvaldes14/decision-gateway
    newTag: v0.1.0
```

```yaml
# deploy/kubernetes/overlays/lab/deployment-patch.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: decision-gateway
spec:
  replicas: 1
  template:
    spec:
      containers:
        - name: decision-gateway
          env:
            - name: DG_LAYA_BASE_URL
              value: http://laya.ai.svc.cluster.local:8000
```

Apply it with:

```sh
kubectl apply -k deploy/kubernetes/overlays/lab
```
