# law/ — ratifiable artifacts and the ratification protocol

This directory holds looplaw's **product law**: the design-basis artifacts
that get ratified, versioned, and — once looplaw runs — migrated into its
own store. Plain CUE, no private dialect. Prose scraps and working notes
stay in `proj/` (gitignored); what lives here is deliverable-grade.

**Lane declaration.** These files model *the product* (any deployment;
"the accountable authority" is a per-deployment role). The files are
themselves *dev-lane artifacts of the looplaw project*, whose accountable
authority is xormania. No workshop facts inside the models.

## The ratification protocol (git era)

Proposals may come from anywhere — authority to ratify may not.

1. An agent (or anyone) opens a **draft PR** changing `law/`: new entries
   arrive with `status: "proposed"` and a rationale; the PR body carries
   the per-decision recommendation.
2. The accountable authority reviews the diff. Inline comments are
   **corrections** — the proposer revises. "Request changes" is a denial —
   recorded, successful, not a failure.
3. **Merge by the accountable authority is the recorded act of
   ratification.** On merge, a follow-up commit in the same PR (or the
   next batch) flips `status: "proposed"` → `"ratified"`. Git history is
   the amendment ledger until looplaw's own store assumes custody.
4. Ratified entries change only by a new PR (amendment). Nothing edits
   ratified law in place; predecessors live in history, never deleted.
5. Entries provisionally ruled under the reversibility rule carry a
   `trigger:` — the recorded event that reopens them.

Machine check: `cue vet` over this directory must pass; CI will enforce it
once CI exists.
