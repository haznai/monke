package app

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/hazn/monkeytype-tui/internal/history"
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

	// Skip "beta" entirely (space with empty input)
	engine.Space()

	// Type "gamma" correctly
	for _, ch := range "gamma" {
		engine.TypeChar(ch)
	}
	engine.Space()

	if !engine.IsFinished() {
		t.Fatal("should be finished")
	}

	ws := engine.Words()
	if !ws[0].Correct {
		t.Error("alpha should be correct")
	}
	if ws[1].Correct {
		t.Error("beta should be incorrect (skipped)")
	}
	if ws[1].Typed != "" {
		t.Errorf("skipped word should have empty typed, got %q", ws[1].Typed)
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
	engine.Backspace()    // fix it
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
