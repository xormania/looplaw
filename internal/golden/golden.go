// Package golden compares output against a recorded file. The wire
// formats consumers script against — the gap feed, the staleness
// report, the refusal stream — change shape only deliberately: a golden
// mismatch is a contract change asking to be noticed, not a test to
// silence.
//
// Update with: go test ./... -update
package golden

import (
	"flag"
	"os"
	"path/filepath"
	"testing"
)

var update = flag.Bool("update", false, "rewrite golden files from current output")

// Assert compares got against the recorded golden file, or records it
// when -update is passed.
func Assert(t *testing.T, path, got string) {
	t.Helper()
	if *update {
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
		t.Fatalf("golden %s missing: %v — run go test ./... -update to record it", path, err)
	}
	if string(want) != got {
		t.Errorf("output differs from golden %s.\nIf the change is deliberate, re-record with -update and say why in the message.\n--- want ---\n%s\n--- got ---\n%s",
			path, want, got)
	}
}
