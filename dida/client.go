package dida

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

const (
	baseURL = "https://api.dida365.com/open/v1"
)

// Client 滴答清单 API 客户端
type Client struct {
	oauth      *OAuth
	httpClient *http.Client
}

// NewClient 创建新的 API 客户端
func NewClient(oauth *OAuth) *Client {
	return &Client{
		oauth:      oauth,
		httpClient: http.DefaultClient,
	}
}

// doRequest 执行 API 请求
func (c *Client) doRequest(ctx context.Context, method, path string, result interface{}) error {
	token := c.oauth.GetToken()
	if token == nil {
		return fmt.Errorf("not authenticated")
	}

	req, err := http.NewRequestWithContext(ctx, method, baseURL+path, nil)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API error: %s", resp.Status)
	}

	if result != nil {
		return json.NewDecoder(resp.Body).Decode(result)
	}
	return nil
}

// GetProjects 获取所有项目/清单
func (c *Client) GetProjects(ctx context.Context) ([]Project, error) {
	var projects []Project
	if err := c.doRequestDebug(ctx, "GET", "/project", &projects); err != nil {
		return nil, err
	}
	return projects, nil
}

// doRequestDebug 带调试输出的请求
func (c *Client) doRequestDebug(ctx context.Context, method, path string, result interface{}) error {
	token := c.oauth.GetToken()
	if token == nil {
		return fmt.Errorf("not authenticated")
	}

	req, err := http.NewRequestWithContext(ctx, method, baseURL+path, nil)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, _ := ioutil.ReadAll(resp.Body)
	fmt.Printf("[DEBUG] API Response for %s:\n%s\n\n", path, string(body))

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API error: %s", resp.Status)
	}

	if result != nil {
		return json.Unmarshal(body, result)
	}
	return nil
}

// GetProjectTasks 获取指定项目的所有任务
func (c *Client) GetProjectTasks(ctx context.Context, projectID string) ([]Task, error) {
	var response struct {
		Tasks []Task `json:"tasks"`
	}
	path := fmt.Sprintf("/project/%s/data", projectID)
	if err := c.doRequest(ctx, "GET", path, &response); err != nil {
		return nil, err
	}
	return response.Tasks, nil
}

// GetAllTasks 获取所有项目的所有任务（包括收件箱）
func (c *Client) GetAllTasks(ctx context.Context) ([]Task, error) {
	projects, err := c.GetProjects(ctx)
	if err != nil {
		return nil, err
	}

	var allTasks []Task

	// 先尝试获取收件箱的任务
	inboxTasks, err := c.GetProjectTasks(ctx, "inbox")
	if err == nil {
		fmt.Printf("[DEBUG] 收件箱任务数: %d\n", len(inboxTasks))
		allTasks = append(allTasks, inboxTasks...)
	} else {
		fmt.Printf("[DEBUG] 获取收件箱失败: %v\n", err)
	}

	// 获取其他项目的任务
	for _, p := range projects {
		tasks, err := c.GetProjectTasks(ctx, p.ID)
		if err != nil {
			fmt.Printf("[DEBUG] 获取项目 %s 失败: %v\n", p.Name, err)
			continue
		}
		allTasks = append(allTasks, tasks...)
	}
	return allTasks, nil
}

// GetTask 获取单个任务
func (c *Client) GetTask(ctx context.Context, projectID, taskID string) (*Task, error) {
	var task Task
	path := fmt.Sprintf("/project/%s/task/%s", projectID, taskID)
	if err := c.doRequest(ctx, "GET", path, &task); err != nil {
		return nil, err
	}
	return &task, nil
}

// UpdateTaskStatus 更新任务状态
func (c *Client) UpdateTaskStatus(ctx context.Context, projectID, taskID string, status int) error {
	// 构建更新任务的请求体
	task := Task{
		ID:        taskID,
		Status:    status,
		ProjectID: projectID,
	}

	// 使用批量更新API
	updatePayload := map[string]interface{}{
		"add":    []interface{}{},
		"update": []Task{task},
		"delete": []interface{}{},
	}

	path := fmt.Sprintf("/project/%s/batch/task", projectID)
	return c.doRequest(ctx, "POST", path, &updatePayload)
}

// UpdateTask 更新任务详情
func (c *Client) UpdateTask(ctx context.Context, projectID string, task Task) error {
	// 使用批量更新API
	updatePayload := map[string]interface{}{
		"add":    []interface{}{},
		"update": []Task{task},
		"delete": []interface{}{},
	}

	path := fmt.Sprintf("/project/%s/batch/task", projectID)
	return c.doRequest(ctx, "POST", path, &updatePayload)
}
