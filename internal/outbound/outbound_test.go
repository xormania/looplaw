package outbound

import (
	"strings"
	"testing"

	"github.com/xormania/looplaw/internal/outcome"
)

// The default decides nothing, and says so. Standing in for a gate is
// not the same as being one; a placeholder that appeared to check would
// be worse than the gap it names.
func TestOpenGateHandsContentBackUnchanged(t *testing.T) {
	got, refusal := Release(Request{Party: "p", Purpose: "export", Content: "held"})
	if refusal != nil || got != "held" {
		t.Fatalf("the open gate altered or refused content: %q %v", got, refusal)
	}
}

// The seam is real: a gate that denies stops content leaving, and every
// path that emits held content goes through it.
func TestSubstitutedGateCanRefuseAndRewrite(t *testing.T) {
	prev := Default
	t.Cleanup(func() { Default = prev })

	Default = denyExport{}
	if _, refusal := Release(Request{Party: "p", Purpose: "export", Content: "held"}); refusal == nil {
		t.Error("a substituted gate could not stop content leaving")
	} else if refusal.Class != outcome.Denial {
		t.Errorf("refusing to release is a denial, got %v", refusal.Class)
	}

	// A gate may also return less than it was given, which is what a
	// disclosure-scoped answer looks like.
	got, refusal := Release(Request{Party: "p", Purpose: "diff", Content: "a\nb\nc"})
	if refusal != nil {
		t.Fatal(refusal)
	}
	if got != "a" {
		t.Errorf("gate could not narrow what leaves: %q", got)
	}
}

type denyExport struct{}

func (denyExport) Release(r Request) (string, *outcome.Refusal) {
	if r.Purpose == "export" {
		return "", &outcome.Refusal{
			Class: outcome.Denial, Check: "custody/release", Subject: r.Subject,
			Reason: "the deciding authority declines this release",
			Remedy: "request the release through the custody system",
		}
	}
	return strings.SplitN(r.Content, "\n", 2)[0], nil
}
