import React, { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import {
  AlertCircle,
  BookA,
  CheckCircle2,
  Edit3,
  Inbox,
  Loader2,
  RefreshCw,
  Search,
  Sparkles,
  Trash2,
} from "lucide-react";
import {
  adminApi,
  type AdminWordSummary,
  type LearnerWordQueueItem,
} from "../api/adminApi";
import { AdminEditWordSenseModal } from "./AdminEditWordSenseModal";
import { PERMISSIONS, usePermissions } from "../model/permissions";
import { Button } from "@/components/ui/button";

type SubTab = "words" | "queue";

export const AdminVocabulary: React.FC = () => {
  const { t } = useTranslation();
  const { can } = usePermissions();

  const [activeTab, setActiveTab] = useState<SubTab>("words");

  // Dictionary Words Tab State
  const wordsLimit = 15;
  const [wordsOffset, setWordsOffset] = useState(0);
  const [wordsSearch, setWordsSearch] = useState("");
  const [appliedWordsSearch, setAppliedWordsSearch] = useState("");
  const [wordsSource, setWordsSource] = useState("");

  // Learner Queue Tab State
  const queueLimit = 15;
  const [queueOffset, setQueueOffset] = useState(0);
  const [queueSearch, setQueueSearch] = useState("");
  const [appliedQueueSearch, setAppliedQueueSearch] = useState("");
  const [queueStatus, setQueueStatus] = useState("");

  // Edit sense modal state
  const [editingQueueItem, setEditingQueueItem] =
    useState<LearnerWordQueueItem | null>(null);

  // Action status message
  const [actionSuccess, setActionSuccess] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);

  // Both tabs read through useQuery rather than a fetch inside an effect. The
  // filters are the cache key, the inactive tab does not fetch, and a withdrawal
  // refetches by invalidating rather than by calling a callback that a stale
  // closure might be holding.
  const wordsQuery = useQuery({
    queryKey: [
      "admin",
      "vocabulary",
      "words",
      appliedWordsSearch,
      wordsSource,
      wordsLimit,
      wordsOffset,
    ],
    queryFn: () =>
      adminApi.listWords({
        q: appliedWordsSearch || undefined,
        source: wordsSource || undefined,
        limit: wordsLimit,
        offset: wordsOffset,
      }),
    enabled: activeTab === "words",
  });

  const queueQuery = useQuery({
    queryKey: [
      "admin",
      "vocabulary",
      "queue",
      appliedQueueSearch,
      queueStatus,
      queueLimit,
      queueOffset,
    ],
    queryFn: () =>
      adminApi.listLearnerWordsQueue({
        q: appliedQueueSearch || undefined,
        status: queueStatus || undefined,
        limit: queueLimit,
        offset: queueOffset,
      }),
    enabled: activeTab === "queue",
  });

  const words: AdminWordSummary[] = wordsQuery.data?.items ?? [];
  const wordsTotal = wordsQuery.data?.total ?? 0;
  const isWordsLoading = wordsQuery.isLoading;
  const queueItems: LearnerWordQueueItem[] = queueQuery.data?.items ?? [];
  const queueTotal = queueQuery.data?.total ?? 0;
  const isQueueLoading = queueQuery.isLoading;

  const message = (err: unknown, fallback: string): string =>
    err instanceof Error ? err.message : fallback;

  const wordsError =
    actionError ??
    (wordsQuery.error
      ? message(wordsQuery.error, t("admin.failedToLoadWords"))
      : null);
  const queueError =
    actionError ??
    (queueQuery.error
      ? message(queueQuery.error, t("admin.failedToLoadQueue"))
      : null);

  // Withdraw a word from the shared dictionary.
  const handleWithdrawWord = async (word: AdminWordSummary) => {
    if (!window.confirm(t("admin.confirmWithdrawWord", { term: word.lemma }))) {
      return;
    }

    setActionError(null);
    try {
      await adminApi.deleteWord(word.id);
      setActionSuccess(t("admin.withdrewWord", { term: word.lemma }));
      setTimeout(() => setActionSuccess(null), 4000);
      await wordsQuery.refetch();
    } catch (err: unknown) {
      setActionError(message(err, t("admin.failedToWithdrawWord")));
    }
  };

  // Withdraw one sense from the shared dictionary.
  const handleWithdrawSense = async (item: LearnerWordQueueItem) => {
    if (!item.word_sense_id) return;
    if (!window.confirm(t("admin.confirmWithdrawSense", { term: item.term }))) {
      return;
    }

    setActionError(null);
    try {
      await adminApi.deleteWordSense(item.word_sense_id);
      setActionSuccess(t("admin.withdrewSense", { term: item.term }));
      setTimeout(() => setActionSuccess(null), 4000);
      await queueQuery.refetch();
    } catch (err: unknown) {
      setActionError(message(err, t("admin.failedToWithdrawSense")));
    }
  };

  const getQueueStatusBadge = (status: string) => {
    switch (status) {
      case "verified":
        return "bg-emerald-500/10 text-emerald-500 border-emerald-500/20";
      case "pending":
      case "queued":
        return "bg-amber-500/10 text-amber-500 border-amber-500/20";
      case "rejected":
        return "bg-destructive/10 text-destructive border-destructive/20";
      case "failed":
        return "bg-rose-500/10 text-rose-500 border-rose-500/20";
      default:
        return "bg-muted text-muted-foreground";
    }
  };

  return (
    <div className="space-y-4">
      {/* Sub-Tabs Switcher */}
      <div className="flex items-center gap-2 border-b border-border pb-3">
        <Button
          variant={activeTab === "words" ? "primary" : "ghost"}
          size="sm"
          onClick={() => setActiveTab("words")}
          className="gap-2 text-xs"
        >
          <BookA className="w-4 h-4" />
          {t("admin.dictionaryWords")}
          {wordsTotal > 0 && activeTab === "words" && (
            <span className="ml-1 px-1.5 py-0.5 rounded-full bg-primary-foreground/20 text-[10px]">
              {wordsTotal}
            </span>
          )}
        </Button>
        <Button
          variant={activeTab === "queue" ? "primary" : "ghost"}
          size="sm"
          onClick={() => setActiveTab("queue")}
          className="gap-2 text-xs"
        >
          <Inbox className="w-4 h-4" />
          {t("admin.learnerQueue")}
          {queueTotal > 0 && activeTab === "queue" && (
            <span className="ml-1 px-1.5 py-0.5 rounded-full bg-primary-foreground/20 text-[10px]">
              {queueTotal}
            </span>
          )}
        </Button>
      </div>

      {actionSuccess && (
        <div className="p-3 rounded-lg bg-emerald-500/10 border border-emerald-500/20 text-emerald-500 flex items-center gap-2 text-xs">
          <CheckCircle2 className="w-4 h-4 flex-shrink-0" />
          <span>{actionSuccess}</span>
        </div>
      )}

      {/* ========================================================================= */}
      {/* TAB 1: DICTIONARY WORDS */}
      {/* ========================================================================= */}
      {activeTab === "words" && (
        <div className="space-y-4">
          {/* Filter Bar */}
          <div className="flex flex-col sm:flex-row items-stretch sm:items-center justify-between gap-3 p-4 bg-card border border-border rounded-xl">
            <form
              onSubmit={(e) => {
                e.preventDefault();
                setWordsOffset(0);
                setAppliedWordsSearch(wordsSearch.trim());
              }}
              className="relative flex-1"
            >
              <Search className="w-4 h-4 absolute left-3 top-3 text-muted-foreground" />
              <input
                type="text"
                value={wordsSearch}
                onChange={(e) => setWordsSearch(e.target.value)}
                placeholder={t("admin.searchByLemma")}
                className="w-full h-11 pl-9 pr-4 rounded-lg border border-input bg-background text-base focus:outline-none focus:ring-1 focus:ring-ring"
              />
            </form>

            <div className="flex items-center gap-2">
              <select
                value={wordsSource}
                onChange={(e) => {
                  setWordsSource(e.target.value);
                  setWordsOffset(0);
                }}
                className="h-11 px-3 pr-8 rounded-lg border border-input bg-background text-base focus:outline-none focus:ring-1 focus:ring-ring"
              >
                <option value="">{t("admin.allSources")}</option>
                <option value="seed">{t("admin.seedOnly")}</option>
                <option value="upload">{t("admin.uploadsOnly")}</option>
              </select>

              <Button
                variant="outline"
                size="sm"
                onClick={() => void wordsQuery.refetch()}
                disabled={wordsQuery.isFetching}
                className="h-11 w-11 shrink-0 p-0 flex items-center justify-center"
              >
                <RefreshCw
                  className={`w-4 h-4 ${wordsQuery.isFetching ? "animate-spin" : ""}`}
                />
              </Button>
            </div>
          </div>

          {wordsError && (
            <div className="p-4 rounded-lg bg-destructive/10 border border-destructive/20 text-destructive flex items-center gap-3 text-sm">
              <AlertCircle className="w-5 h-5 flex-shrink-0" />
              <span>{wordsError}</span>
            </div>
          )}

          {/* Words Table */}
          <div className="bg-card border border-border rounded-xl overflow-hidden shadow-sm">
            <div className="overflow-x-auto">
              <table className="w-full text-left text-sm">
                <thead className="bg-muted/50 border-b border-border text-xs uppercase text-muted-foreground">
                  <tr>
                    <th className="px-4 py-3 font-medium w-12 text-right">
                      {t("common.rowNumber")}
                    </th>
                    <th className="px-4 py-3 font-medium">
                      {t("admin.lemma")}
                    </th>
                    <th className="px-4 py-3 font-medium">
                      {t("admin.partOfSpeech")}
                    </th>
                    <th className="px-4 py-3 font-medium">{t("admin.cefr")}</th>
                    <th className="px-4 py-3 font-medium">{t("admin.ipa")}</th>
                    <th className="px-4 py-3 font-medium">
                      {t("admin.senses")}
                    </th>
                    <th className="px-4 py-3 font-medium">
                      {t("admin.source")}
                    </th>
                    <th className="px-4 py-3 font-medium text-right">
                      Actions
                    </th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-border">
                  {isWordsLoading && words.length === 0 ? (
                    <tr>
                      <td
                        colSpan={8}
                        className="py-12 text-center text-muted-foreground"
                      >
                        <Loader2 className="w-6 h-6 animate-spin mx-auto mb-2 text-primary" />
                        {t("admin.loadingWords")}
                      </td>
                    </tr>
                  ) : words.length === 0 ? (
                    <tr>
                      <td
                        colSpan={8}
                        className="py-12 text-center text-muted-foreground"
                      >
                        <BookA className="w-8 h-8 mx-auto mb-2 opacity-40" />
                        {t("admin.noWordsFound")}
                      </td>
                    </tr>
                  ) : (
                    words.map((w, index) => (
                      <tr
                        key={w.id}
                        className="hover:bg-muted/30 transition-colors"
                      >
                        <td className="px-4 py-3 text-right tabular-nums text-muted-foreground">
                          {wordsOffset + index + 1}
                        </td>
                        <td className="px-4 py-3">
                          <span className="font-semibold text-foreground font-mono">
                            {w.lemma}
                          </span>
                        </td>
                        <td className="px-4 py-3 text-muted-foreground text-xs">
                          {w.pos}
                        </td>
                        <td className="px-4 py-3">
                          <span className="px-2 py-0.5 rounded text-xs bg-primary/10 text-primary font-medium">
                            {w.cefr_level}
                          </span>
                        </td>
                        <td className="px-4 py-3 font-mono text-xs text-muted-foreground">
                          {w.ipa ? `/${w.ipa}/` : "—"}
                        </td>
                        <td className="px-4 py-3 text-xs">
                          <span className="font-semibold">
                            {w.senses_count}
                          </span>
                        </td>
                        <td className="px-4 py-3">
                          <span
                            className={`text-[11px] px-2 py-0.5 rounded-full font-medium border ${
                              w.is_uploaded
                                ? "bg-amber-500/10 text-amber-500 border-amber-500/20"
                                : "bg-muted text-muted-foreground border-border"
                            }`}
                          >
                            {w.is_uploaded
                              ? t("admin.sourceUpload")
                              : t("admin.sourceSeed")}
                          </span>
                        </td>
                        <td className="px-4 py-3 text-right">
                          {can(PERMISSIONS.contentEdit) && (
                            <Button
                              size="sm"
                              variant="ghost"
                              className="text-destructive hover:bg-destructive/10 min-h-11 px-3 text-xs"
                              onClick={() => void handleWithdrawWord(w)}
                              title={t("admin.withdrawWordTitle")}
                            >
                              <Trash2 className="w-3.5 h-3.5 mr-1" />
                              {t("admin.withdraw")}
                            </Button>
                          )}
                        </td>
                      </tr>
                    ))
                  )}
                </tbody>
              </table>
            </div>

            {/* Pagination */}
            <div className="flex items-center justify-between px-4 py-3 border-t border-border bg-muted/20 text-xs text-muted-foreground">
              <div>
                {t("admin.showingRange", {
                  from: words.length > 0 ? wordsOffset + 1 : 0,
                  to: Math.min(wordsOffset + words.length, wordsTotal),
                  total: wordsTotal,
                })}
              </div>
              <div className="flex items-center gap-2">
                <Button
                  variant="outline"
                  size="sm"
                  disabled={wordsOffset === 0 || isWordsLoading}
                  onClick={() =>
                    setWordsOffset((prev) => Math.max(0, prev - wordsLimit))
                  }
                  className="min-h-11 px-4 text-xs"
                >
                  {t("common.previous")}
                </Button>
                <Button
                  variant="outline"
                  size="sm"
                  disabled={
                    wordsOffset + wordsLimit >= wordsTotal || isWordsLoading
                  }
                  onClick={() => setWordsOffset((prev) => prev + wordsLimit)}
                  className="min-h-11 px-4 text-xs"
                >
                  {t("common.next")}
                </Button>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* ========================================================================= */}
      {/* TAB 2: LEARNER CONTRIBUTIONS QUEUE */}
      {/* ========================================================================= */}
      {activeTab === "queue" && (
        <div className="space-y-4">
          {/* Filter Bar */}
          <div className="flex flex-col sm:flex-row items-stretch sm:items-center justify-between gap-3 p-4 bg-card border border-border rounded-xl">
            <form
              onSubmit={(e) => {
                e.preventDefault();
                setQueueOffset(0);
                setAppliedQueueSearch(queueSearch.trim());
              }}
              className="relative flex-1"
            >
              <Search className="w-4 h-4 absolute left-3 top-3 text-muted-foreground" />
              <input
                type="text"
                value={queueSearch}
                onChange={(e) => setQueueSearch(e.target.value)}
                placeholder={t("admin.searchQueue")}
                className="w-full h-11 pl-9 pr-4 rounded-lg border border-input bg-background text-base focus:outline-none focus:ring-1 focus:ring-ring"
              />
            </form>

            <div className="flex items-center gap-2">
              <select
                value={queueStatus}
                onChange={(e) => {
                  setQueueStatus(e.target.value);
                  setQueueOffset(0);
                }}
                className="h-11 px-3 pr-8 rounded-lg border border-input bg-background text-base focus:outline-none focus:ring-1 focus:ring-ring"
              >
                <option value="">{t("admin.allStatusesFilter")}</option>
                <option value="verified">{t("admin.statusVerified")}</option>
                <option value="pending">{t("admin.statusPending")}</option>
                <option value="queued">{t("admin.statusQueued")}</option>
                <option value="rejected">{t("admin.statusRejected")}</option>
                <option value="failed">{t("admin.statusFailed")}</option>
              </select>

              <Button
                variant="outline"
                size="sm"
                onClick={() => void queueQuery.refetch()}
                disabled={queueQuery.isFetching}
                className="h-11 w-11 shrink-0 p-0 flex items-center justify-center"
              >
                <RefreshCw
                  className={`w-4 h-4 ${queueQuery.isFetching ? "animate-spin" : ""}`}
                />
              </Button>
            </div>
          </div>

          {queueError && (
            <div className="p-4 rounded-lg bg-destructive/10 border border-destructive/20 text-destructive flex items-center gap-3 text-sm">
              <AlertCircle className="w-5 h-5 flex-shrink-0" />
              <span>{queueError}</span>
            </div>
          )}

          {/* Queue Table */}
          <div className="bg-card border border-border rounded-xl overflow-hidden shadow-sm">
            <div className="overflow-x-auto">
              <table className="w-full text-left text-sm">
                <thead className="bg-muted/50 border-b border-border text-xs uppercase text-muted-foreground">
                  <tr>
                    <th className="px-4 py-3 font-medium w-12 text-right">
                      {t("common.rowNumber")}
                    </th>
                    <th className="px-4 py-3 font-medium">
                      {t("admin.contributedTerm")}
                    </th>
                    <th className="px-4 py-3 font-medium">
                      {t("admin.providedMeaning")}
                    </th>
                    <th className="px-4 py-3 font-medium">
                      {t("admin.modelVerdict")}
                    </th>
                    <th className="px-4 py-3 font-medium">
                      {t("admin.definitionAndGloss")}
                    </th>
                    <th className="px-4 py-3 font-medium">
                      {t("admin.targetDeck")}
                    </th>
                    <th className="px-4 py-3 font-medium text-right">
                      {t("common.actions")}
                    </th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-border">
                  {isQueueLoading && queueItems.length === 0 ? (
                    <tr>
                      <td
                        colSpan={7}
                        className="py-12 text-center text-muted-foreground"
                      >
                        <Loader2 className="w-6 h-6 animate-spin mx-auto mb-2 text-primary" />
                        {t("admin.loadingQueue")}
                      </td>
                    </tr>
                  ) : queueItems.length === 0 ? (
                    <tr>
                      <td
                        colSpan={7}
                        className="py-12 text-center text-muted-foreground"
                      >
                        <Inbox className="w-8 h-8 mx-auto mb-2 opacity-40" />
                        {t("admin.noQueueItems")}
                      </td>
                    </tr>
                  ) : (
                    queueItems.map((item, index) => (
                      <tr
                        key={item.upload_item_id}
                        className="hover:bg-muted/30 transition-colors"
                      >
                        <td className="px-4 py-3 text-right tabular-nums text-muted-foreground">
                          {queueOffset + index + 1}
                        </td>
                        <td className="px-4 py-3">
                          <div className="font-semibold text-foreground font-mono">
                            {item.term}
                          </div>
                          <div className="text-[11px] text-muted-foreground font-mono">
                            User: {item.user_id.slice(0, 8)}...
                          </div>
                        </td>
                        <td className="px-4 py-3 text-xs text-foreground max-w-xs">
                          {item.provided_meaning || (
                            <span className="text-muted-foreground italic">
                              —
                            </span>
                          )}
                        </td>
                        <td className="px-4 py-3">
                          <div className="space-y-1">
                            <span
                              className={`text-xs px-2.5 py-0.5 rounded-full font-medium border ${getQueueStatusBadge(
                                item.status,
                              )}`}
                            >
                              {item.status}
                            </span>
                            {item.verified_by_model && (
                              <div className="text-[11px] text-muted-foreground flex items-center gap-1">
                                <Sparkles className="w-3 h-3 text-amber-500" />
                                <span>{item.verified_by_model}</span>
                              </div>
                            )}
                            {item.reason && (
                              <div
                                className="text-[11px] text-muted-foreground italic max-w-xs truncate"
                                title={item.reason}
                              >
                                {item.reason}
                              </div>
                            )}
                          </div>
                        </td>
                        <td className="px-4 py-3 text-xs max-w-sm">
                          <div className="text-foreground">
                            {item.definition || "—"}
                          </div>
                          {item.definition_vi && (
                            <div className="text-primary font-medium mt-0.5">
                              VN: {item.definition_vi}
                            </div>
                          )}
                          {item.topic && (
                            <span className="inline-block mt-1 text-[10px] px-1.5 py-0.2 rounded bg-muted text-muted-foreground font-mono">
                              #{item.topic}
                            </span>
                          )}
                        </td>
                        <td className="px-4 py-3 text-xs text-muted-foreground">
                          {item.deck_name || "—"}
                        </td>
                        <td className="px-4 py-3 text-right">
                          <div className="flex items-center justify-end gap-1">
                            {item.word_sense_id &&
                              can(PERMISSIONS.contentEdit) && (
                                <Button
                                  size="sm"
                                  variant="outline"
                                  className="min-h-11 text-xs px-3"
                                  onClick={() => setEditingQueueItem(item)}
                                  title={t("admin.editEntryTitle")}
                                >
                                  <Edit3 className="w-3.5 h-3.5 mr-1" />
                                  {t("admin.editEntry")}
                                </Button>
                              )}
                            {item.word_sense_id &&
                              can(PERMISSIONS.contentEdit) && (
                                <Button
                                  size="sm"
                                  variant="ghost"
                                  className="min-h-11 min-w-11 text-xs px-3 text-destructive hover:bg-destructive/10"
                                  onClick={() => void handleWithdrawSense(item)}
                                  title={t("admin.withdrawSenseTitle")}
                                >
                                  <Trash2 className="w-3.5 h-3.5" />
                                </Button>
                              )}
                          </div>
                        </td>
                      </tr>
                    ))
                  )}
                </tbody>
              </table>
            </div>

            {/* Pagination */}
            <div className="flex items-center justify-between px-4 py-3 border-t border-border bg-muted/20 text-xs text-muted-foreground">
              <div>
                {t("admin.showingRange", {
                  from: queueItems.length > 0 ? queueOffset + 1 : 0,
                  to: Math.min(queueOffset + queueItems.length, queueTotal),
                  total: queueTotal,
                })}
              </div>
              <div className="flex items-center gap-2">
                <Button
                  variant="outline"
                  size="sm"
                  disabled={queueOffset === 0 || isQueueLoading}
                  onClick={() =>
                    setQueueOffset((prev) => Math.max(0, prev - queueLimit))
                  }
                  className="min-h-11 px-4 text-xs"
                >
                  {t("common.previous")}
                </Button>
                <Button
                  variant="outline"
                  size="sm"
                  disabled={
                    queueOffset + queueLimit >= queueTotal || isQueueLoading
                  }
                  onClick={() => setQueueOffset((prev) => prev + queueLimit)}
                  className="min-h-11 px-4 text-xs"
                >
                  {t("common.next")}
                </Button>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* Edit Word Sense Modal */}
      {editingQueueItem && (
        <AdminEditWordSenseModal
          item={editingQueueItem}
          onClose={() => setEditingQueueItem(null)}
          onSuccess={() => {
            setActionSuccess(
              t("admin.updatedSense", { term: editingQueueItem.term }),
            );
            setTimeout(() => setActionSuccess(null), 4000);
            void queueQuery.refetch();
          }}
        />
      )}
    </div>
  );
};
