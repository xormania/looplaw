package absorb

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xormania/looplaw/internal/golden"
)

const goodManifest = `subject: "demo"

components: {
	"cmd/tool": {
		note: "the command"
		sources: {"cmd/tool/main.go": "1111111111111111111111111111111111111111111111111111111111111111"}
	}
	"internal/core": {
		note: ""
		sources: {"internal/core/core.go": "2222222222222222222222222222222222222222222222222222222222222222"}
	}
}

depends: {
	"cmd/tool": ["internal/core"]
}
`

func manifest(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "components.cue")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// A manifest states only what a tool established; loading derives
// nothing beyond it.
func TestComponentManifestLoads(t *testing.T) {
	m, refusals := LoadComponents(manifest(t, goodManifest))
	if len(refusals) > 0 {
		t.Fatalf("a well-formed manifest was refused: %v", refusals)
	}
	if m.Subject != "demo" {
		t.Errorf("subject = %q", m.Subject)
	}
	if got := m.Names(); len(got) != 2 || got[0] != "cmd/tool" || got[1] != "internal/core" {
		t.Errorf("names are not in stable order: %v", got)
	}
	if got := m.SourcePaths(); len(got) != 2 || got[0] != "cmd/tool/main.go" {
		t.Errorf("source paths are not in stable order: %v", got)
	}
	if !strings.HasPrefix(m.SourceHash("internal/core/core.go"), "2222") {
		t.Errorf("digest lookup failed: %q", m.SourceHash("internal/core/core.go"))
	}
}

// Proving red: a manifest that cannot be read.
func TestUnreadableManifestAborts(t *testing.T) {
	_, refusals := LoadComponents(filepath.Join(t.TempDir(), "absent.cue"))
	if len(refusals) != 1 || refusals[0].Check != "components/load" {
		t.Fatalf("want components/load, got %v", refusals)
	}
}

// Proving red: a manifest must be well-formed CUE before any check can
// read it.
func TestMalformedManifestIsRefused(t *testing.T) {
	_, refusals := LoadComponents(manifest(t, "subject: \"demo\"\ncomponents: {"))
	if len(refusals) != 1 || refusals[0].Check != "components/parse" {
		t.Fatalf("want components/parse, got %v", refusals)
	}
}

// Proving red: the ratified shape holds. A digest that is not a sha256
// hex string would make provenance unverifiable, so it is refused at the
// door rather than emitted into a view.
func TestManifestOffTheRatifiedShapeIsRefused(t *testing.T) {
	for _, tc := range []struct{ name, body string }{
		{"digest is not a sha256", strings.Replace(goodManifest,
			`"1111111111111111111111111111111111111111111111111111111111111111"`, `"not-a-digest"`, 1)},
		{"subject off the grammar", strings.Replace(goodManifest,
			`subject: "demo"`, `subject: "Demo Project"`, 1)},
		{"component name off the grammar", strings.Replace(goodManifest,
			`"cmd/tool": {`, `"CMD/Tool": {`, 1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, refusals := LoadComponents(manifest(t, tc.body))
			if len(refusals) != 1 || refusals[0].Check != "components/shape" {
				t.Fatalf("want components/shape, got %v", refusals)
			}
		})
	}
}

// Proving red: nothing enters a set by being referenced. An edge naming
// a component the manifest does not list would register a party by
// implication, which is a component nobody derived.
func TestEdgeToAnUnlistedComponentIsRefused(t *testing.T) {
	body := strings.Replace(goodManifest,
		`"cmd/tool": ["internal/core"]`, `"cmd/tool": ["internal/core", "internal/ghost"]`, 1)
	_, refusals := LoadComponents(manifest(t, body))
	if len(refusals) != 1 || refusals[0].Check != "components/unlisted" {
		t.Fatalf("want components/unlisted, got %v", refusals)
	}
	if !strings.Contains(refusals[0].Subject, "ghost") {
		t.Errorf("the refusal must name the missing component: %q", refusals[0].Subject)
	}

	// The same holds for a dependent that was never listed.
	body = strings.Replace(goodManifest,
		`"cmd/tool": ["internal/core"]`, `"internal/phantom": ["internal/core"]`, 1)
	_, refusals = LoadComponents(manifest(t, body))
	if len(refusals) != 1 || refusals[0].Check != "components/unlisted" {
		t.Fatalf("want components/unlisted for the dependent, got %v", refusals)
	}
}

