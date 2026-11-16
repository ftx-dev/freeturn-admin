import { useEffect, useState } from "react";
import { api, ApiError, type Server } from "../lib/api";
import { errorText } from "../lib/serverApi";
import { Icon } from "./Icon";

export interface Call { label: string; url: string }

// Звонки сервера: их предлагают при выдаче ссылки пользователю (окно «Доступ»).
export function ServerCalls({ server }: { server: Server }) {
  const [calls, setCalls] = useState<Call[] | null>(null);
  const [saved, setSaved] = useState("");
  const [error, setError] = useState<{ index?: number; message: string } | null>(null);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    api<{ calls: Call[] }>("GET", `/api/servers/${server.id}/calls`)
      .then((r) => { setCalls(r.calls); setSaved(JSON.stringify(r.calls)); })
      .catch((e) => setError({ message: errorText(e) }));
  }, [server.id]);

  const set = (i: number, patch: Partial<Call>) => setCalls((c) => c!.map((x, j) => (j === i ? { ...x, ...patch } : x)));
  const dirty = calls !== null && JSON.stringify(calls) !== saved;

  async function save() {
    setBusy(true);
    setError(null);
    try {
      const r = await api<{ calls: Call[] }>("PUT", `/api/servers/${server.id}/calls`, { calls });
      setCalls(r.calls);
      setSaved(JSON.stringify(r.calls));
    } catch (e) {
      const m = e instanceof ApiError ? /^calls\.(\d+)$/.exec(e.field ?? "") : null;
      setError({ index: m ? Number(m[1]) : undefined, message: errorText(e) });
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="card">
      <div className="card-header row-between">
        <div>
          <div>Звонки</div>
          <div className="cell-sub" style={{ fontWeight: 400 }}>Ссылки на звонки этого сервера — их предложит окно выдачи доступа, чтобы вложить в ссылку пользователя.</div>
        </div>
      </div>
      <div className="card-body form">
        {calls === null ? <div className="cell-sub">{error?.message ?? "Загрузка…"}</div> : (
          <>
            {calls.length === 0 && <div className="cell-sub">Звонков пока нет.</div>}
            {calls.map((c, i) => (
              <div key={i} className="call-row">
                <input className="input" value={c.label} onChange={(e) => set(i, { label: e.target.value })} placeholder="Подпись (необязательно)" />
                <input className="input mono" value={c.url} onChange={(e) => set(i, { url: e.target.value })} placeholder="https://vk.com/call/join/…"
                  spellCheck={false} aria-invalid={error?.index === i ? true : undefined} />
                <button type="button" className="btn btn-icon" aria-label="Убрать" onClick={() => setCalls(calls.filter((_, j) => j !== i))}><Icon name="close" /></button>
              </div>
            ))}
            {error && <div className="alert alert-err">{error.index !== undefined && `Звонок ${error.index + 1}: `}{error.message}</div>}
            <div className="row-between">
              <button type="button" className="btn" onClick={() => setCalls([...calls, { label: "", url: "" }])}><Icon name="plus" />Добавить звонок</button>
              <button type="button" className="btn btn-primary" onClick={save} disabled={!dirty || busy}>{busy ? "Сохраняю…" : dirty ? "Сохранить" : "Сохранено"}</button>
            </div>
          </>
        )}
      </div>
    </div>
  );
}
