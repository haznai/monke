package theme

import "github.com/charmbracelet/lipgloss"

// MonkeyType-inspired dark theme
var (
	// Base colors
	BgColor      = lipgloss.Color("#2c2e34")
	TextColor    = lipgloss.Color("#d4d4d8")
	DimColor     = lipgloss.Color("#646669")
	AccentColor  = lipgloss.Color("#e2b714") // MonkeyType's signature yellow
	ErrorColor   = lipgloss.Color("#ca4754")
	CorrectColor = lipgloss.Color("#7b8496")
	CurrentColor = lipgloss.Color("#d1d0c5")
	CursorColor  = lipgloss.Color("#e2b714")
	PassedColor  = lipgloss.Color("#7ec87e")
	FailedColor  = lipgloss.Color("#ca4754")

	// Text styles
	DimText = lipgloss.NewStyle().
		Foreground(DimColor)

	CurrentWord = lipgloss.NewStyle().
			Foreground(DimColor)

	CorrectChar = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#000000"))

	IncorrectChar = lipgloss.NewStyle().
			Foreground(ErrorColor).
			Bold(true)

	// Incorrect word that was already submitted
	IncorrectWord = lipgloss.NewStyle().
			Foreground(ErrorColor)

	// Correct word that was already submitted
	CompletedWord = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#000000"))

	// Cursor/caret on current character
	Cursor = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#000000")).
		Background(CursorColor)

	// Header
	Title = lipgloss.NewStyle().
		Foreground(AccentColor).
		Bold(true)

	Subtitle = lipgloss.NewStyle().
			Foreground(DimColor)

	// Stats display
	StatLabel = lipgloss.NewStyle().
			Foreground(DimColor)

	StatValue = lipgloss.NewStyle().
			Foreground(AccentColor).
			Bold(true)

	StatValueDim = lipgloss.NewStyle().
			Foreground(TextColor)

	// Results
	PassedText = lipgloss.NewStyle().
			Foreground(PassedColor).
			Bold(true)

	FailedText = lipgloss.NewStyle().
			Foreground(FailedColor).
			Bold(true)

	// Menu
	MenuSelected = lipgloss.NewStyle().
			Foreground(AccentColor).
			Bold(true)

	MenuOption = lipgloss.NewStyle().
			Foreground(DimColor)

	MenuHeader = lipgloss.NewStyle().
			Foreground(TextColor).
			Bold(true)

	// Footer
	FooterStyle = lipgloss.NewStyle().
			Foreground(DimColor)

	FooterKey = lipgloss.NewStyle().
			Foreground(AccentColor)

	// Borders and containers
	Container = lipgloss.NewStyle().
			Padding(1, 2)
)
