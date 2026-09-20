import React, { useState } from "react";
import { useNavigate } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import {
  AlertCircle,
  ArrowUpRight,
  BadgeCheck,
  BookOpen,
  Building2,
  Clock,
  DollarSign,
  Edit3,
  FileCheck,
  History,
  Layers,
  Plus,
  Send,
  Sparkles,
  TrendingUp,
} from "lucide-react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
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

export function CreatorDashboard(): React.JSX.Element {
  const { t } = useTranslation();
  const navigate = useNavigate();

  const { data: profile } = useCreatorProfile();
  const { data: earnings } = useCreatorEarnings();
  const { data: draftsData, isLoading: draftsLoading } = useCreatorDrafts();
  const submitMutation = useSubmitDraft("");

  const [payoutModalOpen, setPayoutModalOpen] = useState(false);
  const [requestPayoutOpen, setRequestPayoutOpen] = useState(false);
  const [selectedDraftForStatus, setSelectedDraftForStatus] =
    useState<CourseDraft | null>(null);
  const [submittingDraftId, setSubmittingDraftId] = useState<string | null>(
    null,
  );

  const drafts = draftsData?.items ?? [];
  const availableBalance = earnings?.available_balance_vnd ?? 0;
  const lifetimeEarnings = earnings?.lifetime_earnings_vnd ?? 0;
  const totalPaidOut = earnings?.total_paid_out_vnd ?? 0;
  const pendingPayout = earnings?.pending_payout_vnd ?? 0;
  const canRequestPayout = earnings?.can_request_payout ?? false;

  const handleSubmitDraft = async (draftId: string) => {
    setSubmittingDraftId(draftId);
    try {
      await submitMutation.mutateAsync();
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
          <Badge variant="warning" className="text-xs animate-pulse">
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

  return (
    <div className="max-w-6xl mx-auto space-y-8 py-6">
      {/* Header */}
      <header className="flex flex-col md:flex-row md:items-center justify-between gap-4 border-b border-border-subtle pb-6">
        <div className="space-y-1">
          <div className="flex items-center gap-2">
            <span className="inline-flex items-center gap-1.5 rounded-md bg-primary/10 px-2.5 py-0.5 text-xs font-semibold text-primary-accent border border-primary/20">
              <Sparkles className="h-3.5 w-3.5" />
              {t("studio.badge", "Creator Studio")}
            </span>
            {profile?.payout_eligible && (
              <span className="inline-flex items-center gap-1 rounded-md bg-success/10 px-2 py-0.5 text-xs font-semibold text-success border border-success/20">
                <BadgeCheck className="h-3.5 w-3.5" />
                {t("studio.trustedCreator", "Trusted Creator")}
              </span>
            )}
          </div>
          <h1 className="text-2xl md:text-3xl font-extrabold text-text">
            {t("studio.dashboardTitle", "Author & Creator Studio")}
          </h1>
          <p className="text-sm text-text-muted">
            {profile?.headline ||
              t(
                "studio.dashboardSubtitle",
                "Design engaging syllabuses, publish community courses, and track earnings.",
              )}
          </p>
        </div>

        <div className="flex flex-wrap items-center gap-3">
          <Button
            variant="outline"
            onClick={() => setPayoutModalOpen(true)}
            className="gap-2 text-xs"
          >
            <Building2 className="h-4 w-4" />
            {earnings?.payout_account_configured
              ? t("studio.payout.editAccount", "Bank Details")
              : t("studio.payout.addAccount", "Setup Bank Account")}
          </Button>
          <Button
            variant="primary"
            onClick={() => void navigate({ to: "/studio/courses/new" })}
            className="gap-2"
          >
            <Plus className="h-4 w-4" />
            {t("studio.newCourse", "New Course")}
          </Button>
        </div>
      </header>

      {/* Stats Cards */}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
        {/* Available Balance */}
        <Card className="border-border-subtle bg-surface-card shadow-sm">
          <CardHeader className="p-4 space-y-1">
            <div className="flex items-center justify-between">
              <CardDescription className="text-xs font-medium">
                {t("studio.earnings.availableBalance", "Available Balance")}
              </CardDescription>
              <DollarSign className="h-4 w-4 text-primary-accent" />
            </div>
            <div className="text-2xl font-extrabold text-primary-accent">
              ₫{availableBalance.toLocaleString("vi-VN")}
            </div>
            <div className="pt-2">
              <Button
                variant="outline"
                size="sm"
                disabled={!canRequestPayout}
                onClick={() => setRequestPayoutOpen(true)}
                className="w-full text-xs gap-1.5 font-semibold"
              >
                <ArrowUpRight className="h-3.5 w-3.5" />
                {t("studio.earnings.requestPayout", "Request Payout")}
              </Button>
            </div>
          </CardHeader>
        </Card>

        {/* Lifetime Earnings */}
        <Card className="border-border-subtle bg-surface-card shadow-sm">
          <CardHeader className="p-4 space-y-1">
            <div className="flex items-center justify-between">
              <CardDescription className="text-xs font-medium">
                {t("studio.earnings.lifetime", "Lifetime Earnings")}
              </CardDescription>
              <TrendingUp className="h-4 w-4 text-success" />
            </div>
            <div className="text-2xl font-extrabold text-text">
              ₫{lifetimeEarnings.toLocaleString("vi-VN")}
            </div>
            <p className="text-[11px] text-text-muted pt-2">
              {t("studio.earnings.splitNote", "70% creator share per purchase")}
            </p>
          </CardHeader>
        </Card>

        {/* Total Paid Out */}
        <Card className="border-border-subtle bg-surface-card shadow-sm">
          <CardHeader className="p-4 space-y-1">
            <div className="flex items-center justify-between">
              <CardDescription className="text-xs font-medium">
                {t("studio.earnings.totalPaidOut", "Total Paid Out")}
              </CardDescription>
              <Building2 className="h-4 w-4 text-text-muted" />
            </div>
            <div className="text-2xl font-extrabold text-text">
              ₫{totalPaidOut.toLocaleString("vi-VN")}
            </div>
            <p className="text-[11px] text-text-muted pt-2">
              {t("studio.earnings.transferredToBank", "Direct bank transfers")}
            </p>
          </CardHeader>
        </Card>

        {/* Pending Payout */}
        <Card className="border-border-subtle bg-surface-card shadow-sm">
          <CardHeader className="p-4 space-y-1">
            <div className="flex items-center justify-between">
              <CardDescription className="text-xs font-medium">
                {t("studio.earnings.pendingPayout", "Pending Payout")}
              </CardDescription>
              <Clock className="h-4 w-4 text-warning" />
            </div>
            <div className="text-2xl font-extrabold text-text">
              ₫{pendingPayout.toLocaleString("vi-VN")}
            </div>
            <p className="text-[11px] text-text-muted pt-2">
              {pendingPayout > 0
                ? t("studio.earnings.inProcessing", "Processing by finance")
                : t("studio.earnings.noPending", "No pending requests")}
            </p>
          </CardHeader>
        </Card>
      </div>

      {/* Bank Account Notification */}
      {!earnings?.payout_account_configured && (
        <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3 rounded-xl border border-warning/30 bg-warning/10 p-4 text-sm">
          <div className="flex items-center gap-3">
            <AlertCircle className="h-5 w-5 text-warning shrink-0" />
            <span className="text-text font-medium">
              {t(
                "studio.payout.missingNotice",
                "Configure your bank account to receive earnings payouts once your courses generate sales.",
              )}
            </span>
          </div>
          <Button
            variant="outline"
            size="sm"
            onClick={() => setPayoutModalOpen(true)}
            className="whitespace-nowrap"
          >
            {t("studio.payout.configureNow", "Configure Bank Account")}
          </Button>
        </div>
      )}

      {/* Courses & Drafts Section */}
      <section className="space-y-4">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <BookOpen className="h-5 w-5 text-primary-accent" />
            <h2 className="text-lg font-bold text-text">
              {t("studio.courses.title", "Authored Courses & Drafts")}
            </h2>
          </div>
          <span className="text-xs text-text-muted">
            {t("studio.courses.count", {
              count: drafts.length,
              defaultValue: `${drafts.length} courses`,
            })}
          </span>
        </div>

        {draftsLoading ? (
          <div className="py-12 text-center text-sm text-text-muted">
            {t("app.loading", "Loading drafts...")}
          </div>
        ) : drafts.length === 0 ? (
          <Card className="border-dashed border-border-subtle bg-surface-card/50 p-8 text-center space-y-3">
            <Layers className="h-10 w-10 text-text-muted mx-auto" />
            <div>
              <CardTitle className="text-base font-bold text-text">
                {t("studio.courses.emptyTitle", "No courses authored yet")}
              </CardTitle>
              <CardDescription className="text-xs mt-1 max-w-sm mx-auto">
                {t(
                  "studio.courses.emptyDesc",
                  "Create your first course with units, lessons, and interactive activities to share your expertise.",
                )}
              </CardDescription>
            </div>
            <Button
              variant="primary"
              size="sm"
              onClick={() => void navigate({ to: "/studio/courses/new" })}
              className="gap-2"
            >
              <Plus className="h-4 w-4" />
              {t("studio.courses.createFirst", "Create First Course")}
            </Button>
          </Card>
        ) : (
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
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
                  className="flex flex-col justify-between border-border-subtle bg-surface-card hover:border-primary/40 transition-colors shadow-sm"
                >
                  <CardHeader className="p-5 space-y-3">
                    <div className="flex items-center justify-between gap-2">
                      <div className="flex items-center gap-2">
                        <Badge variant="primary" className="font-mono text-xs">
                          {draft.cefr_level}
                        </Badge>
                        {draft.price_vnd > 0 ? (
                          <Badge variant="secondary" className="font-mono text-xs font-semibold">
                            ₫{draft.price_vnd.toLocaleString("vi-VN")}
                          </Badge>
                        ) : (
                          <Badge variant="outline" className="text-xs">
                            {t("studio.courses.freeBadge", "Free")}
                          </Badge>
                        )}
                      </div>
                      {getStatusBadge(draft.status)}
                    </div>

                    <div>
                      <h3 className="text-base font-bold text-text line-clamp-1">
                        {draft.title}
                      </h3>
                      {draft.description && (
                        <p className="text-xs text-text-muted line-clamp-2 mt-1">
                          {draft.description}
                        </p>
                      )}
                    </div>

                    <div className="flex items-center gap-4 text-xs text-text-muted pt-2 border-t border-border-subtle">
                      <span>
                        {t("studio.courses.unitsCount", {
                          count: unitsCount,
                          defaultValue: `${unitsCount} units`,
                        })}
                      </span>
                      <span>•</span>
                      <span>
                        {t("studio.courses.lessonsCount", {
                          count: lessonsCount,
                          defaultValue: `${lessonsCount} lessons`,
                        })}
                      </span>
                    </div>
                  </CardHeader>

                  <div className="p-5 pt-0 flex flex-wrap items-center justify-between gap-2 border-t border-border-subtle/50 mt-2">
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() =>
                        void navigate({ to: `/studio/courses/${draft.id}` })
                      }
                      className="gap-1.5 text-xs"
                    >
                      <Edit3 className="h-3.5 w-3.5" />
                      {t("app.edit", "Edit")}
                    </Button>

                    {draft.status !== "draft" ? (
                      <Button
                        variant="secondary"
                        size="sm"
                        onClick={() => setSelectedDraftForStatus(draft)}
                        className="gap-1.5 text-xs"
                      >
                        <FileCheck className="h-3.5 w-3.5" />
                        {t("studio.courses.viewStatus", "Review Status")}
                      </Button>
                    ) : (
                      <Button
                        variant="primary"
                        size="sm"
                        disabled={submittingDraftId === draft.id}
                        onClick={() => { void handleSubmitDraft(draft.id); }}
                        className="gap-1.5 text-xs"
                      >
                        <Send className="h-3.5 w-3.5" />
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

      {/* Ledger History Section */}
      <section className="space-y-4">
        <div className="flex items-center gap-2">
          <History className="h-5 w-5 text-primary-accent" />
          <h2 className="text-lg font-bold text-text">
            {t("studio.earnings.recentTransactions", "Recent Earnings & Ledger")}
          </h2>
        </div>

        {earnings?.recent_ledger && earnings.recent_ledger.length > 0 ? (
          <div className="overflow-x-auto rounded-xl border border-border-subtle bg-surface-card">
            <table className="w-full text-left text-xs">
              <thead className="bg-surface-base border-b border-border-subtle text-text-muted uppercase tracking-wider">
                <tr>
                  <th className="p-3">
                    {t("studio.ledger.date", "Date")}
                  </th>
                  <th className="p-3">
                    {t("studio.ledger.type", "Type")}
                  </th>
                  <th className="p-3">
                    {t("studio.ledger.amount", "Net Amount")}
                  </th>
                  <th className="p-3">
                    {t("studio.ledger.gross", "Gross")}
                  </th>
                  <th className="p-3">
                    {t("studio.ledger.notes", "Notes")}
                  </th>
                </tr>
              </thead>
              <tbody className="divide-y divide-border-subtle">
                {earnings.recent_ledger.map((entry) => (
                  <tr key={entry.id} className="hover:bg-surface-base/50">
                    <td className="p-3 whitespace-nowrap text-text-muted">
                      {new Date(entry.created_at).toLocaleDateString(
                        undefined,
                        {
                          month: "short",
                          day: "numeric",
                          year: "numeric",
                        },
                      )}
                    </td>
                    <td className="p-3 whitespace-nowrap">
                      <Badge
                        variant={
                          entry.kind === "sale"
                            ? "success"
                            : entry.kind === "payout"
                              ? "primary"
                              : "secondary"
                        }
                        className="capitalize text-[11px]"
                      >
                        {entry.kind}
                      </Badge>
                    </td>
                    <td
                      className={`p-3 whitespace-nowrap font-bold ${
                        entry.amount_vnd >= 0 ? "text-success" : "text-danger"
                      }`}
                    >
                      {entry.amount_vnd >= 0 ? "+" : ""}₫
                      {entry.amount_vnd.toLocaleString("vi-VN")}
                    </td>
                    <td className="p-3 whitespace-nowrap text-text-muted">
                      ₫{entry.gross_amount_vnd.toLocaleString("vi-VN")}
                    </td>
                    <td className="p-3 text-text-muted max-w-xs truncate">
                      {entry.note}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        ) : (
          <div className="rounded-xl border border-border-subtle bg-surface-card p-6 text-center text-xs text-text-muted">
            {t(
              "studio.ledger.empty",
              "No transactions recorded yet. Earnings will appear here as learners enroll in your courses.",
            )}
          </div>
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
