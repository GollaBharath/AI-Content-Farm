package script

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/Gollabharath/ai-content-farm/internal/job"
)

type Generator interface {
	Generate(ctx context.Context, req job.Request) (string, error)
}

// GeminiOpenRouterGenerator uses Gemini as primary and OpenRouter as fallback
type GeminiOpenRouterGenerator struct {
	geminiAPIKey      string
	openRouterAPIKey  string
	openRouterModel   string
	http              *http.Client
}

func NewGeminiOpenRouterGenerator(geminiAPIKey, openRouterAPIKey, openRouterModel string, timeout time.Duration) *GeminiOpenRouterGenerator {
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	if openRouterModel == "" {
		openRouterModel = "google/gemini-2.0-flash-001"
	}
	return &GeminiOpenRouterGenerator{
		geminiAPIKey:     geminiAPIKey,
		openRouterAPIKey: openRouterAPIKey,
		openRouterModel:  openRouterModel,
		http:             &http.Client{Timeout: timeout},
	}
}

type geminiRequest struct {
	Contents []struct {
		Parts []struct {
			Text string `json:"text"`
		} `json:"parts"`
	} `json:"contents"`
}

type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
}

type openRouterRequest struct {
	Model             string        `json:"model"`
	Messages          []chatMessage `json:"messages"`
	ResponseFormat    map[string]string `json:"response_format,omitempty"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openRouterResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (g *GeminiOpenRouterGenerator) Generate(ctx context.Context, req job.Request) (string, error) {
	// Try Gemini first
	if strings.TrimSpace(g.geminiAPIKey) != "" {
		script, err := g.callGemini(ctx, req)
		if err == nil {
			return script, nil
		}
		log.Printf("Gemini request failed, falling back to OpenRouter: %v", err)
	}

	// Fall back to OpenRouter
	if strings.TrimSpace(g.openRouterAPIKey) != "" {
		script, err := g.callOpenRouter(ctx, req)
		if err == nil {
			return script, nil
		}
		log.Printf("OpenRouter request failed: %v", err)
		return "", err
	}

	// Both APIs unavailable, use fallback
	return fallbackScript(req), nil
}

func (g *GeminiOpenRouterGenerator) callGemini(ctx context.Context, req job.Request) (string, error) {
		// Always use 45s as target length for script generation
		targetSecs := 60
	topic := strings.TrimSpace(req.Topic)
	if topic == "" {
		topic = "a surprising story"
	}

	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" {
		prompt = "Write a clean, engaging faceless short script."
	}

	userPrompt := fmt.Sprintf("Topic: %s\nTarget length: about %d seconds\nExtra direction: %s\n\nWrite one script with a strong hook, fast pacing, and clear ending CTA.", topic, targetSecs, prompt)

	payload := geminiRequest{
		Contents: []struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		}{
			{
				Parts: []struct {
					Text string `json:"text"`
				}{
					{Text: userPrompt},
				},
			},
		},
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal gemini request: %w", err)
	}

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/gemini-2.0-flash:generateContent?key=%s", g.geminiAPIKey)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		return "", fmt.Errorf("build gemini request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := g.http.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("gemini request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 8*1024))
		return "", fmt.Errorf("gemini status %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	var out geminiResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", fmt.Errorf("decode gemini response: %w", err)
	}

	if out.Error.Message != "" {
		return "", fmt.Errorf("gemini error: %s", out.Error.Message)
	}

	if len(out.Candidates) == 0 || len(out.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("gemini returned no content")
	}

	script := strings.TrimSpace(out.Candidates[0].Content.Parts[0].Text)
	if script == "" {
		return "", fmt.Errorf("gemini returned empty script")
	}
	return script, nil
}

func (g *GeminiOpenRouterGenerator) callOpenRouter(ctx context.Context, req job.Request) (string, error) {
		// Always use 45s as target length for script generation
		targetSecs := 60
	topic := strings.TrimSpace(req.Topic)
	if topic == "" {
		topic = "a surprising story"
	}

	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" {
		prompt = "Write a clean, engaging faceless short script."
	}

	systemMsg := "You write concise scripts for faceless short videos. Output plain script text only. No markdown, no stage directions."
	userMsg := fmt.Sprintf("Topic: %s\nTarget length: about %d seconds\nExtra direction: %s\n\nWrite one script with a strong hook, fast pacing, and clear ending CTA.", topic, targetSecs, prompt)

	payload := openRouterRequest{
		Model: g.openRouterModel,
		Messages: []chatMessage{
			{Role: "system", Content: systemMsg},
			{Role: "user", Content: userMsg},
		},
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal openrouter request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://openrouter.ai/api/v1/chat/completions", bytes.NewReader(raw))
	if err != nil {
		return "", fmt.Errorf("build openrouter request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+g.openRouterAPIKey)

	resp, err := g.http.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("openrouter request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 8*1024))
		return "", fmt.Errorf("openrouter status %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	var out openRouterResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", fmt.Errorf("decode openrouter response: %w", err)
	}

	if out.Error.Message != "" {
		return "", fmt.Errorf("openrouter error: %s", out.Error.Message)
	}

	if len(out.Choices) == 0 {
		return "", fmt.Errorf("openrouter returned no choices")
	}

	script := strings.TrimSpace(out.Choices[0].Message.Content)
	if script == "" {
		return "", fmt.Errorf("openrouter returned empty script")
	}
	return script, nil
}

func fallbackScript(req job.Request) string {
	topic := strings.TrimSpace(req.Topic)
	if topic == "" {
		topic = "a surprising story"
	}
	extra := strings.TrimSpace(req.Prompt)
	if extra == "" {
		extra = "Keep it fast and engaging."
	}
	return "Hook: You are not ready for this. " +
		"Today we break down " + topic + " in under a minute. " +
		"First, the part nobody tells you. " +
		"Second, the trick that changes everything. " +
		"Third, the move you can use right now. " +
		"" + extra + " " +
		"Follow for the next one."
}
