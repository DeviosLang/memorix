import { Link } from "@tanstack/react-router";
import { useSessionTimeout } from "@/lib/session";
import { ThemeToggle } from "@/components/theme-toggle";
import { clearSession } from "@/api/client";
import { useStorage } from "@/api/queries";
import {
  PieChart,
  Pie,
  Cell,
  ResponsiveContainer,
  LineChart,
  Line,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  BarChart,
  Bar,
} from "recharts";

const CHART_COLORS = [
  "hsl(var(--primary))",
  "hsl(var(--chart-2))",
  "hsl(var(--chart-3))",
  "hsl(var(--chart-4))",
  "hsl(var(--chart-5))",
  "#6366f1",
  "#8b5cf6",
  "#ec4899",
  "#f43f5e",
  "#f97316",
];

export function StoragePage() {
  useSessionTimeout();
  const { data, isLoading, error } = useStorage();

  const handleLogout = () => {
    clearSession();
  };

  const formatBytes = (bytes: number) => {
    if (bytes === 0) return "0 B";
    const k = 1024;
    const sizes = ["B", "KB", "MB", "GB", "TB"];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + " " + sizes[i];
  };

  const formatMB = (bytes: number) => {
    return (bytes / (1024 * 1024)).toFixed(2);
  };

  return (
    <div className="flex min-h-screen">
      <Sidebar onLogout={handleLogout} />
      <main className="flex-1">
        <header className="flex h-16 items-center justify-between border-b border-border px-6">
          <h1 className="text-xl font-semibold">Storage Analysis</h1>
          <div className="flex items-center gap-4">
            <ThemeToggle />
          </div>
        </header>
        <div className="p-6">
          {isLoading && <div className="text-muted-foreground">Loading storage analysis...</div>}
          {error && <div className="text-red-500">Error loading storage analysis</div>}

          {data && (
            <>
              {/* Total Storage Card */}
              <div className="mb-6 grid gap-4 md:grid-cols-3">
                <div className="rounded-lg border border-border bg-card p-4">
                  <p className="text-sm font-medium text-muted-foreground">Total Storage</p>
                  <p className="mt-1 text-3xl font-bold">{formatBytes(data.total_bytes)}</p>
                </div>
                <div className="rounded-lg border border-border bg-card p-4">
                  <p className="text-sm font-medium text-muted-foreground">Total Spaces</p>
                  <p className="mt-1 text-3xl font-bold">{data.by_space.length}</p>
                </div>
                <div className="rounded-lg border border-border bg-card p-4">
                  <p className="text-sm font-medium text-muted-foreground">Avg per Space</p>
                  <p className="mt-1 text-3xl font-bold">
                    {formatBytes(
                      data.by_space.length > 0
                        ? Math.round(data.total_bytes / data.by_space.length)
                        : 0
                    )}
                  </p>
                </div>
              </div>

              <div className="grid gap-6 lg:grid-cols-2">
                {/* Storage Distribution Chart */}
                <div className="rounded-lg border border-border p-4">
                  <h2 className="mb-4 text-lg font-semibold">Storage Distribution</h2>
                  <div className="h-80">
                    <ResponsiveContainer width="100%" height="100%">
                      <PieChart>
                        <Pie
                          data={data.by_space.slice(0, 10)}
                          dataKey="storage_bytes"
                          nameKey="tenant_name"
                          cx="50%"
                          cy="50%"
                          outerRadius={100}
                          label={({ tenant_name, percent }) =>
                            `${tenant_name} (${percent.toFixed(1)}%)`
                          }
                          labelLine={false}
                        >
                          {data.by_space.slice(0, 10).map((_, index) => (
                            <Cell
                              key={`cell-${index}`}
                              fill={CHART_COLORS[index % CHART_COLORS.length]}
                            />
                          ))}
                        </Pie>
                        <Tooltip
                          formatter={(value: number) => formatBytes(value)}
                          contentStyle={{
                            backgroundColor: "hsl(var(--card))",
                            border: "1px solid hsl(var(--border))",
                            borderRadius: "6px",
                          }}
                        />
                      </PieChart>
                    </ResponsiveContainer>
                  </div>
                </div>

                {/* Storage by Space Bar Chart */}
                <div className="rounded-lg border border-border p-4">
                  <h2 className="mb-4 text-lg font-semibold">Storage by Space</h2>
                  <div className="h-80">
                    <ResponsiveContainer width="100%" height="100%">
                      <BarChart data={data.by_space.slice(0, 10)} layout="vertical">
                        <CartesianGrid strokeDasharray="3 3" className="stroke-border" />
                        <XAxis type="number" tickFormatter={formatMB} className="text-xs" />
                        <YAxis
                          type="category"
                          dataKey="tenant_name"
                          width={100}
                          className="text-xs"
                        />
                        <Tooltip
                          formatter={(value: number) => formatBytes(value)}
                          contentStyle={{
                            backgroundColor: "hsl(var(--card))",
                            border: "1px solid hsl(var(--border))",
                            borderRadius: "6px",
                          }}
                        />
                        <Bar dataKey="storage_bytes" fill="hsl(var(--primary))" />
                      </BarChart>
                    </ResponsiveContainer>
                  </div>
                </div>
              </div>

              {/* Storage Trend Chart */}
              <div className="mt-6 rounded-lg border border-border p-4">
                <h2 className="mb-4 text-lg font-semibold">Storage Growth Trend (30 Days)</h2>
                <div className="h-64">
                  <ResponsiveContainer width="100%" height="100%">
                    <LineChart data={data.trend}>
                      <CartesianGrid strokeDasharray="3 3" className="stroke-border" />
                      <XAxis
                        dataKey="date"
                        tickFormatter={(value) => value.slice(5)}
                        className="text-xs"
                      />
                      <YAxis
                        yAxisId="left"
                        tickFormatter={formatMB}
                        className="text-xs"
                        label={{
                          value: "Storage (MB)",
                          angle: -90,
                          position: "insideLeft",
                          style: { fontSize: "12px" },
                        }}
                      />
                      <YAxis
                        yAxisId="right"
                        orientation="right"
                        className="text-xs"
                        label={{
                          value: "Memories",
                          angle: 90,
                          position: "insideRight",
                          style: { fontSize: "12px" },
                        }}
                      />
                      <Tooltip
                        formatter={(value: number, name: string) =>
                          name === "storage_bytes" ? formatBytes(value) : value
                        }
                        contentStyle={{
                          backgroundColor: "hsl(var(--card))",
                          border: "1px solid hsl(var(--border))",
                          borderRadius: "6px",
                        }}
                      />
                      <Line
                        yAxisId="left"
                        type="monotone"
                        dataKey="storage_bytes"
                        stroke="hsl(var(--primary))"
                        strokeWidth={2}
                        dot={false}
                        name="Storage"
                      />
                      <Line
                        yAxisId="right"
                        type="monotone"
                        dataKey="memory_count"
                        stroke="hsl(var(--chart-2))"
                        strokeWidth={2}
                        dot={false}
                        name="Memories"
                      />
                    </LineChart>
                  </ResponsiveContainer>
                </div>
              </div>

              {/* Space Storage Table */}
              <div className="mt-6 rounded-lg border border-border">
                <div className="border-b border-border bg-muted/50 px-4 py-3">
                  <h2 className="font-semibold">All Spaces</h2>
                </div>
                <table className="w-full">
                  <thead>
                    <tr className="border-b border-border bg-muted/30">
                      <th className="px-4 py-2 text-left text-sm font-medium">Space</th>
                      <th className="px-4 py-2 text-right text-sm font-medium">Storage</th>
                      <th className="px-4 py-2 text-right text-sm font-medium">Memories</th>
                      <th className="px-4 py-2 text-right text-sm font-medium">% of Total</th>
                    </tr>
                  </thead>
                  <tbody>
                    {data.by_space.map((space, index) => (
                      <tr key={space.tenant_id} className="border-b border-border">
                        <td className="px-4 py-2">
                          <div className="flex items-center gap-2">
                            <div
                              className="h-3 w-3 rounded-full"
                              style={{ backgroundColor: CHART_COLORS[index % CHART_COLORS.length] }}
                            />
                            <span>{space.tenant_name}</span>
                          </div>
                        </td>
                        <td className="px-4 py-2 text-right font-mono text-sm">
                          {formatBytes(space.storage_bytes)}
                        </td>
                        <td className="px-4 py-2 text-right">{space.memory_count.toLocaleString()}</td>
                        <td className="px-4 py-2 text-right">
                          <div className="flex items-center justify-end gap-2">
                            <div className="h-2 w-20 overflow-hidden rounded-full bg-muted">
                              <div
                                className="h-full bg-primary"
                                style={{ width: `${space.percent}%` }}
                              />
                            </div>
                            <span className="w-12 text-right text-sm">
                              {space.percent.toFixed(1)}%
                            </span>
                          </div>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </>
          )}
        </div>
      </main>
    </div>
  );
}

function Sidebar({ onLogout }: { onLogout: () => void }) {
  return (
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
        <NavItem href="/dashboard" label="Overview" />
        <NavItem href="/dashboard/spaces" label="Spaces" />
        <NavItem href="/dashboard/agents" label="Agents" />
        <NavItem href="/dashboard/storage" label="Storage" active />
      </nav>
      <div className="border-t border-border p-2">
        <Link
          to="/"
          onClick={onLogout}
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
