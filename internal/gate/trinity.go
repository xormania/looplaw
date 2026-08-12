// Package gate holds the kernel gates: deterministic checks that refuse
// with a remedy or pass in silence. Gates are mechanism, never authority
// (dev/registry.cue): they verify preconditions of the record act and
// originate nothing.
package gate

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/cuecontext"
	"cuelang.org/go/cue/load"
	"cuelang.org/go/cue/parser"
	"cuelang.org/go/cue/token"

	"github.com/xormania/looplaw/internal/outcome"
	"github.com/xormania/looplaw/schema"
)

// The trinity gates, by the phase that runs them. A caller recording
// what it examined records the phases that executed, so the two lists
// cannot drift: Checks is built from these rather than kept beside them,
// which is the arrangement a hand-written second list did not have — the
// ratify admission claimed trinity/load had run over bytes, and only
// LoadSet can run it.
var (
	// loadChecks belong to the path that reads a file. The bytes path
	// never runs them, because it is handed the bytes.
	loadChecks = []string{"trinity/load"}
	// bytesChecks run over bytes already in hand, in this order.
	schemaChecks    = []string{"trinity/schema-load"}
	parseChecks     = []string{"trinity/parse"}
	optionalChecks  = []string{"trinity/optional"}
	openValueChecks = []string{"trinity/open-value"}
	shapeChecks     = []string{"trinity/shape"}
	// relationalChecksIDs are what the relational lane can emit; it runs
	// as one phase, over whatever regions the set states.
	relationalChecksIDs = []string{
		"trinity/vacuity",
		"trinity/region-unreadable",
		"trinity/act-closure",
		"trinity/party-resolve",
		"trinity/party-coverage",
		"trinity/cite-resolve",
		"trinity/invariant-coverage",
		"trinity/blame-resolve",
		"trinity/authority-free",
		"trinity/related-resolve",
		"trinity/authority-resolve",
		"trinity/experience-cite-resolve",
		"trinity/decomp-resolve",
		"trinity/decomp-tree",
		"trinity/decomp-presents",
		"trinity/decomp-wire",
		"trinity/decomp-dangling",
		"trinity/decomp-cites",
		"trinity/decomp-refinement",
		"trinity/decomp-satisfier",
		"trinity/decomp-grounded",
		"trinity/provenance-address",
		"trinity/provenance-source",
		"trinity/provenance-coverage",
	}
)

// Checks enumerates every check id the trinity gates can emit — the
// gates' action space, enumerable by design. The test suite proves a red
// for each (an undemonstrated gate is an unproven behavior); adding a check
// here without a proving red fails the suite.
//
// This is what the gates CAN emit. What a given run DID examine is the
// list validateTrinityBytes returns, and a record of the second must
// never be written from the first.
var Checks = concat(loadChecks, schemaChecks, parseChecks, optionalChecks,
	openValueChecks, shapeChecks, relationalChecksIDs)

func concat(lists ...[]string) []string {
	var out []string
	for _, l := range lists {
		out = append(out, l...)
	}
	return out
}

// ValidateTrinity runs the trinity gates over one target set file:
// the shape gate (the embedded ratified schema, CUE's lane) and the
// relational gates (closure and reference checks, Go's lane). It returns
// every refusal found — gates report completely rather than stopping at
// the first no.
func ValidateTrinity(setPath string) []outcome.Refusal {
	_, refusals := LoadSet(setPath)
	return refusals
}

// LoadSet runs the trinity gates over a set file and also returns the
// set's value (the unified value when shape passed, the raw value
// otherwise) for read paths — the differ — to consume. The value is
// data for derivation only; it carries no standing.
func LoadSet(setPath string) (cue.Value, []outcome.Refusal) {
	data, err := os.ReadFile(setPath)
	if err != nil {
		return cue.Value{}, []outcome.Refusal{{
			Class:   outcome.Abort,
			Check:   "trinity/load",
			Subject: setPath,
			Reason:  err.Error(),
			Remedy:  "point the gate at a readable set file",
		}}
	}
	set, _, refusals := LoadSetBytes(setPath, data)
	return set, refusals
}

// LoadSetBytes runs the trinity gates over bytes already in hand. A
// caller that will record what it submits gates these bytes and records
// the same slice: gating a path and reading it again leaves a window in
// which the file changes, so what passed the gates is not what enters
// the ledger. The name is only for refusal subjects.
// It also returns the checks this run examined, in the order it ran
// them. A caller that records an act writes that list, never Checks:
// Checks is what the gates can emit, and an entry recording a check as
// run when it was skipped — or when the path cannot run it at all — is
// evidence of an examination that did not happen.
func LoadSetBytes(name string, data []byte) (cue.Value, []string, []outcome.Refusal) {
	return validateTrinityBytes(name, data)
}

