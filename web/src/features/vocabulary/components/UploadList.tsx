import React, { useEffect, useId, useState } from "react";
import {
  AlertTriangle,
  Check,
  ChevronDown,
  Clock,
  Search,
  X,
} from "lucide-react";
import { useTranslation } from "react-i18next";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Progress } from "@/components/ui/progress";
import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/utils";

import { useDebouncedValue } from "../api/searchApi";
import {
  useUploadItems,
  useUploads,
  type UploadItemFilter,
  type UploadItemStatus,
  type VocabUpload,
  type VocabUploadItem,
} from "../api/uploadApi";

/**
 * What became of a learner's uploads, and a way to find one word among them.
 *
 * The counts lead, because the question after pasting is "did it work" and not
 * "which words". Opening a row lists its words twenty at a time, the ones that
 * need a look first: three hundred words are not worth loading, or scrolling,
 * when the learner wants the four that were not added.
 *
 * The search box answers "did I add this one already?" across every upload,
 * which no amount of opening rows one by one does well.
 */
export interface UploadListProps {
  enabled: boolean;
}

const statusStyles: Record<string, string> = {
  verified: "border-success/30 bg-success/10 text-success-accent",
  queued: "border-success/30 bg-success/10 text-success-accent",
  rejected: "border-danger/30 bg-danger/10 text-danger-accent",
  pending: "border-border bg-surface-muted text-text-muted",
  failed: "border-warning/30 bg-warning/10 text-text",
};

/** Longer than this and the server refuses it; the box stops at the same place. */
const SEARCH_MAX_LENGTH = 100;
const SEARCH_DEBOUNCE_MS = 300;

export const UploadList: React.FC<UploadListProps> = ({ enabled }) => {
  const { t } = useTranslation();
  const searchId = useId();
  const [search, setSearch] = useState("");
  const query = useDebouncedValue(search.trim(), SEARCH_DEBOUNCE_MS);
  const uploads = useUploads(enabled);
  const items = uploads.data?.pages.flatMap((page) => page.items) ?? [];

  return (
    <div className="space-y-4">
      {(items.length > 0 || search !== "") && (
        <div className="space-y-1.5">
          <label htmlFor={searchId} className="sr-only">
            {t("uploads.find.label")}
          </label>
          <div className="relative">
            <Search
              className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-text-muted"
              aria-hidden="true"
            />
            <Input
              id={searchId}
              type="search"
              value={search}
              maxLength={SEARCH_MAX_LENGTH}
              onChange={(event) => setSearch(event.target.value)}
              placeholder={t("uploads.find.placeholder")}
              className="pl-9 pr-10"
              autoComplete="off"
              spellCheck={false}
            />
            {search !== "" && (
              <button
                type="button"
                onClick={() => setSearch("")}
                className="absolute right-0 top-1/2 flex h-11 w-11 -translate-y-1/2 items-center justify-center rounded-md text-text-muted hover:text-text"
                aria-label={t("uploads.find.clear")}
              >
                <X className="h-4 w-4" aria-hidden="true" />
              </button>
            )}
          </div>
        </div>
      )}

      {query !== "" ? (
        <SearchResults query={query} />
      ) : (
        <UploadHistory
          uploads={items}
          isLoading={uploads.isLoading}
          hasMore={uploads.hasNextPage}
          isLoadingMore={uploads.isFetchingNextPage}
          onLoadMore={() => void uploads.fetchNextPage()}
        />
      )}
    </div>
  );
};