// Proving red: provenance carries one digest per path. Two components
// disagreeing about a source cannot both be right, and choosing one
// would record a baseline that never existed — staleness measured
// against fiction.
func TestConflictingDigestsForOneSourceAreRefused(t *testing.T) {
	body := strings.Replace(goodManifest,
		`"internal/core/core.go": "2222222222222222222222222222222222222222222222222222222222222222"`,
		`"cmd/tool/main.go": "3333333333333333333333333333333333333333333333333333333333333333"`, 1)
	_, refusals := LoadComponents(manifest(t, body))
	if len(refusals) != 1 || refusals[0].Check != "components/source-conflict" {
		t.Fatalf("want components/source-conflict, got %v", refusals)
	}
	if !strings.Contains(refusals[0].Subject, "main.go") {
		t.Errorf("the refusal must name the contested path: %q", refusals[0].Subject)
	}

	// The same path with the same digest is agreement, not conflict.
	body = strings.Replace(goodManifest,
		`"internal/core/core.go": "2222222222222222222222222222222222222222222222222222222222222222"`,
		`"cmd/tool/main.go": "1111111111111111111111111111111111111111111111111111111111111111"`, 1)
	if _, refusals := LoadComponents(manifest(t, body)); len(refusals) > 0 {
		t.Errorf("two components agreeing about a source were refused: %v", refusals)
	}
}

// The skeleton is what an authoring caller receives, and what a consumer
// scripts against: a shape change is a contract change asking to be
// noticed.
func TestComponentSkeletonGolden(t *testing.T) {
	m, refusals := LoadComponents(manifest(t, goodManifest))
	if len(refusals) > 0 {
		t.Fatal(refusals)
	}
	golden.Assert(t, "testdata/golden/component-skeleton.cue", ComponentSkeleton(m))
}

// What the skeleton states is derivable and nothing more. An authored
// region filled with plausible text would pass the gates while saying
// nothing, which is worse than the refusal it replaced.
func TestSkeletonStatesOnlyWhatWasDerived(t *testing.T) {
	m, _ := LoadComponents(manifest(t, goodManifest))
	out := ComponentSkeleton(m)

	// authority_free is deliberately absent: whether a component holds
	// an authority is a design statement no deriver can make, and the
	// shape gate should refuse until an author states it.
	// Checked as a field rather than by substring: the comment that
	// leaves it to the author contains the same words, and an assertion
	// that cannot tell them apart proves nothing.
	if strings.Contains(out, ", authority_free:") {
		t.Error("the skeleton settles authority_free, which nothing derived")
	}
	if !strings.Contains(out, "// authority_free: true|false") {
		t.Error("the skeleton does not leave authority_free for the author")
	}

	for _, derived := range []string{
		`"cmd-tool": {name: "cmd/tool"`,                            // the component, registered
		`parties: {client: "cmd-tool", supplier: "internal-core"}`, // the edge, as a contract
		`"cmd/tool/main.go": "1111`,                                // provenance, with the client's digest
	} {
		if !strings.Contains(out, derived) {
			t.Errorf("the skeleton omits something derivable: %s", derived)
		}
	}
	for _, authored := range []string{"acts: []", "preconditions: {}", "guarantees: {}", "blame: []"} {
		if !strings.Contains(out, authored) {
			t.Errorf("the skeleton fills an authored region: %s should be empty", authored)
		}
	}
	// An empty note stays empty. A placeholder that reads like prose
	// would pass the gates while stating nothing.
	if strings.Contains(out, "TODO: what this component is") {
		t.Error("the skeleton invents a note")
	}
	if !strings.Contains(out, "// experience_declared_absent: true|false") {
		t.Error("the skeleton declares absence on the author's behalf")
	}
}

