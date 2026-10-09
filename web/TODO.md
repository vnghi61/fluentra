# Frontend backlog

- [x] Sidebar collapse/expand morphs view-transition snapshots instead of
      transitioning the width; theme switches reveal as a circle from the click
      point; language switches blur out/in. Frames inspected in headless
      Chromium (paused pseudo-element animations, 1280px and 390px).
- [ ] Watch the sidebar morph on a real low-end device: the page column still
      cross-fades its old and new layouts for `--motion-nav / 2`. Browsers without
      view transitions keep the width transition (now with a fixed-width footer).

- [x] Refresh the content library with real cross-page counts, slug-prefix search,
      status/kind filters, responsive table/cards and selectable offset page sizes.
      Wire Add content to the existing draft-creation API with permission checks,
      JSON validation and pending-save locking; separate copy-ID and detail actions.

- [x] Refine user management into one searchable directory with real result counts,
      readable identities/statuses/dates, mobile cards and a portalled filter Sheet.
      Add permission-aware page selection, filter reset and stale-response guards;
      retain cursor pagination, audit/detail actions and required justifications.
      Inspect desktop plus 320/390px component previews in Light/Dark; previews
      use test accounts, not a live-account end-to-end acceptance run.

- [x] Remove decorative theme previews and the duplicate bottom Settings link
      from mobile More; retain theme/language controls and all account-section links.

- [x] Fix the dashboard leaderboard link to a signed-in `/leaderboard` page,
      reuse API-backed weekly standings and distinguish load failures from empty data.

- [x] Refresh the lesson runner frame and sentence-rewrite answer form without
      hints; make reporting reachable on mobile before/after answering, with a
      portalled, scrollable report dialog and keyboard regression coverage.
- [x] Keep only the header report action; align the question, options and action
      footer within one responsive canvas and cover duplicate-report regression.
- [x] Refine payout bank-account form spacing, accessible help and mobile actions;
      show the bank picker above dialogs and prevent Enter from submitting.
- [x] Migrate lesson exit confirmation to a portalled AlertDialog with safe default
      focus, focus return, keyboard containment and accurate unsent-answer warning.
- [x] Inspect bank/exit popup component previews at 320/390px and 1440px in
      Vietnamese, light/dark; bank picker stacks above overlay without overflow.
      Previews disable API queries and are not live-account end-to-end acceptance.

- [x] Replace legacy hand-rolled Modal, BottomSheet, SearchableSelect and NavTooltip
      primitives with the Radix-backed shadcn Dialog, Sheet, Popover/Combobox and Tooltip.
- [x] Migrate admin flags, AI usage, moderation, payouts, learners and question-bank
      presentation to the shadcn system; retain API-backed data and actions.
- [x] Align reports, vocabulary, flags, AI usage, moderation, payouts and question bank
      with the content library layout via the shared `AdminSurface` kit.
- [ ] Payout and moderation stat tiles count only the first 50 rows the list endpoint
      returns; a server-side summary (like the content summary) would make them exact.
- [ ] Inspect the aligned admin sections against a live admin account at 390px and
      1440px; this pass was verified by unit tests and typecheck only.
- [ ] Migrate remaining inline `aria-modal` dialogs to shadcn Dialog:
      learning Runner SaveProgressPrompt, account
      ChangePasswordModal/AvatarUploadModal/DevicesList/DataPrivacySettings, studio
      SubmissionStatusModal, resource ResourceDetail, MyWritingPage, MySpeakingPage,
      speaking AudioRecorder, PlacementRunner, exam practice dialog and remaining admin
      review/payout/moderation dialogs. Keep the `html:has([aria-modal])` rule until these
      inline dialogs have been migrated.

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

- [x] Move exam practice options below md into a shadcn `Sheet` (`SheetDialog`
      from `components/ui/sheet.tsx`) from a "Practice options" button; keep
      resume link visible on mobile outside the sheet, preserving md+ inline
      layout.
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
- [x] Add focused accessibility regression coverage for the four admin dialogs'
      accessible names and icon-only close buttons.
- [x] Account settings: learning preferences folded into the profile tab
      (`?section=preferences` aliased), notification/quiet-hours/AI controls
      removed, one grouped "Devices & sign-ins" list, Excel/PDF learning report,
      shell-width layout.
