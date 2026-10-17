# Foreverbull Helm chart

Installs the six platform workloads into the release namespace (default
`foreverbull`). See [deploy/README.md](../../README.md) for the cluster
contract, value keys, and Kind host-port mappings.

```bash
# Local Kind
helm upgrade --install foreverbull deploy/helm/foreverbull \
  --namespace foreverbull \
  --create-namespace \
  --set expose.hostPort=true \
  --set global.imageTag=local

# Remote
helm upgrade --install foreverbull deploy/helm/foreverbull \
  --kubeconfig /path/to/kubeconfig \
  --kube-context my-context \
  --namespace my-namespace \
  --create-namespace \
  --set expose.hostPort=false
```

Generated Postgres/MinIO credentials live in Secret
`foreverbull-credentials` in the release namespace and are reused on
upgrade via a live-cluster lookup. Read them back with:

```bash
kubectl get secret foreverbull-credentials -n <namespace> -o jsonpath='{.data.postgres-password}' | base64 -d; echo
kubectl get secret foreverbull-credentials -n <namespace> -o jsonpath='{.data.minio-accessKey}' | base64 -d; echo
kubectl get secret foreverbull-credentials -n <namespace> -o jsonpath='{.data.minio-secretKey}' | base64 -d; echo
```

Do not pass broker credentials as chart values. `broker.trading.enabled` stays false.
Optional Secret `foreverbull-trading` key `profiles.yaml` is mounted at `/etc/foreverbull/profiles.yaml`.
