import { Link, Outlet } from "@tanstack/react-router";
import { useEffect, useState } from "react";
import { ROLE_NAMES, type Role } from "../lib/api";
import { useSession } from "../lib/session";
import { useServers } from "../lib/servers";
import { applyTheme, loadTheme, type Theme } from "../lib/theme";
import { useHealth } from "../lib/health";
import { HealthDot } from "./HealthDot";
import { Icon, type IconName } from "./Icon";
import { Logo } from "./Logo";

const TELEGRAM = "https://t.me/+ZVdbpZB0w_Q1MTdi";

function NavItem({ to, icon, label, exact }: { to: string; icon: IconName; label: string; exact?: boolean }) {
  return (
    <Link to={to} className="nav-item" activeProps={{ className: "nav-item active" }} activeOptions={{ exact }}>
      <Icon name={icon} />
      <span>{label}</span>
    </Link>
  );
}

export function Layout() {
  const { me, logout } = useSession();
  const { list, refresh } = useServers();
  const [theme, setTheme] = useState<Theme>(loadTheme);
  useEffect(() => { applyTheme(theme); }, [theme]);
  useEffect(() => { refresh(); }, [refresh]);
  const refreshHealth = useHealth((s) => s.refresh);
  useEffect(() => {
    refreshHealth();
    const t = setInterval(refreshHealth, 15_000);
    return () => clearInterval(t);
  }, [refreshHealth, list.length]);

  return (
    <div className="layout">
      <aside className="sidebar">
        <div className="brand">
          <Logo />
          <div>
            <div className="brand-name">FreeTurnX Admin</div>
            <div className="brand-sub">free-turn-proxy</div>
          </div>
        </div>
        <nav className="nav" aria-label="Разделы">
          <NavItem to="/" icon="dashboard" label="Обзор" exact />
          <NavItem to="/servers" icon="server" label="Серверы" exact />
          {list.length > 0 && <div className="nav-label">Серверы</div>}
          {list.map((s) => (
            <Link key={s.id} to="/servers/$id" params={{ id: s.id }} className="nav-item" activeProps={{ className: "nav-item active" }}>
              <HealthDot serverId={s.id} />
              <span>{s.name}</span>
            </Link>
          ))}
          <div className="nav-label">Система</div>
          <NavItem to="/admins" icon="users" label="Учётки админки" />
          <NavItem to="/audit" icon="audit" label="Журнал действий" />
          <a className="nav-item" href={TELEGRAM} target="_blank" rel="noreferrer"><Icon name="telegram" /><span>Telegram-канал</span></a>
        </nav>
        <div className="sidebar-footer">
          <div className="avatar" aria-hidden="true">{me?.login.slice(0, 1)}</div>
          <div className="user">
            <div className="user-name">{me?.login}</div>
            <div className="user-role">{ROLE_NAMES[me?.role as Role] ?? me?.role}</div>
          </div>
          <button className="btn btn-icon" onClick={() => setTheme(theme === "dark" ? "light" : "dark")}
            aria-label={theme === "dark" ? "Светлая тема" : "Тёмная тема"} title="Тема">
            <Icon name={theme === "dark" ? "sun" : "moon"} />
          </button>
          <button className="btn btn-icon" onClick={logout} aria-label="Выйти" title="Выйти"><Icon name="logout" /></button>
        </div>
      </aside>
      <main className="main"><Outlet /></main>
    </div>
  );
}
