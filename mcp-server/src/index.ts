#!/usr/bin/env node
/**
 * index.ts — Entry point for memorix MCP server.
 *
 * Supports two transport modes:
 *   - stdio  (default, for local MCP clients)
 *   - http   (MCP_TRANSPORT=http, for remote/k8s deployment)
 *
 * Tenant ID resolution (HTTP mode):
 *   1. X-Memorix-Tenant-Id header on the initialize request (per-session)
 *   2. MNEMO_TENANT_ID env var (global default, optional in HTTP mode)
 *
 * Environment variables:
 *   MNEMO_API_URL    — memorix server URL (required)
 *   MNEMO_TENANT_ID  — default tenant ID (required for stdio, optional for http)
 *   MNEMO_AGENT_ID   — agent identifier (optional, default: "mcp-agent")
 *   MCP_TRANSPORT    — "stdio" | "http" (default: "stdio")
 *   MCP_PORT         — HTTP listen port (default: 8080)
 */

import { randomUUID } from "node:crypto";
import { createServer as createHTTPServer } from "node:http";
import type { IncomingMessage, ServerResponse } from "node:http";
import { StdioServerTransport } from "@modelcontextprotocol/sdk/server/stdio.js";
import { StreamableHTTPServerTransport } from "@modelcontextprotocol/sdk/server/streamableHttp.js";
import { MemorixClient } from "./memorix-client.js";
import { createServer } from "./server.js";

const apiUrl = process.env.MNEMO_API_URL;
const defaultTenantID = process.env.MNEMO_TENANT_ID;
const agentID = process.env.MNEMO_AGENT_ID || "mcp-agent";
const transportMode = process.env.MCP_TRANSPORT || "stdio";
const port = parseInt(process.env.MCP_PORT || "8080", 10);

if (!apiUrl) {
  console.error("MNEMO_API_URL is required");
  process.exit(1);
}

/** Cache MemorixClient instances per tenant to avoid re-creating. */
const clientCache = new Map<string, MemorixClient>();

function getClient(tenantID: string): MemorixClient {
  let c = clientCache.get(tenantID);
  if (!c) {
    c = new MemorixClient({ apiUrl: apiUrl!, tenantID, agentID });
    clientCache.set(tenantID, c);
  }
  return c;
}

if (transportMode === "http") {
  // --- Streamable HTTP transport (stateful, per-session) ---
  // Each MCP session gets its own transport + server instance.
  // Tenant ID is resolved from header or env default at session creation.
  const sessions = new Map<string, StreamableHTTPServerTransport>();

  async function handleMCP(req: IncomingMessage, res: ServerResponse) {
    // Check for existing session
    const sessionId = req.headers["mcp-session-id"] as string | undefined;

    if (sessionId && sessions.has(sessionId)) {
      // Existing session — route to its transport
      const transport = sessions.get(sessionId)!;
      await transport.handleRequest(req, res);
      return;
    }

    if (sessionId && !sessions.has(sessionId)) {
      // Unknown session
      res.writeHead(404, { "Content-Type": "application/json" });
      res.end(JSON.stringify({ jsonrpc: "2.0", error: { code: -32000, message: "Session not found" }, id: null }));
      return;
    }

    // No session header — must be an initialize request.
    // Resolve tenant ID: header takes precedence over env default.
    const headerTenant = req.headers["x-memorix-tenant-id"] as string | undefined;
    const tenantID = headerTenant || defaultTenantID;

    if (!tenantID) {
      res.writeHead(400, { "Content-Type": "application/json" });
      res.end(JSON.stringify({
        jsonrpc: "2.0",
        error: { code: -32000, message: "Tenant ID required: set X-Memorix-Tenant-Id header or MNEMO_TENANT_ID env" },
        id: null,
      }));
      return;
    }

    const client = getClient(tenantID);

    const transport = new StreamableHTTPServerTransport({
      sessionIdGenerator: () => randomUUID(),
    });

    transport.onerror = (err) => {
      console.error("[mcp transport error]", err);
    };

    transport.onclose = () => {
      const sid = transport.sessionId;
      if (sid) sessions.delete(sid);
    };

    const mcpServer = createServer(client);
    await mcpServer.connect(transport);

    await transport.handleRequest(req, res);

    // After handling initialize, the transport should have a sessionId
    const newSid = transport.sessionId;
    if (newSid) {
      sessions.set(newSid, transport);
    }
  }

  const httpServer = createHTTPServer(async (req, res) => {
    const pathname = req.url?.split("?")[0] ?? "/";

    if (pathname === "/healthz") {
      res.writeHead(200, { "Content-Type": "text/plain" });
      res.end("ok");
      return;
    }

    if (pathname === "/mcp") {
      try {
        await handleMCP(req, res);
      } catch (err) {
        console.error("[mcp handleRequest error]", err);
        if (!res.headersSent) {
          res.writeHead(500, { "Content-Type": "application/json" });
          res.end(JSON.stringify({ error: "internal server error" }));
        }
      }
      return;
    }

    res.writeHead(404, { "Content-Type": "text/plain" });
    res.end("not found");
  });

  httpServer.listen(port, () => {
    console.log(`memorix MCP server (http) listening on :${port}/mcp`);
  });
} else {
  // --- stdio transport (default, for local MCP clients) ---
  if (!defaultTenantID) {
    console.error("MNEMO_TENANT_ID is required for stdio transport");
    process.exit(1);
  }
  const client = getClient(defaultTenantID);
  const mcpServer = createServer(client);
  const stdioTransport = new StdioServerTransport();
  await mcpServer.connect(stdioTransport);
}
