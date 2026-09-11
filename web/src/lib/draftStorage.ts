const DRAFT_PREFIX = "fluentra_writing_draft_";

function getDraftKey(userId: string, activityId: string): string {
  return `${DRAFT_PREFIX}${userId}_${activityId}`;
}

export function getWritingDraft(userId: string, activityId: string): string {
  if (typeof window === "undefined" || !userId || !activityId) return "";
  try {
    return window.localStorage.getItem(getDraftKey(userId, activityId)) ?? "";
  } catch {
    return "";
  }
}

export function saveWritingDraft(
  userId: string,
  activityId: string,
  text: string,
): void {
  if (typeof window === "undefined" || !userId || !activityId) return;
  try {
    const key = getDraftKey(userId, activityId);
    if (!text.trim()) {
      window.localStorage.removeItem(key);
    } else {
      window.localStorage.setItem(key, text);
    }
  } catch {
    // Ignore storage quota or access errors in private browsing
  }
}

export function clearWritingDraft(userId: string, activityId: string): void {
  if (typeof window === "undefined" || !userId || !activityId) return;
  try {
    window.localStorage.removeItem(getDraftKey(userId, activityId));
  } catch {
    // Ignore
  }
}

export function clearAllWritingDrafts(userId?: string): void {
  if (typeof window === "undefined") return;
  try {
    const prefix = userId ? `${DRAFT_PREFIX}${userId}_` : DRAFT_PREFIX;
    const keysToRemove: string[] = [];
    for (let i = 0; i < window.localStorage.length; i++) {
      const key = window.localStorage.key(i);
      if (key && key.startsWith(prefix)) {
        keysToRemove.push(key);
      }
    }
    for (const key of keysToRemove) {
      window.localStorage.removeItem(key);
    }
  } catch {
    // Ignore
  }
}
