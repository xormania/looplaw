// Package absorb is client-layer work: it reads a scope handed to it
// and computes provenance — the content-hash baseline that makes
// staleness deterministic. Derivation itself (what law a scope implies)
// is inference and lives with the agent driving this tool, never here:
// the binary computes what is mechanical and hands the rest out.
//
// The lane matters. This package reads files because the client may
// (loopstrap materializes a tree and invokes the client inside it); the
// kernel never does (T0-4). The comparison half — compare.go — takes
// data only and touches no filesystem, so the staleness check is the
// same computation wherever it runs.
package absorb

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Manifest is a content-hash baseline over a scope: relative path to
// sha256, plus the scope name the client used.
type Manifest struct {
	Scope   string            `json:"scope"`
	Sources map[string]string `json:"sources"`
}

// Paths returns the manifest's paths in sorted order — every output
// derived from a manifest is deterministic (T0-3 is the kernel's rule;
// client output honors it so runs are comparable).
func (m Manifest) Paths() []string {
	paths := make([]string, 0, len(m.Sources))
	for p := range m.Sources {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	return paths
}

// skipDirs are never absorbed: version-control internals, dependency
// caches, and looplaw's own state are not the subject's design.
var skipDirs = map[string]bool{
	".git": true, ".hg": true, ".svn": true,
	"node_modules": true, "vendor": true, ".looplaw": true,
}

// ScanScope walks a scope directory and hashes every regular file,
// refusing symlinks rather than following them (a symlink's target is
// outside the scope the client was handed, so absorbing through one
// would source law from unscoped content).
func ScanScope(root string) (Manifest, error) {
	info, err := os.Stat(root)
	if err != nil {
		return Manifest{}, fmt.Errorf("scan scope: %w", err)
	}
	if !info.IsDir() {
		return Manifest{}, fmt.Errorf("scan scope: %s is not a directory", root)
	}

	m := Manifest{Scope: filepath.Base(strings.TrimRight(root, string(filepath.Separator))), Sources: map[string]string{}}
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
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(data)
		m.Sources[filepath.ToSlash(rel)] = hex.EncodeToString(sum[:])
		return nil
	})
	if err != nil {
		return Manifest{}, fmt.Errorf("scan scope: %w", err)
	}
	return m, nil
}

// Skeleton renders a draft view: the provenance block filled from the
// manifest, the law regions left empty for authoring. It is deliberately
// not a valid set — the gates refuse a set that binds nothing — because
// the authoring is inference and belongs to the agent, not the binary.
// The refusals are the worklist.
func Skeleton(subject string, m Manifest) string {
	var b strings.Builder
	fmt.Fprintf(&b, `// DRAFT VIEW SKELETON — not yet a valid set.
//
// Provenance below was computed from the scope; the law regions are
// empty and the gates will refuse this set until they are authored.
// Those refusals are the worklist. Every contract authored here must
// also be addressed in provenance.derivations, naming the sources it
// was derived from — an unsourced statement cannot go stale, so nothing
// can ever falsify it.
//
// This view is evidence, never law: it records what a party claims the
// scope currently is. Law is authored and ratified separately.
subject:        %q
schema_version: "0"

registry: {}
invariants: {}
lexicon: {}
contracts: {}
experience: {}
experience_declared_absent: true

provenance: {
	scope: %q
	sources: {
`, subject, m.Scope)
	for _, p := range m.Paths() {
		fmt.Fprintf(&b, "\t\t%q: %q\n", p, m.Sources[p])
	}
	b.WriteString("\t}\n\tderivations: {\n\t\t// \"C-EXAMPLE-1\": [\"path/read.go\"]\n\t}\n}\n")
	return b.String()
}
