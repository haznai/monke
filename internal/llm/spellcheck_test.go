package llm

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func fakeServer(t *testing.T, wantResponse string, validate func(t *testing.T, req chatRequest)) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Errorf("auth = %q, want Bearer test-key", got)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("content-type = %q, want application/json", got)
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		var req chatRequest
		if err := json.Unmarshal(body, &req); err != nil {
			t.Fatalf("unmarshal request: %v", err)
		}

		if validate != nil {
			validate(t, req)
		}

		resp := chatResponse{
			ID:    "chatcmpl-test",
			Model: req.Model,
			Choices: []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			}{
				{Message: struct {
					Content string `json:"content"`
				}{Content: wantResponse}},
			},
			Usage: tokenUsage{
				PromptTokens:     12,
				CompletionTokens: 3,
				TotalTokens:      15,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
}

func TestSpellcheck_SendsCorrectRequest(t *testing.T) {
	srv := fakeServer(t, "the quick fox", func(t *testing.T, req chatRequest) {
		if req.Model != "openai/gpt-oss-20b" {
			t.Errorf("model = %q, want openai/gpt-oss-20b", req.Model)
		}
		if req.Temperature != 0 {
			t.Errorf("temperature = %f, want 0", req.Temperature)
		}
		if req.TopP != 0.2 {
			t.Errorf("top_p = %f, want 0.2", req.TopP)
		}
		if req.MaxCompletionToks != 512 {
			t.Errorf("max_completion_tokens = %d, want 512", req.MaxCompletionToks)
		}
		if req.ReasoningEffort != "low" {
			t.Errorf("reasoning_effort = %q, want low", req.ReasoningEffort)
		}
		if len(req.Messages) != 2 {
			t.Fatalf("messages len = %d, want 2", len(req.Messages))
		}
		if req.Messages[0].Role != "system" {
			t.Errorf("messages[0].role = %q, want system", req.Messages[0].Role)
		}
		if req.Messages[0].Content != systemPrompt {
			t.Errorf("system prompt mismatch")
		}
		if req.Messages[1].Role != "user" {
			t.Errorf("messages[1].role = %q, want user", req.Messages[1].Role)
		}
		if req.Messages[1].Content != "teh quick fox" {
			t.Errorf("user content = %q, want %q", req.Messages[1].Content, "teh quick fox")
		}
	})
	defer srv.Close()

	result, err := spellcheck([]string{"teh", "quick", "fox"}, "test-key", srv.URL, srv.Client())
	if err != nil {
		t.Fatalf("spellcheck: %v", err)
	}
	if len(result.CorrectedWords) != 3 {
		t.Fatalf("corrected words len = %d, want 3", len(result.CorrectedWords))
	}
	if result.RawCorrectedText != "the quick fox" {
		t.Fatalf("raw corrected text = %q, want %q", result.RawCorrectedText, "the quick fox")
	}
	if result.WordCountMismatch {
		t.Fatal("word count mismatch should be false")
	}
	if result.ResponseID != "chatcmpl-test" {
		t.Fatalf("response ID = %q, want chatcmpl-test", result.ResponseID)
	}
	if result.ResponseModel != modelName {
		t.Fatalf("response model = %q, want %q", result.ResponseModel, modelName)
	}
	if result.PromptTokens != 12 || result.CompletionTokens != 3 || result.TotalTokens != 15 {
		t.Fatalf("usage = %d/%d/%d, want 12/3/15", result.PromptTokens, result.CompletionTokens, result.TotalTokens)
	}
}

func TestSpellcheckRequestInfo(t *testing.T) {
	info := SpellcheckRequestInfo([]string{"teh", "quick", "fox"})

	if info.Provider != "groq" {
		t.Fatalf("provider = %q, want groq", info.Provider)
	}
	if info.BaseURL != defaultURL {
		t.Fatalf("base URL = %q, want %q", info.BaseURL, defaultURL)
	}
	if info.Model != modelName {
		t.Fatalf("model = %q, want %q", info.Model, modelName)
	}
	if info.SystemPrompt != systemPrompt {
		t.Fatal("system prompt mismatch")
	}
	if info.UserPrompt != "teh quick fox" {
		t.Fatalf("user prompt = %q, want typed words joined", info.UserPrompt)
	}
	if info.MaxCompletionTokens != maxCompletionTokens {
		t.Fatalf("max completion tokens = %d, want %d", info.MaxCompletionTokens, maxCompletionTokens)
	}
	if info.ReasoningEffort != "low" {
		t.Fatalf("reasoning effort = %q, want low", info.ReasoningEffort)
	}
}

