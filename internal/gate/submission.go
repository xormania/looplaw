package gate

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/xormania/looplaw/internal/outcome"
)

// SubmissionChecks enumerates every check the submission gates can
// emit. As with the trinity gates, the suite proves a red for each: an
// undemonstrated gate is an unproven behavior.
var SubmissionChecks = []string{
	"submit/kind",
	"submit/subject",
	"submit/party",
	"submit/content",
	"submit/receipt-shape",
}

// Submission is what a party offers toward the gates: pre-record,
// holding no standing. It becomes a record only if the gates pass it
// and the store records it.
type Submission struct {
	Kind    string // "claim" or "receipt" — the record kinds a party may submit
	Subject string
	Party   string
	Body    string
}

// Receipt is the ratified receipt shape: evidence of something that
// happened elsewhere.
type Receipt struct {
	Subject string `json:"subject"`
	Verdict string `json:"verdict"`
	Source  string `json:"source"`
	Hash    string `json:"hash"`
}

// IsName reports whether a string is a name an act may record: a
// subject, or a party. Exported because the acts that record one are
// not all in this package, and a second copy of the grammar is a second
// thing to keep in step — the authority binding checked its party
// against the empty string alone, which admitted "   " as a
// deployment's accountable authority.
func IsName(s string) bool { return nameRE.MatchString(s) }

var (
	nameRE = regexp.MustCompile(`^[^\s][^\n]*$`)
	hashRE = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

// submittableKinds are the record kinds a party may offer. Admissions
// and versions are produced by the system in the course of an act, so
// no party submits one: an admission a party wrote would say the entry
// happened before it had.
var submittableKinds = map[string]bool{"claim": true, "receipt": true}

// ValidateSubmission verifies the preconditions of the record act. It
// judges nothing about the content: whether a claim is true is not a
// question the gates ask, and recording settles only that it was said.
func ValidateSubmission(sub Submission) []outcome.Refusal {
	var refusals []outcome.Refusal
	refuse := func(check, subject, reason, remedy string) {
		refusals = append(refusals, outcome.Refusal{
			Class: outcome.Rejection, Check: check,
			Subject: subject, Reason: reason, Remedy: remedy,
		})
	}

	if !submittableKinds[sub.Kind] {
		var kinds []string
		for k := range submittableKinds {
			kinds = append(kinds, k)
		}
		reason := fmt.Sprintf("%q is not a kind a party may submit", sub.Kind)
		if sub.Kind == "admission" || sub.Kind == "version" {
			reason = fmt.Sprintf("%q is produced in the course of an act, never offered by a party", sub.Kind)
		}
		refuse("submit/kind", sub.Kind, reason,
			"submit a claim (what you state) or a receipt (evidence of something that happened elsewhere)")
	}

	if !nameRE.MatchString(sub.Subject) {
		refuse("submit/subject", "subject", "a submission names no subject",
			"name what the submission is about; a record about nothing can never be found or falsified")
	}
	if !nameRE.MatchString(sub.Party) {
		refuse("submit/party", "party", "a submission names no submitting party",
			"name the submitting party; recording settles that a party said a thing, which is unstatable without the party")
	}
	if strings.TrimSpace(sub.Body) == "" {
		refuse("submit/content", "body", "a submission carries nothing",
			"state what is claimed, or what the receipt evidences")
	}

	if sub.Kind == "receipt" {
		var r Receipt
		switch err := json.Unmarshal([]byte(sub.Body), &r); {
		case err != nil:
			refuse("submit/receipt-shape", "body",
				fmt.Sprintf("a receipt's body is not readable as its ratified shape: %v", err),
				`a receipt is (subject, verdict, source, hash): {"subject":…,"verdict":…,"source":…,"hash":…}`)
		case r.Subject == "" || r.Verdict == "" || r.Source == "":
			refuse("submit/receipt-shape", "body",
				"a receipt states no subject, verdict, or source",
				"evidence with no source and no verdict evidences nothing; state all three")
		case !hashRE.MatchString(r.Hash):
			refuse("submit/receipt-shape", "hash",
				fmt.Sprintf("%q is not a sha256 hex digest", r.Hash),
				"a receipt carries the digest of what it evidences, so it can be checked later")
		}
	}

	return refusals
}
