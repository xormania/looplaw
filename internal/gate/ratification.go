package gate

import (
	"fmt"

	"github.com/xormania/looplaw/internal/outcome"
)

// RatificationChecks enumerates every check the ratification gates can
// emit. As with the other gates, the suite proves a red for each: an
// undemonstrated gate is an unproven behavior.
var RatificationChecks = []string{
	"ratify/unbound",
	"ratify/authority",
	"ratify/target",
	"ratify/standing",
}

// Ratification is what is offered toward the gates when a draft is to
// become law. The gates verify preconditions and refuse with remedy;
// they confer nothing. Standing arises from the act being recorded, and
// only the accountable authority performs it.
type Ratification struct {
	// Party is the claimed performer. looplaw checks no identity and
	// asserts none — the check is that the claim matches the authority
	// this deployment has on record, not that the claimant is who they
	// say.
	Party string
	// Authority is the party recorded as holding the accountable
	// authority, empty when the deployment has bound none.
	Authority string
	// Subject is what is to become law.
	Subject string
	// Declared is true when a declaration for this subject is on record.
	// Ratification acts on a draft that was declared; there is nothing
	// else for it to act on.
	Declared bool
	// HasStanding is true when law for this subject already exists.
	// Ratification is a draft's first standing; changing law that
	// already holds it is a different act.
	HasStanding bool
}

// ValidateRatification verifies the preconditions of the ratify act. It
// judges nothing about the merits: whether a draft should become law is
// the accountable authority's question, and the gates only establish
// that the act is theirs to perform and that there is a draft to perform
// it on.
func ValidateRatification(r Ratification) []outcome.Refusal {
	var refusals []outcome.Refusal
	refuse := func(class outcome.Class, check, subject, reason, remedy string) {
		refusals = append(refusals, outcome.Refusal{
			Class: class, Check: check,
			Subject: subject, Reason: reason, Remedy: remedy,
		})
	}

	// Nothing can be law where no authority is on record. The binding is
	// not a formality the act can assume: without it there is no party
	// whose act would confer standing, so the whole question is
	// unanswerable rather than merely unmet.
	if r.Authority == "" {
		refuse(outcome.Rejection, "ratify/unbound", "the deployment",
			"no accountable authority is on record for this deployment, so no party's act can make law",
			"record the deployment's accountable authority first; until then every declaration stays a claim and binds nothing")
		return refusals
	}

	// Only the accountable authority ratifies. Every other party
	// proposes, and a proposal is recorded as a claim — which is what
	// declaring already does.
	if r.Party != r.Authority {
		refuse(outcome.Rejection, "ratify/authority", r.Party,
			fmt.Sprintf("%q is not this deployment's accountable authority", r.Party),
			"submit the draft as a declaration and leave it for the accountable authority; ratification is not delegable, and a party cannot confer standing on its own work")
		return refusals
	}

	if !r.Declared {
		refuse(outcome.Rejection, "ratify/target", r.Subject,
			"no declaration for this subject is on record, so there is no draft to ratify",
			"declare the goal set first; ratification acts on a draft, never on a file")
	}

	if r.HasStanding {
		refuse(outcome.Rejection, "ratify/standing", r.Subject,
			"law for this subject already holds standing, and ratification is a draft's first standing",
			"amend the ratified law instead; a successor version arises only by the amend act, and the predecessor is archived rather than replaced in place — this binary does not perform amend yet, so flag it to the accountable authority and leave the ratified law alone")
	}

	return refusals
}
