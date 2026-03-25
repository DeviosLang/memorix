import { useTranslation } from "react-i18next";
import { Sidebar } from "@/components/sidebar";
import { ThemeToggle } from "@/components/theme-toggle";
import { LocaleToggle } from "@/components/locale-toggle";
import { useSessionTimeout } from "@/lib/session";
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
  const { t } = useTranslation();
  useSessionTimeout();
  const { data, isLoading, error } = useStorage();

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
      <Sidebar />
      <main className="flex-1 pt-14 lg:pt-0">
        <header className="flex h-16 items-center justify-between border-b border-border px-4 lg:px-6">
          <h1 className="text-xl font-semibold">{t("storage.title")}</h1>
          <div className="hidden items-center gap-4 lg:flex">
            <LocaleToggle />
            <ThemeToggle />
          </div>
        </header>
        <div className="p-4 lg:p-6">
          {isLoading && (
            <div className="text-muted-foreground">{t("storage.loading")}</div>
          )}
          {error && (
            <div className="text-red-500">{t("storage.error")}</div>
          )}

          {data && (
            <>
              {/* Total Storage Card */}
              <div className="mb-6 grid gap-4 grid-cols-1 sm:grid-cols-3">
                <div className="rounded-lg border border-border bg-card p-4">
                  <p className="text-sm font-medium text-muted-foreground">{t("storage.totalStorage")}</p>
                  <p className="mt-1 text-2xl font-bold sm:text-3xl">{formatBytes(data.total_bytes)}</p>
                </div>
                <div className="rounded-lg border border-border bg-card p-4">
                  <p className="text-sm font-medium text-muted-foreground">{t("storage.totalSpaces")}</p>
                  <p className="mt-1 text-2xl font-bold sm:text-3xl">{data.by_space.length}</p>
                </div>
                <div className="rounded-lg border border-border bg-card p-4">
                  <p className="text-sm font-medium text-muted-foreground">{t("storage.avgPerSpace")}</p>
                  <p className="mt-1 text-2xl font-bold sm:text-3xl">
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
                  <h2 className="mb-4 text-lg font-semibold">{t("storage.distribution")}</h2>
                  <div className="h-64 sm:h-80">
                    <ResponsiveContainer width="100%" height="100%">
                      <PieChart>
                        <Pie
                          data={data.by_space.slice(0, 10)}
                          dataKey="storage_bytes"
                          nameKey="tenant_name"
                          cx="50%"
                          cy="50%"
                          outerRadius={80}
                          label={({ tenant_name, percent }) =>
                            `${tenant_name} (${(percent * 100).toFixed(1)}%)`
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
                  <h2 className="mb-4 text-lg font-semibold">{t("storage.bySpace")}</h2>
                  <div className="h-64 sm:h-80">
                    <ResponsiveContainer width="100%" height="100%">
                      <BarChart data={data.by_space.slice(0, 10)} layout="vertical">
                        <CartesianGrid strokeDasharray="3 3" className="stroke-border" />
                        <XAxis type="number" tickFormatter={formatMB} className="text-xs" />
                        <YAxis
                          type="category"
                          dataKey="tenant_name"
                          width={80}
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
                <h2 className="mb-4 text-lg font-semibold">{t("storage.growthTrend")}</h2>
                <div className="h-48 sm:h-64">
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
                          value: t("storage.chart.storageMB"),
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
                          value: t("storage.chart.memories"),
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
                        name={t("storage.chart.storage")}
                      />
                      <Line
                        yAxisId="right"
                        type="monotone"
                        dataKey="memory_count"
                        stroke="hsl(var(--chart-2))"
                        strokeWidth={2}
                        dot={false}
                        name={t("storage.chart.memories")}
                      />
                    </LineChart>
                  </ResponsiveContainer>
                </div>
              </div>

              {/* Space Storage Table */}
              <div className="mt-6 rounded-lg border border-border">
                <div className="border-b border-border bg-muted/50 px-4 py-3">
                  <h2 className="font-semibold">{t("storage.allSpaces")}</h2>
                </div>
                <div className="overflow-x-auto">
                  <table className="w-full min-w-[500px]">
                    <thead>
                      <tr className="border-b border-border bg-muted/30">
                        <th className="px-4 py-2 text-left text-sm font-medium">{t("storage.table.space")}</th>
                        <th className="px-4 py-2 text-right text-sm font-medium">{t("storage.table.storage")}</th>
                        <th className="px-4 py-2 text-right text-sm font-medium">{t("storage.table.memories")}</th>
                        <th className="px-4 py-2 text-right text-sm font-medium">{t("storage.table.percentOfTotal")}</th>
                      </tr>
                    </thead>
                    <tbody>
                      {data.by_space.map((space, index) => (
                        <tr key={space.tenant_id} className="border-b border-border">
                          <td className="px-4 py-2">
                            <div className="flex items-center gap-2">
                              <div
                                className="h-3 w-3 flex-shrink-0 rounded-full"
                                style={{ backgroundColor: CHART_COLORS[index % CHART_COLORS.length] }}
                              />
                              <span className="truncate">{space.tenant_name}</span>
                            </div>
                          </td>
                          <td className="px-4 py-2 text-right font-mono text-sm">
                            {formatBytes(space.storage_bytes)}
                          </td>
                          <td className="px-4 py-2 text-right">{space.memory_count.toLocaleString()}</td>
                          <td className="px-4 py-2 text-right">
                            <div className="flex items-center justify-end gap-2">
                              <div className="hidden h-2 w-20 overflow-hidden rounded-full bg-muted sm:block">
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
              </div>
            </>
          )}
        </div>
      </main>
    </div>
  );
}
