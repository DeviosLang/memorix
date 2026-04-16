---
description: "Build, push, and deploy memorix MCP server to k8s. Run: tsc → docker build → docker push → kubectl rollout → healthz + MCP tools verify."
---

# /ship-mcp — Build & deploy memorix MCP server

You are deploying the memorix MCP server (`mcp-server/`).

## Steps

Execute these steps sequentially. Stop and report on first failure.

### 1. Type check
```bash
cd /mnt/memorix/mcp-server && npx tsc --noEmit
```
If typecheck fails, stop and show errors.

### 2. Docker build
```bash
cd /mnt/memorix/mcp-server && docker build -t mirrors.tencent.com/cvm/memorix-mcp-server:latest .
```

### 3. Docker push
```bash
docker push mirrors.tencent.com/cvm/memorix-mcp-server:latest
```

### 4. Rollout restart
```bash
kubectl -n rag-etl rollout restart deploy/memorix-mcp
```

### 5. Wait for ready
Poll until the new pod is 1/1 Running (max 60s):
```bash
kubectl -n rag-etl get pods -l app=memorix-mcp
```

### 6. Health check
```bash
POD=$(kubectl -n rag-etl get pod -l app=memorix-mcp -o jsonpath='{.items[0].metadata.name}')
kubectl -n rag-etl exec "$POD" -- wget -q -O- http://localhost:8080/healthz
```

### 7. MCP tools verify
Verify all 6 tools are registered by running inside the pod:
```bash
kubectl -n rag-etl exec "$POD" -- node -e '
async function test() {
  const base = "http://localhost:8080/mcp";
  const hdrs = { "Content-Type": "application/json", "Accept": "application/json, text/event-stream" };
  const r1 = await fetch(base, { method: "POST", headers: hdrs,
    body: JSON.stringify({jsonrpc:"2.0",method:"initialize",params:{protocolVersion:"2025-03-26",capabilities:{},clientInfo:{name:"ship-mcp",version:"1.0"}},id:1})
  });
  const sid = r1.headers.get("mcp-session-id");
  await fetch(base, { method: "POST", headers: { ...hdrs, "mcp-session-id": sid },
    body: JSON.stringify({jsonrpc:"2.0",method:"notifications/initialized"})
  });
  const r3 = await fetch(base, { method: "POST", headers: { ...hdrs, "mcp-session-id": sid },
    body: JSON.stringify({jsonrpc:"2.0",method:"tools/list",id:2})
  });
  const body = await r3.text();
  const match = body.match(/data: (.+)/);
  if (match) {
    const msg = JSON.parse(match[1]);
    const tools = msg.result.tools.map(t => t.name);
    console.log("tools:", tools.join(", "));
    console.log("count:", tools.length);
    process.exit(tools.length === 6 ? 0 : 1);
  } else {
    console.log("ERROR: no tools response");
    process.exit(1);
  }
}
test();
'
```
Expect: 6 tools (memory_store, memory_search, memory_get, memory_update, memory_delete, memory_ingest).

### 8. Report
Print a summary:
- ✅ / ❌ for each step
- Pod name and status
- Service endpoint (from `kubectl -n rag-etl get svc memorix-mcp`)
- Number of MCP tools registered
