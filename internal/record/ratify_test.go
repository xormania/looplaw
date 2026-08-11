package record

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/xormania/looplaw/internal/gate"
	"github.com/xormania/looplaw/internal/outcome"
	"github.com/xormania/looplaw/internal/store"
)

// bound returns a deployment ledger with an accountable authority on
// record, and a project ledger holding a declared draft.
func bound(t *testing.T, authority string, declare bool) (*store.Store, *store.Store) {
	t.Helper()
	d, p := declareStore(t), declareStore(t)
	if authority != "" {
		if _, refusal := BindAuthority(d, authority, authority); refusal != nil {
			t.Fatal(refusal)
		}
	}
	if declare {
		if _, refusals := Declare(p, fixtureZero, "agent:worker"); len(refusals) > 0 {
			t.Fatal(refusals)
		}
	}
	return d, p
}

// The act makes law, and the law is the draft's own content. A version
// that differed from what was declared would be law nobody proposed.
func TestRatificationMakesTheDeclaredDraftLaw(t *testing.T) {
	d, p := bound(t, "xor", true)

	recs, refusals := Ratify(d, p, "lend-library", "xor")
	if len(refusals) > 0 {
		t.Fatalf("the authority could not ratify: %v", refusals)
	}
	if len(recs) != 2 {
		t.Fatalf("want the version and its admission, got %d", len(recs))
	}
	if recs[0].Kind != store.Law || recs[0].Type != "version" {
		t.Errorf("law is law-side: got kind=%s type=%s", recs[0].Kind, recs[0].Type)
	}

	draft, err := latestDeclaration(p, "lend-library")
	if err != nil || draft == nil {
		t.Fatalf("the draft went missing: %v", err)
	}
	if recs[0].Body != draft.Body {
		t.Error("the law differs from the draft that was declared")
	}

	// The admission cites which draft became law, so it can be read back
	// rather than inferred from ordering.
	var entry Ratification
	if err := json.Unmarshal([]byte(recs[1].Body), &entry); err != nil {
		t.Fatal(err)
	}
	if entry.Act != "ratify" || entry.Draft != draft.Hash {
		t.Errorf("the admission does not name the act and its draft: %+v", entry)
	}

	// And the project now holds law, which is what declare reads.
	law, err := CurrentLaw(p)
	if err != nil || law == nil {
		t.Fatalf("ratifying produced no law: %v %v", law, err)
	}
}

// Proving red: nothing can be law where no authority is on record. There
// is no party whose act would confer standing, so the question is
// unanswerable rather than merely unmet.
func TestRatifyingWithNoAuthorityBoundIsRefused(t *testing.T) {
	d, p := bound(t, "", true)
	refusals := ratifyRefusals(t, d, p, "lend-library", "xor")
	if refusals[0].Check != "ratify/unbound" {
		t.Fatalf("want ratify/unbound, got %v", refusals)
	}
}

// Proving red: only the accountable authority ratifies, and a party
// cannot confer standing on its own work.
func TestOnlyTheAccountableAuthorityRatifies(t *testing.T) {
	d, p := bound(t, "xor", true)
	refusals := ratifyRefusals(t, d, p, "lend-library", "agent:worker")
	if refusals[0].Check != "ratify/authority" {
		t.Fatalf("want ratify/authority, got %v", refusals)
	}
	// The gates never deny: "denial — the deciding authority declines
	// ... never the gates' judgment". A party that is not the authority
	// is a precondition the gates check, and they refuse with remedy
	// like every other gate.
	if refusals[0].Class != outcome.Rejection {
		t.Errorf("a gate refuses; only the deciding authority denies. Got %v", refusals[0].Class)
	}
	if !strings.Contains(refusals[0].Reason, "agent:worker") {
		t.Errorf("the refusal must name who asked: %q", refusals[0].Reason)
	}
}

// Proving red: ratification acts on a declared draft, never on a file.
func TestRatifyingWithNoDeclarationIsRefused(t *testing.T) {
	d, p := bound(t, "xor", false)
	refusals := ratifyRefusals(t, d, p, "lend-library", "xor")
	if refusals[0].Check != "ratify/target" {
		t.Fatalf("want ratify/target, got %v", refusals)
	}
}

// Proving red: ratification is a draft's first standing. Changing law
// that already holds standing is amend, and conflating them would let
// law be replaced in place rather than superseded.
func TestRatifyingWhatAlreadyHoldsStandingIsRefused(t *testing.T) {
	d, p := bound(t, "xor", true)
	if _, refusals := Ratify(d, p, "lend-library", "xor"); len(refusals) > 0 {
		t.Fatal(refusals)
	}
	refusals := ratifyRefusals(t, d, p, "lend-library", "xor")
	if refusals[0].Check != "ratify/standing" {
		t.Fatalf("want ratify/standing, got %v", refusals)
	}
	if !strings.Contains(refusals[0].Remedy, "amend") {
		t.Errorf("the remedy must name the act that does apply: %q", refusals[0].Remedy)
	}
}

