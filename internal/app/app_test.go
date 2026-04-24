package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultDataDirUsesUserConfigDir(t *testing.T) {
	t.Setenv("MONKEYTYPE_TUI_HOME", "")
	t.Setenv("HOME", t.TempDir())

	configDir, err := os.UserConfigDir()
	if err != nil {
		t.Fatalf("UserConfigDir: %v", err)
	}

	want := filepath.Join(configDir, "monkeytype-tui")
	if got := defaultDataDir(); got != want {
		t.Fatalf("defaultDataDir() = %q, want %q", got, want)
	}
}

func TestDefaultLLMLogPathUsesDataDir(t *testing.T) {
	t.Setenv("MONKEYTYPE_TUI_HOME", "")
	t.Setenv("MONKEYTYPE_TUI_LLM_LOG_PATH", "")
	t.Setenv("HOME", t.TempDir())

	want := filepath.Join(defaultDataDir(), "llm-calls.sqlite3")
	if got := defaultLLMLogPath(); got != want {
		t.Fatalf("defaultLLMLogPath() = %q, want %q", got, want)
	}
}
