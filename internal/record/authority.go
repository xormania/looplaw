package record

import (
	"encoding/json"
	"fmt"
	"strings"

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

// BindAuthority records which party holds this deployment's accountable
// authority. The store is the deployment's own ledger, not a project's:
// the authority is one per deployment, so binding it per project would
// let two projects disagree about who may make law.
func BindAuthority(s *store.Store, party, bound string) (*store.Record, *outcome.Refusal) {
	if strings.TrimSpace(party) == "" {
		return nil, &outcome.Refusal{
			Class: outcome.Rejection, Check: "authority/claimant", Subject: "party",
			Reason: "the binding names no claiming party",
			Remedy: "name the claiming party (LOOPLAW_PARTY); recording settles that a party said a thing, which is unstatable without the party",
		}
	}
	if bound == "" {
		return nil, &outcome.Refusal{
			Class: outcome.Rejection, Check: "authority/party", Subject: "authority",
			Reason: "no party was named to hold the accountable authority",
			Remedy: "name the party that holds it; an unnamed authority binds nothing and no act could check against it",
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
	rec, err := s.Append(store.Evidence, "claim", "accountable-authority", string(body), party)
	if err != nil {
		return nil, &outcome.Refusal{
			Class: outcome.Abort, Check: "authority/store", Subject: bound,
			Reason: err.Error(), Remedy: "nothing was recorded; the ledger is unchanged",
		}
	}
	return &rec, nil
}

// CurrentAuthority is the party this deployment records as its
// accountable authority, empty when none is bound.
//
// The first binding holds, so this reads forward rather than backward: a
// later claim cannot displace an earlier one, and reading backward would
// let it.
func CurrentAuthority(s *store.Store) (string, error) {
	recs, err := s.Records()
	if err != nil {
		return "", err
	}
	for _, r := range recs {
		if r.Type != "claim" || r.Subject != "accountable-authority" {
			continue
		}
		var b AuthorityBinding
		if json.Unmarshal([]byte(r.Body), &b) == nil && b.Act == "bind-authority" {
			return b.Bound, nil
		}
	}
	return "", nil
}
