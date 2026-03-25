package app

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/hazn/monkeytype-tui/internal/theme"
	"github.com/hazn/monkeytype-tui/internal/typing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// tickMsg is sent every second to update live stats
type tickMsg time.Time

// testFinishedMsg signals the typing test is complete
type testFinishedMsg struct{}

// TypingModel is the bubbletea model for the active typing test screen
type TypingModel struct {
	engine    *typing.Engine
	config    TestConfig
	width     int
	height    int
	liveWPM   float64
	timeLeft  int  // seconds remaining (time mode only)
	timerDone bool
	lastWPM   float64 // ngram mode: WPM from previous attempt
}

func NewTypingModel(words []string, config TestConfig, width, height int) TypingModel {
	return TypingModel{
		engine:   typing.NewEngine(words),
		config:   config,
		width:    width,
		height:   height,
		timeLeft: config.Value, // only meaningful for time mode
	}
}

func (m TypingModel) Init() tea.Cmd {
	return nil
}

func (m TypingModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tickMsg:
		if m.engine.IsFinished() || !m.engine.IsStarted() {
			return m, nil
		}

		// Update live WPM
		elapsed := m.engine.ElapsedTime()
		if elapsed > 0 {
			m.liveWPM = float64(m.engine.TotalTypedChars()) / 5.0 / elapsed.Minutes()
		}

		// Time mode countdown
		if m.config.Mode == "time" {
			remaining := m.config.Value - int(elapsed.Seconds())
			if remaining <= 0 {
				m.timeLeft = 0
				m.timerDone = true
				return m, func() tea.Msg { return testFinishedMsg{} }
			}
			m.timeLeft = remaining
		}

		// Take snapshot for consistency calculation
		m.engine.Snapshot()

		return m, tickCmd()

	case testFinishedMsg:
		// Propagate up to app model
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	return m, nil
}

func (m TypingModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.engine.IsFinished() || m.timerDone {
		return m, nil
	}

	key := msg.String()

	switch key {
	case "tab":
		// Restart
		m.engine.Reset()
		m.liveWPM = 0
		m.timeLeft = m.config.Value
		m.timerDone = false
		return m, nil

	case "esc":
		// Back to menu (handled by parent)
		return m, nil

	case "backspace":
		m.engine.Backspace()
		return m, nil

	case "ctrl+w":
		// Fallback for delete-word
		m.engine.DeleteWord()
		return m, nil

	case " ":
		wasStarted := m.engine.IsStarted()
		m.engine.Space()

		var cmds []tea.Cmd

		// Start tick on first action
		if !wasStarted && m.engine.IsStarted() {
			cmds = append(cmds, tickCmd())
		}

		// Check if test finished
		if m.engine.IsFinished() {
			cmds = append(cmds, func() tea.Msg { return testFinishedMsg{} })
		}

		return m, tea.Batch(cmds...)

	default:
		// Check for Cmd+Backspace (macOS sends this as a specific sequence)
		if msg.Type == tea.KeyRunes && len(msg.Runes) == 0 {
			return m, nil
		}

		// Handle Cmd+Backspace: different terminals send different sequences.
		// Common: 0x17 (ctrl+w), 0x15 (ctrl+u), or alt+backspace
		if key == "alt+backspace" {
			m.engine.DeleteWord()
			return m, nil
		}

		// Regular character input
		if msg.Type == tea.KeyRunes {
			for _, r := range msg.Runes {
				wasStarted := m.engine.IsStarted()
				m.engine.TypeChar(r)

				var cmds []tea.Cmd
				if !wasStarted && m.engine.IsStarted() {
					cmds = append(cmds, tickCmd())
				}
				if m.engine.IsFinished() {
					cmds = append(cmds, func() tea.Msg { return testFinishedMsg{} })
					return m, tea.Batch(cmds...)
				}
				if len(cmds) > 0 {
					return m, tea.Batch(cmds...)
				}
			}
		}
	}

	return m, nil
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m TypingModel) View() string {
	var b strings.Builder

	// Header
	header := m.renderHeader()
	b.WriteString(header)
	b.WriteString("\n\n")

	// Word display area
	wordDisplay := m.renderWords()
	b.WriteString(wordDisplay)
	b.WriteString("\n\n")

	// Footer with live stats
	footer := m.renderFooter()
	b.WriteString(footer)

	return lipgloss.NewStyle().
		Padding(1, 2).
		Width(min(m.width, 80)).
		Render(b.String())
}

func (m TypingModel) renderHeader() string {
	left := theme.Title.Render("monkeytype-tui")

	var right string
	switch m.config.Mode {
	case "words":
		right = theme.Subtitle.Render(fmt.Sprintf("words %d  %s", m.config.Value, m.config.WordList))
	case "time":
		right = theme.Subtitle.Render(fmt.Sprintf("time %ds  %s", m.config.Value, m.config.WordList))
	case "quote":
		right = theme.Subtitle.Render("quote")
	case "ngram":
		right = theme.Subtitle.Render(fmt.Sprintf("lesson %d/%d  %s  top %d",
			m.config.NgramLesson, m.config.NgramTotal, m.config.NgramType, m.config.Scope))
	}

	gap := max(0, min(m.width, 80)-10-lipgloss.Width(left)-lipgloss.Width(right))
	return left + strings.Repeat(" ", gap) + right
}

