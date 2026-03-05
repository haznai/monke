package menu

import (
	"fmt"
	"strings"

	"github.com/hazn/monkeytype-tui/internal/theme"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// SelectMsg is sent when the user confirms their selection
type SelectMsg struct {
	Mode     string
	Value    int
	WordList string
}

// Section tracks which menu section the cursor is in
type Section int

const (
	SectionMode Section = iota
	SectionValue
	SectionWordList
)

type modeOption struct {
	label string
	mode  string
}

type valueOption struct {
	label string
	value int
}

type wordListOption struct {
	label string
	name  string
}

var (
	modes = []modeOption{
		{"words", "words"},
		{"time", "time"},
		{"quote", "quote"},
	}

	wordValues = []valueOption{
		{"10", 10},
		{"25", 25},
		{"50", 50},
		{"100", 100},
	}

	timeValues = []valueOption{
		{"15s", 15},
		{"30s", 30},
		{"60s", 60},
		{"120s", 120},
	}

	quoteValues = []valueOption{
		{"short", 0},
		{"medium", 1},
		{"long", 2},
		{"thicc", 3},
	}

	wordLists = []wordListOption{
		{"english 200", "english"},
		{"english 1k", "english_1k"},
		{"english 5k", "english_5k"},
		{"english 10k", "english_10k"},
	}
)

type Model struct {
	section      Section
	modeIdx      int
	valueIdx     int
	wordListIdx  int
}

func New() Model {
	return Model{
		section:     SectionMode,
		modeIdx:     0,
		valueIdx:    1, // default: 25 words or 30s
		wordListIdx: 1, // default: english_1k
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "left", "h":
			m.moveLeft()
		case "right", "l":
			m.moveRight()
		case "up", "k":
			m.moveUp()
		case "down", "j":
			m.moveDown()
		case "enter":
			return m, m.select_()
		case "tab":
			m.nextSection()
		case "shift+tab":
			m.prevSection()
		}
	}
	return m, nil
}

func (m *Model) moveLeft() {
	switch m.section {
	case SectionMode:
		if m.modeIdx > 0 {
			m.modeIdx--
			m.valueIdx = 0
		}
	case SectionValue:
		if m.valueIdx > 0 {
			m.valueIdx--
		}
	case SectionWordList:
		if m.wordListIdx > 0 {
			m.wordListIdx--
		}
	}
}

func (m *Model) moveRight() {
	switch m.section {
	case SectionMode:
		if m.modeIdx < len(modes)-1 {
			m.modeIdx++
			m.valueIdx = 0
		}
	case SectionValue:
		vals := m.currentValues()
		if m.valueIdx < len(vals)-1 {
			m.valueIdx++
		}
	case SectionWordList:
		if m.wordListIdx < len(wordLists)-1 {
			m.wordListIdx++
		}
	}
}

func (m *Model) moveUp() {
	if m.section > SectionMode {
		m.section--
	}
}

func (m *Model) moveDown() {
	max := SectionWordList
	if modes[m.modeIdx].mode == "quote" {
		max = SectionValue // no word list for quotes
	}
	if m.section < max {
		m.section++
	}
}

func (m *Model) nextSection() {
	m.moveDown()
}

func (m *Model) prevSection() {
	m.moveUp()
}

func (m Model) currentValues() []valueOption {
	switch modes[m.modeIdx].mode {
	case "words":
		return wordValues
	case "time":
		return timeValues
	case "quote":
		return quoteValues
	default:
		return wordValues
	}
}

func (m Model) select_() tea.Cmd {
	vals := m.currentValues()
	wl := "english_1k"
	if modes[m.modeIdx].mode != "quote" && m.wordListIdx < len(wordLists) {
		wl = wordLists[m.wordListIdx].name
	}
	return func() tea.Msg {
		return SelectMsg{
			Mode:     modes[m.modeIdx].mode,
			Value:    vals[m.valueIdx].value,
			WordList: wl,
		}
	}
}

func (m Model) View() string {
	var b strings.Builder

	// Title
	b.WriteString(theme.Title.Render("monkeytype-tui"))
	b.WriteString("\n\n")

	// Mode row
	b.WriteString(m.renderRow("mode", SectionMode, func(i int) string {
		return modes[i].label
	}, len(modes), m.modeIdx))
	b.WriteString("\n\n")

	// Value row
	vals := m.currentValues()
	b.WriteString(m.renderRow("", SectionValue, func(i int) string {
		return vals[i].label
	}, len(vals), m.valueIdx))
	b.WriteString("\n\n")

	// Word list row (not shown for quotes)
	if modes[m.modeIdx].mode != "quote" {
		b.WriteString(m.renderRow("words", SectionWordList, func(i int) string {
			return wordLists[i].label
		}, len(wordLists), m.wordListIdx))
		b.WriteString("\n\n")
	}

	// Footer
	footer := fmt.Sprintf(
		"%s navigate  %s select  %s quit",
		theme.FooterKey.Render("arrows"),
		theme.FooterKey.Render("enter"),
		theme.FooterKey.Render("esc"),
	)
	b.WriteString(theme.FooterStyle.Render(footer))

	return lipgloss.NewStyle().
		Padding(2, 4).
		Render(b.String())
}

func (m Model) renderRow(label string, section Section, getText func(int) string, count int, selected int) string {
	var parts []string

	if label != "" {
		parts = append(parts, theme.Subtitle.Render(label+"  "))
	}

	for i := 0; i < count; i++ {
		text := getText(i)
		if i == selected {
			if m.section == section {
				parts = append(parts, theme.MenuSelected.Render(text))
			} else {
				parts = append(parts, theme.StatValueDim.Render(text))
			}
		} else {
			parts = append(parts, theme.MenuOption.Render(text))
		}
		if i < count-1 {
			parts = append(parts, theme.DimText.Render("  "))
		}
	}

	return strings.Join(parts, "")
}
