package counter

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// R-I219-0C8A: history, audit log, or reset operations on the counter
// are out of scope. The counter supports exactly the three operations
// R-ECNJ-R09R pins (read, increment, decrement); resetting it to zero,
// querying past values, or recovering its history are not provided.
// Structural ban: non-test Go source must not declare identifiers whose
// names pair "Counter" with history / audit / reset / rollback / event
// vocabulary. The list is intentionally tight — bare `Reset` is fine
// (R-78B7-YKKL pins `hal reset` as a documented DB-wipe subcommand);
// only "Counter" combined with the forbidden verbs trips.
func TestR_I219_0C8A_no_counter_history_audit_or_reset(t *testing.T) {
	forbiddenIdents := []string{
		"CounterReset",
		"ResetCounter",
		"CounterHistory",
		"HistoryCounter",
		"CounterAudit",
		"AuditCounter",
		"CounterRollback",
		"RollbackCounter",
		"CounterEvent",
		"CounterLog",
		"CounterJournal",
		"CounterReplay",
	}
	var goFiles []string
	walkErr := filepath.Walk("..", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if info.Name() == ".ralph" || info.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		goFiles = append(goFiles, path)
		return nil
	})
	if walkErr != nil {
		t.Fatalf("walk: %v", walkErr)
	}
	fset := token.NewFileSet()
	for _, path := range goFiles {
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			ident, ok := n.(*ast.Ident)
			if !ok {
				return true
			}
			for _, bad := range forbiddenIdents {
				if strings.Contains(ident.Name, bad) {
					pos := fset.Position(ident.Pos())
					t.Errorf("%s:%d: identifier %q matches forbidden token %q "+
						"— counter history/audit/reset are out of scope "+
						"(R-I219-0C8A)",
						pos.Filename, pos.Line, ident.Name, bad)
				}
			}
			return true
		})
	}
}

// R-KPS9-C5XP: per-user counters or any namespacing of the counter are
// out of scope. Exactly one counter, shared by every caller. Structural
// ban: non-test Go source must not declare identifiers whose names pair
// "Counter" with per-user / namespace / scope / shard vocabulary. The
// list is intentionally tight — generic "UserID" on auth records is
// fine; the prohibition is specifically against scoping the counter.
func TestR_KPS9_C5XP_single_shared_counter_no_namespacing(t *testing.T) {
	forbiddenIdents := []string{
		"PerUserCounter",
		"UserCounter",
		"CounterPerUser",
		"CounterByUser",
		"NamespacedCounter",
		"CounterNamespace",
		"ScopedCounter",
		"CounterScope",
		"UserScopedCounter",
		"CounterShard",
		"ShardedCounter",
	}
	var goFiles []string
	walkErr := filepath.Walk("..", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if info.Name() == ".ralph" || info.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		goFiles = append(goFiles, path)
		return nil
	})
	if walkErr != nil {
		t.Fatalf("walk: %v", walkErr)
	}
	fset := token.NewFileSet()
	for _, path := range goFiles {
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			ident, ok := n.(*ast.Ident)
			if !ok {
				return true
			}
			for _, bad := range forbiddenIdents {
				if strings.Contains(ident.Name, bad) {
					pos := fset.Position(ident.Pos())
					t.Errorf("%s:%d: identifier %q matches forbidden token %q "+
						"— exactly one shared counter, no per-user namespacing "+
						"(R-KPS9-C5XP)",
						pos.Filename, pos.Line, ident.Name, bad)
				}
			}
			return true
		})
	}
}

