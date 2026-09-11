package contextforge

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
)

type Client struct {
	base  *url.URL
	token string
	http  *http.Client
}

type APIError struct {
	Method     string
	Path       string
	StatusCode int
	Body       string
}

func (c *Client) BaseURL() string {
	return c.base.String()
}

func NewClient(baseURL string, token []byte) (*Client, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid base URL: %w", err)
	}
	return &Client{
		base:  u,
		token: string(token),
		http:  &http.Client{Timeout: 30 * time.Second},
	}, nil
}

func (e *APIError) Error() string {
	msg := detailOrRaw(e.Body)
	if msg == "" {
		return fmt.Sprintf("contextforge: %s %s: HTTP %d", e.Method, e.Path, e.StatusCode)
	}
	return fmt.Sprintf("contextforge: %s %s: HTTP %d: %s", e.Method, e.Path, e.StatusCode, msg)
}

func IsNotFound(err error) bool {
	var a *APIError
	return errors.As(err, &a) && a.StatusCode == 404
}

func IsConflict(err error) bool {
	var a *APIError
	return errors.As(err, &a) && a.StatusCode == 409
}

func IsBadGateway(err error) bool {
	var a *APIError
	return errors.As(err, &a) && a.StatusCode == 502
}

func detailOrRaw(body string) string {
	if body == "" {
		return ""
	}

	var s struct {
		Detail string `json:"detail"`
	}
	if json.Unmarshal([]byte(body), &s) == nil && s.Detail != "" {
		return truncate(s.Detail, 400)
	}

	var v struct {
		Detail []struct {
			Loc []any  `json:"loc"`
			Msg string `json:"msg"`
		} `json:"detail"`
	}
	if json.Unmarshal([]byte(body), &v) == nil && len(v.Detail) > 0 {
		parts := make([]string, 0, len(v.Detail))
		for _, d := range v.Detail {
			parts = append(parts, fmt.Sprintf("%v: %s", d.Loc, d.Msg))
		}
		return truncate(strings.Join(parts, "; "), 400)
	}
	return truncate(body, 400)
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func (c *Client) doJSON(ctx context.Context, method, path string, in, out any) error {
	body, err := encodeBody(in)
	if err != nil {
		return err
	}

	rel, err := url.Parse(path)
	if err != nil {
		return errors.Wrap(err, "cannot parse path")
	}
	full := c.base.JoinPath(rel.Path)
	full.RawQuery = rel.RawQuery

	req, err := http.NewRequestWithContext(ctx, method, full.String(), body)
	if err != nil {
		return errors.Wrap(err, "cannot build request")
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	res, err := c.http.Do(req)
	if err != nil {
		return errors.Wrap(err, "request failed")
	}
	defer func() {
		_ = res.Body.Close()
	}()

	raw, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return errors.Wrap(err, "cannot read response")
	}

	if err := checkStatus(method, path, res.StatusCode, raw); err != nil {
		return err
	}

	return decodeResponse(raw, out)
}
