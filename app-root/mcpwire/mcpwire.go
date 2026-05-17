// Package mcpwire owns HAL's Model Context Protocol HTTP capability.
package mcpwire

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"

	counterpkg "github.com/mgreenly/hal/counter"
	jsonapipkg "github.com/mgreenly/hal/jsonapi"
	oauthpkg "github.com/mgreenly/hal/oauth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Surface carries the dependencies needed by the MCP transport and tools.
type Surface struct {
	Counter                     *counterpkg.Counter
	OAuthTokens                 *oauthpkg.TokenStore
	CanonicalResourceIdentifier func() string
	Version                     string
}

func (s Surface) counter() *counterpkg.Counter {
	if s.Counter != nil {
		return s.Counter
	}
	return counterpkg.New()
}

func (s Surface) tokens() *oauthpkg.TokenStore {
	if s.OAuthTokens != nil {
		return s.OAuthTokens
	}
	return oauthpkg.NewTokenStore(oauthpkg.TokenOptions{})
}

func (s Surface) canonicalResourceIdentifier() string {
	if s.CanonicalResourceIdentifier != nil {
		return s.CanonicalResourceIdentifier()
	}
	return ""
}

// Handler returns the Streamable HTTP handler for the fixed /mcp mount.
func (s Surface) Handler() http.Handler {
	server := s.NewServer()
	return s.PromptSignal(mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return server },
		&mcp.StreamableHTTPOptions{
			JSONResponse: true,
			// R-QGEN-MURV: the production posture places a TLS-terminating
			// proxy in front of this service; requests arrive at the loopback
			// listener with a public Host header (e.g. hal.ai.metaspot.org).
			// The SDK's DNS-rebinding protection would reject those requests
			// with 403 Forbidden, defeating any client that completed the
			// OAuth flow against the public host. Trust boundary is the
			// bearer-token gate in PromptSignal, not the Host header.
			DisableLocalhostProtection: true,
		},
	))
}

// R-Z3LX-89W1: tool names and descriptions registered below are written
// for a model audience — each name is the verb-on-resource form
// (counter_read / counter_increment / counter_decrement) and each
// description states what the tool does, what it returns, and when to
// choose it, so a model can pick the right tool without further context.
func (s Surface) NewServer() *mcp.Server {
	server := mcp.NewServer(
		&mcp.Implementation{Name: "hal", Version: s.Version},
		nil,
	)
	// R-XS1U-B7YY: the read tool accepts no arguments and returns the
	// current counter value as a non-negative integer. The counter is
	// uint64, so non-negativity is structural. R-0CQ7-DSBQ allows this
	// tool to be invoked unauthenticated — no auth gate here.
	mcp.AddTool(server, &mcp.Tool{
		Name: "counter_read",
		Description: "Return the current value of the shared counter. Takes no arguments. " +
			"The value is a non-negative integer that any client can observe; reading does " +
			"not modify it. Use this when you need to know the counter's state before " +
			"deciding whether to call counter_increment or counter_decrement.",
	}, s.CounterReadTool())
	// R-YHNQ-CEJJ: the increment tool accepts no arguments. On success it
	// adds one to the counter and returns the post-increment value.
	// R-ZQS0-HWZ8: an inbound request invoking this tool must present a
	// valid bearer access token issued by this service; the gate runs
	// inside counterIncrementTool, reading Authorization from
	// req.Extra.Header (populated by the Streamable HTTP transport).
	mcp.AddTool(server, &mcp.Tool{
		Name: "counter_increment",
		Description: "Add one to the shared counter and return the new value. Takes no " +
			"arguments. The returned value is the counter's state AFTER the increment, " +
			"a non-negative integer. Use this when the user wants the counter to go up by " +
			"one; call counter_read first if you need the pre-increment value.",
	}, s.CounterIncrementTool())
	// R-GG9B-GS8T: the decrement tool accepts no arguments. When the
	// counter is greater than zero, subtract one and return the
	// post-decrement value. When the counter is exactly zero, return
	// the standard MCP tool-error signal naming the cause; the counter
	// is not modified. R-285U-FWW3: the same valid HAL-issued access
	// token accepted for counter_increment also authorizes this
	// bearer-token-protected mutation surface.
	mcp.AddTool(server, &mcp.Tool{
		Name: "counter_decrement",
		Description: "Subtract one from the shared counter and return the new value. Takes no " +
			"arguments. The returned value is the counter's state AFTER the decrement, a " +
			"non-negative integer. The counter cannot go below zero: if it is already zero, " +
			"this tool returns an error and does not modify the counter. Use this when the " +
			"user wants the counter to go down by one.",
	}, s.CounterDecrementTool())
	return server
}

