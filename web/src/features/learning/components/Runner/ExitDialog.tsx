import React from "react";
import * as AlertDialog from "@radix-ui/react-alert-dialog";
import { LogOut } from "lucide-react";
import { useTranslation } from "react-i18next";

import { Button } from "@/components/ui/button";
import { useReturnFocus } from "@/components/ui/use-return-focus";

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
  const focusProps = useReturnFocus({});

  return (
    <AlertDialog.Root
      open={isOpen}
      onOpenChange={(open) => {
        if (!open) onCancel();
      }}
    >
      <AlertDialog.Portal>
        <AlertDialog.Overlay className="fixed inset-0 z-[60] bg-overlay/50" />
        <AlertDialog.Content
          {...focusProps}
          data-scroll-lock="managed"
          className="fixed left-1/2 top-1/2 z-[60] flex max-h-[calc(100dvh-2rem)] w-[calc(100%-2rem)] max-w-md -translate-x-1/2 -translate-y-1/2 flex-col overflow-y-auto rounded-2xl border border-border-subtle bg-surface-card shadow-xl outline-none"
        >
          <div className="space-y-4 p-5 sm:p-6">
            <span className="flex h-12 w-12 items-center justify-center rounded-2xl bg-warning/10 text-warning-accent">
              <LogOut className="h-6 w-6" aria-hidden="true" />
            </span>
            <AlertDialog.Title className="text-lg font-semibold leading-tight text-text">
              {t("runner.exitDialogTitle", "Exit Lesson?")}
            </AlertDialog.Title>
            <AlertDialog.Description className="text-sm leading-relaxed text-text-muted">
              {t(
                "runner.exitDialogDesc",
                "Completed exercises have been saved. Any answer you haven't submitted will be lost. Do you want to leave?",
              )}
            </AlertDialog.Description>
          </div>
          <div className="flex flex-col-reverse justify-end gap-3 border-t border-border-subtle p-4 sm:flex-row">
            <Button
              type="button"
              variant="outline"
              onClick={onConfirm}
              className="w-full sm:w-auto"
            >
              {t("runner.confirmExitBtn", "Exit Lesson")}
            </Button>
            <AlertDialog.Cancel asChild>
              <Button type="button" className="w-full sm:w-auto">
                {t("runner.resumeBtn", "Keep Learning")}
              </Button>
            </AlertDialog.Cancel>
          </div>
        </AlertDialog.Content>
      </AlertDialog.Portal>
    </AlertDialog.Root>
  );
};
