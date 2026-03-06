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
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
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

const systemPrompt = "Fix any spelling and grammar errors in the following text. Return only the corrected text with no explanation. Preserve the original formatting and case."

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

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	input := strings.Join(typedWords, " ")

	reqBody := chatRequest{
		Model:       "openai/gpt-oss-120b",
		Temperature: 0,
		Messages: []chatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: input},
		},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.groq.com/openai/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := http.DefaultClient.Do(req)
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
