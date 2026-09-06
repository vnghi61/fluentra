import React from "react";
import { useTranslation } from "react-i18next";
import { Link } from "@tanstack/react-router";
import { Layers, PlayCircle } from "lucide-react";

import { Button } from "@/components/ui/button";
import { GuestNotice } from "@/features/learning";
import {
  UploadForm,
  UploadList,
  useDecks,
  useUploads,
} from "@/features/vocabulary";
import { useAuthStore } from "@/stores/authStore";

/**
 * A learner's own vocabulary.
 *
 * Paste a list, and an hourly job checks each word against a free dictionary
 * and a model, writes it into a deck of their own, and schedules it for review.
 *
 * The page is deliberately two things and no more: the box you paste into, and
 * what happened to what you pasted. Everything else — the deck, the exercises,
 * the review cards — appears where those things already live rather than being
 * duplicated here.
 */
export function MyWordsPage(): React.JSX.Element {
  const { t } = useTranslation();
  // Uploads belong to a person. Asking for a guest's would earn a 401 on a page
  // they are allowed to be on, which reads as a bug rather than as the honest
  // "there is nothing here for you yet".
  const signedIn = useAuthStore((state) => state.status === "authenticated");
  const uploads = useUploads(signedIn);
  const decks = useDecks(signedIn);

  return (
    <div className="space-y-6 animate-in fade-in duration-200">
      <header className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
        <div className="space-y-1">
          <h1 className="text-2xl md:text-3xl font-extrabold text-text tracking-tight">
            {t("uploads.title", "My words")}
          </h1>
          <p className="text-sm text-text-muted">
            {t(
              "uploads.tagline",
              "Add vocabulary from your own course. We check each word, write example sentences for it, and schedule it for review.",
            )}
          </p>
        </div>
        {signedIn && (
          <div>
            <Link to="/practice/review">
              <Button className="gap-2 bg-primary hover:bg-primary-hover text-surface font-semibold shadow-sm">
                <PlayCircle className="h-4 w-4" aria-hidden="true" />
                {t("uploads.reviewAll", "Review all due words")}
              </Button>
            </Link>
          </div>
        )}
      </header>

      {!signedIn ? (
        <GuestNotice />
      ) : (
        <>
          <section className="rounded-2xl border border-border bg-surface-card p-5 shadow-sm">
            <UploadForm />
          </section>

          {decks.data?.decks && decks.data.decks.length > 0 && (
            <section className="space-y-3">
              <h2 className="text-sm font-semibold uppercase tracking-wide text-text-muted flex items-center gap-2">
                <Layers className="h-4 w-4" />
                {t("uploads.decksTitle", "Decks")}
              </h2>
              <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 gap-3">
                {decks.data.decks.map((deck) => (
                  <div
                    key={deck.id}
                    className="flex flex-col justify-between rounded-xl border border-border bg-surface-card p-4 transition-all hover:border-border-focus hover:shadow-sm"
                  >
                    <div>
                      <h3 className="font-semibold text-text text-sm capitalize">
                        {deck.name}
                      </h3>
                      <div className="mt-2 text-xs font-medium text-text-secondary">
                        {t("uploads.deckWordCount", "{{count}} words", {
                          count: deck.item_count,
                        })}
                      </div>
                    </div>
                    <div className="mt-4 pt-2 border-t border-border/50">
                      <Link
                        to="/practice/review"
                        search={{ deck_id: deck.id }}
                        className="w-full"
                      >
                        <Button
                          variant="outline"
                          size="sm"
                          className="w-full gap-1.5 text-xs font-medium"
                        >
                          <PlayCircle className="h-3.5 w-3.5" />
                          {t("uploads.reviewDeck", "Review deck")}
                        </Button>
                      </Link>
                    </div>
                  </div>
                ))}
              </div>
            </section>
          )}

          <section className="space-y-3">
            <h2 className="text-sm font-semibold uppercase tracking-wide text-text-muted">
              {t("uploads.history", "What you have added")}
            </h2>
            <UploadList
              uploads={uploads.data?.items ?? []}
              isLoading={uploads.isLoading}
            />
          </section>
        </>
      )}
    </div>
  );
}

export default MyWordsPage;
