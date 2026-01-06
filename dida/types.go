package dida

import "time"

// Task 滴答清单任务
type Task struct {
	ID           string      `json:"id"`
	ProjectID    string      `json:"projectId"`
	ParentID     string      `json:"parentId,omitempty"` // 父任务ID（如果是子任务）
	Title        string      `json:"title"`
	Content      string      `json:"content"`
	Priority     int         `json:"priority"` // 0:无, 1:低, 3:中, 5:高
	Status       int         `json:"status"`   // 0:未完成, 2:已完成
	DueDate      string      `json:"dueDate"`  // ISO8601 格式
	StartDate    string      `json:"startDate"`
	Reminders    []string    `json:"reminders"` // 提醒时间列表
	Tags         []string    `json:"tags"`
	TimeZone     string      `json:"timeZone"`
	IsAllDay     bool        `json:"isAllDay"`
	Items        []CheckItem `json:"items,omitempty"`    // 清单项/检查项
	ChildIDs     []string    `json:"childIds,omitempty"` // 子任务ID列表
	ModifiedTime time.Time   `json:"modifiedTime"`
	CreatedTime  time.Time   `json:"createdTime"`
}

// CheckItem 清单项/检查项（任务内的小项）
type CheckItem struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Status    int    `json:"status"` // 0:未完成, 1:已完成
	SortOrder int64  `json:"sortOrder"`
	StartDate string `json:"startDate,omitempty"`
	IsAllDay  bool   `json:"isAllDay,omitempty"`
}

// Project 滴答清单项目/清单
type Project struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Color      string `json:"color"`
	SortOrder  int64  `json:"sortOrder"`
	Closed     bool   `json:"closed"`
	GroupID    string `json:"groupId"`
	ViewMode   string `json:"viewMode"`
	Permission string `json:"permission"`
	Kind       string `json:"kind"`
}

// TokenResponse OAuth token 响应
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	Scope        string `json:"scope"`
	RefreshToken string `json:"refresh_token,omitempty"`
}

// UserInfo 用户信息
type UserInfo struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
}
