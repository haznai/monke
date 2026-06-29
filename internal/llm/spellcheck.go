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
	ProviderSort        string
	SystemPrompt        string
	UserPrompt          string
	Temperature         float64
	TopP                float64
	MaxCompletionTokens int
	ReasoningEffort     string
}

type chatRequest struct {
	Model       string              `json:"model"`
	Messages    []chatMessage       `json:"messages"`
	Temperature float64             `json:"temperature"`
	TopP        float64             `json:"top_p"`
	MaxTokens   int                 `json:"max_tokens,omitempty"`
	Provider    *providerPreference `json:"provider,omitempty"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type providerPreference struct {
	Sort string `json:"sort,omitempty"`
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
	systemPrompt        = "Correct this fast typing-test input. Treat each whitespace-separated input token as one output token: for every input token, output exactly one corrected token in the same position. Fix typos, missing apostrophes, punctuation, and capitalization when obvious. If unsure, leave the token unchanged. Output only the corrected text."
	providerName        = "openrouter"
	defaultURL          = "https://openrouter.ai/api/v1/chat/completions"
	modelName           = "meta-llama/llama-3.2-3b-instruct"
	defaultProviderSort = "latency"

	apiKeyEnv       = "OPENROUTER_API_KEY"
	baseURLEnv      = "OPENROUTER_BASE_URL"
	modelEnv        = "OPENROUTER_MODEL"
	providerSortEnv = "OPENROUTER_PROVIDER_SORT"

	maxCompletionTokens = 512
)

type config struct {
	apiKey       string
	baseURL      string
	model        string
	providerSort string
}

func defaultConfig() config {
	return config{
		baseURL:      defaultURL,
		model:        modelName,
		providerSort: defaultProviderSort,
	}
}

func loadConfig() config {
	cfg := defaultConfig()
	cfg.apiKey = loadAPIKey()
	if baseURL := configValue(baseURLEnv); baseURL != "" {
		cfg.baseURL = normalizeChatCompletionsURL(baseURL)
	}
	if model := configValue(modelEnv); model != "" {
		cfg.model = model
	}
	if providerSort := configValue(providerSortEnv); providerSort != "" {
		cfg.providerSort = normalizeProviderSort(providerSort)
	}
	return cfg
}

func loadAPIKey() string {
	if key := configValue(apiKeyEnv); key != "" {
		return key
	}

	return embeddedAPIKey
}

func configValue(name string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return readEnvValueFromFile(appdir.EnvPath(), name)
}

func readEnvValueFromFile(path, name string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		line = strings.TrimPrefix(line, "export ")
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if k, v, ok := strings.Cut(line, "="); ok && strings.TrimSpace(k) == name {
			return strings.Trim(strings.TrimSpace(v), `"'`)
		}
	}
	return ""
}

func normalizeChatCompletionsURL(baseURL string) string {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if strings.HasSuffix(baseURL, "/chat/completions") {
		return baseURL
	}
	return baseURL + "/chat/completions"
}

func normalizeProviderSort(providerSort string) string {
	providerSort = strings.TrimSpace(providerSort)
	switch strings.ToLower(providerSort) {
	case "none", "off", "default":
		return ""
	default:
		return providerSort
	}
}

func Spellcheck(typedWords []string) (*Result, error) {
	cfg := loadConfig()
	if cfg.apiKey == "" {
		return nil, fmt.Errorf("%s not set (env or %s)", apiKeyEnv, appdir.EnvPath())
	}
	return spellcheckWithConfig(typedWords, cfg, http.DefaultClient)
}

func SpellcheckRequestInfo(typedWords []string) RequestInfo {
	return requestInfo(typedWords, loadConfig())
}

func requestInfo(typedWords []string, cfg config) RequestInfo {
	return RequestInfo{
		Provider:            providerName,
		BaseURL:             cfg.baseURL,
		Model:               cfg.model,
		ProviderSort:        cfg.providerSort,
		SystemPrompt:        systemPrompt,
		UserPrompt:          strings.Join(typedWords, " "),
		Temperature:         0,
		TopP:                0.2,
		MaxCompletionTokens: maxCompletionTokens,
	}
}

func spellcheck(typedWords []string, apiKey, baseURL string, client *http.Client) (*Result, error) {
	cfg := defaultConfig()
	cfg.apiKey = apiKey
	cfg.baseURL = normalizeChatCompletionsURL(baseURL)
	return spellcheckWithConfig(typedWords, cfg, client)
}

func spellcheckWithConfig(typedWords []string, cfg config, client *http.Client) (*Result, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	requestInfo := requestInfo(typedWords, cfg)

	reqBody := chatRequest{
		Model:       requestInfo.Model,
		Temperature: requestInfo.Temperature,
		TopP:        requestInfo.TopP,
		MaxTokens:   requestInfo.MaxCompletionTokens,
		Provider:    providerPreferenceForSort(requestInfo.ProviderSort),
		Messages: []chatMessage{
			{Role: "system", Content: requestInfo.SystemPrompt},
			{Role: "user", Content: requestInfo.UserPrompt},
		},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.baseURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.apiKey)
	req.Header.Set("X-Title", "monke")

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

func providerPreferenceForSort(providerSort string) *providerPreference {
	if providerSort == "" {
		return nil
	}
	return &providerPreference{Sort: providerSort}
}
