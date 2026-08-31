import React from "react";
import { useTranslation } from "react-i18next";

import { GuestNotice } from "@/features/learning";
import { UploadForm, UploadList, useUploads } from "@/features/vocabulary";
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

  return (
    <div className="space-y-6 animate-in fade-in duration-200">
      <header className="space-y-1">
        <h1 className="text-2xl md:text-3xl font-extrabold text-text tracking-tight">
          {t("uploads.title", "My words")}
        </h1>
        <p className="text-sm text-text-muted">
          {t(
            "uploads.tagline",
            "Add vocabulary from your own course. We check each word, write example sentences for it, and schedule it for review.",
          )}
        </p>
      </header>

      {!signedIn ? (
        <GuestNotice />
      ) : (
        <>
          <section className="rounded-2xl border border-border bg-surface-card p-5 shadow-sm">
            <UploadForm />
          </section>

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
