package wechat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// Client calls the WeChat Mini Program jscode2session API.
type Client struct {
	appID     string
	appSecret string
	http      *http.Client
}

// SessionResult is the parsed successful response from jscode2session.
type SessionResult struct {
	OpenID  string
	UnionID string
}

// NewClient creates a WeChat API client. If appID or appSecret is empty,
// CodeToSession will return ErrNotConfigured.
func NewClient(appID, appSecret string) *Client {
	return &Client{
		appID:     appID,
		appSecret: appSecret,
		http: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// ErrNotConfigured is returned when WeChat credentials are missing.
var ErrNotConfigured = errors.New("wechat not configured")

// CodeToSession exchanges a wx.login code for an openid and optional unionid.
func (c *Client) CodeToSession(ctx context.Context, code string) (SessionResult, error) {
	if c.appID == "" || c.appSecret == "" {
		return SessionResult{}, ErrNotConfigured
	}

	u := url.URL{
		Scheme: "https",
		Host:   "api.weixin.qq.com",
		Path:   "/sns/jscode2session",
		RawQuery: url.Values{
			"appid":      {c.appID},
			"secret":     {c.appSecret},
			"js_code":    {code},
			"grant_type": {"authorization_code"},
		}.Encode(),
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return SessionResult{}, fmt.Errorf("wechat: build request: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return SessionResult{}, fmt.Errorf("wechat: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return SessionResult{}, fmt.Errorf("wechat: unexpected status %d", resp.StatusCode)
	}

	var body struct {
		OpenID  string `json:"openid"`
		UnionID string `json:"unionid"`
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return SessionResult{}, fmt.Errorf("wechat: decode response: %w", err)
	}

	if body.ErrCode != 0 {
		return SessionResult{}, fmt.Errorf("wechat: error %d", body.ErrCode)
	}
	if body.OpenID == "" {
		return SessionResult{}, errors.New("wechat: empty openid")
	}

	return SessionResult{OpenID: body.OpenID, UnionID: body.UnionID}, nil
}
