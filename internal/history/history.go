package history

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

type TestRecord struct {
	Timestamp    time.Time `json:"timestamp"`
	Mode         string    `json:"mode"`
	ModeValue    int       `json:"mode_value"`
	WordList     string    `json:"word_list"`
	WPM          float64   `json:"wpm"`
	RawWPM       float64   `json:"raw_wpm"`
	CorrectedWPM float64  `json:"corrected_wpm"`
	Accuracy     float64   `json:"accuracy"`
	Consistency  float64   `json:"consistency"`
	CorrectWords int       `json:"correct_words"`
	TotalWords   int       `json:"total_words"`
	Passed       bool      `json:"passed"`
	DurationSecs float64   `json:"duration_seconds"`
	TypedWords   []string  `json:"typed_words"`
	TargetWords  []string  `json:"target_words"`
}

type PersonalBest struct {
	Mode      string
	ModeValue int
	WordList  string
	WPM       float64
	Accuracy  float64
	Timestamp time.Time
}

type Store struct {
	path  string
	Tests []TestRecord
}

type fileFormat struct {
	Tests []TestRecord `json:"tests"`
}

func NewStore(path string) *Store {
	return &Store{
		path:  path,
		Tests: nil,
	}
}

func (s *Store) Load() error {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			s.Tests = []TestRecord{}
			return nil
		}
		return err
	}

	var f fileFormat
	if err := json.Unmarshal(data, &f); err != nil {
		return err
	}
	s.Tests = f.Tests
	return nil
}

func (s *Store) Save(record TestRecord) error {
	s.Tests = append(s.Tests, record)
	return s.writeToDisk()
}

func (s *Store) writeToDisk() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0755); err != nil {
		return err
	}
	f := fileFormat{Tests: s.Tests}
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0644)
}

func (s *Store) RecentTests(n int) []TestRecord {
	if n <= 0 {
		return []TestRecord{}
	}
	total := len(s.Tests)
	if n > total {
		n = total
	}
	result := make([]TestRecord, n)
	for i := range n {
		result[i] = s.Tests[total-1-i]
	}
	return result
}

func (s *Store) PersonalBest(mode string, modeValue int, wordList string) *PersonalBest {
	var best *TestRecord
	for i := range s.Tests {
		r := &s.Tests[i]
		if r.Mode != mode || r.ModeValue != modeValue || r.WordList != wordList {
			continue
		}
		if best == nil || r.WPM > best.WPM {
			best = r
		}
	}
	if best == nil {
		return nil
	}
	return &PersonalBest{
		Mode:      best.Mode,
		ModeValue: best.ModeValue,
		WordList:  best.WordList,
		WPM:       best.WPM,
		Accuracy:  best.Accuracy,
		Timestamp: best.Timestamp,
	}
}

func (s *Store) IsPersonalBest(record TestRecord) bool {
	pb := s.PersonalBest(record.Mode, record.ModeValue, record.WordList)
	if pb == nil {
		return true
	}
	return record.WPM > pb.WPM
}

func (s *Store) TotalTests() int {
	return len(s.Tests)
}

func (s *Store) AverageWPM() float64 {
	if len(s.Tests) == 0 {
		return 0
	}
	var sum float64
	for _, r := range s.Tests {
		sum += r.WPM
	}
	return sum / float64(len(s.Tests))
}

func (s *Store) AverageAccuracy() float64 {
	if len(s.Tests) == 0 {
		return 0
	}
	var sum float64
	for _, r := range s.Tests {
		sum += r.Accuracy
	}
	return sum / float64(len(s.Tests))
}
