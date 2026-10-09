import React, { useId, useRef, useState } from "react";
import { Link } from "@tanstack/react-router";
import {
  ArrowRight,
  CheckCircle2,
  ChevronRight,
  Layers,
  Map as MapIcon,
} from "lucide-react";
import { useTranslation } from "react-i18next";

import { Card } from "@/components/ui/card";
import { MD_UP, useMediaQuery } from "@/hooks/useMediaQuery";
import { cn } from "@/lib/utils";
import {
  useFoundationNext,
  type FoundationPathNode,
} from "../../api/foundation";
import { ProgressBar } from "../Progress/ProgressBar";
import { CardHeading, cardLinkClass } from "./CardHeading";

export interface ReviewPathCardProps {
  /** `due_reviews_count` from /me/dashboard. */
  dueCount: number;
}

type TabKey = "reviews" | "path";

const primaryLink =
  "inline-flex min-h-[44px] items-center justify-center gap-2 rounded-xl bg-primary px-4 text-sm font-semibold text-primary-fg shadow-sm transition-colors hover:bg-primary-hover focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary focus-visible:ring-offset-2 motion-reduce:transition-none";

const textLink =
  "inline-flex min-h-[44px] items-center gap-1 text-sm font-semibold text-primary-accent hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary rounded-md";

function masteryPercent(node: FoundationPathNode): number {
  return Math.round(Math.min(1, Math.max(0, node.score)) * 100);
}

/**
 * Dashboard row 2, right: the review queue and the next Foundation topic in one
 * card. From `md` two tabs pick which one leads (the other stays as a compact
 * link beside it); on a phone both are stacked with one call to action, as in
 * the mobile design. Without a published Foundation path (`/me/foundation/next`
 * 404) it is a reviews-only card and there are no tabs.
 */
