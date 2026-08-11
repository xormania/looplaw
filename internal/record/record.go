// Package record executes the record act: a submission that has passed
// the kernel gates becomes a record, held append-only by the store.
//
// Two records land per submission, together: the submitted content, and
// an admission recording the entry event — who submitted, what, and
// which checks passed. The admission is what makes an entry accountable
// later; a record without one would be content whose arrival nobody can
// reconstruct. Recording settles that a thing was said, never that it
// is true.
package record

import (
	"encoding/json"
	"fmt"
	"os"

	"cuelang.org/go/cue"

	"github.com/xormania/looplaw/internal/gate"
	"github.com/xormania/looplaw/internal/outcome"
	"github.com/xormania/looplaw/internal/store"
)

// Admission is the entry event, recorded: it names the submitting party, what
// was submitted, and the checks the submission passed. It confers no
// standing — it settles that the entry happened, never that the content
// is true.
type Admission struct {
	Kind        string   `json:"kind"`
	Subject     string   `json:"subject"`
	Party       string   `json:"party"`
	ContentHash string   `json:"content_hash"`
	ChecksRun   []string `json:"checks_run"`
	Grant       string   `json:"grant,omitempty"`
}

// Submit runs the submission gates and, if they pass, performs the
// record act. Refusals carry their remedy and nothing is recorded — a
// refused submission leaves no trace, which is why the gates are
// mechanism: they originate nothing.
func Submit(s *store.Store, sub gate.Submission) ([]store.Record, []outcome.Refusal) {
	if refusals := gate.ValidateSubmission(sub); len(refusals) > 0 {
		return nil, refusals
	}

	content := store.Draft{
		Kind:    store.Evidence,
		Type:    sub.Kind,
		Subject: sub.Subject,
		Body:    sub.Body,
		Party:   sub.Party,
	}

	adm := Admission{
		Kind:      sub.Kind,
		Subject:   sub.Subject,
		Party:     sub.Party,
		ChecksRun: gate.SubmissionChecks,
	}
	adm.ContentHash = store.ContentHash(content)
	body, err := json.Marshal(adm)
	if err != nil {
		return nil, []outcome.Refusal{{
			Class: outcome.Abort, Check: "record/admission",
			Subject: sub.Subject, Reason: err.Error(),
			Remedy: "this binary is broken; the entry was not recorded",
		}}
	}

	recs, err := s.AppendAll([]store.Draft{
		content,
		{
			Kind:    store.Evidence,
			Type:    "admission",
			Subject: sub.Subject,
			Body:    string(body),
			Party:   sub.Party,
		},
	})
	if err != nil {
		return nil, []outcome.Refusal{{
			Class: outcome.Abort, Check: "record/store",
			Subject: sub.Subject, Reason: err.Error(),
			Remedy: "nothing was recorded; the ledger is unchanged — retry once the store is reachable",
		}}
	}
	return recs, nil
}

// Declaration is the entry event for a declared amendment draft: what
// was proposed, by whom, and against which law it was proposed. It
// confers no standing — a declaration settles that a party proposed a
// change, never that the change holds.
type Declaration struct {
	// The act that produced this entry. A declaration and an ordinary
	// submitted claim are both claims in the ledger; without this, the
	// first consumer that must find declared sets — ratify — would have
	// to infer the difference from which checks ran.
	Act         string   `json:"act"`
	Subject     string   `json:"subject"`
	Party       string   `json:"party"`
	ContentHash string   `json:"content_hash"`
	ChecksRun   []string `json:"checks_run"`
	// The law this proposal was measured against, by record hash. Empty
	// when the project holds no ratified law, which is a first
	// declaration rather than an amendment.
	AgainstLaw string `json:"against_law,omitempty"`
}

