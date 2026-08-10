package diff

import "testing"

// The delimiter-injection class, fuzzed. A clause's identity is the pair
// (text, records); distinct pairs must never hash equal, or materially
// different guarantees compare as equilibrium and the staleness hashes
// collide. This is the property a crafted NUL boundary once broke, so a
// machine hunts it from here on rather than a reviewer.
func FuzzClauseHashIsInjective(f *testing.F) {
	f.Add("The loan is retired.", "the return record")
	f.Add("A\x00records:B", "")
	f.Add("", "A\x00records:B")
	f.Add("x", "y")
	f.Add("xy", "")

	f.Fuzz(func(t *testing.T, text, records string) {
		a := clauseInfo{text: text, records: records}
		// Every neighbour that differs in either field must hash apart.
		for _, b := range []clauseInfo{
			{text: text + "x", records: records},
			{text: text, records: records + "x"},
			{text: records, records: text},
			{text: text + "\x00records:" + records, records: ""},
			{text: "", records: text + "\x00records:" + records},
		} {
			if a == b {
				continue // genuinely the same clause
			}
			if a.hash() == b.hash() {
				t.Fatalf("distinct clauses hash equal:\n a = %q / %q\n b = %q / %q",
					a.text, a.records, b.text, b.records)
			}
		}
	})
}