// The binding is recorded as claimed, and the first one holds: a
// deployment cannot acquire a new accountable authority by asserting
// one.
func TestFirstAuthorityBindingHolds(t *testing.T) {
	d := declareStore(t)

	if got, err := CurrentAuthority(d); err != nil || got != "" {
		t.Fatalf("an unbound deployment names no authority: %q %v", got, err)
	}
	if _, refusal := BindAuthority(d, "xor", "xor"); refusal != nil {
		t.Fatal(refusal)
	}
	if got, _ := CurrentAuthority(d); got != "xor" {
		t.Fatalf("the binding did not take: %q", got)
	}

	_, refusal := BindAuthority(d, "agent:worker", "agent:worker")
	if refusal == nil {
		t.Fatal("a second binding displaced the first")
	}
	if refusal.Check != "authority/bound" {
		t.Errorf("want authority/bound, got %s", refusal.Check)
	}
	if got, _ := CurrentAuthority(d); got != "xor" {
		t.Errorf("the authority changed by assertion: %q", got)
	}

	// Recorded as a claim, because nothing can confer standing on it —
	// the party whose act would is the one being named.
	recs, _ := d.Records()
	if recs[0].Kind != store.Evidence || recs[0].Type != "claim" {
		t.Errorf("the binding is a claim, not law: kind=%s type=%s", recs[0].Kind, recs[0].Type)
	}
}

// Proving red: submitting is a party's verb and binding is an act, so a
// submitted claim must never read back as the binding however it is
// shaped. The binding was recorded as a lone claim, and a claim is what
// any party may submit — so record type, subject and body, every one of
// them the submitter's to choose, were the whole of what told them
// apart. A claim carrying {"act":"bind-authority", …} was the binding.
//
// What separates them now is the admission beside the claim: the gates
// refuse a submitted admission (gate.submittableKinds holds claim and
// receipt), so no party can produce the pair, and no lone claim is read.
func TestASubmittedClaimIsNotAnAuthorityBinding(t *testing.T) {
	d := declareStore(t)

	forgery := `{"act":"bind-authority","party":"mallory","bound":"mallory"}`
	if _, refusals := Submit(d, gate.Submission{
		Kind: "claim", Subject: "accountable-authority", Party: "mallory", Body: forgery,
	}); len(refusals) > 0 {
		t.Fatalf("the submission was refused, so the test proves nothing: %v", refusals)
	}

	// Refusing the submission would be the wrong fix: a claim is
	// recorded, never believed, and what is wrong is reading it as an
	// act — not that a party said it.
	if got, err := CurrentAuthority(d); err != nil || got != "" {
		t.Errorf("a submitted claim read back as the accountable authority: %q %v", got, err)
	}

	// The act still binds, over the forgery already on record.
	if _, refusal := BindAuthority(d, "xor", "xor"); refusal != nil {
		t.Fatalf("the act was refused after a party's claim: %v", refusal)
	}
	if got, _ := CurrentAuthority(d); got != "xor" {
		t.Errorf("the binding did not take: %q", got)
	}

	// And the forged claim cannot displace it by being first, either:
	// first binding holds means the first recorded act, not the first
	// record that resembles one.
	recs, _ := d.Records()
	if recs[0].Party != "mallory" {
		t.Fatalf("the forgery is not first in the ledger, so the order is untested: %+v", recs[0])
	}
}

// An unnamed authority binds nothing and no act could check against it.
func TestBindingNoPartyIsRefused(t *testing.T) {
	d := declareStore(t)
	if _, refusal := BindAuthority(d, "xor", ""); refusal == nil {
		t.Fatal("an empty authority was recorded")
	}
	// And a binding nobody is recorded as claiming: recording settles
	// that a party said a thing, which is unstatable without the party.
	r, refusal := BindAuthority(d, "", "xor")
	if refusal == nil {
		t.Fatal("a binding with no claiming party was recorded")
	}
	if refusal.Check != "authority/claimant" {
		t.Errorf("want authority/claimant, got %s", refusal.Check)
	}
	if r != nil {
		t.Error("a refused binding returned a record")
	}
	if got, _ := CurrentAuthority(d); got != "" {
		t.Errorf("a refused binding took effect: %q", got)
	}
}

// The credit is the red itself, not a note about one: each entry holds
// the function that proves its check, so this runs the red rather than
// trusting a comment. Delete the red and this stops compiling.
// Every check the ratification gates can emit has a proving red.
func TestEveryRatificationGateHasAProvingRed(t *testing.T) {
	proven := map[string]bool{}
	for _, red := range []struct {
		check string
		run   func(*testing.T)
	}{
		{"ratify/unbound", TestRatifyingWithNoAuthorityBoundIsRefused},
		{"ratify/authority", TestOnlyTheAccountableAuthorityRatifies},
		{"ratify/target", TestRatifyingWithNoDeclarationIsRefused},
		{"ratify/standing", TestRatifyingWhatAlreadyHoldsStandingIsRefused},
	} {
		if !t.Run("proving "+red.check, red.run) {
			t.Errorf("the red for %s did not pass, so it proves nothing", red.check)
			continue
		}
		proven[red.check] = true
	}
	for _, check := range gate.RatificationChecks {
		if !proven[check] {
			t.Errorf("gate %s has no proving red — an undemonstrated gate is an unproven behavior", check)
		}
	}
	for check := range proven {
		found := false
		for _, c := range gate.RatificationChecks {
			if c == check {
				found = true
			}
		}
		if !found {
			t.Errorf("%s is claimed proven but is not in RatificationChecks — delete the stale entry", check)
		}
	}
}

func ratifyRefusals(t *testing.T, d, p *store.Store, subject, party string) []outcome.Refusal {
	t.Helper()
	recs, refusals := Ratify(d, p, subject, party)
	if len(refusals) == 0 {
		t.Fatal("the act was not refused")
	}
	if len(recs) > 0 {
		t.Error("a refused ratification recorded something")
	}
	return refusals
}
