import React from "react";
import { useTranslation } from "react-i18next";
import { Link, useParams } from "@tanstack/react-router";
import { AlertCircle, ArrowLeft, RotateCcw } from "lucide-react";

import { Button } from "@/components/ui/button";
import {
  Card,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { ResourceDetail, useResource } from "@/features/resource";

/**
 * One resource, on its own address so a reload or a bookmark lands here.
 *
 * While the pipeline is still running the query polls; the page does not need
 * to know that, it just renders whatever state the last answer carried.
 */
export function ResourceDetailPage(): React.JSX.Element {
  const { t } = useTranslation();
  const params: Record<string, string | undefined> = useParams({
    strict: false,
  });
  const resourceId = params.resourceId ?? "";
  const { data, isLoading, isError, error, refetch } = useResource(resourceId);

  return (
    <div className="mx-auto max-w-3xl space-y-4 px-4 py-6 animate-in fade-in duration-200">
      <Link
        to="/my-resources"
        className="inline-flex min-h-[44px] items-center gap-1.5 text-sm font-medium text-primary-accent"
      >
        <ArrowLeft className="h-4 w-4" aria-hidden="true" />
        {t("resources.backToList", "All resources")}
      </Link>

      {isLoading && (
        <div className="space-y-3">
          <Skeleton className="h-8 w-2/3" />
          <Skeleton className="h-64 w-full rounded-xl" />
        </div>
      )}

      {isError && (
        <Card className="border-danger/30 p-6 text-center">
          <CardHeader className="space-y-2">
            <div className="flex justify-center">
              <AlertCircle
                className="h-8 w-8 text-danger-accent"
                aria-hidden="true"
              />
            </div>
            <CardTitle>
              {t("resources.detailErrorTitle", "Unable to load this file")}
            </CardTitle>
            <CardDescription>
              {error?.message ||
                t(
                  "resources.detailErrorDesc",
                  "It may have been deleted, or the connection failed.",
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

      {data && (
        <>
          <header className="space-y-1">
            <h1 className="break-words text-2xl font-extrabold tracking-tight text-text md:text-3xl">
              {data.title || data.original_filename}
            </h1>
            <p className="break-words text-xs text-text-muted">
              {data.original_filename}
            </p>
          </header>
          <ResourceDetail resource={data} />
        </>
      )}
    </div>
  );
}

export default ResourceDetailPage;
