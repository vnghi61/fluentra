import React, { memo, useCallback, useState } from "react";
import { useTranslation } from "react-i18next";
import {
  AlertCircle,
  ArrowDown,
  ArrowUp,
  CheckCircle2,
  ChevronDown,
  Copy,
  FolderPlus,
  Paperclip,
  Plus,
  Trash2,
} from "lucide-react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Input } from "@/components/ui/input";
import { NativeSelect } from "@/components/ui/native-select";
import { cn } from "@/lib/utils";
import {
  activityIssues,
  emptyFields,
  type ActivityFields,
  type ActivityKind,
  type DraftActivity,
} from "../model/activityKinds";
import {
  LIMITS,
  lessonCounts,
  lessonValid,
  moveItem,
  newActivity,
  newId,
  newLesson,
  newUnit,
  type LessonItem,
  type UnitItem,
} from "../model/courseStructure";
import { ActivityFieldsEditor } from "./ActivityFieldsEditor";
import { EXERCISE_GROUPS, EXERCISE_KINDS, KIND_ICON } from "./activityCatalog";

/** An immer recipe over the outline; the editor owns the state. */
export type UpdateUnits = (recipe: (draft: UnitItem[]) => void) => void;

interface PendingDelete {
  title: string;
  run: () => void;
}

interface Props {
  units: UnitItem[];
  update: UpdateUnits;
}

/**
 * The outline: chapters hold lessons, lessons hold exercises and materials.
 * Each level collapses, reorders with buttons (keyboard and touch friendly,
 * no drag library) and confirms before deleting anything that has content.
 */
export function CurriculumBuilder({ units, update }: Props): React.JSX.Element {
  const { t } = useTranslation();
  const [open, setOpen] = useState<Set<string>>(
    () =>
      new Set([
        ...units.slice(0, 1).map((u) => u.id),
        ...(units[0]?.lessons.slice(0, 1).map((l) => l.id) ?? []),
      ]),
  );
  const [pendingDelete, setPendingDelete] = useState<PendingDelete | null>(
    null,
  );

  const toggle = useCallback((id: string, force?: boolean) => {
    setOpen((prev) => {
      const next = new Set(prev);
      if (force ?? !next.has(id)) next.add(id);
      else next.delete(id);
      return next;
    });
  }, []);

  const confirmDelete = useCallback((title: string, run: () => void) => {
    setPendingDelete({ title, run });
  }, []);

  const addUnit = () => {
    const unit = newUnit(
      t("studio.editor.defaultUnitTitle", { n: units.length + 1 }),
    );
    update((d) => {
      d.push(unit);
    });
    toggle(unit.id, true);
  };

  return (
    <section aria-labelledby="curriculum-heading" className="space-y-4">
      <div className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <h2
            id="curriculum-heading"
            className="text-lg font-semibold leading-snug text-text md:text-xl"
          >
            {t("studio.editor.outlineTitle")}
          </h2>
          <p className="text-sm text-text-muted">
            {t("studio.editor.outlineHint")}
          </p>
        </div>
        <Button
          variant="outline"
          onClick={addUnit}
          disabled={units.length >= LIMITS.units}
        >
          <FolderPlus className="h-4 w-4" aria-hidden="true" />
          {t("studio.editor.addUnit")}
        </Button>
      </div>

      {units.length === 0 ? (
        <div className="rounded-2xl border-2 border-dashed border-border px-6 py-10 text-center">
          <p className="text-sm font-semibold text-text">
            {t("studio.editor.emptyOutline")}
          </p>
          <Button className="mt-4" onClick={addUnit}>
            <Plus className="h-4 w-4" aria-hidden="true" />
            {t("studio.editor.addUnit")}
          </Button>
        </div>
      ) : (
        <ol className="space-y-4">
          {units.map((unit, uIdx) => (
            <UnitCard
              key={unit.id}
              unit={unit}
              uIdx={uIdx}
              unitCount={units.length}
              open={open}
              toggle={toggle}
              update={update}
              confirmDelete={confirmDelete}
            />
          ))}
        </ol>
      )}

      <ConfirmDialog
        open={pendingDelete !== null}
        onOpenChange={(o) => !o && setPendingDelete(null)}
        destructive
        title={t("studio.editor.confirmDeleteTitle", {
          name: pendingDelete?.title ?? "",
        })}
        description={t("studio.editor.confirmDeleteDesc")}
        confirmLabel={t("app.delete", "Delete")}
        onConfirm={() => {
          pendingDelete?.run();
          setPendingDelete(null);
        }}
      />
    </section>
  );
}

