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
    <div className="flex w-full flex-1 items-center justify-center px-4 pb-20 pt-4 sm:pb-24 sm:pt-8">
      {challengeState ? (
        <OtpVerificationScreen
          // Keyed on the challenge so a new one remounts the screen. Without
          // this, React reuses the instance and its state comes with it: a
          // learner who burns a code and starts over lands on a screen that is
          // still `isBurned`, with the digit boxes and both buttons disabled
          // and no way back.
          key={challengeState.challenge.challenge_id}
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
          onGoogleSuccess={() => void navigate({ to: "/" })}
        />
      )}
    </div>
  );
}
