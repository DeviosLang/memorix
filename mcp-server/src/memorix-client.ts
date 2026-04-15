/**
 * memorix-client.ts — REST API client for memorix-server.
 * Mirrors the pattern from opencode-plugin/src/server-backend.ts.
 */

export interface MemorixConfig {
  apiUrl: string;
  tenantID: string;
  agentID: string;
}

export interface Memory {
  id: string;
  content: string;
  source?: string | null;
  tags?: string[] | null;
  metadata?: Record<string, unknown> | null;
  version?: number;
  updated_by?: string | null;
  created_at: string;
  updated_at: string;
  score?: number;
  memory_type?: string;
  state?: string;
  agent_id?: string;
  session_id?: string;
}

export interface SearchResult {
  memories: Memory[];
  total: number;
  limit: number;
  offset: number;
}

export interface CreateMemoryInput {
  content: string;
  source?: string;
  tags?: string[];
  metadata?: Record<string, unknown>;
}

export interface UpdateMemoryInput {
  content?: string;
  source?: string;
  tags?: string[];
  metadata?: Record<string, unknown>;
}

export interface SearchInput {
  q?: string;
  tags?: string;
  source?: string;
  limit?: number;
  offset?: number;
}

export interface IngestMessage {
  role: string;
  content: string;
}

export interface IngestInput {
  messages: IngestMessage[];
  session_id?: string;
  agent_id?: string;
  mode?: string;
}

export class MemorixClient {
  private baseUrl: string;
  private tenantID: string;
  private agentID: string;

  constructor(cfg: MemorixConfig) {
    this.baseUrl = cfg.apiUrl.replace(/\/+$/, "");
    this.tenantID = cfg.tenantID;
    this.agentID = cfg.agentID;
  }

  private tenantPath(path: string): string {
    return `/v1alpha1/memorix/${this.tenantID}${path}`;
  }

  private async request<T>(
    method: string,
    path: string,
    body?: unknown,
  ): Promise<T> {
    const resp = await fetch(this.baseUrl + path, {
      method,
      headers: {
        "Content-Type": "application/json",
        "X-Memorix-Agent-Id": this.agentID,
      },
      body: body != null ? JSON.stringify(body) : undefined,
      signal: AbortSignal.timeout(15_000),
    });

    if (resp.status === 204) return undefined as T;

    const data = await resp.json();
    if (!resp.ok) {
      throw new Error(
        (data as { error?: string }).error ?? `HTTP ${resp.status}`,
      );
    }
    return data as T;
  }

  async store(input: CreateMemoryInput): Promise<Memory> {
    return this.request<Memory>(
      "POST",
      this.tenantPath("/memories"),
      input,
    );
  }

  async search(input: SearchInput): Promise<SearchResult> {
    const params = new URLSearchParams();
    if (input.q) params.set("q", input.q);
    if (input.tags) params.set("tags", input.tags);
    if (input.source) params.set("source", input.source);
    if (input.limit != null) params.set("limit", String(input.limit));
    if (input.offset != null) params.set("offset", String(input.offset));

    const qs = params.toString();
    const raw = await this.request<{
      memories: Memory[];
      total: number;
      limit: number;
      offset: number;
    }>("GET", `${this.tenantPath("/memories")}${qs ? "?" + qs : ""}`);

    return {
      memories: raw.memories ?? [],
      total: raw.total,
      limit: raw.limit,
      offset: raw.offset,
    };
  }

  async get(id: string): Promise<Memory | null> {
    try {
      return await this.request<Memory>(
        "GET",
        this.tenantPath(`/memories/${id}`),
      );
    } catch (err) {
      if (
        err instanceof Error &&
        (err.message.includes("not found") || err.message.includes("404"))
      ) {
        return null;
      }
      throw err;
    }
  }

  async update(id: string, input: UpdateMemoryInput): Promise<Memory | null> {
    try {
      return await this.request<Memory>(
        "PUT",
        this.tenantPath(`/memories/${id}`),
        input,
      );
    } catch (err) {
      if (
        err instanceof Error &&
        (err.message.includes("not found") || err.message.includes("404"))
      ) {
        return null;
      }
      throw err;
    }
  }

  async remove(id: string): Promise<boolean> {
    try {
      await this.request("DELETE", this.tenantPath(`/memories/${id}`));
      return true;
    } catch (err) {
      if (
        err instanceof Error &&
        (err.message.includes("not found") || err.message.includes("404"))
      ) {
        return false;
      }
      throw err;
    }
  }

  async ingest(input: IngestInput): Promise<{ status: string }> {
    return this.request<{ status: string }>(
      "POST",
      this.tenantPath("/memories"),
      {
        messages: input.messages,
        session_id: input.session_id,
        agent_id: input.agent_id ?? this.agentID,
        mode: input.mode,
      },
    );
  }
}
