/** RFC 9457 Problem Details, the only error shape this API returns. */
export interface ProblemDetails {
  type?: string | undefined;
  title: string;
  status: number;
  detail?: string | undefined;
  instance?: string | undefined;
  code?: string | undefined;
  /** Set by the API on every problem; what support looks a request up by. */
  request_id?: string | undefined;
  errors?: Record<string, string[]> | undefined;
  meta?: Record<string, unknown> | undefined;
}

export const ERROR_MESSAGES: Record<string, string> = {
  // The rate limiter (internal/shared/httpx/ratelimit.go), for any endpoint.
  RATE_LIMITED:
    "You have tried too many times in a short while. Please wait a few minutes and try again.",

  // Session & token lifecycle
  SESSION_ABSOLUTE_EXPIRED:
    "Your session reached its maximum lifetime. Please sign in again to continue learning.",
  SESSION_REVOKED:
    "Your session was revoked or signed out from another device. Please sign in again.",
  TOKEN_INVALID: "Your session credential is no longer valid. Please sign in.",

  // Account status
  EMAIL_NOT_VERIFIED:
    "Your email address has not been verified yet. Please check your inbox for the verification code.",
  ACCOUNT_SUSPENDED:
    "This account has been suspended. Please contact support if you believe this is an error.",
  ACCOUNT_LOCKED:
    "Too many failed sign-in attempts. Your account is temporarily locked. Please try again later.",
  INVALID_CREDENTIALS: "The email or password you entered is incorrect.",

  // Google OAuth
  OAUTH_ACCOUNT_CONFLICT:
    "An account with this email already exists but has not completed email verification. Please verify your email using the OTP code sent to your inbox before linking Google sign-in.",
  OAUTH_EMAIL_UNVERIFIED:
    "Google reports that this email address is not verified. Please verify your email with Google first.",
  OAUTH_STATE_INVALID:
    "The authentication state was invalid or expired. Please start the sign-in process again.",
  OAUTH_EMAIL_MISMATCH:
    "The Google account email does not match your current account email address.",
  OAUTH_ALREADY_LINKED:
    "This Google identity is already linked to another account.",
  LAST_SIGN_IN_METHOD:
    "Cannot unlink Google: it is currently the only sign-in method for this account. Set a password first.",

  // Challenges (OTP).
  //
  // These are the codes `internal/modules/auth/domain/errors.go` actually
  // emits. The catalogue used to carry CODE_INVALID, CHALLENGE_BURNED and
  // CHALLENGE_EXPIRED, none of which the server has ever sent — so every OTP
  // refusal fell through to the RFC 9457 `title` ("Authentication required",
  // "Rate limited") and the burned state was unreachable in the UI.
  // catalogue.test.ts now fails if a code drifts like that again.
  OTP_INVALID: "The code you entered is incorrect.",
  OTP_EXPIRED: "This verification code has expired. Please request a new code.",
  OTP_ATTEMPTS_EXCEEDED:
    "Too many incorrect attempts. This code has been invalidated. Please request a new one.",
  OTP_ALREADY_USED:
    "This verification code has already been used. Please request a new one.",
  OTP_RESEND_TOO_SOON:
    "A new code was sent recently. Please wait a moment before asking for another.",
  OTP_ISSUE_LIMIT_REACHED:
    "Too many codes have been requested from here. Please wait a while and try again.",
  CHALLENGE_NOT_FOUND:
    "This verification step is no longer available. Please start again.",

  // Registration & credentials
  EMAIL_ALREADY_REGISTERED:
    "An account with this email address already exists. Try signing in instead.",
  PASSWORD_TOO_WEAK:
    "That password does not meet the password policy. Use at least 12 characters and something not easily guessed.",
  DISPLAY_NAME_NOT_ALLOWED: "Please choose a different display name.",

  // Administration
  SELF_ADMIN_ACTION_FORBIDDEN:
    "You cannot perform this administrative action on your own account.",
  REASON_REQUIRED:
    "A reason of at least 10 characters is required for this action.",
  LAST_ADMIN_PROTECTED:
    "This is the last administrator; the role cannot be removed.",

  // Account data
  EXPORT_ALREADY_PENDING:
    "An export is already being prepared for your account. You will be emailed when it is ready.",
  DELETION_ALREADY_PENDING:
    "Your account is already scheduled for deletion. You can cancel it from this page.",
  DELETION_NOT_CANCELLABLE:
    "This deletion can no longer be cancelled because erasure has already begun.",
  ACCOUNT_NOT_USABLE:
    "This action is not available while your account is in its current state.",
};

/**
 * The i18next `t` a caller passes in: lib may not import i18n (eslint
 * boundaries), so the screen that shows the message lends its translator.
 */
export type Translate = (
  key: string,
  options: { count?: number | undefined; defaultValue: string },
) => string;

/** How long the server asked the caller to wait, in whole minutes (at least 1). */
export function retryAfterMinutes(problem: ProblemDetails): number | undefined {
  const seconds = Number(problem.meta?.["retry_after_seconds"]);
  return Number.isFinite(seconds) && seconds > 0
    ? Math.max(1, Math.ceil(seconds / 60))
    : undefined;
}

/**
 * The message a learner reads for a refused request, in their language.
 *
 * A 429 without a catalogued code is the rate limiter's (RATE_LIMITED): it
 * used to fall through to the RFC 9457 title, so a learner who signed in too
 * often read "Rate limited" in English and was not told for how long.
 */
export function getErrorMessage(
  problem: ProblemDetails,
  t?: Translate,
): string {
  const code =
    problem.code ?? (problem.status === 429 ? "RATE_LIMITED" : undefined);
  if (code) {
    const minutes = retryAfterMinutes(problem);
    const fallback = ERROR_MESSAGES[code];
    if (fallback) {
      const timed = minutes !== undefined && TIMED_CODES.has(code);
      const key = timed ? `errors.${code}_timed` : `errors.${code}`;
      const english = timed ? (TIMED_MESSAGES[code] ?? fallback) : fallback;
      return t
        ? t(key, { count: minutes, defaultValue: english })
        : english.replace("{{count}}", String(minutes));
    }
    console.warn(
      `[ErrorCatalogue] Uncatalogued error code received: "${code}". Falling back to title.`,
    );
  }

  return problem.title || "An unexpected error occurred. Please try again.";
}

/** Codes whose refusal is temporary and comes with a Retry-After. */
const TIMED_CODES = new Set([
  "RATE_LIMITED",
  "ACCOUNT_LOCKED",
  "OTP_ISSUE_LIMIT_REACHED",
]);

const TIMED_MESSAGES: Record<string, string> = {
  RATE_LIMITED:
    "You have tried too many times. Please wait {{count}} minute(s) and try again.",
  ACCOUNT_LOCKED:
    "Too many failed sign-in attempts. Your account is locked for {{count}} minute(s); please try again then.",
  OTP_ISSUE_LIMIT_REACHED:
    "Too many codes have been requested from here. Please wait {{count}} minute(s) and try again.",
};
