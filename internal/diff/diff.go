// Package diff is the differ: a read path that computes gaps — the
// structured disequilibrium between goal-law and a view — and holds no
// authority. Gaps inform planning and decide nothing (law/gap.cue); a
// gap is a planning state, never an error state, so a run that finds
// gaps is a successful run.
//
// v0 compares law against law at the contracts region, clause grain
// where clauses exist. The absorbed current view arrives with the
// absorber; until then the view side is any set file. Gap ids are
// deterministic per run (sorted address order) and ephemeral until the
// store assumes gap custody.
package diff

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"

	"cuelang.org/go/cue"

	"github.com/xormania/looplaw/internal/gate"
	"github.com/xormania/looplaw/internal/outcome"
)

// Checks enumerates every check id the differ can emit; the suite proves
// a red for each or declares a reasoned exemption.
var Checks = []string{
	"diff/side",
	"diff/subject-mismatch",
	"diff/self-check",
}

// Gap mirrors law/gap.cue #Gap. Status is always "open" in v0 (the
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
	for side, rs := range map[string][]outcome.Refusal{"goal": goalRefusals, "view": viewRefusals} {
		for _, r := range rs {
			refusals = append(refusals, outcome.Refusal{
				Class:   outcome.Rejection,
				Check:   "diff/side",
				Subject: fmt.Sprintf("%s side (%s)", side, r.Subject),
				Reason:  r.Error(),
				Remedy:  "both sides must pass the trinity gates before a diff means anything",
			})
		}
	}
	if len(refusals) > 0 {
		return nil, refusals
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
				"contract-grain fields differ (parties, acts, cites, or interior)",
				gc.fieldsHash, vc.fieldsHash)
		}
	}
	for id, vc := range viewContracts {
		if _, ok := goalContracts[id]; !ok {
			add(id, "", "added", "fill",
				"contract present in the view but absent from goal-law — reconciliation is the aa's amendment path, never silent adoption",
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
	text string
}

type contractInfo struct {
	hash        string
	fieldsHash  string
	hasInterior bool
	clauses     map[string]clauseInfo // "P-1", "G-1", "LI-1" keys prefixed by region
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
				if records, err := citer.Value().LookupPath(cue.ParsePath("records")).String(); err == nil {
					text += "\x00records:" + records
				}
				info.clauses[cid] = clauseInfo{text: text}
			}
		}

		info.hasInterior = c.LookupPath(cue.ParsePath("interior")).Exists()

		var fields string
		for _, p := range []string{"parties.client", "parties.supplier", "synchronization"} {
			v, _ := c.LookupPath(cue.ParsePath(p)).String()
			fields += hashDelim(v)
		}
		for _, list := range []string{"acts", "cites"} {
			liter, err := c.LookupPath(cue.ParsePath(list)).List()
			if err == nil {
				for liter.Next() {
					s, _ := liter.Value().String()
					fields += hashDelim(s)
				}
			}
			fields += "|"
		}
		fields += fmt.Sprintf("interior:%v", info.hasInterior)
		info.fieldsHash = hashOf(fields)

		var whole string
		var cids []string
		for cid := range info.clauses {
			cids = append(cids, cid)
		}
		sort.Strings(cids)
		for _, cid := range cids {
			whole += hashDelim(cid) + hashDelim(info.clauses[cid].text)
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
			add(contract, id, "absent", "fill", "clause absent from the view", hashOf(g.text), "")
		case !inGoal && inView:
			add(contract, id, "added", "fill", "clause present in the view but absent from goal-law", "", hashOf(v.text))
		case g.text != v.text:
			add(contract, id, "changed", "fill", "clause text differs between goal-law and the view", hashOf(g.text), hashOf(v.text))
		}
	}
}

func hashOf(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func hashDelim(s string) string {
	return fmt.Sprintf("%d:%s|", len(s), s)
}
