package app

import (
	"database/sql"
	"math"
	"path/filepath"
	"testing"
	"time"

	"github.com/hazn/monkeytype-tui/internal/history"
	"github.com/hazn/monkeytype-tui/internal/llm"
	"github.com/hazn/monkeytype-tui/internal/llmlog"
	"github.com/hazn/monkeytype-tui/internal/stats"
	"github.com/hazn/monkeytype-tui/internal/typing"
)

// Integration test: simulates a full typing test from start to finish,
// verifying the engine -> stats -> history pipeline works end to end.

func TestFullTypingFlow_AllCorrect(t *testing.T) {
	words := []string{"the", "quick", "brown", "fox", "jumps"}
	engine := typing.NewEngine(words)

	// Simulate typing each word correctly
	for i, word := range words {
		for _, ch := range word {
			engine.TypeChar(ch)
		}
		engine.Space()

		ws := engine.Words()
		if !ws[i].Correct {
			t.Fatalf("word %d (%q) should be correct", i, word)
		}
	}

	if !engine.IsFinished() {
		t.Fatal("engine should be finished after all words submitted")
	}

	// Build stats
	wordStates := engine.Words()
	var wordResults []stats.WordResult
	for _, w := range wordStates {
		wordResults = append(wordResults, stats.WordResult{
			Target:  w.Target,
			Typed:   w.Typed,
			Correct: w.Correct,
		})
	}

	result := stats.Calculate(stats.TestInput{
		Words:    wordResults,
		Duration: 5 * time.Second,
	})

	if !result.Passed {
		t.Error("result should be passed when all words are correct")
	}
	if result.CorrectWords != 5 {
		t.Errorf("expected 5 correct words, got %d", result.CorrectWords)
	}
	if result.Accuracy != 100.0 {
		t.Errorf("expected 100%% accuracy, got %.1f%%", result.Accuracy)
	}
	if result.WPM <= 0 {
		t.Error("WPM should be positive")
	}
	if result.WPM != result.RawWPM {
		t.Errorf("WPM (%.1f) should equal RawWPM (%.1f) when all correct", result.WPM, result.RawWPM)
	}
	if result.CorrectionDelta != 0 {
		t.Errorf("CorrectionDelta should be 0 when all correct, got %.1f", result.CorrectionDelta)
	}

	// Save to history
	dir := t.TempDir()
	store := history.NewStore(filepath.Join(dir, "history.json"))
	_ = store.Load()

	record := history.TestRecord{
		Timestamp:    time.Now(),
		Mode:         "words",
		ModeValue:    5,
		WordList:     "english_1k",
		WPM:          result.WPM,
		RawWPM:       result.RawWPM,
		CorrectedWPM: result.CorrectedWPM,
		Accuracy:     result.Accuracy,
		CorrectWords: result.CorrectWords,
		TotalWords:   result.TotalWords,
		Passed:       result.Passed,
		DurationSecs: result.Duration.Seconds(),
	}

	if !store.IsPersonalBest(record) {
		t.Error("first test should always be a personal best")
	}

	if err := store.Save(record); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	if store.TotalTests() != 1 {
		t.Errorf("expected 1 test, got %d", store.TotalTests())
	}
}

