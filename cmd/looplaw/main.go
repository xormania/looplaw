// Command looplaw is the law organ of the loopstack: it stores contract
// sets, gates what may enter them, and reports the gap between ratified
// goal-law and absorbed evidence. Design basis: proj/looplaw-spec.md.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"cuelang.org/go/cue"

	"github.com/xormania/looplaw/internal/absorb"
	"github.com/xormania/looplaw/internal/diff"
	"github.com/xormania/looplaw/internal/gate"
	"github.com/xormania/looplaw/internal/outbound"
	"github.com/xormania/looplaw/internal/outcome"
	"github.com/xormania/looplaw/internal/provenance"
	"github.com/xormania/looplaw/internal/record"
	"github.com/xormania/looplaw/internal/store"
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
		// Two inputs, one act. A scope is walked and needs a subject
		// named for it; a component manifest states its own subject, so
		// it needs no second argument — the arity says which was meant,
		// and neither mistake is silent: a directory read as a manifest
		// and a file walked as a scope both refuse loudly.
		// A directory here is the scope form with its subject forgotten,
		// not a manifest: reading it would abort on "is a directory"
		// where the caller needs to be told what they left out.
		if len(os.Args) == 3 && os.Args[2] != "" && !isDir(os.Args[2]) {
			m, refusals := absorb.LoadComponents(os.Args[2])
			if len(refusals) > 0 {
				refuse(refusals...)
			}
			// The skeleton states what a tool established and leaves the
			// statement regions empty: authoring them is inference, and
			// it belongs to the caller driving this tool, never to the
			// binary. The gates' refusals over it are the worklist.
			fmt.Print(absorb.ComponentSkeleton(m))
			os.Exit(outcome.ExitOK)
		}
		if len(os.Args) != 4 || os.Args[3] == "" {
			fmt.Fprintln(os.Stderr, "usage: looplaw absorb <scope-dir> <subject>")
			fmt.Fprintln(os.Stderr, "       looplaw absorb <components.cue>")
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
		// to the caller driving this tool, never to the binary.
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
			// Through refuse, like every other failure: a bare error on
			// stderr carries no check, class, subject or remedy, so a
			// caller parsing the refusal stream gets a line that does not
			// conform to the grammar the rest of them keep.
			refuse(outcome.Refusal{
				Class: outcome.Abort, Check: "status/output",
				Subject: "staleness report", Reason: err.Error(),
				Remedy: "this binary is broken — do not consume its output",
			})
		}
		// Staleness is evidence, never a verdict: a stale view is owed a
		// re-derivation, which is the client's work — so a stale report
		// is a successful run.
		fmt.Println(string(out))
		os.Exit(outcome.ExitOK)
	case "submit":
		if len(os.Args) != 6 {
			fmt.Fprintln(os.Stderr, "usage: looplaw submit <project> claim|receipt <subject> <body-file|->")
			os.Exit(outcome.ExitUsage)
		}
		body, err := readBody(os.Args[5])
		if err != nil {
			refuse(outcome.Refusal{
				Class: outcome.Abort, Check: "submit/read",
				Subject: os.Args[5], Reason: err.Error(),
				Remedy: "point the submission at a readable file, or - for standard input",
			})
		}
		s := openProject(os.Args[2])
		defer s.Close()
		recs, refusals := record.Submit(s, gate.Submission{
			Kind: os.Args[3], Subject: os.Args[4], Party: party(), Body: body,
		})
		if len(refusals) > 0 {
			refuse(refusals...)
		}
		// Recording settles that a thing was said, never that it is
		// true: the receipt printed here is the entry, not agreement.
		for _, r := range recs {
			fmt.Printf("%s seq %d %s\n", r.Type, r.Seq, r.Hash)
		}
		os.Exit(outcome.ExitOK)
	case "declare":
		if len(os.Args) != 4 {
			fmt.Fprintln(os.Stderr, "usage: looplaw declare <project> <proposed.cue>")
			os.Exit(outcome.ExitUsage)
		}
		st := openProject(os.Args[2])
		defer st.Close()
		recs, refusals := record.Declare(st, os.Args[3], party())
		if len(refusals) > 0 {
			refuse(refusals...)
		}
		// A declaration is a party saying what the law should become. It
		// is recorded so it can be adjudicated, and it binds nothing
		// until the accountable authority ratifies it.
		for _, r := range recs {
			fmt.Printf("%s seq %d %s\n", r.Type, r.Seq, r.Hash)
		}
		// Stated from a read, never from a discarded error: reporting
		// "no law is ratified" because the ledger could not be read
		// would be a claim about standing derived from a failure.
		law, err := record.CurrentLaw(st)
		switch {
		case err != nil:
			fmt.Fprintf(os.Stderr, "recorded; the law state could not be read back: %v\n", err)
		case law == nil:
			fmt.Println("no law is ratified for this project: recorded as a first declaration, binding nothing")
		default:
			fmt.Printf("declared against law %s; binds nothing until ratified\n", law.Hash[:12])
		}
		os.Exit(outcome.ExitOK)
	case "authority":
		if len(os.Args) != 3 {
			fmt.Fprintln(os.Stderr, "usage: looplaw authority <party>")
			os.Exit(outcome.ExitUsage)
		}
		d := openDeployment()
		defer d.Close()
		recs, refusal := record.BindAuthority(d, party(), os.Args[2])
		if refusal != nil {
			refuse(*refusal)
		}
		for _, r := range recs {
			fmt.Printf("%s seq %d %s\n", r.Type, r.Seq, r.Hash)
		}
		// Recorded as claimed. Nothing can confer standing on this
		// binding — the party whose act would confer it is the one being
		// named — so what the ledger offers is that it cannot change
		// quietly, not that it is true.
		fmt.Printf("accountable authority recorded as %q (claimed)\n", os.Args[2])
		os.Exit(outcome.ExitOK)
	case "ratify":
		if len(os.Args) != 4 {
			fmt.Fprintln(os.Stderr, "usage: looplaw ratify <project> <subject>")
			os.Exit(outcome.ExitUsage)
		}
		d := openDeployment()
		defer d.Close()
		p := openProject(os.Args[2])
		defer p.Close()
		recs, refusals := record.Ratify(d, p, os.Args[3], party())
		if len(refusals) > 0 {
			refuse(refusals...)
		}
		for _, r := range recs {
			fmt.Printf("%s seq %d %s\n", r.Type, r.Seq, r.Hash)
		}
		fmt.Printf("%s is law from this act onward; it cures nothing before it\n", os.Args[3])
		os.Exit(outcome.ExitOK)
	case "verify":
		if len(os.Args) != 3 {
			fmt.Fprintln(os.Stderr, "usage: looplaw verify <project>")
			os.Exit(outcome.ExitUsage)
		}
		s := openProject(os.Args[2])
		defer s.Close()
		n, refusal := record.Verify(s)
		if refusal != nil {
			refuse(*refusal)
		}
		fmt.Printf("%s: %d records, chain verified\n", os.Args[2], n)
		os.Exit(outcome.ExitOK)
	case "export":
		if len(os.Args) != 3 {
			fmt.Fprintln(os.Stderr, "usage: looplaw export <project>")
			os.Exit(outcome.ExitUsage)
		}
		s := openProject(os.Args[2])
		defer s.Close()
		out, refusal := record.Export(s)
		if refusal != nil {
			refuse(*refusal)
		}
		// Held content leaves through one gate, so the place a custody
		// system attaches is a variable rather than a search.
		out, refusal = outbound.Release(outbound.Request{
			Party: party(), Purpose: "export", Subject: os.Args[2], Content: out,
		})
		if refusal != nil {
			refuse(*refusal)
		}
		fmt.Print(out)
		os.Exit(outcome.ExitOK)
	case "init":
		if len(os.Args) != 3 {
			fmt.Fprintln(os.Stderr, "usage: looplaw init <project>")
			os.Exit(outcome.ExitUsage)
		}
		root, err := store.DefaultRoot()
		if err != nil {
			refuse(outcome.Refusal{
				Class: outcome.Abort, Check: "init/root", Subject: "state root",
				Reason: err.Error(), Remedy: "set LOOPLAW_ROOT to a writable directory",
			})
		}
		s, err := store.InitProject(root, os.Args[2])
		if err != nil {
			refuse(outcome.Refusal{
				Class: outcome.Rejection, Check: "init/project", Subject: os.Args[2],
				Reason: err.Error(),
				Remedy: "choose a name matching ^[a-z][a-z0-9-]*$ that no state root entry already holds",
			})
		}
		s.Close()
		fmt.Printf("%s: state created under %s\n", os.Args[2], store.ProjectPath(root, os.Args[2]))
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
		// The gap feed carries detail derived from goal law, so it leaves
		// through the same gate as anything else held here.
		feed, refusal := outbound.Release(outbound.Request{
			Party: party(), Purpose: "diff", Subject: os.Args[2], Content: string(out),
		})
		if refusal != nil {
			refuse(*refusal)
		}
		fmt.Println(feed)
		os.Exit(outcome.ExitOK)
	default:
		fmt.Fprintf(os.Stderr, "looplaw: unknown command %q\n", os.Args[1])
		usage()
		os.Exit(outcome.ExitUsage)
	}
}

