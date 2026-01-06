package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"time"

	"dida-to-notion-sync/config"
	"dida-to-notion-sync/dida"
	"dida-to-notion-sync/notion"
)

const tokenFile = ".token"

func main() {
	// 加载配置
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("加载配置失败: %v\n", err)
		os.Exit(1)
	}

	if cfg.DidaClientID == "" || cfg.DidaClientSecret == "" {
		fmt.Println("请在 .env 文件中配置 DIDA_CLIENT_ID 和 DIDA_CLIENT_SECRET")
		os.Exit(1)
	}

	// 创建 OAuth 客户端
	oauth := dida.NewOAuth(cfg.DidaClientID, cfg.DidaClientSecret, cfg.DidaRedirectURL)

	// 尝试加载已有的 token
	if err := oauth.LoadToken(tokenFile); err != nil {
		fmt.Println("未找到已保存的授权信息，需要重新授权...")
		if err := authorize(oauth); err != nil {
			fmt.Printf("授权失败: %v\n", err)
			os.Exit(1)
		}
	} else {
		fmt.Println("已加载保存的授权信息")
	}

	// 创建滴答清单 API 客户端
	didaClient := dida.NewClient(oauth)
	ctx := context.Background()

	// 获取项目列表（用于映射项目名称）
	fmt.Println("\n正在获取滴答清单项目...")
	projects, err := didaClient.GetProjects(ctx)
	if err != nil {
		fmt.Printf("获取项目失败: %v\n", err)
		os.Exit(1)
	}

	// 构建项目ID -> 名称映射
	projectMap := make(map[string]string)
	projectMap["inbox"] = "收集箱"
	for _, p := range projects {
		projectMap[p.ID] = p.Name
	}
	fmt.Printf("找到 %d 个项目\n", len(projects)+1) // +1 for inbox

	// 获取所有任务
	fmt.Println("正在获取滴答清单任务...")
	tasks, err := didaClient.GetAllTasks(ctx)
	if err != nil {
		fmt.Printf("获取任务失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("找到 %d 个任务\n", len(tasks))

	// 检查 Notion 配置
	if cfg.NotionToken == "" || cfg.NotionDatabaseID == "" {
		fmt.Println("\n未配置 Notion，跳过同步")
		fmt.Println("请在 .env 文件中配置 NOTION_TOKEN 和 NOTION_DATABASE_ID")
		return
	}

	// 创建 Notion 客户端
	notionClient := notion.NewClient(cfg.NotionToken, cfg.NotionDatabaseID)

	// 同步任务到 Notion
	fmt.Println("\n正在同步到 Notion...")
	syncResult := syncToNotion(ctx, notionClient, tasks, projectMap)

	// 标记已完成的任务
	fmt.Println("\n正在检查已完成的任务...")
	completedCount := markCompletedTasks(ctx, notionClient, didaClient, tasks)

	fmt.Printf("\n同步完成！\n")
	fmt.Printf("  新增: %d\n", syncResult.Created)
	fmt.Printf("  更新: %d\n", syncResult.Updated)
	fmt.Printf("  跳过: %d\n", syncResult.Skipped)
	fmt.Printf("  失败: %d\n", syncResult.Failed)
	fmt.Printf("  标记完成: %d\n", completedCount)
}

// SyncResult 同步结果
type SyncResult struct {
	Created int
	Updated int
	Skipped int
	Failed  int
}

// syncToNotion 同步任务到 Notion
func syncToNotion(ctx context.Context, client *notion.Client, tasks []dida.Task, projectMap map[string]string) SyncResult {
	result := SyncResult{}

	// 构建 滴答ID -> Notion PageID 的映射（用于关联父子任务）
	didaToNotionID := make(map[string]string)

	// 第一轮：创建/更新所有任务，收集 ID 映射
	fmt.Println("第一轮：同步任务...")
	for i, task := range tasks {
		// 获取项目名称
		projectName := projectMap[task.ProjectID]
		if projectName == "" {
			projectName = "收集箱"
		}

		// 检查任务是否已存在
		existingPage, err := client.FindPageByDidaID(ctx, task.ID)
		if err != nil {
			fmt.Printf("  [%d/%d] 查询失败: %s - %v\n", i+1, len(tasks), task.Title, err)
			result.Failed++
			continue
		}

		// 转换为 Notion 属性（不包含父任务关联）
		props := notion.TaskToProperties(task, projectName, "")

		if existingPage != nil {
			// 更新现有页面
			_, err := client.UpdatePage(ctx, existingPage.ID, props)
			if err != nil {
				fmt.Printf("  [%d/%d] 更新失败: %s - %v\n", i+1, len(tasks), task.Title, err)
				result.Failed++
			} else {
				fmt.Printf("  [%d/%d] 已更新: %s\n", i+1, len(tasks), task.Title)
				result.Updated++
				didaToNotionID[task.ID] = existingPage.ID
			}
		} else {
			// 创建新页面
			newPage, err := client.CreatePage(ctx, props)
			if err != nil {
				fmt.Printf("  [%d/%d] 创建失败: %s - %v\n", i+1, len(tasks), task.Title, err)
				result.Failed++
			} else {
				fmt.Printf("  [%d/%d] 已创建: %s\n", i+1, len(tasks), task.Title)
				result.Created++
				didaToNotionID[task.ID] = newPage.ID
			}
		}

		// 避免 API 限流
		time.Sleep(350 * time.Millisecond)
	}

	// 第二轮：更新父子任务关联
	fmt.Println("\n第二轮：关联父子任务...")
	relationUpdated := 0

	// 构建父任务 -> 子任务列表的映射
	parentToChildren := make(map[string][]string)
	for _, task := range tasks {
		if task.ParentID != "" {
			childNotionID, ok := didaToNotionID[task.ID]
			if ok {
				parentToChildren[task.ParentID] = append(parentToChildren[task.ParentID], childNotionID)
			}
		}
	}

	for _, task := range tasks {
		if task.ParentID == "" {
			continue // 没有父任务，跳过
		}

		// 获取当前任务的 Notion ID
		notionID, ok := didaToNotionID[task.ID]
		if !ok {
			continue
		}

		// 获取父任务的 Notion ID
		parentNotionID, ok := didaToNotionID[task.ParentID]
		if !ok {
			continue
		}

		// 更新子任务的父任务关联
		props := map[string]interface{}{
			"父任务": map[string]interface{}{
				"relation": []map[string]interface{}{
					{"id": parentNotionID},
				},
			},
		}

		_, err := client.UpdatePage(ctx, notionID, props)
		if err != nil {
			fmt.Printf("  关联失败: %s -> 父任务 - %v\n", task.Title, err)
		} else {
			fmt.Printf("  已关联: %s -> 父任务\n", task.Title)
			relationUpdated++
		}

		time.Sleep(350 * time.Millisecond)
	}

	// 更新父任务的子任务字段
	fmt.Println("\n第三轮：更新父任务的子任务列表...")
	for parentDidaID, childNotionIDs := range parentToChildren {
		parentNotionID, ok := didaToNotionID[parentDidaID]
		if !ok {
			continue
		}

		// 构建子任务关联列表
		childRelations := make([]map[string]interface{}, len(childNotionIDs))
		for i, childID := range childNotionIDs {
			childRelations[i] = map[string]interface{}{"id": childID}
		}

		props := map[string]interface{}{
			"子任务": map[string]interface{}{
				"relation": childRelations,
			},
		}

		_, err := client.UpdatePage(ctx, parentNotionID, props)
		if err != nil {
			fmt.Printf("  更新子任务列表失败: %v\n", err)
		} else {
			fmt.Printf("  已更新子任务列表 (%d 个子任务)\n", len(childNotionIDs))
		}

		time.Sleep(350 * time.Millisecond)
	}

	fmt.Printf("  父子关联更新: %d\n", relationUpdated)

	return result
}

func authorize(oauth *dida.OAuth) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// 获取授权 URL
	authURL := oauth.GetAuthURL("state")
	fmt.Println("\n请在浏览器中打开以下链接进行授权：")
	fmt.Println(authURL)
	fmt.Println()

	// 尝试自动打开浏览器
	openBrowser(authURL)

	fmt.Println("等待授权回调...")

	// 启动回调服务器
	code, err := oauth.StartCallbackServer(ctx)
	if err != nil {
		return fmt.Errorf("获取授权码失败: %w", err)
	}

	fmt.Println("收到授权码，正在获取 token...")

	// 换取 token
	_, err = oauth.ExchangeToken(ctx, code)
	if err != nil {
		return fmt.Errorf("获取 token 失败: %w", err)
	}

	// 保存 token
	if err := oauth.SaveToken(tokenFile); err != nil {
		fmt.Printf("警告: 保存 token 失败: %v\n", err)
	} else {
		fmt.Println("Token 已保存")
	}

	return nil
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		return
	}
	cmd.Start()
}