const UploadHistory: React.FC<{
  uploads: VocabUpload[];
  isLoading: boolean;
  hasMore: boolean;
  isLoadingMore: boolean;
  onLoadMore: () => void;
}> = ({ uploads, isLoading, hasMore, isLoadingMore, onLoadMore }) => {
  const { t } = useTranslation();

  if (isLoading) {
    return (
      <div className="space-y-3">
        <Skeleton className="h-20 w-full rounded-xl" />
        <Skeleton className="h-20 w-full rounded-xl" />
      </div>
    );
  }

  if (uploads.length === 0) {
    return (
      <p className="rounded-xl border border-dashed border-border p-6 text-center text-base leading-relaxed text-text-muted md:text-sm">
        {t(
          "uploads.empty",
          "Nothing added yet. Paste a list above and we will check it for you.",
        )}
      </p>
    );
  }

  return (
    <div className="space-y-3">
      <ul className="space-y-3">
        {uploads.map((upload) => (
          <UploadRow key={upload.id} upload={upload} />
        ))}
      </ul>
      {hasMore && (
        <Button
          type="button"
          variant="outline"
          className="w-full"
          onClick={onLoadMore}
          isLoading={isLoadingMore}
        >
          {t("uploads.loadOlder")}
        </Button>
      )}
    </div>
  );
};

/** Words that need the learner: not added, or not checked. */
const ATTENTION: readonly UploadItemStatus[] = ["rejected", "failed"];
const ADDED: readonly UploadItemStatus[] = ["verified", "queued"];
const PENDING: readonly UploadItemStatus[] = ["pending"];

type Tab = "attention" | "pending" | "added" | "all";

const tabStatuses: Record<Tab, readonly UploadItemStatus[] | undefined> = {
  attention: ATTENTION,
  pending: PENDING,
  added: ADDED,
  all: undefined,
};

function countsOf(upload: VocabUpload) {
  const attention = upload.rejected_count + (upload.failed_count ?? 0);
  const added = upload.verified_count + (upload.queued_count ?? 0);
  const pending = upload.pending_count;
  return { attention, added, pending, all: upload.item_count };
}

const UploadRow: React.FC<{ upload: VocabUpload }> = ({ upload }) => {
  const { t, i18n } = useTranslation();
  const [open, setOpen] = useState(false);
  const panelId = useId();

  const counts = countsOf(upload);
  const failed = upload.failed_count ?? 0;
  const done = upload.item_count - counts.pending;
  const created = new Date(upload.created_at).toLocaleString(i18n.language);

  return (
    <li className="rounded-xl border border-border bg-surface-card">
      <button
        type="button"
        onClick={() => setOpen((current) => !current)}
        aria-expanded={open}
        aria-controls={panelId}
        className="flex w-full items-center justify-between gap-3 p-4 text-left"
      >
        <div className="min-w-0 flex-1 space-y-2">
          <p className="break-words text-sm font-semibold text-text">
            {upload.item_count} {t("uploads.words", "words")} · {created}
          </p>
          <div className="flex flex-wrap items-center gap-2 text-xs">
            <Count
              icon={<Check className="h-4 w-4" aria-hidden="true" />}
              label={t("uploads.verified", "added")}
              value={counts.added}
              tone="verified"
            />
            {counts.pending > 0 && (
              <Count
                icon={<Clock className="h-4 w-4" aria-hidden="true" />}
                label={t("uploads.checking", "being checked")}
                value={counts.pending}
                tone="pending"
              />
            )}
            {upload.rejected_count > 0 && (
              <Count
                icon={<X className="h-4 w-4" aria-hidden="true" />}
                label={t("uploads.rejected", "not found")}
                value={upload.rejected_count}
                tone="rejected"
              />
            )}
            {failed > 0 && (
              <Count
                icon={<AlertTriangle className="h-4 w-4" aria-hidden="true" />}
                label={t("uploads.failedCount")}
                value={failed}
                tone="failed"
              />
            )}
          </div>
          {counts.pending > 0 && (
            <Progress
              value={done}
              max={upload.item_count}
              aria-label={t("uploads.progress", {
                done,
                total: upload.item_count,
              })}
              className="h-1.5"
            />
          )}
        </div>
        <ChevronDown
          className={cn(
            "h-4 w-4 shrink-0 text-text-muted transition-transform",
            open && "rotate-180",
          )}
          aria-hidden="true"
        />
      </button>

      {open && (
        <div id={panelId} className="border-t border-border-subtle p-4">
          <UploadWords upload={upload} />
        </div>
      )}
    </li>
  );
};

