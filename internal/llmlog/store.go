package llmlog

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hazn/monkeytype-tui/internal/llm"

	_ "modernc.org/sqlite"
)

const schemaVersion = 1

type TestContext struct {
	Mode        string
	ModeValue   int
	WordList    string
	NgramType   string
	NgramScope  int
	NgramLesson int
	NgramTotal  int

	DurationSecs float64
	WPM          float64
	RawWPM       float64
	CorrectedWPM float64
	Accuracy     float64
	Consistency  float64
}

type RecordInput struct {
	CreatedAt time.Time
	Request   llm.RequestInfo
	Test      TestContext

	ResponseID       string
	ResponseModel    string
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int

	TargetWords       []string
	TypedWords        []string
	CorrectedWords    []string
	RawCorrectedText  string
	WordCountMismatch bool

	Latency time.Duration
	Err     error
}

type Record struct {
	SchemaVersion int
	CreatedAt     time.Time

	Mode        string
	ModeValue   int
	WordList    string
	NgramType   string
	NgramScope  int
	NgramLesson int
	NgramTotal  int

	Provider            string
	BaseURL             string
	Model               string
	SystemPrompt        string
	UserPrompt          string
	Temperature         float64
	TopP                float64
	MaxCompletionTokens int
	ReasoningEffort     string
	ResponseID          string
	ResponseModel       string
	PromptTokens        int
	CompletionTokens    int
	TotalTokens         int

	TargetText       string
	TypedText        string
	CorrectedText    string
	RawCorrectedText string
	TargetWordsJSON  string
	TypedWordsJSON   string
	CorrectedJSON    string

	DurationSecs    float64
	WPM             float64
	RawWPM          float64
	CorrectedWPM    float64
	LLMWPM          float64
	Accuracy        float64
	Consistency     float64
	CorrectionDelta float64

	TypedCorrectCount      int
	TypedWrongCount        int
	CorrectAfterLLMCount   int
	LLMFixedCount          int
	LLMChangedCount        int
	LLMMangledCorrectCount int
	StillWrongCount        int
	TargetWordCount        int
	TypedWordCount         int
	CorrectedWordCount     int
	TargetCharCount        int
	TypedCharCount         int
	CorrectedCharCount     int
	WordCountMismatch      bool
	AllCorrectAfterLLM     bool

	LatencyMS int64
	Error     string
}

type Store struct {
	path string
}

func NewStore(path string) *Store {
	return &Store{path: path}
}

func (s *Store) Path() string {
	return s.path
}

