// Package diff is the differ: a read path that computes gaps — the
// structured disequilibrium between goal-law and a view — and holds no
// authority. Gaps inform planning and decide nothing (schema/gap.cue); a
// gap is a planning state, never an error state, so a run that finds
// gaps is a successful run.
//
// v0 compares law against law at the CONTRACTS REGION ONLY, clause
// grain where clauses exist — registry, invariants, lexicon, and
// experience deltas are not yet compared, so an empty gap list means
// contract equilibrium, not whole-set equivalence. The absorbed current view arrives with the
// absorber; until then the view side is any set file. Gap ids are
// deterministic per run (sorted address order) and ephemeral until the
// store assumes gap custody.
package diff

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	"cuelang.org/go/cue"

	"github.com/xormania/looplaw/internal/gate"
	"github.com/xormania/looplaw/internal/outcome"
)

// Checks enumerates every check id the differ can emit; the suite proves
// a red for each or declares a reasoned exemption.
var Checks = []string{
	"diff/side",
	"diff/goal-provenance",
	"diff/subject-mismatch",
	"diff/self-check",
}

// Gap mirrors schema/gap.cue #Gap. Status is always "open" in v0 (the
// fuller lifecycle binds when the store assumes gap custody).
type Gap struct {
	ID       string  `json:"id"`
	Subject  string  `json:"subject"`
	Address  Address `json:"address"`
	Kind     string  `json:"kind"`
	Work     string  `json:"work"`
	Detail   string  `json:"detail"`
	GoalHash string  `json:"goal_hash"`
	ViewHash string  `json:"view_hash"`
	Status   string  `json:"status"`
}

type Address struct {
	Contract string `json:"contract"`
	Clause   string `json:"clause,omitempty"`
}

