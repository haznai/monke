package stats

import (
	"math"
	"testing"
	"time"
)

const tolerance = 0.01

func approxEqual(a, b float64) bool {
	if a == 0 && b == 0 {
		return true
	}
	return math.Abs(a-b) < tolerance || math.Abs(a-b)/math.Max(math.Abs(a), math.Abs(b)) < tolerance
}

func assertApprox(t *testing.T, name string, got, want float64) {
	t.Helper()
	if !approxEqual(got, want) {
		t.Errorf("%s: got %.4f, want %.4f", name, got, want)
	}
}

// Helper to build WordResult slices quickly.
// Each pair is (target, typed). Correct is derived from target == typed.
func makeWords(pairs ...string) []WordResult {
	if len(pairs)%2 != 0 {
		panic("makeWords requires pairs of (target, typed)")
	}
	now := time.Now()
	var words []WordResult
	for i := 0; i < len(pairs); i += 2 {
		target, typed := pairs[i], pairs[i+1]
		words = append(words, WordResult{
			Target:    target,
			Typed:     typed,
			Correct:   target == typed,
			StartTime: now,
			EndTime:   now.Add(time.Second),
		})
	}
	return words
}

// charCount returns the total character count for a set of words (each word's
// length + 1 space), minus the trailing space on the last word.
// This is the counting convention: space after each word except the last.
func charCount(words []string) int {
	total := 0
	for i, w := range words {
		total += len(w)
		if i < len(words)-1 {
			total++ // space between words
		}
	}
	return total
}

func TestAllCorrect(t *testing.T) {
	// 5 words, all typed correctly, 6 seconds
	words := makeWords(
		"hello", "hello",
		"world", "world",
		"these", "these",
		"tests", "tests",
		"rock", "rock",
	)
	duration := 6 * time.Second
	minutes := 6.0 / 60.0 // 0.1

	// correct_chars: 5+5+5+5+4 = 24 chars + 4 spaces = 28
	// all_typed_chars: same = 28
	// total_target_chars: same = 28
	correctChars := float64(charCount([]string{"hello", "world", "these", "tests", "rock"})) // 28
	wantWPM := (correctChars / 5.0) / minutes                                                 // 56
	wantRawWPM := (correctChars / 5.0) / minutes                                              // 56
	wantCorrectedWPM := (correctChars / 5.0) / minutes                                        // 56

	result := Calculate(TestInput{Words: words, Duration: duration})

	assertApprox(t, "WPM", result.WPM, wantWPM)
	assertApprox(t, "RawWPM", result.RawWPM, wantRawWPM)
	assertApprox(t, "CorrectedWPM", result.CorrectedWPM, wantCorrectedWPM)
	assertApprox(t, "Accuracy", result.Accuracy, 100.0)
	assertApprox(t, "CorrectionDelta", result.CorrectionDelta, 0)
	if result.CorrectWords != 5 {
		t.Errorf("CorrectWords: got %d, want 5", result.CorrectWords)
	}
	if result.TotalWords != 5 {
		t.Errorf("TotalWords: got %d, want 5", result.TotalWords)
	}
	if !result.Passed {
		t.Error("Passed: got false, want true")
	}
}