func NewRecord(input RecordInput) (Record, error) {
	createdAt := input.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now()
	}
	createdAt = createdAt.UTC()

	correctedWordsForMetrics := input.CorrectedWords
	if len(correctedWordsForMetrics) == 0 {
		correctedWordsForMetrics = input.TypedWords
	}

	targetJSON, err := wordsJSON(input.TargetWords)
	if err != nil {
		return Record{}, fmt.Errorf("encode target words: %w", err)
	}
	typedJSON, err := wordsJSON(input.TypedWords)
	if err != nil {
		return Record{}, fmt.Errorf("encode typed words: %w", err)
	}
	correctedJSON, err := wordsJSON(input.CorrectedWords)
	if err != nil {
		return Record{}, fmt.Errorf("encode corrected words: %w", err)
	}

	metrics := countOutcomes(input.TargetWords, input.TypedWords, correctedWordsForMetrics, input.Test.DurationSecs)
	errorText := ""
	if input.Err != nil {
		errorText = input.Err.Error()
	}

	correctedText := strings.Join(input.CorrectedWords, " ")
	wordCountMismatch := input.WordCountMismatch
	if len(input.CorrectedWords) > 0 && len(input.CorrectedWords) != len(input.TypedWords) {
		wordCountMismatch = true
	}

	return Record{
		SchemaVersion: schemaVersion,
		CreatedAt:     createdAt,

		Mode:        input.Test.Mode,
		ModeValue:   input.Test.ModeValue,
		WordList:    input.Test.WordList,
		NgramType:   input.Test.NgramType,
		NgramScope:  input.Test.NgramScope,
		NgramLesson: input.Test.NgramLesson,
		NgramTotal:  input.Test.NgramTotal,

		Provider:            input.Request.Provider,
		BaseURL:             input.Request.BaseURL,
		Model:               input.Request.Model,
		SystemPrompt:        input.Request.SystemPrompt,
		UserPrompt:          input.Request.UserPrompt,
		Temperature:         input.Request.Temperature,
		TopP:                input.Request.TopP,
		MaxCompletionTokens: input.Request.MaxCompletionTokens,
		ReasoningEffort:     input.Request.ReasoningEffort,
		ResponseID:          input.ResponseID,
		ResponseModel:       input.ResponseModel,
		PromptTokens:        input.PromptTokens,
		CompletionTokens:    input.CompletionTokens,
		TotalTokens:         input.TotalTokens,

		TargetText:       strings.Join(input.TargetWords, " "),
		TypedText:        strings.Join(input.TypedWords, " "),
		CorrectedText:    correctedText,
		RawCorrectedText: input.RawCorrectedText,
		TargetWordsJSON:  targetJSON,
		TypedWordsJSON:   typedJSON,
		CorrectedJSON:    correctedJSON,

		DurationSecs:    input.Test.DurationSecs,
		WPM:             input.Test.WPM,
		RawWPM:          input.Test.RawWPM,
		CorrectedWPM:    input.Test.CorrectedWPM,
		LLMWPM:          metrics.LLMWPM,
		Accuracy:        input.Test.Accuracy,
		Consistency:     input.Test.Consistency,
		CorrectionDelta: input.Test.CorrectedWPM - input.Test.WPM,

		TypedCorrectCount:      metrics.TypedCorrectCount,
		TypedWrongCount:        metrics.TypedWrongCount,
		CorrectAfterLLMCount:   metrics.CorrectAfterLLMCount,
		LLMFixedCount:          metrics.LLMFixedCount,
		LLMChangedCount:        metrics.LLMChangedCount,
		LLMMangledCorrectCount: metrics.LLMMangledCorrectCount,
		StillWrongCount:        metrics.StillWrongCount,
		TargetWordCount:        len(input.TargetWords),
		TypedWordCount:         len(input.TypedWords),
		CorrectedWordCount:     len(input.CorrectedWords),
		TargetCharCount:        joinedLen(input.TargetWords),
		TypedCharCount:         joinedLen(input.TypedWords),
		CorrectedCharCount:     joinedLen(input.CorrectedWords),
		WordCountMismatch:      wordCountMismatch,
		AllCorrectAfterLLM:     metrics.AllCorrectAfterLLM,

		LatencyMS: input.Latency.Milliseconds(),
		Error:     errorText,
	}, nil
}

func (s *Store) Save(record Record) error {
	if s == nil || s.path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return fmt.Errorf("create llm log dir: %w", err)
	}

	db, err := sql.Open("sqlite", s.path)
	if err != nil {
		return fmt.Errorf("open llm log db: %w", err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)

	if err := initSchema(db); err != nil {
		return err
	}
	if err := insertRecord(db, record); err != nil {
		return fmt.Errorf("insert llm call: %w", err)
	}
	return nil
}

