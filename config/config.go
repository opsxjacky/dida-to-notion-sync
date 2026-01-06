package config

import (
	"bufio"
	"os"
	"strings"
)

type Config struct {
	// 滴答清单
	DidaClientID     string
	DidaClientSecret string
	DidaRedirectURL  string

	// Notion
	NotionToken      string
	NotionDatabaseID string
}

func Load() (*Config, error) {
	// 加载 .env 文件
	if err := loadEnvFile(".env"); err != nil {
		// .env 文件不存在不是错误
	}

	return &Config{
		DidaClientID:     os.Getenv("DIDA_CLIENT_ID"),
		DidaClientSecret: os.Getenv("DIDA_CLIENT_SECRET"),
		DidaRedirectURL:  os.Getenv("DIDA_REDIRECT_URL"),
		NotionToken:      os.Getenv("NOTION_TOKEN"),
		NotionDatabaseID: os.Getenv("NOTION_DATABASE_ID"),
	}, nil
}

func loadEnvFile(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			os.Setenv(key, value)
		}
	}
	return scanner.Err()
}
