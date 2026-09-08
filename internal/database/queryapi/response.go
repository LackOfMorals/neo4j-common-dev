package queryapi

import "encoding/json"

// NotificationWire is the wire shape of a server notification. It appears
// both as the top-level `notifications` array on buffered query responses
// and inside the stream Summary event body, with the same fields in both.
type NotificationWire struct {
	Code        string `json:"code"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Severity    string `json:"severity"`
	Category    string `json:"category"`
	Position    *struct {
		Offset int `json:"offset"`
		Line   int `json:"line"`
		Column int `json:"column"`
	} `json:"position"`
}

// QueryResponse is the buffered (non-streaming) query response. Note that
// the server does not populate counters on this path — counters arrive via
// the stream Summary event (QuerySummary) or the Bolt driver. Timings are
// in milliseconds.
type QueryResponse struct {
	Data                 DataSection        `json:"data"`
	QueryType            string             `json:"queryType,omitempty"`
	Bookmarks            []string           `json:"bookmarks,omitempty"`
	ResultAvailableAfter int64              `json:"resultAvailableAfter,omitempty"`
	ResultConsumedAfter  int64              `json:"resultConsumedAfter,omitempty"`
	Notifications        []NotificationWire `json:"notifications,omitempty"`
	Errors               []ErrorInfo        `json:"errors,omitempty"`
}

type DataSection struct {
	Fields []string            `json:"fields"`
	Values [][]json.RawMessage `json:"values"`
}

type ErrorInfo struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Legacy types kept for compatibility
type Result struct {
	Columns []string `json:"columns"`
	Data    []Row    `json:"data"`
}

type Row struct {
	Row []json.RawMessage `json:"row"`
}
