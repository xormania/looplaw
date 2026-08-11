// Package store is the recording authority's bones: an append-only,
// hash-chained record ledger in SQLite.
//
// Per the ratified registry (dev/registry.cue): records — claims,
// receipts, admissions, versions — commit here; recording settles that a
// thing was said, never that it is true. Every record carries the ratified
// law-side/evidence-side kind marker. The chain makes the ledger
// tamper-evident: each record hashes its content plus its predecessor's
// hash, and Verify recomputes the whole chain.
//
// The store stamps time itself (server clock authoritative, the loopflow
// rule) and serializes appends, so concurrent recorders queue rather than
// fork the chain.
package store

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	_ "modernc.org/sqlite"
)

// Kind is the ratified record-side marker.
type Kind string

const (
	Law      Kind = "law"
	Evidence Kind = "evidence"
)

// RecordKinds are the record kinds the ratified lexicon names, and the
// only types this ledger accepts. The list is closed on purpose: "for
// other stored data write 'data (non-authoritative)' or the mechanism's
// own noun". A new kind is a ratification, not a spelling choice, and
// coining one in a Go string literal is how two unratified types
// ("goal-proposal", "law-set") reached this store — past a conformance
// suite that checks for banned words, workshop words and initialisms,
// none of which a fresh coinage is.
//
// internal/conformance ties this list to the lexicon, so it cannot
// drift into naming something the product has not reserved.
var RecordKinds = []string{"claim", "receipt", "admission", "version"}

func knownKind(t string) bool {
	for _, k := range RecordKinds {
		if k == t {
			return true
		}
	}
	return false
}

// Record is one appended fact. At is stamped by the store in UTC
// RFC3339Nano. Prev is the predecessor's hash ("" for the first record).
type Record struct {
	Seq     int64  `json:"seq"`
	Kind    Kind   `json:"kind"` // the ratified law-side/evidence-side marker
	Type    string `json:"type"` // the record kind: claim, receipt, admission, version
	Subject string `json:"subject"`
	Body    string `json:"body"`
	Party   string `json:"party"`
	At      string `json:"at"`
	Prev    string `json:"prev"`
	Hash    string `json:"hash"`
}

// Store is looplaw's law about records, over whatever Ledger holds
// them. It keeps only what is its own — which record kinds exist — and
// delegates the act itself.
type Store struct{ l Ledger }

// New wraps a ledger. Use it to record on storage other than the
// default; every act above this line is unchanged by the choice.
func New(l Ledger) *Store { return &Store{l: l} }

// OpenDeployment opens (creating if needed) the ledger the deployment
// keeps for itself, using the default backend. It is what records the
// accountable-authority binding, which is one per deployment.
//
// Named as a capability, not reachable as a project key: see
// Catalog.Deployment.
func OpenDeployment(root string) (*Store, error) {
	l, err := DefaultCatalog.Deployment(root)
	if err != nil {
		return nil, err
	}
	return New(l), nil
}

// DefaultRoot resolves the state root: $LOOPLAW_ROOT, else
// $XDG_STATE_HOME/looplaw, else ~/.local/state/looplaw.
func DefaultRoot() (string, error) {
	if root := os.Getenv("LOOPLAW_ROOT"); root != "" {
		return root, nil
	}
	if xdg := os.Getenv("XDG_STATE_HOME"); xdg != "" {
		return filepath.Join(xdg, "looplaw"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve state root: %w", err)
	}
	return filepath.Join(home, ".local", "state", "looplaw"), nil
}

// Project keys share the subject grammar, so they are filesystem-safe
// by construction.
var projectKeyRE = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// ProjectPath says where a project's state lives, for a reader. It is
// not a filesystem path a caller should build on: a service-backed
// deployment answers with something else entirely.
func ProjectPath(root, project string) string { return DefaultCatalog.Describe(root, project) }

// InitProject explicitly creates a project's state. State is never
// created implicitly: an act against a missing key refuses rather than
// minting a fork (the app-shape ruling).
func InitProject(root, project string) (*Store, error) {
	l, err := DefaultCatalog.Init(root, project)
	if err != nil {
		return nil, err
	}
	return New(l), nil
}

// OpenProject opens an existing project's state and refuses a missing
// one, naming the keys that do exist so a renamed or mistyped key is
// loud, never a silent fork.
func OpenProject(root, project string) (*Store, error) {
	l, err := DefaultCatalog.Open(root, project)
	if err != nil {
		return nil, err
	}
	return New(l), nil
}

// ListProjects names the project keys a state root holds.
func ListProjects(root string) ([]string, error) { return DefaultCatalog.List(root) }

func (s *Store) Close() error { return s.l.Close() }

// canonical is the hashed wire form: length-delimited fields in fixed
// order, so no field boundary is ambiguous and no serialization library
// defines the format.
func canonical(kind Kind, rectype, subject, body, party, at, prev string) string {
	var out strings.Builder
	for _, f := range []string{string(kind), rectype, subject, body, party, at, prev} {
		fmt.Fprintf(&out, "%d:%s|", len(f), f)
	}
	return out.String()
}

// Draft is a fact awaiting the record act: what a submitter offers,
// before the store has recorded anything.
type Draft struct {
	Kind    Kind
	Type    string
	Subject string
	Body    string
	Party   string
}

// ContentHash is the digest of a draft's content, independent of where
// it lands in the chain: an admission cites it so the entry event names
// exactly what entered.
func ContentHash(d Draft) string {
	sum := sha256.Sum256([]byte(canonical(d.Kind, d.Type, d.Subject, d.Body, d.Party, "", "")))
	return hex.EncodeToString(sum[:])
}

// AppendAll records several facts as one act.
//
// The only thing checked here is looplaw's own law: a record kind the
// lexicon does not name is refused before the ledger sees it. Sealing —
// ordering, linking, timestamping, hashing — belongs to the ledger,
// which is why a store that performs those itself can be substituted
// without touching an act.
func (s *Store) AppendAll(drafts []Draft) ([]Record, error) {
	if len(drafts) == 0 {
		return nil, fmt.Errorf("append: nothing to record")
	}
	for _, d := range drafts {
		if !knownKind(d.Type) {
			return nil, fmt.Errorf("append: %q is not a record kind; the ledger holds %v", d.Type, RecordKinds)
		}
	}
	return s.l.Append(drafts)
}

// Records returns the full ledger in sequence order.
func (s *Store) Records() ([]Record, error) { return s.l.Records() }

// Append records one fact, chained to the current tail.
func (s *Store) Append(kind Kind, rectype, subject, body, party string) (Record, error) {
	recs, err := s.AppendAll([]Draft{{Kind: kind, Type: rectype, Subject: subject, Body: body, Party: party}})
	if err != nil {
		return Record{}, err
	}
	return recs[0], nil
}

// Verify re-checks that what is read is what was written. How that is
// established — a hash chain, a signature, a remote attestation — is the
// ledger's business; a break is reported, never repaired.
func (s *Store) Verify() (int, error) { return s.l.Verify() }
