package outcome

import "testing"

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
