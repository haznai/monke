package app

import (
	"fmt"
	"strings"

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
	result    stats.TestResult
	config    TestConfig
	isPB      bool
	width     int
	height    int
}

func NewResultsModel(result stats.TestResult, config TestConfig, isPB bool, width, height int) ResultsModel {
	return ResultsModel{
		result: result,
		config: config,
		isPB:   isPB,
		width:  width,
		height: height,
	}
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

	// Title
	b.WriteString(theme.Title.Render("results"))
	b.WriteString("\n\n")

	// Main stats in a grid
	maxWidth := min(m.width-8, 76)

	// Row 1: WPM and Raw WPM
	row1Left := m.renderStat("wpm", fmt.Sprintf("%.0f", m.result.WPM))
	row1Right := m.renderStat("raw", fmt.Sprintf("%.0f", m.result.RawWPM))
	b.WriteString(m.twoColumn(row1Left, row1Right, maxWidth))
	b.WriteString("\n")

	// Row 2: Corrected WPM and Accuracy
	row2Left := m.renderStat("corrected", fmt.Sprintf("%.0f", m.result.CorrectedWPM))
	row2Right := m.renderStat("accuracy", fmt.Sprintf("%.1f%%", m.result.Accuracy))
	b.WriteString(m.twoColumn(row2Left, row2Right, maxWidth))
	b.WriteString("\n")

	// Row 3: Consistency and Delta
	row3Left := m.renderStat("consistency", fmt.Sprintf("%.0f%%", m.result.Consistency))
	delta := m.result.CorrectionDelta
	deltaStr := fmt.Sprintf("+%.0f wpm", delta)
	if delta <= 0 {
		deltaStr = fmt.Sprintf("%.0f wpm", delta)
	}
	row3Right := m.renderStat("delta", deltaStr)
	b.WriteString(m.twoColumn(row3Left, row3Right, maxWidth))
	b.WriteString("\n\n")

	// Divider
	divider := theme.DimText.Render(strings.Repeat("━", min(maxWidth, 40)))
	b.WriteString(divider)
	b.WriteString("\n\n")

	// Pass/fail
	statusLine := fmt.Sprintf(
		"correct %d/%d words",
		m.result.CorrectWords,
		m.result.TotalWords,
	)
	if m.result.Passed {
		b.WriteString(theme.PassedText.Render(statusLine + "  PASSED"))
	} else {
		b.WriteString(theme.FailedText.Render(statusLine + "  FAILED"))
	}

	// Personal best indicator
	if m.isPB {
		b.WriteString("  ")
		b.WriteString(theme.PassedText.Render("NEW PB!"))
	}

	b.WriteString("\n")

	// Duration
	b.WriteString(theme.DimText.Render(fmt.Sprintf("%.1fs", m.result.Duration.Seconds())))
	b.WriteString("\n\n")

	// Footer
	footer := fmt.Sprintf(
		"%s restart  %s next  %s menu",
		theme.FooterKey.Render("tab"),
		theme.FooterKey.Render("enter"),
		theme.FooterKey.Render("esc"),
	)
	b.WriteString(theme.FooterStyle.Render(footer))

	return lipgloss.NewStyle().
		Padding(1, 2).
		Width(min(m.width-4, 80)).
		Render(b.String())
}

func (m ResultsModel) renderStat(label, value string) string {
	return fmt.Sprintf(
		"%s  %s",
		theme.StatLabel.Render(label),
		theme.StatValue.Render(value),
	)
}

func (m ResultsModel) twoColumn(left, right string, maxWidth int) string {
	leftWidth := lipgloss.Width(left)
	rightWidth := lipgloss.Width(right)
	gap := max(2, maxWidth-leftWidth-rightWidth)
	return left + strings.Repeat(" ", gap) + right
}
