# NEXT — one transformation

## Relocate the site-root page's pure rendering behavior tests to live with the site-root capability

**Outcome.** The behavior tests that pin the observable site-root page
— its signed-out form, its signed-in form, the status, the content
type, and the rendered banner / identity / agent / counter /
client-configuration markup it produces for each session state — and
that exercise the site root purely at the request/response boundary,
live in a test file co-located with the already-extracted site-root
request-assembly capability and drive it through that capability's
existing public surface. They assert exactly the same observable
properties as before. The remaining site-root tests stay where they are
this round. No production source change and no behavior change.

**Why.** The site-root request-assembly capability was extracted into
its own package, but its tests still sit in the entry-point test
monolith and reach the capability only through entry-point
compatibility wrappers. Co-locating the tests that need nothing but the
capability's public surface makes the package self-verifying, shrinks
the monolith, and is the prerequisite for later deleting the now-
superfluous entry-point wrappers and collapsing the entry point. Doing
this first — before the other capabilities' test relocations — keeps
the move small and fully green, because the bulk of the site-root tests
are pure request-in / rendered-page-out checks with no entanglement.

## Scope

- Relocate exactly the subset of site-root behavior tests that assert
  the rendered page purely from an HTTP request and its response,
  needing nothing beyond the capability's existing public surface to
  construct and drive it. They move into a test file co-located with
  that capability and assert byte-for-byte the same observable
  properties (signed-out vs signed-in page, status, content type,
  banner, identity area, agent list and its ordering, counter card and
  its enabled/disabled controls, client-configuration interface,
  accessibility/motion attributes). Do not weaken, rename, skip, or
  delete any assertion in the move.
- Tests that, to set up a signed-in visitor with live agent chains,
  depend on another capability's internal storage representation or on
  an entry-point-private fixture helper, and the single test that
  drives a full sign-in-through-federation round trip before observing
  the site root, are NOT part of this slice. Leave them unchanged in
  the entry-point test monolith this round; they relocate in a later
  round, once an observable fixture seam exists and alongside the
  federation-flow tests. Name in the result note exactly which tests
  moved and which remain.
- The entry-point compatibility wrappers that let the not-yet-moved
  tests reach the capability stay in place this round. Their deletion
  is a later round and is gated on every caller having migrated; do not
  delete them now, and do not delete or alter the structural
  source-text invariant that pins how the single entry-owned counter is
  threaded through the entry point — relocating behavior tests must not
  touch it.
- The relocated tests construct and drive the capability through its
  existing injected-input surface only — not through a global, the
  request context, or application configuration as a convenience. The
  new test file introduces no cross-capability test-only seam.
- This slice is deliberately partial. Move the largest coherent set of
  pure site-root rendering tests that keeps the whole suite green;
  never loosen the invariant to fit more in. If even that must be
  split, move the largest coherent green slice and name precisely what
  moved and what remains so the next round continues it.
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

Relocated the pure site-root rendering behavior tests into
`app-root/siteindex/siteindex_test.go`. The moved tests now construct the
site-root capability through `siteindex.Surface`, injecting the counter,
web-session store, OAuth token/client stores, request base-URL helper, and
version string directly.

Moved tests: `TestR_8KKV_TDWF_index_renders_banner_card`,
`TestR_BZQY_DN3B_index_displays_mcp_client_config`,
`TestR_5GQZ_KWCD_mcp_snippets_in_client_documented_format`,
`TestR_G5FO_DXHS_claude_code_panel_has_two_stacked_scope_blocks`,
`TestR_H4LJ_G9HR_mcp_client_instructions_is_tab_interface`,
`TestR_CO4Y_11X7_mcp_snippets_url_is_request_derived`,
`TestR_GVMQ_ZCBQ_index_renders_counter_card`,
`TestR_G0K2_UUJ0_index_motion_and_aria`,
`TestR_GUEU_LKL1_index_reflects_web_session_state`,
`TestR_UBYN_1LY0_client_tab_inner_markup`,
`TestR_8031_9QQ9_banner_title_is_hal_9000`,
`TestR_1ZS0_XSZ7_document_title_is_short_form`,
`TestR_0WB7_RV1W_banner_auth_placement_and_shape`,
`TestR_EJAP_XUSB_counter_card_structure`,
`TestR_TEP7_Q6UT_signed_in_email_renders_as_inert_text`, and
`TestR_A2L2_1NA1_signed_in_sign_out_is_post_form_without_href`.

Remaining entry-point site-root tests include the live-server root/footer
checks, the full federation-through-index check
`TestR_8GJG_64MR_web_login_flow_records_google_email_as_identity`, the
counter mutation wiring check `TestR_FY4A_3B1M_index_wires_counter_mutations`,
layout/CSS source checks such as `TestR_G6NK_RP8H_index_visual_fidelity`,
`TestR_WOEN_ND69_named_blocks_are_children_of_page`,
`TestR_9TPL_HQBV_named_blocks_separate_children_of_page`,
`TestR_GTPJ_Z8EL_three_sections_share_uniform_gap_markup`,
`TestR_NBGD_KUHA_instructions_head_not_card_chrome`,
`TestR_MCHV_YEO4_no_shadowed_classes`,
`TestR_UAQQ_NU7B_title_subtitle_are_page_scope_only`,
`TestR_772N_VHQE_default_active_tab_first_render`, and
`TestR_UBPK_DLTT_code_blocks_use_canonical_code_class`, plus the live-agent
chain and browser-update tests that still depend on token/client internals or
entry-point fixtures: `TestR_0NRX_3GV1_agents_block_structure`,
`TestR_0OZT_H8LQ_agent_row_three_elements`,
`TestR_10ZV_8OFH_agent_client_name_renders_as_inert_text`,
`TestR_VWEX_WYWJ_agent_rows_ordered_by_rendered_identity`,
`TestR_KSI8_M0JX_agents_block_zero_to_one_browser_update`,
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
`app-root/main_test.go`, `NEXT.md`.

Verification: `gofmt` on changed Go files passed; `GOROOT=/usr/local/go go
test ./siteindex` passed; focused relocated-test run passed; focused
`GOROOT=/usr/local/go go test -race` for `./siteindex`, `./counter`,
`./mcpwire`, and `./websession` passed; focused relocated-test race run passed;
`GOROOT=/usr/local/go go vet ./...` passed; line-length check passed; static
`CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build` passed. Broad
`GOROOT=/usr/local/go go test ./...` and `GOROOT=/usr/local/go go test -race
./...` both reached project tests and failed only on local Ralph state:
`.ralph/requirements-verified.jsonl` is unreadable, which this refactor prompt
treats as out-of-scope.

Blockers/follow-up: no production source changed. The remaining entry-point
site-root tests still use compatibility wrappers until later slices provide an
observable fixture seam for live-agent chains and move the federation-flow
coverage.
