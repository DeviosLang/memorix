import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Sidebar } from "@/components/sidebar";
import { ThemeToggle } from "@/components/theme-toggle";
import { LocaleToggle } from "@/components/locale-toggle";
import { useSessionTimeout } from "@/lib/session";
import { useSpaces, useSpaceDetail } from "@/api/queries";
import { formatRelativeTime, formatDateTime } from "@/lib/i18n";
import type { SpaceListItem } from "@/types/metrics";

export function SpacesPage() {
  const { t, i18n } = useTranslation();
  useSessionTimeout();
  const { data, isLoading, error } = useSpaces();
  const [sortBy, setSortBy] = useState<"memory" | "activity">("memory");
  const [expandedSpace, setExpandedSpace] = useState<string | null>(null);

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

  const timeAgo = (dateStr?: string) => {
    return formatRelativeTime(t, dateStr);
  };

  const formatDate = (dateStr?: string) => {
    return formatDateTime(dateStr, i18n.language);
  };

  return (
    <div className="flex min-h-screen">
      <Sidebar />
      <main className="flex-1 pt-14 lg:pt-0">
        <header className="flex h-16 items-center justify-between border-b border-border px-4 lg:px-6">
          <h1 className="text-xl font-semibold">{t("spaces.title")}</h1>
          <div className="hidden items-center gap-4 lg:flex">
            <LocaleToggle />
            <ThemeToggle />
          </div>
        </header>
        <div className="p-4 lg:p-6">
          {isLoading && (
            <div className="text-muted-foreground">{t("spaces.loading")}</div>
          )}
          {error && (
            <div className="text-red-500">{t("spaces.error")}</div>
          )}

          {data && (
            <>
              <div className="mb-4 flex flex-wrap items-center gap-2 lg:gap-4">
                <span className="text-sm text-muted-foreground">{t("spaces.sortBy")}</span>
                <button
                  onClick={() => setSortBy("memory")}
                  className={`rounded-md px-3 py-1 text-sm ${
                    sortBy === "memory"
                      ? "bg-primary text-primary-foreground"
                      : "bg-muted hover:bg-muted/80"
                  }`}
                >
                  {t("spaces.memoryCount")}
                </button>
                <button
                  onClick={() => setSortBy("activity")}
                  className={`rounded-md px-3 py-1 text-sm ${
                    sortBy === "activity"
                      ? "bg-primary text-primary-foreground"
                      : "bg-muted hover:bg-muted/80"
                  }`}
                >
                  {t("spaces.lastActivity")}
                </button>
              </div>

              <div className="overflow-x-auto rounded-lg border border-border">
                <table className="w-full min-w-[600px]">
                  <thead>
                    <tr className="border-b border-border bg-muted/50">
                      <th className="px-4 py-3 text-left text-sm font-medium">
                        {t("spaces.table.name")}
                      </th>
                      <th className="px-4 py-3 text-left text-sm font-medium">
                        {t("spaces.table.memories")}
                      </th>
                      <th className="px-4 py-3 text-left text-sm font-medium">
                        {t("spaces.table.agents")}
                      </th>
                      <th className="px-4 py-3 text-left text-sm font-medium">
                        {t("spaces.table.lastActive")}
                      </th>
                      <th className="px-4 py-3 text-left text-sm font-medium">
                        {t("spaces.table.storage")}
                      </th>
                      <th className="px-4 py-3 text-left text-sm font-medium">
                        {t("spaces.table.status")}
                      </th>
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
                {t("spaces.totalSpaces", { count: data.total_count })}
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
  const { t } = useTranslation();

  const getStatusLabel = (status: string): string => {
    return status === "active" ? t("status.active") : t("status.inactive");
  };

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
            {getStatusLabel(space.status)}
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
  const { t } = useTranslation();
  const { data, isLoading } = useSpaceDetail(tenantId);

  if (isLoading) {
    return <div className="text-sm text-muted-foreground">{t("spaces.detail.loading")}</div>;
  }

  if (!data) {
    return null;
  }

  return (
    <div className="grid gap-6 md:grid-cols-2">
      <div>
        <h3 className="mb-2 font-semibold">{t("spaces.detail.agents", { count: data.agents?.length || 0 })}</h3>
        {data.agents && data.agents.length > 0 ? (
          <div className="overflow-x-auto rounded border border-border">
            <table className="w-full min-w-[300px] text-sm">
              <thead>
                <tr className="border-b border-border bg-muted/50">
                  <th className="px-3 py-2 text-left">{t("spaces.detail.agentId")}</th>
                  <th className="px-3 py-2 text-left">{t("spaces.detail.agentType")}</th>
                  <th className="px-3 py-2 text-left">{t("spaces.detail.lastActive")}</th>
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
          <p className="text-sm text-muted-foreground">{t("spaces.detail.noAgents")}</p>
        )}
      </div>
      <div>
        <h3 className="mb-2 font-semibold">{t("spaces.detail.recentMemories")}</h3>
        {data.recent_memories && data.recent_memories.length > 0 ? (
          <div className="space-y-2">
            {data.recent_memories.map((memory) => (
              <div key={memory.memory_id} className="rounded border border-border p-2">
                <p className="text-sm">{memory.content.slice(0, 100)}...</p>
                <p className="mt-1 text-xs text-muted-foreground">
                  {t("spaces.detail.by", { agent: memory.agent_id, date: formatDate(memory.created_at) })}
                </p>
              </div>
            ))}
          </div>
        ) : (
          <p className="text-sm text-muted-foreground">{t("spaces.detail.noMemories")}</p>
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
