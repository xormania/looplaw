// Package schema carries the set schema into the binary: the gates
// validate a target project's law against the schema the binary ships,
// so a deployment's checks match its build rather than whatever file
// happens to be on disk.
//
// This is the form a project's law must take, not law itself — law is
// what an accountable authority ratifies, and it lives in the ledger.
// The same files are importable by anything authoring or checking a
// set with plain cue, which is how CI runs a second producer over the
// same data.
package schema

import "embed"

// Files is the complete schema package (every schema/*.cue file).
//
//go:embed *.cue
var Files embed.FS
