package queryapi

import (
	"net/http"
)

// Service is the internal Query API backend used by external/database.
// It is not exported outside the module.
type Service struct {
	BaseURL    string
	Database   string
	HTTPClient *http.Client
	AuthHeader string
}

func NewService(baseURL, database, authHeader string, client *http.Client) *Service {
	if client == nil {
		client = http.DefaultClient
	}
	return &Service{
		BaseURL:    baseURL,
		Database:   database,
		HTTPClient: client,
		AuthHeader: authHeader,
	}
}
