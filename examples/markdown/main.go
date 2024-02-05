package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"

	"github.com/halliday/go-problems"
)

func main() {
	addr := flag.String("addr", ":8080", "address to listen on")

	http.Handle("/", http.HandlerFunc(index))
	http.Handle("/api/withdraw", http.HandlerFunc(postWithdraw))

	log.Printf("Listening on %s...", *addr)

	http.ListenAndServe(*addr, nil)
}

func index(resp http.ResponseWriter, req *http.Request) {
	http.ServeFile(resp, req, "index.html")
}

func postWithdraw(resp http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		serveErrorf(resp, req, ErrMethodNotAllowed, "Use POST instead of %s.", req.Method)
		return
	}

	body, err := io.ReadAll(req.Body)
	if err != nil {
		serveErrorf(resp, req, ErrBadRequest, "Cannot read request body.", err)
		return
	}
	amount, err := strconv.ParseFloat(string(body), 64)
	if err != nil {
		serveErrorf(resp, req, ErrBadRequest, "Cannot parse amount.", err)
		return
	}

	if err := withdraw(req.Context(), amount); err != nil {
		serveError(resp, req, err)
		return
	}
}

func withdraw(ctx context.Context, amount float64) error {
	if amount <= 0 {
		return errorf(ErrBadRequest, "The amount to withdraw must be a positive number.", "requestedAmount", amount, "availableAmount", 4200)
	}
	if amount > 4200 {
		return errorf(ErrOutOfCredits, "Cannot withdraw %.2f when you have only 4200.00 available.", amount, "requestedAmount", amount, "availableAmount", 4200)
	}
	return nil
}

//

// make sure the Error type implements the Problem interface
var _ problems.Problem = (*Error)(nil)

// it also implements the error interface
var _ error = (*Error)(nil)

const ProblemsLocation = "http://localhost/problems/"

func serveErrorf(resp http.ResponseWriter, req *http.Request, err *Error, format string, args ...any) {
	serveError(resp, req, errorf(err, format, args...))
}

func serveError(resp http.ResponseWriter, req *http.Request, err error) {
	if e, ok := err.(*Error); ok {
		p := *e // copy
		p.Instance = req.RequestURI
		p.Type = fmt.Sprintf("%s%d-%s", ProblemsLocation, e.Status, e.Type)
		problems.ServeProblem(resp, &p)
	} else {
		log.Printf("error: %v", err)
		serveError(resp, req, ErrInternalServerError)
	}
}

func errorf(err *Error, format string, args ...any) *Error {
	detail, data, wraps := problems.Pprintf(format, args...)
	return &Error{err.Type, err.Status, err.Title, detail, err.Instance, data, wraps}
}
