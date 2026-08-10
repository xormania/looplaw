package record

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/xormania/looplaw/internal/gate"
	"github.com/xormania/looplaw/internal/store"
)

const (
	fixtureZero  = "../gate/testdata/library/set.cue"
	absorbedView = "../absorb/testdata/view.cue"
	brokenSet    = "../gate/testdata/attacks/self-wire.cue"
)

func declareStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

// A declaration is recorded as a claim, evidence-side, with an admission
// naming what entered. Nothing about it is law: the point of the act is
// that a proposal can be adjudicated later, not that it holds now.
func TestDeclarationIsRecordedAsAClaimAndBindsNothing(t *testing.T) {
	s := declareStore(t)
	recs, refusals := Declare(s, fixtureZero, "harness:test")
	if len(refusals) > 0 {
		t.Fatalf("fixture zero should declare cleanly, got %v", refusals)
	}
	if len(recs) != 2 {
		t.Fatalf("want the proposal and its admission, got %d records", len(recs))
	}
	if recs[0].Type != "goal-proposal" || recs[0].Kind != store.Evidence {
		t.Errorf("a proposal is evidence-side: got kind=%s type=%s", recs[0].Kind, recs[0].Type)
	}
	if recs[1].Type != "admission" {
		t.Errorf("want an admission beside the proposal, got %s", recs[1].Type)
	}

	var decl Declaration
	if err := json.Unmarshal([]byte(recs[1].Body), &decl); err != nil {
		t.Fatal(err)
	}
	if decl.ContentHash != store.ContentHash(store.Draft{
		Kind: store.Evidence, Type: "goal-proposal", Subject: recs[0].Subject,
		Body: recs[0].Body, Party: "harness:test",
	}) {
		t.Error("the admission does not name the content that entered")
	}
	if decl.AgainstLaw != "" {
		t.Errorf("no law is ratified, so a declaration is measured against nothing: got %q", decl.AgainstLaw)
	}

	// Declaring makes no law. Anything else would confer standing by
	// mechanism, which is the thing ratification exists to prevent.
	law, err := CurrentLaw(s)
	if err != nil {
		t.Fatal(err)
	}
	if law != nil {
		t.Errorf("declaring produced law: %+v", law)
	}
}

// Proving red: an absorbed view is evidence of what is. Accepting one as
// a proposal for what should be would let a party's claim about the
// present become the standard the present is judged by.
func TestDeclaringAnAbsorbedViewIsRefused(t *testing.T) {
	s := declareStore(t)
	_, refusals := Declare(s, absorbedView, "harness:test")
	if len(refusals) != 1 || refusals[0].Check != "declare/provenance" {
		t.Fatalf("want declare/provenance, got %v", refusals)
	}
	if !strings.Contains(refusals[0].Reason, "provenance") {
		t.Errorf("the refusal must name what was wrong: %q", refusals[0].Reason)
	}
	assertNothingRecorded(t, s)
}

// Proving red: recording settles that a party proposed a thing, which is
// unstatable without the party.
func TestDeclarationWithoutAPartyIsRefused(t *testing.T) {
	s := declareStore(t)
	_, refusals := Declare(s, fixtureZero, "")
	if len(refusals) != 1 || refusals[0].Check != "declare/party" {
		t.Fatalf("want declare/party, got %v", refusals)
	}
	assertNothingRecorded(t, s)
}

// Proving red: a project's law is about one subject. A proposal naming a
// different one is the wrong project or an undeclared rename, and both
// beat silently forking a second subject into one ledger.
func TestDeclarationAgainstDifferentSubjectIsRefused(t *testing.T) {
	s := declareStore(t)

	// Ratification does not exist yet, so the law record is written
	// directly: this test is about the gate, not about how law arrives.
	body, err := os.ReadFile(fixtureZero)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Append(store.Law, "law-set", "other-project", string(body), "harness:test"); err != nil {
		t.Fatal(err)
	}

	_, refusals := Declare(s, fixtureZero, "harness:test")
	if len(refusals) != 1 || refusals[0].Check != "declare/subject-mismatch" {
		t.Fatalf("want declare/subject-mismatch, got %v", refusals)
	}
	if !strings.Contains(refusals[0].Reason, "other-project") {
		t.Errorf("the refusal must name both subjects: %q", refusals[0].Reason)
	}
}

