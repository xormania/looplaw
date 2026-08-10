// Package dev holds the dev-lane lexicon and the one mechanical rule
// worth enforcing about it: our workshop words must never reach what
// the product says.
package dev

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
)

// Product-facing text is what a reader of the product sees: ratified
// law, the strings the binary emits, and every recorded output. Test
// files are dev-lane by nature and internal/golden is dev-lane
// infrastructure, so both are out of scope — a test may say "proving
// red" as often as it likes.
func TestDevWordsNeverReachTheProduct(t *testing.T) {
	reserved := reservedDev(t)
	if len(reserved) == 0 {
		t.Fatal("the dev lexicon reserves nothing")
	}

	for _, src := range productText(t) {
		lower := strings.ToLower(src.text)
		for _, term := range reserved {
			re := regexp.MustCompile(`\b` + regexp.QuoteMeta(strings.ToLower(term)) + `\b`)
			if re.MatchString(lower) {
				t.Errorf("dev-lane word %q reached product-facing text (%s).\n"+
					"Say it in a comment, a test, or a pull request — not in what the product says.\n"+
					"  %s", term, src.where, excerpt(src.text, term))
			}
		}
	}
}

type source struct{ where, text string }

// productText collects ratified law, every recorded output, and the
// string literals of non-test product code. Literals rather than whole
// files: a comment explaining a golden is dev-lane talk about the code,
// while a string is what the code says out loud.
func productText(t *testing.T) []source {
	t.Helper()
	var out []source

	add := func(where, text string) { out = append(out, source{where, text}) }

	for _, glob := range []string{"../law/*.cue", "../law/*.md", "../internal/*/testdata/golden/*"} {
		paths, err := filepath.Glob(glob)
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range paths {
			b, err := os.ReadFile(p)
			if err != nil {
				t.Fatal(err)
			}
			add(p, string(b))
		}
	}

	fset := token.NewFileSet()
	err := filepath.WalkDir("..", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "testdata", "golden", "dev", "tools":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		f, perr := parser.ParseFile(fset, path, nil, 0)
		if perr != nil {
			return perr
		}
		ast.Inspect(f, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			if s, uerr := strconv.Unquote(lit.Value); uerr == nil {
				add(path, s)
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func reservedDev(t *testing.T) []string {
	t.Helper()
	b, err := os.ReadFile("lexicon.cue")
	if err != nil {
		t.Fatal(err)
	}
	v := cuecontext.New().CompileBytes(b)
	if v.Err() != nil {
		t.Fatalf("dev lexicon: %v", v.Err())
	}
	iter, err := v.LookupPath(cue.ParsePath("reserved_dev")).Fields()
	if err != nil {
		t.Fatal(err)
	}
	var terms []string
	for iter.Next() {
		terms = append(terms, iter.Selector().Unquoted())
	}
	return terms
}

// Every reserved term states what it means: a word without a meaning
// cannot be misused consistently.
func TestEveryDevTermStatesItsMeaning(t *testing.T) {
	b, err := os.ReadFile("lexicon.cue")
	if err != nil {
		t.Fatal(err)
	}
	v := cuecontext.New().CompileBytes(b)
	for _, region := range []string{"reserved_dev", "shared"} {
		iter, err := v.LookupPath(cue.ParsePath(region)).Fields()
		if err != nil {
			t.Fatal(err)
		}
		for iter.Next() {
			s, err := iter.Value().String()
			if err != nil || strings.TrimSpace(s) == "" {
				t.Errorf("%s.%s states no meaning", region, iter.Selector().Unquoted())
			}
		}
	}
}

func excerpt(text, term string) string {
	i := strings.Index(strings.ToLower(text), strings.ToLower(term))
	if i < 0 {
		return ""
	}
	start := max(0, i-60)
	end := min(len(text), i+len(term)+60)
	return "…" + strings.ReplaceAll(text[start:end], "\n", " ") + "…"
}
