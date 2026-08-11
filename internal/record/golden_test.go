package record

import (
	"regexp"
	"testing"
	"time"

	"github.com/xormania/looplaw/internal/golden"
	"github.com/xormania/looplaw/internal/store"
)

// The ledger export is a wire format: consumers read recorded facts from
// it, so its shape changes only deliberately.
//
// Nothing is masked. Timestamps and the hashes derived from them used to
// be, because the clock was the wall — and a masked hash is a hash no
// golden pins, so the canonical form and the hash function were both
// unguarded. With store.Clock fixed the chain is reproducible and the
// real values are compared: changing the canonical delimiter now fails
// this golden, where before it failed nothing.
func TestLedgerExportGolden(t *testing.T) {
	fixedClock(t)
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
	// Unmasked: a fixed clock makes the chain reproducible, so this pins
	// the real hashes rather than a shape with the chain cut out of it.
	golden.Assert(t, "testdata/golden/ledger-export.json", out)
}

// The law path, end to end, as the ledger records it: bind an authority,
// declare a draft, ratify it. The submission golden above holds only a
// claim and a receipt, so no act's admission body ever reached a golden —
// which is how six wire keys were renamed with the whole suite green.
//
// Nothing here is masked. Renaming a wire key changes this file, so the
// golden fails rather than quietly following the code.
func TestLawPathLedgerGolden(t *testing.T) {
	fixedClock(t)
	d, p := declareStore(t), declareStore(t)
	if _, refusal := BindAuthority(d, "xor", "xor"); refusal != nil {
		t.Fatal(refusal)
	}
	if _, refusals := Declare(p, fixtureZero, "harness:worker"); len(refusals) > 0 {
		t.Fatal(refusals)
	}
	if _, refusals := Ratify(d, p, "lend-library", "xor"); len(refusals) > 0 {
		t.Fatal(refusals)
	}

	deployment, refusal := Export(d)
	if refusal != nil {
		t.Fatal(refusal.Error())
	}
	project, refusal := Export(p)
	if refusal != nil {
		t.Fatal(refusal.Error())
	}
	out := "== deployment ==\n" + deployment + "== project ==\n" + project

	// The set-body of a ratified version is the whole fixture; its shape
	// is pinned by that fixture's own goldens, so it is collapsed here to
	// keep this golden about the ledger.
	// The ratified set is the whole fixture; its shape is pinned by that
	// fixture's own goldens, so it collapses here to keep this golden
	// about the ledger. A JSON string carries escaped quotes, so the
	// pattern must span them — "[^"]*" stops at the first \" and mangles
	// the rest of the record.
	out = regexp.MustCompile(`"body": "(?:// Fixture|subject:)(?:[^"\\]|\\.)*"`).
		ReplaceAllString(out, `"body": "<the ratified set>"`)

	// Nothing is masked. The clock is fixed, so the timestamps and every
	// hash derived from them are reproducible — which means this golden
	// pins the chain itself rather than a shape with the chain cut out.
	// A mask over "hash" hides the canonical form and the hash function,
	// which are the two things here most worth pinning.
	golden.Assert(t, "testdata/golden/law-path-ledger.json", out)
}

// fixedClock pins the ledger's clock for the duration of a test.
//
// Duplicated from internal/store's own helper rather than shared: a Go
// test helper is invisible outside its package, and the alternatives are
// worse — putting it in the product would ship a mutation path for a
// test's benefit, which is why Tamper was removed from the store, and a
// package existing for five lines is more machinery than the duplication
// it saves.
func fixedClock(t *testing.T) {
	t.Helper()
	at, err := time.Parse(time.RFC3339Nano, "2026-01-01T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	prev := store.Clock
	t.Cleanup(func() { store.Clock = prev })
	store.Clock = func() time.Time { return at }
}
