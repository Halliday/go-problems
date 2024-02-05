//go:generate goproblems -i "codes/*.md" -with-struct

package main

// Error is the generic error type for this package.
type Error struct {
	Type     string
	Status   int
	Title    string
	Detail   string
	Instance string
	Data     map[string]any
	Wraps    error
}

// Error implements the error interface.
func (e Error) Error() string {
	return e.Detail
}

// Unwrap implements the errors.Unwrap function.
func (e Error) Unwrap() error {
	return e.Wraps
}

// Problem implements the Problem interface.
func (e Error) Problem() (typ string, title string, status int, detail string, instance string, data map[string]any) {
	return e.Type, e.Title, e.Status, e.Detail, e.Instance, e.Data
}

// ErrBadRequest means: "Bad Request" Type: "bad-request", Status: 400
var ErrBadRequest = &Error{Type: "bad-request", Status: 400, Title: "Bad Request"}

// ErrInternalServerError means: "Internal Server Error" Type: "internal-server-error", Status: 500
var ErrInternalServerError = &Error{Type: "internal-server-error", Status: 500, Title: "Internal Server Error"}

// ErrMethodNotAllowed means: "Method Not Allowed" Type: "method-not-allowed", Status: 405
var ErrMethodNotAllowed = &Error{Type: "method-not-allowed", Status: 405, Title: "Method Not Allowed"}

// ErrOutOfCredits means: "Out Of Credits" Type: "out-of-credits", Status: 4001
var ErrOutOfCredits = &Error{Type: "out-of-credits", Status: 4001, Title: "Out Of Credits"}
