#!/usr/bin/env bash
# R-FUXB-TE9Z: runs the project's full test suite; non-zero on failure.
set -euo pipefail
cd "$(dirname "$0")"
# R-F64T-WYOO: silence RVM's PATH-mismatch warning.
export rvm_silence_path_mismatch_check_flag=1
exec bash -lc 'export rvm_silence_path_mismatch_check_flag=1; source "$HOME/.rvm/scripts/rvm" && rvm use "$(cat .ruby-version)" >/dev/null && exec bundle exec rspec'
