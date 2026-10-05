import React, { useEffect } from "react";
import { createPortal } from "react-dom";
import { AlertCircle } from "lucide-react";
import { useTranslation } from "react-i18next";

import { Button } from "@/components/ui/button";
import {
  Card,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";

export interface ExitDialogProps {
  isOpen: boolean;
  onCancel: () => void;
  onConfirm: () => void;
}

export const ExitDialog: React.FC<ExitDialogProps> = ({
  isOpen,
  onCancel,
  onConfirm,
}) => {
  const { t } = useTranslation();

  // Escape keeps learning, and the page behind does not scroll under the
  // overlay.
  useEffect(() => {
    if (!isOpen) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onCancel();
    };
    const previousOverflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    window.addEventListener("keydown", onKey);
    return () => {
      document.body.style.overflow = previousOverflow;
      window.removeEventListener("keydown", onKey);
    };
  }, [isOpen, onCancel]);

  if (!isOpen) return null;

  // Portalled to <body>: inside the lesson an animated ancestor became the
  // containing block for `position: fixed`, so the overlay covered only the
  // content column and the dialog sat off-centre in a framed box.
  return createPortal(
    <div
      role="dialog"
      aria-modal="true"
      aria-labelledby="exit-dialog-title"
      className="fixed inset-0 z-[60] flex items-center justify-center bg-overlay/60 p-4 animate-in fade-in duration-150 motion-reduce:animate-none"
    >
      <Card className="w-full max-w-md rounded-2xl border-border-subtle bg-surface-card shadow-2xl">
        <CardHeader className="space-y-2">
          <div className="flex items-center gap-2 text-warning-accent">
            <AlertCircle className="h-6 w-6 shrink-0" aria-hidden="true" />
            <CardTitle id="exit-dialog-title" className="text-xl font-bold">
              {t("runner.exitDialogTitle", "Exit Lesson?")}
            </CardTitle>
          </div>
          <CardDescription className="text-sm">
            {t(
              "runner.exitDialogDesc",
              "Your progress on completed exercises has been saved. Are you sure you want to leave?",
            )}
          </CardDescription>
        </CardHeader>

        <CardFooter className="flex flex-col-reverse sm:flex-row justify-end gap-3 pt-2">
          <Button
            variant="outline"
            onClick={onCancel}
            className="w-full sm:w-auto min-h-[44px]"
          >
            {t("runner.resumeBtn", "Keep Learning")}
          </Button>
          <Button
            variant="destructive"
            onClick={onConfirm}
            className="w-full sm:w-auto min-h-[44px]"
          >
            {t("runner.confirmExitBtn", "Exit Lesson")}
          </Button>
        </CardFooter>
      </Card>
    </div>,
    document.body,
  );
};
