#!/usr/bin/env bash
# R-1D56-BHLP: bring the local-development database back to the state
# of a fresh, never-launched checkout — every application table gone,
# no schema present, no migrations applied. The next `./launch.sh`
# reaches the same end state as on a never-launched checkout per
# R-901W-IT88. Test database is untouched; the suite manages its own.
set -euo pipefail
cd "$(dirname "$0")"
# R-F64T-WYOO: silence RVM's PATH-mismatch warning.
export rvm_silence_path_mismatch_check_flag=1
# Deleting the SQLite file is sufficient: schema, migration ledger
# (schema_migrations / ar_internal_metadata), and every application
# table all live inside that single file. db:prepare on the next
# launch will recreate it from migrations.
rm -f storage/development.sqlite3 storage/development.sqlite3-shm storage/development.sqlite3-wal
