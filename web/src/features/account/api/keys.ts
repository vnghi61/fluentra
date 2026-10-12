/** Query keys for the account slice, in one place so invalidation is greppable. */
export const accountKeys = {
  all: ["account"] as const,
  sessions: () => [...accountKeys.all, "sessions"] as const,
  devices: () => [...accountKeys.all, "devices"] as const,
  googleLink: () => [...accountKeys.all, "googleLink"] as const,
};
