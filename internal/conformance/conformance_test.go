// Package conformance checks what the product says against the
// vocabulary this project reserves for it. It lives with the product
// because it reads the product: a check filed under dev/ would be
// skipped by anything that filters on paths, exactly when a product
// edit is what introduced the violation.
package conformance

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
	"cuelang.org/go/cue/load"
)

type source struct{ where, text string }

func devPackage(t *testing.T) cue.Value {
	t.Helper()
	insts := load.Instances([]string{"../../dev"}, nil)
	if len(insts) == 0 || insts[0].Err != nil {
		t.Fatalf("dev package: %v", insts[0].Err)
	}
	v := cuecontext.New().BuildInstance(insts[0])
	if v.Err() != nil {
		t.Fatalf("dev package: %v", v.Err())
	}
	return v
}

func reservedDev(t *testing.T) []string {
	t.Helper()
	iter, err := devPackage(t).LookupPath(cue.ParsePath("reserved_dev")).Fields()
	if err != nil {
		t.Fatal(err)
	}
	var terms []string
	for iter.Next() {
		terms = append(terms, iter.Selector().Unquoted())
	}
	return terms
}

// Product-facing text is what a reader of the product sees: the schemas
// the binary enforces, the strings it emits, and every recorded output.
// Everything under dev/ is workshop text by definition, test files are
// dev-lane by nature, and internal/golden is dev-lane infrastructure —
// all out of scope, so a test may say "proving red" as often as it
// likes.
func TestDevWordsNeverReachTheProduct(t *testing.T) {
	reserved := reservedDev(t)
	if len(reserved) == 0 {
		t.Fatal("the dev lexicon reserves nothing")
	}

	for _, src := range productText(t) {
		lower := strings.ToLower(stripPaths(src.text))
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

// The product's own refused vocabulary, enforced on what the product
// says. The lexicon method requires this — a lexicon without mechanical
// enforcement decays into aspiration — and until now the workshop had a
// conformance test while the ratified vocabulary had none, which is
// backwards. Review agents have been finding these by hand at cost.
func TestProductTextObeysItsOwnRefusedVocabulary(t *testing.T) {
	banned := bannedProductTerms(t)
	if len(banned) == 0 {
		t.Fatal("the product lexicon refuses nothing")
	}
	for _, src := range productText(t) {
		lower := strings.ToLower(stripPaths(src.text))
		for term, instead := range banned {
			re := regexp.MustCompile(`\b` + regexp.QuoteMeta(term) + `\b`)
			if re.MatchString(lower) {
				t.Errorf("the product says %q, which its own lexicon refuses (%s).\n  %s\n  instead: %s",
					term, src.where, excerpt(src.text, term), instead)
			}
		}
	}
}

// productText collects ratified law, every recorded output, and the
// string literals of non-test product code. Literals rather than whole
// files: a comment explaining a golden is dev-lane talk about the code,
// while a string is what the code says out loud.
func productText(t *testing.T) []source {
	t.Helper()
	var out []source

	add := func(where, text string) { out = append(out, source{where, text}) }

	for _, glob := range []string{"../../schema/*.cue", "../../schema/*.md", "../../internal/*/testdata/golden/*"} {
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
	err := filepath.WalkDir("../..", func(path string, d fs.DirEntry, err error) error {
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

var pathish = regexp.MustCompile(`\S*/\S+`)

func stripPaths(s string) string { return pathish.ReplaceAllString(s, " ") }

func excerpt(text, term string) string {
	i := strings.Index(strings.ToLower(text), strings.ToLower(term))
	if i < 0 {
		return ""
	}
	start := max(0, i-60)
	end := min(len(text), i+len(term)+60)
	return "…" + strings.ReplaceAll(text[start:end], "\n", " ") + "…"
}

// Only the outright bans: a qualified-form rule needs a reader, and a
// test that fires on legitimate qualified use would be noise.
func bannedProductTerms(t *testing.T) map[string]string {
	t.Helper()
	out := map[string]string{}
	v := devPackage(t)
	for _, region := range []string{"process_vocab", "forbidden_vocab"} {
		iter, err := v.LookupPath(cue.ParsePath(region)).Fields()
		if err != nil {
			t.Fatal(err)
		}
		for iter.Next() {
			tier, _ := iter.Value().LookupPath(cue.ParsePath("tier")).String()
			if tier != "BANNED" {
				continue
			}
			term := strings.ToLower(iter.Selector().Unquoted())
			if strings.Contains(term, " ") {
				continue // phrase rules need a reader
			}
			instead, _ := iter.Value().LookupPath(cue.ParsePath("instead")).String()
			out[term] = instead
		}
	}
	return out
}
