// The lexicon — batch 3: the reserved act verbs + the process-vocabulary
// disposition.
//
// Batch 3 RATIFIED: PR #7 merged by xormania 2026-08-09 (the recorded act
// per law/README.md; flip performed in batch 5 — protocol debt noted and
// discharged). Amendments only by new PR.
//
// SCOPE OF REGISTER (per the collision-resistant lexicon method §9): this
// lexicon binds looplaw's LAW ARTIFACTS and product prose — everything an
// agent consumes as authoritative. Dev-lane documents (CONTRIBUTING.md,
// proj/ notes) are out of scope; design conversation is unpoliced.
// Lane: models the product; 'aa' is the per-deployment accountable
// authority role, never a person. Ratification protocol: law/README.md.
//
// RENDERING RULE for violation examples: a violation string is shown only
// to ban it. Render it only with a '✗ banned:' prefix and its rewrite
// with '✓ write:'; never quote a violation bare — bare, it reads as a
// usage example.
//
// Entry texts were drafted against cold prior-probe measurements and
// adversarially verified (evidence: proj/law-batch-3-evidence.md).
package law

#Tier: "CANON" | "REVIEW" | "QUALIFY" | "BANNED"

// A canonical entry: the unit of specification. Field semantics follow
// the method's entry anatomy (§6); every 'not' item ends in an
// executable redirect (§7), ordered by danger.
#Entry: {
	term:       string
	tier:       #Tier
	definition: string // authority placement + lifecycle position explicit
	authority:  "aa" | "recording" | "none"
	related: [...string]
	aliases: [...string] // exhaustive; empty = none allowed
	not: [...{misreading: string, write_instead: string}]
	collision: string // the strongest outside prior, as the misreading to defuse
	docs:      string // documentation-register phrasing
	prompts:   string // agent-prompt register: self-contained, pasteable verbatim
	violation: string // one banned sentence, shown only to ban it
	rewrite:   string // its corrected form — the pair teaches the repair
	status:    #Status
	trigger?:  string
}

lexicon: [T=string]: #Entry & {term: T}

// Terms referenced by entries but deliberately not yet coined — reserved
// future vocabulary, listed so references to them are declared, not
// dangling. Coining any of these is a ratification, not a drafting act.
reserved_future: [
	"claim, receipt, admission, version — the record kinds (entries pending; the admission kind is flagged to the aa as undefined in ratified law)",
	"submission — what a party sends toward the gates (entry pending)",
	"gap — the differ's disequilibrium unit (entry pending)",
	"standing grant — fixed compound of the grant entry; the ratified instrument",
	"the commission family (incl. run-time removal) — no-coin zone until ratified",
]

