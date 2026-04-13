package grpcinterceptor

import (
	"context"
	"errors"
	"io"
	"net"
	"testing"

	"github.com/cyrus-wg/gobox/pkg/logger"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

func init() {
	logger.InitGlobalLogger(logger.LoggerConfig{})
}

// ---------------------------------------------------------------------------
// RecoverUnaryInterceptor
// ---------------------------------------------------------------------------

func TestRecoverUnaryInterceptor_NoPanic(t *testing.T) {
	interceptor := RecoverUnaryInterceptor()

	resp, err := interceptor(
		context.Background(),
		"request",
		&grpc.UnaryServerInfo{FullMethod: "/pkg.Svc/Method"},
		func(ctx context.Context, req any) (any, error) {
			return "ok", nil
		},
	)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if resp != "ok" {
		t.Fatalf("expected 'ok', got %v", resp)
	}
}

func TestRecoverUnaryInterceptor_PanicString(t *testing.T) {
	interceptor := RecoverUnaryInterceptor()

	resp, err := interceptor(
		context.Background(),
		"request",
		&grpc.UnaryServerInfo{FullMethod: "/pkg.Svc/Method"},
		func(ctx context.Context, req any) (any, error) {
			panic("something went wrong")
		},
	)

	if resp != nil {
		t.Fatalf("expected nil response, got %v", resp)
	}
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatal("expected gRPC status error")
	}
	if st.Code() != codes.Internal {
		t.Fatalf("expected Internal code, got %v", st.Code())
	}
}

func TestRecoverUnaryInterceptor_PanicError(t *testing.T) {
	interceptor := RecoverUnaryInterceptor()

	_, err := interceptor(
		context.Background(),
		"request",
		&grpc.UnaryServerInfo{FullMethod: "/pkg.Svc/Method"},
		func(ctx context.Context, req any) (any, error) {
			panic(errors.New("error panic"))
		},
	)

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	st, _ := status.FromError(err)
	if st.Code() != codes.Internal {
		t.Fatalf("expected Internal, got %v", st.Code())
	}
}

func TestRecoverUnaryInterceptor_PanicInt(t *testing.T) {
	interceptor := RecoverUnaryInterceptor()

	_, err := interceptor(
		context.Background(),
		"request",
		&grpc.UnaryServerInfo{FullMethod: "/pkg.Svc/Method"},
		func(ctx context.Context, req any) (any, error) {
			panic(42)
		},
	)

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	st, _ := status.FromError(err)
	if st.Code() != codes.Internal {
		t.Fatalf("expected Internal, got %v", st.Code())
	}
}

func TestRecoverUnaryInterceptor_HandlerReturnsError(t *testing.T) {
	interceptor := RecoverUnaryInterceptor()

	_, err := interceptor(
		context.Background(),
		"request",
		&grpc.UnaryServerInfo{FullMethod: "/pkg.Svc/Method"},
		func(ctx context.Context, req any) (any, error) {
			return nil, status.Errorf(codes.NotFound, "not found")
		},
	)

	st, _ := status.FromError(err)
	if st.Code() != codes.NotFound {
		t.Fatalf("expected NotFound, got %v", st.Code())
	}
}

// ---------------------------------------------------------------------------
// RecoverStreamInterceptor
// ---------------------------------------------------------------------------

type mockServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (m *mockServerStream) Context() context.Context { return m.ctx }