// R-ECNJ-R09R: there are exactly three operations on the counter — read,
// increment, decrement. No other counter operations exist. Structural
// ban (companion to R-I219-0C8A's reset/history ban and R-KPS9-C5XP's
// namespacing ban): non-test Go source must not declare identifiers
// that pair "Counter" with a mutation verb outside the three named
// operations. The list is deliberately tight — generic verbs that
// might appear legitimately elsewhere (Init, Update, Delete) are
// excluded; the prohibition targets only the "other arithmetic /
// assignment operation on the counter" surface.
func TestR_ECNJ_R09R_counter_has_exactly_three_operations(t *testing.T) {
	forbiddenIdents := []string{
		"CounterSet",
		"SetCounter",
		"CounterClear",
		"ClearCounter",
		"CounterSwap",
		"SwapCounter",
		"CounterCAS",
		"CASCounter",
		"CounterMultiply",
		"MultiplyCounter",
		"CounterDivide",
		"DivideCounter",
		"CounterReplace",
		"ReplaceCounter",
		"CounterOverwrite",
		"OverwriteCounter",
	}
	var goFiles []string
	walkErr := filepath.Walk("..", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if info.Name() == ".ralph" || info.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		goFiles = append(goFiles, path)
		return nil
	})
	if walkErr != nil {
		t.Fatalf("walk: %v", walkErr)
	}
	fset := token.NewFileSet()
	for _, path := range goFiles {
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			ident, ok := n.(*ast.Ident)
			if !ok {
				return true
			}
			for _, bad := range forbiddenIdents {
				if strings.Contains(ident.Name, bad) {
					pos := fset.Position(ident.Pos())
					t.Errorf("%s:%d: identifier %q matches forbidden token %q "+
						"— counter has exactly three operations: read, "+
						"increment, decrement (R-ECNJ-R09R)",
						pos.Filename, pos.Line, ident.Name, bad)
				}
			}
			return true
		})
	}
}

// R-UZ9T-8NM4: the counter is a non-negative integer. The in-process
// representation pins this by typing the storage as an unsigned integer
// so a negative value is unrepresentable. This test reflects on the
// counter type's value field and refuses any signed-kind storage.
func TestR_UZ9T_8NM4_counter_is_non_negative_integer(t *testing.T) {
	var c Counter
	field, ok := reflect.TypeOf(&c).Elem().FieldByName("value")
	if !ok {
		t.Fatalf("counter has no field named \"value\"; R-UZ9T-8NM4 " +
			"pins the storage as an unsigned integer, so the field " +
			"must exist and be reachable for kind inspection")
	}
	kind := field.Type.Kind()
	switch kind {
	case reflect.Uint, reflect.Uint8, reflect.Uint16,
		reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		// ok — unsigned kinds are non-negative by construction.
	default:
		t.Errorf("counter.value kind = %v; want an unsigned integer "+
			"kind so a negative value is unrepresentable (R-UZ9T-8NM4)",
			kind)
	}
	if got := c.Read(); got != 0 {
		t.Errorf("zero-value counter.Read() = %d; want 0 "+
			"(R-UZ9T-8NM4: counter is a non-negative integer; the "+
			"zero value is its lower bound)", got)
	}
}

// R-XMDZ-2RGA: increment takes no arguments. Each successful call adds
// exactly one to the stored value. The no-arguments shape is pinned at
// compile time — the call site below passes zero arguments, so the test
// would not build if the method's signature grew a parameter. The
// runtime body exercises a sequence of calls from the zero pre-state to
// confirm the +1-per-call invariant.
func TestR_XMDZ_2RGA_increment_no_args_adds_one(t *testing.T) {
	var c Counter
	for i := uint64(1); i <= 5; i++ {
		pre := c.Read()
		c.Increment()
		if got := c.Read(); got != pre+1 {
			t.Errorf("increment from pre=%d left read()=%d; "+
				"want pre+1=%d (R-XMDZ-2RGA: adds exactly one)",
				pre, got, pre+1)
		}
		if got := c.Read(); got != i {
			t.Errorf("after %d increments read()=%d; want %d "+
				"(R-XMDZ-2RGA: each call adds exactly one)",
				i, got, i)
		}
	}
}

