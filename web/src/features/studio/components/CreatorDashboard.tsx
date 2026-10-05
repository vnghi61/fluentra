import React, { useState } from "react";
import { useNavigate } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import {
  AlertCircle,
  ArrowRight,
  BadgeCheck,
  BookOpen,
  Building2,
  Clock,
  Edit3,
  FileCheck,
  History,
  Landmark,
  Layers,
  Plus,
  Receipt,
  Send,
  Sparkles,
  TrendingUp,
  Wallet,
  type LucideIcon,
} from "lucide-react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { formatVnd } from "@/lib/currency";
import { cn } from "@/lib/utils";
import {
  useCreatorDrafts,
  useCreatorEarnings,
  useCreatorProfile,
  useSubmitDraft,
} from "../hooks/useStudio";
import { PayoutAccountModal } from "./PayoutAccountModal";
import { RequestPayoutModal } from "./RequestPayoutModal";
import { SubmissionStatusModal } from "./SubmissionStatusModal";
import type { CourseDraft } from "../api/studioApi";

// Icon tints for the four finance figures; the cards themselves stay neutral.
const FINANCE_TONE = {
  balance:
    "bg-violet-500/10 text-violet-700 dark:bg-violet-400/15 dark:text-violet-300",
  revenue:
    "bg-emerald-500/10 text-emerald-700 dark:bg-emerald-400/15 dark:text-emerald-300",
  paid: "bg-cyan-500/10 text-cyan-700 dark:bg-cyan-400/15 dark:text-cyan-300",
  pending:
    "bg-amber-500/10 text-amber-700 dark:bg-amber-400/15 dark:text-amber-300",
} as const;

interface FinanceStatCardProps {
  icon: LucideIcon;
  tone: keyof typeof FINANCE_TONE;
  label: string;
  value: number;
  helper?: string;
  action?: React.ReactNode;
  testId: string;
}

function FinanceStatCard({
  icon: Icon,
  tone,
  label,
  value,
  helper,
  action,
  testId,
}: FinanceStatCardProps): React.JSX.Element {
  return (
    <Card className="flex min-w-0 flex-col gap-2 rounded-2xl p-3 sm:p-4 animate-in fade-in slide-in-from-bottom-1 duration-300 motion-reduce:animate-none">
      <div className="flex min-w-0 items-center gap-2">
        <span
          className={cn(
            "flex h-8 w-8 shrink-0 items-center justify-center rounded-lg sm:h-9 sm:w-9 sm:rounded-xl",
            FINANCE_TONE[tone],
          )}
          aria-hidden="true"
        >
          <Icon className="h-4 w-4" />
        </span>
        <p className="min-w-0 truncate text-xs font-medium text-text-muted">
          {label}
        </p>
      </div>
      <p
        className="whitespace-nowrap text-xl font-bold tabular-nums tracking-tight text-text sm:text-2xl"
        data-testid={testId}
      >
        {formatVnd(value)}
      </p>
      {helper && (
        <p className="line-clamp-2 text-[11px] leading-snug text-text-muted sm:text-xs">
          {helper}
        </p>
      )}
      {action && <div className="mt-auto pt-1">{action}</div>}
    </Card>
  );
}

function SectionError({
  message,
  onRetry,
}: {
  message: string;
  onRetry: () => void;
}): React.JSX.Element {
  const { t } = useTranslation();
  return (
    <div
      role="alert"
      className="flex flex-col gap-3 rounded-2xl border border-danger/30 bg-danger/5 p-4 text-sm sm:flex-row sm:items-center sm:justify-between"
    >
      <div className="flex items-center gap-3">
        <AlertCircle className="h-5 w-5 shrink-0 text-danger" aria-hidden />
        <span className="font-medium text-text">{message}</span>
      </div>
      <Button
        variant="outline"
        size="sm"
        onClick={onRetry}
        className="min-h-[44px] self-start whitespace-nowrap rounded-xl sm:min-h-10 sm:self-auto"
      >
        {t("action.retry", "Retry")}
      </Button>
    </div>
  );
}

function SectionHeader({
  icon: Icon,
  title,
  count,
}: {
  icon: LucideIcon;
  title: string;
  count?: React.ReactNode;
}): React.JSX.Element {
  return (
    <div className="flex items-center gap-2">
      <Icon className="h-5 w-5 shrink-0 text-primary-accent" aria-hidden />
      <h2 className="min-w-0 text-base font-bold text-text sm:text-lg">
        {title}
      </h2>
      {count}
    </div>
  );
}

