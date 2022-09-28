package myshoes

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/whywaita/myshoes/pkg/web"
)

func decodeBody(resp *http.Response, out interface{}) error {
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to io.ReadAll(resp.Body): %w", err)
	}

	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("failed to json.Unmarshal() (out: %s): %w", body, err)
	}
	return nil
}

func decodeErrorBody(resp *http.Response) error {
	var e web.ErrorResponse

	if err := decodeBody(resp, &e); err != nil {
		return fmt.Errorf(errDecodeBody, err)
	}

	return fmt.Errorf("%s", e.Error)
}

func (c *Client) request(req *http.Request, out interface{}) error {
	c.Logger.Printf("Do request: %+v", req)
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to do HTTP request: %w", err)
	}

	switch {
	case resp.StatusCode == http.StatusNoContent:
		return nil
	case resp.StatusCode >= 400:
		return decodeErrorBody(resp)
	}

	if err := decodeBody(resp, out); err != nil {
		return fmt.Errorf(errDecodeBody, err)
	}
	return nil
}
