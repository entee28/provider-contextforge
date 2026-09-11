package contextforge

import (
	"context"
	"net/http"
	"net/url"
)

type A2AAgent struct {
	ID                 string         `json:"id"`
	Name               string         `json:"name"`
	Slug               string         `json:"slug"`
	Description        string         `json:"description"`
	EndpointURL        string         `json:"endpoint_url"`
	AgentType          string         `json:"agent_type"`
	ProtocolVersion    string         `json:"protocol_version"`
	Capabilities       map[string]any `json:"capabilities"`
	Config             map[string]any `json:"config"`
	Enabled            *bool          `json:"enabled"`
	Reachable          *bool          `json:"reachable"`
	TeamID             string         `json:"team_id"`
	Visibility         Visibility     `json:"visibility"`
	Tags               []Tag          `json:"tags"`
	PassthroughHeaders []string       `json:"passthrough_headers"`
	LastInteraction    string         `json:"last_interaction"`
	CreatedAt          string         `json:"created_at"`
	UpdatedAt          string         `json:"updated_at"`
}

type A2AAgentCreate struct {
	Name               string         `json:"name"`
	EndpointURL        string         `json:"endpoint_url"`
	Description        *string        `json:"description,omitempty"`
	AgentType          *string        `json:"agent_type,omitempty"`
	ProtocolVersion    *string        `json:"protocol_version,omitempty"`
	TeamID             string         `json:"team_id"`
	Visibility         Visibility     `json:"visibility"`
	Tags               []string       `json:"tags,omitempty"`
	Capabilities       map[string]any `json:"capabilities,omitempty"`
	Config             map[string]any `json:"config,omitempty"`
	PassthroughHeaders []string       `json:"passthrough_headers,omitempty"`

	AuthType            *string             `json:"auth_type,omitempty"`
	AuthUsername        *string             `json:"auth_username,omitempty"`
	AuthPassword        *string             `json:"auth_password,omitempty"`
	AuthToken           *string             `json:"auth_token,omitempty"`
	AuthHeaders         []map[string]string `json:"auth_headers,omitempty"`
	AuthQueryParamKey   *string             `json:"auth_query_param_key,omitempty"`
	AuthQueryParamValue *string             `json:"auth_query_param_value,omitempty"`
	OAuthConfig         map[string]any      `json:"oauth_config,omitempty"`
}

type A2AAgentUpdate struct {
	Name                *string             `json:"name,omitempty"`
	EndpointURL         *string             `json:"endpoint_url,omitempty"`
	Description         *string             `json:"description,omitempty"`
	AgentType           *string             `json:"agent_type,omitempty"`
	ProtocolVersion     *string             `json:"protocol_version,omitempty"`
	TeamID              *string             `json:"team_id,omitempty"`
	Visibility          *Visibility         `json:"visibility,omitempty"`
	Tags                []string            `json:"tags,omitempty"`
	Capabilities        map[string]any      `json:"capabilities,omitempty"`
	Config              map[string]any      `json:"config,omitempty"`
	PassthroughHeaders  []string            `json:"passthrough_headers,omitempty"`
	AuthType            *string             `json:"auth_type,omitempty"`
	AuthUsername        *string             `json:"auth_username,omitempty"`
	AuthPassword        *string             `json:"auth_password,omitempty"`
	AuthToken           *string             `json:"auth_token,omitempty"`
	AuthHeaders         []map[string]string `json:"auth_headers,omitempty"`
	AuthQueryParamKey   *string             `json:"auth_query_param_key,omitempty"`
	AuthQueryParamValue *string             `json:"auth_query_param_value,omitempty"`
	OAuthConfig         map[string]any      `json:"oauth_config,omitempty"`
}

func (c *Client) GetA2AAgent(ctx context.Context, idOrNameOrSlug string) (*A2AAgent, error) {
	var out A2AAgent
	if err := c.doJSON(ctx, http.MethodGet, "/v1/a2aagents/"+url.PathEscape(idOrNameOrSlug), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) DeleteA2AAgent(ctx context.Context, id string) error {
	if err := c.doJSON(ctx, http.MethodDelete, "/v1/a2aagents/"+url.PathEscape(id), nil, nil); err != nil && !IsNotFound(err) {
		return err
	}
	return nil
}

func (c *Client) UpdateA2AAgent(ctx context.Context, id string, in A2AAgentUpdate) (*A2AAgent, error) {
	var out A2AAgent
	if err := c.doJSON(ctx, http.MethodPut, "/v1/a2aagents/"+url.PathEscape(id), in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) CreateA2AAgent(ctx context.Context, in A2AAgentCreate) (*A2AAgent, error) {
	var out A2AAgent
	if err := c.doJSON(ctx, http.MethodPost, "/v1/a2aagents/", in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
