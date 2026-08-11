// Package outbound is the single point where content held by looplaw
// leaves it, and a placeholder for a system that does not exist yet.
//
// A context custody system is coming that will be the sole gate for
// content crossing a boundary: it decides, it performs the crossing, and
// anything crossing that it did not perform is exfiltration by its own
// definition. looplaw is not that system and should not become it — one
// component holding both the law and the custody of what may be seen
// would be the same component deciding and enforcing, which this project
// refuses everywhere else.
//
// Until that system exists something has to hand content over, so
// looplaw does, through here and nowhere else. The default hands it back
// unchanged: standing in for a gate is not the same as being one, and
// pretending otherwise would be worse than the gap. What this buys is
// that the gap is in one place, named, and attached to by replacing a
// variable rather than by finding every fmt.Print in the tree.
//
// Deliberately not modelled here: the custody system's own vocabulary
// and its decision shape. Guessing at those would coin a second
// product's terms in this one, and its lexicon is not ratified yet. This
// interface says only what looplaw needs to say — a party asked for
// content, for a stated purpose — and the replacement is free to require
// more.
package outbound

import "github.com/xormania/looplaw/internal/outcome"

// Request is what looplaw knows about content leaving: who asked, and
// what for. Both are as claimed — looplaw checks no identity and asserts
// none, which is also the custody system's stance on identity.
type Request struct {
	// Party is the claimed requester.
	Party string
	// Purpose is the act that produced this content, so a later gate can
	// decide against what was being done rather than guessing.
	Purpose string
	// Subject is what the content is about, where the act names one.
	Subject string
	Content string
}

// Gate decides whether content may leave and returns what may leave. A
// refusal is a denial: the recorded non-happening of a requested act,
// which is a successful execution rather than a failure to retry around.
type Gate interface {
	Release(Request) (string, *outcome.Refusal)
}

// Open hands content over unchanged. It is the honest default while no
// custody system exists: it decides nothing, and its name says so rather
// than implying a check that is not happening.
type Open struct{}

func (Open) Release(r Request) (string, *outcome.Refusal) { return r.Content, nil }

// Default is the gate content leaves through. A deployment — or the
// custody system, once it exists — substitutes it here, and no act
// changes.
var Default Gate = Open{}

// Release sends content out through the configured gate. Every path that
// emits held content calls this, so there is one place to attach and one
// place to audit.
func Release(r Request) (string, *outcome.Refusal) { return Default.Release(r) }
