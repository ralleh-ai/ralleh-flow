package config

import (
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	APIAddr                     string
	DataDir                     string
	DBPath                      string
	RepoRoot                    string
	AllowedOrigins              string
	RedisAddr                   string
	RedisStream                 string
	RedisConsumerGrp            string
	OpenClawHookURL             string
	OpenClawHookToken           string
	OpenClawSessionKeyPrefix    string
	OpenClawDefaultAgentID      string
	OpenClawModel               string
	OpenClawThinking            string
	OpenClawHTTPTimeoutSeconds  int
	OpenClawAgentTimeoutSeconds int
}

func Load() Config {
	_ = godotenv.Load()

	return Config{
		APIAddr:                     getEnv("RALLEH_FLOW_API_ADDR", ":4310"),
		DataDir:                     getEnv("RALLEH_FLOW_DATA_DIR", "./data"),
		DBPath:                      getEnv("RALLEH_FLOW_DB_PATH", "./data/ralleh-flow.db"),
		RepoRoot:                    getEnv("RALLEH_FLOW_REPO_ROOT", ""),
		AllowedOrigins:              getEnv("RALLEH_FLOW_ALLOWED_ORIGINS", "http://localhost:4300,http://127.0.0.1:4300"),
		RedisAddr:                   getEnv("RALLEH_FLOW_REDIS_ADDR", ""),
		RedisStream:                 getEnv("RALLEH_FLOW_REDIS_STREAM", "ralleh-flow:runs"),
		RedisConsumerGrp:            getEnv("RALLEH_FLOW_REDIS_CONSUMER_GROUP", "ralleh-flow-orchestrators"),
		OpenClawHookURL:             getEnv("RALLEH_FLOW_OPENCLAW_HOOK_URL", ""),
		OpenClawHookToken:           getEnv("RALLEH_FLOW_OPENCLAW_HOOK_TOKEN", ""),
		OpenClawSessionKeyPrefix:    getEnv("RALLEH_FLOW_OPENCLAW_SESSION_KEY_PREFIX", "hook:ralleh-flow"),
		OpenClawDefaultAgentID:      getEnv("RALLEH_FLOW_OPENCLAW_DEFAULT_AGENT_ID", ""),
		OpenClawModel:               getEnv("RALLEH_FLOW_OPENCLAW_MODEL", ""),
		OpenClawThinking:            getEnv("RALLEH_FLOW_OPENCLAW_THINKING", ""),
		OpenClawHTTPTimeoutSeconds:  getEnvInt("RALLEH_FLOW_OPENCLAW_HTTP_TIMEOUT_SECONDS", 15),
		OpenClawAgentTimeoutSeconds: getEnvInt("RALLEH_FLOW_OPENCLAW_AGENT_TIMEOUT_SECONDS", 0),
	}
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}
