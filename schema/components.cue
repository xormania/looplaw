// The component manifest: what a client derived about a system's shape
// and submitted.
//
// looplaw derives nothing here. Working out which components a codebase
// has, and which depends on which, is language-specific and lives with
// whatever can read that language — a package lister, a compiler's own
// index, an import scanner. The kernel decides over the manifest it is
// handed, exactly as it does for the content-hash manifest a scope scan
// produces (T0-4: the kernel never fetches or inspects work-product
// content).
//
// What this carries is only what a tool can establish mechanically:
// components exist, they were derived from sources with these digests,
// and one holds a compiled reference to another. Preconditions,
// guarantees and blame are not here because nothing derives them from
// source — they are authored, and the draft view leaves them empty so
// the gates' refusals become the worklist.
package schema

#ComponentManifest: {
	// The subject this manifest describes. The draft view carries it, so
	// it shares the set's subject grammar.
	subject: string & =~"^[a-z][a-z0-9-]*$"

	// Every component the client found. The key is the component's name
	// as the system calls it — a package path, a module, a service.
	components: [Name=string & =~"^[a-z][a-z0-9/._-]*$"]: {
		name: Name
		// What the component is, in the client's words. Empty is honest
		// where the client cannot tell, and stays empty: a placeholder
		// that reads like prose would pass the gates while saying
		// nothing.
		note: string | *""
		// The sources this component was derived from, with their
		// content hash at derivation time — the same shape and the same
		// purpose as #Provenance.sources, because that is what these
		// become. Provenance cites them: a statement about a component
		// that names no source cannot go stale, so nothing could ever
		// falsify it.
		sources: [string]: =~"^[0-9a-f]{64}$"
	}

	// Which components hold a compiled reference to which. Each edge
	// becomes a contract: the referencing component is the client, which
	// owes the preconditions, and the referenced one is the supplier,
	// which owes the guarantees.
	//
	// An edge naming a component the manifest does not list is refused
	// rather than registering one by implication.
	depends: [Name=string]: [...string]
}
