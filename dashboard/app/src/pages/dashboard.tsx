import { Link } from "@tanstack/react-router";
import { useSessionTimeout } from "@/lib/session";
import { ThemeToggle } from "@/components/theme-toggle";
import { clearSession } from "@/api/client";
import { useOverview, useSpaceStats, useSearchStats } from "@/api/queries";

export function DashboardPage() {
  // Set up session timeout (auto-logout after 30 min inactivity)
  useSessionTimeout();

  const { data: overview, isLoading: overviewLoading } = useOverview();
  const { data: spaceStats } = useSpaceStats();
  const { data: searchStats } = useSearchStats();

  const handleLogout = () => {
    clearSession();
  };

  const formatErrorRate = (rate: number) => {
    return (rate * 100).toFixed(2) + "%";
  };

  return (
    <div className="flex min-h-screen">
      {/* Sidebar */}
      <aside className="flex w-64 flex-col border-r border-border bg-card">
        <div className="flex h-16 items-center gap-2 border-b border-border px-4">
          <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-primary">
            <svg
              xmlns="http://www.w3.org/2000/svg"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth="2"
              strokeLinecap="round"
              strokeLinejoin="round"
              className="h-4 w-4 text-primary-foreground"
            >
              <path d="M12 2L2 7l10 5 10-5-10-5z" />
              <path d="M2 17l10 5 10-5" />
              <path d="M2 12l10 5 10-5" />
            </svg>
          </div>
          <span className="text-lg font-semibold">Memorix</span>
        </div>
        <nav className="flex-1 space-y-1 p-2">
          <NavItem href="/dashboard" label="Overview" active />
          <NavItem href="/dashboard/spaces" label="Spaces" />
          <NavItem href="/dashboard/agents" label="Agents" />
          <NavItem href="/dashboard/storage" label="Storage" />
        </nav>
        <div className="border-t border-border p-2">
          <Link
            to="/"
            onClick={handleLogout}
            className="flex items-center gap-2 rounded-md px-3 py-2 text-sm text-muted-foreground hover:bg-accent hover:text-accent-foreground"
          >
            <svg
              xmlns="http://www.w3.org/2000/svg"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth="2"
              strokeLinecap="round"
              strokeLinejoin="round"
              className="h-4 w-4"
            >
              <path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4" />
              <polyline points="16,17 21,12 16,7" />
              <line x1="21" y1="12" x2="9" y2="12" />
            </svg>
            Logout
          </Link>
        </div>
      </aside>

      {/* Main content */}
      <main className="flex-1">
        <header className="flex h-16 items-center justify-between border-b border-border px-6">
          <h1 className="text-xl font-semibold">Overview</h1>
          <div className="flex items-center gap-4">
            <div className="flex items-center gap-2">
              <span className="text-sm text-muted-foreground">System Status:</span>
              <StatusBadge status={overview?.status || "healthy"} />
            </div>
            <ThemeToggle />
          </div>
        </header>
        <div className="p-6">
          {overviewLoading ? (
            <div className="text-muted-foreground">Loading...</div>
          ) : (
            <>
              {/* Stats Cards */}
              <div className="grid gap-6 md:grid-cols-2 lg:grid-cols-4">
                <StatCard
                  title="Active Tenants"
                  value={overview?.active_tenants ?? spaceStats?.active_tenants ?? 0}
                />
                <StatCard
                  title="Active Agents"
                  value={overview?.active_agents ?? 0}
                />
                <StatCard
                  title="Requests/sec"
                  value={(overview?.request_stats?.requests_per_sec ?? 0).toFixed(2)}
                />
                <StatCard
                  title="Error Rate"
                  value={formatErrorRate(overview?.request_stats?.error_rate ?? 0)}
                />
              </div>

              {/* Request Stats */}
              <div className="mt-6 grid gap-6 lg:grid-cols-2">
                <div className="rounded-lg border border-border p-6">
                  <h2 className="text-lg font-semibold">Request Statistics</h2>
                  <div className="mt-4 space-y-3">
                    <StatRow
                      label="Total Requests"
                      value={(overview?.request_stats?.total_requests ?? 0).toLocaleString()}
                    />
                    <StatRow
                      label="Avg Latency"
                      value={(overview?.request_stats?.avg_latency_ms ?? 0).toFixed(2) + " ms"}
                    />
                    <StatRow
                      label="P50 Latency"
                      value={(overview?.request_stats?.p50_latency_ms ?? 0).toFixed(2) + " ms"}
                    />
                    <StatRow
                      label="P95 Latency"
                      value={(overview?.request_stats?.p95_latency_ms ?? 0).toFixed(2) + " ms"}
                    />
                    <StatRow
                      label="P99 Latency"
                      value={(overview?.request_stats?.p99_latency_ms ?? 0).toFixed(2) + " ms"}
                    />
                  </div>
                </div>

                <div className="rounded-lg border border-border p-6">
                  <h2 className="text-lg font-semibold">Search Statistics</h2>
                  <div className="mt-4 space-y-3">
                    <StatRow
                      label="Vector Searches"
                      value={(searchStats?.vector_searches ?? 0).toLocaleString()}
                    />
                    <StatRow
                      label="Keyword Searches"
                      value={(searchStats?.keyword_searches ?? 0).toLocaleString()}
                    />
                    <StatRow
                      label="Hybrid Searches"
                      value={(searchStats?.hybrid_searches ?? 0).toLocaleString()}
                    />
                    <StatRow
                      label="Avg Search Latency"
                      value={(searchStats?.avg_search_latency_ms ?? 0).toFixed(2) + " ms"}
                    />
                    <StatRow
                      label="Success Rate"
                      value={((searchStats?.success_rate ?? 0) * 100).toFixed(1) + "%"}
                    />
                  </div>
                </div>
              </div>

              {/* Top Active Tenants */}
              {spaceStats?.top_active_tenants && spaceStats.top_active_tenants.length > 0 && (
                <div className="mt-6 rounded-lg border border-border p-6">
                  <h2 className="text-lg font-semibold">Top Active Spaces</h2>
                  <div className="mt-4">
                    <table className="w-full">
                      <thead>
                        <tr className="border-b border-border text-left">
                          <th className="pb-2 text-sm font-medium text-muted-foreground">Name</th>
                          <th className="pb-2 text-sm font-medium text-muted-foreground">Requests</th>
                          <th className="pb-2 text-sm font-medium text-muted-foreground">Agents</th>
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
              <div className="mt-6 rounded-lg border border-border p-6">
                <h2 className="text-lg font-semibold">System Info</h2>
                <div className="mt-4 space-y-2">
                  <p className="text-sm text-muted-foreground">
                    <span className="font-medium">Uptime:</span>{" "}
                    {overview?.uptime || "--"}
                  </p>
                  <p className="text-sm text-muted-foreground">
                    <span className="font-medium">Started:</span>{" "}
                    {overview?.start_time
                      ? new Date(overview.start_time).toLocaleString()
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

function StatusBadge({ status }: { status: string }) {
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
      {status.charAt(0).toUpperCase() + status.slice(1)}
    </span>
  );
}

function NavItem({
  href,
  label,
  active = false,
}: {
  href: string;
  label: string;
  active?: boolean;
}) {
  return (
    <Link
      to={href}
      className={`flex items-center gap-2 rounded-md px-3 py-2 text-sm ${
        active
          ? "bg-accent text-accent-foreground"
          : "text-muted-foreground hover:bg-accent hover:text-accent-foreground"
      }`}
    >
      {label}
    </Link>
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
