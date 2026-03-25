import { useTranslation } from "react-i18next";
import { Sidebar } from "@/components/sidebar";
import { ThemeToggle } from "@/components/theme-toggle";
import { LocaleToggle } from "@/components/locale-toggle";
import { useSessionTimeout } from "@/lib/session";
import { useOverview, useSpaceStats, useSearchStats } from "@/api/queries";

export function DashboardPage() {
  const { t, i18n } = useTranslation();
  // Set up session timeout (auto-logout after 30 min inactivity)
  useSessionTimeout();

  const { data: overview, isLoading: overviewLoading } = useOverview();
  const { data: spaceStats } = useSpaceStats();
  const { data: searchStats } = useSearchStats();

  const formatErrorRate = (rate: number) => {
    return (rate * 100).toFixed(2) + "%";
  };

  const getStatusLabel = (status: string): string => {
    switch (status) {
      case "healthy":
        return t("status.healthy");
      case "degraded":
        return t("status.degraded");
      case "unhealthy":
        return t("status.unhealthy");
      default:
        return status;
    }
  };

  return (
    <div className="flex min-h-screen">
      <Sidebar />

      {/* Main content */}
      <main className="flex-1 pt-14 lg:pt-0">
        <header className="flex h-16 items-center justify-between border-b border-border px-4 lg:px-6">
          <h1 className="text-xl font-semibold">{t("overview.title")}</h1>
          <div className="hidden items-center gap-4 lg:flex">
            <div className="flex items-center gap-2">
              <span className="text-sm text-muted-foreground">{t("status.systemStatus")}:</span>
              <StatusBadge status={overview?.status || "healthy"} getStatusLabel={getStatusLabel} />
            </div>
            <LocaleToggle />
            <ThemeToggle />
          </div>
        </header>
        <div className="p-4 lg:p-6">
          {overviewLoading ? (
            <div className="text-muted-foreground">{t("common.loading")}</div>
          ) : (
            <>
              {/* Stats Cards */}
              <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
                <StatCard
                  title={t("overview.activeTenants")}
                  value={overview?.active_tenants ?? spaceStats?.active_tenants ?? 0}
                />
                <StatCard
                  title={t("overview.activeAgents")}
                  value={overview?.active_agents ?? 0}
                />
                <StatCard
                  title={t("overview.requestsPerSec")}
                  value={(overview?.request_stats?.requests_per_sec ?? 0).toFixed(2)}
                />
                <StatCard
                  title={t("overview.errorRate")}
                  value={formatErrorRate(overview?.request_stats?.error_rate ?? 0)}
                />
              </div>

              {/* Request Stats */}
              <div className="mt-6 grid gap-4 lg:grid-cols-2">
                <div className="rounded-lg border border-border p-4 lg:p-6">
                  <h2 className="text-lg font-semibold">{t("overview.requestStats")}</h2>
                  <div className="mt-4 space-y-3">
                    <StatRow
                      label={t("overview.totalRequests")}
                      value={(overview?.request_stats?.total_requests ?? 0).toLocaleString()}
                    />
                    <StatRow
                      label={t("overview.avgLatency")}
                      value={(overview?.request_stats?.avg_latency_ms ?? 0).toFixed(2) + " ms"}
                    />
                    <StatRow
                      label={t("overview.p50Latency")}
                      value={(overview?.request_stats?.p50_latency_ms ?? 0).toFixed(2) + " ms"}
                    />
                    <StatRow
                      label={t("overview.p95Latency")}
                      value={(overview?.request_stats?.p95_latency_ms ?? 0).toFixed(2) + " ms"}
                    />
                    <StatRow
                      label={t("overview.p99Latency")}
                      value={(overview?.request_stats?.p99_latency_ms ?? 0).toFixed(2) + " ms"}
                    />
                  </div>
                </div>

                <div className="rounded-lg border border-border p-4 lg:p-6">
                  <h2 className="text-lg font-semibold">{t("overview.searchStats")}</h2>
                  <div className="mt-4 space-y-3">
                    <StatRow
                      label={t("overview.vectorSearches")}
                      value={(searchStats?.vector_searches ?? 0).toLocaleString()}
                    />
                    <StatRow
                      label={t("overview.keywordSearches")}
                      value={(searchStats?.keyword_searches ?? 0).toLocaleString()}
                    />
                    <StatRow
                      label={t("overview.hybridSearches")}
                      value={(searchStats?.hybrid_searches ?? 0).toLocaleString()}
                    />
                    <StatRow
                      label={t("overview.avgSearchLatency")}
                      value={(searchStats?.avg_search_latency_ms ?? 0).toFixed(2) + " ms"}
                    />
                    <StatRow
                      label={t("overview.successRate")}
                      value={((searchStats?.success_rate ?? 0) * 100).toFixed(1) + "%"}
                    />
                  </div>
                </div>
              </div>

              {/* Top Active Tenants */}
              {spaceStats?.top_active_tenants && spaceStats.top_active_tenants.length > 0 && (
                <div className="mt-6 rounded-lg border border-border p-4 lg:p-6">
                  <h2 className="text-lg font-semibold">{t("overview.topActiveSpaces")}</h2>
                  <div className="mt-4 overflow-x-auto">
                    <table className="w-full min-w-[400px]">
                      <thead>
                        <tr className="border-b border-border text-left">
                          <th className="pb-2 text-sm font-medium text-muted-foreground">
                            {t("spaces.table.name")}
                          </th>
                          <th className="pb-2 text-sm font-medium text-muted-foreground">
                            {t("spaces.table.memories")}
                          </th>
                          <th className="pb-2 text-sm font-medium text-muted-foreground">
                            {t("spaces.table.agents")}
                          </th>
                        </tr>
                      </thead>
                      <tbody>
                        {spaceStats.top_active_tenants.slice(0, 5).map((tenant) => (
                          <tr key={tenant.tenant_id} className="border-b border-border">
                            <td className="py-2">{tenant.tenant_name || tenant.tenant_id}</td>
                            <td className="py-2">{tenant.request_count.toLocaleString()}</td>
                            <td className="py-2">{tenant.agent_count}</td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                </div>
              )}

              {/* Uptime */}
              <div className="mt-6 rounded-lg border border-border p-4 lg:p-6">
                <h2 className="text-lg font-semibold">{t("overview.systemInfo")}</h2>
                <div className="mt-4 space-y-2">
                  <p className="text-sm text-muted-foreground">
                    <span className="font-medium">{t("overview.uptime")}:</span>{" "}
                    {overview?.uptime || "--"}
                  </p>
                  <p className="text-sm text-muted-foreground">
                    <span className="font-medium">{t("overview.started")}:</span>{" "}
                    {overview?.start_time
                      ? new Date(overview.start_time).toLocaleString(
                          i18n.language === "zh-CN" ? "zh-CN" : "en-US"
                        )
                      : "--"}
                  </p>
                </div>
              </div>
            </>
          )}
        </div>
      </main>
    </div>
  );
}

function StatusBadge({
  status,
  getStatusLabel,
}: {
  status: string;
  getStatusLabel: (status: string) => string;
}) {
  const colors: Record<string, string> = {
    healthy: "bg-green-100 text-green-700 dark:bg-green-900 dark:text-green-300",
    degraded: "bg-yellow-100 text-yellow-700 dark:bg-yellow-900 dark:text-yellow-300",
    unhealthy: "bg-red-100 text-red-700 dark:bg-red-900 dark:text-red-300",
  };

  return (
    <span
      className={`inline-flex items-center rounded-full px-2 py-1 text-xs font-medium ${
        colors[status] || colors.healthy
      }`}
    >
      {getStatusLabel(status)}
    </span>
  );
}

function StatCard({ title, value }: { title: string; value: string | number }) {
  return (
    <div className="rounded-lg border border-border bg-card p-4">
      <p className="text-sm font-medium text-muted-foreground">{title}</p>
      <p className="mt-1 text-2xl font-bold">{value}</p>
    </div>
  );
}

function StatRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-center justify-between">
      <span className="text-sm text-muted-foreground">{label}</span>
      <span className="font-mono text-sm">{value}</span>
    </div>
  );
}
