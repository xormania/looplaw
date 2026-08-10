# looplaw — core

**Read `AGENTS.md` in the repository root first.** It is the brief, it is
harness-neutral, and it is the only copy — this memory deliberately does
not restate it, because two briefs drift and the one you happen to read
would then be the wrong one.

Pointers that save the most:
- Vocabulary, authority, what the law says → `dev/DIGEST.md` (generated,
  18KB), not `dev/*.cue`. Full text only when exact wording matters.
- Before pushing → `dev/check`. After changing law → `dev/basis`.
- Before a merge request → `dev/review.md`.
- Before inventing an adversarial input → `internal/gate/testdata/attacks/`.

## Package map

- `schema/` — ratified law as CUE: registry (authorities, acts), tier0
  (invariants), lexicon (reserved terms, refused vocabulary), trinity
  (the set schema, interiors, provenance), gap.
- `internal/gate` — shape gate over embedded law + the relational lane;
  `Checks` and `SubmissionChecks` enumerate everything it can emit.
- `internal/diff` — goal-law against a view, producing the planning feed.
- `internal/absorb` (client: reads a scope) and `internal/provenance`
  (kernel: pure comparison, no IO) — the lane split made visible.
- `internal/record` — the record act; `internal/store` — the ledger.
- `internal/golden` — recorded outputs; `internal/conformance` — the
  checks that read product text, filed with the product they check.
- `dev/` — dev-lane only: the workshop lexicon, scripts, review protocol.

## What not to re-litigate

Ratified and settled: law descends and only evidence ascends; the kernel
neither infers nor reads work trees; two authorities exist; denial is not
failure; a gap is a planning state. Check `dev/DIGEST.md` and
`proj/looplaw-spec.md` before raising a design question — most are already
ruled.
