package dida

import (
	"context"
	"encoding/json"
	"fmt"
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
	if err := c.doRequest(ctx, "GET", "/project", &projects); err != nil {
		return nil, err
	}
	return projects, nil
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

// GetAllTasks 获取所有项目的所有任务（包括收件箱和缺失的子任务）
func (c *Client) GetAllTasks(ctx context.Context) ([]Task, error) {
	projects, err := c.GetProjects(ctx)
	if err != nil {
		return nil, err
	}

	var allTasks []Task

	// 先尝试获取收件箱的任务
	inboxTasks, err := c.GetProjectTasks(ctx, "inbox")
	if err == nil {
		allTasks = append(allTasks, inboxTasks...)
	}

	// 获取其他项目的任务
	for _, p := range projects {
		tasks, err := c.GetProjectTasks(ctx, p.ID)
		if err != nil {
			fmt.Printf("获取项目 %s 失败: %v\n", p.Name, err)
			continue
		}
		allTasks = append(allTasks, tasks...)
	}

	// 补充获取缺失的子任务
	allTasks, err = c.fetchMissingSubtasks(ctx, allTasks)
	if err != nil {
		fmt.Printf("获取缺失子任务失败: %v\n", err)
	}

	return allTasks, nil
}

// fetchMissingSubtasks 补充获取缺失的子任务
func (c *Client) fetchMissingSubtasks(ctx context.Context, tasks []Task) ([]Task, error) {
	// 构建已有任务的 ID 集合
	existingIDs := make(map[string]bool)
	for _, task := range tasks {
		existingIDs[task.ID] = true
	}

	// 收集所有缺失的子任务 ID 及其父任务信息
	type missingInfo struct {
		childID   string
		projectID string
	}
	var missingTasks []missingInfo

	for _, task := range tasks {
		for _, childID := range task.ChildIDs {
			if !existingIDs[childID] {
				missingTasks = append(missingTasks, missingInfo{
					childID:   childID,
					projectID: task.ProjectID,
				})
			}
		}
	}

	if len(missingTasks) == 0 {
		return tasks, nil
	}

	fmt.Printf("发现 %d 个缺失的子任务，正在补充获取...\n", len(missingTasks))

	// 逐个获取缺失的子任务
	for _, missing := range missingTasks {
		task, err := c.GetTask(ctx, missing.projectID, missing.childID)
		if err != nil {
			fmt.Printf("获取子任务 %s 失败: %v\n", missing.childID, err)
			continue
		}
		tasks = append(tasks, *task)
		existingIDs[task.ID] = true
	}

	return tasks, nil
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
