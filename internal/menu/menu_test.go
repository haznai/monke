package menu

import (
	"regexp"
	"strings"
	"testing"

	"github.com/hazn/monkeytype-tui/internal/theme"
)

var ansiEscapePattern = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func plainText(s string) string {
	return ansiEscapePattern.ReplaceAllString(s, "")
}

func TestNew_DefaultSelectionIsShortQuote(t *testing.T) {
	cmd := New().select_()
	msg := cmd()
	selectMsg, ok := msg.(SelectMsg)
	if !ok {
		t.Fatalf("message type = %T, want SelectMsg", msg)
	}

	if selectMsg.Mode != "quote" {
		t.Fatalf("mode = %q, want quote", selectMsg.Mode)
	}
	if selectMsg.Value != 0 {
		t.Fatalf("quote value = %d, want 0 (short)", selectMsg.Value)
	}
}

func TestMenuOnlyOffersQuoteAndNgramModes(t *testing.T) {
	view := New().View()

	for _, want := range []string{"quote", "ngram"} {
		if !strings.Contains(view, want) {
			t.Fatalf("menu view missing mode %q:\n%s", want, view)
		}
	}
	for _, removed := range []string{"time", "words"} {
		if strings.Contains(view, removed) {
			t.Fatalf("menu view still contains removed mode %q:\n%s", removed, view)
		}
	}
}

func TestMenuUsesTypingFrameWidthAndPadding(t *testing.T) {
	view := plainText(New().View())
	lines := strings.Split(view, "\n")
	if len(lines) < 2 {
		t.Fatalf("menu view has too few lines: %q", view)
	}
	if len(lines) != theme.ScreenHeight {
		t.Fatalf("menu view height = %d, want %d", len(lines), theme.ScreenHeight)
	}
	if strings.TrimSpace(lines[0]) != "" {
		t.Fatalf("menu should start with one padded blank line, got %q", lines[0])
	}

	titleLine := lines[1]
	if !strings.HasPrefix(titleLine, "  monke") {
		t.Fatalf("menu title padding = %q, want same left padding as typing view", titleLine)
	}
	if len(titleLine) != menuWidth {
		t.Fatalf("menu title line width = %d, want %d", len(titleLine), menuWidth)
	}
	if strings.Contains(titleLine, "monkeytype-tui") {
		t.Fatalf("menu title still uses old app name: %q", titleLine)
	}
}

func TestNgramSelectionUsesScopeAsTheOnlyValue(t *testing.T) {
	m := New()
	m.moveRight() // quote -> ngram

	cmd := m.select_()
	msg := cmd()
	selectMsg, ok := msg.(SelectMsg)
	if !ok {
		t.Fatalf("message type = %T, want SelectMsg", msg)
	}

	if selectMsg.Mode != "ngram" {
		t.Fatalf("mode = %q, want ngram", selectMsg.Mode)
	}
	if selectMsg.Value != 50 {
		t.Fatalf("value = %d, want 50", selectMsg.Value)
	}
	if selectMsg.Scope != 50 {
		t.Fatalf("scope = %d, want 50", selectMsg.Scope)
	}
}

func TestNgramMenuShowsScopeValuesWithoutAHeaderOrTypeChoice(t *testing.T) {
	m := New()
	m.moveRight() // quote -> ngram
	view := m.View()

	for _, want := range []string{"top 50", "top 100", "top 150", "top 200"} {
		if !strings.Contains(view, want) {
			t.Fatalf("ngram menu missing %q:\n%s", want, view)
		}
	}
	for _, removed := range []string{"scope", "bigrams", "trigrams"} {
		if strings.Contains(view, removed) {
			t.Fatalf("ngram menu still contains removed text %q:\n%s", removed, view)
		}
	}
}

func TestMenuDoesNotNavigatePastTheValueRow(t *testing.T) {
	m := New()
	m.moveRight() // quote -> ngram
	m.moveDown()
	m.moveDown()

	if m.section != SectionValue {
		t.Fatalf("section = %d, want SectionValue", m.section)
	}
}
