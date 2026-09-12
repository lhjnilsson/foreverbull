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

Do not pass Alpaca keys as chart values. `broker.trading.enabled` stays false.
