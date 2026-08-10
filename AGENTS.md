# Working in this repo

For any agent or harness working here — this is the one brief, and it
points at authorities rather than copying them, so nothing in it can
drift from what the gates enforce. Harness-specific files (CLAUDE.md
and any equivalent) are pointers to this file, never second copies.

## What this is

A contract-set validator for agent-driven development. The permissible
shapes of a project's law are declared in `schema/*.cue`; a Go gate embeds
that law at build time and refuses anything it does not admit. The Go
side holds no policy of its own.

## Before writing product-facing text

Every refusal string, remedy, comment, usage line, and schema doc is
governed by the ratified lexicon. **Read the digest first:**

    go run ./dev/cmd/digest

18KB, generated from `dev/*.cue` — this project's design basis, not the
schemas the binary embeds: invariants, authorities and acts, a pasteable
card per reserved term, and the vocabulary that is refused.
`dev/DIGEST.md` is the committed copy. Open the full text in `dev/*.cue`
only when exact wording is at stake — the corpus is 91KB and re-reading
it to check one sentence is the expensive habit this digest exists to
retire.

The traps that catch newcomers: `commit`, `merge`, `push`, `build`,
`deploy`, `release`, `publish`, `ship`, `rollback` are banned bare;
`authorize`, `permit`, `allow`, `approve` are banned outright; `version`
and `environment` need qualification. A denial is not a failure. Claims
are recorded, never believed. The outright bans are enforced by
`go test ./internal/conformance/` over everything the product says, so a
slip fails rather than waiting for review. That check lives with the
product because it reads the product: filed under `dev/`, anything
filtering on paths would skip it exactly when a product edit is what
introduced the violation.

The reverse direction matters more and is also enforced: workshop
concepts must not borrow the product's words, and nothing under `dev/`
may be named after a product concept. Every conflation this project has
had ran that way — a script called `dev/law`, a ratification ritual
copied from the product's act.

## First run on a machine

    dev/serena-setup     # if you use Serena: copies the tracked config
                         # into the central location Serena reads

The tracked config (`.serena/project.yml`) turns on richer Go
diagnostics, skips recorded outputs when searching, and points a new
session at the digest rather than the corpus; `dev/serena-memories/`
holds the project memories, kept as pointers to this file rather than a
second copy of it.

## Verifying

    dev/check                                       # CI's fast lane, same order
    dev/basis                                       # regenerate dev/DIGEST.md
    go test -race ./...                             # before pushing
    LOOPLAW_GOLDEN_UPDATE=1 go test ./... -count=1  # re-record goldens deliberately

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
  declared exemption fails the demonstration-coverage test.
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

Two properties, and everything else about the split follows from them:

- **dev-lane is not polluted by prod-lane.** Product concepts never
  become workshop things — no workshop artifact named after one, and
  the two vocabularies never intersect. Every conflation this project
  has had ran this way.
- **prod-lane is not tied to dev-lane.** The product builds and runs
  with `dev/` deleted; CI proves it on every push by archiving the tree,
  removing the workshop, and validating a set with what is left.

Checks that read what the *product* says live with the product
(`internal/conformance`), so a product edit cannot slip past them.
Checks that read only `dev/` live in `dev/`.

Kernel code (`internal/gate`, `internal/provenance`, `internal/store`)
never reads a work tree and never infers: it decides over submitted
claims, manifests, and recorded state. Client code (`internal/absorb`)
may read a scope it was handed. Derivation — what law a scope implies —
is inference and belongs to the agent driving the tool, not the binary.

## The design basis is locked

`dev/*.cue` — the vocabulary, the registry, the Tier 0 invariants — is
sealed. `dev/LOCKED` holds each file's hash; `dev/lock` compares them and
runs first in both `dev/check` and CI. A harness may propose a change on
a branch; master takes pull requests only, and only xormania merges.

**If you are reaching for a word: do not coin one, and do not ask for
one.** Write the plain sentence. "The golden is out of date" needs no
term. Nearly every defect this project has had was a coinage that could
have been a sentence — `aa` for accountable authority, `actor` for party,
`drift` reserved as ours while the spec used it for what the provenance
check reports. Raise a term only once it has recurred across batches, in
a pull request body, batched with real work. The lexicon is meant to be
static; changing it is dev-lane work, and dev-lane work is rare.

The lock is tamper-evident, not tamper-proof, and that is the design
rather than a shortfall. Nothing inside a repository stops an agent with
write access — file permissions least of all, since `git checkout`
overwrites a read-only tracked file and resets its mode, and committed
content can be changed through plumbing without the file on disk ever
being written. So the goal is not prevention but that no change is
silent: turning the check green again means updating `dev/LOCKED`, which
puts a line in the diff saying the basis was unlocked here. CI is what
catches the plumbing path, because it hashes a tree checked out from the
commit.

When xormania has consented: `dev/lock --seal`, and commit `dev/LOCKED`
with the change it covers.

## Contributing

`CONTRIBUTING.md` is authoritative: branch per theme, one unit of work
per commit, batch a coherent theme into one draft PR, xormania-only
attribution with no AI trailers, and only xormania marks ready or
merges. Merging ratifies nothing — ratification is a recorded act in the
ledger, which is the thing this product exists to replace merging with.

Working notes live in `proj/` (gitignored — scope, not secrecy).

Clean up after yourself: scratch programs, probe tests, and worktrees
you create for an investigation do not belong in the repository. Write
them under a temp directory; `dev/check` refuses a tree with stray
scratch files, because more than one harness has left them behind.