func TestFullTypingFlow_WithErrors(t *testing.T) {
	words := []string{"hello", "world", "test"}
	engine := typing.NewEngine(words)

	// Type "hello" correctly
	for _, ch := range "hello" {
		engine.TypeChar(ch)
	}
	engine.Space()

	// Type "worlt" (wrong) for "world"
	for _, ch := range "worlt" {
		engine.TypeChar(ch)
	}
	engine.Space()

	// Type "te", delete word, retype "test" correctly
	engine.TypeChar('t')
	engine.TypeChar('e')
	engine.DeleteWord() // Ctrl+W / Cmd+Backspace
	if engine.CurrentInput() != "" {
		t.Fatal("DeleteWord should clear input")
	}
	for _, ch := range "test" {
		engine.TypeChar(ch)
	}
	engine.Space()

	if !engine.IsFinished() {
		t.Fatal("engine should be finished")
	}

	ws := engine.Words()
	if !ws[0].Correct {
		t.Error("'hello' should be correct")
	}
	if ws[1].Correct {
		t.Error("'worlt' for 'world' should be incorrect")
	}
	if !ws[2].Correct {
		t.Error("'test' should be correct after delete-word and retype")
	}

	var wordResults []stats.WordResult
	for _, w := range ws {
		wordResults = append(wordResults, stats.WordResult{
			Target:  w.Target,
			Typed:   w.Typed,
			Correct: w.Correct,
		})
	}

	result := stats.Calculate(stats.TestInput{
		Words:    wordResults,
		Duration: 3 * time.Second,
	})

	if result.Passed {
		t.Error("should NOT be passed with incorrect words")
	}
	if result.CorrectWords != 2 {
		t.Errorf("expected 2 correct, got %d", result.CorrectWords)
	}
	if result.WPM >= result.RawWPM {
		t.Error("WPM should be less than RawWPM when there are errors")
	}
	if result.CorrectedWPM <= result.WPM {
		t.Error("CorrectedWPM should be greater than WPM (LLM would fix the errors)")
	}
	if result.CorrectionDelta <= 0 {
		t.Error("CorrectionDelta should be positive (we gain WPM from LLM correction)")
	}
}

func TestFullTypingFlow_SkipWord(t *testing.T) {
	words := []string{"alpha", "beta", "gamma"}
	engine := typing.NewEngine(words)

	// Type "alpha" correctly
	for _, ch := range "alpha" {
		engine.TypeChar(ch)
	}
	engine.Space()

	// "Skip" beta by typing garbage past the threshold (threshold for "beta" = 2)
	engine.TypeChar('x')
	engine.TypeChar('y')
	engine.Space()

	// Type "gamma" correctly (last word, auto-finishes)
	for _, ch := range "gamma" {
		engine.TypeChar(ch)
	}

	if !engine.IsFinished() {
		t.Fatal("should be finished")
	}

	ws := engine.Words()
	if !ws[0].Correct {
		t.Error("alpha should be correct")
	}
	if ws[1].Correct {
		t.Error("beta should be incorrect (garbage typed)")
	}
	if ws[1].Typed != "xy" {
		t.Errorf("skipped word typed = %q, want 'xy'", ws[1].Typed)
	}
	if !ws[2].Correct {
		t.Error("gamma should be correct")
	}
}

func TestFullTypingFlow_BackspaceCorrection(t *testing.T) {
	words := []string{"code"}
	engine := typing.NewEngine(words)

	// Type "co", then "x" (mistake), then backspace, then "d", "e"
	engine.TypeChar('c')
	engine.TypeChar('o')
	engine.TypeChar('x') // mistake
	engine.Backspace()   // fix it
	engine.TypeChar('d')
	engine.TypeChar('e')
	engine.Space()

	if !engine.IsFinished() {
		t.Fatal("should be finished")
	}

	ws := engine.Words()
	if !ws[0].Correct {
		t.Error("should be correct after backspace correction")
	}
	if ws[0].Typed != "code" {
		t.Errorf("typed should be 'code', got %q", ws[0].Typed)
	}
}

func TestFullTypingFlow_ResetAndRetype(t *testing.T) {
	words := []string{"one", "two"}
	engine := typing.NewEngine(words)

	// Type "one" wrong
	for _, ch := range "xxx" {
		engine.TypeChar(ch)
	}
	engine.Space()

	// Reset (Tab key)
	engine.Reset()

	if engine.IsStarted() {
		t.Error("should not be started after reset")
	}
	if engine.IsFinished() {
		t.Error("should not be finished after reset")
	}
	if engine.CurrentWordIndex() != 0 {
		t.Error("should be back to word 0")
	}

	// Now type both correctly
	for _, ch := range "one" {
		engine.TypeChar(ch)
	}
	engine.Space()
	for _, ch := range "two" {
		engine.TypeChar(ch)
	}
	engine.Space()

	ws := engine.Words()
	if !ws[0].Correct || !ws[1].Correct {
		t.Error("both words should be correct after reset and retype")
	}
}

