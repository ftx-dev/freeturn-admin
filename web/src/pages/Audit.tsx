import { useEffect, useState } from "react";
import { api, type AuditEntry } from "../lib/api";
import { useServers } from "../lib/servers";
import { ACTIONS, formatDate } from "../lib/format";

function details(raw: string): string {
  try {
    return Object.entries(JSON.parse(raw) as Record<string, string>).map(([k, v]) => `${k}: ${v}`).join(", ");
  } catch {
    return raw;
  }
}

export function AuditPage() {
  const servers = useServers((s) => s.list);
  const [list, setList] = useState<AuditEntry[] | null>(null);
  const [error, setError] = useState("");
  useEffect(() => { api<AuditEntry[]>("GET", "/api/audit?limit=200").then(setList).catch((e) => setError(e.message)); }, []);
  const serverName = (id: string) => servers.find((s) => s.id === id)?.name ?? "";

  return (
    <div className="page">
      <div>
        <h1 className="page-title">Журнал действий</h1>
        <p className="page-subtitle">Кто, когда и что делал в админке</p>
      </div>
      {error && <div className="alert alert-err">{error}</div>}
      <div className="table-wrap">
        {list?.length === 0 ? <div className="empty">Записей пока нет</div> : (
          <table className="table">
            <thead><tr><th>Время</th><th>Кто</th><th>Действие</th><th className="hide-sm">Подробности</th></tr></thead>
            <tbody>
              {list?.map((e) => (
                <tr key={e.id}>
                  <td className="cell-sub" style={{ whiteSpace: "nowrap" }}>{formatDate(e.at)}</td>
                  <td>{e.login || "—"}</td>
                  <td>
                    {ACTIONS[e.action] ?? e.action}
                    {e.serverId && serverName(e.serverId) && <span className="cell-sub"> · {serverName(e.serverId)}</span>}
                    {e.result !== "ok" && <span className="badge badge-err" style={{ marginLeft: 8 }}>неудачно</span>}
                  </td>
                  <td className="hide-sm cell-sub mono">{details(e.details)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </div>
  );
}
