---
description: "Create a new memorix tenant. Returns the tenant ID for use in MCP client config or API calls."
---

# /tenant-create — Provision a new memorix tenant

Create a new tenant on the memorix server and return its ID.

## Steps

### 1. Create tenant
```bash
POD=$(kubectl -n rag-etl get pod -l k8s-app=memorix -o jsonpath='{.items[0].metadata.name}')
kubectl -n rag-etl exec "$POD" -- wget -q -O- --post-data='' http://localhost:8080/v1alpha1/memorix
```

### 2. Report
Parse the JSON response and present:
- **Tenant ID**: the `id` field from the response
- **MCP config example** using the new tenant ID:
```json
{
  "mcpServers": {
    "memorix": {
      "url": "http://<memorix-mcp-svc-address>/mcp",
      "headers": {
        "X-Memorix-Tenant-Id": "<new-tenant-id>"
      }
    }
  }
}
```

Get the actual memorix-mcp service address dynamically:
```bash
kubectl -n rag-etl get svc memorix-mcp -o jsonpath='{.status.loadBalancer.ingress[0].ip}'
```
If no LB IP, fall back to `memorix-mcp.rag-etl.svc:8080`.
