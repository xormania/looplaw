package absorb

import (
	"encoding/json"
	"testing"

	"github.com/xormania/looplaw/internal/golden"
	"github.com/xormania/looplaw/internal/provenance"
)

// The staleness report is a wire format: it tells a client which
// statements are owed a re-derivation, so its shape changes only
// deliberately.
func TestStalenessReportGolden(t *testing.T) {
	m, err := ScanScope(scope, "scope")
	if err != nil {
		t.Fatal(err)
	}
	m.Sources["lending.go"] = "0000000000000000000000000000000000000000000000000000000000000000"
	delete(m.Sources, "returning.go")
	m.Sources["renewals.go"] = "1111111111111111111111111111111111111111111111111111111111111111"

	rep := provenance.Compare(viewProvenance(t, view), m)
	out, err := json.MarshalIndent(rep, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	golden.Assert(t, "testdata/golden/staleness-report.json", string(out)+"\n")
}

// The skeleton is what the authoring caller receives; its shape is the
// hand-off contract.
func TestSkeletonGolden(t *testing.T) {
	m, err := ScanScope(scope, "scope")
	if err != nil {
		t.Fatal(err)
	}
	golden.Assert(t, "testdata/golden/skeleton.cue", Skeleton("lend-library", m))
}