- [ ] `components/layout/MoreSheet.tsx` still lists a separate "Learning
      preferences" row (`section: "preferences"`); it works through the alias but
      duplicates the Profile row — drop it with the next layout change.
- [x] Account settings redesign: section strip/side nav, profile hero +
      completeness, three-card rows, security overview, device cards with ⋮ menu
      and "sign out all other devices".
- [ ] Settings mockup items with no API behind them, left out until the backend
      adds them: two-factor auth, remove avatar, IP/location per session, a
      self "revoke all sessions" endpoint (the UI loops per item), unusual-session
      locking, password strength, interface density.
- [ ] `startGoogleLink` returns to `/settings` (profile), not
      `/settings?section=security`; the redirect lives in `accountApi.ts`.
- [ ] `SessionSummary` has no `trusted_device_id`, so the devices list groups by
      the coarse label; exposing it in the API would let two identical browsers
      show as two devices.
- [ ] Finish internal banner management in its separate API/schema scope.
- [ ] Export the complete screen matrix with an index and label mock/live data.

## Studio editor redesign and UI standardisation (2026-10-08)

- [x] Course editor rebuilt on PageHeader, react-hook-form + zod, immer and a live
      Gate 1 checklist; "Add material" with drag-and-drop, per-kind size caps,
      upload progress and TanStack Query status polling.
- [x] Spreadsheet (XLS/XLSX) and image (PNG/JPEG/WebP) materials in the editor
      and the lesson runner; minimum paid price 5,000 VND.
- [x] Twelve hand-made modals moved to `ui/dialog` / `ui/confirm-dialog`; raw
      select/textarea moved to `ui/native-select` / `ui/textarea`; 10-11 px text
      raised to 12 px; undefined colour tokens mapped to real ones.
- [x] `motion` and `@tanstack/react-table` removed; React and TanStack split into
      vendor chunks (initial download 183 kB gzip).
- [ ] Visually verify the course editor at 375 / 768 / 1440 px in light and dark
      mode: the browser check could not sign in during this session.
- [ ] ExitDialog keeps its bespoke AlertDialog (primary "Keep learning" layout).
- [ ] Remaining per-page container widths (max-w-2xl … 1200px) and hand-written
      h1s outside Studio should move to PageHeader page by page.
- [ ] Off-token amber/emerald/violet tints in gamification widgets and the Studio
      finance cards.

## Vocabulary review and practice hub (2026-10-08)

- [x] Review session, Practice hub and My words on PageHeader and the shell width
      (`max-w-[1440px]`); flashcard capped at `max-w-2xl` and centred.
- [x] Session progress (card n of total, cards left, graded-count bar), Show answer
      button carrying the word, Space/Enter reveal, 1-4 grade (hints md+ only,
      modifiers and dialogs ignored), pressed-grade spinner, last-grade status line
      from `next_due_at`, focus to the answer on reveal and to the next card after a
      grade, inert turned-away face, sticky grade bar above the mobile bottom nav.
- [x] Summary with grade breakdown and next steps; empty queue offers lessons and
      My words.
- [ ] Visually verify /practice, /practice/review and /practice/my-words at 375 /
      1280 / 1920 px in light and dark mode.
- [ ] `ReviewSessionResponse.total_due` is never set by the backend, so the summary
      cannot say how many cards are still due after a capped session.

## Dashboard / Practice / Progress redesign (2026-10-08)

- [x] Dashboard: greeting + streak/XP tiles, hero with stacked-card art, Review & learning
      path card (tabs md+, stacked on a phone), weekly plan beside CEFR skills (xl),
      one skill per row on every width.
- [x] Practice: hero set card, spaced-repetition card with art, forecast bars, practice
      type cards with meta line and "See all".
- [x] Progress: PageHeader, four KPI tiles (study time this week from the weekly plan).
- [ ] Backend needed before the design's remaining pieces can be drawn: all-time study
      time and week-over-week deltas on `/me/progress`, a range parameter ("last 7
      days"), per-day completed-exercise counts for the progress-over-time chart,
      per-topic progress on `/me/foundation/next` (only a mastery `score` exists).
- [x] `e2e/responsive/dashboard-layout.spec.ts` now drives the single "Show N more"
      expander at every width; it also caught a 768px overflow (greeting and goal
      tiles side by side beside the sidebar), fixed by keeping them stacked below `xl`.
