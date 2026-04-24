package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/hazn/monkeytype-tui/internal/appdir"
)

type Correction struct {
	Index    int
	Original string
	Fixed    string
}

type Result struct {
	CorrectedWords    []string
	Corrections       []Correction
	RawCorrectedText  string
	WordCountMismatch bool
	ResponseID        string
	ResponseModel     string
	PromptTokens      int
	CompletionTokens  int
	TotalTokens       int
}

type RequestInfo struct {
	Provider            string
	BaseURL             string
	Model               string
	SystemPrompt        string
	UserPrompt          string
	Temperature         float64
	TopP                float64
	MaxCompletionTokens int
	ReasoningEffort     string
}

type chatRequest struct {
	Model             string        `json:"model"`
	Messages          []chatMessage `json:"messages"`
	Temperature       float64       `json:"temperature"`
	TopP              float64       `json:"top_p"`
	MaxCompletionToks int           `json:"max_completion_tokens"`
	ReasoningEffort   string        `json:"reasoning_effort"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	ID      string `json:"id"`
	Model   string `json:"model"`
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Usage tokenUsage `json:"usage"`
}

type tokenUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type errorResponse struct {
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Set at build time via -ldflags "-X github.com/hazn/monkeytype-tui/internal/llm.embeddedAPIKey=..."
var embeddedAPIKey string

const (
	systemPrompt = "You are a spellchecker. Fix spelling errors in the text below. Output ONLY the corrected text, nothing else. Do not change capitalization, punctuation, or word count. Do not add or remove words."
	defaultURL   = "https://api.groq.com/openai/v1/chat/completions"
	modelName    = "openai/gpt-oss-20b"
	// gpt-oss spends completion budget on hidden reasoning. 128 can return
	// empty content with finish_reason=length for noisy typo-heavy input.
	maxCompletionTokens = 512
)

func loadAPIKey() string {
	if key := os.Getenv("GROQ_API_KEY"); key != "" {
		return key
	}

	if key := readAPIKeyFromEnvFile(appdir.EnvPath()); key != "" {
		return key
	}

	return embeddedAPIKey
}

func readAPIKeyFromEnvFile(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if k, v, ok := strings.Cut(line, "="); ok && k == "GROQ_API_KEY" {
			return v
		}
	}
	return ""
}

func Spellcheck(typedWords []string) (*Result, error) {
	apiKey := loadAPIKey()
	if apiKey == "" {
		return nil, fmt.Errorf("GROQ_API_KEY not set (env or %s)", appdir.EnvPath())
	}
	return spellcheck(typedWords, apiKey, defaultURL, http.DefaultClient)
}

func SpellcheckRequestInfo(typedWords []string) RequestInfo {
	return RequestInfo{
		Provider:            "groq",
		BaseURL:             defaultURL,
		Model:               modelName,
		SystemPrompt:        systemPrompt,
		UserPrompt:          strings.Join(typedWords, " "),
		Temperature:         0,
		TopP:                0.2,
		MaxCompletionTokens: maxCompletionTokens,
		ReasoningEffort:     "low",
	}
}

func spellcheck(typedWords []string, apiKey, baseURL string, client *http.Client) (*Result, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	requestInfo := SpellcheckRequestInfo(typedWords)
	requestInfo.BaseURL = baseURL

	reqBody := chatRequest{
		Model:             requestInfo.Model,
		Temperature:       requestInfo.Temperature,
		TopP:              requestInfo.TopP,
		MaxCompletionToks: requestInfo.MaxCompletionTokens,
		ReasoningEffort:   requestInfo.ReasoningEffort,
		Messages: []chatMessage{
			{Role: "system", Content: requestInfo.SystemPrompt},
			{Role: "user", Content: requestInfo.UserPrompt},
		},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("api call: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var apiErr errorResponse
		if err := json.NewDecoder(resp.Body).Decode(&apiErr); err == nil && apiErr.Error.Message != "" {
			return nil, fmt.Errorf("api returned %d: %s", resp.StatusCode, apiErr.Error.Message)
		}
		return nil, fmt.Errorf("api returned %d", resp.StatusCode)
	}

	var chatResp chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	rawCorrected := strings.TrimSpace(chatResp.Choices[0].Message.Content)
	correctedWords := strings.Fields(rawCorrected)
	wordCountMismatch := len(correctedWords) != len(typedWords)

	// The model sometimes splits or inserts words despite the prompt. That output
	// cannot be safely aligned word-by-word, so ignore it instead of turning a
	// completed typing test into an LLM error screen.
	if wordCountMismatch {
		correctedWords = make([]string, len(typedWords))
		copy(correctedWords, typedWords)
	}

	var corrections []Correction
	for i, orig := range typedWords {
		if correctedWords[i] != orig {
			corrections = append(corrections, Correction{
				Index:    i,
				Original: orig,
				Fixed:    correctedWords[i],
			})
		}
	}

	return &Result{
		CorrectedWords:    correctedWords,
		Corrections:       corrections,
		RawCorrectedText:  rawCorrected,
		WordCountMismatch: wordCountMismatch,
		ResponseID:        chatResp.ID,
		ResponseModel:     chatResp.Model,
		PromptTokens:      chatResp.Usage.PromptTokens,
		CompletionTokens:  chatResp.Usage.CompletionTokens,
		TotalTokens:       chatResp.Usage.TotalTokens,
	}, nil
}
