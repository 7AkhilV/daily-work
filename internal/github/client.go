package github

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/go-github/v69/github"
	"golang.org/x/oauth2"
)

type Client struct {
	gh        *github.Client
	user      *github.User
	ownerType map[string]string // login -> "User" | "Organization"
}

func NewClient(ctx context.Context, token string) (*Client, error) {
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(ctx, ts)
	gh := github.NewClient(tc)

	user, _, err := gh.Users.Get(ctx, "")
	if err != nil {
		if isUnauthorized(err) {
			return nil, fmt.Errorf("GitHub authentication expired.\n\nRun:\n\n  daily-work auth")
		}
		return nil, fmt.Errorf("unable to fetch GitHub user: %w", err)
	}
	return &Client{gh: gh, user: user, ownerType: map[string]string{}}, nil
}

func (c *Client) Login() string {
	return c.user.GetLogin()
}

func (c *Client) Validate(ctx context.Context) (string, error) {
	user, _, err := c.gh.Users.Get(ctx, "")
	if err != nil {
		if isUnauthorized(err) {
			return "", fmt.Errorf("GitHub authentication expired.\n\nRun:\n\n  daily-work auth")
		}
		return "", err
	}
	return user.GetLogin(), nil
}

func (c *Client) allowRepo(ctx context.Context, owner, repoName string) bool {
	owner = strings.TrimSpace(owner)
	repoName = strings.TrimSpace(repoName)
	if owner == "" {
		return false
	}
	// Never include this CLI's own repo in work summaries.
	if strings.EqualFold(repoName, "daily-work") {
		return false
	}
	t := c.lookupOwnerType(ctx, owner)
	return strings.EqualFold(t, "Organization")
}

func (c *Client) lookupOwnerType(ctx context.Context, owner string) string {
	if c.ownerType == nil {
		c.ownerType = map[string]string{}
	}
	key := strings.ToLower(owner)
	if t, ok := c.ownerType[key]; ok {
		return t
	}
	u, _, err := c.gh.Users.Get(ctx, owner)
	if err != nil || u == nil {
		c.ownerType[key] = "Unknown"
		return "Unknown"
	}
	t := u.GetType()
	c.ownerType[key] = t
	return t
}

func (c *Client) userEmails(ctx context.Context) ([]string, error) {
	emails, _, err := c.gh.Users.ListEmails(ctx, &github.ListOptions{PerPage: 50})
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range emails {
		if e.GetEmail() != "" {
			out = append(out, e.GetEmail())
		}
	}
	return out, nil
}

func isUnauthorized(err error) bool {
	if err == nil {
		return false
	}
	if ghErr, ok := err.(*github.ErrorResponse); ok {
		return ghErr.Response != nil && ghErr.Response.StatusCode == http.StatusUnauthorized
	}
	return strings.Contains(err.Error(), "401")
}
