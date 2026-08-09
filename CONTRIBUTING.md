# Contributing

Working rules for this repository. Short on ceremony, long on evidence:
records exist so that someday, someone (human or agent) can reconstruct what
happened and why without asking anyone.

## Workflow

- Never commit to `master` directly.
- Branch per unit of work: `feat/<short-name>` or `fix/<short-name>`.
- Commit to the branch, push, open a **draft** PR to `master`.
- Only xormania marks a PR ready and merges. Agents open drafts.
- One unit of work per **commit** — small and auditable beats big and
  heroic.
- PRs are **batches, not single commits**: open a draft early, keep
  pushing units to it, and present it when a coherent theme's worth is
  ready. Merge cadence is the accountable authority's attention budget —
  spend it on batches reviewable in one sitting, not on a PR per commit.

## Commit messages

- Subject: imperative mood, capitalized, ≤72 chars, no trailing period.
  `Initialize Go module and CLI skeleton` — not `initialized…` or `misc`.
- Body (for anything non-trivial): what changed and **why** — the intent,
  not a diff narration. Name the design basis when one exists (e.g.
  `proj/looplaw-spec.md §10`, a PR, an issue). Call out behavior changes:
  new/changed commands, flags, exit codes, formats.
- Attribution: the commit author is the human account (`xormania`). No
  `Co-Authored-By` trailers, no tool signatures.

## PR titles

Same style as a commit subject: one imperative line naming the change. If
the PR has one commit, reuse its subject.

## PR bodies

Include, briefly (a sentence or three each — this is the "useful someday"
record):

- **What** — the change, in plain words.
- **Why** — motivation and design basis: link the spec/doc section, ruling,
  or issue that made this the thing to do.
- **Verification** — how it was checked: build, vet, tests, a manual run
  with its output. Claimed-but-unverified is worth saying explicitly too.
- **Notes** — anything a future reader needs: deferrals, known gaps,
  follow-ups, decisions punted and to whom.

Nothing fancy. The test: a year from now, `git log` plus the PR trail should
answer "what is this, why is it here, and how did we know it worked" without
archaeology.
