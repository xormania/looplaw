package record

import (
	"encoding/json"
	"fmt"

	"github.com/xormania/looplaw/internal/gate"
	"github.com/xormania/looplaw/internal/outcome"
	"github.com/xormania/looplaw/internal/store"
)

// AuthorityBinding names the party a deployment records as holding its
// accountable authority. It is recorded as a claim, and calling it that
// is not modesty — it is what it is.
//
// Nothing can confer standing on this binding, because the party whose
// act would confer it is the one being named. That is the bootstrap, and
// the honest response is to record the claim, note who made it and when,
// and let the chain make it tamper-evident. looplaw checks no identity
// and asserts none; what it can do is refuse to let the answer change
// quietly.
//
// First binding holds. A second is refused rather than layered, so a
// deployment cannot acquire a new accountable authority by asserting
// one — changing it is an act of the authority already on record, which
// is not built yet and is a naming gap flagged rather than coined.
type AuthorityBinding struct {
	Act   string `json:"act"`
	Party string `json:"party"`
	// Bound is the party claimed to hold the accountable authority.
	Bound string `json:"bound"`
}

// AuthorityAdmission is the entry event for the binding act: it names
// the act and cites the claim that carried it, by content hash.
//
// It is what tells a binding from a claim that looks like one. Submit is
// a party's verb and binding is an act, but the binding was recorded as
// a lone claim — so record type, subject and body were the whole of the
// difference, and a submitter chooses all three. The gates refuse a
// submitted admission (gate's submittable kinds are claim and receipt),
// so no party can produce this pair.
//
// It carries no copy of the bound party. Two representations of one
// field are two things that can disagree, and the claim is where the
// binding is stated.
type AuthorityAdmission struct {
	Act         string `json:"act"`
	Party       string `json:"party"`
	ContentHash string `json:"content_hash"`
}

// BindAuthority records which party holds this deployment's accountable
// authority. The store is the deployment's own ledger, not a project's:
// the authority is one per deployment, so binding it per project would
// let two projects disagree about who may make law.
func BindAuthority(s *store.Store, party, bound string) ([]store.Record, *outcome.Refusal) {
	if !gate.IsName(party) {
		return nil, &outcome.Refusal{
			Class: outcome.Rejection, Check: "authority/claimant", Subject: "party",
			Reason: "the binding names no claiming party",
			Remedy: "name the claiming party (LOOPLAW_PARTY); recording settles that a party said a thing, which is unstatable without the party",
		}
	}
	// Held to the grammar every recorded name is held to, not merely to
	// being non-empty. First binding holds and looplaw names no act that
	// changes one, so a party nobody can type again is bound for good:
	// no act ratifies afterwards, and the honest binding is refused as
	// already-bound.
	if !gate.IsName(bound) {
		return nil, &outcome.Refusal{
			Class: outcome.Rejection, Check: "authority/party", Subject: "authority",
			Reason: fmt.Sprintf("%q does not name a party to hold the accountable authority", bound),
			Remedy: "name the party that holds it; an unnamed authority binds nothing, and this binding is the one act that cannot be corrected afterwards",
		}
	}

	existing, err := CurrentAuthority(s)
	if err != nil {
		return nil, &outcome.Refusal{
			Class: outcome.Abort, Check: "authority/read", Subject: "the deployment ledger",
			Reason: err.Error(), Remedy: "nothing was recorded; retry once the ledger is readable",
		}
	}
	if existing != "" {
		return nil, &outcome.Refusal{
			Class: outcome.Rejection, Check: "authority/bound", Subject: existing,
			Reason: fmt.Sprintf("this deployment already records %q as its accountable authority", existing),
			Remedy: "the binding is not replaced by asserting another; changing it is an act of the authority on record, which looplaw does not name yet — flag the naming gap and leave the binding alone",
		}
	}

	body, err := json.Marshal(AuthorityBinding{Act: "bind-authority", Party: party, Bound: bound})
	if err != nil {
		return nil, &outcome.Refusal{
			Class: outcome.Abort, Check: "authority/record", Subject: bound,
			Reason: err.Error(), Remedy: "this binary is broken; nothing was recorded",
		}
	}

	// Content and its admission enter together, which the Ledger contract
	// already requires of every entry and this act alone did not do. Here
	// it is also what makes the act an act: the claim states the binding,
	// the admission records that binding it happened, and only this path
	// can write the second.
	claim := store.Draft{
		Kind: store.Evidence, Type: "claim", Subject: "accountable-authority",
		Body: string(body), Party: party,
	}
	entry, err := json.Marshal(AuthorityAdmission{
		Act: "bind-authority", Party: party, ContentHash: store.ContentHash(claim),
	})
	if err != nil {
		return nil, &outcome.Refusal{
			Class: outcome.Abort, Check: "authority/record", Subject: bound,
			Reason: err.Error(), Remedy: "this binary is broken; nothing was recorded",
		}
	}

	recs, err := s.AppendAll([]store.Draft{
		claim,
		{
			Kind: store.Evidence, Type: "admission", Subject: "accountable-authority",
			Body: string(entry), Party: party,
		},
	})
	if err != nil {
		return nil, &outcome.Refusal{
			Class: outcome.Abort, Check: "authority/store", Subject: bound,
			Reason: err.Error(), Remedy: "nothing was recorded; the ledger is unchanged",
		}
	}
	return recs, nil
}

// CurrentAuthority is the party this deployment records as its
// accountable authority, empty when none is bound.
//
// The first binding holds, so this reads forward rather than backward: a
// later act cannot displace an earlier one, and reading backward would
// let it.
//
// It reads the admission, not the claim. A claim is what any party may
// submit, so a lone claim carrying the right subject and body is a
// submission that resembles an act — and reading it as one is how a
// party bound the authority to itself. The admission names the content
// that entered, so the claim is found by content hash rather than
// assumed to be the record before: that assumption is true today and
// false the moment two acts interleave.
func CurrentAuthority(s *store.Store) (string, error) {
	recs, err := s.Records()
	if err != nil {
		return "", err
	}
	for _, r := range recs {
		if r.Type != "admission" || r.Subject != "accountable-authority" {
			continue
		}
		var entry AuthorityAdmission
		if json.Unmarshal([]byte(r.Body), &entry) != nil || entry.Act != "bind-authority" {
			continue
		}
		for _, c := range recs {
			if c.Type != "claim" {
				continue
			}
			if store.ContentHash(store.Draft{
				Kind: c.Kind, Type: c.Type, Subject: c.Subject, Body: c.Body, Party: c.Party,
			}) != entry.ContentHash {
				continue
			}
			var b AuthorityBinding
			if json.Unmarshal([]byte(c.Body), &b) == nil && b.Act == "bind-authority" {
				return b.Bound, nil
			}
		}
	}
	return "", nil
}
