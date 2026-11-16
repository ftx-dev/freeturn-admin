import { useState, type FormEvent } from "react";
import { ApiError } from "../lib/api";
import { useSession } from "../lib/session";

// Вход и первичная настройка (создание первого администратора) - одна форма.
export function AuthPage({ mode }: { mode: "login" | "setup" }) {
  const { login, setup } = useSession();
  const [name, setName] = useState("");
  const [password, setPassword] = useState("");
  const [repeat, setRepeat] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const isSetup = mode === "setup";

  async function submit(e: FormEvent) {
    e.preventDefault();
    setError("");
    if (isSetup && password !== repeat) return setError("Пароли не совпадают");
    setBusy(true);
    try {
      await (isSetup ? setup : login)(name.trim(), password);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : String(err));
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="auth-page">
      <div className="auth-card">
        <div className="auth-head">
          <img src="/favicon.png" alt="" />
          <h1>{isSetup ? "Добро пожаловать" : "FreeTurnX Admin"}</h1>
          <p>{isSetup ? "Создайте учётную запись администратора" : "Войдите, чтобы управлять серверами"}</p>
        </div>
        <form className="form" onSubmit={submit}>
          <label className="field">
            <span className="label">Логин</span>
            <input className="input" value={name} onChange={(e) => setName(e.target.value)} autoComplete="username" autoFocus required />
          </label>
          <label className="field">
            <span className="label">Пароль</span>
            <input className="input" type="password" value={password} onChange={(e) => setPassword(e.target.value)}
              autoComplete={isSetup ? "new-password" : "current-password"} required />
            {isSetup && <span className="hint">Не короче 10 символов</span>}
          </label>
          {isSetup && (
            <label className="field">
              <span className="label">Пароль ещё раз</span>
              <input className="input" type="password" value={repeat} onChange={(e) => setRepeat(e.target.value)} autoComplete="new-password" required />
            </label>
          )}
          {error && <div className="alert alert-err" role="alert">{error}</div>}
          <button className="btn btn-primary btn-block" disabled={busy}>
            {busy ? "Подождите…" : isSetup ? "Создать и войти" : "Войти"}
          </button>
        </form>
      </div>
    </div>
  );
}
