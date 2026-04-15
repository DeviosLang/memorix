#!/usr/bin/env node
/**
 * index.ts — Entry point for memorix MCP server.
 *
 * Supports two transport modes:
 *   - stdio  (default, for local MCP clients)
 *   - http   (MCP_TRANSPORT=http, for remote/k8s deployment)
 *
 * Environment variables:
 *   MNEMO_API_URL    — memorix server URL (required)
 *   MNEMO_TENANT_ID  — tenant ID (required)
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
const tenantID = process.env.MNEMO_TENANT_ID;
const agentID = process.env.MNEMO_AGENT_ID || "mcp-agent";
const transportMode = process.env.MCP_TRANSPORT || "stdio";
const port = parseInt(process.env.MCP_PORT || "8080", 10);

if (!apiUrl) {
  console.error("MNEMO_API_URL is required");
  process.exit(1);
}
if (!tenantID) {
  console.error("MNEMO_TENANT_ID is required");
  process.exit(1);
}

const client = new MemorixClient({ apiUrl, tenantID, agentID });

if (transportMode === "http") {
  // --- Streamable HTTP transport (stateful, per-session) ---
  // Each MCP session gets its own transport + server instance.
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

    // No session header — must be an initialize request. Create new session.
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
  const mcpServer = createServer(client);
  const stdioTransport = new StdioServerTransport();
  await mcpServer.connect(stdioTransport);
}
