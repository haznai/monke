package dataset

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadWordList(t *testing.T) {
	wl, err := LoadWordList("testdata/english_test.json")
	if err != nil {
		t.Fatalf("LoadWordList returned error: %v", err)
	}
	if wl.Name != "english_test" {
		t.Errorf("Name = %q, want %q", wl.Name, "english_test")
	}
	if !wl.OrderedByFrequency {
		t.Error("OrderedByFrequency = false, want true")
	}
	if len(wl.Words) != 10 {
		t.Errorf("len(Words) = %d, want 10", len(wl.Words))
	}
	if wl.Words[0] != "the" {
		t.Errorf("Words[0] = %q, want %q", wl.Words[0], "the")
	}
	if wl.Words[9] != "that" {
		t.Errorf("Words[9] = %q, want %q", wl.Words[9], "that")
	}
}

func TestLoadWordListInvalidJSON(t *testing.T) {
	_, err := LoadWordList("testdata/invalid.json")
	if err == nil {
		t.Fatal("LoadWordList with invalid JSON should return error")
	}
}

func TestLoadWordListMissingFile(t *testing.T) {
	_, err := LoadWordList("testdata/does_not_exist.json")
	if err == nil {
		t.Fatal("LoadWordList with missing file should return error")
	}
}

func TestLoadQuotes(t *testing.T) {
	qc, err := LoadQuotes("testdata/quotes_test.json")
	if err != nil {
		t.Fatalf("LoadQuotes returned error: %v", err)
	}
	if qc.Language != "english" {
		t.Errorf("Language = %q, want %q", qc.Language, "english")
	}
	if len(qc.Groups) != 4 {
		t.Errorf("len(Groups) = %d, want 4", len(qc.Groups))
	}
	if len(qc.Quotes) != 5 {
		t.Errorf("len(Quotes) = %d, want 5", len(qc.Quotes))
	}
	if qc.Quotes[0].Text != "Short quote here." {
		t.Errorf("Quotes[0].Text = %q, want %q", qc.Quotes[0].Text, "Short quote here.")
	}
	if qc.Quotes[0].Source != "Author A" {
		t.Errorf("Quotes[0].Source = %q, want %q", qc.Quotes[0].Source, "Author A")
	}
	if qc.Quotes[0].ID != 1 {
		t.Errorf("Quotes[0].ID = %d, want 1", qc.Quotes[0].ID)
	}
	if qc.Quotes[0].Length != 17 {
		t.Errorf("Quotes[0].Length = %d, want 17", qc.Quotes[0].Length)
	}
}

func TestLoadQuotesInvalidJSON(t *testing.T) {
	_, err := LoadQuotes("testdata/invalid.json")
	if err == nil {
		t.Fatal("LoadQuotes with invalid JSON should return error")
	}
}

func TestRandomWordsCount(t *testing.T) {
	wl, err := LoadWordList("testdata/english_test.json")
	if err != nil {
		t.Fatalf("LoadWordList returned error: %v", err)
	}

	for _, n := range []int{1, 5, 10} {
		words := wl.RandomWords(n)
		if len(words) != n {
			t.Errorf("RandomWords(%d) returned %d words, want %d", n, len(words), n)
		}
	}
}

func TestRandomWordsExistInList(t *testing.T) {
	wl, err := LoadWordList("testdata/english_test.json")
	if err != nil {
		t.Fatalf("LoadWordList returned error: %v", err)
	}

	wordSet := make(map[string]bool)
	for _, w := range wl.Words {
		wordSet[w] = true
	}

	words := wl.RandomWords(50)
	for i, w := range words {
		if !wordSet[w] {
			t.Errorf("RandomWords returned %q at index %d, which is not in the word list", w, i)
		}
	}
}

func TestRandomWordsExceedingListLength(t *testing.T) {
	wl, err := LoadWordList("testdata/english_test.json")
	if err != nil {
		t.Fatalf("LoadWordList returned error: %v", err)
	}

	// List has 10 words, ask for 25. Should still return 25 words.
	words := wl.RandomWords(25)
	if len(words) != 25 {
		t.Errorf("RandomWords(25) returned %d words, want 25", len(words))
	}

	wordSet := make(map[string]bool)
	for _, w := range wl.Words {
		wordSet[w] = true
	}
	for i, w := range words {
		if !wordSet[w] {
			t.Errorf("RandomWords returned %q at index %d, which is not in the word list", w, i)
		}
	}
}

func TestRandomWordsZero(t *testing.T) {
	wl, err := LoadWordList("testdata/english_test.json")
	if err != nil {
		t.Fatalf("LoadWordList returned error: %v", err)
	}

	words := wl.RandomWords(0)
	if len(words) != 0 {
		t.Errorf("RandomWords(0) returned %d words, want 0", len(words))
	}
}

