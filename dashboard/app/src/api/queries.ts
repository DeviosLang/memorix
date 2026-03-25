import { queryOptions, useQuery } from "@tanstack/react-query";
import { api } from "./client";

export const dashboardKeys = {
  all: ["dashboard"] as const,
  overview: () => [...dashboardKeys.all, "overview"] as const,
  memoryStats: () => [...dashboardKeys.all, "memory-stats"] as const,
  searchStats: () => [...dashboardKeys.all, "search-stats"] as const,
  gcStats: () => [...dashboardKeys.all, "gc-stats"] as const,
  spaceStats: () => [...dashboardKeys.all, "space-stats"] as const,
  conflictStats: () => [...dashboardKeys.all, "conflict-stats"] as const,
};

export const overviewOptions = queryOptions({
  queryKey: dashboardKeys.overview(),
  queryFn: () => api.getOverview(),
  refetchInterval: 30000, // Refresh every 30 seconds
});

export const memoryStatsOptions = queryOptions({
  queryKey: dashboardKeys.memoryStats(),
  queryFn: () => api.getMemoryStats(),
  refetchInterval: 60000, // Refresh every minute
});

export const searchStatsOptions = queryOptions({
  queryKey: dashboardKeys.searchStats(),
  queryFn: () => api.getSearchStats(),
  refetchInterval: 60000,
});

export const gcStatsOptions = queryOptions({
  queryKey: dashboardKeys.gcStats(),
  queryFn: () => api.getGCStats(),
  refetchInterval: 60000,
});

export const spaceStatsOptions = queryOptions({
  queryKey: dashboardKeys.spaceStats(),
  queryFn: () => api.getSpaceStats(),
  refetchInterval: 60000,
});

export const conflictStatsOptions = queryOptions({
  queryKey: dashboardKeys.conflictStats(),
  queryFn: () => api.getConflictStats(),
  refetchInterval: 60000,
});

// Hook exports
export function useOverview() {
  return useQuery(overviewOptions);
}

export function useMemoryStats() {
  return useQuery(memoryStatsOptions);
}

export function useSearchStats() {
  return useQuery(searchStatsOptions);
}

export function useGCStats() {
  return useQuery(gcStatsOptions);
}

export function useSpaceStats() {
  return useQuery(spaceStatsOptions);
}

export function useConflictStats() {
  return useQuery(conflictStatsOptions);
}