/** Up, down and delete, at a 44 px touch size, with labels for screen readers. */
function RowActions({
  index,
  count,
  onMove,
  onDelete,
  labels,
}: {
  index: number;
  count: number;
  onMove: (delta: -1 | 1) => void;
  onDelete: () => void;
  labels: { up: string; down: string; remove: string };
}): React.JSX.Element {
  return (
    <div className="flex shrink-0 items-center">
      <Button
        variant="ghost"
        size="icon"
        onClick={() => onMove(-1)}
        disabled={index === 0}
        aria-label={labels.up}
        title={labels.up}
        className="hidden text-text-muted sm:inline-flex"
      >
        <ArrowUp className="h-4 w-4" aria-hidden="true" />
      </Button>
      <Button
        variant="ghost"
        size="icon"
        onClick={() => onMove(1)}
        disabled={index === count - 1}
        aria-label={labels.down}
        title={labels.down}
        className="hidden text-text-muted sm:inline-flex"
      >
        <ArrowDown className="h-4 w-4" aria-hidden="true" />
      </Button>
      <Button
        variant="ghost"
        size="icon"
        onClick={onDelete}
        aria-label={labels.remove}
        title={labels.remove}
        className="text-text-muted hover:text-danger"
      >
        <Trash2 className="h-4 w-4" aria-hidden="true" />
      </Button>
    </div>
  );
}

function CollapseButton({
  expanded,
  onClick,
  controls,
  label,
}: {
  expanded: boolean;
  onClick: () => void;
  controls: string;
  label: string;
}): React.JSX.Element {
  return (
    <Button
      variant="ghost"
      size="icon"
      onClick={onClick}
      aria-expanded={expanded}
      aria-controls={controls}
      aria-label={label}
      className="shrink-0 text-text-muted"
    >
      <ChevronDown
        className={cn(
          "h-5 w-5 transition-transform duration-150 motion-reduce:transition-none",
          !expanded && "-rotate-90",
        )}
        aria-hidden="true"
      />
    </Button>
  );
}

interface UnitCardProps {
  unit: UnitItem;
  uIdx: number;
  unitCount: number;
  open: Set<string>;
  toggle: (id: string, force?: boolean) => void;
  update: UpdateUnits;
  confirmDelete: (title: string, run: () => void) => void;
}

