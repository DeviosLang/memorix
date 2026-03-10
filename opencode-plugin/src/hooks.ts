import type { Hooks } from "@opencode-ai/plugin";
import type { MemoryBackend } from "./backend.js";
import type { Memory } from "./types.js";

const MAX_RECENT = 10;

/**
 * Format memories into a system prompt block.
 * Matches the ccplugin SessionStart format.
 */
function formatMemoriesBlock(memories: Memory[]): string {
  if (memories.length === 0) return "";

  const lines = memories.map((m, i) => {
    const tags = m.tags?.length ? ` [${m.tags.join(", ")}]` : "";
    const date = m.updated_at ? ` (${m.updated_at})` : "";
    return `${i + 1}. ${m.content.slice(0, 500)}${tags}${date}`;
  });

  return [
    "",
    "---",
    "[memorix] Shared agent memory — recent entries:",
    ...lines,
    "",
    "Use memory_store/memory_search/memory_update/memory_delete tools to manage shared memories.",
    "---",
    "",
  ].join("\n");
}

/**
 * Build hooks for the OpenCode plugin.
 *
 * - `experimental.chat.system.transform`: Inject recent memories into system prompt.
 */
export function buildHooks(backend: MemoryBackend): Pick<
  Hooks,
  "experimental.chat.system.transform"
> {
  return {
    /**
     * Inject memories into the system prompt.
     */
    "experimental.chat.system.transform": async (_input, output) => {
      try {
        const memories = await backend.listRecent(MAX_RECENT);
        const block = formatMemoriesBlock(memories);
        if (block) {
          output.system.push(block);
        }
      } catch {
        // Graceful degradation — if memory fetch fails, continue without it.
      }
    },
  };
}
