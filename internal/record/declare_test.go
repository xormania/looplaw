package record

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
	if recs[0].Type != "claim" || recs[0].Kind != store.Evidence {
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
		Kind: store.Evidence, Type: "claim", Subject: recs[0].Subject,
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
	if _, err := s.Append(store.Law, "version", "other-project", string(body), "harness:test"); err != nil {
		t.Fatal(err)
	}

	_, refusals := Declare(s, fixtureZero, "harness:test")
	if len(refusals) != 1 || refusals[0].Check != "declare/subject-mismatch" {
		t.Fatalf("want declare/subject-mismatch, got %v", refusals)
	}
	if !strings.Contains(refusals[0].Reason, "other-project") {
		t.Errorf("the refusal must name both subjects: %q", refusals[0].Reason)
	}
	assertNothingRecorded(t, s)
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
	lawRec, err := s.Append(store.Law, "version", "lend-library", string(body), "harness:test")
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
		if _, err := s.Append(store.Law, "version", subject, "{}", "harness:test"); err != nil {
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

// The credit is the red itself, not a note about one: each entry holds
// the function that proves its check, so this runs the red rather than
// trusting a comment. Delete the red and this stops compiling.
// Every check the declaration gates can emit has a proving red. A check
// added to DeclarationChecks without one fails here.
func TestEveryDeclarationGateHasAProvingRed(t *testing.T) {
	proven := map[string]bool{}
	for _, red := range []struct {
		check string
		run   func(*testing.T)
	}{
		{"declare/provenance", TestDeclaringAnAbsorbedViewIsRefused},
		{"declare/party", TestDeclarationWithoutAPartyIsRefused},
		{"declare/subject-mismatch", TestDeclarationAgainstDifferentSubjectIsRefused},
	} {
		if !t.Run("proving "+red.check, red.run) {
			t.Errorf("the red for %s did not pass, so it proves nothing", red.check)
			continue
		}
		proven[red.check] = true
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
		if r.Type == "claim" || r.Type == "admission" {
			t.Errorf("a refused declaration left a trace: seq %d %s", r.Seq, r.Type)
		}
	}
}

// The subject check has to be reachable without ratified law, or it can
// never fire: nothing ratifies yet, so a ledger would quietly accumulate
// proposals about different subjects. The first declaration settles what
// this ledger is about.
func TestFirstDeclarationSettlesTheSubject(t *testing.T) {
	s := declareStore(t)
	if _, refusals := Declare(s, fixtureZero, "harness:test"); len(refusals) > 0 {
		t.Fatalf("the first declaration should pass, got %v", refusals)
	}

	src, err := os.ReadFile(fixtureZero)
	if err != nil {
		t.Fatal(err)
	}
	other := filepath.Join(t.TempDir(), "other.cue")
	if err := os.WriteFile(other, bytes.Replace(src,
		[]byte(`subject:        "lend-library"`),
		[]byte(`subject:        "other-thing"`), 1), 0o644); err != nil {
		t.Fatal(err)
	}

	_, refusals := Declare(s, other, "harness:test")
	if len(refusals) != 1 || refusals[0].Check != "declare/subject-mismatch" {
		t.Fatalf("want declare/subject-mismatch against the prior declaration, got %v", refusals)
	}
	if !strings.Contains(refusals[0].Reason, "lend-library") {
		t.Errorf("the refusal must name the subject this ledger carries: %q", refusals[0].Reason)
	}
	// The remedy must not hand a reserved verb to the caller: only the
	// accountable authority amends.
	if strings.Contains(refusals[0].Remedy, "amend the") {
		t.Errorf("the remedy tells the caller to amend: %q", refusals[0].Remedy)
	}
}

// What passed the gates is what enters the ledger. The act reads the
// proposal once and gates those bytes; gating a path and reading it
// again would leave a window in which the file changes.
func TestRecordedProposalIsExactlyWhatWasGated(t *testing.T) {
	s := declareStore(t)
	recs, refusals := Declare(s, fixtureZero, "harness:test")
	if len(refusals) > 0 {
		t.Fatal(refusals)
	}
	onDisk, err := os.ReadFile(fixtureZero)
	if err != nil {
		t.Fatal(err)
	}
	if recs[0].Body != string(onDisk) {
		t.Error("the recorded proposal is not byte-identical to the file that passed the gates")
	}
}

// A declaration and an ordinary submitted claim are both claims. The
// admission says which act produced it, so ratify can find declared sets
// without inferring it from which checks ran.
func TestAdmissionNamesTheActAndTheChecksThatRan(t *testing.T) {
	s := declareStore(t)
	recs, refusals := Declare(s, fixtureZero, "harness:test")
	if len(refusals) > 0 {
		t.Fatal(refusals)
	}
	var decl Declaration
	if err := json.Unmarshal([]byte(recs[1].Body), &decl); err != nil {
		t.Fatal(err)
	}
	if decl.Act != "declare" {
		t.Errorf("the admission does not name the act: %q", decl.Act)
	}
	// Checks that did not run are not recorded as run.
	for _, c := range decl.ChecksRun {
		if c == "declare/subject-mismatch" && len(decl.ChecksRun) < 3 {
			t.Error("a skipped check is recorded as run")
		}
	}
	if len(decl.ChecksRun) == 0 {
		t.Error("the admission records no checks at all")
	}
	// The trinity gates did the bulk of the work; an admission naming
	// only the declare checks would understate what was verified.
	trinity := 0
	for _, c := range decl.ChecksRun {
		if strings.HasPrefix(c, "trinity/") {
			trinity++
		}
	}
	if trinity == 0 {
		t.Error("the admission names none of the gates that actually checked the set")
	}
}

// foreignLedger is storage looplaw did not write: no chain, no hashes of
// its own devising, no file. If an act reaches past the Ledger interface
// for anything — a table, a chain, a path — it fails here rather than on
// the day the storage is swapped.
type foreignLedger struct{ recs []store.Record }

func (f *foreignLedger) Append(drafts []store.Draft) ([]store.Record, error) {
	var staged []store.Record
	for i, d := range drafts {
		seq := int64(len(f.recs) + i + 1)
		staged = append(staged, store.Record{
			Seq: seq, Kind: d.Kind, Type: d.Type, Subject: d.Subject,
			Body: d.Body, Party: d.Party, At: "2026-01-01T00:00:00Z",
			Hash: fmt.Sprintf("foreign:%d", seq),
		})
	}
	f.recs = append(f.recs, staged...)
	return staged, nil
}
func (f *foreignLedger) Records() ([]store.Record, error) { return f.recs, nil }
func (f *foreignLedger) Verify() (int, error)             { return len(f.recs), nil }
func (f *foreignLedger) Close() error                     { return nil }

// The record acts are storage-independent. This is the property the
// Ledger interface exists to hold, so it is asserted rather than assumed.
func TestActsRunOverForeignStorage(t *testing.T) {
	s := store.New(&foreignLedger{})
	defer s.Close()

	recs, refusals := Declare(s, fixtureZero, "harness:test")
	if len(refusals) > 0 {
		t.Fatalf("declare over foreign storage: %v", refusals)
	}
	if len(recs) != 2 || recs[0].Type != "claim" || recs[1].Type != "admission" {
		t.Fatalf("want a claim and its admission, got %+v", recs)
	}

	// The gates still hold: an absorbed view is refused whatever holds
	// the ledger.
	if _, refusals := Declare(s, absorbedView, "harness:test"); len(refusals) != 1 ||
		refusals[0].Check != "declare/provenance" {
		t.Errorf("the provenance gate changed with the storage: %v", refusals)
	}

	// And the subject check, which reads recorded state back.
	if law, err := CurrentLaw(s); err != nil || law != nil {
		t.Errorf("nothing ratified, so no law: %v %v", law, err)
	}
	if _, err := Export(s); err != nil {
		t.Errorf("export over foreign storage: %v", err)
	}
}
