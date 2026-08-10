//go:build tools

// Package tools pins the cue command to the same module version as the
// embedded CUE library. The two producers CI runs over the law — the
// gate's embedded evaluator and the cue binary — must be the same
// version, or "the producers agree" proves only that two different
// evaluators agreed.
package tools

import _ "cuelang.org/go/cmd/cue"
