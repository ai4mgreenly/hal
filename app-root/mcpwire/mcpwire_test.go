package mcpwire_test

import (
	"bytes"
	"context"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	counterpkg "github.com/mgreenly/hal/counter"
	mcpwirepkg "github.com/mgreenly/hal/mcpwire"
	oauthpkg "github.com/mgreenly/hal/oauth"
)

var oauthTokenStore = oauthpkg.NewTokenStore(oauthpkg.TokenOptions{})
var theCounter = counterpkg.New()
var onListenerReady func(net.Addr)

func canonicalResourceIdentifier() string {
	return "http://127.0.0.1:3000/mcp"
}

func contextWithTestStores(ctx context.Context) context.Context {
	return ctx
}

func runServeForTest(t testing.TB, ctx context.Context, _ []string, _, _ io.Writer) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	mux := http.NewServeMux()
	mux.Handle("/mcp", mcpwirepkg.Surface{
		Counter:                     theCounter,
		OAuthTokens:                 oauthTokenStore,
		CanonicalResourceIdentifier: canonicalResourceIdentifier,
		Version:                     "test",
	}.Handler())
	srv := &http.Server{Handler: mux}
	if onListenerReady != nil {
		onListenerReady(ln.Addr())
	}
	done := make(chan error, 1)
	go func() { done <- srv.Serve(ln) }()
	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
	if err := <-done; err != nil && err != http.ErrServerClosed {
		t.Fatalf("serve: %v", err)
	}
	return 0
}

// R-325I-TX6C: the MCP server is built on the official MCP Go SDK
// (github.com/modelcontextprotocol/go-sdk). JSON-RPC and transport framing
// are not hand-rolled. Structural test asserts (a) go.mod requires the SDK
// module, (b) the extracted MCP wiring package imports the SDK's mcp
// subpackage, and (c) that package constructs a server through the SDK
// constructor (mcp.NewServer) rather than wiring a hand-rolled stand-in.
func TestR_325I_TX6C_mcp_server_built_on_official_sdk(t *testing.T) {
	const sdkModule = "github.com/modelcontextprotocol/go-sdk"
	const sdkPkg = sdkModule + "/mcp"

	gomod, err := os.ReadFile(filepath.Join("..", "go.mod"))
	if err != nil {
		t.Fatalf("read go.mod: %v", err)
	}
	requireRe := regexp.MustCompile(`(?m)^\s*` + regexp.QuoteMeta(sdkModule) + `\s+v\S+`)
	if !requireRe.Match(gomod) {
		t.Errorf("go.mod does not require %s; MCP server must be built on the official SDK", sdkModule)
	}

	src, err := os.ReadFile("mcpwire.go")
	if err != nil {
		t.Fatalf("read mcpwire/mcpwire.go: %v", err)
	}
	if !strings.Contains(string(src), `"`+sdkPkg+`"`) {
		t.Errorf("mcpwire does not import %q; MCP server must be built on the official SDK", sdkPkg)
	}
	if !strings.Contains(string(src), "mcp.NewServer(") {
		t.Errorf("mcpwire does not call mcp.NewServer(...); MCP server must be constructed via the SDK")
	}
}

// R-UK7D-Z0IZ: the MCP server speaks the Streamable HTTP transport. Behavioral
// test: spin up a real runServe listener, POST a JSON-RPC initialize request
// to /mcp, and assert that the SDK-backed handler responds 200 with a
// well-formed initialize result naming the "hal" server. A request that
// reaches an unrouted path on this mux returns 404; receiving a valid MCP
// initialize result proves both that /mcp is wired and that the response is
// framed by the SDK's Streamable HTTP transport — exactly what R-UK7D-Z0IZ
// requires.
func TestR_UK7D_Z0IZ_mcp_streamable_http_transport(t *testing.T) {
	ready := make(chan net.Addr, 1)
	onListenerReady = func(a net.Addr) { ready <- a }
	defer func() { onListenerReady = nil }()

	ctx, cancel := context.WithCancel(contextWithTestStores(context.Background()))
	defer cancel()
	var stdout, stderr bytes.Buffer
	done := make(chan int, 1)
	go func() {
		done <- runServeForTest(t, ctx, []string{"--port", "0"}, &stdout, &stderr)
	}()

	var addr net.Addr
	select {
	case addr = <-ready:
	case <-time.After(2 * time.Second):
		cancel()
		<-done
		t.Fatalf("listener never ready within 2s; stderr=%q", stderr.String())
	}
	defer func() {
		cancel()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatalf("runServe did not exit within 2s after cancel")
		}
	}()

	url := "http://" + addr.String() + "/mcp"
	body := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize",` +
		`"params":{"protocolVersion":"2025-11-25","capabilities":{},` +
		`"clientInfo":{"name":"hal-test","version":"0.0.1"}}}`)
	req, err := http.NewRequest(http.MethodPost, url, body)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, "+"text"+"/"+"event-stream")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		buf, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, want 200; body=%q", resp.StatusCode, string(buf))
	}
	buf, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	var rpc struct {
		JSONRPC string `json:"jsonrpc"`
		ID      any    `json:"id"`
		Result  struct {
			ProtocolVersion string `json:"protocolVersion"`
			ServerInfo      struct {
				Name    string `json:"name"`
				Version string `json:"version"`
			} `json:"serverInfo"`
		} `json:"result"`
		Error any `json:"error"`
	}
	if err := json.Unmarshal(buf, &rpc); err != nil {
		t.Fatalf("decode JSON-RPC response: %v; body=%q", err, string(buf))
	}
	if rpc.JSONRPC != "2.0" {
		t.Fatalf("jsonrpc = %q, want %q; body=%q", rpc.JSONRPC, "2.0", string(buf))
	}
	if rpc.Error != nil {
		t.Fatalf("response carried error: %v; body=%q", rpc.Error, string(buf))
	}
	if rpc.Result.ServerInfo.Name != "hal" {
		t.Fatalf("serverInfo.name = %q, want %q; body=%q",
			rpc.Result.ServerInfo.Name, "hal", string(buf))
	}
	if rpc.Result.ProtocolVersion == "" {
		t.Fatalf("result.protocolVersion empty; body=%q", string(buf))
	}
}

