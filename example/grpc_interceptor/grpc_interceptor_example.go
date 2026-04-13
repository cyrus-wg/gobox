package main

import (
	"fmt"

	grpcinterceptor "github.com/cyrus-wg/gobox/pkg/grpc_interceptor"
	"github.com/cyrus-wg/gobox/pkg/logger"
	"google.golang.org/grpc"
)

func main() {
	logger.InitGlobalLogger(logger.LoggerConfig{})
	defer logger.Flush()

	case1_RecoverInterceptors()
	printSeparator()
	case2_RequestLoggingInterceptors()
	printSeparator()
	case3_ValidationInterceptors()
	printSeparator()
	case4_BypassLoggingWithAntPattern()
	printSeparator()
	case5_BypassLoggingWithRegex()
	printSeparator()
	case6_BypassLoggingMultipleRules()
	printSeparator()
	case7_CombinedProductionSetup()
}

// case1_RecoverInterceptors demonstrates setting up panic recovery interceptors.
//
// These interceptors catch any panics inside gRPC handlers and convert them
// to Internal gRPC status errors, preventing the server from crashing.
//
// Usage:
//
//	grpc.NewServer(
//	    grpc.ChainUnaryInterceptor(grpcinterceptor.RecoverUnaryInterceptor()),
//	    grpc.ChainStreamInterceptor(grpcinterceptor.RecoverStreamInterceptor()),
//	)
func case1_RecoverInterceptors() {
	fmt.Println("Case 1: Recover Interceptors")

	_ = grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			grpcinterceptor.RecoverUnaryInterceptor(),
		),
		grpc.ChainStreamInterceptor(
			grpcinterceptor.RecoverStreamInterceptor(),
		),
	)

	fmt.Println("  Server created with recover interceptors")
}

// case2_RequestLoggingInterceptors demonstrates request logging interceptors.
//
// Parameters:
//   - logRequestDetails: log method, peer address, metadata, etc.
//   - logCompleteTime: log latency and gRPC status code on completion
//
// Usage:
//
//	grpc.NewServer(
//	    grpc.ChainUnaryInterceptor(grpcinterceptor.RequestUnaryInterceptor(true, true)),
//	    grpc.ChainStreamInterceptor(grpcinterceptor.RequestStreamInterceptor(true, true)),
//	)
func case2_RequestLoggingInterceptors() {
	fmt.Println("Case 2: Request Logging Interceptors")

	// Log both request details and completion time
	_ = grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			grpcinterceptor.RequestUnaryInterceptor(true, true),
		),
		grpc.ChainStreamInterceptor(
			grpcinterceptor.RequestStreamInterceptor(true, true),
		),
	)
	fmt.Println("  Server created with full request logging (details + latency)")

	// Log only completion time (latency tracking)
	_ = grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			grpcinterceptor.RequestUnaryInterceptor(false, true),
		),
	)
	fmt.Println("  Server created with latency-only logging")

	// Silent mode — no logging, only request ID injection
	_ = grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			grpcinterceptor.RequestUnaryInterceptor(false, false),
		),
	)
	fmt.Println("  Server created with silent mode (request ID only)")
}

// case3_ValidationInterceptors demonstrates request validation interceptors.
//
// These interceptors use the `validate` struct tags to validate incoming
// gRPC request messages. Invalid requests are rejected with InvalidArgument status.
//
// The optional logRequestBody parameter controls whether request bodies are logged.
//
// Usage:
//
//	grpc.NewServer(
//	    grpc.ChainUnaryInterceptor(grpcinterceptor.ValidateRequestUnaryInterceptor()),
//	    grpc.ChainStreamInterceptor(grpcinterceptor.ValidateRequestStreamInterceptor()),
//	)
func case3_ValidationInterceptors() {
	fmt.Println("Case 3: Validation Interceptors")

	// With request body logging enabled (default)
	_ = grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			grpcinterceptor.ValidateRequestUnaryInterceptor(),
		),
		grpc.ChainStreamInterceptor(
			grpcinterceptor.ValidateRequestStreamInterceptor(),
		),
	)
	fmt.Println("  Server created with validation interceptors (body logging enabled)")

	// With request body logging disabled
	_ = grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			grpcinterceptor.ValidateRequestUnaryInterceptor(false),
		),
		grpc.ChainStreamInterceptor(
			grpcinterceptor.ValidateRequestStreamInterceptor(false),
		),
	)
	fmt.Println("  Server created with validation interceptors (body logging disabled)")
}

