package record

import (
	"regexp"
	"testing"

	"github.com/xormania/looplaw/internal/golden"
)

// The ledger export is a wire format: consumers read recorded facts
// from it, so its shape changes only deliberately. Timestamps and the
// hashes that depend on them are masked — they are the one part that
// cannot be identical run to run, and masking them keeps the rest
// honestly compared.
func TestLedgerExportGolden(t *testing.T) {
	s := open(t)
	if _, refusals := Submit(s, goodClaim()); len(refusals) != 0 {
		t.Fatalf("refused: %+v", refusals)
	}
	if _, refusals := Submit(s, goodReceipt()); len(refusals) != 0 {
		t.Fatalf("refused: %+v", refusals)
	}
	out, refusal := Export(s)
	if refusal != nil {
		t.Fatal(refusal.Error())
	}
	masked := regexp.MustCompile(`"(at|hash|prev)": "[^"]*"`).ReplaceAllString(out, `"$1": "<masked>"`)
	golden.Assert(t, "testdata/golden/ledger-export.json", masked)
}
