// Package law carries the ratified law artifacts into the binary: the
// gates validate target sets against the law the binary ships, so a
// deployment's checks match its build, not whatever file happens to be
// on disk.
package law

import "embed"

// Files is the complete ratified law package (every law/*.cue file).
//
//go:embed *.cue
var Files embed.FS