lexicon: {
	ratify: {
		tier: "CANON"
		definition: "The aa's recorded act by which a draft — goal contract, lexicon entry, registry change, Tier 0 invariant, or standing grant — becomes law; the only law-making act, performed by the accountable authority alone (one human per deployment, singular, non-delegable). Before it, a draft binds nothing; after it, the ratified version controls until amended or withdrawn."
		authority: "aa"
		related: ["amend", "withdraw", "defer", "grant", "record"]
		aliases: []
		not: [
			{
				misreading:    "the agency-law retroactive sense: ratification reaching back to authorize or cure an act already executed before the recorded act"
				write_instead: "standing runs forward from the recorded act only — write 'binds from ratification onward; the earlier execution remains a claim, standing unchanged' (see record)"
			},
			{
				misreading:    "self-ratification: the executing actor conferring standing on its own completed work by ratifying it"
				write_instead: "write 'submitted as a claim; binds nothing until the aa ratifies the draft' (see record)"
			},
			{
				misreading:    "the dev-tool approve-and-commit reading: a gate, pipeline, merge, or store operation that flips a draft from provisional to committed state"
				write_instead: "gates and the store are mechanism — for the mechanical entry of a submission write 'record'; for gate behavior write 'the gates enforced the aa's ratification'"
			},
			{
				misreading:    "loose endorsement: any sign-off, review approval, or acceptance called ratification"
				write_instead: "write the non-authoritative verb — propose, submit, declare — and for a party's assertion write 'claim'"
			},
			{
				misreading:    "revision by re-approval: 'ratifying' a change to law that is already ratified"
				write_instead: "for a change to ratified law write 'amend' (see amend)"
			},
		]
		collision: "The agency-law retroactive-validation prior: readers take 'ratify' as license to legitimize an already-executed unauthorized act after the fact — and as an act the executing agent may perform on its own behalf. Here ratification is exclusively the aa's forward-acting act on a draft: it confers standing from the recorded act onward and cures nothing done before it."
		docs:      "'ratified by the aa' — the recorded act; until it, a draft binds nothing. (Git era: the aa's ratification merge is the recorded act's front.)"
		prompts:   "ratify — reserved term. Only the aa (the deployment's accountable authority: one human, singular, non-delegable) ratifies. Ratification is the recorded act by which a draft (goal contract, lexicon entry, registry change, Tier 0 invariant, standing grant) becomes law — the only law-making act. Subject rule: no other subject ever takes 'ratify'; the gates enforce ratified outcomes and the store records them — neither ratifies. Never retroactive, never self-serve: an agent's already-executed change is a claim, standing unchanged, until the aa ratifies the draft. Write 'record' for a submission's mechanical entry, 'amend' for changing ratified law, 'claim' for anything a party asserts."
		violation: "The agent ratified its own registry change after the fact."
		rewrite:   "The agent submitted the registry change as a claim; the change binds nothing until the aa ratifies the draft."
		status:    "ratified"
	}

	amend: {
		tier: "CANON"
		definition: "The accountable authority's act that replaces a ratified law with a successor version: the predecessor is archived — never deleted — and the successor becomes the sole live version (never-fork). Amend operates only downstream of ratify, on law that already holds ratified standing, and is the only act by which ratified law gains a successor version."
		authority: "aa"
		related: ["ratify", "withdraw", "record"]
		aliases: []
		not: [
			{
				misreading:    "the 'git commit --amend' prior: a quiet in-place rewrite that folds new text into the existing law and destroys the prior state"
				write_instead: "write 'amend to a successor version; the predecessor is archived' — the predecessor persists as a version record in the store under the record act; nothing is ever overwritten or deleted"
			},
			{
				misreading:    "the solo-author prior: an agent or component amending law unilaterally, with no authorized process"
				write_instead: "write 'submits an amendment draft' for every non-aa party — drafts enter through the gates as claims by the record act; only the accountable authority amends"
			},
			{
				misreading:    "the statute-book prior: the original instrument remains in force in amended form, the change layered onto a still-live original"
				write_instead: "write 'replaced by the successor version' — never-fork: exactly one live version exists; the predecessor is archived and never live"
			},
			{
				misreading:    "amend as retirement: removing a clause or goal with nothing in its place"
				write_instead: "write 'withdraw' — retirement without replacement is the withdraw act"
			},
			{
				misreading:    "amend as first standing: bringing a new draft into law"
				write_instead: "write 'ratify' — a draft's first standing arises only by the ratify act"
			},
		]
		collision: "The 'git commit --amend' prior: dev-context readers — agent readers above all — resolve 'amend' to a quiet, unilateral, in-place rewrite of the most recent item that destroys prior state; here amend is the accountable authority's act on any ratified law, producing a successor version while the predecessor is archived, never deleted."
		docs:      "amended by the accountable authority; the predecessor is archived, and the successor is the sole live version."
		prompts:   "amend = the accountable authority (aa — one human per deployment, singular, non-delegable) replaces a ratified law with a successor version; the predecessor is archived in the store, never deleted, and exactly one version is live (never-fork). Only the aa amends; agents and components submit amendment drafts, entered as claims by the record act. Never read as 'git commit --amend': nothing is rewritten in place, and any ratified law — not only the most recent item — can be amended. Retirement without a successor is withdraw; a draft's first standing is ratify."
		violation: "The pipeline amended the ratified contract, overwriting v3 with the corrected text."
		rewrite:   "The pipeline submitted an amendment draft; the accountable authority amended the contract to v4 — v3 is archived, never overwritten."
		status:    "ratified"
	}

	withdraw: {
		tier: "CANON"
		definition: "The aa's design-time act that retires a ratified law clause or goal without replacement. Before the act the clause is live ratified law; after it, the clause holds withdrawn standing — archived, never deleted, with no successor version (run-time removal is never this act; it is reserved to the commission family, not yet ratified)."
		authority: "aa"
		related: ["amend", "defer", "ratify", "record"]
		aliases: ["withdrawal — noun form; allowed only in 'withdrawal proposal' and 'proposed withdrawal', both registers"]
		not: [
			{
				misreading:    "the submitter-retracts prior: the party that submitted a claim, goal, or advisory takes its own submission back, as one withdraws an application or an offer"
				write_instead: "write 'proposed withdrawal' (a submission through the gates like any other) — parties propose, only the aa withdraws; the original submission stays recorded"
			},
			{
				misreading:    "the extraction prior: moving funds or value out of a balance, account, or escrow to a destination — an irreversible transfer"
				write_instead: "nothing — looplaw holds no balances and withdraw moves no value; a sentence that needs value movement is outside looplaw's vocabulary — flag the naming gap to the aa and continue writing without naming it"
			},
			{
				misreading:    "the run-time removal prior: taking a running component, service, or capability out of operation"
				write_instead: "nothing yet — run-time removal is reserved to the commission family and has no ratified name; flag the naming gap to the aa and continue without naming the act. Never 'withdraw'"
			},
			{
				misreading:    "the swap prior: 'withdraw and replace' — retiring a clause as the first half of installing a successor"
				write_instead: "write 'amend' — replacement is one act with the predecessor archived; withdraw is the exit without replacement"
			},
			{
				misreading:    "the deletion prior: the withdrawn clause is erased from the store"
				write_instead: "write 'archived' — the store is append-only; withdrawal appends a record, and the retired clause is archived, never deleted"
			},
		]
		collision: "The ATM prior: cold, 'withdraw' reads as extracting funds out of a balance to a destination — an irreversible money-moving command, performed by whoever owns the account. Here no value exists and nothing moves anywhere: withdraw is the aa's design-time act retiring a law clause or goal without replacement, and the retired clause remains archived in the store."
		docs:      "withdrawn by the aa — retired without replacement and archived."
		prompts:   "withdraw (looplaw reserved term): the accountable authority's design-time act that retires a ratified law clause or goal without replacement; the retired clause is archived, never deleted. Only the accountable authority — one human per deployment, singular, non-delegable — withdraws; every other party or component may only propose withdrawal, and the withdrawal proposal enters through the gates as a submission like any other. Withdraw moves no funds or value (looplaw has no balances), does not retract or erase any recorded submission (the store is append-only), is not run-time removal (looplaw names no run-time removal act yet: say nothing and flag the naming gap to the accountable authority), is not replacement by a new law version (write: amend), and is not parking pending a trigger (write: defer)."
		violation: "The absorber withdrew the goal once costs blew past the estimate."
		rewrite:   "The absorber's withdrawal proposal entered through the gates citing cost; the aa withdrew the goal, which is archived without replacement."
		status:    "ratified"
		trigger:   "if the process-vocabulary lexicon sweep dispositions 'withdraw' differently, rename by amendment"
	}

	defer: {
		tier: "CANON"
		definition: "The accountable authority's recorded act that parks an open gap or clause with three named fields — destination, authority (a registry authority id), and trigger — where the trigger is monitored. Between the defer and its trigger the item is parked, not retired: its standing is exactly what it was — a gap or draft gains none, a ratified clause loses none — and it is resolved only by a later act of the accountable authority."
		authority: "aa"
		related: ["ratify", "amend", "withdraw", "record"]
		aliases: ["deferral — noun form of the act; allowed only in 'deferral record' and 'proposed deferral', both registers"]
		not: [
			{
				misreading:    "a self-service postponement — the scheduler prior in which any actor (agent, component, runtime) parks an item on its own authority as a low-privilege, reversible act"
				write_instead: "only the accountable authority defers. For any other subject write 'proposed deferral' or 'submitted the gap for deferral' — propose, submit, and report (the read path's verb for surfacing gaps) are the non-authoritative verbs; a proposal parks nothing"
			},
			{
				misreading:    "'defer to' — yielding judgment or transferring decision authority to another party (a counterparty, a tool, a caller)"
				write_instead: "defer never takes 'to' plus anything; authority is non-delegable and never moves. Write 'submit to the accountable authority for decision'. A destination is written as the record's field ('destination: the v2 queue'), never as 'deferred to X' — the 'defer to' surface form is banned outright, party or not"
			},
			{
				misreading:    "cancellation or waiver — reading a deferred clause as dropped, its obligation gone"
				write_instead: "a defer retires nothing. For retirement without replacement write 'withdraw' (also an aa-only act)"
			},
			{
				misreading:    "deferred execution — the deferred-promise prior in which the parked item runs automatically later"
				write_instead: "nothing executes on a trigger; the deferral's resolution is a later act of the accountable authority. Write 'the trigger fired; the deferral is before the aa'. Go's defer statement in source code is outside lexicon scope: code is not law prose; the keyword never licenses the reserved term and the reserved term never describes it"
			},
			{
				misreading:    "a backlog entry — parking with no destination, authority, or trigger recorded"
				write_instead: "nothing defers into the void: a defer missing any of the three fields is not a defer. Record all three, or leave it written as an open gap (the differ reports it)"
			},
		]
		collision: "The scheduler prior: cold, 'defer' reads as a low-privilege postponement the current actor performs on its own authority (deferred loading, snoozing a task, pushing an item down a backlog) — so an agent parks a contested item itself, unrecorded, and believes it acted correctly; here defer is the accountable authority's recorded act whose record must name destination, authority, and trigger, with the trigger monitored. (The strongest misreading of the bare verb; 'defer to' ranks second and is prepositionally detectable.)"
		docs:      "deferred by the accountable authority; the deferral record names its destination, authority, and trigger, and the trigger is monitored until it fires. Never agentless: the subject or by-phrase must name the accountable authority — 'the clause was deferred', with no by-phrase, is a violation."
		prompts:   "defer (reserved term): the accountable authority's recorded act that parks a gap or clause with three fields — destination, authority, trigger; the trigger is monitored. Only the accountable authority defers; agents and components report gaps and propose or submit deferral, never perform it. 'Defer to <anything>' is banned — authority never transfers, and a destination is written as the record's field, never as 'deferred to X'. A defer cancels nothing — to retire a clause without replacement, the term is withdraw. Nothing defers into the void: no destination, authority, and trigger recorded means no defer happened. Passive without the by-phrase is banned: write 'deferred by the accountable authority', never bare 'was deferred'."
		violation: "The differ deferred the unresolved clause to the backlog."
		rewrite:   "The differ reported the gap; the accountable authority deferred the clause, recording its destination, authority, and trigger."
		status:    "ratified"
		trigger:   "on-fire behavior and the resolution set bind when ratified in the registry; until then the entry states only that resolution is a later aa act"
	}

	grant: {
		tier: "CANON"
		definition: "The aa's class-licensing act: the aa ratifies a standing grant once — ratification, not grant, is the law-making act — and from then on the kernel gates check each submission for class membership and the record act enters members with no further per-submission aa act. A draft grant licenses nothing until ratified, and accountability never moves with it — the aa remains accountable for every entry the grant licenses. 'Standing grant' is this entry's fixed compound for the ratified instrument; no separate entry."
		authority: "aa"
		related: ["ratify", "record"]
		aliases: []
		not: [
			{
				misreading:    "the SQL/IAM prior: grant as a live access-control command any sufficiently privileged caller can invoke — letting a component or agent grant itself or others entry"
				write_instead: "only the aa grants. For a component seeking entry, write 'submitted under standing grant <id>' (see record); where no grant covers the case, write 'flagged to the aa as a candidate standing grant' — never coin the permission"
			},
			{
				misreading:    "authority relocation to mechanism: reading the gates' per-submission membership check as the gates granting access"
				write_instead: "write 'the gates checked membership under standing grant <id>; the store recorded the submission' — gates enforce grants and originate nothing"
			},
			{
				misreading:    "record-of-promise as live license: reading a recorded claim that access was promised or is due as itself a grant"
				write_instead: "write 'recorded claim' (see record) until the aa ratifies a standing grant; no record licenses entry on its own — claims are recorded, never believed"
			},
			{
				misreading:    "delegation: reading a grant as moving authority or accountability onto the granted class or its members"
				write_instead: "write 'licensed under standing grant <id> — accountability remains with the aa'; the aa role is singular and non-delegable"
			},
			{
				misreading:    "the OAuth prior: grant as a token flow or bearer credential whose possession confers access at run time"
				write_instead: "write 'standing grant' — a ratified law-side class definition; nothing is issued or carried, and membership is checked per submission by the gates"
			},
		]
		collision: "The access-control prior (SQL GRANT, IAM role grants) reads grant as a self-executing mutation of live permissions that any sufficiently privileged caller may invoke — silently blurring 'X was promised Y' into 'give X the power now' and inviting self-escalation; here grant is the aa's ratify-once act over a class, the gates only check membership per submission, and no component or agent can grant anything, least of all to itself."
		docs:      "granted by the aa: a standing grant is ratified once and licenses a class of automatic entries into the record; the gates check membership per submission; accountability never moves with the grant."
		prompts:   "grant — reserved verb; its only subject is the aa (the deployment's accountable authority: one human, singular, non-delegable). A grant is ratified once as a standing grant licensing a class of automatic entries into the record (e.g. guest-mode working sets); thereafter the kernel gates check each submission for class membership and the store records members — the gates never grant, components and agents never grant, and nothing may grant to itself. Accountability never moves with a grant: the aa remains accountable for every entry it licenses. Never write 'the gate/server/agent granted access'; write 'recorded under standing grant <id> (gates checked membership)'. If no standing grant covers the case, do not grant — write 'flagged to the aa as a candidate standing grant'. A recorded claim that access was promised licenses nothing — claims are recorded, never believed. Never rotate synonyms: 'authorized', 'permitted', 'allowed', 'approved' are not substitutes for 'recorded under standing grant <id>' — an unlisted synonym is a violation."
		violation: "The gate granted the guest-mode working set access to the store."
		rewrite:   "The aa's standing grant licenses guest-mode working sets; the gates checked membership and the store recorded the submission."
		status:    "ratified"
		trigger:   "if the grant/ratify specialization proves confusing in use, amend the registry to fold grant into ratify plus a standing-grant law kind"
	}

	record: {
		tier: "CANON"
		definition: "The privileged act of the recording authority — the store: a submission that has passed the kernel gates becomes a record (a claim, receipt, admission, or version — the record kinds), held append-only by the store. Recording settles that a thing was said — never that it is true; standing on law arises only by the aa's separate acts (ratify, amend, withdraw, defer, grant)."
		authority: "recording"
		related: ["ratify", "grant", "admit"]
		aliases: []
		not: [
			{
				misreading:    "recording as enactment — the deed-recordation prior, where 'record X' makes X official, binding, or law"
				write_instead: "for standing on law, write 'the aa ratifies'; keep 'record' for the store's act, which settles only that X was said"
			},
			{
				misreading:    "recorded as true — reading 'recorded' as verified, established, or believed content"
				write_instead: "write 'recorded claim' — said, unverified; claims are recorded, never believed, and 'verify' is a read path that changes no standing"
			},
			{
				misreading:    "anyone records — giving the record verb to the gates, server, absorber, or any client with write access"
				write_instead: "write 'submit' for clients and components, and 'the submission passed the gates and the store recorded it' — gates are mechanism, never authority; only the store records"
			},
			{
				misreading:    "record as capture — 'record the session' read as starting screen, audio, log, or trace capture for later replay"
				write_instead: "write 'capture (non-authoritative)'; capture is not a looplaw act, and nothing captured is a record until submitted and gate-passed"
			},
			{
				misreading:    "record as any stored row, file, or struct — the database-record prior"
				write_instead: "name the record kind — claim, receipt, admission, or version — for store content; for other stored data write 'data (non-authoritative)' or the mechanism's own noun (row, file)"
			},
		]
		collision: "The legal-recordation prior: 'record X' read as an enactment with binding force — an agent that records a contract, ruling, or obligation believes recording made it official or true. Here recording is transcription with custody: it settles only that X was said; force and standing arise solely by the aa's ratify."
		docs:      "recorded by the store (said, not settled); a record exists only after the gates pass its submission."
		prompts:   "record (reserved, looplaw): the store — the single recording authority — turns a gate-passed submission into a record (claim, receipt, admission, or version), append-only. Recording settles that a thing was said, never that it is true and never that it is law. Subject rule: only the store records; clients and components submit; gates pass or refuse with remedy and never record. A recorded claim stays a claim — making it law is a different act, ratify, held solely by the accountable authority (aa: one human per deployment, never a component), and believing it is never licensed. Do not use 'record' for session/screen/log capture — write 'capture (non-authoritative)'."
		violation: "Once the server records the ruling, it becomes binding law."
		rewrite:   "The ruling was submitted and, having passed the gates, the store recorded it as a claim — said, not settled; it becomes law only when the aa ratifies it."
		status:    "ratified"
	}

	admit: {
		tier: "REVIEW"
		definition: "Ruled not an act: no component performs 'admit', and no new act may take the verb — the verb is reserved. 'Admission' is licensed only in the ratified registry's own uses — the admission record kind in the recording authority's holds, and a grant's licensed class of automatic admissions — records produced, like every record, by the record act executing behind the kernel gates; recorded, never believed. The event is the record act: write record."
		authority: "none"
		related: ["record", "ratify", "grant"]
		aliases: ["admission — noun only, and only in the ratified registry's own uses: the admission record kind, and a grant's 'automatic admissions'; bare 'admission' outside those forms is a QUALIFY finding — write 'record', or name the admission-kind record"]
		not: [
			{
				misreading:    "not a gate act — the admission-control prior (Kubernetes admission controllers, queue/cluster admission) reads 'admit' as the gates' own authoritative act of letting a submission in, installing the gates as an authority"
				write_instead: "gates are mechanism, never authority: they verify preconditions and refuse with remedy, originating nothing. Write record — 'the submission passed the gates; the store recorded it'"
			},
			{
				misreading:    "not a confession — the legal prior reads 'admit' as a party conceding a fault or fact as true, binding the admitting party"
				write_instead: "claims are recorded, never believed. Write claim — 'the party's claim, recorded' — and never generate any concession of fault, truth, or liability for any party"
			},
			{
				misreading:    "not entry into force — the membership prior (admit a student, admit to the bar) reads 'admit the claim' as conferring standing: validity, membership, or force of law"
				write_instead: "recording confers no standing of truth or law. For law standing write ratify — the accountable authority's act, the only act that makes law; for mere entry into the store write record"
			},
			{
				misreading:    "not a free verb — the coinage prior treats 'admit' as available for some new privileged act ('the X admits Y')"
				write_instead: "admission is exhausted by the record act; no new act may take this verb. If a task needs an act the lexicon does not name, flag the naming gap to the accountable authority and continue without naming it"
			},
		]
		collision: "The admission-control prior: a reader — dev-tool readers above all — assumes 'admit' is a gatekeeper's own authoritative act of letting an entity past a boundary, and so reads the kernel gates as the admitting authority. Here nothing performs 'admit': what looks like admitting is the record act (recording authority: the store) executing behind gates that verify and refuse, originating nothing."
		docs:      "prefer 'the store recorded it (behind the gates)'; never 'the gates admitted it', and never 'admit' for conferring validity or standing — that is ratify, and it is the accountable authority's alone."
		prompts:   "admit is not an act in this system. Nothing admits: the kernel gates verify a submission's preconditions and refuse with remedy — they hold no authority and originate nothing — and the store (the recording authority) records what passes the gates; that record act is all 'admission' ever names. Never write 'the gates admit'; never draft an admission of fault, truth, or liability on behalf of any party — claims are recorded, never believed; never read 'admit' as conferring validity, membership, or force of law — only the accountable authority's ratify act makes law. Write instead: 'the submission passed the gates; the store recorded it.' If a task seems to need an 'admit' act the lexicon does not name, flag the naming gap to the accountable authority and continue without naming it."
		violation: "The gates admitted the claim into the record."
		rewrite:   "The submission passed the gates; the store recorded it as a claim — recorded, not believed."
		status:    "ratified"
		trigger:   "the admission record kind is flagged to the aa as undefined in ratified law; this entry re-opens when the record kinds are ratified"
	}
}

