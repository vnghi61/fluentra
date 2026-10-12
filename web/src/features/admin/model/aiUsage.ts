export type QuotaStatus = "normal" | "highlight" | "warning" | "critical";

export function quotaStatus(percent: number, exhausted = false): QuotaStatus {
  if (exhausted || percent > 95) return "critical";
  if (percent > 85) return "warning";
  if (percent > 70) return "highlight";
  return "normal";
}

export function quotaPercent(
  used: number,
  limit?: number | null,
): number | null {
  if (limit == null || limit <= 0) return null;
  return Math.min(100, Math.round((used / limit) * 100));
}
