// Package absorb is client-layer work: it reads a scope handed to it
// and computes provenance — the content-hash baseline that makes
// staleness deterministic. Derivation itself (what law a scope implies)
// is inference and lives with the caller driving this tool, never here:
// the binary computes what is mechanical and hands the rest out.
//
// The lane matters. This package reads files because the client may
// (loopstrap materializes a tree and invokes the client inside it); the
// kernel never does (T0-4). The comparison half lives in
// internal/provenance and takes data only, so a client submits a
// manifest and the kernel compares it — the split the spec requires,
// made visible in the package boundary rather than promised in a
// comment.
package absorb

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"cuelang.org/go/cue/literal"

	"github.com/xormania/looplaw/internal/provenance"
)

// Manifest is the kernel's submitted-manifest type; the client builds
// one and hands it over.
type Manifest = provenance.Manifest

// skipDirs are never absorbed: version-control internals, dependency
// caches, and looplaw's own state are not the subject's design.
var skipDirs = map[string]bool{
	".git": true, ".hg": true, ".svn": true,
	"node_modules": true, "vendor": true, ".looplaw": true,
}

// ScanScope walks a scope directory and hashes every regular file,
// refusing symlinks rather than following them (a symlink's target lies
// outside the scope the client was handed, so absorbing through one
// would derive statements from content no baseline covers). The scope
// name is supplied by the caller: identity is never derived from a path
// spelling.
func ScanScope(root, scopeName string) (Manifest, error) {
	// Lstat, not Stat: a symlinked root would otherwise walk as a
	// non-directory and yield an empty manifest, which reads downstream
	// as total staleness rather than as the refusal it is.
	info, err := os.Lstat(root)
	if err != nil {
		return Manifest{}, fmt.Errorf("scan scope: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return Manifest{}, fmt.Errorf("scan scope: %s is a symlink; hand the client the scope itself, not a link to it", root)
	}
	if !info.IsDir() {
		return Manifest{}, fmt.Errorf("scan scope: %s is not a directory", root)
	}
	if scopeName == "" {
		return Manifest{}, fmt.Errorf("scan scope: no scope name supplied; the caller names the scope, the client never derives it from a path")
	}

	m := Manifest{Scope: scopeName, Sources: map[string]string{}}
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		if d.IsDir() {
			if skipDirs[d.Name()] && rel != "." {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil // symlinks, sockets, devices: not scope content
		}
		slash := filepath.ToSlash(rel)
		// A path that is not valid UTF-8 cannot be written into CUE
		// without lossy replacement, and two distinct paths could then
		// collide in the baseline — a silent provenance forgery.
		if !utf8.ValidString(slash) {
			return fmt.Errorf("path is not valid UTF-8 and cannot be baselined without collision risk: %q", slash)
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		h := sha256.New()
		// Streamed, not read whole: a large file in a scope must draw a
		// refusal with a remedy, never a runtime fatal no caller can
		// catch.
		if _, err := io.Copy(h, f); err != nil {
			f.Close()
			return err
		}
		f.Close()
		m.Sources[slash] = hex.EncodeToString(h.Sum(nil))
		return nil
	})
	if err != nil {
		return Manifest{}, fmt.Errorf("scan scope: %w", err)
	}
	return m, nil
}

// Skeleton renders a draft view: the provenance block filled from the
// manifest, the statement regions left empty for authoring. It is
// deliberately not a valid set — the gates refuse a set that binds
// nothing — because the authoring is inference and belongs to the
// caller, not the binary. The refusals are the worklist.
//
// Values are quoted through CUE's own literal quoter: Go's %q emits
// \x escapes that CUE cannot parse, so a control byte in a path would
// otherwise produce unparseable output at exit 0.
func Skeleton(subject string, m Manifest) string {
	var b strings.Builder
	fmt.Fprintf(&b, `// DRAFT VIEW SKELETON — not yet a valid set.
//
// Provenance below was computed from the scope; the statement regions
// are empty and the gates will refuse this set until they are authored.
// Those refusals are the worklist. Every contract authored here must
// also be addressed in provenance.derivations, naming the sources it
// was derived from — an unsourced statement cannot go stale, so nothing
// can ever falsify it.
//
// This view is evidence, never law: it states what a party claims the
// scope currently is — submitted as a claim, recorded never believed.
// Law is authored and ratified separately.
//
// Declare experience_declared_absent yourself: silence is not a
// declaration, so the binary leaves it to the author.
subject:        %s
schema_version: "0"

registry: {}
invariants: {}
lexicon: {}
contracts: {}
experience: {}
// experience_declared_absent: true|false

provenance: {
	scope: %s
	sources: {
`, q(subject), q(m.Scope))
	for _, p := range m.Paths() {
		fmt.Fprintf(&b, "\t\t%s: %s\n", q(p), q(m.Sources[p]))
	}
	b.WriteString("\t}\n\tderivations: {\n\t\t// \"C-EXAMPLE-1\": [\"path/read.go\"]\n\t}\n}\n")
	return b.String()
}

// q quotes a value as CUE, not as Go: %q's \x escapes are not CUE
// syntax, so a control byte in a path would make the skeleton
// unparseable.
func q(s string) string {
	return literal.String.Quote(s)
}
