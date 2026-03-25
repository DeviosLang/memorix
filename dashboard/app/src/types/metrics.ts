// TypeScript types matching the backend dashboard domain types

export interface DashboardOverview {
  status: "healthy" | "degraded" | "unhealthy";
  start_time: string;
  uptime: string;
  request_stats: RequestStats;
  active_tenants: number;
  active_agents: number;
  collected_at: string;
}

export interface RequestStats {
  total_requests: number;
  requests_by_path: Record<string, number>;
  requests_by_code: Record<number, number>;
  avg_latency_ms: number;
  p50_latency_ms: number;
  p95_latency_ms: number;
  p99_latency_ms: number;
  requests_per_sec: number;
  error_rate: number;
}

export interface DashboardMemoryStats {
  total_memories: number;
  by_state: Record<string, number>;
  by_type: Record<string, number>;
  total_content_bytes: number;
  avg_content_bytes: number;
  top_tenants?: TenantMemoryCount[];
  collected_at: string;
}

export interface TenantMemoryCount {
  tenant_id: string;
  tenant_name?: string;
  memory_count: number;
}

export interface DashboardSearchStats {
  vector_searches: number;
  keyword_searches: number;
  hybrid_searches: number;
  fts_searches: number;
  vector_search_percent: number;
  keyword_search_percent: number;
  hybrid_search_percent: number;
  fts_search_percent: number;
  avg_search_latency_ms: number;
  p50_search_latency_ms: number;
  p95_search_latency_ms: number;
  p99_search_latency_ms: number;
  success_rate: number;
  collected_at: string;
}

export interface DashboardGCStats {
  last_run_time?: string;
  last_run_id?: string;
  last_run_deleted: number;
  last_run_duration?: string;
  total_runs: number;
  total_deleted: number;
  total_recovered: number;
  next_scheduled_time?: string;
  recent_runs?: GCSummary[];
  collected_at: string;
}

export interface GCSummary {
  gc_run_id: string;
  run_time: string;
  deleted_count: number;
  duration: string;
  reason: string;
}

export interface DashboardSpaceStats {
  total_tenants: number;
  active_tenants: number;
  suspended_tenants: number;
  total_agents: number;
  active_agents: number;
  agents_by_tenant?: Record<string, number>;
  top_active_tenants?: TenantActivity[];
  collected_at: string;
}

export interface TenantActivity {
  tenant_id: string;
  tenant_name?: string;
  agent_count: number;
  memory_count: number;
  request_count: number;
  last_activity_at?: string;
}

export interface DashboardConflictStats {
  total_conflicts: number;
  lww_resolutions: number;
  llm_merge_resolutions: number;
  lww_percent: number;
  llm_merge_percent: number;
  merge_success_rate: number;
  recent_conflicts?: ConflictSummary[];
  collected_at: string;
}

export interface ConflictSummary {
  fact_id: string;
  user_id: string;
  resolution: "lww" | "llm_merge";
  resolved_at: string;
}
