package database

import (
	"errors"
	"fmt"
)

// ErrInvalidDatabase is returned when a database name (constructor option or
// per-call override) contains characters outside [A-Za-z0-9_-]. The empty
// string is valid and means the server's default database.
var ErrInvalidDatabase = errors.New("invalid database name")

// ErrConflictingAuth is returned by New when more than one authentication
// option is given (e.g. both WithBasicAuth and WithBearerToken). The last
// one silently winning is a security-relevant footgun, so it is a loud
// constructor-time error instead.
var ErrConflictingAuth = errors.New("conflicting authentication options")

type VersionError struct {
	ServerVersion string
	MinRequired   string
}

func (e *VersionError) Error() string {
	return fmt.Sprintf("unsupported neo4j version %q, requires %s or newer", e.ServerVersion, e.MinRequired)
}

type StatementError struct {
	Code    string
	Message string
}

func (e *StatementError) Error() string {
	return fmt.Sprintf("statement error %s: %s", e.Code, e.Message)
}

// TransportError describes a Query API request that failed at the transport
// layer: a non-2xx HTTP response, or a request that never got a usable
// response (network, TLS, timeout). Body holds the raw server response for
// diagnostics; it may echo the submitted statement text and parameters, so
// treat it as sensitive and do not log it verbatim.
type TransportError struct {
	// StatusCode is the HTTP status code, 0 when no response was received.
	StatusCode int
	// Body is the raw response body (may be empty); see the type comment.
	Body []byte
	// Err is the underlying error when one exists (e.g. the network error
	// for an unreachable server).
	Err error
}

func (e *TransportError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("transport error %d: %v", e.StatusCode, e.Err)
	}
	return fmt.Sprintf("transport error %d", e.StatusCode)
}

func (e *TransportError) Unwrap() error { return e.Err }

type CommitError struct {
	TransactionID string
	Ambiguous     bool
	Err           error
}

func (e *CommitError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("commit error for tx %s: %v (ambiguous=%v)", e.TransactionID, e.Err, e.Ambiguous)
	}
	return fmt.Sprintf("commit error for tx %s", e.TransactionID)
}

func (e *CommitError) Unwrap() error { return e.Err }
