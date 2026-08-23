package env

import (
	"log/slog"
	"os"
	"strconv"
	"time"
)

func Get(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func GetInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		slog.Warn("invalid env int, using default", "key", key, "value", v, "default", def, "error", err)
		return def
	}
	return n
}

func GetInt64(key string, def int64) int64 {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		slog.Warn("invalid env int64, using default", "key", key, "value", v, "default", def, "error", err)
		return def
	}
	return n
}

func GetDuration(key string, def time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		slog.Warn("invalid env duration, using default", "key", key, "value", v, "default", def, "error", err)
		return def
	}
	return d
}
