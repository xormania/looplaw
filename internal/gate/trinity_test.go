package gate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xormania/looplaw/internal/outcome"
)

const fixture = "testdata/library/set.cue"

// The green fixture must pass every gate. A red for the wrong reason
// proves nothing — so the mutation table below derives every red from
// this green by exactly one edit, and each test asserts the refusal
// names the mutated thing, not merely that something failed.
func TestFixtureZeroIsGreen(t *testing.T) {
	refusals := ValidateTrinity(fixture)
	for _, r := range refusals {
		t.Errorf("green fixture refused: %s", r.Error())
	}
}

func TestMutationsAreRedForTheirDeclaredReason(t *testing.T) {
	base, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}

	mutations := []struct {
		name      string
		old, new  string
		wantCheck string
		wantIn    string // substring the refusal must name (the mutated thing)
	}{
		{
			name:      "act-double-coverage",
			old:       `acts: ["return"]`,
			new:       `acts: ["return", "lend"]`,
			wantCheck: "trinity/act-closure",
			wantIn:    `"lend"`,
		},
		{
			name:      "actless-contract",
			old:       `acts: ["return"]`,
			new:       `acts: []`,
			wantCheck: "trinity/act-closure",
			wantIn:    "C-RETURN-1",
		},
		{
			name:      "unregistered-party",
			old:       "client:   \"borrower\"\n\t\t\tsupplier: \"librarian\"\n\t\t}\n\t\tacts: [\"lend\"]",
			new:       "client:   \"stranger\"\n\t\t\tsupplier: \"librarian\"\n\t\t}\n\t\tacts: [\"lend\"]",
			wantCheck: "trinity/party-resolve",
			wantIn:    `"stranger"`,
		},
		{
			name:      "dangling-invariant-cite",
			old:       `cites: ["L-1"]
		blame: [
			{violation_class: "late return"`,
			new: `cites: ["L-9"]
		blame: [
			{violation_class: "late return"`,
			wantCheck: "trinity/cite-resolve",
			wantIn:    `"L-9"`,
		},
		{
			name:      "blame-names-nobody",
			old:       `at_fault: "borrower", evidence: "the member records at submission"`,
			new:       `at_fault: "the-void", evidence: "the member records at submission"`,
			wantCheck: "trinity/blame-resolve",
			wantIn:    `"the-void"`,
		},
		{
			name:      "uncovered-party",
			old:       "registry: {\n\tlibrarian: {",
			new:       "registry: {\n\tauditor: {\n\t\tname:           \"the auditor\"\n\t\tnote:           \"registered but wired to nothing\"\n\t\tauthority_free: true\n\t}\n\tlibrarian: {",
			wantCheck: "trinity/party-coverage",
			wantIn:    `"auditor"`,
		},
		{
			name:      "dangling-related-term",
			old:       `related: ["due date"]`,
			new:       `related: ["overdue notice"]`,
			wantCheck: "trinity/related-resolve",
			wantIn:    `"overdue notice"`,
		},
		{
			name:      "term-authority-unregistered",
			old:       "definition: \"The recorded standing created by the librarian's lend act: one book, one borrower, one due date. Only the lend act creates a loan; return retires it.\"\n\t\tauthority:  \"librarian\"",
			new:       "definition: \"The recorded standing created by the librarian's lend act: one book, one borrower, one due date. Only the lend act creates a loan; return retires it.\"\n\t\tauthority:  \"head-office\"",
			wantCheck: "trinity/authority-resolve",
			wantIn:    `"head-office"`,
		},
		{
			name:      "floating-judgment",
			old:       `cites: ["C-LEND-1"]`,
			new:       `cites: ["C-VIBES-1"]`,
			wantCheck: "trinity/experience-cite-resolve",
			wantIn:    `"C-VIBES-1"`,
		},
		{
			name:      "shape-bad-status",
			old:       `rewrite:    "The borrower requested renewal; the librarian's lend act created the successor loan."
		status:     "ratified"`,
			new: `rewrite:    "The borrower requested renewal; the librarian's lend act created the successor loan."
		status:     "vibes"`,
			wantCheck: "trinity/shape",
			wantIn:    "status",
		},
		{
			name:      "shape-experience-not-advisory",
			old:       `advisory: true`,
			new:       `advisory: false`,
			wantCheck: "trinity/shape",
			wantIn:    "advisory",
		},
		{
			name:      "shape-missing-absence-declaration",
			old:       "experience_declared_absent: false\n",
			new:       "",
			wantCheck: "trinity/shape",
			wantIn:    "experience_declared_absent",
		},
	}

	for _, m := range mutations {
		t.Run(m.name, func(t *testing.T) {
			if !strings.Contains(string(base), m.old) {
				t.Fatalf("mutation target not found in fixture — the mutation table drifted from the fixture: %q", m.old)
			}
			mutated := strings.Replace(string(base), m.old, m.new, 1)

			dir := t.TempDir()
			path := filepath.Join(dir, "set.cue")
			if err := os.WriteFile(path, []byte(mutated), 0o644); err != nil {
				t.Fatal(err)
			}

			refusals := ValidateTrinity(path)
			if len(refusals) == 0 {
				t.Fatalf("mutation accepted — the gate is blind to %s", m.name)
			}
			found := false
			for _, r := range refusals {
				if r.Check == m.wantCheck && strings.Contains(r.Error(), m.wantIn) {
					found = true
				}
				if r.Class != outcome.Rejection {
					t.Errorf("refusal class = %s, want rejection (malformed submission): %s", r.Class, r.Error())
				}
				if r.Remedy == "" {
					t.Errorf("refusal without a remedy: %s", r.Error())
				}
			}
			if !found {
				var got []string
				for _, r := range refusals {
					got = append(got, r.Error())
				}
				t.Errorf("red, but not for the declared reason.\nwant check %s naming %s\ngot:\n  %s",
					m.wantCheck, m.wantIn, strings.Join(got, "\n  "))
			}
		})
	}
}

func TestUnreadablePathAborts(t *testing.T) {
	refusals := ValidateTrinity("testdata/does-not-exist.cue")
	if len(refusals) != 1 || refusals[0].Class != outcome.Abort {
		t.Fatalf("want a single abort, got %+v", refusals)
	}
}