function UnitCard({
  unit,
  uIdx,
  unitCount,
  open,
  toggle,
  update,
  confirmDelete,
}: UnitCardProps): React.JSX.Element {
  const { t } = useTranslation();
  const expanded = open.has(unit.id);
  const bodyId = `${unit.id}-body`;
  const counts = unit.lessons.reduce(
    (acc, l) => {
      const c = lessonCounts(l);
      return {
        exercises: acc.exercises + c.exercises,
        materials: acc.materials + c.materials,
      };
    },
    { exercises: 0, materials: 0 },
  );
  const unitTitle =
    unit.title || t("studio.editor.defaultUnitTitle", { n: uIdx + 1 });

  const addLesson = () => {
    const lesson = newLesson(
      t("studio.editor.defaultLessonTitle", { n: unit.lessons.length + 1 }),
    );
    update((d) => {
      d[uIdx]?.lessons.push(lesson);
    });
    toggle(unit.id, true);
    toggle(lesson.id, true);
  };

  const remove = () =>
    update((d) => {
      d.splice(uIdx, 1);
    });

  return (
    <li className="overflow-hidden rounded-2xl border border-border-subtle bg-surface-card shadow-[var(--shadow-card)]">
      <div className="flex items-center gap-1 border-b border-border-subtle bg-surface-muted/60 py-2 pl-1 pr-2 sm:pl-2">
        <CollapseButton
          expanded={expanded}
          onClick={() => toggle(unit.id)}
          controls={bodyId}
          label={t("studio.editor.toggleSection", { name: unitTitle })}
        />
        <span
          className="hidden h-8 shrink-0 items-center rounded-lg bg-primary/10 px-2.5 text-xs font-bold text-primary-accent sm:inline-flex"
          aria-hidden="true"
        >
          {t("studio.editor.unitShort", { n: uIdx + 1 })}
        </span>
        <Input
          value={unit.title}
          onChange={(e) => {
            const value = e.target.value;
            update((d) => {
              const u = d[uIdx];
              if (u) u.title = value;
            });
          }}
          placeholder={t("studio.editor.unitTitlePlaceholder")}
          aria-label={t("studio.editor.unitTitleLabel", { n: uIdx + 1 })}
          maxLength={120}
          className="min-w-0 flex-1 border-transparent bg-transparent px-2 font-semibold hover:border-border focus-visible:bg-surface"
        />
        <span className="hidden shrink-0 whitespace-nowrap text-sm text-text-muted md:inline">
          {t("studio.editor.unitSummary", {
            lessons: unit.lessons.length,
            exercises: counts.exercises,
          })}
          {counts.materials > 0 &&
            ` · ${t("studio.editor.materialsCount", { count: counts.materials })}`}
        </span>
        <RowActions
          index={uIdx}
          count={unitCount}
          onMove={(delta) => update((d) => moveItem(d, uIdx, delta))}
          onDelete={() =>
            unit.lessons.length > 0
              ? confirmDelete(unitTitle, remove)
              : remove()
          }
          labels={{
            up: t("studio.editor.moveUp"),
            down: t("studio.editor.moveDown"),
            remove: t("studio.editor.deleteUnit"),
          }}
        />
      </div>

      {expanded && (
        <div id={bodyId} className="space-y-3 p-3 sm:p-4">
          {unit.lessons.length === 0 && (
            <p className="rounded-xl bg-surface-muted/60 px-4 py-3 text-sm text-text-muted">
              {t("studio.editor.emptyUnit")}
            </p>
          )}
          <ol className="space-y-3">
            {unit.lessons.map((lesson, lIdx) => (
              <LessonCard
                key={lesson.id}
                lesson={lesson}
                uIdx={uIdx}
                lIdx={lIdx}
                lessonCount={unit.lessons.length}
                open={open}
                toggle={toggle}
                update={update}
                confirmDelete={confirmDelete}
              />
            ))}
          </ol>
          <Button
            variant="ghost"
            onClick={addLesson}
            disabled={unit.lessons.length >= LIMITS.lessonsPerUnit}
            className="w-full justify-center border border-dashed border-border sm:w-auto"
          >
            <Plus className="h-4 w-4" aria-hidden="true" />
            {t("studio.editor.addLessonLong")}
          </Button>
        </div>
      )}
    </li>
  );
}

interface LessonCardProps {
  lesson: LessonItem;
  uIdx: number;
  lIdx: number;
  lessonCount: number;
  open: Set<string>;
  toggle: (id: string, force?: boolean) => void;
  update: UpdateUnits;
  confirmDelete: (title: string, run: () => void) => void;
}

