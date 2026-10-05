# Fluentra UI reference audit

## Latest checkpoint — 2026-10-04

The latest user prompt scopes active work to Dashboard refinement, not an
all-page redesign. References guide the design rather than pixel-perfect
acceptance. Earlier observations below describe their original check date.

- Supporting cards/empty states are compact; skills share the desktop plan
  aside. Empty Foundation/Placement wrappers no longer leave blank spacing.
- The hero opens the actual recommended lesson and uses matching course
  activity progress. XP tiles show earned XP within the current level and XP
  remaining. Streak freeze remains actionable without a duplicate streak card;
  league opt-in remains reachable through the standings disclosure.
- Weekly plans show five initial items and retain every lesson through an
  accessible View all/Show less button.
- Current broader regression run: 141/141 passed across eight suites (Dashboard,
  gamification, account-settings, shell/status, i18n keys, Foundation, contrast
  and tokens). A separate 39/39 run also covered Studio submission and auth.
- Production compilation and bundle gate passed: 199.4 KB gzip initial download
  against 200 KB. Auth pages are lazy-loaded and the root imports auth API
  directly rather than evaluating all form exports.
- Frontend lint passed with zero errors and three existing React Compiler
  library warnings. The E2E fixture type-check and `git diff --check` passed.
  The earlier Studio test handler's `Array.isArray` narrowing-to-`any` problem
  was corrected using its explicit string route-param type.
- `make check` completed formatting and Go vet, then failed on 11 Go lint
  findings under `internal/modules/studio/` (constant extraction, complexity,
  line length and staticcheck). These backend files are outside Dashboard
  refinement; no business-logic changes were made to fix them.
- Both localhost and IPv4 loopback port 5173 refused connection during the
  current check. No new server was started. Rendered Dashboard visual acceptance
  and the 20–30% height target remain unverified. The fixture-only capture matrix
  and no-server review config are ready; see `dashboard-refinement/README.md`.

## Reference and scope

The two user-supplied images are the visual reference, not just inspiration.
Keep the royal-blue learner interface, compact cards, light-blue backgrounds,
navy dark theme, and five-item mobile navigation. The user chose no character
illustrations: reserve the visual space for restrained book/icon artwork instead.

## Verified observations

- The live guest `/learn` screen was inspected in Chromium at 1440×900. Its
  document width was 1440px and its updated sidebar measured 208px.
- The inspector browser is signed out. Authenticated dashboard and personal
  learning data have **not** been visually verified in that session.
- `/learn` currently expands the first course into a long syllabus by default.
  This is unlike the reference's compact course overview. Learners also have
  no visible catalogue selector for the other courses.
- `LearnPage` passes `completedLessons={0}` to `CourseHeader` even for signed-in
  learners. A visual redesign must not present that placeholder as real progress.
- Dashboard originally placed gamification and daily practice before the
  continue-learning hero, with the weekly plan stacked below. The current
  layout edit puts the hero first and the weekly plan in a desktop aside;
  its authenticated appearance still requires visual verification.
- The original gamification section used four large widgets in two stacked
  grids. The reference requires four compact summary tiles (two columns on
  mobile), with detailed missions and leaderboard secondary.
- The original continue-learning action returned to `/learn` rather than
  opening the supplied next lesson. This needs a direct next-lesson link.

## Functional problems found by actual checks

- Type-check initially failed on five `layoutId` props, progress-effect return
  paths, and three optional submit mutation calls in tests. These were fixed.
- The More sheet did not render in either Vitest or live Chromium: the portal
  was a child of `AnimatePresence` and was filtered out. Moving the portal
  outside the presence wrapper fixed the actual rendering problem.
- After these fixes, `pnpm exec tsc -b` succeeded and the four focused suites
  (`app-shell-phase1`, `studio-submit-draft`, `i18n-keys`, `primitives`) passed
  all 24 tests. This is functional evidence, **not visual approval**.
- Chromium at 390px verified the More dialog and a 390px document width.
  The initial screenshot was taken during its entrance animation; it must
  not be used as a final visual comparison.

## Remaining acceptance work

1. Finish hero artwork, metadata, CTA and compact statistic tiles.
2. Compare the rendered signed-in dashboard at desktop/mobile, light/dark,
   including genuine empty/loading/error states. Label fixtures explicitly if
   used instead of a live authenticated account.
3. Redesign the course overview without losing direct syllabus links, enrolment,
   purchases or guest access; show progress only from a real progress source.
4. Compare practice, progress and settings against their reference panels.
5. Run build/bundle checks, lint, broader regression tests and `make check`.
6. Obtain independent review of the actual changes; the current visual edits
   are not independently approved.
7. Implement the previously requested internal banner management and complete
   screen-export folder in their subsequent feature scopes. They are not done.
