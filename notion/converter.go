package notion

import (
	"dida-to-notion-sync/dida"
)

// TaskToProperties 将滴答清单任务转换为 Notion 属性
// 根据你的数据库结构：名称(title), 状态(status), 日期(date), 项目(select), 标签(select), 描述(rich_text), 滴答ID(rich_text)
func TaskToProperties(task dida.Task, projectName string, parentTaskTitle string) map[string]interface{} {
	props := map[string]interface{}{
		// 名称 (Title)
		"名称": map[string]interface{}{
			"title": []map[string]interface{}{
				{
					"text": map[string]interface{}{
						"content": task.Title,
					},
				},
			},
		},
		// 滴答ID (rich_text) - 用于去重
		"滴答ID": map[string]interface{}{
			"rich_text": []map[string]interface{}{
				{
					"text": map[string]interface{}{
						"content": task.ID,
					},
				},
			},
		},
		// 状态 (status) - Notion 的 status 类型需要用 name
		"状态": map[string]interface{}{
			"status": map[string]interface{}{
				"name": statusToName(task.Status),
			},
		},
		// 项目 (Select)
		"项目": map[string]interface{}{
			"select": map[string]interface{}{
				"name": projectName,
			},
		},
	}

	// 日期 (Date) - 截止日期
	if task.DueDate != "" {
		props["日期"] = map[string]interface{}{
			"date": map[string]interface{}{
				"start": formatDate(task.DueDate),
			},
		}
	}

	// 标签 (Select) - 使用优先级作为标签
	props["标签"] = map[string]interface{}{
		"select": map[string]interface{}{
			"name": priorityToLabel(task.Priority),
		},
	}

	// 描述 (rich_text)
	if task.Content != "" {
		props["描述"] = map[string]interface{}{
			"rich_text": []map[string]interface{}{
				{
					"text": map[string]interface{}{
						"content": truncateString(task.Content, 2000),
					},
				},
			},
		}
	}

	return props
}

// statusToName 状态转换
func statusToName(status int) string {
	if status == 2 {
		return "完成"
	}
	return "未开始"
}

// priorityToLabel 优先级转换为标签
func priorityToLabel(priority int) string {
	switch priority {
	case 5:
		return "高优先级"
	case 3:
		return "中优先级"
	case 1:
		return "低优先级"
	default:
		return "无优先级"
	}
}

// formatDate 格式化日期 (滴答清单格式 -> Notion 格式)
func formatDate(dateStr string) string {
	// 滴答清单日期格式: 2026-01-06T00:00:00.000+0000
	// Notion 日期格式: 2026-01-06 或 2026-01-06T00:00:00
	if len(dateStr) >= 10 {
		return dateStr[:10]
	}
	return dateStr
}

// truncateString 截断字符串
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}
