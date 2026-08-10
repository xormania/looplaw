package absorb

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"cuelang.org/go/cue"

	"github.com/xormania/looplaw/internal/gate"
)

const (
	scope = "testdata/scope"
	view  = "testdata/view.cue"
)

func viewProvenance(t *testing.T, path string) cue.Value {
	t.Helper()
	v, refusals := gate.LoadSet(path)
	if len(refusals) != 0 {
		t.Fatalf("view refused: %+v", refusals)
	}
	prov := v.LookupPath(cue.ParsePath("provenance"))
	if !prov.Exists() {
		t.Fatal("fixture view carries no provenance")
	}
	return prov
}

func TestScanIsDeterministicAndSkipsNonScope(t *testing.T) {
	a, err := ScanScope(scope)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := ScanScope(scope)
	if !reflect.DeepEqual(a, b) {
		t.Fatal("two scans of one scope differ")
	}
	if len(a.Sources) != 2 {
		t.Fatalf("want 2 sources, got %d: %v", len(a.Sources), a.Paths())
	}
	for p := range a.Sources {
		if strings.HasPrefix(p, ".git") {
			t.Errorf("version-control internals absorbed: %s", p)
		}
	}
	if _, err := ScanScope(filepath.Join(scope, "lending.go")); err == nil {
		t.Error("scanning a file rather than a directory must fail")
	}
	if _, err := ScanScope("testdata/nowhere"); err == nil {
		t.Error("scanning a missing scope must fail")
	}
}

// A symlink's target lies outside the scope the client was handed, so
// absorbing through one would source law from unscoped content.
func TestScanRefusesToFollowSymlinks(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "real.go"), []byte("package x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "outside.go")
	if err := os.WriteFile(outside, []byte("package outside\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, "linked.go")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	m, err := ScanScope(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := m.Sources["linked.go"]; ok {
		t.Error("a symlink was absorbed as scope content")
	}
	if len(m.Sources) != 1 {
		t.Errorf("want only the regular file, got %v", m.Paths())
	}
}

func TestUnchangedScopeIsNotStale(t *testing.T) {
	m, err := ScanScope(scope)
	if err != nil {
		t.Fatal(err)
	}
	rep := Compare(viewProvenance(t, view), m)
	if rep.Stale || len(rep.Changed)+len(rep.Missing)+len(rep.Affected) != 0 {
		t.Fatalf("unchanged scope reported stale: %+v", rep)
	}
	if rep.Counts.Baselined != 2 || rep.Counts.Current != 2 {
		t.Errorf("counts wrong: %+v", rep.Counts)
	}
}

// The payoff of provenance: an edited source names the statements
// derived from it, so re-derivation is scoped rather than wholesale.
func TestChangedSourceNamesTheAffectedStatements(t *testing.T) {
	m, err := ScanScope(scope)
	if err != nil {
		t.Fatal(err)
	}
	m.Sources["lending.go"] = strings.Repeat("a", 64)

	rep := Compare(viewProvenance(t, view), m)
	if !rep.Stale {
		t.Fatal("edited source did not make the view stale")
	}
	if !reflect.DeepEqual(rep.Changed, []string{"lending.go"}) {
		t.Errorf("changed = %v", rep.Changed)
	}
	var addrs []string
	for _, a := range rep.Affected {
		addrs = append(addrs, a.Address)
	}
	if !reflect.DeepEqual(addrs, []string{"C-LEND-1", "C-LEND-1/G-1"}) {
		t.Errorf("affected = %v, want the lending statements only", addrs)
	}
}

func TestMissingSourceIsStale(t *testing.T) {
	m, err := ScanScope(scope)
	if err != nil {
		t.Fatal(err)
	}
	delete(m.Sources, "returning.go")

	rep := Compare(viewProvenance(t, view), m)
	if !rep.Stale || !reflect.DeepEqual(rep.Missing, []string{"returning.go"}) {
		t.Fatalf("deleted source not reported missing: %+v", rep)
	}
	if len(rep.Affected) != 2 {
		t.Errorf("want both return statements affected, got %+v", rep.Affected)
	}
}

// A grown scope is not a stale view: whether new content bears on the
// law is a judgment, and the differ reports it as evidence, not a
// verdict.
func TestAddedSourceIsReportedButNotStale(t *testing.T) {
	m, err := ScanScope(scope)
	if err != nil {
		t.Fatal(err)
	}
	m.Sources["renewals.go"] = strings.Repeat("b", 64)

	rep := Compare(viewProvenance(t, view), m)
	if rep.Stale {
		t.Error("an added source alone must not make a view stale")
	}
	if !reflect.DeepEqual(rep.Added, []string{"renewals.go"}) {
		t.Errorf("added = %v", rep.Added)
	}
}

// Compare takes data only: given identical inputs it must not consult
// the filesystem, so the same comparison holds wherever it runs.
func TestCompareTouchesNoFilesystem(t *testing.T) {
	prov := viewProvenance(t, view)
	phantom := Manifest{Scope: "phantom", Sources: map[string]string{
		"lending.go":   strings.Repeat("c", 64),
		"returning.go": strings.Repeat("d", 64),
	}}
	rep := Compare(prov, phantom)
	if len(rep.Changed) != 2 || rep.Scope != "phantom" {
		t.Fatalf("Compare did not work purely from the submitted manifest: %+v", rep)
	}
}

// The skeleton is deliberately not a valid set: authoring the law is
// inference, and the gates' refusals are the worklist handed to whoever
// does it.
func TestSkeletonCarriesProvenanceAndDoesNotValidate(t *testing.T) {
	m, err := ScanScope(scope)
	if err != nil {
		t.Fatal(err)
	}
	out := Skeleton("lend-library", m)
	for _, want := range []string{`subject:        "lend-library"`, "provenance: {", "lending.go", "returning.go"} {
		if !strings.Contains(out, want) {
			t.Errorf("skeleton missing %q", want)
		}
	}

	path := filepath.Join(t.TempDir(), "skeleton.cue")
	if err := os.WriteFile(path, []byte(out), 0o644); err != nil {
		t.Fatal(err)
	}
	_, refusals := gate.LoadSet(path)
	if len(refusals) == 0 {
		t.Fatal("the skeleton validated — an empty view must be refused, not accepted as law-shaped")
	}
	vacuity := false
	for _, r := range refusals {
		if r.Check == "trinity/vacuity" {
			vacuity = true
		}
	}
	if !vacuity {
		t.Errorf("want the vacuity refusal as the worklist, got %+v", refusals)
	}
}
