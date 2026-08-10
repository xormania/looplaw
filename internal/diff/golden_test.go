package diff

import (
	"encoding/json"
	"testing"

	"github.com/xormania/looplaw/internal/golden"
)

// The gap feed is a wire format: loopstrap and any planner script
// against its shape, so it changes only deliberately.
func TestGapFeedGolden(t *testing.T) {
	gaps, refusals := Diff(goal, view)
	if len(refusals) != 0 {
		t.Fatalf("refused: %+v", refusals)
	}
	out, err := json.MarshalIndent(gaps, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	golden.Assert(t, "testdata/golden/gap-feed.json", string(out)+"\n")
}

// The refusal stream is protocol too — callers retry off it.
func TestDiffRefusalStreamGolden(t *testing.T) {
	_, refusals := Diff(goal, "testdata/library-view-split.cue")
	if len(refusals) != 0 {
		t.Fatalf("the split view must diff cleanly: %+v", refusals)
	}
	gaps, _ := Diff(goal, "testdata/library-view-split.cue")
	out, err := json.MarshalIndent(gaps, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	golden.Assert(t, "testdata/golden/gap-feed-split.json", string(out)+"\n")
}
