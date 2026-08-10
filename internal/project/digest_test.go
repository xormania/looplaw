package project

import (
	"os"
	"strings"
	"testing"

	"github.com/xormania/looplaw/internal/golden"
)

// The digest is what agents and readers consult instead of the corpus,
// so it is generated from the embedded law and checked against the
// committed copy: a drifted digest would brief its reader on law the
// gates do not enforce.
func TestLawDigestMatchesTheCommittedCopy(t *testing.T) {
	got, err := LawDigest()
	if err != nil {
		t.Fatal(err)
	}
	committed, err := os.ReadFile("../../law/DIGEST.md")
	if err != nil {
		t.Fatalf("law/DIGEST.md missing: %v — regenerate with go run ./cmd/looplaw project law > law/DIGEST.md", err)
	}
	if string(committed) != got {
		t.Error("law/DIGEST.md is stale — regenerate with: go run ./cmd/looplaw project law > law/DIGEST.md")
	}
	golden.Assert(t, "testdata/golden/law-digest.md", got)
}

// The digest must carry the load-bearing content, or it quietly becomes
// a summary that omits the rules its readers rely on.
func TestLawDigestCarriesTheEssentials(t *testing.T) {
	got, err := LawDigest()
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"T0-1", "T0-9", // the whole invariant tier
		"ratify", "record", // acts
		"claim", "gap", // reserved terms
		"rollback", "authorize", // refused vocabulary
		"recorded, never believed",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("digest omits %q", want)
		}
	}
}

// Deterministic: same law in, same brief out.
func TestLawDigestIsDeterministic(t *testing.T) {
	a, _ := LawDigest()
	b, _ := LawDigest()
	if a != b {
		t.Fatal("two renderings of one law differ")
	}
}
