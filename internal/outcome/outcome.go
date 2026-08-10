// Package outcome encodes the failure doctrine: every non-advancing result
// is one of four classes, and none of them is a generic "error".
//
//   - Rejection: a malformed submission or breached client precondition.
//     Nothing commits; the gate keeps operating.
//   - Denial: a well-formed submission the deciding authority declines. A
//     successful execution — recorded, never routed through error handling.
//   - Abort: an infrastructure failure. Nothing commits, no partial records.
//   - Finding: on verification paths (which commit nothing), an item the
//     checks cannot process — first-class, never skipped.
//
// A refusal names what was refused, which check refused it, why, and the
// remedy. Refusals are protocol, not UX: they are fed back to authoring
// callers for retry, so the remedy is part of the contract.
//
// Design basis: proj/looplaw-spec.md (failure taxonomy, refusals-as-
// protocol) and the contract method's shared doctrine.
package outcome

import "fmt"

// Class is the outcome class of an operation.
type Class int

const (
	// OK means the operation advanced.
	OK Class = iota
	// Rejection: malformed submission or client precondition breach.
	Rejection
	// Denial: well-formed, declined by the deciding authority.
	Denial
	// Abort: infrastructure failure; nothing committed.
	Abort
	// Finding: a verification-path item the checks cannot process.
	Finding
)

func (c Class) String() string {
	switch c {
	case OK:
		return "ok"
	case Rejection:
		return "rejection"
	case Denial:
		return "denial"
	case Abort:
		return "abort"
	case Finding:
		return "finding"
	default:
		return fmt.Sprintf("outcome.Class(%d)", int(c))
	}
}

// Process exit codes. Distinct on purpose: a denial is not a rejection and
// neither is an abort; scripts and callers branch on the class without
// parsing text. Values are pre-1.0 and may be renumbered until a consumer
// contract pins them.
const (
	ExitOK        = 0
	ExitRejection = 1
	ExitDenial    = 2
	ExitAbort     = 3
	ExitFinding   = 4
	ExitUsage     = 64
)

// ExitCode maps a class to its process exit code.
func (c Class) ExitCode() int {
	switch c {
	case Rejection:
		return ExitRejection
	case Denial:
		return ExitDenial
	case Abort:
		return ExitAbort
	case Finding:
		return ExitFinding
	default:
		return ExitOK
	}
}

// Refusal is a non-advancing outcome with its remedy attached. Class is
// never OK in a valid Refusal.
type Refusal struct {
	Class   Class  // rejection, denial, abort, or finding
	Check   string // the gate or check that refused (e.g. "manifest-hash")
	Subject string // what was refused (e.g. a set, clause id, file)
	Reason  string // what is wrong, concretely
	Remedy  string // what the submitter does next; "" only when none exists
}

// Error renders the refusal in its wire shape:
//
//	<check>: <class> <subject>: <reason> — remedy: <remedy>
//
// Every field that is present appears; the remedy clause is omitted only
// when no remedy exists.
func (r *Refusal) Error() string {
	s := r.Check
	if s != "" {
		s += ": "
	}
	s += r.Class.String()
	if r.Subject != "" {
		s += " " + r.Subject
	}
	if r.Reason != "" {
		s += ": " + r.Reason
	}
	if r.Remedy != "" {
		s += " — remedy: " + r.Remedy
	}
	return s
}
