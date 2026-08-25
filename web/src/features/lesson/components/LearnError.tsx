import React from "react";
import { AlertCircle, RotateCcw } from "lucide-react";
import { useTranslation } from "react-i18next";

import { Button } from "@/components/ui/button";
import {
  Card,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";

export interface LearnErrorProps {
  onRetry: () => void;
  error?: Error | null;
}

export const LearnError: React.FC<LearnErrorProps> = ({ onRetry, error }) => {
  const { t } = useTranslation();

  return (
    <div className="py-8" role="alert">
      <Card className="border-danger/30 max-w-lg mx-auto bg-surface-card">
        <CardHeader>
          <div className="flex items-center gap-2 text-danger-accent mb-2">
            <AlertCircle className="h-6 w-6 shrink-0" aria-hidden="true" />
            <CardTitle className="text-lg font-bold">
              {t("learn.errorTitle", "Unable to Load Curriculum")}
            </CardTitle>
          </div>
          <CardDescription>
            {error?.message ||
              t(
                "learn.errorDesc",
                "We encountered an issue loading course materials. Please try again.",
              )}
          </CardDescription>
        </CardHeader>
        <CardFooter>
          <Button
            variant="outline"
            onClick={onRetry}
            className="gap-2 min-h-[44px]"
          >
            <RotateCcw className="h-4 w-4" aria-hidden="true" />
            {t("action.retry", "Try again")}
          </Button>
        </CardFooter>
      </Card>
    </div>
  );
};
