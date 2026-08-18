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
    <div className="flex min-h-[calc(100vh-4rem)] items-center justify-center px-4 py-12">
      {challengeState ? (
        <ResetPasswordForm
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
