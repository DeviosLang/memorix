import type { Plugin } from "@opencode-ai/plugin";
import { loadConfig } from "./types.js";
import { ServerBackend } from "./server-backend.js";
import { buildTools } from "./tools.js";
import { buildHooks } from "./hooks.js";

/**
 * memorix-opencode — AI agent memory plugin for OpenCode.
 *
 * Requires MNEMO_API_URL + MNEMO_TENANT_ID to connect to memorix-server.
 */
const memorixPlugin: Plugin = async (_input) => {
  const cfg = loadConfig();

  if (!cfg.apiUrl) {
    console.warn(
      "[memorix] No MNEMO_API_URL configured. Plugin disabled."
    );
    return {};
  }

  if (!cfg.tenantID) {
    console.warn(
      "[memorix] Server mode requires MNEMO_TENANT_ID (or legacy MNEMO_API_TOKEN). Plugin disabled."
    );
    return {};
  }

  console.info("[memorix] Server mode (memorix-server REST API)");
  const backend = new ServerBackend(cfg);

  const tools = buildTools(backend);
  const hooks = buildHooks(backend);

  return {
    tool: tools,
    ...hooks,
  };
};

export default memorixPlugin;
