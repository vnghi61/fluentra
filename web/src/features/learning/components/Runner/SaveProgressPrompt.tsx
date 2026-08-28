import React from "react";
import { Link } from "@tanstack/react-router";
import { CloudOff } from "lucide-react";
import { useTranslation } from "react-i18next";

import { Button } from "@/components/ui/button";
import {
  Card,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";

export interface SaveProgressPromptProps {
  isOpen: boolean;
  /** How many activities were answered correctly, and out of how many. */
  score: number;
  total: number;
  onDismiss: () => void;
}

/**
 * Tells a guest, at the end, that nothing was kept.
 *
 * It appears once the lesson is finished rather than at the start, because a
 * warning nobody has a reason to read yet is a warning nobody reads. By this
 * point they have done the work and the offer is concrete: sign in and the next
 * one counts.
 *
 * It states the fact plainly and does not trap them — dismissing is a real
 * choice, and the result they just earned stays on the screen behind it. The
 * lesson was still worth doing.
 */
export const SaveProgressPrompt: React.FC<SaveProgressPromptProps> = ({
  isOpen,
  score,
  total,
  onDismiss,
}) => {
  const { t } = useTranslation();

  if (!isOpen) return null;

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-labelledby="save-progress-title"
      className="fixed inset-0 z-50 flex items-center justify-center bg-overlay/75 p-4 backdrop-blur-sm animate-in fade-in"
    >
      <Card className="max-w-md w-full border-border bg-surface-card shadow-2xl">
        <CardHeader className="space-y-2">
          <div className="flex items-center gap-2 text-text-muted">
            <CloudOff className="h-6 w-6 shrink-0" aria-hidden="true" />
            <CardTitle id="save-progress-title" className="text-xl font-bold">
              {t("guest.savePrompt.title", "This result was not saved")}
            </CardTitle>
          </div>
          <CardDescription className="text-sm">
            {t("guest.savePrompt.desc", {
              score,
              total,
              defaultValue:
                "You answered {{score}} of {{total}} correctly. You are not signed in, so nothing was recorded — no progress, no streak, and no review cards. Sign in and your next lesson counts.",
            })}
          </CardDescription>
        </CardHeader>

        <CardFooter className="flex flex-col-reverse sm:flex-row justify-end gap-3 pt-2">
          <Button
            variant="outline"
            onClick={onDismiss}
            className="w-full sm:w-auto min-h-[44px]"
          >
            {t("guest.savePrompt.dismissBtn", "Keep looking around")}
          </Button>
          <Link to="/register" className="w-full sm:w-auto">
            <Button className="w-full min-h-[44px]">
              {t("guest.savePrompt.createAccountBtn", "Create an account")}
            </Button>
          </Link>
          <Link to="/login" className="w-full sm:w-auto">
            <Button variant="secondary" className="w-full min-h-[44px]">
              {t("guest.savePrompt.signInBtn", "Sign in")}
            </Button>
          </Link>
        </CardFooter>
      </Card>
    </div>
  );
};
