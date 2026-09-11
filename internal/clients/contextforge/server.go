package contextforge

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
)

type createServerRequest struct {
	Server     ServerCreate `json:"server"`
	TeamID     string       `json:"team_id"`
	Visibility Visibility   `json:"visibility"`
}

type ServerCreate struct {
	ID          string   `json:"id,omitempty"`
	Name        string   `json:"name"`
	Description *string  `json:"description,omitempty"`
	Icon        *string  `json:"icon,omitempty"`
	Tags        []string `json:"tags"`

	AssociatedTools     []string `json:"associated_tools,omitempty"`
	AssociatedResources []string `json:"associated_resources,omitempty"`
	AssociatedPrompts   []string `json:"associated_prompts,omitempty"`
	AssociatedA2AAgents []string `json:"associated_a2a_agents,omitempty"`
}

type ServerUpdate struct {
	Name        *string     `json:"name,omitempty"`
	Description *string     `json:"description,omitempty"`
	Icon        *string     `json:"icon,omitempty"`
	Visibility  *Visibility `json:"visibility,omitempty"`
	TeamID      *string     `json:"teamId,omitempty"`

	Tags                []string `json:"tags"`
	AssociatedTools     []string `json:"associated_tools"`
	AssociatedResources []string `json:"associated_resources"`
	AssociatedPrompts   []string `json:"associated_prompts"`
	AssociatedA2AAgents []string `json:"associated_a2a_agents"`
}

type Server struct {
	ID                  string     `json:"id"`
	Name                string     `json:"name"`
	Description         string     `json:"description"`
	Icon                string     `json:"icon"`
	TeamID              string     `json:"teamId"`
	OwnerEmail          string     `json:"ownerEmail"`
	Visibility          Visibility `json:"visibility"`
	Enabled             bool       `json:"enabled"`
	Tags                []Tag      `json:"tags"`
	Version             int        `json:"version"`
	CreatedAt           string     `json:"createdAt"`
	UpdatedAt           string     `json:"updatedAt"`
	AssociatedToolIDs   []string   `json:"associatedToolIds"`
	AssociatedResources []string   `json:"associatedResources"`
	AssociatedPrompts   []string   `json:"associatedPrompts"`
	AssociatedA2AAgents []string   `json:"associatedA2aAgents"`
}

type Tag struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

func TagLabels(tags []Tag) []string {
	labels := make([]string, len(tags))
	for i, t := range tags {
		labels[i] = t.Label
	}
	return labels
}

type Visibility string

const (
	VisibilityTeam   Visibility = "team"
	VisibilityPublic Visibility = "public"
)

func encodeBody(in any) (io.Reader, error) {
	if in == nil {
		return nil, nil
	}
	b, err := json.Marshal(in)
	if err != nil {
		return nil, errors.Wrap(err, "cannot marshal request")
	}
	return bytes.NewReader(b), nil
}

func checkStatus(method, path string, statusCode int, raw []byte) error {
	if statusCode >= 200 && statusCode <= 299 {
		return nil
	}
	return &APIError{
		Method:     method,
		Path:       path,
		StatusCode: statusCode,
		Body:       string(raw),
	}
}

func decodeResponse(raw []byte, out any) error {
	if out == nil || len(raw) == 0 {
		return nil
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return errors.Wrap(err, "cannot decode response")
	}
	return nil
}

func (c *Client) GetServer(ctx context.Context, id string) (*Server, error) {
	var out Server
	if err := c.doJSON(ctx, http.MethodGet, "/v1/servers/"+url.PathEscape(id), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) CreateServer(ctx context.Context, teamID string, visibility Visibility, s ServerCreate) (*Server, error) {
	in := createServerRequest{
		Server:     s,
		TeamID:     teamID,
		Visibility: visibility,
	}

	var out Server
	if err := c.doJSON(ctx, http.MethodPost, "/v1/servers", in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) UpdateServer(ctx context.Context, id string, u ServerUpdate) (*Server, error) {
	var out Server
	if err := c.doJSON(ctx, http.MethodPut, "/v1/servers/"+url.PathEscape(id), u, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) DeleteServer(ctx context.Context, id string) error {
	if err := c.doJSON(ctx, http.MethodDelete, "/v1/servers/"+url.PathEscape(id), nil, nil); err != nil && !IsNotFound(err) {
		return err
	}
	return nil
}
