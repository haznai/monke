package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hazn/monkeytype-tui/internal/dataset"
	"github.com/hazn/monkeytype-tui/internal/history"
	"github.com/hazn/monkeytype-tui/internal/llm"
	"github.com/hazn/monkeytype-tui/internal/menu"
	"github.com/hazn/monkeytype-tui/internal/stats"
	"github.com/hazn/monkeytype-tui/internal/theme"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Screen int

const (
	ScreenLoading Screen = iota
	ScreenMenu
	ScreenTyping
	ScreenResults
)

type TestConfig struct {
	Mode     string // "words", "time", "quote"
	Value    int    // word count or seconds or QuoteLength
	WordList string // "english", "english_1k", etc.
}

// Messages
type datasetsLoadedMsg struct {
	store *dataset.Store
	err   error
}

type spellcheckMsg struct {
	result *llm.Result
	err    error
}

type Model struct {
	screen  Screen
	width   int
	height  int
	menu    menu.Model
	typing  *TypingModel
	results *ResultsModel
	config  TestConfig
	dataDir string
	store   *dataset.Store
	history *history.Store
	err     string
}

func New() Model {
	dataDir := defaultDataDir()
	histPath := filepath.Join(dataDir, "history.json")
	hist := history.NewStore(histPath)
	_ = hist.Load()

	return Model{
		screen:  ScreenLoading,
		menu:    menu.New(),
		dataDir: filepath.Join(dataDir, "datasets"),
		history: hist,
	}
}

func defaultDataDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".monkeytype-tui"
	}
	return filepath.Join(home, ".monkeytype-tui")
}

func (m Model) Init() tea.Cmd {
	return m.loadDatasets()
}

func (m Model) loadDatasets() tea.Cmd {
	dataDir := m.dataDir
	return func() tea.Msg {
		// Try loading cached first
		store, err := dataset.LoadCached(dataDir)
		if err == nil {
			return datasetsLoadedMsg{store: store}
		}

		// Fetch from GitHub
		if fetchErr := dataset.FetchAndCache(dataDir); fetchErr != nil {
			return datasetsLoadedMsg{err: fmt.Errorf("fetch failed: %w", fetchErr)}
		}

		store, err = dataset.LoadCached(dataDir)
		if err != nil {
			return datasetsLoadedMsg{err: fmt.Errorf("load after fetch failed: %w", err)}
		}
		return datasetsLoadedMsg{store: store}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}

	case datasetsLoadedMsg:
		if msg.err != nil {
			m.err = msg.err.Error()
			return m, nil
		}
		m.store = msg.store
		m.screen = ScreenMenu
		return m, nil
	}

	switch m.screen {
	case ScreenLoading:
		return m, nil
	case ScreenMenu:
		return m.updateMenu(msg)
	case ScreenTyping:
		return m.updateTyping(msg)
	case ScreenResults:
		return m.updateResults(msg)
	}

	return m, nil
}

func (m Model) View() string {
	var content string

	switch m.screen {
	case ScreenLoading:
		if m.err != "" {
			content = theme.FailedText.Render("Error: "+m.err) + "\n\n" +
				theme.DimText.Render("Press ctrl+c to quit")
		} else {
			content = theme.Title.Render("monkeytype-tui") + "\n\n" +
				theme.DimText.Render("loading datasets...")
		}
	case ScreenMenu:
		content = m.menu.View()
	case ScreenTyping:
		if m.typing != nil {
			content = m.typing.View()
		}
	case ScreenResults:
		if m.results != nil {
			content = m.results.View()
		}
	}

	return lipgloss.Place(
		m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		content,
	)
}

func (m Model) updateMenu(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "esc" || msg.String() == "q" {
			return m, tea.Quit
		}
	case menu.SelectMsg:
		m.config = TestConfig{
			Mode:     msg.Mode,
			Value:    msg.Value,
			WordList: msg.WordList,
		}
		return m.startTypingTest()
	}

	var cmd tea.Cmd
	m.menu, cmd = m.menu.Update(msg)
	return m, cmd
}

