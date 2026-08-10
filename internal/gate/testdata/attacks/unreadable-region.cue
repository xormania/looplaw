// ATTACK FIXTURE — a set that MUST be refused.
//
// Preserved from an adversarial review round rather than discarded:
// every blocking defect this project has found came from a hand-built
// attack, and each one is kept here so the defect can never return
// silently and so the next reviewer starts from what has already been
// tried. Expected refusals are declared in index.cue beside this file.
//
// Round 1 (gate): a scalar where a region belongs made every relational check in that region no-op silently; it is now a first-class finding.
subject: "bad-sys"
schema_version: "0"
experience_declared_absent: true
contracts: 42
