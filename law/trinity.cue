// The trinity — the product's set model, v0. Batch 4, status: proposed.
//
// A TARGET PROJECT's contract set instantiates these definitions;
// looplaw's gates validate instances. This file is looplaw's definitional
// contract for the wire format; it becomes law when the accountable
// authority ratifies it — the aa's ratification merge is the recorded act
// (law/README.md). Lane note: everything here models an arbitrary target
// system, not looplaw and not any operator. The target system names its
// own parties and authorities in its own registry; looplaw's aa/recording
// authorities never appear in a target set.
//
// Form follows the System Design Contract Method §3 (parties · acts ·
// preconditions · guarantees · local invariants + cited globals ·
// synchronization · blame-and-evidence · markers) and §4 binding levels.
// Deliberately unbound in v0 — open gaps reported to the accountable
// authority, each with its reopening trigger:
//   - decomposition relations (parent/child sets, assembly satisfaction)
//     — trigger: the next law batch, with its own fixtures
//   - quantitative QoS clauses — trigger: any guarantee depending on a
//     quantitative bound
//   - wire compatibility with the fugit seal schema — trigger: that
//     schema is ratified and its wire shape is fixed
//   - a reserved-acts registry with two-way closure, and the legality of
//     self-party contracts (client == supplier) — trigger: the aa rules
package law

// Reference grammars: party and act ids share one grammar; every
// reference field carries it, so an empty or malformed reference is
// refused by shape, and the relational lane only ever adjudicates
// well-formed names.
#PartyRef: =~"^[a-z][a-z0-9-]*$"
#ActRef:   =~"^[a-z][a-z0-9-]*$"

// A term entry in a target project's lexicon: same anatomy as looplaw's
// own (#Entry), but authority points into the TARGET set's registry.
#TermEntry: {
	term:       string
	tier:       #Tier
	definition: string
	authority:  #PartyRef | "none" // a party id in the set's registry, or "none"
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
	id:             #PartyRef
	name:           string
	note:           string
	authority_free: bool
}

// A global invariant of the target system (its own tier, cited by id).
#SetInvariant: {
	id:        string & =~"^L-[0-9]+$" // disjoint from clause-id grammars by prefix
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
		client:   #PartyRef // party id: owes the preconditions
		supplier: #PartyRef // party id: owes the guarantees
	}
	acts: [...#ActRef] // reserved operations this contract holds; >= 1
	// Client obligations: each verifiable from the submission and
	// recorded state — never from good faith or live component internals.
	preconditions: [ID=string & =~"^P-[0-9]+$"]: {text: string}
	// Supplier postconditions: what becomes true, and what is recorded —
	// every state transition a guarantee produces is recorded (no silent
	// transitions).
	guarantees: [ID=string & =~"^G-[0-9]+$"]: {text: string, records: string}
	// Genuinely local invariants only; the set's globals are cited by id
	// in `cites`, never restated.
	invariants_local: [ID=string & =~"^LI-[0-9]+$"]: {text: string}
	cites: [...string] // #SetInvariant ids this contract binds under
	synchronization?: string // only where atomicity/ordering is doctrine
	// Which party is at fault for which violation class, adjudicated
	// from which recorded evidence — never from live component state.
	blame: [...{violation_class: string, at_fault: #PartyRef, evidence: string}]
	status:   #Status
	trigger?: string
}

// An experience entry: the judgment register. Advisory force always —
// never binding, never a contract clause in disguise.
#ExperienceEntry: {
	id:       string & =~"^X-[0-9]+$"
	judgment: string
	// At least one cite: judgment attaches to law, it never floats.
	cites: [string, ...string] // contract or invariant ids
	advisory: true
}

// A target project's complete set.
#TrinitySet: {
	subject:        string & =~"^[a-z][a-z0-9-]*$"
	schema_version: "0"
	registry: [ID=string]: #Party & {id: ID}
	invariants: [ID=string]: #SetInvariant & {id: ID}
	// Term keys are ASCII-constrained: a homoglyph fork of a term is a
	// collision generator and is refused by shape.
	lexicon: [T=string & =~"^[a-z][a-z0-9 -]*$"]: #TermEntry & {term: T}
	contracts: [ID=string]: #Contract & {id: ID}
	experience: [ID=string]: #ExperienceEntry & {id: ID}
	// A set with no judgment register declares the absence; silence is
	// not a declaration.
	experience_declared_absent: bool
}
