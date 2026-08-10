// The product's authority registry and privileged acts — batch 1.
//
package dev

#Authority: {
	id:        string
	holder:    string // who/what holds it (a role, or a component id)
	holds:     string // the placement, one line
	rationale: string
}

#Component: {
	id:        string
	name:      string
	authority: "none" | "recording" // pointer into registry.authorities
	side:      "design" | "run"
	note:      string
	trigger?:  string // reopening condition, where provisionally ruled
}

#Act: {
	verb:      string
	changes:   string // the standing change this act alone produces
	authority: "accountable" | "recording" // pointer into registry.authorities
	rationale: string
	trigger?:  string
}

registry: {
	authorities: [ID=string]: #Authority & {id: ID}
	components: [ID=string]: #Component & {id: ID}
	acts: [V=string]: #Act & {verb: V}

	// Read paths: derived, rebuildable, no authority, never on the write
	// path. Refusal/denial is not an act — it is the non-happening of one,
	// recorded (denial-is-not-failure).
	readpaths: [...string]

	// Parties that appear in contracts but are not looplaw components;
	// all deliberately authority-free within looplaw's set.
	parties: [...string]
}

registry: {
	authorities: {
		accountable: {
			holder: "the deployment's accountable authority — a role: one human per deployment, singular, non-delegable"
			holds:  "all law-making acts (ratify, amend, withdraw, defer, grant); accountability assumed through these recorded acts"
			rationale: "contract method's accountability doctrine + the goal-contract-is-law ruling; no component is ever accountable"
		}
		recording: {
			holder: "store"
			holds:  "the record act: claims, receipts, admissions, versions commit here, append-only; recording settles that a thing was said, never that it is true"
			rationale: "claims-recorded-never-believed; one recording authority with record kinds carrying a law-side/evidence-side marker. Considered and rejected: looplearn as recorder — its store is derived/rebuildable/advisory by definition; holding standing records would put it on decision paths, make its availability load-bearing, and let the learner alter the evidence the law consumes. Looplearn ingests records and submits its advisories back through this act like any party."
		}
	}

	components: {
		store: {
			name:      "the store"
			authority: "recording"
			side:      "design"
			note:      "append-only; single recording authority across law-side and evidence-side record kinds"
			trigger:   "split into two recording authorities only if custody or clearance requirements ever diverge between law-side and evidence-side records"
		}
		gates: {
			name:      "the kernel gates"
			authority: "none"
			side:      "design"
			note:      "mechanism, never authority: gates verify preconditions of the record act and refuse with remedy; they execute ratifications and grants, originate nothing"
		}
		server: {
			name:      "the server"
			authority: "none"
			side:      "design"
			note:      "transport for store and gates over the wire; deliberately authority-free"
		}
		absorber: {
			name:      "the absorber (client)"
			authority: "none"
			side:      "design"
			note:      "proposer only: produces claims that enter through gates like any client's; deliberately authority-free"
		}
		projector: {
			name:      "the projector (context/brief)"
			authority: "none"
			side:      "design"
			note:      "read path: derived, rebuildable; outputs never on decision paths"
		}
		differ: {
			name:      "the differ"
			authority: "none"
			side:      "design"
			note:      "read path: gap computation over law and recorded evidence"
		}
		skins: {
			name:      "CLI / MCP skins"
			authority: "none"
			side:      "design"
			note:      "two skins over one client library; transport only"
		}
	}

	acts: {
		ratify: {
			changes:   "a draft becomes law (goal contract, lexicon entry, registry change, Tier 0 invariant, standing grant)"
			authority: "accountable"
			rationale: "the only law-making act; nothing else confers standing on law"
		}
		amend: {
			changes:   "ratified law is replaced by a new version; the predecessor is archived, never deleted"
			authority: "accountable"
			rationale: "re-enters the artifact loop; never-fork — exactly one live version"
		}
		withdraw: {
			changes:   "a law clause or goal is retired without replacement"
			authority: "accountable"
			rationale: "the proven-too-expensive exit; design-time retirement — no collision with run-time decommission, which stays reserved for the commission family"
			trigger:   "if the process-vocabulary lexicon sweep dispositions 'withdraw' differently, rename by amendment"
		}
		defer: {
			changes:   "a gap or clause is parked with destination, authority, and trigger"
			authority: "accountable"
			rationale: "deferral discipline: nothing defers into the void; triggers are monitored"
		}
		grant: {
			changes:   "a standing grant licenses a class of automatic admissions (e.g. guest-mode working sets)"
			authority: "accountable"
			rationale: "the process method's device for hands-free throughput: ratify the class once, gates check membership per submission"
			trigger:   "grant record schema to be fixed when the first grant is drafted"
		}
		record: {
			changes:   "a submission becomes a record (claim, receipt, admission, version) after passing the gates"
			authority: "recording"
			rationale: "admission is not a separate privileged act: it is the record act executing behind gates — mechanism words must not smuggle authority"
		}
	}

	readpaths: ["diff", "project", "verify", "status", "export"]

	parties: ["loopstrap instance", "loopmaster", "harness agent", "looplearn", "fugit"]
}
