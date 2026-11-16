import { create } from "zustand";
import { api, type Server, type ServerInput } from "./api";

// Список серверов - общий для сайдбара, дашборда и страницы «Серверы».
interface ServersState {
  list: Server[];
  loaded: boolean;
  error: string;
  refresh: () => Promise<void>;
  create: (input: ServerInput) => Promise<Server>;
  update: (id: string, input: ServerInput) => Promise<Server>;
  remove: (id: string) => Promise<void>;
}

export const useServers = create<ServersState>((set, get) => ({
  list: [],
  loaded: false,
  error: "",
  async refresh() {
    try {
      set({ list: await api<Server[]>("GET", "/api/servers"), loaded: true, error: "" });
    } catch (e) {
      set({ loaded: true, error: (e as Error).message });
    }
  },
  async create(input) {
    const s = await api<Server>("POST", "/api/servers", input);
    set({ list: [...get().list, s] });
    return s;
  },
  async update(id, input) {
    const s = await api<Server>("PATCH", `/api/servers/${id}`, input);
    set({ list: get().list.map((x) => (x.id === id ? s : x)) });
    return s;
  },
  async remove(id) {
    await api("DELETE", `/api/servers/${id}`);
    set({ list: get().list.filter((x) => x.id !== id) });
  },
}));
