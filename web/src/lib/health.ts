import { create } from "zustand";
import { api } from "./api";

export interface Health { state: "unknown" | "up" | "down"; latencyMs?: number; error?: string }

// Доступность серверов: сервер админки кэширует и перепроверяет сам, здесь - опрос раз в 15 с.
interface HealthState {
  byId: Record<string, Health>;
  refresh: () => Promise<void>;
}

export const useHealth = create<HealthState>((set) => ({
  byId: {},
  async refresh() {
    try {
      set({ byId: await api<Record<string, Health>>("GET", "/api/servers/health") });
    } catch { /* сеть пропала - оставляем последнее известное */ }
  },
}));

export function healthTitle(h?: Health): string {
  if (!h || h.state === "unknown") return "Проверяю доступность…";
  if (h.state === "up") return `Доступен по SSH · ${h.latencyMs ?? 0} мс`;
  return `Не отвечает: ${h.error ?? "нет связи"}`;
}
