package httpclient

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

const DefaultMaxResponseSize = 10 * 1024 * 1024

type Service struct {
	BaseURL string
	Timeout time.Duration
	client  *http.Client
}

func New(baseURL string, timeout time.Duration) *Service {
	return &Service{
		BaseURL: baseURL,
		Timeout: timeout,
		client: &http.Client{Timeout: timeout},
	}
}

func (s *Service) Do(ctx context.Context, method, endpoint string, headers map[string]string, body []byte) (*http.Response, []byte, error) {
	url := s.BaseURL + endpoint
	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return nil, nil, err
	}
	if headers != nil {
		for k, v := range headers {
			req.Header.Set(k, v)
		}
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	limited := io.LimitReader(resp.Body, int64(DefaultMaxResponseSize)+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return resp, nil, fmt.Errorf("read response: %w", err)
	}
	return resp, data, nil
}
