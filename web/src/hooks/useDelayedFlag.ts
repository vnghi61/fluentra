import { useCallback, useId, useSyncExternalStore } from "react";

interface DelayEntry {
  value: boolean;
  ready: boolean;
  timer?: number;
  listeners: Set<() => void>;
}

const entries = new Map<string, DelayEntry>();

export function useDelayedFlag(value: boolean, delayMs = 300): boolean {
  const id = useId();
  const getSnapshot = useCallback(() => {
    const entry = entries.get(id);
    return value && entry?.value === value && entry.ready;
  }, [id, value]);
  const subscribe = useCallback(
    (listener: () => void) => {
      const entry = entries.get(id) ?? {
        value,
        ready: false,
        listeners: new Set(),
      };
      if (entry.timer !== undefined) window.clearTimeout(entry.timer);
      entry.value = value;
      entry.ready = false;
      entry.listeners.add(listener);
      if (value) {
        entry.timer = window.setTimeout(() => {
          entry.ready = true;
          entry.listeners.forEach((notify) => notify());
        }, delayMs);
      }
      entries.set(id, entry);
      return () => {
        entry.listeners.delete(listener);
        if (entry.listeners.size === 0) {
          if (entry.timer !== undefined) window.clearTimeout(entry.timer);
          entries.delete(id);
        }
      };
    },
    [delayMs, id, value],
  );

  return useSyncExternalStore(subscribe, getSnapshot, () => false);
}
