import React from "react";
import { useParams, useNavigate } from "@tanstack/react-router";
import { ArrowLeft, BookOpen, Layers, CheckSquare, Sparkles } from "lucide-react";
import { useTranslation } from "react-i18next";

import { useAuthStore } from "@/stores/authStore";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card";
import {
  useFoundationTopic,
  useLearnerFoundationPath,
  usePublicFoundationPath,
  type FoundationPathNode,
} from "@/features/learning/api/foundation";
import { FoundationPathView } from "@/features/learning/components/Foundation/FoundationPathView";
import {
  FoundationTopicBody,
  type FoundationTopicBodyShape,
} from "@/features/learning/components/Foundation/FoundationTopicBody";

export function FoundationTopicPage(): React.JSX.Element {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const params: Record<string, string | undefined> = useParams({
    strict: false,
  });
  const code = params["code"] ?? "";
  const signedIn = useAuthStore((state) => state.status === "authenticated");

  const {
    data: topic,
    isLoading: topicLoading,
    isError: topicError,
    error: topicErr,
  } = useFoundationTopic(code);

  const {
    data: learnerPath,
    isLoading: learnerPathLoading,
  } = useLearnerFoundationPath(code, signedIn);

  const {
    data: publicPath,
    isLoading: publicPathLoading,
  } = usePublicFoundationPath(code, !signedIn);

  const pathItems: FoundationPathNode[] = React.useMemo(() => {
    if (signedIn && learnerPath?.items) {
      return learnerPath.items;
    }
    if (!signedIn && publicPath?.items) {
      return publicPath.items.map((node, index) => ({
        id: node.id,
        namespace: node.namespace,
        code: node.code,
        label: node.label,
        cefr_level: node.cefr_level ?? null,
        attempts: 0,
        score: 0,
        mastered: false,
        next: index === 0, // for a guest, mark the first foundational node as next
      }));
    }
    return [];
  }, [signedIn, learnerPath, publicPath]);

  const pathLoading = signedIn ? learnerPathLoading : publicPathLoading;

  if (topicLoading) {
    return (
      <div className="max-w-4xl mx-auto space-y-6 animate-pulse p-4 sm:p-6">
        <div className="h-8 w-48 bg-muted rounded" />
        <div className="h-40 bg-muted rounded-xl" />
        <div className="h-64 bg-muted rounded-xl" />
      </div>
    );
  }

  if (topicError || !topic) {
    return (
      <div className="max-w-2xl mx-auto p-8 text-center space-y-4">
        <BookOpen className="h-12 w-12 text-destructive mx-auto opacity-75" />
        <h2 className="text-xl font-bold text-text">
          {t("foundation.topicNotFound", "Foundation Topic Not Found")}
        </h2>
        <p className="text-sm text-text-muted">
          {topicErr instanceof Error
            ? topicErr.message
            : t("foundation.topicNotFoundDesc", "Could not load the requested foundation topic.")}
        </p>
        <Button variant="outline" onClick={() => void navigate({ to: "/learn" })}>
          <ArrowLeft className="h-4 w-4 mr-2" />
          {t("foundation.backToLearn", "Back to Learn")}
        </Button>
      </div>
    );
  }

  // The structured body renders through the same component the lesson runner
  // uses, so a topic reads identically in both places (WO 22 Stage F). A plain
  // string body is the older shape and still renders as prose.
  const topicBody: FoundationTopicBodyShape | null =
    topic.body && typeof topic.body === "object" ? topic.body : null;
  const bodyText = typeof topic.body === "string" ? topic.body : "";

  return (
    <div className="max-w-4xl mx-auto space-y-8 p-4 sm:p-6 animate-in fade-in duration-200">
      {/* Navigation Breadcrumb */}
      <div>
        <Button
          variant="ghost"
          size="sm"
          onClick={() => void navigate({ to: "/learn" })}
          className="text-text-muted hover:text-text mb-2 -ml-2"
        >
          <ArrowLeft className="h-4 w-4 mr-2" />
          {t("foundation.backToCatalogue", "Learning Catalogue")}
        </Button>
      </div>

      {/* Topic Header Card */}
      <Card className="border border-border-subtle bg-surface shadow-sm overflow-hidden">
        <div className="bg-primary/5 border-b border-border-subtle px-6 py-4 flex flex-wrap items-center justify-between gap-3">
          <div className="flex items-center gap-2">
            <Layers className="h-5 w-5 text-primary" />
            <span className="text-xs uppercase font-bold tracking-wider text-text-muted">
              {topic.namespace} Strand
            </span>
          </div>
          <div className="flex items-center gap-2">
            {topic.cefr_level && (
              <Badge variant="secondary" className="font-semibold text-xs">
                {topic.cefr_level}
              </Badge>
            )}
            <Badge variant="outline" className="font-mono text-xs">
              {topic.code}
            </Badge>
          </div>
        </div>

        <CardHeader className="space-y-2">
          <CardTitle className="text-2xl sm:text-3xl font-extrabold text-text tracking-tight">
            {topic.label}
          </CardTitle>
          {topic.description && (
            <CardDescription className="text-base text-text-muted leading-relaxed">
              {topic.description}
            </CardDescription>
          )}
        </CardHeader>


        <CardContent className="space-y-6">
          {/* Practice Material Counts */}
          <div className="grid grid-cols-3 gap-3 p-4 rounded-xl bg-surface-subtle border border-border-subtle text-center">
            <div>
              <div className="text-2xl font-bold text-text">{topic.exercise_count}</div>
              <div className="text-xs text-text-muted flex items-center justify-center gap-1 mt-1">
                <CheckSquare className="h-3.5 w-3.5" />
                {t("foundation.exercises", "Exercises")}
              </div>
            </div>
            <div>
              <div className="text-2xl font-bold text-text">{topic.quiz_count}</div>
              <div className="text-xs text-text-muted flex items-center justify-center gap-1 mt-1">
                <Sparkles className="h-3.5 w-3.5" />
                {t("foundation.quizzes", "Quizzes")}
              </div>
            </div>
            <div>
              <div className="text-2xl font-bold text-text">{topic.review_count}</div>
              <div className="text-xs text-text-muted flex items-center justify-center gap-1 mt-1">
                <BookOpen className="h-3.5 w-3.5" />
                {t("foundation.reviews", "Reviews")}
              </div>
            </div>
          </div>

          {/* Topic Body Content (if published) */}
          {topicBody && (
            <div className="border-t border-border-subtle pt-4">
              <FoundationTopicBody body={topicBody} />
            </div>
          )}
          {bodyText && (
            <div className="prose dark:prose-invert max-w-none text-sm text-text border-t border-border-subtle pt-4 whitespace-pre-wrap">
              {bodyText}
            </div>
          )}
        </CardContent>
      </Card>

      {/* Learning Path View */}
      <section aria-label={t("foundation.pathSection", "Foundation Learning Path")}>
        <FoundationPathView
          items={pathItems}
          targetCode={code}
          currentCode={code}
          isLoading={pathLoading}
        />
      </section>
    </div>
  );
}
