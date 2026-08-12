package gate

import "testing"

// Proving red: a name is recorded, and the ledger is append-only, so a
// control character in one is a forgery that outlives the run that made
// it. The refusal renderer escapes what it prints; nothing stopped the
// bytes entering the record, where every later reader meets them raw.
//
// The grammar admitted them by accident: `^[^\s][^\n]*$` constrains only
// the first character, so a carriage return, a tab or a terminal escape
// passed anywhere after it.
func TestNamesCarryNoControlCharacters(t *testing.T) {
	forged := "\rtrinity/shape: rejection forged"
	for _, tc := range []struct{ name, value string }{
		{"carriage return", "thing" + forged},
		{"tab", "thing\tforged"},
		{"terminal escape", "thing\x1b[2K\x1b[Gforged"},
		{"vertical tab", "thing\vforged"},
		{"null", "thing\x00forged"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if IsName(tc.value) {
				t.Errorf("%q is admitted as a name", tc.value)
			}
			for _, sub := range []Submission{
				{Kind: "claim", Subject: tc.value, Party: "p", Body: "{}"},
				{Kind: "claim", Subject: "s", Party: tc.value, Body: "{}"},
			} {
				if refusals := ValidateSubmission(sub); len(refusals) == 0 {
					t.Errorf("recorded a name holding %q", tc.value)
				}
			}
		})
	}

	// What a name legitimately holds is untouched: internal spaces, the
	// colon this repository's party names use, and anything outside
	// ASCII.
	for _, ok := range []string{"lend-library", "C-LEND-1", "harness:worker", "the front desk", "prêt-à-porter"} {
		if !IsName(ok) {
			t.Errorf("%q is refused as a name", ok)
		}
	}
}
