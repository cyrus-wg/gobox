package grpcinterceptor

import (
	"context"
	"io"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ---------------------------------------------------------------------------
// Test types
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
	// Copy the message content by assigning
	src := m.messages[m.index]
	m.index++

	// Use reflection-free approach: type-switch for known test types
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
