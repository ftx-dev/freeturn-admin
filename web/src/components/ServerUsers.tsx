import { useCallback, useEffect, useState, type FormEvent } from "react";
import { api, ApiError, type ClientConf, type Server, type UserList } from "../lib/api";
import { timeAgo } from "../lib/format";
import { errorText, useHostKey } from "../lib/serverApi";
import { LinkModal } from "./LinkModal";
import { Modal } from "./Modal";

const ONLINE_SECONDS = 180;

export function ServerUsers({ server }: { server: Server }) {
  const report = useHostKey((s) => s.report);
  const [list, setList] = useState<UserList | null>(null);
  const [error, setError] = useState("");
  const [name, setName] = useState("");
  const [nameError, setNameError] = useState("");
  const [busy, setBusy] = useState("");
  const [shown, setShown] = useState<{ name: string; link: string } | null>(null);
  const [revoke, setRevoke] = useState("");

  const refresh = useCallback(async () => {
    setError("");
    try {
      setList(await api<UserList>("GET", `/api/servers/${server.id}/users`));
    } catch (e) {
      report(server.id, e);
      setError(errorText(e));
    }
  }, [server.id, report]);
  useEffect(() => { refresh(); }, [refresh]);

  async function add(e: FormEvent) {
    e.preventDefault();
    setNameError("");
    setBusy("add");
    try {
      const c = await api<ClientConf>("POST", `/api/servers/${server.id}/users`, { name: name.trim() });
      setShown({ name: c.client.name, link: c.link });
      setName("");
      refresh();
    } catch (err) {
      report(server.id, err);
      if (err instanceof ApiError && err.field) setNameError(err.message); else setError(errorText(err));
    } finally {
      setBusy("");
    }
  }

  async function showLink(user: string) {
    setBusy(user);
    try {
      const c = await api<ClientConf>("GET", `/api/servers/${server.id}/users/${encodeURIComponent(user)}`);
      setShown({ name: user, link: c.link });
    } catch (e) {
      report(server.id, e);
      setError(errorText(e));
    } finally {
      setBusy("");
    }
  }

  async function doRevoke() {
    const user = revoke;
    setRevoke("");
    setBusy(user);
    try {
      await api("DELETE", `/api/servers/${server.id}/users/${encodeURIComponent(user)}`);
      refresh();
    } catch (e) {
      report(server.id, e);
      setError(errorText(e));
    } finally {
      setBusy("");
    }
  }

  const now = Date.now() / 1000;
  return (
    <>
      {error && <div className="alert alert-err">{error}</div>}
      <form className="card card-body add-row" onSubmit={add}>
        <label className="field" style={{ flex: 1 }}>
          <span className="label">Выдать доступ</span>
          <input className="input" value={name} onChange={(e) => setName(e.target.value)} placeholder="Имя: например, anna-phone"
            aria-invalid={nameError ? true : undefined} spellCheck={false} />
          {nameError && <span className="field-error">{nameError}</span>}
        </label>
        <button className="btn btn-primary" disabled={!name.trim() || busy === "add"}>{busy === "add" ? "Выдаю…" : "Выдать"}</button>
      </form>
      <div className="table-wrap">
        {!list ? <div className="empty">{error ? "Не удалось получить список" : "Загрузка…"}</div> : list.clients.length === 0 ? (
          <div className="empty">Пользователей пока нет</div>
        ) : (
          <table className="table">
            <thead><tr><th>Имя</th><th className="hide-sm">Адрес WG</th><th>В сети</th><th /></tr></thead>
            <tbody>
              {list.clients.map((u, i) => {
                const online = !!u.hs && now - u.hs < ONLINE_SECONDS;
                return (
                  <tr key={u.name} style={{ animationDelay: `${i * 25}ms` }}>
                    <td>
                      <span style={{ fontWeight: 500 }}>{u.name}</span>
                      {u.self && <span className="badge" style={{ marginLeft: 8 }}>владелец</span>}
                      <div className="cell-sub mono">{u.client_id}</div>
                    </td>
                    <td className="hide-sm mono">{u.ip ?? "—"}</td>
                    <td>{u.pub === undefined ? <span className="cell-sub">—</span> : online
                      ? <span className="badge badge-ok"><span className="dot" />в сети</span>
                      : <span className="cell-sub">{u.hs ? timeAgo(u.hs) : "не подключался"}</span>}</td>
                    <td style={{ textAlign: "right", whiteSpace: "nowrap" }}>
                      <button className="btn" onClick={() => showLink(u.name)} disabled={!!busy}>{busy === u.name ? "…" : "Ссылка"}</button>
                      {!u.self && <button className="btn btn-danger" onClick={() => setRevoke(u.name)} disabled={!!busy}>Отозвать</button>}
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        )}
      </div>
      {shown && <LinkModal title={`Доступ «${shown.name}»`} link={shown.link} fileName={`${server.name}-${shown.name}`} onClose={() => setShown(null)} />}
      {revoke && (
        <Modal title={`Отозвать доступ «${revoke}»?`} onClose={() => setRevoke("")}>
          <div className="form">
            <p style={{ margin: 0 }}>Ссылка этого пользователя сразу перестанет работать.</p>
            <div className="actions">
              <button className="btn" onClick={() => setRevoke("")}>Отмена</button>
              <button className="btn btn-primary btn-destructive" onClick={doRevoke}>Отозвать</button>
            </div>
          </div>
        </Modal>
      )}
    </>
  );
}
