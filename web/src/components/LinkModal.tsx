import QRCode from "qrcode";
import { useEffect, useState } from "react";
import { api, ApiError } from "../lib/api";
import { Modal } from "./Modal";
import type { Call } from "./ServerCalls";


// Ссылка freeturn:// и QR - как экран «Поделиться» клиентов; по желанию с вложенной ссылкой на звонок.
export function LinkModal({ title, link: baseLink, fileName, serverId, onClose }: {
  title: string; link: string; fileName: string; serverId: string; onClose: () => void;
}) {
  const [withCall, setWithCall] = useState(false);
  const [call, setCall] = useState("");
  const [calls, setCalls] = useState<Call[]>([]);
  useEffect(() => {
    api<{ calls: Call[] }>("GET", `/api/servers/${serverId}/calls`).then((r) => setCalls(r.calls)).catch(() => {});
  }, [serverId]);
  const [callError, setCallError] = useState("");
  const [link, setLink] = useState(baseLink);
  const [qr, setQr] = useState("");
  const [copied, setCopied] = useState(false);

  // Пересобираем ссылку при включении/смене звонка (с небольшой паузой, пока человек вставляет ссылку).
  useEffect(() => {
    setCopied(false);
    if (!withCall || !call.trim()) {
      setLink(baseLink);
      setCallError("");
      return;
    }
    const t = setTimeout(async () => {
      try {
        const r = await api<{ link: string }>("POST", "/api/links/call", { link: baseLink, callLink: call.trim() });
        setLink(r.link);
        setCallError("");
      } catch (e) {
        setLink(baseLink);
        setCallError(e instanceof ApiError ? e.message : String(e));
      }
    }, 300);
    return () => clearTimeout(t);
  }, [withCall, call, baseLink]);

  useEffect(() => {
    QRCode.toDataURL(link, { errorCorrectionLevel: "M", margin: 2, width: 512 }).then(setQr).catch(() => setQr(""));
  }, [link]);

  async function copy() {
    try {
      await navigator.clipboard.writeText(link);
      setCopied(true);
    } catch { /* без https буфер обмена недоступен - ссылку можно выделить вручную */ }
  }

  const embedded = withCall && !!call.trim() && !callError && link !== baseLink;
  return (
    <Modal title={title} onClose={onClose}>
      <div className="form">
        {qr ? <img className="qr" src={qr} alt="QR-код ссылки" /> : <div className="qr" />}
        <label className="check">
          <input type="checkbox" checked={withCall} onChange={(e) => {
            setWithCall(e.target.checked);
            if (e.target.checked && !call && calls.length) setCall(calls[0].url); // первый звонок сервера - сразу
          }} />
          <span>Вложить ссылку на звонок</span>
        </label>
        {withCall && (
          <label className="field">
            {calls.length > 0 && (
              <div className="call-chips">
                {calls.map((c) => (
                  <button key={c.url} type="button" className="chip" aria-pressed={call === c.url} onClick={() => setCall(c.url)} title={c.url}>
                    {c.label || c.url.replace(/^https:\/\//, "")}
                  </button>
                ))}
              </div>
            )}
            <input className="input" value={call} onChange={(e) => setCall(e.target.value)}
              placeholder={calls.length ? "Или вставьте другую ссылку на звонок" : "https://vk.com/call/join/… — звонки сервера задаются в «Установка и настройки»"}
              spellCheck={false} autoFocus={!calls.length} aria-invalid={callError ? true : undefined} />
            {callError ? <span className="field-error">{callError}</span>
              : <span className="hint">{embedded ? "Звонок вложен: получателю не нужна своя ссылка на звонок." : "Вставьте ссылку на звонок для режима «через релей»."}</span>}
          </label>
        )}
        <textarea className="textarea link-box" readOnly value={link} onFocus={(e) => e.currentTarget.select()} />
        <span className="hint">Ссылка равносильна ключам: передавайте её только тому, кому выдаёте доступ.</span>
        <div className="actions">
          {qr && <a className="btn" href={qr} download={`${fileName}.png`}>Скачать QR</a>}
          <button className="btn btn-primary" onClick={copy}>{copied ? "Скопировано" : "Копировать ссылку"}</button>
        </div>
      </div>
    </Modal>
  );
}
