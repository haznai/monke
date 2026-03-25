package app

import (
	"fmt"
	"strings"
	"unicode/utf8"

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

	if m.llmLoading {
		b.WriteString(theme.DimText.Render("checking with llm..."))
		return lipgloss.NewStyle().
			Padding(1, 2).
			Width(min(m.width-4, 80)).
			Render(b.String())
	}

	if m.llmErr != nil || m.llmResult == nil {
		b.WriteString(m.renderRawResults())
		b.WriteString("\n")
		b.WriteString(theme.DimText.Render("llm unavailable"))
		b.WriteString("\n\n")
		b.WriteString(m.renderFooter())
		return lipgloss.NewStyle().
			Padding(1, 2).
			Width(min(m.width-4, 80)).
			Render(b.String())
	}

	// Stats
	allCorrect, matchCount := m.llmMatchCount()
	minutes := m.result.Duration.Seconds() / 60.0

	if allCorrect {
		b.WriteString(theme.StatValue.Render(fmt.Sprintf("%.0f", m.result.CorrectedWPM)))
		b.WriteString("  ")
		b.WriteString(theme.StatLabel.Render("wpm"))
		if m.isPB {
			b.WriteString("  ")
			b.WriteString(theme.PassedText.Render("NEW PB!"))
		}
	} else {
		b.WriteString(theme.FailedText.Render(fmt.Sprintf("%d/%d", matchCount, len(m.targetWords))))
		b.WriteString("  ")
		b.WriteString(theme.StatLabel.Render("correct after llm"))
		b.WriteString("\n")
		llmWPM := m.calcLLMWPM(matchCount, minutes)
		b.WriteString(theme.DimText.Render(fmt.Sprintf("%.0f wpm effective  %.0f wpm if perfect", llmWPM, m.result.CorrectedWPM)))
	}
	b.WriteString("\n")
	b.WriteString(theme.DimText.Render(fmt.Sprintf("%.1fs", m.result.Duration.Seconds())))
	b.WriteString("\n\n")

	// Inline diff: corrected text colored by word status
	b.WriteString(m.renderInlineDiff())
	b.WriteString("\n\n")

	// Correction details
	corrections := m.renderCorrections()
	if corrections != "" {
		b.WriteString(corrections)
		b.WriteString("\n")
	}

	b.WriteString(m.renderFooter())

	return lipgloss.NewStyle().
		Padding(1, 2).
		Width(min(m.width-4, 80)).
		Render(b.String())
}

// renderInlineDiff shows the corrected text flowing inline, word by word.
// dim = typed correctly, green = LLM fixed, red = still wrong after LLM.
func (m ResultsModel) renderInlineDiff() string {
	maxWidth := min(m.width-8, 76)

	var lines []string
	var currentLine strings.Builder
	lineWidth := 0

	for i, target := range m.targetWords {
		typed := ""
		if i < len(m.typedWords) {
			typed = m.typedWords[i]
		}
		corrected := typed
		if i < len(m.llmResult.CorrectedWords) {
			corrected = m.llmResult.CorrectedWords[i]
		}

		// Color indicates status: dim = typed correctly, green = LLM fixed, red = still wrong.
		// Red words show what you actually typed so you can see your typos.
		var rendered string
		var displayWord string
		if typed == target {
			displayWord = target
			rendered = theme.DimText.Render(displayWord)
		} else if corrected == target {
			displayWord = target
			rendered = theme.PassedText.Render(displayWord)
		} else {
			displayWord = typed
			if displayWord == "" {
				displayWord = target // fallback if nothing was typed
			}
			rendered = theme.FailedText.Render(displayWord)
		}
		wordWidth := utf8.RuneCountInString(displayWord)

		if lineWidth > 0 && lineWidth+1+wordWidth > maxWidth {
			lines = append(lines, currentLine.String())
			currentLine.Reset()
			lineWidth = 0
		}
		if lineWidth > 0 {
			currentLine.WriteString(" ")
			lineWidth++
		}
		currentLine.WriteString(rendered)
		lineWidth += wordWidth
	}

	if currentLine.Len() > 0 {
		lines = append(lines, currentLine.String())
	}

	return strings.Join(lines, "\n")
}

type correctionEntry struct {
	rendered string
	width    int
}

