# Helper TODO — follow-ups the refactor loop is deferring

This file is helper-side memory. It is not read by the refactor executor
(REFACTOR.md tells it to ignore helper/). It records work that the
push-mode structure refactor is knowingly leaving unfinished so it is
not lost when NEXT.md is overwritten each round.

## Pin graceful drain + DB-close as real requirements in reqs/

**Status:** open. Introduced as behavior via the refactor channel
(NEXT.md step 3), which by design carries no `R-` ID and does not touch
`reqs/`.

**What.** The serve path is gaining two behaviors the user explicitly
wants kept:

1. On lifecycle shutdown (operator signal or context cancel) the server
   stops accepting new connections and lets in-flight requests finish
   within a bounded grace period before any remaining connections are
   forced closed.
2. The database opened at serve startup is closed as part of shutdown
   teardown — released on the normal exit path, not only on start-up
   error paths.

**Why this is unresolved.** The refactor is structure-only and holds
`reqs/` fixed as the behavioral invariant; the existing suite proves we
did not move behavior. But these two are genuinely *new* behaviors, not
restructurings. They currently live only in code and in devlog rationale
— not in the behavioral contract the build agent verifies against. Until
they are real requirements, nothing pins them: a later iteration could
regress graceful drain or reintroduce the DB leak and no test would
fail.

**Definition of "resolved completely."** Author requirements in `reqs/`,
to the normal WHAT/WHY-not-HOW discipline, that pin these behaviors as
testable claims, each with a fresh ID minted via `ralph newid`. Expect
this to be more than two IDs once split into checkable claims, e.g.:

- a shutdown signal causes new connections to be refused;
- in-flight requests are allowed to complete within a bounded period;
- once that period is exceeded, remaining connections are forced closed
  and shutdown still returns promptly (no hang);
- the startup-opened database is closed exactly once on every serve
  exit path.

Do this only with the user — the helper does not edit `reqs/`
unilaterally. Once these requirements exist and verify green, delete
this entry.
