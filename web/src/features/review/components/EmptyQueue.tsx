import React from "react";
import { Link } from "@tanstack/react-router";
import { BookMarked, CheckCircle2, GraduationCap } from "lucide-react";
import { useTranslation } from "react-i18next";

import { Button } from "@/components/ui/button";
import { EmptyState } from "@/components/ui/empty-state";

/**
 * Nothing due is not a finished session.
 *
 * It is the common state during the alpha — most learners have no cards — and it
 * gets its own screen rather than a session summary reporting zero reviews, which
 * reads as a session that went wrong. It says where new cards come from: a
 * finished lesson, or words the learner adds themselves.
 */
export const EmptyQueue: React.FC = () => {
  const { t } = useTranslation();

  return (
    <EmptyState
      variant="card"
      icon={CheckCircle2}
      title={t("review.emptyTitle")}
      description={t("review.emptyDesc")}
      className="mx-auto w-full max-w-2xl py-12 sm:py-16"
      action={
        <>
          <Button asChild>
            <Link to="/learn">
              <GraduationCap className="h-4 w-4" aria-hidden="true" />
              {t("review.emptyLearnBtn")}
            </Link>
          </Button>
          <Button asChild variant="outline">
            <Link to="/practice/my-words">
              <BookMarked className="h-4 w-4" aria-hidden="true" />
              {t("review.emptyAddWordsBtn")}
            </Link>
          </Button>
        </>
      }
    />
  );
};
