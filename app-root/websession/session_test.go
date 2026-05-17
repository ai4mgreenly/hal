package websession

import (
	"database/sql"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

var webSessionNow = time.Now

const (
	webSessionAbsoluteTTL = 12 * time.Hour
	webSessionIdleTTL     = time.Hour
)

func newWebSessionStorage() *Store {
	return New(Options{
		Now: func() time.Time {
			return webSessionNow()
		},
		AbsoluteTTL: func() time.Duration {
			return webSessionAbsoluteTTL
		},
		IdleTTL: func() time.Duration {
			return webSessionIdleTTL
		},
	})
}

func openWebSessionDB(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(
		`CREATE TABLE IF NOT EXISTS web_sessions (` +
			`session_hash TEXT PRIMARY KEY, ` +
			`owner_email TEXT NOT NULL, ` +
			`issued_at INTEGER NOT NULL, ` +
			`expires_at INTEGER NOT NULL, ` +
			`last_seen_at INTEGER NOT NULL, ` +
			`revoked_at INTEGER` +
			`)`); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

// R-KJ15-9P17: a web session is bounded by two ceilings beyond explicit
// revocation — 1 hour of inactivity (idle, restarts on each successful
// authenticated request) and 12 hours from issue (absolute, regardless
// of activity). The earlier of the two governs.
func TestR_KJ15_9P17_session_expires_at_idle_and_absolute_ceilings(t *testing.T) {
	prev := webSessionNow
	t.Cleanup(func() { webSessionNow = prev })
	start := time.Unix(1_700_000_000, 0)

	t.Run("idle_ceiling_expires_at_one_hour_of_inactivity", func(t *testing.T) {
		webSessionStore := newWebSessionStorage()
		webSessionNow = func() time.Time { return start }
		plaintext, err := webSessionStore.Issue("dave@discovery.one")
		if err != nil {
			t.Fatalf("issue: %v", err)
		}
		webSessionNow = func() time.Time {
			return start.Add(webSessionIdleTTL + time.Second)
		}
		if rec := webSessionStore.Lookup(plaintext); rec != nil {
			t.Errorf("session still live 1h+1s after issue with no activity "+
				"(R-KJ15-9P17); rec=%+v", rec)
		}
	})

	t.Run("idle_clock_restarts_on_each_successful_lookup", func(t *testing.T) {
		webSessionStore := newWebSessionStorage()
		webSessionNow = func() time.Time { return start }
		plaintext, err := webSessionStore.Issue("dave@discovery.one")
		if err != nil {
			t.Fatalf("issue: %v", err)
		}
		// 30 minutes in: still live, lastSeenAt advances.
		webSessionNow = func() time.Time { return start.Add(30 * time.Minute) }
		if rec := webSessionStore.Lookup(plaintext); rec == nil {
			t.Fatalf("session expired 30m after issue (R-KJ15-9P17)")
		}
		// 50 more minutes (80m total from issue, 50m from last lookup):
		// still live because the 1h clock restarted at the prior lookup.
		webSessionNow = func() time.Time { return start.Add(80 * time.Minute) }
		if rec := webSessionStore.Lookup(plaintext); rec == nil {
			t.Fatalf("idle clock did not restart on prior lookup " +
				"(R-KJ15-9P17); 80m total / 50m since last seen")
		}
		// 61 more minutes with no activity: idle ceiling fires.
		webSessionNow = func() time.Time {
			return start.Add(80*time.Minute + webSessionIdleTTL + time.Second)
		}
		if rec := webSessionStore.Lookup(plaintext); rec != nil {
			t.Errorf("session still live 1h+1s after last successful lookup "+
				"(R-KJ15-9P17); rec=%+v", rec)
		}
	})

	t.Run("absolute_ceiling_expires_at_twelve_hours_regardless_of_activity",
		func(t *testing.T) {
			webSessionStore := newWebSessionStorage()
			webSessionNow = func() time.Time { return start }
			plaintext, err := webSessionStore.Issue("dave@discovery.one")
			if err != nil {
				t.Fatalf("issue: %v", err)
			}
			// Keep idle alive: lookup every 30 minutes for the full 12 hours.
			for m := 30; m < int(webSessionAbsoluteTTL/time.Minute); m += 30 {
				offset := time.Duration(m) * time.Minute
				webSessionNow = func() time.Time { return start.Add(offset) }
				if rec := webSessionStore.Lookup(plaintext); rec == nil {
					t.Fatalf("session prematurely expired at +%dm despite "+
						"continuous activity (R-KJ15-9P17)", m)
				}
			}
			// One second past absolute: even a still-warm idle clock cannot
			// save the session.
			webSessionNow = func() time.Time {
				return start.Add(webSessionAbsoluteTTL + time.Second)
			}
			if rec := webSessionStore.Lookup(plaintext); rec != nil {
				t.Errorf("session still live 12h+1s after issue despite "+
					"absolute ceiling (R-KJ15-9P17); rec=%+v", rec)
			}
		})
}

// R-8CBQ-IKKA: web session records live in SQLite and are reloaded when
// the service restarts against the same database. A still-valid
// hal_session cookie remains signed in across that restart; explicit
// revocation makes the same cookie unknown.
func TestR_8CBQ_IKKA_web_sessions_survive_restart(t *testing.T) {
	prev := webSessionNow
	start := time.Date(2026, 5, 14, 12, 0, 0, 0, time.UTC)
	webSessionNow = func() time.Time { return start }
	t.Cleanup(func() { webSessionNow = prev })

	dbPath := filepath.Join(t.TempDir(), "hal.DB")

	db1, err := openWebSessionDB(dbPath)
	if err != nil {
		t.Fatalf("open first db: %v (R-8CBQ-IKKA)", err)
	}
	first := newWebSessionStorage()
	if err := first.Attach(db1); err != nil {
		t.Fatalf("first attach: %v (R-8CBQ-IKKA)", err)
	}
	plaintext, err := first.Issue("restart-session@example.com")
	if err != nil {
		t.Fatalf("issue session: %v (R-8CBQ-IKKA)", err)
	}
	if err := db1.Close(); err != nil {
		t.Fatalf("close first db: %v (R-8CBQ-IKKA)", err)
	}

	webSessionNow = func() time.Time { return start.Add(10 * time.Minute) }
	db2, err := openWebSessionDB(dbPath)
	if err != nil {
		t.Fatalf("reopen db: %v (R-8CBQ-IKKA)", err)
	}
	restarted := newWebSessionStorage()
	if err := restarted.Attach(db2); err != nil {
		t.Fatalf("restart attach: %v (R-8CBQ-IKKA)", err)
	}
	rec := restarted.Lookup(plaintext)
	if rec == nil {
		t.Fatalf("session cookie became unknown after restart " +
			"(R-8CBQ-IKKA)")
	}
	if rec.OwnerEmail() != "restart-session@example.com" {
		t.Fatalf("ownerEmail after restart = %q, want %q (R-8CBQ-IKKA)",
			rec.OwnerEmail(), "restart-session@example.com")
	}
	restarted.Revoke(plaintext)
	if got := restarted.Lookup(plaintext); got != nil {
		t.Fatalf("revoked session still validates before close " +
			"(R-8CBQ-IKKA)")
	}
	if err := db2.Close(); err != nil {
		t.Fatalf("close second db: %v (R-8CBQ-IKKA)", err)
	}

	db3, err := openWebSessionDB(dbPath)
	if err != nil {
		t.Fatalf("reopen revoked db: %v (R-8CBQ-IKKA)", err)
	}
	revoked := newWebSessionStorage()
	if err := revoked.Attach(db3); err != nil {
		t.Fatalf("revoked attach: %v (R-8CBQ-IKKA)", err)
	}
	if got := revoked.Lookup(plaintext); got != nil {
		t.Fatalf("revoked session validated after restart " +
			"(R-8CBQ-IKKA)")
	}
	if err := db3.Close(); err != nil {
		t.Fatalf("close revoked db: %v (R-8CBQ-IKKA)", err)
	}
}

// R-SLGL-B5B4: web sessions are persisted in a dedicated store distinct
// from the OAuth token store. Each record carries owner, hash of the
// session identifier, issued-at, expires-at, and revoked-at. The
// plaintext identifier never lands in the store — only the
// Set-Cookie response carries it. Validation is a single hash lookup
// that accepts iff the record is un-expired and un-revoked; revoke
// writes revoked-at and the same value cannot be redeemed again.
func TestR_SLGL_B5B4_web_session_store_properties(t *testing.T) {
	t.Run("record_carries_required_fields", func(t *testing.T) {
		type webSession = Session
		rt := reflect.TypeOf(webSession{})
		want := map[string]string{
			"ownerEmail": "string",
			"issuedAt":   "time.Time",
			"expiresAt":  "time.Time",
			"revokedAt":  "time.Time",
		}
		for name, ty := range want {
			f, ok := rt.FieldByName(name)
			if !ok {
				t.Errorf("webSession is missing required field %q "+
					"(R-SLGL-B5B4)", name)
				continue
			}
			if got := f.Type.String(); got != ty {
				t.Errorf("webSession.%s type = %q, want %q "+
					"(R-SLGL-B5B4)", name, got, ty)
			}
		}
	})

	t.Run("store_keys_records_by_hash_not_plaintext", func(t *testing.T) {
		webSessionStore := newWebSessionStorage()
		plaintext, err := webSessionStore.Issue("user-slgl@example.com")
		if err != nil {
			t.Fatalf("issue: %v (R-SLGL-B5B4)", err)
		}
		t.Cleanup(func() { webSessionStore.Revoke(plaintext) })

		plaintextKeyed := webSessionStore.HasPlaintextKeyForTest(plaintext)
		hashKeyed := webSessionStore.HasHashKeyForTest(plaintext)
		// Defense-in-depth: confirm no record's string fields hold
		// the plaintext.
		plaintextLeaked := webSessionStore.PlaintextLeakedForTest(plaintext)

		if plaintextKeyed {
			t.Errorf("webSessionStore is keyed by plaintext — must " +
				"key by hash (R-SLGL-B5B4)")
		}
		if !hashKeyed {
			t.Errorf("webSessionStore has no record at the plaintext's " +
				"hash — issue() did not persist by hash (R-SLGL-B5B4)")
		}
		if plaintextLeaked {
			t.Errorf("a webSession record holds the plaintext session " +
				"identifier — plaintext must appear only in the " +
				"Set-Cookie response (R-SLGL-B5B4)")
		}
	})

	t.Run("record_owner_issued_expires_match_inputs", func(t *testing.T) {
		webSessionStore := newWebSessionStorage()
		fixed := time.Date(2026, 5, 12, 9, 0, 0, 0, time.UTC)
		prev := webSessionNow
		webSessionNow = func() time.Time { return fixed }
		t.Cleanup(func() { webSessionNow = prev })

		plaintext, err := webSessionStore.Issue("owner-slgl@example.com")
		if err != nil {
			t.Fatalf("issue: %v (R-SLGL-B5B4)", err)
		}
		t.Cleanup(func() { webSessionStore.Revoke(plaintext) })

		rec := webSessionStore.RecordForPlaintextForTest(plaintext)
		if rec == nil {
			t.Fatalf("record missing after issue (R-SLGL-B5B4)")
		}
		if rec.OwnerEmail() != "owner-slgl@example.com" {
			t.Errorf("ownerEmail = %q, want %q (R-SLGL-B5B4)",
				rec.OwnerEmail(), "owner-slgl@example.com")
		}
		if !rec.IssuedAt().Equal(fixed) {
			t.Errorf("issuedAt = %v, want %v (R-SLGL-B5B4)",
				rec.IssuedAt(), fixed)
		}
		if !rec.ExpiresAt().Equal(fixed.Add(webSessionAbsoluteTTL)) {
			t.Errorf("expiresAt = %v, want %v (R-SLGL-B5B4)",
				rec.ExpiresAt(), fixed.Add(webSessionAbsoluteTTL))
		}
		if !rec.RevokedAt().IsZero() {
			t.Errorf("revokedAt = %v, want zero on a fresh record "+
				"(R-SLGL-B5B4)", rec.RevokedAt())
		}
	})

	t.Run("lookup_is_single_hash_lookup_accepting_live_records", func(t *testing.T) {
		webSessionStore := newWebSessionStorage()
		plaintext, err := webSessionStore.Issue("lookup-slgl@example.com")
		if err != nil {
			t.Fatalf("issue: %v (R-SLGL-B5B4)", err)
		}
		t.Cleanup(func() { webSessionStore.Revoke(plaintext) })

		if got := webSessionStore.Lookup(plaintext); got == nil {
			t.Fatalf("lookup of live session returned nil "+
				"(R-SLGL-B5B4); plaintext=%q", plaintext)
		}
		if got := webSessionStore.Lookup(plaintext + "x"); got != nil {
			t.Errorf("lookup of unrelated plaintext returned a " +
				"record — store must miss on hash mismatch " +
				"(R-SLGL-B5B4)")
		}
		if got := webSessionStore.Lookup(""); got != nil {
			t.Errorf("lookup of empty plaintext returned a record " +
				"(R-SLGL-B5B4)")
		}
	})

	t.Run("revoke_writes_revoked_at_and_blocks_redemption", func(t *testing.T) {
		webSessionStore := newWebSessionStorage()
		fixed := time.Date(2026, 5, 12, 10, 0, 0, 0, time.UTC)
		prev := webSessionNow
		webSessionNow = func() time.Time { return fixed }
		t.Cleanup(func() { webSessionNow = prev })

		plaintext, err := webSessionStore.Issue("revoke-slgl@example.com")
		if err != nil {
			t.Fatalf("issue: %v (R-SLGL-B5B4)", err)
		}

		webSessionStore.Revoke(plaintext)

		rec := webSessionStore.RecordForPlaintextForTest(plaintext)
		if rec == nil {
			t.Fatalf("revoke removed the record — must update in " +
				"place by setting revokedAt (R-SLGL-B5B4)")
		}
		if rec.RevokedAt().IsZero() {
			t.Errorf("revoke did not set revokedAt (R-SLGL-B5B4)")
		}
		if got := webSessionStore.Lookup(plaintext); got != nil {
			t.Errorf("revoked session still validates — same value " +
				"must not be redeemable again (R-SLGL-B5B4)")
		}
	})
}