function LessonCard({
  lesson,
  uIdx,
  lIdx,
  lessonCount,
  open,
  toggle,
  update,
  confirmDelete,
}: LessonCardProps): React.JSX.Element {
  const { t } = useTranslation();
  const expanded = open.has(lesson.id);
  const bodyId = `${lesson.id}-body`;
  const { exercises, materials } = lessonCounts(lesson);
  const valid = lessonValid(lesson);
  const full = lesson.activities.length >= LIMITS.activitiesPerLesson;
  const lessonTitle =
    lesson.title || t("studio.editor.defaultLessonTitle", { n: lIdx + 1 });

  const add = (kind: ActivityKind) => {
    const activity = newActivity(
      kind,
      kind === "lesson_material"
        ? t("studio.editor.defaultMaterialTitle")
        : t(`studio.activity.kind.${kind}`),
    );
    update((d) => {
      d[uIdx]?.lessons[lIdx]?.activities.push(activity);
    });
    toggle(lesson.id, true);
    toggle(activity.id, true);
  };

  const remove = () =>
    update((d) => {
      d[uIdx]?.lessons.splice(lIdx, 1);
    });

  return (
    <li className="rounded-xl border border-border-subtle bg-surface">
      <div className="flex items-center gap-1 py-1.5 pl-1 pr-2">
        <CollapseButton
          expanded={expanded}
          onClick={() => toggle(lesson.id)}
          controls={bodyId}
          label={t("studio.editor.toggleSection", { name: lessonTitle })}
        />
        <span
          className="hidden shrink-0 text-xs font-semibold tabular-nums text-text-muted sm:inline"
          aria-hidden="true"
        >
          {uIdx + 1}.{lIdx + 1}
        </span>
        <Input
          value={lesson.title}
          onChange={(e) => {
            const value = e.target.value;
            update((d) => {
              const l = d[uIdx]?.lessons[lIdx];
              if (l) l.title = value;
            });
          }}
          placeholder={t("studio.editor.lessonTitlePlaceholder")}
          aria-label={t("studio.editor.lessonTitleLabel", {
            n: `${uIdx + 1}.${lIdx + 1}`,
          })}
          maxLength={120}
          className="min-w-0 flex-1 border-transparent bg-transparent px-2 font-medium hover:border-border focus-visible:bg-surface"
        />
        <span
          className={cn(
            "hidden shrink-0 items-center gap-1.5 whitespace-nowrap text-sm md:inline-flex",
            valid ? "text-text-muted" : "text-warning-accent",
          )}
        >
          {valid ? (
            <CheckCircle2 className="h-4 w-4 text-success" aria-hidden="true" />
          ) : (
            <AlertCircle className="h-4 w-4" aria-hidden="true" />
          )}
          {t("studio.editor.lessonSummary", { exercises, materials })}
        </span>
        <RowActions
          index={lIdx}
          count={lessonCount}
          onMove={(delta) =>
            update((d) => {
              const u = d[uIdx];
              if (u) moveItem(u.lessons, lIdx, delta);
            })
          }
          onDelete={() =>
            lesson.activities.length > 0
              ? confirmDelete(lessonTitle, remove)
              : remove()
          }
          labels={{
            up: t("studio.editor.moveUp"),
            down: t("studio.editor.moveDown"),
            remove: t("studio.editor.deleteLesson"),
          }}
        />
      </div>

      {expanded && (
        <div
          id={bodyId}
          className="space-y-3 border-t border-border-subtle p-3"
        >
          {!valid && (
            <p className="flex items-start gap-2 rounded-lg bg-warning/10 px-3 py-2 text-sm font-medium text-warning-accent">
              <AlertCircle
                className="mt-0.5 h-4 w-4 shrink-0"
                aria-hidden="true"
              />
              {t("studio.editor.lessonRule")}
            </p>
          )}

          {lesson.activities.length > 0 && (
            <ol className="space-y-2">
              {lesson.activities.map((activity, aIdx) => (
                <ActivityItem
                  key={activity.id}
                  activity={activity}
                  uIdx={uIdx}
                  lIdx={lIdx}
                  aIdx={aIdx}
                  count={lesson.activities.length}
                  expanded={open.has(activity.id)}
                  toggle={toggle}
                  update={update}
                />
              ))}
            </ol>
          )}

          <div className="grid gap-2 sm:flex sm:flex-wrap">
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button variant="outline" disabled={full}>
                  <Plus className="h-4 w-4" aria-hidden="true" />
                  {t("studio.editor.addExercise")}
                  <ChevronDown
                    className="h-4 w-4 opacity-60"
                    aria-hidden="true"
                  />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent
                align="start"
                className="max-h-[min(70vh,28rem)] w-72 overflow-y-auto"
              >
                {EXERCISE_GROUPS.map((group, gi) => (
                  <React.Fragment key={group.key}>
                    {gi > 0 && <DropdownMenuSeparator />}
                    <DropdownMenuLabel>
                      {t(`studio.editor.group.${group.key}`)}
                    </DropdownMenuLabel>
                    {group.kinds.map((kind) => {
                      const Icon = KIND_ICON[kind];
                      return (
                        <DropdownMenuItem key={kind} onSelect={() => add(kind)}>
                          <Icon
                            className="h-4 w-4 shrink-0 text-primary-accent"
                            aria-hidden="true"
                          />
                          {t(`studio.activity.kind.${kind}`)}
                        </DropdownMenuItem>
                      );
                    })}
                  </React.Fragment>
                ))}
              </DropdownMenuContent>
            </DropdownMenu>
            <Button
              variant="outline"
              disabled={full}
              onClick={() => add("lesson_material")}
            >
              <Paperclip className="h-4 w-4" aria-hidden="true" />
              {t("studio.editor.addMaterial")}
            </Button>
          </div>
        </div>
      )}
    </li>
  );
}

