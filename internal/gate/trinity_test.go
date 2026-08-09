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

type mutation struct {
	name      string
	old, new  string
	wantCheck string
	wantIn    string // substring the refusal must name (the mutated thing)
}

var mutations = []mutation{
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
		name: "dangling-invariant-cite",
		old: `cites: ["L-1"]
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
		old:       `cites: ["C-LEND-1", "L-1"]`,
		new:       `cites: ["C-VIBES-1", "L-1"]`,
		wantCheck: "trinity/experience-cite-resolve",
		wantIn:    `"C-VIBES-1"`,
	},
	{
		name: "shape-bad-status",
		old: `rewrite:    "The borrower requested renewal; the librarian's lend act created the successor loan."
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
	{
		name:      "unparseable-set",
		old:       "subject:        \"lend-library\"",
		new:       "subject:        \"lend-library\" %%% not cue",
		wantCheck: "trinity/parse",
		wantIn:    "set.cue",
	},
	{
		// The supplier side must have its own direct witness: coverage of
		// the client side proves nothing about the supplier branch.
		name:      "unregistered-supplier",
		old:       "client:   \"borrower\"\n\t\t\tsupplier: \"librarian\"\n\t\t}\n\t\tacts: [\"return\"]",
		new:       "client:   \"borrower\"\n\t\t\tsupplier: \"phantom\"\n\t\t}\n\t\tacts: [\"return\"]",
		wantCheck: "trinity/party-resolve",
		wantIn:    `"phantom"`,
	},
	{
		// Empty-string references are shape-illegal (reference grammar),
		// so they can never slip past the relational guards.
		name:      "empty-at-fault",
		old:       `at_fault: "borrower", evidence: "the loan record's due date against the return record's date"`,
		new:       `at_fault: "", evidence: "the loan record's due date against the return record's date"`,
		wantCheck: "trinity/shape",
		wantIn:    "at_fault",
	},
	{
		name:      "empty-act",
		old:       `acts: ["lend"]`,
		new:       `acts: [""]`,
		wantCheck: "trinity/shape",
		wantIn:    "acts",
	},
	{
		name:      "same-contract-duplicate-act",
		old:       `acts: ["lend"]`,
		new:       `acts: ["lend", "lend"]`,
		wantCheck: "trinity/act-closure",
		wantIn:    "held twice by C-LEND-1",
	},
	{
		// A homoglyph fork of a term key (Cyrillic а) is a collision
		// generator and must be refused by shape.
		name:      "homoglyph-term-key",
		old:       "lexicon: {\n\tloan: {",
		new:       "lexicon: {\n\t\"loаn\": {",
		wantCheck: "trinity/shape",
		wantIn:    "loаn",
	},
	{
		name:      "floating-empty-cites",
		old:       `cites: ["C-LEND-1", "L-1"]`,
		new:       `cites: []`,
		wantCheck: "trinity/shape",
		wantIn:    "cites",
	},
	{
		name:      "dead-invariant",
		old:       "invariants: {\n\t\"L-1\": {",
		new:       "invariants: {\n\t\"L-9\": {\n\t\ttext:      \"No book is lent to a party holding an overdue loan.\"\n\t\trationale: \"authored but bound to nothing — dead law for this red\"\n\t}\n\t\"L-1\": {",
		wantCheck: "trinity/invariant-coverage",
		wantIn:    `"L-9"`,
	},
	{
		name:      "authority-free-supplier",
		old:       "note:           \"holds the lending authority: lend and return are its acts\"\n\t\tauthority_free: false",
		new:       "note:           \"holds the lending authority: lend and return are its acts\"\n\t\tauthority_free: true",
		wantCheck: "trinity/authority-free",
		wantIn:    `"librarian"`,
	},
}

