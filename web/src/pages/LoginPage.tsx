import * as React from "react";
import { useNavigate } from "@tanstack/react-router";
import { LoginForm } from "@/features/auth";

export function LoginPage(): React.JSX.Element {
  const navigate = useNavigate();

  return (
    // Fills <main> below the 64px header and centres there; the bottom padding
    // is the top padding plus that header, so the card sits on the middle of
    // the screen rather than 32px below it.
    <div className="flex w-full flex-1 items-center justify-center px-4 pb-20 pt-4 sm:pb-24 sm:pt-8">
      <LoginForm onSuccess={() => void navigate({ to: "/" })} />
    </div>
  );
}
