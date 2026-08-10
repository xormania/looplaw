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

	"github.com/xormania/looplaw/internal/gate"
	"github.com/xormania/looplaw/internal/outcome"
	"github.com/xormania/looplaw/internal/store"
)

// Admission is the entry event, recorded: it names the submitter, what
// was submitted, and the checks the submission passed. It confers no
// standing — it settles that the entry happened, never that the content
// is true.
type Admission struct {
	Kind        string   `json:"kind"`
	Subject     string   `json:"subject"`
	Actor       string   `json:"actor"`
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
		Actor:   sub.Actor,
	}

	adm := Admission{
		Kind:      sub.Kind,
		Subject:   sub.Subject,
		Actor:     sub.Actor,
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
			Actor:   sub.Actor,
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
