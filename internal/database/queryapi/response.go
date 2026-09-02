package queryapi

import "encoding/json"

type QueryResponse struct {
	Data      DataSection `json:"data"`
	QueryType string      `json:"queryType,omitempty"`
	Bookmarks []string    `json:"bookmarks,omitempty"`
	Errors    []ErrorInfo `json:"errors,omitempty"`
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