function defaultTab(counts: ReturnType<typeof countsOf>): Tab {
  if (counts.attention > 0) return "attention";
  if (counts.pending > 0) return "pending";
  return "all";
}

/** An opened upload: its words by status, twenty at a time. */
const UploadWords: React.FC<{ upload: VocabUpload }> = ({ upload }) => {
  const { t } = useTranslation();
  const counts = countsOf(upload);
  const [tab, setTab] = useState<Tab>(() => defaultTab(counts));

  const filter: UploadItemFilter = {
    uploadId: upload.id,
    statuses: tabStatuses[tab],
  };
  const words = useUploadItems(filter);

  // The summary row polls; the words follow it. Refetching on every change of
  // the counts keeps the open list honest without a second timer.
  const { refetch } = words;
  const signature = `${counts.added}/${counts.pending}/${counts.attention}`;
  useEffect(() => {
    void refetch();
  }, [signature, refetch]);

  const tabs = (["attention", "pending", "added", "all"] as const).filter(
    (candidate) => candidate === "all" || counts[candidate] > 0,
  );
  const loaded = words.data?.pages.flatMap((page) => page.items) ?? [];
  const remaining = Math.max(0, counts[tab] - loaded.length);

  return (
    <div className="space-y-3">
      {tabs.length > 1 && (
        <div
          role="group"
          aria-label={t("uploads.history", "What you have added")}
          className="flex flex-wrap gap-2"
        >
          {tabs.map((candidate) => (
            <button
              key={candidate}
              type="button"
              aria-pressed={tab === candidate}
              onClick={() => setTab(candidate)}
              className={cn(
                "min-h-11 rounded-full border px-4 text-sm font-medium transition-colors",
                tab === candidate
                  ? "border-primary bg-primary/10 text-primary-accent"
                  : "border-border text-text-muted hover:text-text",
              )}
            >
              {t(`uploads.tabs.${candidate}`)} ({counts[candidate]})
            </button>
          ))}
        </div>
      )}

      {words.isLoading ? (
        <Skeleton className="h-16 w-full rounded-lg" />
      ) : loaded.length === 0 ? (
        <p className="text-sm text-text-muted">{t("uploads.tabEmpty")}</p>
      ) : (
        <ul className="divide-y divide-border-subtle">
          {loaded.map((item) => (
            <WordRow key={item.id ?? item.term} item={item} />
          ))}
        </ul>
      )}

      {words.hasNextPage && (
        <Button
          type="button"
          variant="outline"
          className="w-full"
          onClick={() => void words.fetchNextPage()}
          isLoading={words.isFetchingNextPage}
        >
          {remaining > 0
            ? t("uploads.loadMoreRemaining", { count: remaining })
            : t("uploads.loadMore")}
        </Button>
      )}
    </div>
  );
};

/** Every upload's words matching the search box. */
const SearchResults: React.FC<{ query: string }> = ({ query }) => {
  const { t } = useTranslation();
  const results = useUploadItems({ q: query });
  const found = results.data?.pages.flatMap((page) => page.items) ?? [];

  if (results.isLoading) {
    return <Skeleton className="h-20 w-full rounded-xl" />;
  }
  if (found.length === 0) {
    return (
      <p
        className="rounded-xl border border-dashed border-border p-6 text-center text-sm text-text-muted"
        aria-live="polite"
      >
        {t("uploads.find.empty", { query })}
      </p>
    );
  }
  return (
    <div className="space-y-3">
      <ul className="divide-y divide-border-subtle rounded-xl border border-border bg-surface-card px-4">
        {found.map((item) => (
          <WordRow key={item.id ?? item.term} item={item} showDate />
        ))}
      </ul>
      {results.hasNextPage && (
        <Button
          type="button"
          variant="outline"
          className="w-full"
          onClick={() => void results.fetchNextPage()}
          isLoading={results.isFetchingNextPage}
        >
          {t("uploads.loadMore")}
        </Button>
      )}
    </div>
  );
};

