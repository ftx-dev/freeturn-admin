import { useState, type FormEvent } from "react";
import { ApiError, type Server, type ServerInput } from "../lib/api";

// Форма сервера: добавление (все поля) и правка (секреты пустые = «не менять»).
export function ServerForm({ server, submitLabel, onSubmit, onCancel }: {
  server?: Server;
  submitLabel: string;
  onSubmit: (input: ServerInput) => Promise<void>;
  onCancel?: () => void;
}) {
  const editing = !!server;
  const [name, setName] = useState(server?.name ?? "");
  const [host, setHost] = useState(server?.host ?? "");
  const [port, setPort] = useState(String(server?.sshPort ?? 22));
  const [user, setUser] = useState(server?.sshUser ?? "root");
  const [authType, setAuthType] = useState<"password" | "key">(server?.authType ?? "password");
  const [password, setPassword] = useState("");
  const [privateKey, setPrivateKey] = useState("");
  const [keyPassphrase, setKeyPassphrase] = useState("");
  const [sudoPassword, setSudoPassword] = useState("");
  const [error, setError] = useState<{ field?: string; message: string } | null>(null);
  const [busy, setBusy] = useState(false);

  const keep = editing ? "Оставьте пустым, чтобы не менять" : undefined;
  const authChanged = editing && authType !== server.authType;

  async function submit(e: FormEvent) {
    e.preventDefault();
    setError(null);
    const input: ServerInput = { name, host, sshPort: Number(port), sshUser: user, authType };
    if (authType === "password" && password) input.password = password;
    if (authType === "key" && privateKey) input.privateKey = privateKey;
    if (authType === "key" && keyPassphrase) input.keyPassphrase = keyPassphrase;
    if (sudoPassword) input.sudoPassword = sudoPassword;
    setBusy(true);
    try {
      await onSubmit(input);
    } catch (err) {
      setError(err instanceof ApiError ? { field: err.field, message: err.message } : { message: String(err) });
    } finally {
      setBusy(false);
    }
  }

  const invalid = (f: string) => (error?.field === f ? true : undefined);
  const errorFor = (f: string) => error?.field === f && <span className="field-error">{error.message}</span>;

  return (
    <form className="form" onSubmit={submit} noValidate>
      <label className="field">
        <span className="label">Название</span>
        <input className="input" value={name} onChange={(e) => setName(e.target.value)} placeholder="Нидерланды" aria-invalid={invalid("name")} autoFocus={!editing} />
        {errorFor("name")}
      </label>
      <div className="row-3">
        <label className="field">
          <span className="label">Адрес</span>
          <input className="input mono" value={host} onChange={(e) => setHost(e.target.value)} placeholder="203.0.113.10" aria-invalid={invalid("host")} autoComplete="off" spellCheck={false} />
          {errorFor("host")}
        </label>
        <label className="field">
          <span className="label">Порт SSH</span>
          <input className="input mono" value={port} onChange={(e) => setPort(e.target.value.replace(/\D/g, ""))} inputMode="numeric" aria-invalid={invalid("sshPort")} />
          {errorFor("sshPort")}
        </label>
        <label className="field">
          <span className="label">Пользователь</span>
          <input className="input mono" value={user} onChange={(e) => setUser(e.target.value)} aria-invalid={invalid("sshUser")} autoComplete="off" spellCheck={false} />
          {errorFor("sshUser")}
        </label>
      </div>
      <div className="field">
        <span className="label">Вход</span>
        <div className="segmented" role="group" aria-label="Способ входа">
          <button type="button" aria-pressed={authType === "password"} onClick={() => setAuthType("password")}>Пароль</button>
          <button type="button" aria-pressed={authType === "key"} onClick={() => setAuthType("key")}>Ключ SSH</button>
        </div>
      </div>
      {authType === "password" ? (
        <label className="field">
          <span className="label">Пароль SSH</span>
          <input className="input" type="password" value={password} onChange={(e) => setPassword(e.target.value)}
            placeholder={authChanged ? undefined : keep} aria-invalid={invalid("password")} autoComplete="new-password" />
          {errorFor("password")}
        </label>
      ) : (
        <>
          <label className="field">
            <span className="label">Приватный ключ</span>
            <textarea className="textarea" value={privateKey} onChange={(e) => setPrivateKey(e.target.value)} spellCheck={false}
              placeholder={authChanged || !editing ? "-----BEGIN OPENSSH PRIVATE KEY-----" : keep} aria-invalid={invalid("privateKey")} />
            {errorFor("privateKey")}
          </label>
          <label className="field">
            <span className="label">Пароль ключа</span>
            <input className="input" type="password" value={keyPassphrase} onChange={(e) => setKeyPassphrase(e.target.value)}
              placeholder={server?.hasKeyPassphrase ? keep : "Если ключ зашифрован"} autoComplete="new-password" />
          </label>
        </>
      )}
      {user !== "root" && (
        <label className="field">
          <span className="label">Пароль sudo</span>
          <input className="input" type="password" value={sudoPassword} onChange={(e) => setSudoPassword(e.target.value)}
            placeholder={server?.hasSudoPassword ? keep : "Не нужен, если sudo без пароля"} autoComplete="new-password" />
        </label>
      )}
      <span className="hint">Пароли и ключи хранятся в базе админки только зашифрованными.</span>
      {error && !error.field && <div className="alert alert-err">{error.message}</div>}
      <div className="actions">
        {onCancel && <button type="button" className="btn" onClick={onCancel}>Отмена</button>}
        <button className="btn btn-primary" disabled={busy}>{busy ? "Сохраняю…" : submitLabel}</button>
      </div>
    </form>
  );
}
