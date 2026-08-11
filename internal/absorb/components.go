package absorb

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"

	"github.com/xormania/looplaw/internal/gate"
	"github.com/xormania/looplaw/internal/outcome"
)

// ComponentChecks enumerates every check loading a component manifest
// can emit. As with the trinity and submission gates, the suite proves a
// red for each: an undemonstrated check is an unproven behavior.
var ComponentChecks = []string{
	"components/load",
	"components/schema-load",
	"components/parse",
	"components/shape",
	"components/decode",
	"components/source-conflict",
	"components/unlisted",
	"components/sourceless",
	"components/id-collision",
}

// ComponentManifest is what a client derived about a system's shape:
// which components exist, what they were derived from, and which holds a
// compiled reference to which.
//
// looplaw derives none of it. Working out a codebase's components is
// language-specific and belongs with whatever reads that language; the
// kernel decides over the manifest it is handed, as it does for the
// content-hash manifest a scope scan produces.
type ComponentManifest struct {
	Subject    string
	Components map[string]Component
	Depends    map[string][]string
}

// Component is one part of the system, as the client found it: what it
// is called, what it is, and the sources it was derived from with their
// content hash at derivation time.
type Component struct {
	Note    string
	Sources map[string]string
}

// SourcePaths returns a component's source paths in a stable order.
// Output order is part of what consumers script against, so nothing
// here walks a map directly.
func (c Component) SourcePaths() []string {
	out := make([]string, 0, len(c.Sources))
	for p := range c.Sources {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

// SourcePaths returns every source path in the manifest, deduplicated
// and in a stable order — the paths provenance will carry.
func (m ComponentManifest) SourcePaths() []string {
	seen := map[string]bool{}
	var out []string
	for _, name := range m.Names() {
		for _, p := range m.Components[name].SourcePaths() {
			if !seen[p] {
				seen[p] = true
				out = append(out, p)
			}
		}
	}
	sort.Strings(out)
	return out
}

// SourceHash is the digest recorded for a path. Load refuses a manifest
// where two components disagree about one, so by here there is one
// answer.
func (m ComponentManifest) SourceHash(path string) string {
	for _, name := range m.Names() {
		if h, ok := m.Components[name].Sources[path]; ok {
			return h
		}
	}
	return ""
}

// Names returns the component names in a stable order. Refusal and
// output order is part of what consumers script against, so nothing here
// walks a map directly.
func (m ComponentManifest) Names() []string {
	out := make([]string, 0, len(m.Components))
	for name := range m.Components {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// LoadComponents reads a submitted component manifest and checks it
// against the ratified shape. Refusals carry their remedy; nothing is
// derived that the manifest did not state.
func LoadComponents(path string) (ComponentManifest, []outcome.Refusal) {
	data, err := os.ReadFile(path)
	if err != nil {
		return ComponentManifest{}, []outcome.Refusal{{
			Class: outcome.Abort, Check: "components/load", Subject: path,
			Reason: err.Error(), Remedy: "point the act at a readable component manifest",
		}}
	}

	ctx := cuecontext.New()
	sch, err := gate.Law(ctx)
	if err != nil {
		return ComponentManifest{}, []outcome.Refusal{{
			Class: outcome.Abort, Check: "components/schema-load", Subject: "schema (embedded)",
			Reason: err.Error(),
			Remedy: "the embedded schema is broken; replace this binary with one embedding the ratified schema",
		}}
	}

	v := ctx.CompileBytes(data, cue.Filename(path))
	if v.Err() != nil {
		return ComponentManifest{}, []outcome.Refusal{{
			Class: outcome.Rejection, Check: "components/parse", Subject: path,
			Reason: v.Err().Error(),
			Remedy: "the manifest must be well-formed CUE before any check can read it",
		}}
	}

	unified := sch.LookupPath(cue.ParsePath("#ComponentManifest")).Unify(v)
	if err := unified.Validate(cue.Concrete(true)); err != nil {
		return ComponentManifest{}, []outcome.Refusal{{
			Class: outcome.Rejection, Check: "components/shape", Subject: path,
			Reason: err.Error(),
			Remedy: "align the manifest with #ComponentManifest (schema/components.cue)",
		}}
	}

	var m ComponentManifest
	if err := unified.Decode(&m); err != nil {
		return ComponentManifest{}, []outcome.Refusal{{
			Class: outcome.Abort, Check: "components/decode", Subject: path,
			Reason: err.Error(), Remedy: "this binary is broken; do not consume its output",
		}}
	}

	var refusals []outcome.Refusal

	// Provenance is one digest per path. Two components claiming the same
	// source with different digests cannot both be right, and picking one
	// would record a baseline that never existed — staleness would then be
	// measured against fiction.
	digests := map[string]string{}
	owners := map[string]string{}
	for _, name := range m.Names() {
		for _, path := range m.Components[name].SourcePaths() {
			h := m.Components[name].Sources[path]
			if prior, seen := digests[path]; seen && prior != h {
				refusals = append(refusals, outcome.Refusal{
					Class: outcome.Rejection, Check: "components/source-conflict", Subject: path,
					Reason: fmt.Sprintf("%q records digest %s and %q records %s for the same source",
						owners[path], prior[:12], name, h[:12]),
					Remedy: "hash each source once and give every component the same digest; a baseline that never existed measures staleness against fiction",
				})
				continue
			}
			digests[path], owners[path] = h, name
		}
	}

	// A component derived from nothing produces a contract addressed by
	// no source. Provenance is what makes a statement falsifiable — an
	// unsourced one cannot go stale, so nothing could ever contradict it.
	for _, name := range m.Names() {
		if len(m.Components[name].Sources) == 0 {
			refusals = append(refusals, outcome.Refusal{
				Class: outcome.Rejection, Check: "components/sourceless", Subject: name,
				Reason: "the component names no source it was derived from",
				Remedy: "name the sources with their digests, or leave the component out; a statement no source can falsify is not evidence",
			})
		}
	}

	// Ids are folded from names, and two names can fold to one id. A
	// party or contract sharing an id names neither of the things it came
	// from, and the view would carry one where the manifest stated two.
	byParty := map[string]string{}
	for _, name := range m.Names() {
		id := partyID(name)
		if prior, seen := byParty[id]; seen {
			refusals = append(refusals, outcome.Refusal{
				Class: outcome.Rejection, Check: "components/id-collision", Subject: id,
				Reason: fmt.Sprintf("%q and %q both become party %q", prior, name, id),
				Remedy: "name the components so their ids differ; one id for two components names neither",
			})
			continue
		}
		byParty[id] = name
	}
	byContract := map[string]string{}
	for _, from := range m.Names() {
		deps := append([]string(nil), m.Depends[from]...)
		sort.Strings(deps)
		for _, to := range deps {
			id := contractID(from, to)
			edge := from + " -> " + to
			if prior, seen := byContract[id]; seen {
				refusals = append(refusals, outcome.Refusal{
					Class: outcome.Rejection, Check: "components/id-collision", Subject: id,
					Reason: fmt.Sprintf("the dependencies %q and %q both become contract %q", prior, edge, id),
					Remedy: "name the components so their ids differ; one contract for two dependencies governs neither",
				})
				continue
			}
			byContract[id] = edge
		}
	}

	// An edge to a component the manifest does not list would register a
	// party by implication. Nothing enters a set by being referenced.
	for _, from := range sortedDependKeys(m.Depends) {
		if _, ok := m.Components[from]; !ok {
			refusals = append(refusals, outcome.Refusal{
				Class: outcome.Rejection, Check: "components/unlisted", Subject: from,
				Reason: "depends on entries name a component the manifest does not list",
				Remedy: "list every component the manifest refers to; a component that enters by being referenced was never derived",
			})
			continue
		}
		for _, to := range m.Depends[from] {
			if _, ok := m.Components[to]; !ok {
				refusals = append(refusals, outcome.Refusal{
					Class: outcome.Rejection, Check: "components/unlisted", Subject: to,
					Reason: fmt.Sprintf("%q depends on it, and the manifest does not list it", from),
					Remedy: "list every component the manifest refers to; a component that enters by being referenced was never derived",
				})
			}
		}
	}
	return m, refusals
}

func sortedDependKeys(d map[string][]string) []string {
	out := make([]string, 0, len(d))
	for k := range d {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// partyID turns a component name into a party id. Names carry separators
// a party id may not, so they are folded rather than rejected — the
// component's own name is kept in the registry entry, which is where a
// reader looks.
func partyID(name string) string {
	id := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			return r
		case r == '-':
			return r
		default:
			return '-'
		}
	}, strings.ToLower(name))
	id = strings.Trim(id, "-")
	for strings.Contains(id, "--") {
		id = strings.ReplaceAll(id, "--", "-")
	}
	return id
}

// contractID names the contract governing one dependency, derived from
// the pair so the same edge always produces the same id.
func contractID(from, to string) string {
	return "C-" + strings.ToUpper(partyID(from)+"-"+partyID(to))
}

// ComponentSkeleton renders a draft view from a component manifest.
//
// What a tool can establish is filled in: the components exist, so they
// are registered; one references another, so a contract governs the
// pair; each was derived from files, so provenance cites them. What only
// an author can state is left empty — acts, preconditions, guarantees,
// blame — and the gates refuse the set until they are stated. Those
// refusals are the worklist, which is the point: on an existing system
// they arrive already scoped to what the code actually is.
func ComponentSkeleton(m ComponentManifest) string {
	var b strings.Builder
	fmt.Fprintf(&b, `// DRAFT VIEW SKELETON — not yet a valid set.
//
// Derived from a submitted component manifest. Registry, contracts and
// provenance below state only what a tool established: these components
// exist, each was derived from these sources, and one holds a compiled
// reference to another. The statement regions are empty and the gates
// will refuse this set until they are authored — those refusals are the
// worklist.
//
// A contract here says only that a dependency exists, not what it
// promises. Filling in acts, preconditions, guarantees and blame is the
// work; where a dependency turns out to promise nothing worth stating,
// that is a finding about the system rather than a gap in this file.
//
// This view is evidence, never law: it states what a party claims the
// system currently is — submitted as a claim, recorded never believed.
// Law is authored and ratified separately.
//
// Declare experience_declared_absent yourself: silence is not a
// declaration, so the binary leaves it to the author.
subject:        %s
schema_version: "0"

registry: {
`, q(m.Subject))

	for _, name := range m.Names() {
		// authority_free is deliberately absent, not defaulted. Whether a
		// component holds an authority is a design statement no deriver
		// can make, and asserting either value would settle by tooling
		// what the shape gate should be refusing until an author states
		// it. Left off, every party draws trinity/shape — a worklist
		// entry, which is what the header promises.
		fmt.Fprintf(&b, "\t%s: {name: %s, note: %s}  // authority_free: true|false\n",
			q(partyID(name)), q(name), q(m.Components[name].Note))
	}
	b.WriteString("}\n\ninvariants: {}\nlexicon: {}\n\ncontracts: {\n")

	for _, from := range m.Names() {
		deps := append([]string(nil), m.Depends[from]...)
		sort.Strings(deps)
		for _, to := range deps {
			fmt.Fprintf(&b, `	%s: {
		id:   %s
		name: %s
		parties: {client: %s, supplier: %s}
		acts: []              // TODO: the reserved operations this contract holds
		preconditions: {}     // TODO: what %s owes before calling
		guarantees: {}        // TODO: what %s promises, and what it records
		invariants_local: {}
		cites: []
		blame: []             // TODO: who is at fault for which violation class
		status: "proposed"
	}
`, q(contractID(from, to)), q(contractID(from, to)),
				q(from+" depends on "+to),
				q(partyID(from)), q(partyID(to)), from, to)
		}
	}

	b.WriteString("}\n\nexperience: {}\n// experience_declared_absent: true|false\n\nprovenance: {\n\tscope: ")
	fmt.Fprintf(&b, "%s\n\tsources: {\n", q(m.Subject))
	// One entry per path, in path order. A path claimed by two
	// components with different digests is refused at load rather than
	// silently resolved here.
	for _, path := range m.SourcePaths() {
		fmt.Fprintf(&b, "\t\t%s: %s\n", q(path), q(m.SourceHash(path)))
	}
	b.WriteString("\t}\n\tderivations: {\n")
	for _, from := range m.Names() {
		deps := append([]string(nil), m.Depends[from]...)
		sort.Strings(deps)
		for _, to := range deps {
			// A contract about a dependency derived from both sides'
			// sources: the claim is about the pair, so both are what
			// could falsify it.
			seen := map[string]bool{}
			var srcs []string
			for _, side := range []string{from, to} {
				for _, p := range m.Components[side].SourcePaths() {
					if !seen[p] {
						seen[p] = true
						srcs = append(srcs, p)
					}
				}
			}
			sort.Strings(srcs)
			quoted := make([]string, 0, len(srcs))
			for _, p := range srcs {
				quoted = append(quoted, q(p))
			}
			fmt.Fprintf(&b, "\t\t%s: [%s]\n", q(contractID(from, to)), strings.Join(quoted, ", "))
		}
	}
	b.WriteString("\t}\n}\n")
	return b.String()
}
