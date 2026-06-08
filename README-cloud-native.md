# Cloud-Native Branch README

This document explains how the `yunyuansheng` branch differs from the main branch and how to use the new Kubernetes and Knative deployment path.

## What Changed Compared With `main`

The `main` branch runs qqqai as a single Go service. It exposes the WebSocket endpoint, initializes the RAG pipeline, handles group-file indexing in the same process, and is mainly deployed through Docker Compose.

This branch keeps that behavior available, but adds a cloud-native deployment shape:

- The Go binary now supports two runtime modes:
  - `server`: runs the WebSocket bot service.
  - `indexer`: runs a small HTTP service for group-file indexing.
- The WebSocket service still exposes `/ws`, and now also exposes:
  - `/healthz` for liveness checks.
  - `/readyz` for readiness checks.
- Group-file indexing has been moved into `tool/groupfile`, so it can run either:
  - locally inside the main server process, like `main`; or
  - remotely through an HTTP indexer service, controlled by `INDEXER_URL`.
- A Helm chart was added under `charts/qqqai` for cloud-neutral Kubernetes deployment.
- The file indexer can be deployed as a Knative Serving Service with scale-to-zero behavior.

## Runtime Modes

The default Docker command is still the main bot service:

```sh
./qqqai server
```

The same image can also run the indexer:

```sh
./qqqai indexer
```

You can also choose the mode through environment variables:

```sh
APP_MODE=server ./qqqai
APP_MODE=indexer ./qqqai
```

`QQQAI_MODE` is also supported and takes precedence over `APP_MODE`.

## Group-File Indexing Behavior

In `main`, group-file upload events are downloaded and indexed directly inside the WebSocket service.

In this branch:

- If `INDEXER_URL` is empty, the server keeps the same local indexing behavior as `main`.
- If `INDEXER_URL` is set, the server sends the group-file indexing request to:

```text
POST {INDEXER_URL}/index/group-file
```

This allows the long-running WebSocket service to stay as a normal Kubernetes Deployment, while file indexing can run as a serverless Knative workload.

## Kubernetes And Knative Deployment

The new Helm chart lives in:

```text
charts/qqqai
```

It deploys:

- `qqqai-app` as a Kubernetes Deployment.
- `qqqai-indexer` as a Knative Serving Service.
- MySQL, Redis, Milvus, Elasticsearch, etcd, and MinIO as in-cluster demo dependencies.
- ConfigMap, Secret, PVCs, Services, and optional Ingress resources.

The app is intentionally set to one replica by default, because some conversation state still lives in process memory. Scaling the app horizontally should be done only after moving all session memory to shared storage such as Redis.

The indexer defaults to:

```yaml
autoscaling.knative.dev/min-scale: "0"
autoscaling.knative.dev/max-scale: "3"
```

This demonstrates serverless scale-to-zero while keeping the main WebSocket service stable.

## Example Install

Install Knative Serving in the target cluster first, then run:

```sh
helm upgrade --install qqqai ./charts/qqqai \
  --namespace qqqai --create-namespace \
  --set image.repository=your-registry/qqqai \
  --set image.tag=latest \
  --set config.botQQ=123456 \
  --set secrets.deepseekKey=your-key \
  --set secrets.napcatAccessToken=your-token
```

For production, replace the in-cluster MySQL, Redis, Milvus, Elasticsearch, etcd, and MinIO services with managed services and override the corresponding values.

## Quick Difference Table

| Area | `main` | `yunyuansheng` |
| --- | --- | --- |
| Process shape | One Go server | One image, two modes: `server` and `indexer` |
| WebSocket | `/ws` only | `/ws`, `/healthz`, `/readyz` |
| File indexing | In-process | In-process fallback or remote HTTP indexer |
| Deployment | Docker Compose focused | Docker Compose-compatible plus Helm/K8s/Knative |
| Serverless | Not included | Knative Serving indexer with scale-to-zero |
| App replicas | Single process | Still defaults to one app replica |

## Validation

The Go code has been verified with:

```sh
go test ./...
```

Helm template validation still needs to be run in an environment with the Helm CLI installed:

```sh
helm lint ./charts/qqqai
helm template qqqai ./charts/qqqai --namespace qqqai
```
