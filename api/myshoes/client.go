package myshoes

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"path"
	"strings"
)

// Client is a client for myshoes
type Client struct {
	HTTPClient http.Client
	URL        *url.URL

	UserAgent string
	Logger    *log.Logger
}

const (
	defaultUserAgent = "myshoes-sdk-go"
)

// NewClient create a Client
func NewClient(endpoint string, client *http.Client, logger *log.Logger) (*Client, error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to parse endpoint: %w", err)
	}

	httpClient := client
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	l := logger
	if l == nil {
		// Default is discard logger
		l = log.New(io.Discard, "", log.LstdFlags)
	}

	return &Client{
		HTTPClient: *httpClient,
		URL:        u,

		Logger: logger,
	}, nil
}

func (c *Client) newRequest(ctx context.Context, method, spath string, body io.Reader) (*http.Request, error) {
	u := *c.URL
	u.Path = path.Join(c.URL.Path, spath)

	req, err := http.NewRequestWithContext(ctx, method, u.String(), body)
	if err != nil {
		return nil, fmt.Errorf("failed to create a new HTTP request: %w", err)
	}

	ua := c.UserAgent
	if strings.EqualFold(ua, "") {
		ua = defaultUserAgent
	}
	req.Header.Set("User-Agent", ua)

	req.Header.Set("Content-Type", "application/json")

	return req, nil
}

// Error values
var (
	errCreateRequest = "failed to create request: %w"
	errRequest       = "failed to request: %w"
	errDecodeBody    = "failed to decodeBody: %w"
)