func TestSomeIncorrect(t *testing.T) {
	// 5 words, 3 correct (hello, these, rock), 2 wrong (worlt, tesst), 6 seconds
	words := makeWords(
		"hello", "hello",
		"world", "worlt",
		"these", "these",
		"tests", "tesst",
		"rock", "rock",
	)
	duration := 6 * time.Second
	minutes := 6.0 / 60.0

	// correct words: hello(5), these(5), rock(4) = 14 chars + 2 spaces (hello->world, these->tests) = wait...
	// Spaces: between each pair of adjacent words. For 5 words there are 4 spaces.
	// But only the CORRECT words contribute to correct_chars, and the space
	// associated with each word is the space typed after it.
	//
	// The convention: for correct_chars, each correct word contributes
	// len(typed) + 1 (for the space after it), except the last word overall
	// which gets no trailing space. BUT a correct word might not be last.
	//
	// Actually, rethinking this: the space is between words. Every word except
	// the last gets +1 for the space that follows it. For correct_chars, we
	// sum len(typed)+1 for each correct word that isn't the last word, and
	// len(typed) for a correct last word.
	//
	// Wait, that's getting complicated. Let me think about this differently.
	// The "chars typed" for the full test is: sum of each word's len(typed)
	// plus (N-1) spaces. The spaces go between words. For correct_chars,
	// we need to figure out which characters are "correct".
	//
	// I think the simplest interpretation: a correct word contributes its
	// characters. Spaces between words are always typed. A space after a
	// correct word AND before the next word counts as correct if the current
	// word is correct.
	//
	// Actually, let me just define it pragmatically:
	// all_typed_chars = sum(len(typed)) + (N-1) for spaces
	// correct_chars = sum(len(typed) for correct words) + (number of correct words that aren't last) [spaces after correct words]
	//   ... actually no, if word i is correct but word i is not the last word,
	//   you get +1 for the space.
	//   But what about: correct word followed by incorrect word? The space is
	//   typed between them. That space is part of the input. If we attribute
	//   the space to the preceding word, then correct word -> +1 space regardless
	//   of next word.
	//
	// Simplest consistent rule matching the "hello" single-word example (5 chars, no space):
	// - Each word except the last contributes len + 1 (word chars + space)
	// - Last word contributes just len
	// - For correct_chars, only count the words where Correct == true
	//
	// With that rule:
	// Correct words: hello (pos 0, not last: 5+1=6), these (pos 2, not last: 5+1=6), rock (pos 4, last: 4)
	// correct_chars = 6 + 6 + 4 = 16
	// all_typed_chars = (5+1) + (5+1) + (5+1) + (5+1) + 4 = 28
	// total_target_chars = 28
	//
	// WPM = (16/5) / 0.1 = 3.2 / 0.1 = 32
	// RawWPM = (28/5) / 0.1 = 56
	// CorrectedWPM = (28/5) / 0.1 = 56
	// Accuracy = 16/28 * 100 = 57.142857...%
	// CorrectionDelta = 56 - 32 = 24

	allTyped := float64(charCount([]string{"hello", "worlt", "these", "tesst", "rock"}))       // 28
	totalTarget := float64(charCount([]string{"hello", "world", "these", "tests", "rock"}))     // 28
	_ = totalTarget

	// correct chars: hello at pos 0 (not last) = 5+1=6, these at pos 2 (not last) = 5+1=6, rock at pos 4 (last) = 4
	correctChars := 6.0 + 6.0 + 4.0 // 16

	wantWPM := (correctChars / 5.0) / minutes
	wantRawWPM := (allTyped / 5.0) / minutes
	wantAccuracy := correctChars / allTyped * 100.0

	result := Calculate(TestInput{Words: words, Duration: duration})

	assertApprox(t, "WPM", result.WPM, wantWPM)
	assertApprox(t, "RawWPM", result.RawWPM, wantRawWPM)
	assertApprox(t, "Accuracy", result.Accuracy, wantAccuracy)
	if result.WPM >= result.RawWPM {
		t.Errorf("WPM (%.2f) should be less than RawWPM (%.2f)", result.WPM, result.RawWPM)
	}
	if result.CorrectedWPM <= result.WPM {
		t.Errorf("CorrectedWPM (%.2f) should be greater than WPM (%.2f)", result.CorrectedWPM, result.WPM)
	}
	if result.Passed {
		t.Error("Passed: got true, want false")
	}
	if result.CorrectWords != 3 {
		t.Errorf("CorrectWords: got %d, want 3", result.CorrectWords)
	}
}

