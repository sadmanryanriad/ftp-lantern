package config

import (
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Port                     string
	SourceURLs               []string
	BaseCrawlIntervalMinutes int
	CrawlConcurrency         int
	DataDir                  string
	ThumbsDir                string
	MaxThumbsDiskMB          int
	PublicBaseURL            string
	TelegramBotToken         string
	TelegramAdminChatID      int64
	TelegramEnableListener   bool
	LogLevel                 string
}

func Load() *Config {
	port := getEnv("PORT", "8087")
	dataDir := getEnv("DATA_DIR", "/data")
	thumbsDir := getEnv("THUMBS_DIR", dataDir+"/thumbs")
	publicBaseURL := getEnv("PUBLIC_BASE_URL", "http://localhost:"+port)
	if port != "8085" && strings.Contains(publicBaseURL, ":8085") {
		publicBaseURL = strings.Replace(publicBaseURL, ":8085", ":"+port, 1)
	}
	publicBaseURL = strings.TrimRight(publicBaseURL, "/")

	maxThumbsMB, _ := strconv.Atoi(getEnv("MAX_THUMBS_MB", "250"))
	if maxThumbsMB <= 0 {
		maxThumbsMB = 250
	}

	rawSourceURLs := getEnv("SOURCE_URLS", "http://172.16.50.12/DHAKA-FLIX-12/TV-WEB-Series/")
	var sourceURLs []string
	for _, u := range strings.Split(rawSourceURLs, ",") {
		u = strings.TrimSpace(u)
		if u != "" {
			sourceURLs = append(sourceURLs, u)
		}
	}

	intervalMin, _ := strconv.Atoi(getEnv("BASE_CRAWL_INTERVAL_MINUTES", "60"))
	if intervalMin <= 0 {
		intervalMin = 60
	}

	concurrency, _ := strconv.Atoi(getEnv("CRAWL_CONCURRENCY", "4"))
	if concurrency <= 0 {
		concurrency = 4
	}

	adminChatID, _ := strconv.ParseInt(getEnv("TELEGRAM_ADMIN_CHAT_ID", "0"), 10, 64)

	return &Config{
		Port:                     port,
		SourceURLs:               sourceURLs,
		BaseCrawlIntervalMinutes: intervalMin,
		CrawlConcurrency:         concurrency,
		DataDir:                  dataDir,
		ThumbsDir:                thumbsDir,
		MaxThumbsDiskMB:          maxThumbsMB,
		PublicBaseURL:            publicBaseURL,
		TelegramBotToken:         getEnv("TELEGRAM_BOT_TOKEN", ""),
		TelegramAdminChatID:      adminChatID,
		TelegramEnableListener:   getEnv("TELEGRAM_ENABLE_LISTENER", "false") == "true",
		LogLevel:                 getEnv("LOG_LEVEL", "info"),
	}
}

func getEnv(key, defaultVal string) string {
	val := os.Getenv(key)
	if val == "" {
		return defaultVal
	}
	return val
}