func TestRecoverStreamInterceptor_NoPanic(t *testing.T) {
	interceptor := RecoverStreamInterceptor()

	err := interceptor(
		nil,
		&mockServerStream{ctx: context.Background()},
		&grpc.StreamServerInfo{FullMethod: "/pkg.Svc/Stream"},
		func(srv any, stream grpc.ServerStream) error {
			return nil
		},
	)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestRecoverStreamInterceptor_PanicString(t *testing.T) {
	interceptor := RecoverStreamInterceptor()

	err := interceptor(
		nil,
		&mockServerStream{ctx: context.Background()},
		&grpc.StreamServerInfo{FullMethod: "/pkg.Svc/Stream"},
		func(srv any, stream grpc.ServerStream) error {
			panic("stream panic")
		},
	)

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	st, _ := status.FromError(err)
	if st.Code() != codes.Internal {
		t.Fatalf("expected Internal, got %v", st.Code())
	}
}

func TestRecoverStreamInterceptor_HandlerReturnsError(t *testing.T) {
	interceptor := RecoverStreamInterceptor()

	err := interceptor(
		nil,
		&mockServerStream{ctx: context.Background()},
		&grpc.StreamServerInfo{FullMethod: "/pkg.Svc/Stream"},
		func(srv any, stream grpc.ServerStream) error {
			return status.Errorf(codes.Unavailable, "unavailable")
		},
	)

	st, _ := status.FromError(err)
	if st.Code() != codes.Unavailable {
		t.Fatalf("expected Unavailable, got %v", st.Code())
	}
}

// ---------------------------------------------------------------------------
// RequestUnaryInterceptor
// ---------------------------------------------------------------------------

func TestRequestUnaryInterceptor_SetsRequestID(t *testing.T) {
	interceptor := RequestUnaryInterceptor(false, false)

	var capturedID string
	_, _ = interceptor(
		context.Background(),
		"request",
		&grpc.UnaryServerInfo{FullMethod: "/pkg.Svc/Method"},
		func(ctx context.Context, req any) (any, error) {
			id, ok := logger.GetRequestID(ctx)
			if !ok {
				t.Fatal("request ID not found in context")
			}
			capturedID = id
			return "ok", nil
		},
	)

	if capturedID == "" {
		t.Fatal("expected non-empty request ID")
	}
}

func TestRequestUnaryInterceptor_LogsDetailsAndCompletionTime(t *testing.T) {
	interceptor := RequestUnaryInterceptor(true, true)

	resp, err := interceptor(
		context.Background(),
		"request",
		&grpc.UnaryServerInfo{FullMethod: "/pkg.Svc/Method"},
		func(ctx context.Context, req any) (any, error) {
			return "ok", nil
		},
	)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if resp != "ok" {
		t.Fatalf("expected 'ok', got %v", resp)
	}
}

func TestRequestUnaryInterceptor_LogsErrorCode(t *testing.T) {
	interceptor := RequestUnaryInterceptor(false, true)

	_, err := interceptor(
		context.Background(),
		"request",
		&grpc.UnaryServerInfo{FullMethod: "/pkg.Svc/Method"},
		func(ctx context.Context, req any) (any, error) {
			return nil, status.Errorf(codes.InvalidArgument, "bad request")
		},
	)

	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRequestUnaryInterceptor_BypassAntPattern(t *testing.T) {
	interceptor := RequestUnaryInterceptor(true, true, BypassRequestLogging{
		Method: "/grpc.health.v1.Health/*",
	})

	resp, err := interceptor(
		context.Background(),
		"request",
		&grpc.UnaryServerInfo{FullMethod: "/grpc.health.v1.Health/Check"},
		func(ctx context.Context, req any) (any, error) {
			return "ok", nil
		},
	)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if resp != "ok" {
		t.Fatalf("expected 'ok', got %v", resp)
	}
}

func TestRequestUnaryInterceptor_BypassRegexPattern(t *testing.T) {
	interceptor := RequestUnaryInterceptor(true, true, BypassRequestLogging{
		Method:  "/grpc\\.health.*",
		IsRegex: true,
	})

	resp, err := interceptor(
		context.Background(),
		"request",
		&grpc.UnaryServerInfo{FullMethod: "/grpc.health.v1.Health/Check"},
		func(ctx context.Context, req any) (any, error) {
			return "ok", nil
		},
	)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if resp != "ok" {
		t.Fatalf("expected 'ok', got %v", resp)
	}
}

func TestRequestUnaryInterceptor_NoBypassWhenNotMatched(t *testing.T) {
	interceptor := RequestUnaryInterceptor(true, true, BypassRequestLogging{
		Method: "/grpc.health.v1.Health/*",
	})

	resp, err := interceptor(
		context.Background(),
		"request",
		&grpc.UnaryServerInfo{FullMethod: "/pkg.Svc/Method"},
		func(ctx context.Context, req any) (any, error) {
			return "ok", nil
		},
	)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if resp != "ok" {
		t.Fatalf("expected 'ok', got %v", resp)
	}
}

func TestRequestUnaryInterceptor_WithMetadata(t *testing.T) {
	interceptor := RequestUnaryInterceptor(true, true)

	md := metadata.Pairs(
		"user-agent", "test-agent/1.0",
		"authorization", "Bearer token",
		"x-forwarded-for", "1.2.3.4",
		"content-type", "application/grpc",
	)
	ctx := metadata.NewIncomingContext(context.Background(), md)

	resp, err := interceptor(
		ctx,
		"request",
		&grpc.UnaryServerInfo{FullMethod: "/pkg.Svc/Method"},
		func(ctx context.Context, req any) (any, error) {
			return "ok", nil
		},
	)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if resp != "ok" {
		t.Fatalf("expected 'ok', got %v", resp)
	}
}

func TestRequestUnaryInterceptor_WithPeerInfo(t *testing.T) {
	interceptor := RequestUnaryInterceptor(true, false)

	ctx := peer.NewContext(context.Background(), &peer.Peer{
		Addr: &net.TCPAddr{IP: net.ParseIP("10.0.0.1"), Port: 12345},
	})

	resp, err := interceptor(
		ctx,
		"request",
		&grpc.UnaryServerInfo{FullMethod: "/pkg.Svc/Method"},
		func(ctx context.Context, req any) (any, error) {
			return "ok", nil
		},
	)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if resp != "ok" {
		t.Fatalf("expected 'ok', got %v", resp)
	}
}

// ---------------------------------------------------------------------------
// RequestStreamInterceptor
// ---------------------------------------------------------------------------

func TestRequestStreamInterceptor_SetsRequestID(t *testing.T) {
	interceptor := RequestStreamInterceptor(false, false)

	var capturedID string
	err := interceptor(
		nil,
		&mockServerStream{ctx: context.Background()},
		&grpc.StreamServerInfo{FullMethod: "/pkg.Svc/Stream"},
		func(srv any, stream grpc.ServerStream) error {
			id, ok := logger.GetRequestID(stream.Context())
			if !ok {
				t.Fatal("request ID not found in stream context")
			}
			capturedID = id
			return nil
		},
	)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if capturedID == "" {
		t.Fatal("expected non-empty request ID")
	}
}

func TestRequestStreamInterceptor_LogsDetailsAndCompletionTime(t *testing.T) {
	interceptor := RequestStreamInterceptor(true, true)

	err := interceptor(
		nil,
		&mockServerStream{ctx: context.Background()},
		&grpc.StreamServerInfo{
			FullMethod:     "/pkg.Svc/Stream",
			IsClientStream: true,
			IsServerStream: false,
		},
		func(srv any, stream grpc.ServerStream) error {
			return nil
		},
	)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestRequestStreamInterceptor_LogsErrorCode(t *testing.T) {
	interceptor := RequestStreamInterceptor(false, true)

	err := interceptor(
		nil,
		&mockServerStream{ctx: context.Background()},
		&grpc.StreamServerInfo{FullMethod: "/pkg.Svc/Stream"},
		func(srv any, stream grpc.ServerStream) error {
			return status.Errorf(codes.ResourceExhausted, "rate limited")
		},
	)

	if err == nil {
		t.Fatal("expected error")
	}
	st, _ := status.FromError(err)
	if st.Code() != codes.ResourceExhausted {
		t.Fatalf("expected ResourceExhausted, got %v", st.Code())
	}
}

func TestRequestStreamInterceptor_BypassAntPattern(t *testing.T) {
	interceptor := RequestStreamInterceptor(true, true, BypassRequestLogging{
		Method: "/grpc.health.v1.Health/*",
	})

	err := interceptor(
		nil,
		&mockServerStream{ctx: context.Background()},
		&grpc.StreamServerInfo{FullMethod: "/grpc.health.v1.Health/Watch"},
		func(srv any, stream grpc.ServerStream) error {
			return nil
		},
	)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestRequestStreamInterceptor_BypassRegexPattern(t *testing.T) {
	interceptor := RequestStreamInterceptor(true, true, BypassRequestLogging{
		Method:  "/grpc\\.health.*",
		IsRegex: true,
	})

	err := interceptor(
		nil,
		&mockServerStream{ctx: context.Background()},
		&grpc.StreamServerInfo{FullMethod: "/grpc.health.v1.Health/Watch"},
		func(srv any, stream grpc.ServerStream) error {
			return nil
		},
	)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestRequestStreamInterceptor_WithMetadata(t *testing.T) {
	interceptor := RequestStreamInterceptor(true, true)

	md := metadata.Pairs(
		"user-agent", "test-agent/1.0",
		"content-type", "application/grpc",
	)
	ctx := metadata.NewIncomingContext(context.Background(), md)

	err := interceptor(
		nil,
		&mockServerStream{ctx: ctx},
		&grpc.StreamServerInfo{FullMethod: "/pkg.Svc/Stream"},
		func(srv any, stream grpc.ServerStream) error {
			return nil
		},
	)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Chaining interceptors
// ---------------------------------------------------------------------------

func TestInterceptorChaining_RecoverAndRequest(t *testing.T) {
	recoverInterceptor := RecoverUnaryInterceptor()
	requestInterceptor := RequestUnaryInterceptor(true, true)

	// Simulate chaining: recover wraps request wraps handler
	resp, err := recoverInterceptor(
		context.Background(),
		"request",
		&grpc.UnaryServerInfo{FullMethod: "/pkg.Svc/Method"},
		func(ctx context.Context, req any) (any, error) {
			return requestInterceptor(ctx, req,
				&grpc.UnaryServerInfo{FullMethod: "/pkg.Svc/Method"},
				func(ctx context.Context, req any) (any, error) {
					return "ok", nil
				},
			)
		},
	)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if resp != "ok" {
		t.Fatalf("expected 'ok', got %v", resp)
	}
}

func TestInterceptorChaining_RecoverCatchesPanicInRequest(t *testing.T) {
	recoverInterceptor := RecoverUnaryInterceptor()
	requestInterceptor := RequestUnaryInterceptor(true, true)

	_, err := recoverInterceptor(
		context.Background(),
		"request",
		&grpc.UnaryServerInfo{FullMethod: "/pkg.Svc/Method"},
		func(ctx context.Context, req any) (any, error) {
			return requestInterceptor(ctx, req,
				&grpc.UnaryServerInfo{FullMethod: "/pkg.Svc/Method"},
				func(ctx context.Context, req any) (any, error) {
					panic("inside chained handler")
				},
			)
		},
	)

	if err == nil {
		t.Fatal("expected error from recovered panic")
	}
	st, _ := status.FromError(err)
	if st.Code() != codes.Internal {
		t.Fatalf("expected Internal, got %v", st.Code())
	}
}

// ---------------------------------------------------------------------------
// compileBypassPatterns
// ---------------------------------------------------------------------------

func TestCompileBypassPatterns_InvalidRegex(t *testing.T) {
	patterns := compileBypassPatterns([]BypassRequestLogging{
		{Method: "[invalid", IsRegex: true},
	})
	if patterns[0].regex != nil {
		t.Fatal("expected nil regex on invalid pattern")
	}
}

func TestCompileBypassPatterns_ValidRegex(t *testing.T) {
	patterns := compileBypassPatterns([]BypassRequestLogging{
		{Method: "/api/v\\d+/.*", IsRegex: true},
	})
	if patterns[0].regex == nil {
		t.Fatal("expected compiled regex")
	}
}

func TestCompileBypassPatterns_NonRegex(t *testing.T) {
	patterns := compileBypassPatterns([]BypassRequestLogging{
		{Method: "/api/v1/**"},
	})
	if patterns[0].regex != nil {
		t.Fatal("expected nil regex for non-regex pattern")
	}
}

// ---------------------------------------------------------------------------
// shouldBypassLogging
// ---------------------------------------------------------------------------

func TestShouldBypassLogging_EmptyList(t *testing.T) {
	if shouldBypassLogging(nil, "/pkg.Svc/Method") {
		t.Fatal("empty list should not bypass")
	}
}

func TestShouldBypassLogging_AntMatch(t *testing.T) {
	list := compileBypassPatterns([]BypassRequestLogging{
		{Method: "/grpc.health.v1.Health/*"},
	})
	if !shouldBypassLogging(list, "/grpc.health.v1.Health/Check") {
		t.Fatal("expected bypass for health check")
	}
	if shouldBypassLogging(list, "/pkg.Svc/Method") {
		t.Fatal("should not bypass for non-matching method")
	}
}

func TestShouldBypassLogging_RegexMatch(t *testing.T) {
	list := compileBypassPatterns([]BypassRequestLogging{
		{Method: "/grpc\\.health\\.v1\\.Health/.*", IsRegex: true},
	})
	if !shouldBypassLogging(list, "/grpc.health.v1.Health/Check") {
		t.Fatal("expected bypass for health check")
	}
	if shouldBypassLogging(list, "/pkg.Svc/Method") {
		t.Fatal("should not bypass for non-matching method")
	}
}

func TestShouldBypassLogging_RegexFallback(t *testing.T) {
	// Invalid regex should fallback to pattern.MatchRegex
	list := compileBypassPatterns([]BypassRequestLogging{
		{Method: "[invalid", IsRegex: true},
	})
	// With an invalid regex, the compiled regex is nil and pattern.MatchRegex is used.
	// pattern.MatchRegex("[invalid", ...) should also fail, so no bypass.
	if shouldBypassLogging(list, "/anything") {
		t.Fatal("invalid regex should not match anything")
	}
}

func TestShouldBypassLogging_MultiplePatterns(t *testing.T) {
	list := compileBypassPatterns([]BypassRequestLogging{
		{Method: "/grpc.health.v1.Health/*"},
		{Method: "/grpc\\.reflection.*", IsRegex: true},
	})

	if !shouldBypassLogging(list, "/grpc.health.v1.Health/Check") {
		t.Fatal("expected bypass for health check")
	}
	if !shouldBypassLogging(list, "/grpc.reflection.v1alpha.ServerReflection/ServerReflectionInfo") {
		t.Fatal("expected bypass for reflection")
	}
	if shouldBypassLogging(list, "/pkg.Svc/Method") {
		t.Fatal("should not bypass for non-matching method")
	}
}

// ---------------------------------------------------------------------------
// matchRegex
// ---------------------------------------------------------------------------

func TestMatchRegex_PreCompiled(t *testing.T) {
	patterns := compileBypassPatterns([]BypassRequestLogging{
		{Method: "/api/users/\\d+", IsRegex: true},
	})
	if !matchRegex(&patterns[0], "/api/users/123") {
		t.Fatal("expected pre-compiled regex match")
	}
}

func TestMatchRegex_Fallback(t *testing.T) {
	// When regex is nil, it should fallback to pattern.MatchRegex
	b := &BypassRequestLogging{Method: "/api/users/\\d+", IsRegex: true}
	if !matchRegex(b, "/api/users/123") {
		t.Fatal("expected fallback match")
	}
}

// ---------------------------------------------------------------------------
// buildRequestData
// ---------------------------------------------------------------------------

func TestBuildRequestData_BasicMethod(t *testing.T) {
	data := buildRequestData(context.Background(), "/pkg.Svc/Method")
	if data["method"] != "/pkg.Svc/Method" {
		t.Fatalf("expected method /pkg.Svc/Method, got %v", data["method"])
	}
}

func TestBuildRequestData_WithPeer(t *testing.T) {
	ctx := peer.NewContext(context.Background(), &peer.Peer{
		Addr: &net.TCPAddr{IP: net.ParseIP("192.168.1.1"), Port: 5000},
	})
	data := buildRequestData(ctx, "/pkg.Svc/Method")
	if data["peer_addr"] == nil {
		t.Fatal("expected peer_addr in request data")
	}
}

func TestBuildRequestData_WithMetadata(t *testing.T) {
	md := metadata.Pairs(
		"user-agent", "grpc-go/1.0",
		"authorization", "Bearer xyz",
		"content-type", "application/grpc",
		":authority", "api.example.com",
		"origin", "https://example.com",
		"x-forwarded-for", "10.0.0.1",
		"x-forwarded-proto", "https",
		"x-forwarded-host", "lb.example.com",
		"x-real-ip", "172.16.0.1",
		"x-client-ip", "192.168.0.1",
	)
	ctx := metadata.NewIncomingContext(context.Background(), md)
	data := buildRequestData(ctx, "/pkg.Svc/Method")

	if data["user_agent"] != "grpc-go/1.0" {
		t.Fatalf("expected user_agent, got %v", data["user_agent"])
	}
	if data["has_authorization"] != true {
		t.Fatal("expected has_authorization=true")
	}
	if data["content_type"] != "application/grpc" {
		t.Fatalf("expected content_type, got %v", data["content_type"])
	}
	if data["host"] != "api.example.com" {
		t.Fatalf("expected host, got %v", data["host"])
	}
	if data["origin"] != "https://example.com" {
		t.Fatalf("expected origin, got %v", data["origin"])
	}
	if data["x_forwarded_for"] != "10.0.0.1" {
		t.Fatalf("expected x_forwarded_for, got %v", data["x_forwarded_for"])
	}
	if data["x_forwarded_proto"] != "https" {
		t.Fatalf("expected x_forwarded_proto, got %v", data["x_forwarded_proto"])
	}
	if data["x_forwarded_host"] != "lb.example.com" {
		t.Fatalf("expected x_forwarded_host, got %v", data["x_forwarded_host"])
	}
	if data["x_real_ip"] != "172.16.0.1" {
		t.Fatalf("expected x_real_ip, got %v", data["x_real_ip"])
	}
	if data["x_client_ip"] != "192.168.0.1" {
		t.Fatalf("expected x_client_ip, got %v", data["x_client_ip"])
	}
}

func TestBuildRequestData_WithoutMetadata(t *testing.T) {
	data := buildRequestData(context.Background(), "/pkg.Svc/Method")
	if _, ok := data["user_agent"]; ok {
		t.Fatal("expected no user_agent without metadata")
	}
}

// ---------------------------------------------------------------------------
// wrappedServerStream
// ---------------------------------------------------------------------------

func TestWrappedServerStream_ReturnsEnrichedContext(t *testing.T) {
	originalCtx := context.Background()
	enrichedCtx := context.WithValue(originalCtx, "test_key", "test_value")

	stream := &mockServerStream{ctx: originalCtx}
	wrapped := &wrappedServerStream{ServerStream: stream, ctx: enrichedCtx}

	if wrapped.Context().Value("test_key") != "test_value" {
		t.Fatal("expected enriched context value")
	}
}

// ---------------------------------------------------------------------------
// Test types for validation
// ---------------------------------------------------------------------------

type validRequest struct {
	Name  string `json:"name" validate:"required"`
	Email string `json:"email" validate:"required,email"`
	Age   int    `json:"age" validate:"gte=0"`
}

type noValidationRequest struct {
	Data string `json:"data"`
}

// ---------------------------------------------------------------------------
// ValidateRequestUnaryInterceptor
// ---------------------------------------------------------------------------

func TestValidateRequestUnaryInterceptor_ValidRequest(t *testing.T) {
	interceptor := ValidateRequestUnaryInterceptor()

	req := &validRequest{Name: "Alice", Email: "alice@example.com", Age: 30}
	resp, err := interceptor(
		context.Background(),
		req,
		&grpc.UnaryServerInfo{FullMethod: "/pkg.Svc/Method"},
		func(ctx context.Context, req any) (any, error) {
			return "ok", nil
		},
	)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if resp != "ok" {
		t.Fatalf("expected 'ok', got %v", resp)
	}
}

func TestValidateRequestUnaryInterceptor_InvalidRequest(t *testing.T) {
	interceptor := ValidateRequestUnaryInterceptor()

	req := &validRequest{Name: "Bob", Age: 25} // missing required email
	_, err := interceptor(
		context.Background(),
		req,
		&grpc.UnaryServerInfo{FullMethod: "/pkg.Svc/Method"},
		func(ctx context.Context, req any) (any, error) {
			t.Fatal("handler should not be called on validation failure")
			return nil, nil
		},
	)

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatal("expected gRPC status error")
	}
	if st.Code() != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", st.Code())
	}
	if st.Message() == "" {
		t.Fatal("expected non-empty error message")
	}
}

func TestValidateRequestUnaryInterceptor_MultipleValidationErrors(t *testing.T) {
	interceptor := ValidateRequestUnaryInterceptor()

	req := &validRequest{Age: -1} // missing name, missing email, age < 0
	_, err := interceptor(
		context.Background(),
		req,
		&grpc.UnaryServerInfo{FullMethod: "/pkg.Svc/Method"},
		func(ctx context.Context, req any) (any, error) {
			t.Fatal("handler should not be called")
			return nil, nil
		},
	)

	if err == nil {
		t.Fatal("expected error")
	}
	st, _ := status.FromError(err)
	if st.Code() != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", st.Code())
	}
}

func TestValidateRequestUnaryInterceptor_NoValidationTags(t *testing.T) {
	interceptor := ValidateRequestUnaryInterceptor()

	req := &noValidationRequest{Data: "anything"}
	resp, err := interceptor(
		context.Background(),
		req,
		&grpc.UnaryServerInfo{FullMethod: "/pkg.Svc/Method"},
		func(ctx context.Context, req any) (any, error) {
			return "ok", nil
		},
	)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if resp != "ok" {
		t.Fatalf("expected 'ok', got %v", resp)
	}
}

func TestValidateRequestUnaryInterceptor_NilRequest(t *testing.T) {
	interceptor := ValidateRequestUnaryInterceptor()

	resp, err := interceptor(
		context.Background(),
		nil,
		&grpc.UnaryServerInfo{FullMethod: "/pkg.Svc/Method"},
		func(ctx context.Context, req any) (any, error) {
			return "ok", nil
		},
	)

	if err != nil {
		t.Fatalf("expected no error for nil request, got %v", err)
	}
	if resp != "ok" {
		t.Fatalf("expected 'ok', got %v", resp)
	}
}

func TestValidateRequestUnaryInterceptor_LoggingDisabled(t *testing.T) {
	interceptor := ValidateRequestUnaryInterceptor(false)

	req := &validRequest{Name: "Alice", Email: "alice@example.com", Age: 30}
	resp, err := interceptor(
		context.Background(),
		req,
		&grpc.UnaryServerInfo{FullMethod: "/pkg.Svc/Method"},
		func(ctx context.Context, req any) (any, error) {
			return "ok", nil
		},
	)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if resp != "ok" {
		t.Fatalf("expected 'ok', got %v", resp)
	}
}

// ---------------------------------------------------------------------------
// ValidateRequestStreamInterceptor
// ---------------------------------------------------------------------------

type mockRecvStream struct {
	grpc.ServerStream
	ctx      context.Context
	messages []any
	index    int
}

func (m *mockRecvStream) Context() context.Context { return m.ctx }

func (m *mockRecvStream) RecvMsg(msg any) error {
	if m.index >= len(m.messages) {
		return io.EOF
	}
	src := m.messages[m.index]
	m.index++

	switch dst := msg.(type) {
	case *validRequest:
		if s, ok := src.(*validRequest); ok {
			*dst = *s
		}
	case *noValidationRequest:
		if s, ok := src.(*noValidationRequest); ok {
			*dst = *s
		}
	}
	return nil
}

func TestValidateRequestStreamInterceptor_ValidMessages(t *testing.T) {
	interceptor := ValidateRequestStreamInterceptor()

	stream := &mockRecvStream{
		ctx: context.Background(),
		messages: []any{
			&validRequest{Name: "Alice", Email: "alice@example.com", Age: 30},
		},
	}

	var receivedCount int
	err := interceptor(
		nil,
		stream,
		&grpc.StreamServerInfo{FullMethod: "/pkg.Svc/Stream"},
		func(srv any, ss grpc.ServerStream) error {
			msg := &validRequest{}
			for {
				if err := ss.RecvMsg(msg); err != nil {
					if err == io.EOF {
						return nil
					}
					return err
				}
				receivedCount++
			}
		},
	)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if receivedCount != 1 {
		t.Fatalf("expected 1 message received, got %d", receivedCount)
	}
}

func TestValidateRequestStreamInterceptor_InvalidMessage(t *testing.T) {
	interceptor := ValidateRequestStreamInterceptor()

	stream := &mockRecvStream{
		ctx: context.Background(),
		messages: []any{
			&validRequest{Name: "Bob"}, // missing required email
		},
	}

	err := interceptor(
		nil,
		stream,
		&grpc.StreamServerInfo{FullMethod: "/pkg.Svc/Stream"},
		func(srv any, ss grpc.ServerStream) error {
			msg := &validRequest{}
			return ss.RecvMsg(msg) // should fail validation
		},
	)

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatal("expected gRPC status error")
	}
	if st.Code() != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", st.Code())
	}
}

func TestValidateRequestStreamInterceptor_LoggingDisabled(t *testing.T) {
	interceptor := ValidateRequestStreamInterceptor(false)

	stream := &mockRecvStream{
		ctx: context.Background(),
		messages: []any{
			&validRequest{Name: "Alice", Email: "alice@example.com", Age: 30},
		},
	}

	err := interceptor(
		nil,
		stream,
		&grpc.StreamServerInfo{FullMethod: "/pkg.Svc/Stream"},
		func(srv any, ss grpc.ServerStream) error {
			msg := &validRequest{}
			for {
				if err := ss.RecvMsg(msg); err != nil {
					if err == io.EOF {
						return nil
					}
					return err
				}
			}
		},
	)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestValidateRequestStreamInterceptor_EOF(t *testing.T) {
	interceptor := ValidateRequestStreamInterceptor()

	stream := &mockRecvStream{
		ctx:      context.Background(),
		messages: []any{}, // empty — immediate EOF
	}

	err := interceptor(
		nil,
		stream,
		&grpc.StreamServerInfo{FullMethod: "/pkg.Svc/Stream"},
		func(srv any, ss grpc.ServerStream) error {
			msg := &validRequest{}
			err := ss.RecvMsg(msg)
			if err == io.EOF {
				return nil
			}
			return err
		},
	)

	if err != nil {
		t.Fatalf("expected no error on empty stream, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// validateRequest
// ---------------------------------------------------------------------------

func TestValidateRequest_NilInput(t *testing.T) {
	err := validateRequest(context.Background(), nil)
	if err != nil {
		t.Fatalf("expected nil error for nil input, got %v", err)
	}
}

func TestValidateRequest_ValidStruct(t *testing.T) {
	req := &validRequest{Name: "Alice", Email: "alice@example.com", Age: 0}
	err := validateRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestValidateRequest_InvalidStruct(t *testing.T) {
	req := &validRequest{Name: "", Email: "not-an-email", Age: -1}
	err := validateRequest(context.Background(), req)
	if err == nil {
		t.Fatal("expected error")
	}
	st, _ := status.FromError(err)
	if st.Code() != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", st.Code())
	}
}

func TestValidateRequest_NonStructInput(t *testing.T) {
	err := validateRequest(context.Background(), "a plain string")
	if err != nil {
		t.Fatalf("expected nil error for non-struct input, got %v", err)
	}
}
