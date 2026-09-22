import React from "react";
import { Link } from "@tanstack/react-router";
import { CheckCircle2, ArrowRightCircle, Circle, BookOpen, Award } from "lucide-react";
import { useTranslation } from "react-i18next";

import type { FoundationPathNode } from "../../api/foundation";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card";

export interface FoundationPathViewProps {
  items: FoundationPathNode[];
  targetCode: string;
  currentCode?: string;
  isLoading?: boolean;
}

export function FoundationPathView({
  items,
  targetCode,
  currentCode,
  isLoading,
}: FoundationPathViewProps): React.JSX.Element {
  const { t } = useTranslation();

  if (isLoading) {
    return (
      <Card className="border border-border-subtle bg-surface">
        <CardHeader>
          <div className="h-6 w-48 bg-muted animate-pulse rounded" />
          <div className="h-4 w-72 bg-muted animate-pulse rounded mt-2" />
        </CardHeader>
        <CardContent className="space-y-4">
          {[1, 2, 3].map((i) => (
            <div key={i} className="flex items-center space-x-4 p-4 border rounded-lg bg-surface-subtle animate-pulse">
              <div className="h-10 w-10 rounded-full bg-muted" />
              <div className="space-y-2 flex-1">
                <div className="h-4 w-1/3 bg-muted rounded" />
                <div className="h-3 w-1/4 bg-muted rounded" />
              </div>
            </div>
          ))}
        </CardContent>
      </Card>
    );
  }

  if (!items || items.length === 0) {
    return (
      <Card className="border border-border-subtle bg-surface">
        <CardContent className="p-6 text-center text-text-muted">
          <BookOpen className="h-10 w-10 mx-auto text-text-muted mb-2 opacity-60" />
          <p>{t("foundation.path.empty", "No prerequisite path recorded for this topic.")}</p>
        </CardContent>
      </Card>
    );
  }

  return (
    <Card className="border border-border-subtle bg-surface shadow-sm">
      <CardHeader>
        <div className="flex items-center justify-between">
          <div>
            <CardTitle className="text-xl font-bold flex items-center gap-2">
              <Award className="h-5 w-5 text-primary" />
              {t("foundation.path.title", "Prerequisite Learning Path")}
            </CardTitle>
            <CardDescription className="text-sm text-text-muted mt-1">
              {t(
                "foundation.path.description",
                "Follow this sequence of foundation topics to master this subject effectively.",
              )}
            </CardDescription>
          </div>
          <Badge variant="outline" className="font-mono text-xs">
            {items.length} {items.length === 1 ? "step" : "steps"}
          </Badge>
        </div>
      </CardHeader>

      <CardContent>
        <div className="relative pl-6 sm:pl-8 space-y-6 before:absolute before:bottom-3 before:left-3 sm:before:left-4 before:top-3 before:w-0.5 before:bg-border-subtle">
          {items.map((node, index) => {
            const isTarget = node.code === targetCode;
            const isCurrent = node.code === currentCode;
            const isNext = node.next;
            const isMastered = node.mastered;

            return (
              <div key={node.id} className="relative group">
                {/* Node icon marker */}
                <div className="absolute -left-6 sm:-left-8 top-1 transform -translate-x-1/2 flex items-center justify-center">
                  {isMastered ? (
                    <span className="flex h-7 w-7 rounded-full bg-success/15 text-success items-center justify-center ring-4 ring-surface">
                      <CheckCircle2 className="h-5 w-5" />
                    </span>
                  ) : isNext ? (
                    <span className="flex h-7 w-7 rounded-full bg-primary/20 text-primary items-center justify-center ring-4 ring-surface animate-bounce">
                      <ArrowRightCircle className="h-5 w-5" />
                    </span>
                  ) : (
                    <span className="flex h-7 w-7 rounded-full bg-surface-subtle text-text-muted items-center justify-center ring-4 ring-surface border border-border">
                      <Circle className="h-4 w-4" />
                    </span>
                  )}
                </div>

                {/* Node Card */}
                <div
                  className={`p-4 rounded-xl border transition-all duration-200 ${
                    isNext
                      ? "bg-primary/5 border-primary/40 shadow-sm ring-1 ring-primary/20"
                      : isCurrent
                        ? "bg-surface-subtle border-border ring-1 ring-border"
                        : "bg-surface hover:bg-surface-subtle border-border-subtle"
                  }`}
                >
                  <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-2">
                    <div className="space-y-1">
                      <div className="flex items-center gap-2 flex-wrap">
                        <span className="text-xs font-semibold text-text-muted">
                          Step {index + 1}
                        </span>
                        {node.cefr_level && (
                          <Badge variant="secondary" className="text-xs px-2 py-0">
                            {node.cefr_level}
                          </Badge>
                        )}
                        {isMastered && (
                          <Badge className="bg-success text-success-foreground text-xs px-2 py-0">
                            {t("foundation.mastered", "Mastered")}
                          </Badge>
                        )}
                        {isNext && (
                          <Badge className="bg-primary text-primary-foreground text-xs px-2 py-0 animate-pulse">
                            {t("foundation.nextToLearn", "Next to Learn")}
                          </Badge>
                        )}
                        {isTarget && (
                          <Badge variant="outline" className="text-xs border-primary/50 text-primary">
                            {t("foundation.target", "Target")}
                          </Badge>
                        )}
                      </div>

                      <h4 className="text-base font-bold text-text">
                        <Link
                          to={`/foundation/topics/${node.code}`}
                          className="hover:underline hover:text-primary transition-colors"
                        >
                          {node.label || node.code}
                        </Link>
                      </h4>
                      <p className="text-xs font-mono text-text-muted">{node.code}</p>
                    </div>

                    {/* Mastery Stats */}
                    <div className="flex sm:flex-col items-end justify-between sm:justify-center text-xs text-text-muted mt-2 sm:mt-0 pt-2 sm:pt-0 border-t sm:border-t-0 border-border-subtle">
                      <span>
                        {t("foundation.attempts", "Attempts")}:{" "}
                        <strong className="text-text">{node.attempts}</strong>
                      </span>
                      {node.attempts > 0 && (
                        <span>
                          {t("foundation.score", "Score")}:{" "}
                          <strong
                            className={
                              node.score >= 0.8
                                ? "text-success font-semibold"
                                : "text-text"
                            }
                          >
                            {Math.round(node.score * 100)}%
                          </strong>
                        </span>
                      )}
                    </div>
                  </div>
                </div>
              </div>
            );
          })}
        </div>
      </CardContent>
    </Card>
  );
}
