package diff

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strings"
	"testing"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"

	"github.com/xormania/looplaw/internal/gate"
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
	proven := map[string]bool{}
	for _, red := range []struct {
		check string
		run   func(*testing.T)
	}{
		{"diff/side", TestInvalidSideRefusedAndNamed},
		{"diff/subject-mismatch", TestSubjectMismatchRefused},
		{"diff/goal-provenance", TestAbsorbedViewRefusedAsGoalLaw},
	} {
		if !t.Run("proving "+red.check, red.run) {
			t.Errorf("the red for %s did not pass, so it proves nothing", red.check)
			continue
		}
		proven[red.check] = true
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

// Evidence never sets the standard it is measured against: an absorbed
// view on the goal side would let a party's claim become the law
// reality is compared to (T0-2). Reproduced by review before this red
// existed — both orientations were accepted, and the inverted one
// emitted fill orders directing ratified law to conform to a claim.
func TestAbsorbedViewRefusedAsGoalLaw(t *testing.T) {
	const absorbed = "../absorb/testdata/view.cue"

	gaps, refusals := Diff(absorbed, view)
	if gaps != nil {
		t.Fatalf("an absorbed view was accepted as goal-law: %+v", gaps)
	}
	if len(refusals) != 1 || refusals[0].Check != "diff/goal-provenance" {
		t.Fatalf("want a single diff/goal-provenance refusal, got %+v", refusals)
	}
	if refusals[0].Remedy == "" {
		t.Error("refusal without a remedy")
	}

	// The view side stays unconstrained: with or without provenance it
	// is legitimately a view.
	if _, refusals := Diff(goal, absorbed); len(refusals) != 0 {
		for _, r := range refusals {
			if r.Check == "diff/goal-provenance" {
				t.Errorf("provenance on the view side was refused: %s", r.Error())
			}
		}
	}
}

// One mutation per field #Contract admits, each of which must produce a
// gap naming the contract.
//
// Proving red: contractsOf hashed parties, acts, cites, synchronization
// and interior, and nothing else — so a contract could move from
// "ratified" to "withdrawn", reassign blame from one registered party to
// another, change the evidence a violation is adjudicated from, gain an
// activation trigger, or be renamed, and the gap feed answered []. That
// is a false statement of equilibrium at the reconciliation boundary,
// which is the one place a consumer asks whether goal-law and the view
// still agree.
//
// The mutations are hand-written because a mutation needs to know what
// the field means; the completeness assertion below is what keeps the
// table honest when the schema grows.
var contractFieldMutations = map[string]struct {
	old, new string
	// The contracts the resulting gaps must address. A gap for the wrong
	// reason proves nothing, and most of these mutations touch one
	// contract; renaming one addresses two, because a rename is a
	// contract absent and another added.
	addresses []string
}{
	"name": {
		old: `name: "the lending contract"`, new: `name: "the borrowing contract"`,
		addresses: []string{"C-LEND-1"},
	},
	// Mutated on the contract with no interior: changing the client of a
	// decomposed contract stops its children's preconditions being
	// client-owed, and the set is refused before the differ sees it.
	"parties": {
		old:       "client:   \"borrower\"\n\t\t\tsupplier: \"librarian\"\n\t\t}\n\t\tacts: [\"return\"]",
		new:       "client:   \"borrower\"\n\t\t\tsupplier: \"desk\"\n\t\t}\n\t\tacts: [\"return\"]",
		addresses: []string{"C-RETURN-1"},
	},
	"acts": {
		old: `acts: ["lend"]`, new: `acts: ["lend", "renew"]`,
		addresses: []string{"C-LEND-1"},
	},
	"cites": {
		old:       "cites: [\"L-1\"]\n\t\tblame: [\n\t\t\t{violation_class: \"lending an already-lent book\"",
		new:       "cites: []\n\t\tblame: [\n\t\t\t{violation_class: \"lending an already-lent book\"",
		addresses: []string{"C-LEND-1"},
	},
	"synchronization": {
		old:       `name: "the lending contract"`,
		new:       "name: \"the lending contract\"\n\t\tsynchronization: \"the loan record and the standing attestation commit together\"",
		addresses: []string{"C-LEND-1"},
	},
	"blame": {
		old:       `at_fault: "librarian", evidence: "the loan records at lending time"`,
		new:       `at_fault: "borrower", evidence: "the loan records at lending time"`,
		addresses: []string{"C-LEND-1"},
	},
	"status": {
		old:       "\t\tstatus: \"ratified\"\n\t\tinterior:",
		new:       "\t\tstatus: \"withdrawn\"\n\t\tinterior:",
		addresses: []string{"C-LEND-1"},
	},
	"trigger": {
		old:       "\t\tstatus: \"ratified\"\n\t\tinterior:",
		new:       "\t\tstatus: \"ratified\"\n\t\ttrigger: \"a borrower requests a book\"\n\t\tinterior:",
		addresses: []string{"C-LEND-1"},
	},
	"interior": {
		old: `children: ["C-STANDING-1", "C-ISSUE-1"]`, new: `children: ["C-ISSUE-1", "C-STANDING-1"]`,
		addresses: []string{"C-LEND-1"},
	},
	"preconditions": {
		old:       `"P-2": {text: "The requested book carries no live loan, verifiable from the loan records."}`,
		new:       `"P-2": {text: "The requested book may carry a live loan."}`,
		addresses: []string{"C-LEND-1"},
	},
	"guarantees": {
		old:       `"G-1": {text: "The loan is retired and the retirement is recorded; the book is lendable again.", records: "the return record"}`,
		new:       `"G-1": {text: "The loan is retired and the retirement is recorded; the book is lendable again.", records: "nothing"}`,
		addresses: []string{"C-RETURN-1"},
	},
	"invariants_local": {
		old:       "invariants_local: {}\n\t\tcites: [\"L-1\"]\n\t\tblame: [\n\t\t\t{violation_class: \"late return\"",
		new:       "invariants_local: {\"LI-1\": {text: \"a returned book is lendable before the next lend act\"}}\n\t\tcites: [\"L-1\"]\n\t\tblame: [\n\t\t\t{violation_class: \"late return\"",
		addresses: []string{"C-RETURN-1"},
	},
	// The map key is the contract's identity, so changing it is not a
	// field that differs: it is one contract absent from the view and
	// another added. Renamed on the contract nothing cites and no
	// interior contains, so the set stays valid and the differ is what
	// answers.
	"id": {
		old: `"C-RETURN-1": {`, new: `"C-RETURNING-1": {`,
		addresses: []string{"C-RETURN-1", "C-RETURNING-1"},
	},
}

func TestEveryContractFieldProducesAGap(t *testing.T) {
	base, err := os.ReadFile(goal)
	if err != nil {
		t.Fatal(err)
	}

	for _, field := range sortedFieldNames(contractFieldMutations) {
		m := contractFieldMutations[field]
		t.Run(field, func(t *testing.T) {
			if !strings.Contains(string(base), m.old) {
				t.Fatalf("mutation target drifted from the fixture: %q", m.old)
			}
			mutated := strings.Replace(string(base), m.old, m.new, 1)
			path := filepath.Join(t.TempDir(), "view.cue")
			if err := os.WriteFile(path, []byte(mutated), 0o644); err != nil {
				t.Fatal(err)
			}

			gaps, refusals := Diff(goal, path)
			if len(refusals) != 0 {
				t.Fatalf("the mutation must stay a valid set, or it tests the gates instead: %+v", refusals)
			}
			if len(gaps) == 0 {
				t.Fatalf("%s changed and the gap feed reported equilibrium", field)
			}
			// A gap for the wrong reason proves nothing: every gap must
			// address a contract the mutation touched, and each of those
			// must draw one.
			addressed := map[string]bool{}
			for _, g := range gaps {
				addressed[g.Address.Contract] = true
				if !slices.Contains(m.addresses, g.Address.Contract) {
					t.Errorf("gap addresses %q, which the mutation did not touch: %+v", g.Address.Contract, g)
				}
			}
			for _, want := range m.addresses {
				if !addressed[want] {
					t.Errorf("%s changed and no gap addresses %s", field, want)
				}
			}
		})
	}
}

// The table covers every field #Contract admits, read from the schema
// the binary embeds rather than from a list kept beside it. A field
// added to the schema with no mutation here fails, which is the point:
// the differ was blind to four fields for as long as nothing compared
// the two lists.
func TestContractFieldMutationsCoverTheSchema(t *testing.T) {
	ctx := cuecontext.New()
	law, err := gate.Law(ctx)
	if err != nil {
		t.Fatal(err)
	}
	iter, err := law.LookupPath(cue.ParsePath("#Contract")).Fields(cue.Optional(true))
	if err != nil {
		t.Fatal(err)
	}
	schemaFields := map[string]bool{}
	for iter.Next() {
		schemaFields[iter.Selector().Unquoted()] = true
	}
	if len(schemaFields) == 0 {
		t.Fatal("no #Contract fields read from the embedded schema")
	}

	for field := range schemaFields {
		if _, ok := contractFieldMutations[field]; !ok {
			t.Errorf("#Contract admits %q and no mutation proves the differ sees it", field)
		}
	}
	for field := range contractFieldMutations {
		if !schemaFields[field] {
			t.Errorf("a mutation names %q, which #Contract does not admit — delete the stale entry", field)
		}
	}
}

func sortedFieldNames[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
