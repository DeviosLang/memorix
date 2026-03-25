import { Link } from "@tanstack/react-router";

export function DashboardPage() {
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
          <NavItem href="/dashboard/memory" label="Memory Stats" />
          <NavItem href="/dashboard/search" label="Search Stats" />
          <NavItem href="/dashboard/gc" label="GC Stats" />
          <NavItem href="/dashboard/tenants" label="Tenants" />
        </nav>
        <div className="border-t border-border p-2">
          <Link
            to="/"
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
          <div className="flex items-center gap-2">
            <span className="text-sm text-muted-foreground">System Status:</span>
            <span className="inline-flex items-center rounded-full bg-green-100 px-2 py-1 text-xs font-medium text-green-700 dark:bg-green-900 dark:text-green-300">
              Healthy
            </span>
          </div>
        </header>
        <div className="p-6">
          {/* Placeholder content */}
          <div className="grid gap-6 md:grid-cols-2 lg:grid-cols-4">
            <StatCard title="Total Memories" value="--" />
            <StatCard title="Active Tenants" value="--" />
            <StatCard title="Requests/min" value="--" />
            <StatCard title="Error Rate" value="--" />
          </div>

          <div className="mt-6 rounded-lg border border-border p-6">
            <h2 className="text-lg font-semibold">System Overview</h2>
            <p className="mt-2 text-sm text-muted-foreground">
              Dashboard metrics will appear here once connected to the backend.
              Configure your dashboard token and ensure the server is running.
            </p>
          </div>
        </div>
      </main>
    </div>
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

function StatCard({ title, value }: { title: string; value: string }) {
  return (
    <div className="rounded-lg border border-border bg-card p-4">
      <p className="text-sm font-medium text-muted-foreground">{title}</p>
      <p className="mt-1 text-2xl font-bold">{value}</p>
    </div>
  );
}