// Declare performs the record act over a proposed goal set: the gates
// check it, and if they pass, the proposal is recorded as a claim.
//
// Nothing here makes law. A declaration is a party saying what the law
// should become, entered so that it can be adjudicated later; standing
// arises only by the accountable authority's ratify act. The proposal is
// recorded evidence-side for exactly that reason.
func Declare(s *store.Store, path, party string) ([]store.Record, []outcome.Refusal) {
	law, err := CurrentLaw(s)
	if err != nil {
		return nil, []outcome.Refusal{{
			Class: outcome.Abort, Check: "declare/read",
			Subject: "the ledger", Reason: err.Error(),
			Remedy: "nothing was recorded; the ledger is unchanged — retry once the store is readable",
		}}
	}

	// The subject this ledger already carries: ratified law when there
	// is any, otherwise the subject of the last declaration. Without the
	// fallback the check could never fire, because nothing ratifies yet
	// — and one ledger would quietly accumulate proposals about
	// different subjects.
	subject := ""
	againstLaw := ""
	switch {
	case law != nil:
		subject, againstLaw = law.Subject, law.Hash
	default:
		prior, err := lastDeclaredSubject(s)
		if err != nil {
			return nil, []outcome.Refusal{{
				Class: outcome.Abort, Check: "declare/read",
				Subject: "the ledger", Reason: err.Error(),
				Remedy: "nothing was recorded; the ledger is unchanged — retry once the store is readable",
			}}
		}
		subject = prior
	}

	// Read once, gate the bytes, record the same bytes. Gating a path and
	// reading it again leaves a window in which the file changes, so what
	// passed the gates would not be what enters the ledger.
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, []outcome.Refusal{{
			Class: outcome.Abort, Check: "declare/read",
			Subject: path, Reason: err.Error(),
			Remedy: "point the act at a readable set file; nothing was recorded",
		}}
	}

	set, ran, refusals := gate.ValidateDeclaration(gate.Declaration{
		Name: path, Body: body, Party: party, Subject: subject,
	})
	if len(refusals) > 0 {
		return nil, refusals
	}
	proposedSubject, _ := set.LookupPath(cue.ParsePath("subject")).String()

	content := store.Draft{
		Kind:    store.Evidence,
		Type:    "claim",
		Subject: proposedSubject,
		Body:    string(body),
		Party:   party,
	}

	decl := Declaration{
		Act:        "declare",
		Subject:    proposedSubject,
		Party:      party,
		ChecksRun:  ran,
		AgainstLaw: againstLaw,
	}
	decl.ContentHash = store.ContentHash(content)
	admission, err := json.Marshal(decl)
	if err != nil {
		return nil, []outcome.Refusal{{
			Class: outcome.Abort, Check: "declare/admission",
			Subject: proposedSubject, Reason: err.Error(),
			Remedy: "this binary is broken; the proposal was not recorded",
		}}
	}

	recs, err := s.AppendAll([]store.Draft{
		content,
		{Kind: store.Evidence, Type: "admission", Subject: proposedSubject, Body: string(admission), Party: party},
	})
	if err != nil {
		return nil, []outcome.Refusal{{
			Class: outcome.Abort, Check: "declare/store",
			Subject: proposedSubject, Reason: err.Error(),
			Remedy: "nothing was recorded; the ledger is unchanged — retry once the store is reachable",
		}}
	}
	return recs, nil
}

// lastDeclaredSubject is what this ledger has been about so far, read
// from the most recent declaration. Empty when none has been made, which
// is the first declaration and settles the subject.
func lastDeclaredSubject(s *store.Store) (string, error) {
	recs, err := s.Records()
	if err != nil {
		return "", err
	}
	for i := len(recs) - 1; i >= 0; i-- {
		if recs[i].Type != "admission" {
			continue
		}
		var d Declaration
		if json.Unmarshal([]byte(recs[i].Body), &d) == nil && d.Act == "declare" {
			return d.Subject, nil
		}
	}
	return "", nil
}

// CurrentLaw returns the project's live law, or nil when none has been
// ratified. Law is law-side and arises only from a ratifying act, so an
// unratified project has none — which is the correct answer, not a
// missing value: until then every proposal is a first declaration and
// nothing binds.
//
// A version record is what ratification writes — "the live version is
// the one the ratification's recorded act names" — and the live one is
// the most recent, because the chain is append-only: order in the ledger
// is the order acts happened.
func CurrentLaw(s *store.Store) (*store.Record, error) {
	return s.LatestOf(store.Law, "version")
}

// Verify re-checks the whole ledger: every hash recomputed, every link
// followed. Verification here means one thing only — the record being
// read is the record that was written. A break is a finding: a
// verification path commits nothing, and an item re-verification cannot
// process is never skipped or quarantined.
func Verify(s *store.Store) (int, *outcome.Refusal) {
	n, err := s.Verify()
	if err != nil {
		return 0, &outcome.Refusal{
			Class: outcome.Finding, Check: "verify/chain",
			Subject: "the ledger", Reason: err.Error(),
			Remedy: "the ledger no longer re-hashes to what was recorded; treat every record after the break as unevidenced and investigate the custody of this state root",
		}
	}
	return n, nil
}

// Export renders the ledger as JSON: a read path over recorded facts,
// deriving nothing and deciding nothing.
func Export(s *store.Store) (string, *outcome.Refusal) {
	recs, err := s.Records()
	if err != nil {
		return "", &outcome.Refusal{
			Class: outcome.Abort, Check: "export/read",
			Subject: "the ledger", Reason: err.Error(),
			Remedy: "the ledger could not be read; nothing was exported",
		}
	}
	if recs == nil {
		return "[]\n", nil
	}
	out, err := json.MarshalIndent(recs, "", "  ")
	if err != nil {
		return "", &outcome.Refusal{
			Class: outcome.Abort, Check: "export/render",
			Subject: "the ledger", Reason: err.Error(),
			Remedy: "this binary is broken; do not consume its output",
		}
	}
	return fmt.Sprintf("%s\n", out), nil
}
