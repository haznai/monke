package llmlog

import (
	"database/sql"
	"errors"
	"math"
	"path/filepath"
	"testing"
	"time"

	"github.com/hazn/monkeytype-tui/internal/llm"
)

func testRequestInfo() llm.RequestInfo {
	return llm.RequestInfo{
		Provider:            "groq",
		BaseURL:             "https://example.test/chat",
		Model:               "test-model",
		SystemPrompt:        "fix spelling only",
		UserPrompt:          "teh and fxo jumps",
		Temperature:         0,
		TopP:                0.2,
		MaxCompletionTokens: 512,
		ReasoningEffort:     "low",
	}
}

func testContext() TestContext {
	return TestContext{
		Mode:         "words",
		ModeValue:    25,
		WordList:     "english_1k",
		DurationSecs: 6,
		WPM:          20,
		RawWPM:       40,
		CorrectedWPM: 50,
		Accuracy:     50,
		Consistency:  91.5,
	}
}

func TestNewRecordCountsLLMOutcomes(t *testing.T) {
	record, err := NewRecord(RecordInput{
		CreatedAt:        time.Date(2026, 5, 4, 12, 30, 0, 0, time.FixedZone("test", 2*60*60)),
		Request:          testRequestInfo(),
		ResponseID:       "chatcmpl-test",
		ResponseModel:    "served-model",
		PromptTokens:     42,
		CompletionTokens: 9,
		TotalTokens:      51,
		Test:             testContext(),
		TargetWords:      []string{"the", "and", "fox", "jumps"},
		TypedWords:       []string{"teh", "and", "fxo", "jumps"},
		CorrectedWords:   []string{"the", "AND", "far", "jumps"},
		RawCorrectedText: "the AND far jumps",
		Latency:          1234 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("NewRecord: %v", err)
	}

	if record.CreatedAt.Location() != time.UTC {
		t.Fatalf("CreatedAt location = %v, want UTC", record.CreatedAt.Location())
	}
	if record.TargetText != "the and fox jumps" {
		t.Fatalf("TargetText = %q", record.TargetText)
	}
	if record.TypedText != "teh and fxo jumps" {
		t.Fatalf("TypedText = %q", record.TypedText)
	}
	if record.CorrectedText != "the AND far jumps" {
		t.Fatalf("CorrectedText = %q", record.CorrectedText)
	}
	if record.TargetWordsJSON != `["the","and","fox","jumps"]` {
		t.Fatalf("TargetWordsJSON = %q", record.TargetWordsJSON)
	}
	if record.TypedCorrectCount != 2 {
		t.Fatalf("TypedCorrectCount = %d, want 2", record.TypedCorrectCount)
	}
	if record.TypedWrongCount != 2 {
		t.Fatalf("TypedWrongCount = %d, want 2", record.TypedWrongCount)
	}
	if record.CorrectAfterLLMCount != 3 {
		t.Fatalf("CorrectAfterLLMCount = %d, want 3", record.CorrectAfterLLMCount)
	}
	if record.LLMFixedCount != 1 {
		t.Fatalf("LLMFixedCount = %d, want 1", record.LLMFixedCount)
	}
	if record.LLMChangedCount != 3 {
		t.Fatalf("LLMChangedCount = %d, want 3", record.LLMChangedCount)
	}
	if record.LLMMangledCorrectCount != 1 {
		t.Fatalf("LLMMangledCorrectCount = %d, want 1", record.LLMMangledCorrectCount)
	}
	if record.StillWrongCount != 1 {
		t.Fatalf("StillWrongCount = %d, want 1", record.StillWrongCount)
	}
	if record.AllCorrectAfterLLM {
		t.Fatal("AllCorrectAfterLLM should be false")
	}
	if !approx(record.LLMWPM, 26) {
		t.Fatalf("LLMWPM = %.2f, want 26", record.LLMWPM)
	}
	if record.CorrectionDelta != 30 {
		t.Fatalf("CorrectionDelta = %.2f, want 30", record.CorrectionDelta)
	}
	if record.TargetCharCount != len("the and fox jumps") {
		t.Fatalf("TargetCharCount = %d", record.TargetCharCount)
	}
	if record.LatencyMS != 1234 {
		t.Fatalf("LatencyMS = %d, want 1234", record.LatencyMS)
	}
	if record.Model != "test-model" || record.SystemPrompt != "fix spelling only" || record.UserPrompt != "teh and fxo jumps" {
		t.Fatalf("request metadata not preserved: %#v", record)
	}
	if record.ResponseID != "chatcmpl-test" || record.ResponseModel != "served-model" {
		t.Fatalf("response metadata not preserved: %#v", record)
	}
	if record.PromptTokens != 42 || record.CompletionTokens != 9 || record.TotalTokens != 51 {
		t.Fatalf("token usage = %d/%d/%d, want 42/9/51", record.PromptTokens, record.CompletionTokens, record.TotalTokens)
	}
}

func TestNewRecordFallsBackToTypedWordsForErrorMetrics(t *testing.T) {
	record, err := NewRecord(RecordInput{
		Request:     testRequestInfo(),
		Test:        testContext(),
		TargetWords: []string{"the", "quick"},
		TypedWords:  []string{"teh", "quick"},
		Err:         errors.New("api returned 401"),
	})
	if err != nil {
		t.Fatalf("NewRecord: %v", err)
	}

	if record.Error != "api returned 401" {
		t.Fatalf("Error = %q", record.Error)
	}
	if record.CorrectedWordCount != 0 {
		t.Fatalf("CorrectedWordCount = %d, want 0 because the model produced no text", record.CorrectedWordCount)
	}
	if record.CorrectAfterLLMCount != 1 {
		t.Fatalf("CorrectAfterLLMCount = %d, want 1 from typed fallback", record.CorrectAfterLLMCount)
	}
	if record.StillWrongCount != 1 {
		t.Fatalf("StillWrongCount = %d, want 1", record.StillWrongCount)
	}
}

func TestStoreSaveCreatesSQLiteDatabaseAndPersistsRecord(t *testing.T) {
	path := filepath.Join(t.TempDir(), "logs", "llm-calls.sqlite3")
	store := NewStore(path)
	record, err := NewRecord(RecordInput{
		CreatedAt:      time.Date(2026, 5, 4, 12, 30, 0, 0, time.UTC),
		Request:        testRequestInfo(),
		Test:           testContext(),
		TargetWords:    []string{"the", "quick"},
		TypedWords:     []string{"teh", "quick"},
		CorrectedWords: []string{"the", "quick"},
		Latency:        50 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("NewRecord: %v", err)
	}

	if err := store.Save(record); err != nil {
		t.Fatalf("Save: %v", err)
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	var count int
	var model, typedText, targetJSON string
	var fixed, stillWrong, allCorrect int
	err = db.QueryRow(`
SELECT COUNT(*), model, typed_text, target_words_json, llm_fixed_count, still_wrong_count, all_correct_after_llm
FROM llm_calls
`).Scan(&count, &model, &typedText, &targetJSON, &fixed, &stillWrong, &allCorrect)
	if err != nil {
		t.Fatalf("query llm_calls: %v", err)
	}
	if count != 1 {
		t.Fatalf("count = %d, want 1", count)
	}
	if model != "test-model" {
		t.Fatalf("model = %q", model)
	}
	if typedText != "teh quick" {
		t.Fatalf("typed_text = %q", typedText)
	}
	if targetJSON != `["the","quick"]` {
		t.Fatalf("target_words_json = %q", targetJSON)
	}
	if fixed != 1 || stillWrong != 0 || allCorrect != 1 {
		t.Fatalf("outcome counts fixed=%d stillWrong=%d allCorrect=%d, want 1/0/1", fixed, stillWrong, allCorrect)
	}
}

func TestStoreSavePersistsErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "llm-calls.sqlite3")
	store := NewStore(path)
	record, err := NewRecord(RecordInput{
		Request:     testRequestInfo(),
		Test:        testContext(),
		TargetWords: []string{"hello"},
		TypedWords:  []string{"helo"},
		Err:         errors.New("api call: timeout"),
	})
	if err != nil {
		t.Fatalf("NewRecord: %v", err)
	}
	if err := store.Save(record); err != nil {
		t.Fatalf("Save: %v", err)
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	var got string
	if err := db.QueryRow(`SELECT error FROM llm_calls`).Scan(&got); err != nil {
		t.Fatalf("query error: %v", err)
	}
	if got != "api call: timeout" {
		t.Fatalf("error = %q", got)
	}
}

func approx(got, want float64) bool {
	return math.Abs(got-want) < 0.0001
}
