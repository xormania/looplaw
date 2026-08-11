// Package provenance holds the kernel-side half of staleness: the
// comparison of recorded provenance against a submitted manifest.
//
// The lane is the point. A client reads a scope and submits a manifest;
// the kernel compares what it was given against what it recorded and
// touches no filesystem (T0-4: the kernel never fetches or inspects
// work-product content). Nothing here opens a file, so the comparison
// is the same computation wherever it runs — which is what makes
// staleness deterministic while re-derivation stays inference.
package provenance

import (
	"sort"

	"cuelang.org/go/cue"
)

// Manifest is a content-hash baseline over a scope: path to sha256,
// with the scope name its submitter used. Plain data, submitted.
type Manifest struct {
	Scope   string            `json:"scope"`
	Sources map[string]string `json:"sources"`
}

// Paths returns the manifest's paths in sorted order, so every output
// derived from a manifest is deterministic.
func (m Manifest) Paths() []string {
	paths := make([]string, 0, len(m.Sources))
	for p := range m.Sources {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	return paths
}

// Report is the staleness finding: which sources moved under a view,
// and which statements were derived from them. Evidence about the view,
// not a judgment on it — a stale view is not wrong, it is owed a
// re-derivation, which only a client can perform.
type Report struct {
	ScopeRecorded  string `json:"scope_recorded"`
	ScopeSubmitted string `json:"scope_submitted"`
	ScopeMismatch  bool   `json:"scope_mismatch"`
	Stale          bool   `json:"stale"`
	// BaselineUnreadable is true when the recorded provenance carries no
	// readable sources. Distinct from "no sources recorded": one is a
	// baseline of nothing, the other is a baseline that could not be
	// read, and only the first supports a verdict.
	BaselineUnreadable bool        `json:"baseline_unreadable"`
	Changed            []string    `json:"changed"` // baselined, hash differs
	Missing            []string    `json:"missing"` // baselined, absent now
	Added              []string    `json:"added"`   // submitted, never baselined
	Affected           []Affected  `json:"affected"`
	Counts             ReportCount `json:"counts"`
}

type Affected struct {
	Address string   `json:"address"`
	Sources []string `json:"sources"`
}

type ReportCount struct {
	Baselined int `json:"baselined"`
	Current   int `json:"current"`
}

// Compare reads no filesystem: the caller supplies both sides.
func Compare(prov cue.Value, m Manifest) Report {
	recordedScope, _ := prov.LookupPath(cue.ParsePath("scope")).String()
	rep := Report{
		ScopeRecorded:  recordedScope,
		ScopeSubmitted: m.Scope,
		// A manifest from a different scope than the one recorded makes
		// every comparison below meaningless, so it is stated rather
		// than left for the reader to notice.
		ScopeMismatch: recordedScope != m.Scope,
	}

	// A baseline that cannot be read is not an empty baseline. Skipping
	// silently left every submitted source classified "Added", and added
	// alone is not stale — so provenance nobody could read reported
	// "stale": false, which is a verdict drawn from a failure. The gates
	// refuse such a set today, which makes this latent rather than live;
	// a check that is correct only because something upstream is doing
	// its job is one refactor from being wrong.
	baseline := map[string]string{}
	sources := prov.LookupPath(cue.ParsePath("sources"))
	iter, err := sources.Fields()
	switch {
	// Absent is not empty, and Fields() cannot tell them apart: on a
	// region that is not there it returns an empty iterator and no
	// error, which reads as a baseline of nothing. The schema requires
	// the region, so its absence means the provenance could not be read
	// rather than that nothing was recorded.
	case !sources.Exists(), err != nil:
		rep.BaselineUnreadable = true
	default:
		for iter.Next() {
			h, herr := iter.Value().String()
			if herr != nil {
				// A digest that is not a string cannot be compared
				// against one. Recording it as "" would make every
				// current file look changed, which is a different wrong
				// answer, so the baseline is declared unreadable instead.
				rep.BaselineUnreadable = true
				continue
			}
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
	// whether the new content bears on the statements is a judgment the
	// client makes. Moved baselined sources do — what the view was
	// derived from is no longer what it was. A scope mismatch is stale
	// on its face: nothing recorded describes what was submitted.
	// An unreadable baseline is stale by construction. Nothing here can
	// show the view current, and "not stale" is the one answer a caller
	// acts on by doing nothing — so the failure must not render as the
	// reassuring outcome.
	rep.Stale = len(rep.Changed) > 0 || len(rep.Missing) > 0 || rep.ScopeMismatch || rep.BaselineUnreadable
	return rep
}
