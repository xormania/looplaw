package store

import (
	"fmt"
	"testing"
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
