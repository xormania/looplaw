package diff

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/xormania/looplaw/internal/outcome"
)

const (
	goal      = "../gate/testdata/library/set.cue"
	view      = "testdata/library-view.cue"
	splitView = "testdata/library-view-split.cue"
)

// The view fixture carries exactly three deltas; the differ must find
// exactly those, in deterministic address order, with stable ids.
func TestDiffFindsTheDeltas(t *testing.T) {
	gaps, refusals := Diff(goal, view)
	if len(refusals) != 0 {
		t.Fatalf("refused: %+v", refusals)
	}

	want := []struct {
		id, contract, clause, kind, work string
	}{
		{"GAP-1", "C-AUDIT-1", "", "added", "fill"},
		{"GAP-2", "C-LEND-1", "P-1", "changed", "fill"},
		{"GAP-3", "C-RETURN-1", "", "absent", "fill"},
	}
	if len(gaps) != len(want) {
		t.Fatalf("got %d gaps, want %d: %+v", len(gaps), len(want), gaps)
	}
	for i, w := range want {
		g := gaps[i]
		if g.ID != w.id || g.Address.Contract != w.contract || g.Address.Clause != w.clause || g.Kind != w.kind || g.Work != w.work {
			t.Errorf("gap %d = %+v, want %+v", i, g, w)
		}
		if g.Subject != "lend-library" || g.Status != "open" {
			t.Errorf("gap %d subject/status wrong: %+v", i, g)
		}
	}

	// Hash sides: absent has goal only; added has view only; changed both.
	if gaps[0].GoalHash != "" || gaps[0].ViewHash == "" {
		t.Errorf("added gap hashes wrong: %+v", gaps[0])
	}
	if gaps[1].GoalHash == "" || gaps[1].ViewHash == "" || gaps[1].GoalHash == gaps[1].ViewHash {
		t.Errorf("changed gap hashes wrong: %+v", gaps[1])
	}
	if gaps[2].GoalHash == "" || gaps[2].ViewHash != "" {
		t.Errorf("absent gap hashes wrong: %+v", gaps[2])
	}
}

// A contract the goal decomposes calls for split work when absent.
func TestAbsentDecomposedContractIsSplitWork(t *testing.T) {
	gaps, refusals := Diff(goal, splitView)
	if len(refusals) != 0 {
		t.Fatalf("refused: %+v", refusals)
	}
	found := false
	for _, g := range gaps {
		if g.Address.Contract == "C-LEND-1" && g.Kind == "absent" {
			found = true
			if g.Work != "split" {
				t.Errorf("absent decomposed contract work = %q, want split", g.Work)
			}
		}
	}
	if !found {
		t.Fatalf("C-LEND-1 absent gap not reported: %+v", gaps)
	}
}

// The differ is deterministic: identical inputs, identical output —
// including ids (T0-3 is the kernel's rule; the read path honors it).
func TestDiffIsDeterministic(t *testing.T) {
	a, _ := Diff(goal, view)
	b, _ := Diff(goal, view)
	if !reflect.DeepEqual(a, b) {
		t.Fatal("two runs over identical inputs differ")
	}
}

func TestIdenticalSetsYieldNoGaps(t *testing.T) {
	gaps, refusals := Diff(goal, goal)
	if len(refusals) != 0 || len(gaps) != 0 {
		t.Fatalf("goal vs goal: gaps=%d refusals=%+v", len(gaps), refusals)
	}
}

func TestSubjectMismatchRefused(t *testing.T) {
	base, err := os.ReadFile(view)
	if err != nil {
		t.Fatal(err)
	}
	other := strings.Replace(string(base), `subject:        "lend-library"`, `subject:        "other-library"`, 1)
	path := filepath.Join(t.TempDir(), "set.cue")
	if err := os.WriteFile(path, []byte(other), 0o644); err != nil {
		t.Fatal(err)
	}
	gaps, refusals := Diff(goal, path)
	if gaps != nil || len(refusals) != 1 || refusals[0].Check != "diff/subject-mismatch" {
		t.Fatalf("want a single subject-mismatch refusal, got gaps=%v refusals=%+v", gaps, refusals)
	}
}

func TestInvalidSideRefusedAndNamed(t *testing.T) {
	base, err := os.ReadFile(view)
	if err != nil {
		t.Fatal(err)
	}
	broken := strings.Replace(string(base), `status: "ratified"`, `status: "vibes"`, 1)
	path := filepath.Join(t.TempDir(), "set.cue")
	if err := os.WriteFile(path, []byte(broken), 0o644); err != nil {
		t.Fatal(err)
	}
	gaps, refusals := Diff(goal, path)
	if gaps != nil || len(refusals) == 0 {
		t.Fatalf("invalid view accepted: gaps=%v", gaps)
	}
	for _, r := range refusals {
		if r.Check != "diff/side" {
			t.Errorf("check = %s, want diff/side", r.Check)
		}
		if !strings.Contains(r.Subject, "view side") {
			t.Errorf("refusal must name the failing side: %s", r.Error())
		}
		if r.Class != outcome.Rejection {
			t.Errorf("class = %s, want rejection", r.Class)
		}
	}
}

