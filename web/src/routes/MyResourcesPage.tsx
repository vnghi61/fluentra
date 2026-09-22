import React from "react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "@tanstack/react-router";
import { AlertCircle, FileText, RotateCcw } from "lucide-react";

import { Button } from "@/components/ui/button";
import { Card, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import {
  ResourceList,
  ResourceQuota,
  ResourceUpload,
  useResources,
} from "@/features/resource";

/**
 * A learner's own material: a private library of what they uploaded, what we
 * made of it, and what can be generated from it.
 *
 * Signed-in only (the route guards it): a resource belongs to a person, and a
 * guest has none.
 */
export function MyResourcesPage(): React.JSX.Element {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const { data, isLoading, isError, error, refetch } = useResources();

  const resources = data?.items ?? [];

  return (
    <div className="mx-auto max-w-3xl space-y-6 px-4 py-6 animate-in fade-in duration-200">
      <header className="space-y-1">
        <h1 className="text-2xl font-extrabold tracking-tight text-text md:text-3xl">
          {t("resources.title", "My resources")}
        </h1>
        <p className="text-sm text-text-muted">
          {t(
            "resources.tagline",
            "Upload a file from your own course. We read it, estimate its level and can build practice from it — and it stays private to you.",
          )}
        </p>
      </header>

      <ResourceUpload
        onUploaded={(id) =>
          void navigate({
            to: "/my-resources/$resourceId",
            params: { resourceId: id },
          })
        }
      />

      {!isLoading && !isError && resources.length > 0 && (
        <ResourceQuota resources={resources} />
      )}

      {isLoading && (
        <div className="space-y-2">
          {[1, 2, 3].map((i) => (
            <Skeleton key={i} className="h-20 w-full rounded-xl" />
          ))}
        </div>
      )}

      {isError && (
        <Card className="border-danger/30 p-6 text-center">
          <CardHeader className="space-y-2">
            <div className="flex justify-center">
              <AlertCircle className="h-8 w-8 text-danger-accent" aria-hidden="true" />
            </div>
            <CardTitle>
              {t("resources.errorTitle", "Unable to load your resources")}
            </CardTitle>
            <CardDescription>
              {error?.message ||
                t(
                  "resources.errorDesc",
                  "We could not fetch your files. Please try again.",
                )}
            </CardDescription>
          </CardHeader>
          <div className="flex justify-center">
            <Button
              variant="outline"
              onClick={() => void refetch()}
              className="gap-2"
            >
              <RotateCcw className="h-4 w-4" aria-hidden="true" />
              {t("action.retry", "Try again")}
            </Button>
          </div>
        </Card>
      )}

      {!isLoading && !isError && resources.length === 0 && (
        <Card className="border-border bg-surface-card p-8 text-center">
          <CardHeader className="space-y-2">
            <div className="flex justify-center">
              <div className="flex h-12 w-12 items-center justify-center rounded-full bg-primary/10 text-primary">
                <FileText className="h-6 w-6" aria-hidden="true" />
              </div>
            </div>
            <CardTitle className="text-lg font-semibold text-text">
              {t("resources.emptyTitle", "No files yet")}
            </CardTitle>
            <CardDescription className="mx-auto max-w-md text-sm">
              {t(
                "resources.emptyDesc",
                "Upload a PDF, a lesson recording or a video from your course and it will appear here.",
              )}
            </CardDescription>
          </CardHeader>
        </Card>
      )}

      {!isLoading && !isError && resources.length > 0 && (
        <ResourceList resources={resources} />
      )}
    </div>
  );
}

export default MyResourcesPage;
