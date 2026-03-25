# Memorix Dashboard API Data Contract

**Version**: 0.1.0  
**Last Updated**: 2026-03-26

## Authentication

All dashboard API endpoints require authentication via the `X-Dashboard-Token` header.

```
X-Dashboard-Token: <your-dashboard-token>
```

The token is configured on the server via the `MNEMO_DASHBOARD_TOKEN` environment variable.

## Base URL

All endpoints are prefixed with `/api/dashboard`.

## Response Format

All responses are JSON. Error responses follow this structure:

```json
{
  "error": "Error message"
}
```

## Endpoints

### GET /api/dashboard/overview

Returns a system-wide overview with key metrics.

**Response (200 OK)**:

```typescript
interface DashboardOverview {
  // System status
  status: "healthy" | "degraded" | "unhealthy";
  start_time: string;        // ISO 8601 timestamp
  uptime: string;            // Human-readable duration (e.g., "2h 30m 15s")

  // Request statistics
  request_stats: {
    total_requests: number;
    requests_by_path: Record<string, number>;
    requests_by_code: Record<number, number>;
    avg_latency_ms: number;
    p50_latency_ms: number;
    p95_latency_ms: number;
    p99_latency_ms: number;
    requests_per_sec: number;
    error_rate: number;      // 0.0 to 1.0
  };

  // Active resources
  active_tenants: number;
  active_agents: number;

  // Timestamp
  collected_at: string;      // ISO 8601 timestamp
}
```

**Example Response**:

```json
{
  "status": "healthy",
  "start_time": "2026-03-26T10:00:00Z",
  "uptime": "2h 30m 15s",
  "request_stats": {
    "total_requests": 15000,
    "requests_by_path": {
      "/api/memory": 8000,
      "/api/search": 5000,
      "/api/health": 2000
    },
    "requests_by_code": {
      "200": 14500,
      "400": 300,
      "500": 200
    },
    "avg_latency_ms": 45.2,
    "p50_latency_ms": 32.0,
    "p95_latency_ms": 120.0,
    "p99_latency_ms": 250.0,
    "requests_per_sec": 125.5,
    "error_rate": 0.013
  },
  "active_tenants": 42,
  "active_agents": 156,
  "collected_at": "2026-03-26T12:30:15Z"
}
```

---

### GET /api/dashboard/memory-stats

Returns memory storage statistics.

**Response (200 OK)**:

```typescript
interface DashboardMemoryStats {
  // Total counts
  total_memories: number;

  // Distribution by state
  by_state: Record<string, number>;  // e.g., {"active": 10000, "archived": 500}

  // Distribution by type
  by_type: Record<string, number>;   // e.g., {"fact": 8000, "summary": 2000, "experience": 500}

  // Storage metrics
  total_content_bytes: number;
  avg_content_bytes: number;

  // Per-tenant breakdown (top tenants)
  top_tenants?: Array<{
    tenant_id: string;
    tenant_name?: string;
    memory_count: number;
  }>;

  // Timestamp
  collected_at: string;
}
```

**Example Response**:

```json
{
  "total_memories": 10500,
  "by_state": {
    "active": 10000,
    "archived": 500
  },
  "by_type": {
    "fact": 8000,
    "summary": 2000,
    "experience": 500
  },
  "total_content_bytes": 52428800,
  "avg_content_bytes": 4993.22,
  "top_tenants": [
    {
      "tenant_id": "tenant-001",
      "tenant_name": "Acme Corp",
      "memory_count": 3500
    },
    {
      "tenant_id": "tenant-002",
      "tenant_name": "Globex Inc",
      "memory_count": 2800
    }
  ],
  "collected_at": "2026-03-26T12:30:15Z"
}
```

---

### GET /api/dashboard/search-stats

Returns search performance metrics.

**Response (200 OK)**:

```typescript
interface DashboardSearchStats {
  // Search counts by type
  vector_searches: number;
  keyword_searches: number;
  hybrid_searches: number;
  fts_searches: number;

  // Search distribution (percentage of each type)
  vector_search_percent: number;
  keyword_search_percent: number;
  hybrid_search_percent: number;
  fts_search_percent: number;

  // Latency metrics
  avg_search_latency_ms: number;
  p50_search_latency_ms: number;
  p95_search_latency_ms: number;
  p99_search_latency_ms: number;

  // Success rate
  success_rate: number;      // 0.0 to 1.0

  // Timestamp
  collected_at: string;
}
```

**Example Response**:

```json
{
  "vector_searches": 5000,
  "keyword_searches": 2000,
  "hybrid_searches": 2500,
  "fts_searches": 500,
  "vector_search_percent": 50.0,
  "keyword_search_percent": 20.0,
  "hybrid_search_percent": 25.0,
  "fts_search_percent": 5.0,
  "avg_search_latency_ms": 85.5,
  "p50_search_latency_ms": 65.0,
  "p95_search_latency_ms": 200.0,
  "p99_search_latency_ms": 350.0,
  "success_rate": 0.98,
  "collected_at": "2026-03-26T12:30:15Z"
}
```

