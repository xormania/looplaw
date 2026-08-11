// DRAFT VIEW SKELETON — not yet a valid set.
//
// Derived from a submitted component manifest. Registry, contracts and
// provenance below state only what a tool established: these components
// exist, each was derived from these sources, and one holds a compiled
// reference to another. The statement regions are empty and the gates
// will refuse this set until they are authored — those refusals are the
// worklist.
//
// A contract here says only that a dependency exists, not what it
// promises. Filling in acts, preconditions, guarantees and blame is the
// work; where a dependency turns out to promise nothing worth stating,
// that is a finding about the system rather than a gap in this file.
//
// This view is evidence, never law: it states what a party claims the
// system currently is — submitted as a claim, recorded never believed.
// Law is authored and ratified separately.
//
// Declare experience_declared_absent yourself: silence is not a
// declaration, so the binary leaves it to the author.
subject:        "demo"
schema_version: "0"

registry: {
	"cmd-tool": {name: "cmd/tool", note: "the command"}  // authority_free: true|false
	"internal-core": {name: "internal/core", note: ""}  // authority_free: true|false
}

invariants: {}
lexicon: {}

contracts: {
	"C-CMD-TOOL-INTERNAL-CORE": {
		id:   "C-CMD-TOOL-INTERNAL-CORE"
		name: "cmd/tool depends on internal/core"
		parties: {client: "cmd-tool", supplier: "internal-core"}
		acts: []              // TODO: the reserved operations this contract holds
		preconditions: {}     // TODO: what cmd/tool owes before calling
		guarantees: {}        // TODO: what internal/core promises, and what it records
		invariants_local: {}
		cites: []
		blame: []             // TODO: who is at fault for which violation class
		status: "proposed"
	}
}

experience: {}
// experience_declared_absent: true|false

provenance: {
	scope: "demo"
	sources: {
		"cmd/tool/main.go": "1111111111111111111111111111111111111111111111111111111111111111"
		"internal/core/core.go": "2222222222222222222222222222222222222222222222222222222222222222"
	}
	derivations: {
		"C-CMD-TOOL-INTERNAL-CORE": ["cmd/tool/main.go", "internal/core/core.go"]
	}
}
