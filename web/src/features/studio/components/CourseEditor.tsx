import React, { useState } from "react";
import { useNavigate } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import {
  AlertCircle,
  ArrowLeft,
  CheckCircle2,
  ChevronDown,
  ChevronRight,
  Layers,
  Plus,
  Save,
  Send,
  Sparkles,
  Trash2,
} from "lucide-react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Card } from "@/components/ui/card";
import {
  useCourseDraft,
  useCreateDraft,
  useUpdateDraft,
  useSubmitDraft,
} from "../hooks/useStudio";
import {
  ACTIVITY_KINDS,
  activityIssues,
  emptyFields,
  readActivity,
  serialiseActivity,
  type ActivityFields,
  type ActivityKind,
  type DraftActivity,
} from "../model/activityKinds";
import { ActivityFieldsEditor } from "./ActivityFieldsEditor";

const CEFR_LEVELS = ["A1", "A2", "B1", "B2", "C1", "C2"];

export interface LessonItem {
  id: string;
  title: string;
  description?: string | undefined;
  activities: DraftActivity[];
}

export interface UnitItem {
  id: string;
  title: string;
  description?: string | undefined;
  lessons: LessonItem[];
}

export interface CourseStructure {
  units: UnitItem[];
}

export interface CourseEditorProps {
  draftId?: string | undefined;
}

