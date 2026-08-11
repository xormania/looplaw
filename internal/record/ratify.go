package record

import (
	"encoding/json"

	"github.com/xormania/looplaw/internal/gate"
	"github.com/xormania/looplaw/internal/outcome"
	"github.com/xormania/looplaw/internal/store"
)

// Ratification is the entry event for the act that makes law: what was
// ratified, by whom, and which declaration it acted on.
//
// The version record beside it is the law. This names the act.
type Ratification struct {
	Act string `json:"act"`
	// Party is the claimed performer, recorded as claimed.
	Party   string `json:"party"`
	Subject string `json:"subject"`
	// Draft is the record hash of the declaration that became law, so
	// what was ratified can be read back rather than inferred from
	// timing.
	Draft     string   `json:"draft"`
	ChecksRun []string `json:"checks_run"`
}

// Ratify performs the accountable authority's act: a declared draft
// becomes law.
//
// The gates verify preconditions and refuse with remedy; they confer
// nothing. Standing comes from this act being recorded, which is why the
// version record is law-side while everything the act consumed is
// evidence.
//
// deployment holds the authority binding; project holds the draft and
// receives the law. They are separate ledgers because the accountable
// authority is one per deployment while law is per project — binding it
// per project would let two projects disagree about who may make law.
func Ratify(deployment, project *store.Store, subject, party string) ([]store.Record, []outcome.Refusal) {
	abort := func(check, reason string) []outcome.Refusal {
		return []outcome.Refusal{{
			Class: outcome.Abort, Check: check, Subject: subject,
			Reason: reason,
			Remedy: "nothing was recorded; the ledger is unchanged — retry once it is readable",
		}}
	}

	authority, err := CurrentAuthority(deployment)
	if err != nil {
		return nil, abort("ratify/read", err.Error())
	}

	draft, err := latestDeclaration(project, subject)
	if err != nil {
		return nil, abort("ratify/read", err.Error())
	}
	law, err := CurrentLaw(project)
	if err != nil {
		return nil, abort("ratify/read", err.Error())
	}

	if refusals := gate.ValidateRatification(gate.Ratification{
		Party:       party,
		Authority:   authority,
		Subject:     subject,
		Declared:    draft != nil,
		HasStanding: law != nil && law.Subject == subject,
	}); len(refusals) > 0 {
		return nil, refusals
	}

	// The law is the declared draft's own content. Ratification changes
	// nothing about what was proposed — it changes what the proposal is:
	// the same bytes, now law-side. A version that differed from the
	// draft would be law nobody declared.
	version := store.Draft{
		Kind: store.Law, Type: "version", Subject: subject,
		Body: draft.Body, Party: party,
	}

	entry := Ratification{
		Act: "ratify", Party: party, Subject: subject,
		Draft: draft.Hash, ChecksRun: gate.RatificationChecks,
	}
	body, err := json.Marshal(entry)
	if err != nil {
		return nil, abort("ratify/record", err.Error())
	}

	recs, err := project.AppendAll([]store.Draft{
		version,
		{Kind: store.Law, Type: "admission", Subject: subject, Body: string(body), Party: party},
	})
	if err != nil {
		return nil, abort("ratify/store", err.Error())
	}
	return recs, nil
}

// latestDeclaration is the most recent declared draft for a subject, or
// nil when none is on record.
//
// Like lastDeclaredSubject this scans rather than asking the ledger, and
// for the same reason: the predicate is body content, which the ledger
// stores opaquely and cannot index.
func latestDeclaration(s *store.Store, subject string) (*store.Record, error) {
	recs, err := s.Records()
	if err != nil {
		return nil, err
	}
	var contentHash string
	for i := len(recs) - 1; i >= 0; i-- {
		if recs[i].Type != "admission" {
			continue
		}
		var d Declaration
		if json.Unmarshal([]byte(recs[i].Body), &d) == nil && d.Act == "declare" && d.Subject == subject {
			contentHash = d.ContentHash
			break
		}
	}
	if contentHash == "" {
		return nil, nil
	}
	// The admission names the content that entered; find it rather than
	// assuming it is the record before, which would be true today and
	// false the moment two acts interleave.
	for i := len(recs) - 1; i >= 0; i-- {
		r := recs[i]
		if r.Type != "claim" {
			continue
		}
		if store.ContentHash(store.Draft{
			Kind: r.Kind, Type: r.Type, Subject: r.Subject, Body: r.Body, Party: r.Party,
		}) == contentHash {
			return &recs[i], nil
		}
	}
	return nil, nil
}
