---
description: "Build, push, and deploy memorix server to k8s. Run: go vet → docker build → docker push → kubectl rollout → healthz verify."
---

# /ship-server — Build & deploy memorix server

You are deploying the memorix Go server (`server/`).

## Steps

Execute these steps sequentially. Stop and report on first failure.

### 1. Quality check
```bash
cd /mnt/memorix/server && go vet ./...
```
If vet fails, stop and show errors.

### 2. Docker build
```bash
cd /mnt/memorix && docker build -f server/Dockerfile -t mirrors.tencent.com/cvm/memorix:latest .
```

### 3. Docker push
```bash
docker push mirrors.tencent.com/cvm/memorix:latest
```

### 4. Rollout restart
```bash
kubectl -n rag-etl rollout restart deploy/memorix
```

### 5. Wait for ready
Poll until the new pod is 1/1 Running (max 60s):
```bash
kubectl -n rag-etl get pods -l k8s-app=memorix
```

### 6. Health check
Verify healthz from inside the new pod:
```bash
POD=$(kubectl -n rag-etl get pod -l k8s-app=memorix -o jsonpath='{.items[0].metadata.name}')
kubectl -n rag-etl exec "$POD" -- wget -q -O- http://localhost:8080/healthz
```

### 7. Report
Print a summary:
- ✅ / ❌ for each step
- Pod name and status
- Service endpoint: `memorix.rag-etl.svc:8080`
