import { useState } from "react";
import { Link } from "@tanstack/react-router";
import { useSessionTimeout } from "@/lib/session";
import { ThemeToggle } from "@/components/theme-toggle";
import { clearSession } from "@/api/client";
import { useSpaces, useSpaceDetail } from "@/api/queries";
import type { SpaceListItem } from "@/types/metrics";

export function SpacesPage() {
  useSessionTimeout();
  const { data, isLoading, error } = useSpaces();
  const [sortBy, setSortBy] = useState<"memory" | "activity">("memory");
  const [expandedSpace, setExpandedSpace] = useState<string | null>(null);

  const handleLogout = () => {
    clearSession();
  };

  const sortedSpaces = data?.spaces ? [...data.spaces] : [];
  if (sortBy === "memory") {
    sortedSpaces.sort((a, b) => b.memory_count - a.memory_count);
  } else {
    sortedSpaces.sort((a, b) => {
      const aTime = a.last_active_at ? new Date(a.last_active_at).getTime() : 0;
      const bTime = b.last_active_at ? new Date(b.last_active_at).getTime() : 0;
      return bTime - aTime;
    });
  }

  const formatBytes = (bytes: number) => {
    if (bytes === 0) return "0 B";
    const k = 1024;
    const sizes = ["B", "KB", "MB", "GB"];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + " " + sizes[i];
  };

  const formatDate = (dateStr?: string) => {
    if (!dateStr) return "--";
    const date = new Date(dateStr);
    return date.toLocaleDateString() + " " + date.toLocaleTimeString();
  };

  const timeAgo = (dateStr?: string) => {
    if (!dateStr) return "Never";
    const date = new Date(dateStr);
    const now = new Date();
    const seconds = Math.floor((now.getTime() - date.getTime()) / 1000);
    if (seconds < 60) return "Just now";
    if (seconds < 3600) return Math.floor(seconds / 60) + "m ago";
    if (seconds < 86400) return Math.floor(seconds / 3600) + "h ago";
    return Math.floor(seconds / 86400) + "d ago";
  };

  return (
    <div className="flex min-h-screen">
      <Sidebar onLogout={handleLogout} />
      <main className="flex-1">
        <header className="flex h-16 items-center justify-between border-b border-border px-6">
          <h1 className="text-xl font-semibold">Spaces</h1>
          <div className="flex items-center gap-4">
            <ThemeToggle />
          </div>
        </header>
        <div className="p-6">
          {isLoading && <div className="text-muted-foreground">Loading spaces...</div>}
          {error && <div className="text-red-500">Error loading spaces</div>}
          
          {data && (
            <>
              <div className="mb-4 flex items-center gap-4">
                <span className="text-sm text-muted-foreground">Sort by:</span>
                <button
                  onClick={() => setSortBy("memory")}
                  className={`rounded-md px-3 py-1 text-sm ${
                    sortBy === "memory"
                      ? "bg-primary text-primary-foreground"
                      : "bg-muted hover:bg-muted/80"
                  }`}
                >
                  Memory Count
                </button>
                <button
                  onClick={() => setSortBy("activity")}
                  className={`rounded-md px-3 py-1 text-sm ${
                    sortBy === "activity"
                      ? "bg-primary text-primary-foreground"
                      : "bg-muted hover:bg-muted/80"
                  }`}
                >
                  Last Activity
                </button>
              </div>

              <div className="rounded-lg border border-border">
                <table className="w-full">
                  <thead>
                    <tr className="border-b border-border bg-muted/50">
                      <th className="px-4 py-3 text-left text-sm font-medium">Name</th>
                      <th className="px-4 py-3 text-left text-sm font-medium">Memories</th>
                      <th className="px-4 py-3 text-left text-sm font-medium">Agents</th>
                      <th className="px-4 py-3 text-left text-sm font-medium">Last Active</th>
                      <th className="px-4 py-3 text-left text-sm font-medium">Storage</th>
                      <th className="px-4 py-3 text-left text-sm font-medium">Status</th>
                    </tr>
                  </thead>
                  <tbody>
                    {sortedSpaces.map((space) => (
                      <SpaceRow
                        key={space.tenant_id}
                        space={space}
                        isExpanded={expandedSpace === space.tenant_id}
                        onToggle={() =>
                          setExpandedSpace(
                            expandedSpace === space.tenant_id ? null : space.tenant_id
                          )
                        }
                        formatBytes={formatBytes}
                        timeAgo={timeAgo}
                        formatDate={formatDate}
                      />
                    ))}
                  </tbody>
                </table>
              </div>

              <div className="mt-4 text-sm text-muted-foreground">
                Total: {data.total_count} spaces
              </div>
            </>
          )}
        </div>
      </main>
    </div>
  );
}

