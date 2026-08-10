// The gap schema — the differ's output and the planning feed.
//
// RATIFIED: PR #12 merged by xormania 2026-08-10 (the recorded act per
// law/README.md). Amendments only by new PR.
//
// A gap measures contract disequilibrium: goal-law against the absorbed
// current view. A gap is a planning state, never an error state — a red
// for law is work, not failure. Lifecycle: open → in-work → satisfied,
// or → proven-too-expensive, which initiates the amendment path. Two
// terminal paths, no third; a gap is never silently abandoned.
//
// The differ is a read path: it computes gaps and holds no authority —
// gaps inform planning decisions and decide nothing themselves. A
// persisted gap enters the store as a claim of the differ's computation
// (recorded, never believed) — 'gap record' names no record kind. Only
// the differ derives gaps; parties report circumstances as claims.
package schema

// Structural kinds (law against view), the v0 set. "violated" and
// "unproven" join when receipts feed the differ — reopening trigger:
// the receipt path lands in the store.
#GapKind: "absent" | "added" | "changed"

// The two kinds of work a gap can call for: satisfy the contract as it
// stands, or decompose further before anyone fills anything.
#WorkKind: "fill" | "split"

#GapStatus: "open" | "in-work" | "satisfied" | "proven-too-expensive"

#Gap: {
	id:      string & =~"^GAP-[0-9]+$" // stable; never renumbered
	subject: string // the set subject the gap belongs to
	// Clause-addressed: the contract, and the clause within it when the
	// gap is clause-grain (absent contract-level gaps carry no clause).
	address: {
		contract: string
		clause?:  string
	}
	kind:   #GapKind
	work:   #WorkKind
	detail: string // what differs, concretely
	// Content hashes of the two sides, for staleness detection and
	// stable re-identification; "" on a side where the thing is absent.
	goal_hash: string
	view_hash: string
	// v0 diff emits "open" only; the fuller lifecycle binds when the
	// store assumes gap custody.
	status: #GapStatus
	// For proven-too-expensive: the recorded amendment-path reference
	// this termination initiated (law is never left standing violated).
	amendment_ref?: string
}