func TestRandomQuoteShort(t *testing.T) {
	qc, err := LoadQuotes("testdata/quotes_test.json")
	if err != nil {
		t.Fatalf("LoadQuotes returned error: %v", err)
	}

	q, err := qc.RandomQuote(QuoteShort)
	if err != nil {
		t.Fatalf("RandomQuote(QuoteShort) returned error: %v", err)
	}
	if len(q.Text) > 100 {
		t.Errorf("QuoteShort returned quote with length %d (> 100)", len(q.Text))
	}
}

func TestRandomQuoteMedium(t *testing.T) {
	qc, err := LoadQuotes("testdata/quotes_test.json")
	if err != nil {
		t.Fatalf("LoadQuotes returned error: %v", err)
	}

	q, err := qc.RandomQuote(QuoteMedium)
	if err != nil {
		t.Fatalf("RandomQuote(QuoteMedium) returned error: %v", err)
	}
	textLen := len(q.Text)
	if textLen < 101 || textLen > 300 {
		t.Errorf("QuoteMedium returned quote with length %d (want 101-300)", textLen)
	}
}

func TestRandomQuoteLong(t *testing.T) {
	qc, err := LoadQuotes("testdata/quotes_test.json")
	if err != nil {
		t.Fatalf("LoadQuotes returned error: %v", err)
	}

	q, err := qc.RandomQuote(QuoteLong)
	if err != nil {
		t.Fatalf("RandomQuote(QuoteLong) returned error: %v", err)
	}
	textLen := len(q.Text)
	if textLen < 301 || textLen > 600 {
		t.Errorf("QuoteLong returned quote with length %d (want 301-600)", textLen)
	}
}

func TestRandomQuoteThicc(t *testing.T) {
	qc, err := LoadQuotes("testdata/quotes_test.json")
	if err != nil {
		t.Fatalf("LoadQuotes returned error: %v", err)
	}

	q, err := qc.RandomQuote(QuoteThicc)
	if err != nil {
		t.Fatalf("RandomQuote(QuoteThicc) returned error: %v", err)
	}
	if len(q.Text) < 601 {
		t.Errorf("QuoteThicc returned quote with length %d (want 601+)", len(q.Text))
	}
}

func TestRandomQuoteNoMatchReturnsError(t *testing.T) {
	// Create a collection with only short quotes, then ask for thicc.
	qc := &QuoteCollection{
		Language: "english",
		Groups:   [][]int{{0, 100}},
		Quotes: []Quote{
			{Text: "Short.", Source: "Test", ID: 1, Length: 6},
		},
	}

	_, err := qc.RandomQuote(QuoteThicc)
	if err == nil {
		t.Fatal("RandomQuote should return error when no quotes match the length filter")
	}
}

func TestQuoteWords(t *testing.T) {
	qc, err := LoadQuotes("testdata/quotes_test.json")
	if err != nil {
		t.Fatalf("LoadQuotes returned error: %v", err)
	}

	q := &qc.Quotes[0] // "Short quote here."
	words := qc.QuoteWords(q)

	expected := []string{"short", "quote", "here."}
	if len(words) != len(expected) {
		t.Fatalf("QuoteWords returned %d words, want %d", len(words), len(expected))
	}
	for i, w := range words {
		if w != expected[i] {
			t.Errorf("QuoteWords[%d] = %q, want %q", i, w, expected[i])
		}
	}
}

func TestQuoteWordsMultipleSpaces(t *testing.T) {
	qc := &QuoteCollection{}
	q := &Quote{Text: "hello   world  test"}
	words := qc.QuoteWords(q)

	// strings.Fields handles multiple spaces correctly
	expected := []string{"hello", "world", "test"}
	if len(words) != len(expected) {
		t.Fatalf("QuoteWords returned %d words, want %d", len(words), len(expected))
	}
	for i, w := range words {
		if w != expected[i] {
			t.Errorf("QuoteWords[%d] = %q, want %q", i, w, expected[i])
		}
	}
}

func TestAvailableWordLists(t *testing.T) {
	lists := AvailableWordLists()
	expected := []string{"english", "english_1k", "english_5k", "english_10k"}

	if len(lists) != len(expected) {
		t.Fatalf("AvailableWordLists returned %d items, want %d", len(lists), len(expected))
	}
	for i, name := range lists {
		if name != expected[i] {
			t.Errorf("AvailableWordLists[%d] = %q, want %q", i, name, expected[i])
		}
	}
}

