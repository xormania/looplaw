package store

import "time"

// Ledger is the recording authority's storage, entire. It records facts
// as acts, returns what it recorded, and can re-check its own integrity.
//
// The seam is here rather than lower on purpose. An earlier draft put
// only byte storage behind an interface and kept chaining and hashing
// above it — which assumes every ledger links records the same way. A
// store that performs the append itself, forms its own chain, and knows
// how to verify it cannot implement that; it would have to be taken
// apart to fit. So the whole act is the interface, and looplaw keeps
// only what is its own law rather than the storage's mechanism.
//
// What a Ledger must promise, whatever it is underneath:
//
//   - Append is one act: every draft is recorded or none is, and no
//     partial state is observable to a later Records. Content and its
//     admission enter together; a record without its admission is an
//     entry nobody can reconstruct, which T0-6 forbids.
//   - Ordering is stable across processes and restarts, and Records
//     returns them in the order Append received them.
//   - Bodies come back unaltered. A ledger that normalises, re-encodes,
//     or trims a body has changed what a party said.
//   - Nothing is edited or removed. The store is append-only; a
//     correction is a further record.
//   - Every returned Record carries an identity in Hash that is stable
//     for the life of the ledger, so one record can cite another.
//   - Verify re-checks that what is read is what was written, and
//     reports a break rather than repairing it. How is the ledger's
//     business: a hash chain, a signature, a remote attestation.
//
// What looplaw keeps above this line is only its own law: which record
// kinds exist, what an act may enter, and who may perform one. None of
// that is storage's business.
type Ledger interface {
	// Append records drafts as one act, returning them as recorded.
	Append(drafts []Draft) ([]Record, error)

	// Records returns everything recorded, in order.
	Records() ([]Record, error)

	// Verify re-checks integrity, returning the number of records
	// checked. An error means the ledger no longer reads back as what
	// was written.
	Verify() (int, error)

	Close() error
}

// Catalog is how projects are addressed: created, opened, listed, and
// described. It sits beside Ledger because addressing is as
// storage-specific as recording — a directory of databases, a namespace
// on a service, a set of streams — and a caller that assumed one shape
// would have to be rewritten for the next.
//
// Nothing here promises a filesystem. "root" is an opaque locator, from
// LOOPLAW_ROOT or a deployment default, and Describe returns text for a
// reader rather than a path anything should join onto.
type Catalog interface {
	// Init creates a project's state, refusing one that already exists.
	// State is never created implicitly: an act against a missing
	// project refuses rather than minting a fork.
	Init(root, project string) (Ledger, error)

	// Open opens existing state, refusing a project that has none.
	Open(root, project string) (Ledger, error)

	// List names the projects a root holds.
	List(root string) ([]string, error)

	// Describe says where a project's state lives, for a reader. It is
	// not an address anything computes with.
	Describe(root, project string) string
}

// Clock is where a ledger reads the time it stamps on an act.
//
// The store stamps time itself and the server's clock is authoritative —
// that is doctrine and does not change here. What changes is that
// *which* clock is explicit rather than compiled in. A clock is an
// ownership boundary like storage or custody: it belongs to the
// deployment, it is the one input no code controls, and leaving it
// implicit makes every derived value untestable.
//
// The cost of leaving it implicit was concrete. Timestamps flow into the
// chain hash, so every recorded output was nondeterministic, so every
// golden masked "at", "hash" and "prev" — which meant no golden pinned
// the chain at all. A change to the canonical form or the hash function
// would have failed nothing. Masking a value to make a test pass hides
// exactly the thing the test was for.
//
// A test sets this to a fixed instant and gets real hashes it can pin.
var Clock func() time.Time = time.Now

// DefaultCatalog is used when a caller does not choose. A variable
// rather than a constant so a deployment — or a test — can substitute
// storage without touching any act.
var DefaultCatalog Catalog = sqliteCatalog{}

// Latest is an optional capability. A ledger that can answer "the most
// recent record of this kind and type" itself should, and a caller
// should ask before scanning.
//
// The Ledger interface is a floor, not a ceiling. Two acts here already
// walk every record backwards to find one — the live law, and what a
// project's ledger has been about — which is a full scan standing in for
// a query the storage may well serve directly. Declaring the capability
// now means a store that offers views is a substitution rather than a
// rewrite of its callers.
//
// A ledger that does not implement it is not deficient: the fallback is
// correct, only slower, and correctness must not depend on which
// storage is underneath.
type Latest interface {
	// Latest returns the most recent record of the given kind and type,
	// or nil when the ledger holds none.
	Latest(kind Kind, rectype string) (*Record, error)
}

// LatestOf answers the query through the ledger when it can, and by
// scanning when it cannot. Callers use this rather than type-asserting,
// so the fallback exists in one place.
func (s *Store) LatestOf(kind Kind, rectype string) (*Record, error) {
	if q, ok := s.l.(Latest); ok {
		return q.Latest(kind, rectype)
	}
	recs, err := s.l.Records()
	if err != nil {
		return nil, err
	}
	for i := len(recs) - 1; i >= 0; i-- {
		if recs[i].Kind == kind && recs[i].Type == rectype {
			return &recs[i], nil
		}
	}
	return nil, nil
}
