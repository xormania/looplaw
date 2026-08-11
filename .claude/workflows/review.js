export const meta = {
  name: 'review',
  description: 'Adversarially review a batch: independent lenses find, independent skeptics try to refute',
  // Pure literal, no concatenation: meta is parsed statically, and a
  // joined string is a BinaryExpression the loader refuses. That refusal
  // made this workflow uninvokable until the first attempt to run it.
  whenToUse: 'Before asking for a merge on any batch that changes behavior. Pass args {target, focus, files?, lenses?}. Every blocking defect this repo has found came from a review like this rather than from its own tests. Match depth to risk: a tooling-only batch does not need three adversarial lenses.',
  phases: [
    { title: 'Scope', detail: 'list the changed files so lenses read those, not the repository' },
    { title: 'Review', detail: 'independent lenses over the batch' },
    { title: 'Verify', detail: 'refute or confirm each finding against the built binary' },
  ],
}

const a = args || {}
const target = a.target || 'the current branch (git diff master...HEAD)'
const focus = a.focus || 'the changed code'

// Findings are read by a person deciding what to fix, not by an
// archive. Long detail costs tokens on every finding and is discarded
// wholesale for anything that does not survive verification, so the
// caps are mechanical rather than requested.
const FINDINGS = {
  type: 'object',
  required: ['findings'],
  properties: {
    findings: {
      type: 'array',
      items: {
        type: 'object',
        required: ['title', 'file', 'detail', 'severity', 'fix'],
        properties: {
          title: { type: 'string', maxLength: 90 },
          file: { type: 'string', maxLength: 120, description: 'path, with symbol or line where it helps' },
          detail: {
            type: 'string',
            maxLength: 600,
            description: 'the command run, the output observed, and the failure. No essay: a note is one line.',
          },
          severity: { enum: ['blocking', 'should-fix', 'note'] },
          fix: { type: 'string', maxLength: 400, description: 'the concrete edit or replacement text' },
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
    reason: { type: 'string', maxLength: 500, description: 'what was reproduced, or why it cannot happen' },
  },
}

const SCOPE = {
  type: 'object',
  required: ['files'],
  properties: {
    files: { type: 'array', items: { type: 'string' }, description: 'changed paths, product code first' },
  },
}

// Lenses read the changed files. Without a list they wander the
// repository, reading whole files to decide whether they matter.
let files = a.files
if (!files) {
  const scope = await agent(
    `In this repository, run: git diff --name-only master...HEAD\nReturn the changed paths. Drop anything under testdata/golden (recorded copies of files that already exist). No commentary.`,
    { label: 'scope', phase: 'Scope', schema: SCOPE, effort: 'low' }
  )
  files = scope ? scope.files : []
}
const fileList = files.length ? files.join('\n') : '(unknown — inspect the diff yourself)'
log(`reviewing ${files.length} changed files`)

// Output discipline every lens inherits. Stated once here rather than
// re-argued in each prompt.
const TERSE = `
Output rules, enforced by the schema and by how these are read:
- A finding is a command, the output it produced, and the failure it shows. Not an argument that something looks wrong.
- severity "note" gets ONE line of detail. Notes are never verified, so anything longer is written and then thrown away.
- Do not restate the code, the law, or this prompt back. Do not summarise what you did.
- Report nothing you did not reproduce or cannot argue airtight. An empty findings list is a real answer.
- Clean up anything you write inside the repository; use a temp directory.`

const DEFAULT_LENSES = [
  {
    key: 'correctness',
    prompt: `Hunt correctness bugs in ${target}, in this repository. Focus: ${focus}.

Changed files:
${fileList}

Attack by building inputs under /tmp and running the built binary (go run ./cmd/looplaw ...), not by reading for style. Classes that have actually bitten this repository, in the order they have appeared:
- silent skips — a check that no-ops when a lookup errors, so malformed input passes unexamined
- hash and delimiter forgery — two materially different things comparing equal
- values that satisfy the schema and mean nothing: empty strings, absent regions, self-references
- nondeterminism reaching anything a consumer reads, refusal order included
- inputs the schema permits that no check examines at all
${TERSE}`,
  },
  {
    key: 'law-conformance',
    prompt: `Review ${target} against this project's own ratified law. Focus: ${focus}.

Changed files:
${fileList}

Read dev/DIGEST.md — this project's design basis as a brief (invariants, authorities, acts, a card per reserved term, refused vocabulary). It is 18KB against the corpus's 91KB; open dev/*.cue only for a passage the digest does not settle, and schema/*.cue only for the shape a set must take.

You own the strings; the correctness lens owns behavior. Audit every user-facing string the batch adds or changes — refusal reasons, remedies, usage text, schema comments, generated output — for refused vocabulary, authority relocation (a reserved verb handed to a subject that does not hold it), status laundering (evidence treated as law, a claim treated as believed, processing treated as a standing change), and behavior contradicting a ratified statement. Quote the offending string and give its replacement.
${TERSE}`,
  },
  {
    key: 'lane-discipline',
    prompt: `Audit the kernel/client lane split in ${target}. Focus: ${focus}.

Changed files:
${fileList}

Ratified rules (see dev/DIGEST.md): the kernel performs no inference and never fetches or inspects work-product content — it decides over submitted claims, manifests, and recorded state; gates are mechanism, never authority; only the store records; law descends and only evidence ascends.

Look for kernel paths that read a tree, client output implying standing it cannot confer, dev-lane conveniences shipped in product code, and unratified artifacts treated as law.
${TERSE}`,
  },
]

const lenses = a.lenses || DEFAULT_LENSES

const reviewed = await pipeline(
  lenses,
  (l) => agent(l.prompt, { label: `review:${l.key}`, phase: 'Review', schema: FINDINGS }),
  (r, l) =>
    parallel(
      (r.findings || [])
        .filter((f) => f.severity !== 'note')
        .map((f) => () =>
          agent(
            `Adversarially verify this finding, in this repository. Try to REFUTE it: reproduce the claimed failure or prove it cannot happen. A red for the wrong reason proves nothing, so check that what you reproduce is what the finding actually claims.

Finding: ${JSON.stringify(f)}

real=true only on reproduction or an airtight argument. Answer in at most a few sentences: what you ran, what it printed, and the verdict. Clean up anything you write inside the repository.`,
            { label: `verify:${f.title.slice(0, 40)}`, phase: 'Verify', schema: VERDICT }
          ).then((v) => ({ ...f, lens: l.key, verdict: v }))
        )
    ).then((verified) => ({
      lens: l.key,
      noteCount: (r.findings || []).filter((f) => f.severity === 'note').length,
      notes: (r.findings || []).filter((f) => f.severity === 'note').map((f) => `${f.file}: ${f.title}`),
      verified: verified.filter(Boolean),
    }))
)

const all = reviewed.filter(Boolean)
const checked = all.flatMap((r) => r.verified)
const confirmed = checked.filter((f) => f.verdict?.real)
const refuted = checked.length - confirmed.length

log(`${confirmed.length} confirmed, ${refuted} refuted, ${all.reduce((n, r) => n + r.noteCount, 0)} notes`)

// The full record — every finding, every verdict and its reasoning —
// is already persisted per agent in the run's journal.jsonl. Returning
// it again would pay for the same text twice, so this returns only what
// a decision needs.
return {
  confirmed: confirmed.map((f) => ({
    severity: f.severity,
    lens: f.lens,
    file: f.file,
    title: f.title,
    fix: f.fix,
  })),
  notes: all.flatMap((r) => r.notes),
  counts: {
    confirmed: confirmed.length,
    refuted,
    blocking: confirmed.filter((f) => f.severity === 'blocking').length,
  },
  detail: 'full findings, verdicts, and reasoning: journal.jsonl in this run transcript directory',
}
