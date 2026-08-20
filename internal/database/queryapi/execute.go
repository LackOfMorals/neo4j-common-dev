package queryapi

import (
	"context"
	"encoding/json"
)

type Service struct {
	// placeholder
}

func (s *Service) Execute(ctx context.Context, req ExecuteRequest) (*QueryResponse, error) {
	// TODO: marshal req, POST to /db/{db}/query/v2, decode response
	return &QueryResponse{}, nil
}

func (s *Service) ExecuteStream(ctx context.Context, req ExecuteRequest) (<-chan StreamEvent, error) {
	// TODO: POST with Accept jsonl, stream events
	ch := make(chan StreamEvent)
	close(ch)
	return ch, nil
}

func EncodeValue(v any) (json.RawMessage, error) {
	// TODO: mirror DecodeValue for outbound params
	return json.RawMessage(`{"$type":"Null","_value":null}`), nil
}
