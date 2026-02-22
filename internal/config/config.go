package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Port int

	DBDSN string

	JWTAccessSecret  string
	JWTRefreshSecret string
	AccessTTL        time.Duration
	RefreshTTL       time.Duration

	AIBaseURL    string
	AITimeout    time.Duration
	WorkersCount int

	DefaultModelVersion string
	DefaultThreshold    float64

	CORSOrigins []string
}

func Load() (Config, error) {
	_ = godotenv.Load()

	var c Config

	c.Port = mustInt("PORT", 8080)
	if c.Port < 1 || c.Port > 65535 {
		return Config{}, fmt.Errorf("PORT must be between 1 and 65535")
	}

	c.DBDSN = strings.TrimSpace(os.Getenv("DB_DSN"))
	if c.DBDSN == "" {
		// Backward compatible alias (some setups still use DB_URL).
		c.DBDSN = strings.TrimSpace(os.Getenv("DB_URL"))
	}
	if c.DBDSN == "" {
		return Config{}, fmt.Errorf("DB_DSN env variable not set")
	}

	c.JWTAccessSecret = strings.TrimSpace(os.Getenv("JWT_ACCESS_SECRET"))
	if c.JWTAccessSecret == "" {
		return Config{}, fmt.Errorf("JWT_ACCESS_SECRET env variable not set")
	}

	c.JWTRefreshSecret = strings.TrimSpace(os.Getenv("JWT_REFRESH_SECRET"))
	if c.JWTRefreshSecret == "" {
		return Config{}, fmt.Errorf("JWT_REFRESH_SECRET env variable not set")
	}

	accessTTLMin := mustInt("ACCESS_TTL_MIN", 15)
	if accessTTLMin <= 0 {
		return Config{}, fmt.Errorf("ACCESS_TTL_MIN must be > 0")
	}
	c.AccessTTL = time.Duration(accessTTLMin) * time.Minute

	refreshTTLDays := mustInt("REFRESH_TTL_DAYS", 14)
	if refreshTTLDays <= 0 {
		return Config{}, fmt.Errorf("REFRESH_TTL_DAYS must be > 0")
	}
	c.RefreshTTL = time.Duration(refreshTTLDays) * 24 * time.Hour

	c.AIBaseURL = strings.TrimSpace(os.Getenv("AI_BASE_URL"))
	if c.AIBaseURL == "" {
		c.AIBaseURL = "http://localhost:8000"
	}

	aiTimeoutMS := mustInt("AI_TIMEOUT_MS", 5000)
	if aiTimeoutMS <= 0 {
		return Config{}, fmt.Errorf("AI_TIMEOUT_MS must be > 0")
	}
	c.AITimeout = time.Duration(aiTimeoutMS) * time.Millisecond

	c.DefaultModelVersion = strings.TrimSpace(os.Getenv("DEFAULT_MODEL_VERSION"))
	if c.DefaultModelVersion == "" {
		c.DefaultModelVersion = "baseline"
	}

	c.DefaultThreshold = mustFloat("DEFAULT_THRESHOLD", 0.5)

	c.WorkersCount = mustInt("WORKERS_COUNT", 2)
	if c.WorkersCount <= 0 {
		return Config{}, fmt.Errorf("WORKERS_COUNT must be > 0")
	}

	c.CORSOrigins = splitCSV(os.Getenv("CORS_ORIGINS"))

	return c, nil
}

func mustInt(key string, def int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func mustFloat(key string, def float64) float64 {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return def
	}
	return f
}

func splitCSV(v string) []string {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}

	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		out = append(out, p)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
