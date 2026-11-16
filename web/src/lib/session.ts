import { create } from "zustand";
import { api, ApiError, setCsrf, setUnauthorizedHandler, type Me } from "./api";

type Status = "loading" | "setup" | "anon" | "authed" | "offline";

interface SessionState {
  status: Status;
  me: Me | null;
  load: () => Promise<void>;
  login: (login: string, password: string) => Promise<void>;
  setup: (login: string, password: string) => Promise<void>;
  logout: () => Promise<void>;
}

export const useSession = create<SessionState>((set, get) => {
  const signedIn = (me: Me) => {
    setCsrf(me.csrf);
    set({ status: "authed", me });
  };
  setUnauthorizedHandler(() => {
    if (get().status === "authed") set({ status: "anon", me: null });
  });
  return {
    status: "loading",
    me: null,
    async load() {
      try {
        const state = await api<{ needsSetup: boolean }>("GET", "/api/auth/state");
        if (state.needsSetup) return set({ status: "setup", me: null });
        signedIn(await api<Me>("GET", "/api/auth/me"));
      } catch (e) {
        set({ status: e instanceof ApiError && e.status === 401 ? "anon" : "offline", me: null });
      }
    },
    async login(login, password) {
      signedIn(await api<Me>("POST", "/api/auth/login", { login, password }));
    },
    async setup(login, password) {
      signedIn(await api<Me>("POST", "/api/auth/setup", { login, password }));
    },
    async logout() {
      try { await api("POST", "/api/auth/logout"); } catch { /* сессия уже недействительна */ }
      setCsrf("");
      set({ status: "anon", me: null });
    },
  };
});
