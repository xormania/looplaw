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
	Seq     int64
	Kind    Kind
	Type    string // record type, e.g. "claim", "receipt", "admission"
	Subject string
	Body    string
	Actor   string
	At      string
	Prev    string
	Hash    string
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

// Append records one fact, chained to the current tail. The read-tail
// and insert share one transaction on the store's single connection, so
// concurrent appenders serialize instead of forking the chain (proven by
// the concurrency behavior test).
func (s *Store) Append(kind Kind, rectype, subject, body, actor string) (Record, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return Record{}, fmt.Errorf("append: %w", err)
	}
	defer tx.Rollback()

	var prev string
	err = tx.QueryRow("SELECT hash FROM records ORDER BY seq DESC LIMIT 1").Scan(&prev)
	if err != nil && err != sql.ErrNoRows {
		return Record{}, fmt.Errorf("append: read tail: %w", err)
	}

	rec := Record{
		Kind:    kind,
		Type:    rectype,
		Subject: subject,
		Body:    body,
		Actor:   actor,
		At:      time.Now().UTC().Format(time.RFC3339Nano),
		Prev:    prev,
	}
	rec.Hash = hashOf(rec.Kind, rec.Type, rec.Subject, rec.Body, rec.Actor, rec.At, rec.Prev)

	res, err := tx.Exec(
		"INSERT INTO records (kind, rectype, subject, body, actor, at, prev, hash) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
		string(rec.Kind), rec.Type, rec.Subject, rec.Body, rec.Actor, rec.At, rec.Prev, rec.Hash,
	)
	if err != nil {
		return Record{}, fmt.Errorf("append: %w", err)
	}
	if rec.Seq, err = res.LastInsertId(); err != nil {
		return Record{}, fmt.Errorf("append: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return Record{}, fmt.Errorf("append: %w", err)
	}
	return rec, nil
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