interface ActivityItemProps {
  activity: DraftActivity;
  uIdx: number;
  lIdx: number;
  aIdx: number;
  count: number;
  expanded: boolean;
  toggle: (id: string, force?: boolean) => void;
  update: UpdateUnits;
}

/**
 * One exercise or material. Memoised: immer keeps untouched activities
 * referentially equal, so typing in one does not re-render the other twenty.
 */
const ActivityItem = memo(function ActivityItem({
  activity,
  uIdx,
  lIdx,
  aIdx,
  count,
  expanded,
  toggle,
  update,
}: ActivityItemProps): React.JSX.Element {
  const { t } = useTranslation();
  const issues = activityIssues(activity);
  const Icon = KIND_ICON[activity.kind];
  const isMaterial = activity.kind === "lesson_material";
  const bodyId = `${activity.id}-body`;

  const edit = (recipe: (a: DraftActivity) => void) =>
    update((d) => {
      const a = d[uIdx]?.lessons[lIdx]?.activities[aIdx];
      if (a) recipe(a);
    });
  const onFieldsChange = (patch: Partial<ActivityFields>) =>
    edit((a) => {
      Object.assign(a.fields, patch);
    });

  return (
    <li
      className={cn(
        "rounded-xl border bg-surface-card",
        issues.length > 0 && expanded
          ? "border-warning/40"
          : "border-border-subtle",
      )}
    >
      <div className="flex items-center gap-1 py-1 pl-1 pr-1.5">
        <CollapseButton
          expanded={expanded}
          onClick={() => toggle(activity.id)}
          controls={bodyId}
          label={t("studio.editor.toggleSection", {
            name: activity.title || t(`studio.activity.kind.${activity.kind}`),
          })}
        />
        <span
          className={cn(
            "flex h-8 w-8 shrink-0 items-center justify-center rounded-lg",
            isMaterial
              ? "bg-success/10 text-success-accent"
              : "bg-primary/10 text-primary-accent",
          )}
          title={t(`studio.activity.kind.${activity.kind}`)}
          aria-hidden="true"
        >
          <Icon className="h-4 w-4" />
        </span>
        <div className="min-w-0 flex-1 px-1">
          <input
            type="text"
            value={activity.title}
            onChange={(e) => {
              const value = e.target.value;
              edit((a) => {
                a.title = value;
              });
            }}
            placeholder={t("studio.activity.titlePlaceholder")}
            aria-label={t("studio.activity.titlePlaceholder")}
            maxLength={120}
            className="min-h-11 w-full truncate rounded-md bg-transparent px-1 text-base font-medium text-text placeholder:text-text-muted focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary md:text-sm"
          />
          <p className="truncate px-1 text-sm text-text-muted">
            {t(`studio.activity.kind.${activity.kind}`)}
          </p>
        </div>
        {issues.length > 0 ? (
          <Badge variant="warning" className="shrink-0 gap-1 whitespace-nowrap">
            <AlertCircle className="h-4 w-4" aria-hidden="true" />
            {t("studio.editor.issueCount", { count: issues.length })}
          </Badge>
        ) : (
          <CheckCircle2
            className="mx-1 h-4 w-4 shrink-0 text-success"
            role="img"
            aria-label={t("studio.editor.activityComplete")}
          />
        )}
        <Button
          variant="ghost"
          size="icon"
          onClick={() => {
            const copy: DraftActivity = {
              ...structuredClone(activity),
              id: newId("a"),
              // A copied material would point at the same private file; the
              // creator uploads its own for the copy.
              ...(isMaterial && {
                fields: {
                  ...emptyFields(),
                  materialTitle: activity.fields.materialTitle,
                },
              }),
            };
            update((d) => {
              d[uIdx]?.lessons[lIdx]?.activities.splice(aIdx + 1, 0, copy);
            });
          }}
          disabled={count >= LIMITS.activitiesPerLesson}
          aria-label={t("studio.editor.duplicate")}
          title={t("studio.editor.duplicate")}
          className="hidden text-text-muted sm:inline-flex"
        >
          <Copy className="h-4 w-4" aria-hidden="true" />
        </Button>
        <RowActions
          index={aIdx}
          count={count}
          onMove={(delta) =>
            update((d) => {
              const l = d[uIdx]?.lessons[lIdx];
              if (l) moveItem(l.activities, aIdx, delta);
            })
          }
          onDelete={() =>
            update((d) => {
              d[uIdx]?.lessons[lIdx]?.activities.splice(aIdx, 1);
            })
          }
          labels={{
            up: t("studio.editor.moveUp"),
            down: t("studio.editor.moveDown"),
            remove: t("studio.activity.remove"),
          }}
        />
      </div>

      {expanded && (
        <div
          id={bodyId}
          className="space-y-4 border-t border-border-subtle p-3 sm:p-4"
        >
          {!isMaterial && (
            <div className="max-w-xs space-y-1.5">
              <label
                htmlFor={`${activity.id}-kind`}
                className="block text-sm font-semibold text-text"
              >
                {t("studio.activity.kindLabel")}
              </label>
              <NativeSelect
                id={`${activity.id}-kind`}
                value={activity.kind}
                onChange={(e) => {
                  const kind = e.target.value as ActivityKind;
                  edit((a) => {
                    a.kind = kind;
                  });
                }}
              >
                {EXERCISE_KINDS.map((kind) => (
                  <option key={kind} value={kind}>
                    {t(`studio.activity.kind.${kind}`)}
                  </option>
                ))}
              </NativeSelect>
            </div>
          )}

          <ActivityFieldsEditor
            kind={activity.kind}
            fields={activity.fields}
            idPrefix={activity.id}
            onChange={onFieldsChange}
          />

          {issues.length > 0 && (
            <ul className="space-y-1 rounded-lg bg-warning/10 px-3 py-2 text-sm text-warning-accent">
              {issues.map((issue) => (
                <li key={issue} className="flex items-start gap-1.5">
                  <AlertCircle
                    className="mt-0.5 h-4 w-4 shrink-0"
                    aria-hidden="true"
                  />
                  {t(`studio.activity.issue.${issue}`)}
                </li>
              ))}
            </ul>
          )}
        </div>
      )}
    </li>
  );
});
