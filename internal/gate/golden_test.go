package gate

import (
	"strings"
	"testing"

	"github.com/xormania/looplaw/internal/golden"
)

// The refusal stream is protocol: agents retry off it, so both its
// content and its order are recorded. Order is part of the contract —
// identical input, identical stream, run to run.
func TestRefusalStreamGolden(t *testing.T) {
	for _, tc := range []struct{ name, attack, file string }{
		{"ungrounded", "testdata/attacks/ungrounded-interior.cue", "testdata/golden/refusals-ungrounded.txt"},
		{"unsourced", "testdata/attacks/provenance-unsourced-contract.cue", "testdata/golden/refusals-unsourced.txt"},
		{"vacuous", "testdata/attacks/vacuous-set.cue", "testdata/golden/refusals-vacuous.txt"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var b strings.Builder
			for _, r := range ValidateTrinity(tc.attack) {
				// The path is where the input happened to live, not
				// part of the refusal's meaning: recording it would
				// make every file move rewrite outputs that did not
				// change.
				b.WriteString(strings.ReplaceAll(r.Error(), tc.attack, "<input>") + "\n")
			}
			golden.Assert(t, tc.file, b.String())
		})
	}
}
