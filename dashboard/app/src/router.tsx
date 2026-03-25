import {
  createRouter,
  createRootRoute,
  createRoute,
  Outlet,
  redirect,
} from "@tanstack/react-router";
import { LoginPage } from "./pages/login";
import { DashboardPage } from "./pages/dashboard";
import { SpacesPage } from "./pages/spaces";
import { AgentsPage } from "./pages/agents";
import { StoragePage } from "./pages/storage";
import { getDashboardToken } from "./api/client";

const rootRoute = createRootRoute({
  component: () => (
    <div className="min-h-screen bg-background">
      <Outlet />
    </div>
  ),
});

const loginRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/",
  component: LoginPage,
});

const dashboardRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/dashboard",
  component: DashboardPage,
  beforeLoad: () => {
    const token = getDashboardToken();
    if (!token) {
      throw redirect({ to: "/" });
    }
  },
});

const spacesRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/dashboard/spaces",
  component: SpacesPage,
  beforeLoad: () => {
    const token = getDashboardToken();
    if (!token) {
      throw redirect({ to: "/" });
    }
  },
});

const agentsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/dashboard/agents",
  component: AgentsPage,
  beforeLoad: () => {
    const token = getDashboardToken();
    if (!token) {
      throw redirect({ to: "/" });
    }
  },
});

const storageRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/dashboard/storage",
  component: StoragePage,
  beforeLoad: () => {
    const token = getDashboardToken();
    if (!token) {
      throw redirect({ to: "/" });
    }
  },
});

const routeTree = rootRoute.addChildren([loginRoute, dashboardRoute, spacesRoute, agentsRoute, storageRoute]);

export const router = createRouter({ routeTree });

declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router;
  }
}
