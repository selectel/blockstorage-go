package v1

import "github.com/selectel/blockstorage-go/pkg/v1/internal/transport"

// Custom HTTPClient implementations receive authentication headers.
type HTTPClient = transport.HTTPClient

type Client = transport.Client

// Endpoint and Token are required. UserAgent and HTTPClient are optional.
type Config struct {
	Endpoint   string
	Token      string
	UserAgent  string
	HTTPClient HTTPClient
}

type Response = transport.Response

func NewClient(config Config) (*Client, error) {
	return transport.NewClient(config.Endpoint, config.Token, config.UserAgent, config.HTTPClient)
}
