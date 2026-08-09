// Command looplaw is the law organ of the loopstack: it stores contract
// sets, gates what may enter them, and reports the gap between ratified
// goal-law and absorbed evidence. Design basis: proj/looplaw-spec.md.
package main

import (
	"fmt"
	"os"
)

const version = "0.0.0-dev"

// Exit codes follow the failure doctrine (rejection/denial are not the same
// thing and neither is an infrastructure abort); the full enumeration lands
// with the first real gate.
const (
	exitOK    = 0
	exitUsage = 64
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(exitUsage)
	}
	switch os.Args[1] {
	case "version":
		fmt.Println("looplaw " + version)
		os.Exit(exitOK)
	default:
		fmt.Fprintf(os.Stderr, "looplaw: unknown command %q\n", os.Args[1])
		usage()
		os.Exit(exitUsage)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `usage: looplaw <command>

commands:
  version    print the looplaw version

The kernel surface (serve, validate, admit, diff, context, verify, status)
arrives as it is designed; see proj/looplaw-spec.md §10.
`)
}