// case4_BypassLoggingWithAntPattern demonstrates bypassing logging for specific
// gRPC methods using Ant-style glob patterns.
//
// Supported Ant-style wildcards:
//   - *  matches any sequence within a single path segment
//   - ** matches any sequence across path segments
//   - ?  matches exactly one character
//
// Usage:
//
//	grpcinterceptor.RequestUnaryInterceptor(true, true,
//	    grpcinterceptor.BypassRequestLogging{Method: "/grpc.health.v1.Health/*"},
//	)
func case4_BypassLoggingWithAntPattern() {
	fmt.Println("Case 4: Bypass Logging with Ant-style Pattern")

	_ = grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			grpcinterceptor.RequestUnaryInterceptor(true, true,
				grpcinterceptor.BypassRequestLogging{
					Method: "/grpc.health.v1.Health/*",
				},
			),
		),
	)

	fmt.Println("  Health check endpoints will not be logged")
}

// case5_BypassLoggingWithRegex demonstrates bypassing logging using regex patterns.
//
// Set IsRegex to true to treat the Method field as a regular expression.
// The regex is anchored with ^ and $ automatically.
//
// Usage:
//
//	grpcinterceptor.RequestUnaryInterceptor(true, true,
//	    grpcinterceptor.BypassRequestLogging{
//	        Method:  `/grpc\.health.*`,
//	        IsRegex: true,
//	    },
//	)
func case5_BypassLoggingWithRegex() {
	fmt.Println("Case 5: Bypass Logging with Regex Pattern")

	_ = grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			grpcinterceptor.RequestUnaryInterceptor(true, true,
				grpcinterceptor.BypassRequestLogging{
					Method:  `/grpc\.health.*`,
					IsRegex: true,
				},
			),
		),
	)

	fmt.Println("  All health-related endpoints will not be logged")
}

// case6_BypassLoggingMultipleRules demonstrates combining multiple bypass rules.
//
// Multiple BypassRequestLogging entries can be passed as variadic arguments.
// Ant-style and regex patterns can be mixed freely.
//
// Usage:
//
//	grpcinterceptor.RequestUnaryInterceptor(true, true,
//	    grpcinterceptor.BypassRequestLogging{Method: "/grpc.health.v1.Health/*"},
//	    grpcinterceptor.BypassRequestLogging{Method: `/grpc\.reflection.*`, IsRegex: true},
//	)
func case6_BypassLoggingMultipleRules() {
	fmt.Println("Case 6: Bypass Logging with Multiple Rules")

	_ = grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			grpcinterceptor.RequestUnaryInterceptor(true, true,
				grpcinterceptor.BypassRequestLogging{
					Method: "/grpc.health.v1.Health/*",
				},
				grpcinterceptor.BypassRequestLogging{
					Method:  `/grpc\.reflection.*`,
					IsRegex: true,
				},
			),
		),
	)

	fmt.Println("  Health check and reflection endpoints will not be logged")
}

// case7_CombinedProductionSetup demonstrates a recommended production setup
// with all interceptors chained together.
//
// Recommended order:
//  1. Recover (outermost — catches panics from all inner interceptors)
//  2. Request logging (logs method, latency, and metadata)
//  3. Validation (innermost — validates before reaching the handler)
//
// Usage:
//
//	grpc.NewServer(
//	    grpc.ChainUnaryInterceptor(
//	        grpcinterceptor.RecoverUnaryInterceptor(),
//	        grpcinterceptor.RequestUnaryInterceptor(true, true, bypass...),
//	        grpcinterceptor.ValidateRequestUnaryInterceptor(),
//	    ),
//	    grpc.ChainStreamInterceptor(
//	        grpcinterceptor.RecoverStreamInterceptor(),
//	        grpcinterceptor.RequestStreamInterceptor(true, true, bypass...),
//	        grpcinterceptor.ValidateRequestStreamInterceptor(),
//	    ),
//	)
func case7_CombinedProductionSetup() {
	fmt.Println("Case 7: Combined Production Setup")

	bypass := []grpcinterceptor.BypassRequestLogging{
		{Method: "/grpc.health.v1.Health/*"},
		{Method: `/grpc\.reflection.*`, IsRegex: true},
	}

	_ = grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			grpcinterceptor.RecoverUnaryInterceptor(),
			grpcinterceptor.RequestUnaryInterceptor(true, true, bypass...),
			grpcinterceptor.ValidateRequestUnaryInterceptor(),
		),
		grpc.ChainStreamInterceptor(
			grpcinterceptor.RecoverStreamInterceptor(),
			grpcinterceptor.RequestStreamInterceptor(true, true, bypass...),
			grpcinterceptor.ValidateRequestStreamInterceptor(),
		),
	)

	fmt.Println("  Production server created with recover + logging + validation")
}

func printSeparator() {
	fmt.Println("---")
}