func TestAllIncorrect(t *testing.T) {
	words := makeWords(
		"hello", "hxllo",
		"world", "worlt",
		"these", "thxse",
		"tests", "tesst",
		"rock", "rxck",
	)
	duration := 6 * time.Second
	minutes := 6.0 / 60.0

	// correct_chars = 0 (no words are correct)
	// all_typed_chars = (5+1)+(5+1)+(5+1)+(5+1)+(4) = 28
	allTyped := float64(charCount([]string{"hxllo", "worlt", "thxse", "tesst", "rxck"}))

	result := Calculate(TestInput{Words: words, Duration: duration})

	assertApprox(t, "WPM", result.WPM, 0)
	wantRaw := (allTyped / 5.0) / minutes
	assertApprox(t, "RawWPM", result.RawWPM, wantRaw)
	assertApprox(t, "Accuracy", result.Accuracy, 0)
	if result.Passed {
		t.Error("Passed: got true, want false")
	}
	if result.CorrectWords != 0 {
		t.Errorf("CorrectWords: got %d, want 0", result.CorrectWords)
	}
}

func TestSingleWordCorrect(t *testing.T) {
	// One word "hello" typed correctly in 1 second
	// correct_chars = 5 (last word, no trailing space)
	// WPM = (5/5) / (1/60) = 1 / 0.01667 = 60
	words := makeWords("hello", "hello")
	duration := 1 * time.Second

	result := Calculate(TestInput{Words: words, Duration: duration})

	assertApprox(t, "WPM", result.WPM, 60.0)
	assertApprox(t, "RawWPM", result.RawWPM, 60.0)
	assertApprox(t, "CorrectedWPM", result.CorrectedWPM, 60.0)
	assertApprox(t, "Accuracy", result.Accuracy, 100.0)
	if !result.Passed {
		t.Error("Passed: got false, want true")
	}
}

func TestZeroDuration(t *testing.T) {
	words := makeWords("hello", "hello")
	result := Calculate(TestInput{Words: words, Duration: 0})

	assertApprox(t, "WPM", result.WPM, 0)
	assertApprox(t, "RawWPM", result.RawWPM, 0)
	assertApprox(t, "CorrectedWPM", result.CorrectedWPM, 0)
	assertApprox(t, "Accuracy", result.Accuracy, 0)
}

func TestEmptyInput(t *testing.T) {
	result := Calculate(TestInput{})

	assertApprox(t, "WPM", result.WPM, 0)
	assertApprox(t, "RawWPM", result.RawWPM, 0)
	assertApprox(t, "CorrectedWPM", result.CorrectedWPM, 0)
	assertApprox(t, "Accuracy", result.Accuracy, 0)
	assertApprox(t, "Consistency", result.Consistency, 0)
	if result.Passed {
		t.Error("Passed: got true, want false (empty input)")
	}
	if result.TotalWords != 0 {
		t.Errorf("TotalWords: got %d, want 0", result.TotalWords)
	}
}

