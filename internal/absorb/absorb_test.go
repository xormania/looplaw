package absorb

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"cuelang.org/go/cue"

	"github.com/xormania/looplaw/internal/gate"
	"github.com/xormania/looplaw/internal/provenance"
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
	a, err := ScanScope(scope, "scope")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := ScanScope(scope, "scope")
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
	if _, err := ScanScope(filepath.Join(scope, "lending.go"), "scope"); err == nil {
		t.Error("scanning a file rather than a directory must fail")
	}
	if _, err := ScanScope("testdata/nowhere", "scope"); err == nil {
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
	m, err := ScanScope(dir, "scope")
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
	m, err := ScanScope(scope, "scope")
	if err != nil {
		t.Fatal(err)
	}
	rep := provenance.Compare(viewProvenance(t, view), m)
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
	m, err := ScanScope(scope, "scope")
	if err != nil {
		t.Fatal(err)
	}
	m.Sources["lending.go"] = strings.Repeat("a", 64)

	rep := provenance.Compare(viewProvenance(t, view), m)
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
	m, err := ScanScope(scope, "scope")
	if err != nil {
		t.Fatal(err)
	}
	delete(m.Sources, "returning.go")

	rep := provenance.Compare(viewProvenance(t, view), m)
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
	m, err := ScanScope(scope, "scope")
	if err != nil {
		t.Fatal(err)
	}
	m.Sources["renewals.go"] = strings.Repeat("b", 64)

	rep := provenance.Compare(viewProvenance(t, view), m)
	if rep.Stale {
		t.Error("an added source alone must not make a view stale")
	}
	if !reflect.DeepEqual(rep.Added, []string{"renewals.go"}) {
		t.Errorf("added = %v", rep.Added)
	}
}

// provenance.Compare takes data only: given identical inputs it must not consult
// the filesystem, so the same comparison holds wherever it runs.
func TestCompareTouchesNoFilesystem(t *testing.T) {
	prov := viewProvenance(t, view)
	phantom := provenance.Manifest{Scope: "phantom", Sources: map[string]string{
		"lending.go":   strings.Repeat("c", 64),
		"returning.go": strings.Repeat("d", 64),
	}}
	rep := provenance.Compare(prov, phantom)
	if len(rep.Changed) != 2 || rep.ScopeSubmitted != "phantom" || !rep.ScopeMismatch {
		t.Fatalf("provenance.Compare did not work purely from the submitted manifest: %+v", rep)
	}
}

// The skeleton is deliberately not a valid set: authoring the law is
// inference, and the gates' refusals are the worklist handed to whoever
// does it.
func TestSkeletonCarriesProvenanceAndDoesNotValidate(t *testing.T) {
	m, err := ScanScope(scope, "scope")
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

// A manifest from a different scope than the one recorded makes every
// comparison meaningless, so the mismatch is stated and the view counts
// as stale rather than quietly green.
func TestScopeMismatchIsStated(t *testing.T) {
	m, err := ScanScope(scope, "some-other-scope")
	if err != nil {
		t.Fatal(err)
	}
	rep := provenance.Compare(viewProvenance(t, view), m)
	if !rep.ScopeMismatch || !rep.Stale {
		t.Fatalf("a mismatched scope reported clean: %+v", rep)
	}
	if rep.ScopeRecorded != "scope" || rep.ScopeSubmitted != "some-other-scope" {
		t.Errorf("both scopes must be named: %+v", rep)
	}
}

// A symlinked scope root must refuse: walking it yields an empty
// manifest, which reads downstream as total staleness rather than as
// the refusal it is.
func TestSymlinkedScopeRootRefused(t *testing.T) {
	dir := t.TempDir()
	real := filepath.Join(dir, "real")
	if err := os.MkdirAll(real, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(real, "x.go"), []byte("package x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, err := ScanScope(link, "scope"); err == nil {
		t.Fatal("a symlinked scope root was scanned rather than refused")
	}
}

// The caller names the scope; the client never derives identity from a
// path spelling.
func TestScopeNameIsCallerSupplied(t *testing.T) {
	if _, err := ScanScope(scope, ""); err == nil {
		t.Fatal("an unnamed scope must refuse rather than have a name invented for it")
	}
	m, err := ScanScope(scope, "caller-chosen")
	if err != nil {
		t.Fatal(err)
	}
	if m.Scope != "caller-chosen" {
		t.Errorf("scope = %q, want the caller's name", m.Scope)
	}
}

// Control bytes and other CUE-hostile characters in a path must not
// produce unparseable output: the skeleton is quoted as CUE, not as Go.
func TestSkeletonQuotesAsCUE(t *testing.T) {
	dir := t.TempDir()
	odd := filepath.Join(dir, "a\x01b\"c.go")
	if err := os.WriteFile(odd, []byte("package x\n"), 0o644); err != nil {
		t.Skipf("filesystem rejects the name: %v", err)
	}
	m, err := ScanScope(dir, "odd")
	if err != nil {
		t.Fatal(err)
	}
	out := Skeleton("odd-subject", m)
	path := filepath.Join(t.TempDir(), "skeleton.cue")
	if err := os.WriteFile(path, []byte(out), 0o644); err != nil {
		t.Fatal(err)
	}
	// It must be refused for binding nothing, never for being
	// unparseable: unparseable output at exit 0 is the defect.
	for _, r := range gate.ValidateTrinity(path) {
		if r.Check == "trinity/parse" {
			t.Fatalf("skeleton is not parseable CUE: %s", r.Error())
		}
	}
}
