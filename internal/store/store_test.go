package store

import (
	"fmt"
	"strings"
	"sync"
	"testing"
)

func open(t *testing.T) *Store {
	t.Helper()
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestAppendChainsAndVerifies(t *testing.T) {
	s := open(t)

	r1, err := s.Append(Law, "admission", "registry-batch-1", `{"pr":4}`, "aa:test")
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
	s := open(t)
	if _, err := s.Append(Law, "admission", "a", "body-a", "t"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Append(Law, "admission", "b", "body-b", "t"); err != nil {
		t.Fatal(err)
	}

	if _, err := s.db.Exec("UPDATE records SET body = 'forged' WHERE seq = 1"); err != nil {
		t.Fatalf("tamper: %v", err)
	}
	if _, err := s.Verify(); err == nil {
		t.Fatal("verify accepted a tampered record")
	} else if !strings.Contains(err.Error(), "seq 1") {
		t.Errorf("verify error names wrong record: %v", err)
	}
}

func TestDeletionBreaksChain(t *testing.T) {
	s := open(t)
	for _, subj := range []string{"a", "b", "c"} {
		if _, err := s.Append(Evidence, "claim", subj, "x", "t"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.db.Exec("DELETE FROM records WHERE seq = 2"); err != nil {
		t.Fatal(err)
	}
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