func TestCorrectedWPMCalculation(t *testing.T) {
	// Scenario from the task:
	// Target is 50 chars total, typed 50 chars, 30 seconds, 40 correct chars.
	// We need to construct words that produce these numbers.
	//
	// Let's use 5 words, 4 correct + 1 wrong.
	// Target words: 10 chars each = 50 total target chars (with spaces).
	// 5 words, 4 spaces = 50 total. So each word is (50-4)/5 = 9.2 chars. Not clean.
	//
	// Let's use 6 words: 6 words + 5 spaces = total. Need total = 50.
	// Each word: (50-5)/6 = 7.5, not clean.
	//
	// Let's try 10 words of 4 chars each: 10*4 + 9 spaces = 49. Close.
	// Or 10 words: 9 of length 4 + 1 of length 5 = 36+5+9 = 50. Nice.
	//
	// Actually let's approach this differently. We need:
	// - total_target_chars = 50
	// - all_typed_chars = 50
	// - correct_chars = 40
	// - duration = 30s
	//
	// WPM = (40/5) / (30/60) = 8/0.5 = 16
	// RawWPM = (50/5) / 0.5 = 10/0.5 = 20
	// CorrectedWPM = (50/5) / 0.5 = 20
	// Delta = 20 - 16 = 4
	//
	// Build: 10 words, 9 spaces. Words sum to 41 chars.
	// 8 correct words contributing correct_chars.
	// If 8 words are correct (all 4-char) + 2 wrong (also 4-char):
	// correct_chars = 8 words of 4 chars each. 8 correct words.
	// If a correct word at position i (not last): contributes 4+1=5.
	// If a correct word at position 9 (last): contributes 4.
	// So if all 8 correct words are positions 0-7: 8*5 = 40.
	// Wrong words at positions 8,9: contribute 0 correct chars.
	// correct_chars = 40. Check!
	// all_typed_chars = 10*4 + 9 = 49. That's 49, not 50.
	//
	// Need all_typed = 50. Let's make one word 5 chars.
	// 9 words of 4 chars + 1 word of 5 chars = 36+5 = 41 + 9 spaces = 50.
	// Put the 5-char word at position 9 (last, wrong word).
	// Correct words: positions 0-7, each 4 chars, not last: 8*(4+1) = 40.
	// Wrong words: position 8 (4 chars) and position 9 (5 chars).
	// all_typed = 50, correct = 40, total_target = 50.

	targets := []string{"alfa", "beta", "code", "data", "edit", "find", "goal", "help", "item", "jumps"}
	typeds := []string{"alfa", "beta", "code", "data", "edit", "find", "goal", "help", "itme", "junps"}
	// positions 0-7 correct, 8-9 wrong. Last word "jumps" is 5 chars.

	var words []WordResult
	now := time.Now()
	for i := 0; i < len(targets); i++ {
		words = append(words, WordResult{
			Target:    targets[i],
			Typed:     typeds[i],
			Correct:   targets[i] == typeds[i],
			StartTime: now,
			EndTime:   now.Add(time.Second),
		})
	}

	duration := 30 * time.Second

	result := Calculate(TestInput{Words: words, Duration: duration})

	assertApprox(t, "WPM", result.WPM, 16.0)
	assertApprox(t, "RawWPM", result.RawWPM, 20.0)
	assertApprox(t, "CorrectedWPM", result.CorrectedWPM, 20.0)
	assertApprox(t, "CorrectionDelta", result.CorrectionDelta, 4.0)
}

func TestConsistencyUniformSamples(t *testing.T) {
	// All samples the same -> stddev = 0, CV = 0, consistency = 100
	samples := []float64{80, 80, 80, 80, 80}
	words := makeWords("hello", "hello")
	duration := 1 * time.Second

	result := Calculate(TestInput{Words: words, Duration: duration, WPMSamples: samples})

	assertApprox(t, "Consistency", result.Consistency, 100.0)
}

func TestConsistencyVaryingSamples(t *testing.T) {
	// Samples: [50, 100, 50, 100]
	// Mean = 75
	// Variance = ((50-75)^2 + (100-75)^2 + (50-75)^2 + (100-75)^2) / 4
	//          = (625 + 625 + 625 + 625) / 4 = 625
	// StdDev = 25
	// CV = 25/75 * 100 = 33.333...
	// Consistency = 100 - 33.333 = 66.667
	samples := []float64{50, 100, 50, 100}
	words := makeWords("hello", "hello")
	duration := 1 * time.Second

	result := Calculate(TestInput{Words: words, Duration: duration, WPMSamples: samples})

	assertApprox(t, "Consistency", result.Consistency, 66.6667)
}

func TestConsistencySingleSample(t *testing.T) {
	// Single sample -> can't measure variance -> consistency = 100
	samples := []float64{80}
	words := makeWords("hello", "hello")
	duration := 1 * time.Second

	result := Calculate(TestInput{Words: words, Duration: duration, WPMSamples: samples})

	assertApprox(t, "Consistency", result.Consistency, 100.0)
}

