# NEXT — one transformation

## Decouple the OAuth state-expiry tests from the entry-point process clock

**Outcome.** Two existing behavior tests assert that a recorded OAuth
sign-in state is rejected once its lifetime has elapsed before the
callback consumes it — one in the context of state-binding enforcement
across every redirect path, the other in the context of state being
bound to a browser session. Today each simulates the passage of time by
mutating a process-wide clock owned by the application entry point.
After this change, each of those tests instead obtains its
expiry-clock determinism from a caller-supplied time source injected
through the state-record dependency store's existing public
construction options, on the test's own store instance — never by
reaching the entry-point process-clock global. Both tests stay where
they are, under their existing names; every assertion, expected status,
and message is preserved byte-for-byte. No production source change and
no behavior change.

**Why.** The state-record dependency store already accepts a
caller-supplied time source through its public construction surface;
these two tests are the only ones still simulating expiry by mutating a
shared mutable process clock owned by the entry point. That shared
global is a cross-test state hazard, and it is the specific blocker
that prevents these two large tests from later moving into the
capability's own package without behavior change. Removing it in place
first — as a small, isolated, independently-verifiable rewrite —
reduces the eventual relocation of these tests to the same kind of move
already proven for the callback tests, instead of forcing a clock
decouple, a configuration decouple, a driving-seam swap, and a scenario
split to all happen in one risky step. This round does only the clock
decouple.

## Scope

- Change exactly the two existing behavior tests that assert an expired
  recorded sign-in state is rejected at the callback: the one that
  verifies state-binding is enforced on every redirect path, and the
  one that verifies state is bound to the browser session. In each,
  replace the simulation of elapsed time — currently performed by
  mutating the entry-point process-clock global — with a deterministic
  time source supplied to the test's own state-record store through
  that store's existing public construction options. The store must
  observe time advancing past the configured lifetime solely through
  that injected source.
- Introduce no new production symbol and no test-only seam: the
  caller-supplied time-source option on the dependency store's public
  construction surface already exists and is the only mechanism used.
- Both tests remain in place, under their existing recorded names, this
  round. This is NOT a relocation: do not move either test into the
  capability's package and do not split either test. Do not change how
  the tests drive the flow beyond what is required to give the
  expiry-checked store its injected time source.
- It is acceptable, for this round only, that these tests still read
  the request lifetime from an entry-point configuration global and
  still drive the flow through the entry-point compatibility wrappers;
  relinquishing those is a separate later round. The single property
  this round establishes is that expiry time is simulated through the
  injected store time source, not through the entry-point
  process-clock global.
- Acceptance properties that must hold:
  - Assertion preservation. Across the suite, the complete set of
    assertions and their expected values is unchanged: every assertion
    that existed before still runs, byte-identical in expectation and
    behaviorally-equivalent in setup, none weakened, skipped,
    renamed-away, deleted, or duplicated in a way that changes meaning.
  - Requirement-name traceability. Every requirement whose verified
    record names a specific test must continue to have a test of
    exactly that name in the suite, coherent and non-vacuous; this
    round renames nothing.
- This is a test-structure change only: do not modify production
  source, reqs/ (the behavioral contract), or helper/.

## Done when

From app-root/, with no behavioral change versus before: the full test
suite passes — both expiry tests asserting the same observable
properties, now driving their expiry simulation through a deterministic
time source injected via the dependency store's existing public
construction options rather than the entry-point process-clock global,
every other test unchanged — the race-detector run passes, gofmt and
go vet are clean across the whole module, no source line in the module
exceeds 120 columns, and the static binary still builds; neither test
reaches the entry-point process-clock global, no new production symbol
or test-only seam was introduced, and both acceptance properties above
hold. Name in the result note exactly which two tests were changed and
how the deterministic time source was supplied.
