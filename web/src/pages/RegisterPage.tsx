import * as React from "react";
import { useNavigate } from "@tanstack/react-router";
import {
  OtpVerificationScreen,
  RegisterForm,
  type Challenge,
} from "@/features/auth";

export function RegisterPage(): React.JSX.Element {
  const navigate = useNavigate();
  const [challengeState, setChallengeState] = React.useState<{
    challenge: Challenge;
    email: string;
  } | null>(null);

  return (
    <div className="flex min-h-[calc(100vh-4rem)] items-center justify-center px-4 py-12">
      {challengeState ? (
        <OtpVerificationScreen
          challenge={challengeState.challenge}
          email={challengeState.email}
          onSuccess={() => void navigate({ to: "/" })}
          onBack={() => setChallengeState(null)}
        />
      ) : (
        <RegisterForm
          onChallengeIssued={(challenge, email) => {
            setChallengeState({ challenge, email });
          }}
        />
      )}
    </div>
  );
}
