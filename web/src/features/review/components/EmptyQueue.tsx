import React from "react";
import { Link } from "@tanstack/react-router";
import { CheckCircle2 } from "lucide-react";
import { useTranslation } from "react-i18next";

import { Button } from "@/components/ui/button";
import { Card, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";

/**
 * Nothing due is not a finished session.
 *
 * It is the common state during the alpha — most learners have no cards — and it
 * gets its own screen rather than a session summary reporting zero reviews, which
 * reads as a session that went wrong.
 */
export const EmptyQueue: React.FC = () => {
  const { t } = useTranslation();

  return (
    <div className="py-12 max-w-lg mx-auto">
      <Card className="text-center p-6">
        <CardHeader>
          <div className="flex justify-center mb-2">
            <CheckCircle2 className="h-10 w-10 text-success" aria-hidden="true" />
          </div>
          <CardTitle>{t("review.emptyTitle")}</CardTitle>
          <CardDescription>{t("review.emptyDesc")}</CardDescription>
        </CardHeader>
        <div className="flex justify-center gap-3 pt-2">
          <Link to="/learn">
            <Button>{t("review.emptyLearnBtn")}</Button>
          </Link>
          <Link to="/">
            <Button variant="outline">{t("review.backToDashboardBtn")}</Button>
          </Link>
        </div>
      </Card>
    </div>
  );
};
