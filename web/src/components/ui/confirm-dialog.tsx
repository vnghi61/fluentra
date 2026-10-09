import * as React from "react";
import * as AlertDialog from "@radix-ui/react-alert-dialog";
import { useTranslation } from "react-i18next";

import { Button } from "@/components/ui/button";

export interface ConfirmDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title: React.ReactNode;
  description?: React.ReactNode;
  confirmLabel?: React.ReactNode;
  cancelLabel?: React.ReactNode;
  /** Red confirm button, for deleting or discarding. */
  destructive?: boolean;
  isPending?: boolean;
  /**
   * Keep the dialog open after Confirm, for an action that may fail: the
   * caller closes it on success and the pending spinner stays visible meanwhile.
   */
  keepOpenOnConfirm?: boolean;
  onConfirm: () => void;
}

/**
 * The one confirmation dialog: Radix AlertDialog, so focus is trapped, Escape
 * cancels and the cancel button receives focus first. Use it for anything that
 * deletes or discards work.
 */
export function ConfirmDialog({
  open,
  onOpenChange,
  title,
  description,
  confirmLabel,
  cancelLabel,
  destructive = false,
  isPending = false,
  keepOpenOnConfirm = false,
  onConfirm,
}: ConfirmDialogProps): React.JSX.Element {
  const { t } = useTranslation();
  return (
    <AlertDialog.Root open={open} onOpenChange={onOpenChange}>
      <AlertDialog.Portal>
        <AlertDialog.Overlay className="fixed inset-0 z-[60] bg-overlay/50 data-[state=open]:animate-in data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0" />
        <AlertDialog.Content className="fixed left-1/2 top-1/2 z-[60] w-[min(calc(100%-2rem),28rem)] -translate-x-1/2 -translate-y-1/2 space-y-4 rounded-2xl border border-border-subtle bg-surface-card p-5 shadow-xl outline-none sm:p-6">
          <AlertDialog.Title className="text-lg font-semibold leading-tight text-text">
            {title}
          </AlertDialog.Title>
          {description && (
            <AlertDialog.Description className="text-sm text-text-muted">
              {description}
            </AlertDialog.Description>
          )}
          <div className="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
            <AlertDialog.Cancel asChild>
              <Button variant="outline">
                {cancelLabel ?? t("common.cancel", "Cancel")}
              </Button>
            </AlertDialog.Cancel>
            <AlertDialog.Action asChild>
              <Button
                variant={destructive ? "destructive" : "primary"}
                isLoading={isPending}
                onClick={(e) => {
                  if (keepOpenOnConfirm) e.preventDefault();
                  onConfirm();
                }}
              >
                {confirmLabel ?? t("common.confirm", "Confirm")}
              </Button>
            </AlertDialog.Action>
          </div>
        </AlertDialog.Content>
      </AlertDialog.Portal>
    </AlertDialog.Root>
  );
}
