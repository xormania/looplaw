package provenance

import (
	"reflect"
	"testing"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
)

// This package had no test file. Its code was fully exercised through
// internal/absorb — 100% of statements — which is exactly why the gap was
// invisible: statement coverage cannot see a discarded error, because the
// line executes whether or not the error was real. Only a test that
// builds the bad input can speak about it.

func prov(t *testing.T, src string) cue.Value {
	t.Helper()
	v := cuecontext.New().CompileString(src)
	if v.Err() != nil {
		t.Fatal(v.Err())
	}
	return v
}

func manifest(scope string, sources map[string]string) Manifest {
	return Manifest{Scope: scope, Sources: sources}
}

// The classifier, branch by branch. Absorb's golden pins one report
// shape; this pins the decision behind it.
func TestCompareClassifiesEveryWay(t *testing.T) {
	base := `scope: "s"
sources: {
	"kept.go":    "1111111111111111111111111111111111111111111111111111111111111111"
	"changed.go": "2222222222222222222222222222222222222222222222222222222222222222"
	"gone.go":    "3333333333333333333333333333333333333333333333333333333333333333"
}`
	rep := Compare(prov(t, base), manifest("s", map[string]string{
		"kept.go":    "1111111111111111111111111111111111111111111111111111111111111111",
		"changed.go": "9999999999999999999999999999999999999999999999999999999999999999",
		"new.go":     "4444444444444444444444444444444444444444444444444444444444444444",
	}))

	if len(rep.Changed) != 1 || rep.Changed[0] != "changed.go" {
		t.Errorf("changed = %v", rep.Changed)
	}
	if len(rep.Missing) != 1 || rep.Missing[0] != "gone.go" {
		t.Errorf("missing = %v", rep.Missing)
	}
	if len(rep.Added) != 1 || rep.Added[0] != "new.go" {
		t.Errorf("added = %v", rep.Added)
	}
	if !rep.Stale {
		t.Error("a changed source is stale")
	}
	if rep.Counts.Baselined != 3 || rep.Counts.Current != 3 {
		t.Errorf("counts = %+v", rep.Counts)
	}
}

// A source nobody baselined cannot have moved: it is reported so the
// absorption can be widened, and reporting it is not a verdict.
func TestAddedAloneIsNotStale(t *testing.T) {
	rep := Compare(prov(t, `scope: "s"
sources: {"kept.go": "1111111111111111111111111111111111111111111111111111111111111111"}`),
		manifest("s", map[string]string{
			"kept.go": "1111111111111111111111111111111111111111111111111111111111111111",
			"new.go":  "2222222222222222222222222222222222222222222222222222222222222222",
		}))
	if rep.Stale {
		t.Error("an added source alone made the view stale")
	}
	if len(rep.Added) != 1 {
		t.Errorf("the added source was not reported: %v", rep.Added)
	}
}

// Comparing a manifest against a different scope's baseline compares
// nothing, and the report says so rather than leaving a reader to notice
// two names that do not match.
func TestScopeMismatchIsStaleAndStated(t *testing.T) {
	rep := Compare(prov(t, `scope: "recorded"
sources: {}`), manifest("submitted", nil))
	if !rep.ScopeMismatch || !rep.Stale {
		t.Errorf("a scope mismatch is stated and stale: %+v", rep)
	}
	if rep.ScopeRecorded != "recorded" || rep.ScopeSubmitted != "submitted" {
		t.Errorf("the report does not name both scopes: %+v", rep)
	}
}

// Proving red for the silent skip. A baseline that cannot be read is not
// an empty baseline: skipping it left every current source classified
// "Added", and added alone is not stale, so unreadable provenance
// reported "stale": false — a verdict drawn from a failure, and the one
// answer a caller acts on by doing nothing.
func TestUnreadableBaselineIsStaleNotClean(t *testing.T) {
	for _, tc := range []struct{ name, src string }{
		{"no sources region at all", `scope: "s"`},
		{"sources is not a struct", `scope: "s"
sources: "not a struct"`},
		{"a digest is not a string", `scope: "s"
sources: {"a.go": 42}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rep := Compare(prov(t, tc.src), manifest("s", map[string]string{
				"a.go": "1111111111111111111111111111111111111111111111111111111111111111",
			}))
			if !rep.BaselineUnreadable {
				t.Errorf("an unreadable baseline was not declared: %+v", rep)
			}
			if !rep.Stale {
				t.Error("unreadable provenance reported the view as fresh, " +
					"which is the failure rendering as the reassuring answer")
			}
		})
	}

	// And the distinction it turns on: a baseline of nothing is readable,
	// and a scope with nothing recorded is not stale.
	rep := Compare(prov(t, `scope: "s"
sources: {}`), manifest("s", nil))
	if rep.BaselineUnreadable || rep.Stale {
		t.Errorf("an empty baseline is not an unreadable one: %+v", rep)
	}
}

// Affected derivations name the statements a moved source supported, in
// a stable order: a consumer scripts against this, and a set walked
// through a map would reorder run to run.
func TestAffectedDerivationsAreStableAndNameTheirStatements(t *testing.T) {
	src := `scope: "s"
sources: {
	"a.go": "1111111111111111111111111111111111111111111111111111111111111111"
	"b.go": "2222222222222222222222222222222222222222222222222222222222222222"
}
derivations: {
	"C-2": ["a.go"]
	"C-1": ["a.go", "b.go"]
	"C-3": ["b.go"]
}`
	var first []Affected
	for i := 0; i < 5; i++ {
		rep := Compare(prov(t, src), manifest("s", map[string]string{
			"a.go": "9999999999999999999999999999999999999999999999999999999999999999",
			"b.go": "2222222222222222222222222222222222222222222222222222222222222222",
		}))
		if i == 0 {
			first = rep.Affected
			continue
		}
		if len(rep.Affected) != len(first) {
			t.Fatalf("affected count varies between runs: %d then %d", len(first), len(rep.Affected))
		}
		if !reflect.DeepEqual(rep.Affected, first) {
			t.Fatalf("affected order is not stable:\n  %+v\n  %+v", first, rep.Affected)
		}
	}
	// b.go did not move, so C-3 is untouched; a.go did, so C-1 and C-2 are.
	var addrs []string
	for _, a := range first {
		addrs = append(addrs, a.Address)
	}
	if len(addrs) != 2 || addrs[0] != "C-1" || addrs[1] != "C-2" {
		t.Errorf("affected = %v, want C-1 and C-2 in order", addrs)
	}
}

// The kernel never reads a work tree (T0-4). Compare takes both sides as
// data, so there is nothing for it to read — asserted here because it is
// the package's whole reason for existing separately from absorb.
func TestCompareReadsNothing(t *testing.T) {
	rep := Compare(prov(t, `scope: "s"
sources: {"/etc/passwd": "1111111111111111111111111111111111111111111111111111111111111111"}`),
		manifest("s", map[string]string{"/etc/passwd": "2222222222222222222222222222222222222222222222222222222222222222"}))
	if len(rep.Changed) != 1 {
		t.Fatalf("changed = %v", rep.Changed)
	}
	// The verdict came from the two digests it was handed, not from the
	// file: a real read would have found neither digest.
	if !rep.Stale {
		t.Error("differing digests are stale")
	}
}
