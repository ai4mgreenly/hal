# NEXT — one transformation

## Relocate the signed-in agent-presence rendering tests to live with the site-root capability

**Outcome.** The remaining site-root behavior tests that sign a visitor
in, give that visitor one or more live agent connections through the
capability's public dependencies, and then assert the page's aggregate
banner and agent-stack geometry — the layout the page takes on when
running agents are present — and that need nothing beyond the
capability's existing public surface, join the already-relocated
rendering tests in the co-located test file, constructed and driven
through that same public injected-input surface. They assert
byte-for-byte the same observable properties as before. No production
source change and no behavior change.

**Why.** Two batches of pure site-root rendering tests have already
been relocated cleanly through the capability's public surface. The
next homogeneous batch is the signed-in tests whose assertions are
about the aggregate presence layout — they only need a visitor signed
in with some live agent connections created through public means, never
the generated identity of any particular connection. Moving them now
reuses the proven pattern with no rewrite and no new test machinery,
further shrinks the entry-point test monolith, and continues toward the
point where the entry-point compatibility wrappers can be retired and
the entry point collapsed.

## Scope

- Relocate exactly the site-root behavior tests that (1) establish a
  signed-in visitor and one or more live agent connections using only
  the capability's public dependencies, (2) assert the resulting page's
  aggregate banner / agent-stack geometry, and (3) need nothing beyond
  the capability's existing public surface to construct and drive it.
  They move into the same co-located test file as the already-relocated
  tests and assert byte-for-byte the same observable properties
  (presence versus absence of the agents region, banner growth and
  stack placement, canonical offsets and padding, compact-versus-
  expanded banner auth). Do not weaken, rename, skip, or delete any
  assertion in the move.
- This slice is deliberately limited to relocation that requires NO
  behavioral rewrite of any test. Tests whose assertions depend on
  recovering a specific agent connection's generated identity, and
  which today obtain it by reaching into another capability's internal
  storage, are NOT part of this slice — they relocate in a later round
  that will first rewrite them to obtain that identity through the
  already-available public agent-list view, with no production change.
  A no-op relocation and a behavioral rewrite are separate,
  independently-verifiable claims and must not be combined in one
  round.
- Also NOT part of this slice, left unchanged where they are: tests
  that set up deterministic ordering by mutating another capability's
  internal per-connection timestamps (no public means to do so exists
  yet); tests that read application (non-test) source files; the
  structural source-text invariant test that pins how the single
  entry-owned counter is threaded through the entry point; the test
  that drives a full sign-in-through-federation round trip before
  observing the site root (it travels with the federation-flow test
  relocation); and the tests that stand up a live server to exercise
  its streaming, footer, or onboarding behavior. Name in the result
  note exactly which tests moved and which remain.
- The entry-point compatibility wrappers that the not-yet-moved tests
  still use stay in place this round. Their deletion is a later round
  and is gated on every remaining behavior caller having migrated and
  on the structural source-text invariant being re-expressed; do not
  delete them now, and do not delete or alter that structural invariant
  — relocating behavior tests must not touch it.
- The relocated tests construct and drive the capability through its
  existing injected-input surface and public dependency APIs only — not
  through a global, the request context, application configuration, or
  any other capability's internal storage. The co-located test file
  introduces no cross-capability test-only seam (no federation or
  identity-provider doubles).
- This batch is homogeneous and is expected to move whole. Move the
  largest coherent set that keeps the whole suite green; if it must
  nonetheless be split, move the largest coherent green slice and name
  precisely what moved and what remains so the next round continues it.
  Never loosen the invariant to fit more in.
- This is a test-location change only: do not modify production source,
  reqs/ (the behavioral contract), or helper/.

## Done when

From app-root/, with no behavioral change versus before: the full test
suite passes — the relocated site-root tests asserting the same
observable properties from their new home, the tests left behind
unchanged in place — the race-detector run passes, gofmt and go vet are
clean across the whole module, no source line in the module exceeds 120
columns, and the static binary still builds.

## Result note — 2026-05-17

Completed one relocation iteration for the signed-in aggregate
agent-presence rendering tests. Moved these tests from `app-root/main_test.go`
to `app-root/siteindex/siteindex_test.go`, driving them through the
siteindex capability's injected `Surface` and public dependency APIs:
`TestR_VTZ5_5FF5_agents_block_gating`,
`TestR_2ZZH_LJYA_banner_grows_for_identity_stack`,
`TestR_6QIE_4D71_agent_stack_uses_canonical_bottom_offset`,
`TestR_CNWX_9VB2_agent_stack_matches_zero_agent_bottom_padding`,
`TestR_TS71_XRW4_banner_does_not_reserve_absent_agent_rows`,
`TestR_O87H_RSH4_no_agent_pages_keep_compact_banner_auth`, and
`TestR_3RL1_IUP6_banner_auth_and_agents_share_one_stack`.

Left in `app-root/main_test.go`: `TestR_VV71_J75U_agent_row_visual_signature`
because it depends on recovering generated connection identity,
`TestR_6KK2_AAY0_agent_stack_bottom_right_geometry` because it both recovers
generated chain IDs and reads `web/design.css`, plus the other out-of-scope
tests named in the prompt (ordering timestamp mutation, application source
text invariants, federation round trip, live-server streaming/footer/onboarding
coverage).

Files changed: `app-root/main_test.go`,
`app-root/siteindex/siteindex_test.go`, and `NEXT.md`.

Verification: `GOROOT=/usr/local/go go test ./siteindex` passed;
`GOROOT=/usr/local/go go test ./siteindex -run
'TestR_(VTZ5_5FF5|2ZZH_LJYA|6QIE_4D71|CNWX_9VB2|TS71_XRW4|O87H_RSH4|3RL1_IUP6)'`
passed; `GOROOT=/usr/local/go go test . -run
'TestR_(VV71_J75U|6KK2_AAY0)'` passed; `GOROOT=/usr/local/go go test -race
./siteindex` passed; `GOROOT=/usr/local/go go test -race . -run
'TestR_(VV71_J75U|6KK2_AAY0)'` passed; `GOROOT=/usr/local/go go vet ./...`
passed; `awk 'length($0)>120 {print FILENAME ":" FNR ":" length($0)}'
$(git ls-files '*.go' ':!:vendor/*')` produced no output; `GOROOT=/usr/local/go
CGO_ENABLED=0 go build -o /tmp/hal-static-test .` passed. `GOROOT=/usr/local/go
go test ./...` and `GOROOT=/usr/local/go go test -race ./...` both failed
only at `TestR_K9TD_DC0K_verified_ledger_entries_have_named_tests` because
`.ralph/requirements-verified.jsonl` is permission denied, which this refactor
prompt names as out-of-scope local Ralph state.

Blockers/follow-up risks: broad test and race commands remain locally blocked
by the out-of-scope `.ralph` ledger permission failure unless that local state
is made readable or the Ralph-state test is excluded for refactor verification.
