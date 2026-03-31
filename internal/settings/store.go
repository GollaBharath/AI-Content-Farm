package settings

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"

	_ "modernc.org/sqlite"
)

type Settings struct {
	InputVideosDir          string `json:"input_videos_dir"`
	OutputVideosDir         string `json:"output_videos_dir"`
	DefaultVideoOrientation string `json:"default_video_orientation"`
	DefaultVideoWidth       int    `json:"default_video_width"`
	DefaultVideoHeight      int    `json:"default_video_height"`
	TTSProvider             string `json:"tts_provider"`
	DefaultVoice            string `json:"default_voice"`
	DefaultLanguage         string `json:"default_language"`
	DefaultPromptIdea       string `json:"default_prompt_idea"`
	PromptPresets           []Preset `json:"prompt_presets"`
}

type Preset struct {
	Name            string `json:"name"`
	Topic           string `json:"topic"`
	Prompt          string `json:"prompt"`
	ScriptOverride  string `json:"script_override"`
	Voice           string `json:"voice"`
	Language        string `json:"language"`
	Orientation     string `json:"orientation"`
	CustomWidth     int    `json:"custom_width"`
	CustomHeight    int    `json:"custom_height"`
	BackgroundVideo string `json:"background_video"`
}

type Update struct {
	InputVideosDir          *string `json:"input_videos_dir"`
	OutputVideosDir         *string `json:"output_videos_dir"`
	DefaultVideoOrientation *string `json:"default_video_orientation"`
	DefaultVideoWidth       *int    `json:"default_video_width"`
	DefaultVideoHeight      *int    `json:"default_video_height"`
	TTSProvider             *string `json:"tts_provider"`
	DefaultVoice            *string `json:"default_voice"`
	DefaultLanguage         *string `json:"default_language"`
	DefaultPromptIdea       *string `json:"default_prompt_idea"`
	PromptPresets           []Preset `json:"prompt_presets"`
}

type Store struct {
	db       *sql.DB
	defaults Settings
	mu       sync.Mutex
}

func NewStore(dbPath string, defaults Settings) (*Store, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	s := &Store{db: db, defaults: defaults}
	if err := s.init(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) init() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS app_settings (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL,
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return fmt.Errorf("create app_settings: %w", err)
	}
	return nil
}

func (s *Store) Get() (Settings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	rows, err := s.db.Query(`SELECT key, value FROM app_settings`)
	if err != nil {
		return Settings{}, fmt.Errorf("query app_settings: %w", err)
	}
	defer rows.Close()

	cfg := s.defaults
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return Settings{}, fmt.Errorf("scan app_settings: %w", err)
		}
		switch key {
		case "input_videos_dir":
			cfg.InputVideosDir = value
		case "output_videos_dir":
			cfg.OutputVideosDir = value
		case "default_video_orientation":
			cfg.DefaultVideoOrientation = value
		case "default_video_width":
			cfg.DefaultVideoWidth = atoiOrZero(value)
		case "default_video_height":
			cfg.DefaultVideoHeight = atoiOrZero(value)
		case "tts_provider":
			cfg.TTSProvider = normalizeTTSProvider(value)
		case "default_voice":
			cfg.DefaultVoice = value
		case "default_language":
			cfg.DefaultLanguage = value
		case "default_prompt_idea":
			cfg.DefaultPromptIdea = value
		case "prompt_presets":
			cfg.PromptPresets = parsePresets(value)
		}
	}

	cfg.DefaultVideoOrientation = normalizeOrientation(cfg.DefaultVideoOrientation)
	if strings.TrimSpace(cfg.InputVideosDir) == "" {
		cfg.InputVideosDir = s.defaults.InputVideosDir
	}
	if strings.TrimSpace(cfg.OutputVideosDir) == "" {
		cfg.OutputVideosDir = s.defaults.OutputVideosDir
	}
	cfg.TTSProvider = normalizeTTSProvider(cfg.TTSProvider)
	return cfg, nil
}