// isDir reports whether a path is a directory, treating an unreadable
// path as not one: the act that follows will report why it could not be
// read, which is a better message than this could give.
func isDir(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.IsDir()
}

// openDeployment opens the ledger at the state root itself. The
// accountable authority is one per deployment, so its binding lives
// beside the projects rather than inside one: bound per project, two
// projects could disagree about who may make law.
func openDeployment() *store.Store {
	root, err := store.DefaultRoot()
	if err != nil {
		refuse(outcome.Refusal{
			Class: outcome.Abort, Check: "deployment/root", Subject: "state root",
			Reason: err.Error(), Remedy: "set LOOPLAW_ROOT to a writable location",
		})
	}
	s, err := store.OpenDeployment(root)
	if err != nil {
		refuse(outcome.Refusal{
			Class: outcome.Abort, Check: "deployment/open", Subject: root,
			Reason: err.Error(), Remedy: "the deployment ledger could not be opened",
		})
	}
	return s
}

// refuse prints refusals to stderr and exits with the gravest class's
// code: refusals are protocol, so the stream and the code are what a
// caller branches on.
func refuse(rs ...outcome.Refusal) {
	exit := outcome.ExitOK
	for _, r := range rs {
		fmt.Fprintln(os.Stderr, r.Error())
		if c := r.Class.ExitCode(); c > exit {
			exit = c
		}
	}
	os.Exit(exit)
}

