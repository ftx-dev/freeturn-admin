import { Link, useNavigate } from "@tanstack/react-router";
import { useState } from "react";
import { Modal } from "../components/Modal";
import { ServerForm } from "../components/ServerForm";
import { ServerInstall } from "../components/ServerInstall";
import { ServerLogs } from "../components/ServerLogs";
import { ServerOverview } from "../components/ServerOverview";
import { ServerUsers } from "../components/ServerUsers";
import { api, type Server } from "../lib/api";
import { useServers } from "../lib/servers";
import { errorText, useHostKey } from "../lib/serverApi";
import { serverRoute, type ServerTab } from "../router";

const TABS: [ServerTab, string][] = [["overview", "Обзор"], ["users", "Пользователи"], ["install", "Установка и настройки"], ["logs", "Журнал"]];

function HostKeyBanner({ server }: { server: Server }) {
  const changed = useHostKey((s) => s.changed[server.id]);
  const clear = useHostKey((s) => s.clear);
  const refresh = useServers((s) => s.refresh);
  const [error, setError] = useState("");
  if (!changed) return null;
  async function trust() {
    try {
      await api("POST", `/api/servers/${server.id}/ssh/trust`, { fingerprint: changed.got });
      clear(server.id);
      refresh();
    } catch (e) {
      setError(errorText(e));
    }
  }
  return (
    <div className="alert alert-warn">
      <strong>Ключ хоста сервера изменился.</strong> Так бывает после переустановки системы на VPS. Если вы её не делали —
      возможно, кто-то подменил сервер: не подтверждайте.
      <dl className="kv" style={{ margin: "10px 0" }}>
        <dt>Был</dt><dd className="mono">{changed.want}</dd>
        <dt>Сейчас</dt><dd className="mono">{changed.got}</dd>
      </dl>
      {error && <div className="field-error">{error}</div>}
      <button className="btn" onClick={trust}>Сервер переустанавливали — доверять новому ключу</button>
    </div>
  );
}

export function ServerPage() {
  const { id } = serverRoute.useParams();
  const { tab = "overview" } = serverRoute.useSearch();
  const { list, loaded, update, remove } = useServers();
  const navigate = useNavigate();
  const [editing, setEditing] = useState(false);
  const [confirm, setConfirm] = useState(false);
  const [error, setError] = useState("");
  const server = list.find((s) => s.id === id);
  const setTab = (t: ServerTab) => navigate({ to: "/servers/$id", params: { id }, search: t === "overview" ? {} : { tab: t }, replace: true });

  if (!server) {
    return (
      <div className="page">
        <div className="empty">{loaded ? <>Сервер не найден. <Link to="/servers" className="btn">К списку</Link></> : "Загрузка…"}</div>
      </div>
    );
  }

  async function doRemove() {
    try {
      await remove(id);
      navigate({ to: "/servers" });
    } catch (e) {
      setError(errorText(e));
      setConfirm(false);
    }
  }

  return (
    <div className="page">
      <div className="page-header">
        <div>
          <div className="crumbs"><Link to="/servers">Серверы</Link><span>/</span><span>{server.name}</span></div>
          <h1 className="page-title">{server.name}</h1>
          <p className="page-subtitle mono">{server.sshUser}@{server.host}:{server.sshPort}</p>
        </div>
        <div className="actions">
          <button className="btn" onClick={() => setEditing(true)}>Доступ по SSH</button>
          <button className="btn btn-danger" onClick={() => setConfirm(true)}>Убрать из админки</button>
        </div>
      </div>
      <HostKeyBanner server={server} />
      {error && <div className="alert alert-err">{error}</div>}
      <nav className="tabs" role="tablist">
        {TABS.map(([t, label]) => (
          <button key={t} role="tab" aria-selected={tab === t} className="tab" onClick={() => setTab(t)}>{label}</button>
        ))}
      </nav>
      {tab === "overview" && <ServerOverview key={server.id} server={server} onInstall={() => setTab("install")} />}
      {tab === "users" && <ServerUsers key={server.id} server={server} />}
      {tab === "install" && <ServerInstall key={server.id} server={server} />}
      {tab === "logs" && <ServerLogs key={server.id} server={server} />}

      {editing && (
        <Modal title="Доступ по SSH" onClose={() => setEditing(false)}>
          <ServerForm server={server} submitLabel="Сохранить" onCancel={() => setEditing(false)}
            onSubmit={async (input) => { await update(id, input); setEditing(false); }} />
        </Modal>
      )}
      {confirm && (
        <Modal title="Убрать сервер из админки?" onClose={() => setConfirm(false)}>
          <div className="form">
            <p style={{ margin: 0 }}>«{server.name}» исчезнет из админки вместе с доступом по SSH. FreeTurn на самом VPS продолжит работать.</p>
            <div className="actions">
              <button className="btn" onClick={() => setConfirm(false)}>Отмена</button>
              <button className="btn btn-primary btn-destructive" onClick={doRemove}>Убрать</button>
            </div>
          </div>
        </Modal>
      )}
    </div>
  );
}
