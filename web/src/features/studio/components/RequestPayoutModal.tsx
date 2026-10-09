import React, { useState } from "react";
import { useTranslation } from "react-i18next";
import { ArrowUpRight, CheckCircle2, DollarSign } from "lucide-react";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Dialog,
  DialogBody,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { useCreatorEarnings, useRequestPayout } from "../hooks/useStudio";
import { formatVnd } from "@/lib/currency";

interface RequestPayoutModalProps {
  isOpen: boolean;
  onClose: () => void;
}

export function RequestPayoutModal({
  isOpen,
  onClose,
}: RequestPayoutModalProps): React.JSX.Element | null {
  const { t } = useTranslation();
  const { data: earnings } = useCreatorEarnings();
  const requestMutation = useRequestPayout();

  const availableBalance = earnings?.available_balance_vnd ?? 0;
  const threshold = earnings?.payout_threshold_vnd ?? 500000;
  const [amountVnd, setAmountVnd] = useState<number>(availableBalance);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState(false);

  if (!isOpen) return null;

  const handleMax = () => {
    setAmountVnd(availableBalance);
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    setSuccess(false);

    if (amountVnd < threshold) {
      setError(
        t("studio.payout.errBelowThreshold", {
          threshold: threshold.toLocaleString("vi-VN"),
          defaultValue: `Minimum payout amount is ${formatVnd(threshold)}`,
        }),
      );
      return;
    }

    if (amountVnd > availableBalance) {
      setError(
        t(
          "studio.payout.errExceedsBalance",
          "Requested amount exceeds available balance",
        ),
      );
      return;
    }

    try {
      await requestMutation.mutateAsync({ amount_vnd: amountVnd });
      setSuccess(true);
      setTimeout(() => {
        onClose();
        setSuccess(false);
      }, 1500);
    } catch (err) {
      setError(
        err instanceof Error
          ? err.message
          : t("studio.payout.errRequestFailed", "Failed to request payout"),
      );
    }
  };

  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open && !requestMutation.isPending) onClose();
      }}
    >
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <DollarSign
              className="h-5 w-5 shrink-0 text-primary-accent"
              aria-hidden="true"
            />
            {t("studio.payout.requestTitle", "Request Earnings Payout")}
          </DialogTitle>
        </DialogHeader>
        <form
          onSubmit={(e) => {
            void handleSubmit(e);
          }}
          className="flex min-h-0 flex-1 flex-col"
        >
          <DialogBody>
            {/* Balance Card */}
            <div className="rounded-lg border border-border-subtle bg-surface-muted p-4 space-y-2">
              <div className="flex flex-wrap items-center justify-between gap-x-3 gap-y-1">
                <span className="text-sm text-text-muted">
                  {t("studio.earnings.availableBalance", "Available Balance")}
                </span>
                <span className="text-base font-bold tabular-nums text-primary-accent">
                  {formatVnd(availableBalance)}
                </span>
              </div>
              <div className="flex flex-wrap items-center justify-between gap-x-3 gap-y-1 text-sm text-text-muted pt-1 border-t border-border-subtle">
                <span>
                  {t("studio.payout.minimumThreshold", "Minimum Payout")}
                </span>
                <span>{formatVnd(threshold)}</span>
              </div>
              {earnings?.payout_account_configured && (
                <div className="flex flex-wrap items-center justify-between gap-x-3 gap-y-1 text-sm text-text-muted pt-1">
                  <span>
                    {t("studio.payout.destination", "Destination Bank")}
                  </span>
                  <span className="font-mono text-text">
                    {earnings.payout_bank_code} -{" "}
                    {earnings.payout_masked_account}
                  </span>
                </div>
              )}
            </div>

            {error && (
              <div
                role="alert"
                className="rounded-lg border border-danger/30 bg-danger/10 p-3 text-sm text-danger-accent font-medium"
              >
                {error}
              </div>
            )}

            {success && (
              <div
                role="status"
                className="flex items-center gap-2 rounded-lg border border-success/30 bg-success/10 p-3 text-sm text-success-accent font-medium"
              >
                <CheckCircle2 className="h-5 w-5 shrink-0" aria-hidden="true" />
                {t(
                  "studio.payout.requestSuccess",
                  "Payout requested! It will be reviewed and processed by our finance team.",
                )}
              </div>
            )}

            <div>
              <div className="flex items-center justify-between gap-3 mb-1.5">
                <label
                  htmlFor="payout-amount-input"
                  className="text-xs font-semibold text-text uppercase tracking-wider"
                >
                  {t("studio.payout.amountLabel", "Payout Amount (VND)")}
                </label>
                <button
                  type="button"
                  onClick={handleMax}
                  className="-mr-2 inline-flex min-h-11 shrink-0 cursor-pointer items-center rounded-lg px-2 text-sm font-semibold text-primary-accent hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary"
                >
                  {t("studio.payout.maxButton", "Use Max")}
                </button>
              </div>
              <div className="relative">
                <span className="absolute left-3 top-1/2 -translate-y-1/2 text-sm text-text-muted">
                  ₫
                </span>
                <Input
                  id="payout-amount-input"
                  type="number"
                  min={threshold}
                  max={availableBalance}
                  step={1000}
                  value={amountVnd}
                  onChange={(e) => setAmountVnd(Number(e.target.value))}
                  required
                  className="pl-8 text-base font-semibold"
                />
              </div>
            </div>
          </DialogBody>
          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              onClick={onClose}
              disabled={requestMutation.isPending}
            >
              {t("app.cancel", "Cancel")}
            </Button>
            <Button
              type="submit"
              variant="primary"
              disabled={
                requestMutation.isPending ||
                availableBalance < threshold ||
                !earnings?.payout_account_configured
              }
              className="gap-2"
            >
              <ArrowUpRight className="h-4 w-4" aria-hidden="true" />
              {requestMutation.isPending
                ? t("studio.payout.requesting", "Submitting...")
                : t("studio.payout.submitRequest", "Confirm Payout")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