// Every check the differ can emit has a proving red or a declared,
// reasoned exemption.
func TestEveryDiffCheckHasAProvingRed(t *testing.T) {
	exempt := map[string]string{
		"diff/self-check": "fires only when the differ's own output breaks ratified law — unreachable from any input fixture by construction; guarded by the schema unification running on every diff (TestDiffFindsTheDeltas exercises the green path through it)",
	}
	proven := map[string]bool{
		"diff/side":             true, // TestInvalidSideRefusedAndNamed
		"diff/subject-mismatch": true, // TestSubjectMismatchRefused
	}
	for _, check := range Checks {
		if proven[check] {
			continue
		}
		if reason, ok := exempt[check]; ok {
			t.Logf("exempt (declared): %s — %s", check, reason)
			continue
		}
		t.Errorf("check %s has no proving red and no declared exemption", check)
	}
}

// Rewiring an interior — same clauses, different presents target — is a
// contract-grain changed gap, never equilibrium (review finding,
// reproduced before this red existed).
func TestInteriorRewiringIsAChangedGap(t *testing.T) {
	base, err := os.ReadFile(goal)
	if err != nil {
		t.Fatal(err)
	}
	rewired := strings.Replace(string(base),
		`"G-1": {child: "C-ISSUE-1", guarantee: "G-1"}`,
		`"G-1": {child: "C-STANDING-1", guarantee: "G-1"}`, 1)
	if rewired == string(base) {
		t.Fatal("rewire did not apply — fixture drifted")
	}
	path := filepath.Join(t.TempDir(), "set.cue")
	if err := os.WriteFile(path, []byte(rewired), 0o644); err != nil {
		t.Fatal(err)
	}
	gaps, refusals := Diff(goal, path)
	if len(refusals) != 0 {
		t.Fatalf("refused: %+v", refusals)
	}
	found := false
	for _, g := range gaps {
		if g.Address.Contract == "C-LEND-1" && g.Kind == "changed" && g.Address.Clause == "" {
			found = true
		}
	}
	if !found {
		t.Fatalf("interior rewiring produced no contract-grain changed gap: %+v", gaps)
	}
}

// A crafted NUL boundary in clause text cannot forge equality between
// materially different guarantees (review finding: delimiter injection).
func TestDelimiterInjectionCannotForgeEquality(t *testing.T) {
	base, err := os.ReadFile(goal)
	if err != nil {
		t.Fatal(err)
	}
	origText := `{text: "The loan is retired and the retirement is recorded; the book is lendable again.", records: "the return record"}`
	a := strings.Replace(string(base), origText,
		`{text: "The loan is retired.", records: "A\\u0000records:B"}`, 1)
	b := strings.Replace(string(base), origText,
		`{text: "The loan is retired.\\u0000records:A", records: "B"}`, 1)
	if a == string(base) || b == string(base) {
		t.Fatal("injection rewrite did not apply — fixture drifted")
	}
	dir := t.TempDir()
	pa, pb := filepath.Join(dir, "a.cue"), filepath.Join(dir, "b.cue")
	if err := os.WriteFile(pa, []byte(a), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pb, []byte(b), 0o644); err != nil {
		t.Fatal(err)
	}
	gaps, refusals := Diff(pa, pb)
	if len(refusals) != 0 {
		t.Fatalf("refused: %+v", refusals)
	}
	found := false
	for _, g := range gaps {
		if g.Address.Contract == "C-RETURN-1" && g.Address.Clause == "G-1" && g.Kind == "changed" {
			found = true
		}
	}
	if !found {
		t.Fatalf("forged boundary compared equal — injection survives: %+v", gaps)
	}
}

// When both sides fail their gates, the refusal stream is deterministic:
// goal side first, then view, every run.
func TestBothSidesRefusalOrderIsDeterministic(t *testing.T) {
	base, err := os.ReadFile(goal)
	if err != nil {
		t.Fatal(err)
	}
	broken := strings.Replace(string(base), `status: "ratified"`, `status: "vibes"`, 1)
	dir := t.TempDir()
	p := filepath.Join(dir, "broken.cue")
	if err := os.WriteFile(p, []byte(broken), 0o644); err != nil {
		t.Fatal(err)
	}
	var first []string
	for i := 0; i < 6; i++ {
		_, refusals := Diff(p, p)
		var order []string
		for _, r := range refusals {
			order = append(order, r.Subject[:4])
		}
		if first == nil {
			first = order
		} else if !reflect.DeepEqual(first, order) {
			t.Fatalf("refusal order varies across runs: %v vs %v", first, order)
		}
	}
	if len(first) == 0 || first[0] != "goal" {
		t.Fatalf("goal side must come first: %v", first)
	}
}
