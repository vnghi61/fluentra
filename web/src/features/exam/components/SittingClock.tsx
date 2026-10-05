import React, { useSyncExternalStore } from "react";
import { Clock } from "lucide-react";
import { useTranslation } from "react-i18next";
import { cn } from "@/lib/utils";

let seconds = 0;
const listeners = new Set<() => void>();
let sectionSeconds: number | undefined;
const sectionListeners = new Set<() => void>();
export function setSittingClock(value: number): void {
  seconds = value;
  listeners.forEach((listener) => listener());
}
export function setSectionClock(value: number | undefined): void {
  sectionSeconds = value;
  sectionListeners.forEach((listener) => listener());
}
export function SectionClock({
  format,
  label,
}: {
  format: (seconds: number) => string;
  label: (time: string) => string;
}): React.JSX.Element | null {
  const value = useSyncExternalStore(
    (listener) => {
      sectionListeners.add(listener);
      return () => sectionListeners.delete(listener);
    },
    () => sectionSeconds,
    () => sectionSeconds,
  );
  return value === undefined ? null : (
    <span className="font-mono">{label(format(value))}</span>
  );
}
function subscribe(listener: () => void): () => void {
  listeners.add(listener);
  return () => listeners.delete(listener);
}
function useSeconds(): number {
  return useSyncExternalStore(
    subscribe,
    () => seconds,
    () => seconds,
  );
}
export function formatClock(totalSeconds: number): string {
  const safe = Math.max(0, Math.floor(totalSeconds));
  const hours = Math.floor(safe / 3600);
  const minutes = Math.floor((safe % 3600) / 60)
    .toString()
    .padStart(2, "0");
  const secs = (safe % 60).toString().padStart(2, "0");
  return hours > 0 ? `${hours}:${minutes}:${secs}` : `${minutes}:${secs}`;
}
export function SittingClock({
  unlimited,
  duration,
}: {
  unlimited: boolean;
  duration: number;
}): React.JSX.Element {
  const { t } = useTranslation();
  const remaining = useSeconds();
  const value = unlimited ? duration - remaining : remaining;
  const label = unlimited
    ? t("exam.runner.timeElapsed")
    : t("exam.runner.timeLeft");
  return (
    <div
      className={cn(
        "flex items-center gap-2 rounded-lg border px-3 py-1.5 font-mono text-sm font-bold tabular-nums sm:text-base",
        value <= 60 && !unlimited
          ? "border-danger/30 bg-danger/10 text-danger-accent"
          : value <= 300 && !unlimited
            ? "border-warning/30 bg-warning/10 text-warning-accent"
            : "border-border-subtle bg-surface-muted text-text",
      )}
      role="timer"
      aria-label={label}
    >
      <Clock className="h-4 w-4 shrink-0" aria-hidden="true" />
      <span>{formatClock(value)}</span>
    </div>
  );
}
