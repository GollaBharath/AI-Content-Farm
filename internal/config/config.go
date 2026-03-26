package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	Port             string
	StorageDir       string
	TTSBaseURL       string
	TTSSynthPath     string
	TTSTimeoutSecs   int
	TrendsRegion     string
	FFmpegBinaryPath string
}

func Load() (Config, error) {
	cfg := Config{
		Port:             envOrDefault("PORT", "8080"),
		StorageDir:       envOrDefault("STORAGE_DIR", "./data"),
		TTSBaseURL:       envOrDefault("TTS_BASE_URL", "http://localhost:9000"),
		TTSSynthPath:     envOrDefault("TTS_SYNTH_PATH", "/api/tts"),
		TTSTimeoutSecs:   envIntOrDefault("TTS_TIMEOUT_SECONDS", 30),
		TrendsRegion:     envOrDefault("TRENDS_REGION", "US"),
		FFmpegBinaryPath: envOrDefault("FFMPEG_BIN", "ffmpeg"),
	}

	if _, err := strconv.Atoi(cfg.Port); err != nil {
		return Config{}, fmt.Errorf("invalid PORT %q: %w", cfg.Port, err)
	}
	if cfg.TTSTimeoutSecs < 1 {
		return Config{}, fmt.Errorf("TTS_TIMEOUT_SECONDS must be >= 1")
	}

	return cfg, nil
}

func envIntOrDefault(key string, defaultValue int) int {
	v := os.Getenv(key)
	if v == "" {
		return defaultValue
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return defaultValue
	}
	return n
}

func envOrDefault(key, defaultValue string) string {
	v := os.Getenv(key)
	if v == "" {
		return defaultValue
	}
	return v
}
