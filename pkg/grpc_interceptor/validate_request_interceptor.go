package grpcinterceptor

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/cyrus-wg/gobox/pkg/logger"
	"github.com/go-playground/validator/v10"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var validate *validator.Validate

func init() {
	validate = validator.New()
	validate.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
		if name == "-" {
			return ""
		}
		return name
	})
}

func ValidateRequestUnaryInterceptor(logRequestBody ...bool) grpc.UnaryServerInterceptor {
	enableRequestBodyLogging := true
	if len(logRequestBody) > 0 {
		enableRequestBodyLogging = logRequestBody[0]
	}

	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		logger.Infow(ctx, "gRPC request received",
			"method", info.FullMethod,
			"target_type", reflect.TypeOf(req),
		)

		if enableRequestBodyLogging {
			logger.Infow(ctx, "gRPC request body", "body", req)
		}

		if err := validateRequest(ctx, req); err != nil {
			return nil, err
		}

		return handler(ctx, req)
	}
}

func ValidateRequestStreamInterceptor(logRequestBody ...bool) grpc.StreamServerInterceptor {
	enableRequestBodyLogging := true
	if len(logRequestBody) > 0 {
		enableRequestBodyLogging = logRequestBody[0]
	}

	return func(
		srv any,
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		wrapped := &validatingStream{
			ServerStream:             ss,
			method:                   info.FullMethod,
			enableRequestBodyLogging: enableRequestBodyLogging,
		}

		return handler(srv, wrapped)
	}
}

func validateRequest(ctx context.Context, req any) error {
	if req == nil {
		return nil
	}

	if err := validate.Struct(req); err != nil {
		var invalidErr *validator.InvalidValidationError
		if errors.As(err, &invalidErr) {
			return nil
		}

		var validationErrs validator.ValidationErrors
		if !errors.As(err, &validationErrs) {
			logger.Errorw(ctx, "Failed to process gRPC request validation",
				"target_type", reflect.TypeOf(req),
				"error", err,
			)
			return status.Error(codes.Internal, "failed to process request validation")
		}

		var fieldErrors []string
		for _, fe := range validationErrs {
			message := fmt.Sprintf("%s: failed %s validation", fe.Field(), fe.Tag())
			if fe.Param() != "" {
				message += fmt.Sprintf(" (expected: %s)", fe.Param())
			}
			fieldErrors = append(fieldErrors, message)
		}

		logger.Infow(ctx, "gRPC request validation failed",
			"target_type", reflect.TypeOf(req),
			"request", req,
			"validation_errors", fieldErrors,
		)

		return status.Error(codes.InvalidArgument, strings.Join(fieldErrors, "; "))
	}

	return nil
}

type validatingStream struct {
	grpc.ServerStream
	method                   string
	enableRequestBodyLogging bool
}

func (s *validatingStream) RecvMsg(m any) error {
	if err := s.ServerStream.RecvMsg(m); err != nil {
		return err
	}

	ctx := s.ServerStream.Context()

	logger.Infow(ctx, "gRPC stream message received",
		"method", s.method,
		"target_type", reflect.TypeOf(m),
	)

	if s.enableRequestBodyLogging {
		logger.Infow(ctx, "gRPC stream message body", "body", m)
	}

	if err := validateRequest(ctx, m); err != nil {
		return err
	}

	return nil
}
