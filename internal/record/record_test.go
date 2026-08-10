package record

import (
	"database/sql"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/xormania/looplaw/internal/gate"
	"github.com/xormania/looplaw/internal/outcome"
	"github.com/xormania/looplaw/internal/store"
)

func open(t *testing.T) *store.Store {
	t.Helper()
	s, _ := openAt(t)
	return s
}

func openAt(t *testing.T) (*store.Store, string) {
	t.Helper()
	dir := t.TempDir()
	s, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s, dir
}

// tamper corrupts the ledger from outside, the way anything with the
// file would: the product exposes no way to rewrite a recorded fact, so
// the test does not get one either.
func tamper(t *testing.T, dir string, seq int64, body string) {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(dir, "looplaw.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec("UPDATE records SET body = ? WHERE seq = ?", body, seq); err != nil {
		t.Fatal(err)
	}
}

func goodClaim() gate.Submission {
	return gate.Submission{Kind: "claim", Subject: "scope-x", Party: "absorber:test", Body: `{"states":"a contract exists"}`}
}

func goodReceipt() gate.Submission {
	body, _ := json.Marshal(gate.Receipt{
		Subject: "C-LEND-1", Verdict: "pass", Source: "ci",
		Hash: strings.Repeat("a", 64),
	})
	return gate.Submission{Kind: "receipt", Subject: "C-LEND-1", Party: "ci:test", Body: string(body)}
}

// Every submission lands with its admission, and the admission names
// what entered: a record whose arrival nobody can reconstruct is a
// record with no entry provenance.
func TestSubmitRecordsContentWithItsAdmission(t *testing.T) {
	s := open(t)
	recs, refusals := Submit(s, goodClaim())
	if len(refusals) != 0 {
		t.Fatalf("refused: %+v", refusals)
	}
	if len(recs) != 2 || recs[0].Type != "claim" || recs[1].Type != "admission" {
		t.Fatalf("want a claim and its admission, got %+v", recs)
	}
	if recs[0].At != recs[1].At {
		t.Error("records committed together must carry one timestamp — the ledger shows one act")
	}

	var adm Admission
	if err := json.Unmarshal([]byte(recs[1].Body), &adm); err != nil {
		t.Fatal(err)
	}
	if adm.Party != "absorber:test" || adm.Kind != "claim" || adm.Subject != "scope-x" {
		t.Errorf("admission does not name the entry: %+v", adm)
	}
	if adm.ContentHash == "" || len(adm.ChecksRun) == 0 {
		t.Errorf("admission states neither what entered nor which checks passed: %+v", adm)
	}

	if n, refusal := Verify(s); refusal != nil || n != 2 {
		t.Fatalf("chain after submit: n=%d refusal=%v", n, refusal)
	}
}

// A refused submission leaves no trace: the gates are mechanism and
// originate nothing, so a refusal must not half-record.
func TestRefusedSubmissionRecordsNothing(t *testing.T) {
	s := open(t)
	bad := goodClaim()
	bad.Party = ""
	if _, refusals := Submit(s, bad); len(refusals) == 0 {
		t.Fatal("an unattributed submission was recorded")
	}
	recs, err := s.Records()
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 0 {
		t.Fatalf("a refused submission left records behind: %+v", recs)
	}
}

// Every submission check has a proving red naming the offered thing.
func TestSubmissionChecksAreRedForTheirDeclaredReason(t *testing.T) {
	cases := []struct {
		name      string
		mutate    func(*gate.Submission)
		wantCheck string
	}{
		{"unknown-kind", func(s *gate.Submission) { s.Kind = "rumor" }, "submit/kind"},
		{"system-produced-kind", func(s *gate.Submission) { s.Kind = "admission" }, "submit/kind"},
		{"no-subject", func(s *gate.Submission) { s.Subject = "" }, "submit/subject"},
		{"no-party", func(s *gate.Submission) { s.Party = "" }, "submit/party"},
		{"no-content", func(s *gate.Submission) { s.Body = "   " }, "submit/content"},
		{"receipt-unreadable", func(s *gate.Submission) {
			*s = goodReceipt()
			s.Body = "not json"
		}, "submit/receipt-shape"},
		{"receipt-incomplete", func(s *gate.Submission) {
			*s = goodReceipt()
			s.Body = `{"subject":"x"}`
		}, "submit/receipt-shape"},
		{"receipt-bad-digest", func(s *gate.Submission) {
			*s = goodReceipt()
			s.Body = `{"subject":"x","verdict":"pass","source":"ci","hash":"nope"}`
		}, "submit/receipt-shape"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := open(t)
			sub := goodClaim()
			c.mutate(&sub)
			recs, refusals := Submit(s, sub)
			if len(recs) != 0 {
				t.Fatalf("a refused submission recorded something: %+v", recs)
			}
			found := false
			for _, r := range refusals {
				if r.Check == c.wantCheck {
					found = true
				}
				if r.Class != outcome.Rejection {
					t.Errorf("class = %s, want rejection", r.Class)
				}
				if r.Remedy == "" {
					t.Errorf("refusal without a remedy: %s", r.Error())
				}
			}
			if !found {
				t.Errorf("red, but not for the declared reason: want %s, got %+v", c.wantCheck, refusals)
			}
		})
	}
}

func TestEverySubmissionCheckHasAProvingRed(t *testing.T) {
	proven := map[string]bool{}
	for _, c := range []string{"submit/kind", "submit/subject", "submit/party", "submit/content", "submit/receipt-shape"} {
		proven[c] = true // TestSubmissionChecksAreRedForTheirDeclaredReason
	}
	for _, check := range gate.SubmissionChecks {
		if !proven[check] {
			t.Errorf("check %s has no proving red", check)
		}
	}
}

// A tampered ledger is a finding, never a silent pass: a verification
// path commits nothing, and what re-verification cannot process is
// never skipped.
func TestTamperedLedgerIsAFinding(t *testing.T) {
	s, dir := openAt(t)
	if _, refusals := Submit(s, goodReceipt()); len(refusals) != 0 {
		t.Fatalf("refused: %+v", refusals)
	}
	tamper(t, dir, 1, "forged")
	n, refusal := Verify(s)
	if refusal == nil {
		t.Fatal("a tampered ledger verified clean")
	}
	if refusal.Class != outcome.Finding || refusal.Check != "verify/chain" {
		t.Errorf("want a verify/chain finding, got %s %s", refusal.Class, refusal.Check)
	}
	if n != 0 {
		t.Errorf("a broken chain reported %d verified records", n)
	}
}

func TestExportEmptyLedgerIsAnEmptyList(t *testing.T) {
	s := open(t)
	out, refusal := Export(s)
	if refusal != nil {
		t.Fatal(refusal.Error())
	}
	if strings.TrimSpace(out) != "[]" {
		t.Errorf("empty ledger exported as %q", out)
	}
}
