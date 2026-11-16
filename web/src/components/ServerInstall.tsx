import { useEffect, useState, type FormEvent, type ReactNode } from "react";
import { api, ApiError, waitJob, type ApplySettings, type Job, type Probe, type Server } from "../lib/api";
import { errorText, useHostKey } from "../lib/serverApi";
import { LinkModal } from "./LinkModal";

function Segmented<T extends string>({ value, options, onChange }: { value: T; options: [T, string][]; onChange: (v: T) => void }) {
  return (
    <div className="segmented" role="group">
      {options.map(([v, label]) => (
        <button key={v} type="button" aria-pressed={value === v} onClick={() => onChange(v)}>{label}</button>
      ))}
    </div>
  );
}

function Field({ label, hint, error, children }: { label: string; hint?: string; error?: string; children: ReactNode }) {
  return (
    <label className="field">
      <span className="label">{label}</span>
      {children}
      {error ? <span className="field-error">{error}</span> : hint && <span className="hint">{hint}</span>}
    </label>
  );
}

const STAGES: Record<string, string> = {
  deps: "установка пакетов", download: "загрузка FreeTurn", wg: "настройка WireGuard", apply: "применение настроек",
};

// Мастер установки: значения по умолчанию подобраны админкой (порт 56000-56999, ключ обфускации),
// для установленного сервера поля заполняются его текущими настройками.
export function ServerInstall({ server }: { server: Server }) {
  const report = useHostKey((s) => s.report);
  const [probe, setProbe] = useState<Probe | null>(null);
  const [s, setS] = useState<Required<Omit<ApplySettings, "update">> & { update: boolean }>({
    method: "docker", backend: "new", connect: "", mode: "udp", listenPort: 0, wgPort: 0, wgNet: "",
    obfProfile: "rtpopus3", obfKey: "", host: "", update: false,
  });
  const [loadError, setLoadError] = useState("");
  const [fieldError, setFieldError] = useState<{ field?: string; message: string } | null>(null);
  const [job, setJob] = useState<Job | null>(null);
  const [owner, setOwner] = useState("");
  const [showOwner, setShowOwner] = useState(false);

  useEffect(() => {
    (async () => {
      try {
        const [p, d] = await Promise.all([
          api<Probe>("GET", `/api/servers/${server.id}/status`),
          api<{ listenPort: number; obfKey: string }>("GET", "/api/defaults"),
        ]);
        setProbe(p);
        const c = p.config;
        setS((x) => c ? {
          ...x, method: c.method || "docker", backend: c.backend || "new", connect: c.connect, mode: c.mode || "udp",
          listenPort: c.listen_port, wgPort: c.wg_port, wgNet: c.wg_net, obfProfile: c.obf_profile || "rtpopus3",
          obfKey: c.obf_key, host: c.host,
        } : { ...x, listenPort: d.listenPort, obfKey: d.obfKey });
      } catch (e) {
        report(server.id, e);
        setLoadError(errorText(e));
      }
    })();
  }, [server.id, report]);

  const set = <K extends keyof typeof s>(k: K, v: (typeof s)[K]) => setS((x) => ({ ...x, [k]: v }));
  const err = (f: string) => (fieldError?.field === f ? fieldError.message : undefined);
  const running = job?.status === "running";

  async function submit(e: FormEvent) {
    e.preventDefault();
    setFieldError(null);
    const body: ApplySettings = {
      method: s.method, backend: s.backend, mode: s.backend === "external" ? s.mode : "udp",
      listenPort: s.listenPort || undefined, obfProfile: s.obfProfile, obfKey: s.obfKey || undefined,
      host: s.host || undefined, update: s.update || undefined,
    };
    if (s.backend === "external") body.connect = s.connect;
    else Object.assign(body, { wgPort: s.wgPort || undefined, wgNet: s.wgNet || undefined });
    try {
      const started = await api<Job>("POST", `/api/servers/${server.id}/apply`, body);
      setJob(started);
      const done = await waitJob(started.id, setJob);
      if (done.status === "ok" && done.result?.owner?.link) { setOwner(done.result.owner.link); setShowOwner(true); }
    } catch (e2) {
      report(server.id, e2);
      setFieldError(e2 instanceof ApiError ? { field: e2.field, message: e2.message } : { message: errorText(e2) });
    }
  }

  if (loadError) return <div className="alert alert-err">{loadError}</div>;
  if (!probe) return <div className="empty">Подключаюсь по SSH…</div>;
  const installed = probe.installed;

  return (
    <form className="card card-body form" onSubmit={submit} noValidate>
      <div className="row">
        <Field label="Запуск">
          <Segmented value={s.method} options={[["docker", "Docker"], ["systemd", "systemd"]]} onChange={(v) => set("method", v)} />
        </Field>
        <Field label="Куда идёт трафик">
          <Segmented value={s.backend} options={[["new", "Свой WireGuard"], ["external", "Мой VPN"]]} onChange={(v) => set("backend", v)} />
        </Field>
      </div>
      {s.backend === "external" ? (
        <div className="row">
          <Field label="Адрес VPN" hint="Amnezia, wg-easy, 3x-ui: host:port на этом сервере или другом" error={err("connect")}>
            <input className="input mono" value={s.connect} onChange={(e) => set("connect", e.target.value)} placeholder="127.0.0.1:51820" spellCheck={false} />
          </Field>
          <Field label="Протокол VPN" error={err("mode")}>
            <Segmented value={s.mode} options={[["udp", "UDP · WireGuard"], ["tcp", "TCP · Xray/VLESS"]]} onChange={(v) => set("mode", v)} />
          </Field>
        </div>
      ) : (
        <div className="row">
          <Field label="Порт WireGuard" hint="Наружу не открывается" error={err("wgPort")}>
            <input className="input mono" value={s.wgPort || ""} placeholder="по умолчанию" inputMode="numeric"
              onChange={(e) => set("wgPort", Number(e.target.value.replace(/\D/g, "")) || 0)} />
          </Field>
          <Field label="Подсеть WireGuard" hint="От /16 до /29" error={err("wgNet")}>
            <input className="input mono" value={s.wgNet} placeholder="по умолчанию" onChange={(e) => set("wgNet", e.target.value)} spellCheck={false} />
          </Field>
        </div>
      )}
      <div className="row">
        <Field label="Порт FreeTurn" hint="UDP, открывается наружу" error={err("listenPort")}>
          <input className="input mono" value={s.listenPort || ""} inputMode="numeric"
            onChange={(e) => set("listenPort", Number(e.target.value.replace(/\D/g, "")) || 0)} />
        </Field>
        <Field label="Адрес в ссылках" hint="Пусто — внешний IP сервера" error={err("host")}>
          <input className="input mono" value={s.host} placeholder={server.host} onChange={(e) => set("host", e.target.value)} spellCheck={false} />
        </Field>
      </div>
      <div className="row">
        <Field label="Обфускация" error={err("obfProfile")}>
          <select className="select" value={s.obfProfile} onChange={(e) => set("obfProfile", e.target.value)}>
            <option value="rtpopus3">RTP Opus v3 (рекомендуется)</option>
            <option value="rtpopus2">RTP Opus v2</option>
            <option value="rtpopus">RTP Opus</option>
            <option value="none">Выключена</option>
          </select>
        </Field>
        <Field label="Ключ обфускации" hint="Смена ключа требует обновить ссылки у всех пользователей" error={err("obfKey")}>
          <div className="input-row">
            <input className="input mono" value={s.obfKey} onChange={(e) => set("obfKey", e.target.value.trim())} spellCheck={false} disabled={s.obfProfile === "none"} />
            <button type="button" className="btn" disabled={s.obfProfile === "none"}
              onClick={async () => set("obfKey", (await api<{ obfKey: string }>("GET", "/api/defaults")).obfKey)}>Новый</button>
          </div>
        </Field>
      </div>
      {installed && (
        <label className="check">
          <input type="checkbox" checked={s.update} onChange={(e) => set("update", e.target.checked)} />
          <span>Обновить FreeTurn до последней версии</span>
        </label>
      )}
      {fieldError && !fieldError.field && <div className="alert alert-err">{fieldError.message}</div>}
      {job && job.status !== "running" && (
        job.status === "ok"
          ? <div className="alert alert-ok">{installed ? "Настройки применены." : "FreeTurn установлен."} Ссылка владельца — ниже.</div>
          : <div className="alert alert-err">
              {job.error}{job.stage && ` (этап: ${STAGES[job.stage] ?? job.stage})`}
              {!!job.logs?.length && <pre className="log log-small">{job.logs.join("\n")}</pre>}
            </div>
      )}
      <div className="actions">
        {owner && <button type="button" className="btn" onClick={() => setShowOwner(true)}>Ссылка владельца</button>}
        <button className="btn btn-primary" disabled={running}>
          {running ? (installed ? "Применяю… это займёт до пары минут" : "Устанавливаю… это займёт несколько минут")
            : installed ? "Применить" : "Установить"}
        </button>
      </div>
      {showOwner && <LinkModal title="Доступ владельца" link={owner} serverId={server.id} fileName={`${server.name}-owner`} onClose={() => setShowOwner(false)} />}
    </form>
  );
}
