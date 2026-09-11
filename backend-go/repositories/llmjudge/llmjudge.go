// Package llmjudge talks to an OpenAI-compatible chat-completions endpoint
// to score interview answers. It is a "repository" in the same sense as
// repositories/postgres/*: an adapter to an external system, consumed by
// services/scoring.
package llmjudge

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"oriva/backend-go/config"

	"go.uber.org/zap"
)

// Result is one judge call's verdict.
type Result struct {
	Value     float64 // 0-100
	Rationale string
	Model     string
}

// Client scores one prompt (a turn or a whole-interview summary) via a
// judge LLM.
type Client interface {
	Score(ctx context.Context, systemPrompt, userPrompt string) (Result, error)
}

// New builds a Client. OpenRouter is the primary judge; if
// cfg.OpenRouter.APIKey is empty, or an OpenRouter call fails at runtime
// (rate limit, outage, ...), it falls back to the local Ollama judge.
func New(cfg config.Judge, logger *zap.Logger) Client {
	fallback := newHTTPJudge(cfg.Ollama, "ollama")
	if cfg.OpenRouter.APIKey == "" {
		logger.Info("llmjudge: no openrouter api key configured, using ollama only")
		return fallback
	}
	return &withFallback{
		primary:  newHTTPJudge(cfg.OpenRouter, "openrouter"),
		fallback: fallback,
		logger:   logger,
	}
}

type withFallback struct {
	primary, fallback Client
	logger            *zap.Logger
}

func (c *withFallback) Score(ctx context.Context, systemPrompt, userPrompt string) (Result, error) {
	res, err := c.primary.Score(ctx, systemPrompt, userPrompt)
	if err == nil {
		return res, nil
	}
	c.logger.Warn("llmjudge: openrouter call failed, falling back to ollama", zap.Error(err))
	return c.fallback.Score(ctx, systemPrompt, userPrompt)
}

type httpJudge struct {
	provider config.JudgeProvider
	source   string
	client   *http.Client
}

func newHTTPJudge(provider config.JudgeProvider, source string) *httpJudge {
	return &httpJudge{
		provider: provider,
		source:   source,
		client:   &http.Client{Timeout: 30 * time.Second},
	}
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
}

type chatResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
}

func (j *httpJudge) Score(ctx context.Context, systemPrompt, userPrompt string) (Result, error) {
	body, err := json.Marshal(chatRequest{
		Model: j.provider.Model,
		Messages: []chatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		Temperature: 0.2, // scoring should be consistent, not creative
	})
	if err != nil {
		return Result{}, fmt.Errorf("llmjudge(%s): marshal request: %w", j.source, err)
	}

	url := strings.TrimSuffix(j.provider.BaseURL, "/") + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return Result{}, fmt.Errorf("llmjudge(%s): build request: %w", j.source, err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+j.provider.APIKey)

	resp, err := j.client.Do(req)
	if err != nil {
		return Result{}, fmt.Errorf("llmjudge(%s): request failed: %w", j.source, err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return Result{}, fmt.Errorf("llmjudge(%s): read response: %w", j.source, err)
	}
	if resp.StatusCode != http.StatusOK {
		return Result{}, fmt.Errorf("llmjudge(%s): status %d: %s", j.source, resp.StatusCode, respBody)
	}

	var parsed chatResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return Result{}, fmt.Errorf("llmjudge(%s): decode response: %w", j.source, err)
	}
	if len(parsed.Choices) == 0 {
		return Result{}, fmt.Errorf("llmjudge(%s): no choices in response", j.source)
	}

	value, rationale, err := parseScoreJSON(parsed.Choices[0].Message.Content)
	if err != nil {
		return Result{}, fmt.Errorf("llmjudge(%s): %w", j.source, err)
	}
	return Result{Value: value, Rationale: rationale, Model: j.provider.Model}, nil
}

var jsonObjectPattern = regexp.MustCompile(`(?s)\{.*\}`)

// parseScoreJSON extracts {"score": <number>, "reasoning": "<text>"} from a
// judge model's reply, tolerating markdown code fences and any leading or
// trailing prose around the JSON object (models don't always follow
// "respond with ONLY JSON" precisely).
func parseScoreJSON(content string) (float64, string, error) {
	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	var out struct {
		Score     float64 `json:"score"`
		Reasoning string  `json:"reasoning"`
	}
	if err := json.Unmarshal([]byte(content), &out); err == nil {
		return out.Score, out.Reasoning, nil
	}
	if m := jsonObjectPattern.FindString(content); m != "" {
		if err := json.Unmarshal([]byte(m), &out); err == nil {
			return out.Score, out.Reasoning, nil
		}
	}
	return 0, "", fmt.Errorf("could not parse judge response as JSON: %q", content)
}
