// The workshop's own words: the names this project uses while building
// looplaw, as opposed to the vocabulary the product uses
// (dev/lexicon-product.cue) or the schema the binary enforces (schema/).
//
// Ratified like anything else here — dev-lane names a subject, not a
// lower standing. It exists because borrowing the product's words for
// workshop concepts, or leaking workshop words into the product's
// output, is a conflation that has already cost this project: "actor"
// was coined here for something already called a party, and reached the
// ledger schema before anything caught it.
//
// Deliberately short. This vocabulary has one reader who self-corrects;
// the full collision-resistant anatomy would cost more than it prevents.
package dev

// The tiers this project's vocabulary uses. Dev-lane entries carry no
// standing marker: something merged here is settled, and anything
// genuinely provisional says so in its own trigger.
#Tier: "CANON" | "REVIEW" | "QUALIFY" | "BANNED"

// Words taken from the specification-CI method rather than coined here,
// used in that method's sense. Listed separately because an inherited
// term is not one this project may redefine: changing one means
// disagreeing with the method, a different and larger act than naming a
// new workshop thing.
inherited_dev: {
	corpus:   "A kept collection of fixtures, frozen with its expected verdict. The attack corpus holds every adversarial set that has ever found a defect."
	fixture:  "An authored input a test runs against — input, expected failure, expected pass. Fixture zero is the canonical green set."
	mutation: "A single edit to a green fixture that must draw a declared refusal."
}

// Words coined here, carrying a meaning specific to this workshop, which
// must never appear in anything the product says. internal/conformance
// enforces that direction; TestVocabulariesDoNotIntersect enforces that
// no word sits in both this lexicon and the product's.
reserved_dev: {
	"proving red":  "A test that must fail, and fail naming the thing that was broken. A red for the wrong reason proves nothing."
	demonstration: "A test showing that a behavior holds. A proving red is the demonstration for a refusal; a green fixture is the demonstration for an acceptance."
	cascade:       "An additional check a single-edit mutation draws beyond its declared one. Declared in alsoDraws, never silent."
	golden:        "A recorded output compared byte for byte. A mismatch is a deliberate shape change asking to be noticed."
	attack:        "A preserved input that must be refused, kept because it once found a defect."
	batch:         "One pull request's worth of work."
	"dev-lane":    "The workshop: building looplaw. Its counterpart is prod-lane, the installed product. The compound is the term — bare 'lane' belongs to no one, and the spec uses it for kernel-versus-client."
}

// Words that belong to the product, which the workshop therefore does
// not use for its own concepts. This is the direction that has cost this
// project: a script named dev/law, a ratification ritual copied from the
// product's act, a design basis living in a directory called law.
// Borrowing the product's words for workshop things reads as rigor and
// produces confusion.
//
// Two properties, tested: no product concept becomes a workshop thing
// (no shared word, no artifact named after one), and the product stands
// without the workshop (it builds and runs with dev/ deleted). Judging
// whether a product word is *discussed* or *borrowed* in workshop prose
// needs a reader, so that stays a list rather than a test.
do_not_borrow: [
	"closure — the contract method's coverage check (act closure). The workshop equivalent is demonstration coverage",
	"drift — what the kernel's provenance check reports, and what sync re-derives against (spec §10). A recorded output that has fallen out of step with the code is stale, not drifted",
	"claim, receipt, admission, version — record kinds; a test double is a fixture, not a claim",
	"party — a submitter in the product's model; this project's role is author, not party",
	"gap — the differ's unit; work not yet done is a to-do, not a gap",
	"ratify, admit, record — acts with named authorities. Merging a pull request ratifies nothing: ratification is a recorded act in the ledger, which is the thing this product exists to replace merging with",
	"verify — a read path over recorded facts; running the suite is checking, not verifying",
	"pin — the license check's hash comparison (spec §10); a hash recorded over this project's own files is recorded, not pinned",
]
