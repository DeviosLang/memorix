import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Sidebar } from "@/components/sidebar";
import { ThemeToggle } from "@/components/theme-toggle";
import { LocaleToggle } from "@/components/locale-toggle";
import { useSessionTimeout } from "@/lib/session";
import { useAgents } from "@/api/queries";
import { formatRelativeTime } from "@/lib/i18n";
import type { AgentActivity, ActivityDataPoint } from "@/types/metrics";
import {
  BarChart,
  Bar,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
} from "recharts";

export function AgentsPage() {
  const { t } = useTranslation();
  useSessionTimeout();
  const { data, isLoading, error } = useAgents();
  const [groupBy, setGroupBy] = useState<"type" | "space">("type");

  const timeAgo = (dateStr: string) => {
    return formatRelativeTime(t, dateStr);
  };

  // Group agents
  const groupedAgents = data?.agents.reduce((acc, agent) => {
    let key: string;
    if (groupBy === "type") {
      key = agent.agent_type;
    } else {
      key = agent.tenant_name || agent.tenant_id;
    }
    if (!acc[key]) {
      acc[key] = [];
    }
    acc[key].push(agent);
    return acc;
  }, {} as Record<string, AgentActivity[]>) || {};

  // Aggregate timeline by group
  const aggregateTimeline = (agents: AgentActivity[]): ActivityDataPoint[] => {
    const timelineMap: Record<string, ActivityDataPoint> = {};

    agents.forEach((agent) => {
      agent.timeline.forEach((point) => {
        if (!timelineMap[point.date]) {
          timelineMap[point.date] = {
            date: point.date,
            writes: 0,
            reads: 0,
            total_ops: 0,
          };
        }
        timelineMap[point.date].writes += point.writes;
        timelineMap[point.date].reads += point.reads;
        timelineMap[point.date].total_ops += point.total_ops;
      });
    });

    return Object.values(timelineMap).sort((a, b) => a.date.localeCompare(b.date));
  };

  return (
    <div className="flex min-h-screen">
      <Sidebar />
      <main className="flex-1 pt-14 lg:pt-0">
        <header className="flex h-16 items-center justify-between border-b border-border px-4 lg:px-6">
          <h1 className="text-xl font-semibold">{t("agents.title")}</h1>
          <div className="hidden items-center gap-4 lg:flex">
            <LocaleToggle />
            <ThemeToggle />
          </div>
        </header>
        <div className="p-4 lg:p-6">
          {isLoading && (
            <div className="text-muted-foreground">{t("agents.loading")}</div>
          )}
          {error && (
            <div className="text-red-500">{t("agents.error")}</div>
          )}

          {data && (
            <>
              {/* Stats Cards */}
              <div className="mb-6 grid gap-4 grid-cols-2 lg:grid-cols-4">
                <StatCard title={t("agents.totalAgents")} value={data.total_agents} />
                <StatCard title="Claude Code" value={data.by_type["claude-code"] || 0} />
                <StatCard title="OpenClaw" value={data.by_type["openclaw"] || 0} />
                <StatCard title="OpenCode" value={data.by_type["opencode"] || 0} />
              </div>

              {/* Group By Toggle */}
              <div className="mb-4 flex flex-wrap items-center gap-2 lg:gap-4">
                <span className="text-sm text-muted-foreground">{t("agents.groupBy")}</span>
                <button
                  onClick={() => setGroupBy("type")}
                  className={`rounded-md px-3 py-1 text-sm ${
                    groupBy === "type"
                      ? "bg-primary text-primary-foreground"
                      : "bg-muted hover:bg-muted/80"
                  }`}
                >
                  {t("agents.agentType")}
                </button>
                <button
                  onClick={() => setGroupBy("space")}
                  className={`rounded-md px-3 py-1 text-sm ${
                    groupBy === "space"
                      ? "bg-primary text-primary-foreground"
                      : "bg-muted hover:bg-muted/80"
                  }`}
                >
                  {t("agents.space")}
                </button>
              </div>

              {/* Grouped Activity Views */}
              <div className="space-y-6">
                {Object.entries(groupedAgents).map(([group, agents]) => (
                  <div key={group} className="rounded-lg border border-border p-4">
                    <div className="mb-4 flex items-center justify-between">
                      <div className="flex items-center gap-2">
                        {groupBy === "type" && <AgentTypeBadge type={group} />}
                        <h3 className="font-semibold">{group}</h3>
                        <span className="text-sm text-muted-foreground">
                          {t("agents.agentsCount", { count: agents.length })}
                        </span>
                      </div>
                    </div>

                    {/* Activity Timeline Chart */}
                    <div className="mb-4 h-48">
                      <ResponsiveContainer width="100%" height="100%">
                        <BarChart data={aggregateTimeline(agents)}>
                          <CartesianGrid strokeDasharray="3 3" className="stroke-border" />
                          <XAxis
                            dataKey="date"
                            tickFormatter={(value) => value.slice(5)}
                            className="text-xs"
                          />
                          <YAxis className="text-xs" />
                          <Tooltip
                            contentStyle={{
                              backgroundColor: "hsl(var(--card))",
                              border: "1px solid hsl(var(--border))",
                              borderRadius: "6px",
                            }}
                          />
                          <Bar dataKey="writes" name={t("agents.chart.writes")} fill="hsl(var(--primary))" />
                          <Bar dataKey="reads" name={t("agents.chart.reads")} fill="hsl(var(--chart-2))" />
                        </BarChart>
                      </ResponsiveContainer>
                    </div>

                    {/* Agent List */}
                    <div className="overflow-x-auto rounded border border-border">
                      <table className="w-full min-w-[500px] text-sm">
                        <thead>
                          <tr className="border-b border-border bg-muted/50">
                            <th className="px-3 py-2 text-left">{t("agents.table.agentId")}</th>
                            <th className="px-3 py-2 text-left">{t("agents.table.space")}</th>
                            <th className="px-3 py-2 text-left">{t("agents.table.lastActive")}</th>
                            <th className="px-3 py-2 text-right">{t("agents.table.totalOps")}</th>
                          </tr>
                        </thead>
                        <tbody>
                          {agents.map((agent) => (
                            <tr key={agent.agent_id} className="border-b border-border">
                              <td className="px-3 py-2 font-mono text-xs">{agent.agent_id}</td>
                              <td className="px-3 py-2">{agent.tenant_name || agent.tenant_id}</td>
                              <td className="px-3 py-2">{timeAgo(agent.last_active_at)}</td>
                              <td className="px-3 py-2 text-right">
                                {agent.timeline.reduce((sum, p) => sum + p.total_ops, 0)}
                              </td>
                            </tr>
                          ))}
                        </tbody>
                      </table>
                    </div>
                  </div>
                ))}
              </div>
            </>
          )}
        </div>
      </main>
    </div>
  );
}

function StatCard({ title, value }: { title: string; value: number | string }) {
  return (
    <div className="rounded-lg border border-border bg-card p-4">
      <p className="text-sm font-medium text-muted-foreground">{title}</p>
      <p className="mt-1 text-2xl font-bold">{value}</p>
    </div>
  );
}

function AgentTypeBadge({ type }: { type: string }) {
  const colors: Record<string, string> = {
    "claude-code": "bg-purple-100 text-purple-700 dark:bg-purple-900 dark:text-purple-300",
    openclaw: "bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-300",
    opencode: "bg-orange-100 text-orange-700 dark:bg-orange-900 dark:text-orange-300",
    unknown: "bg-gray-100 text-gray-700 dark:bg-gray-800 dark:text-gray-300",
  };

  return (
    <span
      className={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium ${
        colors[type] || colors.unknown
      }`}
    >
      {type}
    </span>
  );
}