func (s *Store) Update(update Update) (Settings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cfg, err := s.getLocked()
	if err != nil {
		return Settings{}, err
	}

	if update.InputVideosDir != nil {
		cfg.InputVideosDir = strings.TrimSpace(*update.InputVideosDir)
	}
	if update.OutputVideosDir != nil {
		cfg.OutputVideosDir = strings.TrimSpace(*update.OutputVideosDir)
	}
	if update.DefaultVideoOrientation != nil {
		cfg.DefaultVideoOrientation = normalizeOrientation(*update.DefaultVideoOrientation)
	}
	if update.DefaultVideoWidth != nil {
		cfg.DefaultVideoWidth = max(*update.DefaultVideoWidth, 0)
	}
	if update.DefaultVideoHeight != nil {
		cfg.DefaultVideoHeight = max(*update.DefaultVideoHeight, 0)
	}
	if update.TTSProvider != nil {
		cfg.TTSProvider = normalizeTTSProvider(*update.TTSProvider)
	}
	if update.DefaultVoice != nil {
		cfg.DefaultVoice = strings.TrimSpace(*update.DefaultVoice)
	}
	if update.DefaultLanguage != nil {
		cfg.DefaultLanguage = strings.TrimSpace(*update.DefaultLanguage)
	}
	if update.DefaultPromptIdea != nil {
		cfg.DefaultPromptIdea = strings.TrimSpace(*update.DefaultPromptIdea)
	}
	if update.PromptPresets != nil {
		cfg.PromptPresets = sanitizePresets(update.PromptPresets)
	}

	if cfg.InputVideosDir == "" || cfg.OutputVideosDir == "" {
		return Settings{}, fmt.Errorf("input_videos_dir and output_videos_dir are required")
	}

	presetsRaw, err := json.Marshal(cfg.PromptPresets)
	if err != nil {
		return Settings{}, fmt.Errorf("marshal prompt_presets: %w", err)
	}

	pairs := map[string]string{
		"input_videos_dir":          cfg.InputVideosDir,
		"output_videos_dir":         cfg.OutputVideosDir,
		"default_video_orientation": cfg.DefaultVideoOrientation,
		"default_video_width":       strconv.Itoa(cfg.DefaultVideoWidth),
		"default_video_height":      strconv.Itoa(cfg.DefaultVideoHeight),
		"tts_provider":              cfg.TTSProvider,
		"default_voice":             cfg.DefaultVoice,
		"default_language":          cfg.DefaultLanguage,
		"default_prompt_idea":       cfg.DefaultPromptIdea,
		"prompt_presets":            string(presetsRaw),
	}

	for key, value := range pairs {
		if _, err := s.db.Exec(`
			INSERT INTO app_settings(key, value, updated_at)
			VALUES(?, ?, CURRENT_TIMESTAMP)
			ON CONFLICT(key) DO UPDATE SET value=excluded.value, updated_at=CURRENT_TIMESTAMP
		`, key, value); err != nil {
			return Settings{}, fmt.Errorf("save setting %s: %w", key, err)
		}
	}

	return cfg, nil
}

func (s *Store) getLocked() (Settings, error) {
	rows, err := s.db.Query(`SELECT key, value FROM app_settings`)
	if err != nil {
		return Settings{}, fmt.Errorf("query app_settings: %w", err)
	}
	defer rows.Close()

	cfg := s.defaults
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return Settings{}, fmt.Errorf("scan app_settings: %w", err)
		}
		switch key {
		case "input_videos_dir":
			cfg.InputVideosDir = value
		case "output_videos_dir":
			cfg.OutputVideosDir = value
		case "default_video_orientation":
			cfg.DefaultVideoOrientation = value
		case "default_video_width":
			cfg.DefaultVideoWidth = atoiOrZero(value)
		case "default_video_height":
			cfg.DefaultVideoHeight = atoiOrZero(value)
		case "tts_provider":
			cfg.TTSProvider = normalizeTTSProvider(value)
		case "default_voice":
			cfg.DefaultVoice = value
		case "default_language":
			cfg.DefaultLanguage = value
		case "default_prompt_idea":
			cfg.DefaultPromptIdea = value
		case "prompt_presets":
			cfg.PromptPresets = parsePresets(value)
		}
	}

	cfg.DefaultVideoOrientation = normalizeOrientation(cfg.DefaultVideoOrientation)
	if strings.TrimSpace(cfg.InputVideosDir) == "" {
		cfg.InputVideosDir = s.defaults.InputVideosDir
	}
	if strings.TrimSpace(cfg.OutputVideosDir) == "" {
		cfg.OutputVideosDir = s.defaults.OutputVideosDir
	}
	cfg.TTSProvider = normalizeTTSProvider(cfg.TTSProvider)
	return cfg, nil
}

func atoiOrZero(s string) int {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return 0
	}
	return n
}

func normalizeOrientation(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "portrait", "landscape", "square", "original", "custom":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return "portrait"
	}
}

func normalizeTTSProvider(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "elevenlabs":
		return "elevenlabs"
	case "auto":
		return "auto"
	default:
		return "piper"
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func parsePresets(raw string) []Preset {
	if strings.TrimSpace(raw) == "" {
		return []Preset{}
	}
	var presets []Preset
	if err := json.Unmarshal([]byte(raw), &presets); err != nil {
		return []Preset{}
	}
	return sanitizePresets(presets)
}

func sanitizePresets(presets []Preset) []Preset {
	out := make([]Preset, 0, len(presets))
	for _, p := range presets {
		p.Name = strings.TrimSpace(p.Name)
		if p.Name == "" {
			continue
		}
		p.Topic = strings.TrimSpace(p.Topic)
		p.Prompt = strings.TrimSpace(p.Prompt)
		p.ScriptOverride = strings.TrimSpace(p.ScriptOverride)
		p.Voice = strings.TrimSpace(p.Voice)
		p.Language = strings.TrimSpace(p.Language)
		p.Orientation = normalizeOrientation(p.Orientation)
		if p.CustomWidth < 0 {
			p.CustomWidth = 0
		}
		if p.CustomHeight < 0 {
			p.CustomHeight = 0
		}
		p.BackgroundVideo = strings.TrimSpace(p.BackgroundVideo)
		out = append(out, p)
	}
	return out
}
