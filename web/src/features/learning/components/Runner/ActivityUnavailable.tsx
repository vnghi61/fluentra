import React from "react";
import { FileQuestion } from "lucide-react";
import { useTranslation } from "react-i18next";

import { Button } from "@/components/ui/button";
import {
  Card,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";

export interface ActivityUnavailableProps {
  /** The activity kind the runner was handed, when there was one. */
  kind?: string;
  /** Moves past the activity so a single bad one does not trap the learner. */
  onSkip: () => void;
}

/**
 * Rendered when an activity cannot be presented: an unknown kind, or a config
 * missing the fields its exercise needs.
 *
 * The runner used to fall back to hard-coded content — "She was very ___ about
 * the test results", answer "meticulous" — for any activity whose config did not
 * carry a prompt. A learner then answered a question that was not theirs, and the
 * grade went to an attempt on the real activity. Skipping is the only honest
 * option; the runner supports three kinds and says so when it meets a fourth.
 */
export const ActivityUnavailable: React.FC<ActivityUnavailableProps> = ({
  kind,
  onSkip,
}) => {
  const { t } = useTranslation();

  return (
    <Card className="max-w-2xl mx-auto w-full text-center p-6 border-dashed">
      <CardHeader>
        <div className="flex justify-center mb-2">
          <FileQuestion
            className="h-10 w-10 text-text-muted"
            aria-hidden="true"
          />
        </div>
        <CardTitle>{t("runner.activityUnavailable")}</CardTitle>
        <CardDescription>
          {t("runner.activityUnavailableDesc")}
          {kind ? ` (${kind})` : null}
        </CardDescription>
      </CardHeader>
      <div className="flex justify-center pt-2">
        <Button variant="outline" onClick={onSkip}>
          {t("runner.skipActivityBtn")}
        </Button>
      </div>
    </Card>
  );
};
