package history

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func makeRecord(wpm, accuracy float64, mode string, modeValue int, wordList string, ts time.Time) TestRecord {
	return TestRecord{
		Timestamp:    ts,
		Mode:         mode,
		ModeValue:    modeValue,
		WordList:     wordList,
		WPM:          wpm,
		RawWPM:       wpm + 20,
		CorrectedWPM: wpm + 10,
		Accuracy:     accuracy,
		Consistency:  90,
		CorrectWords: 42,
		TotalWords:   50,
		Passed:       accuracy > 95,
		DurationSecs: 30.0,
		TypedWords:   []string{"the", "quick"},
		TargetWords:  []string{"the", "quick"},
	}
}

func TestNewStoreAndLoadMissingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.json")

	s := NewStore(path)
	if err := s.Load(); err != nil {
		t.Fatalf("Load on missing file should not error, got: %v", err)
	}
	if s.TotalTests() != 0 {
		t.Fatalf("expected 0 tests, got %d", s.TotalTests())
	}
}

func TestSaveThenLoadRoundtrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.json")

	s := NewStore(path)
	_ = s.Load()

	ts := time.Date(2026, 3, 5, 14, 0, 0, 0, time.UTC)
	rec := makeRecord(120, 95.5, "words", 50, "english_1k", ts)

	if err := s.Save(rec); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// New store, same path, load from disk
	s2 := NewStore(path)
	if err := s2.Load(); err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if s2.TotalTests() != 1 {
		t.Fatalf("expected 1 test, got %d", s2.TotalTests())
	}
	if s2.Tests[0].WPM != 120 {
		t.Fatalf("expected WPM 120, got %f", s2.Tests[0].WPM)
	}
	if s2.Tests[0].Accuracy != 95.5 {
		t.Fatalf("expected Accuracy 95.5, got %f", s2.Tests[0].Accuracy)
	}
	if s2.Tests[0].Mode != "words" {
		t.Fatalf("expected mode 'words', got %q", s2.Tests[0].Mode)
	}
}

func TestMultipleSaves(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.json")

	s := NewStore(path)
	_ = s.Load()

	base := time.Date(2026, 3, 5, 14, 0, 0, 0, time.UTC)
	for i := range 3 {
		rec := makeRecord(float64(100+i*10), 90, "words", 50, "english_1k", base.Add(time.Duration(i)*time.Minute))
		if err := s.Save(rec); err != nil {
			t.Fatalf("Save %d failed: %v", i, err)
		}
	}

	s2 := NewStore(path)
	_ = s2.Load()
	if s2.TotalTests() != 3 {
		t.Fatalf("expected 3 tests, got %d", s2.TotalTests())
	}
	// Verify order (chronological in storage)
	if s2.Tests[0].WPM != 100 {
		t.Fatalf("expected first WPM 100, got %f", s2.Tests[0].WPM)
	}
	if s2.Tests[2].WPM != 120 {
		t.Fatalf("expected third WPM 120, got %f", s2.Tests[2].WPM)
	}
}

func TestRecentTests(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.json")

	s := NewStore(path)
	_ = s.Load()

	base := time.Date(2026, 3, 5, 14, 0, 0, 0, time.UTC)
	for i := range 5 {
		rec := makeRecord(float64(100+i*10), 90, "words", 50, "english_1k", base.Add(time.Duration(i)*time.Minute))
		_ = s.Save(rec)
	}

	recent := s.RecentTests(3)
	if len(recent) != 3 {
		t.Fatalf("expected 3 recent tests, got %d", len(recent))
	}
	// Newest first: WPM 140, 130, 120
	if recent[0].WPM != 140 {
		t.Fatalf("expected newest WPM 140, got %f", recent[0].WPM)
	}
	if recent[1].WPM != 130 {
		t.Fatalf("expected second WPM 130, got %f", recent[1].WPM)
	}
	if recent[2].WPM != 120 {
		t.Fatalf("expected third WPM 120, got %f", recent[2].WPM)
	}
}

func TestRecentTestsMoreThanTotal(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.json")

	s := NewStore(path)
	_ = s.Load()

	base := time.Date(2026, 3, 5, 14, 0, 0, 0, time.UTC)
	for i := range 2 {
		rec := makeRecord(float64(100+i*10), 90, "words", 50, "english_1k", base.Add(time.Duration(i)*time.Minute))
		_ = s.Save(rec)
	}

	recent := s.RecentTests(10)
	if len(recent) != 2 {
		t.Fatalf("expected 2 tests (all available), got %d", len(recent))
	}
}

func TestRecentTestsZero(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.json")

	s := NewStore(path)
	_ = s.Load()

	_ = s.Save(makeRecord(100, 90, "words", 50, "english_1k", time.Now()))

	recent := s.RecentTests(0)
	if len(recent) != 0 {
		t.Fatalf("expected 0 tests, got %d", len(recent))
	}
}

func TestPersonalBest(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.json")

	s := NewStore(path)
	_ = s.Load()

	base := time.Date(2026, 3, 5, 14, 0, 0, 0, time.UTC)
	wpms := []float64{100, 130, 110, 90, 120}
	for i, wpm := range wpms {
		rec := makeRecord(wpm, 90+float64(i), "words", 50, "english_1k", base.Add(time.Duration(i)*time.Minute))
		_ = s.Save(rec)
	}

	pb := s.PersonalBest("words", 50, "english_1k")
	if pb == nil {
		t.Fatal("expected a personal best, got nil")
	}
	if pb.WPM != 130 {
		t.Fatalf("expected PB WPM 130, got %f", pb.WPM)
	}
}

