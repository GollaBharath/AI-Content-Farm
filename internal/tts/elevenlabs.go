package tts

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type ElevenLabsClient struct {
	apiKey       string
	baseURL      string
	defaultVoice string
	defaultModel string
	outputFormat string
	http         *http.Client
}

type ElevenLabsConfig struct {
	APIKey       string
	BaseURL      string
	DefaultVoice string
	DefaultModel string
	OutputFormat string
	Timeout      time.Duration
}

func NewElevenLabsClient(cfg ElevenLabsConfig) *ElevenLabsClient {
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if baseURL == "" {
		baseURL = "https://api.elevenlabs.io"
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	defaultModel := strings.TrimSpace(cfg.DefaultModel)
	if defaultModel == "" {
		defaultModel = "eleven_multilingual_v2"
	}
	if strings.EqualFold(defaultModel, "eleven_multilingual_v2_5") {
		defaultModel = "eleven_flash_v2_5"
	}
	outputFormat := strings.TrimSpace(cfg.OutputFormat)
	if outputFormat == "" {
		outputFormat = "mp3_44100_128"
	}

	return &ElevenLabsClient{
		apiKey:       strings.TrimSpace(cfg.APIKey),
		baseURL:      baseURL,
		defaultVoice: strings.TrimSpace(cfg.DefaultVoice),
		defaultModel: defaultModel,
		outputFormat: outputFormat,
		http:         &http.Client{Timeout: timeout},
	}
}

func (c *ElevenLabsClient) Synthesize(ctx context.Context, text, voice, language, outDir string) (string, error) {
	if strings.TrimSpace(text) == "" {
		return "", fmt.Errorf("text cannot be empty")
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return "", fmt.Errorf("create output dir: %w", err)
	}

	bytesOut, err := c.synthesizeBytes(ctx, text, voice, language)
	if err != nil {
		return "", err
	}

	fileName := fmt.Sprintf("voiceover-%d.mp3", time.Now().UnixNano())
	outPath := filepath.Join(outDir, fileName)
	if err := os.WriteFile(outPath, bytesOut, 0o644); err != nil {
		return "", fmt.Errorf("write elevenlabs output: %w", err)
	}
	return outPath, nil
}

func (c *ElevenLabsClient) Preview(ctx context.Context, text, voice, language string) ([]byte, error) {
	if strings.TrimSpace(text) == "" {
		text = "This is an ElevenLabs voice preview."
	}
	return c.synthesizeBytes(ctx, text, voice, language)
}

func (c *ElevenLabsClient) synthesizeBytes(ctx context.Context, text, voice, language string) ([]byte, error) {
	if strings.TrimSpace(c.apiKey) == "" {
		return nil, fmt.Errorf("elevenlabs api key is not configured")
	}
	voiceID := strings.TrimSpace(voice)
	if voiceID == "" {
		voiceID = c.defaultVoice
	}
	if voiceID == "" {
		return nil, fmt.Errorf("elevenlabs voice is required")
	}

	payload := map[string]any{
		"text":     text,
		"model_id": c.defaultModel,
	}
	if strings.TrimSpace(language) != "" {
		payload["language_code"] = strings.TrimSpace(language)
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal elevenlabs request: %w", err)
	}

	doer := c.http
	if timeout := estimatedSynthesisTimeout(text, c.http.Timeout); timeout > c.http.Timeout {
		clone := *c.http
		clone.Timeout = timeout
		doer = &clone
	}

	u := c.baseURL + "/v1/text-to-speech/" + url.PathEscape(voiceID)
	q := url.Values{}
	q.Set("output_format", c.outputFormat)
	u += "?" + q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build elevenlabs request: %w", err)
	}
	req.Header.Set("xi-api-key", c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "audio/mpeg, application/octet-stream")

	resp, err := doer.Do(req)
	if err != nil {
		return nil, fmt.Errorf("elevenlabs request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, 8*1024))
		return nil, fmt.Errorf("elevenlabs returned %s: %s", resp.Status, strings.TrimSpace(string(errBody)))
	}

	audioBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read elevenlabs response: %w", err)
	}
	if len(audioBytes) == 0 {
		return nil, fmt.Errorf("elevenlabs response body was empty")
	}
	return audioBytes, nil
}

func (c *ElevenLabsClient) ListVoices(ctx context.Context) ([]Voice, error) {
	if strings.TrimSpace(c.apiKey) == "" {
		return nil, fmt.Errorf("elevenlabs api key is not configured")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/v1/voices", nil)
	if err != nil {
		return nil, fmt.Errorf("build elevenlabs voices request: %w", err)
	}
	req.Header.Set("xi-api-key", c.apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("elevenlabs voices request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, 8*1024))
		return nil, fmt.Errorf("elevenlabs voices returned %s: %s", resp.Status, strings.TrimSpace(string(errBody)))
	}

	var parsed struct {
		Voices []struct {
			VoiceID  string            `json:"voice_id"`
			Name     string            `json:"name"`
			Category string            `json:"category"`
			Labels   map[string]string `json:"labels"`
		} `json:"voices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("decode elevenlabs voices: %w", err)
	}

	voices := make([]Voice, 0, len(parsed.Voices))
	for _, v := range parsed.Voices {
		if strings.TrimSpace(v.VoiceID) == "" {
			continue
		}
		lang := strings.TrimSpace(v.Labels["language"])
		if lang == "" {
			lang = strings.TrimSpace(v.Labels["accent"])
		}
		voices = append(voices, Voice{
			Key:          v.VoiceID,
			Name:         strings.TrimSpace(v.Name),
			LanguageCode: lang,
			Quality:      strings.TrimSpace(v.Category),
		})
	}

	return voices, nil
}

func (c *ElevenLabsClient) SupportedLanguages() []string {
	model := strings.ToLower(strings.TrimSpace(c.defaultModel))
	base := []string{
		"ar", "bg", "cs", "da", "de", "el", "en", "es", "fi", "fil", "fr", "hi", "hr", "id", "it", "ja", "ko", "ms", "nl", "pl", "pt", "ro", "ru", "sk", "sv", "ta", "tr", "uk", "zh",
	}

	switch model {
	case "eleven_flash_v2_5", "eleven_turbo_v2_5", "eleven_multilingual_v2_5":
		langs := append([]string{}, base...)
		langs = append(langs, "hu", "no", "vi")
		sort.Strings(langs)
		return langs
	case "eleven_multilingual_v2", "eleven_flash_v2":
		sort.Strings(base)
		return base
	default:
		sort.Strings(base)
		return base
	}
}