func TestFetchAndCache(t *testing.T) {
	// Set up a test HTTP server that serves fake word list and quote data.
	wordListData := WordList{
		Name:               "english",
		OrderedByFrequency: true,
		Words:              []string{"alpha", "bravo", "charlie"},
	}
	quoteData := QuoteCollection{
		Language: "english",
		Groups:   [][]int{{0, 100}},
		Quotes: []Quote{
			{Text: "Test quote.", Source: "Tester", ID: 1, Length: 11},
		},
	}

	wordListJSON, _ := json.Marshal(wordListData)
	quoteJSON, _ := json.Marshal(quoteData)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		switch {
		case strings.Contains(path, "languages/english.json"):
			w.Write(wordListJSON)
		case strings.Contains(path, "languages/english_1k.json"):
			wl := wordListData
			wl.Name = "english_1k"
			b, _ := json.Marshal(wl)
			w.Write(b)
		case strings.Contains(path, "languages/english_5k.json"):
			wl := wordListData
			wl.Name = "english_5k"
			b, _ := json.Marshal(wl)
			w.Write(b)
		case strings.Contains(path, "languages/english_10k.json"):
			wl := wordListData
			wl.Name = "english_10k"
			b, _ := json.Marshal(wl)
			w.Write(b)
		case strings.Contains(path, "quotes/english.json"):
			w.Write(quoteJSON)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	dataDir := t.TempDir()

	err := FetchAndCacheFrom(srv.URL, dataDir)
	if err != nil {
		t.Fatalf("FetchAndCache returned error: %v", err)
	}

	// Verify all expected files exist.
	expectedFiles := []string{
		"english.json",
		"english_1k.json",
		"english_5k.json",
		"english_10k.json",
		"quotes_english.json",
	}
	for _, fname := range expectedFiles {
		fpath := filepath.Join(dataDir, fname)
		if _, err := os.Stat(fpath); os.IsNotExist(err) {
			t.Errorf("expected file %q does not exist after FetchAndCache", fname)
		}
	}

	// Verify the content of one word list file.
	data, err := os.ReadFile(filepath.Join(dataDir, "english.json"))
	if err != nil {
		t.Fatalf("failed to read cached english.json: %v", err)
	}
	var cached WordList
	if err := json.Unmarshal(data, &cached); err != nil {
		t.Fatalf("cached english.json is not valid JSON: %v", err)
	}
	if cached.Name != "english" {
		t.Errorf("cached word list name = %q, want %q", cached.Name, "english")
	}
	if len(cached.Words) != 3 {
		t.Errorf("cached word list has %d words, want 3", len(cached.Words))
	}
}

func TestLoadCached(t *testing.T) {
	// Build a temp directory with all necessary files.
	dataDir := t.TempDir()

	wordLists := map[string]WordList{
		"english":     {Name: "english", OrderedByFrequency: true, Words: []string{"one", "two"}},
		"english_1k":  {Name: "english_1k", OrderedByFrequency: true, Words: []string{"three", "four"}},
		"english_5k":  {Name: "english_5k", OrderedByFrequency: true, Words: []string{"five", "six"}},
		"english_10k": {Name: "english_10k", OrderedByFrequency: true, Words: []string{"seven", "eight"}},
	}
	for name, wl := range wordLists {
		data, _ := json.Marshal(wl)
		os.WriteFile(filepath.Join(dataDir, name+".json"), data, 0644)
	}

	quotes := QuoteCollection{
		Language: "english",
		Groups:   [][]int{{0, 100}},
		Quotes:   []Quote{{Text: "Hello world.", Source: "Test", ID: 1, Length: 12}},
	}
	qdata, _ := json.Marshal(quotes)
	os.WriteFile(filepath.Join(dataDir, "quotes_english.json"), qdata, 0644)

	store, err := LoadCached(dataDir)
	if err != nil {
		t.Fatalf("LoadCached returned error: %v", err)
	}

	if len(store.WordLists) != 4 {
		t.Errorf("Store.WordLists has %d entries, want 4", len(store.WordLists))
	}
	for _, name := range AvailableWordLists() {
		wl, ok := store.WordLists[name]
		if !ok {
			t.Errorf("Store.WordLists missing %q", name)
			continue
		}
		if wl.Name != name {
			t.Errorf("WordList name = %q, want %q", wl.Name, name)
		}
	}

	if store.Quotes == nil {
		t.Fatal("Store.Quotes is nil")
	}
	if store.Quotes.Language != "english" {
		t.Errorf("Store.Quotes.Language = %q, want %q", store.Quotes.Language, "english")
	}
	if len(store.Quotes.Quotes) != 1 {
		t.Errorf("Store.Quotes has %d quotes, want 1", len(store.Quotes.Quotes))
	}
}

func TestLoadCachedMissingDir(t *testing.T) {
	_, err := LoadCached("/nonexistent/path/that/does/not/exist")
	if err == nil {
		t.Fatal("LoadCached with missing directory should return error")
	}
}
