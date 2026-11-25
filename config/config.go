package config

import (
	"bufio"
	"os"
	"strings"
)

type Config struct {
	Port               string
	JWTSecret          string
	DBPath             string
	GeminiAPIKey       string
	GeminiModel        string
	ExchangeRateAPIKey string
	BaseCurrency       string
	EnableAIServices   bool
}

func Load() *Config {
	loadDotEnv(".env")

	return &Config{
		Port:               getEnv("PORT", "8080"),
		JWTSecret:          getEnv("JWT_SECRET", "pulsematrix-super-secret-jwt-key-2026"),
		DBPath:             getEnv("DB_PATH", "./data/pulsematrix.db"),
		GeminiAPIKey:       getEnv("GEMINI_API_KEY", ""),
		GeminiModel:        getEnv("GEMINI_MODEL", "gemini-2.0-flash"),
		ExchangeRateAPIKey: getEnv("EXCHANGERATE_API_KEY", ""),
		BaseCurrency:       getEnv("BASE_CURRENCY", "USD"),
		EnableAIServices:   getEnv("ENABLE_AI_SERVICES", "true") == "true",
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// loadDotEnv reads KEY=VALUE pairs from a local .env file without overriding
// environment variables already set by the OS or cloud platform.
func loadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		idx := strings.IndexByte(line, '=')
		if idx <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])
		if len(val) >= 2 && ((val[0] == '"' && val[len(val)-1] == '"') || (val[0] == '\'' && val[len(val)-1] == '\'')) {
			val = val[1 : len(val)-1]
		}
		if key != "" && os.Getenv(key) == "" {
			_ = os.Setenv(key, val)
		}
	}
}