func TestSpellcheck_FixesTypos(t *testing.T) {
	srv := fakeServer(t, "the quick brown fox", nil)
	defer srv.Close()

	result, err := spellcheck([]string{"teh", "quikc", "brown", "fox"}, "test-key", srv.URL, srv.Client())
	if err != nil {
		t.Fatalf("spellcheck: %v", err)
	}

	if len(result.Corrections) != 2 {
		t.Fatalf("corrections len = %d, want 2", len(result.Corrections))
	}
	if result.Corrections[0].Original != "teh" || result.Corrections[0].Fixed != "the" {
		t.Errorf("correction[0] = %v, want teh->the", result.Corrections[0])
	}
	if result.Corrections[1].Original != "quikc" || result.Corrections[1].Fixed != "quick" {
		t.Errorf("correction[1] = %v, want quikc->quick", result.Corrections[1])
	}
	if result.RawCorrectedText != "the quick brown fox" {
		t.Errorf("raw corrected text = %q", result.RawCorrectedText)
	}
	if result.WordCountMismatch {
		t.Error("word count mismatch should be false")
	}
}

func TestSpellcheck_NoChangesNeeded(t *testing.T) {
	srv := fakeServer(t, "hello world", nil)
	defer srv.Close()

	result, err := spellcheck([]string{"hello", "world"}, "test-key", srv.URL, srv.Client())
	if err != nil {
		t.Fatalf("spellcheck: %v", err)
	}

	if len(result.Corrections) != 0 {
		t.Errorf("corrections len = %d, want 0", len(result.Corrections))
	}
	if result.CorrectedWords[0] != "hello" || result.CorrectedWords[1] != "world" {
		t.Errorf("corrected = %v, want [hello world]", result.CorrectedWords)
	}
}

func TestSpellcheck_WordCountMismatchFallback(t *testing.T) {
	// The LLM sometimes changes word count despite the prompt. That cannot be
	// aligned safely, but it also should not surface as a scary runtime error.
	srv := fakeServer(t, "the quick brown fox jumps", nil)
	defer srv.Close()

	result, err := spellcheck([]string{"teh", "quikc", "fox"}, "test-key", srv.URL, srv.Client())
	if err != nil {
		t.Fatalf("spellcheck: %v", err)
	}

	if len(result.CorrectedWords) != 3 {
		t.Fatalf("corrected words len = %d, want 3", len(result.CorrectedWords))
	}
	if result.CorrectedWords[0] != "teh" || result.CorrectedWords[1] != "quikc" || result.CorrectedWords[2] != "fox" {
		t.Errorf("corrected = %v, want original typed words", result.CorrectedWords)
	}
	if len(result.Corrections) != 0 {
		t.Errorf("corrections len = %d, want 0", len(result.Corrections))
	}
	if result.RawCorrectedText != "the quick brown fox jumps" {
		t.Errorf("raw corrected text = %q", result.RawCorrectedText)
	}
	if !result.WordCountMismatch {
		t.Error("word count mismatch should be true")
	}
}

func TestSpellcheck_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	_, err := spellcheck([]string{"test"}, "bad-key", srv.URL, srv.Client())
	if err == nil {
		t.Fatal("expected error for 401 response")
	}
	if got := err.Error(); got != "api returned 401" {
		t.Errorf("error = %q, want 'api returned 401'", got)
	}
}

func TestSpellcheck_APIErrorIncludesMessage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusRequestEntityTooLarge)
		json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{
				"message": "Request too large for model",
			},
		})
	}))
	defer srv.Close()

	_, err := spellcheck([]string{"test"}, "test-key", srv.URL, srv.Client())
	if err == nil {
		t.Fatal("expected error for 413 response")
	}
	if got := err.Error(); got != "api returned 413: Request too large for model" {
		t.Errorf("error = %q, want detailed API message", got)
	}
}

