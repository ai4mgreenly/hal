# NEXT — one transformation

## Extract the Google identity-provider capability into its own package

**Outcome.** The Google identity-provider capability — the seam that
builds a Google authorization URL and exchanges an authorization code
for a verified end-user identity, its test double, its real
OAuth2-backed implementation, and the ID-token validation behind it
(issuer, audience, expiry, and signature against Google's published
keys) — lives in its own package with a deliberately small exported
surface, following the package layout the earlier extractions
established. The entry-point package depends on that surface, no longer
contains the identity-provider internals, constructs the provider at
startup, and keeps only the seam by which a test double is substituted.
The program still builds as one binary and every observable behavior is
unchanged.

**Why.** The Google identity provider is its own capability, distinct
from the OAuth-flow HTTP handlers and the stores already extracted. It
is a hard prerequisite for extracting the OAuth-flow handlers next —
those handlers depend on this seam, and cannot become a clean package
while the identity-provider type still lives in the entry-point
package. Isolating it continues reducing the entry-point package toward
routing and wiring only, and unblocks the remaining test relocations,
shim deletions, and the eventual entry-point collapse.

## Scope

- Extract exactly the **Google identity-provider** capability this
  round: the provider seam (build the authorization URL; exchange a
  code for a verified identity), the test double, the real
  OAuth2-backed implementation, and the ID-token validation it
  performs. The OAuth-flow HTTP handlers stay in the entry-point
  package and call the new seam, the way earlier rounds left their
  consumers in place; they are their own later round.
- Every observable behavior is byte-for-byte identical: the
  authorization URL produced, the code-exchange result, and every
  accept/reject decision of ID-token validation (issuer, audience,
  expiry, signature, key selection) do not change, including error
  cases and the workspace-domain constraint. The existing tests pin
  these precisely and are the proof; do not weaken, skip, or delete
  any test or its assertions.
- The capability must not read application configuration or
  application stores. The workspace domain and client credentials
  remain inputs supplied when the provider is constructed, exactly as
  today. The single current-time read it needs (the ID-token expiry
  check) must become an injected time source defaulting to the real
  clock — no hidden package-level clock global — consistent with the
  decoupling the earlier rounds established.
- The package exposes only the minimal surface its consumers need;
  consumers use only that surface. The seam by which a test double
  replaces the real provider continues to work; the existing
  identity-provider tests continue to run and assert unchanged from
  where they are (they relocate to the new package in a later round).
- Follow the module and package-layout precedent the earlier
  extractions set; the exact name and path are yours to choose,
  idiomatic Go. Do not leave new compatibility aliases in the
  entry-point package beyond what is unavoidable to keep this round
  green; prefer migrating the consumer to the new surface.
- If the whole capability cannot move while staying green in one round,
  move the largest coherent green slice and name in the result note
  exactly what moved and what remains, so the next round continues it.
  Never loosen the invariant to fit more in.
- Production behavior is identical. Do not edit reqs/ (the behavioral
  contract) or helper/.

## Done when

From app-root/, with no behavioral change versus before: the full test
suite passes, the race-detector run passes, gofmt and go vet are clean
across the whole module, no source line in the module exceeds 120
columns, and the static binary still builds.

## Result

Completed one coherent extraction of the Google identity-provider capability
into `app-root/googleidp`. The new package owns the provider seam, identity
value, fake provider, real OAuth2-backed provider, Google ID-token validation,
JWK fetching, audience/issuer/expiry/signature checks, and an injected clock
option for expiry validation. `main.go` now imports that package, keeps only
the context/wiring seam needed by existing handlers and tests, and constructs
the real provider at startup with `appNow` injected.

Files changed:
- `app-root/googleidp/googleidp.go`
- `app-root/main.go`
- `app-root/main_test.go`
- `NEXT.md`

Verification:
- `env -u GOROOT go test -run 'TestR_(T0B2_A4E5|VF61_2Y6I|W3K0_QD0E|ZBV4_KEJ6|33DF_7OX1|ANRQ_04PK)' .`
  passed.
- `env -u GOROOT go test -race -run 'TestR_(T0B2_A4E5|VF61_2Y6I|W3K0_QD0E|ZBV4_KEJ6|33DF_7OX1|ANRQ_04PK)' .`
  passed.
- `env -u GOROOT go vet ./...` passed.
- `awk 'length($0) > 120 { print FILENAME ":" FNR ":" length($0) }' $(find . -name '*.go' -not -path './.ralph/*' | sort)`
  produced no output.
- `env -u GOROOT CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /tmp/hal-refactor-build .`
  passed.
- `env -u GOROOT go test ./...` reached project code and failed only at
  `TestR_K9TD_DC0K_verified_ledger_entries_have_named_tests` because
  `.ralph/requirements-verified.jsonl` is permission-denied local Ralph state,
  which `helper/REFACTOR.md` marks out of scope.
- `env -u GOROOT go test -race ./...` likewise failed only at that same Ralph
  ledger permission check.

Blockers / follow-up risks:
- The shell environment has `GOROOT` set to a Go 1.23.5 tree while `go` is
  Go 1.26.2; verification commands had to unset `GOROOT` to use the matching
  `/usr/local/go` tree.
- Existing main-package tests still refer to the old unexported seam names via
  type aliases and use small test endpoint hooks on `googleidp.RealProvider`;
  later test relocation can remove those shims.
