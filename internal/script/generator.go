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
	Generate(ctx context.Context, req job.Request) (GeneratedContent, error)
}

type GeneratedContent struct {
	Title  string   `json:"title,omitempty"`
	Script string   `json:"script"`
	Tags   []string `json:"tags,omitempty"`
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
	Model          string            `json:"model"`
	Messages       []chatMessage     `json:"messages"`
	ResponseFormat map[string]string `json:"response_format,omitempty"`
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

func (g *GeminiOpenRouterGenerator) Generate(ctx context.Context, req job.Request) (GeneratedContent, error) {
	// Try Gemini first
	if strings.TrimSpace(g.geminiAPIKey) != "" {
		content, err := g.callGemini(ctx, req)
		if err == nil {
			return content, nil
		}
		log.Printf("Gemini request failed, falling back to OpenRouter: %v", err)
	}

	// Fall back to OpenRouter
	if strings.TrimSpace(g.openRouterAPIKey) != "" {
		content, err := g.callOpenRouter(ctx, req)
		if err == nil {
			return content, nil
		}
		log.Printf("OpenRouter request failed: %v", err)
		return GeneratedContent{}, err
	}

	// Both APIs unavailable, use fallback
	return fallbackContent(req), nil
}

func (g *GeminiOpenRouterGenerator) callGemini(ctx context.Context, req job.Request) (GeneratedContent, error) {
	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" {
		prompt = "Write a clean, engaging faceless short video script."
	}

	userPrompt := prompt

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
		return GeneratedContent{}, fmt.Errorf("marshal gemini request: %w", err)
	}

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/gemini-2.0-flash:generateContent?key=%s", g.geminiAPIKey)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		return GeneratedContent{}, fmt.Errorf("build gemini request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := g.http.Do(httpReq)
	if err != nil {
		return GeneratedContent{}, fmt.Errorf("gemini request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 8*1024))
		return GeneratedContent{}, fmt.Errorf("gemini status %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	var out geminiResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return GeneratedContent{}, fmt.Errorf("decode gemini response: %w", err)
	}

	if out.Error.Message != "" {
		return GeneratedContent{}, fmt.Errorf("gemini error: %s", out.Error.Message)
	}

	if len(out.Candidates) == 0 || len(out.Candidates[0].Content.Parts) == 0 {
		return GeneratedContent{}, fmt.Errorf("gemini returned no content")
	}

	content, err := normalizeGeneratedContent(strings.TrimSpace(out.Candidates[0].Content.Parts[0].Text))
	if err != nil {
		return GeneratedContent{}, fmt.Errorf("gemini returned empty script")
	}
	return content, nil
}

func (g *GeminiOpenRouterGenerator) callOpenRouter(ctx context.Context, req job.Request) (GeneratedContent, error) {
	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" {
		prompt = "Write a clean, engaging faceless short video script."
	}

	systemMsg := "You write concise scripts for faceless short videos. Output as JSON with 'script' field containing the script."
	userMsg := prompt

	payload := openRouterRequest{
		Model: g.openRouterModel,
		Messages: []chatMessage{
			{Role: "system", Content: systemMsg},
			{Role: "user", Content: userMsg},
		},
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		return GeneratedContent{}, fmt.Errorf("marshal openrouter request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://openrouter.ai/api/v1/chat/completions", bytes.NewReader(raw))
	if err != nil {
		return GeneratedContent{}, fmt.Errorf("build openrouter request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+g.openRouterAPIKey)

	resp, err := g.http.Do(httpReq)
	if err != nil {
		return GeneratedContent{}, fmt.Errorf("openrouter request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 8*1024))
		return GeneratedContent{}, fmt.Errorf("openrouter status %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	var out openRouterResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return GeneratedContent{}, fmt.Errorf("decode openrouter response: %w", err)
	}

	if out.Error.Message != "" {
		return GeneratedContent{}, fmt.Errorf("openrouter error: %s", out.Error.Message)
	}

	if len(out.Choices) == 0 {
		return GeneratedContent{}, fmt.Errorf("openrouter returned no choices")
	}

	content, err := normalizeGeneratedContent(strings.TrimSpace(out.Choices[0].Message.Content))
	if err != nil {
		return GeneratedContent{}, fmt.Errorf("openrouter returned empty script")
	}
	return content, nil
}

func normalizeGeneratedContent(raw string) (GeneratedContent, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return GeneratedContent{}, fmt.Errorf("empty script")
	}

	cleaned := stripMarkdownFence(raw)
	if content := extractContentFromJSON(cleaned); content.Script != "" {
		return content, nil
	}

	if content := extractContentFromJSON(raw); content.Script != "" {
		return content, nil
	}

	cleaned = strings.TrimSpace(cleaned)
	if cleaned == "" {
		return GeneratedContent{}, fmt.Errorf("empty script")
	}
	return GeneratedContent{Script: cleaned}, nil
}

func stripMarkdownFence(s string) string {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "```") {
		return s
	}

	withoutOpen := strings.TrimPrefix(s, "```")
	if idx := strings.IndexByte(withoutOpen, '\n'); idx >= 0 {
		withoutOpen = withoutOpen[idx+1:]
	} else {
		withoutOpen = strings.TrimPrefix(withoutOpen, "json")
	}

	if end := strings.LastIndex(withoutOpen, "```"); end >= 0 {
		withoutOpen = withoutOpen[:end]
	}

	return strings.TrimSpace(withoutOpen)
}

func extractContentFromJSON(s string) GeneratedContent {
	s = strings.TrimSpace(s)
	if s == "" {
		return GeneratedContent{}
	}

	var payload any
	if err := json.Unmarshal([]byte(s), &payload); err != nil {
		content := GeneratedContent{
			Title:  extractStringFieldHeuristic(s, []string{"title", "vid_title"}),
			Script: extractStringFieldHeuristic(s, []string{"script", "content", "text", "narration", "voiceover", "output"}),
			Tags:   extractStringArrayFieldHeuristic(s, []string{"tags", "vid_tags"}),
		}
		if content.Script == "" {
			return GeneratedContent{}
		}
		return content
	}

	script := findValueByKey(payload, []string{"script", "content", "text", "narration", "voiceover", "output"})
	if script == "" {
		script = extractStringFieldHeuristic(s, []string{"script", "content", "text", "narration", "voiceover", "output"})
	}
	if script == "" {
		return GeneratedContent{}
	}

	title := findValueByKey(payload, []string{"title", "vid_title"})
	if title == "" {
		title = extractStringFieldHeuristic(s, []string{"title", "vid_title"})
	}

	tags := findStringArrayByKey(payload, []string{"tags", "vid_tags"})
	if len(tags) == 0 {
		tags = extractStringArrayFieldHeuristic(s, []string{"tags", "vid_tags"})
	}

	return GeneratedContent{
		Title:  strings.TrimSpace(title),
		Script: strings.TrimSpace(script),
		Tags:   tags,
	}
}

func extractStringFieldHeuristic(s string, keys []string) string {
	lower := strings.ToLower(s)
	for _, key := range keys {
		needle := `"` + strings.ToLower(strings.TrimSpace(key)) + `"`
		keyIdx := strings.Index(lower, needle)
		if keyIdx < 0 {
			continue
		}

		afterKey := s[keyIdx+len(needle):]
		colonIdx := strings.Index(afterKey, ":")
		if colonIdx < 0 {
			continue
		}

		value := strings.TrimSpace(afterKey[colonIdx+1:])
		if value == "" || value[0] != '"' {
			continue
		}

		parsed, ok := readJSONStringValue(value)
		if !ok {
			continue
		}
		parsed = strings.TrimSpace(parsed)
		if parsed != "" {
			return parsed
		}
	}
	return ""
}

func readJSONStringValue(s string) (string, bool) {
	value, _, ok := readJSONStringValueWithConsumed(s)
	return value, ok
}

func readJSONStringValueWithConsumed(s string) (string, int, bool) {
	if s == "" || s[0] != '"' {
		return "", 0, false
	}

	var b strings.Builder
	for i := 1; i < len(s); i++ {
		ch := s[i]

		if ch == '\\' {
			if i+1 >= len(s) {
				return "", 0, false
			}
			next := s[i+1]
			switch next {
			case 'n':
				b.WriteByte('\n')
			case 'r':
				b.WriteByte('\r')
			case 't':
				b.WriteByte('\t')
			case '"':
				b.WriteByte('"')
			case '\\':
				b.WriteByte('\\')
			case '/':
				b.WriteByte('/')
			default:
				// Preserve unknown escapes as-is so content is not dropped.
				b.WriteByte(next)
			}
			i++
			continue
		}

		if ch == '"' {
			return b.String(), i + 1, true
		}

		b.WriteByte(ch)
	}

	return "", 0, false
}

func extractStringArrayFieldHeuristic(s string, keys []string) []string {
	lower := strings.ToLower(s)
	for _, key := range keys {
		needle := `"` + strings.ToLower(strings.TrimSpace(key)) + `"`
		keyIdx := strings.Index(lower, needle)
		if keyIdx < 0 {
			continue
		}

		afterKey := s[keyIdx+len(needle):]
		colonIdx := strings.Index(afterKey, ":")
		if colonIdx < 0 {
			continue
		}

		value := strings.TrimSpace(afterKey[colonIdx+1:])
		if value == "" {
			continue
		}

		if value[0] == '"' {
			parsed, ok := readJSONStringValue(value)
			if !ok {
				continue
			}
			tags := splitTags(parsed)
			if len(tags) > 0 {
				return tags
			}
			continue
		}

		if value[0] != '[' {
			continue
		}

		tags, ok := readJSONArrayStrings(value)
		if ok && len(tags) > 0 {
			return tags
		}
	}
	return nil
}

func readJSONArrayStrings(s string) ([]string, bool) {
	if s == "" || s[0] != '[' {
		return nil, false
	}

	tags := make([]string, 0, 8)
	for i := 1; i < len(s); {
		for i < len(s) && (s[i] == ' ' || s[i] == '\n' || s[i] == '\r' || s[i] == '\t' || s[i] == ',') {
			i++
		}
		if i >= len(s) {
			break
		}
		if s[i] == ']' {
			return dedupeTags(tags), true
		}
		if s[i] != '"' {
			return nil, false
		}

		value, consumed, ok := readJSONStringValueWithConsumed(s[i:])
		if !ok {
			return nil, false
		}
		if tag := strings.TrimSpace(value); tag != "" {
			tags = append(tags, tag)
		}
		i += consumed
	}

	return nil, false
}

func findValueByKey(v any, keys []string) string {
	switch t := v.(type) {
	case map[string]any:
		for _, key := range keys {
			if value, ok := t[key]; ok {
				if result := stringifyScriptValue(value); result != "" {
					return result
				}
			}
		}
		for _, value := range t {
			if result := findValueByKey(value, keys); result != "" {
				return result
			}
		}
	case []any:
		for _, value := range t {
			if result := findValueByKey(value, keys); result != "" {
				return result
			}
		}
	}

	return ""
}

func findStringArrayByKey(v any, keys []string) []string {
	switch t := v.(type) {
	case map[string]any:
		for _, key := range keys {
			if value, ok := t[key]; ok {
				tags := stringifyTagValue(value)
				if len(tags) > 0 {
					return tags
				}
			}
		}
		for _, value := range t {
			if tags := findStringArrayByKey(value, keys); len(tags) > 0 {
				return tags
			}
		}
	case []any:
		for _, value := range t {
			if tags := findStringArrayByKey(value, keys); len(tags) > 0 {
				return tags
			}
		}
	}

	return nil
}

func stringifyScriptValue(v any) string {
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case []any:
		lines := make([]string, 0, len(t))
		for _, part := range t {
			if s, ok := part.(string); ok {
				s = strings.TrimSpace(s)
				if s != "" {
					lines = append(lines, s)
				}
			}
		}
		return strings.TrimSpace(strings.Join(lines, "\n"))
	default:
		return ""
	}
}

func stringifyTagValue(v any) []string {
	switch t := v.(type) {
	case string:
		return splitTags(t)
	case []any:
		tags := make([]string, 0, len(t))
		for _, item := range t {
			if s, ok := item.(string); ok {
				s = strings.TrimSpace(s)
				if s != "" {
					tags = append(tags, s)
				}
			}
		}
		return dedupeTags(tags)
	default:
		return nil
	}
}

func splitTags(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	tags := make([]string, 0, len(parts))
	for _, part := range parts {
		p := strings.TrimSpace(part)
		if p != "" {
			tags = append(tags, p)
		}
	}
	return dedupeTags(tags)
}

func dedupeTags(tags []string) []string {
	if len(tags) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(tags))
	result := make([]string, 0, len(tags))
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		k := strings.ToLower(tag)
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		result = append(result, tag)
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func fallbackContent(req job.Request) GeneratedContent {
	script := strings.TrimSpace(req.Prompt)
	if script == "" {
		script = "Hook: You won't believe this. Today we break down something surprising in under a minute. First, the part nobody tells you. Second, the trick that changes everything. Third, the move you can use right now. Follow for the next one."
	}
	return GeneratedContent{
		Script: script,
	}
}