// openProject opens an existing project's state. A name no init act has
// made refuses and names what does exist, so a mistyped or renamed key
// is loud rather than a fresh, empty fork. A name that is not a project
// key at all refuses in the same place: the argument is whatever the
// caller typed, and the deployment's own ledger is not something it may
// name.
func openProject(name string) *store.Store {
	root, err := store.DefaultRoot()
	if err != nil {
		refuse(outcome.Refusal{
			Class: outcome.Abort, Check: "project/root", Subject: "state root",
			Reason: err.Error(), Remedy: "set LOOPLAW_ROOT to a writable directory",
		})
	}
	s, err := store.OpenProject(root, name)
	if err != nil {
		refuse(outcome.Refusal{
			Class: outcome.Rejection, Check: "project/state", Subject: name,
			Reason: err.Error(),
			Remedy: "name a project matching ^[a-z][a-z0-9-]*$ that looplaw init <project> has made; state is never created implicitly",
		})
	}
	return s
}

// party names the submitting party. It is supplied, never inferred: a
// record settles that a party said a thing, and a party the tool
// guessed is not one that said anything. What is recorded is the
// party as claimed — looplaw checks no identity and asserts none.
func party() string {
	if a := os.Getenv("LOOPLAW_PARTY"); a != "" {
		return a
	}
	return ""
}

func readBody(path string) (string, error) {
	if path == "-" {
		b, err := io.ReadAll(os.Stdin)
		return string(b), err
	}
	b, err := os.ReadFile(path)
	return string(b), err
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
  absorb <components.cue>
                       the same act over a submitted component manifest:
                       components become parties, each dependency becomes
                       a contract, and provenance cites the sources the
                       client hashed. Deriving the manifest is
                       language-specific and belongs to the caller
  init <project>       create a project's state; state is never created
                       implicitly, so every later verb refuses a name
                       this act has not made
  submit <project> <kind> <subject> <body>
                       record a claim or a receipt, with the admission
                       of its entry; recording settles that a thing was
                       said, never that it is true
  declare <project> <proposed.cue>
                       record a proposed goal set as a claim; the gates
                       check it and their refusals are the worklist. A
                       declaration binds nothing until it is ratified
  authority <party>    record which party holds this deployment's
                       accountable authority; recorded as claimed, and
                       the first binding holds
  ratify <project> <subject>
                       the accountable authority's act: a declared draft
                       becomes law, from the act onward
  verify <project>     recompute every hash and link in the ledger
  export <project>     print the ledger as recorded
  status <view> <scope> report which sources moved under an absorbed
                       view and which statements they were derived from
  diff <goal> <view>   compute the gaps between goal-law and a view
                       (both must pass the gates); the gap list is the
                       planning feed, printed as JSON — a diff that
                       finds gaps is a successful run, exit 0

The rest of the kernel surface (serve) arrives as it is
designed; see proj/looplaw-spec.md §10.
`)
}
