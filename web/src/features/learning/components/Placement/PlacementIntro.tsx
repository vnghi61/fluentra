import React, { useState } from "react";
import { Link } from "@tanstack/react-router";
import { Clock, Compass, ListChecks, MoveRight, PenLine } from "lucide-react";
import { useTranslation } from "react-i18next";

import { Button } from "@/components/ui/button";
import {
  placementApi,
  placementProblemCode,
  type PlacementSession,
} from "../../api/placement";

export interface PlacementIntroProps {
  /** Shown above the start button, e.g. after a test ended too early to place. */
  notice?: string | undefined;
  retakeAvailableAt?: string | null | undefined;
  durationMinutes?: number | undefined;
  duration?: string | number | undefined;
  itemCount?: number | undefined;
  whatHappensAfter?: string | undefined;
  onStarted: (session: PlacementSession) => void;
  /** A session is already in progress: the page should load it. */
  onConflict: () => void;
}

export const PlacementIntro: React.FC<PlacementIntroProps> = ({
  notice,
  retakeAvailableAt,
  durationMinutes,
  duration,
  itemCount,
  whatHappensAfter,
  onStarted,
  onConflict,
}) => {
  const { t, i18n } = useTranslation();
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const durationValue =
    durationMinutes !== undefined
      ? t("placement.intro.summaryDuration", {
          minutes: durationMinutes,
          defaultValue: `${durationMinutes} minutes`,
        })
      : duration !== undefined
        ? String(duration)
        : null;

  const itemCountValue =
    itemCount !== undefined
      ? t("placement.intro.summaryItems", {
          count: itemCount,
          defaultValue: `${itemCount} questions`,
        })
      : null;

  const afterValue =
    whatHappensAfter ||
    t(
      "placement.intro.summaryAfter",
      "Places you between A1 and C1 to personalize your lessons.",
    );

  const start = async () => {
    setPending(true);
    setError(null);
    try {
      onStarted(await placementApi.start());
    } catch (err: unknown) {
      switch (placementProblemCode(err)) {
        case "PLACEMENT_IN_PROGRESS":
          onConflict();
          break;
        case "PLACEMENT_RETAKE_TOO_SOON":
          setError(
            t("placement.intro.retakeTooSoon", {
              date: retakeAvailableAt
                ? new Date(retakeAvailableAt).toLocaleDateString(i18n.language)
                : "",
            }),
          );
          break;
        case "PLACEMENT_UNAVAILABLE":
          setError(t("placement.intro.unavailable"));
          break;
        default:
          setError(t("placement.intro.startFailed"));
      }
    } finally {
      setPending(false);
    }
  };

  const facts = [
    { icon: Clock, text: t("placement.intro.time") },
    { icon: ListChecks, text: t("placement.intro.questions") },
    { icon: PenLine, text: t("placement.intro.productive") },
  ];

  return (
    <div className="flex min-h-screen items-center justify-center bg-surface-muted px-3 py-4 sm:px-4 sm:py-8">
      <div className="w-full max-w-xl space-y-4 rounded-2xl border border-border bg-surface-card p-4 shadow-lg sm:p-6 md:p-8">
        <div className="space-y-1.5 text-center">
          <Compass
            className="mx-auto h-8 w-8 text-primary"
            aria-hidden="true"
          />
          <h1 className="text-2xl md:text-3xl font-bold leading-tight tracking-tight text-text">
            {t("placement.intro.title")}
          </h1>
          <p className="text-base leading-relaxed text-text-muted">
            {t("placement.intro.subtitle")}
          </p>
        </div>

        {/* Short summary above the detail */}
        <div
          data-testid="placement-summary"
          className="space-y-1.5 rounded-xl border border-primary/20 bg-primary/5 p-3.5 text-base leading-relaxed"
        >
          <div className="font-semibold text-text">
            {t("placement.intro.summaryTitle", "Overview")}
          </div>
          <ul className="space-y-1 text-text-muted">
            {durationValue && (
              <li className="flex items-center gap-2">
                <Clock
                  className="h-4 w-4 shrink-0 text-primary"
                  aria-hidden="true"
                />
                <span>{durationValue}</span>
              </li>
            )}
            {itemCountValue && (
              <li className="flex items-center gap-2">
                <ListChecks
                  className="h-4 w-4 shrink-0 text-primary"
                  aria-hidden="true"
                />
                <span>{itemCountValue}</span>
              </li>
            )}
            <li className="flex items-center gap-2">
              <Compass
                className="h-4 w-4 shrink-0 text-primary"
                aria-hidden="true"
              />
              <span>{afterValue}</span>
            </li>
            <li className="flex items-center gap-2">
              <MoveRight
                className="h-4 w-4 shrink-0 text-primary"
                aria-hidden="true"
              />
              <span>{t("placement.intro.noGoingBack")}</span>
            </li>
          </ul>
        </div>

        {/* Detail */}
        <details className="group rounded-xl border border-border-subtle bg-surface-muted p-3 text-base leading-relaxed">
          <summary className="flex min-h-11 cursor-pointer select-none items-center justify-between gap-2 font-semibold text-text">
            <span>{t("placement.intro.detailTitle", "Test details")}</span>
            <span className="text-sm text-text-muted group-open:hidden">
              {t("placement.intro.showDetails", "Show")}
            </span>
            <span className="hidden text-sm text-text-muted group-open:inline">
              {t("placement.intro.hideDetails", "Hide")}
            </span>
          </summary>
          <ul className="mt-2.5 space-y-2 border-t border-border-subtle pt-2">
            {facts.map((fact) => (
              <li
                key={fact.text}
                className="flex items-start gap-2.5 text-text"
              >
                <fact.icon
                  className="mt-1 h-4 w-4 shrink-0 text-primary"
                  aria-hidden="true"
                />
                <span>{fact.text}</span>
              </li>
            ))}
          </ul>
        </details>

        {notice && (
          <p className="rounded-lg bg-warning/10 p-3 text-base leading-relaxed text-text">
            {notice}
          </p>
        )}
        {error && (
          <p
            role="alert"
            className="rounded-lg bg-danger/10 p-3 text-base leading-relaxed text-danger-accent"
          >
            {error}
          </p>
        )}

        <div className="space-y-2 pt-1">
          <Button
            type="button"
            onClick={() => void start()}
            disabled={pending}
            size="lg"
            className="w-full"
          >
            {pending
              ? t("placement.intro.starting")
              : t("placement.intro.start")}
          </Button>
          <Link
            to="/"
            className="flex min-h-[44px] w-full items-center justify-center rounded-lg text-sm font-semibold text-text-muted hover:text-text"
          >
            {t("placement.intro.notNow")}
          </Link>
        </div>
      </div>
    </div>
  );
};
