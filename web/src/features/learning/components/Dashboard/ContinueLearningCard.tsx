import React from "react";
import { Link } from "@tanstack/react-router";
import {
  ArrowRight,
  BookOpen,
  CheckCircle2,
  Clock,
  Sparkles,
} from "lucide-react";
import { useTranslation } from "react-i18next";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import type { DashboardResponse } from "../../api/learningApi";

export interface ContinueLearningCardProps {
  dashboard: DashboardResponse;
}

export const ContinueLearningCard: React.FC<ContinueLearningCardProps> = ({
  dashboard,
}) => {
  const { t } = useTranslation();
  const { state, next_activity } = dashboard;

  if (state === "completed") {
    return (
      <Card className="border-success/30 bg-gradient-to-br from-surface-card to-success/5 shadow-md">
        <CardHeader>
          <div className="flex items-center gap-2 text-success-accent mb-1">
            <CheckCircle2 className="h-5 w-5" aria-hidden="true" />
            <span className="text-xs font-semibold uppercase tracking-wider">
              {t("dashboard.continue.completedBadge", "Course Completed")}
            </span>
          </div>
          <CardTitle className="text-xl font-bold">
            {t("dashboard.continue.completedTitle", "All Caught Up!")}
          </CardTitle>
          <CardDescription>
            {t(
              "dashboard.continue.completedDesc",
              "You have completed all activities in your enrolled course. Keep practicing to maintain mastery.",
            )}
          </CardDescription>
        </CardHeader>
        <CardFooter>
          <Link to="/learn" className="w-full sm:w-auto">
            <Button variant="outline" className="w-full sm:w-auto gap-2">
              {t("dashboard.continue.reviewBtn", "Review Lessons")}
              <ArrowRight className="h-4 w-4" aria-hidden="true" />
            </Button>
          </Link>
        </CardFooter>
      </Card>
    );
  }

  if (state === "in_progress" && next_activity) {
    return (
      <Card className="border-primary/40 bg-gradient-to-br from-surface-card to-primary/10 shadow-md">
        <CardHeader>
          <div className="flex flex-wrap items-center justify-between gap-2 mb-1">
            <div className="flex items-center gap-2 text-primary-accent">
              <BookOpen className="h-5 w-5" aria-hidden="true" />
              <span className="text-xs font-semibold uppercase tracking-wider">
                {t("dashboard.continue.title", "Continue Learning")}
              </span>
            </div>
            <div className="flex items-center gap-2">
              <Badge variant="primary" className="capitalize">
                {next_activity.skill}
              </Badge>
              {next_activity.estimated_minutes && (
                <span className="inline-flex items-center gap-1 text-xs text-text-muted">
                  <Clock className="h-3.5 w-3.5" aria-hidden="true" />
                  {t("dashboard.continue.estimatedTime", {
                    minutes: next_activity.estimated_minutes,
                    defaultValue: `~${next_activity.estimated_minutes} mins`,
                  })}
                </span>
              )}
            </div>
          </div>
          <CardTitle className="text-xl md:text-2xl font-bold text-text">
            {next_activity.title}
          </CardTitle>
          <CardDescription className="line-clamp-2">
            {t(
              "dashboard.continue.resumePrompt",
              "Pick up right where you left off in your study plan.",
            )}
          </CardDescription>
        </CardHeader>
        <CardFooter>
          <Link to="/learn" className="w-full sm:w-auto">
            <Button className="w-full sm:w-auto gap-2 text-sm font-semibold">
              {t("dashboard.continue.continueBtn", "Continue Lesson")}
              <ArrowRight className="h-4 w-4" aria-hidden="true" />
            </Button>
          </Link>
        </CardFooter>
      </Card>
    );
  }

  // Default: not_started
  return (
    <Card className="border-primary/30 bg-gradient-to-br from-surface-card to-primary/5 shadow-md">
      <CardHeader>
        <div className="flex items-center gap-2 text-primary-accent mb-1">
          <Sparkles className="h-5 w-5" aria-hidden="true" />
          <span className="text-xs font-semibold uppercase tracking-wider">
            {t("dashboard.continue.notStartedBadge", "Get Started")}
          </span>
        </div>
        <CardTitle className="text-xl font-bold">
          {t(
            "dashboard.continue.notStartedTitle",
            "Start Your English Journey",
          )}
        </CardTitle>
        <CardDescription>
          {t(
            "dashboard.continue.notStartedDesc",
            "You haven't enrolled in any courses yet. Explore our structured curriculum to begin.",
          )}
        </CardDescription>
      </CardHeader>
      <CardFooter>
        <Link to="/learn" className="w-full sm:w-auto">
          <Button className="w-full sm:w-auto gap-2">
            {t("dashboard.continue.exploreBtn", "Explore Syllabus")}
            <ArrowRight className="h-4 w-4" aria-hidden="true" />
          </Button>
        </Link>
      </CardFooter>
    </Card>
  );
};
