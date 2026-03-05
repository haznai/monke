package typing

import (
	"testing"
	"time"
)

// --- Basic flow ---

func TestNewEngine_InitialState(t *testing.T) {
	e := NewEngine([]string{"hello", "world", "foo"})

	if e.IsStarted() {
		t.Error("engine should not be started initially")
	}
	if e.IsFinished() {
		t.Error("engine should not be finished initially")
	}
	if e.CurrentWordIndex() != 0 {
		t.Errorf("currentIdx = %d, want 0", e.CurrentWordIndex())
	}
	if e.CurrentInput() != "" {
		t.Errorf("input = %q, want empty", e.CurrentInput())
	}

	words := e.Words()
	if len(words) != 3 {
		t.Fatalf("len(words) = %d, want 3", len(words))
	}
	for i, w := range words {
		if w.Done {
			t.Errorf("word %d should not be done", i)
		}
		if w.Typed != "" {
			t.Errorf("word %d typed = %q, want empty", i, w.Typed)
		}
	}
	if words[0].Target != "hello" || words[1].Target != "world" || words[2].Target != "foo" {
		t.Error("target words mismatch")
	}
}

func TestTypeAndSpace_CorrectWord(t *testing.T) {
	e := NewEngine([]string{"hello", "world", "foo"})

	for _, c := range "hello" {
		e.TypeChar(c)
	}
	e.Space()

	words := e.Words()
	if !words[0].Done {
		t.Error("word 0 should be done")
	}
	if !words[0].Correct {
		t.Error("word 0 should be correct")
	}
	if words[0].Typed != "hello" {
		t.Errorf("word 0 typed = %q, want hello", words[0].Typed)
	}
	if e.CurrentWordIndex() != 1 {
		t.Errorf("currentIdx = %d, want 1", e.CurrentWordIndex())
	}
	if e.CurrentInput() != "" {
		t.Errorf("input after space = %q, want empty", e.CurrentInput())
	}
}

func TestFullTest_ThreeWords(t *testing.T) {
	e := NewEngine([]string{"hello", "world", "foo"})

	typeWord(e, "hello")
	e.Space()
	typeWord(e, "world")
	e.Space()
	typeWord(e, "foo")
	e.Space()

	if !e.IsFinished() {
		t.Error("engine should be finished after all words submitted")
	}
	words := e.Words()
	for i, w := range words {
		if !w.Done {
			t.Errorf("word %d should be done", i)
		}
		if !w.Correct {
			t.Errorf("word %d should be correct", i)
		}
	}
}

func TestNoOpsAfterFinished(t *testing.T) {
	e := NewEngine([]string{"a"})
	e.TypeChar('a')
	e.Space()

	if !e.IsFinished() {
		t.Fatal("should be finished")
	}

	// all of these should be no-ops
	e.TypeChar('x')
	e.Space()
	e.Backspace()
	e.DeleteWord()

	if e.CurrentInput() != "" {
		t.Errorf("input should still be empty, got %q", e.CurrentInput())
	}
	if e.CurrentWordIndex() != 1 {
		t.Errorf("currentIdx should still be 1, got %d", e.CurrentWordIndex())
	}
}

// --- Correctness ---

func TestCorrectWord(t *testing.T) {
	e := NewEngine([]string{"apple"})
	typeWord(e, "apple")
	e.Space()

	w := e.Words()[0]
	if !w.Correct {
		t.Error("word should be correct")
	}
}

func TestIncorrectWord(t *testing.T) {
	e := NewEngine([]string{"apple"})
	typeWord(e, "aple")
	e.Space()

	w := e.Words()[0]
	if w.Correct {
		t.Error("word should be incorrect")
	}
	if w.Typed != "aple" {
		t.Errorf("typed = %q, want aple", w.Typed)
	}
}

func TestEmptySpaceSubmitsIncorrect(t *testing.T) {
	e := NewEngine([]string{"hello", "world"})
	e.Space() // empty submit

	w := e.Words()[0]
	if !w.Done {
		t.Error("word 0 should be done")
	}
	if w.Correct {
		t.Error("word 0 should be incorrect (empty input)")
	}
	if w.Typed != "" {
		t.Errorf("typed = %q, want empty", w.Typed)
	}
	if e.CurrentWordIndex() != 1 {
		t.Errorf("currentIdx = %d, want 1", e.CurrentWordIndex())
	}
}

