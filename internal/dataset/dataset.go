package dataset

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"
)

type WordList struct {
	Name               string   `json:"name"`
	OrderedByFrequency bool     `json:"orderedByFrequency"`
	Words              []string `json:"words"`
}

type Quote struct {
	Text   string `json:"text"`
	Source string `json:"source"`
	ID     int    `json:"id"`
	Length int    `json:"length"`
}

type QuoteCollection struct {
	Language string  `json:"language"`
	Groups   [][]int `json:"groups"`
	Quotes   []Quote `json:"quotes"`
}

type QuoteLength int

const (
	QuoteShort  QuoteLength = iota // 0-100 chars
	QuoteMedium                    // 101-300
	QuoteLong                      // 301-600
	QuoteThicc                     // 601+
)

type Store struct {
	WordLists map[string]*WordList
	Quotes    *QuoteCollection
}

func LoadWordList(path string) (*WordList, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading word list %s: %w", path, err)
	}
	var wl WordList
	if err := json.Unmarshal(data, &wl); err != nil {
		return nil, fmt.Errorf("parsing word list %s: %w", path, err)
	}
	return &wl, nil
}

func LoadQuotes(path string) (*QuoteCollection, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading quotes %s: %w", path, err)
	}
	var qc QuoteCollection
	if err := json.Unmarshal(data, &qc); err != nil {
		return nil, fmt.Errorf("parsing quotes %s: %w", path, err)
	}
	return &qc, nil
}

// RandomWords picks n random words from the list. If n exceeds the list
// length, words are picked with wrapping (repeated random selection).
func (wl *WordList) RandomWords(n int) []string {
	if n <= 0 || len(wl.Words) == 0 {
		return []string{}
	}
	result := make([]string, n)
	for i := range n {
		result[i] = wl.Words[rand.IntN(len(wl.Words))]
	}
	return result
}

func quoteLengthRange(ql QuoteLength) (int, int) {
	switch ql {
	case QuoteShort:
		return 0, 100
	case QuoteMedium:
		return 101, 300
	case QuoteLong:
		return 301, 600
	case QuoteThicc:
		return 601, 9999
	default:
		return 0, 9999
	}
}

// RandomQuote returns a random quote whose text length falls within the given
// length category. Returns an error if no quotes match.
func (qc *QuoteCollection) RandomQuote(length QuoteLength) (*Quote, error) {
	lo, hi := quoteLengthRange(length)

	var candidates []int
	for i, q := range qc.Quotes {
		textLen := len(q.Text)
		if textLen >= lo && textLen <= hi {
			candidates = append(candidates, i)
		}
	}
	if len(candidates) == 0 {
		return nil, errors.New("no quotes found matching the requested length")
	}

	idx := candidates[rand.IntN(len(candidates))]
	return &qc.Quotes[idx], nil
}

// QuoteWords splits a quote's text into words using whitespace.
func (qc *QuoteCollection) QuoteWords(q *Quote) []string {
	return strings.Fields(strings.ToLower(q.Text))
}

// AvailableWordLists returns the names of all supported word lists.
func AvailableWordLists() []string {
	return []string{"english", "english_1k", "english_5k", "english_10k"}
}

// LoadCached loads all cached datasets from the given directory into a Store.
func LoadCached(dataDir string) (*Store, error) {
	store := &Store{
		WordLists: make(map[string]*WordList),
	}

	for _, name := range AvailableWordLists() {
		path := filepath.Join(dataDir, name+".json")
		wl, err := LoadWordList(path)
		if err != nil {
			return nil, fmt.Errorf("loading cached word list %s: %w", name, err)
		}
		store.WordLists[name] = wl
	}

	quotesPath := filepath.Join(dataDir, "quotes_english.json")
	qc, err := LoadQuotes(quotesPath)
	if err != nil {
		return nil, fmt.Errorf("loading cached quotes: %w", err)
	}
	store.Quotes = qc

	return store, nil
}
