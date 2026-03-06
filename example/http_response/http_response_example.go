package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"

	httperror "github.com/cyrus-wg/gobox/pkg/http_error"
	httpresponse "github.com/cyrus-wg/gobox/pkg/http_response"
	"github.com/cyrus-wg/gobox/pkg/logger"
)

func main() {
	logger.InitGlobalLogger(logger.LoggerConfig{})
	_ = httperror.SetErrorIDLength(0)

	fmt.Println("=== http_response package examples ===")
	fmt.Println()

	ctx := context.Background()

	// --- SendJSONResponse ---
	fmt.Println("// SendJSONResponse (200 OK)")
	w := httptest.NewRecorder()
	httpresponse.SendJSONResponse(ctx, w, map[string]string{"status": "healthy"})
	fmt.Printf("  Code: %d  Body: %s\n", w.Code, w.Body.String())

	// --- SendJSONResponseWithStatus ---
	fmt.Println("// SendJSONResponseWithStatus (201 Created)")
	w = httptest.NewRecorder()
	httpresponse.SendJSONResponseWithStatus(ctx, w, http.StatusCreated, map[string]int{"id": 42})
	fmt.Printf("  Code: %d  Body: %s\n", w.Code, w.Body.String())

	// --- Nil payload → {} ---
	fmt.Println("// Nil payload → {}")
	w = httptest.NewRecorder()
	httpresponse.SendJSONResponse(ctx, w, nil)
	fmt.Printf("  Code: %d  Body: %s\n", w.Code, w.Body.String())

	// --- SendErrorJSONResponse ---
	fmt.Println("// SendErrorJSONResponse (HTTPError)")
	w = httptest.NewRecorder()
	httpErr := httperror.NewBadRequestError("BAD_INPUT", "invalid email", nil, nil)
	httpresponse.SendErrorJSONResponse(ctx, w, httpErr)
	fmt.Printf("  Code: %d  Body: %s\n", w.Code, w.Body.String())

	fmt.Println("// SendErrorJSONResponse (generic error → 500)")
	w = httptest.NewRecorder()
	httpresponse.SendErrorJSONResponse(ctx, w, errors.New("unexpected failure"))
	fmt.Printf("  Code: %d  Body: %s\n", w.Code, w.Body.String())

	// --- SendPlainTextResponse ---
	fmt.Println("// SendPlainTextResponse")
	w = httptest.NewRecorder()
	httpresponse.SendPlainTextResponse(ctx, w, "pong")
	fmt.Printf("  Code: %d  Content-Type: %s  Body: %s\n", w.Code, w.Header().Get("Content-Type"), w.Body.String())
}
