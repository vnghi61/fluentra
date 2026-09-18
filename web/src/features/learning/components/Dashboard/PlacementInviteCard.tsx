import React from "react";
import { Link } from "@tanstack/react-router";
import { ArrowRight, Compass } from "lucide-react";
import { useTranslation } from "react-i18next";

import {
  Card,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { cn } from "@/lib/utils";
import { usePlacementOverview } from "../../api/placement";

export interface PlacementInviteCardProps {
  className?: string | undefined;
}

/**
 * Invites a learner to take the placement test. Shown only while the server
 * says so: the flag placement.invite is on, and the learner has no result and
 * no session in progress.
 */
export const PlacementInviteCard: React.FC<PlacementInviteCardProps> = ({
  className,
}) => {
  const { t } = useTranslation();
  const { data } = usePlacementOverview();
  if (!data?.invite_available) return null;

  return (
    <Card className={cn("border-primary/30 bg-primary/5", className)}>
      <CardHeader className="space-y-2">
        <div className="flex items-center gap-2">
          <Compass className="h-5 w-5 text-primary" aria-hidden="true" />
          <CardTitle className="text-lg font-bold text-text">
            {t("placement.invite.title")}
          </CardTitle>
        </div>
        <CardDescription className="text-sm text-text-muted">
          {t("placement.invite.description")}
        </CardDescription>
      </CardHeader>
      <CardFooter>
        <Link
          to="/placement"
          className="inline-flex min-h-[44px] w-full items-center justify-center rounded-lg bg-primary px-6 text-base font-medium text-primary-fg hover:bg-primary-hover sm:w-auto"
        >
          {t("placement.invite.action")}
          <ArrowRight className="ml-2 h-4 w-4" aria-hidden="true" />
        </Link>
      </CardFooter>
    </Card>
  );
};