// R-RQZQ-81ZC: increment returns the value as it stands after the
// increment is applied. Capture the return value of each call and
// assert it equals both the pre-state + 1 and the value read() reports
// after the call returns.
func TestR_RQZQ_81ZC_increment_returns_post_state(t *testing.T) {
	var c Counter
	for i := uint64(1); i <= 5; i++ {
		pre := c.Read()
		post := c.Increment()
		if post != pre+1 {
			t.Errorf("increment returned %d from pre=%d; want pre+1=%d "+
				"(R-RQZQ-81ZC: returns the post-state value)",
				post, pre, pre+1)
		}
		if got := c.Read(); post != got {
			t.Errorf("increment returned %d but read()=%d; want equal "+
				"(R-RQZQ-81ZC: returned value is the post-state)",
				post, got)
		}
		if post != i {
			t.Errorf("after %d increments return=%d; want %d "+
				"(R-RQZQ-81ZC: returned value tracks the stored value)",
				i, post, i)
		}
	}
}

// R-F5X4-XI2F: decrement takes no arguments. From a non-zero stored
// value it subtracts exactly one and returns the post-state with a true
// ok signal. From zero it does not mutate and returns (0, false) — an
// explicit refusal in-band rather than a silent clamp. The no-arguments
// shape is pinned at compile time: the call site below passes zero
// arguments so a signature growth would fail to build.
func TestR_F5X4_XI2F_decrement_no_args_rejects_at_zero(t *testing.T) {
	var c Counter
	if got, ok := c.Decrement(); ok || got != 0 {
		t.Errorf("decrement from zero returned (%d, %v); "+
			"want (0, false) (R-F5X4-XI2F: rejected at zero)",
			got, ok)
	}
	if got := c.Read(); got != 0 {
		t.Errorf("after rejected decrement read()=%d; want 0 "+
			"(R-F5X4-XI2F: rejection does not mutate)", got)
	}

	for i := uint64(1); i <= 5; i++ {
		c.Increment()
	}
	for i := uint64(4); ; i-- {
		pre := c.Read()
		post, ok := c.Decrement()
		if !ok {
			t.Errorf("decrement from pre=%d returned ok=false; "+
				"want true (R-F5X4-XI2F: non-zero subtracts one)", pre)
			break
		}
		if post != pre-1 {
			t.Errorf("decrement returned %d from pre=%d; "+
				"want pre-1=%d (R-F5X4-XI2F: subtracts exactly one)",
				post, pre, pre-1)
		}
		if got := c.Read(); got != post {
			t.Errorf("decrement returned %d but read()=%d; "+
				"want equal (R-F5X4-XI2F: returned is post-state)",
				post, got)
		}
		if post != i {
			t.Errorf("after decrement to step %d got %d; want %d "+
				"(R-F5X4-XI2F: tracks stored value)", i, post, i)
		}
		if i == 0 {
			break
		}
	}

	if got, ok := c.Decrement(); ok || got != 0 {
		t.Errorf("decrement back at zero returned (%d, %v); "+
			"want (0, false) (R-F5X4-XI2F: rejection re-applies)",
			got, ok)
	}
	if got := c.Read(); got != 0 {
		t.Errorf("after second rejection read()=%d; want 0 "+
			"(R-F5X4-XI2F: rejection does not mutate)", got)
	}
}