func initSchema(db *sql.DB) error {
	pragmas := []string{
		"PRAGMA busy_timeout = 5000",
		"PRAGMA journal_mode = WAL",
		"PRAGMA foreign_keys = ON",
	}
	for _, pragma := range pragmas {
		if _, err := db.Exec(pragma); err != nil {
			return fmt.Errorf("apply %s: %w", pragma, err)
		}
	}

	_, err := db.Exec(`
CREATE TABLE IF NOT EXISTS llm_calls (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	schema_version INTEGER NOT NULL,
	created_at TEXT NOT NULL,
	mode TEXT NOT NULL,
	mode_value INTEGER NOT NULL,
	word_list TEXT NOT NULL,
	ngram_type TEXT NOT NULL,
	ngram_scope INTEGER NOT NULL,
	ngram_lesson INTEGER NOT NULL,
	ngram_total INTEGER NOT NULL,
	provider TEXT NOT NULL,
	base_url TEXT NOT NULL,
	model TEXT NOT NULL,
	system_prompt TEXT NOT NULL,
	user_prompt TEXT NOT NULL,
	temperature REAL NOT NULL,
	top_p REAL NOT NULL,
	max_completion_tokens INTEGER NOT NULL,
	reasoning_effort TEXT NOT NULL,
	response_id TEXT NOT NULL,
	response_model TEXT NOT NULL,
	prompt_tokens INTEGER NOT NULL,
	completion_tokens INTEGER NOT NULL,
	total_tokens INTEGER NOT NULL,
	target_text TEXT NOT NULL,
	typed_text TEXT NOT NULL,
	corrected_text TEXT NOT NULL,
	raw_corrected_text TEXT NOT NULL,
	target_words_json TEXT NOT NULL,
	typed_words_json TEXT NOT NULL,
	corrected_words_json TEXT NOT NULL,
	duration_secs REAL NOT NULL,
	wpm REAL NOT NULL,
	raw_wpm REAL NOT NULL,
	corrected_wpm REAL NOT NULL,
	llm_wpm REAL NOT NULL,
	accuracy REAL NOT NULL,
	consistency REAL NOT NULL,
	correction_delta REAL NOT NULL,
	typed_correct_count INTEGER NOT NULL,
	typed_wrong_count INTEGER NOT NULL,
	correct_after_llm_count INTEGER NOT NULL,
	llm_fixed_count INTEGER NOT NULL,
	llm_changed_count INTEGER NOT NULL,
	llm_mangled_correct_count INTEGER NOT NULL,
	still_wrong_count INTEGER NOT NULL,
	target_word_count INTEGER NOT NULL,
	typed_word_count INTEGER NOT NULL,
	corrected_word_count INTEGER NOT NULL,
	target_char_count INTEGER NOT NULL,
	typed_char_count INTEGER NOT NULL,
	corrected_char_count INTEGER NOT NULL,
	word_count_mismatch INTEGER NOT NULL,
	all_correct_after_llm INTEGER NOT NULL,
	latency_ms INTEGER NOT NULL,
	error TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS llm_calls_created_at_idx ON llm_calls(created_at);
CREATE INDEX IF NOT EXISTS llm_calls_model_idx ON llm_calls(model);
CREATE INDEX IF NOT EXISTS llm_calls_mode_idx ON llm_calls(mode);
`)
	if err != nil {
		return fmt.Errorf("init llm log schema: %w", err)
	}
	return nil
}

