package gate

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/xormania/looplaw/internal/outcome"
)

// SubmissionChecks enumerates every check the submission gates can
// emit. As with the trinity gates, the suite proves a red for each: an
// undemonstrated gate is an unproven behavior.
var SubmissionChecks = []string{
	"submit/kind",
	"submit/subject",
	"submit/party",
	"submit/content",
	"submit/receipt-shape",
}

// Submission is what a party offers toward the gates: pre-record,
// holding no standing. It becomes a record only if the gates pass it
// and the store records it.
type Submission struct {
	Kind    string // "claim" or "receipt" — the record kinds a party may submit
	Subject string
	Party   string
	Body    string
}

// MaxBytes bounds what a caller may hand the gates in one piece: a
// submitted body, or a set file.
//
// Every read was unbounded, so a 16 MiB claim streamed through stdin was
// allocated, hashed and stored, growing the ledger by the same amount,
// and repetition scaled it linearly. Any wrapper putting these commands
// in front of an untrusted submitter could spend a deployment's memory
// and disk at the submitter's choosing.
//
// One megabyte, against the largest artifacts anyone writes here: the
// biggest CUE in this repository is its own design basis at 71 KB, and a
// law fixture is 7.6 KB. Fourteen times the largest real thing is a
// bound that refuses an attack without ever meeting honest work — which
// is the only kind of limit worth setting from here, since what a
// deployment considers a large claim is not this binary's to know.
const MaxBytes = 1 << 20

// Receipt is the ratified receipt shape: evidence of something that
// happened elsewhere.
type Receipt struct {
	Subject string `json:"subject"`
	Verdict string `json:"verdict"`
	Source  string `json:"source"`
	Hash    string `json:"hash"`
}

// IsName reports whether a string is a name an act may record: a
// subject, or a party. Exported because the acts that record one are
// not all in this package, and a second copy of the grammar is a second
// thing to keep in step — the authority binding checked its party
// against the empty string alone, which admitted "   " as a
// deployment's accountable authority.
func IsName(s string) bool { return nameRE.MatchString(s) }

