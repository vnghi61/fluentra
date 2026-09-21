import React, { useEffect, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";

import { Button } from "@/components/ui/button";
import {
  useLearningProfile,
  useUpdateLearningProfile,
} from "@/features/account";
import {
  learningKeys,
  placementApi,
  placementProblemCode,
  usePlacementOverview,
} from "@/features/learning";
import { cn } from "@/lib/utils";

const EXAMS = ["none", "ielts", "toeic"] as const;
const TARGET_LEVELS = ["A1", "A2", "B1", "B2", "C1", "C2"] as const;
const STARTING_LEVELS = ["A1", "A2", "B1", "B2", "C1"] as const;
const WEEKLY_MINUTES = [60, 90, 150, 300] as const;

type Exam = (typeof EXAMS)[number];
type Level = (typeof TARGET_LEVELS)[number];

const choiceClass = (selected: boolean) =>
  cn(
    "min-h-[44px] rounded-xl border px-3 py-2 text-base font-medium transition-colors",
    selected
      ? "border-primary bg-primary/10 text-text"
      : "border-border-subtle bg-surface-base text-text hover:border-border",
  );

/**
 * Onboarding, shown once after sign-up to a learner with no learning profile:
 * the goal, the minutes a week, then the placement test or a chosen level.
 * Saving the profile is what ends it, so it never shows again.
 */
export function WelcomePage(): React.JSX.Element {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const profile = useLearningProfile();
  const overview = usePlacementOverview();
  const saveProfile = useUpdateLearningProfile();

  const [step, setStep] = useState<1 | 2 | 3>(1);
  const [exam, setExam] = useState<Exam>("none");
  const [target, setTarget] = useState<Level>("B2");
  const [minutes, setMinutes] = useState<number>(90);
  const [choosing, setChoosing] = useState(false);
  const [declared, setDeclared] = useState<Level>("A2");
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (profile.data) void navigate({ to: "/" });
  }, [profile.data, navigate]);

  const save = (declaredLevel: Level | null) =>
    saveProfile.mutateAsync({
      declared_level: declaredLevel,
      target_level: target,
      target_exam: exam,
      weekly_minutes_goal: minutes,
      motivations: [],
    });

  const takeTest = async () => {
    setPending(true);
    setError(null);
    try {
      try {
        const session = await placementApi.start();
        queryClient.setQueryData(
          learningKeys.placementSession(session.id),
          session,
        );
      } catch (err: unknown) {
        if (placementProblemCode(err) !== "PLACEMENT_IN_PROGRESS") throw err;
      }
      await save(null);
      void navigate({ to: "/placement" });
    } catch {
      setError(t("welcome.startFailed"));
      setChoosing(true);
      setPending(false);
    }
  };

  const chooseLevel = async () => {
    setPending(true);
    setError(null);
    try {
      await save(declared);
      void navigate({ to: "/" });
    } catch {
      setError(t("welcome.saveFailed"));
      setPending(false);
    }
  };

  const inviteAvailable = overview.data?.invite_available === true;

  return (
    <div className="flex min-h-screen items-center justify-center bg-surface-base px-4 py-10">
      <div className="w-full max-w-xl space-y-6 rounded-2xl border border-border bg-surface-card p-6 shadow-lg md:p-8">
        <p className="text-sm font-medium text-text-muted">
          {t("welcome.step", { current: step, total: 3 })}
        </p>

        {step === 1 && (
          <section className="space-y-5">
            <h1 className="text-2xl font-extrabold text-text">
              {t("welcome.goalTitle")}
            </h1>
            <div className="space-y-2">
              <p className="text-sm font-semibold text-text">
                {t("welcome.examLabel")}
              </p>
              <div
                className="grid grid-cols-1 gap-2 sm:grid-cols-3"
                role="group"
                aria-label={t("welcome.examLabel")}
              >
                {EXAMS.map((value) => (
                  <button
                    key={value}
                    type="button"
                    aria-pressed={exam === value}
                    onClick={() => setExam(value)}
                    className={choiceClass(exam === value)}
                  >
                    {value === "none"
                      ? t("welcome.exam.none")
                      : value === "ielts"
                        ? t("welcome.exam.ielts")
                        : t("welcome.exam.toeic")}
                  </button>
                ))}
              </div>
            </div>
            <div className="space-y-2">
              <p className="text-sm font-semibold text-text">
                {t("welcome.targetLabel")}
              </p>
              <div
                className="grid grid-cols-3 gap-2 sm:grid-cols-6"
                role="group"
                aria-label={t("welcome.targetLabel")}
              >
                {TARGET_LEVELS.map((level) => (
                  <button
                    key={level}
                    type="button"
                    aria-pressed={target === level}
                    onClick={() => setTarget(level)}
                    className={choiceClass(target === level)}
                  >
                    {level}
                  </button>
                ))}
              </div>
            </div>
            <Button
              type="button"
              onClick={() => setStep(2)}
              className="min-h-[44px] w-full text-base"
            >
              {t("welcome.continue")}
            </Button>
          </section>
        )}

        {step === 2 && (
          <section className="space-y-5">
            <h1 className="text-2xl font-extrabold text-text">
              {t("welcome.minutesTitle")}
            </h1>
            <p className="text-base text-text-muted">
              {t("welcome.minutesSubtitle")}
            </p>
            <div
              className="grid grid-cols-2 gap-2"
              role="group"
              aria-label={t("welcome.minutesTitle")}
            >
              {WEEKLY_MINUTES.map((value) => (
                <button
                  key={value}
                  type="button"
                  aria-pressed={minutes === value}
                  onClick={() => setMinutes(value)}
                  className={choiceClass(minutes === value)}
                >
                  {t("welcome.minutesOption", { minutes: value })}
                </button>
              ))}
            </div>
            <div className="flex gap-3">
              <Button
                type="button"
                variant="outline"
                onClick={() => setStep(1)}
                className="min-h-[44px] text-base"
              >
                {t("welcome.back")}
              </Button>
              <Button
                type="button"
                onClick={() => setStep(3)}
                className="min-h-[44px] flex-1 text-base"
              >
                {t("welcome.continue")}
              </Button>
            </div>
          </section>
        )}

        {step === 3 && (
          <section className="space-y-5">
            <h1 className="text-2xl font-extrabold text-text">
              {t("welcome.placementTitle")}
            </h1>
            {inviteAvailable && !choosing ? (
              <>
                <p className="text-base text-text-muted">
                  {t("welcome.placementSubtitle")}
                </p>
                <Button
                  type="button"
                  onClick={() => void takeTest()}
                  disabled={pending}
                  className="min-h-[44px] w-full text-base"
                >
                  {pending ? t("welcome.starting") : t("welcome.takeTest")}
                </Button>
                <Button
                  type="button"
                  variant="ghost"
                  onClick={() => setChoosing(true)}
                  className="min-h-[44px] w-full text-base"
                >
                  {t("welcome.chooseInstead")}
                </Button>
              </>
            ) : (
              <>
                <p className="text-base text-text-muted">
                  {inviteAvailable
                    ? t("welcome.chooseSubtitle")
                    : t("welcome.notOpen")}
                </p>
                <div
                  className="grid grid-cols-5 gap-2"
                  role="group"
                  aria-label={t("welcome.chooseTitle")}
                >
                  {STARTING_LEVELS.map((level) => (
                    <button
                      key={level}
                      type="button"
                      aria-pressed={declared === level}
                      onClick={() => setDeclared(level)}
                      className={choiceClass(declared === level)}
                    >
                      {level}
                    </button>
                  ))}
                </div>
                <Button
                  type="button"
                  onClick={() => void chooseLevel()}
                  disabled={pending}
                  className="min-h-[44px] w-full text-base"
                >
                  {pending ? t("welcome.saving") : t("welcome.finish")}
                </Button>
              </>
            )}
            {error && (
              <p
                role="alert"
                className="rounded-lg bg-danger/10 p-3 text-sm text-danger"
              >
                {error}
              </p>
            )}
            <Button
              type="button"
              variant="outline"
              onClick={() => setStep(2)}
              className="min-h-[44px] text-base"
            >
              {t("welcome.back")}
            </Button>
          </section>
        )}
      </div>
    </div>
  );
}

export default WelcomePage;
