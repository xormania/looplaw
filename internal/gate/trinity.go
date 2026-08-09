// Package gate holds the kernel gates: deterministic checks that refuse
// with a remedy or pass in silence. Gates are mechanism, never authority
// (law/registry.cue): they verify preconditions of the record act and
// originate nothing.
package gate

import (
	"fmt"
	"os"
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"cuelang.org/go/cue/load"

	"github.com/xormania/looplaw/internal/outcome"
	"github.com/xormania/looplaw/law"
)

// ValidateTrinity runs the trinity gates over one target set file:
// the shape gate (the embedded ratified schema, CUE's lane) and the
// relational gates (closure and reference checks, Go's lane). It returns
// every refusal found — gates report completely rather than stopping at
// the first no.
func ValidateTrinity(setPath string) []outcome.Refusal {
	data, err := os.ReadFile(setPath)
	if err != nil {
		return []outcome.Refusal{{
			Class:   outcome.Abort,
			Check:   "trinity/load",
			Subject: setPath,
			Reason:  err.Error(),
			Remedy:  "point the gate at a readable set file",
		}}
	}
	return validateTrinityBytes(setPath, data)
}

func validateTrinityBytes(subject string, data []byte) []outcome.Refusal {
	ctx := cuecontext.New()

	schema, err := embeddedLaw(ctx)
	if err != nil {
		return []outcome.Refusal{{
			Class:   outcome.Abort,
			Check:   "trinity/schema-load",
			Subject: "law (embedded)",
			Reason:  err.Error(),
			Remedy:  "the shipped law is broken; rebuild from ratified law",
		}}
	}

	set := ctx.CompileBytes(data, cue.Filename(subject))
	if set.Err() != nil {
		return []outcome.Refusal{{
			Class:   outcome.Rejection,
			Check:   "trinity/parse",
			Subject: subject,
			Reason:  set.Err().Error(),
			Remedy:  "the set must be well-formed CUE before any gate can read it",
		}}
	}

	var refusals []outcome.Refusal

	// Shape gate: unify the set with #TrinitySet and validate. CUE is the
	// shape gate; a mismatch is a rejection naming the failing path.
	def := schema.LookupPath(cue.ParsePath("#TrinitySet"))
	unified := def.Unify(set)
	if err := unified.Validate(cue.Concrete(true)); err != nil {
		refusals = append(refusals, outcome.Refusal{
			Class:   outcome.Rejection,
			Check:   "trinity/shape",
			Subject: subject,
			Reason:  err.Error(),
			Remedy:  "align the set with the ratified #TrinitySet schema (law/trinity.cue)",
		})
		// Relational checks still run where the raw fields allow: a shape
		// failure in one region must not mask a relational failure in
		// another.
	}

	refusals = append(refusals, relationalChecks(subject, set)...)
	return refusals
}

// relationalChecks carries the checks CUE's lattice cannot state: the
// closures and cross-references. Go is the relational lane.
// embeddedLaw builds the complete ratified law package the binary ships,
// loaded as one CUE instance so cross-file references resolve exactly as
// they do on disk.
func embeddedLaw(ctx *cue.Context) (cue.Value, error) {
	overlay := map[string]load.Source{}
	entries, err := law.Files.ReadDir(".")
	if err != nil {
		return cue.Value{}, err
	}
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".cue") {
			continue
		}
		b, err := law.Files.ReadFile(e.Name())
		if err != nil {
			return cue.Value{}, err
		}
		overlay["/embedded/law/"+e.Name()] = load.FromBytes(b)
	}
	insts := load.Instances([]string{"./law"}, &load.Config{
		Dir:     "/embedded",
		Overlay: overlay,
	})
	if len(insts) == 0 {
		return cue.Value{}, fmt.Errorf("embedded law: no instance")
	}
	if insts[0].Err != nil {
		return cue.Value{}, insts[0].Err
	}
	v := ctx.BuildInstance(insts[0])
	if v.Err() != nil {
		return cue.Value{}, v.Err()
	}
	return v, nil
}