export function CreatorDashboard(): React.JSX.Element {
  const { t } = useTranslation();
  const navigate = useNavigate();

  const { data: profile } = useCreatorProfile();
  const {
    data: earnings,
    isLoading: earningsLoading,
    isError: earningsError,
    refetch: refetchEarnings,
  } = useCreatorEarnings();
  const {
    data: draftsData,
    isLoading: draftsLoading,
    isError: draftsError,
    refetch: refetchDrafts,
  } = useCreatorDrafts();
  const submitMutation = useSubmitDraft();

  const [payoutModalOpen, setPayoutModalOpen] = useState(false);
  const [requestPayoutOpen, setRequestPayoutOpen] = useState(false);
  const [selectedDraftForStatus, setSelectedDraftForStatus] =
    useState<CourseDraft | null>(null);
  const [submittingDraftId, setSubmittingDraftId] = useState<string | null>(
    null,
  );

  const drafts = draftsData?.items ?? [];
  const ledger = earnings?.recent_ledger ?? [];
  const canRequestPayout = earnings?.can_request_payout ?? false;
  // The API exposes only configured / not configured; there is no "pending
  // verification" state for a payout account, so the notice has two states.
  // It waits for the earnings query so it never flashes before the data lands.
  const showBankNotice =
    !!earnings && earnings.payout_account_configured === false;
  const goToNewCourse = () => void navigate({ to: "/studio/courses/new" });

  const handleSubmitDraft = async (draftId: string) => {
    setSubmittingDraftId(draftId);
    try {
      await submitMutation.mutateAsync(draftId);
    } catch {
      // Handled by query mutation error
    } finally {
      setSubmittingDraftId(null);
    }
  };

  const getStatusBadge = (status: string) => {
    switch (status) {
      case "published":
      case "approved":
        return (
          <Badge variant="success" className="text-xs">
            {t("studio.status.published", "Published")}
          </Badge>
        );
      case "in_review":
        return (
          <Badge variant="primary" className="text-xs">
            {t("studio.status.inReview", "In Review")}
          </Badge>
        );
      case "verifying":
        return (
          <Badge
            variant="warning"
            className="text-xs animate-pulse motion-reduce:animate-none"
          >
            {t("studio.status.verifying", "Verifying...")}
          </Badge>
        );
      case "changes_requested":
        return (
          <Badge variant="warning" className="text-xs">
            {t("studio.status.changesRequested", "Changes Requested")}
          </Badge>
        );
      case "rejected":
        return (
          <Badge variant="danger" className="text-xs">
            {t("studio.status.rejected", "Rejected")}
          </Badge>
        );
      default:
        return (
          <Badge variant="secondary" className="text-xs">
            {t("studio.status.draft", "Draft")}
          </Badge>
        );
    }
  };

  const bankLabel = earnings?.payout_account_configured
    ? t("studio.payout.editAccount", "Bank Details")
    : t("studio.payout.addAccount", "Setup Bank Account");

  return (
    <div className="mx-auto w-full max-w-[1240px] space-y-5 md:space-y-6">
      {/* Header */}
      <header className="flex flex-col gap-4 lg:flex-row lg:items-end lg:justify-between">
        <div className="min-w-0 space-y-1.5">
          <div className="flex flex-wrap items-center gap-2">
            <span className="inline-flex items-center gap-1.5 rounded-full border border-primary/20 bg-primary/10 px-2.5 py-0.5 text-xs font-semibold text-primary-accent">
              <Sparkles className="h-3.5 w-3.5" aria-hidden />
              {t("studio.badge", "Creator Studio")}
            </span>
            {profile?.payout_eligible && (
              <span className="inline-flex items-center gap-1 rounded-full border border-success/20 bg-success/10 px-2.5 py-0.5 text-xs font-semibold text-success">
                <BadgeCheck className="h-3.5 w-3.5" aria-hidden />
                {t("studio.trustedCreator", "Trusted Creator")}
              </span>
            )}
          </div>
          <h1 className="text-[26px] font-bold tracking-tight text-text md:text-3xl lg:text-[34px]">
            {t("studio.dashboardTitle", "Author & Creator Studio")}
          </h1>
          <p className="max-w-[680px] text-sm text-text-muted">
            {profile?.headline ||
              t(
                "studio.dashboardSubtitle",
                "Design engaging syllabuses, publish community courses, and track earnings.",
              )}
          </p>
        </div>

        <div className="grid grid-cols-2 gap-2 sm:flex sm:flex-wrap sm:gap-3 lg:shrink-0">
          <Button
            variant="outline"
            onClick={() => setPayoutModalOpen(true)}
            className="h-11 gap-2 whitespace-nowrap rounded-xl px-3 md:h-10 md:px-4"
            aria-label={bankLabel}
          >
            <Building2 className="h-4 w-4 shrink-0" aria-hidden />
            <span className="sm:hidden">
              {t("studio.payout.accountShort", "Bank")}
            </span>
            <span className="hidden sm:inline">{bankLabel}</span>
          </Button>
          <Button
            variant="primary"
            onClick={goToNewCourse}
            className="h-11 gap-2 whitespace-nowrap rounded-xl px-3 md:h-10 md:px-4"
          >
            <Plus className="h-4 w-4 shrink-0" aria-hidden />
            {t("studio.newCourse", "New Course")}
          </Button>
        </div>
      </header>

      {/* Finance KPIs: 2x2 below xl, one row of four above */}
      {earningsError ? (
        <SectionError
          message={t("studio.earnings.loadError", "Couldn't load earnings.")}
          onRetry={() => void refetchEarnings()}
        />
      ) : earningsLoading || !earnings ? (
        <div
          className="grid grid-cols-2 gap-3 md:gap-4 xl:grid-cols-4"
          data-testid="studio-finance-skeleton"
        >
          {[0, 1, 2, 3].map((i) => (
            <Card key={i} className="space-y-3 rounded-2xl p-3 sm:p-4">
              <Skeleton className="h-4 w-24 max-w-full" />
              <Skeleton className="h-7 w-20" />
              <Skeleton className="h-3 w-28 max-w-full" />
            </Card>
          ))}
        </div>
      ) : (
        <div className="grid grid-cols-2 gap-3 md:gap-4 xl:grid-cols-4">
          <FinanceStatCard
            icon={Wallet}
            tone="balance"
            label={t("studio.earnings.availableBalance", "Available Balance")}
            value={earnings.available_balance_vnd}
            testId="studio-available-balance"
            action={
              <Button
                variant="ghost"
                size="sm"
                disabled={!canRequestPayout}
                onClick={() => setRequestPayoutOpen(true)}
                className="-ml-2 min-h-[44px] gap-1.5 whitespace-nowrap px-2 font-semibold text-primary-accent sm:min-h-10"
              >
                {t("studio.earnings.requestPayout", "Request Payout")}
                <ArrowRight className="h-4 w-4" aria-hidden />
              </Button>
            }
          />
          <FinanceStatCard
            icon={TrendingUp}
            tone="revenue"
            label={t("studio.earnings.lifetime", "Lifetime Earnings")}
            value={earnings.lifetime_earnings_vnd}
            helper={t(
              "studio.earnings.splitNote",
              "70% creator share per purchase",
            )}
            testId="studio-lifetime-earnings"
          />
          <FinanceStatCard
            icon={Landmark}
            tone="paid"
            label={t("studio.earnings.totalPaidOut", "Total Paid Out")}
            value={earnings.total_paid_out_vnd}
            helper={t(
              "studio.earnings.transferredToBank",
              "Direct bank transfers",
            )}
            testId="studio-total-paid-out"
          />
          <FinanceStatCard
            icon={Clock}
            tone="pending"
            label={t("studio.earnings.pendingPayout", "Pending Payout")}
            value={earnings.pending_payout_vnd}
            helper={
              earnings.pending_payout_vnd > 0
                ? t("studio.earnings.inProcessing", "Processing by finance")
                : t("studio.earnings.noPending", "No pending requests")
            }
            testId="studio-pending-payout"
          />
        </div>
      )}

      {/* Bank account notice: only once earnings say no account is set up */}
      {showBankNotice && (
        <div
          className="flex flex-col gap-3 rounded-2xl border border-amber-500/30 bg-amber-50 p-3 text-sm sm:flex-row sm:items-center sm:justify-between sm:p-4 dark:border-amber-400/25 dark:bg-amber-400/10"
          data-testid="studio-bank-notice"
        >
          <div className="flex items-start gap-3 sm:items-center">
            <AlertCircle
              className="mt-0.5 h-5 w-5 shrink-0 text-amber-600 sm:mt-0 dark:text-amber-300"
              aria-hidden
            />
            <span className="font-medium text-text">
              <span className="sm:hidden">
                {t(
                  "studio.payout.missingNoticeShort",
                  "Add a bank account to receive your earnings.",
                )}
              </span>
              <span className="hidden sm:inline">
                {t(
                  "studio.payout.missingNotice",
                  "Configure your bank account to receive earnings payouts once your courses generate sales.",
                )}
              </span>
            </span>
          </div>
          <Button
            variant="outline"
            size="sm"
            onClick={() => setPayoutModalOpen(true)}
            className="min-h-[44px] self-start whitespace-nowrap rounded-xl sm:min-h-10 sm:self-auto"
          >
            {t("studio.payout.configureNow", "Configure Bank Account")}
          </Button>
        </div>
      )}

      {/* Courses & Drafts Section */}
      <section className="space-y-3">
        <SectionHeader
          icon={BookOpen}
          title={t("studio.courses.title", "Authored Courses & Drafts")}
          count={
            !draftsLoading &&
            !draftsError && (
              <span
                className="ml-1 inline-flex min-w-[1.5rem] shrink-0 items-center justify-center rounded-full bg-surface-base px-2 py-0.5 text-xs font-semibold tabular-nums text-text-muted"
                aria-label={t("studio.courses.count", {
                  count: drafts.length,
                  defaultValue: `${drafts.length} courses`,
                })}
                data-testid="studio-course-count"
              >
                {drafts.length}
              </span>
            )
          }
        />

        {draftsError ? (
          <SectionError
            message={t(
              "studio.courses.loadError",
              "Couldn't load your courses.",
            )}
            onRetry={() => void refetchDrafts()}
          />
        ) : draftsLoading ? (
          <div
            className="grid grid-cols-1 gap-3 md:grid-cols-2 md:gap-4 lg:grid-cols-3"
            data-testid="studio-courses-skeleton"
          >
            {[0, 1, 2].map((i) => (
              <Card key={i} className="space-y-3 rounded-2xl p-4">
                <Skeleton className="h-5 w-24" />
                <Skeleton className="h-5 w-3/4" />
                <Skeleton className="h-4 w-full" />
                <Skeleton className="h-9 w-full" />
              </Card>
            ))}
          </div>
        ) : drafts.length === 0 ? (
          <Card className="flex min-h-[220px] flex-col items-center justify-center gap-3 rounded-2xl p-6 text-center md:min-h-[240px]">
            <span
              className="flex h-16 w-16 items-center justify-center rounded-2xl bg-primary/10 text-primary-accent"
              aria-hidden="true"
            >
              <Layers className="h-8 w-8" />
            </span>
            <div className="space-y-1">
              <h3 className="text-base font-bold text-text">
                {t("studio.courses.emptyTitle", "No courses authored yet")}
              </h3>
              <p className="mx-auto max-w-sm text-sm text-text-muted">
                {t(
                  "studio.courses.emptyDesc",
                  "Create your first course with units, lessons, and interactive activities to share your expertise.",
                )}
              </p>
            </div>
            <Button
              variant="primary"
              onClick={goToNewCourse}
              className="h-11 gap-2 whitespace-nowrap rounded-xl md:h-10"
            >
              <Plus className="h-4 w-4" aria-hidden />
              {t("studio.courses.createFirst", "Create First Course")}
            </Button>
          </Card>
        ) : (
          <div className="grid grid-cols-1 gap-3 md:grid-cols-2 md:gap-4 lg:grid-cols-3">
            {drafts.map((draft) => {
              const structure = draft.structure as
                | {
                    units?: Array<{
                      lessons?: Array<{ activities?: unknown[] }>;
                    }>;
                  }
                | undefined;
              const unitsCount = structure?.units?.length ?? 0;
              const lessonsCount =
                structure?.units?.reduce(
                  (acc, u) => acc + (u.lessons?.length ?? 0),
                  0,
                ) ?? 0;

              return (
                <Card
                  key={draft.id}
                  className="flex flex-col justify-between gap-4 rounded-2xl p-4 transition-[transform,box-shadow] duration-200 hover:-translate-y-px hover:shadow-md motion-reduce:transition-none motion-reduce:hover:translate-y-0"
                >
                  <div className="space-y-3">
                    <div className="flex items-start justify-between gap-2">
                      <div className="flex flex-wrap items-center gap-1.5">
                        <Badge variant="primary" className="font-mono text-xs">
                          {draft.cefr_level}
                        </Badge>
                        {draft.price_vnd > 0 ? (
                          <Badge
                            variant="secondary"
                            className="whitespace-nowrap font-mono text-xs font-semibold"
                          >
                            {formatVnd(draft.price_vnd)}
                          </Badge>
                        ) : (
                          <Badge variant="outline" className="text-xs">
                            {t("studio.courses.freeBadge", "Free")}
                          </Badge>
                        )}
                      </div>
                      <div className="shrink-0">
                        {getStatusBadge(draft.status)}
                      </div>
                    </div>

                    <div>
                      <h3 className="line-clamp-1 text-base font-bold text-text">
                        {draft.title}
                      </h3>
                      {draft.description && (
                        <p className="mt-1 line-clamp-2 text-xs text-text-muted">
                          {draft.description}
                        </p>
                      )}
                    </div>

                    <p className="text-xs text-text-muted">
                      {t("studio.courses.unitsCount", {
                        count: unitsCount,
                        defaultValue: `${unitsCount} units`,
                      })}
                      {" · "}
                      {t("studio.courses.lessonsCount", {
                        count: lessonsCount,
                        defaultValue: `${lessonsCount} lessons`,
                      })}
                    </p>
                  </div>

                  <div className="flex items-center gap-2 border-t border-border-subtle pt-3">
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() =>
                        void navigate({ to: `/studio/courses/${draft.id}` })
                      }
                      className="min-h-[44px] flex-1 gap-1.5 whitespace-nowrap rounded-xl text-xs sm:min-h-10"
                    >
                      <Edit3 className="h-3.5 w-3.5" aria-hidden />
                      {t("app.edit", "Edit")}
                    </Button>

                    {draft.status !== "draft" ? (
                      <Button
                        variant="secondary"
                        size="sm"
                        onClick={() => setSelectedDraftForStatus(draft)}
                        className="min-h-[44px] flex-1 gap-1.5 whitespace-nowrap rounded-xl text-xs sm:min-h-10"
                      >
                        <FileCheck className="h-3.5 w-3.5" aria-hidden />
                        {t("studio.courses.viewStatus", "Review Status")}
                      </Button>
                    ) : (
                      <Button
                        variant="primary"
                        size="sm"
                        disabled={submittingDraftId === draft.id}
                        onClick={() => {
                          void handleSubmitDraft(draft.id);
                        }}
                        className="min-h-[44px] flex-1 gap-1.5 whitespace-nowrap rounded-xl text-xs sm:min-h-10"
                      >
                        <Send className="h-3.5 w-3.5" aria-hidden />
                        {t("studio.courses.submit", "Submit")}
                      </Button>
                    )}
                  </div>
                </Card>
              );
            })}
          </div>
        )}
      </section>

      {/* Earnings & ledger history */}
      <section className="space-y-3">
        <SectionHeader
          icon={History}
          title={t(
            "studio.earnings.recentTransactions",
            "Recent Earnings & Ledger",
          )}
        />

        {earningsError ? null : earningsLoading || !earnings ? (
          <Card className="space-y-3 rounded-2xl p-4">
            {[0, 1, 2].map((i) => (
              <Skeleton key={i} className="h-10 w-full" />
            ))}
          </Card>
        ) : ledger.length > 0 ? (
          <Card className="overflow-hidden rounded-2xl p-0">
            {/* Desktop: table */}
            <div className="hidden overflow-x-auto md:block">
              <table className="w-full text-left text-sm">
                <thead className="border-b border-border-subtle bg-surface-base text-xs text-text-muted">
                  <tr>
                    <th className="px-4 py-3 font-medium">
                      {t("studio.ledger.date", "Date")}
                    </th>
                    <th className="px-4 py-3 font-medium">
                      {t("studio.ledger.type", "Type")}
                    </th>
                    <th className="px-4 py-3 text-right font-medium">
                      {t("studio.ledger.amount", "Net Amount")}
                    </th>
                    <th className="px-4 py-3 text-right font-medium">
                      {t("studio.ledger.gross", "Gross")}
                    </th>
                    <th className="px-4 py-3 font-medium">
                      {t("studio.ledger.notes", "Notes")}
                    </th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-border-subtle">
                  {ledger.map((entry) => (
                    <tr key={entry.id} className="hover:bg-surface-base/50">
                      <td className="whitespace-nowrap px-4 py-3 text-text-muted">
                        {formatLedgerDate(entry.created_at)}
                      </td>
                      <td className="whitespace-nowrap px-4 py-3">
                        <LedgerKindBadge kind={entry.kind} />
                      </td>
                      <td
                        className={cn(
                          "whitespace-nowrap px-4 py-3 text-right font-bold tabular-nums",
                          entry.amount_vnd >= 0
                            ? "text-success"
                            : "text-danger",
                        )}
                      >
                        {entry.amount_vnd >= 0 ? "+" : ""}
                        {formatVnd(entry.amount_vnd)}
                      </td>
                      <td className="whitespace-nowrap px-4 py-3 text-right tabular-nums text-text-muted">
                        {formatVnd(entry.gross_amount_vnd)}
                      </td>
                      <td className="max-w-xs truncate px-4 py-3 text-text-muted">
                        {entry.note}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>

            {/* Mobile: stacked rows */}
            <ul
              className="divide-y divide-border-subtle md:hidden"
              data-testid="studio-ledger-list"
            >
              {ledger.map((entry) => (
                <li key={entry.id} className="flex items-start gap-3 p-3">
                  <span
                    className="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-surface-base text-text-muted"
                    aria-hidden="true"
                  >
                    <Receipt className="h-4 w-4" />
                  </span>
                  <div className="min-w-0 flex-1 space-y-1">
                    <div className="flex items-center justify-between gap-2">
                      <LedgerKindBadge kind={entry.kind} />
                      <span
                        className={cn(
                          "whitespace-nowrap text-sm font-bold tabular-nums",
                          entry.amount_vnd >= 0
                            ? "text-success"
                            : "text-danger",
                        )}
                      >
                        {entry.amount_vnd >= 0 ? "+" : ""}
                        {formatVnd(entry.amount_vnd)}
                      </span>
                    </div>
                    <p className="text-xs text-text-muted">
                      {formatLedgerDate(entry.created_at)}
                      {entry.note ? ` · ${entry.note}` : ""}
                    </p>
                  </div>
                </li>
              ))}
            </ul>
          </Card>
        ) : (
          <Card className="flex min-h-[160px] flex-col items-center justify-center gap-2 rounded-2xl p-6 text-center">
            <span
              className="flex h-12 w-12 items-center justify-center rounded-2xl bg-emerald-500/10 text-emerald-700 dark:bg-emerald-400/15 dark:text-emerald-300"
              aria-hidden="true"
            >
              <Receipt className="h-6 w-6" />
            </span>
            <h3 className="text-sm font-bold text-text">
              {t("studio.ledger.emptyTitle", "No transactions yet")}
            </h3>
            <p className="max-w-sm text-xs text-text-muted sm:text-sm">
              {t(
                "studio.ledger.emptyDesc",
                "Earnings will appear here once learners enrol in your courses.",
              )}
            </p>
          </Card>
        )}
      </section>

      {/* Modals */}
      <PayoutAccountModal
        isOpen={payoutModalOpen}
        onClose={() => setPayoutModalOpen(false)}
      />

      <RequestPayoutModal
        isOpen={requestPayoutOpen}
        onClose={() => setRequestPayoutOpen(false)}
      />

      {selectedDraftForStatus && (
        <SubmissionStatusModal
          draftId={selectedDraftForStatus.id}
          draftTitle={selectedDraftForStatus.title}
          isOpen={Boolean(selectedDraftForStatus)}
          onClose={() => setSelectedDraftForStatus(null)}
        />
      )}
    </div>
  );
}

function formatLedgerDate(iso: string): string {
  return new Date(iso).toLocaleDateString(undefined, {
    month: "short",
    day: "numeric",
    year: "numeric",
  });
}

function LedgerKindBadge({ kind }: { kind: string }): React.JSX.Element {
  return (
    <Badge
      variant={
        kind === "sale"
          ? "success"
          : kind === "payout"
            ? "primary"
            : "secondary"
      }
      className="capitalize text-[11px]"
    >
      {kind}
    </Badge>
  );
}
