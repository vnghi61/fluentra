import React, { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import {
  ArrowUpRight,
  Building2,
  CheckCircle2,
  Clock,
  DollarSign,
  Send,
  X,
  XCircle,
} from "lucide-react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Card,
  CardDescription,
  CardTitle,
} from "@/components/ui/card";
import {
  adminApi,
  type PayoutResponse,
} from "../api/adminApi";

export function AdminPayoutsList(): React.JSX.Element {
  const { t } = useTranslation();

  const {
    data,
    isLoading: loading,
    error: queryError,
    refetch,
  } = useQuery({
    queryKey: ["admin", "payouts"],
    queryFn: () => adminApi.listBillingPayouts({ limit: 50 }),
  });

  const payouts = data?.items ?? [];
  const total = data?.total ?? 0;
  const error = queryError instanceof Error ? queryError.message : null;

  const [activePayout, setActivePayout] = useState<PayoutResponse | null>(null);
  const [bankReference, setBankReference] = useState("");
  const [note, setNote] = useState("");
  const [actionLoading, setActionLoading] = useState(false);
  const [actionError, setActionError] = useState<string | null>(null);

  const handleFulfill = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!activePayout) return;
    setActionError(null);

    if (!bankReference.trim()) {
      setActionError(
        t(
          "adminPayouts.errBankRefRequired",
          "Bank transfer transaction reference number is required.",
        ),
      );
      return;
    }

    setActionLoading(true);
    try {
      await adminApi.fulfillBillingPayout(activePayout.id, {
        bank_reference: bankReference.trim(),
        note: note.trim() || null,
      });
      setActivePayout(null);
      setBankReference("");
      setNote("");
      void refetch();
    } catch (err) {
      setActionError(
        err instanceof Error
          ? err.message
          : t("adminPayouts.errFulfillFailed", "Failed to record payout fulfillment"),
      );
    } finally {
      setActionLoading(false);
    }
  };

  const getStatusBadge = (status: string) => {
    switch (status) {
      case "sent":
        return (
          <Badge variant="success" className="gap-1 text-xs">
            <CheckCircle2 className="h-3 w-3" />
            {t("adminPayouts.statusSent", "Sent / Fulfilled")}
          </Badge>
        );
      case "failed":
        return (
          <Badge variant="danger" className="gap-1 text-xs">
            <XCircle className="h-3 w-3" />
            {t("adminPayouts.statusFailed", "Failed")}
          </Badge>
        );
      default:
        return (
          <Badge variant="warning" className="gap-1 text-xs">
            <Clock className="h-3 w-3" />
            {t("adminPayouts.statusPending", "Pending Review")}
          </Badge>
        );
    }
  };

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
        <div>
          <h2 className="text-xl font-bold text-text">
            {t("adminPayouts.title", "Creator Payouts Management")}
          </h2>
          <p className="text-xs text-text-muted mt-0.5">
            {t(
              "adminPayouts.subtitle",
              "Review requested creator earnings payouts and record manual bank transfer fulfillment receipts.",
            )}
          </p>
        </div>
        <Badge variant="primary" className="text-xs px-2.5 py-1 self-start sm:self-auto">
          {t("adminPayouts.count", {
            count: total,
            defaultValue: `${total} payouts total`,
          })}
        </Badge>
      </div>

      {error && (
        <div className="rounded-lg border border-danger/30 bg-danger/10 p-3 text-sm text-danger font-medium">
          {error}
        </div>
      )}

      {loading ? (
        <div className="py-16 text-center text-sm text-text-muted">
          {t("app.loading", "Loading payouts...")}
        </div>
      ) : payouts.length === 0 ? (
        <Card className="border-border-subtle bg-surface-card p-10 text-center space-y-2">
          <DollarSign className="h-10 w-10 text-text-muted mx-auto" />
          <CardTitle className="text-base font-bold text-text">
            {t("adminPayouts.emptyTitle", "No payouts requested")}
          </CardTitle>
          <CardDescription className="text-xs">
            {t(
              "adminPayouts.emptyDesc",
              "When creators request earnings withdrawals, they will appear here for processing.",
            )}
          </CardDescription>
        </Card>
      ) : (
        <div className="overflow-x-auto rounded-xl border border-border-subtle bg-surface-card shadow-sm">
          <table className="w-full text-left text-xs">
            <thead className="bg-surface-base border-b border-border-subtle text-text-muted uppercase tracking-wider">
              <tr>
                <th className="p-3.5">
                  {t("adminPayouts.colCreator", "Creator ID")}
                </th>
                <th className="p-3.5">
                  {t("adminPayouts.colAmount", "Amount")}
                </th>
                <th className="p-3.5">
                  {t("adminPayouts.colStatus", "Status")}
                </th>
                <th className="p-3.5">
                  {t("adminPayouts.colReference", "Bank Reference")}
                </th>
                <th className="p-3.5">
                  {t("adminPayouts.colDate", "Requested At")}
                </th>
                <th className="p-3.5 text-right">
                  {t("adminPayouts.colAction", "Action")}
                </th>
              </tr>
            </thead>
            <tbody className="divide-y divide-border-subtle">
              {payouts.map((payout) => (
                <tr
                  key={payout.id}
                  className="hover:bg-surface-base/50 transition-colors"
                >
                  <td className="p-3.5 font-mono text-[11px] text-text">
                    {payout.creator_id}
                  </td>
                  <td className="p-3.5 font-bold text-text">
                    ₫{payout.amount_vnd.toLocaleString("vi-VN")}
                  </td>
                  <td className="p-3.5">{getStatusBadge(payout.status)}</td>
                  <td className="p-3.5 font-mono text-[11px] text-text-muted">
                    {payout.bank_reference || "—"}
                  </td>
                  <td className="p-3.5 text-text-muted">
                    {new Date(payout.created_at).toLocaleDateString(undefined, {
                      month: "short",
                      day: "numeric",
                      year: "numeric",
                      hour: "2-digit",
                      minute: "2-digit",
                    })}
                  </td>
                  <td className="p-3.5 text-right">
                    {payout.status === "pending" && (
                      <Button
                        variant="primary"
                        size="sm"
                        onClick={() => {
                          setActivePayout(payout);
                          setBankReference("");
                          setNote("");
                          setActionError(null);
                        }}
                        className="gap-1 text-xs"
                      >
                        <ArrowUpRight className="h-3.5 w-3.5" />
                        {t("adminPayouts.fulfillBtn", "Fulfill")}
                      </Button>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {/* Fulfill Payout Modal */}
      {activePayout && (
        <div
          role="dialog"
          aria-modal="true"
          aria-labelledby="fulfill-payout-title"
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm p-4 animate-in fade-in duration-200"
        >
          <div className="relative w-full max-w-md rounded-xl border border-border-subtle bg-surface-card p-6 shadow-2xl space-y-5">
            <div className="flex items-center justify-between border-b border-border-subtle pb-4">
              <div className="flex items-center gap-2">
                <Building2 className="h-5 w-5 text-primary-accent" />
                <h2
                  id="fulfill-payout-title"
                  className="text-lg font-bold text-text"
                >
                  {t("adminPayouts.fulfillModalTitle", "Fulfill Creator Payout")}
                </h2>
              </div>
              <button
                type="button"
                onClick={() => setActivePayout(null)}
                aria-label={t("app.close", "Close")}
                className="rounded-lg p-1.5 text-text-muted hover:bg-surface-muted hover:text-text transition-colors"
              >
                <X className="h-5 w-5" />
              </button>
            </div>

            {/* Payout Details */}
            <div className="rounded-lg border border-border-subtle bg-surface-base p-4 space-y-2 text-xs">
              <div className="flex items-center justify-between">
                <span className="text-text-muted">
                  {t("adminPayouts.colAmount", "Amount")}:
                </span>
                <span className="text-base font-extrabold text-primary-accent">
                  ₫{activePayout.amount_vnd.toLocaleString("vi-VN")}
                </span>
              </div>
              <div className="flex items-center justify-between text-text-muted pt-1 border-t border-border-subtle">
                <span>{t("adminPayouts.colCreator", "Creator ID")}:</span>
                <span className="font-mono text-text">
                  {activePayout.creator_id.slice(0, 12)}...
                </span>
              </div>
            </div>

            {actionError && (
              <div className="rounded-lg border border-danger/30 bg-danger/10 p-3 text-sm text-danger font-medium">
                {actionError}
              </div>
            )}

            <form onSubmit={(e) => { void handleFulfill(e); }} className="space-y-4">
              <div>
                <label
                  htmlFor="bank-ref-input"
                  className="block text-xs font-semibold text-text uppercase tracking-wider mb-1.5"
                >
                  {t("adminPayouts.bankRefLabel", "Bank Transaction Reference")}
                </label>
                <Input
                  id="bank-ref-input"
                  value={bankReference}
                  onChange={(e) => setBankReference(e.target.value)}
                  placeholder="e.g. VCB-TRF-20260920-001"
                  required
                  className="font-mono text-sm"
                />
                <span className="text-[11px] text-text-muted mt-1 block">
                  {t(
                    "adminPayouts.bankRefHelp",
                    "The transaction number from your banking portal once the transfer completes.",
                  )}
                </span>
              </div>

              <div>
                <label
                  htmlFor="payout-note-input"
                  className="block text-xs font-semibold text-text uppercase tracking-wider mb-1.5"
                >
                  {t("adminPayouts.noteLabel", "Note (Optional)")}
                </label>
                <Input
                  id="payout-note-input"
                  value={note}
                  onChange={(e) => setNote(e.target.value)}
                  placeholder="e.g. Sent via corporate Vietcombank account"
                />
              </div>

              <div className="flex justify-end gap-3 pt-3 border-t border-border-subtle">
                <Button
                  type="button"
                  variant="outline"
                  onClick={() => setActivePayout(null)}
                  disabled={actionLoading}
                >
                  {t("app.cancel", "Cancel")}
                </Button>
                <Button
                  type="submit"
                  variant="primary"
                  disabled={actionLoading}
                  className="gap-1.5"
                >
                  <Send className="h-3.5 w-3.5" />
                  {actionLoading
                    ? t("adminPayouts.fulfilling", "Recording...")
                    : t("adminPayouts.confirmFulfill", "Confirm Fulfillment")}
                </Button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
}
