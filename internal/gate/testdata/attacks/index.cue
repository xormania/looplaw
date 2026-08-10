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
	"double-satisfier.cue": {found_in:            "decomposition", must_draw: ["trinity/decomp-satisfier"]}
	"duplicate-child.cue": {found_in:             "decomposition", must_draw: ["trinity/decomp-resolve"]}
	"containment-cycle.cue": {found_in:           "decomposition", must_draw: ["trinity/decomp-tree"]}
	"ungrounded-interior.cue": {found_in:         "decomposition", must_draw: ["trinity/decomp-grounded"]}
	"provenance-phantom-source.cue": {found_in:   "absorber", must_draw: ["trinity/provenance-source"]}
	"provenance-unsourced-contract.cue": {found_in: "absorber", must_draw: ["trinity/provenance-coverage"]}
}
