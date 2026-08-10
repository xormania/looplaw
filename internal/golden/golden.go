// Package golden compares output against a recorded file. The wire
// formats consumers script against — the gap feed, the staleness
// report, the refusal stream — change shape only deliberately: a golden
// mismatch is a contract change asking to be noticed, not a test to
// silence.
//
// Record with: LOOPLAW_GOLDEN_UPDATE=1 go test ./... -count=1
//
// Through the environment rather than a flag: a flag exists only in
// packages that import this one, so a repository-wide "go test ./...
// -update" fails in every package that has no goldens.
package golden

import (
	"os"
	"path/filepath"
	"testing"
)

func updating() bool { return os.Getenv("LOOPLAW_GOLDEN_UPDATE") != "" }

// Assert compares got against the recorded golden file, or records it
// when -update is passed.
func Assert(t *testing.T, path, got string) {
	t.Helper()
	if updating() {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("golden updated: %s", path)
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("golden %s missing: %v — run LOOPLAW_GOLDEN_UPDATE=1 go test ./... -count=1 to record it", path, err)
	}
	if string(want) != got {
		t.Errorf("output differs from golden %s.\nIf the change is deliberate, re-record with LOOPLAW_GOLDEN_UPDATE=1 and say why in the message.\n--- want ---\n%s\n--- got ---\n%s",
			path, want, got)
	}
}
