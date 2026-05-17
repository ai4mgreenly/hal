# NEXT — one transformation

## Relocate the now-decoupled identity tests to live with the site-root capability

**Outcome.** The small set of site-root behavior tests that were just
decoupled from another capability's internal storage — they now obtain
a live agent connection's identity solely through the capability's
public live-connections view — are relocated out of the entry-point
test monolith into the test file co-located with the site-root
capability, constructed and driven through that capability's existing
public injected-input surface, exactly as the earlier rendering and
presence batches were. They assert byte-for-byte the same observable
properties as before. No production source change and no behavior
change.

**Why.** These were the last identity-coupled site-root tests; the
preceding round removed their only entanglement with another
capability's internals, leaving them pure public-surface consumers —
structurally identical to the batches already relocated cleanly.
Moving them now completes the mechanical relocation of every site-root
test that can be relocated without a separate decision, further empties
the entry-point test monolith, and makes explicit that what then keeps
the entry-point compatibility wrappers alive is only tests that are
gated on a separate decision, permanently source-bound, or belong to a
different capability — clarifying exactly what remains before those
wrappers can be retired.

## Scope

- Relocate exactly the site-root behavior tests that were just
  rewritten to obtain a connection's identity through the capability's
  public live-connections view and now require nothing beyond the
  capability's existing public surface and public dependency APIs. They
  move into the same co-located test file as the already-relocated
  tests and assert byte-for-byte the same observable properties (the
  rendered agent row, its identity attribute, the escaping, the
  multi-element row, and every other property each pins). This is a
  pure relocation, not a further rewrite: do not weaken, rename, skip,
  or delete any assertion, and do not change how identity is acquired.
- The fixture-side identity lookup these tests now use — a
  self-validating selection over the public live-connections view that
  fails loudly when there is no match or more than one — moves with
  them unchanged. It must remain a fixture-side query over the public
  view only, with no reintroduced access to any other capability's
  internal storage. Any shared test fixture helper that already exists
  in the destination test file must be reused, not duplicated.
- NOT part of this slice, left unchanged where they are: tests that set
  up their scenario by writing another capability's internal state
  (revoked, expired, or specifically-timed connections); tests that
  additionally read application (non-test) source files; the structural
  source-text invariant test that pins how the single entry-owned
  counter is threaded through the entry point; the test that drives a
  full sign-in-through-federation round trip before observing the site
  root; and the tests that stand up a live server to exercise its
  streaming, footer, or onboarding behavior. Name in the result note
  exactly which tests moved and which remain.
- The entry-point compatibility wrappers stay in place this round (the
  not-yet-moved tests still use them). Their deletion is a later round
  and is gated on the remaining callers being resolved and on the
  structural source-text invariant being re-expressed; do not delete
  them now, and do not delete or alter that invariant.
- Construct and drive the capability through its existing
  injected-input surface and public dependency APIs only — not through
  a global, the request context, application configuration, or any
  other capability's internal storage. Introduce no new production
  symbol, no test-only seam, and no cross-capability test double.
- This batch is small and homogeneous and is expected to move whole.
  Move the largest coherent set that keeps the whole suite green; if it
  must nonetheless be split, move the largest coherent green slice and
  name precisely what moved and what remains so the next round
  continues it.
- This is a test-location change only: do not modify production source,
  reqs/ (the behavioral contract), or helper/.

## Done when

From app-root/, with no behavioral change versus before: the full test
suite passes — the relocated tests asserting the same observable
properties from their new home, every other test unchanged — the
race-detector run passes, gofmt and go vet are clean across the whole
module, no source line in the module exceeds 120 columns, and the
static binary still builds; and each relocated test obtains the
connection identity solely through the capability's public
live-connections view with no access to another capability's internal
storage, and no shared fixture helper is duplicated.

## Result note — 2026-05-17

Completed the relocation for `TestR_0OZT_H8LQ_agent_row_three_elements`,
`TestR_10ZV_8OFH_agent_client_name_renders_as_inert_text`, and
`TestR_VV71_J75U_agent_row_visual_signature`. They now live in
`app-root/siteindex/siteindex_test.go`, drive `siteindex.Surface` through the
existing `handleTestIndex` injected-input fixture, and still select chain IDs
through `testOAuthTokenStore.LiveAgentChains(...)` with the moved
self-validating helper. Removed the relocated copies and helper from
`app-root/main_test.go`; no production files changed.

Left in `app-root/main_test.go`: `TestR_G6NK_RP8H_index_visual_fidelity`,
`TestR_UC3P_Z0IX_exactly_one_shared_counter`,
`TestR_8GJG_64MR_web_login_flow_records_google_email_as_identity`,
`TestR_0NRX_3GV1_agents_block_structure`,
`TestR_VWEX_WYWJ_agent_rows_ordered_by_rendered_identity`,
`TestR_D0XD_1YT0_chain_revoke_action`,
`TestR_0TVF_0BKI_agents_stream_live_updates`,
`TestR_T6VA_9U84_agents_stream_resource_budget`, and
`TestR_6KK2_AAY0_agent_stack_bottom_right_geometry`, matching the scoped
exceptions for source-file reads, entry-point structural invariants,
federation round trips, internal revoked/expired/specifically timed setup,
revoke behavior, live server streams, or geometry checks.

Files changed: `app-root/main_test.go`,
`app-root/siteindex/siteindex_test.go`, and `NEXT.md`.

Verification: `GOROOT=/usr/local/go go test ./siteindex -run
'TestR_(0OZT_H8LQ|10ZV_8OFH|VV71_J75U|2ZZH_LJYA|6QIE_4D71|3RL1_IUP6)'`
passed; `GOROOT=/usr/local/go go test -race ./siteindex -run
'TestR_(0OZT_H8LQ|10ZV_8OFH|VV71_J75U)'` passed; `GOROOT=/usr/local/go go vet
./...` passed; `awk 'length($0)>120 {print FILENAME ":" FNR ":" length($0)}'
$(git ls-files '*.go' ':!:vendor/*')` produced no output; `GOROOT=/usr/local/go
CGO_ENABLED=0 go build -o /tmp/hal-static-test .` passed. `GOROOT=/usr/local/go
go test ./...` and `GOROOT=/usr/local/go go test -race ./...` both failed only
at `TestR_K9TD_DC0K_verified_ledger_entries_have_named_tests` because
`.ralph/requirements-verified.jsonl` is permission denied, which the refactor
prompt identifies as out-of-scope local Ralph state.

Blockers/follow-up risks: broad full-suite and full-race verification remain
locally blocked by unreadable `.ralph/requirements-verified.jsonl`; focused
site-root verification for the relocated tests is green.