func TestSpellcheck_EmptyChoices(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"choices": []any{}})
	}))
	defer srv.Close()

	_, err := spellcheck([]string{"test"}, "test-key", srv.URL, srv.Client())
	if err == nil {
		t.Fatal("expected error for empty choices")
	}
}

func TestSpellcheck_SingleWord(t *testing.T) {
	srv := fakeServer(t, "hello", nil)
	defer srv.Close()

	result, err := spellcheck([]string{"helo"}, "test-key", srv.URL, srv.Client())
	if err != nil {
		t.Fatalf("spellcheck: %v", err)
	}

	if len(result.Corrections) != 1 {
		t.Fatalf("corrections len = %d, want 1", len(result.Corrections))
	}
	if result.Corrections[0].Index != 0 {
		t.Errorf("correction index = %d, want 0", result.Corrections[0].Index)
	}
}

func TestLoadAPIKeyReadsAppDirEnvFile(t *testing.T) {
	t.Setenv("GROQ_API_KEY", "")
	root := t.TempDir()
	t.Setenv("MONKEYTYPE_TUI_HOME", root)

	path := filepath.Join(root, ".env")
	if err := os.WriteFile(path, []byte("GROQ_API_KEY=test-from-env-file\n"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if got := loadAPIKey(); got != "test-from-env-file" {
		t.Fatalf("loadAPIKey() = %q, want %q", got, "test-from-env-file")
	}
}

func TestSpellcheck_MissingAPIKey(t *testing.T) {
	t.Setenv("GROQ_API_KEY", "")
	t.Setenv("MONKEYTYPE_TUI_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir()) // prevent legacy .env fallback
	_, err := Spellcheck([]string{"test"})
	if err == nil {
		t.Fatal("expected error when API key is missing")
	}
}

// Integration test: hits the real Groq API.
// Only runs when GROQ_API_KEY is set.
func TestSpellcheck_Integration(t *testing.T) {
	key := os.Getenv("GROQ_API_KEY")
	if key == "" {
		t.Skip("GROQ_API_KEY not set, skipping integration test")
	}

	result, err := spellcheck(
		[]string{"teh", "quikc", "brown", "fox", "jumpd", "over", "teh", "lazzy", "dog"},
		key, defaultURL, http.DefaultClient,
	)
	if err != nil {
		t.Fatalf("spellcheck: %v", err)
	}

	t.Logf("input:     teh quikc brown fox jumpd over teh lazzy dog")
	t.Logf("corrected: %v", result.CorrectedWords)
	t.Logf("corrections: %v", result.Corrections)

	// The LLM should fix at least some of these obvious typos
	if len(result.Corrections) == 0 {
		t.Error("expected at least some corrections for obviously misspelled input")
	}

	// Verify word count preserved
	if len(result.CorrectedWords) != 9 {
		t.Errorf("corrected word count = %d, want 9", len(result.CorrectedWords))
	}
}

func TestSpellcheck_IntegrationNoisyUserText(t *testing.T) {
	key := os.Getenv("GROQ_API_KEY")
	if key == "" {
		t.Skip("GROQ_API_KEY not set, skipping integration test")
	}

	input := []string{"ichekc", "the", "latst", "sommit,", "teat", "itih", "eht", "embedded", "pai", "key,", "is", "the", "llm", "even", "doing", "something,", "it", "feels", "like", "it", "is", "not", "even", "correcting", "thing"}
	result, err := spellcheck(input, key, defaultURL, http.DefaultClient)
	if err != nil {
		t.Fatalf("spellcheck: %v", err)
	}

	t.Logf("input:     %v", input)
	t.Logf("corrected: %v", result.CorrectedWords)
	t.Logf("corrections: %v", result.Corrections)

	if len(result.CorrectedWords) != len(input) {
		t.Fatalf("corrected word count = %d, want %d", len(result.CorrectedWords), len(input))
	}
	if len(result.Corrections) < 3 {
		t.Fatalf("corrections len = %d, want at least 3 for noisy user text", len(result.Corrections))
	}
}
