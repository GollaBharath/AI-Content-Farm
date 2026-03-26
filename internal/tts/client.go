package tts

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Client interface {
	Synthesize(ctx context.Context, text, voice, outDir string) (string, error)
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
	Text  string `json:"text"`
	Voice string `json:"voice,omitempty"`
}

func (c *HTTPClient) Synthesize(ctx context.Context, text, voice, outDir string) (string, error) {
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

	payload, err := json.Marshal(synthRequest{Text: text, Voice: voice})
	if err != nil {
		return "", fmt.Errorf("marshal tts request: %w", err)
	}

	url := c.baseURL + c.synthPath
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("build tts request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("tts request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4*1024))
		return "", fmt.Errorf("tts service returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	audioBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read tts response: %w", err)
	}
	if len(audioBytes) == 0 {
		return "", fmt.Errorf("tts response body was empty")
	}

	if err := os.WriteFile(outPath, audioBytes, 0o644); err != nil {
		return "", fmt.Errorf("write tts output: %w", err)
	}

	return outPath, nil
}
