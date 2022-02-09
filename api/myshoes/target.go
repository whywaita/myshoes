package myshoes

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/whywaita/myshoes/pkg/web"
)

// CreateTarget create a target
func (c *Client) CreateTarget(ctx context.Context, param web.TargetCreateParam) (*web.UserTarget, error) {
	spath := "/target"

	jb, err := json.Marshal(param)
	if err != nil {
		return nil, fmt.Errorf("failed to json.Marshal: %w", err)
	}

	req, err := c.newRequest(ctx, http.MethodPost, spath, bytes.NewBuffer(jb))
	if err != nil {
		return nil, fmt.Errorf(errCreateRequest, err)
	}

	var target web.UserTarget
	if err := c.request(req, &target); err != nil {
		return nil, fmt.Errorf(errRequest, err)
	}

	return &target, nil
}

// GetTarget get a target
func (c *Client) GetTarget(ctx context.Context, targetID string) (*web.UserTarget, error) {
	spath := fmt.Sprintf("/target/%s", targetID)

	req, err := c.newRequest(ctx, http.MethodGet, spath, nil)
	if err != nil {
		return nil, fmt.Errorf(errCreateRequest, err)
	}

	var target web.UserTarget
	if err := c.request(req, &target); err != nil {
		return nil, fmt.Errorf(errRequest, err)
	}

	return &target, nil
}

// UpdateTarget update a target
func (c *Client) UpdateTarget(ctx context.Context, targetID string, param web.TargetCreateParam) (*web.UserTarget, error) {
	spath := fmt.Sprintf("/target/%s", targetID)

	jb, err := json.Marshal(param)
	if err != nil {
		return nil, fmt.Errorf("failed to json.Marshal: %w", err)
	}

	req, err := c.newRequest(ctx, http.MethodPost, spath, bytes.NewBuffer(jb))
	if err != nil {
		return nil, fmt.Errorf(errCreateRequest, err)
	}

	var target web.UserTarget
	if err := c.request(req, &target); err != nil {
		return nil, fmt.Errorf(errRequest, err)
	}

	return &target, nil
}

// DeleteTarget delete a target
func (c *Client) DeleteTarget(ctx context.Context, targetID string) error {
	spath := fmt.Sprintf("/target/%s", targetID)

	req, err := c.newRequest(ctx, http.MethodDelete, spath, nil)
	if err != nil {
		return fmt.Errorf(errCreateRequest, err)
	}

	var i interface{} // this endpoint return N/A
	if err := c.request(req, &i); err != nil {
		return fmt.Errorf(errRequest, err)
	}

	return nil
}

// ListTarget get a list of target
func (c *Client) ListTarget(ctx context.Context) ([]web.UserTarget, error) {
	spath := "/target"

	req, err := c.newRequest(ctx, http.MethodGet, spath, nil)
	if err != nil {
		return nil, fmt.Errorf(errCreateRequest, err)
	}

	var targets []web.UserTarget
	if err := c.request(req, targets); err != nil {
		return nil, fmt.Errorf(errRequest, err)
	}

	return targets, nil
}
