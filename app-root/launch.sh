#!/usr/bin/env bash
# R-EN0Y-1NKZ: starts the service.
# R-FA71-BAO6: default Rails port is 3000.
# R-PVA6-Q6OB: plain HTTP — no TLS termination here.
set -euo pipefail
cd "$(dirname "$0")"
# R-F64T-WYOO: silence RVM's PATH-mismatch warning. The login shell's PATH
# is set by /etc init scripts, not by us, and the project gemset bin is
# guaranteed first by rvm use anyway.
export rvm_silence_path_mismatch_check_flag=1
# R-901W-IT88: bring the schema up to date before the service begins
# accepting requests. `db:prepare` creates the database on a fresh
# checkout and applies any pending migrations; on an already-current
# checkout it is a no-op. Either way, by the time `bin/rails server`
# starts, the first inbound request is served against an up-to-date
# schema rather than failing with ActiveRecord::PendingMigrationError.
exec bash -lc 'export rvm_silence_path_mismatch_check_flag=1; source "$HOME/.rvm/scripts/rvm" && rvm use "$(cat .ruby-version)" >/dev/null && bin/rails db:prepare && exec bin/rails server'
