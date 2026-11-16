import { Link, useNavigate } from "@tanstack/react-router";
import { useState } from "react";
import { Modal } from "../components/Modal";
import { ServerForm } from "../components/ServerForm";
import { useServers } from "../lib/servers";
import { formatDate } from "../lib/format";
import { serverRoute } from "../router";
import { HostKeyBadge } from "./Servers";

export function ServerPage() {
  const { id } = serverRoute.useParams();
  const { list, loaded, update, remove } = useServers();
  const navigate = useNavigate();
  const [editing, setEditing] = useState(false);
  const [confirm, setConfirm] = useState(false);
  const [error, setError] = useState("");
  const server = list.find((s) => s.id === id);

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
      setError((e as Error).message);
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
          <button className="btn" onClick={() => setEditing(true)}>Изменить</button>
          <button className="btn btn-danger" onClick={() => setConfirm(true)}>Удалить</button>
        </div>
      </div>
      {error && <div className="alert alert-err">{error}</div>}
      <div className="card">
        <div className="card-header">Подключение</div>
        <div className="card-body">
          <dl className="kv">
            <dt>Состояние</dt><dd><HostKeyBadge fingerprint={server.hostFingerprint} /></dd>
            <dt>Вход</dt><dd>{server.authType === "key" ? "Ключ SSH" : "Пароль"}{server.hasKeyPassphrase && " (с паролем ключа)"}</dd>
            <dt>Ключ хоста</dt><dd className={server.hostFingerprint ? "mono" : "cell-sub"}>{server.hostFingerprint || "Запомнится при первом подключении"}</dd>
            {server.sshUser !== "root" && <><dt>sudo</dt><dd>{server.hasSudoPassword ? "С паролем" : "Без пароля"}</dd></>}
            <dt>Добавлен</dt><dd>{formatDate(server.createdAt)}</dd>
            <dt>Изменён</dt><dd>{formatDate(server.updatedAt)}</dd>
          </dl>
        </div>
      </div>
      {editing && (
        <Modal title="Изменить сервер" onClose={() => setEditing(false)}>
          <ServerForm server={server} submitLabel="Сохранить" onCancel={() => setEditing(false)}
            onSubmit={async (input) => { await update(id, input); setEditing(false); }} />
        </Modal>
      )}
      {confirm && (
        <Modal title="Удалить сервер?" onClose={() => setConfirm(false)}>
          <div className="form">
            <p style={{ margin: 0 }}>«{server.name}» и его доступы удалятся из админки. FreeTurn на самом VPS останется работать.</p>
            <div className="actions">
              <button className="btn" onClick={() => setConfirm(false)}>Отмена</button>
              <button className="btn btn-primary" style={{ background: "var(--error)", color: "#fff" }} onClick={doRemove}>Удалить</button>
            </div>
          </div>
        </Modal>
      )}
    </div>
  );
}
