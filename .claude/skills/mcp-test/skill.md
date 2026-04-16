---
description: "End-to-end MCP protocol test against the live memorix-mcp server. Tests initialize, tools/list, memory_store, and memory_search."
---

# /mcp-test — MCP end-to-end test

Run a full MCP protocol test against the live memorix-mcp pod.

## Steps

### 1. Find the MCP pod
```bash
POD=$(kubectl -n rag-etl get pod -l app=memorix-mcp -o jsonpath='{.items[0].metadata.name}')
```

### 2. Run end-to-end test
Execute inside the pod:
```bash
kubectl -n rag-etl exec "$POD" -- node -e '
async function test() {
  const base = "http://localhost:8080/mcp";
  const hdrs = { "Content-Type": "application/json", "Accept": "application/json, text/event-stream" };
  const results = [];

  // Step 1: Initialize
  const r1 = await fetch(base, { method: "POST", headers: hdrs,
    body: JSON.stringify({jsonrpc:"2.0",method:"initialize",params:{protocolVersion:"2025-03-26",capabilities:{},clientInfo:{name:"mcp-test",version:"1.0"}},id:1})
  });
  const sid = r1.headers.get("mcp-session-id");
  results.push({ step: "initialize", ok: r1.status === 200, session: sid });

  const sessHdrs = { ...hdrs, "mcp-session-id": sid };
  await fetch(base, { method: "POST", headers: sessHdrs,
    body: JSON.stringify({jsonrpc:"2.0",method:"notifications/initialized"})
  });

  // Step 2: tools/list
  const r2 = await fetch(base, { method: "POST", headers: sessHdrs,
    body: JSON.stringify({jsonrpc:"2.0",method:"tools/list",id:2})
  });
  const b2 = await r2.text();
  const m2 = b2.match(/data: (.+)/);
  const tools = m2 ? JSON.parse(m2[1]).result.tools.map(t => t.name) : [];
  results.push({ step: "tools/list", ok: tools.length === 6, tools });

  // Step 3: memory_store
  const r3 = await fetch(base, { method: "POST", headers: sessHdrs,
    body: JSON.stringify({jsonrpc:"2.0",method:"tools/call",params:{name:"memory_store",arguments:{content:"mcp-test: " + new Date().toISOString(),tags:["mcp-test"]}},id:3})
  });
  const b3 = await r3.text();
  const m3 = b3.match(/data: (.+)/);
  let storeResult = {};
  if (m3) { storeResult = JSON.parse(JSON.parse(m3[1]).result.content[0].text); }
  results.push({ step: "memory_store", ok: storeResult.ok === true });

  // Step 4: memory_search
  const r4 = await fetch(base, { method: "POST", headers: sessHdrs,
    body: JSON.stringify({jsonrpc:"2.0",method:"tools/call",params:{name:"memory_search",arguments:{tags:"mcp-test",limit:3}},id:4})
  });
  const b4 = await r4.text();
  const m4 = b4.match(/data: (.+)/);
  let searchResult = {};
  if (m4) { searchResult = JSON.parse(JSON.parse(m4[1]).result.content[0].text); }
  results.push({ step: "memory_search", ok: searchResult.ok === true, total: searchResult.total });

  // Report
  for (const r of results) {
    console.log((r.ok ? "✅" : "❌") + " " + r.step + " " + JSON.stringify(r));
  }
  process.exit(results.every(r => r.ok) ? 0 : 1);
}
test();
'
```

### 3. Report
Show the test results. If all 4 steps pass → ✅ MCP server fully operational. If any fail → ❌ with details.
