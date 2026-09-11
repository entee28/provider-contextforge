package contextforge

import (
	"context"
	"net/http"
	"net/url"
)

type Gateway struct {
	ID                     string     `json:"id"`
	Name                   string     `json:"name"`
	Slug                   string     `json:"slug"`
	URL                    string     `json:"url"`
	Description            string     `json:"description"`
	Transport              string     `json:"transport"`
	TeamID                 string     `json:"teamId"`
	Visibility             Visibility `json:"visibility"`
	Tags                   []string   `json:"tags"`
	GatewayMode            string     `json:"gatewayMode"`
	Enabled                *bool      `json:"enabled"`
	Reachable              *bool      `json:"reachable"`
	Status                 string     `json:"status"`
	StatusMessage          string     `json:"statusMessage"`
	LastError              string     `json:"lastError"`
	RegistrationAttempts   int        `json:"registrationAttempts"`
	NextRetryAt            string     `json:"nextRetryAt"`
	LastSeen               string     `json:"lastSeen"`
	ToolCount              int        `json:"toolCount"`
	RefreshIntervalSeconds *int       `json:"refreshIntervalSeconds"`
	CreatedAt              string     `json:"createdAt"`
	UpdatedAt              string     `json:"updatedAt"`
}

type GatewayCreate struct {
	Name        string     `json:"name"`
	URL         string     `json:"url"`
	Description *string    `json:"description,omitempty"`
	Transport   string     `json:"transport"`
	TeamID      string     `json:"team_id"`
	Visibility  Visibility `json:"visibility"`
	Tags        []string   `json:"tags,omitempty"`

	AuthType            *string             `json:"auth_type,omitempty"`
	AuthUsername        *string             `json:"auth_username,omitempty"`
	AuthPassword        *string             `json:"auth_password,omitempty"`
	AuthToken           *string             `json:"auth_token,omitempty"`
	AuthHeaders         []map[string]string `json:"auth_headers,omitempty"`
	AuthQueryParamKey   *string             `json:"auth_query_param_key,omitempty"`
	AuthQueryParamValue *string             `json:"auth_query_param_value,omitempty"`
	OAuthConfig         map[string]any      `json:"oauth_config,omitempty"`
}

type GatewayUpdate struct {
	Name                   *string             `json:"name,omitempty"`
	URL                    *string             `json:"url,omitempty"`
	Description            *string             `json:"description,omitempty"`
	Transport              *string             `json:"transport,omitempty"`
	TeamID                 *string             `json:"team_id,omitempty"`
	Visibility             *Visibility         `json:"visibility,omitempty"`
	Tags                   []string            `json:"tags,omitempty"`
	GatewayMode            *string             `json:"gateway_mode,omitempty"`
	RefreshIntervalSeconds *int                `json:"refresh_interval_seconds,omitempty"`
	AuthType               *string             `json:"auth_type,omitempty"`
	AuthUsername           *string             `json:"auth_username,omitempty"`
	AuthPassword           *string             `json:"auth_password,omitempty"`
	AuthToken              *string             `json:"auth_token,omitempty"`
	AuthHeaders            []map[string]string `json:"auth_headers,omitempty"`
	AuthQueryParamKey      *string             `json:"auth_query_param_key,omitempty"`
	AuthQueryParamValue    *string             `json:"auth_query_param_value,omitempty"`
	OAuthConfig            map[string]any      `json:"oauth_config,omitempty"`
}

func (c *Client) GetGateway(ctx context.Context, idOrNameOrSlug string) (*Gateway, error) {
	var out Gateway
	if err := c.doJSON(ctx, http.MethodGet, "/v1/gateways/"+url.PathEscape(idOrNameOrSlug), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) DeleteGateway(ctx context.Context, id string) error {
	if err := c.doJSON(ctx, http.MethodDelete, "/v1/gateways/"+url.PathEscape(id), nil, nil); err != nil && !IsNotFound(err) {
		return err
	}
	return nil
}

func (c *Client) UpdateGateway(ctx context.Context, id string, in GatewayUpdate) (*Gateway, error) {
	var out Gateway
	if err := c.doJSON(ctx, http.MethodPut, "/v1/gateways/"+url.PathEscape(id), in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) CreateGateway(ctx context.Context, in GatewayCreate) (*Gateway, error) {
	var out Gateway
	if err := c.doJSON(ctx, http.MethodPost, "/v1/gateways/", in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