// A set that cannot pass the trinity gates cannot become law, and the
// refusals are the worklist. They arrive as themselves — rewrapping them
// would restate a remedy beside the precise one.
func TestABrokenProposalReturnsTheTrinityRefusalsUnwrapped(t *testing.T) {
	s := declareStore(t)
	_, refusals := Declare(s, brokenSet, "harness:test")
	if len(refusals) == 0 {
		t.Fatal("a set failing the gates must be refused")
	}
	for _, r := range refusals {
		if !strings.HasPrefix(r.Check, "trinity/") {
			t.Errorf("want the trinity check itself, got %s", r.Check)
		}
		if strings.Count(r.Error(), "remedy:") != 1 {
			t.Errorf("a refusal carries one remedy, not two: %s", r.Error())
		}
	}
	assertNothingRecorded(t, s)
}

// Once law exists, a declaration is measured against it and says so.
func TestDeclarationNamesTheLawItWasMeasuredAgainst(t *testing.T) {
	s := declareStore(t)
	body, err := os.ReadFile(fixtureZero)
	if err != nil {
		t.Fatal(err)
	}
	lawRec, err := s.Append(store.Law, "law-set", "lend-library", string(body), "harness:test")
	if err != nil {
		t.Fatal(err)
	}

	recs, refusals := Declare(s, fixtureZero, "harness:test")
	if len(refusals) > 0 {
		t.Fatalf("declaring against matching law should pass, got %v", refusals)
	}
	var decl Declaration
	if err := json.Unmarshal([]byte(recs[1].Body), &decl); err != nil {
		t.Fatal(err)
	}
	if decl.AgainstLaw != lawRec.Hash {
		t.Errorf("want the law's record hash %s, got %q", lawRec.Hash, decl.AgainstLaw)
	}
}

// CurrentLaw reads the live law, and the live one is the most recent:
// the chain is append-only, so ledger order is the order acts happened.
func TestCurrentLawIsTheMostRecentRatifiedSet(t *testing.T) {
	s := declareStore(t)
	if law, err := CurrentLaw(s); err != nil || law != nil {
		t.Fatalf("an unratified project holds no law: got %v %v", law, err)
	}
	for _, subject := range []string{"first", "second"} {
		if _, err := s.Append(store.Law, "law-set", subject, "{}", "harness:test"); err != nil {
			t.Fatal(err)
		}
	}
	// Evidence recorded after the law must not be mistaken for it.
	if _, err := s.Append(store.Evidence, "claim", "noise", "{}", "harness:test"); err != nil {
		t.Fatal(err)
	}
	law, err := CurrentLaw(s)
	if err != nil {
		t.Fatal(err)
	}
	if law == nil || law.Subject != "second" {
		t.Errorf("want the most recent law-set, got %+v", law)
	}
}

// Every check the declaration gates can emit has a proving red. A check
// added to DeclarationChecks without one fails here.
func TestEveryDeclarationGateHasAProvingRed(t *testing.T) {
	proven := map[string]bool{
		"declare/provenance":       true, // TestDeclaringAnAbsorbedViewIsRefused
		"declare/party":            true, // TestDeclarationWithoutAPartyIsRefused
		"declare/subject-mismatch": true, // TestDeclarationAgainstDifferentSubjectIsRefused
	}
	for _, check := range gate.DeclarationChecks {
		if !proven[check] {
			t.Errorf("gate %s has no proving red — an undemonstrated gate is an unproven behavior", check)
		}
	}
	for check := range proven {
		found := false
		for _, c := range gate.DeclarationChecks {
			if c == check {
				found = true
			}
		}
		if !found {
			t.Errorf("%s is claimed proven but is not in DeclarationChecks — delete the stale entry", check)
		}
	}
}

func assertNothingRecorded(t *testing.T, s *store.Store) {
	t.Helper()
	recs, err := s.Records()
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range recs {
		if r.Type == "goal-proposal" || r.Type == "admission" {
			t.Errorf("a refused declaration left a trace: seq %d %s", r.Seq, r.Type)
		}
	}
}