// Diff loads both sides through the trinity gates and computes the gap
// list. Refusals are non-empty only when a side fails its gates or the
// subjects differ — a diff over refused law would be meaningless.
func Diff(goalPath, viewPath string) ([]Gap, []outcome.Refusal) {
	goal, goalRefusals := gate.LoadSet(goalPath)
	view, viewRefusals := gate.LoadSet(viewPath)

	var refusals []outcome.Refusal
	for _, side := range []struct {
		name string
		rs   []outcome.Refusal
	}{{"goal", goalRefusals}, {"view", viewRefusals}} {
		for _, r := range side.rs {
			// The class travels. Which side failed is what this adds;
			// what kind of failure it was is the gate's to say, and
			// replacing it with a constant told a caller a readable set
			// had been refused by policy when the file was not there —
			// a rejection is repaired by editing the set, an abort by
			// retrying once the infrastructure is back, and the exit
			// code a caller branches on follows the class.
			refusals = append(refusals, outcome.Refusal{
				Class:   r.Class,
				Check:   "diff/side",
				Subject: fmt.Sprintf("%s side (%s)", side.name, r.Subject),
				Reason:  r.Error(),
				Remedy:  "both sides must be readable and pass the trinity gates before a diff means anything",
			})
		}
	}
	if len(refusals) > 0 {
		return nil, refusals
	}

	// Evidence never sets the standard it is measured against: a set
	// carrying provenance is an absorbed view, so accepting one as the
	// goal side would let a party's claim become the law reality is
	// compared to (T0-2, nothing ascending confers standing). The view
	// side is unconstrained — with or without provenance it is
	// legitimately a view.
	if goal.LookupPath(cue.ParsePath("provenance")).Exists() {
		return nil, []outcome.Refusal{{
			Class:   outcome.Rejection,
			Check:   "diff/goal-provenance",
			Subject: goalPath,
			Reason:  "the goal side carries provenance, so it is an absorbed view — evidence, not goal-law",
			Remedy:  "diff against ratified goal-law; a view becomes law only through the accountable authority's amendment path",
		}}
	}

	goalSubject, _ := goal.LookupPath(cue.ParsePath("subject")).String()
	viewSubject, _ := view.LookupPath(cue.ParsePath("subject")).String()
	if goalSubject != viewSubject {
		return nil, []outcome.Refusal{{
			Class:   outcome.Rejection,
			Check:   "diff/subject-mismatch",
			Subject: fmt.Sprintf("goal %q vs view %q", goalSubject, viewSubject),
			Reason:  "the sides describe different systems",
			Remedy:  "diff a goal against a view of the same subject",
		}}
	}

	goalContracts := contractsOf(goal)
	viewContracts := contractsOf(view)

	var gaps []Gap
	add := func(contract, clause, kind, work, detail, goalHash, viewHash string) {
		gaps = append(gaps, Gap{
			Subject:  goalSubject,
			Address:  Address{Contract: contract, Clause: clause},
			Kind:     kind,
			Work:     work,
			Detail:   detail,
			GoalHash: goalHash,
			ViewHash: viewHash,
			Status:   "open",
		})
	}

	for id, gc := range goalContracts {
		vc, ok := viewContracts[id]
		if !ok {
			work := "fill"
			detail := "contract absent from the view"
			if gc.hasInterior {
				work = "split"
				detail = "contract absent from the view; the goal decomposes it, so the work begins by decomposing"
			}
			add(id, "", "absent", work, detail, gc.hash, "")
			continue
		}
		diffClauses(add, id, gc, vc)
		if gc.fieldsHash != vc.fieldsHash {
			add(id, "", "changed", "fill",
				"contract-grain fields differ: "+strings.Join(differingFields(gc.fields, vc.fields), ", "),
				gc.fieldsHash, vc.fieldsHash)
		}
	}
	for id, vc := range viewContracts {
		if _, ok := goalContracts[id]; !ok {
			add(id, "", "added", "fill",
				"contract present in the view but absent from goal-law — reconciliation is the accountable authority's amendment path, never silent adoption",
				"", vc.hash)
		}
	}

	sort.Slice(gaps, func(i, j int) bool {
		a, b := gaps[i], gaps[j]
		if a.Address.Contract != b.Address.Contract {
			return a.Address.Contract < b.Address.Contract
		}
		if a.Address.Clause != b.Address.Clause {
			return a.Address.Clause < b.Address.Clause
		}
		return a.Kind < b.Kind
	})
	for i := range gaps {
		gaps[i].ID = fmt.Sprintf("GAP-%d", i+1)
	}

	if r := selfCheck(gaps); r != nil {
		return nil, []outcome.Refusal{*r}
	}
	return gaps, nil
}

type clauseInfo struct {
	text    string
	records string
}

func (c clauseInfo) hash() string {
	return hashOf(hashDelim(c.text) + hashDelim(c.records))
}

type contractInfo struct {
	hash        string
	fieldsHash  string
	hasInterior bool
	// Every contract-grain field, by name, so a gap can say which ones
	// differ rather than listing the fields the differ happens to know.
	fields  map[string]string
	clauses map[string]clauseInfo // raw clause ids; disjoint across regions by the shape gate's P-/G-/LI- grammar closure
}

// clauseRegions are diffed at clause grain, so they are not part of the
// contract-grain digest: a changed precondition is a gap addressed to
// that precondition, not to the contract.
var clauseRegions = map[string]bool{
	"preconditions": true, "guarantees": true, "invariants_local": true,
}

// fieldDigest determines a value: length-delimited so no boundary is
// ambiguous, and struct fields in sorted order so a map's iteration
// order cannot change the digest while its content is the same.
//
// Written over any value rather than field by field, because field by
// field is how four of them went missing. #Contract states name, blame,
// status and trigger, and the digest covered none of them — so a
// contract could move from ratified to withdrawn, or reassign fault
// between two registered parties, and the gap feed answered [].
func fieldDigest(v cue.Value) string {
	switch v.Kind() {
	case cue.StructKind:
		iter, err := v.Fields()
		if err != nil {
			return hashDelim("unreadable:" + err.Error())
		}
		digests := map[string]string{}
		var keys []string
		for iter.Next() {
			k := iter.Selector().Unquoted()
			digests[k] = fieldDigest(iter.Value())
			keys = append(keys, k)
		}
		sort.Strings(keys)
		out := hashDelim("{")
		for _, k := range keys {
			out += hashDelim(k) + digests[k]
		}
		return out + hashDelim("}")
	case cue.ListKind:
		list, err := v.List()
		if err != nil {
			return hashDelim("unreadable:" + err.Error())
		}
		out := hashDelim("[")
		for list.Next() {
			out += fieldDigest(list.Value())
		}
		return out + hashDelim("]")
	default:
		// Scalars through CUE's own encoding, so a string, a number and
		// a bool that spell the same are still different values.
		b, err := v.MarshalJSON()
		if err != nil {
			return hashDelim("unreadable:" + err.Error())
		}
		return hashDelim(string(b))
	}
}

