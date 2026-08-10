package diff

import (
	"encoding/json"
	"fmt"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"

	"github.com/xormania/looplaw/internal/gate"
	"github.com/xormania/looplaw/internal/outcome"
)

// selfCheck unifies every emitted gap with the ratified #Gap schema.
// The differ validating its own output is the two-producer discipline
// turned inward: an engine that emits law-nonconformant gaps is broken,
// and it says so loudly rather than feeding planning bad records.
func selfCheck(gaps []Gap) *outcome.Refusal {
	ctx := cuecontext.New()
	law, err := gate.Law(ctx)
	if err != nil {
		return &outcome.Refusal{
			Class: outcome.Abort, Check: "diff/self-check",
			Subject: "law (embedded)", Reason: err.Error(),
			Remedy: "the embedded law is broken; replace this binary with one embedding the ratified law",
		}
	}
	def := law.LookupPath(cue.ParsePath("#Gap"))
	for _, g := range gaps {
		b, err := json.Marshal(g)
		if err != nil {
			return &outcome.Refusal{
				Class: outcome.Abort, Check: "diff/self-check",
				Subject: g.ID, Reason: err.Error(),
				Remedy: "the differ emitted an unmarshalable gap; this binary is broken",
			}
		}
		v := ctx.CompileBytes(b)
		if err := def.Unify(v).Validate(cue.Concrete(true)); err != nil {
			return &outcome.Refusal{
				Class: outcome.Abort, Check: "diff/self-check",
				Subject: g.ID,
				Reason:  fmt.Sprintf("emitted gap does not satisfy the ratified #Gap schema: %v", err),
				Remedy:  "the differ is out of step with ratified law; this binary is broken — do not consume its output",
			}
		}
	}
	return nil
}
