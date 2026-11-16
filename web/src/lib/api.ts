// Обёртка над fetch: JSON, CSRF-токен для изменяющих запросов, ошибки API как ApiError.
export class ApiError extends Error {
  constructor(public status: number, public code: string, message: string, public field?: string) {
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
    throw new ApiError(res.status, e.code ?? "http", e.message ?? `Ошибка ${res.status}`, e.field);
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
