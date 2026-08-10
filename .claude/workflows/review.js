export const meta = {
  name: 'review',
  description: 'Adversarially review a batch: independent lenses find, independent skeptics try to refute',
  whenToUse:
    'Before asking for a merge on any batch that changes behavior. Pass args {target, focus, lenses?}. ' +
    'Every blocking defect this repo has found came from a review like this rather than from its own tests.',
  phases: [
    { title: 'Review', detail: 'independent lenses over the batch' },
    { title: 'Verify', detail: 'refute or confirm each finding against the built binary' },
  ],
}

const REPO = '/home/work/projects/xormania/looplaw'

const a = args || {}
const target = a.target || 'the current branch (git diff master...HEAD)'
const focus = a.focus || 'the changed code'

// The three lenses that have found every confirmed defect so far. A
// caller may supply its own; these are the default because each one
// catches a class the others miss.
const DEFAULT_LENSES = [
  {
    key: 'correctness',
    prompt: `Hunt correctness bugs in ${target} on ${REPO}. Focus: ${focus}.

Attack concretely rather than reading for style: build inputs under /tmp, run the built binary against them (go run ./cmd/looplaw ...), and report only what you reproduced or can argue airtight. Classes that have actually bitten this repo, so try them first:
- silent skips — a check that no-ops when a lookup errors, letting malformed input pass unexamined
- hash and delimiter forgery — two materially different things comparing equal
- values that satisfy a schema while meaning nothing (empty strings, absent regions, self-references)
- nondeterminism — map iteration reaching any output a consumer reads, including refusal order
- inputs that are legal to the schema but that no check examines at all
For each finding: the exact command, the observed output, and the failure scenario.`,
  },
  {
    key: 'law-conformance',
    prompt: `Review ${target} on ${REPO} against its own ratified law. Focus: ${focus}.

Read ${REPO}/law/DIGEST.md first — it is the ratified law as a brief (invariants, authorities, acts, a card per reserved term, refused vocabulary). Open law/*.cue only where exact wording is at stake.

Audit every user-facing string the batch adds or changes — refusal reasons, remedies, usage text, schema comments, generated output:
- refused vocabulary (the digest lists it; note that allow/permit/authorize/approve are banned outright)
- authority relocation: does any sentence give a reserved verb to a subject that does not hold it?
- status laundering: does anything treat evidence as law, a claim as believed, or a processing outcome as a standing change?
- does the code's behavior contradict a ratified statement, in law or in the schema comments it ships?
Report violations with concrete replacement text.`,
  },
  {
    key: 'lane-discipline',
    prompt: `Audit the kernel/client lane split in ${target} on ${REPO}. Focus: ${focus}.

Ratified rules: the kernel performs no inference and never fetches or inspects work-product content — it decides over submitted claims, manifests, and recorded state (T0-3, T0-4 in law/tier0.cue); gates are mechanism, never authority; only the store records; law descends and only evidence ascends (T0-1, T0-2).

Check: does any kernel path now read a tree, poll, or infer? Does any client output imply standing it cannot confer? Is any dev-lane convenience (a test hook, a workshop assumption, a fixture path) shipped in product code? Does anything treat an unratified artifact as law? Report violations with fixes.`,
  },
]

const lenses = a.lenses || DEFAULT_LENSES

const FINDINGS = {
  type: 'object',
  required: ['findings'],
  properties: {
    findings: {
      type: 'array',
      items: {
        type: 'object',
        required: ['title', 'detail', 'severity', 'fix'],
        properties: {
          title: { type: 'string' },
          detail: { type: 'string', description: 'file, symbol, exact repro, observed output' },
          severity: { enum: ['blocking', 'should-fix', 'note'] },
          fix: { type: 'string', description: 'concrete edit or replacement text' },
        },
      },
    },
  },
}

const VERDICT = {
  type: 'object',
  required: ['real', 'reason'],
  properties: {
    real: { type: 'boolean' },
    reason: { type: 'string', description: 'what was reproduced, or why it cannot happen' },
  },
}

const reviewed = await pipeline(
  lenses,
  (l) => agent(l.prompt, { label: `review:${l.key}`, phase: 'Review', schema: FINDINGS }),
  (r, l) =>
    parallel(
      (r.findings || [])
        .filter((f) => f.severity !== 'note')
        .map((f) => () =>
          agent(
            `Adversarially verify this finding against ${REPO}. Try to REFUTE it: reproduce the claimed failure or prove it cannot happen. Run tests and scratch inputs as needed, and clean up anything you write inside the repository. A red for the wrong reason proves nothing, so check that what you reproduce is what the finding actually claims.\n\nFinding: ${JSON.stringify(f)}\n\nreal=true only on reproduction or an airtight argument.`,
            { label: `verify:${f.title.slice(0, 40)}`, phase: 'Verify', schema: VERDICT }
          ).then((v) => ({ ...f, lens: l.key, verdict: v }))
        )
    ).then((verified) => ({
      lens: l.key,
      notes: (r.findings || []).filter((f) => f.severity === 'note'),
      verified: verified.filter(Boolean),
    }))
)

const all = reviewed.filter(Boolean)
const confirmed = all.flatMap((r) => r.verified.filter((f) => f.verdict?.real))
log(`${confirmed.length} confirmed findings (${confirmed.filter((f) => f.severity === 'blocking').length} blocking)`)

return { confirmed, all }
