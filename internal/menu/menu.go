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
	Mode      string
	Value     int
	WordList  string
	NgramType string // "bigrams" or "trigrams" (ngram mode only)
	Scope     int    // top N ngrams to use (ngram mode only)
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
		{"quote", "quote"},
		{"ngram", "ngram"},
		{"time", "time"},
		{"words", "words"},
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

	ngramTypeValues = []valueOption{
		{"bigrams", 0},
		{"trigrams", 1},
	}

	ngramScopeValues = []wordListOption{
		{"top 50", "50"},
		{"top 100", "100"},
		{"top 150", "150"},
		{"top 200", "200"},
	}

	wordLists = []wordListOption{
		{"english 200", "english"},
		{"english 1k", "english_1k"},
		{"english 5k", "english_5k"},
		{"english 10k", "english_10k"},
	}
)

type Model struct {
	section     Section
	modeIdx     int
	valueIdx    int
	wordListIdx int
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
			m.moveRight()
		case "shift+tab":
			m.moveLeft()
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
			m.wordListIdx = 0
		}
	case SectionValue:
		if m.valueIdx > 0 {
			m.valueIdx--
		}
	case SectionWordList:
		wlOpts := m.currentWordListOptions()
		if wlOpts != nil && m.wordListIdx > 0 {
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
			m.wordListIdx = 0
		}
	case SectionValue:
		vals := m.currentValues()
		if m.valueIdx < len(vals)-1 {
			m.valueIdx++
		}
	case SectionWordList:
		wlOpts := m.currentWordListOptions()
		if wlOpts != nil && m.wordListIdx < len(wlOpts)-1 {
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

func (m Model) currentMode() string {
	return modes[m.modeIdx].mode
}

func (m Model) currentValues() []valueOption {
	switch m.currentMode() {
	case "words":
		return wordValues
	case "time":
		return timeValues
	case "quote":
		return quoteValues
	case "ngram":
		return ngramTypeValues
	default:
		return wordValues
	}
}

// currentWordListOptions returns the word list / scope options for the third row,
// or nil if the current mode has no third row.
func (m Model) currentWordListOptions() []wordListOption {
	switch m.currentMode() {
	case "words", "time":
		return wordLists
	case "ngram":
		return ngramScopeValues
	default:
		return nil
	}
}

func (m Model) select_() tea.Cmd {
	vals := m.currentValues()
	mode := m.currentMode()

	msg := SelectMsg{
		Mode:  mode,
		Value: vals[m.valueIdx].value,
	}

	switch mode {
	case "ngram":
		if m.valueIdx == 0 {
			msg.NgramType = "bigrams"
		} else {
			msg.NgramType = "trigrams"
		}
		scopes := ngramScopeValues
		if m.wordListIdx < len(scopes) {
			// Parse scope from the name field ("50", "100", etc.)
			switch scopes[m.wordListIdx].name {
			case "50":
				msg.Scope = 50
			case "100":
				msg.Scope = 100
			case "150":
				msg.Scope = 150
			case "200":
				msg.Scope = 200
			}
		}
	case "quote":
		// no word list
	default:
		if m.wordListIdx < len(wordLists) {
			msg.WordList = wordLists[m.wordListIdx].name
		} else {
			msg.WordList = "english_1k"
		}
	}

	return func() tea.Msg { return msg }
}

const menuWidth = 60

func (m Model) View() string {
	var b strings.Builder

	// Title
	b.WriteString(theme.Title.Render("monkeytype-tui"))
	b.WriteString("\n\n")

	// Mode
	b.WriteString(theme.MenuHeader.Render("mode"))
	b.WriteString("\n")
	b.WriteString(m.renderRow(SectionMode, func(i int) string {
		return modes[i].label
	}, len(modes), m.modeIdx))
	b.WriteString("\n\n")

	// Value
	vals := m.currentValues()
	b.WriteString(m.renderRow(SectionValue, func(i int) string {
		return vals[i].label
	}, len(vals), m.valueIdx))
	b.WriteString("\n\n")

	// Word list / scope (always reserve the line to prevent layout shift)
	wlOpts := m.currentWordListOptions()
	if wlOpts != nil {
		label := "words"
		if m.currentMode() == "ngram" {
			label = "scope"
		}
		b.WriteString(theme.MenuHeader.Render(label))
		b.WriteString("\n")
		b.WriteString(m.renderRow(SectionWordList, func(i int) string {
			return wlOpts[i].label
		}, len(wlOpts), m.wordListIdx))
	}
	b.WriteString("\n\n")

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
		Width(menuWidth).
		Render(b.String())
}

func (m Model) renderRow(section Section, getText func(int) string, count int, selected int) string {
	var parts []string

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
