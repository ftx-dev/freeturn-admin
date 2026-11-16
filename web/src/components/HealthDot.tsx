import { healthTitle, useHealth } from "../lib/health";

export function HealthDot({ serverId }: { serverId: string }) {
  const h = useHealth((s) => s.byId[serverId]);
  return <span className={`health-dot health-${h?.state ?? "unknown"}`} title={healthTitle(h)} aria-label={healthTitle(h)} />;
}