// Process-vocabulary disposition (the mandatory sweep: the family pairs
// the strongest outside priors with acute meta-collision for agent
// readers). Danger states what a reader will DO, never taste. Every ban
// ships a replacement or an explicit 'say nothing'.
#Disposition: {
	term:     string
	tier:     "BANNED" | "QUALIFY" | "REVIEW"
	danger:   string
	instead:  string
	status:   #Status
	trigger?: string
}

process_vocab: [T=string]: #Disposition & {term: T}

process_vocab: {
	commit: {
		tier:    "BANNED"
		danger:  "a coding agent reads its own first-person git act and/or the database-commit prior; fuses git history with the record act and writes 'the pipeline committed the record'"
		instead: "'record' (the store's act); git mechanics are named only in dev-lane documents"
		status:  "ratified"
		trigger: "registry.authorities.recording.holds says 'commit here' (ratified before this row); filed as a divergence — align at the next registry amendment"
	}
	merge: {
		tier:    "QUALIFY"
		danger:  "first-person git operation to agent readers; bare use invites authority relocation ('the agent merged the law')"
		instead: "bare banned; allowed only in the fixed compound 'ratification merge' — always the accountable authority's, anchored in the ratify entry's docs register"
		status:  "ratified"
	}
	push: {
		tier:    "BANNED"
		danger:  "a coding agent reads its own first-person git act and infers that moving work to a remote changed standing"
		instead: "say nothing; name the dev-lane act in dev-lane documents only"
		status:  "ratified"
	}
	build: {
		tier:    "BANNED"
		danger:  "a reader infers a lifecycle stage that confers standing and treats a green build as advancement"
		instead: "say nothing (process/run-time acts await the commission family)"
		status:  "ratified"
	}
	deploy: {
		tier:    "BANNED"
		danger:  "a reader will assume live standing changed, when looplaw's law defines no design-to-run crossing"
		instead: "say nothing (no-coin zone until the commission family binds)"
		status:  "ratified"
		trigger: "the commission family entering the registry reopens this row"
	}
	release: {
		tier:    "BANNED"
		danger:  "a reader fuses 'make public' with 'confer standing' and reports both when one (at most) happened"
		instead: "say nothing"
		status:  "ratified"
	}
	publish: {
		tier:    "BANNED"
		danger:  "same fusion class as release — a reader treats visibility as standing"
		instead: "say nothing"
		status:  "ratified"
	}
	ship: {
		tier:    "BANNED"
		danger:  "same fusion class as release, plus a completion connotation a reader will launder into status"
		instead: "say nothing"
		status:  "ratified"
	}
	rollback: {
		tier:    "BANNED"
		danger:  "a reader will look for an undo that must not exist and may attempt history rewrite — the ledger is append-only and amendments never erase"
		instead: "'amend' (new version) or 'withdraw'"
		status:  "ratified"
	}
	version: {
		tier:    "QUALIFY"
		danger:  "bare noun invites the git/semver prior and blurs which thing is versioned"
		instead: "qualified forms: 'law version', 'record version', 'live version', 'predecessor version', 'successor version'"
		status:  "ratified"
	}
	environment: {
		tier:    "QUALIFY"
		danger:  "a reader collapses runtime environment, environment variables, and deployment environment into one referent and reasons about the wrong one"
		instead: "qualified forms: 'execution environment', 'toolchain pins'"
		status:  "ratified"
	}
	archive: {
		tier:    "REVIEW"
		danger:  "the amend and withdraw acts say predecessors are archived, but no archive authority exists in the registry — using 'archive' as an act invites authority relocation onto the store"
		instead: "the participle 'archived' as description only (entries may describe outcomes as 'archived, never deleted'); the act and noun await an archive authority"
		status:  "ratified"
		trigger: "binds (full entry + authority) when an archive authority enters the registry"
	}
}

