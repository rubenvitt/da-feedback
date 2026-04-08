package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type OIDCConfig struct {
	Issuer       string
	ClientID     string
	ClientSecret string
	RedirectURL  string
}

type OIDCClient struct {
	config      OIDCConfig
	authURL     string
	tokenURL    string
	userInfoURL string
	adminGroup  string
	glGroup     string
}

type oidcDiscovery struct {
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	UserinfoEndpoint      string `json:"userinfo_endpoint"`
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	IDToken     string `json:"id_token"`
}

type userInfoResponse struct {
	Sub    string   `json:"sub"`
	Name   string   `json:"name"`
	Email  string   `json:"email"`
	Groups []string `json:"groups"`
}

func NewOIDCClient(config OIDCConfig) (*OIDCClient, error) {
	disc, err := discover(config.Issuer)
	if err != nil {
		return nil, err
	}
	return &OIDCClient{
		config:      config,
		authURL:     disc.AuthorizationEndpoint,
		tokenURL:    disc.TokenEndpoint,
		userInfoURL: disc.UserinfoEndpoint,
		adminGroup:  "da-feedback-admin",
		glGroup:     "da-feedback-gl",
	}, nil
}

func discover(issuer string) (*oidcDiscovery, error) {
	resp, err := http.Get(issuer + "/.well-known/openid-configuration")
	if err != nil {
		return nil, fmt.Errorf("oidc discovery: %w", err)
	}
	defer resp.Body.Close()
	var disc oidcDiscovery
	if err := json.NewDecoder(resp.Body).Decode(&disc); err != nil {
		return nil, fmt.Errorf("decode discovery: %w", err)
	}
	return &disc, nil
}

func (c *OIDCClient) AuthURL(state string) string {
	v := url.Values{
		"client_id":     {c.config.ClientID},
		"redirect_uri":  {c.config.RedirectURL},
		"response_type": {"code"},
		"scope":         {"openid profile email groups"},
		"state":         {state},
	}
	return c.authURL + "?" + v.Encode()
}

func (c *OIDCClient) Exchange(ctx context.Context, code string) (*User, error) {
	data := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {c.config.RedirectURL},
		"client_id":     {c.config.ClientID},
		"client_secret": {c.config.ClientSecret},
	}

	resp, err := http.PostForm(c.tokenURL, data)
	if err != nil {
		return nil, fmt.Errorf("token exchange: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("token exchange failed (%d): %s", resp.StatusCode, body)
	}

	var token tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&token); err != nil {
		return nil, fmt.Errorf("decode token: %w", err)
	}

	return c.getUserInfo(ctx, token.AccessToken)
}

func (c *OIDCClient) getUserInfo(ctx context.Context, accessToken string) (*User, error) {
	req, _ := http.NewRequestWithContext(ctx, "GET", c.userInfoURL, nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("userinfo: %w", err)
	}
	defer resp.Body.Close()

	var info userInfoResponse
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("decode userinfo: %w", err)
	}

	role := c.resolveRole(info.Groups)
	if role == "" {
		return nil, fmt.Errorf("user %s has no authorized group", info.Sub)
	}

	return &User{
		ID:    info.Sub,
		Name:  info.Name,
		Email: info.Email,
		Role:  role,
	}, nil
}

func (c *OIDCClient) resolveRole(groups []string) string {
	for _, g := range groups {
		if strings.EqualFold(g, c.adminGroup) {
			return "admin"
		}
	}
	for _, g := range groups {
		if strings.EqualFold(g, c.glGroup) {
			return "groupleader"
		}
	}
	return ""
}
