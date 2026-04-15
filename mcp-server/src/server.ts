/**
 * server.ts — MCP server definition with 6 memorix tools.
 */

import { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { z } from "zod";
import { MemorixClient } from "./memorix-client.js";
import type {
  CreateMemoryInput,
  UpdateMemoryInput,
  SearchInput,
  IngestInput,
} from "./memorix-client.js";

export function createServer(client: MemorixClient): McpServer {
  const server = new McpServer({
    name: "memorix",
    version: "0.1.0",
  });

  // ---------- memory_store ----------
  server.tool(
    "memory_store",
    "Store a memory. Returns the stored memory with its assigned id.",
    {
      content: z
        .string()
        .max(50000)
        .describe("Memory content (required, max 50000 chars)"),
      source: z
        .string()
        .optional()
        .describe("Which agent wrote this memory"),
      tags: z
        .array(z.string())
        .max(20)
        .optional()
        .describe("Filterable tags (max 20)"),
      metadata: z
        .record(z.string(), z.unknown())
        .optional()
        .describe("Arbitrary structured data"),
    },
    async (args) => {
      try {
        const input: CreateMemoryInput = {
          content: args.content,
          source: args.source,
          tags: args.tags,
          metadata: args.metadata as Record<string, unknown> | undefined,
        };
        const result = await client.store(input);
        return {
          content: [
            { type: "text" as const, text: JSON.stringify({ ok: true, data: result }, null, 2) },
          ],
        };
      } catch (err) {
        return {
          content: [
            {
              type: "text" as const,
              text: JSON.stringify({
                ok: false,
                error: err instanceof Error ? err.message : String(err),
              }),
            },
          ],
          isError: true,
        };
      }
    },
  );

  // ---------- memory_search ----------
  server.tool(
    "memory_search",
    "Search memories using hybrid vector + keyword search. Higher score = more relevant.",
    {
      q: z.string().optional().describe("Search query"),
      tags: z
        .string()
        .optional()
        .describe("Comma-separated tags to filter by (AND)"),
      source: z.string().optional().describe("Filter by source agent"),
      limit: z
        .number()
        .int()
        .min(1)
        .max(200)
        .optional()
        .describe("Max results (default 20, max 200)"),
      offset: z
        .number()
        .int()
        .min(0)
        .optional()
        .describe("Pagination offset"),
    },
    async (args) => {
      try {
        const input: SearchInput = {
          q: args.q,
          tags: args.tags,
          source: args.source,
          limit: args.limit,
          offset: args.offset,
        };
        const result = await client.search(input);
        return {
          content: [
            { type: "text" as const, text: JSON.stringify({ ok: true, ...result }, null, 2) },
          ],
        };
      } catch (err) {
        return {
          content: [
            {
              type: "text" as const,
              text: JSON.stringify({
                ok: false,
                error: err instanceof Error ? err.message : String(err),
              }),
            },
          ],
          isError: true,
        };
      }
    },
  );

  // ---------- memory_get ----------
  server.tool(
    "memory_get",
    "Retrieve a single memory by its id.",
    {
      id: z.string().describe("Memory id (UUID)"),
    },
    async (args) => {
      try {
        const result = await client.get(args.id);
        if (!result) {
          return {
            content: [
              { type: "text" as const, text: JSON.stringify({ ok: false, error: "memory not found" }) },
            ],
            isError: true,
          };
        }
        return {
          content: [
            { type: "text" as const, text: JSON.stringify({ ok: true, data: result }, null, 2) },
          ],
        };
      } catch (err) {
        return {
          content: [
            {
              type: "text" as const,
              text: JSON.stringify({
                ok: false,
                error: err instanceof Error ? err.message : String(err),
              }),
            },
          ],
          isError: true,
        };
      }
    },
  );

  // ---------- memory_update ----------
  server.tool(
    "memory_update",
    "Update an existing memory. Only provided fields are changed.",
    {
      id: z.string().describe("Memory id to update"),
      content: z.string().optional().describe("New content"),
      source: z.string().optional().describe("New source"),
      tags: z
        .array(z.string())
        .optional()
        .describe("Replacement tags"),
      metadata: z
        .record(z.string(), z.unknown())
        .optional()
        .describe("Replacement metadata"),
    },
    async (args) => {
      try {
        const { id, ...rest } = args;
        const input: UpdateMemoryInput = {
          content: rest.content,
          source: rest.source,
          tags: rest.tags,
          metadata: rest.metadata as Record<string, unknown> | undefined,
        };
        const result = await client.update(id, input);
        if (!result) {
          return {
            content: [
              { type: "text" as const, text: JSON.stringify({ ok: false, error: "memory not found" }) },
            ],
            isError: true,
          };
        }
        return {
          content: [
            { type: "text" as const, text: JSON.stringify({ ok: true, data: result }, null, 2) },
          ],
        };
      } catch (err) {
        return {
          content: [
            {
              type: "text" as const,
              text: JSON.stringify({
                ok: false,
                error: err instanceof Error ? err.message : String(err),
              }),
            },
          ],
          isError: true,
        };
      }
    },
  );

  // ---------- memory_delete ----------
  server.tool(
    "memory_delete",
    "Delete a memory by id.",
    {
      id: z.string().describe("Memory id to delete"),
    },
    async (args) => {
      try {
        const deleted = await client.remove(args.id);
        if (!deleted) {
          return {
            content: [
              { type: "text" as const, text: JSON.stringify({ ok: false, error: "memory not found" }) },
            ],
            isError: true,
          };
        }
        return {
          content: [
            { type: "text" as const, text: JSON.stringify({ ok: true }) },
          ],
        };
      } catch (err) {
        return {
          content: [
            {
              type: "text" as const,
              text: JSON.stringify({
                ok: false,
                error: err instanceof Error ? err.message : String(err),
              }),
            },
          ],
          isError: true,
        };
      }
    },
  );

  // ---------- memory_ingest ----------
  server.tool(
    "memory_ingest",
    "Ingest conversation messages into memorix. The server extracts and stores memories asynchronously. Returns immediately with status 'accepted'.",
    {
      messages: z
        .array(
          z.object({
            role: z.string().describe("Message role (e.g. 'user', 'assistant')"),
            content: z.string().describe("Message content"),
          }),
        )
        .min(1)
        .describe("Conversation messages to ingest"),
      session_id: z
        .string()
        .optional()
        .describe("Session identifier for grouping messages"),
      agent_id: z
        .string()
        .optional()
        .describe("Agent identifier (overrides default)"),
      mode: z
        .string()
        .optional()
        .describe("Ingest mode (server-defined)"),
    },
    async (args) => {
      try {
        const input: IngestInput = {
          messages: args.messages,
          session_id: args.session_id,
          agent_id: args.agent_id,
          mode: args.mode,
        };
        const result = await client.ingest(input);
        return {
          content: [
            { type: "text" as const, text: JSON.stringify({ ok: true, ...result }, null, 2) },
          ],
        };
      } catch (err) {
        return {
          content: [
            {
              type: "text" as const,
              text: JSON.stringify({
                ok: false,
                error: err instanceof Error ? err.message : String(err),
              }),
            },
          ],
          isError: true,
        };
      }
    },
  );

  return server;
}
