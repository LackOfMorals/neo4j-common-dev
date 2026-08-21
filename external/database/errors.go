package database

import "fmt"

type VersionError struct {
    ServerVersion string
    MinRequired   string
}

func (e *VersionError) Error() string {
    return fmt.Sprintf("unsupported neo4j version %s, requires %s+", e.ServerVersion, e.MinRequired)
}

type StatementError struct {
    Code    string
    Message string
}

func (e *StatementError) Error() string {
    return fmt.Sprintf("statement error %s: %s", e.Code, e.Message)
}

type TransportError struct {
    StatusCode int
    Body       []byte
    Err        error
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
