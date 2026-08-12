// The attack corpus index: what each attack is, and which refusal it
// must draw. Attacks are kept, not discarded — every blocking defect
// this project has found came from a hand-built attack, and a defect
// without a permanent attack can return silently.
//
// found_in records which review round produced the attack: the corpus
// carries its own provenance, like everything else here.
package attacks

#Attack: {
	file:      string
	found_in:  "gate" | "decomposition" | "absorber"
	must_draw: [string, ...string] // check ids the set must be refused by
}

attacks: [F=string]: #Attack & {file: F}

attacks: {
	"empty-blame-party.cue": {found_in:           "gate", must_draw: ["trinity/shape"]}
	"empty-act-name.cue": {found_in:              "gate", must_draw: ["trinity/shape"]}
	"homoglyph-term-fork.cue": {found_in:         "gate", must_draw: ["trinity/shape"]}
	"vacuous-set.cue": {found_in:                 "gate", must_draw: ["trinity/vacuity"]}
	"unreadable-region.cue": {found_in:           "gate", must_draw: ["trinity/region-unreadable"]}
	"dead-invariant.cue": {found_in:              "gate", must_draw: ["trinity/invariant-coverage"]}
	"authority-free-supplier.cue": {found_in:     "gate", must_draw: ["trinity/authority-free"]}
	"self-wire.cue": {found_in:                   "decomposition", must_draw: ["trinity/decomp-wire"]}
	// One character defeated every check over the region it marked. An
	// optional field states a constraint, not a value, so the field is
	// absent and each walk over it examined nothing while the file still
	// read as though it declared one. Found by adversarial review of the
	// declare batch; the same edit neutered the whole decomposition lane
	// and hid an absorbed view's provenance from the goal-law guard.
	"optional-interior.cue": {found_in:   "decomposition", must_draw: ["trinity/optional"]}
	"optional-provenance.cue": {found_in: "absorber", must_draw: ["trinity/optional"]}
	// The optional field's sibling, and the reason one syntax was never
	// the rule: a set states values, and "?:" was the only form the
	// gates held to it. A defaulted disjunction is concrete to CUE, so
	// these passed shape, passed every relational check by reading the
	// default, and were copied into a law version verbatim — law whose
	// value depends on what a consumer later unifies it with. Found by
	// security audit of the master tree.
	"defaulted-contract-status.cue": {found_in: "gate", must_draw: ["trinity/open-value"]}
	"defaulted-subject.cue": {found_in:         "gate", must_draw: ["trinity/open-value"]}
	// The same defect in the two forms that state what would be
	// admissible rather than what there is. The list also resists a
	// later unification where its closed spelling conflicts; the struct
	// does not, and is refused for what its bytes say alone.
	"open-list-of-cites.cue": {found_in:    "gate", must_draw: ["trinity/open-value"]}
	"open-struct-of-clauses.cue": {found_in: "gate", must_draw: ["trinity/open-value"]}
	// The check above examined fields, and a set states values by other
	// declarations too. Both of these passed it while carrying exactly
	// the value it refuses, because the walk had no path to build from a
	// declaration that states no label. Found by adversarial review of
	// the batch that added the check.
	"default-through-let.cue": {found_in:       "gate", must_draw: ["trinity/open-value"]}
	"default-through-embedding.cue": {found_in: "gate", must_draw: ["trinity/open-value"]}
	"double-satisfier.cue": {found_in:            "decomposition", must_draw: ["trinity/decomp-satisfier"]}
	"duplicate-child.cue": {found_in:             "decomposition", must_draw: ["trinity/decomp-resolve"]}
	"containment-cycle.cue": {found_in:           "decomposition", must_draw: ["trinity/decomp-tree"]}
	"ungrounded-interior.cue": {found_in:         "decomposition", must_draw: ["trinity/decomp-grounded"]}
	"provenance-phantom-source.cue": {found_in:   "absorber", must_draw: ["trinity/provenance-source"]}
	"provenance-unsourced-contract.cue": {found_in: "absorber", must_draw: ["trinity/provenance-coverage"]}
}
