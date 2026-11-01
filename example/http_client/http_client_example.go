package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"

	httpclient "github.com/cyrus-wg/gobox/pkg/http_client"
)

func main() {
	fmt.Println("=== http_client package examples ===")
	fmt.Println()

	// Start a local echo server for demonstration.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"method":       r.Method,
			"content_type": r.Header.Get("Content-Type"),
			"body":         string(body),
		})
	}))
	defer srv.Close()
	fmt.Println("Echo server at", srv.URL)
	fmt.Println()

	ctx := context.Background()

	// --- Package-level functions ---
	fmt.Println("// JSONGet")
	status, body, err := httpclient.JSONGet(ctx, srv.URL+"/items", nil)
	fmt.Printf("  status=%d err=%v body=%s\n", status, err, strings.TrimSpace(string(body)))

	fmt.Println("// JSONPost with struct body")
	payload := map[string]string{"name": "widget"}
	status, body, err = httpclient.JSONPost(ctx, srv.URL+"/items", payload, nil)
	fmt.Printf("  status=%d err=%v body=%s\n", status, err, strings.TrimSpace(string(body)))

	fmt.Println("// SendRequest with custom headers")
	status, body, err = httpclient.SendRequest(ctx, "GET", srv.URL+"/test", nil, map[string]string{
		"X-Custom": "value",
	})
	fmt.Printf("  status=%d err=%v body=%s\n", status, err, strings.TrimSpace(string(body)))
	fmt.Println()

	// --- Instance client ---
	fmt.Println("// Instance client (HTTPClient)")
	hc := httpclient.NewHTTPClient(nil)
	status, body, err = hc.JSONPut(ctx, srv.URL+"/items/1", map[string]int{"qty": 5}, nil)
	fmt.Printf("  PUT status=%d err=%v body=%s\n", status, err, strings.TrimSpace(string(body)))

	status, _, err = hc.JSONDelete(ctx, srv.URL+"/items/1", nil)
	fmt.Printf("  DELETE status=%d err=%v\n", status, err)
	fmt.Println()

	// --- Typed JSON decoding with generics ---
	fmt.Println("// GetJSON[T] — decode a 2xx response directly into a struct")
	type echoResponse struct {
		Method string `json:"method"`
		Body   string `json:"body"`
	}
	echo, status, err := httpclient.PostJSON[echoResponse](ctx, srv.URL+"/items", map[string]string{"name": "widget"}, nil)
	fmt.Printf("  status=%d err=%v method=%q body=%q\n", status, err, echo.Method, echo.Body)
	fmt.Println()

	// --- Typed error handling for non-2xx responses ---
	fmt.Println("// ResponseError — inspect status/body on failures via errors.As")
	failSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"missing"}`))
	}))
	defer failSrv.Close()

	_, _, err = httpclient.JSONGet(ctx, failSrv.URL, nil)
	if respErr, ok := httpclient.AsResponseError(err); ok {
		fmt.Printf("  status=%d body=%s\n", respErr.StatusCode, strings.TrimSpace(string(respErr.Body)))
	}
}
