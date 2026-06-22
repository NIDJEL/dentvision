package config

import "os"

type Config struct {
	Port         string
	DatabaseURL  string
	JWTSecret    string
	UploadsDir   string
	MLServiceURL string
}

func Load() Config {
	return Config{
		Port:         getEnv("PORT", "8080"),
		DatabaseURL:  os.Getenv("DATABASE_URL"),
		JWTSecret:    getEnv("JWT_SECRET", "dev_secret_change_me"),
		UploadsDir:   getEnv("UPLOADS_DIR", "/app/uploads"),
		MLServiceURL: getEnv("ML_SERVICE_URL", "http://ml-service:8000"),
	}
}

func getEnv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	return value
}
