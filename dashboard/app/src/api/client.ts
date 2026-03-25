import type {
  DashboardOverview,
  DashboardMemoryStats,
  DashboardSearchStats,
  DashboardGCStats,
  DashboardSpaceStats,
  DashboardConflictStats,
  SpaceListResponse,
  SpaceListItem,
  AgentActivityResponse,
  StorageAnalysisResponse,
} from "@/types/metrics";

const DASHBOARD_TOKEN_KEY = "dashboard-token";
const SERVER_URL_KEY = "server-url";

export function getServerUrl(): string {
  return sessionStorage.getItem(SERVER_URL_KEY) || "http://localhost:8080";
}

export function setServerUrl(url: string): void {
  sessionStorage.setItem(SERVER_URL_KEY, url);
}

export function getDashboardToken(): string | null {
  return sessionStorage.getItem(DASHBOARD_TOKEN_KEY);
}

export function setDashboardToken(token: string): void {
  sessionStorage.setItem(DASHBOARD_TOKEN_KEY, token);
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

  const serverUrl = getServerUrl();
  const response = await fetch(`${serverUrl}/api/dashboard${endpoint}`, {
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
  // New endpoints for space and agent management
  getSpaces: () => fetchAPI<SpaceListResponse>("/spaces"),
  getSpaceDetail: (tenantId: string) => fetchAPI<SpaceListItem>(`/spaces/${tenantId}`),
  getAgentActivity: () => fetchAPI<AgentActivityResponse>("/agents"),
  getStorageAnalysis: () => fetchAPI<StorageAnalysisResponse>("/storage"),
};

export function clearDashboardToken(): void {
  sessionStorage.removeItem(DASHBOARD_TOKEN_KEY);
  sessionStorage.removeItem(SERVER_URL_KEY);
}

export function clearSession(): void {
  clearDashboardToken();
}

/**
 * Verify dashboard credentials by calling the overview endpoint
 */
export async function verifyCredentials(
  serverUrl: string,
  token: string
): Promise<DashboardOverview> {
  const response = await fetch(`${serverUrl}/api/dashboard/overview`, {
    headers: {
      "X-Dashboard-Token": token,
    },
  });

  if (!response.ok) {
    if (response.status === 401) {
      throw new APIError(401, "Invalid dashboard token");
    }
    throw new APIError(
      response.status,
      `Server error: ${response.statusText}`
    );
  }

  return response.json();
}