// renderCorrections shows a compact, wrapped list of what the LLM changed.
// Fixed: typed → corrected (green). Wrong: typed → corrected (red) with target in parens.
func (m ResultsModel) renderCorrections() string {
	maxWidth := min(m.width-8, 72)
	var fixed, wrong []correctionEntry

	for i, target := range m.targetWords {
		typed := ""
		if i < len(m.typedWords) {
			typed = m.typedWords[i]
		}
		if typed == target {
			continue
		}

		corrected := typed
		if i < len(m.llmResult.CorrectedWords) {
			corrected = m.llmResult.CorrectedWords[i]
		}

		typedW := utf8.RuneCountInString(typed)

		if corrected == target {
			targetW := utf8.RuneCountInString(target)
			fixed = append(fixed, correctionEntry{
				rendered: fmt.Sprintf("%s %s %s",
					theme.DimText.Render(typed),
					theme.DimText.Render("→"),
					theme.PassedText.Render(target),
				),
				width: typedW + 3 + targetW,
			})
		} else if corrected == typed {
			// LLM didn't change it
			targetW := utf8.RuneCountInString(target)
			wrong = append(wrong, correctionEntry{
				rendered: fmt.Sprintf("%s %s",
					theme.FailedText.Render(typed),
					theme.DimText.Render("("+target+")"),
				),
				width: typedW + 1 + targetW + 2,
			})
		} else {
			// LLM changed but still wrong
			corrW := utf8.RuneCountInString(corrected)
			targetW := utf8.RuneCountInString(target)
			wrong = append(wrong, correctionEntry{
				rendered: fmt.Sprintf("%s %s %s %s",
					theme.DimText.Render(typed),
					theme.DimText.Render("→"),
					theme.FailedText.Render(corrected),
					theme.DimText.Render("("+target+")"),
				),
				width: typedW + 3 + corrW + 1 + targetW + 2,
			})
		}
	}

	if len(fixed) == 0 && len(wrong) == 0 {
		return ""
	}

	var b strings.Builder

	if len(fixed) > 0 {
		b.WriteString(theme.PassedText.Render(fmt.Sprintf("%d fixed", len(fixed))))
		b.WriteString("\n")
		b.WriteString(wrapCorrectionEntries(fixed, maxWidth))
		b.WriteString("\n")
	}
	if len(wrong) > 0 {
		if len(fixed) > 0 {
			b.WriteString("\n")
		}
		b.WriteString(theme.FailedText.Render(fmt.Sprintf("%d wrong", len(wrong))))
		b.WriteString("\n")
		b.WriteString(wrapCorrectionEntries(wrong, maxWidth))
		b.WriteString("\n")
	}

	return b.String()
}

func wrapCorrectionEntries(entries []correctionEntry, maxWidth int) string {
	var lines []string
	var currentLine strings.Builder
	lineWidth := 0

	for _, e := range entries {
		if lineWidth > 0 && lineWidth+2+e.width > maxWidth {
			lines = append(lines, "  "+currentLine.String())
			currentLine.Reset()
			lineWidth = 0
		}
		if lineWidth > 0 {
			currentLine.WriteString("  ")
			lineWidth += 2
		}
		currentLine.WriteString(e.rendered)
		lineWidth += e.width
	}
	if currentLine.Len() > 0 {
		lines = append(lines, "  "+currentLine.String())
	}

	return strings.Join(lines, "\n")
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
	for i, target := range m.targetWords {
		typed := ""
		if i < len(m.typedWords) {
			typed = m.typedWords[i]
		}
		corrected := typed
		if i < len(m.llmResult.CorrectedWords) {
			corrected = m.llmResult.CorrectedWords[i]
		}
		if typed == target || corrected == target {
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

	n := len(m.targetWords)
	var correctChars float64
	for i, target := range m.targetWords {
		typed := ""
		if i < len(m.typedWords) {
			typed = m.typedWords[i]
		}
		corrected := typed
		if i < len(m.llmResult.CorrectedWords) {
			corrected = m.llmResult.CorrectedWords[i]
		}
		if typed == target || corrected == target {
			space := 0.0
			if i < n-1 {
				space = 1.0
			}
			correctChars += float64(len(target)) + space
		}
	}
	return (correctChars / 5.0) / minutes
}
