# Working in this repo

For any agent or harness working here — this is the one brief, and it
points at authorities rather than copying them, so nothing in it can
drift from what the gates enforce. Harness-specific files (CLAUDE.md
and any equivalent) are pointers to this file, never second copies.

## What this is

A contract-set validator for agent-driven development. The permissible
shapes of a project's law are declared in `law/*.cue`; a Go gate embeds
that law at build time and refuses anything it does not admit. The Go
side holds no policy of its own.

## Before writing product-facing text

Every refusal string, remedy, comment, usage line, and schema doc is
governed by the ratified lexicon. **Read the digest first:**

    go run ./cmd/looplaw project law

18KB, generated from the law the binary carries: invariants, authorities
and acts, a pasteable card per reserved term, and the vocabulary that is
refused. `law/DIGEST.md` is the committed copy. Open the full text in
`law/*.cue` only when exact wording is at stake — the corpus is 91KB and
re-reading it to check one sentence is the expensive habit this digest
exists to retire.

The traps that catch newcomers: `commit`, `merge`, `push`, `build`,
`deploy`, `release`, `publish`, `ship`, `rollback` are banned bare;
`authorize`, `permit`, `allow`, `approve` are banned outright; `version`
and `environment` need qualification. A denial is not a failure. Claims
are recorded, never believed.

## First run on a machine

    dev/serena-setup     # if you use Serena: copies the tracked config
                         # into the central location Serena reads

The tracked config (`.serena/project.yml`) turns on richer Go
diagnostics, skips recorded outputs when searching, and points a new
session at the digest rather than the corpus; `dev/serena-memories/`
holds the project memories, kept as pointers to this file rather than a
second copy of it.

## Verifying

    dev/check                                  # CI's fast lane, same order
    dev/law                                    # regenerate the digest, report ratification debt
    go test -race ./...                        # before pushing
    LOOPLAW_GOLDEN_UPDATE=1 go test ./...      # re-record goldens deliberately

`dev/check` is the one to run: green there means green in CI's fast
lane. CI runs that lane and a race/differential lane in parallel;
fuzzing runs nightly, not per push.

Before asking for a merge on anything that changes behavior, run an
adversarial review — every blocking defect this repo has found came
from one, not from its own tests. The protocol, and why each lens
exists, is `dev/review.md`; it is written to be followed by hand or
driven by whatever automation a harness has.

## Test discipline (non-negotiable, and enforced by the suite)

- **Every check has a proving red.** `gate.Checks` and `diff.Checks`
  enumerate what each package can emit; a check with no red and no
  declared exemption fails the closure test.
- **A red for the wrong reason proves nothing.** Mutations assert the
  refusal names the mutated thing, and any additional check a mutation
  draws must be declared in `alsoDraws`.
- **Attacks are kept, never deleted.** `internal/gate/testdata/attacks/`
  holds every adversarial set that has found a defect, with `index.cue`
  declaring the refusal each must draw. Add to it rather than writing a
  one-off red; check it before inventing an attack that may already be
  there.
- **Goldens are contracts.** The gap feed, staleness report, skeleton,
  and refusal streams are what consumers script against. A golden
  mismatch is a shape change asking to be noticed — re-record
  deliberately and say why in the message.
- **Refusal order is part of the contract.** Walk maps through
  `sortedKeys`; a nondeterministic stream has bitten this repo twice.

## The lanes

Kernel code (`internal/gate`, `internal/provenance`, `internal/store`)
never reads a work tree and never infers: it decides over submitted
claims, manifests, and recorded state. Client code (`internal/absorb`)
may read a scope it was handed. Derivation — what law a scope implies —
is inference and belongs to the agent driving the tool, not the binary.

## Contributing

`CONTRIBUTING.md` is authoritative: branch per theme, one unit of work
per commit, batch a coherent theme into one draft PR, xormania-only
attribution with no AI trailers, and only xormania marks ready or
merges. For `law/` changes the merge *is* the ratification act —
entries arrive `status: "proposed"` and flip in the following batch
(`law/README.md`).

Working notes live in `proj/` (gitignored — scope, not secrecy).

Clean up after yourself: scratch programs, probe tests, and worktrees
you create for an investigation do not belong in the repository. Write
them under a temp directory; `dev/check` refuses a tree with stray
scratch files, because more than one harness has left them behind.
