package menu

import (
	"fmt"
	"strings"

	"github.com/hazn/monkeytype-tui/internal/theme"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// SelectMsg is sent when the user confirms their selection.
type SelectMsg struct {
	Mode  string
	Value int
	Scope int // top N ngrams to use (ngram mode only)
}

// Section tracks which menu section the cursor is in.
type Section int

const (
	SectionMode Section = iota
	SectionValue
)

type modeOption struct {
	label string
	mode  string
}

type valueOption struct {
	label string
	value int
}

var (
	modes = []modeOption{
		{"quote", "quote"},
		{"ngram", "ngram"},
	}

	quoteValues = []valueOption{
		{"short", 0},
		{"medium", 1},
		{"long", 2},
		{"thicc", 3},
	}

	ngramScopeValues = []valueOption{
		{"top 50", 50},
		{"top 100", 100},
		{"top 150", 150},
		{"top 200", 200},
	}
)

type Model struct {
	section  Section
	modeIdx  int
	valueIdx int
}

func New() Model {
	return Model{
		section:  SectionMode,
		modeIdx:  0,
		valueIdx: 0, // default: short quote
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
		}
	case SectionValue:
		if m.valueIdx > 0 {
			m.valueIdx--
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
	}
}

func (m *Model) moveUp() {
	if m.section > SectionMode {
		m.section--
	}
}

func (m *Model) moveDown() {
	if m.section < SectionValue {
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
	case "ngram":
		return ngramScopeValues
	case "quote":
		fallthrough
	default:
		return quoteValues
	}
}

func (m Model) select_() tea.Cmd {
	vals := m.currentValues()
	mode := m.currentMode()
	value := vals[m.valueIdx].value

	msg := SelectMsg{
		Mode:  mode,
		Value: value,
	}
	if mode == "ngram" {
		msg.Scope = value
	}

	return func() tea.Msg { return msg }
}

const menuWidth = theme.ScreenWidth
const menuHeight = theme.ScreenHeight

func (m Model) View() string {
	var b strings.Builder

	// Title
	b.WriteString(theme.Title.Render("monke"))
	b.WriteString("\n\n")

	// Mode
	b.WriteString(theme.MenuHeader.Render("mode"))
	b.WriteString("\n")
	b.WriteString(m.renderRow(SectionMode, func(i int) string {
		return modes[i].label
	}, len(modes), m.modeIdx))
	b.WriteString("\n\n")

	// Mode-specific value. Quote length and ngram scope are self-explanatory,
	// so keep the menu flat instead of adding redundant row headers.
	vals := m.currentValues()
	b.WriteString(m.renderRow(SectionValue, func(i int) string {
		return vals[i].label
	}, len(vals), m.valueIdx))
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
		Padding(theme.ScreenVerticalPadding, theme.ScreenHorizontalPadding).
		Width(menuWidth).
		Height(menuHeight).
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
