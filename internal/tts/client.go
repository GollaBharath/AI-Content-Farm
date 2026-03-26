package tts

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

type Client interface {
	Synthesize(ctx context.Context, text, voice, language, outDir string) (string, error)
	ListVoices(ctx context.Context) ([]Voice, error)
	Preview(ctx context.Context, text, voice, language string) ([]byte, error)
}

type Voice struct {
	Key          string `json:"key"`
	Name         string `json:"name"`
	LanguageCode string `json:"language_code"`
	Quality      string `json:"quality"`
}

type HTTPClient struct {
	baseURL   string
	synthPath string
	http      *http.Client
}

func NewHTTPClient(baseURL, synthPath string, timeout time.Duration) *HTTPClient {
	if synthPath == "" {
		synthPath = "/api/tts"
	}
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &HTTPClient{
		baseURL:   strings.TrimRight(baseURL, "/"),
		synthPath: synthPath,
		http:      &http.Client{Timeout: timeout},
	}
}

type synthRequest struct {
	Text       string `json:"text"`
	VoiceKey   string `json:"voice_key,omitempty"`
	SpeakerID  string `json:"speaker_id,omitempty"`
	LanguageID string `json:"language_id,omitempty"`
}

func (c *HTTPClient) Synthesize(ctx context.Context, text, voice, language, outDir string) (string, error) {
	if text == "" {
		return "", fmt.Errorf("text cannot be empty")
	}
	if c.baseURL == "" {
		return "", fmt.Errorf("tts base URL cannot be empty")
	}

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return "", fmt.Errorf("create output dir: %w", err)
	}

	fileName := fmt.Sprintf("voiceover-%d.wav", time.Now().UnixNano())
	outPath := filepath.Join(outDir, fileName)

	audioBytes, err := c.synthesizeBytes(ctx, text, voice, language)
	if err != nil {
		return "", err
	}

	if err := os.WriteFile(outPath, audioBytes, 0o644); err != nil {
		return "", fmt.Errorf("write tts output: %w", err)
	}

	return outPath, nil
}

func (c *HTTPClient) Preview(ctx context.Context, text, voice, language string) ([]byte, error) {
	if strings.TrimSpace(text) == "" {
		text = "This is a voice preview from Piper text to speech."
	}
	return c.synthesizeBytes(ctx, text, voice, language)
}

func (c *HTTPClient) synthesizeBytes(ctx context.Context, text, voice, language string) ([]byte, error) {
	if text == "" {
		return nil, fmt.Errorf("text cannot be empty")
	}
	if c.baseURL == "" {
		return nil, fmt.Errorf("tts base URL cannot be empty")
	}

	payload, err := json.Marshal(synthRequest{
		Text:       text,
		VoiceKey:   voice,
		SpeakerID:  voice,
		LanguageID: language,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal tts request: %w", err)
	}

	doer := c.http
	if timeout := estimatedSynthesisTimeout(text, c.http.Timeout); timeout > c.http.Timeout {
		clone := *c.http
		clone.Timeout = timeout
		doer = &clone
	}

	jsonURL := c.baseURL + c.synthPath
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, jsonURL, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("build tts request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "audio/wav, application/octet-stream")

	resp, err := doer.Do(req)
	if err != nil {
		return nil, fmt.Errorf("tts request failed: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		resp.Body.Close()

		form := url.Values{}
		form.Set("text", text)
		if strings.TrimSpace(voice) != "" {
			form.Set("voice_key", voice)
			form.Set("speaker_id", voice)
		}
		if strings.TrimSpace(language) != "" {
			form.Set("language_id", language)
		}

		formReq, err := http.NewRequestWithContext(ctx, http.MethodPost, jsonURL, strings.NewReader(form.Encode()))
		if err != nil {
			return nil, fmt.Errorf("build fallback tts request: %w", err)
		}
		formReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		formReq.Header.Set("Accept", "audio/wav, application/octet-stream")

		resp, err = doer.Do(formReq)
		if err != nil {
			return nil, fmt.Errorf("fallback tts request failed: %w", err)
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			defer resp.Body.Close()
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 4*1024))
			return nil, fmt.Errorf("tts service returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
		}
	}
	defer resp.Body.Close()

	audioBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read tts response: %w", err)
	}
	if len(audioBytes) == 0 {
		return nil, fmt.Errorf("tts response body was empty")
	}
	return audioBytes, nil
}

func estimatedSynthesisTimeout(text string, base time.Duration) time.Duration {
	if base <= 0 {
		base = 30 * time.Second
	}

	words := len(strings.Fields(text))
	estimate := 30*time.Second + time.Duration(words)*120*time.Millisecond
	if estimate < base {
		estimate = base
	}
	if estimate > 8*time.Minute {
		estimate = 8 * time.Minute
	}
	return estimate
}

func (c *HTTPClient) ListVoices(ctx context.Context) ([]Voice, error) {
	if c.baseURL == "" {
		return nil, fmt.Errorf("tts base URL cannot be empty")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/", nil)
	if err != nil {
		return nil, fmt.Errorf("build voices request: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("voices request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4*1024))
		return nil, fmt.Errorf("voices status %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("read voices response: %w", err)
	}

	htmlText := string(body)
	keys := parseSelectOptions(htmlText, "speaker_id")
	if len(keys) == 0 {
		return []Voice{}, nil
	}

	voices := make([]Voice, 0, len(keys))
	for _, key := range keys {
		lang := inferLanguageFromVoiceKey(key)
		voices = append(voices, Voice{
			Key:          key,
			Name:         key,
			LanguageCode: lang,
			Quality:      inferQualityFromVoiceKey(key),
		})
	}

	sort.Slice(voices, func(i, j int) bool {
		if voices[i].LanguageCode == voices[j].LanguageCode {
			return voices[i].Key < voices[j].Key
		}
		return voices[i].LanguageCode < voices[j].LanguageCode
	})

	return voices, nil
}

func parseSelectOptions(htmlText, selectID string) []string {
	selectRe := regexp.MustCompile(`(?is)<select[^>]*id=["']` + regexp.QuoteMeta(selectID) + `["'][^>]*>(.*?)</select>`)
	selectMatch := selectRe.FindStringSubmatch(htmlText)
	if len(selectMatch) < 2 {
		return nil
	}

	optionRe := regexp.MustCompile(`(?is)<option[^>]*value=["']([^"']+)["'][^>]*>(.*?)</option>`)
	optionMatches := optionRe.FindAllStringSubmatch(selectMatch[1], -1)
	values := make([]string, 0, len(optionMatches))
	seen := map[string]struct{}{}
	for _, m := range optionMatches {
		if len(m) < 2 {
			continue
		}
		v := strings.TrimSpace(html.UnescapeString(m[1]))
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		values = append(values, v)
	}
	return values
}

func inferLanguageFromVoiceKey(key string) string {
	parts := strings.Split(key, "-")
	if len(parts) == 0 {
		return ""
	}
	lang := strings.TrimSpace(parts[0])
	langRe := regexp.MustCompile(`^[a-z]{2}(?:_[A-Z]{2})?$`)
	if langRe.MatchString(lang) {
		return lang
	}
	return ""
}

func inferQualityFromVoiceKey(key string) string {
	parts := strings.Split(key, "-")
	if len(parts) < 2 {
		return ""
	}
	q := strings.ToLower(strings.TrimSpace(parts[len(parts)-1]))
	switch q {
	case "x_low", "low", "medium", "high":
		return q
	default:
		return ""
	}
}
