import type { SessionSummary, TrustedDevice } from "../api/accountApi";

/**
 * One row of "Devices & sign-ins": everything the API knows about one device.
 *
 * The server keeps one session per sign-in and one trusted-device record per
 * browser that chose "stay signed in", and the two lists used to be rendered
 * separately. A laptop signed in five times and trusted once therefore showed
 * up six times. Neither list carries a stable device identifier the other one
 * shares (sessions expose no `trusted_device_id`), so the coarse label the
 * server derives from the user agent — "Chrome on Windows" — is the grouping
 * key. Two physical machines with an identical label are shown as one device
 * with their sessions together, which the row says by counting them.
 */
export interface DeviceGroup {
  key: string;
  label: string | null;
  /** Sessions on this device, the caller's own first, then most recent. */
  sessions: SessionSummary[];
  /** Trust records for this device (usually zero or one). */
  trusted: TrustedDevice[];
  /** The caller is reading the page on this device. */
  current: boolean;
  /** Latest activity across its sessions and trust records (ISO string). */
  lastSeenAt: string;
}

const UNKNOWN_KEY = "\u0000unknown";

function keyOf(label: string | null | undefined): string {
  const normalised = label?.trim().toLowerCase();
  return normalised ? normalised : UNKNOWN_KEY;
}

function time(iso: string): number {
  const value = Date.parse(iso);
  return Number.isNaN(value) ? 0 : value;
}

export function groupDeviceSessions(
  sessions: readonly SessionSummary[],
  devices: readonly TrustedDevice[],
): DeviceGroup[] {
  const groups = new Map<string, DeviceGroup>();

  const groupFor = (label: string | null | undefined): DeviceGroup => {
    const key = keyOf(label);
    let group = groups.get(key);
    if (!group) {
      group = {
        key,
        label: label?.trim() || null,
        sessions: [],
        trusted: [],
        current: false,
        lastSeenAt: "",
      };
      groups.set(key, group);
    }
    return group;
  };

  // A session id appears once even if the API ever repeated a row.
  const seenSessions = new Set<string>();
  for (const session of sessions) {
    if (seenSessions.has(session.id)) continue;
    seenSessions.add(session.id);
    const group = groupFor(session.device_label);
    group.sessions.push(session);
    if (session.current) group.current = true;
    if (time(session.last_seen_at) > time(group.lastSeenAt)) {
      group.lastSeenAt = session.last_seen_at;
    }
  }

  const seenDevices = new Set<string>();
  for (const device of devices) {
    if (seenDevices.has(device.id)) continue;
    seenDevices.add(device.id);
    const group = groupFor(device.label);
    group.trusted.push(device);
    if (device.current) group.current = true;
    if (time(device.last_seen_at) > time(group.lastSeenAt)) {
      group.lastSeenAt = device.last_seen_at;
    }
  }

  for (const group of groups.values()) {
    group.sessions.sort((a, b) => {
      if (a.current !== b.current) return a.current ? -1 : 1;
      return time(b.last_seen_at) - time(a.last_seen_at);
    });
  }

  return [...groups.values()].sort((a, b) => {
    if (a.current !== b.current) return a.current ? -1 : 1;
    return time(b.lastSeenAt) - time(a.lastSeenAt);
  });
}
