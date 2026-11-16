import { createRootRoute, createRoute, createRouter, Outlet } from "@tanstack/react-router";
import { useEffect } from "react";
import { Layout } from "./components/Layout";
import { useSession } from "./lib/session";
import { AdminsPage } from "./pages/Admins";
import { AuditPage } from "./pages/Audit";
import { AuthPage } from "./pages/Auth";
import { DashboardPage } from "./pages/Dashboard";
import { ServerPage } from "./pages/Server";
import { ServersPage } from "./pages/Servers";

// Корень решает по сессии: первичная настройка, вход или приложение. Отдельных URL для входа нет -
// после входа человек остаётся на той странице, которую открывал.
function Root() {
  const { status, load } = useSession();
  useEffect(() => { load(); }, [load]);
  if (status === "loading") return <div className="center">Загрузка…</div>;
  if (status === "offline") return <div className="center">Сервер админки недоступен. Обновите страницу.</div>;
  if (status === "setup") return <AuthPage mode="setup" />;
  if (status === "anon") return <AuthPage mode="login" />;
  return <Outlet />;
}

const rootRoute = createRootRoute({ component: Root });
const appRoute = createRoute({ getParentRoute: () => rootRoute, id: "app", component: Layout });
const dashboardRoute = createRoute({ getParentRoute: () => appRoute, path: "/", component: DashboardPage });
export const serversRoute = createRoute({
  getParentRoute: () => appRoute,
  path: "/servers",
  component: ServersPage,
  validateSearch: (s: Record<string, unknown>): { add?: boolean } => (s.add === true || s.add === "true" ? { add: true } : {}),
});
export type ServerTab = "overview" | "users" | "install" | "logs";
const serverTabs: ServerTab[] = ["overview", "users", "install", "logs"];
export const serverRoute = createRoute({
  getParentRoute: () => appRoute,
  path: "/servers/$id",
  component: ServerPage,
  validateSearch: (s: Record<string, unknown>): { tab?: ServerTab } =>
    serverTabs.includes(s.tab as ServerTab) ? { tab: s.tab as ServerTab } : {},
});
const auditRoute = createRoute({ getParentRoute: () => appRoute, path: "/audit", component: AuditPage });
const adminsRoute = createRoute({ getParentRoute: () => appRoute, path: "/admins", component: AdminsPage });

export const router = createRouter({
  routeTree: rootRoute.addChildren([appRoute.addChildren([dashboardRoute, serversRoute, serverRoute, auditRoute, adminsRoute])]),
  defaultPreload: "intent",
});

declare module "@tanstack/react-router" {
  interface Register { router: typeof router }
}
