// Package project renders projections of ratified law: the minimal
// governed context a reader needs, rather than the whole corpus.
//
// The lexicon method prescribes this — digests and term cards delivered
// as focused slices, verbatim, never paraphrased — and the reason is
// economy as much as fidelity: an agent that must read every ratified
// file to check one sentence pays for the corpus every time. The digest
// is derived from the law the binary carries, so it cannot drift from
// what the gates enforce.
package project

import (
	"fmt"
	"sort"
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"

	"github.com/xormania/looplaw/internal/gate"
)

// LawDigest renders the ratified law as the pasteable brief: the
// invariants, the authorities and acts, each reserved term's
// prompt-register card, and the vocabulary that is refused. Output is
// deterministic.
func LawDigest() (string, error) {
	ctx := cuecontext.New()
	law, err := gate.Law(ctx)
	if err != nil {
		return "", err
	}

	var b strings.Builder
	b.WriteString(`# Ratified law — digest

Derived from the law this binary carries, so it cannot drift from what
the gates enforce. The prompt cards below are written to be pasted
verbatim; read the full text in law/*.cue when exact wording is at
stake.

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
