// Command looplaw is the law organ of the loopstack: it stores contract
// sets, gates what may enter them, and reports the gap between ratified
// goal-law and absorbed evidence. Design basis: proj/looplaw-spec.md.
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"cuelang.org/go/cue"

	"github.com/xormania/looplaw/internal/absorb"
	"github.com/xormania/looplaw/internal/diff"
	"github.com/xormania/looplaw/internal/gate"
	"github.com/xormania/looplaw/internal/outcome"
	"github.com/xormania/looplaw/internal/project"
	"github.com/xormania/looplaw/internal/provenance"
)

const version = "0.0.0-dev"

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(outcome.ExitUsage)
	}
	switch os.Args[1] {
	case "version":
		fmt.Println("looplaw " + version)
		os.Exit(outcome.ExitOK)
	case "validate":
		if len(os.Args) != 3 {
			fmt.Fprintln(os.Stderr, "usage: looplaw validate <set.cue>")
			os.Exit(outcome.ExitUsage)
		}
		set, refusals := gate.LoadSet(os.Args[2])
		if len(refusals) == 0 {
			// Which lane the set is in is the distinction the gates now
			// draw, so the verdict line carries it: a bare "ok" over a
			// party's claim reads as acceptance of its content.
			if set.LookupPath(cue.ParsePath("provenance")).Exists() {
				fmt.Println("ok (absorbed view — evidence, not law)")
			} else {
				fmt.Println("ok (authored set)")
			}
			os.Exit(outcome.ExitOK)
		}
		exit := outcome.ExitOK
		for _, r := range refusals {
			fmt.Fprintln(os.Stderr, r.Error())
			if c := r.Class.ExitCode(); c > exit {
				exit = c
			}
		}
		os.Exit(exit)
	case "absorb":
		if len(os.Args) != 4 || os.Args[3] == "" {
			fmt.Fprintln(os.Stderr, "usage: looplaw absorb <scope-dir> <subject>")
			os.Exit(outcome.ExitUsage)
		}
		m, err := absorb.ScanScope(os.Args[2], os.Args[3])
		if err != nil {
			fmt.Fprintln(os.Stderr, (&outcome.Refusal{
				Class: outcome.Abort, Check: "absorb/scope",
				Subject: os.Args[2], Reason: err.Error(),
				Remedy: "point the absorber at a readable scope directory",
			}).Error())
			os.Exit(outcome.ExitAbort)
		}
		// The skeleton carries machine-computed provenance and empty
		// statement regions: authoring them is inference, and it belongs
		// to the agent driving this tool, never to the binary.
		fmt.Print(absorb.Skeleton(os.Args[3], m))
		os.Exit(outcome.ExitOK)
	case "status":
		if len(os.Args) != 4 {
			fmt.Fprintln(os.Stderr, "usage: looplaw status <view.cue> <scope-dir>")
			os.Exit(outcome.ExitUsage)
		}
		view, refusals := gate.LoadSet(os.Args[2])
		if len(refusals) > 0 {
			exit := outcome.ExitOK
			for _, r := range refusals {
				fmt.Fprintln(os.Stderr, r.Error())
				if c := r.Class.ExitCode(); c > exit {
					exit = c
				}
			}
			os.Exit(exit)
		}
		prov := view.LookupPath(cue.ParsePath("provenance"))
		if !prov.Exists() {
			fmt.Fprintln(os.Stderr, (&outcome.Refusal{
				Class: outcome.Rejection, Check: "status/no-provenance",
				Subject: os.Args[2],
				Reason:  "the set carries no provenance, so it is an authored set, not an absorbed view",
				Remedy:  "run status against an absorbed view; an authored set carries no baseline to go stale against",
			}).Error())
			os.Exit(outcome.ExitRejection)
		}
		recordedScope, _ := prov.LookupPath(cue.ParsePath("scope")).String()
		m, err := absorb.ScanScope(os.Args[3], recordedScope)
		if err != nil {
			fmt.Fprintln(os.Stderr, (&outcome.Refusal{
				Class: outcome.Abort, Check: "status/scope",
				Subject: os.Args[3], Reason: err.Error(),
				Remedy: "point status at a readable scope directory",
			}).Error())
			os.Exit(outcome.ExitAbort)
		}
		rep := provenance.Compare(prov, m)
		out, err := json.MarshalIndent(rep, "", "  ")
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(outcome.ExitAbort)
		}
		// Staleness is evidence, never a verdict: a stale view is owed a
		// re-derivation, which is the client's work — so a stale report
		// is a successful run.
		fmt.Println(string(out))
		os.Exit(outcome.ExitOK)
	case "project":
		if len(os.Args) != 3 || os.Args[2] != "law" {
			fmt.Fprintln(os.Stderr, "usage: looplaw project law")
			os.Exit(outcome.ExitUsage)
		}
		digest, err := project.LawDigest()
		if err != nil {
			fmt.Fprintln(os.Stderr, (&outcome.Refusal{
				Class: outcome.Abort, Check: "project/law",
				Subject: "law (embedded)", Reason: err.Error(),
				Remedy: "the embedded law is broken; replace this binary with one embedding the ratified law",
			}).Error())
			os.Exit(outcome.ExitAbort)
		}
		fmt.Print(digest)
		os.Exit(outcome.ExitOK)
	case "diff":
		if len(os.Args) != 4 {
			fmt.Fprintln(os.Stderr, "usage: looplaw diff <goal.cue> <view.cue>")
			os.Exit(outcome.ExitUsage)
		}
		gaps, refusals := diff.Diff(os.Args[2], os.Args[3])
		if len(refusals) > 0 {
			exit := outcome.ExitOK
			for _, r := range refusals {
				fmt.Fprintln(os.Stderr, r.Error())
				if c := r.Class.ExitCode(); c > exit {
					exit = c
				}
			}
			os.Exit(exit)
		}
		// A gap is a planning state, never an error state: a diff that
		// finds gaps is a successful run. Output is the planning feed.
		out, err := json.MarshalIndent(gaps, "", "  ")
		if err != nil {
			fmt.Fprintln(os.Stderr, (&outcome.Refusal{
				Class: outcome.Abort, Check: "diff/output",
				Subject: "gap list", Reason: err.Error(),
				Remedy: "this binary is broken — do not consume its output",
			}).Error())
			os.Exit(outcome.ExitAbort)
		}
		if gaps == nil {
			out = []byte("[]")
		}
		fmt.Println(string(out))
		os.Exit(outcome.ExitOK)
	default:
		fmt.Fprintf(os.Stderr, "looplaw: unknown command %q\n", os.Args[1])
		usage()
		os.Exit(outcome.ExitUsage)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `usage: looplaw <command>

commands:
  version              print the looplaw version
  validate <set.cue>   run the trinity gates over a target set; refusals
                       carry their remedy; exit codes follow the failure
                       doctrine (0 ok, 1 rejection, 3 abort)
  absorb <scope> <subj> scan a scope and print a draft view skeleton
                       with machine-computed provenance; the statement
                       regions are left for authoring (that is inference)
  project law          print the ratified law as a pasteable brief:
                       invariants, authorities, acts, term cards, and
                       the vocabulary that is refused
  status <view> <scope> report which sources moved under an absorbed
                       view and which statements they were derived from
  diff <goal> <view>   compute the gaps between goal-law and a view
                       (both must pass the gates); the gap list is the
                       planning feed, printed as JSON — a diff that
                       finds gaps is a successful run, exit 0

The rest of the kernel surface (serve, submit, diff, project, verify,
status, export) arrives as it is designed; see proj/looplaw-spec.md §10.
`)
}
