// Package store is the recording authority's bones: an append-only,
// hash-chained record ledger in SQLite.
//
// Per the ratified registry (law/registry.cue): records — claims,
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
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// Kind is the ratified record-side marker.
type Kind string

const (
	Law      Kind = "law"
	Evidence Kind = "evidence"
)

// Record is one appended fact. At is stamped by the store in UTC
// RFC3339Nano. Prev is the predecessor's hash ("" for the first record).
type Record struct {
	Seq     int64  `json:"seq"`
	Kind    Kind   `json:"kind"` // the ratified law-side/evidence-side marker
	Type    string `json:"type"` // the record kind: claim, receipt, admission, version
	Subject string `json:"subject"`
	Body    string `json:"body"`
	Actor   string `json:"actor"`
	At      string `json:"at"`
	Prev    string `json:"prev"`
	Hash    string `json:"hash"`
}

// Store is an open ledger. Not safe to share a single *Store across
// processes; SQLite serializes at the file level.
type Store struct {
	db *sql.DB
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

// ProjectPath is where a project's ledger lives under a state root.
func ProjectPath(root, project string) string {
	return filepath.Join(root, "projects", project)
}

// InitProject explicitly creates a project's state dir and ledger. State
// is never created implicitly: submit/diff against a missing key refuse
// rather than minting a fork (the app-shape ruling).
func InitProject(root, project string) (*Store, error) {
	if !projectKeyRE.MatchString(project) {
		return nil, fmt.Errorf("init project: key %q does not match %s", project, projectKeyRE)
	}
	dir := ProjectPath(root, project)
	if _, err := os.Stat(dir); err == nil {
		return nil, fmt.Errorf("init project: %q already exists under %s", project, root)
	}
	return Open(dir)
}

// OpenProject opens an existing project's ledger and refuses a missing
// one, naming the keys that do exist so a renamed or mistyped key is
// loud, never a silent fork.
func OpenProject(root, project string) (*Store, error) {
	dir := ProjectPath(root, project)
	if _, err := os.Stat(dir); err != nil {
		existing, _ := ListProjects(root)
		return nil, fmt.Errorf("open project: no state for %q under %s (existing: %v) — projects are created only by the explicit init act", project, root, existing)
	}
	return Open(dir)
}

// ListProjects names the project keys a state root holds.
func ListProjects(root string) ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(root, "projects"))
	if err != nil {
		return nil, nil
	}
	var keys []string
	for _, e := range entries {
		if e.IsDir() {
			keys = append(keys, e.Name())
		}
	}
	return keys, nil
}

const schema = `
CREATE TABLE IF NOT EXISTS records (
	seq     INTEGER PRIMARY KEY AUTOINCREMENT,
	kind    TEXT NOT NULL CHECK (kind IN ('law', 'evidence')),
	rectype TEXT NOT NULL,
	subject TEXT NOT NULL,
	body    TEXT NOT NULL,
	actor   TEXT NOT NULL,
	at      TEXT NOT NULL,
	prev    TEXT NOT NULL,
	hash    TEXT NOT NULL UNIQUE
);`

// Open opens (creating if needed) the ledger under root.
func Open(root string) (*Store, error) {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, fmt.Errorf("open store: %w", err)
	}
	db, err := sql.Open("sqlite", filepath.Join(root, "looplaw.db"))
	if err != nil {
		return nil, fmt.Errorf("open store: %w", err)
	}
	// One connection: every transaction serializes at the Go level, so
	// concurrent recorders queue rather than fork the chain or trip
	// SQLITE_BUSY. The busy timeout covers cross-process contention on
	// the same file.
	db.SetMaxOpenConns(1)
	for _, pragma := range []string{
		"PRAGMA journal_mode = WAL;",
		"PRAGMA foreign_keys = ON;",
		"PRAGMA busy_timeout = 5000;",
	} {
		if _, err := db.Exec(pragma); err != nil {
			db.Close()
			return nil, fmt.Errorf("open store: %s: %w", pragma, err)
		}
	}
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("open store: migrate: %w", err)
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }

// canonical is the hashed wire form: length-delimited fields in fixed
// order, so no field boundary is ambiguous and no serialization library
// defines the format.
func canonical(kind Kind, rectype, subject, body, actor, at, prev string) string {
	var out strings.Builder
	for _, f := range []string{string(kind), rectype, subject, body, actor, at, prev} {
		fmt.Fprintf(&out, "%d:%s|", len(f), f)
	}
	return out.String()
}

func hashOf(kind Kind, rectype, subject, body, actor, at, prev string) string {
	sum := sha256.Sum256([]byte(canonical(kind, rectype, subject, body, actor, at, prev)))
	return hex.EncodeToString(sum[:])
}