// Access-vocabulary bans (recorded per the grant entry's admission: the
// rejected synonyms must bind, not sit in commentary).
forbidden_vocab: [T=string]: #Disposition & {term: T}

forbidden_vocab: {
	authorize: {
		tier:    "BANNED"
		danger:  "reimports the access-control prior — a reader treats entry into the record as a live permission any privileged caller can confer"
		instead: "'recorded under standing grant <id>'"
		status:  "ratified"
	}
	permit: {
		tier:    "BANNED"
		danger:  "same access-control prior as authorize"
		instead: "'recorded under standing grant <id>'"
		status:  "ratified"
	}
	allow: {
		tier:    "BANNED"
		danger:  "same access-control prior as authorize; also the weakest and most tempting synonym under paraphrase pressure"
		instead: "'recorded under standing grant <id>'"
		status:  "ratified"
	}
	approve: {
		tier:    "BANNED"
		danger:  "a reader treats a review sign-off as a standing change — status laundering; only recorded acts change standing"
		instead: "'ratified by the aa' where law standing is meant; otherwise the non-authoritative verbs (propose, submit, declare)"
		status:  "ratified"
	}
	"component-subject admit": {
		tier:    "BANNED"
		danger:  "the phrasings '<any registry component> admits/admitted' and 'admit(ted) into force/law' install the gates or store as an admitting authority — hard-fail phrasings, not judgment calls"
		instead: "'the submission passed the gates; the store recorded it'"
		status:  "ratified"
	}
}
