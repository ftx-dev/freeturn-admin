import { useCallback, useEffect, useState } from "react";
import { api, type Server } from "../lib/api";
import { errorText, useHostKey } from "../lib/serverApi";

export function ServerLogs({ server }: { server: Server }) {
  const report = useHostKey((s) => s.report);
  const [lines, setLines] = useState<string[] | null>(null);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  const refresh = useCallback(async () => {
    setLoading(true);
    setError("");
    try {
      setLines((await api<{ lines: string[] }>("GET", `/api/servers/${server.id}/logs?tail=300`)).lines ?? []);
    } catch (e) {
      report(server.id, e);
      setError(errorText(e));
    } finally {
      setLoading(false);
    }
  }, [server.id, report]);
  useEffect(() => { refresh(); }, [refresh]);

  return (
    <>
      <div className="row-between">
        <span className="cell-sub">Последние 300 строк журнала FreeTurn на сервере</span>
        <button className="btn" onClick={refresh} disabled={loading}>{loading ? "Загружаю…" : "Обновить"}</button>
      </div>
      {error && <div className="alert alert-err">{error}</div>}
      <pre className="log">{lines === null ? "Загрузка…" : lines.length ? lines.join("\n") : "Журнал пуст"}</pre>
    </>
  );
}