// R-XS1U-B7YY: the read tool accepts no arguments and returns the current
// counter value as a non-negative integer. Behavioral test: spin up runServe,
// initialize an MCP session over /mcp, then issue a tools/call for
// counter_read and assert the returned structured value equals theCounter's
// current value (0 on a fresh process). The tool is invoked without any
// bearer credentials, also exercising R-0CQ7-DSBQ's "read may be
// unauthenticated".
func TestR_XS1U_B7YY_mcp_counter_read_tool(t *testing.T) {
	ready := make(chan net.Addr, 1)
	onListenerReady = func(a net.Addr) { ready <- a }
	defer func() { onListenerReady = nil }()

	ctx, cancel := context.WithCancel(contextWithTestStores(context.Background()))
	defer cancel()
	var stdout, stderr bytes.Buffer
	done := make(chan int, 1)
	go func() {
		done <- runServeForTest(t, ctx, []string{"--port", "0"}, &stdout, &stderr)
	}()

	var addr net.Addr
	select {
	case addr = <-ready:
	case <-time.After(2 * time.Second):
		cancel()
		<-done
		t.Fatalf("listener never ready within 2s; stderr=%q", stderr.String())
	}
	defer func() {
		cancel()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatalf("runServe did not exit within 2s after cancel")
		}
	}()

	mcpURL := "http://" + addr.String() + "/mcp"
	acceptHeader := "application/json, " + "text" + "/" + "event-stream"

	post := func(payload string, sessionID string) (*http.Response, []byte) {
		t.Helper()
		req, err := http.NewRequest(http.MethodPost, mcpURL, strings.NewReader(payload))
		if err != nil {
			t.Fatalf("new request: %v", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", acceptHeader)
		if sessionID != "" {
			req.Header.Set("Mcp-Session-Id", sessionID)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("POST %s: %v", mcpURL, err)
		}
		buf, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		return resp, buf
	}

	initBody := `{"jsonrpc":"2.0","id":1,"method":"initialize",` +
		`"params":{"protocolVersion":"2025-11-25","capabilities":{},` +
		`"clientInfo":{"name":"hal-test","version":"0.0.1"}}}`
	resp, buf := post(initBody, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("initialize status = %d, want 200; body=%q", resp.StatusCode, string(buf))
	}
	sessionID := resp.Header.Get("Mcp-Session-Id")
	if sessionID == "" {
		t.Fatalf("initialize did not return Mcp-Session-Id header; body=%q", string(buf))
	}

	want := theCounter.Read()

	callBody := `{"jsonrpc":"2.0","id":2,"method":"tools/call",` +
		`"params":{"name":"counter_read","arguments":{}}}`
	resp, buf = post(callBody, sessionID)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("tools/call status = %d, want 200; body=%q", resp.StatusCode, string(buf))
	}

	var rpc struct {
		JSONRPC string `json:"jsonrpc"`
		Result  struct {
			IsError           bool `json:"isError"`
			StructuredContent struct {
				Value uint64 `json:"value"`
			} `json:"structuredContent"`
		} `json:"result"`
		Error any `json:"error"`
	}
	if err := json.Unmarshal(buf, &rpc); err != nil {
		t.Fatalf("decode JSON-RPC response: %v; body=%q", err, string(buf))
	}
	if rpc.Error != nil {
		t.Fatalf("tools/call carried error: %v; body=%q", rpc.Error, string(buf))
	}
	if rpc.Result.IsError {
		t.Fatalf("tools/call result.isError = true; body=%q", string(buf))
	}
	if rpc.Result.StructuredContent.Value != want {
		t.Fatalf("counter_read returned %d, want %d; body=%q",
			rpc.Result.StructuredContent.Value, want, string(buf))
	}
}

// R-YHNQ-CEJJ: the increment tool accepts no arguments; on success it
// adds one to the counter and returns the post-increment value.
// Behavioral test: spin up runServe, initialize an MCP session, snapshot
// the counter's current value, call counter_increment via tools/call, and
// assert the returned structured value is exactly prev+1 and that the
// in-process counter advanced by one. Asserting a delta rather than an
// absolute value tolerates the package-level singleton's state carrying
// over from earlier tests in the same `go test` binary.
func TestR_YHNQ_CEJJ_mcp_counter_increment_tool(t *testing.T) {
	ready := make(chan net.Addr, 1)
	onListenerReady = func(a net.Addr) { ready <- a }
	defer func() { onListenerReady = nil }()

	ctx, cancel := context.WithCancel(contextWithTestStores(context.Background()))
	defer cancel()
	var stdout, stderr bytes.Buffer
	done := make(chan int, 1)
	go func() {
		done <- runServeForTest(t, ctx, []string{"--port", "0"}, &stdout, &stderr)
	}()

	var addr net.Addr
	select {
	case addr = <-ready:
	case <-time.After(2 * time.Second):
		cancel()
		<-done
		t.Fatalf("listener never ready within 2s; stderr=%q", stderr.String())
	}
	defer func() {
		cancel()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatalf("runServe did not exit within 2s after cancel")
		}
	}()

	mcpURL := "http://" + addr.String() + "/mcp"
	acceptHeader := "application/json, " + "text" + "/" + "event-stream"

	// R-ZQS0-HWZ8: counter_increment now requires a valid bearer access
	// token issued by this service. Mint one bound to the canonical
	// resource so the success path is observable.
	bearer, err := oauthTokenStore.IssueAccess(
		"alice@example.com", "client-yhnq", canonicalResourceIdentifier(),
	)
	if err != nil {
		t.Fatalf("issueAccess: %v", err)
	}

	post := func(payload string, sessionID, bearer string) (*http.Response, []byte) {
		t.Helper()
		req, err := http.NewRequest(http.MethodPost, mcpURL, strings.NewReader(payload))
		if err != nil {
			t.Fatalf("new request: %v", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", acceptHeader)
		if sessionID != "" {
			req.Header.Set("Mcp-Session-Id", sessionID)
		}
		if bearer != "" {
			req.Header.Set("Authorization", "Bearer "+bearer)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("POST %s: %v", mcpURL, err)
		}
		buf, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		return resp, buf
	}

	initBody := `{"jsonrpc":"2.0","id":1,"method":"initialize",` +
		`"params":{"protocolVersion":"2025-11-25","capabilities":{},` +
		`"clientInfo":{"name":"hal-test","version":"0.0.1"}}}`
	resp, buf := post(initBody, "", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("initialize status = %d, want 200; body=%q", resp.StatusCode, string(buf))
	}
	sessionID := resp.Header.Get("Mcp-Session-Id")
	if sessionID == "" {
		t.Fatalf("initialize did not return Mcp-Session-Id header; body=%q", string(buf))
	}

	before := theCounter.Read()

	callBody := `{"jsonrpc":"2.0","id":2,"method":"tools/call",` +
		`"params":{"name":"counter_increment","arguments":{}}}`
	resp, buf = post(callBody, sessionID, bearer)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("tools/call status = %d, want 200; body=%q", resp.StatusCode, string(buf))
	}

	var rpc struct {
		JSONRPC string `json:"jsonrpc"`
		Result  struct {
			IsError           bool `json:"isError"`
			StructuredContent struct {
				Value uint64 `json:"value"`
			} `json:"structuredContent"`
		} `json:"result"`
		Error any `json:"error"`
	}
	if err := json.Unmarshal(buf, &rpc); err != nil {
		t.Fatalf("decode JSON-RPC response: %v; body=%q", err, string(buf))
	}
	if rpc.Error != nil {
		t.Fatalf("tools/call carried error: %v; body=%q", rpc.Error, string(buf))
	}
	if rpc.Result.IsError {
		t.Fatalf("tools/call result.isError = true; body=%q", string(buf))
	}
	if rpc.Result.StructuredContent.Value != before+1 {
		t.Fatalf("counter_increment returned %d, want %d; body=%q",
			rpc.Result.StructuredContent.Value, before+1, string(buf))
	}
	if got := theCounter.Read(); got != before+1 {
		t.Fatalf("theCounter.Read() = %d after increment, want %d", got, before+1)
	}
}