func validateTrinityBytes(subject string, data []byte) (cue.Value, []string, []outcome.Refusal) {
	ctx := cuecontext.New()

	// What this run examined, appended as each phase executes, so the
	// list is true by construction rather than by an argument about
	// which paths reach the end.
	var ran []string

	ran = append(ran, schemaChecks...)
	schema, err := embeddedLaw(ctx)
	if err != nil {
		return cue.Value{}, ran, []outcome.Refusal{{
			Class:   outcome.Abort,
			Check:   "trinity/schema-load",
			Subject: "law (embedded)",
			Reason:  err.Error(),
			Remedy:  "the embedded law is broken; replace this binary with one embedding the ratified law",
		}}
	}

	ran = append(ran, parseChecks...)
	set := ctx.CompileBytes(data, cue.Filename(subject))
	if set.Err() != nil {
		return cue.Value{}, ran, []outcome.Refusal{{
			Class:   outcome.Rejection,
			Check:   "trinity/parse",
			Subject: subject,
			Reason:  set.Err().Error(),
			Remedy:  "the set must be well-formed CUE before any gate can read it",
		}}
	}

	var refusals []outcome.Refusal

	// A submitted set states values, never constraints. An optional
	// field declares nothing: "interior?: {...}" reads as an interior
	// and is not one, so every check that walks concrete fields finds
	// nothing to examine and reports the set clean. That is silence
	// wearing the shape of a statement, and it defeated the whole
	// decomposition lane by one character.
	//
	// Checked against the authored bytes rather than the unified value:
	// the schema legitimately marks fields optional (trigger?,
	// synchronization?, interior?), and unification would make an
	// author's optional field indistinguishable from the schema's.
	ran = append(ran, optionalChecks...)
	if optional := optionalFields(set); len(optional) > 0 {
		for _, path := range optional {
			refusals = append(refusals, outcome.Refusal{
				Class:   outcome.Rejection,
				Check:   "trinity/optional",
				Subject: fmt.Sprintf("%s: %s", subject, path),
				Reason:  "declared with '?:', which states a constraint rather than a value — the field is absent, and every check over it examines nothing",
				Remedy:  "write the field as a value ('field: ...'); to say a region is genuinely absent, omit it and declare the absence where the schema asks for it",
			})
		}
		return set, ran, refusals
	}

	// The same rule as the optional check, for the other syntaxes that
	// leave a value open. A submitted set states values, and a value is
	// open when unifying these bytes with something else changes what
	// they say without conflicting: a defaulted disjunction takes the
	// other arm, an open list gains elements, an open struct gains
	// fields. Ratification copies the declared bytes into a law version,
	// so an open value is ratified law that reads differently in someone
	// else's context — the invariant this gate already claimed and did
	// not hold.
	//
	// Only these three, and the line is drawn by what unification can do
	// rather than by how the syntax looks. A reference, an interpolation
	// or a unification with a type computes one value from the same
	// file, and an overlay that disagrees conflicts rather than winning,
	// so ordinary DRY authoring stays green. The forms that state no
	// value at all — a bare disjunction, a bound, a type — are refused
	// by the shape gate as incomplete, and refusing them twice would say
	// the same thing in two voices.
	//
	// Read from the authored bytes for the reason the optional check is:
	// the schema states these forms legitimately, and after unification
	// an author's would be indistinguishable from the schema's.
	ran = append(ran, openValueChecks...)
	for _, open := range openValues(subject, data) {
		where := subject
		if open.path != "" {
			where = fmt.Sprintf("%s: %s", subject, open.path)
		}
		refusals = append(refusals, outcome.Refusal{
			Class:   outcome.Rejection,
			Check:   "trinity/open-value",
			Subject: where,
			Reason:  open.reason,
			Remedy:  "state the value that holds; law a later unification can change is not the law that was ratified",
		})
	}

	// Shape gate: unify the set with #TrinitySet and validate. CUE is the
	// shape gate; a mismatch is a rejection naming the failing path.
	ran = append(ran, shapeChecks...)
	def := schema.LookupPath(cue.ParsePath("#TrinitySet"))
	unified := def.Unify(set)

	// The relational lane reads the unified value: schema-injected fields
	// (ids) and ordinary CUE references resolve there. Only when shape
	// fails does it fall back to the raw set, so the relational checks still
	// reports what it can — a shape failure in one region must not mask a
	// relational failure in another.
	relational := unified
	if err := unified.Validate(cue.Concrete(true)); err != nil {
		refusals = append(refusals, outcome.Refusal{
			Class:   outcome.Rejection,
			Check:   "trinity/shape",
			Subject: subject,
			Reason:  err.Error(),
			Remedy:  "align the set with the ratified #TrinitySet schema (schema/trinity.cue)",
		})
		relational = set
	}

	ran = append(ran, relationalChecksIDs...)
	refusals = append(refusals, relationalChecks(subject, relational)...)
	return relational, ran, refusals
}