type counterReadOutput struct {
	Value uint64 `json:"value" jsonschema:"current counter value"`
}

// CounterReadTool returns the counter_read MCP tool handler.
func (s Surface) CounterReadTool() func(
	context.Context, *mcp.CallToolRequest, struct{},
) (*mcp.CallToolResult, counterReadOutput, error) {
	c := s.counter()
	return func(
		_ context.Context, _ *mcp.CallToolRequest, _ struct{},
	) (*mcp.CallToolResult, counterReadOutput, error) {
		return nil, counterReadOutput{Value: c.Read()}, nil
	}
}

type counterIncrementOutput struct {
	Value uint64 `json:"value" jsonschema:"post-increment counter value"`
}

// CounterIncrementTool returns the counter_increment MCP tool handler.
func (s Surface) CounterIncrementTool() func(
	context.Context, *mcp.CallToolRequest, struct{},
) (*mcp.CallToolResult, counterIncrementOutput, error) {
	c := s.counter()
	tokens := s.tokens()
	return func(
		_ context.Context, req *mcp.CallToolRequest, _ struct{},
	) (*mcp.CallToolResult, counterIncrementOutput, error) {
		// R-ZQS0-HWZ8: gate on a valid bearer access token issued by this
		// service. The Streamable HTTP transport hands the per-request HTTP
		// headers to handlers on req.Extra.Header; we validate via the same
		// token store the /counter/increment HTTP gate uses, so an MCP
		// client and a browser client see the same accept/reject decision
		// against the same store.
		var hdr http.Header
		if req != nil && req.Extra != nil {
			hdr = req.Extra.Header
		}
		if ok, errDesc := s.CheckBearerWithTokenStore(tokens, hdr); !ok {
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{&mcp.TextContent{Text: errDesc}},
			}, counterIncrementOutput{}, nil
		}
		return nil, counterIncrementOutput{Value: c.Increment()}, nil
	}
}

type counterDecrementOutput struct {
	Value uint64 `json:"value" jsonschema:"post-decrement counter value"`
}

// CounterDecrementTool returns the counter_decrement MCP tool handler.
func (s Surface) CounterDecrementTool() func(
	context.Context, *mcp.CallToolRequest, struct{},
) (*mcp.CallToolResult, counterDecrementOutput, error) {
	c := s.counter()
	tokens := s.tokens()
	return func(
		_ context.Context, req *mcp.CallToolRequest, _ struct{},
	) (*mcp.CallToolResult, counterDecrementOutput, error) {
		// R-285U-FWW3: MCP counter_decrement uses the same bearer-token
		// validation as counter_increment; access tokens are service-wide
		// for counter mutations, not scoped per operation.
		var hdr http.Header
		if req != nil && req.Extra != nil {
			hdr = req.Extra.Header
		}
		if ok, errDesc := s.CheckBearerWithTokenStore(tokens, hdr); !ok {
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{&mcp.TextContent{Text: errDesc}},
			}, counterDecrementOutput{}, nil
		}
		v, ok := c.Decrement()
		if !ok {
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{&mcp.TextContent{
					Text: "the counter cannot go below zero",
				}},
			}, counterDecrementOutput{}, nil
		}
		return nil, counterDecrementOutput{Value: v}, nil
	}
}

// CheckBearer validates the Authorization header carried by an MCP request.
func (s Surface) CheckBearer(h http.Header) (bool, string) {
	return s.CheckBearerWithTokenStore(s.tokens(), h)
}