// R-ZQS0-HWZ8: a request that invokes the increment tool requires a
// valid bearer access token issued by this service. Bad-but-present bearer
// credentials are rejected at the HTTP boundary per R-51PZ-MEQR; this test
// keeps R-ZQS0-HWZ8's increment-specific no-mutation property pinned.
//
// The positive bearer-accepted path is covered by
// TestR_YHNQ_CEJJ_mcp_counter_increment_tool, which now mints a token
// and presents it; this test exercises only the rejection paths so
// the gate's behavior is observable in isolation.
func TestR_ZQS0_HWZ8_mcp_increment_requires_bearer(t *testing.T) {
	ready := make(chan net.Addr, 1)
	onListenerReady = func(a net.Addr) { ready <- a }
	defer func() { onListenerReady = nil }()

	ctx, cancel := context.WithCancel(contextWithTestStores(context.Background()))
	defer cancel()
	var stdout, stderr bytes.Buffer
	done := make(chan int, 1)
	go func() {
		done <- runServeForTest(t, ctx, []string{"--port", "0"}, &stdout, &stderr)
	}()

	var addr net.Addr
	select {
	case addr = <-ready:
	case <-time.After(2 * time.Second):
		cancel()
		<-done
		t.Fatalf("listener never ready within 2s; stderr=%q", stderr.String())
	}
	defer func() {
		cancel()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatalf("runServe did not exit within 2s after cancel")
		}
	}()

	mcpURL := "http://" + addr.String() + "/mcp"
	acceptHeader := "application/json, " + "text" + "/" + "event-stream"

	post := func(payload string, sessionID, bearer string) (*http.Response, []byte) {
		t.Helper()
		req, err := http.NewRequest(http.MethodPost, mcpURL, strings.NewReader(payload))
		if err != nil {
			t.Fatalf("new request: %v", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", acceptHeader)
		if sessionID != "" {
			req.Header.Set("Mcp-Session-Id", sessionID)
		}
		if bearer != "" {
			req.Header.Set("Authorization", "Bearer "+bearer)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("POST %s: %v", mcpURL, err)
		}
		buf, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		return resp, buf
	}

	initBody := `{"jsonrpc":"2.0","id":1,"method":"initialize",` +
		`"params":{"protocolVersion":"2025-11-25","capabilities":{},` +
		`"clientInfo":{"name":"hal-test","version":"0.0.1"}}}`
	resp, buf := post(initBody, "", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("initialize status = %d, want 200; body=%q",
			resp.StatusCode, string(buf))
	}
	sessionID := resp.Header.Get("Mcp-Session-Id")
	if sessionID == "" {
		t.Fatalf("initialize did not return Mcp-Session-Id; body=%q", string(buf))
	}

	// A token bound to a non-canonical resource — the gate must reject
	// it on the resource-binding mismatch, even though the bearer is
	// otherwise a valid service-issued token.
	mismatched, err := oauthTokenStore.IssueAccess(
		"alice@example.com", "client-zqs0", canonicalResourceIdentifier()+"x",
	)
	if err != nil {
		t.Fatalf("issueAccess (mismatched): %v", err)
	}

	// The no-credentials case has its own HTTP-level prompt-signal test
	// (TestR_0YOE_9NO8_*); R-ZQS0-HWZ8 here pins the bad-but-present
	// bearer paths leave the counter unchanged.
	cases := []struct {
		name   string
		bearer string
	}{
		{"unrecognized_bearer", "deadbeef-not-an-issued-token"},
		{"resource_binding_mismatch", mismatched},
	}

	callBody := `{"jsonrpc":"2.0","id":2,"method":"tools/call",` +
		`"params":{"name":"counter_increment","arguments":{}}}`

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			before := theCounter.Read()
			resp, buf := post(callBody, sessionID, tc.bearer)
			if resp.StatusCode != http.StatusUnauthorized {
				t.Fatalf("tools/call status = %d, want 401 at HTTP boundary; "+
					"body=%q", resp.StatusCode, string(buf))
			}
			if got := theCounter.Read(); got != before {
				t.Fatalf("counter advanced past R-ZQS0-HWZ8 gate: before=%d after=%d",
					before, got)
			}
		})
	}
}

