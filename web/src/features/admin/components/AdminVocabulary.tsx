import React, { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import {
  AlertCircle,
  BookA,
  CheckCircle2,
  Edit3,
  Inbox,
  Search,
  Sparkles,
  Trash2,
  X,
} from "lucide-react";
import {
  adminApi,
  type AdminWordSummary,
  type LearnerWordQueueItem,
} from "../api/adminApi";
import { AdminEditWordSenseModal } from "./AdminEditWordSenseModal";
import { PERMISSIONS, usePermissions } from "../model/permissions";
import { CopyableId } from "./CopyableId";
import { StatusBadge, type StatusTone } from "./StatusBadge";
import {
  AdminActiveFilters,
  AdminDataPanel,
  AdminFilterCard,
  AdminFilterChip,
  AdminListSkeleton,
  AdminNotice,
  AdminRefreshButton,
  AdminRowIcon,
  AdminSubTabs,
  AdminTag,
  adminTable,
} from "./AdminSurface";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { EmptyState } from "@/components/ui/empty-state";
import { ErrorState } from "@/components/ui/error-state";
import { Pagination } from "@/components/ui/pagination";
import { cn } from "@/lib/utils";
import { NativeSelect } from "@/components/ui/native-select";

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
  const [confirmWithdraw, setConfirmWithdraw] = useState<
    | { kind: "word"; word: AdminWordSummary }
    | { kind: "sense"; item: LearnerWordQueueItem }
    | null
  >(null);

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

  const wordsError = wordsQuery.error
    ? message(wordsQuery.error, t("admin.failedToLoadWords"))
    : null;
  const queueError = queueQuery.error
    ? message(queueQuery.error, t("admin.failedToLoadQueue"))
    : null;

  // Withdraw a word from the shared dictionary.
  const handleWithdrawWord = async (word: AdminWordSummary) => {
    setConfirmWithdraw(null);
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
    setConfirmWithdraw(null);
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

  const queueTone = (status: string): StatusTone => {
    switch (status) {
      case "verified":
        return "success";
      case "pending":
      case "queued":
        return "warning";
      case "rejected":
      case "failed":
        return "danger";
      default:
        return "neutral";
    }
  };
  const queueBadge = (status: string) => (
    <StatusBadge tone={queueTone(status)}>
      <span
        aria-hidden="true"
        className="h-1.5 w-1.5 shrink-0 rounded-full bg-current"
      />
      {t(`admin.queueStatus.${status}`, status)}
    </StatusBadge>
  );
  const sourceBadge = (uploaded: boolean) => (
    <StatusBadge tone={uploaded ? "warning" : "neutral"}>
      {uploaded ? t("admin.sourceUpload") : t("admin.sourceSeed")}
    </StatusBadge>
  );
  const canEdit = can(PERMISSIONS.contentEdit);
  const sourceLabel = (value: string) =>
    value === "seed"
      ? t("admin.seedOnly")
      : value === "upload"
        ? t("admin.uploadsOnly")
        : t("admin.allSources");
  const queueStatusLabel = (value: string) =>
    t(`admin.status${value.charAt(0).toUpperCase()}${value.slice(1)}`, value);
  const withdrawWordButton = (word: AdminWordSummary) => (
    <Button
      type="button"
      size="sm"
      variant="ghost"
      className="text-danger-accent hover:bg-danger/10"
      onClick={() => setConfirmWithdraw({ kind: "word", word })}
      title={t("admin.withdrawWordTitle")}
    >
      <Trash2 aria-hidden="true" className="h-4 w-4" />
      {t("admin.withdraw")}
    </Button>
  );
  const queueActions = (item: LearnerWordQueueItem, stretch = false) =>
    item.word_sense_id && canEdit ? (
      <div
        className={cn(
          "flex items-center gap-2",
          stretch ? "w-full" : "justify-end",
        )}
      >
        <Button
          type="button"
          size="sm"
          variant="outline"
          className={cn(stretch && "flex-1")}
          onClick={() => setEditingQueueItem(item)}
          title={t("admin.editEntryTitle")}
        >
          <Edit3 aria-hidden="true" className="h-4 w-4" />
          {t("admin.editEntry")}
        </Button>
        <Button
          type="button"
          size="icon"
          variant="ghost"
          aria-label={t("admin.withdrawSenseTitle")}
          title={t("admin.withdrawSenseTitle")}
          className="text-danger-accent hover:bg-danger/10"
          onClick={() => setConfirmWithdraw({ kind: "sense", item })}
        >
          <Trash2 aria-hidden="true" className="h-4 w-4" />
        </Button>
      </div>
    ) : null;
  const searchBox = (
    value: string,
    onChange: (next: string) => void,
    label: string,
  ) => (
    <div className="relative col-span-3 w-full sm:min-w-0 sm:flex-1">
      <Search
        aria-hidden="true"
        className="pointer-events-none absolute left-3.5 top-3.5 h-4 w-4 text-text-muted"
      />
      <Input
        type="search"
        value={value}
        onChange={(event) => onChange(event.target.value)}
        aria-label={label}
        placeholder={label}
        className="bg-surface-muted pl-10"
      />
    </div>
  );
  const searchButton = (
    <Button
      type="submit"
      className="min-w-11 gap-1.5 whitespace-nowrap px-2 sm:flex-none sm:gap-2 sm:px-3"
    >
      <Search aria-hidden="true" className="h-4 w-4" />
      {t("admin.search")}
    </Button>
  );

  return (
    <div className="space-y-4">
      <AdminSubTabs
        label={t("page.vocabulary", "Vocabulary")}
        active={activeTab}
        onChange={setActiveTab}
        tabs={[
          {
            key: "words",
            label: t("admin.dictionaryWords"),
            Icon: BookA,
            ...(activeTab === "words" ? { count: wordsTotal } : {}),
          },
          {
            key: "queue",
            label: t("admin.learnerQueue"),
            Icon: Inbox,
            ...(activeTab === "queue" ? { count: queueTotal } : {}),
          },
        ]}
      />

      {actionSuccess && (
        <AdminNotice tone="success">
          <span className="flex items-center gap-2">
            <CheckCircle2 aria-hidden="true" className="h-5 w-5 shrink-0" />
            {actionSuccess}
          </span>
        </AdminNotice>
      )}

      {actionError && (
        <AdminNotice tone="danger">
          <span className="flex items-center gap-2">
            <AlertCircle aria-hidden="true" className="h-5 w-5 shrink-0" />
            {actionError}
          </span>
          <Button
            type="button"
            variant="ghost"
            size="icon"
            aria-label={t("app.close", "Close")}
            onClick={() => setActionError(null)}
          >
            <X aria-hidden="true" className="h-4 w-4" />
          </Button>
        </AdminNotice>
      )}

      {activeTab === "words" && (
        <div className="space-y-4">
          <AdminFilterCard>
            <form
              onSubmit={(e) => {
                e.preventDefault();
                setWordsOffset(0);
                setAppliedWordsSearch(wordsSearch.trim());
              }}
              className="grid grid-cols-[minmax(0,1fr)_minmax(0,1fr)_2.75rem] items-center gap-2 sm:flex sm:flex-wrap sm:gap-3"
            >
              {searchBox(wordsSearch, setWordsSearch, t("admin.searchByLemma"))}
              <NativeSelect
                value={wordsSource}
                aria-label={t("admin.source")}
                onChange={(e) => {
                  setWordsSource(e.target.value);
                  setWordsOffset(0);
                }}
                wrapperClassName="sm:w-auto"
              >
                <option value="">{t("admin.allSources")}</option>
                <option value="seed">{t("admin.seedOnly")}</option>
                <option value="upload">{t("admin.uploadsOnly")}</option>
              </NativeSelect>
              {searchButton}
              <AdminRefreshButton
                onClick={() => void wordsQuery.refetch()}
                busy={wordsQuery.isFetching}
              />
            </form>
            {(appliedWordsSearch || wordsSource) && (
              <AdminActiveFilters
                onReset={() => {
                  setWordsSearch("");
                  setAppliedWordsSearch("");
                  setWordsSource("");
                  setWordsOffset(0);
                }}
              >
                {appliedWordsSearch && (
                  <AdminFilterChip>{appliedWordsSearch}</AdminFilterChip>
                )}
                {wordsSource && (
                  <AdminFilterChip>{sourceLabel(wordsSource)}</AdminFilterChip>
                )}
              </AdminActiveFilters>
            )}
          </AdminFilterCard>

          <AdminDataPanel>
            {isWordsLoading && words.length === 0 ? (
              <AdminListSkeleton label={t("admin.loadingWords")} />
            ) : wordsError ? (
              <ErrorState
                description={wordsError}
                onRetry={() => void wordsQuery.refetch()}
                className="py-10"
              />
            ) : words.length === 0 ? (
              <EmptyState
                icon={BookA}
                title={t("admin.noWordsFound")}
                description={t("admin.tryAdjustingYourSearchQueryOrFilters")}
                className="py-10"
              />
            ) : (
              <>
                <div className="hidden overflow-x-auto md:block">
                  <table className={cn(adminTable.table, "min-w-[900px]")}>
                    <caption className="sr-only">
                      {t("admin.dictionaryWords")}
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
                          {t("admin.lemma")}
                        </th>
                        <th scope="col" className={cn(adminTable.th, "w-32")}>
                          {t("admin.partOfSpeech")}
                        </th>
                        <th scope="col" className={cn(adminTable.th, "w-20")}>
                          {t("admin.cefr")}
                        </th>
                        <th scope="col" className={cn(adminTable.th, "w-40")}>
                          {t("admin.ipa")}
                        </th>
                        <th scope="col" className={cn(adminTable.th, "w-24")}>
                          {t("admin.senses")}
                        </th>
                        <th scope="col" className={cn(adminTable.th, "w-32")}>
                          {t("admin.source")}
                        </th>
                        <th
                          scope="col"
                          className={cn(adminTable.th, "w-36 text-right")}
                        >
                          {t("admin.actions")}
                        </th>
                      </tr>
                    </thead>
                    <tbody className={adminTable.body}>
                      {words.map((w, index) => (
                        <tr key={w.id} className={cn(adminTable.row, "h-16")}>
                          <td
                            className={cn(
                              adminTable.td,
                              "text-right tabular-nums text-text-muted",
                            )}
                          >
                            {wordsOffset + index + 1}
                          </td>
                          <td className={adminTable.td}>
                            <div className="flex min-w-0 items-center gap-3">
                              <AdminRowIcon icon={BookA} />
                              <span
                                title={w.lemma}
                                className="truncate text-sm font-semibold text-text"
                              >
                                {w.lemma}
                              </span>
                            </div>
                          </td>
                          <td
                            className={cn(
                              adminTable.td,
                              "text-sm text-text-muted",
                            )}
                          >
                            {w.pos}
                          </td>
                          <td className={adminTable.td}>
                            <AdminTag>{w.cefr_level}</AdminTag>
                          </td>
                          <td
                            className={cn(
                              adminTable.td,
                              "truncate font-mono text-sm text-text-muted",
                            )}
                          >
                            {w.ipa ? `/${w.ipa}/` : "—"}
                          </td>
                          <td
                            className={cn(
                              adminTable.td,
                              "tabular-nums text-text",
                            )}
                          >
                            {w.senses_count}
                          </td>
                          <td className={adminTable.td}>
                            {sourceBadge(w.is_uploaded)}
                          </td>
                          <td className={cn(adminTable.td, "text-right")}>
                            {canEdit && withdrawWordButton(w)}
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
                <ul
                  data-testid="vocabulary-word-cards"
                  className="divide-y divide-border-subtle md:hidden"
                >
                  {words.map((word) => (
                    <li key={word.id} className="space-y-3 p-4">
                      <div className="flex items-start gap-3">
                        <AdminRowIcon icon={BookA} className="h-10 w-10" />
                        <div className="min-w-0 flex-1">
                          <p className="break-all text-sm font-semibold text-text">
                            {word.lemma}
                          </p>
                          <p className="break-words text-sm text-text-muted">
                            {word.pos}
                            {word.ipa ? ` · /${word.ipa}/` : ""}
                          </p>
                        </div>
                      </div>
                      <div className="flex flex-wrap items-center gap-2">
                        <AdminTag>{word.cefr_level}</AdminTag>
                        {sourceBadge(word.is_uploaded)}
                        <span className="text-sm text-text-muted">
                          {word.senses_count} {t("admin.senses")}
                        </span>
                      </div>
                      {canEdit && (
                        <div className="flex justify-end">
                          {withdrawWordButton(word)}
                        </div>
                      )}
                    </li>
                  ))}
                </ul>
              </>
            )}
            {!wordsError && !(isWordsLoading && words.length === 0) && (
              <Pagination
                page={Math.floor(wordsOffset / wordsLimit) + 1}
                pageCount={Math.max(1, Math.ceil(wordsTotal / wordsLimit))}
                total={wordsTotal}
                rangeFrom={words.length > 0 ? wordsOffset + 1 : 0}
                rangeTo={Math.min(wordsOffset + words.length, wordsTotal)}
                isBusy={wordsQuery.isFetching}
                canPrevious={wordsOffset > 0}
                canNext={wordsOffset + wordsLimit < wordsTotal}
                onPrevious={() =>
                  setWordsOffset((prev) => Math.max(0, prev - wordsLimit))
                }
                onNext={() => setWordsOffset((prev) => prev + wordsLimit)}
                onPageSelect={(next) => setWordsOffset((next - 1) * wordsLimit)}
                className="bg-surface-card"
              />
            )}
          </AdminDataPanel>
        </div>
      )}

      {activeTab === "queue" && (
        <div className="space-y-4">
          <AdminFilterCard>
            <form
              onSubmit={(e) => {
                e.preventDefault();
                setQueueOffset(0);
                setAppliedQueueSearch(queueSearch.trim());
              }}
              className="grid grid-cols-[minmax(0,1fr)_minmax(0,1fr)_2.75rem] items-center gap-2 sm:flex sm:flex-wrap sm:gap-3"
            >
              {searchBox(queueSearch, setQueueSearch, t("admin.searchQueue"))}
              <NativeSelect
                value={queueStatus}
                aria-label={t("admin.filterByStatus")}
                onChange={(e) => {
                  setQueueStatus(e.target.value);
                  setQueueOffset(0);
                }}
                wrapperClassName="sm:w-auto"
              >
                <option value="">{t("admin.allStatusesFilter")}</option>
                <option value="verified">{t("admin.statusVerified")}</option>
                <option value="pending">{t("admin.statusPending")}</option>
                <option value="queued">{t("admin.statusQueued")}</option>
                <option value="rejected">{t("admin.statusRejected")}</option>
                <option value="failed">{t("admin.statusFailed")}</option>
              </NativeSelect>
              {searchButton}
              <AdminRefreshButton
                onClick={() => void queueQuery.refetch()}
                busy={queueQuery.isFetching}
              />
            </form>
            {(appliedQueueSearch || queueStatus) && (
              <AdminActiveFilters
                onReset={() => {
                  setQueueSearch("");
                  setAppliedQueueSearch("");
                  setQueueStatus("");
                  setQueueOffset(0);
                }}
              >
                {appliedQueueSearch && (
                  <AdminFilterChip>{appliedQueueSearch}</AdminFilterChip>
                )}
                {queueStatus && (
                  <AdminFilterChip>
                    {queueStatusLabel(queueStatus)}
                  </AdminFilterChip>
                )}
              </AdminActiveFilters>
            )}
          </AdminFilterCard>

          <AdminDataPanel>
            {isQueueLoading && queueItems.length === 0 ? (
              <AdminListSkeleton label={t("admin.loadingQueue")} />
            ) : queueError ? (
              <ErrorState
                description={queueError}
                onRetry={() => void queueQuery.refetch()}
                className="py-10"
              />
            ) : queueItems.length === 0 ? (
              <EmptyState
                icon={Inbox}
                title={t("admin.noQueueItems")}
                description={t("admin.tryAdjustingYourSearchQueryOrFilters")}
                className="py-10"
              />
            ) : (
              <>
                <div className="hidden overflow-x-auto md:block">
                  <table className={cn(adminTable.table, "min-w-[1040px]")}>
                    <caption className="sr-only">
                      {t("admin.learnerQueue")}
                    </caption>
                    <thead className={adminTable.head}>
                      <tr>
                        <th
                          scope="col"
                          className={cn(adminTable.th, "w-14 text-right")}
                        >
                          {t("common.rowNumber")}
                        </th>
                        <th scope="col" className={cn(adminTable.th, "w-56")}>
                          {t("admin.contributedTerm")}
                        </th>
                        <th scope="col" className={cn(adminTable.th, "w-44")}>
                          {t("admin.providedMeaning")}
                        </th>
                        <th scope="col" className={cn(adminTable.th, "w-44")}>
                          {t("admin.modelVerdict")}
                        </th>
                        <th scope="col" className={adminTable.th}>
                          {t("admin.definitionAndGloss")}
                        </th>
                        <th scope="col" className={cn(adminTable.th, "w-36")}>
                          {t("admin.targetDeck")}
                        </th>
                        <th
                          scope="col"
                          className={cn(adminTable.th, "w-48 text-right")}
                        >
                          {t("common.actions")}
                        </th>
                      </tr>
                    </thead>
                    <tbody className={adminTable.body}>
                      {queueItems.map((item, index) => (
                        <tr
                          key={item.upload_item_id}
                          className={cn(adminTable.row, "align-top")}
                        >
                          <td
                            className={cn(
                              adminTable.td,
                              "py-3 text-right tabular-nums text-text-muted",
                            )}
                          >
                            {queueOffset + index + 1}
                          </td>
                          <td className={cn(adminTable.td, "py-3")}>
                            <div className="flex min-w-0 items-center gap-3">
                              <AdminRowIcon icon={Inbox} />
                              <div className="min-w-0">
                                <p
                                  title={item.term}
                                  className="truncate text-sm font-semibold text-text"
                                >
                                  {item.term}
                                </p>
                                <CopyableId id={item.user_id} />
                              </div>
                            </div>
                          </td>
                          <td
                            className={cn(
                              adminTable.td,
                              "py-3 text-sm text-text",
                            )}
                          >
                            {item.provided_meaning || (
                              <span className="text-text-muted">—</span>
                            )}
                          </td>
                          <td className={cn(adminTable.td, "py-3")}>
                            <div className="space-y-1">
                              {queueBadge(item.status)}
                              {item.verified_by_model && (
                                <p className="flex items-center gap-1 text-sm text-text-muted">
                                  <Sparkles
                                    aria-hidden="true"
                                    className="h-4 w-4 shrink-0 text-warning-accent"
                                  />
                                  <span className="truncate">
                                    {item.verified_by_model}
                                  </span>
                                </p>
                              )}
                              {item.reason && (
                                <p
                                  className="truncate text-sm italic text-text-muted"
                                  title={item.reason}
                                >
                                  {item.reason}
                                </p>
                              )}
                            </div>
                          </td>
                          <td className={cn(adminTable.td, "py-3 text-sm")}>
                            <p className="text-text">
                              {item.definition || "—"}
                            </p>
                            {item.definition_vi && (
                              <p className="mt-0.5 font-medium text-primary-accent">
                                VN: {item.definition_vi}
                              </p>
                            )}
                            {item.topic && (
                              <span className="mt-1 inline-block">
                                <AdminTag>#{item.topic}</AdminTag>
                              </span>
                            )}
                          </td>
                          <td
                            className={cn(
                              adminTable.td,
                              "py-3 text-sm text-text-muted",
                            )}
                          >
                            {item.deck_name || "—"}
                          </td>
                          <td className={cn(adminTable.td, "py-3")}>
                            {queueActions(item)}
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
                <ul
                  data-testid="vocabulary-queue-cards"
                  className="divide-y divide-border-subtle md:hidden"
                >
                  {queueItems.map((item) => (
                    <li key={item.upload_item_id} className="space-y-3 p-4">
                      <div className="flex items-start gap-3">
                        <AdminRowIcon icon={Inbox} className="h-10 w-10" />
                        <div className="min-w-0 flex-1">
                          <p className="break-all text-sm font-semibold text-text">
                            {item.term}
                          </p>
                          <p className="text-sm text-text-muted">
                            {item.provided_meaning || "—"}
                          </p>
                        </div>
                        {queueBadge(item.status)}
                      </div>
                      <details>
                        <summary className="min-h-11 cursor-pointer py-2 text-sm font-medium text-text">
                          {t("admin.definitionAndGloss")}
                        </summary>
                        <p className="break-words text-sm text-text-muted">
                          {item.definition || "—"}
                        </p>
                      </details>
                      <p className="text-sm text-text-muted">
                        {item.deck_name || t("admin.sourceUpload")}
                      </p>
                      {queueActions(item, true)}
                    </li>
                  ))}
                </ul>
              </>
            )}
            {!queueError && !(isQueueLoading && queueItems.length === 0) && (
              <Pagination
                page={Math.floor(queueOffset / queueLimit) + 1}
                pageCount={Math.max(1, Math.ceil(queueTotal / queueLimit))}
                total={queueTotal}
                rangeFrom={queueItems.length > 0 ? queueOffset + 1 : 0}
                rangeTo={Math.min(queueOffset + queueItems.length, queueTotal)}
                isBusy={queueQuery.isFetching}
                canPrevious={queueOffset > 0}
                canNext={queueOffset + queueLimit < queueTotal}
                onPrevious={() =>
                  setQueueOffset((prev) => Math.max(0, prev - queueLimit))
                }
                onNext={() => setQueueOffset((prev) => prev + queueLimit)}
                onPageSelect={(next) => setQueueOffset((next - 1) * queueLimit)}
                className="bg-surface-card"
              />
            )}
          </AdminDataPanel>
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
      <ConfirmDialog
        open={confirmWithdraw !== null}
        onOpenChange={(open) => {
          if (!open) setConfirmWithdraw(null);
        }}
        title={t("admin.withdraw", "Withdraw")}
        description={
          confirmWithdraw?.kind === "word"
            ? t("admin.confirmWithdrawWord", {
                term: confirmWithdraw.word.lemma,
              })
            : confirmWithdraw?.kind === "sense"
              ? t("admin.confirmWithdrawSense", {
                  term: confirmWithdraw.item.term,
                })
              : ""
        }
        cancelLabel={t("common.cancel")}
        confirmLabel={t("admin.withdraw")}
        destructive
        onConfirm={() => {
          if (confirmWithdraw?.kind === "word")
            void handleWithdrawWord(confirmWithdraw.word);
          else if (confirmWithdraw?.kind === "sense")
            void handleWithdrawSense(confirmWithdraw.item);
        }}
      />
    </div>
  );
};
