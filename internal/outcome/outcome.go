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

import (
	"fmt"
	"strings"
	"unicode"
)

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
	s := oneLine(r.Check)
	if s != "" {
		s += ": "
	}
	s += r.Class.String()
	if r.Subject != "" {
		s += " " + oneLine(r.Subject)
	}
	if r.Reason != "" {
		s += ": " + oneLine(r.Reason)
	}
	if r.Remedy != "" {
		s += " — remedy: " + oneLine(r.Remedy)
	}
	return s
}

// oneLine renders a field so it cannot end the line it is written on.
//
// Every field of a refusal is dynamic — a path a caller typed, a subject
// a party submitted, a parser's own message — and one refusal is one
// line. Concatenated as they arrived, a newline in any of them ended the
// line early and what followed read as an independent refusal that no
// check ever emitted. A carriage return forges the same thing without a
// newline: it returns a terminal's cursor to column zero, so what
// follows overwrites what came before, and consumers that split on line
// breaks the way Python's splitlines does treat it as one.
//
// Escaped rather than stripped, because a refusal names what was
// refused: a field with a newline in it must still show what was there,
// or the reader is left guessing at a path they cannot see.
//
// Only control characters are touched. The wire form carries an em dash
// and refusals quote submitted text, so escaping anything by width or
// by not being ASCII would mangle what a caller needs to read.
func oneLine(field string) string {
	if strings.IndexFunc(field, unicode.IsControl) < 0 {
		return field
	}
	var b strings.Builder
	b.Grow(len(field))
	for _, r := range field {
		switch {
		case !unicode.IsControl(r):
			b.WriteRune(r)
		case r == '\n':
			b.WriteString(`\n`)
		case r == '\r':
			b.WriteString(`\r`)
		case r == '\t':
			b.WriteString(`\t`)
		default:
			fmt.Fprintf(&b, `\x%02x`, r)
		}
	}
	return b.String()
}