/**
 * One word, one line: the word, its gloss and what became of it. A word that
 * was not added says why on the line itself; the definition and examples of an
 * added one wait behind a tap, because a list of a hundred of them is a page
 * nobody reads.
 */
const WordRow: React.FC<{ item: VocabUploadItem; showDate?: boolean }> = ({
  item,
  showDate = false,
}) => {
  const { t, i18n } = useTranslation();
  const [open, setOpen] = useState(false);
  const detailsId = useId();

  const gloss = item.definition_vi || item.provided_meaning;
  const hasDetails =
    Boolean(item.definition) || (item.examples?.length ?? 0) > 0;
  const note = item.note_code
    ? t(`uploads.note.${item.note_code}`, {
        term: item.term,
        corrected: item.corrected_term ?? "",
        suggested: item.suggested_term ?? "",
        defaultValue: item.reason ?? "",
      })
    : item.status === "failed"
      ? t("uploads.failedHint")
      : item.reason || "";
  // A note on an added word is a nicety; on a word that was not added it is
  // the whole answer, so it is shown either way but coloured only then.
  const needsLook = item.status === "rejected" || item.status === "failed";

  return (
    <li className="py-3">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0 space-y-1 break-words text-sm">
          <p>
            <span className="font-semibold text-text">{item.term}</span>
            {gloss && <span className="text-text-muted"> — {gloss}</span>}
          </p>
          {note && (
            <p
              className={cn(
                "text-sm",
                needsLook
                  ? item.status === "failed"
                    ? "text-warning-accent"
                    : "text-danger-accent"
                  : "text-text-muted",
              )}
            >
              {note}
            </p>
          )}
          {showDate && item.created_at && (
            <p className="text-xs text-text-muted">
              {t("uploads.find.addedOn", {
                date: new Date(item.created_at).toLocaleDateString(
                  i18n.language,
                ),
              })}
            </p>
          )}
          {hasDetails && (
            <button
              type="button"
              onClick={() => setOpen((current) => !current)}
              aria-expanded={open}
              aria-controls={detailsId}
              className="text-sm font-medium text-primary-accent hover:underline"
            >
              {open ? t("uploads.hideDetails") : t("uploads.showDetails")}
            </button>
          )}
          {open && (
            <div id={detailsId} className="space-y-1">
              {item.definition && (
                <p className="text-sm text-text-muted">{item.definition}</p>
              )}
              {(item.examples?.length ?? 0) > 0 && (
                <ul className="space-y-0.5 border-l-2 border-border-subtle pl-2 text-sm text-text-muted">
                  {item.examples!.slice(0, 2).map((example, index) => (
                    <li key={index} className="italic">
                      “{example}”
                    </li>
                  ))}
                </ul>
              )}
            </div>
          )}
        </div>
        <span
          className={cn(
            "shrink-0 rounded-md border px-2 py-0.5 text-xs font-semibold",
            statusStyles[item.status] ?? statusStyles["pending"],
          )}
        >
          {t(`uploads.status.${item.status}`, item.status)}
        </span>
      </div>
    </li>
  );
};

const Count: React.FC<{
  icon: React.ReactNode;
  label: string;
  value: number;
  tone: keyof typeof statusStyles;
}> = ({ icon, label, value, tone }) => (
  <span
    className={cn(
      "inline-flex items-center gap-1 rounded-md border px-2 py-0.5 font-semibold",
      statusStyles[tone],
    )}
  >
    {icon}
    {value} {label}
  </span>
);
