import type { GoogleLinkStatus } from "../api/accountApi";

export type SecuritySuggestion = "setPassword" | "connectGoogle";

export interface SecurityStatus {
  /** The account can sign in with a password. */
  hasPassword: boolean;
  googleLinked: boolean;
  /** What would add a second way in; empty when both methods are set up. */
  suggestions: SecuritySuggestion[];
}

/**
 * What the security overview may honestly say, from the Google link status.
 *
 * The API has no "has password" field, but it does not need one: an account
 * has exactly two sign-in methods, and `can_unlink` is false precisely when
 * Google is the only one (BR-AUTH-20). So an unlinked account signs in with a
 * password, and a linked one has a password exactly when it may unlink.
 * Nothing here judges password strength: the server does not report it.
 */
export function securityStatus(google: GoogleLinkStatus): SecurityStatus {
  const googleLinked = google.linked;
  const hasPassword = !googleLinked || google.can_unlink;
  const suggestions: SecuritySuggestion[] = [];
  if (!hasPassword) suggestions.push("setPassword");
  if (!googleLinked) suggestions.push("connectGoogle");
  return { hasPassword, googleLinked, suggestions };
}