func TestTypoMissingChar(t *testing.T) {
	e := NewEngine([]string{"hello"})
	typeWord(e, "helo")
	e.Space()

	if e.Words()[0].Correct {
		t.Error("helo != hello, should be incorrect")
	}
}

// --- Backspace ---

func TestBackspace_RemovesLastChar(t *testing.T) {
	e := NewEngine([]string{"hello"})
	typeWord(e, "hel")
	e.Backspace()

	if e.CurrentInput() != "he" {
		t.Errorf("input = %q, want he", e.CurrentInput())
	}
}

func TestBackspace_AllChars(t *testing.T) {
	e := NewEngine([]string{"hello"})
	typeWord(e, "hel")
	e.Backspace()
	e.Backspace()
	e.Backspace()

	if e.CurrentInput() != "" {
		t.Errorf("input = %q, want empty", e.CurrentInput())
	}
}

func TestBackspace_OnEmptyInput(t *testing.T) {
	e := NewEngine([]string{"hello"})
	e.Backspace() // should not panic
	if e.CurrentInput() != "" {
		t.Errorf("input = %q, want empty", e.CurrentInput())
	}
}

func TestBackspace_AfterFinished(t *testing.T) {
	e := NewEngine([]string{"a"})
	e.TypeChar('a')
	e.Space()
	e.Backspace() // no-op
	// no panic is the test
}

// --- DeleteWord ---

func TestDeleteWord_ClearsInput(t *testing.T) {
	e := NewEngine([]string{"hello"})
	typeWord(e, "hel")
	e.DeleteWord()

	if e.CurrentInput() != "" {
		t.Errorf("input = %q, want empty", e.CurrentInput())
	}
}

func TestDeleteWord_OnEmptyInput(t *testing.T) {
	e := NewEngine([]string{"hello"})
	e.DeleteWord() // should not panic
	if e.CurrentInput() != "" {
		t.Errorf("input = %q, want empty", e.CurrentInput())
	}
}

func TestDeleteWord_AfterFinished(t *testing.T) {
	e := NewEngine([]string{"a"})
	e.TypeChar('a')
	e.Space()
	e.DeleteWord() // no-op, no panic
}

// --- Reset ---

func TestReset_RestoresInitialState(t *testing.T) {
	e := NewEngine([]string{"hello", "world", "foo"})
	typeWord(e, "hello")
	e.Space()
	typeWord(e, "xyz")
	e.Space()

	e.Reset()

	if e.IsStarted() {
		t.Error("should not be started after reset")
	}
	if e.IsFinished() {
		t.Error("should not be finished after reset")
	}
	if e.CurrentWordIndex() != 0 {
		t.Errorf("currentIdx = %d, want 0", e.CurrentWordIndex())
	}
	if e.CurrentInput() != "" {
		t.Errorf("input = %q, want empty", e.CurrentInput())
	}

	words := e.Words()
	if len(words) != 3 {
		t.Fatalf("len(words) = %d, want 3", len(words))
	}
	// targets should be preserved
	if words[0].Target != "hello" || words[1].Target != "world" || words[2].Target != "foo" {
		t.Error("targets changed after reset")
	}
	// all state should be cleared
	for i, w := range words {
		if w.Done || w.Correct || w.Typed != "" {
			t.Errorf("word %d not fully reset: %+v", i, w)
		}
	}
}

func TestReset_CanTypeAgain(t *testing.T) {
	e := NewEngine([]string{"hello"})
	e.TypeChar('h')
	e.Space()
	if !e.IsFinished() {
		t.Fatal("should be finished")
	}

	e.Reset()
	typeWord(e, "hello")
	e.Space()

	if !e.IsFinished() {
		t.Error("should be finished again after re-typing")
	}
	if !e.Words()[0].Correct {
		t.Error("word should be correct after reset and re-type")
	}
}

// --- Timing ---

func TestFirstTypeChar_StartsTimer(t *testing.T) {
	e := NewEngine([]string{"hello", "world"})

	if e.IsStarted() {
		t.Error("should not be started before typing")
	}

	e.TypeChar('h')

	if !e.IsStarted() {
		t.Error("should be started after first TypeChar")
	}
}

