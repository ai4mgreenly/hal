# NEXT — one transformation

## Relocate the next batch of pure site-root rendering behavior tests to live with the site-root capability

**Outcome.** The remaining site-root behavior tests that assert the
observable rendered page purely from an HTTP request and its response —
including the ones that only establish a signed-in session with no
running agents — and that need nothing beyond the site-root capability's
existing public surface, join the already-relocated rendering tests in
the test file co-located with that capability, constructed and driven
through the same public injected-input surface. They assert byte-for-byte
the same observable properties as before. No production source change
and no behavior change.

**Why.** The first batch of pure site-root rendering tests was
relocated cleanly and proved the pattern. The executor sized that batch
conservatively, leaving behind a further homogeneous set of the same
shape — plain "request the site root, inspect the returned page" checks
that require no new test seam. Moving them now, reusing the proven
pattern, continues making the capability self-verifying, further shrinks
the entry-point test monolith, and advances toward the point where the
entry-point compatibility wrappers can be deleted and the entry point
collapsed. It is the lowest-risk next step and keeps every behavior
green.

## Scope

- Relocate exactly the further subset of site-root behavior tests that
  assert the rendered page purely from an HTTP request and its
  response, needing nothing beyond the capability's existing public
  surface to construct and drive it — this explicitly includes tests
  that establish a signed-in session but do not set up any running
  agents, since those need only the session fixture already proven in
  the prior batch. They move into the same co-located test file as the
  already-relocated tests and assert byte-for-byte the same observable
  properties (page structure, named/code blocks, canonical classes,
  section spacing, tab default state, counter-mutation wiring in the
  rendered page, the no-running-agents rendering and its browser-update
  contract, and any other pure request-in / page-out property). Do not
  weaken, rename, skip, or delete any assertion in the move.
- Tests that recover a running agent chain's rendered identity by
  reaching into another capability's internal storage representation or
  a private hashing helper are NOT part of this slice. They relocate in
  a later round, which will first rewrite them to learn that identity
  through the already-available public agent-list view rather than
  internal storage — still with no production change and no new test
  seam. Likewise NOT part of this slice: tests that assert against
  application (non-test) source text rather than rendered output; tests
  that stand up a live server and exercise its streaming/heartbeat
  behavior; and the single test that drives a full
  sign-in-through-federation round trip before observing the site root
  (it travels with the federation-flow test relocation). Leave all of
  these unchanged where they are this round, and name in the result
  note exactly which tests moved and which remain.
- The entry-point compatibility wrappers that the not-yet-moved tests
  still use stay in place this round. Their deletion is a later round
  and is gated on every caller having migrated; do not delete them now,
  and do not delete or alter the structural source-text invariant that
  pins how the single entry-owned counter is threaded through the entry
  point — relocating behavior tests must not touch it.
- The relocated tests construct and drive the capability through its
  existing injected-input surface only — not through a global, the
  request context, or application configuration as a convenience. The
  co-located test file introduces no cross-capability test-only seam
  (no federation or identity-provider doubles, no other capability's
  internal storage).
- Move the largest coherent set of these pure rendering tests that
  keeps the whole suite green; this batch is homogeneous and is
  expected to move whole. If it nonetheless must be split, move the
  largest coherent green slice and name precisely what moved and what
  remains so the next round continues it. Never loosen the invariant to
  fit more in.
- This is a test-location change only: do not modify production source,
  reqs/ (the behavioral contract), or helper/.

## Done when

From app-root/, with no behavioral change versus before: the full test
suite passes — the relocated site-root tests asserting the same
observable properties from their new home, the tests left behind
unchanged in place — the race-detector run passes, gofmt and go vet are
clean across the whole module, no source line in the module exceeds 120
columns, and the static binary still builds.

## Result - 2026-05-17

Relocated the next pure site-root rendering batch into
`app-root/siteindex/siteindex_test.go`, driving each moved test through the
existing `siteindex.Surface`-backed `handleTestIndex` helper and the
co-located injected fixtures. No production source changed.

