// ATTACK FIXTURE — a set that MUST be refused.
//
// Preserved from an adversarial review round rather than discarded:
// every blocking defect this project has found came from a hand-built
// attack, and each one is kept here so the defect can never return
// silently and so the next reviewer starts from what has already been
// tried. Expected refusals are declared in index.cue beside this file.
//
// Round 3 (absorber): a derivation naming a source outside the baseline — an unbaselined source cannot go stale, so it proves nothing.
// An absorbed view of the lending scope: what a party claims the scope
// currently is, with provenance recording what each statement was
// derived from. Evidence, never law — the goal-law it is diffed against
// lives elsewhere and is ratified separately.
subject:        "lend-library"
schema_version: "0"

registry: {
	librarian: {
		name:           "the librarian"
		note:           "holds the lending authority as the scope implements it"
		authority_free: false
	}
	borrower: {
		name:           "the borrower"
		note:           "deliberately authority-free: requests and returns"
		authority_free: true
	}
}

invariants: {
	"L-1": {
		text:      "Every loan is recorded; no book leaves the building on an unrecorded loan."
		rationale: "absorbed from the scope's recording path"
	}
}

lexicon: {
	loan: {
		tier:       "CANON"
		definition: "The recorded standing created by the librarian's lend act: one book, one borrower, one due date."
		authority:  "librarian"
		related: []
		aliases: []
		not: [
			{
				misreading:    "a loan as the physical book"
				write_instead: "the loan is the recorded standing; write 'return the book; the librarian retires the loan'"
			},
		]
		collision: "The finance prior: a loan as money owed at interest; here nothing accrues."
		docs:      "a loan, recorded by the librarian"
		prompts:   "loan (lend-library reserved term): the recorded standing created only by the librarian's lend act."
		violation: "The borrower extended the loan by keeping the book."
		rewrite:   "The borrower requested renewal; the librarian's lend act created the successor loan."
		status:    "proposed"
	}
}

contracts: {
	"C-LEND-1": {
		name: "the lending contract"
		parties: {
			client:   "borrower"
			supplier: "librarian"
		}
		acts: ["lend"]
		preconditions: {
			"P-1": {text: "The borrower's membership is in good standing, verifiable from the member records at submission."}
		}
		guarantees: {
			"G-1": {text: "A loan exists naming the book, the borrower, and the due date.", records: "the loan record"}
		}
		invariants_local: {}
		cites: ["L-1"]
		blame: [
			{violation_class: "lending an already-lent book", at_fault: "librarian", evidence: "the loan records at lending time"},
		]
		status: "proposed"
	}
	"C-RETURN-1": {
		name: "the return contract"
		parties: {
			client:   "borrower"
			supplier: "librarian"
		}
		acts: ["return"]
		preconditions: {
			"P-1": {text: "A live loan names this borrower and this book, verifiable from the loan records."}
		}
		guarantees: {
			"G-1": {text: "The loan is retired and the retirement is recorded; the book is lendable again.", records: "the return record"}
		}
		invariants_local: {}
		cites: ["L-1"]
		blame: [
			{violation_class: "late return", at_fault: "borrower", evidence: "the loan record's due date against the return record's date"},
		]
		status: "proposed"
	}
}

experience: {}
experience_declared_absent: true

provenance: {
	scope: "scope"
	sources: {
		"lending.go": "19e886f2113201d5ff5822926fbf38e945ba9409b03079b802b57342fdac018f"
		"returning.go": "d218acc93e6bdf142f6eb38f216c26425c02345f6dbb209af87d4dca60170c53"
	}
	derivations: {
		"C-LEND-1":       ["lending.go"]
		"C-LEND-1/G-1":   ["lending.go"]
		"C-RETURN-1":     ["phantom.go"]
		"C-RETURN-1/G-1": ["returning.go"]
	}
}
