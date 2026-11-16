import { Fragment, useCallback, useEffect, useState, type FormEvent } from "react";
import { Icon } from "../components/Icon";
import { Modal } from "../components/Modal";
import { api, ApiError, canManageAdmins, ROLE_NAMES, type AdminAccount, type Role } from "../lib/api";
import { formatDate, timeAgo } from "../lib/format";
import { errorText } from "../lib/serverApi";
import { useSession } from "../lib/session";

const ROLE_HINTS: Record<Role, string> = {
  root: "Всё; удалить или понизить нельзя",
  admin: "Всё, включая учётки админки",
  operator: "Серверы и их пользователи; учётки админки — только просмотр",
};

function useFieldError() {
  const [e, setE] = useState<{ field?: string; message: string } | null>(null);
  const capture = (err: unknown) => setE(err instanceof ApiError ? { field: err.field, message: err.message } : { message: errorText(err) });
  const of = (f: string) => (e?.field === f ? e.message : undefined);
  return { error: e, setError: setE, capture, of };
}

function NewAdminForm({ onDone }: { onDone: () => void }) {
  const [login, setLogin] = useState("");
  const [password, setPassword] = useState("");
  const [role, setRole] = useState<Role>("operator");
  const [busy, setBusy] = useState(false);
  const f = useFieldError();
  async function submit(e: FormEvent) {
    e.preventDefault();
    f.setError(null);
    setBusy(true);
    try {
      await api("POST", "/api/admins", { login: login.trim(), password, role });
      onDone();
    } catch (err) {
      f.capture(err);
    } finally {
      setBusy(false);
    }
  }
  return (
    <form className="form" onSubmit={submit} noValidate>
      <label className="field">
        <span className="label">Логин</span>
        <input className="input" value={login} onChange={(e) => setLogin(e.target.value)} autoComplete="off" autoFocus aria-invalid={f.of("login") ? true : undefined} />
        {f.of("login") && <span className="field-error">{f.of("login")}</span>}
      </label>
      <label className="field">
        <span className="label">Пароль</span>
        <input className="input" type="password" value={password} onChange={(e) => setPassword(e.target.value)} autoComplete="new-password" aria-invalid={f.of("password") ? true : undefined} />
        {f.of("password") ? <span className="field-error">{f.of("password")}</span> : <span className="hint">Не короче 10 символов — передайте его человеку сами</span>}
      </label>
      <div className="field">
        <span className="label">Роль</span>
        <div className="segmented" role="group">
          {(["operator", "admin"] as Role[]).map((r) => (
            <button key={r} type="button" aria-pressed={role === r} onClick={() => setRole(r)}>{ROLE_NAMES[r]}</button>
          ))}
        </div>
        <span className="hint">{ROLE_HINTS[role]}</span>
      </div>
      {f.error && !f.error.field && <div className="alert alert-err">{f.error.message}</div>}
      <div className="actions"><button className="btn btn-primary" disabled={busy}>{busy ? "Создаю…" : "Создать"}</button></div>
    </form>
  );
}

function PasswordForm({ onDone }: { onDone: () => void }) {
  const [current, setCurrent] = useState("");
  const [next, setNext] = useState("");
  const [busy, setBusy] = useState(false);
  const f = useFieldError();
  async function submit(e: FormEvent) {
    e.preventDefault();
    f.setError(null);
    setBusy(true);
    try {
      await api("POST", "/api/auth/password", { current, new: next });
      onDone();
    } catch (err) {
      f.capture(err);
    } finally {
      setBusy(false);
    }
  }
  return (
    <form className="form" onSubmit={submit} noValidate>
      <label className="field">
        <span className="label">Текущий пароль</span>
        <input className="input" type="password" value={current} onChange={(e) => setCurrent(e.target.value)} autoComplete="current-password" autoFocus />
        {f.of("current") && <span className="field-error">{f.of("current")}</span>}
      </label>
      <label className="field">
        <span className="label">Новый пароль</span>
        <input className="input" type="password" value={next} onChange={(e) => setNext(e.target.value)} autoComplete="new-password" />
        {f.of("password") ? <span className="field-error">{f.of("password")}</span> : <span className="hint">Не короче 10 символов. Другие ваши входы будут завершены.</span>}
      </label>
      {f.error && !f.error.field && <div className="alert alert-err">{f.error.message}</div>}
      <div className="actions"><button className="btn btn-primary" disabled={busy}>{busy ? "Сохраняю…" : "Сменить пароль"}</button></div>
    </form>
  );
}

