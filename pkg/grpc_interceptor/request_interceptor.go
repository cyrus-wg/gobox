package grpcinterceptor

import (
	"context"
	"regexp"
	"strings"
	"time"

	"github.com/cyrus-wg/gobox/pkg/logger"
	"github.com/cyrus-wg/gobox/pkg/pattern"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

func RequestUnaryInterceptor(logRequestDetails bool, logCompleteTime bool, bypassList ...BypassRequestLogging) grpc.UnaryServerInterceptor {
	compiledBypassList := compileBypassPatterns(bypassList)

	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		startTime := time.Now()

		requestID := logger.GenerateRequestID()
		ctx = logger.SetRequestID(ctx, requestID)

		shouldSkipLogging := shouldBypassLogging(compiledBypassList, info.FullMethod)

		if logRequestDetails && !shouldSkipLogging {
			requestData := buildRequestData(ctx, info.FullMethod)
			logger.Infow(ctx, "Incoming gRPC request", "details", requestData)
		}

		resp, err := handler(ctx, req)

		latency := time.Since(startTime)

		if logCompleteTime && !shouldSkipLogging {
			fields := []any{
				"latency_ms", latency.Milliseconds(),
			}
			if err != nil {
				st, _ := status.FromError(err)
				fields = append(fields, "grpc_code", st.Code().String())
			}
			logger.Infow(ctx, "gRPC request completed", fields...)
		}

		return resp, err
	}
}

func RequestStreamInterceptor(logRequestDetails bool, logCompleteTime bool, bypassList ...BypassRequestLogging) grpc.StreamServerInterceptor {
	compiledBypassList := compileBypassPatterns(bypassList)

	return func(
		srv any,
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		startTime := time.Now()
		ctx := ss.Context()

		requestID := logger.GenerateRequestID()
		ctx = logger.SetRequestID(ctx, requestID)

		wrapped := &wrappedServerStream{ServerStream: ss, ctx: ctx}

		shouldSkipLogging := shouldBypassLogging(compiledBypassList, info.FullMethod)

		if logRequestDetails && !shouldSkipLogging {
			requestData := buildRequestData(ctx, info.FullMethod)
			requestData["is_client_stream"] = info.IsClientStream
			requestData["is_server_stream"] = info.IsServerStream
			logger.Infow(ctx, "Incoming gRPC stream request", "details", requestData)
		}

		err := handler(srv, wrapped)

		latency := time.Since(startTime)

		if logCompleteTime && !shouldSkipLogging {
			fields := []any{
				"latency_ms", latency.Milliseconds(),
			}
			if err != nil {
				st, _ := status.FromError(err)
				fields = append(fields, "grpc_code", st.Code().String())
			}
			logger.Infow(ctx, "gRPC stream request completed", fields...)
		}

		return err
	}
}

type BypassRequestLogging struct {
	Method  string         // Full gRPC method name (e.g., "/package.Service/Method"), Ant-style glob by default
	IsRegex bool           // If true, Method is treated as a regex pattern; otherwise Ant-style pattern
	regex   *regexp.Regexp // Pre-compiled regex (internal use)
}

func compileBypassPatterns(patterns []BypassRequestLogging) []BypassRequestLogging {
	compiled := make([]BypassRequestLogging, len(patterns))
	for i, p := range patterns {
		compiled[i] = p
		if p.IsRegex {
			if re, err := regexp.Compile("^" + p.Method + "$"); err == nil {
				compiled[i].regex = re
			}
		}
	}
	return compiled
}

func shouldBypassLogging(bypassList []BypassRequestLogging, fullMethod string) bool {
	for _, bypass := range bypassList {
		if bypass.IsRegex {
			if matchRegex(&bypass, fullMethod) {
				return true
			}
		} else {
			if pattern.MatchAnt(bypass.Method, fullMethod) {
				return true
			}
		}
	}
	return false
}

func matchRegex(bypass *BypassRequestLogging, fullMethod string) bool {
	if bypass.regex != nil {
		return bypass.regex.MatchString(fullMethod)
	}

	return pattern.MatchRegex(bypass.Method, fullMethod)
}

func buildRequestData(ctx context.Context, fullMethod string) map[string]any {
	data := map[string]any{
		"method": fullMethod,
	}

	if p, ok := peer.FromContext(ctx); ok {
		data["peer_addr"] = p.Addr.String()
	}

	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if ua := md.Get("user-agent"); len(ua) > 0 {
			data["user_agent"] = strings.Join(ua, ", ")
		}
		if auth := md.Get("authorization"); len(auth) > 0 {
			data["has_authorization"] = true
		}
		if ct := md.Get("content-type"); len(ct) > 0 {
			data["content_type"] = strings.Join(ct, ", ")
		}
		if authority := md.Get(":authority"); len(authority) > 0 {
			data["host"] = strings.Join(authority, ", ")
		}
		if origin := md.Get("origin"); len(origin) > 0 {
			data["origin"] = strings.Join(origin, ", ")
		}
		if xff := md.Get("x-forwarded-for"); len(xff) > 0 {
			data["x_forwarded_for"] = strings.Join(xff, ", ")
		}
		if xfp := md.Get("x-forwarded-proto"); len(xfp) > 0 {
			data["x_forwarded_proto"] = strings.Join(xfp, ", ")
		}
		if xfh := md.Get("x-forwarded-host"); len(xfh) > 0 {
			data["x_forwarded_host"] = strings.Join(xfh, ", ")
		}
		if xri := md.Get("x-real-ip"); len(xri) > 0 {
			data["x_real_ip"] = strings.Join(xri, ", ")
		}
		if xci := md.Get("x-client-ip"); len(xci) > 0 {
			data["x_client_ip"] = strings.Join(xci, ", ")
		}
	}

	return data
}

type wrappedServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (w *wrappedServerStream) Context() context.Context {
	return w.ctx
}
