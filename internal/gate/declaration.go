package gate

import (
	"fmt"

	"cuelang.org/go/cue"

	"github.com/xormania/looplaw/internal/outcome"
)

// DeclarationChecks enumerates every check the declaration gates can
// emit. As with the trinity and submission gates, the suite proves a red
// for each: an undemonstrated gate is an unproven behavior.
var DeclarationChecks = []string{
	"declare/provenance",
	"declare/party",
	"declare/subject-mismatch",
}

// Declaration is a proposed goal set offered toward the gates — an
// amendment draft. It holds no standing: what a party declares is a
// claim about what the law should become, and it binds nothing until the
// accountable authority ratifies it.
type Declaration struct {
	Name    string // for refusal subjects; the proposal is the bytes
	Body    []byte
	Party   string
	Subject string // the subject this ledger already carries, "" when it holds none
}

// ValidateDeclaration verifies the preconditions of declaring an
// amendment draft. It judges nothing about the merits: whether the proposed
// law is wise is not a question the gates ask, and recording settles
// only that it was proposed.
// The returned check list names the checks that actually ran, not the
// act's full set: an admission recording a check as run when it was
// skipped is a small laundering of what happened.
func ValidateDeclaration(d Declaration) (cue.Value, []string, []outcome.Refusal) {
	var refusals []outcome.Refusal
	var ran []string
	refuse := func(check, subject, reason, remedy string) {
		refusals = append(refusals, outcome.Refusal{
			Class: outcome.Rejection, Check: check,
			Subject: subject, Reason: reason, Remedy: remedy,
		})
	}

	ran = append(ran, "declare/party")
	if !nameRE.MatchString(d.Party) {
		refuse("declare/party", "party", "a declaration names no declaring party",
			"name the declaring party; recording settles that a party proposed a thing, which is unstatable without the party")
	}

	// The trinity refusals pass through as themselves. Each already
	// names its check and carries the remedy that repairs it, and those
	// refusals are the worklist — rewrapping them would restate a remedy
	// beside the precise one and leave a consumer reading two.
	// The list this returns, not Checks: the bytes path cannot run
	// trinity/load, and recording it as run is evidence of an
	// examination that did not happen.
	set, setRan, setRefusals := LoadSetBytes(d.Name, d.Body)
	ran = append(ran, setRan...)
	refusals = append(refusals, setRefusals...)
	if len(refusals) > 0 {
		return cue.Value{}, ran, refusals
	}

	// Goal law is authored, never derived. A set carrying provenance was
	// absorbed from a scope, which makes it evidence of what is — and
	// evidence never sets the standard it is measured against (T0-2).
	// Declaring one as goal law would let a party's claim about the
	// present become the law the present is judged by.
	ran = append(ran, "declare/provenance")
	if set.LookupPath(cue.ParsePath("provenance")).Exists() {
		refuse("declare/provenance", d.Name,
			"the proposed set carries provenance, so it is an absorbed view — evidence of what is, not a proposal for what should be",
			"declare authored goal law. To propose that current behavior become the standard, author it as goal law without provenance; the view stays evidence")
		return cue.Value{}, ran, refusals
	}

	// A project's law is about one subject. A declaration naming a
	// different one is either the wrong project or a rename, and both
	// are worth refusing rather than silently forking a second subject
	// into one ledger.
	proposed, _ := set.LookupPath(cue.ParsePath("subject")).String()
	ran = append(ran, "declare/subject-mismatch")
	if d.Subject != "" && proposed != d.Subject {
		refuse("declare/subject-mismatch", proposed,
			fmt.Sprintf("the proposed set is about %q, and this project's law is about %q", proposed, d.Subject),
			"declare against the project whose law this proposal addresses; changing the subject is a separate act, and only the accountable authority performs it")
		return cue.Value{}, ran, refusals
	}

	return set, ran, refusals
}