export function ReviewPathCard({
  dueCount,
}: ReviewPathCardProps): React.JSX.Element {
  const { t } = useTranslation();
  const next = useFoundationNext();
  const node = next.data ?? null;
  const wide = useMediaQuery(MD_UP);
  const [picked, setPicked] = useState<TabKey | null>(null);
  const tab: TabKey = picked ?? (dueCount > 0 || !node ? "reviews" : "path");
  const baseId = useId();
  const tabRefs = useRef<Record<TabKey, HTMLButtonElement | null>>({
    reviews: null,
    path: null,
  });

  const title = node
    ? t("overview.dashboard.reviewPathTitle")
    : t("dashboard.reviews.title", "Reviews Due");

  const heading = (
    <CardHeading
      title={title}
      action={
        <Link to="/practice" className={cardLinkClass}>
          {t("dashboard.skills.detailsLink", "View details →")}
        </Link>
      }
    />
  );

  const topicLink = (className: string, children: React.ReactNode) =>
    node ? (
      <Link
        to="/foundation/topics/$code"
        params={{ code: node.code }}
        className={className}
      >
        {children}
      </Link>
    ) : null;

  const topicCta = node
    ? node.attempts > 0
      ? t("foundation.next.revisitBtn", "Revisit this topic")
      : t("foundation.next.startBtn", "Start this topic")
    : "";

  // ── Reviews, as the lead block ──────────────────────────────────────────
  const reviewsMain =
    dueCount > 0 ? (
      <div className="space-y-2">
        <p className="text-xl font-bold tracking-tight text-text md:text-2xl">
          {t("dashboard.reviews.dueCount", {
            count: dueCount,
            defaultValue: `${dueCount} words to review`,
          })}
        </p>
        <p className="text-sm text-text-muted">
          {t(
            "dashboard.reviews.readyDesc",
            "Spaced repetition cards are ready for your daily memory review.",
          )}
        </p>
        <Link to="/practice/review" className={cn(primaryLink, "mt-1")}>
          {t("dashboard.reviews.reviewNow", "Review now")}
          <ArrowRight className="h-4 w-4" aria-hidden="true" />
        </Link>
      </div>
    ) : (
      <div className="space-y-1">
        <p className="flex items-start gap-2 text-sm font-medium text-text">
          <CheckCircle2
            className="mt-0.5 h-5 w-5 shrink-0 text-success"
            aria-hidden="true"
          />
          {t(
            "dashboard.reviews.emptyCompact",
            "All caught up! No cards due right now.",
          )}
        </p>
        <Link to="/practice/my-words" className={textLink}>
          {t("dashboard.reviews.vocabLink", "View vocabulary →")}
        </Link>
      </div>
    );

  // ── The next topic, as the lead block or as a compact side card ─────────
  const pathBlock = (lead: boolean, withMastery = true) => {
    if (!node) return null;
    const percent = masteryPercent(node);
    return (
      <div
        className={cn(
          "space-y-2.5",
          !lead &&
            "rounded-xl border border-border-subtle bg-surface-muted/50 p-3.5",
        )}
      >
        <div className="flex items-start gap-3">
          <span
            className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-primary/10 text-primary-accent"
            aria-hidden="true"
          >
            <MapIcon className="h-5 w-5" />
          </span>
          <div className="min-w-0">
            <p className="text-xs font-medium text-text-muted">
              {t("foundation.next.title", "Next in your path")}
            </p>
            <p className="flex flex-wrap items-center gap-2">
              <span
                className={cn(
                  "font-semibold text-text",
                  lead ? "text-base md:text-lg" : "text-sm",
                )}
              >
                {node.label || node.code}
              </span>
              {node.cefr_level && (
                <span className="rounded-md border border-border-subtle bg-surface-card px-1.5 font-mono text-xs font-semibold text-text-muted">
                  {node.cefr_level}
                </span>
              )}
            </p>
          </div>
        </div>
        {lead && (
          <p className="text-sm text-text-muted">
            {t(
              "foundation.next.desc",
              "The first topic you have not mastered yet, in prerequisite order.",
            )}
          </p>
        )}
        {withMastery && (
          <div className="space-y-1">
            <div className="flex justify-between text-xs text-text-muted">
              <span>{t("overview.dashboard.topicMastery")}</span>
              <span className="font-semibold tabular-nums text-text">
                {percent}%
              </span>
            </div>
            <ProgressBar
              value={percent}
              label={t("overview.dashboard.topicMasteryLabel", {
                topic: node.label || node.code,
              })}
            />
          </div>
        )}
        {topicLink(
          lead ? cn(primaryLink, "mt-1") : textLink,
          <>
            {topicCta}
            <ArrowRight className="h-4 w-4" aria-hidden="true" />
          </>,
        )}
      </div>
    );
  };

  const reviewsMini = (
    <Link
      to={dueCount > 0 ? "/practice/review" : "/practice/my-words"}
      className="group flex min-h-[44px] items-center gap-3 rounded-xl border border-border-subtle bg-surface-muted/50 p-3.5 transition-colors hover:border-primary/40 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary motion-reduce:transition-none"
    >
      <span
        className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-primary/10 text-primary-accent"
        aria-hidden="true"
      >
        <Layers className="h-5 w-5" />
      </span>
      <span className="min-w-0 flex-1">
        <span className="block text-xs font-medium text-text-muted">
          {t("dashboard.reviews.title", "Reviews Due")}
        </span>
        <span className="block text-sm font-semibold text-text">
          {dueCount > 0
            ? t("dashboard.reviews.dueCount", { count: dueCount })
            : t("dashboard.reviews.emptyTitle", "All Caught Up!")}
        </span>
      </span>
      <ChevronRight
        className="h-5 w-5 shrink-0 text-text-muted"
        aria-hidden="true"
      />
    </Link>
  );

  // No path: the card is the review queue alone, at every width.
  if (!node) {
    return (
      <Card className="h-full p-4 md:p-5">
        {heading}
        <div className="pt-3">{reviewsMain}</div>
      </Card>
    );
  }

  // Phone: next topic, two figures, one action.
  if (!wide) {
    const percent = masteryPercent(node);
    return (
      <Card className="h-full space-y-3 p-4">
        {heading}
        {pathBlock(false, false)}
        <div className="grid grid-cols-2 gap-2">
          <Link
            to={dueCount > 0 ? "/practice/review" : "/practice/my-words"}
            className="flex min-h-[44px] items-center justify-between gap-2 rounded-xl border border-border-subtle p-3 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary"
          >
            <span className="min-w-0">
              <span className="block text-xs text-text-muted">
                {t("dashboard.reviews.title", "Reviews Due")}
              </span>
              <span className="block text-sm font-bold text-text">
                {t("overview.dashboard.wordsCount", { count: dueCount })}
              </span>
            </span>
            <ChevronRight
              className="h-4 w-4 shrink-0 text-text-muted"
              aria-hidden="true"
            />
          </Link>
          <div className="rounded-xl border border-border-subtle p-3">
            <span className="block text-xs text-text-muted">
              {t("overview.dashboard.topicMastery")}
            </span>
            <span className="block text-sm font-bold tabular-nums text-text">
              {percent}%
            </span>
          </div>
        </div>
        {dueCount > 0 ? (
          <Link to="/practice/review" className={cn(primaryLink, "w-full")}>
            {t("dashboard.reviews.startBtn", "Start Review")}
            <ArrowRight className="h-4 w-4" aria-hidden="true" />
          </Link>
        ) : (
          topicLink(
            cn(primaryLink, "w-full"),
            <>
              {topicCta}
              <ArrowRight className="h-4 w-4" aria-hidden="true" />
            </>,
          )
        )}
      </Card>
    );
  }

  // md and up: tabs pick the lead block.
  const tabs: { key: TabKey; label: string }[] = [
    { key: "reviews", label: t("overview.dashboard.tabReviews") },
    { key: "path", label: t("overview.dashboard.tabPath") },
  ];
  const tabId = (key: TabKey) => `${baseId}-tab-${key}`;
  const panelId = `${baseId}-panel`;

  const onTabKeyDown = (event: React.KeyboardEvent<HTMLButtonElement>) => {
    const order: TabKey[] = ["reviews", "path"];
    const index = order.indexOf(tab);
    let nextKey: TabKey | null = null;
    if (event.key === "ArrowRight") nextKey = order[(index + 1) % 2] ?? null;
    else if (event.key === "ArrowLeft")
      nextKey = order[(index + 1) % 2] ?? null;
    else if (event.key === "Home") nextKey = "reviews";
    else if (event.key === "End") nextKey = "path";
    if (!nextKey) return;
    event.preventDefault();
    setPicked(nextKey);
    tabRefs.current[nextKey]?.focus();
  };

  return (
    <Card className="h-full p-4 md:p-5">
      {heading}
      <div
        role="tablist"
        aria-label={title}
        className="mt-3 inline-grid w-full grid-cols-2 gap-1 rounded-xl bg-surface-muted p-1 sm:w-auto"
      >
        {tabs.map(({ key, label }) => {
          const selected = tab === key;
          return (
            <button
              key={key}
              ref={(el) => {
                tabRefs.current[key] = el;
              }}
              type="button"
              role="tab"
              id={tabId(key)}
              aria-selected={selected}
              aria-controls={panelId}
              tabIndex={selected ? 0 : -1}
              onClick={() => setPicked(key)}
              onKeyDown={onTabKeyDown}
              className={cn(
                "min-h-[44px] rounded-lg px-4 text-sm font-semibold transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary motion-reduce:transition-none",
                selected
                  ? "bg-surface-card text-text shadow-sm"
                  : "text-text-muted hover:text-text",
              )}
            >
              {label}
            </button>
          );
        })}
      </div>
      <div
        role="tabpanel"
        id={panelId}
        aria-labelledby={tabId(tab)}
        tabIndex={0}
        className="@container mt-4 rounded-lg focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary"
      >
        <div className="grid gap-4 @md:grid-cols-[minmax(0,1.15fr)_minmax(0,1fr)] @md:items-start">
          {tab === "reviews" ? (
            <>
              {reviewsMain}
              {pathBlock(false)}
            </>
          ) : (
            <>
              {pathBlock(true)}
              {reviewsMini}
            </>
          )}
        </div>
      </div>
    </Card>
  );
}
