package queryapi

type TransportError struct {
	StatusCode int
	Body       []byte
}

type StatementError struct {
	Errors []Neo4jError
}

type Neo4jError struct {
	Code    string
	Message string
}
