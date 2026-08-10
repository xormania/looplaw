# looplaw

A contract-set validator for agent-driven development: the permissible
shapes of a project's law are declared in CUE, and a Go gate refuses
anything the law does not admit.

The idea: when software is built by agent loops, the constraints have to
live somewhere agents cannot argue with. Here they live in `schema/` as CUE —
authorities, invariants, a controlled vocabulary, and the schema for a
project's contract set (its parties, acts, preconditions, guarantees,
blame assignments, and judgment register). The Go side holds no policy of
its own: it embeds the law at build time and enforces it. A violation is
not an error to handle — most violations are unrepresentable in the
schema, and the rest are refused before anything acts on them. Changing
what is permitted is a data change with a reviewable diff, not a code
change.

The law is CUE; Go is the enforcer. From `schema/trinity.cue`, verbatim:

```cue
// An experience entry: the judgment register. Advisory force always —
// never binding, never a contract clause in disguise.
#ExperienceEntry: {
	id:       string & =~"^X-[0-9]+$"
	judgment: string
	// At least one cite: judgment attaches to law, it never floats.
	cites: [string, ...string] // contract or invariant ids
	advisory: true
}
```

`advisory: true` is not a default — it is the only value the type admits.
A judgment that claims binding force cannot be written down. The id must
match its grammar; the cites list cannot be empty. None of this needs a
running check: the constraint is the data.

## How the pieces fit

- **`schema/` declares.** Four CUE files: the authority registry (who may
  perform which act — exactly two authorities exist), nine global
  invariants, a controlled lexicon (reserved verbs with anti-definitions;
  banned vocabulary — which binds the binary's own refusal strings), and
  `#TrinitySet`, the schema a project's contract set must satisfy.
- **`internal/gate` enforces.** The complete law package is embedded in
  the binary, so a build's checks are pinned to a law version. Sixteen
  checks are enumerated as data: shape by CUE unification, and a
  relational lane (act closure, party and invariant coverage, reference
  resolution, blame resolution) for what the lattice cannot state. The
  test suite fails if any check lacks a fixture proving it fires — an
  undemonstrated gate is treated as an unproven behavior.
- **`internal/outcome` classifies.** Every non-passing result is one of
  four classes — rejection, denial, abort, finding — with distinct exit
  codes, and every refusal carries a remedy in a fixed, tested grammar:
  `<check>: <class> <subject>: <reason> — remedy: <remedy>`.
- **`internal/store` records.** An append-only, hash-chained ledger
  (SQLite) for law-side and evidence-side records; verification
  recomputes every hash and link. It exists and is tested, including
  under concurrent writers, but is not yet wired to the CLI.
- **CI runs two independent producers** over the same law and the same
  example set — the embedded gate and the stock `cue` binary — and fails
  if they disagree.

## Quickstart

```console
$ git clone https://github.com/xormania/looplaw && cd looplaw
$ go run ./cmd/looplaw validate internal/gate/testdata/library/set.cue
ok

$ sed 's/at_fault: "librarian"/at_fault: "ghost"/' \
    internal/gate/testdata/library/set.cue > /tmp/broken.cue
$ go run ./cmd/looplaw validate /tmp/broken.cue
trinity/blame-resolve: rejection /tmp/broken.cue: C-LEND-1 blame: at_fault names "ghost", not a registered party — remedy: blame attaches to a registered party, adjudicated from recorded evidence
$ echo $?
1
```

The example set is a two-party lending library — small enough to read in
one sitting, complete enough to exercise every check. Its test suite
derives twenty-one red cases from the green fixture by single edits and
asserts each is refused for its declared reason, naming the edited thing.

## Status

Early. What exists today: the law files, the validator (`looplaw
validate`), the outcome taxonomy, the ledger, and CI. What does not exist
yet: a server, store-backed workflows, or any loop integration — the gate
is a command, not a daemon. The design intent is that an agent loop can
only occupy states its law admits; today that holds for the one path
implemented: a contract set either satisfies the law or is refused with a
named remedy. Amendments to `schema/` are ordinary reviewed diffs, followed
by a rebuild, since the law ships inside the binary.
