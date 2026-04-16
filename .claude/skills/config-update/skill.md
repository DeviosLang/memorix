---
description: "Update memorix k8s configmap or secret, then restart the affected deployment. Usage: /config-update [key=value ...] or /config-update secret [key=value ...]."
---

# /config-update — Update configuration and restart

Update memorix ConfigMap or Secret in rag-etl namespace, then restart the affected deployment.

## Arguments

- `key=value` pairs → update ConfigMap `memorix-config`
- `secret key=value` pairs → update Secret `memorix-secrets`
- No arguments → show current config values

## Steps

### 1. Show current config (if no arguments)

```bash
kubectl -n rag-etl get configmap memorix-config -o yaml
```
For secrets (keys only, no values):
```bash
kubectl -n rag-etl get secret memorix-secrets -o jsonpath='{.data}' | python3 -c "import sys,json; print('\n'.join(json.loads(sys.stdin.read()).keys()))"
```

### 2. Update config (if arguments provided)

**ConfigMap update:**
```bash
kubectl -n rag-etl get configmap memorix-config -o json | \
  python3 -c "
import sys, json
cm = json.load(sys.stdin)
updates = {<key>: <value> for each provided pair}
cm['data'].update(updates)
json.dump(cm, sys.stdout)
" | kubectl apply -f -
```

**Secret update:**
```bash
kubectl -n rag-etl get secret memorix-secrets -o json | \
  python3 -c "
import sys, json, base64
s = json.load(sys.stdin)
updates = {<key>: base64.b64encode(<value>.encode()).decode() for each pair}
s['data'].update(updates)
json.dump(s, sys.stdout)
" | kubectl apply -f -
```

### 3. Restart affected deployment

ConfigMap changes affect memorix server:
```bash
kubectl -n rag-etl rollout restart deploy/memorix
```

### 4. Wait for ready

Poll until the new pod is 1/1 Running (max 60s).

### 5. Health check
```bash
POD=$(kubectl -n rag-etl get pod -l k8s-app=memorix -o jsonpath='{.items[0].metadata.name}')
kubectl -n rag-etl exec "$POD" -- wget -q -O- http://localhost:8080/healthz
```

### 6. Report

- Which keys were updated
- Deployment restart status
- ✅ / ❌ healthz result