Moved tests: `TestR_FY4A_3B1M_index_wires_counter_mutations`,
`TestR_WOEN_ND69_named_blocks_are_children_of_page`,
`TestR_9TPL_HQBV_named_blocks_separate_children_of_page`,
`TestR_GTPJ_Z8EL_three_sections_share_uniform_gap_markup`,
`TestR_NBGD_KUHA_instructions_head_not_card_chrome`,
`TestR_MCHV_YEO4_no_shadowed_classes`,
`TestR_UAQQ_NU7B_title_subtitle_are_page_scope_only`,
`TestR_772N_VHQE_default_active_tab_first_render`,
`TestR_UBPK_DLTT_code_blocks_use_canonical_code_class`, and
`TestR_KSI8_M0JX_agents_block_zero_to_one_browser_update`.

Entry-point site-root tests intentionally left in place: source/static checks
`TestR_AOTL_OTYZ_index_html_uses_canonical_class_names`,
`TestR_G6NK_RP8H_index_visual_fidelity`, and
`TestR_UC3P_Z0IX_exactly_one_shared_counter`; live-server/federation/onboarding
checks `TestR_K3PV_GHB3_index_renders_footer`,
`TestR_WHPN_RXSK_base_url_uniform_across_clients`,
`TestR_8GJG_64MR_web_login_flow_records_google_email_as_identity`, and
`TestR_VVRG_W2G2_base_url_is_sufficient_for_mcp_client_onboarding`; live-agent
and agents-stream checks `TestR_0NRX_3GV1_agents_block_structure`,
`TestR_0OZT_H8LQ_agent_row_three_elements`,
`TestR_10ZV_8OFH_agent_client_name_renders_as_inert_text`,
`TestR_VWEX_WYWJ_agent_rows_ordered_by_rendered_identity`,
`TestR_0TVF_0BKI_agents_stream_live_updates`,
`TestR_T6VA_9U84_agents_stream_resource_budget`,
`TestR_VTZ5_5FF5_agents_block_gating`,
`TestR_VV71_J75U_agent_row_visual_signature`,
`TestR_6KK2_AAY0_agent_stack_bottom_right_geometry`,
`TestR_2ZZH_LJYA_banner_grows_for_identity_stack`,
`TestR_6QIE_4D71_agent_stack_uses_canonical_bottom_offset`,
`TestR_CNWX_9VB2_agent_stack_matches_zero_agent_bottom_padding`,
`TestR_TS71_XRW4_banner_does_not_reserve_absent_agent_rows`,
`TestR_O87H_RSH4_no_agent_pages_keep_compact_banner_auth`, and
`TestR_3RL1_IUP6_banner_auth_and_agents_share_one_stack`.

Files changed: `app-root/siteindex/siteindex_test.go`,
`app-root/main_test.go`, and `NEXT.md`.

Verification: `gofmt` on changed Go files passed;
`GOROOT=/usr/local/go go test ./siteindex` passed; focused relocated-test run
passed; `GOROOT=/usr/local/go go test -race ./siteindex` passed;
`GOROOT=/usr/local/go go vet ./...` passed; Go line-length check passed;
`git diff --check` passed; static
`GOROOT=/usr/local/go CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build` passed.
Broad `GOROOT=/usr/local/go go test ./...` and
`GOROOT=/usr/local/go go test -race ./...` reached project tests and failed
only because local Ralph state `.ralph/requirements-verified.jsonl` is
unreadable, which this refactor prompt treats as out-of-scope. Initial
unqualified `go test` attempts failed before project compilation because the
ambient Go command saw mismatched `go1.23.5` standard-library objects with the
`go1.26.2` tool.

Blockers/follow-up: none for this slice. The remaining entry-point tests stay
until later slices move source/static checks, live-server/federation coverage,
and live-agent cases that still need public-observable fixture setup.