// R-0YOE-9NO8: when an MCP client presents no credentials and the
// requested tool needs them, the server responds with the standard
// signal that prompts a conformant client to start the OAuth flow.
// Concretely: HTTP 401 with `WWW-Authenticate: Bearer ...
// resource_metadata="<base>/.well-known/oauth-protected-resource/mcp"`
// (R-7BHQ-VB64).
// The signal fires only for tools that require credentials; an
// unauthenticated call to counter_read (R-0CQ7-DSBQ) passes through
// to the SDK handler and returns a normal 200 tool result, and a
// request that presents an invalid Authorization header is rejected at
// the HTTP boundary per R-51PZ-MEQR.
func TestR_0YOE_9NO8_mcp_no_credentials_prompt_signal(t *testing.T) {
	ready := make(chan net.Addr, 1)
	onListenerReady = func(a net.Addr) { ready <- a }
	defer func() { onListenerReady = nil }()

	ctx, cancel := context.WithCancel(contextWithTestStores(context.Background()))
	defer cancel()
	var stdout, stderr bytes.Buffer
	done := make(chan int, 1)
	go func() {
		done <- runServeForTest(t, ctx, []string{"--port", "0"}, &stdout, &stderr)
	}()

	var addr net.Addr
	select {
	case addr = <-ready:
	case <-time.After(2 * time.Second):
		cancel()
		<-done
		t.Fatalf("listener never ready within 2s; stderr=%q", stderr.String())
	}
	defer func() {
		cancel()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatalf("runServe did not exit within 2s after cancel")
		}
	}()

	mcpURL := "http://" + addr.String() + "/mcp"
	acceptHeader := "application/json, " + "text" + "/" + "event-stream"

	post := func(payload, sessionID, bearer string) (*http.Response, []byte) {
		t.Helper()
		req, err := http.NewRequest(http.MethodPost, mcpURL, strings.NewReader(payload))
		if err != nil {
			t.Fatalf("new request: %v", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", acceptHeader)
		if sessionID != "" {
			req.Header.Set("Mcp-Session-Id", sessionID)
		}
		if bearer != "" {
			req.Header.Set("Authorization", "Bearer "+bearer)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("POST %s: %v", mcpURL, err)
		}
		buf, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		return resp, buf
	}

	initBody := `{"jsonrpc":"2.0","id":1,"method":"initialize",` +
		`"params":{"protocolVersion":"2025-11-25","capabilities":{},` +
		`"clientInfo":{"name":"hal-test","version":"0.0.1"}}}`
	resp, buf := post(initBody, "", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("initialize status = %d, want 200; body=%q",
			resp.StatusCode, string(buf))
	}
	sessionID := resp.Header.Get("Mcp-Session-Id")
	if sessionID == "" {
		t.Fatalf("initialize did not return Mcp-Session-Id; body=%q", string(buf))
	}

	incBody := `{"jsonrpc":"2.0","id":2,"method":"tools/call",` +
		`"params":{"name":"counter_increment","arguments":{}}}`
	readBody := `{"jsonrpc":"2.0","id":3,"method":"tools/call",` +
		`"params":{"name":"counter_read","arguments":{}}}`

	t.Run("no_credentials_increment_returns_401_with_www_authenticate", func(t *testing.T) {
		before := theCounter.Read()
		resp, buf := post(incBody, sessionID, "")
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401; body=%q",
				resp.StatusCode, string(buf))
		}
		wa := resp.Header.Get("WWW-Authenticate")
		if wa == "" {
			t.Fatalf("no WWW-Authenticate header (R-0YOE-9NO8); body=%q",
				string(buf))
		}
		if !strings.HasPrefix(strings.ToLower(wa), "bearer") {
			t.Fatalf("WWW-Authenticate scheme = %q, want Bearer (R-0YOE-9NO8)",
				wa)
		}
		expected := `resource_metadata="http://` + addr.String() +
			`/.well-known/oauth-protected-resource/mcp"`
		if !strings.Contains(wa, expected) {
			t.Fatalf("WWW-Authenticate = %q, want substring %q (R-0YOE-9NO8)",
				wa, expected)
		}
		if got := theCounter.Read(); got != before {
			t.Fatalf("counter advanced past prompt-signal: before=%d after=%d",
				before, got)
		}
	})

	t.Run("no_credentials_read_passes_through", func(t *testing.T) {
		resp, buf := post(readBody, sessionID, "")
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("counter_read no-cred status = %d, want 200 "+
				"(R-0CQ7-DSBQ pass-through); body=%q",
				resp.StatusCode, string(buf))
		}
		if wa := resp.Header.Get("WWW-Authenticate"); wa != "" {
			t.Fatalf("unexpected WWW-Authenticate on read: %q", wa)
		}
	})

	t.Run("bad_bearer_increment_rejected_at_http_boundary", func(t *testing.T) {
		before := theCounter.Read()
		resp, buf := post(incBody, sessionID, "deadbeef-not-issued")
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("bad-bearer status = %d, want 401 "+
				"(R-51PZ-MEQR HTTP-boundary gate); body=%q",
				resp.StatusCode, string(buf))
		}
		wa := resp.Header.Get("WWW-Authenticate")
		if !strings.Contains(wa, `error="invalid_token"`) {
			t.Fatalf("WWW-Authenticate = %q, want invalid_token "+
				"(R-51PZ-MEQR)", wa)
		}
		if got := theCounter.Read(); got != before {
			t.Fatalf("counter advanced past HTTP-boundary gate: before=%d after=%d",
				before, got)
		}
	})
}

