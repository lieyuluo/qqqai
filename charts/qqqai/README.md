# qqqai Helm Chart

This chart deploys qqqai as a cloud-neutral Kubernetes workload:

- `qqqai-app` runs the WebSocket server as a normal Kubernetes Deployment.
- `qqqai-indexer` runs the group-file indexer as a Knative Serving Service.
- MySQL, Redis, Milvus, Elasticsearch, etcd, and MinIO run in-cluster for demos.

Install into a cluster that already has Knative Serving installed:

```sh
helm upgrade --install qqqai ./charts/qqqai \
  --namespace qqqai --create-namespace \
  --set image.repository=your-registry/qqqai \
  --set image.tag=latest \
  --set config.botQQ=123456 \
  --set secrets.deepseekKey=... \
  --set secrets.napcatAccessToken=...
```

For a real production deployment, replace the in-cluster stateful services with managed services and set the matching values.