func insertRecord(db *sql.DB, record Record) error {
	_, err := db.Exec(`
INSERT INTO llm_calls (
	schema_version, created_at, mode, mode_value, word_list, ngram_type, ngram_scope, ngram_lesson, ngram_total,
	provider, base_url, model, system_prompt, user_prompt, temperature, top_p, max_completion_tokens, reasoning_effort,
	response_id, response_model, prompt_tokens, completion_tokens, total_tokens,
	target_text, typed_text, corrected_text, raw_corrected_text, target_words_json, typed_words_json, corrected_words_json,
	duration_secs, wpm, raw_wpm, corrected_wpm, llm_wpm, accuracy, consistency, correction_delta,
	typed_correct_count, typed_wrong_count, correct_after_llm_count, llm_fixed_count, llm_changed_count,
	llm_mangled_correct_count, still_wrong_count, target_word_count, typed_word_count, corrected_word_count,
	target_char_count, typed_char_count, corrected_char_count, word_count_mismatch, all_correct_after_llm,
	latency_ms, error
) VALUES (
	?, ?, ?, ?, ?, ?, ?, ?, ?,
	?, ?, ?, ?, ?, ?, ?, ?, ?,
	?, ?, ?, ?, ?,
	?, ?, ?, ?, ?, ?, ?,
	?, ?, ?, ?, ?, ?, ?, ?,
	?, ?, ?, ?, ?,
	?, ?, ?, ?, ?,
	?, ?, ?, ?, ?,
	?, ?
)`,
		record.SchemaVersion,
		record.CreatedAt.Format(time.RFC3339Nano),
		record.Mode,
		record.ModeValue,
		record.WordList,
		record.NgramType,
		record.NgramScope,
		record.NgramLesson,
		record.NgramTotal,
		record.Provider,
		record.BaseURL,
		record.Model,
		record.SystemPrompt,
		record.UserPrompt,
		record.Temperature,
		record.TopP,
		record.MaxCompletionTokens,
		record.ReasoningEffort,
		record.ResponseID,
		record.ResponseModel,
		record.PromptTokens,
		record.CompletionTokens,
		record.TotalTokens,
		record.TargetText,
		record.TypedText,
		record.CorrectedText,
		record.RawCorrectedText,
		record.TargetWordsJSON,
		record.TypedWordsJSON,
		record.CorrectedJSON,
		record.DurationSecs,
		record.WPM,
		record.RawWPM,
		record.CorrectedWPM,
		record.LLMWPM,
		record.Accuracy,
		record.Consistency,
		record.CorrectionDelta,
		record.TypedCorrectCount,
		record.TypedWrongCount,
		record.CorrectAfterLLMCount,
		record.LLMFixedCount,
		record.LLMChangedCount,
		record.LLMMangledCorrectCount,
		record.StillWrongCount,
		record.TargetWordCount,
		record.TypedWordCount,
		record.CorrectedWordCount,
		record.TargetCharCount,
		record.TypedCharCount,
		record.CorrectedCharCount,
		boolInt(record.WordCountMismatch),
		boolInt(record.AllCorrectAfterLLM),
		record.LatencyMS,
		record.Error,
	)
	return err
}

type outcomeMetrics struct {
	TypedCorrectCount      int
	TypedWrongCount        int
	CorrectAfterLLMCount   int
	LLMFixedCount          int
	LLMChangedCount        int
	LLMMangledCorrectCount int
	StillWrongCount        int
	AllCorrectAfterLLM     bool
	LLMWPM                 float64
}

func countOutcomes(targetWords, typedWords, correctedWords []string, durationSecs float64) outcomeMetrics {
	var metrics outcomeMetrics
	for i, target := range targetWords {
		typed := wordAt(typedWords, i)
		corrected := wordAt(correctedWords, i)
		typedCorrect := typed == target
		correctedCorrect := corrected == target

		if typedCorrect {
			metrics.TypedCorrectCount++
		} else {
			metrics.TypedWrongCount++
		}
		if typed != corrected {
			metrics.LLMChangedCount++
		}
		if typedCorrect || correctedCorrect {
			metrics.CorrectAfterLLMCount++
			metrics.LLMWPM += float64(len(target))
			if i < len(targetWords)-1 {
				metrics.LLMWPM++
			}
		}
		if !typedCorrect && correctedCorrect {
			metrics.LLMFixedCount++
		}
		if typedCorrect && corrected != target {
			metrics.LLMMangledCorrectCount++
		}
		if !typedCorrect && !correctedCorrect {
			metrics.StillWrongCount++
		}
	}

	if durationSecs > 0 {
		metrics.LLMWPM = (metrics.LLMWPM / 5.0) / (durationSecs / 60.0)
	}
	metrics.AllCorrectAfterLLM = len(targetWords) > 0 && metrics.CorrectAfterLLMCount == len(targetWords)
	return metrics
}

func wordsJSON(words []string) (string, error) {
	if words == nil {
		words = []string{}
	}
	encoded, err := json.Marshal(words)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

func joinedLen(words []string) int {
	return len(strings.Join(words, " "))
}

func wordAt(words []string, i int) string {
	if i < 0 || i >= len(words) {
		return ""
	}
	return words[i]
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