// R-6UUW-TQP2: an issued access token grants the holder permission to
// call the increment tool. There are no finer-grained scopes — owner
// identity, client_id, and the moment of issue do not constrain which
// tokens may invoke counter_increment. This property test mints a
// cartesian of (owner, client) pairs (all bound to the canonical
// resource, since R-DH2I-28CK / R-ZQS0-HWZ8 already pin the
// resource-binding axis) and asserts each token successfully advances
// the counter through the gated MCP tool. Together with
// TestR_ZQS0_HWZ8_mcp_increment_requires_bearer (which exercises the
// rejection paths) and TestR_YHNQ_CEJJ_mcp_counter_increment_tool
// (which exercises the wire shape) this pins the "any service-issued
// access token suffices" half of the grant rule.
func TestR_6UUW_TQP2_AccessTokenGrantsIncrement(t *testing.T) {
	ready := make(chan net.Addr, 1)
	onListenerReady = func(a net.Addr) { ready <- a }
	defer func() { onListenerReady = nil }()

	ctx, cancel := context.WithCancel(contextWithTestStores(context.Background()))
	defer cancel()
	var stdout, stderr bytes.Buffer
	done := make(chan int, 1)
	go func() {
		done <- runServeForTest(t, ctx, []string{"--port", "0"}, &stdout, &stderr)
	}()

	var addr net.Addr
	select {
	case addr = <-ready:
	case <-time.After(2 * time.Second):
		cancel()
		<-done
		t.Fatalf("listener never ready within 2s; stderr=%q", stderr.String())
	}
	defer func() {
		cancel()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatalf("runServe did not exit within 2s after cancel")
		}
	}()

	mcpURL := "http://" + addr.String() + "/mcp"
	acceptHeader := "application/json, " + "text" + "/" + "event-stream"

	post := func(payload string, sessionID, bearer string) (*http.Response, []byte) {
		t.Helper()
		req, err := http.NewRequest(http.MethodPost, mcpURL, strings.NewReader(payload))
		if err != nil {
			t.Fatalf("new request: %v", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", acceptHeader)
		if sessionID != "" {
			req.Header.Set("Mcp-Session-Id", sessionID)
		}
		if bearer != "" {
			req.Header.Set("Authorization", "Bearer "+bearer)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("POST %s: %v", mcpURL, err)
		}
		buf, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		return resp, buf
	}

	initBody := `{"jsonrpc":"2.0","id":1,"method":"initialize",` +
		`"params":{"protocolVersion":"2025-11-25","capabilities":{},` +
		`"clientInfo":{"name":"hal-test","version":"0.0.1"}}}`
	resp, buf := post(initBody, "", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("initialize status = %d, want 200; body=%q",
			resp.StatusCode, string(buf))
	}
	sessionID := resp.Header.Get("Mcp-Session-Id")
	if sessionID == "" {
		t.Fatalf("initialize did not return Mcp-Session-Id; body=%q", string(buf))
	}

	owners := []string{"alice@example.com", "bob@example.com", "carol@example.com"}
	clients := []string{"client-6uuw-a", "client-6uuw-b"}

	callBody := `{"jsonrpc":"2.0","id":2,"method":"tools/call",` +
		`"params":{"name":"counter_increment","arguments":{}}}`

	resource := canonicalResourceIdentifier()

	for _, owner := range owners {
		for _, client := range clients {
			name := owner + "/" + client
			t.Run(name, func(t *testing.T) {
				bearer, err := oauthTokenStore.IssueAccess(owner, client, resource)
				if err != nil {
					t.Fatalf("issueAccess(%q,%q): %v", owner, client, err)
				}
				before := theCounter.Read()
				resp, buf := post(callBody, sessionID, bearer)
				if resp.StatusCode != http.StatusOK {
					t.Fatalf("tools/call status = %d, want 200; body=%q",
						resp.StatusCode, string(buf))
				}
				var rpc struct {
					Result struct {
						IsError           bool `json:"isError"`
						StructuredContent struct {
							Value uint64 `json:"value"`
						} `json:"structuredContent"`
					} `json:"result"`
					Error any `json:"error"`
				}
				if err := json.Unmarshal(buf, &rpc); err != nil {
					t.Fatalf("decode: %v; body=%q", err, string(buf))
				}
				if rpc.Error != nil {
					t.Fatalf("JSON-RPC error: %v; body=%q", rpc.Error, string(buf))
				}
				if rpc.Result.IsError {
					t.Fatalf("token from issueAccess(%q,%q) rejected at gate; body=%q",
						owner, client, string(buf))
				}
				if rpc.Result.StructuredContent.Value != before+1 {
					t.Fatalf("counter_increment returned %d, want %d; body=%q",
						rpc.Result.StructuredContent.Value, before+1, string(buf))
				}
				if got := theCounter.Read(); got != before+1 {
					t.Fatalf("theCounter.Read() = %d, want %d", got, before+1)
				}
			})
		}
	}
}

// R-GG9B-GS8T: the decrement tool accepts no arguments. When the counter is
// greater than zero, calling it returns the post-decrement value and
// modifies the counter. When the counter is exactly zero, calling it
// returns the standard MCP tool-error signal naming the cause and does not
// modify the counter. Behavioral test: spin up runServe, initialize an MCP
// session, present a valid bearer access token per R-285U-FWW3, drive the
// counter to a known nonzero state via direct increment (the package-level
// singleton's value carries across tests in the same `go test` binary, so
// we assert deltas, not absolutes), call counter_decrement and assert
// prev-1, then drain the counter to zero via direct calls and assert the
// next authenticated decrement returns isError with the documented message
// and that theCounter.Read() is still zero.
func TestR_GG9B_GS8T_mcp_counter_decrement_tool(t *testing.T) {
	ready := make(chan net.Addr, 1)
	onListenerReady = func(a net.Addr) { ready <- a }
	defer func() { onListenerReady = nil }()

	ctx, cancel := context.WithCancel(contextWithTestStores(context.Background()))
	defer cancel()
	var stdout, stderr bytes.Buffer
	done := make(chan int, 1)
	go func() {
		done <- runServeForTest(t, ctx, []string{"--port", "0"}, &stdout, &stderr)
	}()

	var addr net.Addr
	select {
	case addr = <-ready:
	case <-time.After(2 * time.Second):
		cancel()
		<-done
		t.Fatalf("listener never ready within 2s; stderr=%q", stderr.String())
	}
	defer func() {
		cancel()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatalf("runServe did not exit within 2s after cancel")
		}
	}()

	mcpURL := "http://" + addr.String() + "/mcp"
	acceptHeader := "application/json, " + "text" + "/" + "event-stream"

	bearer, err := oauthTokenStore.IssueAccess(
		"alice@example.com", "client-gg9b", canonicalResourceIdentifier())
	if err != nil {
		t.Fatalf("issueAccess: %v (R-GG9B-GS8T)", err)
	}

	post := func(payload string, sessionID string) (*http.Response, []byte) {
		t.Helper()
		req, err := http.NewRequest(http.MethodPost, mcpURL, strings.NewReader(payload))
		if err != nil {
			t.Fatalf("new request: %v", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", acceptHeader)
		if sessionID != "" {
			req.Header.Set("Mcp-Session-Id", sessionID)
		}
		req.Header.Set("Authorization", "Bearer "+bearer)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("POST %s: %v", mcpURL, err)
		}
		buf, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		return resp, buf
	}

	initBody := `{"jsonrpc":"2.0","id":1,"method":"initialize",` +
		`"params":{"protocolVersion":"2025-11-25","capabilities":{},` +
		`"clientInfo":{"name":"hal-test","version":"0.0.1"}}}`
	resp, buf := post(initBody, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("initialize status = %d, want 200; body=%q", resp.StatusCode, string(buf))
	}
	sessionID := resp.Header.Get("Mcp-Session-Id")
	if sessionID == "" {
		t.Fatalf("initialize did not return Mcp-Session-Id header; body=%q", string(buf))
	}

	// Bring the counter to a known nonzero state so the success path is
	// observable regardless of singleton state at test entry.
	theCounter.Increment()
	before := theCounter.Read()
	if before == 0 {
		t.Fatalf("counter should be nonzero after increment; got 0")
	}

	callBody := `{"jsonrpc":"2.0","id":2,"method":"tools/call",` +
		`"params":{"name":"counter_decrement","arguments":{}}}`
	resp, buf = post(callBody, sessionID)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("tools/call status = %d, want 200; body=%q", resp.StatusCode, string(buf))
	}

	var ok struct {
		JSONRPC string `json:"jsonrpc"`
		Result  struct {
			IsError           bool `json:"isError"`
			StructuredContent struct {
				Value uint64 `json:"value"`
			} `json:"structuredContent"`
		} `json:"result"`
		Error any `json:"error"`
	}
	if err := json.Unmarshal(buf, &ok); err != nil {
		t.Fatalf("decode JSON-RPC response: %v; body=%q", err, string(buf))
	}
	if ok.Error != nil {
		t.Fatalf("tools/call carried error: %v; body=%q", ok.Error, string(buf))
	}
	if ok.Result.IsError {
		t.Fatalf("tools/call result.isError = true on nonzero counter; body=%q", string(buf))
	}
	if ok.Result.StructuredContent.Value != before-1 {
		t.Fatalf("counter_decrement returned %d, want %d; body=%q",
			ok.Result.StructuredContent.Value, before-1, string(buf))
	}
	if got := theCounter.Read(); got != before-1 {
		t.Fatalf("theCounter.Read() = %d after decrement, want %d", got, before-1)
	}

	// Drain to zero, then assert the zero-floor error path.
	for theCounter.Read() > 0 {
		if _, dok := theCounter.Decrement(); !dok {
			break
		}
	}
	if got := theCounter.Read(); got != 0 {
		t.Fatalf("counter not at zero after drain; got %d", got)
	}

	zeroBody := `{"jsonrpc":"2.0","id":3,"method":"tools/call",` +
		`"params":{"name":"counter_decrement","arguments":{}}}`
	resp, buf = post(zeroBody, sessionID)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("tools/call status (zero) = %d, want 200; body=%q", resp.StatusCode, string(buf))
	}

	var zero struct {
		Result struct {
			IsError bool `json:"isError"`
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"result"`
		Error any `json:"error"`
	}
	if err := json.Unmarshal(buf, &zero); err != nil {
		t.Fatalf("decode zero-floor response: %v; body=%q", err, string(buf))
	}
	if zero.Error != nil {
		t.Fatalf("zero-floor tools/call carried protocol error: %v; body=%q",
			zero.Error, string(buf))
	}
	if !zero.Result.IsError {
		t.Fatalf("zero-floor tools/call result.isError = false; want true; body=%q", string(buf))
	}
	foundMsg := false
	for _, c := range zero.Result.Content {
		if strings.Contains(c.Text, "below zero") {
			foundMsg = true
			break
		}
	}
	if !foundMsg {
		t.Fatalf("zero-floor error content does not name the cause; body=%q", string(buf))
	}
	if got := theCounter.Read(); got != 0 {
		t.Fatalf("theCounter.Read() = %d after zero-floor decrement, want 0", got)
	}
}

// R-FUB4-KWWB: the server advertises exactly three tools, one per counter
// operation: counter_read, counter_increment, and counter_decrement, with
// no others. Behavioral test: spin up runServe, initialize an MCP session,
// POST tools/list, and assert the returned tool name set equals exactly
// that triple.
func TestR_FUB4_KWWB_mcp_advertises_exactly_three_tools(t *testing.T) {
	ready := make(chan net.Addr, 1)
	onListenerReady = func(a net.Addr) { ready <- a }
	defer func() { onListenerReady = nil }()

	ctx, cancel := context.WithCancel(contextWithTestStores(context.Background()))
	defer cancel()
	var stdout, stderr bytes.Buffer
	done := make(chan int, 1)
	go func() {
		done <- runServeForTest(t, ctx, []string{"--port", "0"}, &stdout, &stderr)
	}()

	var addr net.Addr
	select {
	case addr = <-ready:
	case <-time.After(2 * time.Second):
		cancel()
		<-done
		t.Fatalf("listener never ready within 2s; stderr=%q", stderr.String())
	}
	defer func() {
		cancel()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatalf("runServe did not exit within 2s after cancel")
		}
	}()

	mcpURL := "http://" + addr.String() + "/mcp"
	acceptHeader := "application/json, " + "text" + "/" + "event-stream"

	post := func(payload string, sessionID string) (*http.Response, []byte) {
		t.Helper()
		req, err := http.NewRequest(http.MethodPost, mcpURL, strings.NewReader(payload))
		if err != nil {
			t.Fatalf("new request: %v", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", acceptHeader)
		if sessionID != "" {
			req.Header.Set("Mcp-Session-Id", sessionID)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("POST %s: %v", mcpURL, err)
		}
		buf, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		return resp, buf
	}

	initBody := `{"jsonrpc":"2.0","id":1,"method":"initialize",` +
		`"params":{"protocolVersion":"2025-11-25","capabilities":{},` +
		`"clientInfo":{"name":"hal-test","version":"0.0.1"}}}`
	resp, buf := post(initBody, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("initialize status = %d, want 200; body=%q", resp.StatusCode, string(buf))
	}
	sessionID := resp.Header.Get("Mcp-Session-Id")
	if sessionID == "" {
		t.Fatalf("initialize did not return Mcp-Session-Id header; body=%q", string(buf))
	}

	listBody := `{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`
	resp, buf = post(listBody, sessionID)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("tools/list status = %d, want 200; body=%q", resp.StatusCode, string(buf))
	}

	var rpc struct {
		JSONRPC string `json:"jsonrpc"`
		Result  struct {
			Tools []struct {
				Name string `json:"name"`
			} `json:"tools"`
		} `json:"result"`
		Error any `json:"error"`
	}
	if err := json.Unmarshal(buf, &rpc); err != nil {
		t.Fatalf("decode tools/list response: %v; body=%q", err, string(buf))
	}
	if rpc.Error != nil {
		t.Fatalf("tools/list carried error: %v; body=%q", rpc.Error, string(buf))
	}

	got := make(map[string]int, len(rpc.Result.Tools))
	for _, tl := range rpc.Result.Tools {
		got[tl.Name]++
	}
	want := map[string]struct{}{
		"counter_read":      {},
		"counter_increment": {},
		"counter_decrement": {},
	}
	if len(got) != len(want) {
		t.Fatalf("tools/list returned %d distinct tools, want %d; got=%v; body=%q",
			len(got), len(want), got, string(buf))
	}
	for name := range want {
		if got[name] != 1 {
			t.Fatalf("tools/list missing or duplicated %q (count=%d); got=%v; body=%q",
				name, got[name], got, string(buf))
		}
	}
	for name := range got {
		if _, ok := want[name]; !ok {
			t.Fatalf("tools/list advertises unexpected tool %q; got=%v; body=%q",
				name, got, string(buf))
		}
	}
}

// R-Z3LX-89W1: tool names and descriptions are written for a model
// audience — a model reading them should be able to choose the right
// tool without further context. Behavioral test: spin up runServe,
// initialize an MCP session, POST tools/list, and assert that every
// advertised tool carries a non-empty description that mentions the
// counter domain. Empty / placeholder descriptions don't meet the
// "model can choose without further context" bar, and mentioning the
// shared resource ("counter") is the minimum semantic signal a model
// needs to disambiguate these three from unrelated tools.
func TestR_Z3LX_89W1_mcp_tool_descriptions_are_model_audience(t *testing.T) {
	ready := make(chan net.Addr, 1)
	onListenerReady = func(a net.Addr) { ready <- a }
	defer func() { onListenerReady = nil }()

	ctx, cancel := context.WithCancel(contextWithTestStores(context.Background()))
	defer cancel()
	var stdout, stderr bytes.Buffer
	done := make(chan int, 1)
	go func() {
		done <- runServeForTest(t, ctx, []string{"--port", "0"}, &stdout, &stderr)
	}()

	var addr net.Addr
	select {
	case addr = <-ready:
	case <-time.After(2 * time.Second):
		cancel()
		<-done
		t.Fatalf("listener never ready within 2s; stderr=%q", stderr.String())
	}
	defer func() {
		cancel()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatalf("runServe did not exit within 2s after cancel")
		}
	}()

	mcpURL := "http://" + addr.String() + "/mcp"
	acceptHeader := "application/json, " + "text" + "/" + "event-stream"

	post := func(payload string, sessionID string) (*http.Response, []byte) {
		t.Helper()
		req, err := http.NewRequest(http.MethodPost, mcpURL, strings.NewReader(payload))
		if err != nil {
			t.Fatalf("new request: %v", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", acceptHeader)
		if sessionID != "" {
			req.Header.Set("Mcp-Session-Id", sessionID)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("POST %s: %v", mcpURL, err)
		}
		buf, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		return resp, buf
	}

	initBody := `{"jsonrpc":"2.0","id":1,"method":"initialize",` +
		`"params":{"protocolVersion":"2025-11-25","capabilities":{},` +
		`"clientInfo":{"name":"hal-test","version":"0.0.1"}}}`
	resp, buf := post(initBody, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("initialize status = %d, want 200; body=%q", resp.StatusCode, string(buf))
	}
	sessionID := resp.Header.Get("Mcp-Session-Id")
	if sessionID == "" {
		t.Fatalf("initialize did not return Mcp-Session-Id header; body=%q", string(buf))
	}

	listBody := `{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`
	resp, buf = post(listBody, sessionID)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("tools/list status = %d, want 200; body=%q", resp.StatusCode, string(buf))
	}

	var rpc struct {
		JSONRPC string `json:"jsonrpc"`
		Result  struct {
			Tools []struct {
				Name        string `json:"name"`
				Description string `json:"description"`
			} `json:"tools"`
		} `json:"result"`
		Error any `json:"error"`
	}
	if err := json.Unmarshal(buf, &rpc); err != nil {
		t.Fatalf("decode tools/list response: %v; body=%q", err, string(buf))
	}
	if rpc.Error != nil {
		t.Fatalf("tools/list carried error: %v; body=%q", rpc.Error, string(buf))
	}
	if len(rpc.Result.Tools) == 0 {
		t.Fatalf("tools/list returned zero tools; body=%q", string(buf))
	}

	for _, tl := range rpc.Result.Tools {
		if tl.Name == "" {
			t.Fatalf("tool with empty name; body=%q", string(buf))
		}
		desc := strings.TrimSpace(tl.Description)
		if desc == "" {
			t.Fatalf("tool %q has empty description; body=%q", tl.Name, string(buf))
		}
		// A model audience needs enough text to decide between tools —
		// a one-word placeholder isn't enough. 40 chars is loose but
		// rules out "TODO" / "counter" / "increment" stubs.
		if len(desc) < 40 {
			t.Fatalf("tool %q description too short for model audience (%d chars): %q",
				tl.Name, len(desc), desc)
		}
		// Every tool in this server is about the shared counter; the
		// description must say so or the model cannot pick the family.
		if !strings.Contains(strings.ToLower(desc), "counter") {
			t.Fatalf("tool %q description does not mention 'counter': %q",
				tl.Name, desc)
		}
	}
}

// R-0CQ7-DSBQ: a request that invokes the read tool may be made
// unauthenticated. Behavioral test: spin up runServe, initialize an MCP
// session, and call counter_read while explicitly verifying the outgoing
// requests carry no Authorization header, no bearer token, and no
// hal_session cookie. The call must still succeed.
func TestR_0CQ7_DSBQ_mcp_counter_read_is_unauthenticated(t *testing.T) {
	ready := make(chan net.Addr, 1)
	onListenerReady = func(a net.Addr) { ready <- a }
	defer func() { onListenerReady = nil }()

	ctx, cancel := context.WithCancel(contextWithTestStores(context.Background()))
	defer cancel()
	var stdout, stderr bytes.Buffer
	done := make(chan int, 1)
	go func() {
		done <- runServeForTest(t, ctx, []string{"--port", "0"}, &stdout, &stderr)
	}()

	var addr net.Addr
	select {
	case addr = <-ready:
	case <-time.After(2 * time.Second):
		cancel()
		<-done
		t.Fatalf("listener never ready within 2s; stderr=%q", stderr.String())
	}
	defer func() {
		cancel()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatalf("runServe did not exit within 2s after cancel")
		}
	}()

	mcpURL := "http://" + addr.String() + "/mcp"
	acceptHeader := "application/json, " + "text" + "/" + "event-stream"

	post := func(payload string, sessionID string) (*http.Response, []byte) {
		t.Helper()
		req, err := http.NewRequest(http.MethodPost, mcpURL, strings.NewReader(payload))
		if err != nil {
			t.Fatalf("new request: %v", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", acceptHeader)
		if sessionID != "" {
			req.Header.Set("Mcp-Session-Id", sessionID)
		}
		if got := req.Header.Get("Authorization"); got != "" {
			t.Fatalf("unauthenticated test sent Authorization header: %q", got)
		}
		if cookies := req.Cookies(); len(cookies) != 0 {
			t.Fatalf("unauthenticated test sent cookies: %v", cookies)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("POST %s: %v", mcpURL, err)
		}
		buf, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		return resp, buf
	}

	initBody := `{"jsonrpc":"2.0","id":1,"method":"initialize",` +
		`"params":{"protocolVersion":"2025-11-25","capabilities":{},` +
		`"clientInfo":{"name":"hal-test","version":"0.0.1"}}}`
	resp, buf := post(initBody, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("initialize status = %d, want 200; body=%q", resp.StatusCode, string(buf))
	}
	sessionID := resp.Header.Get("Mcp-Session-Id")
	if sessionID == "" {
		t.Fatalf("initialize did not return Mcp-Session-Id header; body=%q", string(buf))
	}

	want := theCounter.Read()

	callBody := `{"jsonrpc":"2.0","id":2,"method":"tools/call",` +
		`"params":{"name":"counter_read","arguments":{}}}`
	resp, buf = post(callBody, sessionID)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("tools/call status = %d, want 200; body=%q", resp.StatusCode, string(buf))
	}

	var rpc struct {
		JSONRPC string `json:"jsonrpc"`
		Result  struct {
			IsError           bool `json:"isError"`
			StructuredContent struct {
				Value uint64 `json:"value"`
			} `json:"structuredContent"`
		} `json:"result"`
		Error any `json:"error"`
	}
	if err := json.Unmarshal(buf, &rpc); err != nil {
		t.Fatalf("decode JSON-RPC response: %v; body=%q", err, string(buf))
	}
	if rpc.Error != nil {
		t.Fatalf("tools/call carried error: %v; body=%q", rpc.Error, string(buf))
	}
	if rpc.Result.IsError {
		t.Fatalf("tools/call result.isError = true; body=%q", string(buf))
	}
	if rpc.Result.StructuredContent.Value != want {
		t.Fatalf("counter_read returned %d, want %d; body=%q",
			rpc.Result.StructuredContent.Value, want, string(buf))
	}
}

// R-V65K-UVVH: the legacy HTTP+SSE MCP transport is not provided. Only the
// Streamable HTTP transport (R-UK7D-Z0IZ) is supported. Structural test:
// scan non-test Go source for the SSE MIME type `text/event-stream` (in any
// string literal) and for identifier names that name an SSE / event-stream
// surface. Any occurrence is a defect — the project must never wire up the
// legacy HTTP+SSE transport. Matching on the MIME type uses unquoted string
// literal contents; identifier matching uses substring against AST idents.
func TestR_V65K_UVVH_no_legacy_http_sse_transport(t *testing.T) {
	forbiddenLiteralSubstrings := []string{
		"text/event-stream",
	}
	forbiddenIdentSubstrings := []string{
		"EventStream",
		"EventSource",
		"ServerSentEvent",
		"SSEHandler",
		"SSETransport",
	}

	var goFiles []string
	err := filepath.Walk("..", func(path string, info os.FileInfo, err error) error {
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
	if err != nil {
		t.Fatalf("walk: %v", err)
	}

	fset := token.NewFileSet()
	for _, path := range goFiles {
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.BasicLit:
				if x.Kind != token.STRING {
					return true
				}
				val, err := strconv.Unquote(x.Value)
				if err != nil {
					return true
				}
				for _, bad := range forbiddenLiteralSubstrings {
					if strings.Contains(val, bad) {
						pos := fset.Position(x.Pos())
						t.Errorf("%s:%d: string literal %q contains forbidden "+
							"SSE token %q — legacy HTTP+SSE MCP transport "+
							"is out of scope (R-V65K-UVVH)",
							pos.Filename, pos.Line, val, bad)
					}
				}
			case *ast.Ident:
				for _, bad := range forbiddenIdentSubstrings {
					if strings.Contains(x.Name, bad) {
						pos := fset.Position(x.Pos())
						t.Errorf("%s:%d: identifier %q matches forbidden "+
							"SSE token %q — legacy HTTP+SSE MCP transport "+
							"is out of scope (R-V65K-UVVH)",
							pos.Filename, pos.Line, x.Name, bad)
					}
				}
			}
			return true
		})
	}
}

// R-51PZ-MEQR: presented-but-invalid bearer credentials at `/mcp` are
// rejected at the HTTP authorization boundary with 401 and a Bearer
// challenge before any MCP tool handler runs. The response must not be a
// successful JSON-RPC `tools/call` result carrying a tool-level error.
func TestR_51PZ_MEQR_mcp_invalid_bearer_rejected_at_http_boundary(t *testing.T) {
	ready := make(chan net.Addr, 1)
	onListenerReady = func(a net.Addr) { ready <- a }
	defer func() { onListenerReady = nil }()

	ctx, cancel := context.WithCancel(contextWithTestStores(context.Background()))
	defer cancel()
	var stdout, stderr bytes.Buffer
	done := make(chan int, 1)
	go func() {
		done <- runServeForTest(t, ctx, []string{"--port", "0"}, &stdout, &stderr)
	}()

	var addr net.Addr
	select {
	case addr = <-ready:
	case <-time.After(2 * time.Second):
		cancel()
		<-done
		t.Fatalf("listener never ready within 2s; stderr=%q (R-51PZ-MEQR)",
			stderr.String())
	}
	defer func() {
		cancel()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatalf("runServe did not exit within 2s (R-51PZ-MEQR)")
		}
	}()

	base := "http://" + addr.String()
	mcpURL := base + "/mcp"
	acceptHeader := "application/json, " + "text" + "/" + "event-stream"
	callBody := `{"jsonrpc":"2.0","id":2,"method":"tools/call",` +
		`"params":{"name":"counter_increment","arguments":{}}}`

	postWithAuth := func(authHeader string) (*http.Response, []byte) {
		t.Helper()
		req, err := http.NewRequest(http.MethodPost, mcpURL,
			strings.NewReader(callBody))
		if err != nil {
			t.Fatalf("new request: %v (R-51PZ-MEQR)", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", acceptHeader)
		req.Header.Set("Authorization", authHeader)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("POST %s: %v (R-51PZ-MEQR)", mcpURL, err)
		}
		buf, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			t.Fatalf("read body: %v (R-51PZ-MEQR)", err)
		}
		return resp, buf
	}

	wrongResource, err := oauthTokenStore.IssueAccess(
		"r51pz@example.test", "client-r51pz-wrong-resource",
		canonicalResourceIdentifier()+"x")
	if err != nil {
		t.Fatalf("issue wrong-resource token: %v (R-51PZ-MEQR)", err)
	}
	expired, err := oauthTokenStore.IssueAccess(
		"r51pz@example.test", "client-r51pz-expired",
		canonicalResourceIdentifier())
	if err != nil {
		t.Fatalf("issue expired token: %v (R-51PZ-MEQR)", err)
	}
	revoked, err := oauthTokenStore.IssueAccess(
		"r51pz@example.test", "client-r51pz-revoked",
		canonicalResourceIdentifier())
	if err != nil {
		t.Fatalf("issue revoked token: %v (R-51PZ-MEQR)", err)
	}
	oauthTokenStore.Mu.Lock()
	oauthTokenStore.M[oauthpkg.TokenHash(expired)].ExpiresAt = time.Now().Add(-time.Minute)
	oauthTokenStore.M[oauthpkg.TokenHash(revoked)].RevokedAt = time.Now()
	oauthTokenStore.Mu.Unlock()

	cases := []struct {
		name       string
		authHeader string
		wantDesc   string
	}{
		{
			name:       "malformed",
			authHeader: "Basic not-a-bearer-token",
			wantDesc:   "bearer authorization header malformed",
		},
		{
			name:       "unknown",
			authHeader: "Bearer not-an-issued-token",
			wantDesc:   "bearer token not recognized",
		},
		{
			name:       "expired",
			authHeader: "Bearer " + expired,
			wantDesc:   "bearer token expired",
		},
		{
			name:       "revoked",
			authHeader: "Bearer " + revoked,
			wantDesc:   "bearer token revoked",
		},
		{
			name:       "wrong_resource",
			authHeader: "Bearer " + wrongResource,
			wantDesc:   "bearer token resource binding does not match",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			before := theCounter.Read()
			resp, buf := postWithAuth(tc.authHeader)
			if resp.StatusCode != http.StatusUnauthorized {
				t.Fatalf("status = %d, want 401; body=%q (R-51PZ-MEQR)",
					resp.StatusCode, buf)
			}
			wa := resp.Header.Get("WWW-Authenticate")
			if !strings.HasPrefix(strings.ToLower(wa), "bearer") {
				t.Fatalf("WWW-Authenticate = %q, want Bearer scheme "+
					"(R-51PZ-MEQR)", wa)
			}
			for _, want := range []string{
				`error="invalid_token"`,
				`error_description="` + tc.wantDesc + `"`,
				`resource_metadata="` + base + `/.well-known/oauth-protected-resource/mcp"`,
			} {
				if !strings.Contains(wa, want) {
					t.Fatalf("WWW-Authenticate = %q, want substring %q "+
						"(R-51PZ-MEQR)", wa, want)
				}
			}
			var body struct {
				Error            string          `json:"error"`
				ErrorDescription string          `json:"error_description"`
				Result           json.RawMessage `json:"result"`
			}
			if err := json.Unmarshal(buf, &body); err != nil {
				t.Fatalf("body not JSON: %v; body=%q (R-51PZ-MEQR)", err, buf)
			}
			if body.Error != "invalid_token" || body.ErrorDescription != tc.wantDesc {
				t.Fatalf("body error = (%q, %q), want (invalid_token, %q) "+
					"(R-51PZ-MEQR)", body.Error, body.ErrorDescription, tc.wantDesc)
			}
			if len(body.Result) != 0 {
				t.Fatalf("body unexpectedly contains JSON-RPC result %s; "+
					"invalid bearer must not reach tool layer (R-51PZ-MEQR)",
					body.Result)
			}
			if got := theCounter.Read(); got != before {
				t.Fatalf("counter changed after rejected bearer: before=%d "+
					"after=%d (R-51PZ-MEQR)", before, got)
			}
		})
	}
}
