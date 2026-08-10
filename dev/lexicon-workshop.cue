// The workshop's own words: what we call things while building
// looplaw, as opposed to the vocabulary the product uses
// (dev/lexicon-product.cue) or the schema the binary enforces (law/).
//
// Ratified like anything else here — dev-lane names a subject, not a
// lower standing. It exists because borrowing the product's words for
// workshop talk, or leaking ours into its output, is a conflation that
// has already cost us: "actor" was coined here for something already
// called a party, and reached the ledger schema before anyone noticed.
//
// Deliberately short. This vocabulary is read by one person who
// self-corrects; the full collision-resistant anatomy would cost more
// than the drift it prevents.
package dev

// The standing of a dev-lane artifact, and the tiers this project's
// vocabulary uses. Named here rather than shared with law/: the
// product's #Status describes a target set's entries, and a lane
// boundary is not a place to share a definition across.
#Status: "proposed" | "ratified" | "corrected" | "withdrawn"
#Tier:   "CANON" | "REVIEW" | "QUALIFY" | "BANNED"

// Words that mean something specific to us and must never appear in
// anything the product says. dev/lexicon_test.go enforces exactly
// this — the only mechanical rule here, and the only one worth having.
reserved_dev: {
	"proving red": "A test that must fail, and fail naming the thing that was broken. A red for the wrong reason proves nothing."
	witness:       "A test demonstrating that a behavior holds. A proving red is the witness for a refusal; a green fixture is the witness for an acceptance."
	cascade:       "An additional check a single-edit mutation draws beyond its declared one. Declared in alsoDraws, never silent."
	golden:        "A recorded output compared byte for byte. A mismatch is a deliberate shape change asking to be noticed."
	corpus:        "A kept collection of fixtures. The attack corpus holds every adversarial set that has ever found a defect."
	fixture:       "An authored input a test runs against. Fixture zero is the canonical green set."
}

// Words both lanes use, in different senses. Not tested — a test would
// fire on ratified law itself. Listed so the double meaning is
// deliberate rather than accidental.
shared: {
	lane: "dev: which side of the workshop boundary (dev-lane, prod-lane), or which half of the kernel (the shape lane, the relational lane). law: the same kernel sense, in code comments only."
	batch: "dev: one pull request's worth of work. law: a numbered round of ratification, referenced in law file headers."
	closure: "dev: the test proving every check has a proving red. law: the contract method's coverage check over acts and parties."
	drift: "dev: a fixture or golden falling out of step with the code. law: a divergence between artifacts, filed rather than silently resolved."
	mutation: "dev: a single edit to a green fixture that must draw a declared refusal. law: unused."
	attack: "dev: a preserved adversarial set. law: unused."
}

// Product terms we do not borrow for workshop concepts, because law
// gives each a precise meaning and reusing it blurs exactly the
// distinction that matters. Documented, not tested: judging misuse
// needs a reader, and one reader is what this project has.
do_not_borrow: [
	"claim, receipt, admission, version — record kinds; a test double is a fixture, not a claim",
	"party — a submitter in the product's model; we are the author, not a party",
	"gap — the differ's unit; work we have not done is a to-do, not a gap",
	"ratify, admit, record — acts with named authorities; merging a pull request ratifies law, and nothing else here does",
	"verify — a read path over recorded facts; running the suite is checking, not verifying",
]
