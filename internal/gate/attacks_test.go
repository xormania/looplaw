package gate

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
)

const attacksDir = "testdata/attacks"

// The attack corpus: sets that must be refused, preserved from the
// adversarial review rounds that found them. Every blocking defect this
// project has found came from a hand-built attack; keeping them means a
// defect cannot return silently, and the next reviewer starts from what
// has already been tried rather than reinventing it.
func TestAttackCorpusStaysRefused(t *testing.T) {
	index := loadAttackIndex(t)
	if len(index) == 0 {
		t.Fatal("attack corpus is empty")
	}

	for _, name := range sortedAttackNames(index) {
		t.Run(strings.TrimSuffix(name, ".cue"), func(t *testing.T) {
			path := filepath.Join(attacksDir, name)
			refusals := ValidateTrinity(path)
			if len(refusals) == 0 {
				t.Fatalf("attack accepted — the defect it was written for has returned")
			}
			fired := map[string]bool{}
			for _, r := range refusals {
				fired[r.Check] = true
				if r.Remedy == "" {
					t.Errorf("refusal without a remedy: %s", r.Error())
				}
			}
			for _, want := range index[name] {
				if !fired[want] {
					var got []string
					for c := range fired {
						got = append(got, c)
					}
					sort.Strings(got)
					t.Errorf("attack refused, but not by the declared check.\nwant %s\ngot  %v", want, got)
				}
			}
		})
	}
}

// The corpus is complete: every file present is declared, and every
// declaration has its file. A dropped attack must fail loudly rather
// than quietly stop running.
func TestAttackCorpusIsComplete(t *testing.T) {
	index := loadAttackIndex(t)
	entries, err := os.ReadDir(attacksDir)
	if err != nil {
		t.Fatal(err)
	}
	onDisk := map[string]bool{}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".cue") && e.Name() != "index.cue" {
			onDisk[e.Name()] = true
		}
	}
	for name := range index {
		if !onDisk[name] {
			t.Errorf("declared attack %q has no file", name)
		}
	}
	for name := range onDisk {
		if _, ok := index[name]; !ok {
			t.Errorf("attack file %q is undeclared — every attack states the refusal it must draw", name)
		}
	}
}

// Attacks must exercise the corpus broadly: an attack corpus that only
// ever draws one or two checks is a corpus in name only.
// lanes are the kinds of thing the gates check, each a way a set can be
// wrong that the others cannot catch. An attack corpus is regression
// memory, and memory of one lane says nothing about another.
//
// Named here rather than derived from the check ids: which lane a check
// belongs to is a judgment about what it examines, and a prefix match
// would put trinity/optional with trinity/load because both are one
// word after the slash.
var lanes = map[string]func(check string) bool{
	// The set is not the shape the schema admits, or does not state a
	// value where the schema admits one — the same lane, because both
	// are answered by reading the set alone.
	"shape": func(c string) bool {
		return c == "trinity/shape" || c == "trinity/parse" || c == "trinity/vacuity" ||
			c == "trinity/optional" || c == "trinity/open-value"
	},
	// The set is well-shaped and its parts do not agree: a party, an
	// invariant, an act or a citation that resolves to nothing.
	"relational": func(c string) bool {
		return strings.HasSuffix(c, "-resolve") || strings.HasSuffix(c, "-coverage") ||
			c == "trinity/act-closure" || c == "trinity/authority-free" ||
			c == "trinity/absence-declared"
	},
	// The interior does not compose: wires, grounding, containment.
	"decomposition": func(c string) bool { return strings.HasPrefix(c, "trinity/decomp-") },
	// The view's claims about what it was derived from do not hold.
	"provenance": func(c string) bool { return strings.HasPrefix(c, "trinity/provenance-") },
	// The set, or a region of it, could not be read at all — a distinct
	// kind of wrong from any disagreement within it, and the only lane
	// whose outcome is a finding rather than a rejection.
	"readability": func(c string) bool {
		return c == "trinity/load" || c == "trinity/schema-load" || c == "trinity/region-unreadable"
	},
}

// Every lane keeps at least one attack. The check this replaced counted
// distinct check ids against a floor of eight, which is not what its name
// says and not what the corpus is for: with thirteen ids present, eight
// attack files could be deleted before it complained, and a whole lane
// could go with them. Counting lanes makes the failure name the lane
// that lost its memory.
func TestAttackCorpusSpansTheLanes(t *testing.T) {
	index := loadAttackIndex(t)
	covered := map[string]int{}
	for file, checks := range index {
		for _, c := range checks {
			for lane, inLane := range lanes {
				if inLane(c) {
					covered[lane]++
				}
			}
			matched := false
			for _, inLane := range lanes {
				if inLane(c) {
					matched = true
				}
			}
			if !matched {
				t.Errorf("%s draws %s, which belongs to no lane — place it, or the corpus has memory nothing accounts for",
					file, c)
			}
		}
	}
	for lane := range lanes {
		if covered[lane] == 0 {
			t.Errorf("no attack draws a %s check; that lane has no regression memory, "+
				"so a defect there can return with nothing to notice", lane)
		}
	}
}

func loadAttackIndex(t *testing.T) map[string][]string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(attacksDir, "index.cue"))
	if err != nil {
		t.Fatal(err)
	}
	v := cuecontext.New().CompileBytes(data)
	if v.Err() != nil {
		t.Fatalf("attack index: %v", v.Err())
	}
	out := map[string][]string{}
	iter, err := v.LookupPath(cue.ParsePath("attacks")).Fields()
	if err != nil {
		t.Fatal(err)
	}
	for iter.Next() {
		var checks []string
		list, err := iter.Value().LookupPath(cue.ParsePath("must_draw")).List()
		if err != nil {
			t.Fatalf("attack %s declares no must_draw", iter.Selector().Unquoted())
		}
		for list.Next() {
			s, _ := list.Value().String()
			checks = append(checks, s)
		}
		out[iter.Selector().Unquoted()] = checks
	}
	return out
}

func sortedAttackNames(index map[string][]string) []string {
	names := make([]string, 0, len(index))
	for n := range index {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}
