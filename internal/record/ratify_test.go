package record

import (
	"encoding/json"
	"fmt"
	"os"
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

// Proving red: ratification makes the recorded draft's own bytes law,
// and those bytes were gated by whatever binary was running when they
// were declared. Nothing gated them again, so law entered that the
// binary making it would refuse — proven with two binaries whose
// embedded schemas differed by one constraint: the stricter one refused
// the set, ratified it anyway at exit 0, and then refused the law it had
// just made.
//
// The draft here is entered the way the store holds one rather than
// through Declare, because that is what a ledger looks like after the
// embedded law tightens: the bytes are already recorded, and the act
// that recorded them ran under gates that admitted them. A default is
// the concrete case — the gates admitted it until the batch that
// refused open values, and every ledger declared before that one holds
// drafts like this.
func TestRatificationRegatesTheRecordedDraft(t *testing.T) {
	d, p := declareStore(t), declareStore(t)
	if _, refusal := BindAuthority(d, "xor", "xor"); refusal != nil {
		t.Fatal(refusal)
	}

	body, err := os.ReadFile("../gate/testdata/attacks/defaulted-subject.cue")
	if err != nil {
		t.Fatal(err)
	}
	if refusals := gate.ValidateTrinity("../gate/testdata/attacks/defaulted-subject.cue"); len(refusals) == 0 {
		t.Fatal("the draft passes this binary's gates, so ratifying it proves nothing")
	}

	content := store.Draft{
		Kind: store.Evidence, Type: "claim", Subject: "lend-library",
		Body: string(body), Party: "harness:worker",
	}
	entry, err := json.Marshal(Declaration{
		Act: "declare", Subject: "lend-library", Party: "harness:worker",
		ContentHash: store.ContentHash(content),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.AppendAll([]store.Draft{content, {
		Kind: store.Evidence, Type: "admission", Subject: "lend-library",
		Body: string(entry), Party: "harness:worker",
	}}); err != nil {
		t.Fatal(err)
	}

	recs, refusals := Ratify(d, p, "lend-library", "xor")
	if len(refusals) == 0 {
		t.Fatal("a draft this binary refuses became its law")
	}
	if len(recs) > 0 {
		t.Error("a refused ratification recorded something")
	}
	// A red for the wrong reason proves nothing: the refusal must be the
	// gate's own, naming what is wrong with the bytes.
	named := false
	for _, r := range refusals {
		if r.Check == "trinity/open-value" {
			named = true
		}
		if r.Remedy == "" {
			t.Errorf("refusal without a remedy: %s", r.Error())
		}
	}
	if !named {
		t.Errorf("refused, but not by the gate that refuses these bytes: %v", refusals)
	}
	if law, err := CurrentLaw(p); err != nil || law != nil {
		t.Errorf("law entered from a refused draft: %+v %v", law, err)
	}
}

// Proving red: a party name is what every later act compares against,
// so a binding must name one. `bound` was checked only against the
// empty string, and the claiming party only against being blank after
// trimming — so a deployment could bind "   " as its accountable
// authority. First binding holds and looplaw names no act that changes
// one, so that is permanent: no party anyone can type ratifies again,
// and the honest binding is refused as already-bound.
func TestAuthorityMustNameAParty(t *testing.T) {
	for _, name := range []string{"   ", "\t", " xor", "xor\n", ""} {
		t.Run(fmt.Sprintf("bound=%q", name), func(t *testing.T) {
			d := declareStore(t)
			if _, refusal := BindAuthority(d, "xor", name); refusal == nil {
				t.Fatalf("%q was recorded as the accountable authority", name)
			} else if refusal.Check != "authority/party" {
				t.Errorf("want authority/party, got %s", refusal.Check)
			}
			// Nothing was bound, so the deployment is not locked out of
			// its own authority: the act that should have worked still
			// does.
			if _, refusal := BindAuthority(d, "xor", "xor"); refusal != nil {
				t.Errorf("a refused binding locked the deployment: %v", refusal)
			}
			if got, _ := CurrentAuthority(d); got != "xor" {
				t.Errorf("CurrentAuthority = %q, want xor", got)
			}
		})
	}

	// The claiming party is held to the same grammar, for the same
	// reason: it is recorded as who said the binding, and a name that
	// cannot be typed again names nobody.
	for _, claimant := range []string{"   ", " xor", "xor\n"} {
		d := declareStore(t)
		if _, refusal := BindAuthority(d, claimant, "xor"); refusal == nil {
			t.Errorf("%q was recorded as the claiming party", claimant)
		} else if refusal.Check != "authority/claimant" {
			t.Errorf("want authority/claimant for %q, got %s", claimant, refusal.Check)
		}
	}
}

// Proving red: the acts read recorded state to decide, and nothing
// checked that what they read was what had been written. Records returns
// rows; the chain is only recomputed when someone asks for a
// verification, which is a separate command nobody is obliged to run.
//
// So an edited row was consumed as law and a new act recorded against it
// — declared against a version whose body had been rewritten, while
// verify reported the ledger broken. A preflight verification by the
// caller does not close it either: that leaves a window between the
// check and the use.
func TestActsRefuseToDecideFromAnUnverifiedLedger(t *testing.T) {
	// A deployment with an authority bound and a project holding a
	// declared draft, with the directories in hand so the ledger can be
	// edited from outside the way anything holding the state file does.
	setup := func(t *testing.T) (*store.Store, string, *store.Store, string) {
		t.Helper()
		d, ddir := storeIn(t)
		p, pdir := storeIn(t)
		if _, refusal := BindAuthority(d, "xor", "xor"); refusal != nil {
			t.Fatal(refusal)
		}
		if _, refusals := Declare(p, fixtureZero, "harness:worker"); len(refusals) > 0 {
			t.Fatal(refusals)
		}
		return d, ddir, p, pdir
	}

	t.Run("declare reads the law it declares against", func(t *testing.T) {
		d, _, p, pdir := setup(t)
		if _, refusals := Ratify(d, p, "lend-library", "xor"); len(refusals) > 0 {
			t.Fatal(refusals)
		}
		tamperBody(t, p, pdir, "version", "lend-library", "seize-library")

		_, refusals := Declare(p, fixtureZero, "harness:worker")
		if len(refusals) == 0 {
			t.Fatal("a declaration was recorded against law that no longer re-hashes")
		}
		if refusals[0].Check != "declare/ledger" {
			t.Errorf("want declare/ledger, got %s: %s", refusals[0].Check, refusals[0].Reason)
		}
		if refusals[0].Class != outcome.Finding {
			t.Errorf("class = %s, want finding", refusals[0].Class)
		}
	})

	t.Run("ratify reads the binding and the draft", func(t *testing.T) {
		d, ddir, p, _ := setup(t)
		tamperBody(t, d, ddir, "claim", `"bound":"xor"`, `"bound":"mallory"`)

		_, refusals := Ratify(d, p, "lend-library", "xor")
		if len(refusals) == 0 {
			t.Fatal("law was made from a deployment ledger that no longer re-hashes")
		}
		if refusals[0].Check != "ratify/ledger" {
			t.Errorf("want ratify/ledger, got %s: %s", refusals[0].Check, refusals[0].Reason)
		}
		if law, _ := CurrentLaw(p); law != nil {
			t.Error("a refused ratification made law")
		}
	})

	t.Run("an intact ledger still decides", func(t *testing.T) {
		d, _, p, _ := setup(t)
		if _, refusals := Ratify(d, p, "lend-library", "xor"); len(refusals) > 0 {
			t.Fatalf("an intact ledger was refused: %v", refusals)
		}
	})
}

// storeIn opens a ledger and hands back where it lives, so a test can
// reach it the way anything outside looplaw would.
func storeIn(t *testing.T) (*store.Store, string) {
	t.Helper()
	dir := t.TempDir()
	s, err := store.OpenDeployment(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s, dir
}

// tamperBody rewrites the body of the first record of a type and leaves
// its hash alone — what anything holding the state file does, and what
// the chain exists to make evident.
func tamperBody(t *testing.T, s *store.Store, dir, rectype, from, to string) {
	t.Helper()
	recs, err := s.Records()
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range recs {
		if r.Type != rectype {
			continue
		}
		edited := strings.Replace(r.Body, from, to, 1)
		// A replacement that changed nothing leaves the chain intact and
		// the test asserting against an untampered ledger, which would
		// pass for the wrong reason.
		if edited == r.Body {
			t.Fatalf("the %s record holds no %q, so nothing was tampered with", rectype, from)
		}
		tamper(t, dir, r.Seq, edited)
		return
	}
	t.Fatalf("no %s record to tamper with", rectype)
}
