package store

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// The SQLite ledger: the default storage, and the reference for what a
// Ledger must provide. It seals records into a hash chain, commits them
// atomically, and re-checks that chain on demand.
//
// SQLite is not a placeholder here. The all-or-none promise is a
// transaction, ordering is a primary key, and the record kind and hash
// uniqueness are constraints the engine holds rather than Go the caller
// could bypass. The driver is pure Go, so the binary stays static and
// cross-compiles — which matters where looplaw runs inside ephemeral
// containers.
const sqliteSchema = `
CREATE TABLE IF NOT EXISTS records (
	seq     INTEGER PRIMARY KEY,
	kind    TEXT NOT NULL CHECK (kind IN ('law', 'evidence')),
	rectype TEXT NOT NULL,
	subject TEXT NOT NULL,
	body    TEXT NOT NULL,
	party   TEXT NOT NULL,
	at      TEXT NOT NULL,
	prev    TEXT NOT NULL,
	hash    TEXT NOT NULL UNIQUE
);`

type sqliteLedger struct{ db *sql.DB }

// sqliteCatalog addresses projects as directories under a state root,
// one database each.
type sqliteCatalog struct{}

// projectDir is the one place a project selector becomes a place, so it
// is the one place the key grammar has to hold. Every escape this
// catalog admitted came from a caller that resolved without asking:
// Init checked the grammar and Open did not, and Open is what every verb
// but init goes through.
//
// The grammar does the whole job — it admits no empty string, no dot or
// dot-dot, no separator, no absolute path, and nothing filepath.Join
// would normalise into a different place — so this states the rule once
// rather than enumerating escapes, which is a list that is only ever as
// complete as the last attack anyone thought of.
func projectDir(root, project string) (string, error) {
	if !projectKeyRE.MatchString(project) {
		return "", fmt.Errorf("key %q does not match %s", project, projectKeyRE)
	}
	return filepath.Join(root, "projects", project), nil
}

func (sqliteCatalog) Describe(root, project string) string {
	dir, err := projectDir(root, project)
	if err != nil {
		// A reader is owed the reason, and nothing may join onto this.
		return fmt.Sprintf("(no project: %v)", err)
	}
	return dir
}

// Deployment opens the ledger the deployment itself keeps — the
// accountable-authority binding lives here, beside the projects rather
// than inside one.
//
// It is a capability rather than a project key, because a key is a
// string an untrusted caller supplies and this is not something they may
// name. While the empty string selected it, a submit whose project
// argument was empty recorded a party's claim into this ledger, and a
// claim shaped like an authority binding was read back as one.
func (sqliteCatalog) Deployment(root string) (Ledger, error) { return openSQLite(root) }

func (c sqliteCatalog) Init(root, project string) (Ledger, error) {
	dir, err := projectDir(root, project)
	if err != nil {
		return nil, fmt.Errorf("init project: %w", err)
	}
	if _, err := os.Stat(dir); err == nil {
		return nil, fmt.Errorf("init project: %q already exists under %s", project, root)
	}
	return openSQLite(dir)
}

func (c sqliteCatalog) Open(root, project string) (Ledger, error) {
	dir, err := projectDir(root, project)
	if err != nil {
		return nil, fmt.Errorf("open project: %w", err)
	}
	if _, err := os.Stat(dir); err != nil {
		existing, _ := c.List(root)
		return nil, fmt.Errorf("open project: no state for %q under %s (existing: %v) — projects are created only by the explicit init act", project, root, existing)
	}
	return openSQLite(dir)
}

func (sqliteCatalog) List(root string) ([]string, error) {
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

func openSQLite(dir string) (Ledger, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("open store: %w", err)
	}
	// The ledger holds submitted bodies, party names, the authority
	// binding and the whole law and evidence history. It was created at
	// whatever the umask allowed — 0755 and 0644 under the common 022 —
	// so on a host whose parents are traversable, every other local
	// account could read it. A deployment that means to share one says
	// so by configuring it, which is a different thing from inheriting
	// it from a umask nobody chose for this.
	//
	// Narrowed on open, not only on create: MkdirAll leaves an existing
	// directory alone, and a fix covering only new state leaves every
	// ledger already on disk exactly as exposed as it was.
	if err := os.Chmod(dir, 0o700); err != nil {
		return nil, fmt.Errorf("open store: %w", err)
	}
	path := filepath.Join(dir, "looplaw.db")
	// Created here rather than by the driver, so the mode is this
	// deployment's and not the umask's. The sidecars SQLite writes
	// beside it are covered by the directory.
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open store: %w", err)
	}
	f.Close()
	if err := os.Chmod(path, 0o600); err != nil {
		return nil, fmt.Errorf("open store: %w", err)
	}
	db, err := sql.Open("sqlite", path)
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
	if _, err := db.Exec(sqliteSchema); err != nil {
		db.Close()
		return nil, fmt.Errorf("open store: migrate: %w", err)
	}
	return &sqliteLedger{db: db}, nil
}

