package notion

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const (
	baseURL       = "https://api.notion.com/v1"
	notionVersion = "2022-06-28"
)

// Client Notion API 客户端
type Client struct {
	token      string
	databaseID string
	httpClient *http.Client
}

// NewClient 创建新的 Notion 客户端
func NewClient(token, databaseID string) *Client {
	return &Client{
		token:      token,
		databaseID: databaseID,
		httpClient: http.DefaultClient,
	}
}

// doRequest 执行 API 请求
func (c *Client) doRequest(ctx context.Context, method, path string, body interface{}, result interface{}) error {
	var reqBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reqBody = bytes.NewReader(jsonBody)
	}

	req, err := http.NewRequestWithContext(ctx, method, baseURL+path, reqBody)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Notion-Version", notionVersion)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 400 {
		return fmt.Errorf("Notion API error: %s, body: %s", resp.Status, string(respBody))
	}

	if result != nil {
		return json.Unmarshal(respBody, result)
	}
	return nil
}

// Page Notion 页面
type Page struct {
	ID         string                 `json:"id"`
	Properties map[string]interface{} `json:"properties"`
}

// QueryResponse 查询响应
type QueryResponse struct {
	Results    []Page `json:"results"`
	HasMore    bool   `json:"has_more"`
	NextCursor string `json:"next_cursor,omitempty"`
}

// QueryDatabase 查询数据库
func (c *Client) QueryDatabase(ctx context.Context, filter map[string]interface{}) (*QueryResponse, error) {
	var result QueryResponse
	body := map[string]interface{}{}
	if filter != nil {
		body["filter"] = filter
	}

	path := fmt.Sprintf("/databases/%s/query", c.databaseID)
	if err := c.doRequest(ctx, "POST", path, body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// CreatePage 创建页面
func (c *Client) CreatePage(ctx context.Context, properties map[string]interface{}) (*Page, error) {
	body := map[string]interface{}{
		"parent": map[string]interface{}{
			"database_id": c.databaseID,
		},
		"properties": properties,
	}

	var result Page
	if err := c.doRequest(ctx, "POST", "/pages", body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// UpdatePage 更新页面
func (c *Client) UpdatePage(ctx context.Context, pageID string, properties map[string]interface{}) (*Page, error) {
	body := map[string]interface{}{
		"properties": properties,
	}

	var result Page
	path := fmt.Sprintf("/pages/%s", pageID)
	if err := c.doRequest(ctx, "PATCH", path, body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// FindPageByDidaID 通过滴答ID查找页面
func (c *Client) FindPageByDidaID(ctx context.Context, didaID string) (*Page, error) {
	filter := map[string]interface{}{
		"property": "滴答ID",
		"rich_text": map[string]interface{}{
			"equals": didaID,
		},
	}

	result, err := c.QueryDatabase(ctx, filter)
	if err != nil {
		return nil, err
	}

	if len(result.Results) > 0 {
		return &result.Results[0], nil
	}
	return nil, nil
}

// GetAllPages 获取数据库中所有页面
func (c *Client) GetAllPages(ctx context.Context) ([]Page, error) {
	var allPages []Page
	var cursor string

	for {
		body := map[string]interface{}{}
		if cursor != "" {
			body["start_cursor"] = cursor
		}

		var result QueryResponse
		path := fmt.Sprintf("/databases/%s/query", c.databaseID)
		if err := c.doRequest(ctx, "POST", path, body, &result); err != nil {
			return nil, err
		}

		allPages = append(allPages, result.Results...)

		if !result.HasMore {
			break
		}
		cursor = result.NextCursor
	}

	return allPages, nil
}
