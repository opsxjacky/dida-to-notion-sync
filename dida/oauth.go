package dida

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
)

const (
	authURL  = "https://dida365.com/oauth/authorize"
	tokenURL = "https://dida365.com/oauth/token"
)

type OAuth struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
	token        *TokenResponse
}

func NewOAuth(clientID, clientSecret, redirectURL string) *OAuth {
	return &OAuth{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
	}
}

// GetAuthURL 获取授权 URL，用户需要在浏览器中打开此 URL 进行授权
func (o *OAuth) GetAuthURL(state string) string {
	params := url.Values{}
	params.Set("client_id", o.ClientID)
	params.Set("redirect_uri", o.RedirectURL)
	params.Set("response_type", "code")
	params.Set("scope", "tasks:read tasks:write")
	params.Set("state", state)
	return fmt.Sprintf("%s?%s", authURL, params.Encode())
}

// ExchangeToken 用授权码换取 access token
func (o *OAuth) ExchangeToken(ctx context.Context, code string) (*TokenResponse, error) {
	data := url.Values{}
	data.Set("code", code)
	data.Set("grant_type", "authorization_code")
	data.Set("redirect_uri", o.RedirectURL)
	data.Set("scope", "tasks:read tasks:write")

	req, err := http.NewRequestWithContext(ctx, "POST", tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// 使用 Basic Auth 传递 client credentials
	auth := base64.StdEncoding.EncodeToString([]byte(o.ClientID + ":" + o.ClientSecret))
	req.Header.Set("Authorization", "Basic "+auth)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return nil, fmt.Errorf("token exchange failed: %s, body: %s", resp.Status, string(body))
	}

	var token TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&token); err != nil {
		return nil, err
	}

	o.token = &token
	return &token, nil
}

// GetToken 获取当前 token
func (o *OAuth) GetToken() *TokenResponse {
	return o.token
}

// SetToken 设置 token（从缓存加载时使用）
func (o *OAuth) SetToken(token *TokenResponse) {
	o.token = token
}

// SaveToken 保存 token 到文件
func (o *OAuth) SaveToken(filename string) error {
	if o.token == nil {
		return fmt.Errorf("no token to save")
	}
	data, err := json.MarshalIndent(o.token, "", "  ")
	if err != nil {
		return err
	}
	return ioutil.WriteFile(filename, data, 0600)
}

// LoadToken 从文件加载 token
func (o *OAuth) LoadToken(filename string) error {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return err
	}
	var token TokenResponse
	if err := json.Unmarshal(data, &token); err != nil {
		return err
	}
	o.token = &token
	return nil
}

// StartCallbackServer 启动一个临时 HTTP 服务器来接收 OAuth 回调
func (o *OAuth) StartCallbackServer(ctx context.Context) (string, error) {
	codeChan := make(chan string, 1)
	errChan := make(chan error, 1)

	server := &http.Server{Addr: ":8080"}

	http.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code == "" {
			errChan <- fmt.Errorf("no code in callback")
			w.Write([]byte("授权失败：未收到授权码"))
			return
		}
		codeChan <- code
		w.Write([]byte("授权成功！你可以关闭此页面了。"))
	})

	go func() {
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	select {
	case code := <-codeChan:
		server.Shutdown(ctx)
		return code, nil
	case err := <-errChan:
		server.Shutdown(ctx)
		return "", err
	case <-ctx.Done():
		server.Shutdown(ctx)
		return "", ctx.Err()
	}
}
