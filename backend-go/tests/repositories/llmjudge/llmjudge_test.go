package llmjudge_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"oriva/backend-go/config"
	"oriva/backend-go/repositories/llmjudge"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

// chatServer fakes an OpenAI-compatible /chat/completions endpoint, replying
// with reply (as the assistant message content) or failing every request
// with 500 if fail is true.
func chatServer(t *testing.T, reply string, fail bool) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if fail {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":"boom"}`))
			return
		}
		assert.Equal(t, "/chat/completions", r.URL.Path)
		body := map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"role": "assistant", "content": reply}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(body))
	}))
}

func judgeCfg(baseURL, apiKey string) config.JudgeProvider {
	return config.JudgeProvider{BaseURL: baseURL, APIKey: apiKey, Model: "test-model"}
}

func TestScore_ParsesPlainJSON(t *testing.T) {
	srv := chatServer(t, `{"score": 85, "reasoning": "Solid answer, covered the key tradeoffs."}`, false)
	defer srv.Close()

	client := llmjudge.New(config.Judge{OpenRouter: judgeCfg(srv.URL, "test-key")}, zap.NewNop())
	res, err := client.Score(context.Background(), "system", "user")
	require.NoError(t, err)
	assert.Equal(t, 85.0, res.Value)
	assert.Contains(t, res.Rationale, "tradeoffs")
	assert.Equal(t, "test-model", res.Model)
}

func TestScore_ParsesFencedJSONWithProse(t *testing.T) {
	reply := "Sure, here you go:\n```json\n{\"score\": 42, \"reasoning\": \"Vague on specifics.\"}\n```\nHope that helps!"
	srv := chatServer(t, reply, false)
	defer srv.Close()

	client := llmjudge.New(config.Judge{OpenRouter: judgeCfg(srv.URL, "test-key")}, zap.NewNop())
	res, err := client.Score(context.Background(), "system", "user")
	require.NoError(t, err)
	assert.Equal(t, 42.0, res.Value)
	assert.Contains(t, res.Rationale, "Vague")
}

func TestScore_NoOpenRouterKey_GoesStraightToOllama(t *testing.T) {
	ollama := chatServer(t, `{"score": 70, "reasoning": "ollama answered"}`, false)
	defer ollama.Close()

	client := llmjudge.New(config.Judge{
		OpenRouter: judgeCfg("http://unused.invalid", ""), // empty key -> skipped entirely
		Ollama:     judgeCfg(ollama.URL, "ollama"),
	}, zap.NewNop())

	res, err := client.Score(context.Background(), "system", "user")
	require.NoError(t, err)
	assert.Equal(t, 70.0, res.Value)
}

func TestScore_OpenRouterFails_FallsBackToOllama(t *testing.T) {
	openrouter := chatServer(t, "", true) // always 500s
	defer openrouter.Close()
	ollama := chatServer(t, `{"score": 55, "reasoning": "fallback worked"}`, false)
	defer ollama.Close()

	core, logs := observer.New(zap.WarnLevel)
	logger := zap.New(core)

	client := llmjudge.New(config.Judge{
		OpenRouter: judgeCfg(openrouter.URL, "test-key"),
		Ollama:     judgeCfg(ollama.URL, "ollama"),
	}, logger)

	res, err := client.Score(context.Background(), "system", "user")
	require.NoError(t, err)
	assert.Equal(t, 55.0, res.Value)
	assert.Contains(t, res.Rationale, "fallback worked")
	assert.Equal(t, 1, logs.FilterMessageSnippet("falling back to ollama").Len())
}

func TestScore_BothFail_ReturnsError(t *testing.T) {
	openrouter := chatServer(t, "", true)
	defer openrouter.Close()
	ollama := chatServer(t, "", true)
	defer ollama.Close()

	client := llmjudge.New(config.Judge{
		OpenRouter: judgeCfg(openrouter.URL, "test-key"),
		Ollama:     judgeCfg(ollama.URL, "ollama"),
	}, zap.NewNop())

	_, err := client.Score(context.Background(), "system", "user")
	assert.Error(t, err)
}
