// Command looplaw is the law organ of the loopstack: it stores contract
// sets, gates what may enter them, and reports the gap between ratified
// goal-law and absorbed evidence. Design basis: proj/looplaw-spec.md.
package main

import (
	"fmt"
	"os"

	"github.com/xormania/looplaw/internal/gate"
	"github.com/xormania/looplaw/internal/outcome"
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
		refusals := gate.ValidateTrinity(os.Args[2])
		if len(refusals) == 0 {
			fmt.Println("ok")
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

The rest of the kernel surface (serve, submit, diff, project, verify,
status, export) arrives as it is designed; see proj/looplaw-spec.md §10.
`)
}