// Append seals the drafts into the chain and commits them in one
// transaction: all of it lands or none does, and no partial state is
// observable to a later Records. Reading the tail and inserting share
// the transaction, on the single connection, so concurrent recorders
// serialize instead of forking the chain.
//
// This ledger's identity is a hash over the record and its predecessor.
// A different ledger may establish identity differently; nothing above
// the interface depends on how.
func (b *sqliteLedger) Append(drafts []Draft) ([]Record, error) {
	tx, err := b.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("append: %w", err)
	}
	defer tx.Rollback()

	var prev string
	var seq int64
	err = tx.QueryRow("SELECT hash, seq FROM records ORDER BY seq DESC LIMIT 1").Scan(&prev, &seq)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("append: read tail: %w", err)
	}

	// One timestamp for the act: records committed together are stamped
	// together, so the ledger shows one act rather than a race.
	at := Clock().UTC().Format(time.RFC3339Nano)

	out := make([]Record, 0, len(drafts))
	for _, d := range drafts {
		seq++
		rec := Record{
			Seq: seq, Kind: d.Kind, Type: d.Type, Subject: d.Subject,
			Body: d.Body, Party: d.Party, At: at, Prev: prev,
		}
		rec.Hash = hashOf(rec.Kind, rec.Type, rec.Subject, rec.Body, rec.Party, rec.At, rec.Prev)
		if _, err := tx.Exec(
			"INSERT INTO records (seq, kind, rectype, subject, body, party, at, prev, hash) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
			rec.Seq, string(rec.Kind), rec.Type, rec.Subject, rec.Body, rec.Party, rec.At, rec.Prev, rec.Hash,
		); err != nil {
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

func (b *sqliteLedger) Records() ([]Record, error) {
	rows, err := b.db.Query("SELECT seq, kind, rectype, subject, body, party, at, prev, hash FROM records ORDER BY seq")
	if err != nil {
		return nil, fmt.Errorf("read ledger: %w", err)
	}
	defer rows.Close()

	var out []Record
	for rows.Next() {
		var r Record
		var kind string
		if err := rows.Scan(&r.Seq, &kind, &r.Type, &r.Subject, &r.Body, &r.Party, &r.At, &r.Prev, &r.Hash); err != nil {
			return nil, fmt.Errorf("read ledger: %w", err)
		}
		r.Kind = Kind(kind)
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read ledger: %w", err)
	}
	return out, nil
}

// Verify recomputes every hash and link. It returns the number of
// records verified; an error names the first break. Verification here
// means one thing only: the record being read is the record that was
// written.
func (b *sqliteLedger) Verify() (int, error) {
	recs, err := b.Records()
	if err != nil {
		return 0, err
	}
	prev := ""
	for _, r := range recs {
		if r.Prev != prev {
			return 0, fmt.Errorf("verify: seq %d: chain break: prev %q, want %q", r.Seq, r.Prev, prev)
		}
		want := hashOf(r.Kind, r.Type, r.Subject, r.Body, r.Party, r.At, r.Prev)
		if r.Hash != want {
			return 0, fmt.Errorf("verify: seq %d: content does not re-hash to what was recorded", r.Seq)
		}
		prev = r.Hash
	}
	return len(recs), nil
}

func (b *sqliteLedger) Close() error { return b.db.Close() }

// hashOf is this ledger's record identity: the content, its timestamp,
// and its predecessor. Another ledger may establish identity by other
// means; nothing above the Ledger interface depends on how.
func hashOf(kind Kind, rectype, subject, body, party, at, prev string) string {
	sum := sha256.Sum256([]byte(canonical(kind, rectype, subject, body, party, at, prev)))
	return hex.EncodeToString(sum[:])
}
