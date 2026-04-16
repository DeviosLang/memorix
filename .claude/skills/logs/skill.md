---
description: "View recent logs for memorix components. Usage: /logs [component] [lines]. Component: server or mcp (default: both). Lines: number of log lines (default: 50)."
---

# /logs — View component logs

Show recent logs for memorix components in rag-etl namespace.

## Arguments

- First argument: component name — `server`, `mcp`, or omit for both
- Second argument: number of lines (default 50)

## Steps

### 1. Determine target

Parse the user's input to determine which component(s) to show logs for:
- `server` or `memorix` → label `k8s-app=memorix`
- `mcp` or `memorix-mcp` → label `app=memorix-mcp`
- No argument → show both

### 2. Fetch logs

For each target component:
```bash
# memorix server
POD=$(kubectl -n rag-etl get pod -l k8s-app=memorix -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
kubectl -n rag-etl logs "$POD" --tail=<lines>

# memorix-mcp
POD=$(kubectl -n rag-etl get pod -l app=memorix-mcp -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
kubectl -n rag-etl logs "$POD" --tail=<lines>
```

### 3. Report

Show logs with a clear header for each component. If a pod is not found, report that the component is not running.