// CheckBearerWithTokenStore validates an MCP bearer header against tokens.
func (s Surface) CheckBearerWithTokenStore(tokens *oauthpkg.TokenStore, h http.Header) (bool, string) {
	authHeader := h.Get("Authorization")
	if authHeader == "" {
		return false, "no credentials presented"
	}
	plaintext, parsed := jsonapipkg.ParseBearerAuthHeader(authHeader)
	if !parsed {
		return false, "bearer authorization header malformed"
	}
	rec, reason := tokens.LookupAccessReason(plaintext)
	if rec != nil {
		if rec.Resource != s.canonicalResourceIdentifier() {
			return false, "bearer token resource binding does not match"
		}
		return true, ""
	}
	switch reason {
	case "expired":
		return false, "bearer token expired"
	case "revoked":
		// R-7E4W-K6HL: a user-revoked token chain must stop an
		// already-connected MCP agent's next authenticated mutation.
		return false, "bearer token revoked"
	default:
		return false, "bearer token not recognized"
	}
}

// R-0YOE-9NO8: HTTP-level prompt-signal for the /mcp transport. When an
// MCP request invokes a tool that requires bearer credentials and presents
// no Authorization header, this middleware responds with HTTP 401 plus a
// WWW-Authenticate: Bearer header carrying the standard `resource_metadata`
// parameter pointing at this service's protected-resource metadata document.
// R-51PZ-MEQR: when a request presents malformed, unknown, expired, revoked,
// or wrong-resource bearer credentials, the HTTP authorization boundary
// rejects it before the SDK handler or any MCP tool handler runs.
func (s Surface) PromptSignal(next http.Handler) http.Handler {
	tokens := s.tokens()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			if ok, errDesc := s.CheckBearerWithTokenStore(tokens, r.Header); !ok {
				WriteBearerChallenge(w, r, "invalid_token", errDesc)
				return
			}
			next.ServeHTTP(w, r)
			return
		}
		if r.Method != http.MethodPost {
			next.ServeHTTP(w, r)
			return
		}
		jsonapipkg.LimitRequestBody(w, r)
		buf, err := io.ReadAll(r.Body)
		if err != nil {
			if jsonapipkg.RequestBodyTooLarge(err) {
				jsonapipkg.WriteBodyTooLarge(w)
				return
			}
			next.ServeHTTP(w, r)
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(buf))
		if !JSONRPCInvokesGatedTool(buf) {
			next.ServeHTTP(w, r)
			return
		}
		WriteBearerChallenge(w, r, "invalid_request", "no credentials presented")
	})
}

// WriteBearerChallenge writes the MCP bearer challenge and JSON OAuth error.
func WriteBearerChallenge(w http.ResponseWriter, r *http.Request, code, desc string) {
	// R-7BHQ-VB64: resource_metadata names the path
	// `/.well-known/oauth-protected-resource/mcp` so the URL is
	// scoped to the MCP transport per RFC 9728 §5.1.
	meta := jsonapipkg.RequestBaseURL(r) + "/.well-known/oauth-protected-resource/mcp"
	w.Header().Set("WWW-Authenticate",
		`Bearer realm="hal", error="`+code+`", `+
			`error_description="`+desc+`", `+
			`resource_metadata="`+meta+`"`)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(struct {
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description,omitempty"`
	}{Error: code, ErrorDescription: desc})
}

// JSONRPCInvokesGatedTool reports whether the JSON-RPC request body
// invokes a tool that requires bearer credentials. counter_read is
// explicitly unauthenticated (R-0CQ7-DSBQ). Batch requests and
// unparseable bodies fall through (returns false) so the SDK handler
// handles them on its own terms.
func JSONRPCInvokesGatedTool(buf []byte) bool {
	var msg struct {
		Method string `json:"method"`
		Params struct {
			Name string `json:"name"`
		} `json:"params"`
	}
	if err := json.Unmarshal(buf, &msg); err != nil {
		return false
	}
	return msg.Method == "tools/call" &&
		(msg.Params.Name == "counter_increment" ||
			msg.Params.Name == "counter_decrement")
}
