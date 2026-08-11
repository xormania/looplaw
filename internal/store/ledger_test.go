package store

import (
	"fmt"
	"testing"
	"time"
)

// memLedger is a second implementation, and its only job is to prove the
// seam is real. It holds records in a slice, seals them by position
// rather than by hash chain, and verifies by re-reading — deliberately
// unlike the SQLite ledger, so anything above that quietly assumed a
// chain, a table, or a file fails here.
type memLedger struct {
	recs   []Record
	closed bool
}

func (m *memLedger) Append(drafts []Draft) ([]Record, error) {
	if m.closed {
		return nil, fmt.Errorf("append: ledger closed")
	}
	staged := make([]Record, 0, len(drafts))
	for i, d := range drafts {
		seq := int64(len(m.recs) + i + 1)
		staged = append(staged, Record{
			Seq: seq, Kind: d.Kind, Type: d.Type, Subject: d.Subject,
			Body: d.Body, Party: d.Party, At: "2026-01-01T00:00:00Z",
			// No chain: identity is positional here, which is a
			// different scheme entirely.
			Prev: "", Hash: fmt.Sprintf("mem-%d", seq),
		})
	}
	// All or none: nothing is visible until the whole batch is staged.
	m.recs = append(m.recs, staged...)
	return staged, nil
}

func (m *memLedger) Records() ([]Record, error) {
	out := make([]Record, len(m.recs))
	copy(out, m.recs)
	return out, nil
}

func (m *memLedger) Verify() (int, error) {
	for i, r := range m.recs {
		if r.Hash != fmt.Sprintf("mem-%d", i+1) {
			return 0, fmt.Errorf("verify: seq %d: identity does not match position", r.Seq)
		}
	}
	return len(m.recs), nil
}

func (m *memLedger) Close() error { m.closed = true; return nil }

// The acts do not know which storage is underneath. Every assertion here
// is about looplaw's law — what may be recorded, what comes back, what
// an act settles — and none about how a record is sealed. A property
// that holds on one ledger and not the other means something above the
// interface reached below it.
func TestLedgerContract(t *testing.T) {
	for _, tc := range []struct {
		name string
		open func(t *testing.T) *Store
	}{
		{"sqlite", func(t *testing.T) *Store { return open(t) }},
		{"memory", func(t *testing.T) *Store {
			s := New(&memLedger{})
			t.Cleanup(func() { s.Close() })
			return s
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := tc.open(t)

			// Only ratified record kinds are accepted, and that is
			// looplaw's law rather than the storage's.
			if _, err := s.Append(Evidence, "goal-proposal", "x", "{}", "p"); err == nil {
				t.Error("a coined record kind was accepted")
			}

			// An act records every draft or none.
			recs, err := s.AppendAll([]Draft{
				{Kind: Evidence, Type: "claim", Subject: "s", Body: "content", Party: "p"},
				{Kind: Evidence, Type: "admission", Subject: "s", Body: "{}", Party: "p"},
			})
			if err != nil {
				t.Fatal(err)
			}
			if len(recs) != 2 {
				t.Fatalf("want both records, got %d", len(recs))
			}

			// Records come back in order, unaltered, and every one
			// carries an identity.
			all, err := s.Records()
			if err != nil {
				t.Fatal(err)
			}
			if len(all) != 2 {
				t.Fatalf("want 2 records, got %d", len(all))
			}
			if all[0].Body != "content" {
				t.Errorf("body came back altered: %q", all[0].Body)
			}
			if all[0].Seq >= all[1].Seq {
				t.Error("records are not in the order they were appended")
			}
			for _, r := range all {
				if r.Hash == "" {
					t.Errorf("seq %d has no identity", r.Seq)
				}
			}

			// A ledger re-checks what it holds.
			n, err := s.Verify()
			if err != nil {
				t.Fatalf("verify: %v", err)
			}
			if n != 2 {
				t.Errorf("verified %d records, want 2", n)
			}

			// Nothing empty is an act.
			if _, err := s.AppendAll(nil); err == nil {
				t.Error("an empty act was recorded")
			}
		})
	}
}

// The optional capability answers the same question as the fallback. A
// ledger that offers views must not give a different answer from one
// that does not, or correctness would depend on the storage.
func TestLatestOfAgreesAcrossLedgers(t *testing.T) {
	for _, tc := range []struct {
		name string
		open func(t *testing.T) *Store
	}{
		{"sqlite (scans)", func(t *testing.T) *Store { return open(t) }},
		{"memory (scans)", func(t *testing.T) *Store {
			s := New(&memLedger{})
			t.Cleanup(func() { s.Close() })
			return s
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := tc.open(t)
			if got, err := s.LatestOf(Law, "version"); err != nil || got != nil {
				t.Fatalf("an empty ledger holds no version: %v %v", got, err)
			}
			for _, subj := range []string{"first", "second"} {
				if _, err := s.Append(Law, "version", subj, "{}", "p"); err != nil {
					t.Fatal(err)
				}
			}
			// Later evidence must not be mistaken for the latest version.
			if _, err := s.Append(Evidence, "claim", "noise", "{}", "p"); err != nil {
				t.Fatal(err)
			}
			got, err := s.LatestOf(Law, "version")
			if err != nil {
				t.Fatal(err)
			}
			if got == nil || got.Subject != "second" {
				t.Errorf("want the most recent version, got %+v", got)
			}
		})
	}
}

