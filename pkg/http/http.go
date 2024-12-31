package http

import (
	"fmt"
	"io"
	nethttp "net/http"
	neturl "net/url"
	"time"
)

type Client struct {
	http *nethttp.Client
}

func NewClient(timeout time.Duration) *Client {
	return &Client{
		http: &nethttp.Client{Timeout: timeout},
	}
}

func (c *Client) Get(url string) ([]byte, int, error) {
	urlParsed, err := neturl.Parse(url)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to parse url: %w", err)
	}
	resp, err := c.http.Do(&nethttp.Request{
		Method: nethttp.MethodGet,
		URL:    urlParsed,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("failed to send request: %w", err)
	}
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to read body: %w", err)
	}
	if err := resp.Body.Close(); err != nil {
		return nil, 0, fmt.Errorf("failed to close body: %w", err)
	}
	return bodyBytes, resp.StatusCode, nil
}
