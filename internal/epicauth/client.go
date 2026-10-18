// Package epicauth implements the experimental EGS launch-auth protocol, not game management.
// Protocol reference: https://github.com/legendary-gl/legendary/blob/master/legendary/api/egs.py
package epicauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Public launcher application identifiers, not an end user's credentials.
const clientID = "34a02cf8f4414e29b15921876da36f9a"
const clientSecret = "daafbccc737745039dffe53d94fc76cf"
const accountURL = "https://account-public-service-prod03.ol.epicgames.com"
const commerceURL = "https://ecommerceintegration-public-service-ecomprod02.ol.epicgames.com"
const maxResponse = 2 << 20

type Session struct {
	AccessToken      string    `json:"access_token"`
	RefreshToken     string    `json:"refresh_token"`
	AccountID        string    `json:"account_id"`
	DisplayName      string    `json:"displayName"`
	ExpiresAt        time.Time `json:"expires_at"`
	RefreshExpiresAt time.Time `json:"refresh_expires_at"`
}

type Client struct {
	http              *http.Client
	account, commerce string
}

func New(transport http.RoundTripper) *Client {
	return &Client{http: &http.Client{Transport: transport, Timeout: 30 * time.Second,
		// No redirects, even same-host: a request may contain single-use credentials.
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }},
		account: accountURL, commerce: commerceURL}
}

func LoginURL() string {
	redirect := "https://www.epicgames.com/id/api/redirect?clientId=" + clientID + "&responseType=code"
	return "https://www.epicgames.com/id/login?redirectUrl=" + url.QueryEscape(redirect)
}

func ParseCode(input string) (string, error) {
	s := strings.TrimSpace(input)
	if strings.HasPrefix(s, "{") {
		var v struct {
			Code string `json:"authorizationCode"`
		}
		if json.Unmarshal([]byte(s), &v) != nil {
			return "", fmt.Errorf("授权响应格式不正确")
		}
		s = v.Code
	} else {
		s = strings.Trim(s, `"`)
	}
	if len(s) < 8 || len(s) > 4096 || strings.ContainsAny(s, " \t\r\n\x00") {
		return "", fmt.Errorf("请输入 authorizationCode 或完整 JSON 响应")
	}
	return s, nil
}

// request deliberately discards remote messages and transport error strings: they can contain secrets.
func (c *Client) request(ctx context.Context, stage, method, endpoint, bearer string, form url.Values, basic bool) ([]byte, error) {
	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return nil, fmt.Errorf("%s：请求构造失败", stage)
	}
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	req.Header.Set("User-Agent", "UELauncher/11.0.1-14907503+++Portal+Release-Live Windows/10.0.19041.1.256.64bit")
	if basic {
		req.SetBasicAuth(clientID, clientSecret)
	} else {
		req.Header.Set("Authorization", "bearer "+bearer)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("%s：网络请求失败（请检查代理及网络）", stage)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, maxResponse+1))
	if err != nil || len(b) > maxResponse {
		return nil, fmt.Errorf("%s：响应读取失败或过大", stage)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if resp.StatusCode == 400 || resp.StatusCode == 401 || resp.StatusCode == 403 {
			return nil, fmt.Errorf("%s：HTTP %d；请在客户端设置中重新登录（或运行 --auth-login），并确认账号及游戏权限", stage, resp.StatusCode)
		}
		return nil, fmt.Errorf("%s：HTTP %d", stage, resp.StatusCode)
	}
	return b, nil
}

func (c *Client) token(ctx context.Context, form url.Values) (Session, error) {
	form.Set("token_type", "eg1")
	b, err := c.request(ctx, "登录/刷新", http.MethodPost, c.account+"/account/api/oauth/token", "", form, true)
	if err != nil {
		return Session{}, err
	}
	var s Session
	if json.Unmarshal(b, &s) != nil || s.AccessToken == "" || s.RefreshToken == "" || !validID(s.AccountID) || s.ExpiresAt.Before(time.Now()) || s.RefreshExpiresAt.IsZero() {
		return Session{}, fmt.Errorf("认证响应缺少有效会话信息")
	}
	return s, nil
}

func validID(s string) bool {
	if s == "" || len(s) > 256 {
		return false
	}
	for _, r := range s {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_') {
			return false
		}
	}
	return true
}

func (c *Client) Login(ctx context.Context, code string) (Session, error) {
	return c.token(ctx, url.Values{"grant_type": {"authorization_code"}, "code": {code}})
}
func (c *Client) Refresh(ctx context.Context, s Session) (Session, error) {
	if s.RefreshToken == "" || time.Now().After(s.RefreshExpiresAt) {
		return Session{}, fmt.Errorf("桥接登录已过期，请运行 --auth-login")
	}
	next, err := c.token(ctx, url.Values{"grant_type": {"refresh_token"}, "refresh_token": {s.RefreshToken}})
	if err == nil && next.AccountID != s.AccountID {
		return Session{}, fmt.Errorf("刷新账号与原会话不一致，停止启动")
	}
	return next, err
}
func (c *Client) Exchange(ctx context.Context, s Session) (string, error) {
	b, err := c.request(ctx, "游戏交换码", http.MethodGet, c.account+"/account/api/oauth/exchange", s.AccessToken, nil, false)
	if err != nil {
		return "", err
	}
	var v struct {
		Code string `json:"code"`
	}
	if json.Unmarshal(b, &v) != nil || v.Code == "" || strings.ContainsAny(v.Code, "\x00\r\n") {
		return "", fmt.Errorf("游戏交换码响应无效")
	}
	return v.Code, nil
}
func (c *Client) Ownership(ctx context.Context, s Session, namespace, catalog string) ([]byte, error) {
	if !validID(s.AccountID) || !validID(namespace) || !validID(catalog) {
		return nil, fmt.Errorf("无效的账号或游戏标识")
	}
	b, err := c.request(ctx, "所有权令牌", http.MethodPost, c.commerce+"/ecommerceintegration/api/public/platforms/EPIC/identities/"+s.AccountID+"/ownershipToken", s.AccessToken, url.Values{"nsCatalogItemId": {namespace + ":" + catalog}}, false)
	if err == nil && len(b) == 0 {
		err = fmt.Errorf("所有权令牌为空")
	}
	return b, err
}