func TestAccuracyWithExtraChars(t *testing.T) {
	// Typed extra characters in wrong words.
	// Target "cat" typed "cats" (4 chars vs 3 target)
	// Target "dog" typed "dog" (correct)
	// all_typed_chars = (4+1) + 3 = 8 (cats+space, dog no trailing space)
	// correct_chars = 0 + (3) = 3 (only "dog" is correct, it's the last word so no +1)
	// Wait, "cat" != "cats" so it's wrong. "dog" == "dog" so correct.
	// correct_chars = 3 (dog, last word, no space)
	// all_typed_chars = (4+1) + 3 = 8
	// accuracy = 3/8 * 100 = 37.5%
	words := makeWords(
		"cat", "cats",
		"dog", "dog",
	)
	duration := 2 * time.Second
	minutes := 2.0 / 60.0

	// total_target_chars = 3 + 1 + 3 = 7 ("cat" + space + "dog")
	// all_typed_chars = 4 + 1 + 3 = 8 ("cats" + space + "dog")
	// correct_chars = 3 ("dog" is last word)

	result := Calculate(TestInput{Words: words, Duration: duration})

	assertApprox(t, "Accuracy", result.Accuracy, 3.0/8.0*100.0)
	// RawWPM uses all typed chars
	assertApprox(t, "RawWPM", result.RawWPM, (8.0/5.0)/minutes)
	// CorrectedWPM uses target chars
	assertApprox(t, "CorrectedWPM", result.CorrectedWPM, (7.0/5.0)/minutes)
}

func TestPassedFlag(t *testing.T) {
	t.Run("all correct means passed", func(t *testing.T) {
		words := makeWords("go", "go", "is", "is", "fun", "fun")
		result := Calculate(TestInput{Words: words, Duration: 3 * time.Second})
		if !result.Passed {
			t.Error("Passed: got false, want true")
		}
	})

	t.Run("one wrong means not passed", func(t *testing.T) {
		words := makeWords("go", "go", "is", "is", "fun", "fnu")
		result := Calculate(TestInput{Words: words, Duration: 3 * time.Second})
		if result.Passed {
			t.Error("Passed: got true, want false")
		}
	})

	t.Run("empty means not passed", func(t *testing.T) {
		result := Calculate(TestInput{Duration: 3 * time.Second})
		if result.Passed {
			t.Error("Passed: got true, want false (no words)")
		}
	})
}

func TestDurationStored(t *testing.T) {
	duration := 42 * time.Second
	words := makeWords("test", "test")
	result := Calculate(TestInput{Words: words, Duration: duration})

	if result.Duration != duration {
		t.Errorf("Duration: got %v, want %v", result.Duration, duration)
	}
}

func TestConsistencyClampedToZero(t *testing.T) {
	// If CV > 100, consistency should be clamped to 0 (not go negative)
	// Samples with huge variance relative to mean: [1, 1000]
	// Mean = 500.5
	// Variance = ((1-500.5)^2 + (1000-500.5)^2) / 2 = (249500.25 + 249500.25)/2 = 249500.25
	// StdDev = 499.5
	// CV = 499.5/500.5 * 100 = 99.8...
	// That's < 100, so not clamped. Need more extreme.
	// [1, 10000]: mean = 5000.5, stddev = 4999.5, CV = 99.98. Still < 100.
	// [0.01, 100]: mean = 50.005, stddev ~= 49.995, CV ~= 99.98. Hmm.
	// Need stddev > mean. [1, 1, 1, 1, 1000]:
	// Mean = 200.8, variance = (199.8^2*4 + 799.2^2)/5 = (159520.16 + 638720.64)/5 = 159648.16
	// StdDev = 399.56, CV = 399.56/200.8*100 = 199%. Consistency would be 100-199 = -99, clamped to 0.
	samples := []float64{1, 1, 1, 1, 1000}
	words := makeWords("hello", "hello")
	duration := 1 * time.Second

	result := Calculate(TestInput{Words: words, Duration: duration, WPMSamples: samples})

	if result.Consistency < 0 {
		t.Errorf("Consistency should be >= 0, got %.2f", result.Consistency)
	}
	assertApprox(t, "Consistency", result.Consistency, 0.0)
}