// differingFields names the contract-grain fields the two sides do not
// agree on, including one present on a single side.
func differingFields(a, b map[string]string) []string {
	var out []string
	seen := map[string]bool{}
	for name, digest := range a {
		seen[name] = true
		if b[name] != digest {
			out = append(out, name)
		}
	}
	for name := range b {
		if !seen[name] {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

func contractsOf(set cue.Value) map[string]contractInfo {
	out := map[string]contractInfo{}
	iter, err := set.LookupPath(cue.ParsePath("contracts")).Fields()
	if err != nil {
		return out
	}
	for iter.Next() {
		id := iter.Selector().Unquoted()
		c := iter.Value()
		info := contractInfo{clauses: map[string]clauseInfo{}}

		for _, region := range []string{"preconditions", "guarantees", "invariants_local"} {
			citer, err := c.LookupPath(cue.ParsePath(region)).Fields()
			if err != nil {
				continue
			}
			for citer.Next() {
				cid := citer.Selector().Unquoted()
				text, _ := citer.Value().LookupPath(cue.ParsePath("text")).String()
				records, _ := citer.Value().LookupPath(cue.ParsePath("records")).String()
				info.clauses[cid] = clauseInfo{text: text, records: records}
			}
		}

		info.hasInterior = c.LookupPath(cue.ParsePath("interior")).Exists()

		// Every field the contract states, less the clause regions that
		// are diffed at clause grain. Read from the contract rather than
		// from a list beside it, so a field the schema gains is in the
		// digest the day it is authored.
		info.fields = map[string]string{}
		if fiter, err := c.Fields(); err == nil {
			for fiter.Next() {
				name := fiter.Selector().Unquoted()
				if clauseRegions[name] {
					continue
				}
				info.fields[name] = fieldDigest(fiter.Value())
			}
		}
		var fields string
		for _, name := range sortedFields(info.fields) {
			fields += hashDelim(name) + info.fields[name]
		}
		info.fieldsHash = hashOf(fields)

		var whole string
		var cids []string
		for cid := range info.clauses {
			cids = append(cids, cid)
		}
		sort.Strings(cids)
		for _, cid := range cids {
			whole += hashDelim(cid) + info.clauses[cid].hash()
		}
		info.hash = hashOf(fields + whole)

		out[id] = info
	}
	return out
}

func diffClauses(add func(contract, clause, kind, work, detail, gh, vh string), contract string, gc, vc contractInfo) {
	var ids []string
	seen := map[string]bool{}
	for id := range gc.clauses {
		ids = append(ids, id)
		seen[id] = true
	}
	for id := range vc.clauses {
		if !seen[id] {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)

	for _, id := range ids {
		g, inGoal := gc.clauses[id]
		v, inView := vc.clauses[id]
		switch {
		case inGoal && !inView:
			add(contract, id, "absent", "fill", "clause absent from the view", g.hash(), "")
		case !inGoal && inView:
			add(contract, id, "added", "fill", "clause present in the view but absent from goal-law", "", v.hash())
		case g != v:
			add(contract, id, "changed", "fill", "clause text differs between goal-law and the view", g.hash(), v.hash())
		}
	}
}

func sortedFields(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func hashOf(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func hashDelim(s string) string {
	return fmt.Sprintf("%d:%s|", len(s), s)
}