func TestSaveLLMCallWritesTargetTypedCorrectedAndMetrics(t *testing.T) {
	path := filepath.Join(t.TempDir(), "llm-calls.sqlite3")
	store := llmlog.NewStore(path)
	result := stats.TestResult{
		WPM:          20,
		RawWPM:       40,
		CorrectedWPM: 60,
		Accuracy:     50,
		Consistency:  90,
		Duration:     6 * time.Second,
	}

	err := saveLLMCall(store, llmLogInput{
		started: time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC),
		latency: 75 * time.Millisecond,
		requestInfo: llm.RequestInfo{
			Provider:            "groq",
			BaseURL:             "https://example.test/chat",
			Model:               "test-model",
			SystemPrompt:        "fix it",
			UserPrompt:          "teh quick fxo",
			Temperature:         0,
			TopP:                0.2,
			MaxCompletionTokens: 512,
			ReasoningEffort:     "low",
		},
		config: TestConfig{
			Mode:     "words",
			Value:    25,
			WordList: "english_1k",
		},
		result:      result,
		targetWords: []string{"the", "quick", "fox"},
		typedWords:  []string{"teh", "quick", "fxo"},
		llmResult: &llm.Result{
			CorrectedWords:   []string{"the", "quick", "far"},
			RawCorrectedText: "the quick far",
			ResponseID:       "chatcmpl-test",
			ResponseModel:    "served-model",
			PromptTokens:     42,
			CompletionTokens: 9,
			TotalTokens:      51,
		},
	})
	if err != nil {
		t.Fatalf("saveLLMCall: %v", err)
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	var targetText, typedText, correctedText, model, prompt, responseID string
	var fixed, stillWrong, correctAfterLLM, totalTokens int
	var wpm, rawWPM, correctedWPM, llmWPM float64
	err = db.QueryRow(`
SELECT target_text, typed_text, corrected_text, model, system_prompt, response_id, total_tokens,
	llm_fixed_count, still_wrong_count, correct_after_llm_count,
	wpm, raw_wpm, corrected_wpm, llm_wpm
FROM llm_calls
`).Scan(&targetText, &typedText, &correctedText, &model, &prompt, &responseID, &totalTokens, &fixed, &stillWrong, &correctAfterLLM, &wpm, &rawWPM, &correctedWPM, &llmWPM)
	if err != nil {
		t.Fatalf("query llm_calls: %v", err)
	}
	if targetText != "the quick fox" || typedText != "teh quick fxo" || correctedText != "the quick far" {
		t.Fatalf("texts target=%q typed=%q corrected=%q", targetText, typedText, correctedText)
	}
	if model != "test-model" || prompt != "fix it" {
		t.Fatalf("request metadata model=%q prompt=%q", model, prompt)
	}
	if responseID != "chatcmpl-test" || totalTokens != 51 {
		t.Fatalf("response metadata responseID=%q totalTokens=%d", responseID, totalTokens)
	}
	if fixed != 1 || stillWrong != 1 || correctAfterLLM != 2 {
		t.Fatalf("counts fixed=%d stillWrong=%d correctAfterLLM=%d, want 1/1/2", fixed, stillWrong, correctAfterLLM)
	}
	if wpm != 20 || rawWPM != 40 || correctedWPM != 60 {
		t.Fatalf("typing WPMs = %.1f/%.1f/%.1f, want 20/40/60", wpm, rawWPM, correctedWPM)
	}
	if !approxFloat(llmWPM, 20) {
		t.Fatalf("llmWPM = %.2f, want 20", llmWPM)
	}
}