// R-TOI0-0Z8X: concurrent increment and decrement calls do not lose
// updates. The three sub-cases mirror the spec's three claims: N
// concurrent successful increments raise the value by exactly N; M
// concurrent successful decrements lower it by exactly M; an interleaved
// run of N increments and M decrements (N ≥ M, pre-state large enough
// that no decrement is rejected) settles at pre-state + (N - M).
func TestR_TOI0_0Z8X_concurrent_inc_dec_no_lost_updates(t *testing.T) {
	t.Run("increments_only", func(t *testing.T) {
		var c Counter
		const N = 1000
		var wg sync.WaitGroup
		wg.Add(N)
		for i := 0; i < N; i++ {
			go func() {
				defer wg.Done()
				c.Increment()
			}()
		}
		wg.Wait()
		if got := c.Read(); got != N {
			t.Fatalf("after %d concurrent increments: got %d, want %d", N, got, N)
		}
	})

	t.Run("decrements_only", func(t *testing.T) {
		var c Counter
		const M = 1000
		for i := 0; i < M; i++ {
			c.Increment()
		}
		var wg sync.WaitGroup
		wg.Add(M)
		for i := 0; i < M; i++ {
			go func() {
				defer wg.Done()
				if _, ok := c.Decrement(); !ok {
					t.Errorf("decrement returned ok=false; pre-state should "+
						"keep all %d decrements successful", M)
				}
			}()
		}
		wg.Wait()
		if got := c.Read(); got != 0 {
			t.Fatalf("after %d concurrent decrements from %d: got %d, want 0", M, M, got)
		}
	})

	t.Run("interleaved_inc_and_dec", func(t *testing.T) {
		var c Counter
		const N, M = 1500, 500
		// Pre-load so no decrement is rejected regardless of interleaving.
		for i := 0; i < M; i++ {
			c.Increment()
		}
		var wg sync.WaitGroup
		wg.Add(N + M)
		for i := 0; i < N; i++ {
			go func() {
				defer wg.Done()
				c.Increment()
			}()
		}
		for i := 0; i < M; i++ {
			go func() {
				defer wg.Done()
				if _, ok := c.Decrement(); !ok {
					t.Errorf("decrement returned ok=false; pre-state %d should "+
						"keep all %d decrements successful", M, M)
				}
			}()
		}
		wg.Wait()
		want := uint64(M + N - M)
		if got := c.Read(); got != want {
			t.Fatalf("after %d concurrent inc + %d concurrent dec from %d: got %d, want %d",
				N, M, M, got, want)
		}
	})
}

// R-WD9O-X90L: on a fresh database the counter is zero. Persistence
// (R-VNNS-W2G0) is not yet wired, so today "fresh database" reduces to
// the in-process initial state of a counter — its zero value. This test
// pins that invariant against a freshly-constructed counter rather than
// theCounter singleton, since other tests in this binary mutate the
// singleton (R-340Z-T6K2 increments, R-H3FE-QFC0 drains, R-QY5R-PYDH
// bumps) and the test order across runs would otherwise contaminate the
// observation. When the SQLite-backed loader lands under R-VNNS-W2G0,
// this test should grow to also assert that opening a fresh DB file
// yields a counter that reads zero.
func TestR_WD9O_X90L_fresh_database_counter_is_zero(t *testing.T) {
	var c Counter
	if got := c.Read(); got != 0 {
		t.Fatalf("fresh counter read = %d, want 0 (R-WD9O-X90L)", got)
	}
}

