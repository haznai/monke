package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Correction struct {
	Index    int
	Original string
	Fixed    string
}

type Result struct {
	CorrectedWords []string
	Corrections    []Correction
}

type chatRequest struct {
	Model              string        `json:"model"`
	Messages           []chatMessage `json:"messages"`
	Temperature        float64       `json:"temperature"`
	TopP               float64       `json:"top_p"`
	MaxCompletionToks  int           `json:"max_completion_tokens"`
	ReasoningEffort    string        `json:"reasoning_effort"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

const (
	systemPrompt = "You are a spellchecker. Fix spelling errors in the text below. Output ONLY the corrected text, nothing else. Do not change capitalization, punctuation, or word count. Do not add or remove words."
	defaultURL   = "https://api.groq.com/openai/v1/chat/completions"
)

func loadAPIKey() string {
	if key := os.Getenv("GROQ_API_KEY"); key != "" {
		return key
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	f, err := os.Open(filepath.Join(home, ".monkeytype-tui", ".env"))
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
		return nil, fmt.Errorf("GROQ_API_KEY not set (env or ~/.monkeytype-tui/.env)")
	}
	return spellcheck(typedWords, apiKey, defaultURL, http.DefaultClient)
}

func spellcheck(typedWords []string, apiKey, baseURL string, client *http.Client) (*Result, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	input := strings.Join(typedWords, " ")

	reqBody := chatRequest{
		Model:             "openai/gpt-oss-20b",
		Temperature:       0,
		TopP:              0.2,
		MaxCompletionToks: 8192,
		ReasoningEffort:   "low",
		Messages: []chatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: input},
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
		return nil, fmt.Errorf("api returned %d", resp.StatusCode)
	}

	var chatResp chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	corrected := strings.TrimSpace(chatResp.Choices[0].Message.Content)
	correctedWords := strings.Fields(corrected)

	// Safety valve: if word count doesn't match, return original unchanged
	if len(correctedWords) != len(typedWords) {
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
		CorrectedWords: correctedWords,
		Corrections:    corrections,
	}, nil
}
