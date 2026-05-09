# Development environment

Properties of the dev / CI / runtime environment that the build agent
must satisfy. The "current" Ruby and Rails versions are stated here
explicitly so the spec has a single source of truth. When upstream
releases a new line we want to adopt, edit these requirements (and
mint fresh IDs for material changes).

## Versions

- R-NWET-ALMZ: the project pins one specific Ruby version via a
  `.ruby-version` file at the repo root. That file is the single
  source of truth for the exact patch version in use.
- R-OJKW-K8Q6: the pinned Ruby version is in the 4.0.x line.
- R-P4B7-2CBZ: the Rails version used by the project is pinned via the
  Gemfile to the 8.1.x line.

## Smoke tests for the runtime

These tests exist so that "I forgot to switch Ruby versions" or "this
machine doesn't have Ruby installed" surfaces as a loud, specific
failure instead of confusing downstream errors.

- R-QC7K-U30Z: an automated test fails when the running Ruby
  interpreter's reported version does not match the contents of
  `.ruby-version`.
- R-PP1H-KFXS: an automated test fails when the running interpreter
  is not MRI / CRuby.
- R-RNRN-R4Y2: an automated test fails when the bundled Rails version
  is not in the 8.1.x line.

## Bootstrap

- R-SDDJ-SBIN: a fresh checkout, given a working Ruby of the correct
  version, can be brought to a passing test suite by running a single
  documented setup command. Any further manual steps are a bug.

## Testing

- R-0MVV-UEZW: the project's tests are written in RSpec, not the Rails
  default Minitest. `./test.sh` runs the RSpec suite.
- R-GJY8-Y9C8: when an RSpec example verifies one or more specific
  requirements, each verified requirement's ID appears as a literal
  substring (in the canonical `R-XXXX-XXXX` form) of the example's
  full description — that is, the concatenation of every enclosing
  `describe`/`context` label and the example's own `it` text.
  Multiple IDs are separated by single spaces. A `grep` for any
  single requirement ID across the spec tree returns every example
  that verifies that requirement.
- R-H74C-7WFF: not every example must carry a requirement ID. Helper
  specs, fixtures, and exploratory specs are allowed to be un-tagged.
  The trace is one-way: every R-XXXX-XXXX claim that has been
  verified in code is locatable by ID, but the spec suite may also
  contain examples that don't map to any single requirement.

## Scripts

The repo exposes a small, fixed set of shell entry points at its root.
They are the only things a developer or CI job has to know how to run.

- R-ELEJ-EV2V: the standard scripts (`./launch.sh`, `./test.sh`,
  `./lint.sh`) can be invoked directly from a fresh login shell at
  the repo root with no prior `rvm use`, `bundle exec`,
  `source .envrc`, or any other manual environment activation.
  Whatever Ruby version selection, PATH ordering, gem path, and
  related shell-environment setup is needed is performed by the
  scripts themselves (or by tooling — e.g. direnv, an in-script
  shim, a `bin/` launcher — that the scripts invoke). The choice of
  mechanism is HOW; the property is that the user does not have to
  prepare the shell.
- R-F64T-WYOO: when the standard scripts run, their output is clean
  of environment-setup warnings — in particular, no RVM PATH-
  mismatch warning, and no "already initialized constant" or similar
  duplicate-load warnings caused by stdlib gems being loaded once
  from the system Ruby installation and again from the project's
  gem-set.
- R-ZSMI-4WJF: the standard scripts are invoked from a shell that
  has not had a Ruby environment manually activated beforehand.
  Pre-activating the environment in the calling shell — including
  via `rvm use`, sourcing RVM's shell hooks, `bundle exec`, or any
  equivalent — is unsupported, and the warning-free guarantee of
  R-F64T-WYOO does not extend to invocations made from such a
  shell. The scripts assume responsibility for environment
  activation from a clean caller; that is the only invocation
  pattern under which R-F64T-WYOO holds. A caller that wraps a
  standard script with manual activation (e.g.
  `bash -lc 'source ~/.rvm/scripts/rvm && rvm use … && ./test.sh'`)
  defeats the script's own activation mechanism and is the typical
  cause of the very warnings R-F64T-WYOO forbids: the parent shell
  has already loaded the system Ruby's default gems into RubyGems'
  state, so the script cannot prevent the project gemset's
  differently-versioned copies from triggering duplicate-load
  warnings, and the parent shell's PATH ordering provokes RVM's
  PATH-mismatch warning. Callers — developers, CI, and any
  orchestrator running the scripts on a build loop — invoke them
  directly.

- R-EN0Y-1NKZ: `./launch.sh` at the repo root starts the service.
- R-FA71-BAO6: when started via `./launch.sh`, the service listens on
  TCP port 3000.
- R-901W-IT88: when `./launch.sh` brings the service up, the
  database schema is current — every pending migration has been
  applied — before the service begins accepting requests. A fresh
  checkout, a checkout with newly-pulled migrations, and a checkout
  whose database is already current all reach the same end state:
  a running service whose first inbound request is served against
  an up-to-date schema, not rejected with a pending-migration
  error. The user does not have to remember a separate migration
  step; the launch script owns it.
- R-PVA6-Q6OB: the locally-launched service speaks plain HTTP, not
  HTTPS. TLS termination is a deployment concern handled in front of
  the service at https://hal.ai.metaspot.org; the application
  process itself does not terminate TLS, locally or in production.
  The test suite does not depend on TLS being available.
- R-FUXB-TE9Z: `./test.sh` at the repo root runs the project's full
  test suite and exits non-zero if any test fails.
- R-GGVI-P9MH: `./lint.sh` at the repo root runs Rubocop against the
  project and exits non-zero if Rubocop reports any offense.

## Dependencies

- R-H6HE-QG72: Ruby dependencies are managed with Bundler. The
  `Gemfile.lock` is committed. A fresh checkout produces identical
  gem versions on every machine until someone intentionally adds or
  updates a dependency.
- R-HSFL-MBJK: every gem in the Gemfile is pinned to an exact version
  (e.g. `'1.2.3'`, not `'~> 1.2'`), with one explicit exception
  called out below.
- R-CQXC-KB48: the project uses the Rails-provided linting
  conventions as shipped by the Rails generator for the pinned Rails
  version. In the 8.1 line that means the `rubocop-rails-omakase`
  toolchain and the `.rubocop.yml` Rails generates referencing it.
  The project does not add, remove, or retune cops on top of what
  Rails ships, with one explicit exception called out below.
- R-7NWT-PODV: `Layout/LineLength` is the exception to R-CQXC-KB48.
  The project's `.rubocop.yml` enables `Layout/LineLength` with
  `Max: 120`, overriding the omakase default that disables it. Any
  line longer than 120 characters is a lint failure.
- R-DE3F-TY7F: the lint-toolchain gems (`rubocop`,
  `rubocop-rails-omakase`, and any other `rubocop-*` extensions Rails
  pulls in for this purpose) are the exception to the exact-pin rule.
  Their Gemfile entries carry no version constraint, so each
  `bundle update` may pick up newer versions. A lint-toolchain
  release that legitimately breaks the lint check is acceptable and
  expected.