// memCatalog addresses projects in a map. Together with memLedger it is
// a complete storage substitution: two interfaces, two variables, and
// nothing above them changes.
type memCatalog struct {
	projects   map[string]*memLedger
	deployment *memLedger
}

func (c *memCatalog) Describe(root, project string) string {
	return fmt.Sprintf("in memory (%s/%s)", root, project)
}
func (c *memCatalog) Init(root, project string) (Ledger, error) {
	if _, ok := c.projects[project]; ok {
		return nil, fmt.Errorf("init project: %q already exists", project)
	}
	l := &memLedger{}
	c.projects[project] = l
	return l, nil
}
func (c *memCatalog) Open(root, project string) (Ledger, error) {
	l, ok := c.projects[project]
	if !ok {
		return nil, fmt.Errorf("open project: no state for %q", project)
	}
	return l, nil
}

// The deployment's own ledger is held apart from the projects map, which
// is the substituted storage's version of the property the sqlite
// catalog holds by directory: no project key reaches it.
func (c *memCatalog) Deployment(root string) (Ledger, error) {
	if c.deployment == nil {
		c.deployment = &memLedger{}
	}
	return c.deployment, nil
}
func (c *memCatalog) List(root string) ([]string, error) {
	var out []string
	for k := range c.projects {
		out = append(out, k)
	}
	return out, nil
}

// The whole storage layer substitutes at once. This is the property that
// matters when the ledger is replaced: the swap is two implementations
// and two assignments, and every act, gate and command is untouched.
func TestStorageSubstitutesWholesale(t *testing.T) {
	prevCat := DefaultCatalog
	t.Cleanup(func() { DefaultCatalog = prevCat })
	DefaultCatalog = &memCatalog{projects: map[string]*memLedger{}}

	if _, err := InitProject("anywhere", "demo"); err != nil {
		t.Fatalf("init over substituted storage: %v", err)
	}
	if _, err := InitProject("anywhere", "demo"); err == nil {
		t.Error("init did not refuse an existing project")
	}
	if _, err := OpenProject("anywhere", "absent"); err == nil {
		t.Error("open did not refuse a project with no state")
	}

	s, err := OpenProject("anywhere", "demo")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err := s.Append(Evidence, "claim", "s", "body", "p"); err != nil {
		t.Fatalf("record act over substituted storage: %v", err)
	}
	if n, err := s.Verify(); err != nil || n != 1 {
		t.Errorf("verify over substituted storage: %d %v", n, err)
	}
	if got, _ := ListProjects("anywhere"); len(got) != 1 || got[0] != "demo" {
		t.Errorf("list over substituted storage: %v", got)
	}
	if ProjectPath("anywhere", "demo") == "" {
		t.Error("Describe returned nothing for a reader")
	}
}

// Fixed pins the clock for the duration of a test and restores it. A
// ledger stamps time, so nothing derived from a record — its hash, its
// chain, any output carrying either — is reproducible while the clock is
// the wall.
func Fixed(t *testing.T, rfc3339 string) {
	t.Helper()
	at, err := time.Parse(time.RFC3339Nano, rfc3339)
	if err != nil {
		t.Fatal(err)
	}
	prev := Clock
	t.Cleanup(func() { Clock = prev })
	Clock = func() time.Time { return at }
}

// With the clock fixed, the chain is reproducible: the same acts in the
// same order produce the same hashes. That is what makes a recorded
// output pinnable, and what masking a timestamp gives away.
func TestFixedClockMakesTheChainReproducible(t *testing.T) {
	run := func() []Record {
		Fixed(t, "2026-01-01T00:00:00Z")
		s := New(&memLedger{})
		t.Cleanup(func() { s.Close() })
		sq, err := OpenDeployment(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { sq.Close() })
		if _, err := sq.Append(Evidence, "claim", "s", "body", "party"); err != nil {
			t.Fatal(err)
		}
		if _, err := sq.Append(Law, "version", "s", "law", "party"); err != nil {
			t.Fatal(err)
		}
		recs, err := sq.Records()
		if err != nil {
			t.Fatal(err)
		}
		return recs
	}
	a, b := run(), run()
	if len(a) != 2 || len(b) != 2 {
		t.Fatalf("want 2 records each, got %d and %d", len(a), len(b))
	}
	for i := range a {
		if a[i].Hash != b[i].Hash || a[i].At != b[i].At {
			t.Errorf("seq %d is not reproducible under a fixed clock:\n  %s %s\n  %s %s",
				a[i].Seq, a[i].At, a[i].Hash[:12], b[i].At, b[i].Hash[:12])
		}
	}
	if a[0].At != "2026-01-01T00:00:00Z" {
		t.Errorf("the ledger did not read the fixed clock: %q", a[0].At)
	}
}
