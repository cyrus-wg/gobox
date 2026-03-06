package main

import (
	"encoding/json"
	"errors"
	"fmt"

	httperror "github.com/cyrus-wg/gobox/pkg/http_error"
)

func main() {
	fmt.Println("=== http_error package examples ===")
	fmt.Println()

	// --- Configuration ---
	fmt.Println("// Configure error ID length (0 to disable)")
	_ = httperror.SetErrorIDLength(8)
	fmt.Println("ErrorIDLength:", httperror.GetErrorIDLength())
	fmt.Println()

	// --- Create errors with convenience constructors ---
	fmt.Println("// Convenience constructors")
	badReq := httperror.NewBadRequestError("INVALID_EMAIL", "email format is invalid", nil, map[string]string{"field": "email"})
	printJSON("BadRequest", badReq)

	notFound := httperror.NewNotFoundError("USER_NOT_FOUND", "user does not exist", nil, nil)
	printJSON("NotFound", notFound)
	fmt.Println()

	// --- Error() and Unwrap() ---
	fmt.Println("// Error() for logging")
	orig := errors.New("database connection refused")
	ise := httperror.NewInternalServerError("", "", orig, nil)
	fmt.Println(ise.Error())
	fmt.Println("Unwrap:", errors.Unwrap(ise))
	fmt.Println()

	// --- ConvertToHTTPError ---
	fmt.Println("// ConvertToHTTPError wraps generic errors as 500")
	plain := errors.New("something broke")
	httpErr, converted := httperror.ConvertToHTTPError(plain)
	fmt.Printf("converted=%v, status=%d\n", converted, httpErr.HTTPStatusCode)

	already := httperror.NewForbiddenError("FORBIDDEN", "no access", nil, nil)
	httpErr2, converted2 := httperror.ConvertToHTTPError(already)
	fmt.Printf("converted=%v, status=%d\n", converted2, httpErr2.HTTPStatusCode)
	fmt.Println()

	// --- Extra info toggle ---
	fmt.Println("// ExtraInfo can be disabled globally for production")
	httperror.SetWithExtraInfo(false)
	fmt.Println("ExtraInfo enabled:", httperror.IsWithExtraInfoEnabled())
	httperror.SetWithExtraInfo(true)

	// --- Default ISE message/code ---
	fmt.Println()
	httperror.SetDefaultInternalServerErrorMessage("Something went wrong")
	httperror.SetDefaultInternalServerErrorCode("ISE")
	ise2 := httperror.NewInternalServerError("", "", nil, nil)
	printJSON("Default ISE", ise2)
}

func printJSON(label string, v any) {
	data, _ := json.MarshalIndent(v, "  ", "  ")
	fmt.Printf("%s:\n  %s\n", label, data)
}