// Draft is a fact awaiting the record act: what a submitter offers,
// before the store has recorded anything.
type Draft struct {
	Kind    Kind
	Type    string
	Subject string
	Body    string
	Actor   string
}

// ContentHash is the digest of a draft's content, independent of where
// it lands in the chain: an admission cites it so the entry event names
// exactly what entered.
func ContentHash(d Draft) string {
	sum := sha256.Sum256([]byte(canonical(d.Kind, d.Type, d.Subject, d.Body, d.Actor, "", "")))
	return hex.EncodeToString(sum[:])
}

// Append records one fact, chained to the current tail.
func (s *Store) Append(kind Kind, rectype, subject, body, actor string) (Record, error) {
	recs, err := s.AppendAll([]Draft{{Kind: kind, Type: rectype, Subject: subject, Body: body, Actor: actor}})
	if err != nil {
		return Record{}, err
	}
	return recs[0], nil
}

// AppendAll records several facts as one act: every record lands or
// none does. A claim whose admission failed to land would be a record
// with no entry provenance — a silent transition of exactly the kind
// T0-6 forbids — so the pair commits together. The read of the chain
// tail and the inserts share one transaction on the store's single
// connection, so concurrent recorders serialize instead of forking.
func (s *Store) AppendAll(drafts []Draft) ([]Record, error) {
	if len(drafts) == 0 {
		return nil, fmt.Errorf("append: nothing to record")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("append: %w", err)
	}
	defer tx.Rollback()

	var prev string
	err = tx.QueryRow("SELECT hash FROM records ORDER BY seq DESC LIMIT 1").Scan(&prev)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("append: read tail: %w", err)
	}

	// One timestamp for the act: records committed together are stamped
	// together, so the ledger shows one act rather than a race.
	at := time.Now().UTC().Format(time.RFC3339Nano)

	out := make([]Record, 0, len(drafts))
	for _, d := range drafts {
		rec := Record{
			Kind:    d.Kind,
			Type:    d.Type,
			Subject: d.Subject,
			Body:    d.Body,
			Actor:   d.Actor,
			At:      at,
			Prev:    prev,
		}
		rec.Hash = hashOf(rec.Kind, rec.Type, rec.Subject, rec.Body, rec.Actor, rec.At, rec.Prev)

		res, err := tx.Exec(
			"INSERT INTO records (kind, rectype, subject, body, actor, at, prev, hash) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
			string(rec.Kind), rec.Type, rec.Subject, rec.Body, rec.Actor, rec.At, rec.Prev, rec.Hash,
		)
		if err != nil {
			return nil, fmt.Errorf("append: %w", err)
		}
		if rec.Seq, err = res.LastInsertId(); err != nil {
			return nil, fmt.Errorf("append: %w", err)
		}
		prev = rec.Hash
		out = append(out, rec)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("append: %w", err)
	}
	return out, nil
}

// Tamper rewrites a record's body in place. It exists for tests that
// must prove the chain notices: nothing in the product mutates a
// recorded fact, and the append-only ledger has no other writer.
func (s *Store) Tamper(seq int64, body string) error {
	_, err := s.db.Exec("UPDATE records SET body = ? WHERE seq = ?", body, seq)
	return err
}

// Records returns the full ledger in sequence order.
func (s *Store) Records() ([]Record, error) {
	rows, err := s.db.Query("SELECT seq, kind, rectype, subject, body, actor, at, prev, hash FROM records ORDER BY seq")
	if err != nil {
		return nil, fmt.Errorf("records: %w", err)
	}
	defer rows.Close()
	var out []Record
	for rows.Next() {
		var r Record
		var kind string
		if err := rows.Scan(&r.Seq, &kind, &r.Type, &r.Subject, &r.Body, &r.Actor, &r.At, &r.Prev, &r.Hash); err != nil {
			return nil, fmt.Errorf("records: %w", err)
		}
		r.Kind = Kind(kind)
		out = append(out, r)
	}
	return out, rows.Err()
}

// Verify recomputes every hash and link. It returns the number of records
// verified; an error names the first break. Verification here means one
// thing only: the record being read is the record that was written.
func (s *Store) Verify() (int, error) {
	recs, err := s.Records()
	if err != nil {
		return 0, err
	}
	prev := ""
	for _, r := range recs {
		if r.Prev != prev {
			return 0, fmt.Errorf("verify: seq %d: chain break: prev %q, want %q", r.Seq, r.Prev, prev)
		}
		want := hashOf(r.Kind, r.Type, r.Subject, r.Body, r.Actor, r.At, r.Prev)
		if r.Hash != want {
			return 0, fmt.Errorf("verify: seq %d: content does not re-hash to what was recorded", r.Seq)
		}
		prev = r.Hash
	}
	return len(recs), nil
}
