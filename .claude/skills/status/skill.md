---
description: "Show health and status of all memorix components in the rag-etl k8s namespace. Checks pods, services, endpoints, and healthz."
---

# /status — Memorix cluster status

Check all memorix components in the `rag-etl` namespace. No hardcoded IPs — everything is discovered dynamically.

## Steps

Run all checks and present a unified report.

### 1. Pods
```bash
kubectl -n rag-etl get pods -l 'k8s-app in (memorix),app in (memorix-mcp)' -o wide 2>/dev/null
kubectl -n rag-etl get pods -l k8s-app=memorix -o wide
kubectl -n rag-etl get pods -l app=memorix-mcp -o wide
```

### 2. Services
```bash
kubectl -n rag-etl get svc memorix memorix-mcp -o wide
```

### 3. Endpoints
```bash
kubectl -n rag-etl get endpoints memorix memorix-mcp
```

### 4. Health checks
For each running pod, exec a healthz check:

**memorix server:**
```bash
POD=$(kubectl -n rag-etl get pod -l k8s-app=memorix -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
if [ -n "$POD" ]; then
  kubectl -n rag-etl exec "$POD" -- wget -q -O- http://localhost:8080/healthz
fi
```

**memorix-mcp:**
```bash
POD=$(kubectl -n rag-etl get pod -l app=memorix-mcp -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
if [ -n "$POD" ]; then
  kubectl -n rag-etl exec "$POD" -- wget -q -O- http://localhost:8080/healthz
fi
```

### 5. Report format

Present results as a clean table:

```
Component        Pod                           Status    Svc Type      Svc Address          Healthz
memorix          memorix-xxx-yyy               1/1       LoadBalancer  <ip>:8080            ok
memorix-mcp      memorix-mcp-xxx-yyy           1/1       LoadBalancer  <ip>:8080            ok
```

If any component is unhealthy, show ❌ and the error details.
