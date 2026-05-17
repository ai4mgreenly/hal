# NEXT — one transformation

## Decouple the identity-coupled site-root tests from another capability's internal storage

**Outcome.** The small set of site-root behavior tests that today learn
a specific live agent connection's generated identity by reaching into
another capability's internal in-memory storage are rewritten so they
instead obtain that identity from that capability's already-public
live-connections view — the very same view the site-root page itself
renders those connections from. The tests stay exactly where they are,
assert byte-for-byte the same observable page properties, and afterward
hold no access of any kind to that other capability's internals. No
production source change, no behavior change, and no test relocation
this round.

**Why.** These tests assert on a rendered agent row keyed by a
connection's generated identity, and currently recover that identity by
locking and indexing another capability's internal storage through a
private helper. That coupling is the only thing keeping them from the
proven relocation pattern the earlier batches used. The capability
already exposes a public roll-up of live agent connections (their
identity, owning client, display name, and issue time), and the
site-root page itself renders from exactly that roll-up — so reading
the identity back through it is equivalent to what the page did by
construction. Doing this as its own round, with the tests unmoved and
still green, makes it a single independently-verifiable claim — "these
tests no longer touch another capability's internals and assert
identically" — cleanly separable from the later relocation, so a
regression in either is unambiguously attributable.

## Scope

- Rewrite exactly the site-root behavior tests that learn a specific
  live agent connection's generated identity by reading another
  capability's internal storage and have no other coupling. Each place
  such a test recovers that identity — some recover it in more than one
  place — is changed to obtain it from the capability's already-public
  live-connections view, selecting the relevant entry by the
  per-test-distinct owning-client distinguisher the test already sets
  up. A test that is only partially converted does not satisfy this
  round: after the rewrite, no such test may retain any access to the
  other capability's internal storage or its private identity helper,
  anywhere in it.
- Every observable assertion is byte-for-byte unchanged: the rendered
  agent row, its identity attribute, the escaping, the ordering, and
  every other page property each test pins. This round changes only how
  the expected identity is acquired by the test, never what is asserted
  about the render. Do not weaken, rename, skip, or delete any
  assertion.
- The tests stay exactly where they are this round — no relocation.
  They continue to drive the capability through the existing
  entry-point compatibility wrapper, unchanged. Relocating them through
  the public surface is a separate later round; a behavioral rewrite
  and a relocation are distinct, independently-verifiable claims and
  must not be combined.
- NOT part of this round, left entirely unchanged where they are: tests
  that set up their scenario by writing another capability's internal
  state (for example, simulating revoked, expired, or
  specifically-timed connections) — there is no public means for that
  and none is to be introduced here; tests that additionally read
  application (non-test) source files; the structural source-text
  invariant test that pins how the single entry-owned counter is
  threaded through the entry point; the test that drives a full
  sign-in-through-federation round trip before observing the site root;
  and the tests that stand up a live server to exercise its streaming,
  footer, or onboarding behavior. Name in the result note exactly which
  tests were rewritten.
- This is a behavior-preserving, test-only change. The public
  live-connections view the tests now read already exists and is
  already what the page renders from; it must not be altered or
  extended. Introduce no new production symbol, no test-only seam, and
  no cross-capability test double. Do not modify production source,
  reqs/ (the behavioral contract), or helper/.

## Done when

From app-root/, with no behavioral change versus before: the full test
suite passes — the rewritten tests asserting the same observable
properties from where they already are, every other test unchanged —
the race-detector run passes, gofmt and go vet are clean across the
whole module, no source line in the module exceeds 120 columns, and the
static binary still builds; and each rewritten test obtains the
connection identity solely through the capability's public
live-connections view, with no remaining access to another capability's
internal storage.

## Result note — 2026-05-17

Completed the identity-decoupling rewrite for these site-root behavior tests
in `app-root/main_test.go`: `TestR_0OZT_H8LQ_agent_row_three_elements`,
`TestR_10ZV_8OFH_agent_client_name_renders_as_inert_text`, and
`TestR_VV71_J75U_agent_row_visual_signature`. Each now obtains expected
agent chain IDs through `oauthTokenStore.LiveAgentChains(...)`, selecting by
the test's unique owner email and client ID, instead of reading
`oauthTokenStore.M` directly. Left out-of-scope tests unchanged, including
the ordering test that mutates token timestamps, the geometry test that reads
`web/design.css`, the revoke-action tests, and the agents stream tests.

Files changed: `app-root/main_test.go` and `NEXT.md`.

Verification: `GOROOT=/usr/local/go go test . -run
'TestR_(0OZT_H8LQ|10ZV_8OFH|VV71_J75U|6KK2_AAY0|VWEX_WYWJ)'` passed;
`GOROOT=/usr/local/go go test -race . -run
'TestR_(0OZT_H8LQ|10ZV_8OFH|VV71_J75U)'` passed; `GOROOT=/usr/local/go
go vet ./...` passed; `awk 'length($0)>120 {print FILENAME ":" FNR ":"
length($0)}' $(git ls-files '*.go' ':!:vendor/*')` produced no output;
`GOROOT=/usr/local/go CGO_ENABLED=0 go build -o /tmp/hal-static-test .`
passed. `GOROOT=/usr/local/go go test ./...` and `GOROOT=/usr/local/go
go test -race ./...` both failed only at
`TestR_K9TD_DC0K_verified_ledger_entries_have_named_tests` because
`.ralph/requirements-verified.jsonl` is permission denied, which the refactor
prompt identifies as out-of-scope local Ralph state.

Blockers/follow-up risks: broad full-suite and full-race verification remain
locally blocked by the unreadable `.ralph/requirements-verified.jsonl` state
unless that local file is made readable or the Ralph-state test is excluded
for refactor verification.
