package gate

import "testing"

// The gates read untrusted input: any bytes a party submits reach the
// parser. Whatever arrives, the answer is a refusal carrying a remedy —
// never a panic, and never an accept for input the law does not admit.
func FuzzValidateNeverPanics(f *testing.F) {
	for _, seed := range []string{
		"", "{", "subject: 1", "subject: \"x\"\nschema_version: \"0\"\n",
		"contracts: 42", "lexicon: {\"\": {}}", "provenance: {sources: {x: \"y\"}}",
	} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		_, _, refusals := validateTrinityBytes("fuzz.cue", data)
		for _, r := range refusals {
			if r.Check == "" {
				t.Error("refusal without a check id")
			}
			if r.Remedy == "" {
				t.Errorf("refusal without a remedy: %s", r.Error())
			}
		}
	})
}
