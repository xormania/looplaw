package store

import "testing"

// The ledger's integrity rests on the canonical form being injective:
// distinct record field tuples must never produce the same canonical
// bytes, or two different facts share a hash and the chain's tamper
// evidence is worthless.
func FuzzCanonicalIsInjective(f *testing.F) {
	f.Add("claim", "subject", "body", "party")
	f.Add("claim|subject", "body", "party", "")
	f.Add("", "", "", "")
	f.Add("a|b|c", "d", "e", "f")

	f.Fuzz(func(t *testing.T, rectype, subject, body, party string) {
		const at, prev = "2026-01-01T00:00:00Z", ""
		a := canonical(Law, rectype, subject, body, party, at, prev)
		for _, b := range []struct{ rectype, subject, body, party string }{
			{subject, rectype, body, party},
			{rectype, subject, party, body},
			{rectype + subject, "", body, party},
			{rectype, subject, body, party + "x"},
		} {
			if b.rectype == rectype && b.subject == subject && b.body == body && b.party == party {
				continue
			}
			if canonical(Law, b.rectype, b.subject, b.body, b.party, at, prev) == a {
				t.Fatalf("distinct records share a canonical form:\n a = %q %q %q %q\n b = %q %q %q %q",
					rectype, subject, body, party, b.rectype, b.subject, b.body, b.party)
			}
		}
		// The kind marker must be part of identity: a law-side and an
		// evidence-side record with identical fields are different facts.
		if canonical(Evidence, rectype, subject, body, party, at, prev) == a {
			t.Fatal("law-side and evidence-side records share a canonical form")
		}
	})
}
