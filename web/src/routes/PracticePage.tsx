import React from "react";
import { useTranslation } from "react-i18next";
import { Link } from "@tanstack/react-router";
import {
  AlertCircle,
  BookMarked,
  BookOpen,
  FolderOpen,
  Headphones,
  Layers,
  Mic,
  PenTool,
  Sparkles,
} from "lucide-react";

import { Button } from "@/components/ui/button";
import {
  Card,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { DailyPracticeCard, GuestNotice } from "@/features/learning";
import { useCourse } from "@/features/lesson";
import { useAuthStore } from "@/stores/authStore";
import {
  ForecastStrip,
  ReviewQueueCard,
  useDueCount,
  useForecast,
} from "@/features/review";

/**
 * The practice hub.
 *
 * It replaces a twelve-line placeholder that existed "so the router has
 * something to route to" — while the sidebar advertised Practice, and the
 * dashboard's two review buttons both pointed here. Every one of those led to
 * a heading and a tagline.
 *
 * The page owns no learning logic. It is a door: due count and the way into
 * the session, then the week ahead so an empty queue still says something.
 */
/** The slug cmd/worker's practice generator upserts its course under. */
const generatedPracticeSlug = "generated-vocabulary-practice";

export function PracticePage(): React.JSX.Element {
  const { t } = useTranslation();
  // Review cards belong to a person. A guest has none, and asking for them
  // would earn a 401 on a page they are allowed to be on — which reads as a
  // bug rather than as the honest "there is nothing here for you yet".
  const signedIn = useAuthStore((state) => state.status === "authenticated");
  // The slug the practice generator upserts under. A 404 here is the ordinary
  // case on a deployment whose generator has not run yet, so the card simply
  // does not render.
  const generatedPractice = useCourse(generatedPracticeSlug);
  const due = useDueCount(signedIn);
  const forecast = useForecast(signedIn);

  return (
    <div className="space-y-6 animate-in fade-in duration-200">
      <header className="space-y-1">
        <h1 className="text-2xl md:text-3xl font-extrabold text-text tracking-tight">
          {t("practice.title", "Practice")}
        </h1>
        <p className="text-sm text-text-muted">
          {t(
            "practice.tagline",
            "Keep what you have learned. Reviews are scheduled for the day you are most likely to forget.",
          )}
        </p>
      </header>

      {/* Daily Practice Set */}
      <DailyPracticeCard />

      {!signedIn ? (
        <>
          <GuestNotice />
          <Card>
            <CardHeader>
              <div className="flex items-center gap-2 text-text-muted mb-1">
                <Layers className="h-5 w-5" aria-hidden="true" />
                <span className="text-xs font-semibold uppercase tracking-wider">
                  {t("practice.review.label", "Spaced repetition")}
                </span>
              </div>
              <CardTitle className="text-base font-semibold">
                {t("guest.practice.title", "Reviews need an account")}
              </CardTitle>
              <CardDescription>
                {t(
                  "guest.practice.desc",
                  "Cards are scheduled against the day you are likely to forget, which means they belong to a person. Sign in and the lessons you finish start filling this queue.",
                )}
              </CardDescription>
            </CardHeader>
            <CardFooter className="pt-0">
              <Link to="/learn">
                <Button variant="secondary">
                  {t("practice.review.emptyLearnBtn", "Go to a lesson")}
                </Button>
              </Link>
            </CardFooter>
          </Card>
        </>
      ) : due.isLoading ? (
        <Skeleton className="h-48 w-full rounded-xl" />
      ) : due.isError ? (
        <Card className="border-danger/30">
          <CardHeader>
            <div className="flex items-center gap-2 text-danger-accent mb-1">
              <AlertCircle className="h-5 w-5" aria-hidden="true" />
              <CardTitle className="text-base font-semibold">
                {t("practice.error.title", "Unable to load your review queue")}
              </CardTitle>
            </div>
            <CardDescription>
              {t(
                "practice.error.desc",
                "The review schedule could not be reached. Your progress is safe.",
              )}
            </CardDescription>
          </CardHeader>
          <div className="px-6 pb-6">
            <Button variant="secondary" onClick={() => void due.refetch()}>
              {t("action.retry", "Try again")}
            </Button>
          </div>
        </Card>
      ) : (
        <ReviewQueueCard dueCount={due.data?.due_count ?? 0} />
      )}

      {/*
        The forecast is supporting detail, so it fails quietly: a learner whose
        queue loaded does not need an error banner because a chart did not.
      */}
      {!signedIn ? null : forecast.isLoading ? (
        <Skeleton className="h-40 w-full rounded-xl" />
      ) : forecast.data ? (
        <ForecastStrip days={forecast.data.days} />
      ) : null}

      {/*
        The drills generated from the learner's own dictionary.

        They live in a course of their own, and that course is deliberately not
        in the catalogue: /learn is the authored syllabus, and a machine-made
        drill set sorting above it is what made the catalogue open on generated
        content. Practice is where it belongs, and this is the only door to it.

        Rendered only when the course exists. The generator creates it on its
        first run over a non-empty dictionary, so on a fresh deployment there is
        nothing here yet — and a card advertising an empty course would be the
        same lie the vocabulary card used to tell.
      */}
      {generatedPractice.data && (
        <Card>
          <CardHeader>
            <div className="flex items-center gap-2 text-text-muted mb-1">
              <Sparkles className="h-5 w-5" aria-hidden="true" />
              <span className="text-xs font-semibold uppercase tracking-wider">
                {t("practice.generated.label", "Generated drills")}
              </span>
            </div>
            <CardTitle className="text-base font-semibold">
              {generatedPractice.data.title}
            </CardTitle>
            <CardDescription>
              {t(
                "practice.generated.desc",
                "Exercises built from the words already in your dictionary. They grow as your vocabulary does.",
              )}
            </CardDescription>
          </CardHeader>
          <CardFooter className="pt-0">
            <Link to="/learn" search={{ course: generatedPracticeSlug }}>
              <Button variant="secondary" className="gap-2">
                <Sparkles className="h-4 w-4" aria-hidden="true" />
                {t("practice.generated.openBtn", "Open drills")}
              </Button>
            </Link>
          </CardFooter>
        </Card>
      )}

      {/*
        This card said "Word lists are not here yet" for as long as there was no
        screen behind it. There is one now, so it says what it does instead of
        apologising for what it does not.
      */}
      <Card>
        <CardHeader>
          <div className="flex items-center gap-2 text-text-muted mb-1">
            <BookMarked className="h-5 w-5" aria-hidden="true" />
            <span className="text-xs font-semibold uppercase tracking-wider">
              {t("practice.vocabulary.label", "Vocabulary")}
            </span>
          </div>
          <CardTitle className="text-base font-semibold">
            {t("practice.myWords.title", "Add your own words")}
          </CardTitle>
          <CardDescription>
            {t(
              "practice.myWords.desc",
              "Paste vocabulary from your own course. We check each word against a dictionary, write example sentences for it, and schedule it for review here.",
            )}
          </CardDescription>
        </CardHeader>
        <CardFooter className="pt-0">
          <Link to="/practice/my-words">
            <Button variant="secondary" className="gap-2">
              <BookMarked className="h-4 w-4" aria-hidden="true" />
              {t("uploads.openLink", "My words")}
            </Button>
          </Link>
        </CardFooter>
      </Card>

      {/*
        The learner's own files.

        A private library, not a shared bank (BR-RESOURCE-12): the page lists
        what they uploaded, what we read out of it, and — once Stage B lands —
        the practice generated from it.
      */}
      <Card>
        <CardHeader>
          <div className="flex items-center gap-2 text-text-muted mb-1">
            <FolderOpen className="h-5 w-5" aria-hidden="true" />
            <span className="text-xs font-semibold uppercase tracking-wider">
              {t("practice.resources.label", "Your material")}
            </span>
          </div>
          <CardTitle className="text-base font-semibold">
            {t("practice.resources.title", "My resources")}
          </CardTitle>
          <CardDescription>
            {t(
              "practice.resources.desc",
              "Upload a PDF, a recording or a video from your own course. We read it, estimate its level, and can build practice from it — and it stays private to you.",
            )}
          </CardDescription>
        </CardHeader>
        <CardFooter className="pt-0">
          <Link to="/my-resources">
            <Button variant="secondary" className="gap-2">
              <FolderOpen className="h-4 w-4" aria-hidden="true" />
              {t("practice.resources.openBtn", "Open my resources")}
            </Button>
          </Link>
        </CardFooter>
      </Card>

      {/* Reading Comprehension Card */}      <Card>
        <CardHeader>
          <div className="flex items-center gap-2 text-text-muted mb-1">
            <BookOpen className="h-5 w-5" aria-hidden="true" />
            <span className="text-xs font-semibold uppercase tracking-wider">
              {t("practice.reading.label", "Reading")}
            </span>
          </div>
          <CardTitle className="text-base font-semibold">
            {t("practice.reading.title", "Reading Comprehension")}
          </CardTitle>
          <CardDescription>
            {t(
              "practice.reading.desc",
              "Practice reading comprehension across curated passages from A2 to B2, with questions and speed tracking.",
            )}
          </CardDescription>
        </CardHeader>
        <CardFooter className="pt-0">
          <Link to="/learn" search={{ course: "reading-practice" }}>
            <Button variant="secondary" className="gap-2">
              <BookOpen className="h-4 w-4" aria-hidden="true" />
              {t("practice.reading.openBtn", "Start reading")}
            </Button>
          </Link>
        </CardFooter>
      </Card>

      {/*
        Listening Practice Card.

        Last of the four skills to get a door. The listening module has graded
        `listening_comprehension` since it was written and the play route has
        always counted plays against a learning attempt — but the runner had no
        renderer for the kind, so every listening item in the database sat in
        `pool-exam` or `pool-placement`, and the hub offered nothing to listen
        to. The renderer exists now and `listening-practice` is seeded beside
        the other three courses.

        The clips are rendered offline by `make tts`; until that has run on a
        deployment, the player says the recording is not ready rather than
        failing silently.
      */}
      <Card>
        <CardHeader>
          <div className="flex items-center gap-2 text-text-muted mb-1">
            <Headphones className="h-5 w-5" aria-hidden="true" />
            <span className="text-xs font-semibold uppercase tracking-wider">
              {t("practice.listening.label", "Listening")}
            </span>
          </div>
          <CardTitle className="text-base font-semibold">
            {t("practice.listening.title", "Announcements & Conversations")}
          </CardTitle>
          <CardDescription>
            {t(
              "practice.listening.desc",
              "Listen to announcements, conversations and short talks, then answer comprehension questions. Three plays per clip, and the transcript after marking.",
            )}
          </CardDescription>
        </CardHeader>
        <CardFooter className="pt-0">
          <Link to="/learn" search={{ course: "listening-practice" }}>
            <Button variant="secondary" className="gap-2">
              <Headphones className="h-4 w-4" aria-hidden="true" />
              {t("practice.listening.openBtn", "Start listening")}
            </Button>
          </Link>
        </CardFooter>
      </Card>

      {/* Writing Prompts & Essays Card */}
      <Card>
        <CardHeader>
          <div className="flex items-center gap-2 text-text-muted mb-1">
            <PenTool className="h-5 w-5" aria-hidden="true" />
            <span className="text-xs font-semibold uppercase tracking-wider">
              {t("practice.writing.label", "Writing")}
            </span>
          </div>
          <CardTitle className="text-base font-semibold">
            {t("practice.writing.title", "Writing Prompts & Essays")}
          </CardTitle>
          <CardDescription>
            {t(
              "practice.writing.desc",
              "Practice structured writing tasks from emails to IELTS essays with automated AI feedback and model answers.",
            )}
          </CardDescription>
        </CardHeader>
        <CardFooter className="pt-0">
          <Link to="/learn" search={{ course: "writing-practice" }}>
            <Button variant="secondary" className="gap-2">
              <PenTool className="h-4 w-4" aria-hidden="true" />
              {t("practice.writing.openBtn", "Start writing")}
            </Button>
          </Link>
        </CardFooter>
      </Card>

      {/*
        Speaking Practice Card.

        The hub offered reading and writing and nothing for speaking, while the
        grading pipeline sat finished behind an empty door: every speaking_task
        in the database belonged to the exam or placement pools, which no learner
        opens on purpose. `speaking-practice` is seeded alongside the other two
        courses, so this card leads somewhere — the rule the generated-drills
        card above states, applied here.
      */}
      <Card>
        <CardHeader>
          <div className="flex items-center gap-2 text-text-muted mb-1">
            <Mic className="h-5 w-5" aria-hidden="true" />
            <span className="text-xs font-semibold uppercase tracking-wider">
              {t("practice.speaking.label", "Speaking")}
            </span>
          </div>
          <CardTitle className="text-base font-semibold">
            {t("practice.speaking.title", "Read Aloud & Spoken Answers")}
          </CardTitle>
          <CardDescription>
            {t(
              "practice.speaking.desc",
              "Record answers and read passages aloud. You get a transcript, word accuracy, speaking rate and AI coaching — pronunciation is not assessed.",
            )}
          </CardDescription>
        </CardHeader>
        <CardFooter className="pt-0">
          <Link to="/learn" search={{ course: "speaking-practice" }}>
            <Button variant="secondary" className="gap-2">
              <Mic className="h-4 w-4" aria-hidden="true" />
              {t("practice.speaking.openBtn", "Start speaking")}
            </Button>
          </Link>
        </CardFooter>
      </Card>
    </div>
  );
}

export default PracticePage;
