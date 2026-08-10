# dev/ — this project's design basis, and how it is ratified

Dev-lane: everything here governs how looplaw is built. What the product
enforces is `schema/` — the schemas the binary embeds and applies to other
projects' law.

Dev-lane is a subject, not a lower standing. The registry, invariants and
vocabulary here are ratified and amended deliberately. Plain CUE, no
private dialect; working notes stay in `proj/` (gitignored).

**Lane declaration.** These files model *the product* — "the accountable
authority" is a per-deployment role, not a person — while being *dev-lane
artifacts of this project*, whose accountable authority is xormania. No
workshop facts inside the models.

**How these files change:** propose in a draft pull request, xormania
merges, done. Nothing is marked proposed and later flipped — something
merged here is settled, and anything genuinely provisional says so in
its own `trigger`.

That is deliberately all of it. An earlier version of this file
imitated the product's ratification act — statuses, flips, citations of
the merge that settled them — and the imitation modelled a mechanism
the product does not use: in looplaw, ratification is an act recorded
in the ledger, and git has no bearing on standing. The ceremony caught
nothing and caused two missed steps, so it is gone.