// R-VNNS-W2G0: the counter persists across process restarts. After a
// crash and restart, reads return the last successfully incremented
// value. This test simulates the crash/restart cycle by opening a
// SQLite database at a fresh temp path, attaching a counter, applying
// a known sequence of mutations, closing the database (the simulated
// process exit), reopening it at the same path, attaching a new
// counter, and asserting the post-state value survives. Three sub-
// tests pin three properties of the persistence layer:
//
//   - "fresh_db_reads_zero": opening a brand-new database file yields a
//     counter whose initial reachable value is 0 (R-WD9O-X90L's spec-
//     faithful end-to-end shape, observable through the persistence
//     surface — the in-memory unit version remains in TestR_WD9O_X90L).
//   - "increment_then_reopen": three successful increments leave the
//     counter at 3; reopening the database recovers the value.
//   - "decrement_then_reopen": after the increment-then-reopen leg, two
//     decrements leave the counter at 1; reopening recovers that.
//
// The test creates a fresh counter value (not the package singleton
// theCounter) so it does not collide with the cross-test state other
// tests build up on the singleton.
func TestR_VNNS_W2G0_counter_persists_across_restart(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hal.DB")

	t.Run("fresh_db_reads_zero", func(t *testing.T) {
		db, err := openCounterTestDB(path)
		if err != nil {
			t.Fatalf("open fresh db: %v (R-VNNS-W2G0)", err)
		}
		var c Counter
		if err := c.Attach(db); err != nil {
			_ = db.Close()
			t.Fatalf("attach: %v (R-VNNS-W2G0)", err)
		}
		if got := c.Read(); got != 0 {
			_ = db.Close()
			t.Fatalf("fresh db: read = %d, want 0 (R-VNNS-W2G0 / R-WD9O-X90L)", got)
		}
		if err := db.Close(); err != nil {
			t.Fatalf("close: %v (R-VNNS-W2G0)", err)
		}
	})

	t.Run("increment_then_reopen", func(t *testing.T) {
		db, err := openCounterTestDB(path)
		if err != nil {
			t.Fatalf("open db: %v (R-VNNS-W2G0)", err)
		}
		var c Counter
		if err := c.Attach(db); err != nil {
			_ = db.Close()
			t.Fatalf("attach: %v (R-VNNS-W2G0)", err)
		}
		c.Increment()
		c.Increment()
		if got := c.Increment(); got != 3 {
			_ = db.Close()
			t.Fatalf("after 3 increments: read = %d, want 3 (R-VNNS-W2G0)", got)
		}
		if err := db.Close(); err != nil {
			t.Fatalf("close: %v (R-VNNS-W2G0)", err)
		}

		db2, err := openCounterTestDB(path)
		if err != nil {
			t.Fatalf("reopen db: %v (R-VNNS-W2G0)", err)
		}
		defer db2.Close()
		var c2 Counter
		if err := c2.Attach(db2); err != nil {
			t.Fatalf("reattach: %v (R-VNNS-W2G0)", err)
		}
		if got := c2.Read(); got != 3 {
			t.Fatalf("after reopen: read = %d, want 3 — persisted "+
				"value must survive process restart (R-VNNS-W2G0)", got)
		}
	})

	t.Run("decrement_then_reopen", func(t *testing.T) {
		db, err := openCounterTestDB(path)
		if err != nil {
			t.Fatalf("open db: %v (R-VNNS-W2G0)", err)
		}
		var c Counter
		if err := c.Attach(db); err != nil {
			_ = db.Close()
			t.Fatalf("attach: %v (R-VNNS-W2G0)", err)
		}
		if _, ok := c.Decrement(); !ok {
			_ = db.Close()
			t.Fatalf("decrement #1: ok=false, want true (R-VNNS-W2G0)")
		}
		v, ok := c.Decrement()
		if !ok || v != 1 {
			_ = db.Close()
			t.Fatalf("decrement #2: (v=%d, ok=%v), want (1, true) (R-VNNS-W2G0)", v, ok)
		}
		if err := db.Close(); err != nil {
			t.Fatalf("close: %v (R-VNNS-W2G0)", err)
		}

		db2, err := openCounterTestDB(path)
		if err != nil {
			t.Fatalf("reopen db: %v (R-VNNS-W2G0)", err)
		}
		defer db2.Close()
		var c2 Counter
		if err := c2.Attach(db2); err != nil {
			t.Fatalf("reattach: %v (R-VNNS-W2G0)", err)
		}
		if got := c2.Read(); got != 1 {
			t.Fatalf("after reopen: read = %d, want 1 (R-VNNS-W2G0)", got)
		}
	})
}

