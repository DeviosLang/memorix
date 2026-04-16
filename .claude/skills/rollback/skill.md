---
description: "Roll back a memorix component to the previous revision. Usage: /rollback [server|mcp]. Defaults to showing current revision info."
---

# /rollback — Roll back a deployment

Roll back a memorix component to the previous deployment revision.

## Arguments

- `server` or `memorix` → rollback deploy/memorix
- `mcp` or `memorix-mcp` → rollback deploy/memorix-mcp
- No argument → show rollout history for both (do NOT rollback)

## Steps

### 1. Show current state

Before any rollback, show current revision:
```bash
kubectl -n rag-etl rollout history deploy/<name> | tail -5
```
And current pod image:
```bash
kubectl -n rag-etl get deploy <name> -o jsonpath='{.spec.template.spec.containers[0].image}'
```

### 2. Confirm

If a component was specified, **ask the user to confirm** before proceeding. Show what will happen:
- Current revision number
- The deployment that will be rolled back

### 3. Execute rollback
```bash
kubectl -n rag-etl rollout undo deploy/<name>
```

### 4. Wait for ready

Poll until the pod is 1/1 Running (max 60s):
```bash
kubectl -n rag-etl get pods -l <label-selector>
```

### 5. Health check
```bash
POD=$(kubectl -n rag-etl get pod -l <label-selector> -o jsonpath='{.items[0].metadata.name}')
kubectl -n rag-etl exec "$POD" -- wget -q -O- http://localhost:8080/healthz
```

### 6. Report

- ✅ Rollback successful — new pod name, image, healthz status
- ❌ Rollback failed — show error and current pod state
