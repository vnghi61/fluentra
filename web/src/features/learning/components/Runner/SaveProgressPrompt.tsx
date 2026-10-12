import React from "react";
import { Link } from "@tanstack/react-router";
import { CloudOff } from "lucide-react";
import { useTranslation } from "react-i18next";

import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";

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

  return (
    <Dialog
      open={isOpen}
      onOpenChange={(open) => {
        if (!open) onDismiss();
      }}
    >
      <DialogContent className="max-w-md">
        <DialogHeader className="space-y-2">
          <DialogTitle className="flex items-center gap-2">
            <CloudOff
              className="h-5 w-5 shrink-0 text-text-muted"
              aria-hidden="true"
            />
            {t("guest.savePrompt.title", "This result was not saved")}
          </DialogTitle>
          <DialogDescription>
            {t("guest.savePrompt.desc", {
              score,
              total,
              defaultValue:
                "You answered {{score}} of {{total}} correctly. You are not signed in, so nothing was recorded — no progress, no streak, and no review cards. Sign in and your next lesson counts.",
            })}
          </DialogDescription>
        </DialogHeader>

        <DialogFooter className="flex-col-reverse sm:flex-row">
          <Button
            variant="outline"
            onClick={onDismiss}
            className="w-full sm:w-auto"
          >
            {t("guest.savePrompt.dismissBtn", "Keep looking around")}
          </Button>
          <Link to="/register" className="w-full sm:w-auto">
            <Button className="w-full">
              {t("guest.savePrompt.createAccountBtn", "Create an account")}
            </Button>
          </Link>
          <Link to="/login" className="w-full sm:w-auto">
            <Button variant="secondary" className="w-full">
              {t("guest.savePrompt.signInBtn", "Sign in")}
            </Button>
          </Link>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
};
