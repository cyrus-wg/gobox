package grpcinterceptor

import (
	"context"
	"fmt"

	"github.com/cyrus-wg/gobox/pkg/logger"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// RecoverUnaryInterceptor returns a gRPC unary server interceptor that
// recovers from panics inside the handler, logs them, and returns an
// Internal gRPC status to the caller.
func RecoverUnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (resp any, err error) {
		defer func() {
			if r := recover(); r != nil {
				logger.Errorw(ctx, "Recovered from panic in gRPC unary handler",
					"method", info.FullMethod,
					"panic", fmt.Sprintf("%v", r),
				)
				err = status.Error(codes.Internal, "internal server error")
			}
		}()

		return handler(ctx, req)
	}
}

// RecoverStreamInterceptor returns a gRPC stream server interceptor that
// recovers from panics inside the handler, logs them, and returns an
// Internal gRPC status to the caller.
func RecoverStreamInterceptor() grpc.StreamServerInterceptor {
	return func(
		srv any,
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) (err error) {
		ctx := ss.Context()
		defer func() {
			if r := recover(); r != nil {
				logger.Errorw(ctx, "Recovered from panic in gRPC stream handler",
					"method", info.FullMethod,
					"panic", fmt.Sprintf("%v", r),
				)
				err = status.Error(codes.Internal, "internal server error")
			}
		}()
		return handler(srv, ss)
	}
}
