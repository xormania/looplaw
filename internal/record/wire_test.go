package record

import (
	"encoding/json"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/xormania/looplaw/internal/gate"
)

// The JSON keys of a persisted body are a contract, and a stricter one
// than an ordinary wire format.
//
// These bodies are written into an append-only ledger and read back by
// later acts. Renaming a key does not break a consumer at the boundary
// where the change was made — it orphans every record already written,
// permanently, because nothing can rewrite them. The reader then finds
// the field absent and reports its zero value as fact.
//
// That is not hypothetical. Renaming AuthorityBinding.bound made a
// deployment holding a recorded accountable authority answer "no
// accountable authority is on record for this deployment" and refuse
// every law-making act. The binding was still there. Nothing failed: not
// one test in the suite, not dev/check. The export golden pins the
// ledger's own columns, but only ever holds a claim and a receipt, so no
// act's admission body reached it.
//
// A key set that changes deliberately changes here first, and the commit
// that does it must say what happens to ledgers already written.
func TestPersistedBodyKeysAreContract(t *testing.T) {
	for _, tc := range []struct {
		name  string
		value any
		keys  []string
	}{
		{"admission — the entry event for a submission", Admission{},
			[]string{"kind", "subject", "party", "content_hash", "checks_run", "grant"}},
		{"declaration — the entry event for an amendment draft", Declaration{},
			[]string{"act", "subject", "party", "content_hash", "checks_run", "against_law"}},
		{"ratification — the entry event for the act that makes law", Ratification{},
			[]string{"act", "party", "subject", "draft", "checks_run"}},
		{"authority binding — who may make law here", AuthorityBinding{},
			[]string{"act", "party", "bound"}},
		// Read from what a party submits rather than written by an act,
		// which makes it an input contract: a rename refuses every
		// receipt already being sent.
		{"receipt — evidence submitted by its source", gate.Receipt{},
			[]string{"subject", "verdict", "source", "hash"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := jsonKeys(t, tc.value)
			want := append([]string(nil), tc.keys...)
			sort.Strings(got)
			sort.Strings(want)
			if !reflect.DeepEqual(got, want) {
				t.Errorf("persisted keys changed.\n got: %v\nwant: %v\n"+
					"A key here is written into an append-only ledger and read back by a later act. "+
					"Renaming one orphans every record already written, and the reader reports the zero value as fact. "+
					"If the change is deliberate, state here what happens to ledgers already written.",
					got, want)
			}
		})
	}
}

// Omitempty is part of the contract too: a field that disappears when
// empty reads as absent, and absent is what a reader of an older record
// sees. Only fields whose absence is meaningful may carry it.
func TestOnlyMeaningfulAbsenceIsOmitempty(t *testing.T) {
	meaningful := map[string]string{
		// Empty when no law is ratified, which is a first declaration
		// rather than a missing value.
		"against_law": "absence means the project holds no ratified law, which is a first declaration",
		"grant":       "absence means no standing grant licensed the entry; the citation confers nothing either way",
	}
	for _, v := range []any{Admission{}, Declaration{}, Ratification{}, AuthorityBinding{}, gate.Receipt{}} {
		rt := reflect.TypeOf(v)
		for i := 0; i < rt.NumField(); i++ {
			tag := rt.Field(i).Tag.Get("json")
			name, opts, _ := strings.Cut(tag, ",")
			if !strings.Contains(opts, "omitempty") {
				continue
			}
			if _, ok := meaningful[name]; !ok {
				t.Errorf("%s.%s is omitempty with no stated meaning for its absence.\n"+
					"A reader of an older record cannot tell an omitted field from one that was never written; "+
					"say what absence means, or drop omitempty.", rt.Name(), name)
			}
		}
	}
}

func jsonKeys(t *testing.T, v any) []string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	// omitempty fields vanish from a zero value, so they are read from
	// the tags rather than the marshalled output.
	rt := reflect.TypeOf(v)
	for i := 0; i < rt.NumField(); i++ {
		name, _, _ := strings.Cut(rt.Field(i).Tag.Get("json"), ",")
		if name != "" && name != "-" {
			m[name] = nil
		}
	}
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
