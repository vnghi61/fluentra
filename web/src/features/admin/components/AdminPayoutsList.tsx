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
  Wallet,
  X,
  XCircle,
} from "lucide-react";

import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { EmptyState } from "@/components/ui/empty-state";
import { ErrorState } from "@/components/ui/error-state";
import { cn } from "@/lib/utils";
import { adminApi, type PayoutResponse } from "../api/adminApi";
import { formatVnd } from "@/lib/currency";
import { CopyableId } from "./CopyableId";
import { StatusBadge } from "./StatusBadge";
import {
  AdminDataPanel,
  AdminListSkeleton,
  AdminRefreshButton,
  AdminRowIcon,
  AdminStatGrid,
  adminStatTones,
  adminTable,
  type AdminStat,
} from "./AdminSurface";

export function AdminPayoutsList(): React.JSX.Element {
  const { t, i18n } = useTranslation();

  const {
    data,
    isLoading: loading,
    isFetching,
    error: queryError,
    refetch,
  } = useQuery({
    queryKey: ["admin", "payouts"],
    queryFn: () => adminApi.listBillingPayouts({ limit: 50 }),
  });

  const payouts = data?.items ?? [];
  const total = data?.total ?? 0;
  const error = queryError
    ? queryError instanceof Error
      ? queryError.message
      : t("adminPayouts.errLoadFailed")
    : null;

  const [activePayout, setActivePayout] = useState<PayoutResponse | null>(null);
  const [bankReference, setBankReference] = useState("");
  const [note, setNote] = useState("");
  const [actionLoading, setActionLoading] = useState(false);
  const [actionError, setActionError] = useState<string | null>(null);
  const [confirmFulfill, setConfirmFulfill] = useState(false);

  const handleFulfill = (e: React.FormEvent) => {
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

    setConfirmFulfill(true);
  };

  const performFulfill = async () => {
    if (!activePayout) return;
    setConfirmFulfill(false);

    setActionLoading(true);
    try {
      await adminApi.fulfillBillingPayout(activePayout.id, {
        bank_reference: bankReference.trim(),
        note: note.trim() || null,
      });
      setActivePayout(null);
      setBankReference("");
      setNote("");
      setConfirmFulfill(false);
      void refetch();
    } catch (err) {
      setActionError(
        err instanceof Error
          ? err.message
          : t(
              "adminPayouts.errFulfillFailed",
              "Failed to record payout fulfillment",
            ),
      );
    } finally {
      setActionLoading(false);
    }
  };

  const getStatusBadge = (status: string) => {
    switch (status) {
      case "sent":
        return (
          <StatusBadge tone="success" icon={CheckCircle2}>
            {t("adminPayouts.statusSent", "Sent / Fulfilled")}
          </StatusBadge>
        );
      case "failed":
        return (
          <StatusBadge tone="danger" icon={XCircle}>
            {t("adminPayouts.statusFailed", "Failed")}
          </StatusBadge>
        );
      default:
        return (
          <StatusBadge tone="warning" icon={Clock}>
            {t("adminPayouts.statusPending", "Pending Review")}
          </StatusBadge>
        );
    }
  };

  const openFulfill = (payout: PayoutResponse) => {
    setActivePayout(payout);
    setBankReference("");
    setNote("");
    setActionError(null);
  };
  const requestedAt = (date: string) => (
    <time
      dateTime={date}
      className="whitespace-nowrap text-sm tabular-nums text-text-muted"
    >
      {new Date(date).toLocaleString(i18n.resolvedLanguage, {
        day: "2-digit",
        month: "short",
        year: "numeric",
        hour: "2-digit",
        minute: "2-digit",
      })}
    </time>
  );
  const fulfillButton = (payout: PayoutResponse, className?: string) =>
    payout.status === "pending" ? (
      <Button
        type="button"
        size="sm"
        onClick={() => openFulfill(payout)}
        className={className}
      >
        <ArrowUpRight className="h-4 w-4" aria-hidden="true" />
        {t("adminPayouts.fulfillBtn", "Fulfill")}
      </Button>
    ) : null;
  const countOf = (status: string) =>
    payouts.filter((payout) => payout.status === status).length;
  const pendingAmount = payouts
    .filter((payout) => payout.status === "pending")
    .reduce((sum, payout) => sum + payout.amount_vnd, 0);
  const stats: AdminStat[] = [
    {
      key: "total",
      label: t("adminPayouts.statTotal"),
      value: total.toLocaleString(i18n.resolvedLanguage),
      Icon: Wallet,
      tone: adminStatTones.primary,
    },
    {
      key: "pending",
      label: t("adminPayouts.statusPending", "Pending Review"),
      value: countOf("pending").toLocaleString(i18n.resolvedLanguage),
      Icon: Clock,
      tone: adminStatTones.warning,
    },
    {
      key: "sent",
      label: t("adminPayouts.statusSent", "Sent / Fulfilled"),
      value: countOf("sent").toLocaleString(i18n.resolvedLanguage),
      Icon: CheckCircle2,
      tone: adminStatTones.success,
    },
    {
      key: "pending-amount",
      label: t("adminPayouts.statPendingAmount"),
      value: formatVnd(pendingAmount),
      Icon: DollarSign,
      tone: adminStatTones.neutral,
    },
  ];

  return (
    <div className="space-y-4">
      <div className="space-y-2">
        <AdminStatGrid
          stats={stats}
          loading={loading}
          label={t("adminPayouts.summary")}
        />
        {!loading && total > payouts.length && (
          <p className="text-sm text-text-muted">
            {t("adminPayouts.statScope", { count: payouts.length })}
          </p>
        )}
      </div>

      <ConfirmDialog
        open={confirmFulfill}
        onOpenChange={setConfirmFulfill}
        title={t("adminPayouts.confirmFulfill", "Confirm Fulfillment")}
        description={
          activePayout
            ? `${formatVnd(activePayout.amount_vnd)} · ${activePayout.creator_id}`
            : ""
        }
        cancelLabel={t("app.cancel", "Cancel")}
        confirmLabel={t("adminPayouts.confirmFulfill", "Confirm Fulfillment")}
        isPending={actionLoading}
        onConfirm={() => void performFulfill()}
      />

      <AdminDataPanel>
        <div className="flex items-center justify-between gap-3 border-b border-border-subtle px-4 py-3">
          <p className="text-sm text-text-muted">
            {t("adminPayouts.count", {
              count: total,
              defaultValue: `${total} payouts total`,
            })}
          </p>
          <AdminRefreshButton
            onClick={() => void refetch()}
            busy={isFetching}
          />
        </div>
        {loading ? (
          <AdminListSkeleton label={t("app.loading", "Loading payouts...")} />
        ) : error ? (
          <ErrorState
            description={error}
            onRetry={() => void refetch()}
            className="py-10"
          />
        ) : payouts.length === 0 ? (
          <EmptyState
            icon={DollarSign}
            title={t("adminPayouts.emptyTitle", "No payouts requested")}
            description={t(
              "adminPayouts.emptyDesc",
              "When creators request earnings withdrawals, they will appear here for processing.",
            )}
            className="py-10"
          />
        ) : (
          <>
            <div className="hidden overflow-x-auto md:block">
              <table className={cn(adminTable.table, "min-w-[880px]")}>
                <caption className="sr-only">
                  {t("adminPayouts.title", "Creator Payouts Management")}
                </caption>
                <thead className={adminTable.head}>
                  <tr>
                    <th
                      scope="col"
                      className={cn(adminTable.th, "w-14 text-right")}
                    >
                      {t("common.rowNumber")}
                    </th>
                    <th scope="col" className={adminTable.th}>
                      {t("adminPayouts.colCreator", "Creator ID")}
                    </th>
                    <th scope="col" className={cn(adminTable.th, "w-40")}>
                      {t("adminPayouts.colAmount", "Amount")}
                    </th>
                    <th scope="col" className={cn(adminTable.th, "w-44")}>
                      {t("adminPayouts.colStatus", "Status")}
                    </th>
                    <th scope="col" className={cn(adminTable.th, "w-48")}>
                      {t("adminPayouts.colReference", "Bank Reference")}
                    </th>
                    <th scope="col" className={cn(adminTable.th, "w-44")}>
                      {t("adminPayouts.colDate", "Requested At")}
                    </th>
                    <th
                      scope="col"
                      className={cn(adminTable.th, "w-36 text-right")}
                    >
                      {t("adminPayouts.colAction", "Action")}
                    </th>
                  </tr>
                </thead>
                <tbody className={adminTable.body}>
                  {payouts.map((payout, index) => (
                    <tr key={payout.id} className={cn(adminTable.row, "h-16")}>
                      <td
                        className={cn(
                          adminTable.td,
                          "text-right tabular-nums text-text-muted",
                        )}
                      >
                        {index + 1}
                      </td>
                      <td className={adminTable.td}>
                        <div className="flex min-w-0 items-center gap-3">
                          <AdminRowIcon icon={Wallet} />
                          <CopyableId id={payout.creator_id} />
                        </div>
                      </td>
                      <td
                        className={cn(
                          adminTable.td,
                          "font-semibold tabular-nums text-text",
                        )}
                      >
                        {formatVnd(payout.amount_vnd)}
                      </td>
                      <td className={adminTable.td}>
                        {getStatusBadge(payout.status)}
                      </td>
                      <td
                        className={cn(
                          adminTable.td,
                          "truncate font-mono text-sm text-text-muted",
                        )}
                        title={payout.bank_reference || undefined}
                      >
                        {payout.bank_reference || "—"}
                      </td>
                      <td className={adminTable.td}>
                        {requestedAt(payout.created_at)}
                      </td>
                      <td className={cn(adminTable.td, "text-right")}>
                        {fulfillButton(payout)}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
            <ul className="divide-y divide-border-subtle md:hidden">
              {payouts.map((payout) => (
                <li key={payout.id} className="space-y-3 p-4">
                  <div className="flex items-start gap-3">
                    <AdminRowIcon icon={Wallet} className="h-10 w-10" />
                    <div className="min-w-0 flex-1">
                      <p className="text-lg font-bold tabular-nums text-text">
                        {formatVnd(payout.amount_vnd)}
                      </p>
                      <CopyableId id={payout.creator_id} />
                    </div>
                    {getStatusBadge(payout.status)}
                  </div>
                  <div className="flex flex-wrap items-center justify-between gap-2">
                    <span className="break-all font-mono text-sm text-text-muted">
                      {payout.bank_reference || "—"}
                    </span>
                    {requestedAt(payout.created_at)}
                  </div>
                  {fulfillButton(payout, "w-full")}
                </li>
              ))}
            </ul>
          </>
        )}
      </AdminDataPanel>

      {/* Fulfill Payout Modal */}
      {activePayout && (
        <div
          role="dialog"
          aria-modal="true"
          aria-labelledby="fulfill-payout-title"
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm p-4 animate-in fade-in duration-200"
        >
          <div className="relative w-full max-w-md space-y-5 rounded-xl border border-border-subtle bg-surface-card p-4 shadow-2xl sm:p-6">
            <div className="flex items-center justify-between gap-2 border-b border-border-subtle pb-4">
              <div className="flex min-w-0 items-center gap-2">
                <Building2
                  aria-hidden="true"
                  className="h-5 w-5 shrink-0 text-primary-accent"
                />
                <h2
                  id="fulfill-payout-title"
                  className="min-w-0 break-words text-lg font-semibold leading-tight text-text"
                >
                  {t(
                    "adminPayouts.fulfillModalTitle",
                    "Fulfill Creator Payout",
                  )}
                </h2>
              </div>
              <button
                type="button"
                onClick={() => setActivePayout(null)}
                aria-label={t("app.close", "Close")}
                className="flex min-h-11 min-w-11 shrink-0 items-center justify-center rounded-lg text-text-muted transition-colors hover:bg-surface-muted hover:text-text"
              >
                <X aria-hidden="true" className="h-5 w-5" />
              </button>
            </div>

            {/* Payout Details */}
            <div className="space-y-2 rounded-lg border border-border-subtle bg-surface-muted p-4 text-sm">
              <div className="flex flex-wrap items-center justify-between gap-2">
                <span className="text-text-muted">
                  {t("adminPayouts.colAmount", "Amount")}:
                </span>
                <span className="text-base font-bold tabular-nums text-primary-accent">
                  {formatVnd(activePayout.amount_vnd)}
                </span>
              </div>
              <div className="flex flex-wrap items-center justify-between gap-2 border-t border-border-subtle pt-1 text-text-muted">
                <span>{t("adminPayouts.colCreator", "Creator ID")}:</span>
                <span className="font-mono text-text">
                  {activePayout.creator_id.slice(0, 12)}...
                </span>
              </div>
            </div>

            {actionError && (
              <div className="rounded-lg border border-danger/30 bg-danger/10 p-3 text-sm font-medium text-danger-accent">
                {actionError}
              </div>
            )}

            <form
              onSubmit={(e) => {
                void handleFulfill(e);
              }}
              className="space-y-4"
            >
              <div>
                <label
                  htmlFor="bank-ref-input"
                  className="mb-1.5 block text-sm font-medium text-text"
                >
                  {t("adminPayouts.bankRefLabel", "Bank Transaction Reference")}
                </label>
                <Input
                  id="bank-ref-input"
                  value={bankReference}
                  onChange={(e) => setBankReference(e.target.value)}
                  placeholder={t(
                    "adminPayouts.bankRefPlaceholder",
                    "e.g. VCB-TRF-20260920-001",
                  )}
                  required
                  className="font-mono"
                />
                <span className="mt-1 block text-sm text-text-muted">
                  {t(
                    "adminPayouts.bankRefHelp",
                    "The transaction number from your banking portal once the transfer completes.",
                  )}
                </span>
              </div>

              <div>
                <label
                  htmlFor="payout-note-input"
                  className="mb-1.5 block text-sm font-medium text-text"
                >
                  {t("adminPayouts.noteLabel", "Note (Optional)")}
                </label>
                <Input
                  id="payout-note-input"
                  value={note}
                  onChange={(e) => setNote(e.target.value)}
                  placeholder={t(
                    "adminPayouts.notePlaceholder",
                    "e.g. Sent via corporate Vietcombank account",
                  )}
                />
              </div>

              <div className="flex flex-wrap justify-end gap-3 border-t border-border-subtle pt-3">
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
                >
                  <Send aria-hidden="true" className="h-4 w-4" />
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
