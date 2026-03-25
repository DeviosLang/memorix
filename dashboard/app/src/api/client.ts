import type {
  DashboardOverview,
  DashboardMemoryStats,
  DashboardSearchStats,
  DashboardGCStats,
  DashboardSpaceStats,
  DashboardConflictStats,
} from "@/types/metrics";

const DASHBOARD_TOKEN_KEY = "dashboard-token";

function getDashboardToken(): string | null {
  return localStorage.getItem(DASHBOARD_TOKEN_KEY);
}

export class APIError extends Error {
  constructor(
    public status: number,
    message: string
  ) {
    super(message);
    this.name = "APIError";
  }
}

async function fetchAPI<T>(endpoint: string): Promise<T> {
  const token = getDashboardToken();
  if (!token) {
    throw new APIError(401, "No dashboard token found");
  }

  const response = await fetch(`/api/dashboard${endpoint}`, {
    headers: {
      "X-Dashboard-Token": token,
    },
  });

  if (!response.ok) {
    if (response.status === 401) {
      throw new APIError(401, "Invalid or expired dashboard token");
    }
    throw new APIError(
      response.status,
      `API request failed: ${response.statusText}`
    );
  }

  return response.json();
}

export const api = {
  getOverview: () => fetchAPI<DashboardOverview>("/overview"),
  getMemoryStats: () => fetchAPI<DashboardMemoryStats>("/memory-stats"),
  getSearchStats: () => fetchAPI<DashboardSearchStats>("/search-stats"),
  getGCStats: () => fetchAPI<DashboardGCStats>("/gc-stats"),
  getSpaceStats: () => fetchAPI<DashboardSpaceStats>("/space-stats"),
  getConflictStats: () => fetchAPI<DashboardConflictStats>("/conflict-stats"),
};

export function clearDashboardToken(): void {
  localStorage.removeItem(DASHBOARD_TOKEN_KEY);
}