// extractDidaIDFromPage 从 Notion 页面中提取 TickTick ID
func extractDidaIDFromPage(page notion.Page) (string, bool) {
	if didaIDProp, exists := page.Properties["滴答ID"]; exists {
		if richText, ok := didaIDProp.(map[string]interface{}); ok {
			if texts, ok := richText["rich_text"].([]interface{}); ok && len(texts) > 0 {
				if textObj, ok := texts[0].(map[string]interface{}); ok {
					if textContent, ok := textObj["text"].(map[string]interface{}); ok {
						if content, ok := textContent["content"].(string); ok {
							return content, true
						}
					}
				}
			}
		}
	}
	return "", false
}

// extractStatusFromPage 从 Notion 页面中提取状态
func extractStatusFromPage(page notion.Page) (string, bool) {
	if statusProp, exists := page.Properties["状态"]; exists {
		if statusObj, ok := statusProp.(map[string]interface{}); ok {
			if status, ok := statusObj["status"].(map[string]interface{}); ok {
				if statusName, ok := status["name"].(string); ok {
					return statusName, true
				}
			}
		}
	}
	return "", false
}

// markCompletedTasks 标记已完成的任务
// 1. 获取 Notion 数据库中的所有页面
// 2. 与 TickTick 任务进行比较
// 3. 如果 Notion 显示任务已完成但 TickTick 中未完成，则更新 TickTick
func markCompletedTasks(ctx context.Context, notionClient *notion.Client, didaClient *dida.Client, tickTickTasks []dida.Task) int {
	// 获取 Notion 数据库中的所有页面
	notionPages, err := notionClient.GetAllPages(ctx)
	if err != nil {
		fmt.Printf("获取 Notion 页面失败: %v\n", err)
		return 0
	}

	// 创建 TickTick 任务 ID 映射
	tickTickTaskMap := make(map[string]dida.Task)
	for _, task := range tickTickTasks {
		tickTickTaskMap[task.ID] = task
	}

	// 创建 Notion 中任务的映射
	notionTaskMap := make(map[string]notion.Page)
	for _, page := range notionPages {
		if didaID, exists := extractDidaIDFromPage(page); exists {
			notionTaskMap[didaID] = page
		}
	}

	completedCount := 0

	// 检查 Notion 状态是否需要同步到 TickTick
	for notionTaskID, notionPage := range notionTaskMap {
		tickTickTask, exists := tickTickTaskMap[notionTaskID]
		if !exists {
			// 任务在 Notion 中存在但在 TickTick 中不存在
			// 可能是因为它已经在 TickTick 中被删除或完成
			continue
		}

		notionStatus, exists := extractStatusFromPage(notionPage)
		if !exists {
			continue
		}

		// 同步 Notion 完成状态到 TickTick
		tickTickCompleted := tickTickTask.Status == 2
		notionCompleted := notionStatus == "完成"

		if notionCompleted && !tickTickCompleted {
			err := didaClient.UpdateTaskStatus(ctx, tickTickTask.ProjectID, tickTickTask.ID, 2)
			if err != nil {
				fmt.Printf("更新 TickTick 任务状态失败: %s - %v\n", tickTickTask.Title, err)
			} else {
				fmt.Printf("已同步完成状态到 TickTick: %s\n", tickTickTask.Title)
				completedCount++
			}
		} else if !notionCompleted && tickTickCompleted {
			// 如果 Notion 中是未完成状态而 TickTick 中已完成，则根据策略决定是否更新
			// 根据同步策略，可能需要将 TickTick 任务状态改回未完成
			// 这取决于同步方向策略
			fmt.Printf("Notion 与 TickTick 状态不一致: %s\n", tickTickTask.Title)
		}
	}

	return completedCount
}