export function AdminsPage() {
  const me = useSession((s) => s.me);
  const manage = canManageAdmins(me?.role);
  const [list, setList] = useState<AdminAccount[] | null>(null);
  const [error, setError] = useState("");
  const [modal, setModal] = useState<"" | "new" | "password">("");
  const [removing, setRemoving] = useState<AdminAccount | null>(null);
  const [done, setDone] = useState("");

  const refresh = useCallback(() => {
    api<AdminAccount[]>("GET", "/api/admins").then(setList).catch((e) => setError(errorText(e)));
  }, []);
  useEffect(refresh, [refresh]);

  async function setRole(a: AdminAccount, role: Role) {
    try {
      await api("PATCH", `/api/admins/${a.id}`, { role });
      refresh();
    } catch (e) {
      setError(errorText(e));
    }
  }

  async function doRemove() {
    const a = removing!;
    setRemoving(null);
    try {
      await api("DELETE", `/api/admins/${a.id}`);
      refresh();
    } catch (e) {
      setError(errorText(e));
    }
  }

  return (
    <div className="page">
      <div className="page-header">
        <div>
          <h1 className="page-title">Учётки админки</h1>
          <p className="page-subtitle">{manage ? "Кто может входить в админку и что ему разрешено" : "Вы можете смотреть список; менять учётки может администратор"}</p>
        </div>
        <div className="actions">
          <button className="btn" onClick={() => setModal("password")}>Сменить мой пароль</button>
          {manage && <button className="btn btn-primary" onClick={() => setModal("new")}><Icon name="plus" />Добавить</button>}
        </div>
      </div>
      {error && <div className="alert alert-err">{error}</div>}
      {done && <div className="alert alert-ok">{done}</div>}
      <div className="table-wrap">
        <table className="table">
          <thead><tr><th>Логин</th><th>Роль</th><th className="hide-sm">Последний вход</th><th className="hide-sm">Создана</th><th /></tr></thead>
          <tbody>
            {list?.map((a, i) => {
              const editable = manage && a.role !== "root" && a.login !== me?.login;
              return (
                <tr key={a.id} style={{ animationDelay: `${i * 25}ms` }}>
                  <td style={{ fontWeight: 500 }}>{a.login}{a.login === me?.login && <span className="cell-sub"> · вы</span>}</td>
                  <td>
                    {editable ? (
                      <select className="select select-inline" value={a.role} onChange={(e) => setRole(a, e.target.value as Role)}>
                        <option value="admin">{ROLE_NAMES.admin}</option>
                        <option value="operator">{ROLE_NAMES.operator}</option>
                      </select>
                    ) : <span className="badge">{ROLE_NAMES[a.role] ?? a.role}</span>}
                  </td>
                  <td className="hide-sm cell-sub">{timeAgo(a.lastLoginAt)}</td>
                  <td className="hide-sm cell-sub">{formatDate(a.createdAt)}</td>
                  <td style={{ textAlign: "right" }}>{editable && <button className="btn btn-danger" onClick={() => setRemoving(a)}>Удалить</button>}</td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
      <div className="card card-body">
        <dl className="kv">
          {(Object.keys(ROLE_NAMES) as Role[]).map((r) => <Fragment key={r}><dt>{ROLE_NAMES[r]}</dt><dd>{ROLE_HINTS[r]}</dd></Fragment>)}
        </dl>
      </div>
      {modal === "new" && <Modal title="Новая учётка" onClose={() => setModal("")}><NewAdminForm onDone={() => { setModal(""); refresh(); }} /></Modal>}
      {modal === "password" && <Modal title="Сменить мой пароль" onClose={() => setModal("")}><PasswordForm onDone={() => { setModal(""); setDone("Пароль изменён."); }} /></Modal>}
      {removing && (
        <Modal title={`Удалить «${removing.login}»?`} onClose={() => setRemoving(null)}>
          <div className="form">
            <p style={{ margin: 0 }}>Учётка исчезнет, её входы сразу завершатся. Серверы и их пользователи не затрагиваются.</p>
            <div className="actions">
              <button className="btn" onClick={() => setRemoving(null)}>Отмена</button>
              <button className="btn btn-primary btn-destructive" onClick={doRemove}>Удалить</button>
            </div>
          </div>
        </Modal>
      )}
    </div>
  );
}