func (m TypingModel) renderWords() string {
	words := m.engine.Words()
	currentIdx := m.engine.CurrentWordIndex()
	currentInput := m.engine.CurrentInput()
	maxWidth := min(m.width, 80) - 10

	var lines []string
	var currentLine strings.Builder
	currentLineWidth := 0

	for i, w := range words {
		var rendered string

		if w.Done {
			// Already submitted word
			if w.Correct {
				rendered = theme.CompletedWord.Render(w.Target)
			} else {
				rendered = m.renderDoneWord(w.Target, w.Typed)
			}
		} else if i == currentIdx {
			// Currently typing this word
			rendered = m.renderCurrentWord(w.Target, currentInput)
		} else {
			// Future word
			rendered = theme.DimText.Render(w.Target)
		}

		// Line-breaking uses target width only, so overtyped chars
		// overflow instead of reflowing the layout mid-typing.
		wordWidth := utf8.RuneCountInString(w.Target)

		if currentLineWidth > 0 && currentLineWidth+1+wordWidth > maxWidth {
			lines = append(lines, currentLine.String())
			currentLine.Reset()
			currentLineWidth = 0
		}

		if currentLineWidth > 0 {
			// Cursor sits on the space after a fully-typed current word
			if i == currentIdx+1 && utf8.RuneCountInString(currentInput) >= utf8.RuneCountInString(words[currentIdx].Target) {
				currentLine.WriteString(theme.Cursor.Render(" "))
			} else {
				currentLine.WriteString(" ")
			}
			currentLineWidth++
		}

		currentLine.WriteString(rendered)
		currentLineWidth += wordWidth
	}

	if currentLine.Len() > 0 {
		lines = append(lines, currentLine.String())
	}

	// Find which line the current word is on
	currentLineIdx := m.findCurrentLine(words, currentIdx, maxWidth)

	// Show 3 lines starting from the line before current
	startLine := max(0, currentLineIdx-1)
	endLine := min(len(lines), startLine+3)

	visibleLines := lines[startLine:endLine]
	return strings.Join(visibleLines, "\n")
}

func (m TypingModel) findCurrentLine(words []typing.WordState, currentIdx int, maxWidth int) int {
	line := 0
	lineWidth := 0

	for i, w := range words {
		wordWidth := utf8.RuneCountInString(w.Target)

		if lineWidth > 0 && lineWidth+1+wordWidth > maxWidth {
			line++
			lineWidth = 0
		}

		if i == currentIdx {
			return line
		}

		if lineWidth > 0 {
			lineWidth++
		}
		lineWidth += wordWidth
	}

	return line
}

// renderDoneWord shows a submitted word with correct/incorrect char coloring
// but no cursor. Untyped chars render as incorrect (you skipped them).
func (m TypingModel) renderDoneWord(target, typed string) string {
	targetRunes := []rune(target)
	typedRunes := []rune(typed)
	var b strings.Builder

	for i, tr := range targetRunes {
		if i < len(typedRunes) {
			if typedRunes[i] == tr {
				b.WriteString(theme.CorrectChar.Render(string(tr)))
			} else {
				b.WriteString(theme.IncorrectChar.Render(string(tr)))
			}
		} else {
			// Untyped chars (space pressed early)
			b.WriteString(theme.IncorrectChar.Render(string(tr)))
		}
	}

	if len(typedRunes) > len(targetRunes) {
		for _, r := range typedRunes[len(targetRunes):] {
			b.WriteString(theme.IncorrectChar.Render(string(r)))
		}
	}

	return b.String()
}

func (m TypingModel) renderCurrentWord(target, input string) string {
	targetRunes := []rune(target)
	inputRunes := []rune(input)
	var b strings.Builder

	for i, tr := range targetRunes {
		if i < len(inputRunes) {
			if inputRunes[i] == tr {
				b.WriteString(theme.CorrectChar.Render(string(tr)))
			} else {
				b.WriteString(theme.IncorrectChar.Render(string(tr)))
			}
		} else if i == len(inputRunes) {
			// Cursor position
			b.WriteString(theme.Cursor.Render(string(tr)))
		} else {
			b.WriteString(theme.CurrentWord.Render(string(tr)))
		}
	}

	// Extra characters typed beyond target length
	if len(inputRunes) > len(targetRunes) {
		for _, r := range inputRunes[len(targetRunes):] {
			b.WriteString(theme.IncorrectChar.Render(string(r)))
		}
	}

	return b.String()
}

func (m TypingModel) renderFooter() string {
	var parts []string

	if m.engine.IsStarted() && m.config.Mode == "time" {
		parts = append(parts, fmt.Sprintf(
			"%s %s",
			theme.StatLabel.Render("time"),
			theme.StatValue.Render(fmt.Sprintf("%ds", m.timeLeft)),
		))
	}

	if m.config.Mode == "ngram" && m.lastWPM > 0 {
		parts = append(parts, fmt.Sprintf(
			"%s %s",
			theme.StatLabel.Render("last"),
			theme.DimText.Render(fmt.Sprintf("%.0f wpm", m.lastWPM)),
		))
	}

	keys := fmt.Sprintf(
		"%s restart  %s menu",
		theme.FooterKey.Render("tab"),
		theme.FooterKey.Render("esc"),
	)
	parts = append(parts, keys)

	return theme.FooterStyle.Render(strings.Join(parts, "    "))
}
