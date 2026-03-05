package stats

import (
	"math"
	"time"
)

type WordResult struct {
	Target    string
	Typed     string
	Correct   bool
	StartTime time.Time
	EndTime   time.Time
}

type TestInput struct {
	Words      []WordResult
	Duration   time.Duration
	WPMSamples []float64 // per-second raw WPM snapshots
}

type TestResult struct {
	WPM             float64
	RawWPM          float64
	CorrectedWPM    float64
	Accuracy        float64
	Consistency     float64
	CorrectionDelta float64
	CorrectWords    int
	TotalWords      int
	Passed          bool // all words correct
	Duration        time.Duration
}

// Calculate computes all typing test metrics from the given input.
func Calculate(input TestInput) TestResult {
	n := len(input.Words)
	if n == 0 || input.Duration == 0 {
		return TestResult{
			Duration:   input.Duration,
			TotalWords: n,
		}
	}

	minutes := input.Duration.Seconds() / 60.0

	var correctChars, allTypedChars, totalTargetChars float64
	correctWords := 0

	for i, w := range input.Words {
		typed := float64(len(w.Typed))
		target := float64(len(w.Target))

		// Every word except the last gets +1 for the space after it
		space := 0.0
		if i < n-1 {
			space = 1.0
		}

		allTypedChars += typed + space
		totalTargetChars += target + space

		if w.Correct {
			correctChars += typed + space
			correctWords++
		}
	}

	wpm := (correctChars / 5.0) / minutes
	rawWPM := (allTypedChars / 5.0) / minutes
	correctedWPM := (totalTargetChars / 5.0) / minutes

	accuracy := 0.0
	if allTypedChars > 0 {
		accuracy = correctChars / allTypedChars * 100.0
	}

	consistency := calcConsistency(input.WPMSamples)

	return TestResult{
		WPM:             wpm,
		RawWPM:          rawWPM,
		CorrectedWPM:    correctedWPM,
		Accuracy:        accuracy,
		Consistency:     consistency,
		CorrectionDelta: correctedWPM - wpm,
		CorrectWords:    correctWords,
		TotalWords:      n,
		Passed:          correctWords == n,
		Duration:        input.Duration,
	}
}

// calcConsistency returns 100 - CV (coefficient of variation as percentage),
// clamped to [0, 100]. Returns 0 if there are no samples, 100 if there's
// only one sample or zero variance.
func calcConsistency(samples []float64) float64 {
	n := len(samples)
	if n <= 1 {
		if n == 1 {
			return 100.0
		}
		return 0.0
	}

	var sum float64
	for _, s := range samples {
		sum += s
	}
	mean := sum / float64(n)
	if mean == 0 {
		return 0.0
	}

	var variance float64
	for _, s := range samples {
		d := s - mean
		variance += d * d
	}
	variance /= float64(n) // population variance

	stddev := math.Sqrt(variance)
	cv := stddev / mean * 100.0

	consistency := 100.0 - cv
	if consistency < 0 {
		return 0.0
	}
	if consistency > 100 {
		return 100.0
	}
	return consistency
}
