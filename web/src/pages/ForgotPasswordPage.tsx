import * as React from "react";
import {
  ForgotPasswordForm,
  ResetPasswordForm,
  type Challenge,
} from "@/features/auth";

export function ForgotPasswordPage(): React.JSX.Element {
  const [challengeState, setChallengeState] = React.useState<{
    challenge: Challenge;
    email: string;
  } | null>(null);

  return (
    <div className="flex w-full flex-1 items-center justify-center px-4 pb-20 pt-4 sm:pb-24 sm:pt-8">
      {challengeState ? (
        <ResetPasswordForm
          // Same reason as RegisterPage: a fresh challenge must not inherit the
          // previous one's state.
          key={challengeState.challenge.challenge_id}
          challengeId={challengeState.challenge.challenge_id}
          email={challengeState.email}
        />
      ) : (
        <ForgotPasswordForm
          onChallengeIssued={(challenge, email) => {
            setChallengeState({ challenge, email });
          }}
        />
      )}
    </div>
  );
}