func relationalChecks(subject string, set cue.Value) []outcome.Refusal {
	var refusals []outcome.Refusal

	partyIDs := fieldNames(set, "registry")
	invariantIDs := fieldNames(set, "invariants")
	lexiconTerms := fieldNames(set, "lexicon")

	// Act closure: every act appears in exactly one contract; every
	// contract holds at least one act.
	actHolder := map[string]string{}
	partiesSeen := map[string]bool{}

	contracts := set.LookupPath(cue.ParsePath("contracts"))
	iter, err := contracts.Fields()
	if err == nil {
		for iter.Next() {
			cid := iter.Selector().Unquoted()
			c := iter.Value()

			acts := stringList(c, "acts")
			if len(acts) == 0 {
				refusals = append(refusals, outcome.Refusal{
					Class:   outcome.Rejection,
					Check:   "trinity/act-closure",
					Subject: fmt.Sprintf("%s: %s", subject, cid),
					Reason:  "an act-bearing contract holds no acts",
					Remedy:  "give the contract its reserved act, or model the relationship as a non-act-bearing kind",
				})
			}
			for _, act := range acts {
				if holder, dup := actHolder[act]; dup {
					refusals = append(refusals, outcome.Refusal{
						Class:   outcome.Rejection,
						Check:   "trinity/act-closure",
						Subject: fmt.Sprintf("%s: act %q", subject, act),
						Reason:  fmt.Sprintf("held by both %s and %s — every act belongs to exactly one contract", holder, cid),
						Remedy:  "remove the act from one contract; one act, one authority, one contract",
					})
				} else {
					actHolder[act] = cid
				}
			}

			for _, role := range []string{"client", "supplier"} {
				p, _ := c.LookupPath(cue.ParsePath("parties." + role)).String()
				partiesSeen[p] = true
				if p != "" && !contains(partyIDs, p) {
					refusals = append(refusals, outcome.Refusal{
						Class:   outcome.Rejection,
						Check:   "trinity/party-resolve",
						Subject: fmt.Sprintf("%s: %s.%s", subject, cid, role),
						Reason:  fmt.Sprintf("party %q is not in the set's registry", p),
						Remedy:  "name a registered party, or add the party to the registry",
					})
				}
			}

			for _, cite := range stringList(c, "cites") {
				if !contains(invariantIDs, cite) {
					refusals = append(refusals, outcome.Refusal{
						Class:   outcome.Rejection,
						Check:   "trinity/cite-resolve",
						Subject: fmt.Sprintf("%s: %s cites %q", subject, cid, cite),
						Reason:  "cited invariant does not exist in the set",
						Remedy:  "cite an existing invariant id; invariants are cited, never restated",
					})
				}
			}

			for _, b := range structList(c, "blame") {
				at, _ := b.LookupPath(cue.ParsePath("at_fault")).String()
				if at != "" && !contains(partyIDs, at) {
					refusals = append(refusals, outcome.Refusal{
						Class:   outcome.Rejection,
						Check:   "trinity/blame-resolve",
						Subject: fmt.Sprintf("%s: %s blame", subject, cid),
						Reason:  fmt.Sprintf("at_fault names %q, not a registered party", at),
						Remedy:  "blame attaches to a registered party, adjudicated from recorded evidence",
					})
				}
			}
		}
	}

	// Parties coverage: every registered party appears in some contract.
	// A party in no contract is an unplaced component — a design defect,
	// not a formality.
	for _, p := range partyIDs {
		if !partiesSeen[p] {
			refusals = append(refusals, outcome.Refusal{
				Class:   outcome.Rejection,
				Check:   "trinity/party-coverage",
				Subject: fmt.Sprintf("%s: party %q", subject, p),
				Reason:  "registered but party to no contract",
				Remedy:  "make the party a client or supplier somewhere, or remove it from the registry",
			})
		}
	}

	// Lexicon reference closure: related terms and term authorities
	// resolve within the set.
	lex := set.LookupPath(cue.ParsePath("lexicon"))
	liter, err := lex.Fields()
	if err == nil {
		for liter.Next() {
			term := liter.Selector().Unquoted()
			for _, rel := range stringList(liter.Value(), "related") {
				if !contains(lexiconTerms, rel) {
					refusals = append(refusals, outcome.Refusal{
						Class:   outcome.Rejection,
						Check:   "trinity/related-resolve",
						Subject: fmt.Sprintf("%s: lexicon %q related %q", subject, term, rel),
						Reason:  "related term is not in the set's lexicon",
						Remedy:  "reference a coined term, or coin the neighbor before pointing at it",
					})
				}
			}
			auth, _ := liter.Value().LookupPath(cue.ParsePath("authority")).String()
			if auth != "" && auth != "none" && !contains(partyIDs, auth) {
				refusals = append(refusals, outcome.Refusal{
					Class:   outcome.Rejection,
					Check:   "trinity/authority-resolve",
					Subject: fmt.Sprintf("%s: lexicon %q authority %q", subject, term, auth),
					Reason:  "term authority is not a registered party",
					Remedy:  "point authority at a registry party id, or declare it \"none\"",
				})
			}
		}
	}

	// Experience cites resolve to contracts or invariants.
	contractIDs := fieldNames(set, "contracts")
	exp := set.LookupPath(cue.ParsePath("experience"))
	eiter, err := exp.Fields()
	if err == nil {
		for eiter.Next() {
			xid := eiter.Selector().Unquoted()
			for _, cite := range stringList(eiter.Value(), "cites") {
				if !contains(contractIDs, cite) && !contains(invariantIDs, cite) {
					refusals = append(refusals, outcome.Refusal{
						Class:   outcome.Rejection,
						Check:   "trinity/experience-cite-resolve",
						Subject: fmt.Sprintf("%s: %s cites %q", subject, xid, cite),
						Reason:  "judgment cites neither a contract nor an invariant of the set",
						Remedy:  "cite an existing contract or invariant id; judgment attaches to law, it never floats",
					})
				}
			}
		}
	}

	return refusals
}

func fieldNames(v cue.Value, path string) []string {
	var names []string
	iter, err := v.LookupPath(cue.ParsePath(path)).Fields()
	if err != nil {
		return nil
	}
	for iter.Next() {
		names = append(names, iter.Selector().Unquoted())
	}
	return names
}

func stringList(v cue.Value, path string) []string {
	var out []string
	list, err := v.LookupPath(cue.ParsePath(path)).List()
	if err != nil {
		return nil
	}
	for list.Next() {
		if s, err := list.Value().String(); err == nil {
			out = append(out, s)
		}
	}
	return out
}

func structList(v cue.Value, path string) []cue.Value {
	var out []cue.Value
	list, err := v.LookupPath(cue.ParsePath(path)).List()
	if err != nil {
		return nil
	}
	for list.Next() {
		out = append(out, list.Value())
	}
	return out
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}
