package store

import "testing"

// The ledger's integrity rests on the canonical form being injective:
// distinct record field tuples must never produce the same canonical
// bytes, or two different facts share a hash and the chain's tamper
// evidence is worthless.
func FuzzCanonicalIsInjective(f *testing.F) {
	f.Add("claim", "subject", "body", "actor")
	f.Add("claim|subject", "body", "actor", "")
	f.Add("", "", "", "")
	f.Add("a|b|c", "d", "e", "f")

	f.Fuzz(func(t *testing.T, rectype, subject, body, actor string) {
		const at, prev = "2026-01-01T00:00:00Z", ""
		a := canonical(Law, rectype, subject, body, actor, at, prev)
		for _, b := range []struct{ rectype, subject, body, actor string }{
			{subject, rectype, body, actor},
			{rectype, subject, actor, body},
			{rectype + subject, "", body, actor},
			{rectype, subject, body, actor + "x"},
		} {
			if b.rectype == rectype && b.subject == subject && b.body == body && b.actor == actor {
				continue
			}
			if canonical(Law, b.rectype, b.subject, b.body, b.actor, at, prev) == a {
				t.Fatalf("distinct records share a canonical form:\n a = %q %q %q %q\n b = %q %q %q %q",
					rectype, subject, body, actor, b.rectype, b.subject, b.body, b.actor)
			}
		}
		// The kind marker must be part of identity: a law-side and an
		// evidence-side record with identical fields are different facts.
		if canonical(Evidence, rectype, subject, body, actor, at, prev) == a {
			t.Fatal("law-side and evidence-side records share a canonical form")
		}
	})
}
