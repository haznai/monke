package app

import (
	"regexp"
	"strings"
	"testing"
)

var ansiEscapePattern = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func plainText(s string) string {
	return ansiEscapePattern.ReplaceAllString(s, "")
}

func TestTypingHeaderUsesShortBrandAndRightAlignedStatus(t *testing.T) {
	m := NewTypingModel([]string{"th", "he"}, TestConfig{
		Mode:        "ngram",
		Scope:       50,
		NgramLesson: 3,
		NgramTotal:  25,
	}, 80, 24)

	header := plainText(m.renderHeader())
	if !strings.HasPrefix(header, "monke") {
		t.Fatalf("header = %q, want short brand prefix", header)
	}
	if !strings.HasSuffix(header, "3/25  top 50") {
		t.Fatalf("header = %q, want right-aligned status suffix", header)
	}
	if header != "monke"+strings.Repeat(" ", 53)+"3/25  top 50" {
		t.Fatalf("header spacing changed: %q", header)
	}

	for _, removed := range []string{"monkeytype-tui", "lesson"} {
		if strings.Contains(header, removed) {
			t.Fatalf("header still contains %q: %q", removed, header)
		}
	}
}

func TestTypingHeaderOmitsRedundantQuoteStatus(t *testing.T) {
	m := NewTypingModel([]string{"short", "quote"}, TestConfig{Mode: "quote"}, 80, 24)

	header := plainText(m.renderHeader())
	if header != "monke" {
		t.Fatalf("header = %q, want %q", header, "monke")
	}
	if strings.Contains(header, "quote") {
		t.Fatalf("header still contains redundant quote mode: %q", header)
	}
}
