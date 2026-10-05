# Fluentra product UI redesign — source-adapted plan

## Latest steering: Dashboard refinement only

The latest prompt narrows the active work to the existing Dashboard. Preserve
navigation, routes, API contracts and learner actions; compact empty states,
remove repeated statistics and use the right column for weekly plan and skills.
The 20–30% page-height reduction is a measurement target, not a verified result.
The broader sequence below is historical follow-on scope, not active acceptance
for this refinement.

The user's latest design prompt is authoritative. Screenshots provide visual
direction, not pixel-perfect acceptance. Reuse the existing React 19 SPA,
Tailwind 4 semantic variables, TanStack Router/Query, Zustand preferences,
Radix dropdown and Motion. Do not rebuild the app or change backend contracts.

## Source-specific decisions

- Studio is creator course authoring/review, not AI conversation. Keep its
  description accurate and preserve the recently repaired submit-ID flow.
- Only Vietnamese and English are supported. Do not offer Japanese/Korean
  until complete translations exist.
- No mascot/character artwork (the user's earlier selection). Use a restrained
  book/icon illustration system without a new 3D runtime.
- Never invent CEFR levels, completed lessons, study time, XP, streak history,
  missions or weekly-plan entries to match example screenshots.
- Permission-filtered administration navigation stays owned by the admin
  feature; the shared shell must not import feature internals.
- Header search/notifications must connect to real functionality before being
  shown. The current notification slot has no feature wired into it; do not
  draw a fake badge or unread count.
- Retain profile/avatar, learning profile, preferences, security/devices and
  privacy/export/deletion settings. Group them instead of deleting existing
  capabilities to reproduce the shorter concept menu.
- Guest course access, slug deep links, purchases, enrolment and lesson
  prerequisites must continue working.

## Incremental delivery

1. Shell/design system: 232px desktop sidebar, five mobile destinations, compact
   authenticated mobile header, theme controls reachable through settings,
   accessible 24px-radius More sheet; three semantic surface levels, 16px card
   radius and restrained light/dark shadows.
2. Dashboard: primary next-lesson hero, compact four statistics, foundation
   next step, useful weekly-plan aside, compact missions/practice. Keep ranking
   details secondary and preserve opt-in/streak-freeze actions.
3. Learning surfaces: catalogue selection and truthful progress, compact daily
   practice and review entry, honest skills/course progress, grouped settings
   with reusable loading/error states. Cover long Vietnamese labels and empty
   states without oversized blank cards.
4. Interaction checks: focus return, Escape/backdrop closure, touch drag,
   reduced motion, completed/failed/loading feedback. Avoid continuous
   decoration and false hover affordances on static cards.
5. Evidence: formatter, lint, type-check, frontend tests, build/bundle budget,
   `make check`, independent review and visual captures in both locales,
   light/dark and desktop/mobile. Label fixture-based captures and blocked
   signed-in checks explicitly.
6. Follow-on scopes already requested: internal banner management (its own
   API/schema-first backend work) and a complete screen-export index for
   other reviewers. Neither is implied completed by the UI finishing work.

## Current acceptance status

In progress. Prior passing tests covered shell/status/Studio fixes, not the
entire redesigned interface. Do not describe the product redesign as finished
until visual and independent review evidence exists.
