import { Link } from "@tanstack/react-router";
import { useEffect, useState } from "react";
import { api, type AuditEntry } from "../lib/api";
import { useServers } from "../lib/servers";
import { ACTIONS, timeAgo } from "../lib/format";

export function DashboardPage() {
  const servers = useServers((s) => s.list);
  const [audit, setAudit] = useState<AuditEntry[]>([]);
  useEffect(() => { api<AuditEntry[]>("GET", "/api/audit?limit=8").then(setAudit).catch(() => {}); }, []);
  const trusted = servers.filter((s) => s.hostFingerprint).length;

  return (
    <div className="page">
      <div className="page-header">
        <div>
          <h1 className="page-title">Обзор</h1>
          <p className="page-subtitle">Серверы FreeTurn и последние действия</p>
        </div>
        <Link to="/servers" search={{ add: true }} className="btn btn-primary">Добавить сервер</Link>
      </div>
      <div className="stats">
        <div className="stat"><div className="stat-value">{servers.length}</div><div className="stat-label">Серверов</div></div>
        <div className="stat"><div className="stat-value">{trusted}</div><div className="stat-label">С проверенным ключом хоста</div></div>
        <div className="stat"><div className="stat-value">{servers.length - trusted}</div><div className="stat-label">Ждут первого подключения</div></div>
      </div>
      <h2 className="section-title">Последние действия</h2>
      <div className="table-wrap">
        {audit.length === 0 ? (
          <div className="empty">Пока ничего не происходило</div>
        ) : (
          <table className="table">
            <tbody>
              {audit.map((e, i) => (
                <tr key={e.id} style={{ animationDelay: `${i * 25}ms` }}>
                  <td>{ACTIONS[e.action] ?? e.action}{e.result !== "ok" && <span className="badge badge-err" style={{ marginLeft: 8 }}>неудачно</span>}</td>
                  <td className="cell-sub">{e.login || "—"}</td>
                  <td className="cell-sub" style={{ textAlign: "right" }}>{timeAgo(e.at)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </div>
  );
}
