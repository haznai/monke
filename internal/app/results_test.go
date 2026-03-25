package app

import (
	"math"
	"testing"
	"time"

	"github.com/hazn/monkeytype-tui/internal/llm"
	"github.com/hazn/monkeytype-tui/internal/stats"
)

func makeResultsModel(targetWords, typedWords, correctedWords []string) ResultsModel {
	return ResultsModel{
		targetWords: targetWords,
		typedWords:  typedWords,
		llmResult:   &llm.Result{CorrectedWords: correctedWords},
		result: stats.TestResult{
			Duration: 10 * time.Second,
		},
	}
}

func TestLLMMatchCount_AllTypedCorrectly(t *testing.T) {
	m := makeResultsModel(
		[]string{"the", "quick", "fox"},
		[]string{"the", "quick", "fox"},
		[]string{"the", "quick", "fox"}, // LLM doesn't change anything
	)
	allCorrect, count := m.llmMatchCount()
	if !allCorrect {
		t.Error("should be allCorrect when everything typed right")
	}
	if count != 3 {
		t.Errorf("count = %d, want 3", count)
	}
}

func TestLLMMatchCount_LLMFixesAllTypos(t *testing.T) {
	m := makeResultsModel(
		[]string{"the", "quick", "fox"},
		[]string{"teh", "quikc", "fxo"},
		[]string{"the", "quick", "fox"}, // LLM fixes all
	)
	allCorrect, count := m.llmMatchCount()
	if !allCorrect {
		t.Error("should be allCorrect when LLM fixes everything")
	}
	if count != 3 {
		t.Errorf("count = %d, want 3", count)
	}
}

func TestLLMMatchCount_LLMFixesSome(t *testing.T) {
	m := makeResultsModel(
		[]string{"the", "quick", "fox"},
		[]string{"teh", "qqqq", "fxo"},
		[]string{"the", "qqqq", "fox"}, // LLM fixes 2, can't fix "qqqq"
	)
	allCorrect, count := m.llmMatchCount()
	if allCorrect {
		t.Error("should NOT be allCorrect when LLM can't fix one")
	}
	if count != 2 {
		t.Errorf("count = %d, want 2", count)
	}
}

// Regression test: the LLM sometimes "corrects" words that were already right.
// e.g. user types "and" but LLM capitalizes it to "And", or adds punctuation.
// The word should still count as correct because the user typed it right.
func TestLLMMatchCount_LLMManglesCorrectlyTypedWord(t *testing.T) {
	m := makeResultsModel(
		[]string{"all", "we", "know", "nothing.", "and", "wisdom."},
		[]string{"all", "we", "know", "nothing.", "and", "wisdom."},
		[]string{"all", "we", "know", "nothing.", "And", "wisdom."}, // LLM capitalizes "and"
	)
	allCorrect, count := m.llmMatchCount()
	if !allCorrect {
		t.Error("should be allCorrect: user typed everything right, LLM mangling is irrelevant")
	}
	if count != 6 {
		t.Errorf("count = %d, want 6", count)
	}
}

func TestLLMMatchCount_MixedMangledAndFixed(t *testing.T) {
	// "height" was mistyped as "heigh" -> LLM fixes to "height" (good)
	// "and" was typed correctly -> LLM mangles to "And" (should still count)
	// "human" was mistyped as "huan" -> LLM fixes to "human" (good)
	m := makeResultsModel(
		[]string{"the", "height", "of", "human", "and", "wisdom."},
		[]string{"the", "heigh", "of", "huan", "and", "wisdom."},
		[]string{"the", "height", "of", "human", "And", "wisdom."},
	)
	allCorrect, count := m.llmMatchCount()
	if !allCorrect {
		t.Error("should be allCorrect: 4 typed right, 2 LLM-fixed, 'and' mangled but typed right")
	}
	if count != 6 {
		t.Errorf("count = %d, want 6", count)
	}
}

func TestLLMMatchCount_LLMManglesAndCantFix(t *testing.T) {
	// "and" typed correctly but LLM mangles -> still correct (typed right)
	// "heigh" mistyped, LLM can't fix -> wrong
	m := makeResultsModel(
		[]string{"the", "height", "and"},
		[]string{"the", "heigh", "and"},
		[]string{"the", "heigh", "And"}, // LLM can't fix "heigh", mangles "and"
	)
	allCorrect, count := m.llmMatchCount()
	if allCorrect {
		t.Error("should NOT be allCorrect: 'height' still wrong after LLM")
	}
	if count != 2 {
		t.Errorf("count = %d, want 2 (the + and)", count)
	}
}

func TestLLMMatchCount_QuotePunctuation(t *testing.T) {
	// Quotes have punctuation attached to words. LLM might strip or alter it.
	m := makeResultsModel(
		[]string{"it's", "okay,", "really."},
		[]string{"it's", "okay,", "really."},
		[]string{"It's", "okay,", "really."}, // LLM capitalizes first word
	)
	allCorrect, count := m.llmMatchCount()
	if !allCorrect {
		t.Error("should be allCorrect: user typed everything right including punctuation")
	}
	if count != 3 {
		t.Errorf("count = %d, want 3", count)
	}
}

func TestCalcLLMWPM_IncludesCorrectlyTypedWords(t *testing.T) {
	// 3 words, 10 seconds. LLM mangles "and" but user typed it right.
	// All 3 words should count: "the"(3) + " " + "and"(3) + " " + "fox"(3) = 11 chars
	// WPM = (11/5) / (10/60) = 2.2 / 0.1667 = 13.2
	m := makeResultsModel(
		[]string{"the", "and", "fox"},
		[]string{"the", "and", "fox"},
		[]string{"the", "And", "fox"}, // LLM mangles "and"
	)
	minutes := m.result.Duration.Seconds() / 60.0
	wpm := m.calcLLMWPM(3, minutes)
	expected := (11.0 / 5.0) / minutes
	if math.Abs(wpm-expected) > 0.1 {
		t.Errorf("wpm = %.1f, want %.1f", wpm, expected)
	}
}

func TestCalcLLMWPM_ExcludesStillWrongWords(t *testing.T) {
	// "heigh" not fixed by LLM, should not count.
	// Only "the"(3) + " " + "fox"(3) = 7 chars count
	// WPM = (7/5) / (10/60) = 1.4 / 0.1667 = 8.4
	m := makeResultsModel(
		[]string{"the", "height", "fox"},
		[]string{"the", "heigh", "fox"},
		[]string{"the", "heigh", "fox"}, // LLM can't fix "heigh"
	)
	minutes := m.result.Duration.Seconds() / 60.0
	wpm := m.calcLLMWPM(2, minutes)
	// "the"(3) + space(1) = 4, skip "height", "fox"(3) no trailing space = 3. Total = 7
	expected := (7.0 / 5.0) / minutes
	if math.Abs(wpm-expected) > 0.1 {
		t.Errorf("wpm = %.1f, want %.1f", wpm, expected)
	}
}

func TestCalcLLMWPM_ZeroMinutes(t *testing.T) {
	m := makeResultsModel(
		[]string{"test"},
		[]string{"test"},
		[]string{"test"},
	)
	m.result.Duration = 0
	wpm := m.calcLLMWPM(1, 0)
	if wpm != 0 {
		t.Errorf("wpm = %.1f, want 0 for zero duration", wpm)
	}
}
