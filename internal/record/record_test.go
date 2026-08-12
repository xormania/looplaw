package record

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
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
	s, err := store.OpenDeployment(dir)
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

	if n, _, refusal := Verify(s, ""); refusal != nil || n != 2 {
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
// submissionReds is the one table: the reds run from it, and the
// demonstration-coverage test credits from it. A row deleted takes both
// its red and its credit; a row that stops drawing its declared check
// fails the red. Credit cannot outlive the thing that earns it.
var submissionReds = []struct {
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

func TestSubmissionChecksAreRedForTheirDeclaredReason(t *testing.T) {
	cases := submissionReds

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
	// Two links, because either alone leaves a way for credit to outlive
	// the thing that earns it. The runner is called here, so deleting it
	// stops this compiling; the credit is derived from the table it
	// walks, so deleting a row takes its red and its credit together.
	// The list this replaced was a second copy of the check ids, and
	// deleting the red test left it reporting five checks demonstrated
	// with nothing demonstrating them.
	if !t.Run("running the reds", TestSubmissionChecksAreRedForTheirDeclaredReason) {
		t.Fatal("the submission reds did not pass, so they prove nothing")
	}
	proven := map[string]bool{}
	for _, c := range submissionReds {
		proven[c.wantCheck] = true
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
	n, _, refusal := Verify(s, "")
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

// rewrite rebuilds the ledger as a fresh, self-consistent chain holding
// only the records it is given back — what a writer with the state file
// does, rather than what any looplaw path can do.
func rewrite(t *testing.T, dir string, keep func(store.Record) bool) {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(dir, "looplaw.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	rows, err := db.Query("SELECT kind, rectype, subject, body, party, at FROM records ORDER BY seq")
	if err != nil {
		t.Fatal(err)
	}
	var kept []store.Record
	for rows.Next() {
		var r store.Record
		var kind string
		if err := rows.Scan(&kind, &r.Type, &r.Subject, &r.Body, &r.Party, &r.At); err != nil {
			t.Fatal(err)
		}
		r.Kind = store.Kind(kind)
		if keep(r) {
			kept = append(kept, r)
		}
	}
	rows.Close()
	if _, err := db.Exec("DELETE FROM records"); err != nil {
		t.Fatal(err)
	}
	prev := ""
	for i, r := range kept {
		h := chainHash(r, prev)
		if _, err := db.Exec(
			"INSERT INTO records (seq, kind, rectype, subject, body, party, at, prev, hash) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
			int64(i+1), string(r.Kind), r.Type, r.Subject, r.Body, r.Party, r.At, prev, h); err != nil {
			t.Fatal(err)
		}
		prev = h
	}
}

// chainHash re-derives the ledger's record identity here rather than
// calling the store's, so this test is a second implementation of the
// canonical form: a producer that verifies itself proves nothing.
func chainHash(r store.Record, prev string) string {
	var b strings.Builder
	for _, f := range []string{string(r.Kind), r.Type, r.Subject, r.Body, r.Party, r.At, prev} {
		fmt.Fprintf(&b, "%d:%s|", len(f), f)
	}
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}

// Proving red: the chain is checked against itself, so a writer who
// controls the state file can erase an act and rebuild what remains as a
// fresh chain that verifies. Every local invariant survives the rewrite
// — the sequence is contiguous from 1, and each act's records are still
// paired — because they are computed from the same rewritten rows.
//
// Detecting it needs a value the rewriter could not reach, which the
// ledger cannot hold and looplaw cannot fetch (T0-3, T0-4). So the
// caller keeps it and submits it, the way a client submits a provenance
// manifest for the kernel to compare.
func TestVerifyAgainstAnExpectationTheLedgerCannotSupply(t *testing.T) {
	s, dir := openAt(t)
	if _, refusals := Submit(s, goodClaim()); len(refusals) != 0 {
		t.Fatal(refusals)
	}
	if _, refusals := Submit(s, goodReceipt()); len(refusals) != 0 {
		t.Fatal(refusals)
	}
	n, expected, refusal := Verify(s, "")
	if refusal != nil {
		t.Fatal(refusal)
	}
	if n != 4 {
		t.Fatalf("verified %d records, want 4", n)
	}

	// What the caller writes down, somewhere the state root's writer
	// cannot reach.
	rewrite(t, dir, func(r store.Record) bool { return r.Subject != "C-LEND-1" })

	// Unchecked, the rewrite is invisible: this is the limit being
	// closed, and stating it is half the point.
	if n, _, refusal := Verify(s, ""); refusal != nil || n != 2 {
		t.Fatalf("a rewritten ledger must still verify against itself: n=%d %v", n, refusal)
	}

	n, _, refusal = Verify(s, expected)
	if refusal == nil {
		t.Fatal("the rewrite passed a verification that was given what the ledger should hold")
	}
	if refusal.Check != "verify/expected" {
		t.Errorf("want verify/expected, got %s", refusal.Check)
	}
	// A verification path commits nothing, so this is a finding.
	if refusal.Class != outcome.Finding {
		t.Errorf("class = %s, want finding", refusal.Class)
	}
	if !strings.Contains(refusal.Reason, "2") || !strings.Contains(refusal.Reason, "4") {
		t.Errorf("the reason must name both counts: %q", refusal.Reason)
	}
	if n != 0 {
		t.Errorf("a refused verification reported %d records as verified", n)
	}
}

// The expectation is a claim about the ledger, so a malformed one is
// refused as a malformed submission rather than read as a mismatch: a
// caller who mistypes it must not be told their ledger was rewritten.
func TestMalformedExpectationIsNotAMismatch(t *testing.T) {
	s, _ := openAt(t)
	if _, refusals := Submit(s, goodClaim()); len(refusals) != 0 {
		t.Fatal(refusals)
	}
	for _, bad := range []string{"2", "2:", ":abc", "two:" + strings.Repeat("a", 64), "2:zzz", "-1:" + strings.Repeat("a", 64)} {
		_, _, refusal := Verify(s, bad)
		if refusal == nil {
			t.Errorf("%q was read as an expectation", bad)
			continue
		}
		if refusal.Check != "verify/expected-form" {
			t.Errorf("%q: want verify/expected-form, got %s", bad, refusal.Check)
		}
		if refusal.Class != outcome.Rejection {
			t.Errorf("%q: class = %s, want rejection", bad, refusal.Class)
		}
	}
}

// unreadableLedger verifies but cannot be read back. Storage that
// answers one and not the other is not hypothetical — a remote ledger
// can hold a checkpoint it can compare while the read path is down.
type unreadableLedger struct{ n int }

func (u *unreadableLedger) Append([]store.Draft) ([]store.Record, error) {
	return nil, fmt.Errorf("append: unavailable")
}
func (u *unreadableLedger) Records() ([]store.Record, error) {
	return nil, fmt.Errorf("read ledger: storage unavailable")
}
func (u *unreadableLedger) Verify() (int, error) { return u.n, nil }
func (u *unreadableLedger) Close() error         { return nil }

// A ledger that cannot be read reports an abort, not a mismatch: an
// unreadable ledger is an infrastructure failure, and reporting it as a
// finding would tell a caller their records had been rewritten.
func TestUnreadableLedgerAbortsRatherThanReportingAMismatch(t *testing.T) {
	s := store.New(&unreadableLedger{n: 2})
	defer s.Close()

	n, current, refusal := Verify(s, "2:"+strings.Repeat("a", 64))
	if refusal == nil {
		t.Fatal("an unreadable ledger verified")
	}
	if refusal.Check != "verify/read" {
		t.Errorf("want verify/read, got %s", refusal.Check)
	}
	if refusal.Class != outcome.Abort {
		t.Errorf("class = %s, want abort — nothing was checked", refusal.Class)
	}
	if n != 0 || current != "" {
		t.Errorf("a failed read reported state: n=%d current=%q", n, current)
	}
}
