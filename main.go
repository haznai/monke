package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/hazn/monkeytype-tui/internal/app"
	"github.com/hazn/monkeytype-tui/internal/dataset"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "fetch" {
		fetchDatasets()
		return
	}

	p := tea.NewProgram(
		app.New(),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func fetchDatasets() {
	home, _ := os.UserHomeDir()
	dataDir := filepath.Join(home, ".monkeytype-tui", "datasets")

	fmt.Println("Fetching datasets from MonkeyType GitHub...")
	if err := dataset.FetchAndCache(dataDir); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	store, err := dataset.LoadCached(dataDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading: %v\n", err)
		os.Exit(1)
	}

	for name, wl := range store.WordLists {
		fmt.Printf("  %s: %d words\n", name, len(wl.Words))
	}
	if store.Quotes != nil {
		fmt.Printf("  quotes: %d\n", len(store.Quotes.Quotes))
	}
	fmt.Println("Done!")
}
