import { useNavigate } from "@tanstack/react-router";
import { HealthDot } from "../components/HealthDot";
import { Icon } from "../components/Icon";
import { Modal } from "../components/Modal";
import { ServerForm } from "../components/ServerForm";
import { healthTitle, useHealth } from "../lib/health";
import { useServers } from "../lib/servers";
import { formatDate } from "../lib/format";
import { serversRoute } from "../router";

// Состояние - по проверке доступности (та же, что у точек): SSH-порт отвечает или нет.
function HealthBadge({ serverId, fingerprint }: { serverId: string; fingerprint: string }) {
  const h = useHealth((s) => s.byId[serverId]);
  const badge = !h || h.state === "unknown"
    ? <span className="badge"><span className="dot" />проверяю…</span>
    : h.state === "up"
      ? <span className="badge badge-ok"><span className="dot" />доступен · {h.latencyMs} мс</span>
      : <span className="badge badge-err"><span className="dot" />не отвечает</span>;
  return (
    <div title={healthTitle(h)}>
      {badge}
      {!fingerprint && <div className="cell-sub">ключ хоста ещё не запомнен</div>}
    </div>
  );
}

export function ServersPage() {
  const { list, loaded, error, create } = useServers();
  const { add } = serversRoute.useSearch();
  const navigate = useNavigate();
  const setAdd = (v: boolean) => navigate({ to: "/servers", search: v ? { add: true } : {}, replace: true });

  return (
    <div className="page">
      <div className="page-header">
        <div>
          <h1 className="page-title">Серверы</h1>
          <p className="page-subtitle">VPS с FreeTurn, к которым админка ходит по SSH</p>
        </div>
        <button className="btn btn-primary" onClick={() => setAdd(true)}><Icon name="plus" />Добавить сервер</button>
      </div>
      {error && <div className="alert alert-err">{error}</div>}
      <div className="table-wrap">
        {loaded && list.length === 0 ? (
          <div className="empty">
            <div className="empty-title">Серверов пока нет</div>
            <div>Добавьте VPS: адрес и доступ по SSH — паролем или ключом.</div>
            <button className="btn btn-primary" onClick={() => setAdd(true)}><Icon name="plus" />Добавить сервер</button>
          </div>
        ) : (
          <table className="table">
            <thead>
              <tr><th>Название</th><th>Адрес</th><th className="hide-sm">Вход</th><th>Состояние</th><th className="hide-sm">Добавлен</th></tr>
            </thead>
            <tbody>
              {list.map((s, i) => (
                <tr key={s.id} className="clickable" style={{ animationDelay: `${i * 25}ms` }}
                  onClick={() => navigate({ to: "/servers/$id", params: { id: s.id } })}>
                  <td style={{ fontWeight: 500 }}><span className="name-with-dot"><HealthDot serverId={s.id} />{s.name}</span></td>
                  <td className="mono">{s.sshUser}@{s.host}{s.sshPort !== 22 && `:${s.sshPort}`}</td>
                  <td className="hide-sm cell-sub">{s.authType === "key" ? "ключ SSH" : "пароль"}</td>
                  <td><HealthBadge serverId={s.id} fingerprint={s.hostFingerprint} /></td>
                  <td className="hide-sm cell-sub">{formatDate(s.createdAt)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
      {add && (
        <Modal title="Новый сервер" onClose={() => setAdd(false)}>
          <ServerForm submitLabel="Добавить" onCancel={() => setAdd(false)} onSubmit={async (input) => {
            const s = await create(input);
            navigate({ to: "/servers/$id", params: { id: s.id } });
          }} />
        </Modal>
      )}
    </div>
  );
}
