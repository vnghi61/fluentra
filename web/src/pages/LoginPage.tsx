import * as React from "react";
import { useNavigate } from "@tanstack/react-router";
import { LoginForm } from "@/features/auth";

export function LoginPage(): React.JSX.Element {
  const navigate = useNavigate();

  return (
    <div className="flex min-h-[calc(100vh-4rem)] items-center justify-center px-4 py-12">
      <LoginForm onSuccess={() => void navigate({ to: "/" })} />
    </div>
  );
}
