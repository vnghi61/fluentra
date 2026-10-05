# Dashboard refinement capture index

**Captured and inspected on 2026-10-05.** Playwright's two locale matrix tests
passed. The Vietnamese mobile light, tablet light and desktop dark screenshots
were inspected directly. These are authenticated geometry fixtures, not live
learner statistics.

The layout spec uses generated-schema fixtures, not real account data. Its
captures measure layout only; they do not establish backend/auth correctness.

Once the existing app is reachable, from `web/` run:

```powershell
pnpm exec playwright test --config=playwright.ui-review.config.ts
```

The review configuration has `webServer: []`: it never starts another server.
`E2E_BASE_URL` may point it at the user's existing app instead of port 5173.

## Capture files

Each locale (`en`, `vi`) × theme (`light`, `dark`) × width (`320`, `390`,
`768`, `1440`) produces `<locale>-<theme>-<width>.png`. Full-page PNGs wait for fonts
and data and disable animations. Mobile starts with the weekly card collapsed;
tablet/desktop show up to five lessons before expansion.
`<locale>-measurements.json` records viewport, document width and main height.

The spec asserts no document overflow, a genuine matching course-progress
fixture, the direct next-lesson destination, full-width weekly card geometry and access
to the seventh weekly lesson after expansion. It does not exercise mutations
against fixtures. No same-data pre-redesign baseline was captured, so a numerical
20–30% page-shortening claim is not supported.

## Final verification (2026-10-05)

- `make check`: exit 0 (format, vet, lint, OpenAPI lint, architecture checks and Go unit tests).
- Frontend format, typecheck, lint and build: exit 0; 62 Vitest files / 535 tests pass.
- Initial download: 200.3 kB gzip against the user-approved 500 kB cap.
- PostgreSQL integration: Studio repository, learning service and seed pass.
- `playwright.ui-review.config.ts`: both EN/VI geometry matrix tests pass.
- Three existing React Compiler compatibility warnings remain non-blocking.
- Full real-account end-to-end journeys were not run in this verification pass.
