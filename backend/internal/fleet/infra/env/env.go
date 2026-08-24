package env

import "os"

func GetDatabaseURL() string {
	if v, ok := os.LookupEnv("DATABASE_URL"); ok && v != "" {
		return v
	}
	return ""
}

func GetNATSURL() string {
	if v, ok := os.LookupEnv("NATS_URL"); ok && v != "" {
		return v
	}
	return "nats://localhost:4222"
}

func GetAPIPort() string {
	if v := os.Getenv("API_PORT"); v != "" {
		return v
	}
	if v := os.Getenv("HTTP_PORT"); v != "" {
		return v
	}
	if v := os.Getenv("PORT"); v != "" {
		return v
	}
	return "8080"
}

func Get(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
