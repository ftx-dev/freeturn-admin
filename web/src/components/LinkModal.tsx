import QRCode from "qrcode";
import { useEffect, useState } from "react";
import { Modal } from "./Modal";

// Ссылка freeturn:// и QR - как экран «Поделиться» клиентов.
export function LinkModal({ title, link, fileName, onClose }: { title: string; link: string; fileName: string; onClose: () => void }) {
  const [qr, setQr] = useState("");
  const [copied, setCopied] = useState(false);
  useEffect(() => {
    QRCode.toDataURL(link, { errorCorrectionLevel: "M", margin: 2, width: 512 }).then(setQr).catch(() => setQr(""));
  }, [link]);

  async function copy() {
    try {
      await navigator.clipboard.writeText(link);
      setCopied(true);
    } catch { /* без https буфер обмена недоступен - ссылку можно выделить вручную */ }
  }

  return (
    <Modal title={title} onClose={onClose}>
      <div className="form">
        {qr ? <img className="qr" src={qr} alt="QR-код ссылки" /> : <div className="qr" />}
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
