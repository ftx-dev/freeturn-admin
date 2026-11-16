import { useCallback, useEffect, useState } from "react";
import { api, waitJob, type Probe, type Server } from "../lib/api";
import { errorText, useHostKey } from "../lib/serverApi";
import { Modal } from "./Modal";

const OBF: Record<string, string> = { rtpopus3: "RTP Opus v3", rtpopus2: "RTP Opus v2", rtpopus: "RTP Opus", none: "выключена" };

export function StateBadge({ probe }: { probe: Probe | null }) {
  if (!probe) return <span className="badge"><span className="dot" />неизвестно</span>;
  if (!probe.installed) return <span className="badge badge-warn"><span className="dot" />не установлен</span>;
  return probe.running
    ? <span className="badge badge-ok"><span className="dot" />работает</span>
    : <span className="badge badge-err"><span className="dot" />остановлен</span>;
}

export function ServerOverview({ server, onInstall }: { server: Server; onInstall: () => void }) {
  const report = useHostKey((s) => s.report);
  const [probe, setProbe] = useState<Probe | null>(null);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState("");
  const [error, setError] = useState("");
  const [showKey, setShowKey] = useState(false);
  const [uninstall, setUninstall] = useState(false);

  const refresh = useCallback(async () => {
    setLoading(true);
    setError("");
    try {
      setProbe(await api<Probe>("GET", `/api/servers/${server.id}/status`));
    } catch (e) {
      report(server.id, e);
      setError(errorText(e));
    } finally {
      setLoading(false);
    }
  }, [server.id, report]);
  useEffect(() => { refresh(); }, [refresh]);

  async function action(name: "restart" | "stop") {
    setBusy(name);
    setError("");
    try {
      await api("POST", `/api/servers/${server.id}/${name}`);
      await refresh();
    } catch (e) {
      report(server.id, e);
      setError(errorText(e));
    } finally {
      setBusy("");
    }
  }

  async function doUninstall() {
    setUninstall(false);
    setBusy("uninstall");
    try {
      const job = await waitJob((await api<{ id: string }>("POST", `/api/servers/${server.id}/uninstall`, { target: "all", purge: true })).id);
      if (job.status === "error") setError(job.error ?? "Не удалось удалить");
      await refresh();
    } catch (e) {
      setError(errorText(e));
    } finally {
      setBusy("");
    }
  }

  const c = probe?.config;
  return (
    <>
      {error && <div className="alert alert-err">{error}</div>}
      <div className="card">
        <div className="card-header row-between">
          <span>FreeTurn</span>
          <div className="actions">
            <button className="btn" onClick={refresh} disabled={loading}>{loading ? "Проверяю…" : "Обновить"}</button>
            {probe?.installed && (
              <>
                <button className="btn" onClick={() => action("restart")} disabled={!!busy}>{busy === "restart" ? "Перезапускаю…" : "Перезапустить"}</button>
                {probe.running && <button className="btn" onClick={() => action("stop")} disabled={!!busy}>{busy === "stop" ? "Останавливаю…" : "Остановить"}</button>}
              </>
            )}
          </div>
        </div>
        <div className="card-body">
          {loading && !probe ? <div className="cell-sub">Подключаюсь по SSH…</div> : (
            <dl className="kv">
              <dt>Состояние</dt><dd><StateBadge probe={probe} /></dd>
              {probe && <><dt>Процессор</dt><dd>{probe.arch || "—"}</dd></>}
              {probe && !probe.installed && (
                <><dt /><dd><button className="btn btn-primary" onClick={onInstall}>Установить FreeTurn</button></dd></>
              )}
              {c && (
                <>
                  <dt>Версия</dt><dd>{probe?.version || c.version || "—"}</dd>
                  <dt>Запуск</dt><dd>{c.method === "systemd" ? "systemd" : "Docker"}</dd>
                  <dt>Трафик</dt><dd>{c.backend === "external" ? <>Мой VPN · <span className="mono">{c.connect}</span></> : <>Свой WireGuard · <span className="mono">{c.wg_net}</span></>}</dd>
                  <dt>Режим</dt><dd>{c.mode === "tcp" ? "TCP (Xray/VLESS)" : "UDP (WireGuard/AmneziaWG)"}</dd>
                  <dt>Адрес в ссылках</dt><dd className="mono">{c.host || server.host}:{c.listen_port}</dd>
                  <dt>Обфускация</dt><dd>{OBF[c.obf_profile] ?? c.obf_profile}</dd>
                  {c.obf_key && (
                    <><dt>Ключ обфускации</dt><dd className="mono">
                      {showKey ? c.obf_key : "••••••••••••••••"}{" "}
                      <button className="btn btn-icon" onClick={() => setShowKey(!showKey)}>{showKey ? "скрыть" : "показать"}</button>
                    </dd></>
                  )}
                </>
              )}
            </dl>
          )}
        </div>
      </div>
      {probe?.installed && (
        <div className="card danger">
          <div className="card-body row-between">
            <div>
              <div style={{ fontWeight: 500 }}>Удалить FreeTurn с сервера</div>
              <div className="cell-sub">Остановит FreeTurn и свой WireGuard, удалит их настройки и всех пользователей.</div>
            </div>
            <button className="btn btn-danger" onClick={() => setUninstall(true)} disabled={!!busy}>{busy === "uninstall" ? "Удаляю…" : "Удалить"}</button>
          </div>
        </div>
      )}
      {uninstall && (
        <Modal title="Удалить FreeTurn с сервера?" onClose={() => setUninstall(false)}>
          <div className="form">
            <p style={{ margin: 0 }}>Все выданные ссылки перестанут работать. Это не отменить.</p>
            <div className="actions">
              <button className="btn" onClick={() => setUninstall(false)}>Отмена</button>
              <button className="btn btn-primary btn-destructive" onClick={doUninstall}>Удалить</button>
            </div>
          </div>
        </Modal>
      )}
    </>
  );
}
