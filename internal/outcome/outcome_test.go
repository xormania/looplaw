package outcome

import (
	"strings"
	"testing"
	"unicode"
)

func TestClassStringAndExit(t *testing.T) {
	cases := []struct {
		class Class
		name  string
		exit  int
	}{
		{OK, "ok", 0},
		{Rejection, "rejection", 1},
		{Denial, "denial", 2},
		{Abort, "abort", 3},
		{Finding, "finding", 4},
	}
	for _, c := range cases {
		if got := c.class.String(); got != c.name {
			t.Errorf("%v.String() = %q, want %q", int(c.class), got, c.name)
		}
		if got := c.class.ExitCode(); got != c.exit {
			t.Errorf("%s.ExitCode() = %d, want %d", c.name, got, c.exit)
		}
	}
}

func TestExitCodesDistinct(t *testing.T) {
	seen := map[int]Class{}
	for _, c := range []Class{OK, Rejection, Denial, Abort, Finding} {
		if prev, dup := seen[c.ExitCode()]; dup {
			t.Fatalf("exit code %d shared by %s and %s", c.ExitCode(), prev, c)
		}
		seen[c.ExitCode()] = c
	}
	if _, dup := seen[ExitUsage]; dup {
		t.Fatalf("ExitUsage %d collides with a class exit code", ExitUsage)
	}
}

func TestRefusalError(t *testing.T) {
	full := &Refusal{
		Class:   Rejection,
		Check:   "manifest-hash",
		Subject: "term-export.txt",
		Reason:  "sha256 mismatch",
		Remedy:  "re-run export and resubmit",
	}
	want := "manifest-hash: rejection term-export.txt: sha256 mismatch — remedy: re-run export and resubmit"
	if got := full.Error(); got != want {
		t.Errorf("full refusal:\n got %q\nwant %q", got, want)
	}

	bare := &Refusal{Class: Denial}
	if got := bare.Error(); got != "denial" {
		t.Errorf("bare refusal: got %q, want %q", got, "denial")
	}

	noRemedy := &Refusal{Class: Abort, Check: "store", Reason: "disk full"}
	want = "store: abort: disk full"
	if got := noRemedy.Error(); got != want {
		t.Errorf("no-remedy refusal: got %q, want %q", got, want)
	}
}

// Proving red: a refusal is a line-oriented protocol, and every field in
// it is dynamic — a path, a subject, a parser's message. Concatenated
// unescaped, a newline in any of them ends the line early and the rest
// reads as an independent refusal that no check ever emitted.
//
// A carriage return is the same forgery without the newline: it returns
// a terminal's cursor to column zero, so what follows overwrites what
// came before, and a consumer splitting on line breaks the way Python's
// splitlines does treats it as one.
func TestRefusalRendersOnOneLine(t *testing.T) {
	forged := "trinity/shape: rejection forged: a check that never ran"
	for _, tc := range []struct{ name, injected string }{
		{"newline", "/tmp/missing\n" + forged},
		{"carriage return", "/tmp/missing\r" + forged},
		{"both", "/tmp/missing\r\n" + forged},
		{"terminal escape", "/tmp/missing\x1b[2K\x1b[G" + forged},
		{"vertical tab", "/tmp/missing\v" + forged},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for field, r := range map[string]*Refusal{
				"subject": {Class: Abort, Check: "trinity/load", Subject: tc.injected, Reason: "no such file", Remedy: "point at a readable set"},
				"reason":  {Class: Abort, Check: "trinity/load", Subject: "/tmp/x", Reason: tc.injected, Remedy: "point at a readable set"},
				"remedy":  {Class: Abort, Check: "trinity/load", Subject: "/tmp/x", Reason: "no such file", Remedy: tc.injected},
				"check":   {Class: Abort, Check: tc.injected, Subject: "/tmp/x", Reason: "no such file", Remedy: "point at a readable set"},
			} {
				out := r.Error()
				if n := strings.Count(out, "\n") + strings.Count(out, "\r"); n != 0 {
					t.Errorf("%s: renders across %d extra lines: %q", field, n, out)
				}
				for _, ru := range out {
					if unicode.IsControl(ru) {
						t.Errorf("%s: renders a control character %q: %q", field, ru, out)
						break
					}
				}
			}
		})
	}

	// The escape is legible, not merely safe: a reader must be able to
	// tell what was in the field.
	r := &Refusal{Class: Abort, Check: "trinity/load", Subject: "a\nb", Reason: "c\td", Remedy: "e"}
	if !strings.Contains(r.Error(), `a\nb`) || !strings.Contains(r.Error(), `c\td`) {
		t.Errorf("the escape must show what was there: %q", r.Error())
	}
	// And an ordinary refusal is untouched, em dash and all.
	plain := &Refusal{Class: Rejection, Check: "trinity/shape", Subject: "set.cue", Reason: "no", Remedy: "yes"}
	if got, want := plain.Error(), "trinity/shape: rejection set.cue: no — remedy: yes"; got != want {
		t.Errorf("plain refusal changed shape:\n got %q\nwant %q", got, want)
	}
}