func TestLastSpace_SetsEndTime(t *testing.T) {
	e := NewEngine([]string{"hi"})
	typeWord(e, "hi")
	e.Space()

	elapsed := e.ElapsedTime()
	if elapsed <= 0 {
		t.Errorf("elapsed = %v, want > 0", elapsed)
	}
}

func TestElapsedTime_ReasonableDuration(t *testing.T) {
	e := NewEngine([]string{"hello", "world"})
	typeWord(e, "hello")
	e.Space()
	time.Sleep(10 * time.Millisecond)
	typeWord(e, "world")
	e.Space()

	elapsed := e.ElapsedTime()
	if elapsed < 10*time.Millisecond {
		t.Errorf("elapsed = %v, expected at least 10ms", elapsed)
	}
	if elapsed > 5*time.Second {
		t.Errorf("elapsed = %v, way too long", elapsed)
	}
}

// --- Char counting ---

func TestTotalTypedChars(t *testing.T) {
	e := NewEngine([]string{"hello", "world", "foo"})
	typeWord(e, "hello") // 5 chars
	e.Space()
	typeWord(e, "wor") // 3 chars in progress

	// submitted: "hello" (5) + current input: "wor" (3) = 8
	if got := e.TotalTypedChars(); got != 8 {
		t.Errorf("TotalTypedChars = %d, want 8", got)
	}
}

func TestCorrectChars(t *testing.T) {
	e := NewEngine([]string{"hello", "world", "foo"})
	typeWord(e, "hello") // correct
	e.Space()
	typeWord(e, "xyz") // incorrect
	e.Space()
	typeWord(e, "foo") // correct
	e.Space()

	// correct words: "hello" (5) + "foo" (3) = 8
	if got := e.CorrectChars(); got != 8 {
		t.Errorf("CorrectChars = %d, want 8", got)
	}
}

func TestTargetChars(t *testing.T) {
	e := NewEngine([]string{"hello", "world", "foo"})
	// "hello"(5) + "world"(5) + "foo"(3) = 13
	if got := e.TargetChars(); got != 13 {
		t.Errorf("TargetChars = %d, want 13", got)
	}
}

// --- Multi-word flow ---

func TestMultiWordMixedCorrectness(t *testing.T) {
	targets := []string{"the", "quick", "brown", "fox", "jumps"}
	e := NewEngine(targets)

	typeWord(e, "the")   // correct
	e.Space()
	typeWord(e, "quikc") // incorrect
	e.Space()
	typeWord(e, "brown") // correct
	e.Space()
	typeWord(e, "fxo")   // incorrect
	e.Space()
	typeWord(e, "jumps") // correct
	e.Space()

	if !e.IsFinished() {
		t.Error("should be finished")
	}

	words := e.Words()
	expected := []struct {
		correct bool
		typed   string
	}{
		{true, "the"},
		{false, "quikc"},
		{true, "brown"},
		{false, "fxo"},
		{true, "jumps"},
	}

	for i, exp := range expected {
		if words[i].Correct != exp.correct {
			t.Errorf("word %d correct = %v, want %v", i, words[i].Correct, exp.correct)
		}
		if words[i].Typed != exp.typed {
			t.Errorf("word %d typed = %q, want %q", i, words[i].Typed, exp.typed)
		}
		if !words[i].Done {
			t.Errorf("word %d should be done", i)
		}
	}
}

func TestCurrentWordIndex_Advances(t *testing.T) {
	e := NewEngine([]string{"a", "b", "c", "d", "e"})

	for i := 0; i < 5; i++ {
		if e.CurrentWordIndex() != i {
			t.Errorf("before word %d: currentIdx = %d", i, e.CurrentWordIndex())
		}
		e.TypeChar('x')
		e.Space()
	}

	if e.CurrentWordIndex() != 5 {
		t.Errorf("after all words: currentIdx = %d, want 5", e.CurrentWordIndex())
	}
}

// --- Edge cases ---

func TestSingleWordTest(t *testing.T) {
	e := NewEngine([]string{"go"})
	typeWord(e, "go")
	e.Space()

	if !e.IsFinished() {
		t.Error("single word test should be finished")
	}
	if !e.Words()[0].Correct {
		t.Error("word should be correct")
	}
}

