import React from "react";
import { Link } from "@tanstack/react-router";
import { ArrowRight, BookOpen } from "lucide-react";
import { useTranslation } from "react-i18next";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Progress } from "@/components/ui/progress";
import type { CourseProgress } from "../../api/learningApi";

export interface CourseProgressListProps {
  courses: CourseProgress[];
}

export const CourseProgressList: React.FC<CourseProgressListProps> = ({
  courses,
}) => {
  const { t } = useTranslation();

  return (
    <Card className="border-border-subtle bg-surface-card">
      <CardHeader>
        <CardTitle className="text-xl font-bold">
          {t("progress.coursesTitle", "Enrolled Courses")}
        </CardTitle>
        <CardDescription>
          {t(
            "progress.coursesDesc",
            "Track your completion percentage across active and finished courses.",
          )}
        </CardDescription>
      </CardHeader>

      <CardContent>
        {courses.length === 0 ? (
          <div className="text-center py-8 text-text-muted space-y-3">
            <BookOpen className="h-8 w-8 mx-auto opacity-50" aria-hidden="true" />
            <p className="text-sm">
              {t("progress.noCourses", "No course progress recorded yet.")}
            </p>
            <Link to="/learn">
              <Button variant="outline" size="sm" className="gap-1.5">
                {t("dashboard.continue.exploreBtn", "Explore Syllabus")}
                <ArrowRight className="h-3.5 w-3.5" aria-hidden="true" />
              </Button>
            </Link>
          </div>
        ) : (
          <div className="space-y-4">
            {courses.map((course, idx) => {
              const isDone = course.status === "completed";
              return (
                <div
                  key={course.course_id || idx}
                  className="p-4 rounded-xl border border-border bg-surface-muted/30 space-y-3"
                >
                  <div className="flex items-center justify-between">
                    <div className="flex items-center gap-2">
                      <span className="font-semibold text-text text-sm">
                        Course #{idx + 1}
                      </span>
                      {isDone ? (
                        <Badge variant="success" className="text-[10px] py-0 uppercase">
                          {t("learn.completedBadge", "Completed")}
                        </Badge>
                      ) : (
                        <Badge variant="primary" className="text-[10px] py-0 uppercase">
                          {t("dashboard.continue.title", "In Progress")}
                        </Badge>
                      )}
                    </div>
                    <span className="text-xs font-semibold text-text-muted">
                      {course.completed_activities} / {course.total_activities} {t("progress.activitiesDone", "activities")}
                    </span>
                  </div>

                  <Progress
                    value={course.percentage}
                    aria-label={`Course ${idx + 1} progress: ${course.percentage}%`}
                  />
                </div>
              );
            })}
          </div>
        )}
      </CardContent>
    </Card>
  );
};
