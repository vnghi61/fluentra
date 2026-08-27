import React from "react";
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

  if (!isOpen) return null;

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-labelledby="exit-dialog-title"
      className="fixed inset-0 z-50 flex items-center justify-center bg-overlay/75 p-4 backdrop-blur-sm animate-in fade-in"
    >
      <Card className="max-w-md w-full border-border bg-surface-card shadow-2xl">
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
    </div>
  );
};
