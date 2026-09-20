import React, { useState } from "react";
import { useTranslation } from "react-i18next";
import { ArrowUpRight, CheckCircle2, DollarSign, X } from "lucide-react";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  useCreatorEarnings,
  useRequestPayout,
} from "../hooks/useStudio";

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
          defaultValue: `Minimum payout amount is ₫${threshold.toLocaleString("vi-VN")}`,
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
    <div
      role="dialog"
      aria-modal="true"
      aria-labelledby="request-payout-title"
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm p-4 animate-in fade-in duration-200"
    >
      <div className="relative w-full max-w-md rounded-xl border border-border-subtle bg-surface-card p-6 shadow-2xl space-y-5">
        <div className="flex items-center justify-between border-b border-border-subtle pb-4">
          <div className="flex items-center gap-2">
            <DollarSign className="h-5 w-5 text-primary-accent" />
            <h2
              id="request-payout-title"
              className="text-lg font-bold text-text"
            >
              {t("studio.payout.requestTitle", "Request Earnings Payout")}
            </h2>
          </div>
          <button
            type="button"
            onClick={onClose}
            aria-label={t("app.close", "Close")}
            className="rounded-lg p-1.5 text-text-muted hover:bg-surface-muted hover:text-text transition-colors"
          >
            <X className="h-5 w-5" />
          </button>
        </div>

        {/* Balance Card */}
        <div className="rounded-lg border border-border-subtle bg-surface-base p-4 space-y-2">
          <div className="flex items-center justify-between">
            <span className="text-xs text-text-muted">
              {t("studio.earnings.availableBalance", "Available Balance")}
            </span>
            <span className="text-base font-bold text-primary-accent">
              ₫{availableBalance.toLocaleString("vi-VN")}
            </span>
          </div>
          <div className="flex items-center justify-between text-xs text-text-muted pt-1 border-t border-border-subtle">
            <span>{t("studio.payout.minimumThreshold", "Minimum Payout")}</span>
            <span>₫{threshold.toLocaleString("vi-VN")}</span>
          </div>
          {earnings?.payout_account_configured && (
            <div className="flex items-center justify-between text-xs text-text-muted pt-1">
              <span>{t("studio.payout.destination", "Destination Bank")}</span>
              <span className="font-mono text-text">
                {earnings.payout_bank_code} - {earnings.payout_masked_account}
              </span>
            </div>
          )}
        </div>

        {error && (
          <div className="rounded-lg border border-danger/30 bg-danger/10 p-3 text-sm text-danger font-medium">
            {error}
          </div>
        )}

        {success && (
          <div className="flex items-center gap-2 rounded-lg border border-success/30 bg-success/10 p-3 text-sm text-success font-medium">
            <CheckCircle2 className="h-4 w-4 shrink-0" />
            {t(
              "studio.payout.requestSuccess",
              "Payout requested! It will be reviewed and processed by our finance team.",
            )}
          </div>
        )}

        <form onSubmit={(e) => { void handleSubmit(e); }} className="space-y-4">
          <div>
            <div className="flex items-center justify-between mb-1.5">
              <label
                htmlFor="payout-amount-input"
                className="text-xs font-semibold text-text uppercase tracking-wider"
              >
                {t("studio.payout.amountLabel", "Payout Amount (VND)")}
              </label>
              <button
                type="button"
                onClick={handleMax}
                className="text-xs font-semibold text-primary-accent hover:underline cursor-pointer"
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

          <div className="flex justify-end gap-3 pt-3 border-t border-border-subtle">
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
              <ArrowUpRight className="h-4 w-4" />
              {requestMutation.isPending
                ? t("studio.payout.requesting", "Submitting...")
                : t("studio.payout.submitRequest", "Confirm Payout")}
            </Button>
          </div>
        </form>
      </div>
    </div>
  );
}
