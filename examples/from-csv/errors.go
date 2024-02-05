//go:generate goerrors -with-struct

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

// ErrBadRequest means: Your request provides invalid parameters. Type: "bad-request", Status: 400
var ErrBadRequest = &Error{Type: "bad-request", Status: 400, Title: "Your request provides invalid parameters."}
