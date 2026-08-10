package absorb

import (
	"sort"

	"cuelang.org/go/cue"
)

// Report is the staleness finding: which sources moved under a view,
// and which absorbed statements those sources were derived from. It is
// evidence about the view, not a judgment on it — a stale view is not
// wrong, it is owed a re-derivation, and only the client can do that.
type Report struct {
	Scope    string      `json:"scope"`
	Stale    bool        `json:"stale"`
	Changed  []string    `json:"changed"`  // baselined, hash differs
	Missing  []string    `json:"missing"`  // baselined, absent from the scope now
	Added    []string    `json:"added"`    // in the scope, never baselined
	Affected []Affected  `json:"affected"` // statements whose sources moved
	Counts   ReportCount `json:"counts"`
}

type Affected struct {
	Address string   `json:"address"`
	Sources []string `json:"sources"`
}

type ReportCount struct {
	Baselined int `json:"baselined"`
	Current   int `json:"current"`
}

// Compare is the staleness comparison: recorded provenance against a
// submitted manifest. It reads no filesystem — the caller supplies both
// sides, so this is the same computation whoever runs it, which is what
// makes staleness detection deterministic while re-derivation stays
// inference.
func Compare(prov cue.Value, m Manifest) Report {
	rep := Report{Scope: m.Scope}

	baseline := map[string]string{}
	if iter, err := prov.LookupPath(cue.ParsePath("sources")).Fields(); err == nil {
		for iter.Next() {
			h, _ := iter.Value().String()
			baseline[iter.Selector().Unquoted()] = h
		}
	}
	rep.Counts = ReportCount{Baselined: len(baseline), Current: len(m.Sources)}

	moved := map[string]bool{}
	for path, was := range baseline {
		now, present := m.Sources[path]
		switch {
		case !present:
			rep.Missing = append(rep.Missing, path)
			moved[path] = true
		case now != was:
			rep.Changed = append(rep.Changed, path)
			moved[path] = true
		}
	}
	for path := range m.Sources {
		if _, ok := baseline[path]; !ok {
			rep.Added = append(rep.Added, path)
		}
	}
	sort.Strings(rep.Changed)
	sort.Strings(rep.Missing)
	sort.Strings(rep.Added)

	if iter, err := prov.LookupPath(cue.ParsePath("derivations")).Fields(); err == nil {
		for iter.Next() {
			addr := iter.Selector().Unquoted()
			var hit []string
			if list, err := iter.Value().List(); err == nil {
				for list.Next() {
					src, _ := list.Value().String()
					if moved[src] {
						hit = append(hit, src)
					}
				}
			}
			if len(hit) > 0 {
				sort.Strings(hit)
				rep.Affected = append(rep.Affected, Affected{Address: addr, Sources: hit})
			}
		}
	}
	sort.Slice(rep.Affected, func(i, j int) bool { return rep.Affected[i].Address < rep.Affected[j].Address })

	// Added sources alone do not make a view stale: the scope grew, and
	// whether the new content bears on the law is a judgment the client
	// makes. Moved baselined sources do — what the view was derived
	// from is no longer what it was.
	rep.Stale = len(rep.Changed) > 0 || len(rep.Missing) > 0
	return rep
}
