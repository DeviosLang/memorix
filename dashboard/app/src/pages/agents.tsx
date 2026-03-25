import { useState } from "react";
import { Link } from "@tanstack/react-router";
import { useSessionTimeout } from "@/lib/session";
import { ThemeToggle } from "@/components/theme-toggle";
import { clearSession } from "@/api/client";
import { useAgents } from "@/api/queries";
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
  useSessionTimeout();
  const { data, isLoading, error } = useAgents();
  const [groupBy, setGroupBy] = useState<"type" | "space">("type");

  const handleLogout = () => {
    clearSession();
  };

  const timeAgo = (dateStr: string) => {
    const date = new Date(dateStr);
    const now = new Date();
    const seconds = Math.floor((now.getTime() - date.getTime()) / 1000);
    if (seconds < 60) return "Just now";
    if (seconds < 3600) return Math.floor(seconds / 60) + "m ago";
    if (seconds < 86400) return Math.floor(seconds / 3600) + "h ago";
    return Math.floor(seconds / 86400) + "d ago";
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
      <Sidebar onLogout={handleLogout} />
      <main className="flex-1">
        <header className="flex h-16 items-center justify-between border-b border-border px-6">
          <h1 className="text-xl font-semibold">Agent Activity</h1>
          <div className="flex items-center gap-4">
            <ThemeToggle />
          </div>
        </header>
        <div className="p-6">
          {isLoading && <div className="text-muted-foreground">Loading agent activity...</div>}
          {error && <div className="text-red-500">Error loading agent activity</div>}

          {data && (
            <>
              {/* Stats Cards */}
              <div className="mb-6 grid gap-4 md:grid-cols-4">
                <StatCard title="Total Agents" value={data.total_agents} />
                <StatCard title="Claude Code" value={data.by_type["claude-code"] || 0} />
                <StatCard title="OpenClaw" value={data.by_type["openclaw"] || 0} />
                <StatCard title="OpenCode" value={data.by_type["opencode"] || 0} />
              </div>

              {/* Group By Toggle */}
              <div className="mb-4 flex items-center gap-4">
                <span className="text-sm text-muted-foreground">Group by:</span>
                <button
                  onClick={() => setGroupBy("type")}
                  className={`rounded-md px-3 py-1 text-sm ${
                    groupBy === "type"
                      ? "bg-primary text-primary-foreground"
                      : "bg-muted hover:bg-muted/80"
                  }`}
                >
                  Agent Type
                </button>
                <button
                  onClick={() => setGroupBy("space")}
                  className={`rounded-md px-3 py-1 text-sm ${
                    groupBy === "space"
                      ? "bg-primary text-primary-foreground"
                      : "bg-muted hover:bg-muted/80"
                  }`}
                >
                  Space
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
                          ({agents.length} agents)
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
                          <Bar dataKey="writes" name="Writes" fill="hsl(var(--primary))" />
                          <Bar dataKey="reads" name="Reads" fill="hsl(var(--chart-2))" />
                        </BarChart>
                      </ResponsiveContainer>
                    </div>

                    {/* Agent List */}
                    <div className="rounded border border-border">
                      <table className="w-full text-sm">
                        <thead>
                          <tr className="border-b border-border bg-muted/50">
                            <th className="px-3 py-2 text-left">Agent ID</th>
                            <th className="px-3 py-2 text-left">Space</th>
                            <th className="px-3 py-2 text-left">Last Active</th>
                            <th className="px-3 py-2 text-right">Total Ops (7d)</th>
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
        <NavItem href="/dashboard/agents" label="Agents" active />
        <NavItem href="/dashboard/storage" label="Storage" />
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