func (m Model) updateTyping(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.typing == nil {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "esc" {
			m.screen = ScreenMenu
			m.typing = nil
			return m, nil
		}
	case testFinishedMsg:
		return m.finishTest()
	}

	updated, cmd := m.typing.Update(msg)
	if tm, ok := updated.(TypingModel); ok {
		m.typing = &tm
	}
	return m, cmd
}

func (m Model) updateResults(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.results == nil {
		return m, nil
	}

	switch msg := msg.(type) {
	case restartMsg:
		return m.startTypingTest()
	case menuMsg:
		m.screen = ScreenMenu
		m.results = nil
		return m, nil
	case spellcheckMsg:
		m.results.SetSpellcheck(msg.result, msg.err)
		return m, nil
	default:
		updated, cmd := m.results.Update(msg)
		if rm, ok := updated.(ResultsModel); ok {
			m.results = &rm
		}
		return m, cmd
	}
}

func (m Model) startTypingTest() (Model, tea.Cmd) {
	var words []string

	switch m.config.Mode {
	case "words":
		wl, ok := m.store.WordLists[m.config.WordList]
		if !ok {
			m.err = "word list not found: " + m.config.WordList
			m.screen = ScreenMenu
			return m, nil
		}
		words = wl.RandomWords(m.config.Value)

	case "time":
		// Generate a large batch of words for time mode
		wl, ok := m.store.WordLists[m.config.WordList]
		if !ok {
			m.err = "word list not found: " + m.config.WordList
			m.screen = ScreenMenu
			return m, nil
		}
		words = wl.RandomWords(200) // generate plenty

	case "quote":
		if m.store.Quotes == nil {
			m.err = "no quotes loaded"
			m.screen = ScreenMenu
			return m, nil
		}
		q, err := m.store.Quotes.RandomQuote(dataset.QuoteLength(m.config.Value))
		if err != nil {
			m.err = err.Error()
			m.screen = ScreenMenu
			return m, nil
		}
		words = strings.Fields(q.Text)
	}

	tm := NewTypingModel(words, m.config, m.width, m.height)
	m.typing = &tm
	m.screen = ScreenTyping
	return m, nil
}

func (m Model) finishTest() (Model, tea.Cmd) {
	if m.typing == nil {
		return m, nil
	}

	engine := m.typing.engine
	wordStates := engine.Words()

	// Build stats input
	var wordResults []stats.WordResult
	var typedWords, targetWords []string
	for _, w := range wordStates {
		wordResults = append(wordResults, stats.WordResult{
			Target:  w.Target,
			Typed:   w.Typed,
			Correct: w.Correct,
		})
		typedWords = append(typedWords, w.Typed)
		targetWords = append(targetWords, w.Target)
	}

	duration := engine.ElapsedTime()
	if m.config.Mode == "time" {
		duration = time.Duration(m.config.Value) * time.Second
	}

	result := stats.Calculate(stats.TestInput{
		Words:      wordResults,
		Duration:   duration,
		WPMSamples: engine.Snapshots(),
	})

	// Check personal best before saving
	record := history.TestRecord{
		Timestamp:    time.Now(),
		Mode:         m.config.Mode,
		ModeValue:    m.config.Value,
		WordList:     m.config.WordList,
		WPM:          result.WPM,
		RawWPM:       result.RawWPM,
		CorrectedWPM: result.CorrectedWPM,
		Accuracy:     result.Accuracy,
		Consistency:  result.Consistency,
		CorrectWords: result.CorrectWords,
		TotalWords:   result.TotalWords,
		Passed:       result.Passed,
		DurationSecs: duration.Seconds(),
		TypedWords:   typedWords,
		TargetWords:  targetWords,
	}

	isPB := m.history.IsPersonalBest(record)
	_ = m.history.Save(record)

	rm := NewResultsModel(result, m.config, isPB, typedWords, targetWords, m.width, m.height)
	m.results = &rm
	m.typing = nil
	m.screen = ScreenResults

	// Fire async LLM spellcheck
	spellcheckCmd := func() tea.Msg {
		res, err := llm.Spellcheck(typedWords)
		return spellcheckMsg{result: res, err: err}
	}

	return m, spellcheckCmd
}
