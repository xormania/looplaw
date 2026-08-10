// ATTACK FIXTURE — a set that MUST be refused.
//
// Preserved from an adversarial review round rather than discarded:
// every blocking defect this project has found came from a hand-built
// attack, and each one is kept here so the defect can never return
// silently and so the next reviewer starts from what has already been
// tried. Expected refusals are declared in index.cue beside this file.
//
// Round 2 (decomposition): two children feeding only each other, with no client-owed input, validated green — a closed feed loop that no execution order can ever enter.
subject:        "loop-sys"
schema_version: "0"
registry: {
	boss: {name: "the boss", note: "outer client", authority_free: true}
	w1: {name: "worker one", note: "supplier", authority_free: false}
	w2: {name: "worker two", note: "supplier", authority_free: false}
}
invariants: {}
lexicon: {}
contracts: {
	"C-P-1": {
		name: "the parent"
		parties: {client: "boss", supplier: "w1"}
		acts: ["outer-act"]
		preconditions: {}
		guarantees: {}
		invariants_local: {}
		cites: []
		blame: []
		status: "ratified"
		interior: {
			children: ["C-A-1", "C-B-1"]
			wires: [
				{from: {child: "C-A-1", guarantee: "G-1"}, to: {child: "C-B-1", precondition: "P-1"}},
				{from: {child: "C-B-1", guarantee: "G-1"}, to: {child: "C-A-1", precondition: "P-1"}},
			]
			presents: {}
		}
	}
	"C-A-1": {
		name: "child a"
		parties: {client: "w2", supplier: "w1"}
		acts: ["a-act"]
		preconditions: {"P-1": {text: "B's output exists."}}
		guarantees: {"G-1": {text: "A's output exists.", records: "the a record"}}
		invariants_local: {}
		cites: []
		blame: []
		status: "ratified"
	}
	"C-B-1": {
		name: "child b"
		parties: {client: "w1", supplier: "w2"}
		acts: ["b-act"]
		preconditions: {"P-1": {text: "A's output exists."}}
		guarantees: {"G-1": {text: "B's output exists.", records: "the b record"}}
		invariants_local: {}
		cites: []
		blame: []
		status: "ratified"
	}
}
experience: {}
experience_declared_absent: true
