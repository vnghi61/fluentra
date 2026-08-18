/** RFC 9457 Problem Details, the only error shape this API returns. */
export interface ProblemDetails {
  type?: string | undefined;
  title: string;
  status: number;
  detail?: string | undefined;
  instance?: string | undefined;
  code?: string | undefined;
  errors?: Record<string, string[]> | undefined;
  meta?: Record<string, unknown> | undefined;
}

export const ERROR_MESSAGES: Record<string, string> = {
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

  // Challenges (OTP)
  CHALLENGE_EXPIRED:
    "This verification code has expired. Please request a new code.",
  CHALLENGE_BURNED:
    "Too many incorrect attempts. This challenge has been invalidated. Please request a new code.",
  CODE_INVALID: "The code you entered is incorrect.",
};

export function getErrorMessage(problem: ProblemDetails): string {
  if (problem.code) {
    const msg = ERROR_MESSAGES[problem.code];
    if (msg) {
      return msg;
    }
    console.warn(
      `[ErrorCatalogue] Uncatalogued error code received: "${problem.code}". Falling back to title.`,
    );
  }

  return problem.title || "An unexpected error occurred. Please try again.";
}