function SpaceRow({
  space,
  isExpanded,
  onToggle,
  formatBytes,
  timeAgo,
  formatDate,
}: {
  space: SpaceListItem;
  isExpanded: boolean;
  onToggle: () => void;
  formatBytes: (bytes: number) => string;
  timeAgo: (date?: string) => string;
  formatDate: (date?: string) => string;
}) {
  return (
    <>
      <tr
        className="cursor-pointer border-b border-border hover:bg-muted/30"
        onClick={onToggle}
      >
        <td className="px-4 py-3">
          <div className="flex items-center gap-2">
            <svg
              xmlns="http://www.w3.org/2000/svg"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth="2"
              strokeLinecap="round"
              strokeLinejoin="round"
              className={`h-4 w-4 transition-transform ${isExpanded ? "rotate-90" : ""}`}
            >
              <polyline points="9,18 15,12 9,6" />
            </svg>
            <span className="font-medium">{space.tenant_name}</span>
          </div>
        </td>
        <td className="px-4 py-3">{space.memory_count.toLocaleString()}</td>
        <td className="px-4 py-3">{space.agent_count}</td>
        <td className="px-4 py-3">{timeAgo(space.last_active_at)}</td>
        <td className="px-4 py-3">{formatBytes(space.storage_bytes)}</td>
        <td className="px-4 py-3">
          <span
            className={`inline-flex items-center rounded-full px-2 py-1 text-xs font-medium ${
              space.status === "active"
                ? "bg-green-100 text-green-700 dark:bg-green-900 dark:text-green-300"
                : "bg-yellow-100 text-yellow-700 dark:bg-yellow-900 dark:text-yellow-300"
            }`}
          >
            {space.status}
          </span>
        </td>
      </tr>
      {isExpanded && (
        <tr>
          <td colSpan={6} className="bg-muted/20 px-4 py-4">
            <SpaceDetail tenantId={space.tenant_id} formatDate={formatDate} />
          </td>
        </tr>
      )}
    </>
  );
}

function SpaceDetail({
  tenantId,
  formatDate,
}: {
  tenantId: string;
  formatDate: (date?: string) => string;
}) {
  const { data, isLoading } = useSpaceDetail(tenantId);

  if (isLoading) {
    return <div className="text-sm text-muted-foreground">Loading details...</div>;
  }

  if (!data) {
    return null;
  }

  return (
    <div className="grid gap-6 md:grid-cols-2">
      <div>
        <h3 className="mb-2 font-semibold">Agents ({data.agents?.length || 0})</h3>
        {data.agents && data.agents.length > 0 ? (
          <div className="rounded border border-border">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-border bg-muted/50">
                  <th className="px-3 py-2 text-left">ID</th>
                  <th className="px-3 py-2 text-left">Type</th>
                  <th className="px-3 py-2 text-left">Last Active</th>
                </tr>
              </thead>
              <tbody>
                {data.agents.map((agent) => (
                  <tr key={agent.agent_id} className="border-b border-border">
                    <td className="px-3 py-2">{agent.agent_id}</td>
                    <td className="px-3 py-2">
                      <AgentTypeBadge type={agent.agent_type} />
                    </td>
                    <td className="px-3 py-2">{formatDate(agent.last_active_at)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        ) : (
          <p className="text-sm text-muted-foreground">No agents in this space</p>
        )}
      </div>
      <div>
        <h3 className="mb-2 font-semibold">Recent Memories</h3>
        {data.recent_memories && data.recent_memories.length > 0 ? (
          <div className="space-y-2">
            {data.recent_memories.map((memory) => (
              <div key={memory.memory_id} className="rounded border border-border p-2">
                <p className="text-sm">{memory.content.slice(0, 100)}...</p>
                <p className="mt-1 text-xs text-muted-foreground">
                  by {memory.agent_id} on {formatDate(memory.created_at)}
                </p>
              </div>
            ))}
          </div>
        ) : (
          <p className="text-sm text-muted-foreground">No recent memories</p>
        )}
      </div>
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
        <NavItem href="/dashboard/spaces" label="Spaces" active />
        <NavItem href="/dashboard/agents" label="Agents" />
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
