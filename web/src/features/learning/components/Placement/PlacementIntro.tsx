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
  onStarted: (session: PlacementSession) => void;
  /** A session is already in progress: the page should load it. */
  onConflict: () => void;
}

export const PlacementIntro: React.FC<PlacementIntroProps> = ({
  notice,
  retakeAvailableAt,
  onStarted,
  onConflict,
}) => {
  const { t, i18n } = useTranslation();
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string | null>(null);

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
    { icon: MoveRight, text: t("placement.intro.noGoingBack") },
    { icon: PenLine, text: t("placement.intro.productive") },
  ];

  return (
    <div className="flex min-h-screen items-center justify-center bg-surface-base px-4 py-10">
      <div className="w-full max-w-xl space-y-6 rounded-2xl border border-border bg-surface-card p-6 shadow-lg md:p-8">
        <div className="space-y-3 text-center">
          <Compass className="mx-auto h-10 w-10 text-primary" aria-hidden="true" />
          <h1 className="text-2xl font-extrabold text-text md:text-3xl">
            {t("placement.intro.title")}
          </h1>
          <p className="text-base text-text-muted">
            {t("placement.intro.subtitle")}
          </p>
        </div>

        <ul className="space-y-3 rounded-xl border border-border-subtle bg-surface-base p-4">
          {facts.map((fact) => (
            <li key={fact.text} className="flex items-start gap-3 text-sm text-text">
              <fact.icon className="mt-0.5 h-5 w-5 shrink-0 text-primary" aria-hidden="true" />
              <span>{fact.text}</span>
            </li>
          ))}
        </ul>

        {notice && (
          <p className="rounded-lg bg-warning/10 p-3 text-sm text-text">{notice}</p>
        )}
        {error && (
          <p role="alert" className="rounded-lg bg-danger/10 p-3 text-sm text-danger">
            {error}
          </p>
        )}

        <div className="space-y-2">
          <Button
            type="button"
            onClick={() => void start()}
            disabled={pending}
            className="min-h-[44px] w-full text-base"
          >
            {pending ? t("placement.intro.starting") : t("placement.intro.start")}
          </Button>
          <Link
            to="/"
            className="flex min-h-[44px] w-full items-center justify-center rounded-lg text-sm font-medium text-text-muted hover:text-text"
          >
            {t("placement.intro.notNow")}
          </Link>
        </div>
      </div>
    </div>
  );
};
