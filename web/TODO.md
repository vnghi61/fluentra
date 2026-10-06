# Frontend backlog

- [ ] Migrate the remaining inline `aria-modal` dialogs to `components/ui/modal.tsx`
  (learning Runner ExitDialog/ReportDialog/SaveProgressPrompt, account
  ChangePasswordModal/AvatarUploadModal/DevicesList/DataPrivacySettings, studio
  SubmissionStatusModal, resource ResourceDetail, MyWritingPage, MySpeakingPage,
  speaking AudioRecorder, PlacementRunner, exam practice dialog; admin modals are
  deferred to the admin workflow), then drop the `html:has([aria-modal])` rule in
  `index.css`.

- [x] Group 2: responsive admin shell, Learners and Question bank (anchored
  sidebar, account menu with email, More-sheet controls slot, learner and
  question tables/cards with row menus, `CopyableId`, section-local states).
  Desktop tables scroll horizontally below their minimum width (820px/760px)
  at md beside the expanded sidebar.

- [x] Keep the open-sitting resume link visible across the mock-test hub and explain
  fixed/random/custom start conflicts in Vietnamese and English.
- [x] Show a proper empty state for lessons and practice sets without activities;
  never display a phantom first activity or award completion.
- [x] Link all practice skills to published foundation courses and explain the
  different A2/B1/B2 difficulty despite the shared exercise-count composition.
- [x] Poll explicit resource processing states, recheck content without reuploading,
  block unsuitable files and open existing ready exercises directly.
- [x] Render answer feedback and explanations only in the chosen interface
  language instead of English and Vietnamese together.
- [ ] Localized server grader feedback (feedback_vi on the objective graders)
  needs an OpenAPI change; until then English-only feedback (e.g.
  reading/listening "Reading speed: N WPM.") is hidden in Vietnamese.
## Dashboard refinement — active scope

- [x] Compact supporting cards and empty states; pair FoundationNext with SkillProgress in Row 3.
- [x] Keep all weekly lesson links reachable through View all/Show less.
- [x] Keep streak freeze and leaderboard opt-in without duplicate streak panels.
- [x] Rebuild DashboardPage grid (1240px container, dynamic greeting, TodayProgressCard, 4-row grid, mobile order).
- [x] Update DashboardSkeleton to mirror new 4-row grid.
- [x] Pass frontend typecheck/lint and targeted regression checks; initial-load
  budget increased to 500 kB gzip with explicit user approval.
- [ ] Per-day weekly strip in WeeklyPlanCard once the API exposes per-day minutes.
- [ ] Daily-set done/total and estimated minutes in DailyPracticeCard once the API exposes them.
- [x] Capture EN/VI × light/dark at 320/390/768/1440 against a running app;
  inspect mobile/tablet/desktop screenshots (geometry fixtures, not live data).
- [ ] Measure a same-state baseline before claiming 20–30% reduction.
- [x] Resolve the 11 Studio Go lint findings and missing CourseDraftDetail OpenAPI
  registration caught by the repository-wide checks.
- [x] Read typed latest submission/report from draft detail, poll async Gate 1,
  and distinguish loading failures from a draft with no submissions.
- [x] Final verification: `make check`, frontend format/typecheck/lint/build,
  62 Vitest files / 535 tests, targeted PostgreSQL integration (Studio repository,
  learning service, seed), and the EN/VI responsive Dashboard capture matrix pass.
  This does not replace full real-account end-to-end acceptance.

## Product UI redesign — follow-on scope

- [x] Move exam practice options below md into a BottomSheet from a "Practice
  options" button; keep resume link visible on mobile outside the sheet,
  preserving md+ inline layout.
- [x] Lead PlacementIntro with a summary (duration, item count only if
  present in data/props, post-test outcome) above collapsible details,
  keeping primary start action visible without scrolling on 375px.
- [x] Redesign learner Progress page with tighter header, API-backed KPIs (completed
  activities, courses in progress, courses completed), CEFR mastery list with tinted
  progress bars, catalogue-backed course list with client-side status filter,
  ProgressSkeleton, and progress error state.
- [ ] Study minutes, streak and weekly deltas on /progress wait for dedicated fields
  on GET /me/progress.
- [ ] Visual captures and the frontend quality gates are unverified in this
  non-interactive worker run.
- [ ] Admin question bank shows a humanized kind as the card title because the
  Question schema (api/openapi/components/questionbank.yaml) has no prompt
  or summary field; adding one needs an OpenAPI change.
- [ ] Finish and visually verify source-adapted dashboard, course catalogue,
  practice/progress and grouped account settings against the latest brief.
- [ ] Run frontend quality gates and obtain independent review of the full UI
  changes; functional test PASS alone does not establish visual acceptance.
- [ ] Finish internal banner management in its separate API/schema scope.
- [ ] Export the complete screen matrix with an index and label mock/live data.