// Law exposes the embedded ratified law package to read paths (the
// differ self-checks its output against #Gap). Data for derivation
// only; no standing travels with it.
func Law(ctx *cue.Context) (cue.Value, error) {
	return embeddedLaw(ctx)
}

// embeddedLaw builds the complete ratified law package the binary
// carries, loaded as one CUE instance so cross-file references resolve
// exactly as they do on disk.
func embeddedLaw(ctx *cue.Context) (cue.Value, error) {
	overlay := map[string]load.Source{}
	entries, err := schema.Files.ReadDir(".")
	if err != nil {
		return cue.Value{}, err
	}
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".cue") {
			continue
		}
		b, err := schema.Files.ReadFile(e.Name())
		if err != nil {
			return cue.Value{}, err
		}
		overlay["/embedded/schema/"+e.Name()] = load.FromBytes(b)
	}
	insts := load.Instances([]string{"./schema"}, &load.Config{
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

// relationalChecks carries the checks CUE's lattice cannot state: the
// closures and cross-references. Go is the relational lane. A region the
// checks cannot read is a first-class finding — never skipped.
func relationalChecks(subject string, set cue.Value) []outcome.Refusal {
	var refusals []outcome.Refusal
	regionFinding := func(region string, err error) {
		refusals = append(refusals, outcome.Refusal{
			Class:   outcome.Finding,
			Check:   "trinity/region-unreadable",
			Subject: fmt.Sprintf("%s: %s", subject, region),
			Reason:  err.Error(),
			Remedy:  "make the region a struct of the schema's shape; an unreadable region is reported, never skipped",
		})
	}

	partyIDs, err := fieldNames(set, "registry")
	if err != nil {
		regionFinding("registry", err)
	}
	invariantIDs, err := fieldNames(set, "invariants")
	if err != nil {
		regionFinding("invariants", err)
	}
	lexiconTerms, err := fieldNames(set, "lexicon")
	if err != nil {
		regionFinding("lexicon", err)
	}
	contractIDs, err := fieldNames(set, "contracts")
	if err != nil {
		regionFinding("contracts", err)
	}

	// Vacuity: a set with no parties or no contracts binds nothing.
	// Silence is not a declaration (the experience_declared_absent
	// precedent); an empty set is refused, not vacuously green.
	if len(partyIDs) == 0 || len(contractIDs) == 0 {
		refusals = append(refusals, outcome.Refusal{
			Class:   outcome.Rejection,
			Check:   "trinity/vacuity",
			Subject: subject,
			Reason:  fmt.Sprintf("%d registered parties, %d contracts — the set binds nothing", len(partyIDs), len(contractIDs)),
			Remedy:  "author the registry and at least one act-bearing contract; a set that binds nothing is not submitted",
		})
	}

	// Which parties are declared authority-free: certified by the checks
	// below, not decorative.
	authorityFree := map[string]bool{}
	if reg, err := regionFields(set, "registry"); err == nil {
		for reg.Next() {
			free, _ := reg.Value().LookupPath(cue.ParsePath("authority_free")).Bool()
			authorityFree[reg.Selector().Unquoted()] = free
		}
	}

	// Act closure: every act appears in exactly one contract; every
	// contract holds at least one act.
	actHolder := map[string]string{}
	partiesSeen := map[string]bool{}
	citedInvariants := map[string]bool{}
	infos := map[string]*contractInfo{}

	if iter, err := regionFields(set, "contracts"); err == nil {
		for iter.Next() {
			cid := iter.Selector().Unquoted()
			c := iter.Value()
			infos[cid] = collectContract(c)

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
				switch holder, dup := actHolder[act]; {
				case dup && holder == cid:
					refusals = append(refusals, outcome.Refusal{
						Class:   outcome.Rejection,
						Check:   "trinity/act-closure",
						Subject: fmt.Sprintf("%s: act %q", subject, act),
						Reason:  fmt.Sprintf("held twice by %s — every act belongs to exactly one contract, once", cid),
						Remedy:  "list the act once",
					})
				case dup:
					refusals = append(refusals, outcome.Refusal{
						Class:   outcome.Rejection,
						Check:   "trinity/act-closure",
						Subject: fmt.Sprintf("%s: act %q", subject, act),
						Reason:  fmt.Sprintf("held by both %s and %s — every act belongs to exactly one contract", holder, cid),
						Remedy:  "remove the act from one contract; one act, one authority, one contract",
					})
				default:
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
				if role == "supplier" && authorityFree[p] {
					refusals = append(refusals, outcome.Refusal{
						Class:   outcome.Rejection,
						Check:   "trinity/authority-free",
						Subject: fmt.Sprintf("%s: %s.supplier", subject, cid),
						Reason:  fmt.Sprintf("supplier %q is declared authority-free, but a supplier owes the guarantees of an act-bearing contract", p),
						Remedy:  "give the contract an authority-holding supplier, or amend the party's registry entry",
					})
				}
			}

			for _, cite := range stringList(c, "cites") {
				if contains(invariantIDs, cite) {
					citedInvariants[cite] = true
					continue
				}
				refusals = append(refusals, outcome.Refusal{
					Class:   outcome.Rejection,
					Check:   "trinity/cite-resolve",
					Subject: fmt.Sprintf("%s: %s cites %q", subject, cid, cite),
					Reason:  "cited invariant does not exist in the set",
					Remedy:  "cite an existing invariant id; invariants are cited, never restated",
				})
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

	refusals = append(refusals, decompositionChecks(subject, infos)...)
	refusals = append(refusals, provenanceChecks(subject, set, infos)...)

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

	// Invariant coverage: an invariant cited by no contract is dead law.
	// Experience cites are advisory and do not count as binding coverage.
	for _, inv := range invariantIDs {
		if !citedInvariants[inv] {
			refusals = append(refusals, outcome.Refusal{
				Class:   outcome.Rejection,
				Check:   "trinity/invariant-coverage",
				Subject: fmt.Sprintf("%s: invariant %q", subject, inv),
				Reason:  "cited by no contract — dead law",
				Remedy:  "bind at least one contract under it (cites), or retire it from the set",
			})
		}
	}

	// Lexicon reference closure: related terms and term authorities
	// resolve within the set, and no term is owned by an authority-free
	// party.
	if liter, err := regionFields(set, "lexicon"); err == nil {
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
			if auth == "" || auth == "none" {
				continue
			}
			if !contains(partyIDs, auth) {
				refusals = append(refusals, outcome.Refusal{
					Class:   outcome.Rejection,
					Check:   "trinity/authority-resolve",
					Subject: fmt.Sprintf("%s: lexicon %q authority %q", subject, term, auth),
					Reason:  "term authority is not a registered party",
					Remedy:  "point authority at a registry party id, or declare it \"none\"",
				})
			} else if authorityFree[auth] {
				refusals = append(refusals, outcome.Refusal{
					Class:   outcome.Rejection,
					Check:   "trinity/authority-free",
					Subject: fmt.Sprintf("%s: lexicon %q authority %q", subject, term, auth),
					Reason:  "the term's authority is a party declared authority-free",
					Remedy:  "point authority at an authority-holding party, declare it \"none\", or amend the registry entry",
				})
			}
		}
	}

	// Experience cites resolve to contracts or invariants.
	if eiter, err := regionFields(set, "experience"); err == nil {
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

// provenanceChecks gate an absorbed view's provenance as data: every
// addressed statement exists, every named source is in the baseline,
// and every contract is addressed — an absorbed statement with no
// derivation is unfalsifiable. The kernel reads no tree here; it
// compares what was submitted.
func provenanceChecks(subject string, set cue.Value, infos map[string]*contractInfo) []outcome.Refusal {
	prov := set.LookupPath(cue.ParsePath("provenance"))
	if !prov.Exists() {
		return nil // authored law, not an absorbed view
	}

	var refusals []outcome.Refusal
	refuse := func(check, subj, reason, remedy string) {
		refusals = append(refusals, outcome.Refusal{
			Class: outcome.Rejection, Check: check,
			Subject: fmt.Sprintf("%s: %s", subject, subj),
			Reason:  reason, Remedy: remedy,
		})
	}

	sources, _ := fieldNames(prov, "sources")
	addressed := map[string]bool{}

	if iter, err := regionFields(prov, "derivations"); err == nil {
		for iter.Next() {
			addr := iter.Selector().Unquoted()
			contract, clause, _ := strings.Cut(addr, "/")
			info, ok := infos[contract]
			switch {
			case !ok:
				refuse("trinity/provenance-address", fmt.Sprintf("derivation %q", addr),
					fmt.Sprintf("contract %q is not in the set", contract),
					"address a contract the set states, or drop the derivation")
			case clause != "" && !contains(info.preconditions, clause) &&
				!contains(info.guarantees, clause) && !contains(info.localInvariants, clause):
				refuse("trinity/provenance-address", fmt.Sprintf("derivation %q", addr),
					fmt.Sprintf("contract %q states no clause %q", contract, clause),
					"address a clause the contract states, or address the contract alone")
			default:
				addressed[contract] = true
			}

			for _, src := range stringListValue(iter.Value()) {
				if !contains(sources, src) {
					refuse("trinity/provenance-source", fmt.Sprintf("derivation %q", addr),
						fmt.Sprintf("source %q is absent from the provenance baseline", src),
						"name a source the absorption baselined in provenance.sources; an unbaselined source cannot go stale, so it proves nothing")
				}
			}
		}
	}

	for _, cid := range sortedKeys(infos) {
		if !addressed[cid] {
			refuse("trinity/provenance-coverage", fmt.Sprintf("contract %q", cid),
				"absorbed but derived from nothing — an unsourced statement cannot go stale, so nothing can ever falsify it",
				"state in provenance.derivations what the contract was derived from, or drop it from the view")
		}
	}

	return refusals
}

// contractInfo is the relational lane's view of one contract, collected
// in a single pass so the decomposition checks work over data, not
// repeated CUE lookups.
type contractInfo struct {
	client          string
	preconditions   []string
	guarantees      []string
	localInvariants []string
	cites           []string
	hasInterior     bool
	children        []string
	wires           []wireInfo
	presents        map[string][2]string // parent guarantee -> (child, child guarantee)
}

type wireInfo struct {
	fromChild, fromGuarantee string
	toChild, toPrecondition  string
}

func collectContract(c cue.Value) *contractInfo {
	info := &contractInfo{presents: map[string][2]string{}}
	info.client, _ = c.LookupPath(cue.ParsePath("parties.client")).String()
	info.preconditions, _ = fieldNames(c, "preconditions")
	info.guarantees, _ = fieldNames(c, "guarantees")
	info.localInvariants, _ = fieldNames(c, "invariants_local")
	info.cites = stringList(c, "cites")

	interior := c.LookupPath(cue.ParsePath("interior"))
	if !interior.Exists() {
		return info
	}
	info.hasInterior = true
	info.children = stringList(interior, "children")
	for _, w := range structList(interior, "wires") {
		var wi wireInfo
		wi.fromChild, _ = w.LookupPath(cue.ParsePath("from.child")).String()
		wi.fromGuarantee, _ = w.LookupPath(cue.ParsePath("from.guarantee")).String()
		wi.toChild, _ = w.LookupPath(cue.ParsePath("to.child")).String()
		wi.toPrecondition, _ = w.LookupPath(cue.ParsePath("to.precondition")).String()
		info.wires = append(info.wires, wi)
	}
	if piter, err := regionFields(interior, "presents"); err == nil {
		for piter.Next() {
			child, _ := piter.Value().LookupPath(cue.ParsePath("child")).String()
			g, _ := piter.Value().LookupPath(cue.ParsePath("guarantee")).String()
			info.presents[piter.Selector().Unquoted()] = [2]string{child, g}
		}
	}
	return info
}

// decompositionChecks: the interior gates. The boundary is held —
// children and wiring jointly present the parent's guarantees — the
// containment relation is a tree, and refinement never widens what the
// shared client owes.
func decompositionChecks(subject string, infos map[string]*contractInfo) []outcome.Refusal {
	var refusals []outcome.Refusal
	refuse := func(check, subj, reason, remedy string) {
		refusals = append(refusals, outcome.Refusal{
			Class: outcome.Rejection, Check: check,
			Subject: fmt.Sprintf("%s: %s", subject, subj),
			Reason:  reason, Remedy: remedy,
		})
	}

	// Children resolve; the containment relation is single-parent.
	parents := map[string][]string{}
	for _, cid := range sortedKeys(infos) {
		info := infos[cid]
		seen := map[string]bool{}
		for _, ch := range info.children {
			if seen[ch] {
				refuse("trinity/decomp-resolve", cid,
					fmt.Sprintf("child %q is listed twice in the interior", ch),
					"list each child once")
				continue
			}
			seen[ch] = true
			if _, ok := infos[ch]; !ok {
				refuse("trinity/decomp-resolve", cid,
					fmt.Sprintf("interior child %q is not a contract in the set", ch),
					"name an existing contract, or author the child before decomposing into it")
				continue
			}
			parents[ch] = append(parents[ch], cid)
		}
	}
	for _, ch := range sortedKeys(parents) {
		ps := parents[ch]
		if len(ps) > 1 {
			refuse("trinity/decomp-tree", ch,
				fmt.Sprintf("a child of %d interiors (%s) — containment is a tree, one parent per child", len(ps), strings.Join(ps, ", ")),
				"keep the contract in one interior; shared behavior is wired, not doubly contained")
		}
	}

	// Containment is acyclic: a contract never contains its ancestor.
	const (
		white = 0
		gray  = 1
		black = 2
	)
	color := map[string]int{}
	var visit func(cid string) bool
	visit = func(cid string) bool {
		switch color[cid] {
		case gray:
			return false
		case black:
			return true
		}
		color[cid] = gray
		for _, ch := range infos[cid].children {
			if _, ok := infos[ch]; ok && !visit(ch) {
				return false
			}
		}
		color[cid] = black
		return true
	}
	for _, cid := range sortedKeys(infos) {
		if color[cid] == white && !visit(cid) {
			refuse("trinity/decomp-tree", cid,
				"containment cycle: the contract contains itself through its interior",
				"decomposition descends; remove the cycle from the containment relation")
		}
	}

	// Per-interior gates.
	for _, cid := range sortedKeys(infos) {
		info := infos[cid]
		if !info.hasInterior {
			continue
		}
		childSet := map[string]bool{}
		for _, ch := range info.children {
			if _, ok := infos[ch]; ok {
				childSet[ch] = true
			}
		}

		// Assembly satisfaction: every parent guarantee presented by
		// exactly one existing child guarantee.
		for _, g := range info.guarantees {
			if _, ok := info.presents[g]; !ok {
				refuse("trinity/decomp-presents", fmt.Sprintf("%s guarantee %q", cid, g),
					"presented by no child — the assembly does not present the parent's boundary",
					"map the guarantee to the child guarantee that presents it")
			}
		}
		// Presenting is one-to-one: a child guarantee presents at most
		// one parent guarantee, or the boundary claims more than the
		// assembly produces. Keys are walked sorted so the named
		// collider is deterministic.
		presentKeys := make([]string, 0, len(info.presents))
		for pg := range info.presents {
			presentKeys = append(presentKeys, pg)
		}
		sort.Strings(presentKeys)
		usedTarget := map[[2]string]string{}
		for _, pg := range presentKeys {
			target := info.presents[pg]
			if first, dup := usedTarget[target]; dup {
				refuse("trinity/decomp-presents", fmt.Sprintf("%s guarantee %q", cid, pg),
					fmt.Sprintf("presented by %s.%q, which already presents %q — one child guarantee presents at most one parent guarantee", target[0], target[1], first),
					"present each parent guarantee through its own child guarantee")
			} else {
				usedTarget[target] = pg
			}
			if !contains(info.guarantees, pg) {
				refuse("trinity/decomp-presents", fmt.Sprintf("%s presents %q", cid, pg),
					"maps a guarantee the parent does not state",
					"present only the parent's own guarantees; the boundary is held, not extended")
				continue
			}
			child, cg := target[0], target[1]
			switch {
			case !childSet[child]:
				refuse("trinity/decomp-presents", fmt.Sprintf("%s guarantee %q", cid, pg),
					fmt.Sprintf("presented by %q, which is not a child of this interior", child),
					"present through a child of the interior")
			case !contains(infos[child].guarantees, cg):
				refuse("trinity/decomp-presents", fmt.Sprintf("%s guarantee %q", cid, pg),
					fmt.Sprintf("child %q has no guarantee %q", child, cg),
					"map to a guarantee the child actually states")
			}
		}

		// Wires resolve inside the interior. A wire names two different
		// children: a self-wire feeds nothing — the guarantee exists only
		// after the act whose precondition it claims to feed.
		fed := map[string]map[string]bool{}
		fedBy := map[string]map[string][]string{} // child -> precondition -> feeding children
		for _, w := range info.wires {
			bad := false
			if w.fromChild == w.toChild {
				refuse("trinity/decomp-wire", cid,
					fmt.Sprintf("wire from %q to itself — a wire feeds a sibling, and a child's guarantee cannot feed its own precondition", w.fromChild),
					"wire the precondition from a sibling's guarantee, or restate the interior")
				continue
			}
			if !childSet[w.fromChild] {
				refuse("trinity/decomp-wire", cid,
					fmt.Sprintf("wire from %q, not a child of this interior", w.fromChild),
					"wire only between the interior's children")
				bad = true
			} else if !contains(infos[w.fromChild].guarantees, w.fromGuarantee) {
				refuse("trinity/decomp-wire", cid,
					fmt.Sprintf("wire from %s.%q, a guarantee it does not state", w.fromChild, w.fromGuarantee),
					"wire from a guarantee the child states")
				bad = true
			}
			if !childSet[w.toChild] {
				refuse("trinity/decomp-wire", cid,
					fmt.Sprintf("wire to %q, not a child of this interior", w.toChild),
					"wire only between the interior's children")
				bad = true
			} else if !contains(infos[w.toChild].preconditions, w.toPrecondition) {
				refuse("trinity/decomp-wire", cid,
					fmt.Sprintf("wire to %s.%q, a precondition it does not state", w.toChild, w.toPrecondition),
					"wire to a precondition the child states")
				bad = true
			}
			if !bad {
				if fed[w.toChild] == nil {
					fed[w.toChild] = map[string]bool{}
					fedBy[w.toChild] = map[string][]string{}
				}
				fed[w.toChild][w.toPrecondition] = true
				fedBy[w.toChild][w.toPrecondition] = append(fedBy[w.toChild][w.toPrecondition], w.fromChild)
			}
		}

		// No dangling requirements, and refinement never widens what the
		// shared client owes: a child precondition is either fed by a
		// wire, or (for a child sharing the parent's client) one the
		// parent itself states.
		walked := map[string]bool{}
		for _, ch := range info.children {
			ci, ok := infos[ch]
			if !ok || walked[ch] {
				continue
			}
			walked[ch] = true
			for _, p := range ci.preconditions {
				clientOwed := ci.client == info.client && contains(info.preconditions, p)
				switch {
				case fed[ch][p] && clientOwed:
					// One obligation, one satisfier of record: blame
					// adjudicates from recorded evidence, and two
					// satisfiers make the failed one unnameable.
					refuse("trinity/decomp-satisfier", fmt.Sprintf("%s precondition %q", ch, p),
						"owed by the shared client and fed by a wire — two satisfiers of record for one obligation",
						"remove the wire or the shared obligation; every obligation has exactly one satisfier of record")
				case fed[ch][p]:
				case clientOwed:
				case ci.client == info.client:
					refuse("trinity/decomp-refinement", fmt.Sprintf("%s precondition %q", ch, p),
						fmt.Sprintf("imposes an obligation on the shared client that the parent %s does not state", cid),
						"a child never strengthens the client's preconditions; state it in the parent or satisfy it inside the interior")
				default:
					refuse("trinity/decomp-dangling", fmt.Sprintf("%s precondition %q", ch, p),
						"fed by no wire and owed by no shared client — a dangling requirement",
						"wire a sibling guarantee into it, or restate the interior")
				}
			}
			// Invariants inherited, never weakened: the child cites
			// everything the parent cites.
			for _, inv := range info.cites {
				if !contains(ci.cites, inv) {
					refuse("trinity/decomp-cites", fmt.Sprintf("%s", ch),
						fmt.Sprintf("does not cite %q, inherited from %s — invariants are inherited, never weakened", inv, cid),
						"cite the parent's invariants in the child")
				}
			}
		}

		// Groundability: the interior must be executable. A child can act
		// once every precondition is owed by the shared client or fed by
		// a child that can already act; a closed feed loop grounds
		// nothing and is refused — an obligation that no execution order
		// can ever meet is not law, it is decoration.
		grounded := map[string]bool{}
		childIDs := sortedKeys(childSet)
		for changed := true; changed; {
			changed = false
			for _, ch := range childIDs {
				if grounded[ch] {
					continue
				}
				ci := infos[ch]
				ok := true
				for _, p := range ci.preconditions {
					if ci.client == info.client && contains(info.preconditions, p) {
						continue
					}
					fedByGrounded := false
					for _, f := range fedBy[ch][p] {
						if grounded[f] {
							fedByGrounded = true
							break
						}
					}
					if !fedByGrounded {
						ok = false
						break
					}
				}
				if ok {
					grounded[ch] = true
					changed = true
				}
			}
		}
		for _, ch := range childIDs {
			if !grounded[ch] {
				refuse("trinity/decomp-grounded", ch,
					fmt.Sprintf("cannot be reached by any execution order inside the interior of %s — its feeds never ground in client-owed input", cid),
					"feed the child from a groundable sibling or the shared client; a feed loop with no entry executes nothing")
			}
		}
	}

	return refusals
}

// regionFields returns an iterator over a struct region. An absent region
// iterates zero times with no error; a present-but-unreadable region
// returns the error for the caller to report as a finding.
func regionFields(v cue.Value, path string) (*cue.Iterator, error) {
	region := v.LookupPath(cue.ParsePath(path))
	if !region.Exists() {
		return cuecontext.New().CompileString("{}").Fields()
	}
	return region.Fields()
}

func fieldNames(v cue.Value, path string) ([]string, error) {
	iter, err := regionFields(v, path)
	if err != nil {
		return nil, err
	}
	var names []string
	for iter.Next() {
		names = append(names, iter.Selector().Unquoted())
	}
	return names, nil
}

// stringListValue reads a list value directly (the caller already holds
// it), as opposed to stringList which looks one up by path.
func stringListValue(v cue.Value) []string {
	var out []string
	list, err := v.List()
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

// sortedKeys walks a map deterministically: the refusal stream is
// protocol, so its order honors T0-3 exactly as the checks' verdicts do
// — identical inputs, identical output, run to run.
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// optionalFields walks authored data for fields declared with "?:",
// returning their paths in a stable order. A set is data: it states what
// is, and a constraint smuggled in as data is a statement nothing can
// check.
func optionalFields(v cue.Value) []string {
	var found []string
	var walk func(cue.Value, string)
	walk = func(node cue.Value, prefix string) {
		iter, err := node.Fields(cue.Optional(true))
		if err != nil {
			return
		}
		for iter.Next() {
			label := iter.Selector().String()
			path := label
			if prefix != "" {
				path = prefix + "." + label
			}
			if iter.Selector().ConstraintType()&cue.OptionalConstraint != 0 {
				found = append(found, path)
				continue // its interior is already unreachable
			}
			walk(iter.Value(), path)
		}
		if list, err := node.List(); err == nil {
			for i := 0; list.Next(); i++ {
				walk(list.Value(), fmt.Sprintf("%s[%d]", prefix, i))
			}
		}
	}
	walk(v, "")
	sort.Strings(found)
	return found
}

// openValue is one place a set leaves a value open: where it stands, and
// which form it is. The form is named because a red for the wrong reason
// proves nothing — a default and an open list are fixed differently.
type openValue struct{ path, reason string }

const (
	defaultReason    = "a default takes this arm only until something else names another: unified with a set naming a different arm, these same bytes read as that arm, so what is ratified here is a choice rather than a value"
	openListReason   = "an open list says more elements would be admissible rather than which elements there are: unified with a longer list, these same bytes gain the additions, so ratified law grows without an act"
	openStructReason = "an open struct says more fields would be admissible rather than which fields there are: unified with a set carrying more, these same bytes gain them, so ratified law grows without an act"
)

// openValues walks the authored syntax and reports every open value.
//
// The walk is structural where a path can be built — fields, structs and
// lists — so a refusal names where the open value stands. Anything else
// is an expression that computes one value from this same file, which is
// determinate and therefore fine; it is still swept, because an
// expression can enclose a default, and that is reported at the field it
// stands under.
//
// A parse failure is not reported: trinity/parse has already refused
// those bytes, and a second refusal would say the same thing twice.
func openValues(name string, data []byte) []openValue {
	f, err := parser.ParseFile(name, data)
	if err != nil {
		return nil
	}

	var found []openValue
	note := func(path, reason string) { found = append(found, openValue{path, reason}) }

	sweep := func(path string, n ast.Node) {
		ast.Walk(n, func(n ast.Node) bool {
			switch v := n.(type) {
			case *ast.UnaryExpr:
				if v.Op == token.MUL {
					note(path, defaultReason)
					return false
				}
			case *ast.Ellipsis:
				note(path, openListReason)
				return false
			}
			return true
		}, nil)
	}

	var expr func(path string, e ast.Expr)
	var decls func(path string, ds []ast.Decl)

	expr = func(path string, e ast.Expr) {
		switch v := e.(type) {
		case *ast.BasicLit: // a string, number, bool or null: a value
		case *ast.StructLit:
			decls(path, v.Elts)
		case *ast.ListLit:
			for i, el := range v.Elts {
				if _, open := el.(*ast.Ellipsis); open {
					note(path, openListReason)
					continue
				}
				expr(fmt.Sprintf("%s[%d]", path, i), el)
			}
		case *ast.UnaryExpr:
			if v.Op == token.MUL {
				note(path, defaultReason)
				return
			}
			expr(path, v.X)
		case *ast.BinaryExpr:
			expr(path, v.X)
			expr(path, v.Y)
		case *ast.ParenExpr:
			expr(path, v.X)
		default:
			sweep(path, e)
		}
	}

	decls = func(path string, ds []ast.Decl) {
		for _, d := range ds {
			switch v := d.(type) {
			case *ast.Field:
				if label, ok := labelOf(v.Label); ok {
					expr(join(path, label), v.Value)
					continue
				}
				// A computed or pattern label states which fields would
				// be admissible, which is the open-struct form written
				// another way.
				note(path, openStructReason)
			case *ast.Ellipsis:
				note(path, openStructReason)
			case *ast.EmbedDecl:
				// An embedding contributes its fields to this struct,
				// so it is walked as a value at the same path rather
				// than swept: a struct inside it is still a struct, and
				// its open forms are named where any other field's are.
				expr(path, v.Expr)
			case *ast.LetClause:
				// A let states a name, so the refusal points at it
				// rather than at the file: a set with one open value
				// somewhere is not a worklist.
				sweep(join(path, "let "+v.Ident.Name), d)
			default:
				// Everything else a struct can declare — a comprehension,
				// an attribute — states no label to build a path from,
				// and was walked past for exactly that reason. It still
				// reaches the value, so it is swept where it stands.
				sweep(path, d)
			}
		}
	}

	decls("", f.Decls)
	sort.Slice(found, func(i, j int) bool {
		if found[i].path != found[j].path {
			return found[i].path < found[j].path
		}
		return found[i].reason < found[j].reason
	})
	return found
}

// labelOf is the field name a label states, and whether it states one at
// all. A quoted label is rendered quoted, so a path reads the way the
// rest of the gates' paths read: contracts."C-LEND-1".status.
func labelOf(l ast.Label) (string, bool) {
	switch v := l.(type) {
	case *ast.Ident:
		return v.Name, true
	case *ast.BasicLit:
		if v.Kind == token.STRING {
			return v.Value, true
		}
	}
	return "", false
}

func join(path, label string) string {
	if path == "" {
		return label
	}
	return path + "." + label
}
