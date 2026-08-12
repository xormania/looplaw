// ATTACK — a default reached through a let binding.
//
// The open-value walk builds a path through fields, and a let states no
// field, so it was walked past without being examined: this set passed
// the check written to refuse exactly the value it carries. Found by
// adversarial review of the batch that added that check.
// Fixture zero: a target project's trinity set — a tiny lending library.
// Green by construction; the mutation tests derive every red from it.
let chosen = *"lend-library" | "other-library"
subject:        chosen
schema_version: "0"

registry: {
	librarian: {
		name:           "the librarian"
		note:           "holds the lending authority: lend and return are its acts"
		authority_free: false
	}
	borrower: {
		name:           "the borrower"
		note:           "deliberately authority-free: proposes, requests, returns — holds nothing"
		authority_free: true
	}
	desk: {
		name:           "the front desk"
		note:           "interior party: holds the attestation authority for standing"
		authority_free: false
	}
}

invariants: {
	"L-1": {
		text:      "Every loan is recorded; no book leaves the building on an unrecorded loan."
		rationale: "no silent transitions — an unrecorded loan is unverifiable and unblameable"
	}
}

lexicon: {
	loan: {
		tier:       "CANON"
		definition: "The recorded standing created by the librarian's lend act: one book, one borrower, one due date. Only the lend act creates a loan; return retires it."
		authority:  "librarian"
		related: ["due date"]
		aliases: []
		not: [
			{
				misreading:    "a loan as the physical book — 'return the loan' read as handing over any copy"
				write_instead: "the loan is the recorded standing, not the object; write 'return the book; the librarian retires the loan'"
			},
		]
		collision:  "The finance prior: a loan as money owed at interest; here nothing accrues and the standing is possession, not debt."
		docs:       "a loan, recorded by the librarian"
		prompts:    "loan (lend-library reserved term): the recorded standing created only by the librarian's lend act — one book, one borrower, one due date. Not the physical book; not a debt. The borrower never creates or retires a loan."
		violation:  "The borrower extended the loan by keeping the book."
		rewrite:    "The borrower requested renewal; the librarian's lend act created the successor loan."
		status:     "ratified"
	}
	"due date": {
		tier:       "CANON"
		definition: "The date recorded on a loan by the lend act after which the borrower is at fault; fixed at lending, changed only by a new lend act (renewal)."
		authority:  "librarian"
		related: ["loan"]
		aliases: []
		not: [
			{
				misreading:    "a soft target the borrower may move by intention or apology"
				write_instead: "the due date moves only by a new lend act; write 'requested renewal'"
			},
		]
		collision:  "The calendar-app prior: a reschedulable reminder owned by whoever holds the calendar; here it is a recorded fact only the librarian's act changes."
		docs:       "the due date recorded on the loan"
		prompts:    "due date (lend-library reserved term): recorded on the loan at lending; only a new lend act (renewal) changes it. The borrower requests; the librarian lends."
		violation:  "The borrower moved the due date to next month."
		rewrite:    "The borrower requested renewal; the librarian lent again, and the successor loan records the new due date."
		status:     "ratified"
	}
	overdue: {
		tier:       "QUALIFY"
		definition: "Bare description of a loan past its due date: a state derived from recorded dates, owned by no party — standing changes only by the return act or a renewal."
		authority:  "none"
		related: ["loan", "due date"]
		aliases: []
		not: [
			{
				misreading:    "overdue as an act, or as a fault finding in itself"
				write_instead: "write 'overdue loan' (the qualified form); fault is adjudicated under the return contract's blame clause, from the records"
			},
		]
		collision:  "The collections prior: 'overdue' as an escalation state someone triggers; here it is a description derived from recorded dates, triggered by nobody."
		docs:       "an overdue loan (qualified; derived from the records)"
		prompts:    "overdue (lend-library, QUALIFY): bare 'overdue' is banned — write 'overdue loan'. A derived description from recorded dates, owned by no party; never an act, and it assigns no fault by itself."
		violation:  "The librarian declared the book overdue and fined the borrower."
		rewrite:    "The loan record's due date passed with no return record; the overdue loan's fault question is adjudicated under the return contract's blame clause."
		status:     "ratified"
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
			"P-2": {text: "The requested book carries no live loan, verifiable from the loan records."}
		}
		guarantees: {
			"G-1": {text: "A loan exists naming the book, the borrower, and the due date.", records: "the loan record"}
		}
		invariants_local: {}
		cites: ["L-1"]
		blame: [
			{violation_class: "lending an already-lent book", at_fault: "librarian", evidence: "the loan records at lending time"},
			{violation_class: "borrowing in bad standing", at_fault: "borrower", evidence: "the member records at submission"},
		]
		status: "ratified"
		interior: {
			children: ["C-STANDING-1", "C-ISSUE-1"]
			wires: [
				{from: {child: "C-STANDING-1", guarantee: "G-1"}, to: {child: "C-ISSUE-1", precondition: "P-1"}},
			]
			presents: {
				"G-1": {child: "C-ISSUE-1", guarantee: "G-1"}
			}
		}
	}
	"C-STANDING-1": {
		name: "the standing-attestation contract"
		parties: {
			client:   "borrower"
			supplier: "desk"
		}
		acts: ["attest-standing"]
		preconditions: {
			"P-1": {text: "The borrower's membership is in good standing, verifiable from the member records at submission."}
			"P-2": {text: "The requested book carries no live loan, verifiable from the loan records."}
		}
		guarantees: {
			"G-1": {text: "A standing attestation exists naming the borrower and the requested book.", records: "the standing attestation record"}
		}
		invariants_local: {}
		cites: ["L-1"]
		blame: [
			{violation_class: "attesting bad standing", at_fault: "desk", evidence: "the member records at attestation time"},
		]
		status: "ratified"
	}
	"C-ISSUE-1": {
		name: "the issuance contract"
		parties: {
			client:   "desk"
			supplier: "librarian"
		}
		acts: ["issue-loan"]
		preconditions: {
			"P-1": {text: "A standing attestation exists naming the borrower and the requested book."}
		}
		guarantees: {
			"G-1": {text: "A loan exists naming the book, the borrower, and the due date.", records: "the loan record"}
		}
		invariants_local: {}
		cites: ["L-1"]
		blame: [
			{violation_class: "issuing over a live loan", at_fault: "librarian", evidence: "the loan records at issuance"},
		]
		status: "ratified"
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
		status: "ratified"
	}
}

experience: {
	"X-1": {
		judgment: "December renewals are adjudicated leniently: exam season produces late requests in good faith; advise renewal over fault-finding."
		cites: ["C-LEND-1", "L-1"]
		advisory: true
	}
}

experience_declared_absent: false