func TestEmptyWordList(t *testing.T) {
	e := NewEngine([]string{})

	if e.IsStarted() {
		t.Error("should not be started")
	}
	// should already be finished (nothing to type)
	if !e.IsFinished() {
		t.Error("empty word list should be immediately finished")
	}
	if len(e.Words()) != 0 {
		t.Error("words should be empty")
	}

	// all ops should be safe no-ops
	e.TypeChar('x')
	e.Space()
	e.Backspace()
	e.DeleteWord()
	e.Reset()
}

// --- Snapshots ---

func TestSnapshot_RecordsValue(t *testing.T) {
	e := NewEngine([]string{"hello", "world"})
	typeWord(e, "hello")
	e.Space()

	// wait a tiny bit so elapsed > 0
	time.Sleep(5 * time.Millisecond)
	e.Snapshot()

	snaps := e.Snapshots()
	if len(snaps) != 1 {
		t.Fatalf("len(snapshots) = %d, want 1", len(snaps))
	}
	if snaps[0] <= 0 {
		t.Errorf("snapshot value = %f, want > 0", snaps[0])
	}
}

func TestSnapshots_MultipleCaptures(t *testing.T) {
	e := NewEngine([]string{"hello", "world", "foo", "bar"})
	typeWord(e, "hello")
	e.Space()
	time.Sleep(5 * time.Millisecond)
	e.Snapshot()

	typeWord(e, "world")
	e.Space()
	time.Sleep(5 * time.Millisecond)
	e.Snapshot()

	snaps := e.Snapshots()
	if len(snaps) != 2 {
		t.Fatalf("len(snapshots) = %d, want 2", len(snaps))
	}
}

func TestSnapshots_ClearedOnReset(t *testing.T) {
	e := NewEngine([]string{"hello", "world"})
	typeWord(e, "hello")
	e.Space()
	time.Sleep(5 * time.Millisecond)
	e.Snapshot()

	e.Reset()
	if len(e.Snapshots()) != 0 {
		t.Error("snapshots should be cleared after reset")
	}
}

// --- Space starts timer ---

func TestSpaceOnEmpty_StartsTimer(t *testing.T) {
	e := NewEngine([]string{"hello", "world"})
	e.Space() // empty submit, but should start timer

	if !e.IsStarted() {
		t.Error("Space should start the timer even with empty input")
	}
}

// --- Auto-finish on last word ---

func TestAutoFinish_LastWord_Correct(t *testing.T) {
	e := NewEngine([]string{"hello", "world"})
	typeWord(e, "hello")
	e.Space()
	typeWord(e, "world")

	if !e.IsFinished() {
		t.Error("should be finished after typing last char of last word")
	}
	w := e.Words()[1]
	if !w.Done {
		t.Error("last word should be done")
	}
	if !w.Correct {
		t.Error("last word should be correct")
	}
	if w.Typed != "world" {
		t.Errorf("last word typed = %q, want world", w.Typed)
	}
}

func TestAutoFinish_LastWord_Incorrect(t *testing.T) {
	e := NewEngine([]string{"hello", "world"})
	typeWord(e, "hello")
	e.Space()
	typeWord(e, "worxd")

	if !e.IsFinished() {
		t.Error("should be finished after typing last char of last word (even if wrong)")
	}
	w := e.Words()[1]
	if w.Correct {
		t.Error("last word should be incorrect")
	}
	if w.Typed != "worxd" {
		t.Errorf("last word typed = %q, want worxd", w.Typed)
	}
}

func TestAutoFinish_SingleWord(t *testing.T) {
	e := NewEngine([]string{"go"})
	typeWord(e, "go")

	if !e.IsFinished() {
		t.Error("should be finished after typing last char of single word")
	}
	if !e.Words()[0].Correct {
		t.Error("word should be correct")
	}
}

func TestAutoFinish_SpaceAfterIsNoop(t *testing.T) {
	e := NewEngine([]string{"hi"})
	typeWord(e, "hi")

	if !e.IsFinished() {
		t.Fatal("should be finished")
	}

	// Space after auto-finish should be a no-op
	e.Space()
	if e.CurrentWordIndex() != 1 {
		t.Errorf("currentIdx = %d, want 1", e.CurrentWordIndex())
	}
}

// --- helpers ---

func typeWord(e *Engine, word string) {
	for _, c := range word {
		e.TypeChar(c)
	}
}