export function CourseEditor({
  draftId,
}: CourseEditorProps): React.JSX.Element {
  const { t } = useTranslation();
  const navigate = useNavigate();

  const isEditing = Boolean(draftId);
  const { data: existingDraft, isLoading: draftLoading } =
    useCourseDraft(draftId);
  const createMutation = useCreateDraft();
  const updateMutation = useUpdateDraft(draftId ?? "");
  const submitMutation = useSubmitDraft(draftId ?? "");

  // Metadata form
  const [title, setTitle] = useState("");
  const [slug, setSlug] = useState("");
  const [description, setDescription] = useState("");
  const [cefrLevel, setCefrLevel] = useState("B1");
  const [isFree, setIsFree] = useState(true);
  const [priceVnd, setPriceVnd] = useState<number>(49000);

  // Curriculum outline
  const [units, setUnits] = useState<UnitItem[]>([
    {
      id: "u-1",
      title: "Unit 1: Introduction",
      lessons: [
        {
          id: "l-1",
          title: "Lesson 1: Foundations",
          activities: [
            {
              id: "a-1",
              kind: "vocab_multiple_choice",
              title: "Activity 1",
              fields: emptyFields(),
            },
          ],
        },
      ],
    },
  ]);

  const [expandedUnits, setExpandedUnits] = useState<Record<string, boolean>>({
    "u-1": true,
  });
  const [expandedLessons, setExpandedLessons] = useState<
    Record<string, boolean>
  >({
    "l-1": true,
  });

  const [statusMessage, setStatusMessage] = useState<{
    type: "success" | "error";
    text: string;
  } | null>(null);

  // Seed existing draft data during render rather than from an effect
  const [seededDraftId, setSeededDraftId] = useState<string | null>(null);
  if (existingDraft && existingDraft.id !== seededDraftId) {
    setSeededDraftId(existingDraft.id);
    setTitle(existingDraft.title || "");
    setSlug(existingDraft.slug || "");
    setDescription(existingDraft.description || "");
    setCefrLevel(existingDraft.cefr_level || "B1");
    const price = existingDraft.price_vnd || 0;
    setIsFree(price === 0);
    if (price > 0) setPriceVnd(price);

    const structure = existingDraft.structure as unknown as
      CourseStructure | undefined;
    if (structure?.units && structure.units.length > 0) {
      setUnits(
        structure.units.map((unit) => ({
          ...unit,
          lessons: (unit.lessons ?? []).map((lesson) => ({
            ...lesson,
            activities: (lesson.activities ?? []).map((raw, i) =>
              readActivity(raw, `${lesson.id}-a${i}`),
            ),
          })),
        })),
      );
    }
  }

  // Generate slug from title
  const handleTitleChange = (newTitle: string) => {
    setTitle(newTitle);
    if (!isEditing || !slug) {
      const generated = newTitle
        .toLowerCase()
        .replace(/[^a-z0-9]+/g, "-")
        .replace(/(^-|-$)/g, "");
      setSlug(generated);
    }
  };

  // Unit manipulation
  const toggleUnit = (unitId: string) => {
    setExpandedUnits((prev) => ({ ...prev, [unitId]: !prev[unitId] }));
  };

  const addUnit = () => {
    const newId = `u-${crypto.randomUUID()}`;
    setUnits((prev) => [
      ...prev,
      {
        id: newId,
        title: `Unit ${prev.length + 1}`,
        lessons: [],
      },
    ]);
    setExpandedUnits((prev) => ({ ...prev, [newId]: true }));
  };

  const removeUnit = (unitIndex: number) => {
    setUnits((prev) => prev.filter((_, idx) => idx !== unitIndex));
  };

  // Lesson manipulation
  const toggleLesson = (lessonId: string) => {
    setExpandedLessons((prev) => ({ ...prev, [lessonId]: !prev[lessonId] }));
  };

  const addLesson = (unitIndex: number) => {
    const newLessonId = `l-${crypto.randomUUID()}`;
    setUnits((prev) => {
      const copy = [...prev];
      const targetUnit = copy[unitIndex];
      if (targetUnit) {
        targetUnit.lessons = [
          ...targetUnit.lessons,
          {
            id: newLessonId,
            title: `Lesson ${targetUnit.lessons.length + 1}`,
            activities: [],
          },
        ];
      }
      return copy;
    });
    setExpandedLessons((prev) => ({ ...prev, [newLessonId]: true }));
  };

  const removeLesson = (unitIndex: number, lessonIndex: number) => {
    setUnits((prev) => {
      const copy = [...prev];
      const targetUnit = copy[unitIndex];
      if (targetUnit) {
        targetUnit.lessons = targetUnit.lessons.filter(
          (_, idx) => idx !== lessonIndex,
        );
      }
      return copy;
    });
  };

  // Activity manipulation
  const addActivity = (unitIndex: number, lessonIndex: number) => {
    const newActivityId = `a-${crypto.randomUUID()}`;
    setUnits((prev) => {
      const copy = [...prev];
      const lesson = copy[unitIndex]?.lessons[lessonIndex];
      if (lesson) {
        lesson.activities = [
          ...lesson.activities,
          {
            id: newActivityId,
            kind: "vocab_multiple_choice",
            title: `Activity ${lesson.activities.length + 1}`,
            fields: emptyFields(),
          },
        ];
      }
      return copy;
    });
  };

  const updateActivity = (
    unitIndex: number,
    lessonIndex: number,
    activityIndex: number,
    patch: {
      kind?: ActivityKind;
      title?: string;
      fields?: Partial<ActivityFields>;
    },
  ) => {
    setUnits((prev) => {
      const copy = [...prev];
      const lesson = copy[unitIndex]?.lessons[lessonIndex];
      const act = lesson?.activities[activityIndex];
      if (lesson && act) {
        const next = [...lesson.activities];
        next[activityIndex] = {
          ...act,
          ...(patch.kind && { kind: patch.kind }),
          ...(patch.title !== undefined && { title: patch.title }),
          fields: { ...act.fields, ...patch.fields },
        };
        lesson.activities = next;
      }
      return copy;
    });
  };

  const removeActivity = (
    unitIndex: number,
    lessonIndex: number,
    activityIndex: number,
  ) => {
    setUnits((prev) => {
      const copy = [...prev];
      const lesson = copy[unitIndex]?.lessons[lessonIndex];
      if (lesson) {
        lesson.activities = lesson.activities.filter(
          (_, idx) => idx !== activityIndex,
        );
      }
      return copy;
    });
  };

  // Metrics & Rules
  const totalLessons = units.reduce(
    (acc, u) => acc + (u.lessons?.length ?? 0),
    0,
  );
  const totalActivities = units.reduce(
    (acc, u) =>
      acc +
      u.lessons.reduce((lAcc, l) => lAcc + (l.activities?.length ?? 0), 0),
    0,
  );
  // D20-3: the twenty are graded exercises. A material is something to read or
  // watch, and a course of twenty videos is not a practice course.
  const isMaterial = (a: DraftActivity) => a.kind === "lesson_material";
  const totalExercises = units.reduce(
    (acc, u) =>
      acc +
      u.lessons.reduce(
        (lAcc, l) => lAcc + l.activities.filter((a) => !isMaterial(a)).length,
        0,
      ),
    0,
  );
  const totalMaterials = totalActivities - totalExercises;
  // A lesson is valid with at least one material, or with 3-30 exercises.
  const everyLessonValid = units.every((u) =>
    u.lessons.every((l) => {
      const materials = l.activities.filter(isMaterial).length;
      const exercises = l.activities.length - materials;
      return materials > 0 || (exercises >= 3 && exercises <= 30);
    }),
  );
  // A material may be saved while its video processes, but it may not be
  // submitted: Gate 1's material_ready would reject it, and the creator should
  // not spend a submission round-trip to learn that (WO 20 Stage B.1).
  const everyMaterialReady = units.every((u) =>
    u.lessons.every((l) =>
      l.activities.every(
        (a) =>
          !isMaterial(a) ||
          (a.fields.resourceId.trim() !== "" &&
            a.fields.materialStatus === "ready"),
      ),
    ),
  );

  const meetsMinLessons = totalLessons >= 3;
  const meetsMinActivities = totalExercises >= 20;
  const meetsPriceBounds = isFree || (priceVnd >= 49000 && priceVnd <= 5000000);
  const canSubmit =
    title.trim().length > 0 &&
    slug.trim().length > 0 &&
    meetsMinLessons &&
    meetsMinActivities &&
    everyLessonValid &&
    everyMaterialReady &&
    meetsPriceBounds;

  const handleSaveDraft = async () => {
    setStatusMessage(null);
    const finalPrice = isFree ? 0 : priceVnd;
    const structureData = {
      units: units.map((unit) => ({
        ...unit,
        lessons: unit.lessons.map((lesson) => ({
          ...lesson,
          activities: lesson.activities.map(serialiseActivity),
        })),
      })),
    } as unknown as Record<string, unknown>;

    try {
      if (isEditing && draftId) {
        await updateMutation.mutateAsync({
          title: title.trim(),
          slug: slug.trim(),
          description: description.trim(),
          cefr_level: cefrLevel,
          price_vnd: finalPrice,
          structure: structureData,
        });
        setStatusMessage({
          type: "success",
          text: t("studio.editor.savedSuccess", "Draft saved successfully!"),
        });
      } else {
        const created = await createMutation.mutateAsync({
          title: title.trim(),
          slug: slug.trim(),
          description: description.trim(),
          cefr_level: cefrLevel,
          price_vnd: finalPrice,
          structure: structureData,
        });
        setStatusMessage({
          type: "success",
          text: t("studio.editor.createdSuccess", "Course draft created!"),
        });
        void navigate({ to: `/studio/courses/${created.id}` });
      }
    } catch (err) {
      setStatusMessage({
        type: "error",
        text:
          err instanceof Error
            ? err.message
            : t("studio.editor.saveFailed", "Failed to save draft"),
      });
    }
  };

  const handleSubmit = async () => {
    if (!draftId) return;
    setStatusMessage(null);
    try {
      // First save current changes
      await handleSaveDraft();
      // Then submit for verification
      await submitMutation.mutateAsync();
      setStatusMessage({
        type: "success",
        text: t(
          "studio.editor.submittedSuccess",
          "Course submitted for Gate 1 automated review!",
        ),
      });
      setTimeout(() => {
        void navigate({ to: "/studio" });
      }, 1500);
    } catch (err) {
      setStatusMessage({
        type: "error",
        text:
          err instanceof Error
            ? err.message
            : t("studio.editor.submitFailed", "Failed to submit course"),
      });
    }
  };

  if (isEditing && draftLoading) {
    return (
      <div className="py-24 text-center text-sm text-text-muted">
        {t("app.loading", "Loading course editor...")}
      </div>
    );
  }

  return (
    <div className="max-w-5xl mx-auto space-y-8 py-6">
      {/* Top Bar */}
      <div className="flex items-center justify-between border-b border-border-subtle pb-4">
        <button
          type="button"
          onClick={() => void navigate({ to: "/studio" })}
          className="flex items-center gap-2 text-xs font-semibold text-text-muted hover:text-text transition-colors cursor-pointer"
        >
          <ArrowLeft className="h-4 w-4" />
          {t("studio.backToStudio", "Back to Studio")}
        </button>

        <div className="flex items-center gap-3">
          <Button
            variant="outline"
            size="sm"
            onClick={() => {
              void handleSaveDraft();
            }}
            disabled={createMutation.isPending || updateMutation.isPending}
            className="gap-1.5 text-xs font-semibold"
          >
            <Save className="h-3.5 w-3.5" />
            {t("app.save", "Save Draft")}
          </Button>

          {isEditing && (
            <Button
              variant="primary"
              size="sm"
              onClick={() => {
                void handleSubmit();
              }}
              disabled={!canSubmit || submitMutation.isPending}
              className="gap-1.5 text-xs font-semibold"
            >
              <Send className="h-3.5 w-3.5" />
              {t("studio.editor.submitForReview", "Submit for Review")}
            </Button>
          )}
        </div>
      </div>

      {statusMessage && (
        <div
          className={`flex items-center gap-2.5 rounded-lg p-3 text-sm font-medium border ${
            statusMessage.type === "success"
              ? "border-success/30 bg-success/10 text-success"
              : "border-danger/30 bg-danger/10 text-danger"
          }`}
        >
          {statusMessage.type === "success" ? (
            <CheckCircle2 className="h-4 w-4 shrink-0" />
          ) : (
            <AlertCircle className="h-4 w-4 shrink-0" />
          )}
          <span>{statusMessage.text}</span>
        </div>
      )}

      {/* Rules Validation Checklist Card */}
      <Card className="border-border-subtle bg-surface-card/60 p-4 space-y-2 text-xs">
        <div className="flex items-center justify-between font-bold text-text">
          <span>
            {t(
              "studio.editor.requirementsTitle",
              "Submission Quality Gates Checklist",
            )}
          </span>
          <span className="text-text-muted">
            {totalLessons} lessons • {totalExercises} exercises
            {totalMaterials > 0 ? ` • ${totalMaterials} materials` : ""}
          </span>
        </div>
        <div className="grid grid-cols-1 sm:grid-cols-3 gap-2 pt-1">
          <div className="flex items-center gap-2">
            {meetsMinLessons ? (
              <CheckCircle2 className="h-4 w-4 text-success shrink-0" />
            ) : (
              <AlertCircle className="h-4 w-4 text-warning shrink-0" />
            )}
            <span className={meetsMinLessons ? "text-text" : "text-text-muted"}>
              {t(
                "studio.editor.ruleMinLessons",
                "Min 3 lessons (Current: {{count}})",
                {
                  count: totalLessons,
                },
              )}
            </span>
          </div>

          <div className="flex items-center gap-2">
            {meetsMinActivities ? (
              <CheckCircle2 className="h-4 w-4 text-success shrink-0" />
            ) : (
              <AlertCircle className="h-4 w-4 text-warning shrink-0" />
            )}
            <span
              className={meetsMinActivities ? "text-text" : "text-text-muted"}
            >
              {t(
                "studio.editor.ruleMinActivities",
                "Min 20 exercises (Current: {{count}})",
                {
                  count: totalExercises,
                },
              )}
            </span>
          </div>

          <div className="flex items-center gap-2">
            {meetsPriceBounds ? (
              <CheckCircle2 className="h-4 w-4 text-success shrink-0" />
            ) : (
              <AlertCircle className="h-4 w-4 text-warning shrink-0" />
            )}
            <span
              className={meetsPriceBounds ? "text-text" : "text-text-muted"}
            >
              {isFree
                ? t("studio.editor.ruleFree", "Free Community Course")
                : t("studio.editor.rulePriceBounds", "₫49,000 - ₫5,000,000")}
            </span>
          </div>
        </div>
      </Card>

      {/* Course Metadata Section */}
      <Card className="border-border-subtle bg-surface-card p-6 space-y-5">
        <div className="flex items-center gap-2 border-b border-border-subtle pb-3">
          <Sparkles className="h-5 w-5 text-primary-accent" />
          <h2 className="text-base font-bold text-text">
            {t("studio.editor.metadataTitle", "Course Details & Pricing")}
          </h2>
        </div>

        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          <div>
            <label
              htmlFor="course-title-input"
              className="block text-xs font-semibold text-text uppercase tracking-wider mb-1.5"
            >
              {t("studio.editor.titleLabel", "Course Title")}
            </label>
            <Input
              id="course-title-input"
              value={title}
              onChange={(e) => handleTitleChange(e.target.value)}
              placeholder="e.g. Master Daily English Conversation"
              required
            />
          </div>

          <div>
            <label
              htmlFor="course-slug-input"
              className="block text-xs font-semibold text-text uppercase tracking-wider mb-1.5"
            >
              {t("studio.editor.slugLabel", "URL Slug")}
            </label>
            <Input
              id="course-slug-input"
              value={slug}
              onChange={(e) => setSlug(e.target.value)}
              placeholder="e.g. master-daily-english"
              required
            />
          </div>
        </div>

        <div>
          <label
            htmlFor="course-desc-input"
            className="block text-xs font-semibold text-text uppercase tracking-wider mb-1.5"
          >
            {t("studio.editor.descLabel", "Description")}
          </label>
          <textarea
            id="course-desc-input"
            rows={3}
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            placeholder="A concise syllabus overview describing what learners will achieve..."
            className="w-full rounded-lg border border-border-subtle bg-surface-base px-3 py-2 text-base sm:text-sm text-text focus:border-primary focus:outline-none focus:ring-1 focus:ring-primary"
          />
        </div>

        <div className="grid grid-cols-1 md:grid-cols-2 gap-4 pt-2">
          {/* CEFR Level */}
          <div>
            <label
              htmlFor="cefr-select"
              className="block text-xs font-semibold text-text uppercase tracking-wider mb-1.5"
            >
              {t("studio.editor.cefrLabel", "CEFR Proficiency Level")}
            </label>
            <select
              id="cefr-select"
              value={cefrLevel}
              onChange={(e) => setCefrLevel(e.target.value)}
              className="w-full rounded-lg border border-border-subtle bg-surface-base px-3 py-2 text-base sm:text-sm text-text focus:border-primary focus:outline-none focus:ring-1 focus:ring-primary"
            >
              {CEFR_LEVELS.map((lvl) => (
                <option key={lvl} value={lvl}>
                  {lvl} -{" "}
                  {lvl === "A1"
                    ? "Beginner"
                    : lvl === "B1"
                      ? "Intermediate"
                      : lvl === "C1"
                        ? "Advanced"
                        : lvl}
                </option>
              ))}
            </select>
          </div>

          {/* Pricing Model */}
          <div>
            <label className="block text-xs font-semibold text-text uppercase tracking-wider mb-1.5">
              {t("studio.editor.pricingLabel", "Pricing Model")}
            </label>
            <div className="flex items-center gap-4 pt-1">
              <label className="flex items-center gap-2 text-sm text-text cursor-pointer">
                <input
                  type="radio"
                  name="pricing"
                  checked={isFree}
                  onChange={() => setIsFree(true)}
                  className="text-primary focus:ring-primary"
                />
                {t("studio.editor.priceFree", "Free (Community)")}
              </label>

              <label className="flex items-center gap-2 text-sm text-text cursor-pointer">
                <input
                  type="radio"
                  name="pricing"
                  checked={!isFree}
                  onChange={() => setIsFree(false)}
                  className="text-primary focus:ring-primary"
                />
                {t("studio.editor.pricePaid", "One-Time Paid")}
              </label>
            </div>

            {!isFree && (
              <div className="mt-3 relative">
                <span className="absolute left-3 top-1/2 -translate-y-1/2 text-sm text-text-muted">
                  ₫
                </span>
                <Input
                  type="number"
                  min={49000}
                  max={5000000}
                  step={1000}
                  value={priceVnd}
                  onChange={(e) => setPriceVnd(Number(e.target.value))}
                  placeholder="49,000"
                  className="pl-8 text-sm font-semibold"
                />
                <span className="text-[11px] text-text-muted mt-1 block">
                  {t(
                    "studio.editor.priceHelp",
                    "70% creator payout share (₫{{share}}) upon purchase.",
                    {
                      share: Math.floor(priceVnd * 0.7).toLocaleString("vi-VN"),
                    },
                  )}
                </span>
              </div>
            )}
          </div>
        </div>
      </Card>

      {/* Curriculum Outline Builder */}
      <section className="space-y-4">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <Layers className="h-5 w-5 text-primary-accent" />
            <h2 className="text-lg font-bold text-text">
              {t("studio.editor.outlineTitle", "Curriculum Outline")}
            </h2>
          </div>
          <Button
            variant="outline"
            size="sm"
            onClick={addUnit}
            disabled={units.length >= 12}
            className="gap-1.5 text-xs"
          >
            <Plus className="h-3.5 w-3.5" />
            {t("studio.editor.addUnit", "Add Unit")}
          </Button>
        </div>

        {/* Units list */}
        <div className="space-y-4">
          {units.map((unit, uIdx) => {
            const isUnitExpanded = expandedUnits[unit.id] ?? false;

            return (
              <div
                key={unit.id}
                className="rounded-xl border border-border-subtle bg-surface-card overflow-hidden shadow-sm"
              >
                {/* Unit Header */}
                <div className="flex items-center justify-between bg-surface-base/80 p-3.5 border-b border-border-subtle">
                  <button
                    type="button"
                    onClick={() => toggleUnit(unit.id)}
                    className="flex items-center gap-2 text-sm font-bold text-text hover:text-primary-accent transition-colors text-left"
                  >
                    {isUnitExpanded ? (
                      <ChevronDown className="h-4 w-4 shrink-0 text-text-muted" />
                    ) : (
                      <ChevronRight className="h-4 w-4 shrink-0 text-text-muted" />
                    )}
                    <span>{unit.title || `Unit ${uIdx + 1}`}</span>
                    <span className="text-xs font-normal text-text-muted">
                      ({unit.lessons.length} lessons)
                    </span>
                  </button>

                  <div className="flex items-center gap-2">
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => addLesson(uIdx)}
                      disabled={unit.lessons.length >= 20}
                      className="gap-1 text-xs px-2"
                    >
                      <Plus className="h-3.5 w-3.5" />
                      {t("studio.editor.addLesson", "Lesson")}
                    </Button>
                    <button
                      type="button"
                      onClick={() => removeUnit(uIdx)}
                      className="p-1 text-text-muted hover:text-danger rounded transition-colors"
                      title={t("app.delete", "Delete Unit")}
                    >
                      <Trash2 className="h-4 w-4" />
                    </button>
                  </div>
                </div>

                {isUnitExpanded && (
                  <div className="p-4 space-y-4">
                    <div className="space-y-2">
                      <Input
                        value={unit.title}
                        onChange={(e) => {
                          const val = e.target.value;
                          setUnits((prev) => {
                            const c = [...prev];
                            if (c[uIdx]) c[uIdx].title = val;
                            return c;
                          });
                        }}
                        placeholder="Unit title..."
                        className="text-sm font-semibold"
                      />
                    </div>

                    {/* Lessons inside Unit */}
                    <div className="space-y-3 pl-3 border-l-2 border-primary/30">
                      {unit.lessons.map((lesson, lIdx) => {
                        const isLessonExpanded =
                          expandedLessons[lesson.id] ?? false;

                        return (
                          <div
                            key={lesson.id}
                            className="rounded-lg border border-border-subtle bg-surface-base p-3 space-y-3"
                          >
                            <div className="flex items-center justify-between">
                              <button
                                type="button"
                                onClick={() => toggleLesson(lesson.id)}
                                className="flex items-center gap-2 text-xs font-bold text-text hover:text-primary-accent"
                              >
                                {isLessonExpanded ? (
                                  <ChevronDown className="h-3.5 w-3.5 text-text-muted" />
                                ) : (
                                  <ChevronRight className="h-3.5 w-3.5 text-text-muted" />
                                )}
                                <span>
                                  {lesson.title || `Lesson ${lIdx + 1}`}
                                </span>
                                <span className="text-[11px] font-normal text-text-muted">
                                  ({lesson.activities.length} activities)
                                </span>
                              </button>

                              <div className="flex items-center gap-1.5">
                                <Button
                                  variant="ghost"
                                  size="sm"
                                  onClick={() => addActivity(uIdx, lIdx)}
                                  disabled={lesson.activities.length >= 30}
                                  className="gap-1 text-[11px] px-1.5"
                                >
                                  <Plus className="h-3 w-3" />
                                  {t("studio.editor.addActivity", "Activity")}
                                </Button>
                                <button
                                  type="button"
                                  onClick={() => removeLesson(uIdx, lIdx)}
                                  className="p-1 text-text-muted hover:text-danger rounded"
                                >
                                  <Trash2 className="h-3.5 w-3.5" />
                                </button>
                              </div>
                            </div>

                            {isLessonExpanded && (
                              <div className="space-y-3 pt-1">
                                <Input
                                  value={lesson.title}
                                  onChange={(e) => {
                                    const val = e.target.value;
                                    setUnits((prev) => {
                                      const c = [...prev];
                                      const les = c[uIdx]?.lessons[lIdx];
                                      if (les) les.title = val;
                                      return c;
                                    });
                                  }}
                                  placeholder="Lesson title..."
                                  className="text-xs font-medium"
                                />

                                {/* Activities inside Lesson */}
                                <div className="space-y-2 pt-1">
                                  {lesson.activities.map((activity, aIdx) => {
                                    const issues = activityIssues(activity);
                                    return (
                                      <div
                                        key={activity.id}
                                        className="rounded-md border border-border-subtle bg-surface-card p-2.5 space-y-2 text-xs"
                                      >
                                        <div className="flex flex-wrap items-center justify-between gap-2">
                                          <div className="flex flex-1 min-w-0 items-center gap-2">
                                            <Badge
                                              variant="secondary"
                                              className="text-[10px] shrink-0"
                                            >
                                              {t(
                                                `studio.activity.kind.${activity.kind}`,
                                              )}
                                            </Badge>
                                            <input
                                              type="text"
                                              value={activity.title}
                                              onChange={(e) =>
                                                updateActivity(
                                                  uIdx,
                                                  lIdx,
                                                  aIdx,
                                                  {
                                                    title: e.target.value,
                                                  },
                                                )
                                              }
                                              placeholder={t(
                                                "studio.activity.titlePlaceholder",
                                              )}
                                              aria-label={t(
                                                "studio.activity.titlePlaceholder",
                                              )}
                                              className="font-semibold text-text bg-transparent border-0 focus:outline-none focus:ring-0 text-base sm:text-xs min-w-11 flex-1 sm:flex-none sm:w-48"
                                            />
                                          </div>

                                          <div className="flex w-full shrink-0 items-center gap-2 sm:w-auto">
                                            <select
                                              value={activity.kind}
                                              aria-label={t(
                                                "studio.activity.kindLabel",
                                              )}
                                              onChange={(e) =>
                                                updateActivity(
                                                  uIdx,
                                                  lIdx,
                                                  aIdx,
                                                  {
                                                    kind: e.target
                                                      .value as ActivityKind,
                                                  },
                                                )
                                              }
                                              className="rounded border border-border-subtle bg-surface-base px-2 py-0.5 text-base sm:text-[11px] text-text min-h-11 sm:min-h-0 flex-1 min-w-11 sm:flex-none"
                                            >
                                              {ACTIVITY_KINDS.map((kind) => (
                                                <option key={kind} value={kind}>
                                                  {t(
                                                    `studio.activity.kind.${kind}`,
                                                  )}
                                                </option>
                                              ))}
                                            </select>

                                            <button
                                              type="button"
                                              onClick={() =>
                                                removeActivity(uIdx, lIdx, aIdx)
                                              }
                                              aria-label={t(
                                                "studio.activity.remove",
                                              )}
                                              className="text-text-muted hover:text-danger p-0.5"
                                            >
                                              <Trash2 className="h-3 w-3" />
                                            </button>
                                          </div>
                                        </div>

                                        <ActivityFieldsEditor
                                          kind={activity.kind}
                                          fields={activity.fields}
                                          idPrefix={activity.id}
                                          onChange={(fields) =>
                                            updateActivity(uIdx, lIdx, aIdx, {
                                              fields,
                                            })
                                          }
                                        />

                                        {issues.length > 0 && (
                                          <ul className="space-y-0.5 rounded bg-warning/10 px-2 py-1.5 text-[11px] text-warning-accent">
                                            {issues.map((issue) => (
                                              <li
                                                key={issue}
                                                className="flex items-start gap-1.5"
                                              >
                                                <AlertCircle className="mt-0.5 h-3 w-3 shrink-0" />
                                                {t(
                                                  `studio.activity.issue.${issue}`,
                                                )}
                                              </li>
                                            ))}
                                          </ul>
                                        )}
                                      </div>
                                    );
                                  })}
                                </div>
                              </div>
                            )}
                          </div>
                        );
                      })}
                    </div>
                  </div>
                )}
              </div>
            );
          })}
        </div>
      </section>
    </div>
  );
}
