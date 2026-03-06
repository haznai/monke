package app

import (
	"fmt"
	"strings"

	"github.com/hazn/monkeytype-tui/internal/llm"
	"github.com/hazn/monkeytype-tui/internal/stats"
	"github.com/hazn/monkeytype-tui/internal/theme"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// restartMsg signals user wants to restart with same config
type restartMsg struct{}

// menuMsg signals user wants to go back to menu
type menuMsg struct{}

// ResultsModel displays test results
type ResultsModel struct {
	result      stats.TestResult
	config      TestConfig
	isPB        bool
	width       int
	height      int
	targetWords []string
	typedWords  []string
	llmResult   *llm.Result
	llmErr      error
	llmLoading  bool
}

func NewResultsModel(result stats.TestResult, config TestConfig, isPB bool, typedWords, targetWords []string, width, height int) ResultsModel {
	return ResultsModel{
		result:      result,
		config:      config,
		isPB:        isPB,
		targetWords: targetWords,
		typedWords:  typedWords,
		llmLoading:  true,
		width:       width,
		height:      height,
	}
}

func (m *ResultsModel) SetSpellcheck(result *llm.Result, err error) {
	m.llmLoading = false
	m.llmResult = result
	m.llmErr = err
}

func (m ResultsModel) Init() tea.Cmd {
	return nil
}

func (m ResultsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		// Block input while waiting for LLM
		if m.llmLoading {
			return m, nil
		}
		switch msg.String() {
		case "tab":
			return m, func() tea.Msg { return restartMsg{} }
		case "enter":
			return m, func() tea.Msg { return restartMsg{} }
		case "esc":
			return m, func() tea.Msg { return menuMsg{} }
		}
	}

	return m, nil
}

func (m ResultsModel) View() string {
	var b strings.Builder

	b.WriteString(theme.Title.Render("results"))
	b.WriteString("\n\n")

	// Wait for LLM before showing anything
	if m.llmLoading {
		b.WriteString(theme.DimText.Render("checking with llm..."))
		return lipgloss.NewStyle().
			Padding(1, 2).
			Width(min(m.width-4, 80)).
			Render(b.String())
	}

	// LLM failed or unavailable: fall back to raw results
	if m.llmErr != nil || m.llmResult == nil {
		b.WriteString(m.renderRawResults())
		b.WriteString("\n")
		b.WriteString(theme.DimText.Render("llm unavailable, showing raw results"))
		b.WriteString("\n\n")
		b.WriteString(m.renderFooter())
		return lipgloss.NewStyle().
			Padding(1, 2).
			Width(min(m.width-4, 80)).
			Render(b.String())
	}

	// LLM succeeded: check if all corrected words match target
	allCorrect, matchCount := m.llmMatchCount()
	minutes := m.result.Duration.Seconds() / 60.0

	if allCorrect {
		// WPM = total target chars / 5 / minutes (corrected WPM)
		wpm := m.result.CorrectedWPM
		b.WriteString(theme.StatValue.Render(fmt.Sprintf("%.0f", wpm)))
		b.WriteString("  ")
		b.WriteString(theme.StatLabel.Render("wpm"))
		if m.isPB {
			b.WriteString("  ")
			b.WriteString(theme.PassedText.Render("NEW PB!"))
		}
	} else {
		b.WriteString(theme.FailedText.Render(fmt.Sprintf("%d/%d correct after llm", matchCount, len(m.targetWords))))
		// Show what WPM would have been if all were right
		b.WriteString("\n")
		b.WriteString(theme.DimText.Render(fmt.Sprintf("%.0f wpm if perfect", m.result.CorrectedWPM)))

		// Also show real LLM WPM (only correct words counted)
		llmWPM := m.calcLLMWPM(matchCount, minutes)
		b.WriteString(fmt.Sprintf("  %.0f wpm actual", llmWPM))
	}
	b.WriteString("\n")

	// Duration
	b.WriteString(theme.DimText.Render(fmt.Sprintf("%.1fs", m.result.Duration.Seconds())))
	b.WriteString("\n\n")

	// Corrections list (if any)
	if len(m.llmResult.Corrections) > 0 {
		b.WriteString(theme.DimText.Render(fmt.Sprintf("%d corrected", len(m.llmResult.Corrections))))
		b.WriteString("\n")
		for _, c := range m.llmResult.Corrections {
			b.WriteString(fmt.Sprintf("  %s %s %s\n",
				theme.IncorrectWord.Render(c.Original),
				theme.DimText.Render("->"),
				theme.PassedText.Render(c.Fixed),
			))
		}
		b.WriteString("\n")
	}

	b.WriteString(m.renderFooter())

	return lipgloss.NewStyle().
		Padding(1, 2).
		Width(min(m.width-4, 80)).
		Render(b.String())
}

// renderRawResults shows results without LLM (fallback)
func (m ResultsModel) renderRawResults() string {
	if m.result.Passed {
		return fmt.Sprintf("%s  %s",
			theme.StatValue.Render(fmt.Sprintf("%.0f", m.result.CorrectedWPM)),
			theme.StatLabel.Render("wpm"),
		)
	}
	return theme.FailedText.Render(fmt.Sprintf("%d/%d correct", m.result.CorrectWords, m.result.TotalWords))
}

func (m ResultsModel) renderFooter() string {
	return theme.FooterStyle.Render(fmt.Sprintf(
		"%s restart  %s next  %s menu",
		theme.FooterKey.Render("tab"),
		theme.FooterKey.Render("enter"),
		theme.FooterKey.Render("esc"),
	))
}

// llmMatchCount returns whether all LLM-corrected words match targets,
// and the count of matches.
func (m ResultsModel) llmMatchCount() (allCorrect bool, count int) {
	for i, cw := range m.llmResult.CorrectedWords {
		if i < len(m.targetWords) && cw == m.targetWords[i] {
			count++
		}
	}
	return count == len(m.targetWords), count
}

// calcLLMWPM computes WPM from only the words the LLM got right.
func (m ResultsModel) calcLLMWPM(matchCount int, minutes float64) float64 {
	if minutes == 0 || m.llmResult == nil {
		return 0
	}

	n := len(m.llmResult.CorrectedWords)
	var correctChars float64
	for i, cw := range m.llmResult.CorrectedWords {
		if i >= len(m.targetWords) {
			break
		}
		if cw == m.targetWords[i] {
			space := 0.0
			if i < n-1 {
				space = 1.0
			}
			correctChars += float64(len(cw)) + space
		}
	}
	return (correctChars / 5.0) / minutes
}
