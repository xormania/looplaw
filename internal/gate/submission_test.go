package gate

import (
	"slices"
	"strings"
	"testing"
)

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
				if _, refusals := ValidateSubmission(sub); len(refusals) == 0 {
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

// Proving red: the submission gate reports the checks it can emit, and
// record.Submit wrote that list into every admission. submit/receipt-shape
// runs only for a receipt, so a plain claim's admission claimed a check
// that never touched it — the same laundering the ratify admission
// carried, from the same cause: a registry recorded where an execution
// record belongs.
func TestSubmissionReportsTheChecksItRan(t *testing.T) {
	emittable := map[string]bool{}
	for _, c := range SubmissionChecks {
		emittable[c] = true
	}

	for _, tc := range []struct {
		name    string
		sub     Submission
		wantRun []string
	}{
		{
			name:    "a claim never reaches the receipt shape",
			sub:     Submission{Kind: "claim", Subject: "s", Party: "p", Body: "{}"},
			wantRun: []string{"submit/kind", "submit/subject", "submit/party", "submit/content"},
		},
		{
			name:    "a receipt does",
			sub:     Submission{Kind: "receipt", Subject: "s", Party: "p", Body: `{"subject":"s","verdict":"pass","source":"ci","hash":"` + strings.Repeat("a", 64) + `"}`},
			wantRun: SubmissionChecks,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ran, refusals := ValidateSubmission(tc.sub)
			if len(refusals) > 0 {
				t.Fatalf("the submission must pass, or it reports a partial run: %v", refusals)
			}
			if !slices.Equal(ran, tc.wantRun) {
				t.Errorf("reported %v, want %v", ran, tc.wantRun)
			}
			for _, c := range ran {
				if !emittable[c] {
					t.Errorf("%s is reported as run and is not a check this gate can emit", c)
				}
			}
		})
	}

	// A refused submission reports what it reached, not the whole list:
	// a kind the gate rejects is refused before the body is read.
	ran, refusals := ValidateSubmission(Submission{Kind: "version", Subject: "s", Party: "p", Body: "{}"})
	if len(refusals) == 0 {
		t.Fatal("a kind no party may submit was admitted")
	}
	if slices.Contains(ran, "submit/receipt-shape") {
		t.Errorf("a refused claim reports the receipt shape as run: %v", ran)
	}
}
