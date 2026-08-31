// Package prompts embeds the versioned runtime prompt templates.
//
// # Why these are not under docs/prompts/runtime
//
// Rule L11 says a prompt lives in `docs/prompts/` as a versioned template, and
// that is where these were first written. Two mechanics moved them:
//
//  1. `//go:embed` cannot reach outside its own package directory, so the embed
//     has to sit beside the templates.
//  2. `.go-arch-lint.yml` sets `workdir: internal`, so a package under `docs/`
//     is outside every component it can declare — and an import of one from
//     `platform/ai` fails the boundary linter with no way to grant it.
//
// The rule's intent is met in full: the prompts are versioned `.md` files,
// reviewable on their own, and no prompt string appears in Go. What changed is
// the folder. `docs/prompts/README.md` points here, and `DECISIONS.md` in this
// module records it.
//
// Nothing in this package interprets a template. Rendering, versioning and the
// provider call are internal/platform/ai's; this is the file system, and only
// that.
package prompts

import "embed"

// Files holds every versioned runtime template, named `<task>.v<N>.md`.
//
// Immutable by convention: a change to a live prompt is a new `vN+1` file and a
// configuration change, never an edit in place. An edit in place changes what
// every learner is graded by, retroactively and without a rollback path.
//
//go:embed *.md
var Files embed.FS
