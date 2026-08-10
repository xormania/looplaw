# Adversarial review

Run one before asking for a merge on anything that changes behavior.
Not because the tests are weak — every check here has a proving red —
but because of what the record shows: **every blocking defect this
repository has found came from a hand-built adversarial attempt, and
none from its own suite.** The suite catches regressions. Reviews find
defects.

Written to be followed by hand. A harness with orchestration can drive
it (a Claude session has it saved as `Workflow({name: "review"})`);
nothing here depends on that.

Match depth to risk. A batch that changes no product behavior does not
need three adversarial lenses, and reviewing it anyway costs more than
it finds.

## Shape

Two passes, and the second is what makes the first trustworthy.

1. **Find.** Independent lenses over the batch, each blind to the
   others' output. One reader looking for everything finds less than
   three readers each looking for one thing.
2. **Refute.** A separate skeptic per finding, told to *disprove* it:
   reproduce the failure or show it cannot happen. A finding that
   survives an attempt to kill it is worth acting on; one that does not
   would have cost a fix and taught nothing.

A finding without a command, an observed output, and a failure scenario
is not a finding. A red for the wrong reason proves nothing — check
that what you reproduced is what the finding actually claims.

Keep the writing short, and not for tidiness: a finding that does not
survive refutation is discarded whole, so every sentence spent
justifying one before it is checked is a sentence thrown away. A note
is one line. Give each lens the list of changed files rather than the
repository, or it will read the repository to find out which files
changed.

## The three lenses

Each catches a class the others miss. Use all three unless the batch
plainly has no surface for one.

**Correctness.** Attack by building inputs and running the binary, not
by reading for style. Classes that have actually bitten this repository,
in the order they have appeared:

- silent skips — a check that no-ops when a lookup errors, so malformed
  input passes unexamined
- hash and delimiter forgery — two materially different things
  comparing equal
- values that satisfy the schema and mean nothing: empty strings,
  absent regions, self-references
- nondeterminism reaching anything a consumer reads, refusal order
  included
- inputs the schema permits that no check examines at all

**Law conformance.** Read `dev/DIGEST.md` first — the ratified law as a
brief. Audit every user-facing string the batch adds: refused
vocabulary, authority relocation (a reserved verb handed to a subject
that does not hold it), status laundering (evidence treated as law, a
claim treated as believed, processing treated as a standing change),
and any behavior contradicting a ratified statement.

**Lane discipline.** The kernel performs no inference and never fetches
or inspects work-product content; gates are mechanism, never authority;
only the store records; law descends and only evidence ascends. Look
for kernel paths that read a tree, client output implying standing it
cannot confer, dev-lane conveniences shipped in product code, and
unratified artifacts treated as law.

## After

- **Keep the attacks.** Anything that found a defect goes into
  `internal/gate/testdata/attacks/` with its expected refusal declared
  in `index.cue`. The corpus is why a defect cannot return quietly, and
  why the next reviewer starts from what has already been tried.
- **Fix with a proving red.** A fix without a demonstration is a fix that can
  be undone silently.
- **Record what was rejected.** A considered-and-rejected finding, and
  why, belongs in the pull request — unrecorded rejections get
  re-litigated by whoever next has the same idea.
- **Clean up.** Scratch programs, probe tests, and worktrees created
  during a review do not belong in the repository. `dev/check` refuses
  a tree with stray scratch files, because more than one harness has
  left them behind.
