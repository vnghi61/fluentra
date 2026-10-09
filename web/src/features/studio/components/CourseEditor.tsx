import React, {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { useBlocker, useNavigate } from "@tanstack/react-router";
import { useForm, useWatch } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { produce } from "immer";
import type { TFunction } from "i18next";
import { useTranslation } from "react-i18next";
import {
  AlertCircle,
  CheckCircle2,
  ClipboardList,
  Save,
  Send,
  Sparkles,
} from "lucide-react";

import { PageHeader } from "@/components/layout/PageHeader";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { ErrorState } from "@/components/ui/error-state";
import { Skeleton } from "@/components/ui/skeleton";
import { formatVnd } from "@/lib/currency";
import { cn } from "@/lib/utils";
import {
  useCourseDraft,
  useCreateDraft,
  useSubmitDraft,
  useUpdateDraft,
} from "../hooks/useStudio";
import { activityIssues } from "../model/activityKinds";
import {
  courseFormSchema,
  defaultCourseForm,
  type CourseFormValues,
} from "../model/courseForm";
import {
  CEFR_LEVELS,
  LIMITS,
  PRICE,
  courseMetrics,
  newLesson,
  newUnit,
  priceInBounds,
  readUnits,
  serialiseUnits,
  type UnitItem,
} from "../model/courseStructure";
import { CourseDetailsCard } from "./CourseDetailsCard";
import { CurriculumBuilder } from "./CurriculumBuilder";
import { SubmissionChecklist, type ChecklistItem } from "./SubmissionChecklist";
import { SubmissionStatusModal } from "./SubmissionStatusModal";

export interface CourseEditorProps {
  draftId?: string | undefined;
}

/** Submission states, mapped onto the labels the Studio dashboard already uses. */
const SUBMISSION_LABEL: Record<string, string> = {
  submitted: "studio.status.submitted",
  verifying: "studio.status.verifying",
  in_review: "studio.status.inReview",
  changes_requested: "studio.status.changesRequested",
  approved: "studio.status.approved",
  published: "studio.status.published",
  rejected: "studio.status.rejected",
};

function starterOutline(t: TFunction): UnitItem[] {
  const unit = newUnit(t("studio.editor.defaultUnitTitle", { n: 1 }));
  unit.lessons.push(newLesson(t("studio.editor.defaultLessonTitle", { n: 1 })));
  return [unit];
}

type Feedback = { type: "success" | "error"; text: string } | null;

/**
 * The course editor: details and price on the left with the outline below,
 * the live Gate 1 checklist on the right. Saving needs a valid details form;
 * submitting also needs every checklist item, and saves pending edits first.
 */
export function CourseEditor({
  draftId,
}: CourseEditorProps): React.JSX.Element {
  const { t, i18n } = useTranslation();
  const navigate = useNavigate();
  const isEditing = Boolean(draftId);

  const draftQuery = useCourseDraft(draftId);
  const existingDraft = draftQuery.data;
  const createMutation = useCreateDraft();
  const updateMutation = useUpdateDraft(draftId ?? "");
  const submitMutation = useSubmitDraft(draftId);

  const form = useForm<CourseFormValues>({
    resolver: zodResolver(courseFormSchema),
    defaultValues: defaultCourseForm,
    mode: "onTouched",
  });
  const [slugTouched, setSlugTouched] = useState(false);
  const [units, setUnits] = useState<UnitItem[]>(() => starterOutline(t));
  const [outlineDirty, setOutlineDirty] = useState(false);
  const [feedback, setFeedback] = useState<Feedback>(null);
  const [savedAt, setSavedAt] = useState<Date | null>(null);
  const [statusOpen, setStatusOpen] = useState(false);
  // Set just before an intended navigation (created, submitted) so the
  // unsaved-changes guard does not stop it.
  const leavingRef = useRef(false);

  // Seed from the server once per draft, during render rather than in an effect.
  const [seededId, setSeededId] = useState<string | null>(null);
  if (existingDraft && existingDraft.id !== seededId) {
    setSeededId(existingDraft.id);
    const price = existingDraft.price_vnd || 0;
    const level = existingDraft.cefr_level as (typeof CEFR_LEVELS)[number];
    form.reset({
      title: existingDraft.title || "",
      slug: existingDraft.slug || "",
      description: existingDraft.description || "",
      cefrLevel: CEFR_LEVELS.includes(level) ? level : "B1",
      isFree: price === 0,
      priceVnd: price > 0 ? price : defaultCourseForm.priceVnd,
    });
    setSlugTouched(true);
    setUnits(readUnits(existingDraft.structure) ?? starterOutline(t));
    setOutlineDirty(false);
  }

  const update = useCallback((recipe: (draft: UnitItem[]) => void) => {
    setUnits((prev) => produce(prev, recipe));
    setOutlineDirty(true);
  }, []);

  const dirty = form.formState.isDirty || outlineDirty;
  const [title, slug, isFree, priceVnd] = useWatch({
    control: form.control,
    name: ["title", "slug", "isFree", "priceVnd"],
  });

  const metrics = useMemo(() => courseMetrics(units), [units]);
  const incompleteActivities = useMemo(
    () =>
      units
        .flatMap((u) => u.lessons.flatMap((l) => l.activities))
        .filter((a) =>
          activityIssues(a).some((issue) => issue !== "materialProcessing"),
        ).length,
    [units],
  );

  const checklist: ChecklistItem[] = [
    {
      key: "details",
      done: title.trim().length > 0 && slug.trim().length >= 3,
      label: t("studio.editor.check.details"),
    },
    {
      key: "lessons",
      done: metrics.lessons >= LIMITS.minLessons,
      label: t("studio.editor.ruleMinLessons", { count: metrics.lessons }),
    },
    {
      key: "exercises",
      done: metrics.exercises >= LIMITS.minExercises,
      label: t("studio.editor.ruleMinActivities", { count: metrics.exercises }),
      detail: t("studio.editor.check.exercisesDetail"),
    },
    {
      key: "lessonShape",
      done: metrics.lessons > 0 && metrics.invalidLessons === 0,
      label: t("studio.editor.check.lessonShape"),
      detail:
        metrics.invalidLessons > 0
          ? t("studio.editor.check.invalidLessons", {
              count: metrics.invalidLessons,
            })
          : undefined,
    },
    {
      key: "complete",
      done: incompleteActivities === 0,
      label: t("studio.editor.check.complete"),
      detail:
        incompleteActivities > 0
          ? t("studio.editor.check.incomplete", { count: incompleteActivities })
          : undefined,
    },
    {
      key: "materials",
      done: metrics.materialsNotReady === 0,
      label: t("studio.editor.check.materials"),
      detail:
        metrics.materialsNotReady > 0
          ? t("studio.editor.check.materialsPending", {
              count: metrics.materialsNotReady,
            })
          : undefined,
    },
    {
      key: "price",
      done: priceInBounds(isFree, priceVnd),
      label: isFree
        ? t("studio.editor.ruleFree")
        : t("studio.editor.rulePriceBounds", {
            min: formatVnd(PRICE.min),
            max: formatVnd(PRICE.max),
          }),
    },
  ];
  const ready = checklist.every((c) => c.done);
  const doneCount = checklist.filter((c) => c.done).length;
  const saving = createMutation.isPending || updateMutation.isPending;

  /** Saves and answers the draft id, or null when validation or the request failed. */
  const save = async (): Promise<string | null> => {
    setFeedback(null);
    if (!(await form.trigger())) {
      const first = Object.keys(form.formState.errors)[0] as
        keyof CourseFormValues | undefined;
      if (first) form.setFocus(first);
      setFeedback({ type: "error", text: t("studio.editor.fixErrors") });
      return null;
    }
    const v = form.getValues();
    const body = {
      title: v.title.trim(),
      slug: v.slug.trim(),
      description: v.description.trim(),
      cefr_level: v.cefrLevel,
      price_vnd: v.isFree ? 0 : v.priceVnd,
      structure: serialiseUnits(units),
    };
    try {
      if (isEditing && draftId) {
        await updateMutation.mutateAsync(body);
        form.reset(v);
        setOutlineDirty(false);
        setSavedAt(new Date());
        setFeedback({ type: "success", text: t("studio.editor.savedSuccess") });
        return draftId;
      }
      const created = await createMutation.mutateAsync(body);
      form.reset(v);
      setOutlineDirty(false);
      leavingRef.current = true;
      void navigate({
        to: "/studio/courses/$draftId",
        params: { draftId: created.id },
        replace: true,
      });
      return created.id;
    } catch (err) {
      setFeedback({
        type: "error",
        text:
          err instanceof Error && err.message
            ? err.message
            : t("studio.editor.saveFailed"),
      });
      return null;
    }
  };

  const submit = async () => {
    if (!draftId || !ready) return;
    const id = dirty ? await save() : draftId;
    if (!id) return;
    try {
      await submitMutation.mutateAsync(id);
      leavingRef.current = true;
      void navigate({ to: "/studio" });
    } catch (err) {
      setFeedback({
        type: "error",
        text:
          err instanceof Error && err.message
            ? err.message
            : t("studio.editor.submitFailed"),
      });
    }
  };

  // Ctrl/Cmd+S saves, as creators expect from any editor.
  const saveRef = useRef(save);
  useEffect(() => {
    saveRef.current = save;
  });
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === "s") {
        e.preventDefault();
        void saveRef.current();
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, []);

  const blocker = useBlocker({
    shouldBlockFn: () => dirty && !leavingRef.current,
    enableBeforeUnload: () => dirty && !leavingRef.current,
    withResolver: true,
  });

  if (isEditing && draftQuery.isLoading) {
    return (
      <div className="mx-auto w-full max-w-[1440px] space-y-6" aria-busy="true">
        <Skeleton className="h-5 w-24" />
        <Skeleton className="h-9 w-72 max-w-full" />
        <div className="grid gap-6 lg:grid-cols-[minmax(0,1fr)_320px]">
          <div className="space-y-6">
            <Skeleton className="h-80 w-full rounded-2xl" />
            <Skeleton className="h-64 w-full rounded-2xl" />
          </div>
          <Skeleton className="hidden h-96 w-full rounded-2xl lg:block" />
        </div>
      </div>
    );
  }

  if (isEditing && draftQuery.isError) {
    return (
      <ErrorState
        title={t("studio.editor.loadError")}
        onRetry={() => void draftQuery.refetch()}
      />
    );
  }

  const submission = existingDraft?.latest_submission;
  const submissionLabel = submission
    ? SUBMISSION_LABEL[submission.status]
    : undefined;

  return (
    <div className="mx-auto w-full max-w-[1440px] space-y-6">
      <PageHeader
        back={{ to: "/studio", label: t("studio.backToStudio") }}
        eyebrow={
          <>
            <Badge variant="primary" icon={<Sparkles className="h-4 w-4" />}>
              {t("studio.badge")}
            </Badge>
            <Badge variant="secondary">
              {submissionLabel ? t(submissionLabel) : t("studio.status.draft")}
            </Badge>
          </>
        }
        title={
          title.trim() ||
          (isEditing
            ? t("studio.editor.untitled")
            : t("studio.editor.newCourseTitle"))
        }
        description={t("studio.editor.pageSubtitle")}
        actions={
          <>
            <span
              className="w-full text-sm text-text-muted sm:w-auto"
              role="status"
              aria-live="polite"
            >
              {saving
                ? t("studio.editor.saving")
                : dirty
                  ? t("studio.editor.unsaved")
                  : savedAt
                    ? t("studio.editor.savedAt", {
                        time: savedAt.toLocaleTimeString(i18n.language, {
                          hour: "2-digit",
                          minute: "2-digit",
                        }),
                      })
                    : null}
            </span>
            {submission && draftId && (
              <Button variant="ghost" onClick={() => setStatusOpen(true)}>
                <ClipboardList className="h-4 w-4" aria-hidden="true" />
                {t("studio.courses.viewStatus")}
              </Button>
            )}
            <Button
              variant="outline"
              onClick={() => void save()}
              isLoading={saving && !submitMutation.isPending}
              disabled={saving || (isEditing && !dirty)}
              title="Ctrl+S"
            >
              {!saving && <Save className="h-4 w-4" aria-hidden="true" />}
              {t("studio.editor.saveDraft")}
            </Button>
            <Button
              onClick={() => void submit()}
              disabled={!isEditing || !ready || saving}
              isLoading={submitMutation.isPending}
              title={
                !isEditing
                  ? t("studio.editor.saveBeforeSubmit")
                  : !ready
                    ? t("studio.editor.notReady")
                    : undefined
              }
            >
              {!submitMutation.isPending && (
                <Send className="h-4 w-4" aria-hidden="true" />
              )}
              {t("studio.editor.submitForReview")}
            </Button>
          </>
        }
      />

      {feedback && (
        <div
          role={feedback.type === "error" ? "alert" : "status"}
          className={cn(
            "flex items-start gap-2.5 rounded-xl border p-3 text-sm font-medium",
            feedback.type === "success"
              ? "border-success/30 bg-success/10 text-success-accent"
              : "border-danger/30 bg-danger/10 text-danger-accent",
          )}
        >
          {feedback.type === "success" ? (
            <CheckCircle2 className="h-5 w-5 shrink-0" aria-hidden="true" />
          ) : (
            <AlertCircle className="h-5 w-5 shrink-0" aria-hidden="true" />
          )}
          <span className="min-w-0 break-words">{feedback.text}</span>
        </div>
      )}

      {/* Phones get the checklist score up front; the full list sits below. */}
      <Card className="flex items-center justify-between gap-3 p-3 text-sm lg:hidden">
        <span className="text-text-muted">
          {t("studio.editor.requirementsTitle")}
        </span>
        <span
          className={cn(
            "font-semibold tabular-nums",
            ready ? "text-success-accent" : "text-text",
          )}
        >
          {doneCount}/{checklist.length}
        </span>
      </Card>

      <div className="grid items-start gap-6 lg:grid-cols-[minmax(0,1fr)_320px] xl:grid-cols-[minmax(0,1fr)_360px]">
        <div className="min-w-0 space-y-6">
          <CourseDetailsCard
            form={form}
            slugTouched={slugTouched}
            onSlugTouched={() => setSlugTouched(true)}
          />
          <CurriculumBuilder units={units} update={update} />
        </div>
        <aside className="lg:sticky lg:top-6">
          <SubmissionChecklist items={checklist} metrics={metrics} />
        </aside>
      </div>

      {draftId && (
        <SubmissionStatusModal
          draftId={draftId}
          draftTitle={title}
          isOpen={statusOpen}
          onClose={() => setStatusOpen(false)}
        />
      )}

      <ConfirmDialog
        open={blocker.status === "blocked"}
        onOpenChange={(o) => {
          if (!o) blocker.reset?.();
        }}
        destructive
        title={t("studio.editor.leaveTitle")}
        description={t("studio.editor.leaveDesc")}
        confirmLabel={t("studio.editor.leaveConfirm")}
        cancelLabel={t("studio.editor.leaveCancel")}
        onConfirm={() => blocker.proceed?.()}
      />
    </div>
  );
}
