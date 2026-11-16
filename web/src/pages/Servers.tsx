import { useNavigate } from "@tanstack/react-router";
import { Icon } from "../components/Icon";
import { Modal } from "../components/Modal";
import { ServerForm } from "../components/ServerForm";
import { useServers } from "../lib/servers";
import { formatDate } from "../lib/format";
import { serversRoute } from "../router";

export function HostKeyBadge({ fingerprint }: { fingerprint: string }) {
  return fingerprint
    ? <span className="badge badge-ok"><span className="dot" />ключ хоста проверен</span>
    : <span className="badge badge-warn"><span className="dot" />не подключался</span>;
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
                  <td style={{ fontWeight: 500 }}>{s.name}</td>
                  <td className="mono">{s.sshUser}@{s.host}{s.sshPort !== 22 && `:${s.sshPort}`}</td>
                  <td className="hide-sm cell-sub">{s.authType === "key" ? "ключ SSH" : "пароль"}</td>
                  <td><HostKeyBadge fingerprint={s.hostFingerprint} /></td>
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