---

### GET /api/dashboard/gc-stats

Returns garbage collection statistics.

**Response (200 OK)**:

```typescript
interface DashboardGCStats {
  // Last run info
  last_run_time?: string;      // ISO 8601 timestamp
  last_run_id?: string;
  last_run_deleted: number;
  last_run_duration?: string;  // e.g., "2.5s"

  // Historical totals
  total_runs: number;
  total_deleted: number;
  total_recovered: number;

  // Next scheduled run
  next_scheduled_time?: string;

  // Recent GC logs (last 10 runs)
  recent_runs?: Array<{
    gc_run_id: string;
    run_time: string;
    deleted_count: number;
    duration: string;
    reason: string;
  }>;

  // Timestamp
  collected_at: string;
}
```

**Example Response**:

```json
{
  "last_run_time": "2026-03-26T12:00:00Z",
  "last_run_id": "gc-20260326-120000",
  "last_run_deleted": 150,
  "last_run_duration": "2.5s",
  "total_runs": 48,
  "total_deleted": 7200,
  "total_recovered": 7200,
  "next_scheduled_time": "2026-03-26T13:00:00Z",
  "recent_runs": [
    {
      "gc_run_id": "gc-20260326-120000",
      "run_time": "2026-03-26T12:00:00Z",
      "deleted_count": 150,
      "duration": "2.5s",
      "reason": "scheduled"
    }
  ],
  "collected_at": "2026-03-26T12:30:15Z"
}
```

---

### GET /api/dashboard/space-stats

Returns tenant and agent statistics.

**Response (200 OK)**:

```typescript
interface DashboardSpaceStats {
  // Tenant stats
  total_tenants: number;
  active_tenants: number;
  suspended_tenants: number;

  // Agent stats
  total_agents: number;
  active_agents: number;        // Agents active in last 24h
  agents_by_tenant?: Record<string, number>;

  // Top tenants by activity
  top_active_tenants?: Array<{
    tenant_id: string;
    tenant_name?: string;
    agent_count: number;
    memory_count: number;
    request_count: number;
    last_activity_at?: string;
  }>;

  // Timestamp
  collected_at: string;
}
```

**Example Response**:

```json
{
  "total_tenants": 50,
  "active_tenants": 42,
  "suspended_tenants": 8,
  "total_agents": 200,
  "active_agents": 156,
  "agents_by_tenant": {
    "tenant-001": 25,
    "tenant-002": 18,
    "tenant-003": 12
  },
  "top_active_tenants": [
    {
      "tenant_id": "tenant-001",
      "tenant_name": "Acme Corp",
      "agent_count": 25,
      "memory_count": 3500,
      "request_count": 15000,
      "last_activity_at": "2026-03-26T12:29:00Z"
    }
  ],
  "collected_at": "2026-03-26T12:30:15Z"
}
```

---

### GET /api/dashboard/conflict-stats

Returns conflict resolution statistics.

**Response (200 OK)**:

```typescript
interface DashboardConflictStats {
  // Resolution counts
  total_conflicts: number;
  lww_resolutions: number;        // Last-Write-Wins
  llm_merge_resolutions: number;  // LLM-based merge

  // Resolution distribution
  lww_percent: number;
  llm_merge_percent: number;

  // Success metrics
  merge_success_rate: number;     // 0.0 to 1.0

  // Recent conflicts (sample)
  recent_conflicts?: Array<{
    fact_id: string;
    user_id: string;
    resolution: "lww" | "llm_merge";
    resolved_at: string;
  }>;

  // Timestamp
  collected_at: string;
}
```

**Example Response**:

```json
{
  "total_conflicts": 500,
  "lww_resolutions": 400,
  "llm_merge_resolutions": 100,
  "lww_percent": 80.0,
  "llm_merge_percent": 20.0,
  "merge_success_rate": 0.95,
  "recent_conflicts": [
    {
      "fact_id": "fact-abc123",
      "user_id": "user-xyz",
      "resolution": "llm_merge",
      "resolved_at": "2026-03-26T12:25:00Z"
    }
  ],
  "collected_at": "2026-03-26T12:30:15Z"
}
```

---

## Error Responses

### 401 Unauthorized

Returned when the dashboard token is missing or invalid.

```json
{
  "error": "unauthorized"
}
```

### 500 Internal Server Error

Returned when an unexpected error occurs.

```json
{
  "error": "internal server error"
}
```

## TypeScript Types

TypeScript type definitions matching this contract are available in the dashboard frontend at:

```
dashboard/app/src/types/metrics.ts
```

## Changes from mem9

The Memorix dashboard differs from mem9's dashboard in the following ways:

1. **Purpose**: System operations vs. end-user memory management
2. **Metrics Focus**: System-level metrics (tenants, agents, GC, conflicts) vs. user memories
3. **UI Library**: Recharts for data visualization vs. @xyflow/react for graph visualization
4. **Authentication**: Dashboard token vs. user OAuth
