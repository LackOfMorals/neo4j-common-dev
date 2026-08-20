package queryapi

import "bufio"

type StreamEvent struct {
	Event string `json:"$event"`
	Body  []byte `json:"_body"`
}

func NewStreamScanner(r *bufio.Reader) *bufio.Scanner {
	s := bufio.NewScanner(r)
	s.Buffer(make([]byte, 64*1024), 10*1024*1024)
	return s
}
