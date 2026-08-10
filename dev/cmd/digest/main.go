// Command digest renders this project's own design basis — the dev-lane
// registry, invariants and vocabulary under dev/ — as the brief a
// reader actually needs.
//
// A dev tool, not product surface: the product enforces schemas on
// other people's law and has no business printing ours. The lexicon
// method prescribes the digest — focused slices, verbatim, never
// paraphrased — and the reason is economy as much as fidelity: reading
// every file to check one sentence pays for the whole corpus each time.
package main

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"cuelang.org/go/cue/load"
)

func main() {
	out, err := Digest("./dev")
	if err != nil {
		fmt.Fprintln(os.Stderr, "digest:", err)
		os.Exit(1)
	}
	fmt.Print(out)
}

// Digest renders the design basis as the pasteable brief: the
// invariants, the authorities and acts, each reserved term's
// prompt-register card, and the vocabulary that is refused. Output is
// deterministic.
func Digest(dir string) (string, error) {
	ctx := cuecontext.New()
	insts := load.Instances([]string{dir}, nil)
	if len(insts) == 0 || insts[0].Err != nil {
		if len(insts) == 0 {
			return "", fmt.Errorf("no instance at %s", dir)
		}
		return "", insts[0].Err
	}
	law := ctx.BuildInstance(insts[0])
	if law.Err() != nil {
		return "", law.Err()
	}

	var b strings.Builder
	b.WriteString(`# looplaw's design basis — digest

Generated from dev/*.cue by dev/law, so it cannot drift from the files
it summarises. The prompt cards below are written to be pasted
verbatim; read the full text in dev/*.cue when exact wording is at
stake. The schema the binary enforces on input is law/, and it is not
summarised here.

`)

	b.WriteString("## Invariants (cited by id, never restated)\n\n")
	for _, id := range sortedFields(law, "tier0") {
		rule, _ := law.LookupPath(cue.ParsePath(fmt.Sprintf("tier0[%q].rule", id))).String()
		fmt.Fprintf(&b, "- **%s** %s\n", id, rule)
	}

	b.WriteString("\n## Authorities\n\n")
	for _, id := range sortedFields(law, "registry.authorities") {
		holds, _ := law.LookupPath(cue.ParsePath(fmt.Sprintf("registry.authorities[%q].holds", id))).String()
		fmt.Fprintf(&b, "- **%s** — %s\n", id, holds)
	}

	b.WriteString("\n## Acts (one verb, one authority)\n\n")
	for _, verb := range sortedFields(law, "registry.acts") {
		auth, _ := law.LookupPath(cue.ParsePath(fmt.Sprintf("registry.acts[%q].authority", verb))).String()
		changes, _ := law.LookupPath(cue.ParsePath(fmt.Sprintf("registry.acts[%q].changes", verb))).String()
		fmt.Fprintf(&b, "- **%s** (%s) — %s\n", verb, auth, changes)
	}

	b.WriteString("\n## Reserved terms — prompt cards\n\n")
	for _, term := range sortedFields(law, "lexicon") {
		tier, _ := law.LookupPath(cue.ParsePath(fmt.Sprintf("lexicon[%q].tier", term))).String()
		prompts, _ := law.LookupPath(cue.ParsePath(fmt.Sprintf("lexicon[%q].prompts", term))).String()
		fmt.Fprintf(&b, "### %s (%s)\n\n%s\n\n", term, tier, prompts)
	}

	b.WriteString("## Refused vocabulary\n\n")
	for _, region := range []string{"process_vocab", "forbidden_vocab"} {
		for _, term := range sortedFields(law, region) {
			tier, _ := law.LookupPath(cue.ParsePath(fmt.Sprintf("%s[%q].tier", region, term))).String()
			instead, _ := law.LookupPath(cue.ParsePath(fmt.Sprintf("%s[%q].instead", region, term))).String()
			fmt.Fprintf(&b, "- **%s** (%s) → %s\n", term, tier, instead)
		}
	}

	return b.String(), nil
}

func sortedFields(v cue.Value, path string) []string {
	iter, err := v.LookupPath(cue.ParsePath(path)).Fields()
	if err != nil {
		return nil
	}
	var out []string
	for iter.Next() {
		out = append(out, iter.Selector().Unquoted())
	}
	sort.Strings(out)
	return out
}
