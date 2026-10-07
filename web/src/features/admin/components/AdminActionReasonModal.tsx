import React, { useState } from "react";
import { useTranslation } from "react-i18next";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { AlertCircle, FileText, Loader2, X } from "lucide-react";
import {
  adminActionReasonSchema,
  type AdminActionReasonFormValues,
} from "../model/schemas";
import { ApiError } from "@/api/client";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import * as Dialog from "@radix-ui/react-dialog";

interface AdminActionReasonModalProps {
  isOpen: boolean;
  title: string;
  description: string;
  actionButtonLabel: string;
  variant?: "destructive" | "primary" | undefined;
  onClose: () => void;
  onConfirm: (reason: string) => Promise<void>;
}

export const AdminActionReasonModal: React.FC<AdminActionReasonModalProps> = ({
  isOpen,
  title,
  description,
  actionButtonLabel,
  variant = "destructive",
  onClose,
  onConfirm,
}) => {
  const { t } = useTranslation();
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const {
    register,
    handleSubmit,
    formState: { errors },
    reset,
  } = useForm<AdminActionReasonFormValues>({
    resolver: (zodResolver as (schema: unknown) => never)(
      adminActionReasonSchema,
    ),
    defaultValues: {
      reason: "",
    },
  });

  if (!isOpen) return null;

  const handleFormSubmit = async (values: AdminActionReasonFormValues) => {
    setIsSubmitting(true);
    setError(null);

    try {
      await onConfirm(values.reason.trim());
      reset();
      onClose();
    } catch (err: unknown) {
      if (
        err instanceof ApiError &&
        (err.problem.code === "SELF_ADMIN_ACTION_FORBIDDEN" ||
          err.problem.status === 403)
      ) {
        setError(
          "Self-administration is forbidden: you cannot suspend, reinstate, or revoke sessions on your own account.",
        );
      } else {
        setError(
          err instanceof Error
            ? err.message
            : t("admin.failedToPerformAdministrativeAction"),
        );
      }
    } finally {
      setIsSubmitting(false);
    }
  };

  const handleClose = () => {
    reset();
    setError(null);
    onClose();
  };

  return (
    <Dialog.Root
      open
      onOpenChange={(open) => {
        if (!open && !isSubmitting) handleClose();
      }}
    >
      <Dialog.Portal>
        <Dialog.Overlay className="fixed inset-0 z-50 bg-overlay/80 backdrop-blur-sm" />
        <Dialog.Content
          aria-describedby="admin-reason-description"
          className="fixed inset-x-0 bottom-0 z-50 max-h-[90dvh] w-full overflow-y-auto rounded-t-2xl border border-border-subtle bg-surface-card p-6 shadow-2xl md:inset-auto md:left-1/2 md:top-1/2 md:w-[min(100%-2rem,28rem)] md:-translate-x-1/2 md:-translate-y-1/2 md:rounded-2xl"
        >
          <div className="flex items-center justify-between border-b border-border-subtle pb-4">
            <Dialog.Title
              className="text-lg font-semibold text-text flex items-center gap-2"
            >
              <FileText className="h-5 w-5 text-primary-accent" />
              {title}
            </Dialog.Title>
            <button
              type="button"
              onClick={handleClose}
              disabled={isSubmitting}
              aria-label={t("common.close", "Close")}
              className="rounded-lg p-1 text-text-muted hover:bg-surface-muted hover:text-text transition-colors"
            >
              <X aria-hidden="true" className="h-5 w-5" />
            </button>
          </div>

          <Dialog.Description
            id="admin-reason-description"
            className="text-xs text-text-muted leading-relaxed"
          >
            {description}
          </Dialog.Description>

          {error && (
            <div className="flex items-start gap-2.5 rounded-lg border border-danger/30 bg-danger/10 p-3.5 text-xs text-danger-accent">
              <AlertCircle className="h-4 w-4 shrink-0 text-danger-accent mt-0.5" />
              <span>{error}</span>
            </div>
          )}

          <form
            onSubmit={(e) => {
              void handleSubmit(handleFormSubmit)(e);
            }}
            className="space-y-4"
          >
            <div className="space-y-2">
              <Label
                htmlFor="action-reason"
                className="text-xs text-text-muted"
              >
                Audit Reason (Minimum 10 characters required)
              </Label>
              <textarea
                id="action-reason"
                rows={3}
                {...register("reason")}
                placeholder={t("admin.stateTheJustificationForThis")}
                className="w-full rounded-lg border border-border-subtle bg-surface-muted p-3 text-base md:text-sm text-text placeholder:text-text-muted focus:outline-none focus:ring-2 focus:ring-primary"
                aria-invalid={!!errors.reason}
              />
              {errors.reason && (
                <p className="text-xs text-danger-accent">
                  {/* The schema carries translation keys, not sentences. */}
                  {t(errors.reason.message ?? "")}
                </p>
              )}
            </div>

            <div className="flex items-center justify-end gap-3 border-t border-border-subtle pt-4">
              <Button
                type="button"
                variant="ghost"
                onClick={handleClose}
                disabled={isSubmitting}
              >
                {t("admin.cancel")}
              </Button>
              <Button type="submit" variant={variant} disabled={isSubmitting}>
                {isSubmitting ? (
                  <>
                    <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                    {t("admin.executing")}
                  </>
                ) : (
                  actionButtonLabel
                )}
              </Button>
            </div>
          </form>
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  );
};
