// The trinity — the product's set model, v0. Batch 4, status: proposed.
//
// A TARGET PROJECT's contract set instantiates these definitions;
// looplaw's gates validate instances. This file is looplaw's definitional
// contract for the wire format — law, ratified by merge per law/README.md.
// Lane note: everything here models an arbitrary target system, not
// looplaw and not any operator. The target system names its own parties
// and authorities in its own registry; looplaw's aa/recording authorities
// never appear in a target set.
//
// Form follows the System Design Contract Method §3 (parties · acts ·
// preconditions · guarantees · local invariants + cited globals ·
// synchronization · blame-and-evidence · markers) and §4 binding levels.
// Deliberately unbound in v0, each a recorded deferral:
//   - decomposition relations (parent/child sets, assembly satisfaction)
//     [drafting decision — confirm] — next batch, with its own fixtures
//   - quantitative QoS clauses — binds when any guarantee depends on a
//     quantitative bound
//   - wire compatibility with the fugit seal schema — binds when that
//     schema publishes
package law

// A term entry in a target project's lexicon: same anatomy as looplaw's
// own (#Entry), but authority points into the TARGET set's registry.
#TermEntry: {
	term:       string
	tier:       #Tier
	definition: string
	authority:  string // a party id in the set's registry, or "none"
	related: [...string]
	aliases: [...string]
	not: [...{misreading: string, write_instead: string}]
	collision: string
	docs:      string
	prompts:   string
	violation: string
	rewrite:   string
	status:    #Status
	trigger?:  string
}

// A party: an architectural component of the target system. Deliberately
// authority-free parties are a design statement the closure check
// certifies, not an omission.
#Party: {
	id:             string & =~"^[a-z][a-z0-9-]*$"
	name:           string
	note:           string
	authority_free: bool
}

// A global invariant of the target system (its own tier, cited by id).
#SetInvariant: {
	id:        string & =~"^[A-Z][A-Z0-9-]*$"
	text:      string
	rationale: string
}

// An act-bearing contract between two parties. Every reserved act of the
// target system appears in exactly one contract (relational closure,
// enforced by the gates, not expressible here).
#Contract: {
	id:   string & =~"^C-[A-Z0-9-]+$"
	name: string
	parties: {
		client:   string // party id: owes the preconditions
		supplier: string // party id: owes the guarantees
	}
	acts: [...string] // reserved operations this contract holds; >= 1
	// Client obligations: each verifiable from the submission and
	// recorded state — never from good faith or live component internals.
	preconditions: [ID=string]: {text: string}
	// Supplier postconditions: what becomes true, and what is recorded —
	// every state transition a guarantee produces is recorded (no silent
	// transitions).
	guarantees: [ID=string]: {text: string, records: string}
	// Genuinely local invariants only; the set's globals are cited by id
	// in `cites`, never restated.
	invariants_local: [ID=string]: {text: string}
	cites: [...string] // #SetInvariant ids this contract binds under
	synchronization?: string // only where atomicity/ordering is doctrine
	// Which party is at fault for which violation class, adjudicated
	// from which recorded evidence — never from live component state.
	blame: [...{violation_class: string, at_fault: string, evidence: string}]
	status:   #Status
	trigger?: string
}

// An experience entry: the judgment register. Advisory force always —
// never binding, never a contract clause in disguise.
#ExperienceEntry: {
	id:       string & =~"^X-[0-9]+$"
	judgment: string
	cites: [...string] // contract or invariant ids this judgment attaches to
	advisory: true
}

// A target project's complete set.
#TrinitySet: {
	subject:        string & =~"^[a-z][a-z0-9-]*$"
	schema_version: "0"
	registry: [ID=string]: #Party & {id: ID}
	invariants: [ID=string]: #SetInvariant & {id: ID}
	lexicon: [T=string]: #TermEntry & {term: T}
	contracts: [ID=string]: #Contract & {id: ID}
	experience: [ID=string]: #ExperienceEntry & {id: ID}
	// A set with no judgment register declares the absence; silence is
	// not a declaration.
	experience_declared_absent: bool
}