func TestPersonalBestNoMatch(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.json")

	s := NewStore(path)
	_ = s.Load()

	_ = s.Save(makeRecord(100, 90, "words", 50, "english_1k", time.Now()))

	pb := s.PersonalBest("time", 60, "english_1k")
	if pb != nil {
		t.Fatalf("expected nil for non-matching mode combo, got %+v", pb)
	}
}

func TestPersonalBestAcrossDifferentCombos(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.json")

	s := NewStore(path)
	_ = s.Load()

	base := time.Date(2026, 3, 5, 14, 0, 0, 0, time.UTC)

	// words/50/english_1k tests
	_ = s.Save(makeRecord(100, 90, "words", 50, "english_1k", base))
	_ = s.Save(makeRecord(130, 92, "words", 50, "english_1k", base.Add(time.Minute)))

	// words/100/english_1k tests
	_ = s.Save(makeRecord(110, 88, "words", 100, "english_1k", base.Add(2*time.Minute)))
	_ = s.Save(makeRecord(105, 85, "words", 100, "english_1k", base.Add(3*time.Minute)))

	pb50 := s.PersonalBest("words", 50, "english_1k")
	pb100 := s.PersonalBest("words", 100, "english_1k")

	if pb50 == nil || pb100 == nil {
		t.Fatal("expected both PBs to be non-nil")
	}
	if pb50.WPM != 130 {
		t.Fatalf("expected words/50 PB WPM 130, got %f", pb50.WPM)
	}
	if pb100.WPM != 110 {
		t.Fatalf("expected words/100 PB WPM 110, got %f", pb100.WPM)
	}
}

func TestIsPersonalBest(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.json")

	s := NewStore(path)
	_ = s.Load()

	base := time.Date(2026, 3, 5, 14, 0, 0, 0, time.UTC)
	_ = s.Save(makeRecord(100, 90, "words", 50, "english_1k", base))

	// Higher WPM -> new PB
	newPB := makeRecord(120, 92, "words", 50, "english_1k", base.Add(time.Minute))
	if !s.IsPersonalBest(newPB) {
		t.Fatal("expected 120 WPM to be a new PB over 100")
	}

	// Lower WPM -> not a PB (still compared against saved 100, since newPB isn't saved yet)
	notPB := makeRecord(90, 88, "words", 50, "english_1k", base.Add(2*time.Minute))
	if s.IsPersonalBest(notPB) {
		t.Fatal("expected 90 WPM to NOT be a new PB")
	}
}

func TestIsPersonalBestFirstTest(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.json")

	s := NewStore(path)
	_ = s.Load()

	rec := makeRecord(80, 85, "words", 50, "english_1k", time.Now())
	if !s.IsPersonalBest(rec) {
		t.Fatal("first test should always be a personal best")
	}
}

func TestTotalTests(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.json")

	s := NewStore(path)
	_ = s.Load()

	if s.TotalTests() != 0 {
		t.Fatalf("expected 0, got %d", s.TotalTests())
	}

	base := time.Date(2026, 3, 5, 14, 0, 0, 0, time.UTC)
	for i := range 7 {
		_ = s.Save(makeRecord(float64(100+i), 90, "words", 50, "english_1k", base.Add(time.Duration(i)*time.Minute)))
	}
	if s.TotalTests() != 7 {
		t.Fatalf("expected 7, got %d", s.TotalTests())
	}
}

func TestAverageWPM(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.json")

	s := NewStore(path)
	_ = s.Load()

	base := time.Date(2026, 3, 5, 14, 0, 0, 0, time.UTC)
	wpms := []float64{100, 120, 140}
	for i, wpm := range wpms {
		_ = s.Save(makeRecord(wpm, 90, "words", 50, "english_1k", base.Add(time.Duration(i)*time.Minute)))
	}

	avg := s.AverageWPM()
	expected := 120.0
	if avg != expected {
		t.Fatalf("expected average WPM %f, got %f", expected, avg)
	}
}

func TestAverageAccuracy(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.json")

	s := NewStore(path)
	_ = s.Load()

	base := time.Date(2026, 3, 5, 14, 0, 0, 0, time.UTC)
	accs := []float64{80, 90, 100}
	for i, acc := range accs {
		_ = s.Save(makeRecord(100, acc, "words", 50, "english_1k", base.Add(time.Duration(i)*time.Minute)))
	}

	avg := s.AverageAccuracy()
	expected := 90.0
	if avg != expected {
		t.Fatalf("expected average accuracy %f, got %f", expected, avg)
	}
}

func TestCorruptJSONFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.json")

	if err := os.WriteFile(path, []byte("{not valid json!!!"), 0644); err != nil {
		t.Fatal(err)
	}

	s := NewStore(path)
	err := s.Load()
	if err == nil {
		t.Fatal("expected error loading corrupt JSON, got nil")
	}
}

func TestEmptyTestsArray(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.json")

	data, _ := json.Marshal(struct {
		Tests []TestRecord `json:"tests"`
	}{Tests: []TestRecord{}})
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}

	s := NewStore(path)
	if err := s.Load(); err != nil {
		t.Fatalf("Load with empty tests array should succeed, got: %v", err)
	}
	if s.TotalTests() != 0 {
		t.Fatalf("expected 0 tests, got %d", s.TotalTests())
	}
}
