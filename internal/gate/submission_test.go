package gate

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/xormania/looplaw/internal/outcome"
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

// Proving red: every read was unbounded, so a 16 MiB claim was
// allocated, hashed and stored, growing the ledger by the same amount —
// and repetition scaled it linearly. Any wrapper putting these commands
// in front of an untrusted submitter could spend a deployment's memory
// and disk at the submitter's choosing.
//
// The bound is checked on the bytes as well as at the read, so a caller
// that is not the command line meets it too.
func TestASubmissionIsBounded(t *testing.T) {
	body := strings.Repeat("a", MaxBytes+1)
	_, refusals := ValidateSubmission(Submission{Kind: "claim", Subject: "s", Party: "p", Body: body})
	if len(refusals) == 0 {
		t.Fatalf("a %d-byte body passed the gates", len(body))
	}
	if refusals[0].Check != "submit/content" {
		t.Errorf("want submit/content, got %s", refusals[0].Check)
	}

	// The bound refuses an attack without meeting honest work: the
	// largest CUE in this repository is its own design basis, well
	// inside it.
	if _, refusals := ValidateSubmission(Submission{
		Kind: "claim", Subject: "s", Party: "p", Body: strings.Repeat("a", MaxBytes),
	}); len(refusals) != 0 {
		t.Errorf("a body at the bound was refused: %v", refusals)
	}
}

// And a set file, bounded before the allocation rather than after it: a
// reader that allocates whatever it is handed has already paid the cost
// by the time it could refuse.
func TestASetFileIsBounded(t *testing.T) {
	path := filepath.Join(t.TempDir(), "big.cue")
	if err := os.WriteFile(path, []byte(strings.Repeat("a", MaxBytes+1)), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadSetFile(path); err == nil {
		t.Fatal("an oversized set file was read whole")
	}

	refusals := ValidateTrinity(path)
	if len(refusals) == 0 || refusals[0].Check != "trinity/load" {
		t.Fatalf("want trinity/load, got %v", refusals)
	}
	if refusals[0].Class != outcome.Abort {
		t.Errorf("class = %s, want abort", refusals[0].Class)
	}
}
