package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Port                 string
	StorageDir           string
	DatabasePath         string
	InputVideosDir       string
	OutputVideosDir      string
	GeminiAPIKey         string
	OpenRouterAPIKey     string
	OpenRouterModel      string
	TTSBaseURL           string
	TTSSynthPath         string
	TTSTimeoutSecs       int
	FFmpegBinaryPath     string
	AutoPilotEnabled     bool
	AutoPilotEvery       int
	AutoTopic            string
	AutoPrompt           string
	AutoVoice            string
}

func Load() (Config, error) {
	cfg := Config{
		Port:             envOrDefault("PORT", "8080"),
		StorageDir:       envOrDefault("STORAGE_DIR", "./data"),
		DatabasePath:     envOrDefault("DB_PATH", "./data/app.db"),
		InputVideosDir:   envOrDefault("INPUT_VIDEOS_DIR", "./videos"),
		OutputVideosDir:  envOrDefault("OUTPUT_VIDEOS_DIR", "./data"),
		GeminiAPIKey:     os.Getenv("GEMINI_API_KEY"),
		OpenRouterAPIKey: os.Getenv("OPENROUTER_API_KEY"),
		OpenRouterModel:  envOrDefault("OPENROUTER_MODEL", "google/gemini-2.0-flash-001"),
		TTSBaseURL:       envOrDefault("TTS_BASE_URL", "http://localhost:5002"),
		TTSSynthPath:     envOrDefault("TTS_SYNTH_PATH", "/api/tts"),
		TTSTimeoutSecs:   envIntOrDefault("TTS_TIMEOUT_SECONDS", 30),
		FFmpegBinaryPath: envOrDefault("FFMPEG_BIN", "ffmpeg"),
		AutoPilotEnabled: envBoolOrDefault("AUTOPILOT_ENABLED", false),
		AutoPilotEvery:   envIntOrDefault("AUTOPILOT_EVERY_SECONDS", 1800),
		AutoTopic:        envOrDefault("AUTOPILOT_TOPIC", "daily high-retention story"),
		AutoPrompt:       envOrDefault("AUTOPILOT_PROMPT", "Create one punchy, high-retention short script with a strong hook."),
		AutoVoice:        os.Getenv("AUTOPILOT_VOICE"),
	}

	if _, err := strconv.Atoi(cfg.Port); err != nil {
		return Config{}, fmt.Errorf("invalid PORT %q: %w", cfg.Port, err)
	}
	if cfg.TTSTimeoutSecs < 1 {
		return Config{}, fmt.Errorf("TTS_TIMEOUT_SECONDS must be >= 1")
	}
	if strings.TrimSpace(cfg.InputVideosDir) == "" || strings.TrimSpace(cfg.OutputVideosDir) == "" {
		return Config{}, fmt.Errorf("INPUT_VIDEOS_DIR and OUTPUT_VIDEOS_DIR are required")
	}
	if cfg.AutoPilotEvery < 30 {
		return Config{}, fmt.Errorf("AUTOPILOT_EVERY_SECONDS must be >= 30")
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

func envBoolOrDefault(key string, defaultValue bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return defaultValue
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return defaultValue
	}
	return b
}
