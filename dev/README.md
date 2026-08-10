# dev/ — this project's design basis, and how it is ratified

Dev-lane: everything here governs how looplaw is built. What the product
enforces is `law/` — the schemas the binary embeds and applies to other
people's law.

Dev-lane is a subject, not a lower standing. The registry, invariants and
vocabulary here are ratified and amended deliberately. Plain CUE, no
private dialect; working notes stay in `proj/` (gitignored).

**Lane declaration.** These files model *the product* — "the accountable
authority" is a per-deployment role, not a person — while being *dev-lane
artifacts of this project*, whose accountable authority is xormania. No
workshop facts inside the models.

**The protocol below is a workshop practice for these files only.** It is
not how the product works and never was: in looplaw, ratification is an
act recorded in the ledger, and git has no bearing on standing. Merges
serve here because no looplaw instance governs this project yet — a
convenience, not a model.

## How these files are ratified (workshop practice)

Proposals may come from anywhere — authority to ratify may not.

1. An agent (or anyone) opens a **draft PR** changing `law/`: new entries
   arrive with `status: "proposed"` and a rationale; the PR body carries
   the per-decision recommendation.
2. The accountable authority reviews the diff. Inline comments are
   **corrections** — the proposer revises. "Request changes" is a denial —
   recorded, successful, not a failure.
3. **A merge by xormania settles a proposal here.** The following batch
   flips `status: "proposed"` → `"ratified"` and cites the merge. This
   is bookkeeping for a repository, not a model of the product's
   ratify act: nothing about a merge confers standing in looplaw, and
   when an instance governs this project the ledger takes over
   entirely.
4. Ratified entries change only by a new PR (amendment). Nothing edits
   ratified law in place; predecessors live in history, never deleted.
5. Entries provisionally ruled under the reversibility rule carry a
   `trigger:` — the recorded event that reopens them.

Machine check: `cue vet ./dev/` and `cue vet ./law/` both pass, and CI
runs them.
