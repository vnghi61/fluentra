import * as React from "react";
import { AlertCircle, ArrowLeft, RotateCw } from "lucide-react";
import { Button } from "@/components/ui/button";
import { OtpInput } from "@/components/ui/otp-input";
import { authApi, type Challenge, type VerifiedChallenge } from "@/features/auth/api/authApi";
import { ApiError } from "@/api/client";
import { getErrorMessage } from "@/lib/errors/catalogue";

export interface OtpVerificationScreenProps {
  challenge: Challenge;
  email: string;
  onSuccess: (verified: VerifiedChallenge) => void;
  onBack?: (() => void) | undefined;
}

export function OtpVerificationScreen({
  challenge: initialChallenge,
  email,
  onSuccess,
  onBack,
}: OtpVerificationScreenProps): React.JSX.Element {
  const [challenge, setChallenge] = React.useState<Challenge>(initialChallenge);
  const [code, setCode] = React.useState("");
  const [isVerifying, setIsVerifying] = React.useState(false);
  const [isResending, setIsResending] = React.useState(false);
  const [error, setError] = React.useState<string | null>(null);
  const [attemptsRemaining, setAttemptsRemaining] = React.useState<number>(
    initialChallenge.attempts_remaining ?? 5,
  );
  const [isBurned, setIsBurned] = React.useState(false);

  // Time remaining until challenge expiry
  const [secondsUntilExpiry, setSecondsUntilExpiry] = React.useState<number>(() => {
    const diff = Math.floor(
      (new Date(initialChallenge.expires_at).getTime() - Date.now()) / 1000,
    );
    return Math.max(0, diff);
  });

  // Time remaining until resend cooldown ends
  const [secondsUntilResend, setSecondsUntilResend] = React.useState<number>(() => {
    const diff = Math.floor(
      (new Date(initialChallenge.resend_after).getTime() - Date.now()) / 1000,
    );
    return Math.max(0, diff);
  });

  // Countdown intervals
  React.useEffect(() => {
    const timer = setInterval(() => {
      setSecondsUntilExpiry((prev) => Math.max(0, prev - 1));
      setSecondsUntilResend((prev) => Math.max(0, prev - 1));
    }, 1000);

    return () => clearInterval(timer);
  }, []);

  const handleVerify = async (codeToVerify: string = code) => {
    if (codeToVerify.length !== 6 || isVerifying || isBurned) return;

    setIsVerifying(true);
    setError(null);

    try {
      const verified = await authApi.verifyChallenge(challenge.challenge_id, {
        code: codeToVerify,
      });
      onSuccess(verified);
    } catch (err) {
      if (err instanceof ApiError) {
        setError(getErrorMessage(err.problem));

        // Read server attempts_remaining if present in problem meta
        const serverAttempts =
          typeof err.problem.meta?.attempts_remaining === "number"
            ? err.problem.meta.attempts_remaining
            : null;

        const isWrongCode =
          err.problem.code === "CODE_INVALID" ||
          err.problem.code === "CHALLENGE_BURNED" ||
          (err.problem.status === 400 || err.problem.status === 401 || err.problem.status === 422);

        let newAttempts = attemptsRemaining;
        if (serverAttempts !== null) {
          newAttempts = serverAttempts;
        } else if (isWrongCode) {
          newAttempts = Math.max(0, attemptsRemaining - 1);
        }
        setAttemptsRemaining(newAttempts);

        if (newAttempts === 0 || err.problem.code === "CHALLENGE_BURNED") {
          setIsBurned(true);
          setError(
            "This verification code has been burned after 5 incorrect attempts. Please restart the process to receive a fresh code.",
          );
        }
      } else {
        setError("Failed to verify code. Please check your connection.");
      }
    } finally {
      setIsVerifying(false);
    }
  };

  const handleResend = async () => {
    if (secondsUntilResend > 0 || isResending || isBurned) return;

    setIsResending(true);
    setError(null);
    setCode("");

    try {
      const updatedChallenge = await authApi.resendChallenge(challenge.challenge_id);
      setChallenge(updatedChallenge);
      setAttemptsRemaining(updatedChallenge.attempts_remaining ?? 5);

      const newExpiry = Math.max(
        0,
        Math.floor((new Date(updatedChallenge.expires_at).getTime() - Date.now()) / 1000),
      );
      const newResend = Math.max(
        0,
        Math.floor((new Date(updatedChallenge.resend_after).getTime() - Date.now()) / 1000),
      );
      setSecondsUntilExpiry(newExpiry);
      setSecondsUntilResend(newResend);
    } catch (err) {
      if (err instanceof ApiError) {
        setError(getErrorMessage(err.problem));
      } else {
        setError("Failed to resend code. Please try again.");
      }
    } finally {
      setIsResending(false);
    }
  };

  const formatTime = (secs: number) => {
    const m = Math.floor(secs / 60);
    const s = secs % 60;
    return `${m}:${s.toString().padStart(2, "0")}`;
  };

  return (
    <div className="mx-auto w-full max-w-md space-y-6 rounded-2xl border border-slate-800 bg-slate-900/60 p-6 sm:p-8 shadow-xl backdrop-blur-sm">
      {onBack && (
        <button
          type="button"
          onClick={onBack}
          className="inline-flex items-center gap-1.5 text-xs font-medium text-slate-400 hover:text-slate-200 transition-colors"
        >
          <ArrowLeft className="h-3.5 w-3.5" />
          Back
        </button>
      )}

      <div className="space-y-2 text-center">
        <h1 className="text-2xl font-bold tracking-tight text-slate-100">
          Enter verification code
        </h1>
        <p className="text-sm text-slate-400">
          We emailed a 6-digit code to <span className="font-semibold text-slate-200">{email}</span>
        </p>
      </div>

      {error && (
        <div
          role="alert"
          className="flex items-start gap-2.5 rounded-lg border border-rose-500/50 bg-rose-500/10 p-3.5 text-sm font-medium text-rose-300"
        >
          <AlertCircle className="h-4 w-4 shrink-0 mt-0.5" />
          <span>{error}</span>
        </div>
      )}

      <div className="space-y-4 py-2">
        <OtpInput
          value={code}
          onChange={setCode}
          onComplete={(completedCode) => void handleVerify(completedCode)}
          disabled={isVerifying || isBurned || secondsUntilExpiry === 0}
          error={!!error}
        />

        <div className="flex items-center justify-between text-xs text-slate-400 px-1">
          <span>
            Expires in:{" "}
            <span className={secondsUntilExpiry < 60 ? "font-semibold text-rose-400" : "font-semibold text-slate-200"}>
              {formatTime(secondsUntilExpiry)}
            </span>
          </span>
          <span>
            Attempts left:{" "}
            <span className={attemptsRemaining <= 2 ? "font-semibold text-amber-400" : "font-semibold text-slate-200"}>
              {attemptsRemaining}/5
            </span>
          </span>
        </div>
      </div>

      <div className="space-y-3">
        <Button
          type="button"
          onClick={() => void handleVerify()}
          disabled={code.length !== 6 || isBurned || secondsUntilExpiry === 0}
          isLoading={isVerifying}
          className="w-full"
        >
          Verify & continue
        </Button>

        <div className="text-center">
          <button
            type="button"
            disabled={secondsUntilResend > 0 || isResending || isBurned}
            onClick={() => void handleResend()}
            className="inline-flex items-center gap-1.5 text-xs font-medium text-indigo-400 hover:text-indigo-300 disabled:opacity-50 disabled:pointer-events-none transition-colors"
          >
            <RotateCw className={isResending ? "h-3.5 w-3.5 animate-spin" : "h-3.5 w-3.5"} />
            {secondsUntilResend > 0
              ? `Resend code in ${secondsUntilResend}s`
              : "Didn't receive a code? Resend"}
          </button>
        </div>
      </div>
    </div>
  );
}
