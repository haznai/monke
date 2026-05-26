package app

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/hazn/monkeytype-tui/internal/appdir"
	"github.com/hazn/monkeytype-tui/internal/dataset"
	"github.com/hazn/monkeytype-tui/internal/history"
	"github.com/hazn/monkeytype-tui/internal/llm"
	"github.com/hazn/monkeytype-tui/internal/llmlog"
	"github.com/hazn/monkeytype-tui/internal/menu"
	"github.com/hazn/monkeytype-tui/internal/stats"
	"github.com/hazn/monkeytype-tui/internal/theme"
	"github.com/hazn/monkeytype-tui/internal/typing"

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
	Mode        string // "words", "time", "quote", "ngram"
	Value       int    // word count, seconds, QuoteLength, or ngram scope
	WordList    string // "english", "english_1k", etc.
	Scope       int    // top N ngrams to use (ngram mode only)
	NgramLesson int    // current lesson (1-indexed, for display)
	NgramTotal  int    // total number of lessons
}

// Ngram progression has two independent gates: at least 100 WPM and every
// submitted ngram must exactly match the target. Fast garbage does not advance.
const ngramWPMThreshold = 100.0

// Messages
type datasetsLoadedMsg struct {
	store *dataset.Store
	err   error
}

type spellcheckMsg struct {
	result *llm.Result
	err    error
	logErr error
}

type Model struct {
	screen         Screen
	width          int
	height         int
	menu           menu.Model
	typing         *TypingModel
	results        *ResultsModel
	config         TestConfig
	dataDir        string
	store          *dataset.Store
	history        *history.Store
	llmLog         *llmlog.Store
	err            string
	ngramLessons   [][]string
	ngramLessonIdx int
}

func New() Model {
	_ = appdir.MigrateLegacyData()

	hist := history.NewStore(filepath.Join(defaultDataDir(), "history.json"))
	_ = hist.Load()

	return Model{
		screen:  ScreenLoading,
		menu:    menu.New(),
		dataDir: filepath.Join(defaultDataDir(), "datasets"),
		history: hist,
		llmLog:  llmlog.NewStore(defaultLLMLogPath()),
	}
}

func defaultDataDir() string {
	return appdir.Root()
}

func defaultLLMLogPath() string {
	return appdir.LLMLogPath()
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
			content = theme.Title.Render("monke") + "\n\n" +
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
			Mode:  msg.Mode,
			Value: msg.Value,
			Scope: msg.Scope,
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
		m.results.SetSpellcheck(msg.result, msg.err, msg.logErr)
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
		words = strings.Fields(strings.ToLower(q.Text))

	case "ngram":
		scope := m.config.Scope
		if scope == 0 {
			scope = m.config.Value
		}
		if scope == 0 {
			scope = 50
		}
		m.config.Scope = scope
		m.ngramLessons = dataset.GenerateNgramLessons(dataset.Bigrams, scope, 2, 3)
		m.ngramLessonIdx = 0
		m.config.NgramLesson = 1
		m.config.NgramTotal = len(m.ngramLessons)
		words = m.ngramLessons[0]
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

	// Ngram mode: check WPM threshold and correctness, advance or retry.
	if m.config.Mode == "ngram" {
		engine := m.typing.engine
		duration := engine.ElapsedTime()
		totalChars := engine.TotalTypedChars()
		wpm := float64(totalChars) / 5.0 / duration.Minutes()

		if ngramLessonPassed(engine.Words(), wpm) {
			m.ngramLessonIdx++
			if m.ngramLessonIdx < len(m.ngramLessons) {
				m.config.NgramLesson = m.ngramLessonIdx + 1
				words := m.ngramLessons[m.ngramLessonIdx]
				tm := NewTypingModel(words, m.config, m.width, m.height)
				tm.lastWPM = wpm
				m.typing = &tm
				return m, nil
			}
			// All lessons done, fall through to results
		} else {
			// Failed, restart same lesson
			tm := NewTypingModel(m.ngramLessons[m.ngramLessonIdx], m.config, m.width, m.height)
			tm.lastWPM = wpm
			m.typing = &tm
			return m, nil
		}
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

	spellcheckCmd := spellcheckAndLogCmd(m.llmLog, m.config, result, typedWords, targetWords)

	return m, spellcheckCmd
}

func spellcheckAndLogCmd(store *llmlog.Store, config TestConfig, result stats.TestResult, typedWords, targetWords []string) tea.Cmd {
	return func() tea.Msg {
		requestInfo := llm.SpellcheckRequestInfo(typedWords)
		started := time.Now()
		res, err := llm.Spellcheck(typedWords)
		latency := time.Since(started)

		logErr := saveLLMCall(store, llmLogInput{
			started:     started,
			latency:     latency,
			requestInfo: requestInfo,
			config:      config,
			result:      result,
			typedWords:  typedWords,
			targetWords: targetWords,
			llmResult:   res,
			llmErr:      err,
		})

		return spellcheckMsg{result: res, err: err, logErr: logErr}
	}
}

type llmLogInput struct {
	started     time.Time
	latency     time.Duration
	requestInfo llm.RequestInfo
	config      TestConfig
	result      stats.TestResult
	typedWords  []string
	targetWords []string
	llmResult   *llm.Result
	llmErr      error
}

func saveLLMCall(store *llmlog.Store, input llmLogInput) error {
	var correctedWords []string
	rawCorrected := ""
	wordCountMismatch := false
	responseID := ""
	responseModel := ""
	promptTokens := 0
	completionTokens := 0
	totalTokens := 0
	if input.llmResult != nil {
		correctedWords = input.llmResult.CorrectedWords
		rawCorrected = input.llmResult.RawCorrectedText
		wordCountMismatch = input.llmResult.WordCountMismatch
		responseID = input.llmResult.ResponseID
		responseModel = input.llmResult.ResponseModel
		promptTokens = input.llmResult.PromptTokens
		completionTokens = input.llmResult.CompletionTokens
		totalTokens = input.llmResult.TotalTokens
	}

	record, err := llmlog.NewRecord(llmlog.RecordInput{
		CreatedAt:        input.started,
		Request:          input.requestInfo,
		ResponseID:       responseID,
		ResponseModel:    responseModel,
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		TotalTokens:      totalTokens,
		Test: llmlog.TestContext{
			Mode:         input.config.Mode,
			ModeValue:    input.config.Value,
			WordList:     input.config.WordList,
			NgramType:    "",
			NgramScope:   input.config.Scope,
			NgramLesson:  input.config.NgramLesson,
			NgramTotal:   input.config.NgramTotal,
			DurationSecs: input.result.Duration.Seconds(),
			WPM:          input.result.WPM,
			RawWPM:       input.result.RawWPM,
			CorrectedWPM: input.result.CorrectedWPM,
			Accuracy:     input.result.Accuracy,
			Consistency:  input.result.Consistency,
		},
		TargetWords:       input.targetWords,
		TypedWords:        input.typedWords,
		CorrectedWords:    correctedWords,
		RawCorrectedText:  rawCorrected,
		WordCountMismatch: wordCountMismatch,
		Latency:           input.latency,
		Err:               input.llmErr,
	})
	if err != nil {
		return err
	}
	return store.Save(record)
}

func ngramLessonPassed(words []typing.WordState, wpm float64) bool {
	if wpm < ngramWPMThreshold {
		return false
	}
	if len(words) == 0 {
		return false
	}
	for _, word := range words {
		if !word.Done || !word.Correct {
			return false
		}
	}
	return true
}
