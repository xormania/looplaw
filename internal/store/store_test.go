package store

import (
	"database/sql"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func open(t *testing.T) *Store {
	t.Helper()
	s, _ := openAt(t, t.TempDir())
	return s
}

// openAt returns the store and the root it lives under, so a test can
// reach the storage the way something outside looplaw would.
func openAt(t *testing.T, root string) (*Store, string) {
	t.Helper()
	s, err := OpenDeployment(root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s, root
}

// tamper edits the ledger out of band, which is what an attacker with
// access to the state root actually does. Reaching through the product's
// own type would test a hole looplaw does not have — the store exposes
// no way to alter a record, and adding one for a test would put it in
// the product.
func tamper(t *testing.T, root, statement string) {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(root, "looplaw.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(statement); err != nil {
		t.Fatalf("tamper: %v", err)
	}
}

func TestAppendChainsAndVerifies(t *testing.T) {
	s := open(t)

	r1, err := s.Append(Law, "admission", "registry-batch-1", `{"pr":4}`, "accountable:test")
	if err != nil {
		t.Fatalf("append 1: %v", err)
	}
	if r1.Prev != "" {
		t.Errorf("first record prev = %q, want empty", r1.Prev)
	}
	r2, err := s.Append(Evidence, "claim", "scope-x", `{"hash":"abc"}`, "absorber:test")
	if err != nil {
		t.Fatalf("append 2: %v", err)
	}
	if r2.Prev != r1.Hash {
		t.Errorf("second record prev = %q, want first hash %q", r2.Prev, r1.Hash)
	}

	n, err := s.Verify()
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if n != 2 {
		t.Errorf("verified %d records, want 2", n)
	}

	recs, err := s.Records()
	if err != nil {
		t.Fatalf("records: %v", err)
	}
	if len(recs) != 2 || recs[0].Kind != Law || recs[1].Kind != Evidence {
		t.Errorf("unexpected ledger contents: %+v", recs)
	}
}

func TestTamperIsDetected(t *testing.T) {
	s, root := openAt(t, t.TempDir())
	if _, err := s.Append(Law, "admission", "a", "body-a", "t"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Append(Law, "admission", "b", "body-b", "t"); err != nil {
		t.Fatal(err)
	}

	tamper(t, root, "UPDATE records SET body = 'forged' WHERE seq = 1")
	if _, err := s.Verify(); err == nil {
		t.Fatal("verify accepted a tampered record")
	} else if !strings.Contains(err.Error(), "seq 1") {
		t.Errorf("verify error names wrong record: %v", err)
	}
}

func TestDeletionBreaksChain(t *testing.T) {
	s, root := openAt(t, t.TempDir())
	for _, subj := range []string{"a", "b", "c"} {
		if _, err := s.Append(Evidence, "claim", subj, "x", "t"); err != nil {
			t.Fatal(err)
		}
	}
	tamper(t, root, "DELETE FROM records WHERE seq = 2")
	if _, err := s.Verify(); err == nil {
		t.Fatal("verify accepted a ledger with a deleted record")
	}
}

func TestKindConstraint(t *testing.T) {
	s := open(t)
	if _, err := s.Append(Kind("gossip"), "claim", "s", "b", "t"); err == nil {
		t.Fatal("append accepted an invalid kind")
	}
}

// Concurrent recorders must serialize: the chain may never fork. This is
// the store's load-bearing behavior claim, so it is tested as behavior —
// many goroutines appending at once, then a full chain verification.
func TestConcurrentAppendsDoNotForkTheChain(t *testing.T) {
	s := open(t)
	const writers, each = 8, 25

	var wg sync.WaitGroup
	errs := make(chan error, writers*each)
	for w := range writers {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := range each {
				if _, err := s.Append(Evidence, "claim", fmt.Sprintf("w%d-%d", w, i), "x", "t"); err != nil {
					errs <- err
				}
			}
		}(w)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Errorf("concurrent append failed: %v", err)
	}

	n, err := s.Verify()
	if err != nil {
		t.Fatalf("chain verification after concurrent appends: %v", err)
	}
	if n != writers*each {
		t.Errorf("verified %d records, want %d", n, writers*each)
	}
}

func TestDefaultRootPrecedence(t *testing.T) {
	t.Setenv("LOOPLAW_ROOT", "/explicit/root")
	t.Setenv("XDG_STATE_HOME", "/xdg/state")
	if root, _ := DefaultRoot(); root != "/explicit/root" {
		t.Errorf("LOOPLAW_ROOT should win, got %q", root)
	}

	t.Setenv("LOOPLAW_ROOT", "")
	if root, _ := DefaultRoot(); root != "/xdg/state/looplaw" {
		t.Errorf("XDG fallback wrong, got %q", root)
	}

	t.Setenv("XDG_STATE_HOME", "")
	root, err := DefaultRoot()
	if err != nil {
		t.Fatalf("home fallback: %v", err)
	}
	if !strings.HasSuffix(root, "/.local/state/looplaw") {
		t.Errorf("home fallback wrong, got %q", root)
	}
}

func TestProjectScopingIsExplicit(t *testing.T) {
	root := t.TempDir()

	if _, err := OpenProject(root, "loop-sys"); err == nil {
		t.Fatal("opening a never-initialized project must refuse — state is never created implicitly")
	}

	s, err := InitProject(root, "loop-sys")
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	if _, err := s.Append(Law, "admission", "x", "y", "t"); err != nil {
		t.Fatal(err)
	}
	s.Close()

	if _, err := InitProject(root, "loop-sys"); err == nil {
		t.Fatal("double init must refuse")
	}
	if _, err := InitProject(root, "Bad Key!"); err == nil {
		t.Fatal("a key outside the grammar must refuse")
	}

	s2, err := OpenProject(root, "loop-sys")
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer s2.Close()
	if n, err := s2.Verify(); err != nil || n != 1 {
		t.Fatalf("reopened ledger: n=%d err=%v", n, err)
	}

	if _, err := OpenProject(root, "other-sys"); err == nil {
		t.Fatal("missing key must refuse")
	} else if !strings.Contains(err.Error(), "loop-sys") {
		t.Errorf("refusal must name existing keys: %v", err)
	}

	keys, _ := ListProjects(root)
	if len(keys) != 1 || keys[0] != "loop-sys" {
		t.Errorf("ListProjects = %v", keys)
	}
}

// Proving red: a project selector is untrusted input, and the catalog is
// where it becomes a place. Init checked the key grammar; Open did not,
// and Open is what every verb but init goes through. So "..", "" and
// "demo/../.." each opened a ledger outside the project namespace — one
// of them the deployment's own, where the accountable-authority binding
// lives.
//
// The assertion is containment, not refusal alone: an escape leaves a
// database behind, so this walks the directory the state root sits in
// and fails on any looplaw.db outside root/projects/<key>. A refusal
// that still created the file would pass a refusal-only test.
func TestProjectSelectorAddressesNothingButAProject(t *testing.T) {
	top := t.TempDir()
	root := filepath.Join(top, "state")
	if err := os.MkdirAll(filepath.Join(top, "sibling"), 0o755); err != nil {
		t.Fatal(err)
	}

	s, err := InitProject(root, "demo")
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	s.Close()

	// Empty is the deployment ledger's own selector, dot and dot-dot
	// walk the namespace, the trailing forms differ from "demo" only by
	// what filepath.Join normalises away, and the rest carry separators
	// or a grammar the project key does not admit.
	for _, key := range []string{
		"", ".", "..", "../..", "demo/../..", "../sibling",
		"a/b", "/etc", "Demo", "demo ", "demo/", "demo/.",
	} {
		if s, err := OpenProject(root, key); err == nil {
			s.Close()
			t.Errorf("OpenProject accepted %q", key)
		}
		if s, err := InitProject(root, key); err == nil {
			s.Close()
			t.Errorf("InitProject accepted %q", key)
		}
	}

	namespace := filepath.Join(root, "projects", "demo")
	err = filepath.WalkDir(top, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasPrefix(d.Name(), "looplaw.db") {
			return err
		}
		if dir := filepath.Dir(path); dir != namespace {
			t.Errorf("a ledger stands outside the project namespace: %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// Proving red: the deployment ledger is a capability, not a project key.
// While the empty string selected it, an untrusted project argument
// could name it, and a party's ordinary submission landed in the ledger
// that records who holds the accountable authority.
func TestDeploymentLedgerIsNotReachableAsAProject(t *testing.T) {
	root := t.TempDir()

	d, err := OpenDeployment(root)
	if err != nil {
		t.Fatalf("open deployment: %v", err)
	}
	if _, err := d.Append(Evidence, "claim", "accountable-authority", "{}", "xor"); err != nil {
		t.Fatal(err)
	}
	d.Close()

	if s, err := OpenProject(root, ""); err == nil {
		s.Close()
		t.Error("the deployment ledger is addressable as a project")
	}
	if keys, _ := ListProjects(root); len(keys) != 0 {
		t.Errorf("the deployment ledger is listed as a project: %v", keys)
	}
}

// Proving red: the ledger holds the four ratified record kinds and
// nothing else. A coined type is refused at the only door in, before
// anything is written — which is what "goal-proposal" and "law-set"
// needed and did not have.
func TestLedgerRefusesAnUnratifiedRecordKind(t *testing.T) {
	s, err := OpenDeployment(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	for _, coined := range []string{"goal-proposal", "law-set", "note", ""} {
		if _, err := s.Append(Evidence, coined, "subject", "{}", "party"); err == nil {
			t.Errorf("the ledger accepted %q, which is not a record kind", coined)
		}
	}
	for _, kind := range RecordKinds {
		if _, err := s.Append(Evidence, kind, "subject", "{}", "party"); err != nil {
			t.Errorf("the ledger refused ratified kind %q: %v", kind, err)
		}
	}

	// A refused type writes nothing: the check runs before the chain is
	// read or extended, so a rejected batch cannot leave a partial act.
	recs, err := s.Records()
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != len(RecordKinds) {
		t.Errorf("want only the ratified appends recorded, got %d records", len(recs))
	}
}

// Proving red: the ledger holds submitted bodies, party names, the
// authority binding and the whole law and evidence history. It was
// created at whatever the umask allowed — 0755 directories and a 0644
// database under the common 022 — so on a host where the parents are
// traversable, every other local account could read it.
//
// A deployment that means to share a ledger says so by configuring it,
// which is a different thing from inheriting it from a umask nobody
// chose for this.
func TestStateIsCreatedPrivate(t *testing.T) {
	root := t.TempDir()
	s, err := InitProject(root, "demo")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Append(Evidence, "claim", "s", "b", "p"); err != nil {
		t.Fatal(err)
	}
	s.Close()

	dir := filepath.Join(root, "projects", "demo")
	assertMode(t, dir, 0o700)
	assertMode(t, filepath.Join(dir, "looplaw.db"), 0o600)

	// State an older binary left wide is narrowed when it is opened: a
	// fix that only covers what it creates leaves every ledger already
	// on disk exactly as exposed as it was.
	if err := os.Chmod(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(dir, "looplaw.db"), 0o644); err != nil {
		t.Fatal(err)
	}
	s2, err := OpenProject(root, "demo")
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()
	assertMode(t, dir, 0o700)
	assertMode(t, filepath.Join(dir, "looplaw.db"), 0o600)
}

func assertMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := fi.Mode().Perm(); got != want {
		t.Errorf("%s is %04o, want %04o", filepath.Base(path), got, want)
	}
}
