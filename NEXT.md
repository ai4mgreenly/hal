# NEXT — one transformation

## Remove the Google identity provider's test-only production hooks

**Outcome.** The Google identity-provider package exposes no symbol
that exists solely for tests to manipulate. The endpoints a test needs
to redirect to point at an in-test server (the token-exchange endpoint
and the public-key/JWKS endpoint) are supplied through the same
construction-time injection discipline the package already uses for its
clock: real Google endpoints are the defaults, production constructs the
provider explicitly, and there is no post-construction, test-named
mutator on the provider. Every observable behavior is byte-for-byte
unchanged and the full test suite still passes.

**Why.** An earlier phase of this refactor deliberately eliminated all
test-only production hooks — code paths or exported members that exist
only so tests can reach into production state. The Google
identity-provider extraction reintroduced that class: exported,
test-named mutators on the real provider that production never calls
and only tests invoke. That is implicit, test-coupled production
surface of exactly the kind the project removed on purpose. Restoring
the discipline keeps the boundary explicit and matches the package's
own existing construction-time injection precedent (its clock).

## Scope

- The defect to correct: the real Google provider currently carries
  exported, test-named, post-construction mutators (for its
  token-exchange endpoint and its key/JWKS endpoint) plus the
  corresponding test-only accessor. After this round no such test-only
  symbol exists in the package's non-test source.
- Replace them with the same construction-time injection the package
  already applies to its clock: the test-redirectable endpoints become
  optional construction inputs that default to the real Google
  endpoints when not supplied. Production continues to construct the
  real provider exactly as it does today and continues to get the real
  endpoints. A test supplies its in-test endpoints at construction
  through that same seam — not by mutating the provider after it is
  built, and not through any symbol whose only caller is a test.
- Do not merely rename the smell: re-exporting the underlying fields,
  or adding a differently-named post-construction test mutator, does
  not satisfy this. The only way a test may redirect an endpoint is the
  same explicit construction-time injection production uses, with
  production-real defaults.
- Behavior is byte-for-byte invariant: the authorization URL, the
  code-exchange result, and every accept/reject decision of ID-token
  validation (issuer, audience, expiry, signature, key selection),
  including error cases and the workspace-domain constraint, are
  unchanged. The existing identity-provider tests keep every assertion
  exactly; they migrate from the removed mutators to the
  construction-time seam (migrating the test call sites is expected and
  allowed) but assert the same things. Do not weaken, skip, or delete
  any test or assertion.
- Do not read application configuration or application stores from the
  package, do not introduce a package-level global, and do not add any
  new compatibility alias or shim in the entry-point package beyond
  what is unavoidable to keep this round green.
- If the change cannot be completed while staying green in one round,
  make the largest coherent green portion and name in the result note
  exactly what changed and what remains. Never loosen the invariant to
  fit more in.
- Do not edit reqs/ (the behavioral contract) or helper/.

## Done when

From app-root/, with no behavioral change versus before: no test-only
symbol remains in the Google identity-provider package's non-test
source; the full test suite passes; the race-detector run passes;
gofmt and go vet are clean across the whole module; no source line in
the module exceeds 120 columns; and the static binary still builds.

## Result 2026-05-17

Completed the Google identity-provider hook removal by replacing the
post-construction test mutators and token endpoint accessor with construction-time
endpoint options, then migrated the affected identity-provider tests to inject
their loopback token and JWKS endpoints during provider construction.

Files changed:
- `app-root/googleidp/googleidp.go`
- `app-root/main_test.go`
- `NEXT.md`

Verification:
- `GOROOT=/usr/local/go go test ./... -run 'TestR_W3K0_QD0E|TestR_ZBV4_KEJ6|TestR_33DF_7OX1'` passed.
- `GOROOT=/usr/local/go go vet ./...` passed.
- `test -z "$(gofmt -l $(find . -path './.ralph' -prune -o -type f -name '*.go' -print))"` passed.
- `awk 'length($0) > 120 { print FILENAME ":" FNR ":" length($0) }' $(find . -path './.ralph' -prune -o -type f -name '*.go' -print)` produced no output.
- `GOROOT=/usr/local/go CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o hal .` passed.
- `GOROOT=/usr/local/go go test ./...` failed only at `TestR_K9TD_DC0K_verified_ledger_entries_have_named_tests` because `.ralph/requirements-verified.jsonl` is unreadable locally (`permission denied`), which is out-of-scope Ralph state for this refactor prompt.
- `GOROOT=/usr/local/go go test -race ./...` failed only at the same local Ralph ledger permission check.
- `GOROOT=/usr/local/go go test ./... -skip TestR_K9TD_DC0K_verified_ledger_entries_have_named_tests` passed.
- `GOROOT=/usr/local/go go test -race ./... -skip TestR_K9TD_DC0K_verified_ledger_entries_have_named_tests` passed.

Blockers or follow-up risks:
- The shell environment has `GOROOT=/home/mgreenly/.local/go1.23.5.linux-amd64`
  while `/usr/local/bin/go` is Go 1.26.2; verification required overriding
  `GOROOT=/usr/local/go`.
- Full unskipped suite verification remains blocked by unreadable local `.ralph/`
  state, not by this refactor.
