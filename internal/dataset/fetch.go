package dataset

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

const baseURL = "https://raw.githubusercontent.com/monkeytypegame/monkeytype/master/frontend/static"

var wordListFiles = []struct {
	name     string
	urlPath  string
	cacheAs  string
}{
	{"english", "/languages/english.json", "english.json"},
	{"english_1k", "/languages/english_1k.json", "english_1k.json"},
	{"english_5k", "/languages/english_5k.json", "english_5k.json"},
	{"english_10k", "/languages/english_10k.json", "english_10k.json"},
}

var quoteFiles = []struct {
	urlPath string
	cacheAs string
}{
	{"/quotes/english.json", "quotes_english.json"},
}

// FetchAndCache downloads all datasets from MonkeyType's GitHub repo and
// saves them to dataDir.
func FetchAndCache(dataDir string) error {
	return FetchAndCacheFrom(baseURL, dataDir)
}

// FetchAndCacheFrom is like FetchAndCache but uses a custom base URL.
// Exported for testing with httptest.NewServer.
func FetchAndCacheFrom(base string, dataDir string) error {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return fmt.Errorf("creating data directory: %w", err)
	}

	for _, wl := range wordListFiles {
		url := base + wl.urlPath
		dest := filepath.Join(dataDir, wl.cacheAs)
		if err := downloadFile(url, dest); err != nil {
			return fmt.Errorf("fetching word list %s: %w", wl.name, err)
		}
	}

	for _, q := range quoteFiles {
		url := base + q.urlPath
		dest := filepath.Join(dataDir, q.cacheAs)
		if err := downloadFile(url, dest); err != nil {
			return fmt.Errorf("fetching quotes: %w", err)
		}
	}

	return nil
}

func downloadFile(url, dest string) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("GET %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: status %d", url, resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response from %s: %w", url, err)
	}

	if err := os.WriteFile(dest, data, 0644); err != nil {
		return fmt.Errorf("writing %s: %w", dest, err)
	}

	return nil
}
