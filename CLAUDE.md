# Claude Code

The brief for this repository is [AGENTS.md](AGENTS.md) — read it
first. It is harness-neutral by design and is the only copy: this file
exists so a Claude session finds it, and holds nothing of its own.

One Claude-specific convenience: the review protocol in `dev/review.md`
is automated as a saved workflow —

    Workflow({name: "review", args: {target: "…", focus: "…"}})
