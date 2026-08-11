package main

// Behavior tests: the binary's observable contract — commands, exit
// codes per the failure doctrine, stdout/stderr discipline, and the
// refusal wire grammar. These run the built binary, not the packages:
// what a consumer scripts against is what is tested.

import (
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var bin string

const fixture = "../../internal/gate/testdata/library/set.cue"

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "looplaw-behavior")
	if err != nil {
		panic(err)
	}
	bin = filepath.Join(dir, "looplaw")
	out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput()
	if err != nil {
		panic("building looplaw: " + err.Error() + "\n" + string(out))
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

func run(t *testing.T, args ...string) (stdout, stderr string, exit int) {
	t.Helper()
	return runEnv(t, nil, args...)
}

func runEnv(t *testing.T, env []string, args ...string) (stdout, stderr string, exit int) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	if env != nil {
		cmd.Env = append(os.Environ(), env...)
	}
	var so, se strings.Builder
	cmd.Stdout, cmd.Stderr = &so, &se
	err := cmd.Run()
	exit = 0
	if ee, ok := err.(*exec.ExitError); ok {
		exit = ee.ExitCode()
	} else if err != nil {
		t.Fatalf("running %v: %v", args, err)
	}
	return so.String(), se.String(), exit
}

// mutate writes a one-edit variant of the green fixture and returns its
// path.
func mutate(t *testing.T, old, new string) string {
	t.Helper()
	base, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(base), old) {
		t.Fatalf("mutation target drifted from fixture: %q", old)
	}
	path := filepath.Join(t.TempDir(), "set.cue")
	if err := os.WriteFile(path, []byte(strings.Replace(string(base), old, new, 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// The store-backed verbs run against a state root of their own, so the
// behavior suite never touches a real one.
func TestRecordActBehavior(t *testing.T) {
	root := t.TempDir()
	env := []string{"LOOPLAW_ROOT=" + root, "LOOPLAW_PARTY=behavior:test"}

	body := filepath.Join(t.TempDir(), "claim.json")
	if err := os.WriteFile(body, []byte(`{"states":"a contract exists"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	// State is never created implicitly: every verb refuses a project
	// no init act has made.
	for _, args := range [][]string{{"submit", "ghost", "claim", "s", body}, {"verify", "ghost"}, {"export", "ghost"}} {
		if _, stderr, exit := runEnv(t, env, args...); exit != 1 || !strings.Contains(stderr, "project/state") {
			t.Errorf("%v against an uninitialized project: exit=%d stderr=%s", args, exit, stderr)
		}
	}

	if _, stderr, exit := runEnv(t, env, "init", "demo"); exit != 0 {
		t.Fatalf("init: exit=%d %s", exit, stderr)
	}
	if _, stderr, exit := runEnv(t, env, "init", "demo"); exit != 1 || !strings.Contains(stderr, "init/project") {
		t.Errorf("a second init must refuse: exit=%d %s", exit, stderr)
	}

	stdout, stderr, exit := runEnv(t, env, "submit", "demo", "claim", "scope-x", body)
	if exit != 0 {
		t.Fatalf("submit: exit=%d %s", exit, stderr)
	}
	if !strings.Contains(stdout, "claim seq 1") || !strings.Contains(stdout, "admission seq 2") {
		t.Errorf("submit must report the content and its admission: %q", stdout)
	}

	if stdout, _, exit := runEnv(t, env, "verify", "demo"); exit != 0 || !strings.Contains(stdout, "2 records, chain verified") {
		t.Errorf("verify: exit=%d %q", exit, stdout)
	}
	if stdout, _, exit := runEnv(t, env, "export", "demo"); exit != 0 || !strings.Contains(stdout, `"type": "admission"`) {
		t.Errorf("export: exit=%d %q", exit, stdout)
	}

	// An unattributed submission is refused, and refuses on stderr.
	noParty := []string{"LOOPLAW_ROOT=" + root}
	if stdout, stderr, exit := runEnv(t, noParty, "submit", "demo", "claim", "s", body); exit != 1 {
		t.Errorf("unattributed submit: exit=%d", exit)
	} else if !strings.Contains(stderr, "submit/party") || strings.TrimSpace(stdout) != "" {
		t.Errorf("refusal must name submit/party on stderr only: out=%q err=%q", stdout, stderr)
	}
}

// Proving red: every store-backed verb takes a project argument, and
// that argument is whatever the caller typed. Before the catalog checked
// it, `submit .. …` recorded into the deployment ledger — where the
// accountable-authority binding lives, and where an ordinary claim
// shaped like one was read back as that binding — and `submit
// demo/../.. …` wrote a database above the state root entirely.
//
// The refusal is only half the contract. The other half is that nothing
// was created: an escape that refuses after opening the database has
// still left the file, so this walks the directory holding the state
// root and fails on any ledger outside root/projects/<key>.
func TestProjectArgumentCannotEscapeTheStateRoot(t *testing.T) {
	top := t.TempDir()
	root := filepath.Join(top, "state")
	if err := os.MkdirAll(filepath.Join(top, "sibling"), 0o755); err != nil {
		t.Fatal(err)
	}
	env := []string{"LOOPLAW_ROOT=" + root, "LOOPLAW_PARTY=behavior:test"}

	body := filepath.Join(t.TempDir(), "claim.json")
	if err := os.WriteFile(body, []byte(`{"states":"a contract exists"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, stderr, exit := runEnv(t, env, "init", "demo"); exit != 0 {
		t.Fatalf("init: exit=%d %s", exit, stderr)
	}

	for _, key := range []string{"", ".", "..", "../..", "demo/../..", "../sibling", "demo/"} {
		for _, args := range [][]string{
			{"submit", key, "claim", "s", body},
			{"verify", key},
			{"export", key},
			{"init", key},
		} {
			_, stderr, exit := runEnv(t, env, args...)
			if exit != 1 {
				t.Errorf("%v with project %q: exit=%d, want 1", args[:1], key, exit)
			}
			if !strings.Contains(stderr, "project/state") && !strings.Contains(stderr, "init/project") {
				t.Errorf("%v with project %q must refuse by check id: %s", args[:1], key, stderr)
			}
		}
	}

	namespace := filepath.Join(root, "projects", "demo")
	err := filepath.WalkDir(top, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasPrefix(d.Name(), "looplaw.db") {
			return err
		}
		if dir := filepath.Dir(path); dir != namespace {
			t.Errorf("a ledger stands outside the project namespace: %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestBehaviorContract(t *testing.T) {
	red := mutate(t, `client:   "borrower"`, `client:   "stranger"`)
	unparseable := mutate(t, `subject:        "lend-library"`, `subject: %%%`)

	cases := []struct {
		name      string
		args      []string
		wantExit  int
		stdoutHas string
		stderrHas string
	}{
		{"version", []string{"version"}, 0, "looplaw 0.0.0-dev", ""},
		{"no-args-is-usage", nil, 64, "", "usage: looplaw"},
		{"unknown-command-is-usage", []string{"conquer"}, 64, "", `unknown command "conquer"`},
		{"validate-green-says-ok", []string{"validate", fixture}, 0, "ok", ""},
		{"validate-without-path-is-usage", []string{"validate"}, 64, "", "usage: looplaw validate"},
		{"validate-missing-file-aborts", []string{"validate", "no-such.cue"}, 3, "", "trinity/load: abort"},
		{"validate-red-rejects", []string{"validate", red}, 1, "", `"stranger"`},
		{"validate-unparseable-rejects", []string{"validate", unparseable}, 1, "", "trinity/parse: rejection"},
		{"diff-usage", []string{"diff", fixture}, 64, "", "usage: looplaw diff"},
		{"diff-identical-empty-feed", []string{"diff", fixture, fixture}, 0, "[]", ""},
		{"diff-gaps-are-success", []string{"diff", fixture, "../../internal/diff/testdata/library-view.cue"}, 0, `"kind": "absent"`, ""},
		{"diff-invalid-side-rejects", []string{"diff", fixture, red}, 1, "", "diff/side"},
		{"absorb-usage", []string{"absorb", "../../internal/absorb/testdata/scope"}, 64, "", "usage: looplaw absorb"},
		{"absorb-prints-provenance", []string{"absorb", "../../internal/absorb/testdata/scope", "lend-library"}, 0, "provenance: {", ""},
		{"absorb-missing-scope-aborts", []string{"absorb", "no-such-dir", "x"}, 3, "", "absorb/scope: abort"},
		{"status-unchanged-scope", []string{"status", "../../internal/absorb/testdata/view.cue", "../../internal/absorb/testdata/scope"}, 0, `"stale": false`, ""},
		{"status-on-authored-law-rejects", []string{"status", fixture, "../../internal/absorb/testdata/scope"}, 1, "", "status/no-provenance"},
		{"status-usage", []string{"status", fixture}, 64, "", "usage: looplaw status"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			stdout, stderr, exit := run(t, c.args...)
			if exit != c.wantExit {
				t.Errorf("exit = %d, want %d (stderr: %s)", exit, c.wantExit, stderr)
			}
			if c.stdoutHas != "" && !strings.Contains(stdout, c.stdoutHas) {
				t.Errorf("stdout %q does not contain %q", stdout, c.stdoutHas)
			}
			if c.stderrHas != "" && !strings.Contains(stderr, c.stderrHas) {
				t.Errorf("stderr %q does not contain %q", stderr, c.stderrHas)
			}
			// Output discipline: refusals go to stderr, never stdout; a
			// green result reports on stdout, never stderr.
			if c.wantExit != 0 && strings.TrimSpace(stdout) != "" {
				t.Errorf("refusing run wrote to stdout: %q", stdout)
			}
			if c.wantExit == 0 && strings.TrimSpace(stderr) != "" {
				t.Errorf("green run wrote to stderr: %q", stderr)
			}
		})
	}
}

// Every refusal line follows the wire grammar the failure doctrine
// promises: "<check>: <class> <subject>: <reason> — remedy: <remedy>".
// Callers retry off this shape; it is a contract, not formatting.
func TestRefusalWireGrammar(t *testing.T) {
	red := mutate(t, `client:   "borrower"`, `client:   "stranger"`)
	_, stderr, exit := run(t, "validate", red)
	if exit != 1 {
		t.Fatalf("exit = %d, want 1", exit)
	}
	line := regexp.MustCompile(`^[a-z]+(/[a-z-]+)?: (rejection|denial|abort|finding) .+ — remedy: .+$`)
	lines := strings.Split(strings.TrimSpace(stderr), "\n")
	if len(lines) == 0 {
		t.Fatal("no refusal lines")
	}
	for _, l := range lines {
		if !line.MatchString(l) {
			t.Errorf("refusal line breaks the wire grammar: %q", l)
		}
	}
}