func TestNgramLessonRequires100WPMAndEveryWordCorrect(t *testing.T) {
	correctWords := []typing.WordState{
		{Target: "th", Typed: "th", Done: true, Correct: true},
		{Target: "he", Typed: "he", Done: true, Correct: true},
	}
	wrongWords := []typing.WordState{
		{Target: "th", Typed: "zz", Done: true, Correct: false},
		{Target: "he", Typed: "he", Done: true, Correct: true},
	}
	notDoneWords := []typing.WordState{
		{Target: "th", Typed: "th", Done: true, Correct: true},
		{Target: "he", Typed: "he", Done: false, Correct: true},
	}

	tests := []struct {
		name  string
		words []typing.WordState
		wpm   float64
		want  bool
	}{
		{name: "100 wpm and every word correct advances", words: correctWords, wpm: 100, want: true},
		{name: "99.9 wpm retries instead of rounding up", words: correctWords, wpm: 99.9, want: false},
		{name: "incorrect word retries even far above threshold", words: wrongWords, wpm: 200, want: false},
		{name: "unfinished word retries even if marked correct", words: notDoneWords, wpm: 200, want: false},
		{name: "empty lesson never passes", words: nil, wpm: 200, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ngramLessonPassed(tt.words, tt.wpm)
			if got != tt.want {
				t.Fatalf("ngramLessonPassed() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNgramLessonRetriesWhenAnyWordIsIncorrect(t *testing.T) {
	lessons := [][]string{
		{"th", "he"},
		{"in", "er"},
	}
	config := TestConfig{
		Mode:        "ngram",
		NgramLesson: 1,
		NgramTotal:  len(lessons),
	}
	typingModel := NewTypingModel(lessons[0], config, 80, 24)

	for _, ch := range "zz" {
		typingModel.engine.TypeChar(ch)
	}
	typingModel.engine.Space()
	for _, ch := range "he" {
		typingModel.engine.TypeChar(ch)
	}

	if !typingModel.engine.IsFinished() {
		t.Fatal("ngram attempt should be finished")
	}
	duration := typingModel.engine.ElapsedTime()
	wpm := float64(typingModel.engine.TotalTypedChars()) / 5.0 / duration.Minutes()
	if wpm < ngramWPMThreshold {
		t.Fatalf("test setup error: attempt WPM %.1f is below threshold, so retry would not prove the correctness gate", wpm)
	}

	words := typingModel.engine.Words()
	if words[0].Correct {
		t.Fatal("test setup error: first ngram word should be incorrect")
	}
	if !words[1].Correct {
		t.Fatal("test setup error: second ngram word should be correct")
	}

	m := Model{
		config:         config,
		typing:         &typingModel,
		ngramLessons:   lessons,
		ngramLessonIdx: 0,
	}

	updated, _ := m.finishTest()

	if updated.ngramLessonIdx != 0 {
		t.Fatalf("ngram lesson index = %d, want 0 to retry the failed lesson", updated.ngramLessonIdx)
	}
	if updated.config.NgramLesson != 1 {
		t.Fatalf("ngram lesson display = %d, want 1 to retry the failed lesson", updated.config.NgramLesson)
	}
	if updated.typing == nil {
		t.Fatal("typing model should be reset for retry")
	}
	retryWords := updated.typing.engine.Words()
	if len(retryWords) != len(lessons[0]) {
		t.Fatalf("retry lesson word count = %d, want %d", len(retryWords), len(lessons[0]))
	}
	for i, word := range retryWords {
		if word.Target != lessons[0][i] {
			t.Fatalf("retry word %d target = %q, want %q", i, word.Target, lessons[0][i])
		}
	}
}

func approxFloat(got, want float64) bool {
	return math.Abs(got-want) < 0.0001
}
