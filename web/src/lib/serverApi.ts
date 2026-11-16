import { create } from "zustand";
import { ApiError } from "./api";

// Смена ключа хоста ловится на любом запросе к серверу и показывается баннером на его странице.
interface HostKeyState {
  changed: Record<string, { want: string; got: string }>;
  report: (serverId: string, e: unknown) => void;
  clear: (serverId: string) => void;
}

export const useHostKey = create<HostKeyState>((set, get) => ({
  changed: {},
  report(serverId, e) {
    if (e instanceof ApiError && e.code === "host_key_changed") {
      set({ changed: { ...get().changed, [serverId]: { want: e.data?.want ?? "", got: e.data?.got ?? "" } } });
    }
  },
  clear(serverId) {
    const { [serverId]: _, ...rest } = get().changed;
    set({ changed: rest });
  },
}));

export const errorText = (e: unknown) => (e instanceof Error ? e.message : String(e));
