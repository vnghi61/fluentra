import React, { useState } from "react";
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
            : "Failed to perform administrative action.",
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
    <div
      role="dialog"
      aria-modal="true"
      aria-labelledby="admin-reason-modal-title"
      className="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/80 backdrop-blur-sm p-4"
    >
      <div className="w-full max-w-md rounded-2xl border border-slate-800 bg-slate-900 p-6 shadow-2xl space-y-6">
        <div className="flex items-center justify-between border-b border-slate-800 pb-4">
          <h2
            id="admin-reason-modal-title"
            className="text-lg font-semibold text-slate-100 flex items-center gap-2"
          >
            <FileText className="h-5 w-5 text-indigo-400" />
            {title}
          </h2>
          <button
            type="button"
            onClick={handleClose}
            disabled={isSubmitting}
            className="rounded-lg p-1 text-slate-400 hover:bg-slate-800 hover:text-slate-200 transition-colors"
          >
            <X className="h-5 w-5" />
          </button>
        </div>

        <p className="text-xs text-slate-300 leading-relaxed">{description}</p>

        {error && (
          <div className="flex items-start gap-2.5 rounded-lg border border-rose-500/30 bg-rose-500/10 p-3.5 text-xs text-rose-300">
            <AlertCircle className="h-4 w-4 shrink-0 text-rose-400 mt-0.5" />
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
            <Label htmlFor="action-reason" className="text-xs text-slate-300">
              Audit Reason (Minimum 10 characters required)
            </Label>
            <textarea
              id="action-reason"
              rows={3}
              {...register("reason")}
              placeholder="State the justification for this administrative action..."
              className="w-full rounded-lg border border-slate-800 bg-slate-950 p-3 text-base md:text-sm text-slate-200 placeholder:text-slate-500 focus:outline-none focus:ring-2 focus:ring-indigo-500"
              aria-invalid={!!errors.reason}
            />
            {errors.reason && (
              <p className="text-xs text-rose-400">{errors.reason.message}</p>
            )}
          </div>

          <div className="flex items-center justify-end gap-3 border-t border-slate-800 pt-4">
            <Button
              type="button"
              variant="ghost"
              onClick={handleClose}
              disabled={isSubmitting}
            >
              Cancel
            </Button>
            <Button type="submit" variant={variant} disabled={isSubmitting}>
              {isSubmitting ? (
                <>
                  <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                  Executing...
                </>
              ) : (
                actionButtonLabel
              )}
            </Button>
          </div>
        </form>
      </div>
    </div>
  );
};
