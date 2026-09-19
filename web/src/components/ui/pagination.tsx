import React from "react";
import { ChevronLeft, ChevronRight } from "lucide-react";
import { useTranslation } from "react-i18next";

import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

/**
 * The pager every admin table uses.
 *
 * Two screens had grown their own footer, and they disagreed: the content list
 * counted pages and showed a range, the learner list showed neither and left
 * "Next" in English. A reader cannot tell where they are from "Previous / Next"
 * alone, which is the whole complaint this component answers.
 *
 * It serves both pagination shapes the API offers, because the two admin
 * endpoints do not agree either. /admin/content takes limit+offset, so any page
 * can be reached and the numbers are clickable. /admin/users is cursor-paged —
 * there is no offset to jump to — so it passes no `onPageSelect` and gets the
 * same position readout with plain steppers. Telling someone they are on page 3
 * of 7 does not require being able to leap to page 6.
 */

const PAGE_WINDOW = 5;

/**
 * The page numbers to render, with `null` standing for a gap.
 *
 * Always includes the first and last page, plus a window around the current one,
 * so the control keeps a stable width instead of growing with the result count.
 */
export function pageWindow(
  page: number,
  pageCount: number,
): Array<number | null> {
  if (pageCount <= PAGE_WINDOW + 2) {
    return Array.from({ length: pageCount }, (_, i) => i + 1);
  }

  const span = Math.floor(PAGE_WINDOW / 2);
  let start = Math.max(2, page - span);
  let end = Math.min(pageCount - 1, page + span);

  // Keep the window the same width at both ends of the range, so stepping to
  // page 1 does not shrink the control and shift everything beside it.
  if (page - span < 2) end = Math.min(pageCount - 1, PAGE_WINDOW);
  if (page + span > pageCount - 1) {
    start = Math.max(2, pageCount - PAGE_WINDOW + 1);
  }

  const pages: Array<number | null> = [1];
  if (start > 2) pages.push(null);
  for (let p = start; p <= end; p += 1) pages.push(p);
  if (end < pageCount - 1) pages.push(null);
  pages.push(pageCount);
  return pages;
}

export interface PaginationProps {
  /** 1-based. */
  page: number;
  pageCount: number;
  /** Rows matching the filters, not rows on this page. */
  total: number;
  /** 1-based index of the first row shown; 0 when the page is empty. */
  rangeFrom: number;
  /** 1-based index of the last row shown. */
  rangeTo: number;
  isBusy?: boolean;
  canPrevious: boolean;
  canNext: boolean;
  onPrevious: () => void;
  onNext: () => void;
  /**
   * Supplied only by callers that can reach an arbitrary page — which means
   * offset paging. Without it the numbers are not rendered at all, rather than
   * rendered dead.
   */
  onPageSelect?: (page: number) => void;
  pageSize?: number;
  pageSizeOptions?: readonly number[];
  onPageSizeChange?: (size: number) => void;
  className?: string;
}

export function Pagination({
  page,
  pageCount,
  total,
  rangeFrom,
  rangeTo,
  isBusy = false,
  canPrevious,
  canNext,
  onPrevious,
  onNext,
  onPageSelect,
  pageSize,
  pageSizeOptions,
  onPageSizeChange,
  className,
}: PaginationProps): React.JSX.Element {
  const { t } = useTranslation();
  const safePageCount = Math.max(1, pageCount);

  return (
    <div
      className={cn(
        "flex flex-col gap-3 border-t border-border-subtle bg-surface-muted px-4 py-3 text-xs text-text-muted sm:flex-row sm:items-center sm:justify-between",
        className,
      )}
    >
      <div className="flex flex-wrap items-center gap-x-4 gap-y-2">
        <span>
          {t("admin.showingRange", { from: rangeFrom, to: rangeTo, total })}
        </span>

        {pageSize !== undefined &&
          pageSizeOptions !== undefined &&
          onPageSizeChange !== undefined && (
            <label className="flex items-center gap-2">
              <span>{t("common.rowsPerPage")}</span>
              <select
                value={pageSize}
                disabled={isBusy}
                onChange={(e) => onPageSizeChange(Number(e.target.value))}
                // 44px tall and 16px type: web/AGENT.md R1 and R2. A 12px
                // select is the control iOS Safari zooms the page for.
                className="min-h-11 rounded-md border border-border-subtle bg-surface-base px-2 py-1 text-base text-text"
              >
                {pageSizeOptions.map((size) => (
                  <option key={size} value={size}>
                    {size}
                  </option>
                ))}
              </select>
            </label>
          )}
      </div>

      <nav
        className="flex items-center gap-1.5"
        aria-label={t("common.pagination")}
      >
        <Button
          type="button"
          variant="outline"
          size="sm"
          onClick={onPrevious}
          disabled={!canPrevious || isBusy}
          aria-label={t("common.previous")}
          className="min-h-11 px-3"
        >
          <ChevronLeft className="mr-1 h-3.5 w-3.5" />
          <span className="hidden sm:inline">{t("common.previous")}</span>
        </Button>

        {onPageSelect === undefined ? (
          <span className="px-2 tabular-nums">
            {t("admin.pageOf", { current: page, total: safePageCount })}
          </span>
        ) : (
          <ol className="flex items-center gap-1">
            {pageWindow(page, safePageCount).map((entry, index) =>
              entry === null ? (
                <li
                  key={`gap-${index}`}
                  aria-hidden="true"
                  className="px-1 text-text-muted"
                >
                  …
                </li>
              ) : (
                <li key={entry}>
                  <Button
                    type="button"
                    variant={entry === page ? "primary" : "outline"}
                    size="sm"
                    onClick={() => onPageSelect(entry)}
                    disabled={isBusy}
                    aria-label={t("common.goToPage", { page: entry })}
                    aria-current={entry === page ? "page" : undefined}
                    className="min-h-11 min-w-11 tabular-nums"
                  >
                    {entry}
                  </Button>
                </li>
              ),
            )}
          </ol>
        )}

        <Button
          type="button"
          variant="outline"
          size="sm"
          onClick={onNext}
          disabled={!canNext || isBusy}
          aria-label={t("common.next")}
          className="min-h-11 px-3"
        >
          <span className="hidden sm:inline">{t("common.next")}</span>
          <ChevronRight className="ml-1 h-3.5 w-3.5" />
        </Button>
      </nav>
    </div>
  );
}

export default Pagination;
