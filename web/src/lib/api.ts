// Обёртка над fetch: JSON, CSRF-токен для изменяющих запросов, ошибки API как ApiError.
export class ApiError extends Error {
  constructor(public status: number, public code: string, message: string, public field?: string, public data?: any) {
    super(message);
  }
}

let csrf = "";
let onUnauthorized: () => void = () => {};

export function setCsrf(token: string) { csrf = token; }
export function setUnauthorizedHandler(fn: () => void) { onUnauthorized = fn; }

export async function api<T>(method: string, path: string, body?: unknown): Promise<T> {
  const headers: Record<string, string> = {};
  if (body !== undefined) headers["Content-Type"] = "application/json";
  if (method !== "GET" && csrf) headers["X-CSRF-Token"] = csrf;
  let res: Response;
  try {
    res = await fetch(path, { method, headers, body: body === undefined ? undefined : JSON.stringify(body), credentials: "same-origin" });
  } catch {
    throw new ApiError(0, "network", "Нет связи с сервером админки");
  }
  const data = res.headers.get("Content-Type")?.includes("json") ? await res.json() : null;
  if (!res.ok) {
    const e = data?.error ?? {};
    if (res.status === 401 && !path.startsWith("/api/auth/login")) onUnauthorized();
    throw new ApiError(res.status, e.code ?? "http", e.message ?? `Ошибка ${res.status}`, e.field, data);
  }
  return data as T;
}

export interface Me { login: string; role: string; csrf: string }

export interface Server {
  id: string;
  name: string;
  host: string;
  sshPort: number;
  sshUser: string;
  authType: "password" | "key";
  hostFingerprint: string;
  rootMode: string;
  createdAt: number;
  updatedAt: number;
  hasSecret: boolean;
  hasKeyPassphrase: boolean;
  hasSudoPassword: boolean;
}

export interface ServerInput {
  name?: string;
  host?: string;
  sshPort?: number;
  sshUser?: string;
  authType?: "password" | "key";
  password?: string;
  privateKey?: string;
  keyPassphrase?: string;
  sudoPassword?: string;
}

export interface AuditEntry {
  id: string;
  login: string;
  serverId: string;
  action: string;
  details: string;
  result: string;
  at: number;
}

export interface CoreConfig {
  method: string; backend: string; connect: string; host: string;
  listen_port: number; wg_port: number; wg_net: string; mode: string;
  obf_profile: string; obf_key: string; version: string;
}

export interface Probe {
  installed: boolean; running: boolean; euid: number; arch: string;
  method?: string; version?: string; config?: CoreConfig;
}

export interface ServerUser { name: string; client_id: string; self: boolean; ip?: string; pub?: string; hs?: number }

export interface UserList {
  clients: ServerUser[];
  share: { backend: string; host: string; port: number; mode: string; obf_profile: string; obf_key: string };
}

export interface ClientConf { client: ServerUser; conf_b64: string; link: string }

export interface ApplySettings {
  method?: string; backend?: string; connect?: string; mode?: string;
  listenPort?: number; wgPort?: number; wgNet?: string;
  obfProfile?: string; obfKey?: string; host?: string; update?: boolean;
}

export interface Job {
  id: string; serverId: string; kind: string; status: "running" | "ok" | "error";
  error?: string; errorCode?: string; stage?: string; logs?: string[];
  result?: { owner?: { client_id: string; link: string; conf_b64: string } } & Record<string, unknown>;
  startedAt: string; finishedAt?: string;
}

/** Ждёт окончания фоновой задачи (опрос раз в секунду). */
export async function waitJob(id: string, onUpdate?: (j: Job) => void): Promise<Job> {
  for (;;) {
    const j = await api<Job>("GET", `/api/jobs/${id}`);
    onUpdate?.(j);
    if (j.status !== "running") return j;
    await new Promise((r) => setTimeout(r, 1000));
  }
}