var (
	// A name is one line of printable text: it must not begin with
	// whitespace, and must hold no control character anywhere.
	//
	// The second half was `[^\n]*`, which constrained the newline alone —
	// and `[^\s]` constrains only the first character, so a carriage
	// return, a tab or a terminal escape passed anywhere after it. A name
	// is recorded and the ledger is append-only, so what got in outlived
	// the run: a carriage return returns a terminal's cursor to column
	// zero, and a reader met the forgery raw every time afterwards.
	//
	// Internal spaces stay legal, and so does anything outside ASCII: a
	// name is read by people, and refusing what it can legitimately hold
	// would be a different defect.
	nameRE = regexp.MustCompile(`^[^\s\p{Cc}][^\p{Cc}]*$`)
	hashRE = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

// submittableKinds are the record kinds a party may offer. Admissions
// and versions are produced by the system in the course of an act, so
// no party submits one: an admission a party wrote would say the entry
// happened before it had.
var submittableKinds = map[string]bool{"claim": true, "receipt": true}

// decodeReceipt reads a receipt strictly, because a recorded receipt has
// to say one thing to everyone who reads it.
//
// json.Unmarshal is permissive in two ways that matter here. It keeps
// the last of a repeated key, and parsers disagree about which one wins
// — Go and Python take the last, others take the first — so the same
// recorded bytes carry two verdicts depending on who reads them. And it
// discards a field the shape does not name, which reads as meaningful to
// any consumer that later grows one.
//
// Trailing content is already refused by the standard decoder, so
// nothing here repeats that.
func decodeReceipt(body string) (Receipt, error) {
	var r Receipt
	dec := json.NewDecoder(strings.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&r); err != nil {
		return r, err
	}
	if err := noRepeatedKey(json.NewDecoder(strings.NewReader(body))); err != nil {
		return r, err
	}
	return r, nil
}

// noRepeatedKey walks the document and refuses an object that states the
// same key twice, at any depth: a nested one is as ambiguous as a
// top-level one, and cheaper to refuse than to explain later.
func noRepeatedKey(dec *json.Decoder) error {
	var walk func(seen map[string]bool) error
	walk = func(seen map[string]bool) error {
		for {
			tok, err := dec.Token()
			if err != nil {
				return err
			}
			switch t := tok.(type) {
			case json.Delim:
				switch t {
				case '}', ']':
					return nil
				case '{':
					if err := walk(map[string]bool{}); err != nil {
						return err
					}
				case '[':
					if err := walk(nil); err != nil {
						return err
					}
				}
			case string:
				if seen == nil {
					continue // a string in a list is a value, not a key
				}
				if seen[t] {
					return fmt.Errorf("the key %q is stated twice, and readers disagree about which one holds", t)
				}
				seen[t] = true
				if err := valueOf(dec, &walk); err != nil {
					return err
				}
			}
		}
	}
	tok, err := dec.Token()
	if err != nil {
		return err
	}
	if d, ok := tok.(json.Delim); !ok || d != '{' {
		return nil // not an object; the decoder above has already judged it
	}
	return walk(map[string]bool{})
}

// valueOf consumes the value following a key, descending into it so that
// a repeated key inside is found too.
func valueOf(dec *json.Decoder, walk *func(map[string]bool) error) error {
	tok, err := dec.Token()
	if err != nil {
		return err
	}
	if d, ok := tok.(json.Delim); ok {
		switch d {
		case '{':
			return (*walk)(map[string]bool{})
		case '[':
			return (*walk)(nil)
		}
	}
	return nil
}

// ValidateSubmission verifies the preconditions of the record act. It
// judges nothing about the content: whether a claim is true is not a
// question the gates ask, and recording settles only that it was said.
//
// The returned check list names the checks that actually ran, not the
// gate's full set. submit/receipt-shape runs only for a receipt, so an
// admission written from SubmissionChecks claimed a check that never
// touched the claim it recorded — evidence of an examination that did
// not happen, which is what an admission is least able to afford.
func ValidateSubmission(sub Submission) ([]string, []outcome.Refusal) {
	var ran []string
	var refusals []outcome.Refusal
	refuse := func(check, subject, reason, remedy string) {
		refusals = append(refusals, outcome.Refusal{
			Class: outcome.Rejection, Check: check,
			Subject: subject, Reason: reason, Remedy: remedy,
		})
	}

	ran = append(ran, "submit/kind")
	if !submittableKinds[sub.Kind] {
		var kinds []string
		for k := range submittableKinds {
			kinds = append(kinds, k)
		}
		reason := fmt.Sprintf("%q is not a kind a party may submit", sub.Kind)
		if sub.Kind == "admission" || sub.Kind == "version" {
			reason = fmt.Sprintf("%q is produced in the course of an act, never offered by a party", sub.Kind)
		}
		refuse("submit/kind", sub.Kind, reason,
			"submit a claim (what you state) or a receipt (evidence of something that happened elsewhere)")
	}

	ran = append(ran, "submit/subject")
	if !nameRE.MatchString(sub.Subject) {
		refuse("submit/subject", "subject", "a submission names no subject",
			"name what the submission is about; a record about nothing can never be found or falsified")
	}
	ran = append(ran, "submit/party")
	if !nameRE.MatchString(sub.Party) {
		refuse("submit/party", "party", "a submission names no submitting party",
			"name the submitting party; recording settles that a party said a thing, which is unstatable without the party")
	}
	ran = append(ran, "submit/content")
	switch {
	case strings.TrimSpace(sub.Body) == "":
		refuse("submit/content", "body", "a submission carries nothing",
			"state what is claimed, or what the receipt evidences")
	case !utf8.ValidString(sub.Body):
		// A record is text a party stated, and the export is the ledger
		// as recorded. Bytes that are not text were recorded as
		// themselves and exported as replacement characters, so the
		// export could not reproduce the record it came from, and the
		// record hash covers the original bytes — nothing downstream
		// could re-derive it.
		refuse("submit/content", "body",
			"a submission carries bytes that are not text",
			"submit text; a body that cannot be read back as it was recorded is a file to submit the digest of, not a claim to record")
	case len(sub.Body) > MaxBytes:
		// Checked here as well as where the bytes are read, so a caller
		// that is not the command line meets the same bound.
		refuse("submit/content", "body",
			fmt.Sprintf("a submission carries %d bytes, and the gates take at most %d", len(sub.Body), MaxBytes),
			"submit what is claimed; a body larger than this is a file to record the digest of, not a claim to read")
	}

	if sub.Kind == "receipt" {
		ran = append(ran, "submit/receipt-shape")
		r, err := decodeReceipt(sub.Body)
		switch {
		case err != nil:
			refuse("submit/receipt-shape", "body",
				fmt.Sprintf("a receipt's body is not readable as its ratified shape: %v", err),
				`a receipt is (subject, verdict, source, hash): {"subject":…,"verdict":…,"source":…,"hash":…}`)
		case r.Subject == "" || r.Verdict == "" || r.Source == "":
			refuse("submit/receipt-shape", "body",
				"a receipt states no subject, verdict, or source",
				"evidence with no source and no verdict evidences nothing; state all three")
		case !hashRE.MatchString(r.Hash):
			refuse("submit/receipt-shape", "hash",
				fmt.Sprintf("%q is not a sha256 hex digest", r.Hash),
				"a receipt carries the digest of what it evidences, so it can be checked later")
		case r.Subject != sub.Subject:
			// The envelope subject is what the record is filed under and
			// what an index finds it by; the body carries its own. A
			// receipt whose two subjects disagree is one an index and a
			// reader answer differently about.
			refuse("submit/receipt-shape", "subject",
				fmt.Sprintf("the receipt is about %q and is submitted about %q", r.Subject, sub.Subject),
				"submit the receipt under the subject it names; a record filed under one subject and stating another is found by one and read as the other")
		}
	}

	return ran, refusals
}
