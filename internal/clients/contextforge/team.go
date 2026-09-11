package contextforge

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

type teamListResponse struct {
	Teams []Team `json:"teams"`
	Total int    `json:"total"`
}

type Team struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Slug        string         `json:"slug"`
	CreatedBy   string         `json:"created_by"`
	IsPersonal  bool           `json:"is_personal"`
	Visibility  TeamVisibility `json:"visibility"`
	MaxMembers  *int           `json:"max_members"`
	MemberCount int            `json:"member_count"`
	CreatedAt   string         `json:"created_at"`
	UpdatedAt   string         `json:"updated_at"`
	IsActive    bool           `json:"is_active"`
}

type TeamCreate struct {
	Name        string         `json:"name"`
	Description *string        `json:"description,omitempty"`
	Visibility  TeamVisibility `json:"visibility"`
	MaxMembers  *int           `json:"max_members,omitempty"`
}

type TeamUpdate struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
	MaxMembers  *int    `json:"max_members,omitempty"`
}

type TeamVisibility string

const (
	TeamVisibilityPrivate TeamVisibility = "private"
)

var ErrTeamNotFound = fmt.Errorf("team not found")

func (c *Client) ResolveTeamBySlug(ctx context.Context, slug string) (string, error) {
	var resp teamListResponse
	if err := c.doJSON(ctx, http.MethodGet, "/v1/teams/?limit=500", nil, &resp); err != nil {
		return "", err
	}

	// if resp.Total > len(resp.Teams) {
	// 	// TODO: implement pagination if we ever have more than 500 teams
	// }

	for _, t := range resp.Teams {
		if t.Slug == slug {
			return t.ID, nil
		}
	}

	return "", ErrTeamNotFound
}

func (c *Client) GetTeam(ctx context.Context, id string) (*Team, error) {
	var out Team
	if err := c.doJSON(ctx, http.MethodGet, "/v1/teams/"+url.PathEscape(id), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) CreateTeam(ctx context.Context, t TeamCreate) (*Team, error) {
	var out Team
	if err := c.doJSON(ctx, http.MethodPost, "/v1/teams/", t, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) UpdateTeam(ctx context.Context, id string, t TeamUpdate) (*Team, error) {
	var out Team
	if err := c.doJSON(ctx, http.MethodPut, "/v1/teams/"+url.PathEscape(id), t, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) DeleteTeam(ctx context.Context, id string) error {
	if err := c.doJSON(ctx, http.MethodDelete, "/v1/teams/"+url.PathEscape(id), nil, nil); err != nil && !IsNotFound(err) {
		return err
	}
	return nil
}
