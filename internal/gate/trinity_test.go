package gate

import (
	"os"
	"path/filepath"
	"slices"
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
	// Checks the mutation legitimately cascades into, beyond wantCheck.
	// Declared, never silent: an undeclared cascade means the red is
	// partly proving a gate it was not written for, so the demonstration-coverage test
	// would credit the wrong check.
	alsoDraws []string
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
		// cascade: the parent loses its shared client, so the children's preconditions stop being client-owed
		alsoDraws: []string{"trinity/decomp-dangling", "trinity/decomp-grounded"},
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
		// The supplier side must have its own direct demonstration: coverage of
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
		// cascade: the forked term key breaks the neighbor reference pointing at it
		alsoDraws: []string{"trinity/related-resolve"},
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
	{
		name:      "decomp-unknown-child",
		old:       `children: ["C-STANDING-1", "C-ISSUE-1"]`,
		new:       `children: ["C-STANDING-1", "C-GHOST-1"]`,
		wantCheck: "trinity/decomp-resolve",
		wantIn:    `"C-GHOST-1"`,
		// cascade: presents and the wire both target the child that was renamed away
		alsoDraws: []string{"trinity/decomp-presents", "trinity/decomp-wire"},
	},
	{
		// C-ISSUE-1 containing its own ancestor closes a containment
		// cycle: C-LEND-1 -> C-ISSUE-1 -> C-LEND-1.
		name:      "decomp-cycle",
		old:       `acts: ["issue-loan"]`,
		new:       "acts: [\"issue-loan\"]\n\t\tinterior: {\n\t\t\tchildren: [\"C-LEND-1\"]\n\t\t\twires: []\n\t\t\tpresents: {\"G-1\": {child: \"C-LEND-1\", guarantee: \"G-1\"}}\n\t\t}",
		wantCheck: "trinity/decomp-tree",
		wantIn:    "cycle",
		// cascade: the cycle leaves the cycled child unfed and unreachable
		alsoDraws: []string{"trinity/decomp-dangling", "trinity/decomp-grounded"},
	},
	{
		// C-RETURN-1 also claiming C-ISSUE-1 gives the child two
		// parents: containment is a tree.
		name:      "decomp-multi-parent",
		old:       `acts: ["return"]`,
		new:       "acts: [\"return\"]\n\t\tinterior: {\n\t\t\tchildren: [\"C-ISSUE-1\"]\n\t\t\twires: []\n\t\t\tpresents: {\"G-1\": {child: \"C-ISSUE-1\", guarantee: \"G-1\"}}\n\t\t}",
		wantCheck: "trinity/decomp-tree",
		wantIn:    "C-ISSUE-1",
		// cascade: the second interior claims a child whose feeds live in the first
		alsoDraws: []string{"trinity/decomp-dangling", "trinity/decomp-grounded"},
	},
	{
		name:      "decomp-unpresented-guarantee",
		old:       "presents: {\n\t\t\t\t\"G-1\": {child: \"C-ISSUE-1\", guarantee: \"G-1\"}\n\t\t\t}",
		new:       "presents: {}",
		wantCheck: "trinity/decomp-presents",
		wantIn:    "presented by no child",
	},
	{
		name:      "decomp-presents-missing-target",
		old:       "presents: {\n\t\t\t\t\"G-1\": {child: \"C-ISSUE-1\", guarantee: \"G-1\"}\n\t\t\t}",
		new:       "presents: {\n\t\t\t\t\"G-1\": {child: \"C-STANDING-1\", guarantee: \"G-9\"}\n\t\t\t}",
		wantCheck: "trinity/decomp-presents",
		wantIn:    `"G-9"`,
	},
	{
		name:      "decomp-wire-into-nothing",
		old:       `{from: {child: "C-STANDING-1", guarantee: "G-1"}, to: {child: "C-ISSUE-1", precondition: "P-1"}},`,
		new:       `{from: {child: "C-STANDING-1", guarantee: "G-1"}, to: {child: "C-ISSUE-1", precondition: "P-9"}},`,
		wantCheck: "trinity/decomp-wire",
		wantIn:    `"P-9"`,
		// cascade: the retargeted wire feeds nothing, so the real precondition is left unfed
		alsoDraws: []string{"trinity/decomp-dangling", "trinity/decomp-grounded"},
	},
	{
		name:      "decomp-dangling-requirement",
		old:       "wires: [\n\t\t\t\t{from: {child: \"C-STANDING-1\", guarantee: \"G-1\"}, to: {child: \"C-ISSUE-1\", precondition: \"P-1\"}},\n\t\t\t]",
		new:       "wires: []",
		wantCheck: "trinity/decomp-dangling",
		wantIn:    `"P-1"`,
		// cascade: an unfed precondition is also unreachable by any execution order
		alsoDraws: []string{"trinity/decomp-grounded"},
	},
	{
		name:      "decomp-uninherited-invariant",
		old:       "cites: [\"L-1\"]\n\t\tblame: [\n\t\t\t{violation_class: \"issuing over a live loan\"",
		new:       "cites: []\n\t\tblame: [\n\t\t\t{violation_class: \"issuing over a live loan\"",
		wantCheck: "trinity/decomp-cites",
		wantIn:    `"L-1"`,
	},
	{
		// A shared-client child inventing a precondition widens what the
		// client owes — refinement never strengthens the client's side.
		name:      "decomp-widened-client-obligation",
		old:       "\"P-2\": {text: \"The requested book carries no live loan, verifiable from the loan records.\"}\n\t\t}\n\t\tguarantees: {\n\t\t\t\"G-1\": {text: \"A standing attestation exists",
		new:       "\"P-2\": {text: \"The requested book carries no live loan, verifiable from the loan records.\"}\n\t\t\t\"P-9\": {text: \"The borrower presents a letter of reference.\"}\n\t\t}\n\t\tguarantees: {\n\t\t\t\"G-1\": {text: \"A standing attestation exists",
		wantCheck: "trinity/decomp-refinement",
		wantIn:    `"P-9"`,
		// cascade: the invented obligation is owed by nobody, so the child cannot be reached
		alsoDraws: []string{"trinity/decomp-grounded"},
	},
	{
		// A self-wire "feeds" a precondition from the very act it gates —
		// the one-line laundering attack; refused outright.
		name:      "decomp-self-wire",
		old:       `{from: {child: "C-STANDING-1", guarantee: "G-1"}, to: {child: "C-ISSUE-1", precondition: "P-1"}},`,
		new:       `{from: {child: "C-ISSUE-1", guarantee: "G-1"}, to: {child: "C-ISSUE-1", precondition: "P-1"}},`,
		wantCheck: "trinity/decomp-wire",
		wantIn:    "itself",
		// cascade: the self-wire replaces the only real feed, leaving the precondition unfed
		alsoDraws: []string{"trinity/decomp-dangling", "trinity/decomp-grounded"},
	},
	{
		// A precondition both client-owed and wire-fed has two satisfiers
		// of record; blame could not name the failed one.
		name:      "decomp-double-satisfier",
		old:       "wires: [\n\t\t\t\t{from: {child: \"C-STANDING-1\", guarantee: \"G-1\"}, to: {child: \"C-ISSUE-1\", precondition: \"P-1\"}},\n\t\t\t]",
		new:       "wires: [\n\t\t\t\t{from: {child: \"C-STANDING-1\", guarantee: \"G-1\"}, to: {child: \"C-ISSUE-1\", precondition: \"P-1\"}},\n\t\t\t\t{from: {child: \"C-ISSUE-1\", guarantee: \"G-1\"}, to: {child: \"C-STANDING-1\", precondition: \"P-1\"}},\n\t\t\t]",
		wantCheck: "trinity/decomp-satisfier",
		wantIn:    `"P-1"`,
	},
	{
		name:      "decomp-duplicate-child",
		old:       `children: ["C-STANDING-1", "C-ISSUE-1"]`,
		new:       `children: ["C-STANDING-1", "C-ISSUE-1", "C-STANDING-1"]`,
		wantCheck: "trinity/decomp-resolve",
		wantIn:    "listed twice",
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
			declared := map[string]bool{m.wantCheck: true}
			for _, c := range m.alsoDraws {
				declared[c] = true
			}
			for _, r := range refusals {
				if r.Check == m.wantCheck && strings.Contains(r.Error(), m.wantIn) {
					found = true
				}
				if !declared[r.Check] {
					t.Errorf("undeclared cascade: this one-edit mutation also drew %s — declare it in alsoDraws or narrow the edit, or the demonstration-coverage test credits a gate this red does not prove\n  %s",
						r.Check, r.Error())
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

// Presenting is one-to-one: two parent guarantees sharing one child
// guarantee means the boundary claims more than the assembly produces.
// Needs two edits (a second parent guarantee + its mapping), so it lives
// outside the single-edit mutation table.
func TestPresentsInjectivityRefused(t *testing.T) {
	base, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	s := string(base)
	s = strings.Replace(s,
		"\"G-1\": {text: \"A loan exists naming the book, the borrower, and the due date.\", records: \"the loan record\"}",
		"\"G-1\": {text: \"A loan exists naming the book, the borrower, and the due date.\", records: \"the loan record\"}\n\t\t\t\"G-2\": {text: \"A lending receipt is issued to the borrower.\", records: \"the lending receipt\"}",
		1)
	s = strings.Replace(s,
		"\"G-1\": {child: \"C-ISSUE-1\", guarantee: \"G-1\"}",
		"\"G-1\": {child: \"C-ISSUE-1\", guarantee: \"G-1\"}\n\t\t\t\t\"G-2\": {child: \"C-ISSUE-1\", guarantee: \"G-1\"}",
		1)
	if s == string(base) {
		t.Fatal("injectivity rewrite did not apply — fixture drifted")
	}
	path := filepath.Join(t.TempDir(), "set.cue")
	if err := os.WriteFile(path, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, r := range ValidateTrinity(path) {
		if r.Check == "trinity/decomp-presents" && strings.Contains(r.Error(), "already presents") {
			found = true
		}
	}
	if !found {
		t.Fatal("two parent guarantees sharing one child guarantee were not refused")
	}
}

// A closed feed loop with no entry executes nothing: pure mutual
// feedback between children, with zero client-owed input, is refused as
// ungroundable.
func TestUngroundedInteriorRefused(t *testing.T) {
	set := `subject:        "loop-sys"
schema_version: "0"
registry: {
	boss: {name: "the boss", note: "outer client", authority_free: true}
	w1: {name: "worker one", note: "supplier", authority_free: false}
	w2: {name: "worker two", note: "supplier", authority_free: false}
}
invariants: {}
lexicon: {}
contracts: {
	"C-P-1": {
		name: "the parent"
		parties: {client: "boss", supplier: "w1"}
		acts: ["outer-act"]
		preconditions: {}
		guarantees: {}
		invariants_local: {}
		cites: []
		blame: []
		status: "ratified"
		interior: {
			children: ["C-A-1", "C-B-1"]
			wires: [
				{from: {child: "C-A-1", guarantee: "G-1"}, to: {child: "C-B-1", precondition: "P-1"}},
				{from: {child: "C-B-1", guarantee: "G-1"}, to: {child: "C-A-1", precondition: "P-1"}},
			]
			presents: {}
		}
	}
	"C-A-1": {
		name: "child a"
		parties: {client: "w2", supplier: "w1"}
		acts: ["a-act"]
		preconditions: {"P-1": {text: "B's output exists."}}
		guarantees: {"G-1": {text: "A's output exists.", records: "the a record"}}
		invariants_local: {}
		cites: []
		blame: []
		status: "ratified"
	}
	"C-B-1": {
		name: "child b"
		parties: {client: "w1", supplier: "w2"}
		acts: ["b-act"]
		preconditions: {"P-1": {text: "A's output exists."}}
		guarantees: {"G-1": {text: "B's output exists.", records: "the b record"}}
		invariants_local: {}
		cites: []
		blame: []
		status: "ratified"
	}
}
experience: {}
experience_declared_absent: true
`
	path := filepath.Join(t.TempDir(), "set.cue")
	if err := os.WriteFile(path, []byte(set), 0o644); err != nil {
		t.Fatal(err)
	}
	refusals := ValidateTrinity(path)
	grounded := 0
	for _, r := range refusals {
		if r.Check == "trinity/decomp-grounded" {
			grounded++
		}
		if r.Check == "trinity/decomp-dangling" || r.Check == "trinity/decomp-wire" {
			t.Errorf("the loop should fail grounding, not wiring: %s", r.Error())
		}
	}
	if grounded != 2 {
		t.Fatalf("want both loop children refused as ungroundable, got %d refusals: %+v", grounded, refusals)
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
// failure demonstrating the gate fires. An undemonstrated gate is an
// unproven behavior; a check added to Checks without a proving red (or a
// declared, reasoned exemption) fails here.
func TestEveryGateHasAProvingRed(t *testing.T) {
	exempt := map[string]string{
		"trinity/schema-load": "internal integrity abort: fires only if the law embedded in the binary is itself broken — unreachable from any set fixture by construction; guarded instead by the build (embed of vetted law) and CI's cue vet producer",
	}

	// The mutation table below is honest by construction: it is walked to
	// run the reds and walked again to credit them. These eight are not
	// mutations, so their credit names the function that proves them and
	// runs it — deleting one stops this compiling, where the comment it
	// replaced would have left the check reported as demonstrated.
	proven := map[string]bool{}
	for _, red := range []struct {
		check string
		run   func(*testing.T)
	}{
		{"trinity/load", TestUnreadablePathAborts},
		{"trinity/vacuity", TestVacuousSetRefused},
		{"trinity/region-unreadable", TestUnreadableRegionIsFinding},
		{"trinity/decomp-grounded", TestUngroundedInteriorRefused},
		{"trinity/provenance-address", TestProvenanceRedsAreRedForTheirDeclaredReason},
		{"trinity/provenance-source", TestProvenanceRedsAreRedForTheirDeclaredReason},
		{"trinity/provenance-coverage", TestProvenanceRedsAreRedForTheirDeclaredReason},
		{"trinity/optional", TestOptionalFieldIsRefusedAtEveryDepth},
		{"trinity/open-value", TestOpenValuesAreRefusedAsAuthoredLaw},
	} {
		if !t.Run("proving "+red.check, red.run) {
			t.Errorf("the red for %s did not pass, so it proves nothing", red.check)
			continue
		}
		proven[red.check] = true
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
		t.Errorf("gate %s has no proving red and no declared exemption — an undemonstrated gate is an unproven behavior", check)
	}
	for check := range exempt {
		if proven[check] {
			t.Errorf("gate %s is exempted AND proven — delete the stale exemption", check)
		}
	}
}

// Provenance reds. The absorbed-view fixture lives in the absorber's
// testdata; these mutate it by one edit each, so the gate's provenance
// lane is demonstrated the same way every other lane is.
const provFixture = "../absorb/testdata/view.cue"

func TestProvenanceRedsAreRedForTheirDeclaredReason(t *testing.T) {
	base, err := os.ReadFile(provFixture)
	if err != nil {
		t.Fatal(err)
	}
	if refusals := ValidateTrinity(provFixture); len(refusals) != 0 {
		t.Fatalf("the absorbed-view fixture must be green: %+v", refusals)
	}

	cases := []mutation{
		{
			name:      "provenance-address-unknown-contract",
			old:       `"C-LEND-1":       ["lending.go"]`,
			new:       `"C-GHOST-1":      ["lending.go"]`,
			wantCheck: "trinity/provenance-address",
			wantIn:    `"C-GHOST-1"`,
		},
		{
			name:      "provenance-address-unknown-clause",
			old:       `"C-LEND-1/G-1":   ["lending.go"]`,
			new:       `"C-LEND-1/G-9":   ["lending.go"]`,
			wantCheck: "trinity/provenance-address",
			wantIn:    `"G-9"`,
		},
		{
			name:      "provenance-unbaselined-source",
			old:       `"C-RETURN-1":     ["returning.go"]`,
			new:       `"C-RETURN-1":     ["phantom.go"]`,
			wantCheck: "trinity/provenance-source",
			wantIn:    `"phantom.go"`,
		},
		{
			// Both C-RETURN-1 derivations go: a clause-grain address
			// still covers its contract, so coverage only fails when
			// nothing addresses the contract at all.
			name:      "provenance-uncovered-contract",
			old:       "\t\t\"C-RETURN-1\":     [\"returning.go\"]\n\t\t\"C-RETURN-1/G-1\": [\"returning.go\"]",
			new:       "\t\t\"C-LEND-1/P-1\":   [\"lending.go\"]",
			wantCheck: "trinity/provenance-coverage",
			wantIn:    `"C-RETURN-1"`,
		},
	}

	for _, m := range cases {
		t.Run(m.name, func(t *testing.T) {
			if !strings.Contains(string(base), m.old) {
				t.Fatalf("mutation target drifted from the fixture: %q", m.old)
			}
			mutated := strings.Replace(string(base), m.old, m.new, 1)
			path := filepath.Join(t.TempDir(), "view.cue")
			if err := os.WriteFile(path, []byte(mutated), 0o644); err != nil {
				t.Fatal(err)
			}
			refusals := ValidateTrinity(path)
			found := false
			for _, r := range refusals {
				if r.Check == m.wantCheck && strings.Contains(r.Error(), m.wantIn) {
					found = true
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
				t.Errorf("red, but not for the declared reason.\nwant %s naming %s\ngot:\n  %s",
					m.wantCheck, m.wantIn, strings.Join(got, "\n  "))
			}
		})
	}
}

// Authored law carries no provenance and the provenance lane stays
// silent over it — law is authored and ratified, never derived.
func TestAuthoredLawSkipsTheProvenanceLane(t *testing.T) {
	for _, r := range ValidateTrinity(fixture) {
		if strings.HasPrefix(r.Check, "trinity/provenance") {
			t.Errorf("provenance lane fired on authored law: %s", r.Error())
		}
	}
}

// Proving red: a set states values. A field declared "?:" states a
// constraint instead, so the field is absent and every check over it
// examines nothing while the file still reads as though it declared
// one. One character turned the self-wire attack from three refusals
// into "ok (authored set)", and hid an absorbed view's provenance from
// the guard that keeps evidence from becoming goal-law.
//
// Depth matters: the walk has to reach nested fields, not only the top
// level, or the bypass simply moves one level down.
func TestOptionalFieldIsRefusedAtEveryDepth(t *testing.T) {
	for _, tc := range []struct {
		name, from, find, replace, wantPath string
	}{
		{
			name: "top-level region", from: "../absorb/testdata/view.cue",
			find: "\nprovenance:", replace: "\nprovenance?:", wantPath: "provenance?",
		},
		{
			name: "contract region", from: "testdata/attacks/self-wire.cue",
			find: "\n\t\tinterior:", replace: "\n\t\tinterior?:", wantPath: `contracts."C-LEND-1".interior?`,
		},
		{
			name: "field nested two deep", from: "testdata/library/set.cue",
			find: "\n\t\t\tsupplier:", replace: "\n\t\t\tsupplier?:", wantPath: `parties.supplier?`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			src, err := os.ReadFile(tc.from)
			if err != nil {
				t.Fatal(err)
			}
			mutated := strings.Replace(string(src), tc.find, tc.replace, 1)
			if mutated == string(src) {
				t.Fatalf("the mutation did not apply — fixture %s changed shape", tc.from)
			}
			path := filepath.Join(t.TempDir(), "set.cue")
			if err := os.WriteFile(path, []byte(mutated), 0o644); err != nil {
				t.Fatal(err)
			}

			refusals := ValidateTrinity(path)
			var got *outcome.Refusal
			for i := range refusals {
				if refusals[i].Check == "trinity/optional" {
					got = &refusals[i]
					break
				}
			}
			if got == nil {
				t.Fatalf("an optional field passed the gates: %v", refusals)
			}
			// A red for the wrong reason proves nothing: the refusal must
			// name the field that was marked optional.
			if !strings.Contains(got.Subject, tc.wantPath) {
				t.Errorf("refusal names %q, want it to name %q", got.Subject, tc.wantPath)
			}
		})
	}
}

// The unmutated fixture passes: the guard refuses authored constraints,
// not the schema's own optional fields, which unification injects.
func TestSchemaOptionalFieldsAreNotRefused(t *testing.T) {
	if refusals := ValidateTrinity("testdata/library/set.cue"); len(refusals) > 0 {
		t.Errorf("fixture zero refused: %v", refusals)
	}
}

// Proving red: a submitted set states values, and a value is open when
// unifying these bytes with something else changes what they say without
// conflicting. CUE calls a defaulted disjunction concrete, so
// `status: *"ratified" | "withdrawn"` passed the shape gate, the
// relational lane read the default, and ratification copied the bytes
// into a law version — after which an overlay naming the other arm reads
// the same ratified law as withdrawn.
//
// An open list and an open struct do it from the other direction: the
// bytes admit additions, so ratified law gains a cited invariant, or a
// whole local invariant, that no act ever ratified.
func TestOpenValuesAreRefusedAsAuthoredLaw(t *testing.T) {
	for _, tc := range []struct {
		name, old, new, wantPath, wantReason string
	}{
		{
			name: "defaulted subject", old: `subject:        "lend-library"`,
			new:      `subject:        *"lend-library" | "other-library"`,
			wantPath: "subject", wantReason: "default",
		},
		{
			name: "defaulted contract status", old: "\t\tstatus: \"ratified\"\n\t\tinterior:",
			new:      "\t\tstatus: *\"ratified\" | \"withdrawn\"\n\t\tinterior:",
			wantPath: `contracts."C-LEND-1".status`, wantReason: "default",
		},
		{
			name: "default nested in a list element", old: `acts: ["lend"]`,
			new:      `acts: [*"lend" | "seize"]`,
			wantPath: `contracts."C-LEND-1".acts`, wantReason: "default",
		},
		{
			name: "open list of cites", old: `cites: ["L-1"]`,
			new:      `cites: ["L-1", ...string]`,
			wantPath: `contracts."C-LEND-1".cites`, wantReason: "open list",
		},
		{
			name: "open struct of local invariants", old: `invariants_local: {}`,
			new:      `invariants_local: {...}`,
			wantPath: `contracts."C-LEND-1".invariants_local`, wantReason: "open struct",
		},
		// The walk builds a path through fields, and every other
		// declaration form was walked past without being examined —
		// so the check the batch added to refuse a default admitted
		// one written any way but as a field's value.
		{
			name: "default bound by a let", old: `subject:        "lend-library"`,
			new:      "let chosen = *\"lend-library\" | \"other-library\"\nsubject:        chosen",
			wantPath: "let chosen", wantReason: "default",
		},
		{
			name: "default inside an embedding", old: "\t\tstatus: \"ratified\"\n\t\tinterior:",
			new:      "\t\t{status: *\"ratified\" | \"withdrawn\"}\n\t\tinterior:",
			wantPath: `contracts."C-LEND-1".status`, wantReason: "default",
		},
		{
			name: "open struct as an embedding", old: `invariants_local: {}`,
			new:      "invariants_local: {}\n\t\t{...}",
			wantPath: `contracts."C-LEND-1"`, wantReason: "open struct",
		},
		{
			name: "pattern label admitting unstated fields", old: `invariants_local: {}`,
			new:      `invariants_local: {[=~"^LI-"]: {text: "whatever"}}`,
			wantPath: `contracts."C-LEND-1".invariants_local`, wantReason: "open struct",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			base, err := os.ReadFile(fixture)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(base), tc.old) {
				t.Fatalf("mutation target drifted from the fixture: %q", tc.old)
			}
			path := filepath.Join(t.TempDir(), "set.cue")
			mutated := strings.Replace(string(base), tc.old, tc.new, 1)
			if err := os.WriteFile(path, []byte(mutated), 0o644); err != nil {
				t.Fatal(err)
			}

			refusals := ValidateTrinity(path)
			var got *outcome.Refusal
			for i := range refusals {
				if refusals[i].Check == "trinity/open-value" {
					got = &refusals[i]
					break
				}
			}
			if got == nil {
				t.Fatalf("an open value passed as authored law: %v", refusals)
			}
			// A red for the wrong reason proves nothing: the refusal
			// names where the value is left open and which form it is.
			if !strings.Contains(got.Subject, tc.wantPath) {
				t.Errorf("refusal names %q, want it to name %q", got.Subject, tc.wantPath)
			}
			if !strings.Contains(got.Reason, tc.wantReason) {
				t.Errorf("refusal reads %q, want it to name a %s", got.Reason, tc.wantReason)
			}
			if got.Remedy == "" {
				t.Error("refusal without a remedy")
			}
		})
	}
}

// The line is drawn by what unification can do, not by how the syntax
// looks. These forms compute one value from the same file, and an
// overlay that disagrees conflicts rather than winning — so DRY
// authoring stays green, and the guard refuses no more than it must.
// Refusing them would have overturned TestReferencesResolveGreen, which
// is a position this repository holds rather than an oversight.
func TestDeterminateExpressionsStayGreen(t *testing.T) {
	base, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ name, old, new string }{
		{"reference", `supplier: "librarian"`, `supplier: registry.librarian.id`},
		{"interpolation", `name: "the lending contract"`, `name: "the \(subject) contract"`},
		{"unification with a type", `name: "the lending contract"`, `name: string & "the lending contract"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if !strings.Contains(string(base), tc.old) {
				t.Fatalf("target drifted from the fixture: %q", tc.old)
			}
			path := filepath.Join(t.TempDir(), "set.cue")
			if err := os.WriteFile(path, []byte(strings.Replace(string(base), tc.old, tc.new, 1)), 0o644); err != nil {
				t.Fatal(err)
			}
			for _, r := range ValidateTrinity(path) {
				t.Errorf("a determinate value was refused: %s", r.Error())
			}
		})
	}
}

// The list a run reports as examined is honest: every id in it is one
// the gates can emit, and the only id the bytes path leaves out is the
// one it cannot run.
//
// Proving red: the ratify admission recorded gate.Checks wholesale, so
// it claimed trinity/load had been run over bytes the act was handed —
// and only LoadSet, which reads a file, can emit that. An admission is
// the evidence an entry happened; a check listed there that could not
// have run is evidence of an examination that did not.
func TestReportedChecksAreOnesThatCouldRun(t *testing.T) {
	data, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	_, ran, refusals := LoadSetBytes(fixture, data)
	if len(refusals) > 0 {
		t.Fatalf("the green fixture must pass, or the run is not the whole path: %v", refusals)
	}

	emittable := map[string]bool{}
	for _, c := range Checks {
		emittable[c] = true
	}
	reported := map[string]bool{}
	for _, c := range ran {
		if !emittable[c] {
			t.Errorf("%s is reported as run and is not a check the gates can emit", c)
		}
		if reported[c] {
			t.Errorf("%s is reported twice", c)
		}
		reported[c] = true
	}

	// Everything the gates can emit ran on a set that reaches the end,
	// except what belongs to the path that reads a file.
	for _, c := range Checks {
		if reported[c] {
			continue
		}
		if slices.Contains(loadChecks, c) {
			continue
		}
		t.Errorf("%s is a check the gates can emit, and a complete run does not report it", c)
	}
	for _, c := range loadChecks {
		if reported[c] {
			t.Errorf("%s cannot run over bytes and is reported as run", c)
		}
	}
}
