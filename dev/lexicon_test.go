// Package dev holds the workshop vocabulary and the rules that keep it
// separate from the product's. These read only dev/ — the checks that
// read what the product says live with the product, in
// internal/conformance, so they run when the product changes rather
// than when the workshop does.
package dev

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"cuelang.org/go/cue/load"
)

// devRegions are the two term maps: words coined here and words taken
// from the specification-CI method. Both are the workshop's vocabulary
// for every purpose a check has — neither may collide with the
// product's, and neither may appear in what the product says. They are
// separate files' worth of judgement, not separate standings: the split
// records that an inherited term is not ours to redefine.
var devRegions = []string{"reserved_dev", "inherited_dev"}

func reservedDev(t *testing.T) []string {
	t.Helper()
	b, err := os.ReadFile("lexicon-workshop.cue")
	if err != nil {
		t.Fatal(err)
	}
	v := cuecontext.New().CompileBytes(b)
	if v.Err() != nil {
		t.Fatalf("dev lexicon: %v", v.Err())
	}
	var terms []string
	for _, region := range devRegions {
		iter, err := v.LookupPath(cue.ParsePath(region)).Fields()
		if err != nil {
			t.Fatalf("%s: %v", region, err)
		}
		for iter.Next() {
			terms = append(terms, iter.Selector().Unquoted())
		}
	}
	return terms
}

// Every reserved term states what it means: a word without a meaning
// cannot be misused consistently.
func TestEveryDevTermStatesItsMeaning(t *testing.T) {
	b, err := os.ReadFile("lexicon-workshop.cue")
	if err != nil {
		t.Fatal(err)
	}
	v := cuecontext.New().CompileBytes(b)
	for _, region := range devRegions {
		iter, err := v.LookupPath(cue.ParsePath(region)).Fields()
		if err != nil {
			t.Fatalf("%s: %v", region, err)
		}
		n := 0
		for iter.Next() {
			n++
			s, err := iter.Value().String()
			if err != nil || strings.TrimSpace(s) == "" {
				t.Errorf("%s.%s states no meaning", region, iter.Selector().Unquoted())
			}
		}
		// A region that silently vanished would pass every check in this
		// file by having nothing to check. "shared" did exactly that
		// after it was deleted.
		if n == 0 {
			t.Errorf("%s names no terms — a region that has emptied out passes every check by having nothing to check", region)
		}
	}
}

// stripPaths removes path-like tokens before matching. A refusal that
// quotes the file it was handed is echoing input, not speaking: the
// product does not choose the words in a caller's path, so those
// occurrences are not the product using a workshop word.
// One brief, not several. Harness-specific files (CLAUDE.md and any
// equivalent) point at AGENTS.md rather than copying it: two briefs
// drift, and the one a harness happens to read would then be the one
// that is wrong.
func TestHarnessBriefsArePointers(t *testing.T) {
	canonical, err := os.ReadFile("../AGENTS.md")
	if err != nil {
		t.Fatal("AGENTS.md is the canonical brief and must exist: " + err.Error())
	}

	harnessFiles, err := filepath.Glob("../*.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range harnessFiles {
		name := filepath.Base(path)
		if name == "AGENTS.md" || name == "README.md" || name == "CONTRIBUTING.md" {
			continue
		}
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(b), "AGENTS.md") {
			t.Errorf("%s is a harness brief that does not point at AGENTS.md", name)
		}
		// A pointer that has grown into a second brief has stopped
		// being a pointer.
		if len(b) > len(canonical)/3 {
			t.Errorf("%s is %d bytes against the canonical %d: it has grown into a second brief rather than a pointer",
				name, len(b), len(canonical))
		}
	}
}

// Nothing in the workshop may be named after a product concept. This is
// the direction that has actually cost us: a script called dev/law, a
// ratification ritual copied from the product's act, our design basis
// living in a directory called law. Borrowing the product's words for
// workshop things reads as rigor and produces confusion.
func TestNoWorkshopArtifactIsNamedAfterTheProduct(t *testing.T) {
	product := productTerms(t)
	err := filepath.WalkDir("..", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && (d.Name() == ".git" || d.Name() == "proj") {
			return filepath.SkipDir
		}
		rel, _ := filepath.Rel("..", path)
		if !strings.HasPrefix(rel, "dev/") && rel != "dev" {
			return nil
		}
		// The file that defines the product's vocabulary is allowed to
		// be named for it.
		if strings.Contains(rel, "lexicon-product") {
			return nil
		}
		name := strings.ToLower(strings.TrimSuffix(d.Name(), filepath.Ext(d.Name())))
		if product[name] {
			t.Errorf("%s is named after the product concept %q — name workshop things for what they do", rel, name)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func devPackage(t *testing.T) cue.Value {
	t.Helper()
	insts := load.Instances([]string{"."}, nil)
	if len(insts) == 0 || insts[0].Err != nil {
		t.Fatalf("dev package: %v", insts[0].Err)
	}
	v := cuecontext.New().BuildInstance(insts[0])
	if v.Err() != nil {
		t.Fatalf("dev package: %v", v.Err())
	}
	return v
}

func productTerms(t *testing.T) map[string]bool {
	t.Helper()
	out := map[string]bool{"law": true} // the concept, though no entry names it
	v := devPackage(t)
	for _, region := range []string{"lexicon", "process_vocab", "forbidden_vocab"} {
		iter, err := v.LookupPath(cue.ParsePath(region)).Fields()
		if err != nil {
			t.Fatal(err)
		}
		for iter.Next() {
			out[strings.ToLower(iter.Selector().Unquoted())] = true
		}
	}
	return out
}

// The two vocabularies are disjoint. Every word is ours or the
// product's, never both — a word in both lists means the distinction
// has already collapsed, whatever either list says about it.
func TestVocabulariesDoNotIntersect(t *testing.T) {
	ours := map[string]bool{}
	for _, term := range reservedDev(t) {
		ours[strings.ToLower(term)] = true
	}

	v := devPackage(t)
	for _, region := range []string{"lexicon", "process_vocab", "forbidden_vocab"} {
		iter, err := v.LookupPath(cue.ParsePath(region)).Fields()
		if err != nil {
			t.Fatalf("%s: %v", region, err)
		}
		for iter.Next() {
			term := strings.ToLower(iter.Selector().Unquoted())
			if ours[term] {
				t.Errorf("%q is reserved by both the workshop and the product (%s): a word belongs to one lane or the other, never both",
					term, region)
			}
		}
	}

	// do_not_borrow is the same claim stated the other way: a word we
	// have written down as the product's cannot also be one of ours.
	// Checking only the product lexicon left this hole open, and
	// "drift" sat in it — reserved here while the spec used it for what
	// the provenance check reports.
	yielded, err := v.LookupPath(cue.ParsePath("do_not_borrow")).List()
	if err != nil {
		t.Fatal(err)
	}
	for yielded.Next() {
		entry, err := yielded.Value().String()
		if err != nil {
			continue
		}
		// Entries read "term, term — why"; the words before the em dash
		// are the yielded ones.
		head, _, _ := strings.Cut(entry, " — ")
		for _, w := range strings.Split(head, ",") {
			w = strings.ToLower(strings.TrimSpace(w))
			if w != "" && ours[w] {
				t.Errorf("%q is reserved by the workshop and also listed in do_not_borrow: the lexicon contradicts itself about whose word it is", w)
			}
		}
	}
}
