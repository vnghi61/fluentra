import React from "react";
import { useTranslation } from "react-i18next";
import { Link } from "@tanstack/react-router";
import { Layers, PlayCircle } from "lucide-react";

import { PageHeader } from "@/components/layout/PageHeader";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
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
  const deckList = decks.data?.decks ?? [];

  return (
    <div className="mx-auto w-full max-w-[1440px] space-y-6 animate-in fade-in duration-200 motion-reduce:animate-none md:space-y-8">
      <PageHeader
        back={{ to: "/practice", label: t("practice.title", "Practice") }}
        title={t("uploads.title", "My words")}
        description={t(
          "uploads.tagline",
          "Add vocabulary from your own course. We check each word, write example sentences for it, and schedule it for review.",
        )}
        actions={
          signedIn ? (
            <Button asChild>
              <Link to="/practice/review">
                <PlayCircle className="h-4 w-4" aria-hidden="true" />
                {t("uploads.reviewAll", "Review all due words")}
              </Link>
            </Button>
          ) : undefined
        }
      />

      {!signedIn ? (
        <GuestNotice />
      ) : (
        // The box you paste into beside what happened to what you pasted, from
        // xl up; stacked below that, the form first.
        <div className="grid items-start gap-6 xl:grid-cols-[minmax(0,5fr)_minmax(0,7fr)]">
          <Card className="p-5 md:p-6">
            <UploadForm />
          </Card>

          <div className="min-w-0 space-y-6">
            {deckList.length > 0 && (
              <section className="space-y-3" aria-labelledby="my-words-decks">
                <h2
                  id="my-words-decks"
                  className="flex items-center gap-2 text-lg font-semibold leading-snug text-text md:text-xl"
                >
                  <Layers
                    className="h-5 w-5 shrink-0 text-text-muted"
                    aria-hidden="true"
                  />
                  {t("uploads.decksTitle", "Decks")}
                </h2>
                <ul className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-2">
                  {deckList.map((deck) => (
                    <li key={deck.id}>
                      <Card className="flex h-full flex-col gap-4 p-4">
                        <div className="flex items-start justify-between gap-2">
                          <h3 className="min-w-0 break-words text-base font-semibold capitalize text-text">
                            {deck.name}
                          </h3>
                          <Badge variant="secondary" className="shrink-0">
                            {t("uploads.deckWordCount", "{{count}} words", {
                              count: deck.item_count,
                            })}
                          </Badge>
                        </div>
                        <Button
                          asChild
                          variant="outline"
                          className="mt-auto w-full"
                        >
                          <Link
                            to="/practice/review"
                            search={{ deck_id: deck.id }}
                          >
                            <PlayCircle
                              className="h-4 w-4"
                              aria-hidden="true"
                            />
                            {t("uploads.reviewDeck", "Review deck")}
                          </Link>
                        </Button>
                      </Card>
                    </li>
                  ))}
                </ul>
              </section>
            )}

            <section className="space-y-3" aria-labelledby="my-words-history">
              <h2
                id="my-words-history"
                className="text-lg font-semibold leading-snug text-text md:text-xl"
              >
                {t("uploads.history", "What you have added")}
              </h2>
              <UploadList
                uploads={uploads.data?.items ?? []}
                isLoading={uploads.isLoading}
              />
            </section>
          </div>
        </div>
      )}
    </div>
  );
}

export default MyWordsPage;