// Proving red: a component derived from nothing produces a contract no
// source addresses, and a statement no source can falsify is not
// evidence.
func TestComponentWithNoSourcesIsRefused(t *testing.T) {
	body := strings.Replace(goodManifest,
		`sources: {"internal/core/core.go": "2222222222222222222222222222222222222222222222222222222222222222"}`,
		`sources: {}`, 1)
	_, refusals := LoadComponents(manifest(t, body))
	if len(refusals) != 1 || refusals[0].Check != "components/sourceless" {
		t.Fatalf("want components/sourceless, got %v", refusals)
	}
	if refusals[0].Subject != "internal/core" {
		t.Errorf("the refusal must name the component: %q", refusals[0].Subject)
	}
}

// Proving red: ids are folded from names, and two names can fold to one.
// A party or contract sharing an id names neither of the things it came
// from, and the view would carry one where the manifest stated two.
func TestFoldedIdCollisionIsRefused(t *testing.T) {
	t.Run("two components, one party id", func(t *testing.T) {
		body := strings.Replace(goodManifest, `"internal/core": {`, `"internal-core": {`, 1)
		body = strings.Replace(body, `"internal/core/core.go"`, `"internal-core/core.go"`, 1)
		body = strings.Replace(body, `"cmd/tool": ["internal/core"]`, `"cmd/tool": ["internal-core"]`, 1)
		body = strings.Replace(body, `"cmd/tool": {`, `"internal/core": {`, 1)
		body = strings.Replace(body, `"cmd/tool/main.go"`, `"internal/core/main.go"`, 1)
		body = strings.Replace(body, `"cmd/tool": ["internal-core"]`, `"internal/core": ["internal-core"]`, 1)
		_, refusals := LoadComponents(manifest(t, body))
		var got string
		for _, r := range refusals {
			if r.Check == "components/id-collision" {
				got = r.Reason
			}
		}
		if got == "" {
			t.Fatalf("two names folding to one party id passed: %v", refusals)
		}
		if !strings.Contains(got, "internal/core") || !strings.Contains(got, "internal-core") {
			t.Errorf("the refusal must name both components: %q", got)
		}
	})
}

// Every check loading a manifest can emit has a proving red — and the
// credit is the red itself, not a note about one.
//
// Each entry names the check and holds the function that proves it, so
// the meta-test runs the red rather than trusting a comment. Delete the
// red and this stops compiling; leave it and stop asserting its check
// and the red itself fails. The map this replaced would have gone on
// passing with the test gone, which is what an audit demonstrated by
// deleting one.
func TestEveryComponentCheckHasAProvingRed(t *testing.T) {
	exempt := map[string]string{
		"components/schema-load": "internal integrity abort: fires only if the schema embedded in the binary is broken — unreachable from any manifest by construction",
		"components/decode":      "internal integrity abort: fires only if a value that passed the ratified shape cannot decode into its Go mirror",
	}
	proven := map[string]bool{}
	for _, red := range []struct {
		check string
		run   func(*testing.T)
	}{
		{"components/load", TestUnreadableManifestAborts},
		{"components/parse", TestMalformedManifestIsRefused},
		{"components/shape", TestManifestOffTheRatifiedShapeIsRefused},
		{"components/unlisted", TestEdgeToAnUnlistedComponentIsRefused},
		{"components/source-conflict", TestConflictingDigestsForOneSourceAreRefused},
		{"components/sourceless", TestComponentWithNoSourcesIsRefused},
		{"components/id-collision", TestFoldedIdCollisionIsRefused},
	} {
		if !t.Run("proving "+red.check, red.run) {
			t.Errorf("the red for %s did not pass, so it proves nothing", red.check)
			continue
		}
		proven[red.check] = true
	}
	for _, check := range ComponentChecks {
		if proven[check] {
			continue
		}
		if reason, ok := exempt[check]; ok {
			t.Logf("exempt (declared): %s — %s", check, reason)
			continue
		}
		t.Errorf("check %s has no proving red and no declared exemption", check)
	}
	for check := range exempt {
		if proven[check] {
			t.Errorf("%s is exempted AND proven — delete the stale exemption", check)
		}
	}
}
