// The trinity — the product's set model.
//
// v0 RATIFIED: PR #8 merged by xormania 2026-08-09. The decomposition
// amendment (#Interior, #Wire, #Contract.interior, the groundability and
// one-satisfier rulings) RATIFIED: PR #11 merged by xormania 2026-08-10
// (the recorded acts per law/README.md); predecessors archived in
// history, never deleted.
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
// Deliberately unbound — open gaps reported to the accountable
// authority, each with its reopening trigger:
//   - CROSS-SET decomposition (a child set in its own file/scope, for
//     chunk handoff) — trigger: the store assumes set custody; this
//     amendment binds IN-SET decomposition only (one system, one set,
//     the contract tree within it)
//   - semantic weakening of preconditions in refinement — v0 binds
//     id-equality only (a shared-client child may state exactly the
//     parent's precondition ids); judging 'weaker' needs the judge seam
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
	// The contract's decomposition, when its interior is designed.
	interior?: #Interior
}

// A wire: dataflow inside an interior — a child's guarantee feeds a
// DIFFERENT child's precondition (a self-wire feeds nothing: the
// guarantee exists only after the act whose precondition it claims to
// feed, and is refused). The wiring must be GROUNDABLE: under one-shot
// act semantics a child can act once its preconditions are owed by the
// shared client or fed by a child that can already act, so a closed
// feed loop with no entry executes nothing and is refused. Ruled: this
// supersedes the earlier cycles-permitted note; reopening trigger —
// act semantics gaining multiplicity (streaming/iterative flows), at
// which point legitimate feedback returns as its own construct.
#Wire: {
	from: {child: string, guarantee: string}
	to: {child: string, precondition: string}
}

// The interior of a contract: its decomposition, designed. The boundary
// is held — the children, unified along their wiring, jointly present
// the parent's guarantees — and beyond what presents and wires state,
// the interior stays free (black box: how a child is filled is
// invisible here too). Refinement discipline (contract method §9): a
// child sharing the parent's client may state only precondition ids the
// parent states; children cite every invariant the parent cites, never
// fewer.
#Interior: {
	children: [string, ...string] // contract ids in this set, each listed once
	wires: [...#Wire]
	// Assembly satisfaction: every guarantee of the parent contract is
	// presented by exactly one child guarantee, one-to-one — a child
	// guarantee presents at most one parent guarantee, or the boundary
	// claims more than the assembly produces. Ruled with it: every
	// obligation has exactly one satisfier of record — a precondition
	// both client-owed and wire-fed is refused (blame adjudicates from
	// recorded evidence and must be able to name the failed satisfier);
	// reopening trigger — a legitimate dual-source case, at which point
	// precedence gets ruled.
	presents: [PG=string]: {child: string, guarantee: string}
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