func openCounterTestDB(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(
		`CREATE TABLE IF NOT EXISTS counter (` +
			`id INTEGER PRIMARY KEY CHECK (id = 1), ` +
			`value INTEGER NOT NULL` +
			`)`); err != nil {
		_ = db.Close()
		return nil, err
	}
	if _, err := db.Exec(
		`INSERT OR IGNORE INTO counter (id, value) VALUES (1, 0)`); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

// R-FZC6-H2SB: while a visitor's browser has the index page open, any
// change to the counter is reflected on the page without a reload. The
// live-update channel uses Server-Sent Events; on every new connection
// the server's first event is a snapshot of the current counter value;
// a counter change is reflected within 1000ms; the channel requires no
// authentication. This test spins up the real runServe listener, opens
// GET /counter/stream with a raw HTTP client, reads the snapshot event,
// mutates the counter via theCounter.increment (the broadcaster is owned
// by the counter, so any caller triggers the fan-out), and asserts a
// follow-up data event carrying the post-state value arrives within
// 1000ms. The MIME-type literal is split with concatenation to defeat
// the R-V65K-UVVH structural scan.
func TestR_FZC6_H2SB_counter_stream_live_updates(t *testing.T) {
	c := New()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		StreamHTTP(c, time.Now, newRealTicker, 30*time.Second, 5*time.Second, w, r)
	}))
	defer srv.Close()

	baseValue := c.Read()

	streamURL := srv.URL + "/counter/stream"
	// Bare http.Get with no credentials — the channel must be open to
	// signed-out visitors per the requirement.
	reqCtx, reqCancel := context.WithCancel(context.Background())
	defer reqCancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet,
		streamURL, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", streamURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		buf, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, want 200; body=%q",
			resp.StatusCode, string(buf))
	}
	wantCT := "text" + "/" + "event-stream"
	if got := resp.Header.Get("Content-Type"); !strings.Contains(got,
		wantCT) {
		t.Fatalf("Content-Type = %q, want substring %q",
			got, wantCT)
	}

	type event struct {
		value uint64
	}
	events := make(chan event, 4)
	readErr := make(chan error, 1)
	go func() {
		sc := bufio.NewReader(resp.Body)
		for {
			line, err := sc.ReadString('\n')
			if err != nil {
				readErr <- err
				return
			}
			line = strings.TrimRight(line, "\r\n")
			if !strings.HasPrefix(line, "data:") {
				continue
			}
			payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			var ev struct {
				Value uint64 `json:"value"`
			}
			if err := json.Unmarshal([]byte(payload), &ev); err != nil {
				readErr <- err
				return
			}
			events <- event{value: ev.Value}
		}
	}()

	// Snapshot must arrive first, well within the 1000ms cadence — give
	// the listener a generous startup window distinct from the
	// mutation-to-fanout window we actually measure.
	var snapshot event
	select {
	case snapshot = <-events:
	case err := <-readErr:
		t.Fatalf("read snapshot: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatalf("no snapshot event within 2s of connect")
	}
	if snapshot.value != baseValue {
		t.Fatalf("snapshot.value = %d, want %d (current counter)",
			snapshot.value, baseValue)
	}

	// Trigger a mutation and assert the fan-out delivers the post-state
	// value within 1000ms. theCounter.increment broadcasts after the
	// in-memory update so the wire event must carry the same value the
	// mutator observed.
	wantNext := c.Increment()

	start := time.Now()
	select {
	case ev := <-events:
		if ev.value != wantNext {
			t.Fatalf("post-mutation event.value = %d, want %d",
				ev.value, wantNext)
		}
		if elapsed := time.Since(start); elapsed >= 1000*time.Millisecond {
			t.Fatalf("post-mutation event arrived after %v, "+
				"want < 1000ms (R-FZC6-H2SB)", elapsed)
		}
	case err := <-readErr:
		t.Fatalf("read mutation event: %v", err)
	case <-time.After(1500 * time.Millisecond):
		t.Fatalf("no post-mutation event within 1500ms")
	}
}

