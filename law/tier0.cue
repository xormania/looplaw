// Tier 0 — global invariants. Batch 2, all proposed.
//
// Every contract at every level is bound by this tier. No contract may
// weaken an invariant, restate it divergently, or locally re-derive it;
// contracts cite invariants by id only. Each invariant fixes exactly one
// authority placement or one prohibition (composites get split).
// Ratification protocol: law/README.md.
package law

#Invariant: {
	id:        =~"^T0-[0-9]+$"
	rule:      string // one sentence, binding
	rationale: string
	status:    #Status
	trigger?:  string
}

tier0: [ID=string]: #Invariant & {id: ID}

tier0: {
	"T0-1": {
		rule:      "Law reaches a scope only by descent: from its parent's ratified law, or by a ratified amendment at its own level — never from below, never from a sibling."
		rationale: "the downward half of law-down/evidence-up; keeps every level able to trust what it received"
		status:    "proposed"
	}
	"T0-2": {
		rule:      "Nothing ascending confers standing: what flows up is evidence, which may initiate the amendment path and nothing else."
		rationale: "the upward half; evidence promoted to law anywhere poisons every level above it"
		status:    "proposed"
	}
	"T0-3": {
		rule:      "The kernel performs no model inference: identical recorded inputs yield identical results, offline."
		rationale: "the deterministic organ in a stochastic system; determinism is the verification anchor"
		status:    "proposed"
	}
	"T0-4": {
		rule:      "The kernel never fetches or inspects work-product content: it decides over submitted claims, manifests, and recorded state only."
		rationale: "substrate-blindness; the decider decides over supplied inputs only — surveys are brought to the law"
		status:    "proposed"
	}
	"T0-5": {
		rule:      "No producer statement gains force except through a recorded act that consumes it."
		rationale: "claims are recorded, never believed; recording settles that a thing was said, never that it is true"
		status:    "proposed"
	}
	"T0-6": {
		rule:      "Every standing change commits through the record act; there are no silent transitions."
		rationale: "completeness; an unrecorded transition is unverifiable and unblameable"
		status:    "proposed"
	}
	"T0-7": {
		rule:      "No processing outcome changes standing; only a recorded act of the named authority does."
		rationale: "acts, not adjectives; blocks status laundering at the root"
		status:    "proposed"
	}
	"T0-8": {
		rule:      "Accountability vests in the deployment's accountable authority alone; components and agents carry blame, adjudicated from recorded evidence, never accountability."
		rationale: "the accountability doctrine; blame from recorded evidence only, never live state"
		status:    "proposed"
	}
	"T0-9": {
		rule:      "Advisory outputs never sit on a decision path: what a read-path or advisory component produces informs, and only acts decide."
		rationale: "keeps the learner, projections, and derived views structurally incapable of gating the loop they observe"
		status:    "proposed"
	}
}