func TestMutationsAreRedForTheirDeclaredReason(t *testing.T) {
	base, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
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
				if r.Class != outcome.Rejection && r.Class != outcome.Finding {
					t.Errorf("refusal class = %s, want rejection or finding: %s", r.Class, r.Error())
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

// A valid set written in ordinary DRY CUE — references like
// registry.librarian.id instead of string literals — must be green: the
// relational lane reads the unified value, where references resolve.
func TestReferencesResolveGreen(t *testing.T) {
	base, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	refStyled := strings.ReplaceAll(string(base), `supplier: "librarian"`, `supplier: registry.librarian.id`)
	if refStyled == string(base) {
		t.Fatal("reference rewrite did not apply — fixture drifted")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "set.cue")
	if err := os.WriteFile(path, []byte(refStyled), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, r := range ValidateTrinity(path) {
		t.Errorf("reference-styled valid set refused: %s", r.Error())
	}
}

// A set with no parties or contracts binds nothing and is refused —
// silence is not a declaration.
func TestVacuousSetRefused(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "set.cue")
	vacuous := "subject: \"empty-sys\"\nschema_version: \"0\"\nexperience_declared_absent: true\n"
	if err := os.WriteFile(path, []byte(vacuous), 0o644); err != nil {
		t.Fatal(err)
	}
	refusals := ValidateTrinity(path)
	found := false
	for _, r := range refusals {
		if r.Check == "trinity/vacuity" && r.Class == outcome.Rejection {
			found = true
		}
	}
	if !found {
		t.Fatalf("vacuous set not refused by trinity/vacuity; got %+v", refusals)
	}
}

// A present-but-unreadable region is a first-class Finding, never a
// silent skip.
func TestUnreadableRegionIsFinding(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "set.cue")
	bad := "subject: \"bad-sys\"\nschema_version: \"0\"\nexperience_declared_absent: true\ncontracts: 42\n"
	if err := os.WriteFile(path, []byte(bad), 0o644); err != nil {
		t.Fatal(err)
	}
	refusals := ValidateTrinity(path)
	found := false
	for _, r := range refusals {
		if r.Check == "trinity/region-unreadable" {
			if r.Class != outcome.Finding {
				t.Errorf("region-unreadable class = %s, want finding", r.Class)
			}
			found = true
		}
	}
	if !found {
		t.Fatalf("unreadable contracts region produced no finding; got %+v", refusals)
	}
}

func TestUnreadablePathAborts(t *testing.T) {
	refusals := ValidateTrinity("testdata/does-not-exist.cue")
	if len(refusals) != 1 || refusals[0].Class != outcome.Abort {
		t.Fatalf("want a single abort, got %+v", refusals)
	}
	if refusals[0].Check != "trinity/load" {
		t.Fatalf("want trinity/load, got %s", refusals[0].Check)
	}
}

// Every check the gates can emit has a proving red — an intentional
// failure demonstrating the gate fires. An unwitnessed gate is an
// unproven behavior; a check added to Checks without a proving red (or a
// declared, reasoned exemption) fails here.
func TestEveryGateHasAProvingRed(t *testing.T) {
	exempt := map[string]string{
		"trinity/schema-load": "internal integrity abort: fires only if the law embedded in the binary is itself broken — unreachable from any set fixture by construction; guarded instead by the build (embed of vetted law) and CI's cue vet producer",
	}

	proven := map[string]bool{
		"trinity/load":              true, // TestUnreadablePathAborts
		"trinity/vacuity":           true, // TestVacuousSetRefused
		"trinity/region-unreadable": true, // TestUnreadableRegionIsFinding
	}
	for _, m := range mutations {
		proven[m.wantCheck] = true
	}

	for _, check := range Checks {
		if proven[check] {
			continue
		}
		if reason, ok := exempt[check]; ok {
			t.Logf("exempt (declared): %s — %s", check, reason)
			continue
		}
		t.Errorf("gate %s has no proving red and no declared exemption — an unwitnessed gate is an unproven behavior", check)
	}
	for check := range exempt {
		if proven[check] {
			t.Errorf("gate %s is exempted AND proven — delete the stale exemption", check)
		}
	}
}