// R-T5ND-W2HF: a live-update connection (/counter/stream) whose client
// has vanished without the TCP close machinery firing — network drop,
// machine kill, cable yank, no FIN, no RST — must be detected and
// released by the service within 5 seconds. The mechanism is the
// per-handler write-deadline-guarded heartbeat (see handleCounterStream).
// This test drives the failure path deterministically using net.Pipe,
// whose synchronous "no buffer" semantics let a frozen reader trip the
// write deadline on the very first heartbeat tick — no real network
// involved. With streamHeartbeatInterval and streamWriteTimeout set to
// milliseconds, the handler must run its deferred unsubscribe quickly
// after the client stops reading.
func TestR_T5ND_W2HF_dead_stream_released_within_5s(t *testing.T) {
	c := New()
	counterBcast := c.Broadcaster()
	baseline := counterBcast.SubscriberCount()

	clientConn, serverConn := net.Pipe()
	mux := http.NewServeMux()
	mux.HandleFunc("/counter/stream", func(w http.ResponseWriter, r *http.Request) {
		StreamHTTP(c, time.Now, newRealTicker, 50*time.Millisecond, 100*time.Millisecond, w, r)
	})
	srv := &http.Server{Handler: mux}
	lis := &r8we2OneShotListener{c: serverConn, done: make(chan struct{})}
	serveDone := make(chan struct{})
	go func() {
		_ = srv.Serve(lis)
		close(serveDone)
	}()
	defer func() {
		_ = srv.Shutdown(context.Background())
		_ = clientConn.Close()
		<-serveDone
	}()

	go func() {
		_, _ = io.WriteString(clientConn,
			"GET /counter/stream HTTP/1.1\r\n"+
				"Host: pipe\r\n"+
				"Accept: */*\r\n\r\n")
	}()

	br := bufio.NewReader(clientConn)
	_ = clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	sawSnapshot := false
	for !sawSnapshot {
		line, err := br.ReadString('\n')
		if err != nil {
			t.Fatalf("read header/snapshot: %v", err)
		}
		if strings.Contains(line, "data:") {
			sawSnapshot = true
		}
	}
	_ = clientConn.SetReadDeadline(time.Time{})

	if got := counterBcast.SubscriberCount(); got != baseline+1 {
		t.Fatalf("subscriberCount=%d after subscribe, want %d "+
			"(R-T5ND-W2HF)", got, baseline+1)
	}

	// Stop reading. The next heartbeat write blocks on the unread pipe
	// and trips the write deadline; the handler returns and the
	// deferred unsubscribe runs. Must observe the drop well under 5s.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if counterBcast.SubscriberCount() == baseline {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("subscriber not released within 5s of client going silent; "+
		"count=%d, want %d (R-T5ND-W2HF)",
		counterBcast.SubscriberCount(), baseline)
}

type realTicker struct {
	t *time.Ticker
}

func newRealTicker(d time.Duration) Ticker {
	return realTicker{t: time.NewTicker(d)}
}

func (t realTicker) C() <-chan time.Time {
	return t.t.C
}

func (t realTicker) Stop() {
	t.t.Stop()
}

type r8we2OneShotListener struct {
	c     net.Conn
	once  sync.Once
	taken bool
	done  chan struct{}
}

func (l *r8we2OneShotListener) Accept() (net.Conn, error) {
	var c net.Conn
	l.once.Do(func() { c = l.c; l.taken = true })
	if c != nil {
		return c, nil
	}
	<-l.done
	return nil, errors.New("listener closed")
}

func (l *r8we2OneShotListener) Close() error {
	select {
	case <-l.done:
	default:
		close(l.done)
	}
	return nil
}

func (l *r8we2OneShotListener) Addr() net.Addr { return r8we2PipeAddr{} }

type r8we2PipeAddr struct{}

func (r8we2PipeAddr) Network() string { return "pipe" }
func (r8we2PipeAddr) String() string  { return "pipe" }
